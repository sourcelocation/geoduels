package main

import (
	"errors"
	"geoduels/internal/accounts"
	"net/http"
	"strings"

	"geoduels/pkg/contentfilter"
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

func (a *api) resolveOAuthIdentity(r *http.Request, state oauthStateClaims, provider, providerUserID, email, displayName, avatarURL string) (accounts.Identity, error) {
	provider = strings.TrimSpace(strings.ToLower(provider))
	providerUserID = strings.TrimSpace(providerUserID)
	if provider == "" || providerUserID == "" {
		return accounts.Identity{}, errors.New("provider identity unavailable")
	}
	intent := normalizeOAuthIntent(state.Intent)
	identityExists, err := a.accounts.ProviderIdentityExists(provider, providerUserID)
	if err != nil {
		return accounts.Identity{}, err
	}
	switch intent {
	case oauthIntentLink:
		if state.LinkSub == "" {
			return accounts.Identity{}, errors.New("link requires sign in")
		}
		return a.accounts.LinkProviderIdentity(provider, providerUserID, email, displayName, avatarURL, state.LinkSub)
	case oauthIntentUpgradeGuest:
		if state.LinkSub == "" {
			return accounts.Identity{}, errors.New("guest upgrade requires sign in")
		}
		identity, err := a.accounts.GetIdentity(state.LinkSub)
		if err != nil {
			return accounts.Identity{}, err
		}
		if !identity.IsGuest {
			return accounts.Identity{}, errors.New("guest upgrade requires guest account")
		}
		// Mixed merge/login: attach a brand-new provider to this guest, or
		// sign into the existing GeoDuels account for that provider.
		return a.accounts.UpsertProviderIdentity(provider, providerUserID, email, displayName, avatarURL, state.LinkSub)
	}
	if !identityExists {
		if banned, err := a.moderation.IsSignupIPBanned(r.Context(), a.clientIP(r)); err != nil {
			return accounts.Identity{}, errors.New("signup unavailable")
		} else if banned {
			return accounts.Identity{}, errors.New("signup unavailable")
		}
	}
	return a.accounts.UpsertProviderIdentity(provider, providerUserID, email, displayName, avatarURL, "")
}

func (a *api) oauthSessionPayload(provider, accessToken string, identity accounts.Identity, fallbackName, returnTo string) map[string]any {
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
			"isGuest":      identity.IsGuest,
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
	case strings.Contains(msg, "link requires sign in"):
		return "Sign in before linking another method."
	case strings.Contains(msg, "guest upgrade requires"):
		return "Sign in as a guest before saving progress."
	case strings.Contains(msg, "identity banned"):
		return "This sign-in method is banned from GeoDuels."
	case errors.Is(err, ErrOAuthEmailConflict):
		return "This verified email is linked to multiple GeoDuels accounts. Contact support to recover the account."
	case strings.Contains(msg, "signup unavailable"):
		return "signup unavailable"
	default:
		return "persist identity failed"
	}
}
