package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"geoduels/pkg/auth"
	"geoduels/pkg/persistence"
)

type preferencesTestStore struct {
	persistence.Store
	value persistence.UserPreferences
}

func (s *preferencesTestStore) GetUserPreferences(string) (persistence.UserPreferences, error) {
	return s.value, nil
}

func (s *preferencesTestStore) UpdateUserPreferences(_ string, version int, preferences json.RawMessage, revision int64) (persistence.UserPreferences, error) {
	if revision != s.value.Revision {
		return persistence.UserPreferences{}, persistence.ErrPreferenceRevisionConflict
	}
	s.value = persistence.UserPreferences{
		SchemaVersion: version,
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
	return &api{store: store, appAuthSecret: secret}, request, httptest.NewRecorder()
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
