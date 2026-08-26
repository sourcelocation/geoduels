package main

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"

	"geoduels/pkg/auth"
	"geoduels/pkg/contracts"
	"geoduels/pkg/persistence"
)

type adminModerationTestStore struct {
	persistence.Store
	identity         persistence.Identity
	bannedUserID     string
	bannedReason     string
	banned           bool
	refundsRequested bool
	grantableBadges  []persistence.AdminBadgeDefinition
	grantedNickname  string
	grantedBadgeID   string
}

const moderationTargetUserID = "00000000-0000-7000-8000-000000000002"

func (s *adminModerationTestStore) GetIdentity(sub string) (persistence.Identity, error) {
	return s.identity, nil
}

func (s *adminModerationTestStore) ListAdminGrantableBadges() []persistence.AdminBadgeDefinition {
	return s.grantableBadges
}

func (s *adminModerationTestStore) GrantBadgeToUser(nickname, badgeID, actorUserID string) (contracts.PlayerBadge, bool, error) {
	s.grantedNickname = nickname
	s.grantedBadgeID = badgeID
	return contracts.PlayerBadge{ID: badgeID, Level: 1, Owned: true}, true, nil
}

func TestAdminCanListAndGrantBadges(t *testing.T) {
	secret := []byte("01234567890123456789012345678901")
	token, err := auth.IssueAppAccessToken(secret, "admin-1", "session-1", time.Minute)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	store := &adminModerationTestStore{
		identity:        persistence.Identity{Sub: "admin-1", IsAdmin: true},
		grantableBadges: []persistence.AdminBadgeDefinition{{ID: "event-winner-2026", Label: "2026 Event Winner", MaxLevel: 1}},
	}
	a := &api{store: store, appAuthSecret: secret, adminBootstrapEmails: map[string]struct{}{}}

	listReq := httptest.NewRequest(http.MethodGet, "/v1/admin/badges", nil)
	listReq.Header.Set("Authorization", "Bearer "+token)
	listRec := httptest.NewRecorder()
	a.adminBadgeDefinitions(listRec, listReq)
	if listRec.Code != http.StatusOK || !strings.Contains(listRec.Body.String(), "event-winner-2026") {
		t.Fatalf("badge catalog status=%d body=%q", listRec.Code, listRec.Body.String())
	}

	grantReq := httptest.NewRequest(http.MethodPost, "/v1/admin/badges/grant", strings.NewReader(`{"nickname":"MapMaster","badgeId":"event-winner-2026"}`))
	grantReq.Header.Set("Authorization", "Bearer "+token)
	grantRec := httptest.NewRecorder()
	a.adminGrantBadge(grantRec, grantReq)
	if grantRec.Code != http.StatusOK {
		t.Fatalf("grant status=%d body=%q", grantRec.Code, grantRec.Body.String())
	}
	if store.grantedNickname != "MapMaster" || store.grantedBadgeID != "event-winner-2026" {
		t.Fatalf("grant args nickname=%q badge=%q", store.grantedNickname, store.grantedBadgeID)
	}
}

func TestNonAdminCannotGrantBadges(t *testing.T) {
	secret := []byte("01234567890123456789012345678901")
	token, err := auth.IssueAppAccessToken(secret, "player-1", "session-1", time.Minute)
	if err != nil {
		t.Fatalf("issue token: %v", err)
	}
	store := &adminModerationTestStore{identity: persistence.Identity{Sub: "player-1"}}
	a := &api{store: store, appAuthSecret: secret, adminBootstrapEmails: map[string]struct{}{}}
	req := httptest.NewRequest(http.MethodPost, "/v1/admin/badges/grant", strings.NewReader(`{"nickname":"MapMaster","badgeId":"event-winner-2026"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	a.adminGrantBadge(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status=%d body=%q", rec.Code, rec.Body.String())
	}
	if store.grantedNickname != "" {
		t.Fatalf("non-admin should not grant badge: %q", store.grantedNickname)
	}
}

func (s *adminModerationTestStore) SetPlayerBan(userID, reason, actorUserID string, banned bool) error {
	s.bannedUserID = userID
	s.bannedReason = reason
	s.banned = banned
	return nil
}

func (s *adminModerationTestStore) SetPlayerMute(userID, kind, reason, actorUserID string, until time.Time, muted bool) error {
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

	req := httptest.NewRequest(http.MethodPost, "/v1/admin/players/"+moderationTargetUserID+"/ban", strings.NewReader(`{"reason":"reported cheating"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req = mux.SetURLVars(req, map[string]string{"id": moderationTargetUserID})
	rec := httptest.NewRecorder()

	a.adminBanPlayer(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
	if store.bannedUserID != moderationTargetUserID || !store.banned {
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

	req := httptest.NewRequest(http.MethodPost, "/v1/moderator/subjects/"+moderationTargetUserID+"/cheating-ban", strings.NewReader(`{"reason":"cheating_confirmed: reviewed incident 123"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req = mux.SetURLVars(req, map[string]string{"userId": moderationTargetUserID})
	rec := httptest.NewRecorder()

	a.moderatorSubjectCheatingBan(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
	if store.bannedUserID != moderationTargetUserID || !store.banned || !store.refundsRequested {
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

	req := httptest.NewRequest(http.MethodPost, "/v1/moderator/subjects/"+moderationTargetUserID+"/unban", strings.NewReader(`{"reason":"appeal accepted"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	req = mux.SetURLVars(req, map[string]string{"userId": moderationTargetUserID})
	rec := httptest.NewRecorder()

	a.moderatorSubjectUnban(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, body = %q", rec.Code, rec.Body.String())
	}
	if store.bannedUserID != moderationTargetUserID || store.banned {
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
