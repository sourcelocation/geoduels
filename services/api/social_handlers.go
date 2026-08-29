package main

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gorilla/mux"

	"geoduels/pkg/contracts"
	"geoduels/pkg/social"
)

func (a *api) friendsPage(w http.ResponseWriter, r *http.Request) {
	userID, service, ok := a.socialActor(r)
	if !ok {
		writeSocialError(w, http.StatusUnauthorized, "registration_required")
		return
	}
	result, err := service.FriendsPage(r.Context(), userID, strings.TrimSpace(r.URL.Query().Get("partyId")))
	if err != nil {
		writeSocialError(w, http.StatusInternalServerError, "friends_page_unavailable")
		return
	}
	friends, incoming, outgoing, recent := result.Friends, result.Incoming, result.Outgoing, result.Recent
	attachPartyInvites(friends, result.PartyInvites)
	a.touchViewerPresence(r.Context(), userID)
	a.applySocialPresence(r.Context(), friends)
	a.applySocialPresence(r.Context(), recent)
	writeJSON(w, map[string]any{
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

func (a *api) socialSettings(w http.ResponseWriter, r *http.Request) {
	userID, service, ok := a.socialActor(r)
	if !ok {
		writeSocialError(w, http.StatusUnauthorized, "registration_required")
		return
	}
	if r.Method == http.MethodGet {
		settings, err := service.GetSocialSettings(r.Context(), userID)
		if err != nil {
			writeSocialStoreError(w, err)
			return
		}
		writeJSON(w, settings)
		return
	}
	var settings social.SocialSettings
	if json.NewDecoder(r.Body).Decode(&settings) != nil {
		writeSocialError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	settings, err := service.UpdateSocialSettings(r.Context(), userID, settings)
	if err != nil {
		writeSocialStoreError(w, err)
		return
	}
	writeJSON(w, settings)
}

func (a *api) sendFriendRequest(w http.ResponseWriter, r *http.Request) {
	userID, service, ok := a.socialActor(r)
	if !ok {
		writeSocialError(w, http.StatusUnauthorized, "registration_required")
		return
	}
	if allowed, retry, err := a.allowSocialAction(r, userID, "friend_request"); err != nil || !allowed {
		writeSocialRateLimited(w, retry)
		return
	}
	var body struct {
		UserID string `json:"userId"`
	}
	if json.NewDecoder(r.Body).Decode(&body) != nil {
		writeSocialError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	item, err := service.SendFriendRequest(r.Context(), userID, strings.TrimSpace(body.UserID))
	if err != nil {
		writeSocialStoreError(w, err)
		return
	}
	targetID := strings.TrimSpace(body.UserID)
	a.publishSocialLive(targetID, "friend_request_received", userID, targetID)
	writeJSONStatus(w, http.StatusCreated, item)
}

func (a *api) respondFriendRequest(w http.ResponseWriter, r *http.Request) {
	userID, service, ok := a.socialActor(r)
	if !ok {
		writeSocialError(w, http.StatusUnauthorized, "registration_required")
		return
	}
	action := mux.Vars(r)["action"]
	if action != "accept" && action != "decline" && action != "cancel" {
		writeSocialError(w, http.StatusBadRequest, "invalid_action")
		return
	}
	requestID := mux.Vars(r)["id"]
	if err := service.RespondFriendRequest(r.Context(), userID, requestID, action); err != nil {
		writeSocialStoreError(w, err)
		return
	}
	a.liveInvalidate(userID)
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) removeFriend(w http.ResponseWriter, r *http.Request) {
	userID, service, ok := a.socialActor(r)
	if !ok {
		writeSocialError(w, http.StatusUnauthorized, "registration_required")
		return
	}
	targetID := mux.Vars(r)["userId"]
	if err := service.RemoveFriend(r.Context(), userID, targetID); err != nil {
		writeSocialStoreError(w, err)
		return
	}
	a.liveInvalidate(userID, targetID)
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) userBlock(w http.ResponseWriter, r *http.Request) {
	userID, service, ok := a.socialActor(r)
	if !ok {
		writeSocialError(w, http.StatusUnauthorized, "registration_required")
		return
	}
	targetID := mux.Vars(r)["userId"]
	if err := service.SetUserBlock(r.Context(), userID, targetID, r.Method == http.MethodPost); err != nil {
		writeSocialStoreError(w, err)
		return
	}
	a.liveInvalidate(userID, targetID)
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) socialPlayerSearch(w http.ResponseWriter, r *http.Request) {
	userID, service, ok := a.socialActor(r)
	if !ok {
		writeSocialError(w, http.StatusUnauthorized, "registration_required")
		return
	}
	if allowed, retry, err := a.allowSocialAction(r, userID, "player_search"); err != nil || !allowed {
		writeSocialRateLimited(w, retry)
		return
	}
	players, err := service.SearchSocialPlayers(r.Context(), userID, r.URL.Query().Get("q"), queryLimit(r, 10))
	if err != nil {
		writeSocialError(w, http.StatusInternalServerError, "search_unavailable")
		return
	}
	a.applySocialPresence(r.Context(), players)
	writeJSON(w, map[string]any{"players": players})
}

func (a *api) playerRelationship(w http.ResponseWriter, r *http.Request) {
	userID, service, ok := a.socialActor(r)
	if !ok {
		writeSocialError(w, http.StatusUnauthorized, "registration_required")
		return
	}
	profile, err := a.profiles.GetPublicPlayerProfileByNickname(mux.Vars(r)["nickname"])
	if err != nil {
		writeSocialError(w, http.StatusNotFound, "player_not_found")
		return
	}
	state, requestID, err := service.Relationship(r.Context(), userID, profile.UserID)
	if err != nil {
		writeSocialError(w, http.StatusInternalServerError, "relationship_unavailable")
		return
	}
	writeJSON(w, map[string]any{"state": state, "requestId": requestID})
}

func (a *api) createFriendCode(w http.ResponseWriter, r *http.Request) {
	userID, service, ok := a.socialActor(r)
	if !ok {
		writeSocialError(w, http.StatusUnauthorized, "registration_required")
		return
	}
	code, err := service.CreateFriendCode(r.Context(), userID, 7*24*time.Hour)
	if err != nil {
		writeSocialStoreError(w, err)
		return
	}
	writeJSONStatus(w, http.StatusCreated, code)
}

func (a *api) resolveFriendCode(w http.ResponseWriter, r *http.Request) {
	userID, service, ok := a.socialActor(r)
	if !ok {
		writeSocialError(w, http.StatusUnauthorized, "registration_required")
		return
	}
	if allowed, retry, err := a.allowSocialAction(r, userID, "code_resolve"); err != nil || !allowed {
		writeSocialRateLimited(w, retry)
		return
	}
	player, err := service.ResolveFriendCode(r.Context(), userID, mux.Vars(r)["code"])
	if err != nil {
		writeSocialStoreError(w, err)
		return
	}
	writeJSON(w, player)
}

func (a *api) sendFriendCodeRequest(w http.ResponseWriter, r *http.Request) {
	userID, service, ok := a.socialActor(r)
	if !ok {
		writeSocialError(w, http.StatusUnauthorized, "registration_required")
		return
	}
	player, err := service.ResolveFriendCode(r.Context(), userID, mux.Vars(r)["code"])
	if err == nil {
		_, err = service.SendFriendRequest(r.Context(), userID, player.UserID)
	}
	if err != nil {
		writeSocialStoreError(w, err)
		return
	}
	a.publishSocialLive(player.UserID, "friend_request_received", userID, player.UserID)
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) partyInvitations(w http.ResponseWriter, r *http.Request) {
	userID, service, ok := a.socialActor(r)
	if !ok {
		writeSocialError(w, http.StatusUnauthorized, "registration_required")
		return
	}
	if r.Method == http.MethodGet {
		items, err := service.ListPartyInvitations(r.Context(), userID, queryLimit(r, 10))
		if err != nil {
			writeSocialStoreError(w, err)
			return
		}
		writeJSON(w, map[string]any{"invitations": items})
		return
	}
	if allowed, retry, err := a.allowSocialAction(r, userID, "party_invite"); err != nil || !allowed {
		writeSocialRateLimited(w, retry)
		return
	}
	var body struct {
		UserID string `json:"userId"`
	}
	if json.NewDecoder(r.Body).Decode(&body) != nil {
		writeSocialError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	item, err := service.CreatePartyInvitation(r.Context(), mux.Vars(r)["id"], userID, body.UserID, 20*time.Minute)
	if err != nil {
		writeSocialStoreError(w, err)
		return
	}
	a.publishSocialLive(body.UserID, "party_invitation_received", userID, body.UserID)
	writeJSONStatus(w, http.StatusCreated, item)
}

func (a *api) createPartyAndInvite(w http.ResponseWriter, r *http.Request) {
	userID, service, ok := a.socialActor(r)
	if !ok {
		writeSocialError(w, http.StatusUnauthorized, "registration_required")
		return
	}
	if allowed, retry, err := a.allowSocialAction(r, userID, "party_invite"); err != nil || !allowed {
		writeSocialRateLimited(w, retry)
		return
	}
	var body struct {
		UserID string `json:"userId"`
	}
	if json.NewDecoder(r.Body).Decode(&body) != nil {
		writeSocialError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	party, err := a.parties.CreateParty(userID, contracts.ModeDuel, "world", 2*time.Hour)
	if err != nil {
		writeSocialError(w, http.StatusInternalServerError, "party_unavailable")
		return
	}
	invitation, err := service.CreatePartyInvitation(r.Context(), party.ID, userID, body.UserID, 20*time.Minute)
	if err != nil {
		_, _ = a.parties.LeaveParty(party.ID, userID)
		writeSocialStoreError(w, err)
		return
	}
	a.publishSocialLive(body.UserID, "party_invitation_received", userID, body.UserID)
	writeJSONStatus(w, http.StatusCreated, map[string]any{
		"invitation": invitation,
		"party":      party,
	})
}

func (a *api) respondPartyInvitation(w http.ResponseWriter, r *http.Request) {
	userID, service, ok := a.socialActor(r)
	if !ok {
		writeSocialError(w, http.StatusUnauthorized, "registration_required")
		return
	}
	action := mux.Vars(r)["action"]
	if action != "accept" && action != "decline" {
		writeSocialError(w, http.StatusBadRequest, "invalid_action")
		return
	}
	item, err := service.RespondPartyInvitation(r.Context(), userID, mux.Vars(r)["id"], action)
	if err != nil {
		writeSocialStoreError(w, err)
		return
	}
	a.liveInvalidate(userID)
	writeJSON(w, item)
}

func (a *api) publishSocialLive(notifyUserID, notificationType string, invalidate ...string) {
	if a.live == nil {
		return
	}
	if notifyUserID != "" && notificationType != "" {
		a.live.publishLatestNotification(notifyUserID, notificationType)
	}
	a.live.publishInvalidate(invalidate...)
}

func (a *api) liveInvalidate(userIDs ...string) {
	if a.live == nil {
		return
	}
	a.live.publishInvalidate(userIDs...)
}

func queryLimit(r *http.Request, fallback int) int {
	value, err := strconv.Atoi(r.URL.Query().Get("limit"))
	if err != nil || value <= 0 {
		return fallback
	}
	return value
}

func writeJSON(w http.ResponseWriter, value any) {
	writeJSONStatus(w, http.StatusOK, value)
}

func writeJSONStatus(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeSocialStoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrSocialNotFound):
		writeSocialError(w, http.StatusNotFound, "social_action_unavailable")
	case errors.Is(err, ErrSocialBlocked):
		writeSocialError(w, http.StatusForbidden, "social_action_unavailable")
	case errors.Is(err, ErrSocialLimit):
		writeSocialError(w, http.StatusConflict, "social_limit_reached")
	default:
		writeSocialError(w, http.StatusInternalServerError, "social_action_failed")
	}
}

func writeSocialError(w http.ResponseWriter, status int, code string) {
	writeJSONStatus(w, status, map[string]string{"error": code})
}
