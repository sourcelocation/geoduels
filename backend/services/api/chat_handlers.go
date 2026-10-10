package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v4"

	"geoduels/internal/chat"
	"geoduels/pkg/contentfilter"
	"geoduels/pkg/contracts"
	"geoduels/pkg/entityid"
	"geoduels/pkg/observability"
	"geoduels/pkg/pgnotify"
	pkgstaff "geoduels/pkg/staff"
)

const (
	chatMaxBodyLen      = 180
	chatRateLimitBurst  = 5
	chatRateLimitWindow = 10 * time.Second
)

type chatScope struct {
	ConversationID string
	Kind           string
	ID             string
	MatchID        string
	// ReadOnly is set for staff reviewers who are not participants: they may
	// read (team chat only after the match ended) but never send.
	ReadOnly bool
}

type chatClientCommand struct {
	Type    string         `json:"type"`
	Payload map[string]any `json:"payload"`
}

func (a *api) chatWS(c echo.Context) error {
	r := c.Request()
	claims, err := a.liveClaims(r)
	if err != nil {
		return plainTextError(c, http.StatusUnauthorized, "unauthorized")
	}
	identity, err := a.accounts.GetIdentity(claims.Sub)
	if err != nil {
		return plainTextError(c, http.StatusUnauthorized, "identity not found")
	}
	if identity.IsBanned {
		return writeAPIError(c, http.StatusForbidden, "account_banned", "user is banned")
	}
	staffViewer := identity.StaffActor().Can(pkgstaff.CapReviewReports) || identity.StaffActor().Can(pkgstaff.CapManageAccess)
	scope, err := a.authorizeChatConversation(r.Context(), strings.TrimSpace(c.QueryParam("conversationId")), claims.Sub, staffViewer)
	if err != nil {
		return plainTextError(c, http.StatusForbidden, "forbidden")
	}
	profile, err := a.profiles.GetProfile(claims.Sub)
	if err != nil {
		return plainTextError(c, http.StatusBadGateway, "profile unavailable")
	}
	conn, err := a.live.upgrader.Upgrade(c.Response().Writer, r, nil)
	if err != nil {
		return nil
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	conn.SetReadLimit(2048)
	_ = conn.SetReadDeadline(time.Now().Add(70 * time.Second))
	conn.SetPongHandler(func(string) error {
		return conn.SetReadDeadline(time.Now().Add(70 * time.Second))
	})

	w := &socketWriter{conn: conn}
	// Staff reviewers only see team chat once the match is over; live team
	// messages are never revealed.
	revealTeam := false
	if scope.ReadOnly {
		if over, overErr := a.chat.MatchOver(r.Context(), scope.MatchID); overErr == nil && over {
			revealTeam = true
		}
	}
	if messages, err := a.chat.ListChatMessagesForUser(scope.ConversationID, claims.Sub, 100, revealTeam); err == nil && len(messages) > 0 {
		w.send("chat.history", map[string]any{
			"conversationId": scope.ConversationID,
			"messages":       messages,
			"readOnly":       scope.ReadOnly,
		})
	}

	var chatEvents <-chan pgnotify.Event
	if !scope.ReadOnly {
		events, unsubscribe := a.events.Subscribe(pgnotify.ChatTopic(scope.ConversationID))
		defer unsubscribe()
		chatEvents = events
	}

	go func() {
		defer cancel()
		for {
			var cmd chatClientCommand
			if err := conn.ReadJSON(&cmd); err != nil {
				return
			}
			_ = conn.SetReadDeadline(time.Now().Add(70 * time.Second))
			if scope.ReadOnly {
				w.send("chat.error", map[string]string{"message": "chat is read-only for reviewers"})
				continue
			}
			restriction, restricted, err := a.chat.GetActiveChatRestriction(claims.Sub)
			if err != nil {
				observability.Log("warn", "chat restriction lookup failed", map[string]any{
					"conversationId": scope.ConversationID,
					"userId":         claims.Sub,
					"error":          err.Error(),
				})
				w.send("chat.error", map[string]string{"message": "chat unavailable"})
				continue
			}
			if restricted {
				w.send("chat.error", map[string]string{"message": chatRestrictionErrorMessage(restriction)})
				continue
			}
			message, err := a.buildChatMessage(scope, claims.Sub, profile.DisplayName, cmd)
			if err != nil {
				w.send("chat.error", map[string]string{"message": err.Error()})
				continue
			}
			if message.Audience == contracts.ChatAudienceTeam {
				matchID, teamID, ok, teamErr := a.resolveChatTeam(scope, claims.Sub)
				if teamErr != nil {
					w.send("chat.error", map[string]string{"message": "chat unavailable"})
					continue
				}
				if !ok {
					w.send("chat.error", map[string]string{"message": "team chat is only available during a team duel"})
					continue
				}
				message.MatchID, message.TeamID, message.SenderTeamID = matchID, teamID, teamID
			}
			if message.Audience == contracts.ChatAudienceAll {
				_, teamID, ok, err := a.resolveChatTeam(scope, claims.Sub)
				if err != nil {
					w.send("chat.error", map[string]string{"message": "chat unavailable"})
					continue
				}
				if ok {
					message.SenderTeamID = teamID
				}
			}
			if !a.chatLimits.allow(scope.ConversationID, claims.Sub, time.Now()) {
				w.send("chat.error", map[string]string{"message": "chat is moving too fast"})
				continue
			}
			if err := a.chat.RecordChatMessage(scope.ConversationID, scope.Kind, scope.ID, message); err != nil {
				observability.Log("error", "chat message persistence failed", map[string]any{
					"conversationId": scope.ConversationID,
					"scopeKind":      scope.Kind,
					"scopeId":        scope.ID,
					"userId":         claims.Sub,
					"error":          err.Error(),
				})
				w.send("chat.error", map[string]string{"message": "chat unavailable"})
				continue
			}
			a.publishChatMessage(ctx, scope.ConversationID, message, w)
		}
	}()

	pingTicker := time.NewTicker(20 * time.Second)
	defer pingTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-chatEvents:
			if !ok {
				return nil
			}
			var message contracts.ChatMessage
			if event.Resync || json.Unmarshal(event.Data, &message) != nil {
				continue
			}
			if a.canViewChatMessage(claims.Sub, message) {
				w.send(contracts.EventChatMessage, message)
			}
		case <-pingTicker.C:
			if !w.ping() {
				return nil
			}
		}
	}
}

func (a *api) authorizeChatConversation(ctx context.Context, conversationID, userID string, staffViewer bool) (chatScope, error) {
	kind, id, ok := strings.Cut(conversationID, ":")
	if !ok || strings.TrimSpace(id) == "" {
		return chatScope{}, errors.New("invalid conversation")
	}
	scope := chatScope{ConversationID: conversationID, Kind: kind, ID: id}
	switch kind {
	case "party":
		snap, found, err := a.parties.GetPartyByID(id)
		if err != nil || !found || !partyHasMember(snap, userID) {
			return chatScope{}, errors.New("forbidden")
		}
		return scope, nil
	case "match":
		scope.MatchID = id
		if session, found, err := a.matchStore.GetSession(ctx, id); err == nil && found && session.Seated(userID) {
			return scope, nil
		}
		if participated, err := a.matchStore.PlayerParticipatedInMatch(userID, id); err == nil && participated {
			return scope, nil
		}
		if staffViewer {
			scope.ReadOnly = true
			return scope, nil
		}
		return chatScope{}, errors.New("forbidden")
	default:
		return chatScope{}, errors.New("invalid conversation")
	}
}

func (a *api) buildChatMessage(scope chatScope, userID, displayName string, cmd chatClientCommand) (contracts.ChatMessage, error) {
	message := contracts.ChatMessage{
		ID:                entityid.New(),
		ConversationID:    scope.ConversationID,
		MatchID:           scope.MatchID,
		SenderUserID:      userID,
		SenderDisplayName: strings.TrimSpace(displayName),
		CreatedAt:         time.Now().UTC(),
		Audience:          contracts.ChatAudienceAll,
	}
	if cmd.Payload != nil {
		if rawAudience, exists := cmd.Payload["audience"]; exists {
			audience, ok := rawAudience.(string)
			if !ok {
				return contracts.ChatMessage{}, errors.New("unsupported chat audience")
			}
			switch contracts.ChatAudience(strings.TrimSpace(audience)) {
			case contracts.ChatAudienceAll:
			case contracts.ChatAudienceTeam:
				message.Audience = contracts.ChatAudienceTeam
			default:
				return contracts.ChatMessage{}, errors.New("unsupported chat audience")
			}
		}
	}
	if message.SenderDisplayName == "" {
		message.SenderDisplayName = userID
	}
	switch cmd.Type {
	case "chat.send":
		message.Kind = contracts.ChatMessageText
		if cmd.Payload != nil {
			if body, ok := cmd.Payload["body"].(string); ok {
				message.Body = sanitizeChatBody(body)
			}
		}
		if message.Body == "" {
			return contracts.ChatMessage{}, errors.New("message is empty")
		}
		if err := contentfilter.RejectAbusiveText(message.Body); err != nil {
			return contracts.ChatMessage{}, err
		}
	case "chat.emote":
		message.Kind = contracts.ChatMessageEmote
		if cmd.Payload != nil {
			if emote, ok := cmd.Payload["emote"].(string); ok {
				message.Emote = contracts.ChatEmote(strings.TrimSpace(emote))
			}
		}
		if !validChatEmote(message.Emote) {
			return contracts.ChatMessage{}, errors.New("unsupported emote")
		}
	default:
		return contracts.ChatMessage{}, errors.New("unsupported chat command")
	}
	return message, nil
}

func (a *api) resolveChatTeam(scope chatScope, userID string) (string, string, bool, error) {
	if scope.Kind == "party" {
		return a.chat.ActivePartyChatTeam(scope.ID, userID)
	}
	if scope.Kind == "match" {
		teamID, ok, err := a.chat.ChatTeamForMatch(scope.MatchID, userID)
		return scope.MatchID, teamID, ok, err
	}
	return "", "", false, nil
}

func (a *api) canViewChatMessage(userID string, message contracts.ChatMessage) bool {
	if message.Audience != contracts.ChatAudienceTeam {
		return true
	}
	teamID, ok, err := a.chat.ChatTeamForMatch(message.MatchID, userID)
	return err == nil && ok && teamID == message.TeamID
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

func validChatEmote(emote contracts.ChatEmote) bool {
	switch emote {
	case contracts.ChatEmoteSkull, contracts.ChatEmoteSob, contracts.ChatEmoteThinking, contracts.ChatEmoteSunglasses, contracts.ChatEmoteWave:
		return true
	default:
		return false
	}
}

// chatRateLimiter allows a few messages per conversation and player in a short window. Each process
// counts its own sockets; a player's chat socket lives in one of them.
type chatRateLimiter struct {
	mu     sync.Mutex
	recent map[string][]time.Time
}

func (l *chatRateLimiter) allow(conversationID, userID string, now time.Time) bool {
	key := conversationID + ":" + userID
	cutoff := now.Add(-chatRateLimitWindow)
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.recent == nil {
		l.recent = map[string][]time.Time{}
	}
	recent := l.recent[key]
	kept := recent[:0]
	for _, ts := range recent {
		if ts.After(cutoff) {
			kept = append(kept, ts)
		}
	}
	if len(kept) >= chatRateLimitBurst {
		l.recent[key] = kept
		return false
	}
	l.recent[key] = append(kept, now)
	return true
}

// publishChatMessage sends a message to everyone in the conversation, in every process; it falls
// back to the sender alone if it cannot be published.
func (a *api) publishChatMessage(ctx context.Context, conversationID string, message contracts.ChatMessage, sender *socketWriter) {
	if err := pgnotify.Publish(ctx, a.db.Pool(), pgnotify.ChatTopic(conversationID), message); err != nil {
		observability.Log("warn", "chat publish failed", map[string]any{"conversationId": conversationID, "error": err.Error()})
		sender.send(contracts.EventChatMessage, message)
	}
}
