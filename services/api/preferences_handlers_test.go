package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"geoduels/pkg/auth"
	"geoduels/pkg/persistence"
	preferencesdomain "geoduels/pkg/preferences"
)

type preferencesTestStore struct {
	testRepositories
	value persistence.UserPreferences
}

func (s *preferencesTestStore) GetUserPreferences(context.Context, string) (persistence.UserPreferences, error) {
	return s.value, nil
}

func (s *preferencesTestStore) UpdateUserPreferences(_ context.Context, _ string, _ int, preferences json.RawMessage, revision int64) (persistence.UserPreferences, error) {
	if revision != s.value.Revision {
		return persistence.UserPreferences{}, ErrPreferenceRevisionConflict
	}
	s.value = persistence.UserPreferences{
		SchemaVersion: 1,
		Preferences:   preferences,
		Revision:      revision + 1,
	}
	return s.value, nil
}

func preferenceRequest(t *testing.T, method, body string, store *preferencesTestStore) (*api, *http.Request, *httptest.ResponseRecorder) {
	t.Helper()
	secret := []byte("01234567890123456789012345678901")
	token, err := auth.IssueAppAccessToken(secret, "user-1", "session-1", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	request := httptest.NewRequest(method, "/v1/me/preferences", strings.NewReader(body))
	request.Header.Set("Authorization", "Bearer "+token)
	return &api{accounts: store, sessions: store, profiles: store, preferenceStore: store, badges: store, leaderboardStore: store, matchStore: store, moderation: store, admin: store, content: store, seasons: store, gameplayMaps: store, runtimeStore: store, chatStore: store, parties: store, social: store, preferences: preferencesdomain.NewService(preferenceTestAdapter{store: store}), appAuthSecret: secret}, request, httptest.NewRecorder()
}

type preferenceTestAdapter struct{ store *preferencesTestStore }

func (a preferenceTestAdapter) GetUserPreferences(ctx context.Context, userID string) (preferencesdomain.UserPreferences, error) {
	v, err := a.store.GetUserPreferences(ctx, userID)
	return preferencesdomain.UserPreferences{SchemaVersion: v.SchemaVersion, Preferences: v.Preferences, Revision: v.Revision}, err
}
func (a preferenceTestAdapter) UpdateUserPreferences(ctx context.Context, userID string, version int, value json.RawMessage, revision int64) (preferencesdomain.UserPreferences, error) {
	v, err := a.store.UpdateUserPreferences(ctx, userID, version, value, revision)
	if errors.Is(err, ErrPreferenceRevisionConflict) {
		return preferencesdomain.UserPreferences{}, preferencesdomain.ErrRevisionConflict
	}
	return preferencesdomain.UserPreferences{SchemaVersion: v.SchemaVersion, Preferences: v.Preferences, Revision: v.Revision}, err
}

func TestUpdateUserPreferences(t *testing.T) {
	store := &preferencesTestStore{value: persistence.UserPreferences{
		SchemaVersion: 1,
		Preferences:   json.RawMessage(`{}`),
		Revision:      3,
	}}
	a, request, response := preferenceRequest(t, http.MethodPatch, `{"revision":3,"preferences":{"version":1,"audioMuted":true,"bindings":{}}}`, store)

	a.updateUserPreferences(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", response.Code, response.Body.String())
	}
	if store.value.Revision != 4 {
		t.Fatalf("revision = %d, want 4", store.value.Revision)
	}
}

func TestUpdateUserPreferencesRejectsStaleRevision(t *testing.T) {
	store := &preferencesTestStore{value: persistence.UserPreferences{Revision: 2}}
	a, request, response := preferenceRequest(t, http.MethodPatch, `{"revision":1,"preferences":{"version":1}}`, store)

	a.updateUserPreferences(response, request)

	if response.Code != http.StatusConflict {
		t.Fatalf("status = %d, want %d", response.Code, http.StatusConflict)
	}
}
