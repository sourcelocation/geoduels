package main

import (
	"errors"
	"net/http"

	"geoduels/pkg/auth"
	"geoduels/pkg/contracts"
	"geoduels/pkg/matchlaunch"
	"geoduels/pkg/sessionpolicy"
)

func (a *api) bootstrap(w http.ResponseWriter, r *http.Request) {
	global := a.statusHub().current()
	response := contracts.BootstrapResponse{
		Version:  1,
		Activity: contracts.BootstrapActivity{Notifications: []contracts.UserNotification{}},
		Global: contracts.BootstrapGlobal{
			OnlinePlayers: global.OnlinePlayers,
			Maintenance:   global.Maintenance,
		},
	}
	record, err := a.authSessionFromCookies(r)
	if err != nil {
		if errors.Is(err, errMissingRefreshToken) || errors.Is(err, errUnavailableRefreshSession) {
			writeJSON(w, response)
			return
		}
		http.Error(w, "session restoration failed", http.StatusInternalServerError)
		return
	}
	identity, err := a.accounts.GetIdentity(record.UserID)
	if err != nil {
		http.Error(w, "identity unavailable", http.StatusInternalServerError)
		return
	}
	authPayload, err := a.issueReadOnlyAuthSessionPayload(identity, record.ID)
	if err != nil {
		http.Error(w, "session restoration failed", http.StatusInternalServerError)
		return
	}
	profile, err := a.profiles.GetProfile(record.UserID)
	if err != nil {
		http.Error(w, "profile unavailable", http.StatusInternalServerError)
		return
	}
	response.Auth = &authPayload
	accountType := "registered"
	if profile.IsGuest {
		accountType = "guest"
	}
	response.Viewer = &contracts.BootstrapViewer{
		ID: profile.UserID, Email: identity.Email, DisplayName: profile.DisplayName,
		AvatarURL: profile.AvatarURL, AccountType: accountType, MMR: profile.MMR,
		RatingRD: profile.RatingRD, GamesPlayed: profile.GamesPlayed, Wins: profile.Wins,
		RankedGamesPlayed: profile.RankedGamesPlayed, RankedWins: profile.RankedWins,
		IsAdmin: profile.IsAdmin, IsModerator: profile.IsModerator, IsBanned: profile.IsBanned,
		BanReason: profile.BanReason, LinkedProviders: identity.LinkedProviders,
		SelectedBadge: profile.SelectedBadge,
	}
	if a.preferences != nil {
		if preferences, err := a.preferences.Get(r.Context(), record.UserID); err == nil {
			response.Preferences = &contracts.BootstrapPreferences{Revision: preferences.Revision, Value: preferences.Preferences}
		}
	}
	response.Activity.ActiveMatch = a.activeMatch(r, record.UserID)
	if !profile.IsGuest {
		a.touchViewerPresence(r.Context(), record.UserID)
	}
	if a.notificationService != nil && !profile.IsGuest {
		if notifications, err := a.notificationService.List(r.Context(), record.UserID, 10); err == nil {
			response.Activity.Notifications = notifications
		}
	}
	writeJSON(w, response)
}

func (a *api) activeMatch(r *http.Request, userID string) *contracts.ResumableSessionResponse {
	if a.coord == nil {
		return nil
	}
	assigned, ok, err := a.coord.GetAssignmentByUser(r.Context(), userID)
	if err != nil || !ok {
		return nil
	}
	mode := sessionpolicy.NormalizeMode(assigned.Mode, assigned.MatchID)
	if mode != contracts.ModeDuel || a.launcher().ValidateAssignment(r.Context(), assigned) != matchlaunch.AssignmentValid {
		return nil
	}
	return &contracts.ResumableSessionResponse{Status: "match", MatchID: assigned.MatchID, Mode: string(mode)}
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
