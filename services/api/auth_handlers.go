package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/mux"

	"geoduels/pkg/auth"
	"geoduels/pkg/authsession"
	"geoduels/pkg/contentfilter"
	"geoduels/pkg/contracts"
)

func (a *api) sessionService() *authsession.Service {
	if a.authSessionService != nil {
		return a.authSessionService
	}
	return authsession.NewService(a.sessions)
}

var errMissingRefreshToken = errors.New("missing refresh token")
var errUnavailableRefreshSession = errors.New("session unavailable")

func (a *api) guestLogin(w http.ResponseWriter, r *http.Request) {
	if payload, nextRefreshToken, err := a.rotateSessionFromCookie(r); err == nil {
		a.setRefreshCookie(w, r, nextRefreshToken)
		_ = json.NewEncoder(w).Encode(payload)
		return
	}
	var req struct {
		TurnstileToken string `json:"turnstileToken"`
	}
	if err := decodeJSONBody(r, &req); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	if banned, err := a.moderation.IsSignupIPBanned(a.clientIP(r)); err != nil {
		http.Error(w, "signup unavailable (101)", http.StatusInternalServerError)
		return
	} else if banned {
		http.Error(w, "signup unavailable (102)", http.StatusForbidden)
		return
	}
	if ok, retryAfter, err := a.checkGuestSignupRateLimit(r); err != nil {
		http.Error(w, "signup unavailable (103)", http.StatusInternalServerError)
		return
	} else if !ok {
		writeRateLimited(w, retryAfter)
		return
	}
	if err := a.verifyGuestTurnstile(r.Context(), req.TurnstileToken, a.clientIP(r)); err != nil {
		if errors.Is(err, errTurnstileRejected) {
			http.Error(w, "verification failed", http.StatusForbidden)
			return
		}
		http.Error(w, "verification unavailable", http.StatusServiceUnavailable)
		return
	}
	identity, err := a.accounts.CreateGuestIdentity()
	if err != nil {
		http.Error(w, "persist guest failed", http.StatusInternalServerError)
		return
	}
	if err := a.writeSessionResponse(w, r, identity); err != nil {
		http.Error(w, "issue session failed", http.StatusInternalServerError)
	}
}

func (a *api) refresh(w http.ResponseWriter, r *http.Request) {
	if err := a.writeRotatedSessionResponse(w, r); err != nil {
		a.clearRefreshCookie(w, r)
		http.Error(w, "invalid session", http.StatusUnauthorized)
	}
}

func (a *api) updateNickname(w http.ResponseWriter, r *http.Request) {
	claims, identity, err := a.authenticatedAccount(r)
	if err != nil {
		http.Error(w, "identity not found", http.StatusUnauthorized)
		return
	}
	if identity.AccountType == "guest" {
		http.Error(w, "guest nicknames cannot be changed", http.StatusForbidden)
		return
	}
	var req struct {
		Nickname string `json:"nickname"`
	}
	if err := decodeJSONBody(r, &req); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	nick, err := validatedNickname(req.Nickname)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	if err := a.accounts.SetNickname(claims.Sub, nick); err != nil {
		if errors.Is(err, ErrNicknameTaken) {
			http.Error(w, "nickname already taken", http.StatusConflict)
			return
		}
		http.Error(w, "failed to update nickname", http.StatusInternalServerError)
		return
	}
	updated, err := a.accounts.GetIdentity(claims.Sub)
	if err != nil {
		http.Error(w, "identity not found", http.StatusUnauthorized)
		return
	}
	payload, err := a.issueAuthSessionPayload(updated, claims.SessionID)
	if err != nil {
		http.Error(w, "issue session failed", http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(payload)
}

func (a *api) unlinkAuthProvider(w http.ResponseWriter, r *http.Request) {
	claims, err := a.authenticatedClaims(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	provider := strings.ToLower(strings.TrimSpace(mux.Vars(r)["provider"]))
	if provider != IdentityProviderGoogle && provider != IdentityProviderDiscord {
		http.Error(w, "unknown provider", http.StatusBadRequest)
		return
	}
	identity, err := a.accounts.UnlinkProviderIdentity(claims.Sub, provider)
	if err != nil {
		msg := strings.ToLower(strings.TrimSpace(err.Error()))
		switch {
		case strings.Contains(msg, "last sign-in method"):
			http.Error(w, "cannot unlink the last sign-in method", http.StatusConflict)
		case strings.Contains(msg, "not linked"):
			http.Error(w, "provider is not linked", http.StatusNotFound)
		default:
			http.Error(w, "failed to unlink provider", http.StatusInternalServerError)
		}
		return
	}
	payload, err := a.issueAuthSessionPayload(identity, claims.SessionID)
	if err != nil {
		http.Error(w, "issue session failed", http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(payload)
}

func (a *api) deleteAccount(w http.ResponseWriter, r *http.Request) {
	claims, err := a.authenticatedClaims(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	var req struct {
		Confirm string `json:"confirm"`
	}
	if err := decodeJSONBody(r, &req); err != nil {
		http.Error(w, "invalid payload", http.StatusBadRequest)
		return
	}
	if strings.TrimSpace(req.Confirm) != "DELETE" {
		http.Error(w, "confirmation required", http.StatusBadRequest)
		return
	}
	if err := a.accounts.DeleteAccount(claims.Sub); err != nil {
		http.Error(w, "failed to delete account", http.StatusInternalServerError)
		return
	}
	a.clearRefreshCookie(w, r)
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) logout(w http.ResponseWriter, r *http.Request) {
	sessionID, userID := a.sessionIdentity(r)
	if sessionID != "" {
		_ = a.sessionService().Revoke(r.Context(), sessionID)
	} else if userID != "" {
		_ = a.sessionService().RevokeAll(r.Context(), userID)
	}
	a.clearRefreshCookie(w, r)
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) logoutAll(w http.ResponseWriter, r *http.Request) {
	claims, err := a.authenticatedClaims(r)
	if err != nil {
		http.Error(w, "unauthorized", http.StatusUnauthorized)
		return
	}
	if err := a.sessionService().RevokeAll(r.Context(), claims.Sub); err != nil {
		http.Error(w, "logout failed", http.StatusInternalServerError)
		return
	}
	a.clearRefreshCookie(w, r)
	w.WriteHeader(http.StatusNoContent)
}

func validatedNickname(raw string) (string, error) {
	return contentfilter.ValidateNickname(raw)
}

func (a *api) authenticatedClaims(r *http.Request) (auth.AppClaims, error) {
	if principal, ok := r.Context().Value(requestPrincipalKey{}).(requestPrincipal); ok {
		return principal.claims, nil
	}
	authz := r.Header.Get("Authorization")
	if !strings.HasPrefix(authz, "Bearer ") {
		return auth.AppClaims{}, errors.New("missing bearer token")
	}
	tok := strings.TrimSpace(strings.TrimPrefix(authz, "Bearer "))
	claims, err := auth.ValidateAppAccessToken(a.appAuthSecret, tok)
	if err != nil {
		return auth.AppClaims{}, err
	}
	return claims, nil
}

func (a *api) sessionIdentity(r *http.Request) (string, string) {
	if claims, err := a.authenticatedClaims(r); err == nil {
		return claims.SessionID, claims.Sub
	}
	for _, refreshToken := range a.readRefreshCookies(r) {
		rec, ok, err := a.sessionService().Get(r.Context(), auth.RefreshTokenHash(refreshToken))
		if err == nil && ok {
			return rec.ID, rec.UserID
		}
	}
	return "", ""
}

func (a *api) writeSessionResponse(w http.ResponseWriter, r *http.Request, identity Identity) error {
	refreshToken, sessionRecord, err := a.createSession(identity.Sub, r)
	if err != nil {
		return err
	}
	payload, err := a.issueAuthSessionPayload(identity, sessionRecord.ID)
	if err != nil {
		return err
	}
	a.setRefreshCookie(w, r, refreshToken)
	return json.NewEncoder(w).Encode(payload)
}

func (a *api) writeRotatedSessionResponse(w http.ResponseWriter, r *http.Request) error {
	payload, nextRefreshToken, err := a.rotateSessionFromCookie(r)
	if err != nil {
		return err
	}
	a.setRefreshCookie(w, r, nextRefreshToken)
	return json.NewEncoder(w).Encode(payload)
}

func (a *api) authSessionFromCookies(r *http.Request) (RefreshTokenRecord, error) {
	refreshTokens := a.readRefreshCookies(r)
	if len(refreshTokens) == 0 {
		return RefreshTokenRecord{}, errMissingRefreshToken
	}
	for _, refreshToken := range refreshTokens {
		candidate, ok, err := a.sessionService().Get(r.Context(), auth.RefreshTokenHash(refreshToken))
		if err != nil {
			return RefreshTokenRecord{}, err
		}
		if !ok || candidate.RevokedAt != nil || time.Now().After(candidate.ExpiresAt) {
			continue
		}
		return candidate, nil
	}
	return RefreshTokenRecord{}, errUnavailableRefreshSession
}

func (a *api) rotateSessionFromCookie(r *http.Request) (contracts.AuthSessionPayload, string, error) {
	rec, err := a.authSessionFromCookies(r)
	if err != nil {
		return contracts.AuthSessionPayload{}, "", err
	}
	currentHash := rec.RefreshTokenHash
	nextRefreshToken, nextHash, err := auth.NewRefreshToken()
	if err != nil {
		return contracts.AuthSessionPayload{}, "", err
	}
	rotated, ok, err := a.sessionService().Rotate(r.Context(), rec.ID, currentHash, nextHash, time.Now().Add(a.refreshTokenTTL), time.Now())
	if err != nil {
		return contracts.AuthSessionPayload{}, "", err
	}
	if !ok {
		return contracts.AuthSessionPayload{}, "", errors.New("session rotation failed")
	}
	identity, err := a.accounts.GetIdentity(rotated.UserID)
	if err != nil {
		return contracts.AuthSessionPayload{}, "", err
	}
	payload, err := a.issueAuthSessionPayload(identity, rotated.ID)
	if err != nil {
		return contracts.AuthSessionPayload{}, "", err
	}
	return payload, nextRefreshToken, nil
}

func (a *api) createSession(userID string, r *http.Request) (string, RefreshTokenRecord, error) {
	refreshToken, hash, err := auth.NewRefreshToken()
	if err != nil {
		return "", RefreshTokenRecord{}, err
	}
	record, err := a.sessionService().Create(r.Context(), userID, hash, time.Now().Add(a.refreshTokenTTL), AuthSessionParams{
		UserAgent: strings.TrimSpace(r.UserAgent()),
		IPAddress: a.clientIP(r),
	})
	if err != nil {
		return "", RefreshTokenRecord{}, err
	}
	return refreshToken, record, nil
}

func (a *api) issueAuthSessionPayload(identity Identity, sessionID string) (contracts.AuthSessionPayload, error) {
	bootstrapped, err := a.autoBootstrapAdmin(identity)
	if err != nil {
		return contracts.AuthSessionPayload{}, err
	}
	identity = bootstrapped
	if err := a.badges.SyncLoginBadges(identity.Sub); err != nil {
		return contracts.AuthSessionPayload{}, fmt.Errorf("sync login badges: %w", err)
	}
	accessToken, err := auth.IssueAppAccessToken(a.appAuthSecret, identity.Sub, sessionID, a.accessTokenTTL)
	if err != nil {
		return contracts.AuthSessionPayload{}, err
	}
	suggestedNickname, err := a.suggestedNickname(identity, "")
	if err != nil {
		return contracts.AuthSessionPayload{}, fmt.Errorf("suggest nickname: %w", err)
	}
	payload := contracts.AuthSessionPayload{
		AccessToken:           accessToken,
		NicknameRequired:      identity.NicknameRequired,
		SuggestedNickname:     suggestedNickname,
		LinkedProviders:       identity.LinkedProviders,
		AuthMigrationRequired: false,
		RecoveryAvailable:     false,
		CanPlay:               !identity.NicknameRequired && !identity.IsBanned,
		User:                  sessionUser(identity),
	}
	return payload, nil
}

func (a *api) suggestedNickname(identity Identity, fallbackName string) (string, error) {
	raw := defaultStr(identity.ProviderName, defaultStr(fallbackName, defaultStr(identity.GoogleName, identity.DisplayName)))
	if !identity.NicknameRequired {
		return raw, nil
	}
	return a.accounts.SuggestNickname(identity.Sub, raw)
}

func (a *api) autoBootstrapAdmin(identity Identity) (Identity, error) {
	if identity.IsAdmin {
		return identity, nil
	}
	email := strings.ToLower(strings.TrimSpace(identity.Email))
	if email == "" {
		return identity, nil
	}
	if _, ok := a.adminBootstrapEmails[email]; !ok {
		return identity, nil
	}
	if err := a.admin.SetUserAdmin(identity.Sub, true); err != nil {
		return Identity{}, err
	}
	return a.accounts.GetIdentity(identity.Sub)
}

func sessionUser(identity Identity) contracts.AuthUser {
	return contracts.AuthUser{
		ID:          identity.Sub,
		Email:       identity.Email,
		DisplayName: defaultStr(identity.DisplayName, identity.ProviderName),
		AvatarURL:   identity.AvatarURL,
		IsGuest:     identity.AccountType == "guest",
		IsAdmin:     identity.IsAdmin,
		IsModerator: identity.IsModerator,
	}
}

func (a *api) setRefreshCookie(w http.ResponseWriter, r *http.Request, refreshToken string) {
	http.SetCookie(w, &http.Cookie{
		Name:     a.refreshCookieName,
		Value:    refreshToken,
		Path:     "/",
		Domain:   a.refreshCookieDomain,
		HttpOnly: true,
		Secure:   requestIsHTTPS(r),
		SameSite: a.refreshCookieSameSite,
		Expires:  time.Now().Add(a.refreshTokenTTL),
		MaxAge:   int(a.refreshTokenTTL.Seconds()),
	})
}

func (a *api) clearRefreshCookie(w http.ResponseWriter, r *http.Request) {
	http.SetCookie(w, &http.Cookie{
		Name:     a.refreshCookieName,
		Value:    "",
		Path:     "/",
		Domain:   a.refreshCookieDomain,
		HttpOnly: true,
		Secure:   requestIsHTTPS(r),
		SameSite: a.refreshCookieSameSite,
		Expires:  time.Unix(0, 0),
		MaxAge:   -1,
	})
}

func (a *api) readRefreshCookies(r *http.Request) []string {
	cookies := r.CookiesNamed(a.refreshCookieName)
	values := make([]string, 0, len(cookies))
	seen := make(map[string]struct{}, len(cookies))
	for _, cookie := range cookies {
		value := strings.TrimSpace(cookie.Value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		values = append(values, value)
	}
	return values
}

func requestIsHTTPS(r *http.Request) bool {
	if strings.EqualFold(strings.TrimSpace(r.Header.Get("X-Forwarded-Proto")), "https") {
		return true
	}
	return r.TLS != nil
}
