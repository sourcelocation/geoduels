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
	"github.com/redis/go-redis/v9"

	"geoduels/internal/httpx"
	"geoduels/pkg/contentfilter"
	"geoduels/pkg/contracts"
	"geoduels/pkg/entityid"
	"geoduels/pkg/observability"
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

func (q *matchCoordinator) chatWS(c echo.Context) error {
	r := c.Request()
	claims, identity, err := q.requireActiveAccount(c)
	if err != nil {
		return err
	}
	staffViewer := identity.StaffActor().Can(pkgstaff.CapReviewReports) || identity.StaffActor().Can(pkgstaff.CapManageAccess)
	scope, err := q.authorizeChatConversation(r.Context(), strings.TrimSpace(c.QueryParam("conversationId")), claims.Sub, staffViewer)
	if err != nil {
		return httpx.PlainTextError(c, http.StatusForbidden, "forbidden")
	}
	profile, err := q.profiles.GetProfile(claims.Sub)
	if err != nil {
		return httpx.PlainTextError(c, http.StatusBadGateway, "profile unavailable")
	}
	conn, err := partyUpgrader.Upgrade(c.Response().Writer, r, nil)
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

	var writeMu sync.Mutex
	// Staff reviewers only see team chat once the match is over; live team
	// messages are never revealed.
	revealTeam := false
	if scope.ReadOnly {
		if over, overErr := q.chat.MatchOver(r.Context(), scope.MatchID); overErr == nil && over {
			revealTeam = true
		}
	}
	if messages, err := q.chat.ListChatMessagesForUser(scope.ConversationID, claims.Sub, 100, revealTeam); err == nil && len(messages) > 0 {
		q.writeQueueMessage(conn, &writeMu, "chat.history", map[string]any{
			"conversationId": scope.ConversationID,
			"messages":       messages,
			"readOnly":       scope.ReadOnly,
		})
	}

	var chatEvents <-chan *redis.Message
	if q.redis != nil && !scope.ReadOnly {
		pubsub := q.redis.Subscribe(ctx, chatChannel(scope.ConversationID))
		defer pubsub.Close()
		if _, err := pubsub.Receive(ctx); err != nil {
			observability.Log("warn", "chat subscribe failed", map[string]any{"conversationId": scope.ConversationID, "error": err.Error()})
		} else {
			chatEvents = pubsub.Channel()
		}
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
				q.writeQueueMessage(conn, &writeMu, "chat.error", map[string]string{"message": "chat is read-only for reviewers"})
				continue
			}
			restriction, restricted, err := q.chat.GetActiveChatRestriction(claims.Sub)
			if err != nil {
				observability.Log("warn", "chat restriction lookup failed", map[string]any{
					"conversationId": scope.ConversationID,
					"userId":         claims.Sub,
					"error":          err.Error(),
				})
				q.writeQueueMessage(conn, &writeMu, "chat.error", map[string]string{"message": "chat unavailable"})
				continue
			}
			if restricted {
				q.writeQueueMessage(conn, &writeMu, "chat.error", map[string]string{"message": chatRestrictionErrorMessage(restriction)})
				continue
			}
			message, err := q.buildCoordinatorChatMessage(scope, claims.Sub, profile.DisplayName, cmd)
			if err != nil {
				q.writeQueueMessage(conn, &writeMu, "chat.error", map[string]string{"message": err.Error()})
				continue
			}
			if message.Audience == contracts.ChatAudienceTeam {
				matchID, teamID, ok, teamErr := q.resolveChatTeam(scope, claims.Sub)
				if teamErr != nil {
					q.writeQueueMessage(conn, &writeMu, "chat.error", map[string]string{"message": "chat unavailable"})
					continue
				}
				if !ok {
					q.writeQueueMessage(conn, &writeMu, "chat.error", map[string]string{"message": "team chat is only available during a team duel"})
					continue
				}
				message.MatchID, message.TeamID, message.SenderTeamID = matchID, teamID, teamID
			}
			if message.Audience == contracts.ChatAudienceAll {
				_, teamID, ok, err := q.resolveChatTeam(scope, claims.Sub)
				if err != nil {
					q.writeQueueMessage(conn, &writeMu, "chat.error", map[string]string{"message": "chat unavailable"})
					continue
				}
				if ok {
					message.SenderTeamID = teamID
				}
			}
			if !q.allowChatSend(scope.ConversationID, claims.Sub, time.Now()) {
				q.writeQueueMessage(conn, &writeMu, "chat.error", map[string]string{"message": "chat is moving too fast"})
				continue
			}
			if err := q.chat.RecordChatMessage(scope.ConversationID, scope.Kind, scope.ID, message); err != nil {
				observability.Log("error", "chat message persistence failed", map[string]any{
					"conversationId": scope.ConversationID,
					"scopeKind":      scope.Kind,
					"scopeId":        scope.ID,
					"userId":         claims.Sub,
					"error":          err.Error(),
				})
				q.writeQueueMessage(conn, &writeMu, "chat.error", map[string]string{"message": "chat unavailable"})
				continue
			}
			if q.redis == nil {
				q.writeQueueMessage(conn, &writeMu, contracts.EventChatMessage, message)
				continue
			}
			q.publishChatMessage(ctx, scope.ConversationID, message)
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
			if event == nil || strings.TrimSpace(event.Payload) == "" {
				continue
			}
			var message contracts.ChatMessage
			if err := json.Unmarshal([]byte(event.Payload), &message); err != nil {
				continue
			}
			if q.canViewChatMessage(claims.Sub, message) {
				q.writeQueueMessage(conn, &writeMu, contracts.EventChatMessage, message)
			}
		case <-pingTicker.C:
			if !q.writeQueuePing(conn, &writeMu) {
				return nil
			}
		}
	}
}

func (q *matchCoordinator) authorizeChatConversation(ctx context.Context, conversationID, userID string, staffViewer bool) (chatScope, error) {
	kind, id, ok := strings.Cut(conversationID, ":")
	if !ok || strings.TrimSpace(id) == "" {
		return chatScope{}, errors.New("invalid conversation")
	}
	scope := chatScope{ConversationID: conversationID, Kind: kind, ID: id}
	switch kind {
	case "party":
		snap, found, err := q.parties.GetPartyByID(id)
		if err != nil || !found || !partyHasMember(snap, userID) {
			return chatScope{}, errors.New("forbidden")
		}
		return scope, nil
	case "match":
		scope.MatchID = id
		if assigned, found, err := q.state.GetAssignmentByMatch(ctx, id); err == nil && found {
			for _, playerID := range assigned.Players {
				if playerID == userID {
					return scope, nil
				}
			}
		}
		if participated, err := q.matches.PlayerParticipatedInMatch(userID, id); err == nil && participated {
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

func (q *matchCoordinator) buildCoordinatorChatMessage(scope chatScope, userID, displayName string, cmd chatClientCommand) (contracts.ChatMessage, error) {
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
				message.Body = sanitizeCoordinatorChatBody(body)
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
		if !validCoordinatorChatEmote(message.Emote) {
			return contracts.ChatMessage{}, errors.New("unsupported emote")
		}
	default:
		return contracts.ChatMessage{}, errors.New("unsupported chat command")
	}
	return message, nil
}

func (q *matchCoordinator) resolveChatTeam(scope chatScope, userID string) (string, string, bool, error) {
	if scope.Kind == "party" {
		return q.chat.ActivePartyChatTeam(scope.ID, userID)
	}
	if scope.Kind == "match" {
		teamID, ok, err := q.chat.ChatTeamForMatch(scope.MatchID, userID)
		return scope.MatchID, teamID, ok, err
	}
	return "", "", false, nil
}

func (q *matchCoordinator) canViewChatMessage(userID string, message contracts.ChatMessage) bool {
	if message.Audience != contracts.ChatAudienceTeam {
		return true
	}
	teamID, ok, err := q.chat.ChatTeamForMatch(message.MatchID, userID)
	return err == nil && ok && teamID == message.TeamID
}

func chatRestrictionErrorMessage(restriction chatRestriction) string {
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

func sanitizeCoordinatorChatBody(body string) string {
	return contentfilter.NormalizeText(body, chatMaxBodyLen)
}

func validCoordinatorChatEmote(emote contracts.ChatEmote) bool {
	switch emote {
	case contracts.ChatEmoteSkull, contracts.ChatEmoteSob, contracts.ChatEmoteThinking, contracts.ChatEmoteSunglasses, contracts.ChatEmoteWave:
		return true
	default:
		return false
	}
}

func (q *matchCoordinator) allowChatSend(conversationID, userID string, now time.Time) bool {
	key := conversationID + ":" + userID
	cutoff := now.Add(-chatRateLimitWindow)
	q.chatMu.Lock()
	defer q.chatMu.Unlock()
	if q.chatRecent == nil {
		q.chatRecent = map[string][]time.Time{}
	}
	recent := q.chatRecent[key]
	kept := recent[:0]
	for _, ts := range recent {
		if ts.After(cutoff) {
			kept = append(kept, ts)
		}
	}
	if len(kept) >= chatRateLimitBurst {
		q.chatRecent[key] = kept
		return false
	}
	q.chatRecent[key] = append(kept, now)
	return true
}

func (q *matchCoordinator) publishChatMessage(ctx context.Context, conversationID string, message contracts.ChatMessage) {
	if q.redis == nil {
		return
	}
	body, err := json.Marshal(message)
	if err != nil {
		return
	}
	_ = q.redis.Publish(ctx, chatChannel(conversationID), string(body)).Err()
}

func chatChannel(conversationID string) string {
	return "chat:" + conversationID
}
