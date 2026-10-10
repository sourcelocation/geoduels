package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"geoduels/internal/parties"
	"geoduels/pkg/contracts"
	"geoduels/pkg/matchkind"
	"geoduels/pkg/observability"
	"geoduels/pkg/pgnotify"
)

const defaultPartyTTL = 2 * time.Hour

// requirePlayableUser authenticates a player who may play now: signed in, not banned, with a
// nickname and an account that needs no migration.
func (a *api) requirePlayableUser(c echo.Context) (string, error) {
	claims, err := a.liveClaims(c.Request())
	if err != nil {
		return "", plainTextError(c, http.StatusUnauthorized, "unauthorized")
	}
	identity, err := a.accounts.GetIdentity(claims.Sub)
	if err != nil {
		return "", plainTextError(c, http.StatusUnauthorized, "identity not found")
	}
	switch {
	case identity.IsBanned:
		return "", writeAPIError(c, http.StatusForbidden, "account_banned", "user is banned")
	case identity.NicknameRequired:
		return "", plainTextError(c, http.StatusForbidden, "nickname required")
	case identity.AuthMigrationRequired:
		return "", plainTextError(c, http.StatusForbidden, "connect discord to continue")
	}
	return claims.Sub, nil
}

func (a *api) createParty(c echo.Context) error {
	userID, err := a.requirePlayableUser(c)
	if err != nil {
		return err
	}
	var req contracts.PartyCreateRequest
	if err := decodeJSONBody(c.Request(), &req); err != nil {
		return plainTextError(c, http.StatusBadRequest, "invalid payload")
	}
	mode := req.Mode
	if mode == "" {
		mode = contracts.ModeDuel
	}
	if _, ok := matchkind.ForParty(mode); !ok {
		return plainTextError(c, http.StatusBadRequest, "unsupported party mode")
	}
	snap, err := a.parties.CreateParty(userID, mode, req.MapScope, defaultPartyTTL)
	if err != nil {
		observability.Log("error", "create party failed", map[string]any{"userId": userID, "mode": string(mode), "error": err.Error()})
		return plainTextError(c, http.StatusInternalServerError, "party unavailable")
	}
	if req.Config.Ruleset != "" || req.Config.MapID != "" || req.Config.MapKey != "" || req.Config.RoundTimerMode != "" || req.Config.RoundTimeLimitMS > 0 || req.Config.PressureTimeLimitMS > 0 || req.Config.MultiplierMode != "" {
		if req.Config.MapID == "" && req.Config.MapKey == "" {
			req.Config.MapID = snap.Config.MapID
		}
		snap, err = a.parties.SetPartyConfig(snap.ID, req.Config)
		if err != nil {
			observability.Log("error", "create party settings save failed", map[string]any{"userId": userID, "partyId": snap.ID, "error": err.Error()})
			return plainTextError(c, http.StatusInternalServerError, "party unavailable")
		}
	}
	if changed, _ := a.parties.TouchMemberSeen(snap.ID, userID); changed {
		a.publishPartyChanged(c.Request().Context(), snap.ID)
	}
	return writeJSON(c, map[string]string{"id": snap.ID, "inviteCode": snap.InviteCode})
}

func (a *api) joinParty(c echo.Context) error {
	userID, err := a.requirePlayableUser(c)
	if err != nil {
		return err
	}
	snap, found, err := a.parties.GetPartyByInviteCode(strings.TrimSpace(c.Param("code")))
	if err != nil {
		return plainTextError(c, http.StatusInternalServerError, "party unavailable")
	}
	if !found {
		return plainTextError(c, http.StatusNotFound, "party not found")
	}
	snap, err = a.parties.JoinParty(snap.ID, userID)
	if err != nil {
		return plainTextError(c, http.StatusConflict, err.Error())
	}
	_, _ = a.parties.TouchMemberSeen(snap.ID, userID)
	a.publishPartyChanged(c.Request().Context(), snap.ID)
	return writeJSON(c, map[string]string{"id": snap.ID, "inviteCode": snap.InviteCode})
}

func (a *api) setPartySettings(ctx context.Context, id, userID string, req partySettingsRequest) error {
	snap, found, err := a.parties.GetPartyByID(id)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "party unavailable")
	}
	if !found {
		return echo.NewHTTPError(http.StatusNotFound, "party not found")
	}
	if snap.OwnerUserID != userID {
		return echo.NewHTTPError(http.StatusForbidden, "forbidden")
	}
	if snap.State != contracts.PartyOpen {
		return echo.NewHTTPError(http.StatusConflict, "party settings are locked")
	}
	if req.Mode != "" {
		if _, ok := matchkind.ForParty(req.Mode); !ok {
			return echo.NewHTTPError(http.StatusBadRequest, "unsupported party mode")
		}
	}
	if req.Mode != "" && req.Mode != snap.Mode {
		if err := a.parties.SetPartyMode(snap.ID, req.Mode); err != nil {
			return echo.NewHTTPError(http.StatusBadGateway, "party settings unavailable")
		}
	}
	if _, err := a.parties.SetPartyConfig(snap.ID, req.Config); err != nil {
		if errors.Is(err, parties.ErrPartyMapUnavailable) {
			return echo.NewHTTPError(http.StatusUnprocessableEntity, err.Error())
		}
		observability.Log("error", "update party settings save failed", map[string]any{"userId": userID, "partyId": snap.ID, "mapId": req.Config.MapID, "error": err.Error()})
		return echo.NewHTTPError(http.StatusBadGateway, "party settings unavailable")
	}
	a.publishPartyChanged(ctx, snap.ID)
	return nil
}

func (a *api) partyWS(c echo.Context) error {
	r := c.Request()
	claims, err := a.liveClaims(r)
	if err != nil {
		return plainTextError(c, http.StatusUnauthorized, "unauthorized")
	}
	if banned, err := a.accountBanned(claims.Sub); err != nil || banned {
		return plainTextError(c, http.StatusForbidden, "forbidden")
	}
	userID := claims.Sub
	partyID := strings.TrimSpace(c.Param("id"))
	snap, ok, err := a.parties.GetPartyByID(partyID)
	if err != nil {
		return plainTextError(c, http.StatusBadGateway, "party unavailable")
	}
	conn, err := a.live.upgrader.Upgrade(c.Response().Writer, r, nil)
	if err != nil {
		return nil
	}
	defer conn.Close()
	w := &socketWriter{conn: conn}
	if !ok || !partyHasMember(snap, userID) {
		w.send("party_error", map[string]string{"message": "You left this party"})
		return nil
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	touch := func() {
		if changed, err := a.parties.TouchMemberSeen(partyID, userID); err == nil && changed {
			a.publishPartyChanged(context.Background(), partyID)
		}
		a.touchViewerPresence(context.Background(), userID)
	}
	conn.SetReadLimit(16 * 1024)
	_ = conn.SetReadDeadline(time.Now().Add(queueReadTTL))
	conn.SetPongHandler(func(string) error {
		touch()
		return conn.SetReadDeadline(time.Now().Add(queueReadTTL))
	})
	commands := make(chan []byte, 16)
	go func() {
		defer cancel()
		for {
			_, data, err := conn.ReadMessage()
			if err != nil {
				return
			}
			select {
			case commands <- data:
			case <-ctx.Done():
				return
			}
		}
	}()
	events, unsubscribe := a.events.Subscribe(pgnotify.PartyTopic(partyID))
	defer unsubscribe()
	defer func() {
		if cleared, err := a.parties.ClearMemberSeen(partyID, userID); err == nil && cleared {
			a.publishPartyChanged(context.Background(), partyID)
		}
	}()
	touch()

	var last contracts.PartySnapshot
	lastFingerprint := ""
	revision := int64(0)
	lastMatchID := ""
	refreshParty := func() bool {
		next, ok, err := a.parties.GetPartyByID(partyID)
		if err != nil || !ok {
			w.send("party_error", map[string]string{"message": "Party unavailable"})
			return false
		}
		if !partyHasMember(next, userID) {
			w.send("party_error", map[string]string{"message": "You left this party"})
			return false
		}
		if fingerprint := partyFingerprint(next); fingerprint != lastFingerprint {
			revision++
			if revision == 1 {
				w.send("party_snapshot", next)
			} else {
				w.send("party_patch", partyPatch(last, next, revision))
			}
			last, lastFingerprint = next, fingerprint
		}
		// A member seated in the party's new match goes to it.
		if next.ActiveMatchID != "" && next.ActiveMatchID != lastMatchID && partySeated(next, userID) {
			lastMatchID = next.ActiveMatchID
			if !w.send("match_found", contracts.ActiveMatchSummary{MatchID: next.ActiveMatchID, Status: contracts.MatchViewStarting}) {
				return false
			}
		}
		return next.State == contracts.PartyOpen || next.State == contracts.PartyInMatch
	}
	if !refreshParty() {
		return nil
	}

	presenceTicker := time.NewTicker(10 * time.Second)
	defer presenceTicker.Stop()
	pingTicker := time.NewTicker(queuePingEvery)
	defer pingTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case data := <-commands:
			var command partyCommand
			if err := json.Unmarshal(data, &command); err != nil || command.RequestID == "" || len(command.RequestID) > 128 {
				if !w.send("party_command_result", partyCommandResult{RequestID: command.RequestID, Error: "invalid command"}) {
					return nil
				}
				continue
			}
			commandCtx, commandCancel := context.WithTimeout(ctx, 20*time.Second)
			_, authErr := a.liveClaims(r)
			if authErr != nil {
				err = errors.New("session expired; reconnect to continue")
			} else {
				err = a.executePartyCommand(commandCtx, partyID, userID, command)
			}
			commandCancel()
			result := partyCommandResult{RequestID: command.RequestID, OK: err == nil}
			if err != nil {
				result.Error = partyErrorMessage(err)
			}
			if !w.send("party_command_result", result) || authErr != nil {
				return nil
			}
			if err == nil {
				if command.Type == "leave" {
					return nil
				}
				if !refreshParty() {
					return nil
				}
			}
		case <-events:
			if !refreshParty() {
				return nil
			}
		case <-presenceTicker.C:
			touch()
			if !refreshParty() {
				return nil
			}
		case <-pingTicker.C:
			if _, err := a.liveClaims(r); err != nil {
				return nil
			}
			if !w.ping() {
				return nil
			}
		}
	}
}

// startPartyMatch starts the party's match with every member seated, once they all have it open.
func (a *api) startPartyMatch(ctx context.Context, partyID, userID string) error {
	snap, ok, err := a.parties.GetPartyByID(partyID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, "party unavailable")
	}
	if !ok {
		return echo.NewHTTPError(http.StatusNotFound, "party not found")
	}
	if snap.OwnerUserID != userID {
		return echo.NewHTTPError(http.StatusForbidden, "forbidden")
	}
	if snap.State != contracts.PartyOpen {
		return echo.NewHTTPError(http.StatusConflict, "party is not open")
	}
	kind, ok := matchkind.ForParty(snap.Mode)
	if !ok {
		return echo.NewHTTPError(http.StatusConflict, "unsupported party mode")
	}
	if err := requirePartyPresence(snap); err != nil {
		return echo.NewHTTPError(http.StatusConflict, err.Error())
	}
	spec := matchkind.Of(kind)
	seats := make([]matchkind.Seat, 0, len(snap.Members))
	for _, member := range snap.Members {
		seat := matchkind.Seat{UserID: member.UserID}
		if spec.Teams {
			seat.Team = normalizePartyTeam(member.TeamID)
		}
		seats = append(seats, seat)
	}
	if err := spec.CheckSeats(seats); err != nil {
		return echo.NewHTTPError(http.StatusConflict, err.Error())
	}
	if _, err := a.startMatch(ctx, startRequest{
		Kind: kind, Seats: seats, Config: snap.Config, PartyID: snap.ID, MapAccessUserID: snap.OwnerUserID,
		ReturnTarget: &contracts.MatchReturnTarget{Kind: contracts.MatchReturnParty, PartyID: snap.ID},
	}); err != nil {
		observability.Log("warn", "party match start failed", map[string]any{"partyId": snap.ID, "error": err.Error()})
		return echo.NewHTTPError(http.StatusConflict, a.startErrorMessage(err))
	}
	return nil
}

func requirePartyPresence(snap contracts.PartySnapshot) error {
	missing := make([]string, 0, len(snap.Members))
	for _, member := range snap.Members {
		if !member.Connected {
			name := strings.TrimSpace(member.DisplayName)
			if name == "" {
				name = member.UserID
			}
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		return errors.New("all players must be in the party to start: " + strings.Join(missing, ", "))
	}
	return nil
}

func partySeated(snap contracts.PartySnapshot, userID string) bool {
	for _, member := range snap.Members {
		if member.UserID == userID {
			return member.InActiveMatch
		}
	}
	return false
}

func partyHasMember(snap contracts.PartySnapshot, userID string) bool {
	for _, member := range snap.Members {
		if member.UserID == userID {
			return true
		}
	}
	return false
}

func partyFingerprint(snap contracts.PartySnapshot) string {
	b, _ := json.Marshal(snap)
	return string(b)
}

func partyPatch(prev, next contracts.PartySnapshot, revision int64) contracts.PartyPatch {
	patch := contracts.PartyPatch{Revision: revision}
	if prev.State != next.State {
		v := next.State
		patch.State = &v
	}
	if prev.OwnerUserID != next.OwnerUserID {
		v := next.OwnerUserID
		patch.OwnerUserID = &v
	}
	if prev.Mode != next.Mode {
		v := next.Mode
		patch.Mode = &v
	}
	if prev.MapScope != next.MapScope {
		v := next.MapScope
		patch.MapScope = &v
	}
	if prev.MapName != next.MapName {
		v := next.MapName
		patch.MapName = &v
	}
	if prev.MapLocationCount != next.MapLocationCount {
		v := next.MapLocationCount
		patch.MapLocationCount = &v
	}
	if prev.Config != next.Config {
		v := next.Config
		patch.Config = &v
	}
	if prev.ActiveMatchID != next.ActiveMatchID {
		v := next.ActiveMatchID
		patch.ActiveMatchID = &v
	}
	if prev.LastMatchID != next.LastMatchID {
		v := next.LastMatchID
		patch.LastMatchID = &v
	}
	prevMembers := map[string]contracts.PartyMember{}
	nextMembers := map[string]contracts.PartyMember{}
	for _, member := range prev.Members {
		prevMembers[member.UserID] = member
	}
	for _, member := range next.Members {
		nextMembers[member.UserID] = member
		if partyMemberFingerprint(prevMembers[member.UserID]) != partyMemberFingerprint(member) {
			patch.UpsertMembers = append(patch.UpsertMembers, member)
		}
	}
	for id := range prevMembers {
		if _, ok := nextMembers[id]; !ok {
			patch.RemoveMemberIDs = append(patch.RemoveMemberIDs, id)
		}
	}
	return patch
}

func partyMemberFingerprint(member contracts.PartyMember) string {
	b, _ := json.Marshal(member)
	return string(b)
}

func normalizePartyTeam(teamID string) string {
	if strings.ToLower(strings.TrimSpace(teamID)) == "b" {
		return "b"
	}
	return "a"
}

// The party socket carries commands as well as authoritative state events. Commands are processed
// serially per connection and are never replayed on reconnect.
type partyCommand struct {
	RequestID string          `json:"requestId"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
}

type partyCommandResult struct {
	RequestID string `json:"requestId"`
	OK        bool   `json:"ok"`
	Error     string `json:"error,omitempty"`
}

type partySettingsRequest struct {
	Mode   contracts.MatchMode   `json:"mode"`
	Config contracts.MatchConfig `json:"config"`
}

func partyErrorMessage(err error) string {
	var httpErr *echo.HTTPError
	if errors.As(err, &httpErr) {
		return fmt.Sprint(httpErr.Message)
	}
	return err.Error()
}

func (a *api) executePartyCommand(ctx context.Context, partyID, userID string, command partyCommand) error {
	// Membership, ownership and account eligibility can change after the handshake.
	identity, err := a.accounts.GetIdentity(userID)
	if err != nil {
		return errors.New("identity not found")
	}
	switch {
	case identity.IsBanned:
		return errors.New("user is banned")
	case identity.NicknameRequired:
		return errors.New("nickname required")
	case identity.AuthMigrationRequired:
		return errors.New("connect discord to continue")
	}
	snap, found, err := a.parties.GetPartyByID(partyID)
	if err != nil {
		return errors.New("party unavailable")
	}
	if !found || !partyHasMember(snap, userID) {
		return errors.New("party membership required")
	}
	switch command.Type {
	case "team":
		var req contracts.PartyTeamRequest
		if json.Unmarshal(command.Payload, &req) != nil {
			return errors.New("invalid payload")
		}
		_, err = a.parties.SetPartyMemberTeam(partyID, userID, strings.TrimSpace(req.TeamID))
	case "shuffle_teams":
		_, err = a.parties.ShufflePartyTeams(partyID, userID)
	case "settings":
		var req partySettingsRequest
		if json.Unmarshal(command.Payload, &req) != nil {
			return errors.New("invalid payload")
		}
		return a.setPartySettings(ctx, partyID, userID, req)
	case "kick", "transfer_owner":
		var req contracts.PartyMemberRequest
		if json.Unmarshal(command.Payload, &req) != nil {
			return errors.New("invalid payload")
		}
		if command.Type == "kick" {
			_, err = a.parties.KickPartyMember(partyID, userID, strings.TrimSpace(req.UserID))
		} else {
			_, err = a.parties.TransferPartyOwner(partyID, userID, strings.TrimSpace(req.UserID))
		}
	case "leave":
		_, err = a.parties.LeaveParty(partyID, userID)
	case "start":
		return a.startPartyMatch(ctx, partyID, userID)
	default:
		return errors.New("unknown party command")
	}
	if err == nil {
		a.publishPartyChanged(ctx, partyID)
	}
	return err
}
