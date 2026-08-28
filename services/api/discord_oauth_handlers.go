package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/oauth2"

	"geoduels/pkg/auth"
)

type discordUser struct {
	ID            string `json:"id"`
	Username      string `json:"username"`
	GlobalName    string `json:"global_name"`
	Discriminator string `json:"discriminator"`
	Avatar        string `json:"avatar"`
	Email         string `json:"email"`
	Verified      bool   `json:"verified"`
}

func (a *api) discordOAuthStart(w http.ResponseWriter, r *http.Request) {
	if !a.discordOAuthEnabled() {
		http.Error(w, "discord sign-in unavailable", http.StatusServiceUnavailable)
		return
	}
	var req struct {
		ReturnTo string `json:"returnTo"`
		Intent   string `json:"intent"`
	}
	if err := decodeJSONBody(r, &req); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		http.Error(w, "missing origin", http.StatusBadRequest)
		return
	}
	allowedOrigins := allowedOriginsSet()
	if !allowedOrigins[origin] && !allowedOrigins["*"] {
		http.Error(w, "origin not allowed", http.StatusForbidden)
		return
	}

	intent := normalizeOAuthIntent(req.Intent)
	linkSub, err := a.oauthLinkSubject(r, intent)
	if err != nil {
		http.Error(w, oauthStartError(intent), http.StatusUnauthorized)
		return
	}

	state := oauthStateClaims{
		Intent:   intent,
		Origin:   origin,
		ReturnTo: sanitizeOAuthReturnPath(req.ReturnTo),
		LinkSub:  linkSub,
		Nonce:    randomHex(16),
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(oauthStateTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, state)
	stateToken, err := token.SignedString(a.appAuthSecret)
	if err != nil {
		http.Error(w, "failed to create oauth state", http.StatusInternalServerError)
		return
	}
	authURL := a.discordOAuthConfig(a.discordRedirectURI(r)).AuthCodeURL(
		stateToken,
		oauth2.SetAuthURLParam("prompt", "consent"),
	)
	_ = json.NewEncoder(w).Encode(map[string]string{"authURL": authURL})
}

func (a *api) discordOAuthCallback(w http.ResponseWriter, r *http.Request) {
	if !a.discordOAuthEnabled() {
		http.Error(w, "discord sign-in unavailable", http.StatusServiceUnavailable)
		return
	}
	payload := map[string]any{"ok": false, "error": "Sign-in failed", "provider": "discord"}
	targetOrigin := ""
	defer func() {
		renderOAuthPopup(w, targetOrigin, payload)
	}()

	if errParam := strings.TrimSpace(r.URL.Query().Get("error")); errParam != "" {
		payload["error"] = errParam
		return
	}
	code := strings.TrimSpace(r.URL.Query().Get("code"))
	stateToken := strings.TrimSpace(r.URL.Query().Get("state"))
	if code == "" || stateToken == "" {
		payload["error"] = "missing oauth response"
		return
	}
	state, err := a.parseOAuthState(stateToken)
	if err != nil {
		payload["error"] = "invalid oauth state"
		return
	}
	targetOrigin = state.Origin
	payload["returnTo"] = state.ReturnTo

	profile, err := a.fetchDiscordProfile(r.Context(), code, a.discordRedirectURI(r))
	if err != nil {
		log.Printf("discord oauth callback: profile failed: %v", err)
		payload["error"] = "discord exchange failed"
		return
	}
	displayName := strings.TrimSpace(profile.GlobalName)
	if displayName == "" {
		displayName = strings.TrimSpace(profile.Username)
	}
	if displayName == "" {
		displayName = profile.ID
	}
	email := strings.TrimSpace(profile.Email)
	if email == "" || !profile.Verified {
		email = profile.ID + "@discord.oauth.invalid"
	}
	identity, err := a.resolveOAuthIdentity(
		r,
		state,
		IdentityProviderDiscord,
		profile.ID,
		email,
		displayName,
		discordAvatarURL(profile),
	)
	if err != nil {
		log.Printf("discord oauth callback: persist identity failed: %v", err)
		payload["error"] = oauthUserError(err)
		return
	}
	refreshToken, sessionRecord, err := a.createSession(identity.Sub, r)
	if err != nil {
		log.Printf("discord oauth callback: create session failed for user %s: %v", identity.Sub, err)
		payload["error"] = "issue session failed"
		return
	}
	accessToken, err := auth.IssueAppAccessToken(a.appAuthSecret, identity.Sub, sessionRecord.ID, a.accessTokenTTL)
	if err != nil {
		log.Printf("discord oauth callback: issue access token failed for user %s session %s: %v", identity.Sub, sessionRecord.ID, err)
		payload["error"] = "issue session failed"
		return
	}
	a.setRefreshCookie(w, r, refreshToken)
	payload = a.oauthSessionPayload("discord", accessToken, identity, displayName, state.ReturnTo)
}

func (a *api) discordOAuthEnabled() bool {
	return a.discordClientID != "" && a.discordSecret != ""
}

func (a *api) discordOAuthConfig(redirectURI string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     a.discordClientID,
		ClientSecret: a.discordSecret,
		RedirectURL:  redirectURI,
		Scopes:       []string{"identify", "email"},
		Endpoint: oauth2.Endpoint{
			AuthURL:  "https://discord.com/oauth2/authorize",
			TokenURL: "https://discord.com/api/oauth2/token",
		},
	}
}

func (a *api) discordRedirectURI(r *http.Request) string {
	scheme := strings.TrimSpace(r.Header.Get("X-Forwarded-Proto"))
	if scheme == "" {
		if r.TLS != nil {
			scheme = "https"
		} else {
			scheme = "http"
		}
	}
	host := strings.TrimSpace(r.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = r.Host
	}
	return fmt.Sprintf("%s://%s/v1/auth/discord/callback", scheme, host)
}

func (a *api) fetchDiscordProfile(ctx context.Context, code, redirectURI string) (discordUser, error) {
	token, err := a.discordOAuthConfig(redirectURI).Exchange(ctx, code)
	if err != nil {
		return discordUser{}, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://discord.com/api/users/@me", nil)
	if err != nil {
		return discordUser{}, err
	}
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)
	resp, err := a.httpClient.Do(req)
	if err != nil {
		return discordUser{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return discordUser{}, errors.New("discord profile unavailable")
	}
	var profile discordUser
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		return discordUser{}, err
	}
	if strings.TrimSpace(profile.ID) == "" {
		return discordUser{}, errors.New("discord subject missing")
	}
	return profile, nil
}

func discordAvatarURL(profile discordUser) string {
	if strings.TrimSpace(profile.Avatar) == "" || strings.TrimSpace(profile.ID) == "" {
		return ""
	}
	return "https://cdn.discordapp.com/avatars/" + url.PathEscape(profile.ID) + "/" + url.PathEscape(profile.Avatar) + ".png"
}
