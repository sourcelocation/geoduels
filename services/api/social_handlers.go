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
	"geoduels/pkg/persistence"
	"geoduels/pkg/socialpolicy"
)

func (a *api) socialStore() (persistence.SocialRepository, bool) {
	store, ok := a.store.(persistence.SocialRepository)
	return store, ok
}

func (a *api) socialActor(r *http.Request) (string, persistence.SocialRepository, bool) {
	claims, err := a.authenticatedClaims(r)
	if err != nil {
		return "", nil, false
	}
	store, ok := a.socialStore()
	if !ok {
		return "", nil, false
	}
	isGuest, _, _, err := store.GetSocialAccount(claims.Sub)
	if err != nil || socialpolicy.Authorize(socialpolicy.Account{
		IsGuest: isGuest, ActionEnabled: true, TargetExists: true,
	}) != nil {
		return "", nil, false
	}
	return claims.Sub, store, true
}

func (a *api) friends(w http.ResponseWriter, r *http.Request) {
	userID, store, ok := a.socialActor(r)
	if !ok {
		writeSocialError(w, http.StatusUnauthorized, "registration_required")
		return
	}
	friends, err := store.ListFriends(userID, queryLimit(r, 100))
	if err != nil {
		writeSocialError(w, http.StatusInternalServerError, "friends_unavailable")
		return
	}
	a.applySocialPresence(friends)
	writeJSON(w, map[string]any{"friends": friends})
}

func (a *api) socialSettings(w http.ResponseWriter, r *http.Request) {
	userID, store, ok := a.socialActor(r)
	if !ok {
		writeSocialError(w, http.StatusUnauthorized, "registration_required")
		return
	}
	if r.Method == http.MethodGet {
		settings, err := store.GetSocialSettings(userID)
		if err != nil {
			writeSocialStoreError(w, err)
			return
		}
		writeJSON(w, settings)
		return
	}
	var settings persistence.SocialSettings
	if json.NewDecoder(r.Body).Decode(&settings) != nil {
		writeSocialError(w, http.StatusBadRequest, "invalid_request")
		return
	}
	settings, err := store.UpdateSocialSettings(userID, settings)
	if err != nil {
		writeSocialStoreError(w, err)
		return
	}
	writeJSON(w, settings)
}

func (a *api) friendRequests(w http.ResponseWriter, r *http.Request) {
	userID, store, ok := a.socialActor(r)
	if !ok {
		writeSocialError(w, http.StatusUnauthorized, "registration_required")
		return
	}
	direction := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("direction")))
	if direction != "outgoing" {
		direction = "incoming"
	}
	requests, err := store.ListFriendRequests(userID, direction, queryLimit(r, 20))
	if err != nil {
		writeSocialError(w, http.StatusInternalServerError, "requests_unavailable")
		return
	}
	writeJSON(w, map[string]any{"requests": requests})
}

func (a *api) sendFriendRequest(w http.ResponseWriter, r *http.Request) {
	userID, store, ok := a.socialActor(r)
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
	item, err := store.SendFriendRequest(userID, strings.TrimSpace(body.UserID))
	if err != nil {
		writeSocialStoreError(w, err)
		return
	}
	writeJSONStatus(w, http.StatusCreated, item)
}

func (a *api) respondFriendRequest(w http.ResponseWriter, r *http.Request) {
	userID, store, ok := a.socialActor(r)
	if !ok {
		writeSocialError(w, http.StatusUnauthorized, "registration_required")
		return
	}
	action := mux.Vars(r)["action"]
	if action != "accept" && action != "decline" && action != "cancel" {
		writeSocialError(w, http.StatusBadRequest, "invalid_action")
		return
	}
	if err := store.RespondFriendRequest(userID, mux.Vars(r)["id"], action); err != nil {
		writeSocialStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) removeFriend(w http.ResponseWriter, r *http.Request) {
	userID, store, ok := a.socialActor(r)
	if !ok {
		writeSocialError(w, http.StatusUnauthorized, "registration_required")
		return
	}
	if err := store.RemoveFriend(userID, mux.Vars(r)["userId"]); err != nil {
		writeSocialStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) userBlock(w http.ResponseWriter, r *http.Request) {
	userID, store, ok := a.socialActor(r)
	if !ok {
		writeSocialError(w, http.StatusUnauthorized, "registration_required")
		return
	}
	if err := store.SetUserBlock(userID, mux.Vars(r)["userId"], r.Method == http.MethodPost); err != nil {
		writeSocialStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) socialPlayerSearch(w http.ResponseWriter, r *http.Request) {
	userID, store, ok := a.socialActor(r)
	if !ok {
		writeSocialError(w, http.StatusUnauthorized, "registration_required")
		return
	}
	if allowed, retry, err := a.allowSocialAction(r, userID, "player_search"); err != nil || !allowed {
		writeSocialRateLimited(w, retry)
		return
	}
	players, err := store.SearchSocialPlayers(userID, r.URL.Query().Get("q"), queryLimit(r, 10))
	if err != nil {
		writeSocialError(w, http.StatusInternalServerError, "search_unavailable")
		return
	}
	a.applySocialPresence(players)
	writeJSON(w, map[string]any{"players": players})
}

func (a *api) recentPlayers(w http.ResponseWriter, r *http.Request) {
	userID, store, ok := a.socialActor(r)
	if !ok {
		writeSocialError(w, http.StatusUnauthorized, "registration_required")
		return
	}
	players, err := store.ListRecentPlayers(userID, 3)
	if err != nil {
		writeSocialError(w, http.StatusInternalServerError, "recent_players_unavailable")
		return
	}
	a.applySocialPresence(players)
	writeJSON(w, map[string]any{"players": players})
}

func (a *api) playerRelationship(w http.ResponseWriter, r *http.Request) {
	userID, store, ok := a.socialActor(r)
	if !ok {
		writeSocialError(w, http.StatusUnauthorized, "registration_required")
		return
	}
	profile, err := a.store.GetPublicPlayerProfileByNickname(mux.Vars(r)["nickname"])
	if err != nil {
		writeSocialError(w, http.StatusNotFound, "player_not_found")
		return
	}
	state, requestID, err := store.Relationship(userID, profile.UserID)
	if err != nil {
		writeSocialError(w, http.StatusInternalServerError, "relationship_unavailable")
		return
	}
	writeJSON(w, map[string]any{"state": state, "requestId": requestID})
}

func (a *api) createFriendCode(w http.ResponseWriter, r *http.Request) {
	userID, store, ok := a.socialActor(r)
	if !ok {
		writeSocialError(w, http.StatusUnauthorized, "registration_required")
		return
	}
	code, err := store.CreateFriendCode(userID, 7*24*time.Hour)
	if err != nil {
		writeSocialStoreError(w, err)
		return
	}
	writeJSONStatus(w, http.StatusCreated, code)
}

func (a *api) resolveFriendCode(w http.ResponseWriter, r *http.Request) {
	userID, store, ok := a.socialActor(r)
	if !ok {
		writeSocialError(w, http.StatusUnauthorized, "registration_required")
		return
	}
	if allowed, retry, err := a.allowSocialAction(r, userID, "code_resolve"); err != nil || !allowed {
		writeSocialRateLimited(w, retry)
		return
	}
	player, err := store.ResolveFriendCode(userID, mux.Vars(r)["code"])
	if err != nil {
		writeSocialStoreError(w, err)
		return
	}
	writeJSON(w, player)
}

func (a *api) sendFriendCodeRequest(w http.ResponseWriter, r *http.Request) {
	userID, store, ok := a.socialActor(r)
	if !ok {
		writeSocialError(w, http.StatusUnauthorized, "registration_required")
		return
	}
	player, err := store.ResolveFriendCode(userID, mux.Vars(r)["code"])
	if err == nil {
		_, err = store.SendFriendRequest(userID, player.UserID)
	}
	if err != nil {
		writeSocialStoreError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (a *api) partyInvitations(w http.ResponseWriter, r *http.Request) {
	userID, store, ok := a.socialActor(r)
	if !ok {
		writeSocialError(w, http.StatusUnauthorized, "registration_required")
		return
	}
	if r.Method == http.MethodGet {
		items, err := store.ListPartyInvitations(userID, queryLimit(r, 10))
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
	item, err := store.CreatePartyInvitation(mux.Vars(r)["id"], userID, body.UserID, 20*time.Minute)
	if err != nil {
		writeSocialStoreError(w, err)
		return
	}
	writeJSONStatus(w, http.StatusCreated, item)
}

func (a *api) createPartyAndInvite(w http.ResponseWriter, r *http.Request) {
	userID, store, ok := a.socialActor(r)
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
	party, err := a.store.CreateParty(userID, contracts.ModeDuel, "world", 2*time.Hour)
	if err != nil {
		writeSocialError(w, http.StatusInternalServerError, "party_unavailable")
		return
	}
	invitation, err := store.CreatePartyInvitation(party.ID, userID, body.UserID, 20*time.Minute)
	if err != nil {
		_, _ = a.store.LeaveParty(party.ID, userID)
		writeSocialStoreError(w, err)
		return
	}
	writeJSONStatus(w, http.StatusCreated, map[string]any{
		"invitation": invitation,
		"party":      party,
	})
}

func (a *api) respondPartyInvitation(w http.ResponseWriter, r *http.Request) {
	userID, store, ok := a.socialActor(r)
	if !ok {
		writeSocialError(w, http.StatusUnauthorized, "registration_required")
		return
	}
	action := mux.Vars(r)["action"]
	if action != "accept" && action != "decline" {
		writeSocialError(w, http.StatusBadRequest, "invalid_action")
		return
	}
	item, err := store.RespondPartyInvitation(userID, mux.Vars(r)["id"], action)
	if err != nil {
		writeSocialStoreError(w, err)
		return
	}
	writeJSON(w, item)
}

func (a *api) socialSummary(w http.ResponseWriter, r *http.Request) {
	userID, store, ok := a.socialActor(r)
	if !ok {
		writeSocialError(w, http.StatusUnauthorized, "registration_required")
		return
	}
	incoming, err1 := store.ListFriendRequests(userID, "incoming", 3)
	outgoing, err2 := store.ListFriendRequests(userID, "outgoing", 3)
	invites, err3 := store.ListPartyInvitations(userID, 3)
	notifications, err4 := a.store.ListUserNotifications(userID, 5)
	if err := errors.Join(err1, err2, err3, err4); err != nil {
		writeSocialError(w, http.StatusInternalServerError, "social_summary_unavailable")
		return
	}
	recent := notifications[:0]
	for _, notification := range notifications {
		if notification.Type != "friend_request_received" && notification.Type != "party_invitation_received" {
			recent = append(recent, notification)
		}
	}
	writeJSON(w, map[string]any{
		"incomingRequests": incoming, "outgoingRequests": outgoing,
		"partyInvitations": invites, "notifications": recent,
		"unreadCount": len(recent) + len(incoming) + len(invites),
	})
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
	case errors.Is(err, persistence.ErrSocialNotFound):
		writeSocialError(w, http.StatusNotFound, "social_action_unavailable")
	case errors.Is(err, persistence.ErrSocialBlocked):
		writeSocialError(w, http.StatusForbidden, "social_action_unavailable")
	case errors.Is(err, persistence.ErrSocialLimit):
		writeSocialError(w, http.StatusConflict, "social_limit_reached")
	default:
		writeSocialError(w, http.StatusInternalServerError, "social_action_failed")
	}
}

func writeSocialError(w http.ResponseWriter, status int, code string) {
	writeJSONStatus(w, status, map[string]string{"error": code})
}
