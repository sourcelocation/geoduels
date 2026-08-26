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

	"github.com/gorilla/mux"

	"geoduels/pkg/auth"
	"geoduels/pkg/contracts"
	"geoduels/pkg/coordinator"
	"geoduels/pkg/entityid"
	"geoduels/pkg/maintenance"
	"geoduels/pkg/matchlaunch"
	"geoduels/pkg/persistence"
	"geoduels/pkg/sessionpolicy"
)

func (a *api) me(w http.ResponseWriter, r *http.Request) {
	claims, identity, err := a.authenticatedAccount(r)
	if err != nil {
		http.Error(w, "identity unavailable", http.StatusInternalServerError)
		return
	}
	profile, err := a.store.GetProfile(claims.Sub)
	if err != nil {
		http.Error(w, "profile unavailable", http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"id":                profile.UserID,
		"email":             identity.Email,
		"display_name":      profile.DisplayName,
		"avatar_url":        profile.AvatarURL,
		"mmr":               profile.MMR,
		"ratingRd":          profile.RatingRD,
		"gamesPlayed":       profile.GamesPlayed,
		"wins":              profile.Wins,
		"rankedGamesPlayed": profile.RankedGamesPlayed,
		"rankedWins":        profile.RankedWins,
		"isGuest":           profile.IsGuest,
		"isAdmin":           profile.IsAdmin,
		"isModerator":       profile.IsModerator,
		"isBanned":          profile.IsBanned,
		"banReason":         profile.BanReason,
		"linkedProviders":   identity.LinkedProviders,
		"badges":            profile.Badges,
		"selectedBadge":     profile.SelectedBadge,
	})
}

func (a *api) updateSelectedBadge(w http.ResponseWriter, r *http.Request) {
	claims, err := a.authenticatedClaims(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req struct {
		BadgeID string `json:"badgeId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}
	profile, err := a.store.UpdateSelectedBadge(claims.Sub, req.BadgeID)
	if err != nil {
		if strings.Contains(strings.ToLower(err.Error()), "unavailable") {
			http.Error(w, "badge unavailable", http.StatusBadRequest)
			return
		}
		http.Error(w, "profile unavailable", http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]any{
		"badges":        profile.Badges,
		"selectedBadge": profile.SelectedBadge,
	})
}

func (a *api) userNotifications(w http.ResponseWriter, r *http.Request) {
	claims, err := a.authenticatedClaims(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if strings.EqualFold(r.URL.Query().Get("filter"), "all") {
		if store, ok := a.store.(interface {
			ListNotificationInbox(string, int, int64) ([]persistence.UserNotification, error)
		}); ok {
			limit := queryLimit(r, 30)
			beforeID, _ := strconv.ParseInt(r.URL.Query().Get("beforeId"), 10, 64)
			notifications, err := store.ListNotificationInbox(claims.Sub, limit, beforeID)
			if err != nil {
				http.Error(w, "notifications unavailable", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"notifications": notifications})
			return
		}
	}
	notifications, err := a.store.ListUserNotifications(claims.Sub, 10)
	if err != nil {
		http.Error(w, "notifications unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"notifications": notifications})
}

func (a *api) markAllUserNotificationsRead(w http.ResponseWriter, r *http.Request) {
	claims, err := a.authenticatedClaims(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	store, ok := a.store.(interface{ MarkAllUserNotificationsRead(string) error })
	if !ok {
		http.Error(w, "notifications unavailable", http.StatusNotImplemented)
		return
	}
	if err := store.MarkAllUserNotificationsRead(claims.Sub); err != nil {
		http.Error(w, "failed to mark notifications", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) markUserNotificationRead(w http.ResponseWriter, r *http.Request) {
	claims, err := a.authenticatedClaims(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	notificationID, err := strconv.ParseInt(strings.TrimSpace(mux.Vars(r)["id"]), 10, 64)
	if err != nil {
		http.Error(w, "invalid notification id", http.StatusBadRequest)
		return
	}
	if err := a.store.MarkUserNotificationRead(claims.Sub, notificationID); err != nil {
		http.Error(w, "failed to mark notification", http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) leaderboard(w http.ResponseWriter, r *http.Request) {
	mode := strings.TrimSpace(r.URL.Query().Get("mode"))
	season := strings.TrimSpace(r.URL.Query().Get("season"))
	limit := 100
	offset := 0
	if mode == "" {
		mode = "duel"
	}
	settings, err := a.store.GetRankedSeasonSettings()
	if err != nil {
		http.Error(w, "leaderboard unavailable", http.StatusInternalServerError)
		return
	}
	if season == "" {
		season = settings.ActiveSeasonID
	}

	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			http.Error(w, "invalid limit", http.StatusBadRequest)
			return
		}
		limit = parsed
	}
	if raw := strings.TrimSpace(r.URL.Query().Get("offset")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			http.Error(w, "invalid offset", http.StatusBadRequest)
			return
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

	entries, err := a.store.ListLeaderboard(mode, season, limit, offset)
	if err != nil {
		http.Error(w, "leaderboard unavailable", http.StatusInternalServerError)
		return
	}

	selfRank := 0
	totalPlayers := 0
	if claims, ok := a.optionalAuthenticatedClaims(r); ok {
		overview, err := a.store.GetLeaderboardOverview(claims.Sub, mode, season, 10)
		if err != nil {
			http.Error(w, "leaderboard unavailable", http.StatusInternalServerError)
			return
		}
		selfRank = overview.SelfRank
		totalPlayers = overview.TotalPlayers
	} else {
		overview, err := a.store.GetLeaderboardOverview("", mode, season, 10)
		if err != nil {
			http.Error(w, "leaderboard unavailable", http.StatusInternalServerError)
			return
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
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

func (a *api) match(w http.ResponseWriter, r *http.Request) {
	if _, err := a.authenticatedClaims(r); err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	id := a.resolveEntityID("match", mux.Vars(r)["id"])
	snapshot, found, err := a.getPublicFinalMatchSnapshot(id)
	if err != nil || !found {
		http.Error(w, "match not found", http.StatusNotFound)
		return
	}
	_ = json.NewEncoder(w).Encode(snapshot)
}

func (a *api) matchSession(w http.ResponseWriter, r *http.Request) {
	claims, _, err := a.authenticatedAccount(r)
	if err != nil {
		http.Error(w, "identity not found", http.StatusUnauthorized)
		return
	}
	matchID := a.resolveEntityID("match", mux.Vars(r)["id"])
	if matchID == "" {
		http.Error(w, "invalid match", http.StatusBadRequest)
		return
	}
	resp, err := a.resolveMatchSession(r.Context(), claims.Sub, matchID)
	if err != nil {
		http.Error(w, "match unavailable", http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (a *api) matchRoute(w http.ResponseWriter, r *http.Request) {
	matchID := a.resolveEntityID("match", mux.Vars(r)["id"])
	if matchID == "" {
		http.Error(w, "invalid match", http.StatusBadRequest)
		return
	}
	claims, authenticated := a.optionalAuthenticatedClaims(r)
	userID := ""
	if authenticated {
		userID = claims.Sub
		if banned, err := a.accountBanned(userID); err == nil && banned {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(contracts.MatchSessionResponse{Status: "forbidden", MatchID: matchID})
			return
		}
	}
	resp, err := a.resolveMatchRoute(r.Context(), userID, authenticated, matchID)
	if err != nil {
		http.Error(w, "match unavailable", http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
}

func (a *api) matchBootstrap(w http.ResponseWriter, r *http.Request) {
	matchID := a.resolveEntityID("match", mux.Vars(r)["id"])
	if matchID == "" {
		http.Error(w, "invalid match", http.StatusBadRequest)
		return
	}
	authPayload, nextRefreshToken, err := a.rotateSessionFromCookie(r)
	if err != nil {
		a.clearRefreshCookie(w, r)
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	a.setRefreshCookie(w, r, nextRefreshToken)
	banned, err := a.accountBanned(authPayload.User.ID)
	if err != nil {
		http.Error(w, "identity not found", http.StatusUnauthorized)
		return
	}
	if banned {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(contracts.MatchBootstrapResponse{
			Auth:  authPayload,
			Match: contracts.MatchSessionResponse{Status: "forbidden", MatchID: matchID},
		})
		return
	}
	matchPayload, err := a.resolveMatchSession(r.Context(), authPayload.User.ID, matchID)
	if err != nil {
		http.Error(w, "match unavailable", http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(contracts.MatchBootstrapResponse{
		Auth:  authPayload,
		Match: matchPayload,
	})
}

func (a *api) sessionResumable(w http.ResponseWriter, r *http.Request) {
	claims, err := a.authenticatedClaims(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	resp := contracts.ResumableSessionResponse{Status: "none"}
	if assigned, ok, err := a.coord.GetAssignmentByUser(r.Context(), claims.Sub); err == nil && ok {
		mode := sessionpolicy.NormalizeMode(assigned.Mode, assigned.MatchID)
		if mode == contracts.ModeDuel && a.launcher().ValidateAssignment(r.Context(), assigned) == matchlaunch.AssignmentValid {
			resp = contracts.ResumableSessionResponse{
				Status:  "match",
				MatchID: assigned.MatchID,
				Mode:    string(mode),
			}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(resp)
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
		if rec, ok, err := a.store.GetRuntimeMatch(ctx, targetMatchID); err == nil && ok && rec.State != string(contracts.MatchEnded) {
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
				_ = a.store.RecordRuntimeMatch(ctx, assigned.MatchID, string(contracts.MatchEnded), assigned.NodeEpoch, true)
			}
		case matchlaunch.AssignmentAbandoned, matchlaunch.AssignmentInvalid:
		}
	}

	if found {
		resp := contracts.MatchSessionResponse{Status: "history", MatchID: targetMatchID, Snapshot: history}
		a.attachReturnTarget(ctx, &resp, userID, authenticated, targetMatchID)
		return resp, nil
	}
	if rec, ok, err := a.store.GetRuntimeMatch(ctx, targetMatchID); err == nil && ok && rec.State != string(contracts.MatchEnded) {
		return contracts.MatchSessionResponse{Status: "live_auth_required", MatchID: targetMatchID}, nil
	}
	return contracts.MatchSessionResponse{Status: "missing", MatchID: targetMatchID}, nil
}

func (a *api) attachReturnTarget(ctx context.Context, resp *contracts.MatchSessionResponse, userID string, authenticated bool, matchID string) {
	if resp == nil {
		return
	}
	var target *contracts.MatchReturnTarget
	if repository, ok := a.store.(interface {
		MatchSessionReturnTarget(context.Context, string) (*contracts.MatchReturnTarget, bool, error)
	}); ok {
		if persisted, found, err := repository.MatchSessionReturnTarget(ctx, matchID); err == nil && found {
			target = persisted
		}
	}
	// Rows written before return targets existed retain party provenance. Keep
	// those rows usable, but resolve the current party server-side.
	if target == nil {
		partyID, _, ok, err := a.store.MatchSessionSourceParty(ctx, matchID)
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
		party, found, err := a.store.GetPartyByID(target.PartyID)
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
	raw, ok, err := a.store.GetFinalMatchSnapshot(matchID)
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

func (a *api) createMatchReport(w http.ResponseWriter, r *http.Request) {
	claims, err := a.authenticatedClaims(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	matchID := a.resolveEntityID("match", mux.Vars(r)["id"])
	var req struct {
		ReportedUserID string `json:"reportedUserId"`
		Category       string `json:"category"`
		Reason         string `json:"reason"`
	}
	if err := decodeJSONBody(r, &req); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	reportedUserID := strings.TrimSpace(req.ReportedUserID)
	created, err := a.store.CreatePlayerReportSignal(persistence.CreatePlayerReportSignalParams{
		MatchID:        matchID,
		ReporterUserID: claims.Sub,
		ReportedUserID: reportedUserID,
		Category:       req.Category,
		Reason:         req.Reason,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(created)
}

func (a *api) startSession(w http.ResponseWriter, r *http.Request) {
	status, err := a.maintenanceStatus(r.Context())
	if err != nil {
		http.Error(w, "singleplayer unavailable", http.StatusBadGateway)
		return
	}
	if status.PlayBlocked() {
		http.Error(w, maintenancePlayMessage(status), http.StatusServiceUnavailable)
		return
	}
	var req contracts.SessionStartRequest
	if err := decodeJSONBody(r, &req); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	mode := sessionpolicy.NormalizeMode(req.Mode, "")
	switch mode {
	case contracts.ModeSingleplayer:
		a.startSingleplayerSession(w, r)
	default:
		http.Error(w, "unsupported mode", http.StatusBadRequest)
	}
}

func (a *api) startSingleplayerSession(w http.ResponseWriter, r *http.Request) {
	status, err := a.maintenanceStatus(r.Context())
	if err != nil {
		http.Error(w, "singleplayer unavailable", http.StatusBadGateway)
		return
	}
	if status.PlayBlocked() {
		http.Error(w, maintenancePlayMessage(status), http.StatusServiceUnavailable)
		return
	}
	claims, err := a.authenticatedClaims(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	identity, err := a.authenticatedIdentity(r)
	if err != nil {
		http.Error(w, "identity not found", http.StatusUnauthorized)
		return
	}
	if identity.NicknameRequired {
		http.Error(w, "nickname required", http.StatusForbidden)
		return
	}
	if identity.AuthMigrationRequired {
		http.Error(w, "connect discord to continue", http.StatusForbidden)
		return
	}
	var requestedConfig contracts.MatchConfig
	requestedReturnTarget := &contracts.MatchReturnTarget{Kind: contracts.MatchReturnHome}
	if r.Body != nil {
		raw, readErr := io.ReadAll(io.LimitReader(r.Body, 16<<10))
		if readErr != nil {
			http.Error(w, "invalid singleplayer config", http.StatusBadRequest)
			return
		}
		var keys map[string]json.RawMessage
		if len(raw) > 0 {
			if err := json.Unmarshal(raw, &keys); err != nil {
				http.Error(w, "invalid singleplayer config", http.StatusBadRequest)
				return
			}
		}
		if _, wrapped := keys["config"]; wrapped {
			var request struct {
				Config       contracts.MatchConfig        `json:"config"`
				ReturnTarget *contracts.MatchReturnTarget `json:"returnTarget,omitempty"`
			}
			if err := json.Unmarshal(raw, &request); err != nil {
				http.Error(w, "invalid singleplayer config", http.StatusBadRequest)
				return
			}
			requestedConfig = request.Config
			requestedReturnTarget = contracts.NormalizeMatchReturnTarget(request.ReturnTarget)
		} else if len(raw) > 0 {
			if err := json.Unmarshal(raw, &requestedConfig); err != nil {
				http.Error(w, "invalid singleplayer config", http.StatusBadRequest)
				return
			}
		}
	}
	userID := claims.Sub
	if assigned, ok, err := a.coord.GetAssignmentByUser(r.Context(), userID); err == nil && ok {
		mode := sessionpolicy.NormalizeMode(assigned.Mode, assigned.MatchID)
		switch a.launcher().ValidateAssignment(r.Context(), assigned) {
		case matchlaunch.AssignmentValid:
			if mode == contracts.ModeDuel {
				a.writeSessionConflict(w, "ACTIVE_DUEL_MATCH", "Finish or forfeit your active duel before starting singleplayer.")
				return
			}
			if err := a.replaceActiveSingleplayer(r.Context(), userID, assigned); err != nil {
				http.Error(w, "singleplayer unavailable", http.StatusBadGateway)
				return
			}
		case matchlaunch.AssignmentPending:
			if mode == contracts.ModeDuel {
				a.writeSessionConflict(w, "ACTIVE_DUEL_MATCH", "Finish or forfeit your active duel before starting singleplayer.")
				return
			}
			_ = a.coord.ClearAssignment(context.Background(), assigned)
			_ = a.store.RecordRuntimeMatch(r.Context(), assigned.MatchID, string(contracts.MatchEnded), assigned.NodeEpoch, true)
		case matchlaunch.AssignmentAbandoned, matchlaunch.AssignmentInvalid:
			_ = a.coord.ClearAssignment(context.Background(), assigned)
		}
	}
	profile, err := a.store.GetProfile(userID)
	if err != nil {
		http.Error(w, "profile unavailable", http.StatusInternalServerError)
		return
	}
	if profile.DisplayName == "" {
		profile.DisplayName = userID
	}
	requestedMapID := strings.TrimSpace(requestedConfig.MapID)
	if requestedMapID == "" {
		requestedMapID = strings.TrimSpace(requestedConfig.MapKey)
	}
	if requestedMapID == "" {
		resolvedMapID, err := a.store.ResolveGameplayMapID(contracts.ModeSingleplayer, requestedConfig.Ruleset, "")
		if err != nil {
			http.Error(w, "singleplayer unavailable", http.StatusInternalServerError)
			return
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
		http.Error(w, "singleplayer unavailable", http.StatusBadGateway)
		return
	}
	payload, healthy, err := a.launcher().AssignedPayload(userID, assigned)
	if err != nil || !healthy {
		http.Error(w, "singleplayer unavailable", http.StatusBadGateway)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(payload)
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

func (a *api) writeSessionConflict(w http.ResponseWriter, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusConflict)
	_ = json.NewEncoder(w).Encode(map[string]string{
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
