package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"

	"geoduels/pkg/auth"
	"geoduels/pkg/contracts"
	"geoduels/pkg/maintenance"
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

func (a *api) getPublicFinalMatchSnapshot(matchID string) (*contracts.MatchSnapshot, bool, error) {
	recorded, ok, err := a.matchStore.FinalMatchSnapshot(matchID)
	if err != nil || !ok {
		return nil, ok, err
	}
	snapshot := sanitizeFinalMatchSnapshot(*recorded)
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

func maintenancePlayMessage(status maintenance.Status) string {
	if status.Message != "" {
		return status.Message
	}
	switch status.Phase {
	case maintenance.PhaseActive:
		return "Maintenance in progress. New matches are temporarily unavailable."
	case maintenance.PhaseWarning:
		return "New matches have been paused for scheduled maintenance."
	default:
		return "Matches are temporarily unavailable"
	}
}
