package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/labstack/echo/v4"

	"geoduels/internal/badges"
	"geoduels/internal/inbox"
	"geoduels/pkg/auth"
	"geoduels/pkg/gameplay"
	"geoduels/pkg/maintenance"
)

// playerBadge is a badge as clients show it.
type playerBadge struct {
	ID           string `json:"id"`
	Kind         string `json:"kind"`
	Label        string `json:"label"`
	Description  string `json:"description"`
	ImageURL     string `json:"imageUrl"`
	Rarity       string `json:"rarity,omitempty"`
	Level        int    `json:"level,omitempty"`
	MaxLevel     int    `json:"maxLevel,omitempty"`
	Extra        int    `json:"extra,omitempty"`
	Owned        bool   `json:"owned"`
	Unobtainable bool   `json:"unobtainable,omitempty"`
}

func newPlayerBadge(b badges.Badge) playerBadge {
	return playerBadge{
		ID: b.ID, Kind: b.Kind, Label: b.Label, Description: b.Description, ImageURL: b.ImageURL, Rarity: b.Rarity,
		Level: b.Level, MaxLevel: b.MaxLevel, Extra: b.Extra, Owned: b.Owned, Unobtainable: b.Unobtainable,
	}
}

func newPlayerBadgeRef(b *badges.Badge) *playerBadge {
	if b == nil {
		return nil
	}
	dto := newPlayerBadge(*b)
	return &dto
}

func newPlayerBadges(list []badges.Badge) []playerBadge {
	if list == nil {
		return nil
	}
	out := make([]playerBadge, len(list))
	for i, b := range list {
		out[i] = newPlayerBadge(b)
	}
	return out
}

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
		"badges":        newPlayerBadges(profile.Badges),
		"selectedBadge": newPlayerBadgeRef(profile.SelectedBadge),
	})
}

func (a *api) inboxPage(c echo.Context) error {
	r := c.Request()
	claims, err := a.authenticatedClaims(r)
	if err != nil {
		return plainTextError(c, http.StatusUnauthorized, "unauthorized")
	}
	before, _ := strconv.ParseInt(r.URL.Query().Get("before"), 10, 64)
	messages, err := inbox.Page(r.Context(), a.db.Pool(), claims.Sub, queryLimit(r, inboxStartSize), before)
	if err != nil {
		return plainTextError(c, http.StatusInternalServerError, "inbox unavailable")
	}
	return writeJSON(c, map[string]any{"messages": newInboxMessages(messages)})
}

func (a *api) markInboxRead(c echo.Context) error {
	r := c.Request()
	claims, err := a.authenticatedClaims(r)
	if err != nil {
		return plainTextError(c, http.StatusUnauthorized, "unauthorized")
	}
	id, err := strconv.ParseInt(strings.TrimSpace(c.Param("id")), 10, 64)
	if err != nil {
		return plainTextError(c, http.StatusBadRequest, "invalid message id")
	}
	if err := inbox.MarkRead(r.Context(), a.db.Pool(), claims.Sub, id); err != nil {
		return plainTextError(c, http.StatusInternalServerError, "inbox unavailable")
	}
	a.tell([]string{claims.Sub}, inboxRead{ID: id})
	return c.NoContent(http.StatusNoContent)
}

func (a *api) markAllInboxRead(c echo.Context) error {
	r := c.Request()
	claims, err := a.authenticatedClaims(r)
	if err != nil {
		return plainTextError(c, http.StatusUnauthorized, "unauthorized")
	}
	if err := inbox.MarkAllRead(r.Context(), a.db.Pool(), claims.Sub); err != nil {
		return plainTextError(c, http.StatusInternalServerError, "inbox unavailable")
	}
	a.tell([]string{claims.Sub}, inboxRead{})
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
	result, found, err := a.publicMatchResult(id)
	if err != nil || !found {
		return plainTextError(c, http.StatusNotFound, "match not found")
	}
	return json.NewEncoder(c.Response()).Encode(result)
}

// publicMatchResult is a recorded match's final snapshot as anyone may see it.
func (a *api) publicMatchResult(matchID string) (*gameplay.ClientMatchResult, bool, error) {
	recorded, ok, err := a.matchStore.FinalMatchSnapshot(matchID)
	if err != nil || !ok {
		return nil, ok, err
	}
	snapshot := sanitizeFinalMatchSnapshot(*recorded)
	if snapshot.State == "" {
		snapshot.State = gameplay.MatchEnded
	}
	return gameplay.NewClientMatchResult(&snapshot), true, nil
}

func sanitizeFinalMatchSnapshot(snapshot gameplay.MatchSnapshot) gameplay.MatchSnapshot {
	if snapshot.State == gameplay.MatchEnded {
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
	return writeJSONStatus(c, http.StatusCreated, moderationSignalCreated{SignalID: created.SignalID, Status: created.Status})
}

// moderationSignalCreated is what a player learns of their report.
type moderationSignalCreated struct {
	SignalID int64  `json:"signalId"`
	Status   string `json:"status"`
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
