package main

import (
	socialdomain "geoduels/pkg/social"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"geoduels/pkg/auth"
)

type supportTestStore struct {
	testRepositories
	refUserID string
}

func (s *supportTestStore) CreateDonationRef(userID string) (string, error) {
	s.refUserID = userID
	return "don_test", nil
}

func TestCreateSupportDonationRequiresConfiguredTestLinkInTestMode(t *testing.T) {
	api := &api{
		accounts: &supportTestStore{}, sessions: &supportTestStore{}, profiles: &supportTestStore{}, preferenceStore: &supportTestStore{}, badges: &supportTestStore{}, leaderboardStore: &supportTestStore{}, matchStore: &supportTestStore{}, moderation: &supportTestStore{}, admin: &supportTestStore{}, content: &supportTestStore{}, seasons: &supportTestStore{}, gameplayMaps: &supportTestStore{}, runtimeStore: &supportTestStore{}, chatStore: &supportTestStore{}, parties: &supportTestStore{}, social: socialdomain.NewService(&supportTestStore{}),
		appAuthSecret:  []byte("01234567890123456789012345678901"),
		accessTokenTTL: 15 * time.Minute,
		stripeMode:     "test",
	}
	token, err := auth.IssueAppAccessToken(api.appAuthSecret, "user-1", "session-1", api.accessTokenTTL)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/support/donate", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	api.createSupportDonation(rec, req)

	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusServiceUnavailable)
	}
	if !strings.Contains(rec.Body.String(), "stripe test payment link is not configured") {
		t.Fatalf("unexpected body: %q", rec.Body.String())
	}
}

func TestCreateSupportDonationUsesTestPaymentLinkInTestMode(t *testing.T) {
	store := &supportTestStore{}
	api := &api{
		accounts: store, sessions: store, profiles: store, preferenceStore: store, badges: store, leaderboardStore: store, matchStore: store, moderation: store, admin: store, content: store, seasons: store, gameplayMaps: store, runtimeStore: store, chatStore: store, parties: store, social: socialdomain.NewService(store),
		appAuthSecret:         []byte("01234567890123456789012345678901"),
		accessTokenTTL:        15 * time.Minute,
		stripeMode:            "test",
		stripeTestPaymentLink: "https://buy.stripe.com/test_123",
		stripeLivePaymentLink: "https://donate.stripe.com/live_should_not_be_used",
	}
	token, err := auth.IssueAppAccessToken(api.appAuthSecret, "user-1", "session-1", api.accessTokenTTL)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/v1/support/donate", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	api.createSupportDonation(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %q", rec.Code, rec.Body.String())
	}
	if store.refUserID != "user-1" {
		t.Fatalf("ref user = %q", store.refUserID)
	}
	body := rec.Body.String()
	if !strings.Contains(body, "https://buy.stripe.com/test_123") {
		t.Fatalf("expected test payment link, got %q", body)
	}
	if strings.Contains(body, "live_should_not_be_used") {
		t.Fatalf("live payment link leaked into test response: %q", body)
	}
	if !strings.Contains(body, "client_reference_id=don_test") {
		t.Fatalf("missing client reference: %q", body)
	}
}
