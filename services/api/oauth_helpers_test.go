package main

import (
	"errors"
	"net/http/httptest"
	"testing"

	"geoduels/pkg/persistence"
)

type oauthIntentTestStore struct {
	persistence.Store
	providerExists bool
	providerBanned bool
	signupIPBanned bool
	identity       persistence.Identity
	upsertCalls    int
	linkCalls      int
	lastLinkUserID string
	linkErr        error
}

func TestOAuthSigninAllowsExistingBannedAccount(t *testing.T) {
	store := &oauthIntentTestStore{providerExists: true, providerBanned: true}
	a := &api{store: store}

	identity, err := a.resolveOAuthIdentity(httptest.NewRequest("GET", "/", nil), oauthStateClaims{Intent: oauthIntentSignIn}, "discord", "discord-sub", "player@example.com", "Player", "")
	if err != nil {
		t.Fatalf("resolveOAuthIdentity: %v", err)
	}
	if identity.Sub != "existing-user" || store.upsertCalls != 1 {
		t.Fatalf("identity=%q upsertCalls=%d, want existing-user/1", identity.Sub, store.upsertCalls)
	}
}

func TestOAuthBannedIdentityCannotCreateOrLinkAccount(t *testing.T) {
	for _, state := range []oauthStateClaims{
		{Intent: oauthIntentSignIn},
		{Intent: oauthIntentLink, LinkSub: "user-1"},
	} {
		store := &oauthIntentTestStore{providerBanned: true}
		a := &api{store: store}
		if _, err := a.resolveOAuthIdentity(httptest.NewRequest("GET", "/", nil), state, "discord", "discord-sub", "player@example.com", "Player", ""); err == nil || err.Error() != "provider identity banned" {
			t.Fatalf("intent %q err=%v, want provider identity banned", state.Intent, err)
		}
	}
}

func (s *oauthIntentTestStore) ProviderIdentityExists(provider, providerUserID string) (bool, error) {
	return s.providerExists, nil
}

func (s *oauthIntentTestStore) IsSignupIPBanned(ipAddress string) (bool, error) {
	return s.signupIPBanned, nil
}

func (s *oauthIntentTestStore) UpsertProviderIdentity(provider, providerUserID, email, providerName, avatarURL, linkUserID string) (persistence.Identity, error) {
	s.upsertCalls++
	if s.providerBanned && !s.providerExists {
		return persistence.Identity{}, errors.New("provider identity banned")
	}
	return persistence.Identity{Sub: "existing-user", AccountType: "registered"}, nil
}

func (s *oauthIntentTestStore) LinkProviderIdentity(provider, providerUserID, email, providerName, avatarURL, linkUserID string) (persistence.Identity, error) {
	s.linkCalls++
	s.lastLinkUserID = linkUserID
	if s.providerBanned {
		return persistence.Identity{}, errors.New("provider identity banned")
	}
	if s.linkErr != nil {
		return persistence.Identity{}, s.linkErr
	}
	return persistence.Identity{Sub: linkUserID, AccountType: "registered"}, nil
}

func (s *oauthIntentTestStore) GetIdentity(sub string) (persistence.Identity, error) {
	if s.identity.Sub == "" {
		return persistence.Identity{}, errors.New("identity not found")
	}
	return s.identity, nil
}

func TestOAuthSigninIgnoresLinkSubject(t *testing.T) {
	store := &oauthIntentTestStore{providerExists: true}
	a := &api{store: store}

	identity, err := a.resolveOAuthIdentity(
		httptest.NewRequest("GET", "/", nil),
		oauthStateClaims{Intent: oauthIntentSignIn, LinkSub: "guest-1"},
		"google",
		"google-sub",
		"player@example.com",
		"Player",
		"",
	)
	if err != nil {
		t.Fatalf("resolveOAuthIdentity: %v", err)
	}
	if identity.Sub != "existing-user" {
		t.Fatalf("identity sub = %q, want existing-user", identity.Sub)
	}
	if store.upsertCalls != 1 || store.linkCalls != 0 {
		t.Fatalf("upsert/link calls = %d/%d, want 1/0", store.upsertCalls, store.linkCalls)
	}
}

func TestOAuthLinkRequiresExplicitIntent(t *testing.T) {
	store := &oauthIntentTestStore{}
	a := &api{store: store}

	identity, err := a.resolveOAuthIdentity(
		httptest.NewRequest("GET", "/", nil),
		oauthStateClaims{Intent: oauthIntentLink, LinkSub: "user-1"},
		"discord",
		"discord-sub",
		"player@example.com",
		"Player",
		"",
	)
	if err != nil {
		t.Fatalf("resolveOAuthIdentity: %v", err)
	}
	if identity.Sub != "user-1" || store.lastLinkUserID != "user-1" {
		t.Fatalf("linked identity = %q via %q, want user-1", identity.Sub, store.lastLinkUserID)
	}
	if store.upsertCalls != 0 || store.linkCalls != 1 {
		t.Fatalf("upsert/link calls = %d/%d, want 0/1", store.upsertCalls, store.linkCalls)
	}
}

func TestOAuthGuestUpgradeRejectsExistingProviderAccount(t *testing.T) {
	store := &oauthIntentTestStore{
		identity: persistence.Identity{Sub: "guest-1", AccountType: "guest"},
		linkErr:  errors.New("provider identity already linked"),
	}
	a := &api{store: store}

	_, err := a.resolveOAuthIdentity(
		httptest.NewRequest("GET", "/", nil),
		oauthStateClaims{Intent: oauthIntentUpgradeGuest, LinkSub: "guest-1"},
		"google",
		"google-sub",
		"player@example.com",
		"Player",
		"",
	)
	if err == nil || err.Error() != "provider account already exists" {
		t.Fatalf("err = %v, want provider account already exists", err)
	}
	if got := oauthUserError(err); got != "This sign-in method already has a GeoDuels account. Sign out and continue with that provider." {
		t.Fatalf("oauthUserError = %q", got)
	}
}

func TestOAuthGuestUpgradeRequiresGuestAccount(t *testing.T) {
	store := &oauthIntentTestStore{
		identity: persistence.Identity{Sub: "user-1", AccountType: "registered"},
	}
	a := &api{store: store}

	_, err := a.resolveOAuthIdentity(
		httptest.NewRequest("GET", "/", nil),
		oauthStateClaims{Intent: oauthIntentUpgradeGuest, LinkSub: "user-1"},
		"google",
		"google-sub",
		"player@example.com",
		"Player",
		"",
	)
	if err == nil || err.Error() != "guest upgrade requires guest account" {
		t.Fatalf("err = %v, want guest upgrade requires guest account", err)
	}
	if store.linkCalls != 0 {
		t.Fatalf("link calls = %d, want 0", store.linkCalls)
	}
}

func TestOAuthUserErrorExplainsAmbiguousVerifiedEmail(t *testing.T) {
	got := oauthUserError(persistence.ErrOAuthEmailConflict)
	want := "This verified email is linked to multiple GeoDuels accounts. Contact support to recover the account."
	if got != want {
		t.Fatalf("oauthUserError() = %q, want %q", got, want)
	}
}
