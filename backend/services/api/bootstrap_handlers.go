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
	Version     int                   `json:"version"`
	Auth        *authSessionPayload   `json:"auth"`
	Viewer      *bootstrapViewer      `json:"viewer"`
	Preferences *bootstrapPreferences `json:"preferences"`
	Activity    bootstrapActivity     `json:"activity"`
	Global      bootstrapGlobal       `json:"global"`
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

type bootstrapActivity struct {
	// Included only in bootstrap v2; v1 retains its original JSON shape.
	CurrentParty  *currentParty       `json:"currentParty,omitempty"`
	ActiveMatch   *activeMatchSummary `json:"activeMatch"`
	Notifications []userNotification  `json:"notifications"`
}

type currentParty struct {
	ID         string `json:"id"`
	InviteCode string `json:"inviteCode"`
}

type bootstrapGlobal struct {
	OnlinePlayers int                `json:"onlinePlayers"`
	Maintenance   maintenance.Status `json:"maintenance"`
}

func (a *api) bootstrap(c echo.Context) error {
	switch c.QueryParam("version") {
	case "", "1":
		return a.bootstrapVersion(c, 1)
	case "2":
		return a.bootstrapVersion(c, 2)
	default:
		return plainTextError(c, http.StatusBadRequest, "unsupported bootstrap version")
	}
}

func (a *api) bootstrapVersion(c echo.Context, version int) error {
	r := c.Request()
	global := a.statusHub().current()
	response := bootstrapResponse{
		Version:  version,
		Activity: bootstrapActivity{Notifications: []userNotification{}},
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
	if version >= 2 && a.parties != nil {
		party, err := a.parties.GetCurrentParty(record.UserID)
		if err != nil {
			return plainTextError(c, http.StatusInternalServerError, "party restoration unavailable")
		}
		if party != nil {
			response.Activity.CurrentParty = &currentParty{ID: party.ID, InviteCode: party.InviteCode}
		}
	}
	response.Activity.ActiveMatch = a.activeMatch(r.Context(), record.UserID)
	if !profile.IsGuest {
		a.touchViewerPresence(r.Context(), record.UserID)
	}
	if a.notificationService != nil && !profile.IsGuest {
		if unseen, err := a.notificationService.List(r.Context(), record.UserID, 10); err == nil {
			response.Activity.Notifications = newUserNotifications(unseen)
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
