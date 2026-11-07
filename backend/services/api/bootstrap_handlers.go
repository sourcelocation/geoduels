package main

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"geoduels/pkg/auth"
	"geoduels/pkg/maintenance"
	"geoduels/pkg/staff"
)

// bootstrapResponse is everything a client needs on load. Auth and viewer are null when signed out.
type bootstrapResponse struct {
	Auth        *authSessionPayload   `json:"auth"`
	Viewer      *bootstrapViewer      `json:"viewer"`
	Preferences *bootstrapPreferences `json:"preferences"`
	Client      bootstrapClient       `json:"client"`
	Global      bootstrapGlobal       `json:"global"`
}

// bootstrapClient is what the browser needs to start third-party flows. All of it is public.
type bootstrapClient struct {
	GoogleClientID   string `json:"googleClientId"`
	DiscordClientID  string `json:"discordClientId"`
	TurnstileSiteKey string `json:"turnstileSiteKey"`
	GoogleEmbedKey   string `json:"googleEmbedKey"`
}

type bootstrapViewer struct {
	Roles             staff.Roles   `json:"roles"`
	ID                string        `json:"id"`
	Email             string        `json:"email,omitempty"`
	DisplayName       string        `json:"displayName"`
	AvatarURL         string        `json:"avatarUrl,omitempty"`
	AccountType       string        `json:"accountType"`
	MMR               int           `json:"mmr"`
	RatingRD          float64       `json:"ratingRd,omitempty"`
	GamesPlayed       int           `json:"gamesPlayed"`
	Wins              int           `json:"wins"`
	RankedGamesPlayed int           `json:"rankedGamesPlayed"`
	RankedWins        int           `json:"rankedWins"`
	IsAdmin           bool          `json:"isAdmin,omitempty"`
	IsModerator       bool          `json:"isModerator,omitempty"`
	IsBanned          bool          `json:"isBanned,omitempty"`
	BanReason         string        `json:"banReason,omitempty"`
	LinkedProviders   []string      `json:"linkedProviders,omitempty"`
	Badges            []playerBadge `json:"badges,omitempty"`
	SelectedBadge     *playerBadge  `json:"selectedBadge,omitempty"`
}

type bootstrapPreferences struct {
	Revision int64           `json:"revision"`
	Value    json.RawMessage `json:"value"`
}

type bootstrapGlobal struct {
	OnlinePlayers int                `json:"onlinePlayers"`
	Maintenance   maintenance.Status `json:"maintenance"`
}

func (a *api) bootstrap(c echo.Context) error {
	r := c.Request()
	global := a.status.current()
	response := bootstrapResponse{
		Client: bootstrapClient{
			GoogleClientID: a.googleClientID, DiscordClientID: a.discordClientID,
			TurnstileSiteKey: a.turnstileSiteKey, GoogleEmbedKey: a.googleEmbedKey,
		},
		Global: bootstrapGlobal{
			OnlinePlayers: global.OnlinePlayers,
			Maintenance:   global.Maintenance,
		},
	}
	record, err := a.authSessionFromCookies(r)
	if err != nil {
		if errors.Is(err, errMissingRefreshToken) || errors.Is(err, errUnavailableRefreshSession) {
			return writeJSON(c, response)
		}
		return plainTextError(c, http.StatusInternalServerError, "session restoration failed")
	}
	identity, err := a.accounts.GetIdentity(record.UserID)
	if err != nil {
		return plainTextError(c, http.StatusInternalServerError, "identity unavailable")
	}
	authPayload, err := a.issueReadOnlyAuthSessionPayload(identity, record.ID)
	if err != nil {
		return plainTextError(c, http.StatusInternalServerError, "session restoration failed")
	}
	profile, err := a.profiles.GetProfile(record.UserID)
	if err != nil {
		return plainTextError(c, http.StatusInternalServerError, "profile unavailable")
	}
	response.Auth = &authPayload
	accountType := "registered"
	if profile.IsGuest {
		accountType = "guest"
	}
	response.Viewer = &bootstrapViewer{
		Roles: identity.Roles, ID: profile.UserID, Email: identity.Email, DisplayName: profile.DisplayName,
		AvatarURL: profile.AvatarURL, AccountType: accountType, MMR: profile.MMR,
		RatingRD: profile.RatingRD, GamesPlayed: profile.GamesPlayed, Wins: profile.Wins,
		RankedGamesPlayed: profile.RankedGamesPlayed, RankedWins: profile.RankedWins,
		IsAdmin: profile.IsAdmin, IsModerator: profile.IsModerator, IsBanned: profile.IsBanned,
		BanReason: profile.BanReason, LinkedProviders: identity.LinkedProviders,
		Badges: newPlayerBadges(profile.Badges), SelectedBadge: newPlayerBadgeRef(profile.SelectedBadge),
	}
	if a.preferences != nil {
		if preferences, err := a.preferences.Get(r.Context(), record.UserID); err == nil {
			response.Preferences = &bootstrapPreferences{Revision: preferences.Revision, Value: preferences.Preferences}
		}
	}
	return writeJSON(c, response)
}

func (a *api) issueReadOnlyAuthSessionPayload(identity Identity, sessionID string) (authSessionPayload, error) {
	accessToken, err := auth.IssueAppAccessToken(a.appAuthSecret, identity.Sub, sessionID, a.accessTokenTTL)
	if err != nil {
		return authSessionPayload{}, err
	}
	suggestedNickname, err := a.suggestedNickname(identity, "")
	if err != nil {
		return authSessionPayload{}, err
	}
	return authSessionPayload{
		AccessToken: accessToken, NicknameRequired: identity.NicknameRequired,
		SuggestedNickname: suggestedNickname, LinkedProviders: identity.LinkedProviders,
		CanPlay: !identity.NicknameRequired && !identity.IsBanned, User: sessionUser(identity),
	}, nil
}
