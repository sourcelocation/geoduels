package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"geoduels/internal/chat"
	"geoduels/internal/presence"
	"geoduels/internal/social"
	"geoduels/pkg/matchkind"
	"geoduels/pkg/observability"
)

// friendsPage is what only the friends page needs: recent players, and who was invited to a party.
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
	friends, recent := result.Friends, result.Recent
	attachPartyInvites(friends, result.PartyInvites)
	a.applySocialPresence(r.Context(), friends)
	a.applySocialPresence(r.Context(), recent)
	return writeJSON(c, map[string]any{"friends": friends, "recentPlayers": recent})
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

func (a *api) socialActor(r *http.Request) (string, *social.Service, bool) {
	claims, err := a.authenticatedClaims(r)
	if err != nil {
		return "", nil, false
	}
	if err := a.social.Authorize(r.Context(), claims.Sub); err != nil {
		return "", nil, false
	}
	return claims.Sub, a.social, true
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
	a.tellFriendRequest(r.Context(), userID, targetID)
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
	other, err := service.FriendRequestOther(r.Context(), userID, requestID)
	if err != nil {
		return writeSocialStoreError(c, err)
	}
	if err := service.RespondFriendRequest(r.Context(), userID, requestID, action); err != nil {
		return writeSocialStoreError(c, err)
	}
	a.tell([]string{userID, other}, itemRemoved{ConversationID: chat.DirectID(userID, other), ItemID: requestItemID(requestID)})
	a.tellSocial(r.Context(), userID, other)
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
	a.tellSocial(r.Context(), userID, targetID)
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
	a.tellSocial(r.Context(), userID, targetID)
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
	a.tellFriendRequest(r.Context(), userID, player.UserID)
	return c.NoContent(http.StatusNoContent)
}

func (a *api) partyInvitations(c echo.Context) error {
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
	item, err := service.CreatePartyInvitation(r.Context(), c.Param("id"), userID, body.UserID, 20*time.Minute)
	if err != nil {
		return writeSocialStoreError(c, err)
	}
	a.tellInvitation(r.Context(), userID, body.UserID)
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
	party, err := a.parties.CreateParty(userID, matchkind.ModeDuel, "world", 2*time.Hour)
	if err != nil {
		return writeSocialError(c, http.StatusInternalServerError, "party_unavailable")
	}
	invitation, err := service.CreatePartyInvitation(r.Context(), party.ID, userID, body.UserID, social.DefaultPartyInviteTTL)
	if err != nil {
		_, _ = a.parties.LeaveParty(party.ID, userID)
		return writeSocialStoreError(c, err)
	}
	a.tellParty(r.Context(), party.ID)
	a.tellInvitation(r.Context(), userID, body.UserID)
	return writeJSONStatus(c, http.StatusCreated, map[string]any{
		"invitation": invitation,
		"party":      newPartySnapshot(party),
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
	if item.Inviter.UserID != "" {
		a.tell([]string{userID}, itemRemoved{ConversationID: chat.DirectID(userID, item.Inviter.UserID), ItemID: invitationItemID(item.ID)})
	}
	a.tellSocial(r.Context(), userID)
	return writeJSON(c, item)
}

func emptySocialState() socialState {
	return socialState{
		Friends: []friendView{}, Outgoing: []social.FriendRequest{}, Pending: []pendingItem{},
	}
}

// socialStateFor is everything about a person's friends. A guest has none, and needs no special case.
func (a *api) socialStateFor(ctx context.Context, userID string) (socialState, error) {
	if err := a.social.Authorize(ctx, userID); err != nil {
		return emptySocialState(), nil
	}
	page, err := a.social.FriendsPage(ctx, userID, "")
	if err != nil {
		return socialState{}, err
	}
	invitations, err := a.social.ListPartyInvitations(ctx, userID, 20)
	if err != nil {
		return socialState{}, err
	}
	a.applySocialPresence(ctx, page.Friends)
	ids := make([]string, len(page.Friends))
	for i, friend := range page.Friends {
		ids[i] = chat.DirectID(userID, friend.UserID)
	}
	summaries, err := chat.Summaries(ctx, a.db.Pool(), userID, ids)
	if err != nil {
		return socialState{}, err
	}
	state := socialState{
		Friends: make([]friendView, len(page.Friends)), Outgoing: page.Outgoing, Pending: make([]pendingItem, 0, len(page.Incoming)+len(invitations)),
		incoming: page.Incoming, invitations: invitations,
	}
	for _, request := range page.Incoming {
		state.Pending = append(state.Pending, pendingItem{Item: incomingRequestItem(userID, request), Player: request.Player})
	}
	for _, invitation := range invitations {
		state.Pending = append(state.Pending, pendingItem{Item: invitationItem(userID, invitation), Player: invitation.Inviter})
	}
	for i, friend := range page.Friends {
		state.Friends[i] = friendView{CompactPlayer: friend, ConversationID: ids[i]}
		if summary, ok := summaries[ids[i]]; ok {
			state.Friends[i].Unread = summary.Unread
			state.Friends[i].LastMessageAt = &summary.LastMessageAt
		}
	}
	return state, nil
}

// tellSocial sends each person their friends as they now are.
func (a *api) tellSocial(ctx context.Context, userIDs ...string) {
	for _, userID := range userIDs {
		if userID == "" {
			continue
		}
		state, err := a.socialStateFor(ctx, userID)
		if err != nil {
			observability.Log("warn", "social state failed", map[string]any{"userId": userID, "error": err.Error()})
			continue
		}
		a.tell([]string{userID}, state)
	}
}

// tellFriendRequest puts a new request in the conversation between them, with a toast for whoever it's for.
func (a *api) tellFriendRequest(ctx context.Context, senderID, recipientID string) {
	a.tellSocial(ctx, senderID, recipientID)
	if state, err := a.socialStateFor(ctx, recipientID); err == nil {
		for _, request := range state.incoming {
			if request.Player.UserID == senderID {
				a.tell([]string{recipientID}, itemAdded{Item: incomingRequestItem(recipientID, request), Notify: true})
			}
		}
	}
	if state, err := a.socialStateFor(ctx, senderID); err == nil {
		for _, request := range state.Outgoing {
			if request.Player.UserID == recipientID {
				a.tell([]string{senderID}, itemAdded{Item: outgoingRequestItem(senderID, request)})
			}
		}
	}
}

// tellInvitation puts a new party invitation in the conversation between them, with a toast.
func (a *api) tellInvitation(ctx context.Context, inviterID, recipientID string) {
	a.tellSocial(ctx, recipientID)
	state, err := a.socialStateFor(ctx, recipientID)
	if err != nil {
		return
	}
	for _, invitation := range state.invitations {
		if invitation.Inviter.UserID == inviterID {
			a.tell([]string{recipientID}, itemAdded{Item: invitationItem(recipientID, invitation), Notify: true})
		}
	}
}

// applySocialPresence fills in who is connected and who is playing.
func (a *api) applySocialPresence(ctx context.Context, players []social.CompactPlayer) {
	ids := make([]string, 0, len(players))
	for _, player := range players {
		if player.LastSeenAt != nil {
			ids = append(ids, player.UserID)
		}
	}
	if len(ids) == 0 {
		return
	}
	online, err := presence.Online(ctx, a.db.Pool(), ids)
	if err != nil {
		return
	}
	playing, err := a.matchStore.UsersInLiveMatches(ctx, ids)
	if err != nil {
		playing = map[string]bool{}
	}
	for i := range players {
		if players[i].LastSeenAt == nil {
			continue
		}
		if !online[players[i].UserID] {
			players[i].PresenceStatus = "offline"
			continue
		}
		players[i].PresenceStatus = "online"
		if playing[players[i].UserID] {
			players[i].Activity = "in_match"
		}
	}
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
