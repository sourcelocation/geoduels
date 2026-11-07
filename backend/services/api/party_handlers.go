package main

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"geoduels/internal/matches"
	"geoduels/internal/parties"
	"geoduels/internal/presence"
	"geoduels/pkg/gameplay"
	"geoduels/pkg/matchkind"
	"geoduels/pkg/observability"
)

const defaultPartyTTL = 2 * time.Hour

// requirePlayableUser authenticates a player who may play now: signed in, not banned, with a
// nickname and an account that needs no migration.
func (a *api) requirePlayableUser(c echo.Context) (string, error) {
	claims, err := a.authenticatedClaims(c.Request())
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

// partySnapshot is a party as its members see it.
type partySnapshot struct {
	ID               string                     `json:"id"`
	Revision         int64                      `json:"revision"`
	InviteCode       string                     `json:"inviteCode"`
	OwnerUserID      string                     `json:"ownerUserId"`
	State            string                     `json:"state"`
	Mode             matchkind.Mode             `json:"mode"`
	MapScope         string                     `json:"mapScope"`
	MapName          string                     `json:"mapName,omitempty"`
	MapLocationCount int                        `json:"mapLocationCount,omitempty"`
	Config           gameplay.ClientMatchConfig `json:"config,omitempty"`
	ActiveMatchID    string                     `json:"activeMatchId,omitempty"`
	LastMatchID      string                     `json:"lastMatchId,omitempty"`
	CreatedAt        time.Time                  `json:"createdAt"`
	ExpiresAt        time.Time                  `json:"expiresAt"`
	Members          []partyMember              `json:"members"`
}

type partyMember struct {
	UserID         string       `json:"userId"`
	DisplayName    string       `json:"displayName"`
	AvatarURL      string       `json:"avatarUrl,omitempty"`
	IsGuest        bool         `json:"isGuest,omitempty"`
	IsAdmin        bool         `json:"isAdmin,omitempty"`
	SelectedBadge  *playerBadge `json:"selectedBadge,omitempty"`
	TeamID         string       `json:"teamId,omitempty"`
	Role           string       `json:"role"`
	Ready          bool         `json:"ready"`
	Connected      bool         `json:"connected,omitempty"`
	PresenceStatus string       `json:"presenceStatus,omitempty"`
	InActiveMatch  bool         `json:"inActiveMatch,omitempty"`
	JoinedAt       time.Time    `json:"joinedAt"`
}

func newPartySnapshot(p parties.Party) partySnapshot {
	out := partySnapshot{
		ID: p.ID, Revision: p.Revision, InviteCode: p.InviteCode, OwnerUserID: p.OwnerUserID, State: string(p.State), Mode: p.Mode,
		MapScope: p.MapScope, MapName: p.MapName, MapLocationCount: p.MapLocationCount,
		Config: gameplay.NewClientMatchConfig(p.Config), ActiveMatchID: p.ActiveMatchID, LastMatchID: p.LastMatchID,
		CreatedAt: p.CreatedAt, ExpiresAt: p.ExpiresAt,
	}
	if p.Members != nil {
		out.Members = make([]partyMember, len(p.Members))
		for i, m := range p.Members {
			out.Members[i] = partyMember{
				UserID: m.UserID, DisplayName: m.DisplayName, AvatarURL: m.AvatarURL, IsGuest: m.IsGuest, IsAdmin: m.IsAdmin,
				SelectedBadge: newPlayerBadgeRef(m.SelectedBadge), TeamID: m.TeamID, Role: m.Role, Ready: m.Ready,
				Connected: m.Connected, PresenceStatus: string(m.PresenceStatus), InActiveMatch: m.InActiveMatch, JoinedAt: m.JoinedAt,
			}
		}
	}
	return out
}

type partyCreateRequest struct {
	Mode     matchkind.Mode             `json:"mode,omitempty"`
	MapScope string                     `json:"mapScope,omitempty"`
	Config   gameplay.ClientMatchConfig `json:"config,omitempty"`
}

type partyMemberRequest struct {
	UserID string `json:"userId"`
}

type partyTeamRequest struct {
	TeamID string `json:"teamId"`
}

func (a *api) createParty(c echo.Context) error {
	userID, err := a.requirePlayableUser(c)
	if err != nil {
		return err
	}
	var req partyCreateRequest
	if err := decodeJSONBody(c.Request(), &req); err != nil {
		return plainTextError(c, http.StatusBadRequest, "invalid payload")
	}
	mode := req.Mode
	if mode == "" {
		mode = matchkind.ModeDuel
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
		snap, err = a.parties.SetPartyConfig(snap.ID, req.Config.MatchConfig())
		if err != nil {
			observability.Log("error", "create party settings save failed", map[string]any{"userId": userID, "partyId": snap.ID, "error": err.Error()})
			return plainTextError(c, http.StatusInternalServerError, "party unavailable")
		}
	}
	return a.writeParty(c, snap.ID)
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
	return a.writeParty(c, snap.ID)
}

// writeParty tells the party's members, and answers with the party as it now is.
func (a *api) writeParty(c echo.Context, partyID string) error {
	ctx := c.Request().Context()
	a.tellParty(ctx, partyID)
	party, found, err := a.loadParty(ctx, partyID)
	if err != nil {
		return plainTextError(c, http.StatusBadGateway, "party unavailable")
	}
	if !found {
		return c.NoContent(http.StatusNoContent)
	}
	return writeJSON(c, newPartySnapshot(party))
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
	if snap.State != parties.StateOpen {
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
	if _, err := a.parties.SetPartyConfig(snap.ID, req.Config.MatchConfig()); err != nil {
		if errors.Is(err, parties.ErrPartyMapUnavailable) {
			return echo.NewHTTPError(http.StatusUnprocessableEntity, err.Error())
		}
		observability.Log("error", "update party settings save failed", map[string]any{"userId": userID, "partyId": snap.ID, "mapId": req.Config.MapID, "error": err.Error()})
		return echo.NewHTTPError(http.StatusBadGateway, "party settings unavailable")
	}
	a.tellParty(ctx, snap.ID)
	return nil
}

// partyMember authenticates a member of the party in the route.
func (a *api) partyMember(c echo.Context) (partyID, userID string, err error) {
	userID, err = a.requirePlayableUser(c)
	if err != nil {
		return "", "", err
	}
	partyID = strings.TrimSpace(c.Param("id"))
	snap, found, err := a.parties.GetPartyByID(partyID)
	if err != nil {
		return "", "", plainTextError(c, http.StatusBadGateway, "party unavailable")
	}
	if !found || !partyHasMember(snap, userID) {
		return "", "", plainTextError(c, http.StatusForbidden, "party membership required")
	}
	return partyID, userID, nil
}

// partyCommand is one thing a member can do. It runs, the party's members hear about it, and the
// caller is told if it failed; every command is the same apart from what it does.
func (a *api) partyCommand(run func(c echo.Context, partyID, userID string) error) echo.HandlerFunc {
	return func(c echo.Context) error {
		partyID, userID, err := a.partyMember(c)
		if err != nil {
			return err
		}
		if err := run(c, partyID, userID); err != nil {
			var failure *echo.HTTPError
			if errors.As(err, &failure) {
				return plainTextError(c, failure.Code, partyFailureMessage(failure))
			}
			return plainTextError(c, http.StatusConflict, err.Error())
		}
		return a.writeParty(c, partyID)
	}
}

func partyFailureMessage(failure *echo.HTTPError) string {
	if message, ok := failure.Message.(string); ok {
		return message
	}
	return http.StatusText(failure.Code)
}

func (a *api) setPartyTeam(c echo.Context, partyID, userID string) error {
	var req partyTeamRequest
	if err := decodeJSONBody(c.Request(), &req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid payload")
	}
	_, err := a.parties.SetPartyMemberTeam(partyID, userID, strings.TrimSpace(req.TeamID))
	return err
}

func (a *api) shuffleParty(c echo.Context, partyID, userID string) error {
	_, err := a.parties.ShufflePartyTeams(partyID, userID)
	return err
}

func (a *api) updatePartySettings(c echo.Context, partyID, userID string) error {
	var req partySettingsRequest
	if err := decodeJSONBody(c.Request(), &req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid payload")
	}
	return a.setPartySettings(c.Request().Context(), partyID, userID, req)
}

func (a *api) kickFromParty(c echo.Context, partyID, userID string) error {
	var req partyMemberRequest
	if err := decodeJSONBody(c.Request(), &req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid payload")
	}
	target := strings.TrimSpace(req.UserID)
	if _, err := a.parties.KickPartyMember(partyID, userID, target); err != nil {
		return err
	}
	a.tell([]string{target}, partyLeft{PartyID: partyID, Reason: "You were removed from this party"})
	return nil
}

func (a *api) transferParty(c echo.Context, partyID, userID string) error {
	var req partyMemberRequest
	if err := decodeJSONBody(c.Request(), &req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid payload")
	}
	_, err := a.parties.TransferPartyOwner(partyID, userID, strings.TrimSpace(req.UserID))
	return err
}

func (a *api) leaveParty(c echo.Context, partyID, userID string) error {
	if _, err := a.parties.LeaveParty(partyID, userID); err != nil {
		return err
	}
	a.tell([]string{userID}, partyLeft{PartyID: partyID, Reason: "You left this party"})
	return nil
}

func (a *api) startParty(c echo.Context, partyID, userID string) error {
	return a.startPartyMatch(c.Request().Context(), partyID, userID)
}

// loadParty is a party with who is connected filled in.
func (a *api) loadParty(ctx context.Context, partyID string) (parties.Party, bool, error) {
	party, found, err := a.parties.GetPartyByID(partyID)
	if err != nil || !found {
		return parties.Party{}, found, err
	}
	online, err := presence.Online(ctx, a.db.Pool(), partyMemberIDs(party))
	if err != nil {
		return parties.Party{}, false, err
	}
	for i := range party.Members {
		party.Members[i].Connected = online[party.Members[i].UserID]
		party.Members[i].PresenceStatus = parties.PresenceOffline
		if party.Members[i].Connected {
			party.Members[i].PresenceStatus = parties.PresenceOnline
		}
	}
	return party, true, nil
}

// tellParty sends the party to its members as it now is. The revision comes first, so a higher one
// never carries older state, however many API replicas are sending.
func (a *api) tellParty(ctx context.Context, partyID string) {
	if partyID == "" {
		return
	}
	revision, err := a.parties.NextRevision(partyID)
	if err != nil {
		observability.Log("warn", "party revision failed", map[string]any{"partyId": partyID, "error": err.Error()})
		return
	}
	party, found, err := a.loadParty(ctx, partyID)
	if err != nil || !found {
		return
	}
	view := newPartySnapshot(party)
	view.Revision = revision
	a.tell(partyMemberIDs(party), partyUpdated{Party: view})
}

func partyMemberIDs(party parties.Party) []string {
	out := make([]string, len(party.Members))
	for i, member := range party.Members {
		out[i] = member.UserID
	}
	return out
}

// startPartyMatch starts the party's match with every member seated, once they are all connected.
func (a *api) startPartyMatch(ctx context.Context, partyID, userID string) error {
	snap, ok, err := a.loadParty(ctx, partyID)
	if err != nil {
		return echo.NewHTTPError(http.StatusBadGateway, "party unavailable")
	}
	if !ok {
		return echo.NewHTTPError(http.StatusNotFound, "party not found")
	}
	if snap.OwnerUserID != userID {
		return echo.NewHTTPError(http.StatusForbidden, "forbidden")
	}
	if snap.State != parties.StateOpen {
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
		ReturnTarget: &matches.ReturnTarget{Kind: matches.ReturnParty, PartyID: snap.ID},
	}); err != nil {
		observability.Log("warn", "party match start failed", map[string]any{"partyId": snap.ID, "error": err.Error()})
		return echo.NewHTTPError(http.StatusConflict, a.startErrorMessage(err))
	}
	return nil
}

func requirePartyPresence(snap parties.Party) error {
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
		return errors.New("all players must be connected to start: " + strings.Join(missing, ", "))
	}
	return nil
}

func partyHasMember(snap parties.Party, userID string) bool {
	for _, member := range snap.Members {
		if member.UserID == userID {
			return true
		}
	}
	return false
}

func normalizePartyTeam(teamID string) string {
	if strings.ToLower(strings.TrimSpace(teamID)) == "b" {
		return "b"
	}
	return "a"
}

type partySettingsRequest struct {
	Mode   matchkind.Mode             `json:"mode"`
	Config gameplay.ClientMatchConfig `json:"config"`
}
