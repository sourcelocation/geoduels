package main

import (
	"context"
	"encoding/json"
	"net/http"

	"geoduels/pkg/auth"
	"geoduels/pkg/persistence"
)

type requestPrincipalKey struct{}

type requestPrincipal struct {
	claims   auth.AppClaims
	identity persistence.Identity
}

func (a *api) authenticatedAccount(r *http.Request) (auth.AppClaims, persistence.Identity, error) {
	if principal, ok := r.Context().Value(requestPrincipalKey{}).(requestPrincipal); ok {
		return principal.claims, principal.identity, nil
	}
	claims, err := a.authenticatedClaims(r)
	if err != nil {
		return auth.AppClaims{}, persistence.Identity{}, err
	}
	identity, err := a.accounts.GetIdentity(claims.Sub)
	return claims, identity, err
}

func (a *api) authenticatedIdentity(r *http.Request) (persistence.Identity, error) {
	_, identity, err := a.authenticatedAccount(r)
	return identity, err
}

func (a *api) accountBanned(userID string) (bool, error) {
	identity, err := a.accounts.GetIdentity(userID)
	return identity.IsBanned, err
}

// active protects actions that a signed-in but banned account may not
// perform. Authentication, account management, notifications, and read-only
// routes deliberately do not use this wrapper.
func (a *api) active(next http.HandlerFunc) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims, identity, err := a.authenticatedAccount(r)
		if err != nil {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
		if identity.IsBanned {
			writeAPIError(w, http.StatusForbidden, "account_banned", "user is banned")
			return
		}
		ctx := context.WithValue(r.Context(), requestPrincipalKey{}, requestPrincipal{claims: claims, identity: identity})
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func writeAPIError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": message, "code": code})
}
