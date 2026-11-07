package main

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/labstack/echo/v4"

	"geoduels/internal/chat"
	"geoduels/internal/social"
	"geoduels/pkg/contentfilter"
	"geoduels/pkg/entityid"
	"geoduels/pkg/ratelimit"
	pkgstaff "geoduels/pkg/staff"
)

const (
	chatMaxBodyLen      = 180
	chatRateLimitBurst  = 5
	chatRateLimitWindow = 10 * time.Second
	chatHistorySize     = 100
)

var errConversationForbidden = errors.New("forbidden")

// conversation is one kind of place people talk: a party, a match or two friends. Sending, reading and
// receiving work the same for all of them; only these answers differ.
type conversation interface {
	id() string
	kind() string
	scopeID() string
	// members is everyone a message reaches.
	members(ctx context.Context) ([]string, error)
	// mayRead says whether the viewer can read it, and whether only to read.
	mayRead(ctx context.Context, userID string, staff bool) (allowed, readOnly bool, err error)
	// maySend is nil if the person may write here now.
	maySend(ctx context.Context, userID string) error
	// team is the viewer's team here, if there is one.
	team(ctx context.Context, userID string) (matchID, teamID string, ok bool, err error)
	// pending is what waits for an answer beside the messages.
	pending(ctx context.Context, viewerID string) ([]conversationItem, error)
	// notifies says whether a new message deserves a toast.
	notifies() bool
}

// conversationKinds finds a conversation by its id's prefix.
var conversationKinds = map[string]func(a *api, id string) (conversation, error){
	"party":  func(a *api, id string) (conversation, error) { return partyConversation{a, id}, nil },
	"match":  func(a *api, id string) (conversation, error) { return matchConversation{a, id}, nil },
	"direct": newDirectConversation,
}

func (a *api) findConversation(conversationID string) (conversation, error) {
	// An id holds colons, which clients escape in the path.
	conversationID, err := url.PathUnescape(conversationID)
	if err != nil {
		return nil, errConversationForbidden
	}
	kind, id, ok := strings.Cut(conversationID, ":")
	build, known := conversationKinds[kind]
	if !ok || !known || strings.TrimSpace(id) == "" {
		return nil, errConversationForbidden
	}
	return build(a, id)
}

type partyConversation struct {
	api     *api
	partyID string
}

func (c partyConversation) id() string      { return "party:" + c.partyID }
func (c partyConversation) kind() string    { return "party" }
func (c partyConversation) scopeID() string { return c.partyID }
func (c partyConversation) notifies() bool  { return false }

func (c partyConversation) members(context.Context) ([]string, error) {
	party, found, err := c.api.parties.GetPartyByID(c.partyID)
	if err != nil || !found {
		return nil, err
	}
	return partyMemberIDs(party), nil
}

func (c partyConversation) mayRead(ctx context.Context, userID string, _ bool) (bool, bool, error) {
	party, found, err := c.api.parties.GetPartyByID(c.partyID)
	if err != nil || !found {
		return false, false, err
	}
	return partyHasMember(party, userID), false, nil
}

func (c partyConversation) maySend(ctx context.Context, userID string) error {
	allowed, _, err := c.mayRead(ctx, userID, false)
	if err != nil {
		return err
	}
	if !allowed {
		return errConversationForbidden
	}
	return nil
}

func (c partyConversation) team(_ context.Context, userID string) (string, string, bool, error) {
	return c.api.chat.ActivePartyChatTeam(c.partyID, userID)
}

func (c partyConversation) pending(context.Context, string) ([]conversationItem, error) {
	return nil, nil
}

type matchConversation struct {
	api     *api
	matchID string
}

func (c matchConversation) id() string      { return "match:" + c.matchID }
func (c matchConversation) kind() string    { return "match" }
func (c matchConversation) scopeID() string { return c.matchID }
func (c matchConversation) notifies() bool  { return false }

func (c matchConversation) members(ctx context.Context) ([]string, error) {
	session, found, err := c.api.matchStore.GetSession(ctx, c.matchID)
	if err != nil || !found {
		return nil, err
	}
	out := make([]string, len(session.Seats))
	for i, seat := range session.Seats {
		out[i] = seat.UserID
	}
	return out, nil
}

func (c matchConversation) participant(ctx context.Context, userID string) bool {
	if session, found, err := c.api.matchStore.GetSession(ctx, c.matchID); err == nil && found && session.Seated(userID) {
		return true
	}
	participated, err := c.api.matchStore.PlayerParticipatedInMatch(userID, c.matchID)
	return err == nil && participated
}

func (c matchConversation) mayRead(ctx context.Context, userID string, staff bool) (bool, bool, error) {
	if c.participant(ctx, userID) {
		return true, false, nil
	}
	return staff, staff, nil
}

func (c matchConversation) maySend(ctx context.Context, userID string) error {
	if !c.participant(ctx, userID) {
		return errConversationForbidden
	}
	return nil
}

func (c matchConversation) team(_ context.Context, userID string) (string, string, bool, error) {
	teamID, ok, err := c.api.chat.ChatTeamForMatch(c.matchID, userID)
	return c.matchID, teamID, ok, err
}

func (c matchConversation) pending(context.Context, string) ([]conversationItem, error) {
	return nil, nil
}

// directConversation is between two people, named by its id, so it can't exist twice or have anyone else in it.
type directConversation struct {
	api  *api
	a, b string
}

func newDirectConversation(a *api, id string) (conversation, error) {
	first, second, ok := strings.Cut(id, ":")
	if !ok || first == "" || second == "" || first >= second {
		return nil, errConversationForbidden
	}
	return directConversation{a, first, second}, nil
}

func (c directConversation) id() string      { return chat.DirectID(c.a, c.b) }
func (c directConversation) kind() string    { return "direct" }
func (c directConversation) scopeID() string { return chat.ScopeID(c.id()) }
func (c directConversation) notifies() bool  { return true }

func (c directConversation) members(context.Context) ([]string, error) {
	return []string{c.a, c.b}, nil
}

func (c directConversation) other(userID string) (string, bool) {
	switch userID {
	case c.a:
		return c.b, true
	case c.b:
		return c.a, true
	default:
		return "", false
	}
}

func (c directConversation) mayRead(_ context.Context, userID string, _ bool) (bool, bool, error) {
	_, member := c.other(userID)
	return member, false, nil
}

// maySend holds for friends only; blocking someone ends the friendship.
func (c directConversation) maySend(ctx context.Context, userID string) error {
	other, member := c.other(userID)
	if !member {
		return errConversationForbidden
	}
	state, _, err := c.api.social.Relationship(ctx, userID, other)
	if err != nil {
		return err
	}
	if state != social.RelationshipFriends {
		return errConversationForbidden
	}
	return nil
}

func (c directConversation) team(context.Context, string) (string, string, bool, error) {
	return "", "", false, nil
}

func (c directConversation) pending(ctx context.Context, viewerID string) ([]conversationItem, error) {
	other, member := c.other(viewerID)
	if !member {
		return nil, errConversationForbidden
	}
	state, err := c.api.socialStateFor(ctx, viewerID)
	if err != nil {
		return nil, err
	}
	out := []conversationItem{}
	for _, request := range state.incoming {
		if request.Player.UserID == other {
			out = append(out, incomingRequestItem(viewerID, request))
		}
	}
	for _, request := range state.Outgoing {
		if request.Player.UserID == other {
			out = append(out, outgoingRequestItem(viewerID, request))
		}
	}
	for _, invitation := range state.invitations {
		if invitation.Inviter.UserID == other {
			out = append(out, invitationItem(viewerID, invitation))
		}
	}
	return out, nil
}

// A friend request or party invitation stays in its own table and shows up as an item, carrying what
// can be done about it now.

func requestItemID(requestID string) string       { return "request:" + requestID }
func invitationItemID(invitationID string) string { return "invitation:" + invitationID }

func incomingRequestItem(viewerID string, request social.FriendRequest) conversationItem {
	return conversationItem{
		ID: requestItemID(request.ID), ConversationID: chat.DirectID(viewerID, request.Player.UserID), Kind: "request",
		SenderUserID: request.Player.UserID, SenderDisplayName: request.Player.DisplayName, Audience: string(chat.AudienceAll),
		Body: request.Player.DisplayName + " sent you a friend request", CreatedAt: request.CreatedAt,
		Actions: []itemAction{
			{Label: "Accept", Post: "/api/friend-requests/" + request.ID + "/accept"},
			{Label: "Decline", Post: "/api/friend-requests/" + request.ID + "/decline"},
		},
	}
}

func outgoingRequestItem(viewerID string, request social.FriendRequest) conversationItem {
	return conversationItem{
		ID: requestItemID(request.ID), ConversationID: chat.DirectID(viewerID, request.Player.UserID), Kind: "request",
		SenderUserID: viewerID, Audience: string(chat.AudienceAll),
		Body: "You sent a friend request", CreatedAt: request.CreatedAt,
		Actions: []itemAction{{Label: "Cancel", Post: "/api/friend-requests/" + request.ID + "/cancel"}},
	}
}

func invitationItem(viewerID string, invitation social.PartyInvitation) conversationItem {
	return conversationItem{
		ID: invitationItemID(invitation.ID), ConversationID: chat.DirectID(viewerID, invitation.Inviter.UserID), Kind: "request",
		SenderUserID: invitation.Inviter.UserID, SenderDisplayName: invitation.Inviter.DisplayName, Audience: string(chat.AudienceAll),
		Body: invitation.Inviter.DisplayName + " invited you to a party", CreatedAt: invitation.CreatedAt,
		Actions: []itemAction{
			{Label: "Join", Post: "/api/party-invitations/" + invitation.ID + "/accept", Open: "/party/" + invitation.InviteCode},
			{Label: "Decline", Post: "/api/party-invitations/" + invitation.ID + "/decline"},
		},
	}
}

func newMessageItem(m chat.Message) conversationItem {
	return conversationItem{
		ID: m.ID, ConversationID: m.ConversationID, Kind: string(m.Kind), SenderUserID: m.SenderUserID,
		SenderDisplayName: m.SenderDisplayName, SenderTeamID: m.SenderTeamID, MatchID: m.MatchID,
		Audience: string(m.Audience), TeamID: m.TeamID, Body: m.Body, Emote: string(m.Emote), CreatedAt: m.CreatedAt,
	}
}

// getConversation is a conversation's recent items, oldest first.
func (a *api) getConversation(c echo.Context) error {
	r := c.Request()
	identity, err := a.authenticatedIdentity(r)
	if err != nil {
		return plainTextError(c, http.StatusUnauthorized, "unauthorized")
	}
	found, err := a.findConversation(c.Param("id"))
	if err != nil {
		return plainTextError(c, http.StatusForbidden, "forbidden")
	}
	viewerID := identity.Sub
	staff := identity.StaffActor().Can(pkgstaff.CapReviewReports) || identity.StaffActor().Can(pkgstaff.CapManageAccess)
	allowed, readOnly, err := found.mayRead(r.Context(), viewerID, staff)
	if err != nil {
		return plainTextError(c, http.StatusBadGateway, "chat unavailable")
	}
	if !allowed {
		return plainTextError(c, http.StatusForbidden, "forbidden")
	}
	// Staff reviewers only see team chat once the match is over; live team messages are never revealed.
	revealTeam := false
	if over, err := a.chat.MatchOver(r.Context(), found.scopeID()); readOnly && err == nil && over {
		revealTeam = true
	}
	messages, err := a.chat.ListChatMessagesForUser(found.id(), viewerID, chatHistorySize, revealTeam)
	if err != nil {
		return plainTextError(c, http.StatusBadGateway, "chat unavailable")
	}
	items := make([]conversationItem, 0, len(messages))
	for _, message := range messages {
		items = append(items, newMessageItem(message))
	}
	pending, err := found.pending(r.Context(), viewerID)
	if err != nil {
		return plainTextError(c, http.StatusBadGateway, "chat unavailable")
	}
	items = append(items, pending...)
	sort.SliceStable(items, func(i, j int) bool { return items[i].CreatedAt.Before(items[j].CreatedAt) })
	return writeJSON(c, map[string]any{"conversationId": found.id(), "readOnly": readOnly, "items": items})
}

type sendMessageRequest struct {
	Kind     chat.Kind     `json:"kind"`
	Body     string        `json:"body"`
	Emote    chat.Emote    `json:"emote"`
	Audience chat.Audience `json:"audience"`
}

// sendMessage writes a message and tells everyone in the conversation who may see it.
func (a *api) sendMessage(c echo.Context) error {
	r := c.Request()
	ctx := r.Context()
	identity, err := a.authenticatedIdentity(r)
	if err != nil {
		return plainTextError(c, http.StatusUnauthorized, "unauthorized")
	}
	if identity.IsBanned {
		return writeAPIError(c, http.StatusForbidden, "account_banned", "user is banned")
	}
	userID := identity.Sub
	found, err := a.findConversation(c.Param("id"))
	if err != nil {
		return plainTextError(c, http.StatusForbidden, "forbidden")
	}
	if err := found.maySend(ctx, userID); err != nil {
		return plainTextError(c, http.StatusForbidden, "you can't send messages here")
	}
	restriction, restricted, err := a.chat.GetActiveChatRestriction(userID)
	if err != nil {
		return plainTextError(c, http.StatusBadGateway, "chat unavailable")
	}
	if restricted {
		return plainTextError(c, http.StatusForbidden, chatRestrictionErrorMessage(restriction))
	}
	var req sendMessageRequest
	if err := decodeJSONBody(r, &req); err != nil {
		return plainTextError(c, http.StatusBadRequest, "invalid payload")
	}
	profile, err := a.profiles.GetProfile(userID)
	if err != nil {
		return plainTextError(c, http.StatusBadGateway, "profile unavailable")
	}
	message, err := buildChatMessage(found.id(), userID, profile.DisplayName, req)
	if err != nil {
		return plainTextError(c, http.StatusBadRequest, err.Error())
	}
	matchID, teamID, inTeam, err := found.team(ctx, userID)
	if err != nil {
		return plainTextError(c, http.StatusBadGateway, "chat unavailable")
	}
	if inTeam {
		message.SenderTeamID = teamID
	}
	if message.Audience == chat.AudienceTeam {
		if !inTeam {
			return plainTextError(c, http.StatusBadRequest, "team chat is only available during a team duel")
		}
		message.MatchID, message.TeamID = matchID, teamID
	}
	allowed, _, err := ratelimit.Hit(ctx, a.db.Pool(), "chat:"+found.id()+":"+userID, chatRateLimitBurst, chatRateLimitWindow)
	if err != nil {
		return plainTextError(c, http.StatusBadGateway, "chat unavailable")
	}
	if !allowed {
		return plainTextError(c, http.StatusTooManyRequests, "chat is moving too fast")
	}
	if err := a.chat.RecordChatMessage(found.id(), found.kind(), found.scopeID(), message); err != nil {
		return plainTextError(c, http.StatusBadGateway, "chat unavailable")
	}
	members, err := found.members(ctx)
	if err != nil {
		return plainTextError(c, http.StatusBadGateway, "chat unavailable")
	}
	item := newMessageItem(message)
	for _, member := range members {
		if !a.seesAudience(member, message) {
			continue
		}
		a.tell([]string{member}, itemAdded{Item: item, Notify: found.notifies() && member != userID})
	}
	return writeJSONStatus(c, http.StatusCreated, item)
}

// markConversationRead records how far the person has read.
func (a *api) markConversationRead(c echo.Context) error {
	r := c.Request()
	claims, err := a.authenticatedClaims(r)
	if err != nil {
		return plainTextError(c, http.StatusUnauthorized, "unauthorized")
	}
	found, err := a.findConversation(c.Param("id"))
	if err != nil {
		return plainTextError(c, http.StatusForbidden, "forbidden")
	}
	allowed, readOnly, err := found.mayRead(r.Context(), claims.Sub, false)
	if err != nil || !allowed || readOnly {
		return plainTextError(c, http.StatusForbidden, "forbidden")
	}
	if err := chat.MarkRead(r.Context(), a.db.Pool(), claims.Sub, found.id()); err != nil {
		return plainTextError(c, http.StatusBadGateway, "chat unavailable")
	}
	return c.NoContent(http.StatusNoContent)
}

func (a *api) seesAudience(userID string, message chat.Message) bool {
	if message.Audience != chat.AudienceTeam {
		return true
	}
	teamID, ok, err := a.chat.ChatTeamForMatch(message.MatchID, userID)
	return err == nil && ok && teamID == message.TeamID
}

func buildChatMessage(conversationID, userID, displayName string, req sendMessageRequest) (chat.Message, error) {
	message := chat.Message{
		ID: entityid.New(), ConversationID: conversationID, SenderUserID: userID, Kind: req.Kind,
		SenderDisplayName: strings.TrimSpace(displayName), CreatedAt: time.Now().UTC(), Audience: chat.AudienceAll,
	}
	if message.SenderDisplayName == "" {
		message.SenderDisplayName = userID
	}
	switch req.Audience {
	case "", chat.AudienceAll:
	case chat.AudienceTeam:
		message.Audience = chat.AudienceTeam
	default:
		return chat.Message{}, errors.New("unsupported chat audience")
	}
	switch req.Kind {
	case chat.KindText:
		message.Body = sanitizeChatBody(req.Body)
		if message.Body == "" {
			return chat.Message{}, errors.New("message is empty")
		}
		if err := contentfilter.RejectAbusiveText(message.Body); err != nil {
			return chat.Message{}, err
		}
	case chat.KindEmote:
		message.Emote = chat.Emote(strings.TrimSpace(string(req.Emote)))
		if !validChatEmote(message.Emote) {
			return chat.Message{}, errors.New("unsupported emote")
		}
	default:
		return chat.Message{}, errors.New("unsupported message")
	}
	return message, nil
}

func chatRestrictionErrorMessage(restriction chat.ChatRestriction) string {
	switch restriction.ActionType {
	case "temporary_ban", "permanent_ban":
		if restriction.EndsAt.IsZero() {
			return "your account is banned"
		}
		return "your account is banned until " + restriction.EndsAt.UTC().Format(time.RFC3339)
	default:
		if restriction.EndsAt.IsZero() {
			return "chat access is restricted"
		}
		return "chat access is restricted until " + restriction.EndsAt.UTC().Format(time.RFC3339)
	}
}

func sanitizeChatBody(body string) string {
	return contentfilter.NormalizeText(body, chatMaxBodyLen)
}

func validChatEmote(emote chat.Emote) bool {
	switch emote {
	case chat.EmoteSkull, chat.EmoteSob, chat.EmoteThinking, chat.EmoteSunglasses, chat.EmoteWave:
		return true
	default:
		return false
	}
}
