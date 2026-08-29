package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"geoduels/pkg/auth"
	"geoduels/pkg/persistence"
	socialdomain "geoduels/pkg/social"
)

func TestActiveAccountRejectsBannedUserWithStructuredError(t *testing.T) {
	secret := []byte("01234567890123456789012345678901")
	token, err := auth.IssueAppAccessToken(secret, "user-1", "session-1", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	store := &adminModerationTestStore{identity: persistence.Identity{Sub: "user-1", IsBanned: true}}
	a := &api{accounts: store, sessions: store, profiles: store, preferenceStore: store, badges: store, leaderboardStore: store, matchStore: store, moderation: store, admin: store, content: store, seasons: store, gameplayMaps: store, runtimeStore: store, chatStore: store, parties: store, social: socialdomain.NewService(store), appAuthSecret: secret}
	called := false
	handler := a.active(func(http.ResponseWriter, *http.Request) { called = true })
	req := httptest.NewRequest(http.MethodPost, "/action", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden || called {
		t.Fatalf("status=%d called=%v", rec.Code, called)
	}
	if body := rec.Body.String(); !strings.Contains(body, `"code":"account_banned"`) || !strings.Contains(body, `"error":"user is banned"`) {
		t.Fatalf("body=%q", body)
	}
}

func TestActiveAccountPassesResolvedIdentityToHandler(t *testing.T) {
	secret := []byte("01234567890123456789012345678901")
	token, err := auth.IssueAppAccessToken(secret, "user-1", "session-1", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	store := &adminModerationTestStore{identity: persistence.Identity{Sub: "user-1"}}
	a := &api{accounts: store, sessions: store, profiles: store, preferenceStore: store, badges: store, leaderboardStore: store, matchStore: store, moderation: store, admin: store, content: store, seasons: store, gameplayMaps: store, runtimeStore: store, chatStore: store, parties: store, social: socialdomain.NewService(store), appAuthSecret: secret}
	handler := a.active(func(w http.ResponseWriter, r *http.Request) {
		_, identity, err := a.authenticatedAccount(r)
		if err != nil || identity.Sub != "user-1" {
			t.Fatalf("request identity=%+v err=%v", identity, err)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	req := httptest.NewRequest(http.MethodPost, "/action", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status=%d", rec.Code)
	}
}
