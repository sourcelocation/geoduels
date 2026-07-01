package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"

	"geoduels/pkg/auth"
	"geoduels/pkg/persistence"
)

type adminModerationTestStore struct {
	persistence.Store
	identity         persistence.Identity
	bannedUserID     string
	bannedReason     string
	banned           bool
	refundsRequested bool
}

func (s *adminModerationTestStore) GetIdentity(sub string) (persistence.Identity, error) {
	return s.identity, nil
}

func (s *adminModerationTestStore) SetPlayerBan(userID, reason string, banned bool) error {
	s.bannedUserID = userID
	s.bannedReason = reason
	s.banned = banned
	return nil
}

func (s *adminModerationTestStore) BanPlayerForCheating(userID, reason, actorUserID string) (persistence.CheatingBanSummary, error) {
	s.refundsRequested = true
	s.bannedUserID = userID
	s.bannedReason = reason
	s.banned = true
	return persistence.CheatingBanSummary{UserID: userID, Reason: reason, Refunds: persistence.EloRefundSummary{RefundsIssued: 2, TotalRefunded: 30}}, nil
}

func TestModeratorCanBanPlayer(t *testing.T) {
	secret := []byte("01234567890123456789012345678901")
	token, err := auth.IssueAppAccessToken(secret, "moderator-1", "session-1", time.Minute)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	store := &adminModerationTestStore{
		identity: persistence.Identity{
			Sub:         "moderator-1",
			IsModerator: true,
		},
	}
	a := &api{
		store:                store,
		appAuthSecret:        secret,
		adminBootstrapEmails: map[string]struct{}{},
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/admin/players/user-2/ban", strings.NewReader(`{"reason":"reported cheating"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req = mux.SetURLVars(req, map[string]string{"id": "user-2"})
	rec := httptest.NewRecorder()

	a.adminBanPlayer(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
	if store.bannedUserID != "user-2" || !store.banned {
		t.Fatalf("expected user-2 to be banned, got userID=%q banned=%v", store.bannedUserID, store.banned)
	}
	if store.bannedReason != "reported cheating" {
		t.Fatalf("ban reason = %q", store.bannedReason)
	}
	if !store.refundsRequested {
		t.Fatalf("expected cheating-ban refund flow to run")
	}
}

func TestModeratorCanCheatingBanFromModeratorRoute(t *testing.T) {
	secret := []byte("01234567890123456789012345678901")
	token, err := auth.IssueAppAccessToken(secret, "moderator-1", "session-1", time.Minute)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	store := &adminModerationTestStore{
		identity: persistence.Identity{
			Sub:         "moderator-1",
			IsModerator: true,
		},
	}
	a := &api{
		store:                store,
		appAuthSecret:        secret,
		adminBootstrapEmails: map[string]struct{}{},
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/moderator/subjects/user-2/cheating-ban", strings.NewReader(`{"reason":"cheating_confirmed: reviewed incident 123"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req = mux.SetURLVars(req, map[string]string{"userId": "user-2"})
	rec := httptest.NewRecorder()

	a.moderatorSubjectCheatingBan(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
	if store.bannedUserID != "user-2" || !store.banned || !store.refundsRequested {
		t.Fatalf("expected cheating ban with refunds, userID=%q banned=%v refunds=%v", store.bannedUserID, store.banned, store.refundsRequested)
	}
	if store.bannedReason != "cheating_confirmed: reviewed incident 123" {
		t.Fatalf("ban reason = %q", store.bannedReason)
	}
}

func TestModeratorCanUnbanFromModeratorRoute(t *testing.T) {
	secret := []byte("01234567890123456789012345678901")
	token, err := auth.IssueAppAccessToken(secret, "moderator-1", "session-1", time.Minute)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	store := &adminModerationTestStore{
		identity: persistence.Identity{
			Sub:         "moderator-1",
			IsModerator: true,
		},
	}
	a := &api{
		store:                store,
		appAuthSecret:        secret,
		adminBootstrapEmails: map[string]struct{}{},
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/moderator/subjects/user-2/unban", strings.NewReader(`{"reason":"appeal accepted"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req = mux.SetURLVars(req, map[string]string{"userId": "user-2"})
	rec := httptest.NewRecorder()

	a.moderatorSubjectUnban(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
	if store.bannedUserID != "user-2" || store.banned {
		t.Fatalf("expected user-2 to be unbanned, got userID=%q banned=%v", store.bannedUserID, store.banned)
	}
	if store.bannedReason != "appeal accepted" {
		t.Fatalf("unban reason = %q", store.bannedReason)
	}
}

func TestNonModeratorCannotBanPlayer(t *testing.T) {
	secret := []byte("01234567890123456789012345678901")
	token, err := auth.IssueAppAccessToken(secret, "player-1", "session-1", time.Minute)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	store := &adminModerationTestStore{
		identity: persistence.Identity{Sub: "player-1"},
	}
	a := &api{
		store:                store,
		appAuthSecret:        secret,
		adminBootstrapEmails: map[string]struct{}{},
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/admin/players/user-2/ban", strings.NewReader(`{"reason":"reported cheating"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req = mux.SetURLVars(req, map[string]string{"id": "user-2"})
	rec := httptest.NewRecorder()

	a.adminBanPlayer(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
	if store.bannedUserID != "" || store.refundsRequested {
		t.Fatalf("plain player should not ban or issue refunds, bannedUserID=%q refunds=%v", store.bannedUserID, store.refundsRequested)
	}
}

func TestNormalizeDiscordWebhookURL(t *testing.T) {
	valid := "https://discord.com/api/webhooks/123/token"
	got, err := normalizeDiscordWebhookURL("  " + valid + "  ")
	if err != nil {
		t.Fatalf("valid webhook rejected: %v", err)
	}
	if got != valid {
		t.Fatalf("normalized url = %q, want %q", got, valid)
	}

	if got, err := normalizeDiscordWebhookURL(" "); err != nil || got != "" {
		t.Fatalf("blank webhook = %q, %v; want empty nil", got, err)
	}

	invalid := []string{
		"http://discord.com/api/webhooks/123/token",
		"https://example.com/api/webhooks/123/token",
		"https://discord.com/channels/123",
	}
	for _, raw := range invalid {
		if _, err := normalizeDiscordWebhookURL(raw); err == nil {
			t.Fatalf("expected %q to be rejected", raw)
		}
	}
}

func TestValidateOptionalDiscordSnowflake(t *testing.T) {
	if err := validateOptionalDiscordSnowflake("guild id", "123456789012345678"); err != nil {
		t.Fatalf("valid Discord ID rejected: %v", err)
	}
	if err := validateOptionalDiscordSnowflake("guild id", ""); err != nil {
		t.Fatalf("empty optional Discord ID rejected: %v", err)
	}
	for _, value := range []string{"123", "12345678901234567x", "123456789012345678901"} {
		if err := validateOptionalDiscordSnowflake("guild id", value); err == nil {
			t.Fatalf("invalid Discord ID %q accepted", value)
		}
	}
}
