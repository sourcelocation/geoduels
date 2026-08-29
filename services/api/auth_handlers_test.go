package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"geoduels/pkg/auth"
	"geoduels/pkg/contracts"
	"geoduels/pkg/persistence"
)

type guestAuthTestStore struct {
	testRepositories
	createdGuests int
	identity      Identity
	sessions      map[string]RefreshTokenRecord
}

type nicknameAuthTestStore struct {
	testRepositories
	identity       Identity
	setErr         error
	setName        string
	suggestedName  string
	suggestionFrom string
}

func (s *nicknameAuthTestStore) GetIdentity(sub string) (Identity, error) {
	return s.identity, nil
}

func (s *nicknameAuthTestStore) SetNickname(sub, displayName string) error {
	if s.setErr != nil {
		return s.setErr
	}
	s.setName = displayName
	s.identity.DisplayName = displayName
	s.identity.NicknameRequired = false
	return nil
}

func (s *nicknameAuthTestStore) SyncLoginBadges(userID string) error {
	return nil
}

func (s *nicknameAuthTestStore) SuggestNickname(sub, displayName string) (string, error) {
	s.suggestionFrom = displayName
	return s.suggestedName, nil
}

func (s *guestAuthTestStore) CreateGuestIdentity() (Identity, error) {
	s.createdGuests++
	s.identity = Identity{
		Sub:              "guest-1",
		DisplayName:      "Guest",
		NicknameRequired: false,
		AccountType:      "guest",
	}
	return s.identity, nil
}

func (s *guestAuthTestStore) IsSignupIPBanned(ipAddress string) (bool, error) {
	return false, nil
}

func (s *guestAuthTestStore) CreateAuthSession(userID, refreshTokenHash string, expiresAt time.Time, params AuthSessionParams) (RefreshTokenRecord, error) {
	if s.sessions == nil {
		s.sessions = map[string]RefreshTokenRecord{}
	}
	rec := RefreshTokenRecord{
		ID:               "session-1",
		UserID:           userID,
		RefreshTokenHash: refreshTokenHash,
		ExpiresAt:        expiresAt,
		CreatedAt:        time.Now(),
		LastUsedAt:       time.Now(),
	}
	s.sessions[refreshTokenHash] = rec
	return rec, nil
}

func (s *guestAuthTestStore) GetAuthSessionByRefreshToken(hash string) (RefreshTokenRecord, bool, error) {
	rec, ok := s.sessions[hash]
	return rec, ok, nil
}

func (s *guestAuthTestStore) RotateAuthSession(sessionID, currentHash, nextHash string, expiresAt time.Time, usedAt time.Time) (RefreshTokenRecord, bool, error) {
	rec, ok := s.sessions[currentHash]
	if !ok {
		return RefreshTokenRecord{}, false, nil
	}
	delete(s.sessions, currentHash)
	rec.RefreshTokenHash = nextHash
	rec.ExpiresAt = expiresAt
	rec.LastUsedAt = usedAt
	s.sessions[nextHash] = rec
	return rec, true, nil
}

func (s *guestAuthTestStore) GetIdentity(sub string) (Identity, error) {
	return s.identity, nil
}

func (s *guestAuthTestStore) GetProfile(userID string) (persistence.Profile, error) {
	return persistence.Profile{
		UserID: userID, DisplayName: s.identity.DisplayName,
		IsGuest: s.identity.AccountType == "guest", IsAdmin: s.identity.IsAdmin,
		IsModerator: s.identity.IsModerator, IsBanned: s.identity.IsBanned,
	}, nil
}

func (s *guestAuthTestStore) SyncLoginBadges(userID string) error {
	return nil
}

func TestGuestLoginReusesExistingRefreshSession(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	store := &guestAuthTestStore{}
	a := &api{
		accounts: store, sessions: store, profiles: store, preferenceStore: store, badges: store, leaderboardStore: store, matchStore: store, moderation: store, admin: store, content: store, seasons: store, gameplayMaps: store, runtimeStore: store, chatStore: store, parties: store, social: store,
		redis:                 rdb,
		appAuthSecret:         []byte("01234567890123456789012345678901"),
		accessTokenTTL:        15 * time.Minute,
		refreshTokenTTL:       30 * 24 * time.Hour,
		refreshCookieName:     "geoduels_refresh",
		refreshCookieSameSite: http.SameSiteLaxMode,
		guestSignupIPLimit:    1,
		guestSignupIPWindow:   time.Minute,
	}

	firstReq := httptest.NewRequest(http.MethodPost, "/v1/auth/guest", nil)
	firstRec := httptest.NewRecorder()
	a.guestLogin(firstRec, firstReq)
	if firstRec.Code != http.StatusOK {
		t.Fatalf("first guest login status = %d", firstRec.Code)
	}
	if store.createdGuests != 1 {
		t.Fatalf("created guests after first login = %d", store.createdGuests)
	}
	cookie := firstRec.Result().Cookies()[0]
	if cookie.Value == "" || auth.RefreshTokenHash(cookie.Value) == "" {
		t.Fatal("expected refresh cookie")
	}

	secondReq := httptest.NewRequest(http.MethodPost, "/v1/auth/guest", nil)
	secondReq.AddCookie(cookie)
	secondRec := httptest.NewRecorder()
	a.guestLogin(secondRec, secondReq)
	if secondRec.Code != http.StatusOK {
		t.Fatalf("second guest login status = %d", secondRec.Code)
	}
	if store.createdGuests != 1 {
		t.Fatalf("guest login should reuse cookie session, created guests = %d", store.createdGuests)
	}
}

func TestAnonymousBootstrapReturnsAnonymousPayloadWithoutLogging(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	a := &api{
		accounts: &guestAuthTestStore{}, sessions: &guestAuthTestStore{}, profiles: &guestAuthTestStore{}, preferenceStore: &guestAuthTestStore{}, badges: &guestAuthTestStore{}, leaderboardStore: &guestAuthTestStore{}, matchStore: &guestAuthTestStore{}, moderation: &guestAuthTestStore{}, admin: &guestAuthTestStore{}, content: &guestAuthTestStore{}, seasons: &guestAuthTestStore{}, gameplayMaps: &guestAuthTestStore{}, runtimeStore: &guestAuthTestStore{}, chatStore: &guestAuthTestStore{}, parties: &guestAuthTestStore{}, social: &guestAuthTestStore{},
		refreshCookieName:     "geoduels_refresh",
		refreshCookieSameSite: http.SameSiteLaxMode,
	}
	req := httptest.NewRequest(http.MethodGet, "/v1/bootstrap", nil)
	rec := httptest.NewRecorder()

	a.bootstrap(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("bootstrap status = %d, want %d", rec.Code, http.StatusOK)
	}
	if strings.Contains(buf.String(), "auth session bootstrap failed") {
		t.Fatalf("anonymous restore logged a failure: %s", buf.String())
	}
}

func TestSessionFailureDoesNotClearRefreshCookie(t *testing.T) {
	var buf bytes.Buffer
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	a := &api{
		accounts: &guestAuthTestStore{}, sessions: &guestAuthTestStore{}, profiles: &guestAuthTestStore{}, preferenceStore: &guestAuthTestStore{}, badges: &guestAuthTestStore{}, leaderboardStore: &guestAuthTestStore{}, matchStore: &guestAuthTestStore{}, moderation: &guestAuthTestStore{}, admin: &guestAuthTestStore{}, content: &guestAuthTestStore{}, seasons: &guestAuthTestStore{}, gameplayMaps: &guestAuthTestStore{}, runtimeStore: &guestAuthTestStore{}, chatStore: &guestAuthTestStore{}, parties: &guestAuthTestStore{}, social: &guestAuthTestStore{},
		refreshCookieName:     "geoduels_refresh",
		refreshCookieSameSite: http.SameSiteLaxMode,
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/bootstrap", nil)
	req.AddCookie(&http.Cookie{
		Name:  "geoduels_refresh",
		Value: "stale-token",
	})
	rec := httptest.NewRecorder()

	a.bootstrap(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("bootstrap status = %d, want %d", rec.Code, http.StatusOK)
	}
	if cookies := rec.Result().Cookies(); len(cookies) != 0 {
		t.Fatalf("session failure must not overwrite a possibly newer cookie, got %v", cookies)
	}
	if strings.Contains(buf.String(), "auth session bootstrap failed") {
		t.Fatal("invalid refresh session is an expected unauthenticated state")
	}
}

func TestSessionUsesValidRefreshCookieWhenStaleDuplicateComesFirst(t *testing.T) {
	store := &guestAuthTestStore{
		identity: Identity{
			Sub:         "user-1",
			DisplayName: "Player",
			AccountType: "registered",
		},
		sessions: map[string]RefreshTokenRecord{},
	}
	validToken := "valid-token"
	validHash := auth.RefreshTokenHash(validToken)
	store.sessions[validHash] = RefreshTokenRecord{
		ID:               "session-1",
		UserID:           "user-1",
		RefreshTokenHash: validHash,
		ExpiresAt:        time.Now().Add(time.Hour),
		CreatedAt:        time.Now(),
		LastUsedAt:       time.Now(),
	}
	a := &api{
		accounts: store, sessions: store, profiles: store, preferenceStore: store, badges: store, leaderboardStore: store, matchStore: store, moderation: store, admin: store, content: store, seasons: store, gameplayMaps: store, runtimeStore: store, chatStore: store, parties: store, social: store,
		appAuthSecret:         []byte("01234567890123456789012345678901"),
		accessTokenTTL:        15 * time.Minute,
		refreshTokenTTL:       30 * 24 * time.Hour,
		refreshCookieName:     "geoduels_refresh",
		refreshCookieSameSite: http.SameSiteLaxMode,
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/bootstrap", nil)
	req.Header.Set("Cookie", "geoduels_refresh=stale-token; geoduels_refresh="+validToken)
	rec := httptest.NewRecorder()

	a.bootstrap(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("session status = %d, want %d; body = %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if cookies := rec.Result().Cookies(); len(cookies) != 0 {
		t.Fatalf("session bootstrap must not rotate the refresh cookie, got %v", cookies)
	}
	if _, ok := store.sessions[validHash]; !ok {
		t.Fatal("session bootstrap unexpectedly rotated the stored refresh token")
	}
}

func TestSessionBootstrapIsIdempotent(t *testing.T) {
	store := &guestAuthTestStore{
		identity: Identity{
			Sub:         "user-1",
			DisplayName: "Player",
			AccountType: "registered",
		},
		sessions: map[string]RefreshTokenRecord{},
	}
	refreshToken := "valid-token"
	refreshHash := auth.RefreshTokenHash(refreshToken)
	store.sessions[refreshHash] = RefreshTokenRecord{
		ID:               "session-1",
		UserID:           "user-1",
		RefreshTokenHash: refreshHash,
		ExpiresAt:        time.Now().Add(time.Hour),
		CreatedAt:        time.Now(),
		LastUsedAt:       time.Now(),
	}
	a := &api{
		accounts: store, sessions: store, profiles: store, preferenceStore: store, badges: store, leaderboardStore: store, matchStore: store, moderation: store, admin: store, content: store, seasons: store, gameplayMaps: store, runtimeStore: store, chatStore: store, parties: store, social: store,
		appAuthSecret:         []byte("01234567890123456789012345678901"),
		accessTokenTTL:        15 * time.Minute,
		refreshTokenTTL:       30 * 24 * time.Hour,
		refreshCookieName:     "geoduels_refresh",
		refreshCookieSameSite: http.SameSiteLaxMode,
	}

	for attempt := 1; attempt <= 2; attempt++ {
		req := httptest.NewRequest(http.MethodGet, "/v1/bootstrap", nil)
		req.AddCookie(&http.Cookie{Name: "geoduels_refresh", Value: refreshToken})
		rec := httptest.NewRecorder()

		a.bootstrap(rec, req)

		if rec.Code != http.StatusOK {
			t.Fatalf("session bootstrap %d status = %d, want %d; body = %s", attempt, rec.Code, http.StatusOK, rec.Body.String())
		}
		if cookies := rec.Result().Cookies(); len(cookies) != 0 {
			t.Fatalf("session bootstrap %d unexpectedly changed cookies: %v", attempt, cookies)
		}
	}
	if _, ok := store.sessions[refreshHash]; !ok {
		t.Fatal("repeated bootstrap unexpectedly rotated the stored refresh token")
	}
}

func TestGuestLoginIgnoresNicknamePayload(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	store := &guestAuthTestStore{}
	a := &api{
		accounts: store, sessions: store, profiles: store, preferenceStore: store, badges: store, leaderboardStore: store, matchStore: store, moderation: store, admin: store, content: store, seasons: store, gameplayMaps: store, runtimeStore: store, chatStore: store, parties: store, social: store,
		redis:                 rdb,
		appAuthSecret:         []byte("01234567890123456789012345678901"),
		accessTokenTTL:        15 * time.Minute,
		refreshTokenTTL:       30 * 24 * time.Hour,
		refreshCookieName:     "geoduels_refresh",
		refreshCookieSameSite: http.SameSiteLaxMode,
		guestSignupIPLimit:    1,
		guestSignupIPWindow:   time.Minute,
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/guest", strings.NewReader(`{"nickname":"Custom"}`))
	rec := httptest.NewRecorder()
	a.guestLogin(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("guest login status = %d", rec.Code)
	}
	if store.identity.DisplayName != "Guest" {
		t.Fatalf("guest display name = %q, want Guest", store.identity.DisplayName)
	}
}

func TestGuestLoginRequiresTurnstileWhenEnabled(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	store := &guestAuthTestStore{}
	a := &api{
		accounts: store, sessions: store, profiles: store, preferenceStore: store, badges: store, leaderboardStore: store, matchStore: store, moderation: store, admin: store, content: store, seasons: store, gameplayMaps: store, runtimeStore: store, chatStore: store, parties: store, social: store,
		redis:                  rdb,
		appAuthSecret:          []byte("01234567890123456789012345678901"),
		accessTokenTTL:         15 * time.Minute,
		refreshTokenTTL:        30 * 24 * time.Hour,
		refreshCookieName:      "geoduels_refresh",
		refreshCookieSameSite:  http.SameSiteLaxMode,
		guestSignupIPLimit:     10,
		guestSignupIPWindow:    time.Minute,
		guestTurnstileRequired: true,
		turnstileSecret:        "secret",
		httpClient:             http.DefaultClient,
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/guest", nil)
	rec := httptest.NewRecorder()
	a.guestLogin(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("guest login status = %d", rec.Code)
	}
	if store.createdGuests != 0 {
		t.Fatalf("guest should not be created, created guests = %d", store.createdGuests)
	}
}

func TestGuestLoginValidatesTurnstileToken(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	var sawRemoteIP string
	verifyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		values, err := url.ParseQuery(string(body))
		if err != nil {
			t.Fatalf("parse verify body: %v", err)
		}
		if values.Get("secret") != "secret" {
			t.Fatalf("secret = %q", values.Get("secret"))
		}
		if values.Get("response") != "token-123" {
			t.Fatalf("response = %q", values.Get("response"))
		}
		sawRemoteIP = values.Get("remoteip")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"success":  true,
			"hostname": "play.example.com",
			"action":   guestTurnstileAction,
		})
	}))
	defer verifyServer.Close()

	store := &guestAuthTestStore{}
	a := &api{
		accounts: store, sessions: store, profiles: store, preferenceStore: store, badges: store, leaderboardStore: store, matchStore: store, moderation: store, admin: store, content: store, seasons: store, gameplayMaps: store, runtimeStore: store, chatStore: store, parties: store, social: store,
		redis:                  rdb,
		appAuthSecret:          []byte("01234567890123456789012345678901"),
		accessTokenTTL:         15 * time.Minute,
		refreshTokenTTL:        30 * 24 * time.Hour,
		refreshCookieName:      "geoduels_refresh",
		refreshCookieSameSite:  http.SameSiteLaxMode,
		guestSignupIPLimit:     10,
		guestSignupIPWindow:    time.Minute,
		guestTurnstileRequired: true,
		turnstileSecret:        "secret",
		turnstileVerifyURL:     verifyServer.URL,
		turnstileHostname:      "play.example.com",
		httpClient:             verifyServer.Client(),
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/guest", strings.NewReader(`{"turnstileToken":"token-123"}`))
	req.RemoteAddr = "203.0.113.44:12345"
	rec := httptest.NewRecorder()
	a.guestLogin(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("guest login status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if store.createdGuests != 1 {
		t.Fatalf("created guests = %d", store.createdGuests)
	}
	if sawRemoteIP != "203.0.113.44" {
		t.Fatalf("remoteip = %q", sawRemoteIP)
	}
}

func TestSessionUserIncludesProfileFields(t *testing.T) {
	user := sessionUser(Identity{
		Sub:          "user-1",
		Email:        "player@example.com",
		DisplayName:  "Player",
		ProviderName: "discord-player",
		AvatarURL:    "https://cdn.example/avatar.png",
		AccountType:  "registered",
		IsAdmin:      true,
	})

	if user.ID != "user-1" {
		t.Fatalf("user id = %q, want user-1", user.ID)
	}
	if user.Email != "player@example.com" {
		t.Fatalf("email = %q, want player@example.com", user.Email)
	}
	if user.DisplayName != "Player" {
		t.Fatalf("display name = %q, want Player", user.DisplayName)
	}
	if user.AvatarURL != "https://cdn.example/avatar.png" {
		t.Fatalf("avatar url = %q, want profile avatar", user.AvatarURL)
	}
	if user.IsGuest {
		t.Fatal("registered user should not be marked as guest")
	}
	if !user.IsAdmin {
		t.Fatal("expected admin flag to be preserved")
	}
}

func TestUpdateNicknameClaimsRequiredNickname(t *testing.T) {
	secret := []byte("01234567890123456789012345678901")
	store := &nicknameAuthTestStore{
		identity: Identity{
			Sub:              "user-1",
			DisplayName:      "Old Name",
			NicknameRequired: true,
			AccountType:      "registered",
		},
	}
	a := &api{
		accounts: store, sessions: store, profiles: store, preferenceStore: store, badges: store, leaderboardStore: store, matchStore: store, moderation: store, admin: store, content: store, seasons: store, gameplayMaps: store, runtimeStore: store, chatStore: store, parties: store, social: store,
		appAuthSecret:  secret,
		accessTokenTTL: 15 * time.Minute,
	}
	token, err := auth.IssueAppAccessToken(secret, "user-1", "session-1", 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPut, "/v1/me/nickname", strings.NewReader(`{"nickname":"Player.One"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	a.updateNickname(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("update nickname status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if store.setName != "Player.One" {
		t.Fatalf("stored nickname = %q", store.setName)
	}
	var payload contracts.AuthSessionPayload
	if err := json.NewDecoder(rec.Body).Decode(&payload); err != nil {
		t.Fatal(err)
	}
	if payload.NicknameRequired {
		t.Fatal("nickname should be claimed after successful update")
	}
	if !payload.CanPlay {
		t.Fatal("claimed registered user should be playable")
	}
}

func TestUpdateNicknameReturnsConflictWhenTaken(t *testing.T) {
	secret := []byte("01234567890123456789012345678901")
	store := &nicknameAuthTestStore{
		identity: Identity{
			Sub:              "user-1",
			NicknameRequired: true,
			AccountType:      "registered",
		},
		setErr: ErrNicknameTaken,
	}
	a := &api{accounts: store, sessions: store, profiles: store, preferenceStore: store, badges: store, leaderboardStore: store, matchStore: store, moderation: store, admin: store, content: store, seasons: store, gameplayMaps: store, runtimeStore: store, chatStore: store, parties: store, social: store, appAuthSecret: secret}
	token, err := auth.IssueAppAccessToken(secret, "user-1", "session-1", 15*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPut, "/v1/me/nickname", strings.NewReader(`{"nickname":"Taken"}`))
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()

	a.updateNickname(rec, req)

	if rec.Code != http.StatusConflict {
		t.Fatalf("update nickname status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !errors.Is(store.setErr, ErrNicknameTaken) {
		t.Fatal("expected nickname conflict")
	}
}

func TestSuggestedNicknameUsesAvailableStoreSuggestion(t *testing.T) {
	store := &nicknameAuthTestStore{
		identity: Identity{
			Sub:              "user-1",
			ProviderName:     "Player Name",
			NicknameRequired: true,
			AccountType:      "registered",
		},
		suggestedName: "Player.Name4821",
	}
	a := &api{accounts: store, sessions: store, profiles: store, preferenceStore: store, badges: store, leaderboardStore: store, matchStore: store, moderation: store, admin: store, content: store, seasons: store, gameplayMaps: store, runtimeStore: store, chatStore: store, parties: store, social: store}

	got, err := a.suggestedNickname(store.identity, "")

	if err != nil {
		t.Fatal(err)
	}
	if got != "Player.Name4821" {
		t.Fatalf("suggested nickname = %q", got)
	}
	if store.suggestionFrom != "Player Name" {
		t.Fatalf("suggestion source = %q", store.suggestionFrom)
	}
}

func TestGuestLoginRateLimitsNewGuestsByIP(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	store := &guestAuthTestStore{}
	a := &api{
		accounts: store, sessions: store, profiles: store, preferenceStore: store, badges: store, leaderboardStore: store, matchStore: store, moderation: store, admin: store, content: store, seasons: store, gameplayMaps: store, runtimeStore: store, chatStore: store, parties: store, social: store,
		redis:                 rdb,
		appAuthSecret:         []byte("01234567890123456789012345678901"),
		accessTokenTTL:        15 * time.Minute,
		refreshTokenTTL:       30 * 24 * time.Hour,
		refreshCookieName:     "geoduels_refresh",
		refreshCookieSameSite: http.SameSiteLaxMode,
		guestSignupIPLimit:    2,
		guestSignupIPWindow:   time.Minute,
	}

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/auth/guest", nil)
		req.RemoteAddr = "203.0.113.10:12345"
		rec := httptest.NewRecorder()
		a.guestLogin(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("guest login %d status = %d", i+1, rec.Code)
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/guest", nil)
	req.RemoteAddr = "203.0.113.10:12345"
	rec := httptest.NewRecorder()
	a.guestLogin(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("third guest login status = %d", rec.Code)
	}
	if retryAfter := rec.Header().Get("Retry-After"); retryAfter == "" {
		t.Fatal("expected Retry-After header")
	}
	if store.createdGuests != 2 {
		t.Fatalf("rate-limited request should not create a guest, created guests = %d", store.createdGuests)
	}
}

func TestGuestLoginDailyRateLimitsNewGuestsByIP(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	store := &guestAuthTestStore{}
	a := &api{
		accounts: store, sessions: store, profiles: store, preferenceStore: store, badges: store, leaderboardStore: store, matchStore: store, moderation: store, admin: store, content: store, seasons: store, gameplayMaps: store, runtimeStore: store, chatStore: store, parties: store, social: store,
		redis:                  rdb,
		appAuthSecret:          []byte("01234567890123456789012345678901"),
		accessTokenTTL:         15 * time.Minute,
		refreshTokenTTL:        30 * 24 * time.Hour,
		refreshCookieName:      "geoduels_refresh",
		refreshCookieSameSite:  http.SameSiteLaxMode,
		guestSignupIPLimit:     100,
		guestSignupIPWindow:    time.Minute,
		guestSignupDailyLimit:  2,
		guestSignupDailyWindow: 24 * time.Hour,
	}

	for i := 0; i < 2; i++ {
		req := httptest.NewRequest(http.MethodPost, "/v1/auth/guest", nil)
		req.RemoteAddr = "203.0.113.20:12345"
		rec := httptest.NewRecorder()
		a.guestLogin(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("guest login %d status = %d", i+1, rec.Code)
		}
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/auth/guest", nil)
	req.RemoteAddr = "203.0.113.20:12345"
	rec := httptest.NewRecorder()
	a.guestLogin(rec, req)
	if rec.Code != http.StatusTooManyRequests {
		t.Fatalf("third daily guest login status = %d", rec.Code)
	}
	if retryAfter := rec.Header().Get("Retry-After"); retryAfter == "" {
		t.Fatal("expected Retry-After header")
	}
	if store.createdGuests != 2 {
		t.Fatalf("daily rate-limited request should not create a guest, created guests = %d", store.createdGuests)
	}
}
