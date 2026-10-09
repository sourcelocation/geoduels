package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"geoduels/pkg/auth"
	"geoduels/pkg/contracts"
	"geoduels/pkg/coordinator"
	"geoduels/pkg/entityid"
	"geoduels/pkg/maintenance"
	"geoduels/pkg/matchlaunch"
	"geoduels/pkg/sessionpolicy"
)

func (a *api) updateSelectedBadge(c echo.Context) error {
	claims, err := a.authenticatedClaims(c.Request())
	if err != nil {
		return plainTextError(c, http.StatusUnauthorized, "unauthorized")
	}
	var req struct {
		BadgeID string `json:"badgeId"`
	}
	if err := json.NewDecoder(c.Request().Body).Decode(&req); err != nil {
		return plainTextError(c, http.StatusBadRequest, "invalid request")
	}
	profile, err := a.profiles.UpdateSelectedBadge(claims.Sub, req.BadgeID)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unavailable") {
			return plainTextError(c, http.StatusBadRequest, "badge unavailable")
		}
		return plainTextError(c, http.StatusInternalServerError, "profile unavailable")
	}
	return json.NewEncoder(c.Response()).Encode(map[string]any{
		"badges":        profile.Badges,
		"selectedBadge": profile.SelectedBadge,
	})
}

func (a *api) userNotifications(c echo.Context) error {
	r := c.Request()
	claims, err := a.authenticatedClaims(r)
	if err != nil {
		return plainTextError(c, http.StatusUnauthorized, "unauthorized")
	}
	if strings.EqualFold(r.URL.Query().Get("filter"), "all") {
		if a.notificationService != nil {
			limit := queryLimit(r, 30)
			beforeID, _ := strconv.ParseInt(r.URL.Query().Get("beforeId"), 10, 64)
			notifications, err := a.notificationService.Inbox(r.Context(), claims.Sub, limit, beforeID)
			if err != nil {
				return plainTextError(c, http.StatusInternalServerError, "notifications unavailable")
			}
			return writeJSON(c, map[string]any{"notifications": notifications})
		}
	}
	notifications, err := a.notificationService.List(r.Context(), claims.Sub, 20)
	if err != nil {
		return plainTextError(c, http.StatusInternalServerError, "notifications unavailable")
	}
	return writeJSON(c, map[string]any{"notifications": notifications})
}

func (a *api) markAllUserNotificationsRead(c echo.Context) error {
	claims, err := a.authenticatedClaims(c.Request())
	if err != nil {
		return plainTextError(c, http.StatusUnauthorized, "unauthorized")
	}
	if a.notificationService == nil {
		return plainTextError(c, http.StatusNotImplemented, "notifications unavailable")
	}
	if err := a.notificationService.MarkAllRead(c.Request().Context(), claims.Sub); err != nil {
		return plainTextError(c, http.StatusInternalServerError, "failed to mark notifications")
	}
	if a.live != nil {
		a.live.publish(claims.Sub, contracts.LiveEvent{Type: contracts.LiveNotificationReadAll})
	}
	return c.NoContent(http.StatusNoContent)
}

func (a *api) markUserNotificationRead(c echo.Context) error {
	claims, err := a.authenticatedClaims(c.Request())
	if err != nil {
		return plainTextError(c, http.StatusUnauthorized, "unauthorized")
	}
	notificationID, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil {
		return plainTextError(c, http.StatusBadRequest, "invalid notification id")
	}
	if err := a.notificationService.MarkRead(c.Request().Context(), claims.Sub, notificationID); err != nil {
		return plainTextError(c, http.StatusInternalServerError, "failed to mark notification")
	}
	if a.live != nil {
		a.live.publish(claims.Sub, contracts.LiveEvent{Type: contracts.LiveNotificationRead, NotificationID: notificationID})
	}
	return c.NoContent(http.StatusNoContent)
}

func (a *api) leaderboard(c echo.Context) error {
	r := c.Request()
	mode := strings.TrimSpace(c.QueryParam("mode"))
	season := strings.TrimSpace(c.QueryParam("season"))
	limit := 100
	offset := 0
	if mode == "" {
		mode = "duel"
	}
	settings, err := a.seasons.GetRankedSeasonSettings()
	if err != nil {
		return plainTextError(c, http.StatusInternalServerError, "leaderboard unavailable")
	}
	if season == "" {
		season = settings.ActiveSeasonID
	}

	if raw := strings.TrimSpace(c.QueryParam("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			return plainTextError(c, http.StatusBadRequest, "invalid limit")
		}
		limit = parsed
	}
	if raw := strings.TrimSpace(c.QueryParam("offset")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			return plainTextError(c, http.StatusBadRequest, "invalid offset")
		}
		offset = parsed
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}

	entries, err := a.leaderboardService.List(r.Context(), mode, season, limit, offset)
	if err != nil {
		return plainTextError(c, http.StatusInternalServerError, "leaderboard unavailable")
	}

	selfRank := 0
	totalPlayers := 0
	if claims, ok := a.optionalAuthenticatedClaims(r); ok {
		overview, err := a.leaderboardService.Overview(r.Context(), claims.Sub, mode, season, 10)
		if err != nil {
			return plainTextError(c, http.StatusInternalServerError, "leaderboard unavailable")
		}
		selfRank = overview.SelfRank
		totalPlayers = overview.TotalPlayers
	} else {
		overview, err := a.leaderboardService.Overview(r.Context(), "", mode, season, 10)
		if err != nil {
			return plainTextError(c, http.StatusInternalServerError, "leaderboard unavailable")
		}
		totalPlayers = overview.TotalPlayers
	}

	response := map[string]any{
		"season":       season,
		"mode":         mode,
		"limit":        limit,
		"offset":       offset,
		"entries":      entries,
		"selfRank":     selfRank,
		"totalPlayers": totalPlayers,
	}
	if season == settings.ActiveSeasonID && settings.NextResetAt != nil {
		response["nextResetAt"] = settings.NextResetAt
	}

	return writeJSON(c, response)
}

func (a *api) optionalAuthenticatedClaims(r *http.Request) (auth.AppClaims, bool) {
	authz := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(authz, "Bearer ") {
		return auth.AppClaims{}, false
	}
	claims, err := a.authenticatedClaims(r)
	if err != nil {
		return auth.AppClaims{}, false
	}
	return claims, true
}

func (a *api) match(c echo.Context) error {
	if _, err := a.authenticatedClaims(c.Request()); err != nil {
		return plainTextError(c, http.StatusUnauthorized, "unauthorized")
	}
	id := a.resolveEntityID("match", c.Param("id"))
	snapshot, found, err := a.getPublicFinalMatchSnapshot(id)
	if err != nil || !found {
		return plainTextError(c, http.StatusNotFound, "match not found")
	}
	return json.NewEncoder(c.Response()).Encode(snapshot)
}

func (a *api) matchSession(c echo.Context) error {
	claims, _, err := a.authenticatedAccount(c.Request())
	if err != nil {
		return plainTextError(c, http.StatusUnauthorized, "identity not found")
	}
	matchID := a.resolveEntityID("match", c.Param("id"))
	if matchID == "" {
		return plainTextError(c, http.StatusBadRequest, "invalid match")
	}
	resp, err := a.resolveMatchSession(c.Request().Context(), claims.Sub, matchID)
	if err != nil {
		return plainTextError(c, http.StatusBadGateway, "match unavailable")
	}
	return writeJSON(c, resp)
}

func (a *api) matchRoute(c echo.Context) error {
	matchID := a.resolveEntityID("match", c.Param("id"))
	if matchID == "" {
		return plainTextError(c, http.StatusBadRequest, "invalid match")
	}
	claims, authenticated := a.optionalAuthenticatedClaims(c.Request())
	userID := ""
	if authenticated {
		userID = claims.Sub
		if banned, err := a.accountBanned(userID); err == nil && banned {
			return writeJSON(c, contracts.MatchSessionResponse{Status: "forbidden", MatchID: matchID})
		}
	}
	resp, err := a.resolveMatchRoute(c.Request().Context(), userID, authenticated, matchID)
	if err != nil {
		return plainTextError(c, http.StatusBadGateway, "match unavailable")
	}
	return writeJSON(c, resp)
}

func (a *api) matchBootstrap(c echo.Context) error {
	matchID := a.resolveEntityID("match", c.Param("id"))
	if matchID == "" {
		return plainTextError(c, http.StatusBadRequest, "invalid match")
	}
	authPayload, nextRefreshToken, err := a.rotateSessionFromCookie(c.Request())
	if err != nil {
		a.clearRefreshCookie(c, c.Request())
		return plainTextError(c, http.StatusUnauthorized, "unauthorized")
	}
	a.setRefreshCookie(c, c.Request(), nextRefreshToken)
	banned, err := a.accountBanned(authPayload.User.ID)
	if err != nil {
		return plainTextError(c, http.StatusUnauthorized, "identity not found")
	}
	if banned {
		return writeJSON(c, contracts.MatchBootstrapResponse{
			Auth:  authPayload,
			Match: contracts.MatchSessionResponse{Status: "forbidden", MatchID: matchID},
		})
	}
	matchPayload, err := a.resolveMatchSession(c.Request().Context(), authPayload.User.ID, matchID)
	if err != nil {
		return plainTextError(c, http.StatusBadGateway, "match unavailable")
	}
	return writeJSON(c, contracts.MatchBootstrapResponse{
		Auth:  authPayload,
		Match: matchPayload,
	})
}

func (a *api) resolveMatchSession(ctx context.Context, userID, targetMatchID string) (contracts.MatchSessionResponse, error) {
	return a.resolveMatchRoute(ctx, userID, true, targetMatchID)
}

func (a *api) resolveMatchRoute(ctx context.Context, userID string, authenticated bool, targetMatchID string) (contracts.MatchSessionResponse, error) {
	history, found, err := a.getPublicFinalMatchSnapshot(targetMatchID)
	if err != nil {
		return contracts.MatchSessionResponse{}, err
	}
	if found && !authenticated {
		resp := contracts.MatchSessionResponse{Status: "history", MatchID: targetMatchID, Snapshot: history}
		a.attachReturnTarget(ctx, &resp, userID, authenticated, targetMatchID)
		return resp, nil
	}

	if !authenticated {
		if a.matchLive(ctx, targetMatchID) {
			return contracts.MatchSessionResponse{Status: "live_auth_required", MatchID: targetMatchID}, nil
		}
		return contracts.MatchSessionResponse{Status: "missing", MatchID: targetMatchID}, nil
	}

	if assigned, ok, err := a.coord.GetAssignmentByUser(ctx, userID); err == nil && ok {
		switch a.launcher().ValidateAssignment(ctx, assigned) {
		case matchlaunch.AssignmentValid:
			if assigned.MatchID == targetMatchID {
				payload, healthy, err := a.launcher().AssignedPayload(userID, assigned)
				if err != nil {
					return contracts.MatchSessionResponse{}, err
				}
				if healthy {
					resp := contracts.MatchSessionResponse{
						Status:                "live_connectable",
						MatchID:               payload.MatchID,
						Mode:                  payload.Mode,
						Config:                payload.Config,
						Node:                  payload.Node,
						Ticket:                payload.Ticket,
						WSPath:                payload.WSPath,
						SourcePartyID:         payload.SourcePartyID,
						SourcePartyInviteCode: payload.SourcePartyInviteCode,
						ReturnTarget:          payload.ReturnTarget,
					}
					a.attachReturnTarget(ctx, &resp, userID, authenticated, targetMatchID)
					return resp, nil
				}
				return contracts.MatchSessionResponse{Status: "missing", MatchID: targetMatchID}, nil
			}
			if found {
				resp := contracts.MatchSessionResponse{
					Status:             "history",
					MatchID:            targetMatchID,
					Snapshot:           history,
					ReplacementMatchID: assigned.MatchID,
				}
				a.attachReturnTarget(ctx, &resp, userID, authenticated, targetMatchID)
				if replacement, ok, err := a.launcher().AssignedPayload(userID, assigned); err == nil && ok {
					resp.Replacement = &replacement
				}
				return resp, nil
			}
			resp := contracts.MatchSessionResponse{
				Status:             "replaced",
				MatchID:            targetMatchID,
				ReplacementMatchID: assigned.MatchID,
			}
			if replacement, ok, err := a.launcher().AssignedPayload(userID, assigned); err == nil && ok {
				resp.Replacement = &replacement
			}
			return resp, nil
		case matchlaunch.AssignmentPending:
			if assigned.MatchID == targetMatchID && sessionpolicy.NormalizeMode(assigned.Mode, assigned.MatchID) == contracts.ModeSingleplayer {
				_ = a.coord.ClearAssignment(context.Background(), assigned)
			}
		case matchlaunch.AssignmentAbandoned, matchlaunch.AssignmentInvalid:
		}
	}

	if found {
		resp := contracts.MatchSessionResponse{Status: "history", MatchID: targetMatchID, Snapshot: history}
		a.attachReturnTarget(ctx, &resp, userID, authenticated, targetMatchID)
		return resp, nil
	}
	if a.matchLive(ctx, targetMatchID) {
		return contracts.MatchSessionResponse{Status: "live_auth_required", MatchID: targetMatchID}, nil
	}
	return contracts.MatchSessionResponse{Status: "missing", MatchID: targetMatchID}, nil
}

// matchLive reports whether a match is being played now. One that ended without history or was
// interrupted reads as missing: there is nothing to rejoin or show.
func (a *api) matchLive(ctx context.Context, matchID string) bool {
	status, err := a.matchStore.MatchSessionStatus(ctx, matchID)
	return err == nil && status == contracts.MatchSessionLive
}

func (a *api) attachReturnTarget(ctx context.Context, resp *contracts.MatchSessionResponse, userID string, authenticated bool, matchID string) {
	if resp == nil {
		return
	}
	var target *contracts.MatchReturnTarget
	if a.db != nil {
		if persisted, found, err := a.matchStore.MatchSessionReturnTarget(ctx, matchID); err == nil && found {
			target = persisted
		}
	}
	// Rows written before return targets existed retain party provenance. Keep
	// those rows usable, but resolve the current party server-side.
	if target == nil {
		partyID, _, ok, err := a.matchStore.MatchSessionSourceParty(ctx, matchID)
		if err == nil && ok {
			target = &contracts.MatchReturnTarget{Kind: contracts.MatchReturnParty, PartyID: partyID}
		}
	}
	if target == nil {
		return
	}
	target = contracts.NormalizeMatchReturnTarget(target)
	if target.Kind == contracts.MatchReturnParty {
		if !authenticated || target.PartyID == "" {
			resp.ReturnTarget = &contracts.MatchReturnTarget{Kind: contracts.MatchReturnHome}
			return
		}
		party, found, err := a.parties.GetPartyByID(target.PartyID)
		if err != nil || !found || party.State == contracts.PartyClosed || party.State == contracts.PartyExpired {
			resp.ReturnTarget = &contracts.MatchReturnTarget{Kind: contracts.MatchReturnHome}
			return
		}
		member := false
		for _, candidate := range party.Members {
			if candidate.UserID == userID {
				member = true
				break
			}
		}
		if !member {
			resp.ReturnTarget = &contracts.MatchReturnTarget{Kind: contracts.MatchReturnHome}
			return
		}
		target.PartyInviteCode = party.InviteCode
	}
	resp.ReturnTarget = target
	resp.SourcePartyID = target.PartyID
	if target.Kind == contracts.MatchReturnParty {
		resp.SourcePartyInviteCode = target.PartyInviteCode
	}
}

func (a *api) getPublicFinalMatchSnapshot(matchID string) (*contracts.MatchSnapshot, bool, error) {
	raw, ok, err := a.matchStore.GetFinalMatchSnapshot(matchID)
	if err != nil || !ok {
		return nil, ok, err
	}
	var snapshot contracts.MatchSnapshot
	if err := json.Unmarshal(raw, &snapshot); err != nil {
		return nil, false, err
	}
	snapshot = sanitizeFinalMatchSnapshot(snapshot)
	if snapshot.State == "" {
		snapshot.State = contracts.MatchEnded
	}
	return &snapshot, true, nil
}

func sanitizeFinalMatchSnapshot(snapshot contracts.MatchSnapshot) contracts.MatchSnapshot {
	if snapshot.State == contracts.MatchEnded {
		snapshot.CurrentRound = nil
		snapshot.RoundMSLeft = 0
		snapshot.PhaseEndsAt = 0
		snapshot.GraceWindowSec = 0
		for id, player := range snapshot.Players {
			player.Finalized = false
			player.LastGuessLat = 0
			player.LastGuessLng = 0
			player.HasGuess = false
			player.Disconnected = false
			player.DisconnectDue = 0
			snapshot.Players[id] = player
		}
	}
	return snapshot
}

func (a *api) createMatchReport(c echo.Context) error {
	claims, err := a.authenticatedClaims(c.Request())
	if err != nil {
		return plainTextError(c, http.StatusUnauthorized, "unauthorized")
	}
	matchID := a.resolveEntityID("match", c.Param("id"))
	var req struct {
		ReportedUserID string `json:"reportedUserId"`
		Category       string `json:"category"`
		Reason         string `json:"reason"`
	}
	if err := decodeJSONBody(c.Request(), &req); err != nil {
		return plainTextError(c, http.StatusBadRequest, "invalid payload")
	}
	reportedUserID := strings.TrimSpace(req.ReportedUserID)
	created, err := a.moderation.CreateReport(c.Request().Context(), matchID, claims.Sub, reportedUserID, req.Category, req.Reason)
	if err != nil {
		return plainTextError(c, http.StatusBadRequest, err.Error())
	}
	return writeJSONStatus(c, http.StatusCreated, created)
}

func (a *api) startSession(c echo.Context) error {
	status, err := a.maintenanceStatus(c.Request().Context())
	if err != nil {
		return plainTextError(c, http.StatusBadGateway, "singleplayer unavailable")
	}
	if status.PlayBlocked() {
		return plainTextError(c, http.StatusServiceUnavailable, maintenancePlayMessage(status))
	}
	var req contracts.SessionStartRequest
	if err := decodeJSONBody(c.Request(), &req); err != nil {
		return plainTextError(c, http.StatusBadRequest, "invalid payload")
	}
	mode := sessionpolicy.NormalizeMode(req.Mode, "")
	switch mode {
	case contracts.ModeSingleplayer:
		return a.startSingleplayerSession(c)
	default:
		return plainTextError(c, http.StatusBadRequest, "unsupported mode")
	}
}

func (a *api) startSingleplayerSession(c echo.Context) error {
	r := c.Request()
	status, err := a.maintenanceStatus(r.Context())
	if err != nil {
		return plainTextError(c, http.StatusBadGateway, "singleplayer unavailable")
	}
	if status.PlayBlocked() {
		return plainTextError(c, http.StatusServiceUnavailable, maintenancePlayMessage(status))
	}
	claims, err := a.authenticatedClaims(r)
	if err != nil {
		return plainTextError(c, http.StatusUnauthorized, "unauthorized")
	}
	identity, err := a.authenticatedIdentity(r)
	if err != nil {
		return plainTextError(c, http.StatusUnauthorized, "identity not found")
	}
	if identity.NicknameRequired {
		return plainTextError(c, http.StatusForbidden, "nickname required")
	}
	if identity.AuthMigrationRequired {
		return plainTextError(c, http.StatusForbidden, "connect discord to continue")
	}
	var requestedConfig contracts.MatchConfig
	requestedReturnTarget := &contracts.MatchReturnTarget{Kind: contracts.MatchReturnHome}
	if r.Body != nil {
		raw, readErr := io.ReadAll(io.LimitReader(r.Body, 16<<10))
		if readErr != nil {
			return plainTextError(c, http.StatusBadRequest, "invalid singleplayer config")
		}
		var keys map[string]json.RawMessage
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &keys); err != nil {
				return plainTextError(c, http.StatusBadRequest, "invalid singleplayer config")
			}
		}
		if _, wrapped := keys["config"]; wrapped {
			var request struct {
				Config       contracts.MatchConfig        `json:"config"`
				ReturnTarget *contracts.MatchReturnTarget `json:"returnTarget,omitempty"`
			}
			if err := json.Unmarshal(raw, &request); err != nil {
				return plainTextError(c, http.StatusBadRequest, "invalid singleplayer config")
			}
			requestedConfig = request.Config
			requestedReturnTarget = contracts.NormalizeMatchReturnTarget(request.ReturnTarget)
		} else if len(raw) > 0 {
			if err := json.Unmarshal(raw, &requestedConfig); err != nil {
				return plainTextError(c, http.StatusBadRequest, "invalid singleplayer config")
			}
		}
	}
	userID := claims.Sub
	if assigned, ok, err := a.coord.GetAssignmentByUser(r.Context(), userID); err == nil && ok {
		mode := sessionpolicy.NormalizeMode(assigned.Mode, assigned.MatchID)
		switch a.launcher().ValidateAssignment(r.Context(), assigned) {
		case matchlaunch.AssignmentValid:
			if mode == contracts.ModeDuel {
				return a.writeSessionConflict(c, "ACTIVE_DUEL_MATCH", "Finish or forfeit your active duel before starting singleplayer.")
			}
			if err := a.replaceActiveSingleplayer(r.Context(), userID, assigned); err != nil {
				return plainTextError(c, http.StatusBadGateway, "singleplayer unavailable")
			}
		case matchlaunch.AssignmentPending:
			if mode == contracts.ModeDuel {
				return a.writeSessionConflict(c, "ACTIVE_DUEL_MATCH", "Finish or forfeit your active duel before starting singleplayer.")
			}
			_ = a.coord.ClearAssignment(context.Background(), assigned)
		case matchlaunch.AssignmentAbandoned, matchlaunch.AssignmentInvalid:
			_ = a.coord.ClearAssignment(context.Background(), assigned)
		}
	}
	profile, err := a.profiles.GetProfile(userID)
	if err != nil {
		return plainTextError(c, http.StatusInternalServerError, "profile unavailable")
	}
	if profile.DisplayName == "" {
		profile.DisplayName = userID
	}
	requestedMapID := strings.TrimSpace(requestedConfig.MapID)
	if requestedMapID == "" {
		requestedMapID = strings.TrimSpace(requestedConfig.MapKey)
	}
	if requestedMapID == "" {
		resolvedMapID, err := a.gameplayMaps.ResolveGameplayMapID(contracts.ModeSingleplayer, requestedConfig.Ruleset, "")
		if err != nil {
			return plainTextError(c, http.StatusInternalServerError, "singleplayer unavailable")
		}
		requestedConfig.MapID = resolvedMapID
	}
	requestedReturnTarget = contracts.NormalizeMatchReturnTarget(requestedReturnTarget)
	if requestedReturnTarget.Kind == contracts.MatchReturnMap && requestedReturnTarget.MapID == "" {
		requestedReturnTarget.MapID = requestedConfig.MapID
	}
	found := contracts.MatchFound{
		MatchID: soloSessionID(),
		Mode:    contracts.ModeSingleplayer,
		Config: contracts.NormalizeMatchConfig(contracts.MatchConfig{
			Ruleset:             requestedConfig.Ruleset,
			StreetNames:         requestedConfig.StreetNames,
			MapID:               requestedConfig.MapID,
			MapName:             requestedConfig.MapName,
			MapKey:              requestedConfig.MapKey,
			RoundTimerMode:      requestedConfig.RoundTimerMode,
			RoundTimeLimitMS:    requestedConfig.RoundTimeLimitMS,
			PressureTimeLimitMS: requestedConfig.PressureTimeLimitMS,
			MultiplierMode:      requestedConfig.MultiplierMode,
		}),
		ReturnTarget: requestedReturnTarget,
		Players:      []string{userID},
		Profiles: map[string]contracts.PlayerProfile{
			userID: {
				UserID:            userID,
				DisplayName:       profile.DisplayName,
				MMR:               profile.MMR,
				RatingRD:          profile.RatingRD,
				RankedGamesPlayed: profile.RankedGamesPlayed,
				AvatarURL:         profile.AvatarURL,
				IsGuest:           profile.IsGuest,
				IsAdmin:           profile.IsAdmin,
				SelectedBadge:     profile.SelectedBadge,
			},
		},
		MapAccessUserID: userID,
		MapScope:        "world",
	}
	assigned, err := a.launcher().EnsureAssignment(r.Context(), found)
	if err != nil {
		return plainTextError(c, http.StatusBadGateway, "singleplayer unavailable")
	}
	payload, healthy, err := a.launcher().AssignedPayload(userID, assigned)
	if err != nil || !healthy {
		return plainTextError(c, http.StatusBadGateway, "singleplayer unavailable")
	}
	return writeJSON(c, payload)
}

func soloSessionID() string {
	return entityid.New()
}

func (a *api) replaceActiveSingleplayer(ctx context.Context, userID string, assigned coordinator.Assignment) error {
	node, ok, err := a.coord.GetNodeByRoute(ctx, assigned.PublicRoute)
	if err != nil {
		return err
	}
	if ok {
		body, _ := json.Marshal(map[string]string{"userId": userID})
		reqCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		req, err := http.NewRequestWithContext(
			reqCtx,
			http.MethodPost,
			strings.TrimRight(node.InternalURL, "/")+"/internal/matches/"+url.PathEscape(assigned.MatchID)+"/terminate",
			bytes.NewReader(body),
		)
		if err != nil {
			return err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Coordinator-Secret", a.internalSecret)
		resp, err := a.httpClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
			return errors.New("gameplay node rejected match replacement")
		}
	}
	return a.coord.ClearAssignment(context.Background(), assigned)
}

func (a *api) writeSessionConflict(c echo.Context, code, message string) error {
	return writeJSONStatus(c, http.StatusConflict, map[string]string{
		"code":    code,
		"message": message,
	})
}

func (a *api) maintenanceStatus(ctx context.Context) (maintenance.Status, error) {
	readCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer cancel()
	return maintenance.Read(readCtx, a.redis)
}

func maintenancePlayMessage(status maintenance.Status) string {
	if status.Message != "" {
		return status.Message
	}
	switch status.Phase {
	case maintenance.PhaseActive:
		return "Maintenance in progress. New sessions are temporarily unavailable."
	case maintenance.PhaseWarning:
		return "New sessions have been paused for scheduled maintenance."
	default:
		return "Singleplayer unavailable"
	}
}
