package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"

	"geoduels/pkg/auth"
	"geoduels/pkg/contracts"
	"geoduels/pkg/coordinator"
	"geoduels/pkg/maintenance"
	"geoduels/pkg/matchlaunch"
	"geoduels/pkg/persistence"
	"geoduels/pkg/sessionpolicy"
)

func (a *api) me(w http.ResponseWriter, r *http.Request) {
	claims, err := a.authenticatedClaims(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	identity, err := a.store.GetIdentity(claims.Sub)
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
	notifications, err := a.store.ListUserNotifications(claims.Sub, 10)
	if err != nil {
		http.Error(w, "notifications unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{"notifications": notifications})
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
	if season == "" {
		settings, err := a.store.GetRankedSeasonSettings()
		if err != nil {
			http.Error(w, "leaderboard unavailable", http.StatusInternalServerError)
			return
		}
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

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{
		"season":       season,
		"mode":         mode,
		"limit":        limit,
		"offset":       offset,
		"entries":      entries,
		"selfRank":     selfRank,
		"totalPlayers": totalPlayers,
	})
}

func (a *api) optionalAuthenticatedClaims(r *http.Request) (auth.AppClaims, bool) {
	authz := strings.TrimSpace(r.Header.Get("Authorization"))
	if !strings.HasPrefix(authz, "Bearer ") {
		return auth.AppClaims{}, false
	}
	claims, err := auth.ValidateAppAccessToken(a.appAuthSecret, strings.TrimSpace(strings.TrimPrefix(authz, "Bearer ")))
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
	id := mux.Vars(r)["id"]
	snapshot, found, err := a.getPublicFinalMatchSnapshot(id)
	if err != nil || !found {
		http.Error(w, "match not found", http.StatusNotFound)
		return
	}
	_ = json.NewEncoder(w).Encode(snapshot)
}

func (a *api) matchSession(w http.ResponseWriter, r *http.Request) {
	claims, err := a.authenticatedClaims(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if _, err := a.store.GetIdentity(claims.Sub); err != nil {
		http.Error(w, "identity not found", http.StatusUnauthorized)
		return
	}
	matchID := strings.TrimSpace(mux.Vars(r)["id"])
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
	matchID := strings.TrimSpace(mux.Vars(r)["id"])
	if matchID == "" {
		http.Error(w, "invalid match", http.StatusBadRequest)
		return
	}
	claims, authenticated := a.optionalAuthenticatedClaims(r)
	userID := ""
	if authenticated {
		userID = claims.Sub
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
	matchID := strings.TrimSpace(mux.Vars(r)["id"])
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
		a.attachLobbyReturn(&resp, targetMatchID)
		return resp, nil
	}

	if !authenticated {
		if rec, ok, err := a.store.GetRuntimeMatch(targetMatchID); err == nil && ok && rec.State != string(contracts.MatchEnded) {
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
					return contracts.MatchSessionResponse{
						Status:                "live_connectable",
						MatchID:               payload.MatchID,
						Mode:                  payload.Mode,
						Config:                payload.Config,
						Node:                  payload.Node,
						Ticket:                payload.Ticket,
						WSPath:                payload.WSPath,
						SourceLobbyID:         payload.SourceLobbyID,
						SourceLobbyInviteCode: payload.SourceLobbyInviteCode,
					}, nil
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
				a.attachLobbyReturn(&resp, targetMatchID)
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
				_ = a.store.RecordRuntimeMatch(assigned.MatchID, string(contracts.MatchEnded), assigned.NodeEpoch, true)
			}
		case matchlaunch.AssignmentAbandoned, matchlaunch.AssignmentInvalid:
		}
	}

	if found {
		resp := contracts.MatchSessionResponse{Status: "history", MatchID: targetMatchID, Snapshot: history}
		a.attachLobbyReturn(&resp, targetMatchID)
		return resp, nil
	}
	if rec, ok, err := a.store.GetRuntimeMatch(targetMatchID); err == nil && ok && rec.State != string(contracts.MatchEnded) {
		return contracts.MatchSessionResponse{Status: "live_auth_required", MatchID: targetMatchID}, nil
	}
	return contracts.MatchSessionResponse{Status: "missing", MatchID: targetMatchID}, nil
}

func (a *api) attachLobbyReturn(resp *contracts.MatchSessionResponse, matchID string) {
	if resp == nil || resp.SourceLobbyInviteCode != "" {
		return
	}
	lobby, ok, err := a.store.GetLobbyByMatchID(matchID)
	if err != nil || !ok {
		return
	}
	resp.SourceLobbyID = lobby.ID
	resp.SourceLobbyInviteCode = lobby.InviteCode
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
	matchID := strings.TrimSpace(mux.Vars(r)["id"])
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
	created, err := a.store.CreateModerationReport(persistence.CreateModerationReportParams{
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
		a.startSingleplayerSessionWithConfig(w, r, singleplayerConfigFromRequest(req.Ruleset, req.Config))
	default:
		http.Error(w, "unsupported mode", http.StatusBadRequest)
	}
}

type startSingleplayerSessionRequest struct {
	Ruleset contracts.GameRuleset `json:"ruleset,omitempty"`
	Config  contracts.MatchConfig `json:"config,omitempty"`
}

func singleplayerConfigFromRequest(ruleset contracts.GameRuleset, config contracts.MatchConfig) contracts.MatchConfig {
	if config.Ruleset == "" {
		config.Ruleset = ruleset
	}
	return contracts.NormalizeMatchConfig(config)
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
	var req startSingleplayerSessionRequest
	if err := decodeJSONBody(r, &req); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	a.startSingleplayerSessionWithConfig(w, r, singleplayerConfigFromRequest(req.Ruleset, req.Config))
}

func (a *api) startSingleplayerSessionWithConfig(w http.ResponseWriter, r *http.Request, config contracts.MatchConfig) {
	claims, err := a.authenticatedClaims(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	identity, err := a.store.GetIdentity(claims.Sub)
	if err != nil {
		http.Error(w, "identity not found", http.StatusUnauthorized)
		return
	}
	if identity.IsBanned {
		http.Error(w, "account is banned", http.StatusForbidden)
		return
	}
	if !identity.Onboarded {
		http.Error(w, "onboarding incomplete", http.StatusForbidden)
		return
	}
	if identity.AuthMigrationRequired {
		http.Error(w, "connect discord to continue", http.StatusForbidden)
		return
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
			_ = a.store.RecordRuntimeMatch(assigned.MatchID, string(contracts.MatchEnded), assigned.NodeEpoch, true)
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
	found := contracts.MatchFound{
		MatchID: "solo-" + soloSessionID(),
		Mode:    contracts.ModeSingleplayer,
		Config:  contracts.NormalizeMatchConfig(config),
		Players: []string{userID},
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
		MapScope: "world",
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
	return strconv.FormatInt(time.Now().UnixNano(), 36)
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
