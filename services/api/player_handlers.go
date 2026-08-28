package main

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"
)

type playerMatchesCursor struct {
	EndedAt string `json:"endedAt"`
	MatchID string `json:"matchId"`
}

func (a *api) publicPlayerProfile(w http.ResponseWriter, r *http.Request) {
	nickname := strings.TrimSpace(mux.Vars(r)["nickname"])
	profile, err := a.profiles.GetPublicPlayerProfileByNickname(nickname)
	if err != nil {
		if errors.Is(err, ErrNoRows) {
			http.Error(w, "player not found", http.StatusNotFound)
			return
		}
		http.Error(w, "player profile unavailable", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(profile)
}

func (a *api) publicPlayerMatches(w http.ResponseWriter, r *http.Request) {
	limit := 20
	if raw := strings.TrimSpace(r.URL.Query().Get("limit")); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 100 {
			http.Error(w, "invalid limit", http.StatusBadRequest)
			return
		}
		limit = parsed
	}
	profile, err := a.profiles.GetPublicPlayerProfileByNickname(strings.TrimSpace(mux.Vars(r)["nickname"]))
	if err != nil {
		if errors.Is(err, ErrNoRows) {
			http.Error(w, "player not found", http.StatusNotFound)
			return
		}
		http.Error(w, "player profile unavailable", http.StatusInternalServerError)
		return
	}
	var beforeEndedAt time.Time
	var beforeMatchID string
	rankedOnly := strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("filter")), "ranked") ||
		strings.EqualFold(strings.TrimSpace(r.URL.Query().Get("ranked")), "true")
	if rawCursor := strings.TrimSpace(r.URL.Query().Get("cursor")); rawCursor != "" {
		cursor, err := decodePlayerMatchesCursor(rawCursor)
		if err != nil {
			http.Error(w, "invalid cursor", http.StatusBadRequest)
			return
		}
		beforeEndedAt, err = time.Parse(time.RFC3339Nano, cursor.EndedAt)
		if err != nil || strings.TrimSpace(cursor.MatchID) == "" {
			http.Error(w, "invalid cursor", http.StatusBadRequest)
			return
		}
		beforeMatchID = a.resolveEntityID("match", cursor.MatchID)
	}
	page, err := a.matchStore.ListPlayerMatchHistoryPage(profile.UserID, limit, beforeEndedAt, beforeMatchID, rankedOnly)
	if err != nil {
		http.Error(w, "match history unavailable", http.StatusInternalServerError)
		return
	}
	nextCursor := ""
	if page.HasMore {
		nextCursor = encodePlayerMatchesCursor(playerMatchesCursor{
			EndedAt: page.NextEndedAt.UTC().Format(time.RFC3339Nano),
			MatchID: page.NextMatchID,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]any{
		"matches":    page.Matches,
		"nextCursor": nextCursor,
	})
}

func encodePlayerMatchesCursor(cursor playerMatchesCursor) string {
	raw, _ := json.Marshal(cursor)
	return base64.RawURLEncoding.EncodeToString(raw)
}

func decodePlayerMatchesCursor(raw string) (playerMatchesCursor, error) {
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return playerMatchesCursor{}, err
	}
	var cursor playerMatchesCursor
	if err := json.Unmarshal(decoded, &cursor); err != nil {
		return playerMatchesCursor{}, err
	}
	return cursor, nil
}
