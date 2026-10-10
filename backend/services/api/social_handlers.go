package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"geoduels/internal/social"
	"geoduels/pkg/contracts"
)

func (a *api) friendsPage(c echo.Context) error {
	r := c.Request()
	userID, service, ok := a.socialActor(r)
	if !ok {
		return writeSocialError(c, http.StatusUnauthorized, "registration_required")
	}
	result, err := service.FriendsPage(r.Context(), userID, strings.TrimSpace(c.QueryParam("partyId")))
	if err != nil {
		return writeSocialError(c, http.StatusInternalServerError, "friends_page_unavailable")
	}
	friends, incoming, outgoing, recent := result.Friends, result.Incoming, result.Outgoing, result.Recent
	attachPartyInvites(friends, result.PartyInvites)
	a.touchViewerPresence(r.Context(), userID)
	a.applySocialPresence(r.Context(), friends)
	a.applySocialPresence(r.Context(), recent)
	return writeJSON(c, map[string]any{
		"friends":       friends,
		"requests":      map[string]any{"incoming": incoming, "outgoing": outgoing},
		"recentPlayers": recent,
	})
}

func attachPartyInvites(players []social.CompactPlayer, statuses map[string]social.CompactPartyInvite) {
	if len(statuses) == 0 {
		return
	}
	for i := range players {
		if invite, ok := statuses[players[i].UserID]; ok {
			value := invite
			players[i].PartyInvite = &value
		}
	}
}

func (a *api) socialService() (*social.Service, bool) {
	return a.social, a.social != nil
}

func (a *api) socialActor(r *http.Request) (string, *social.Service, bool) {
	claims, err := a.authenticatedClaims(r)
	if err != nil {
		return "", nil, false
	}
	service, ok := a.socialService()
	if !ok {
		return "", nil, false
	}
	if err := service.Authorize(r.Context(), claims.Sub); err != nil {
		return "", nil, false
	}
	return claims.Sub, service, true
}

func (a *api) socialSettings(c echo.Context) error {
	r := c.Request()
	userID, service, ok := a.socialActor(r)
	if !ok {
		return writeSocialError(c, http.StatusUnauthorized, "registration_required")
	}
	if r.Method == http.MethodGet {
		settings, err := service.GetSocialSettings(r.Context(), userID)
		if err != nil {
			return writeSocialStoreError(c, err)
		}
		return writeJSON(c, settings)
	}
	var settings social.SocialSettings
	if json.NewDecoder(r.Body).Decode(&settings) != nil {
		return writeSocialError(c, http.StatusBadRequest, "invalid_request")
	}
	settings, err := service.UpdateSocialSettings(r.Context(), userID, settings)
	if err != nil {
		return writeSocialStoreError(c, err)
	}
	return writeJSON(c, settings)
}

func (a *api) sendFriendRequest(c echo.Context) error {
	r := c.Request()
	userID, service, ok := a.socialActor(r)
	if !ok {
		return writeSocialError(c, http.StatusUnauthorized, "registration_required")
	}
	if allowed, retry, err := a.allowSocialAction(r, userID, "friend_request"); err != nil || !allowed {
		return writeSocialRateLimited(c, retry)
	}
	var body struct {
		UserID string `json:"userId"`
	}
	if json.NewDecoder(r.Body).Decode(&body) != nil {
		return writeSocialError(c, http.StatusBadRequest, "invalid_request")
	}
	item, err := service.SendFriendRequest(r.Context(), userID, strings.TrimSpace(body.UserID))
	if err != nil {
		return writeSocialStoreError(c, err)
	}
	targetID := strings.TrimSpace(body.UserID)
	a.liveInvalidate(userID, targetID)
	return writeJSONStatus(c, http.StatusCreated, item)
}

func (a *api) respondFriendRequest(c echo.Context) error {
	r := c.Request()
	userID, service, ok := a.socialActor(r)
	if !ok {
		return writeSocialError(c, http.StatusUnauthorized, "registration_required")
	}
	action := c.Param("action")
	if action != "accept" && action != "decline" && action != "cancel" {
		return writeSocialError(c, http.StatusBadRequest, "invalid_action")
	}
	requestID := c.Param("id")
	if err := service.RespondFriendRequest(r.Context(), userID, requestID, action); err != nil {
		return writeSocialStoreError(c, err)
	}
	a.liveInvalidate(userID)
	return c.NoContent(http.StatusNoContent)
}

func (a *api) removeFriend(c echo.Context) error {
	r := c.Request()
	userID, service, ok := a.socialActor(r)
	if !ok {
		return writeSocialError(c, http.StatusUnauthorized, "registration_required")
	}
	targetID := c.Param("userId")
	if err := service.RemoveFriend(r.Context(), userID, targetID); err != nil {
		return writeSocialStoreError(c, err)
	}
	a.liveInvalidate(userID, targetID)
	return c.NoContent(http.StatusNoContent)
}

func (a *api) userBlock(c echo.Context) error {
	r := c.Request()
	userID, service, ok := a.socialActor(r)
	if !ok {
		return writeSocialError(c, http.StatusUnauthorized, "registration_required")
	}
	targetID := c.Param("userId")
	if err := service.SetUserBlock(r.Context(), userID, targetID, r.Method == http.MethodPost); err != nil {
		return writeSocialStoreError(c, err)
	}
	a.liveInvalidate(userID, targetID)
	return c.NoContent(http.StatusNoContent)
}

func (a *api) socialPlayerSearch(c echo.Context) error {
	r := c.Request()
	userID, service, ok := a.socialActor(r)
	if !ok {
		return writeSocialError(c, http.StatusUnauthorized, "registration_required")
	}
	if allowed, retry, err := a.allowSocialAction(r, userID, "player_search"); err != nil || !allowed {
		return writeSocialRateLimited(c, retry)
	}
	players, err := service.SearchSocialPlayers(r.Context(), userID, c.QueryParam("q"), queryLimit(r, 10))
	if err != nil {
		return writeSocialError(c, http.StatusInternalServerError, "search_unavailable")
	}
	a.applySocialPresence(r.Context(), players)
	return writeJSON(c, map[string]any{"players": players})
}

func (a *api) playerRelationship(c echo.Context) error {
	r := c.Request()
	userID, service, ok := a.socialActor(r)
	if !ok {
		return writeSocialError(c, http.StatusUnauthorized, "registration_required")
	}
	profile, err := a.profiles.GetPublicPlayerProfileByNickname(c.Param("nickname"))
	if err != nil {
		return writeSocialError(c, http.StatusNotFound, "player_not_found")
	}
	state, requestID, err := service.Relationship(r.Context(), userID, profile.UserID)
	if err != nil {
		return writeSocialError(c, http.StatusInternalServerError, "relationship_unavailable")
	}
	return writeJSON(c, map[string]any{"state": state, "requestId": requestID})
}

func (a *api) createFriendCode(c echo.Context) error {
	r := c.Request()
	userID, service, ok := a.socialActor(r)
	if !ok {
		return writeSocialError(c, http.StatusUnauthorized, "registration_required")
	}
	code, err := service.CreateFriendCode(r.Context(), userID, social.DefaultFriendCodeTTL)
	if err != nil {
		return writeSocialStoreError(c, err)
	}
	return writeJSONStatus(c, http.StatusCreated, code)
}

func (a *api) resolveFriendCode(c echo.Context) error {
	r := c.Request()
	userID, service, ok := a.socialActor(r)
	if !ok {
		return writeSocialError(c, http.StatusUnauthorized, "registration_required")
	}
	if allowed, retry, err := a.allowSocialAction(r, userID, "code_resolve"); err != nil || !allowed {
		return writeSocialRateLimited(c, retry)
	}
	player, err := service.ResolveFriendCode(r.Context(), userID, c.Param("code"))
	if err != nil {
		return writeSocialStoreError(c, err)
	}
	return writeJSON(c, player)
}

func (a *api) sendFriendCodeRequest(c echo.Context) error {
	r := c.Request()
	userID, service, ok := a.socialActor(r)
	if !ok {
		return writeSocialError(c, http.StatusUnauthorized, "registration_required")
	}
	player, err := service.ResolveFriendCode(r.Context(), userID, c.Param("code"))
	if err == nil {
		_, err = service.SendFriendRequest(r.Context(), userID, player.UserID)
	}
	if err != nil {
		return writeSocialStoreError(c, err)
	}
	a.liveInvalidate(userID, player.UserID)
	return c.NoContent(http.StatusNoContent)
}

func (a *api) partyInvitations(c echo.Context) error {
	r := c.Request()
	userID, service, ok := a.socialActor(r)
	if !ok {
		return writeSocialError(c, http.StatusUnauthorized, "registration_required")
	}
	if r.Method == http.MethodGet {
		items, err := service.ListPartyInvitations(r.Context(), userID, queryLimit(r, 10))
		if err != nil {
			return writeSocialStoreError(c, err)
		}
		return writeJSON(c, map[string]any{"invitations": items})
	}
	if allowed, retry, err := a.allowSocialAction(r, userID, "party_invite"); err != nil || !allowed {
		return writeSocialRateLimited(c, retry)
	}
	var body struct {
		UserID string `json:"userId"`
	}
	if json.NewDecoder(r.Body).Decode(&body) != nil {
		return writeSocialError(c, http.StatusBadRequest, "invalid_request")
	}
	item, err := service.CreatePartyInvitation(r.Context(), c.Param("id"), userID, body.UserID, 20*time.Minute)
	if err != nil {
		return writeSocialStoreError(c, err)
	}
	a.liveInvalidate(userID, body.UserID)
	return writeJSONStatus(c, http.StatusCreated, item)
}

func (a *api) createPartyAndInvite(c echo.Context) error {
	r := c.Request()
	userID, service, ok := a.socialActor(r)
	if !ok {
		return writeSocialError(c, http.StatusUnauthorized, "registration_required")
	}
	if allowed, retry, err := a.allowSocialAction(r, userID, "party_invite"); err != nil || !allowed {
		return writeSocialRateLimited(c, retry)
	}
	var body struct {
		UserID string `json:"userId"`
	}
	if json.NewDecoder(r.Body).Decode(&body) != nil {
		return writeSocialError(c, http.StatusBadRequest, "invalid_request")
	}
	party, err := a.parties.CreateParty(userID, contracts.ModeDuel, "world", 2*time.Hour)
	if err != nil {
		return writeSocialError(c, http.StatusInternalServerError, "party_unavailable")
	}
	invitation, err := service.CreatePartyInvitation(r.Context(), party.ID, userID, body.UserID, social.DefaultPartyInviteTTL)
	if err != nil {
		_, _ = a.parties.LeaveParty(party.ID, userID)
		return writeSocialStoreError(c, err)
	}
	a.liveInvalidate(userID, body.UserID)
	return writeJSONStatus(c, http.StatusCreated, map[string]any{
		"invitation": invitation,
		"party":      party,
	})
}

func (a *api) respondPartyInvitation(c echo.Context) error {
	r := c.Request()
	userID, service, ok := a.socialActor(r)
	if !ok {
		return writeSocialError(c, http.StatusUnauthorized, "registration_required")
	}
	action := c.Param("action")
	if action != "accept" && action != "decline" {
		return writeSocialError(c, http.StatusBadRequest, "invalid_action")
	}
	item, err := service.RespondPartyInvitation(r.Context(), userID, c.Param("id"), action)
	if err != nil {
		return writeSocialStoreError(c, err)
	}
	a.liveInvalidate(userID)
	return writeJSON(c, item)
}

func (a *api) liveInvalidate(userIDs ...string) {
	if a.live == nil {
		return
	}
	a.live.publishInvalidate(userIDs...)
}

func writeSocialStoreError(c echo.Context, err error) error {
	switch {
	case errors.Is(err, ErrSocialNotFound):
		return writeSocialError(c, http.StatusNotFound, "social_action_unavailable")
	case errors.Is(err, ErrSocialBlocked):
		return writeSocialError(c, http.StatusForbidden, "social_action_unavailable")
	case errors.Is(err, ErrSocialLimit):
		return writeSocialError(c, http.StatusConflict, "social_limit_reached")
	default:
		return writeSocialError(c, http.StatusInternalServerError, "social_action_failed")
	}
}

func writeSocialError(c echo.Context, status int, code string) error {
	return writeJSONStatus(c, status, map[string]string{"error": code})
}
