package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gorilla/mux"

	"geoduels/pkg/persistence"
	socialdomain "geoduels/pkg/social"
)

type publicPlayerTestStore struct {
	testRepositories
	profile       persistence.PublicPlayerProfile
	matches       []persistence.MatchHistorySummary
	limit         int
	cursorEndedAt time.Time
	cursorMatchID string
	rankedOnly    bool
	hasMore       bool
}

func (s *publicPlayerTestStore) GetPublicPlayerProfileByNickname(nickname string) (persistence.PublicPlayerProfile, error) {
	profile := s.profile
	if profile.UserID == "" {
		profile.UserID = "player-1"
	}
	if profile.DisplayName == "" {
		profile.DisplayName = nickname
	}
	return profile, nil
}

func (s *publicPlayerTestStore) ListPlayerMatchHistory(userID string, limit int) ([]persistence.MatchHistorySummary, error) {
	s.limit = limit
	return s.matches, nil
}

func (s *publicPlayerTestStore) ListPlayerMatchHistoryPage(userID string, limit int, beforeEndedAt time.Time, beforeMatchID string, rankedOnly bool) (persistence.MatchHistoryPage, error) {
	s.limit = limit
	s.cursorEndedAt = beforeEndedAt
	s.cursorMatchID = beforeMatchID
	s.rankedOnly = rankedOnly
	page := persistence.MatchHistoryPage{Matches: s.matches, HasMore: s.hasMore}
	if s.hasMore && len(s.matches) > 0 {
		last := s.matches[len(s.matches)-1]
		page.NextEndedAt = last.EndedAt
		page.NextMatchID = last.MatchID
	}
	return page, nil
}

func TestPublicPlayerMatchesPassesRankedFilter(t *testing.T) {
	store := &publicPlayerTestStore{}
	a := &api{profiles: store, matchStore: store, social: socialdomain.NewService(store)}
	req := httptest.NewRequest(http.MethodGet, "/v1/players/Explorer/matches?filter=ranked", nil)
	req = mux.SetURLVars(req, map[string]string{"nickname": "Explorer"})
	rec := httptest.NewRecorder()

	a.publicPlayerMatches(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
	if !store.rankedOnly {
		t.Fatal("expected ranked-only match history request")
	}
}

func TestPublicPlayerProfileContainsOnlyPublicFields(t *testing.T) {
	store := &publicPlayerTestStore{
		profile: persistence.PublicPlayerProfile{
			DisplayName: "Explorer",
			MMR:         1420,
			GamesPlayed: 30,
			Wins:        18,
		},
	}
	a := &api{accounts: store, sessions: store, profiles: store, preferenceStore: store, badges: store, leaderboardStore: store, matchStore: store, moderation: store, admin: store, content: store, seasons: store, gameplayMaps: store, runtimeStore: store, chatStore: store, parties: store, social: socialdomain.NewService(store)}
	req := httptest.NewRequest(http.MethodGet, "/v1/players/Explorer", nil)
	req = mux.SetURLVars(req, map[string]string{"nickname": "Explorer"})
	rec := httptest.NewRecorder()

	a.publicPlayerProfile(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
	var body map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatal(err)
	}
	for _, privateField := range []string{"email", "isAdmin", "isModerator", "isBanned", "banReason", "lastIpAddress", "identities"} {
		if _, exists := body[privateField]; exists {
			t.Fatalf("public profile exposed %q", privateField)
		}
	}
	if body["displayName"] != "Explorer" {
		t.Fatalf("displayName = %v", body["displayName"])
	}
}

func TestPublicPlayerMatchesUsesRequestedSummaryLimit(t *testing.T) {
	store := &publicPlayerTestStore{
		matches: []persistence.MatchHistorySummary{{
			MatchID: "match-1",
			Mode:    "duel",
			EndedAt: time.Now(),
		}},
	}
	a := &api{accounts: store, sessions: store, profiles: store, preferenceStore: store, badges: store, leaderboardStore: store, matchStore: store, moderation: store, admin: store, content: store, seasons: store, gameplayMaps: store, runtimeStore: store, chatStore: store, parties: store, social: socialdomain.NewService(store)}
	req := httptest.NewRequest(http.MethodGet, "/v1/players/Explorer/matches?limit=25", nil)
	req = mux.SetURLVars(req, map[string]string{"nickname": "Explorer"})
	rec := httptest.NewRecorder()

	a.publicPlayerMatches(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
	if store.limit != 25 {
		t.Fatalf("limit = %d, want 25", store.limit)
	}
}

func TestPublicPlayerMatchesRejectsInvalidLimit(t *testing.T) {
	a := &api{profiles: &publicPlayerTestStore{}, matchStore: &publicPlayerTestStore{}, social: socialdomain.NewService(&publicPlayerTestStore{})}
	req := httptest.NewRequest(http.MethodGet, "/v1/players/Explorer/matches?limit=101", nil)
	req = mux.SetURLVars(req, map[string]string{"nickname": "Explorer"})
	rec := httptest.NewRecorder()
	a.publicPlayerMatches(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestPublicPlayerMatchesRejectsInvalidCursor(t *testing.T) {
	a := &api{profiles: &publicPlayerTestStore{}, matchStore: &publicPlayerTestStore{}, social: socialdomain.NewService(&publicPlayerTestStore{})}
	req := httptest.NewRequest(http.MethodGet, "/v1/players/Explorer/matches?cursor=bad", nil)
	req = mux.SetURLVars(req, map[string]string{"nickname": "Explorer"})
	rec := httptest.NewRecorder()
	a.publicPlayerMatches(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestPublicPlayerMatchesReturnsNextCursor(t *testing.T) {
	store := &publicPlayerTestStore{hasMore: true, matches: []persistence.MatchHistorySummary{{MatchID: "00000000-0000-0000-0000-000000000001", Mode: "duel", EndedAt: time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)}}}
	a := &api{profiles: store, matchStore: store, social: socialdomain.NewService(store)}
	req := httptest.NewRequest(http.MethodGet, "/v1/players/Explorer/matches", nil)
	req = mux.SetURLVars(req, map[string]string{"nickname": "Explorer"})
	rec := httptest.NewRecorder()
	a.publicPlayerMatches(rec, req)
	var payload struct {
		NextCursor string `json:"nextCursor"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.NextCursor == "" {
		t.Fatal("expected next cursor")
	}
}
