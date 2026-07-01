package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"geoduels/pkg/auth"
	"geoduels/pkg/persistence"
)

type mapUserTestStore struct {
	persistence.Store
	profile persistence.Profile
}

func (s *mapUserTestStore) GetProfile(userID string) (persistence.Profile, error) {
	p := s.profile
	p.UserID = userID
	return p, nil
}

func TestMapUserRejectsGuestWhenRegisteredAccountRequired(t *testing.T) {
	secret := []byte("01234567890123456789012345678901")
	token, err := auth.IssueAppAccessToken(secret, "guest-1", "session-1", time.Minute)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	a := &api{
		store:         &mapUserTestStore{profile: persistence.Profile{IsGuest: true}},
		appAuthSecret: secret,
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/maps/map-1/favorite", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	userID, ok := a.mapUser(rec, req, true)

	if ok || userID != "" {
		t.Fatalf("guest map user unexpectedly allowed: ok=%v userID=%q", ok, userID)
	}
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "guest accounts cannot interact with maps") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestMapUserAllowsRegisteredAccountForMapInteractions(t *testing.T) {
	secret := []byte("01234567890123456789012345678901")
	token, err := auth.IssueAppAccessToken(secret, "user-1", "session-1", time.Minute)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	a := &api{
		store:         &mapUserTestStore{profile: persistence.Profile{IsGuest: false}},
		appAuthSecret: secret,
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/maps/map-1/favorite", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	userID, ok := a.mapUser(rec, req, true)

	if !ok || userID != "user-1" {
		t.Fatalf("registered map user rejected: ok=%v userID=%q", ok, userID)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
}
