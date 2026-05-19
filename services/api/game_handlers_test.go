package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gorilla/mux"
	"github.com/redis/go-redis/v9"

	"geoduels/pkg/auth"
	"geoduels/pkg/contracts"
	"geoduels/pkg/coordinator"
	"geoduels/pkg/persistence"
)

type matchAccessTestStore struct {
	persistence.Store
	snapshot []byte
}

func (s *matchAccessTestStore) GetFinalMatchSnapshot(matchID string) ([]byte, bool, error) {
	if matchID != "match-1" || len(s.snapshot) == 0 {
		return nil, false, nil
	}
	return s.snapshot, true, nil
}

func (s *matchAccessTestStore) GetIdentity(sub string) (persistence.Identity, error) {
	return persistence.Identity{Sub: sub}, nil
}

func (s *matchAccessTestStore) GetRuntimeMatch(matchID string) (persistence.RuntimeMatch, bool, error) {
	return persistence.RuntimeMatch{}, false, nil
}

func (s *matchAccessTestStore) GetLobbyByMatchID(matchID string) (contracts.LobbySnapshot, bool, error) {
	return contracts.LobbySnapshot{}, false, nil
}

func TestPublicFinalMatchSnapshotIsAvailableToAnyViewer(t *testing.T) {
	raw, err := json.Marshal(contracts.MatchSnapshot{
		MatchID: "match-1",
		State:   contracts.MatchEnded,
		Players: map[string]contracts.PlayerState{
			"player-1": {UserID: "player-1"},
			"player-2": {UserID: "player-2"},
		},
	})
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}

	a := &api{store: &matchAccessTestStore{snapshot: raw}}
	snapshot, found, err := a.getPublicFinalMatchSnapshot("match-1")
	if err != nil {
		t.Fatalf("get snapshot: %v", err)
	}
	if !found || snapshot == nil {
		t.Fatal("expected snapshot to be found")
	}
}

func TestMatchRouteReturnsPublicHistoryWithoutAuth(t *testing.T) {
	raw, err := json.Marshal(contracts.MatchSnapshot{
		MatchID: "match-1",
		State:   contracts.MatchEnded,
		Phase:   contracts.PhaseEnded,
		Players: map[string]contracts.PlayerState{
			"player-1": {UserID: "player-1", LastGuessLat: 1, LastGuessLng: 2, Disconnected: true},
			"player-2": {UserID: "player-2"},
		},
	})
	if err != nil {
		t.Fatalf("marshal snapshot: %v", err)
	}

	a := &api{store: &matchAccessTestStore{snapshot: raw}}
	req := httptest.NewRequest(http.MethodGet, "/v1/matches/match-1/route", nil)
	req = mux.SetURLVars(req, map[string]string{"id": "match-1"})
	rec := httptest.NewRecorder()

	a.matchRoute(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
	var resp contracts.MatchSessionResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Status != "history" || resp.Snapshot == nil {
		t.Fatalf("unexpected response: %+v", resp)
	}
	if got := resp.Snapshot.Players["player-1"]; got.LastGuessLat != 0 || got.LastGuessLng != 0 || got.Disconnected {
		t.Fatalf("snapshot was not sanitized: %+v", got)
	}
}

func TestMatchSessionAllowsGuestAssignedToLiveMatch(t *testing.T) {
	const matchID = "match-1"
	appSecret := []byte("01234567890123456789012345678901")
	ticketSecret := []byte("abcdefghijklmnopqrstuvwxyz012345")

	gameplay := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead && r.URL.Path == "/internal/matches/"+matchID {
			w.WriteHeader(http.StatusOK)
			return
		}
		http.NotFound(w, r)
	}))
	defer gameplay.Close()

	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()
	coordStore := coordinator.NewStore(rdb, time.Minute, time.Hour, time.Hour, time.Second)
	if err := coordStore.RegisterNode(t.Context(), coordinator.NodeRecord{
		NodeID:      "node-1",
		PublicRoute: "node-1",
		InternalURL: gameplay.URL,
	}); err != nil {
		t.Fatalf("register node: %v", err)
	}
	if err := coordStore.SaveAssignment(t.Context(), coordinator.Assignment{
		MatchID:     matchID,
		Mode:        contracts.ModeDuel,
		NodeID:      "node-1",
		PublicRoute: "node-1",
		Players:     []string{"guest-1", "guest-2"},
	}); err != nil {
		t.Fatalf("save assignment: %v", err)
	}

	token, err := auth.IssueAppAccessToken(appSecret, "guest-2", "session-1", time.Minute)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}

	a := &api{
		store:          &matchAccessTestStore{},
		coord:          coordStore,
		appAuthSecret:  appSecret,
		ticketAuth:     ticketSecret,
		internalSecret: "",
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/matches/"+matchID+"/session", nil)
	req = mux.SetURLVars(req, map[string]string{"id": matchID})
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	a.matchSession(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
	var resp contracts.MatchSessionResponse
	if err := json.NewDecoder(rec.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if resp.Status != "live_connectable" {
		t.Fatalf("status = %q, want live_connectable", resp.Status)
	}
	if resp.MatchID != matchID || resp.Node != "node-1" || resp.WSPath != "/ws/node-1" || resp.Ticket == "" {
		t.Fatalf("unexpected response: %+v", resp)
	}
}

func TestSingleplayerConfigFromRequest(t *testing.T) {
	tests := []struct {
		name string
		body string
		want contracts.GameRuleset
	}{
		{name: "empty defaults moving", body: `{}`, want: contracts.RulesetMoving},
		{name: "top level nmpz", body: `{"ruleset":"nmpz"}`, want: contracts.RulesetNMPZ},
		{name: "config no move", body: `{"config":{"ruleset":"no_move"}}`, want: contracts.RulesetNoMove},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/v1/singleplayer/session", strings.NewReader(tt.body))
			var payload startSingleplayerSessionRequest
			if err := decodeJSONBody(req, &payload); err != nil {
				t.Fatalf("decode: %v", err)
			}
			cfg := singleplayerConfigFromRequest(payload.Ruleset, payload.Config)
			if cfg.Ruleset != tt.want {
				t.Fatalf("ruleset = %q, want %q", cfg.Ruleset, tt.want)
			}
		})
	}
}
