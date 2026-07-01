package main

import (
	"errors"
	"net/http"
	"strings"

	"geoduels/pkg/contentfilter"
	"geoduels/pkg/persistence"
)

const (
	oauthIntentSignIn       = "signin"
	oauthIntentLink         = "link"
	oauthIntentUpgradeGuest = "upgrade_guest"
)

func normalizeOAuthIntent(raw string) string {
	switch strings.TrimSpace(strings.ToLower(raw)) {
	case oauthIntentLink:
		return oauthIntentLink
	case oauthIntentUpgradeGuest:
		return oauthIntentUpgradeGuest
	default:
		return oauthIntentSignIn
	}
}

func (a *api) oauthLinkSubject(r *http.Request, intent string) (string, error) {
	switch intent {
	case oauthIntentLink, oauthIntentUpgradeGuest:
		claims, err := a.authenticatedClaims(r)
		if err != nil {
			return "", err
		}
		return claims.Sub, nil
	default:
		return "", nil
	}
}

func oauthStartError(intent string) string {
	switch intent {
	case oauthIntentLink:
		return "sign in before linking another method"
	case oauthIntentUpgradeGuest:
		return "sign in before saving guest progress"
	default:
		return "unauthorized"
	}
}

func (a *api) resolveOAuthIdentity(r *http.Request, state oauthStateClaims, provider, providerUserID, email, displayName, avatarURL string) (persistence.Identity, error) {
	provider = strings.TrimSpace(strings.ToLower(provider))
	providerUserID = strings.TrimSpace(providerUserID)
	if provider == "" || providerUserID == "" {
		return persistence.Identity{}, errors.New("provider identity unavailable")
	}
	if banned, _, err := a.store.IsProviderIdentityBanned(provider, providerUserID); err != nil {
		return persistence.Identity{}, err
	} else if banned {
		return persistence.Identity{}, errors.New("provider identity banned")
	}
	switch normalizeOAuthIntent(state.Intent) {
	case oauthIntentLink:
		if state.LinkSub == "" {
			return persistence.Identity{}, errors.New("link requires sign in")
		}
		return a.store.LinkProviderIdentity(provider, providerUserID, email, displayName, avatarURL, state.LinkSub)
	case oauthIntentUpgradeGuest:
		if state.LinkSub == "" {
			return persistence.Identity{}, errors.New("guest upgrade requires sign in")
		}
		identity, err := a.store.GetIdentity(state.LinkSub)
		if err != nil {
			return persistence.Identity{}, err
		}
		if identity.AccountType != "guest" {
			return persistence.Identity{}, errors.New("guest upgrade requires guest account")
		}
		identity, err = a.store.LinkProviderIdentity(provider, providerUserID, email, displayName, avatarURL, state.LinkSub)
		if err != nil && strings.Contains(strings.ToLower(err.Error()), "already linked") {
			return persistence.Identity{}, errors.New("provider account already exists")
		}
		return identity, err
	}
	identityExists, err := a.store.ProviderIdentityExists(provider, providerUserID)
	if err != nil {
		return persistence.Identity{}, err
	}
	if !identityExists {
		if banned, err := a.store.IsSignupIPBanned(a.clientIP(r)); err != nil {
			return persistence.Identity{}, errors.New("signup unavailable")
		} else if banned {
			return persistence.Identity{}, errors.New("signup unavailable")
		}
	}
	return a.store.UpsertProviderIdentity(provider, providerUserID, email, displayName, avatarURL, "")
}

func (a *api) oauthSessionPayload(provider, accessToken string, identity persistence.Identity, fallbackName, returnTo string) map[string]any {
	suggestedNick, err := a.suggestedNickname(identity, fallbackName)
	if err != nil {
		suggestedNick = contentfilter.NicknameSuggestionBase(defaultStr(identity.ProviderName, defaultStr(fallbackName, identity.DisplayName)))
	}
	return map[string]any{
		"ok":                    true,
		"provider":              provider,
		"accessToken":           accessToken,
		"nicknameRequired":      identity.NicknameRequired,
		"suggestedNickname":     suggestedNick,
		"linkedProviders":       identity.LinkedProviders,
		"authMigrationRequired": false,
		"recoveryAvailable":     false,
		"canPlay":               !identity.NicknameRequired && !identity.IsBanned,
		"returnTo":              returnTo,
		"user": map[string]any{
			"id":           identity.Sub,
			"display_name": defaultStr(identity.DisplayName, suggestedNick),
			"avatar_url":   identity.AvatarURL,
			"email":        identity.Email,
			"isGuest":      identity.AccountType == "guest",
			"isAdmin":      identity.IsAdmin,
			"isModerator":  identity.IsModerator,
		},
	}
}

func oauthUserError(err error) string {
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(msg, "already linked"):
		return "This sign-in method is already linked to another GeoDuels account. Sign out first to use it."
	case strings.Contains(msg, "provider account already exists"):
		return "This sign-in method already has a GeoDuels account. Sign out and continue with that provider."
	case strings.Contains(msg, "link requires sign in"):
		return "Sign in before linking another method."
	case strings.Contains(msg, "guest upgrade requires"):
		return "Sign in as a guest before saving progress."
	case strings.Contains(msg, "identity banned"):
		return "This sign-in method is banned from GeoDuels."
	case strings.Contains(msg, "signup unavailable"):
		return "signup unavailable"
	default:
		return "persist identity failed"
	}
}
