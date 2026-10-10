package main

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"

	"geoduels/pkg/auth"
	"geoduels/pkg/contracts"
)

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
	response := contracts.BootstrapResponse{
		Version:  version,
		Activity: contracts.BootstrapActivity{Notifications: []contracts.UserNotification{}},
		Global: contracts.BootstrapGlobal{
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
	response.Viewer = &contracts.BootstrapViewer{
		Roles: identity.Roles, ID: profile.UserID, Email: identity.Email, DisplayName: profile.DisplayName,
		AvatarURL: profile.AvatarURL, AccountType: accountType, MMR: profile.MMR,
		RatingRD: profile.RatingRD, GamesPlayed: profile.GamesPlayed, Wins: profile.Wins,
		RankedGamesPlayed: profile.RankedGamesPlayed, RankedWins: profile.RankedWins,
		IsAdmin: profile.IsAdmin, IsModerator: profile.IsModerator, IsBanned: profile.IsBanned,
		BanReason: profile.BanReason, LinkedProviders: identity.LinkedProviders,
		Badges: profile.Badges, SelectedBadge: profile.SelectedBadge,
	}
	if a.preferences != nil {
		if preferences, err := a.preferences.Get(r.Context(), record.UserID); err == nil {
			response.Preferences = &contracts.BootstrapPreferences{Revision: preferences.Revision, Value: preferences.Preferences}
		}
	}
	if version >= 2 && a.parties != nil {
		party, err := a.parties.GetCurrentParty(record.UserID)
		if err != nil {
			return plainTextError(c, http.StatusInternalServerError, "party restoration unavailable")
		}
		response.Activity.CurrentParty = party
	}
	response.Activity.ActiveMatch = a.activeMatch(r.Context(), record.UserID)
	if !profile.IsGuest {
		a.touchViewerPresence(r.Context(), record.UserID)
	}
	if a.notificationService != nil && !profile.IsGuest {
		if notifications, err := a.notificationService.List(r.Context(), record.UserID, 10); err == nil {
			response.Activity.Notifications = notifications
		}
	}
	return writeJSON(c, response)
}

func (a *api) issueReadOnlyAuthSessionPayload(identity Identity, sessionID string) (contracts.AuthSessionPayload, error) {
	accessToken, err := auth.IssueAppAccessToken(a.appAuthSecret, identity.Sub, sessionID, a.accessTokenTTL)
	if err != nil {
		return contracts.AuthSessionPayload{}, err
	}
	suggestedNickname, err := a.suggestedNickname(identity, "")
	if err != nil {
		return contracts.AuthSessionPayload{}, err
	}
	return contracts.AuthSessionPayload{
		AccessToken: accessToken, NicknameRequired: identity.NicknameRequired,
		SuggestedNickname: suggestedNickname, LinkedProviders: identity.LinkedProviders,
		CanPlay: !identity.NicknameRequired && !identity.IsBanned, User: sessionUser(identity),
	}, nil
}
