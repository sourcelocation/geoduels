package persistence

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"geoduels/pkg/contracts"
	"geoduels/pkg/entityid"
)

func (s *pgStore) RecordChatMessage(conversationID, scopeKind, scopeID string, message ChatMessage) error {
	conversationID = strings.TrimSpace(conversationID)
	scopeKind = strings.TrimSpace(scopeKind)
	scopeID = strings.TrimSpace(scopeID)
	if conversationID == "" || scopeKind == "" || scopeID == "" {
		return errors.New("conversation scope required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	createdAt := message.CreatedAt
	if createdAt.IsZero() {
		createdAt = time.Now()
	}
	return s.recordChatMessage(ctx, conversationID, scopeKind, scopeID, message, createdAt)
}

func (s *pgStore) recordChatMessage(ctx context.Context, conversationID, scopeKind, scopeID string, message ChatMessage, createdAt time.Time) error {
	body := nullable(message.Body)
	emote := nullable(string(message.Emote))
	storageConversationID := entityid.Derive("conversation", conversationID)
	_, err := s.pool.Exec(ctx, `
		insert into chat_conversations (id, scope_kind, scope_id)
		values ($1, $2, $3)
		on conflict (id) do nothing
	`, storageConversationID, scopeKind, scopeID)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		insert into chat_messages (
			id, conversation_id, sender_user_id, sender_display_name, kind, body, emote, created_at
		)
		values ($1, $2, $3, $4, $5, $6, $7, $8)
		on conflict (id) do nothing
	`, message.ID, storageConversationID, message.SenderUserID, message.SenderDisplayName, string(message.Kind), body, emote, createdAt)
	return err
}

func (s *pgStore) ListChatMessages(conversationID string, limit int) ([]ChatMessage, error) {
	conversationID = strings.TrimSpace(conversationID)
	if conversationID == "" {
		return nil, nil
	}
	if limit <= 0 || limit > 500 {
		limit = 200
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	rows, err := s.pool.Query(ctx, `
		select m.id, c.scope_kind || ':' || c.scope_id, m.sender_user_id, m.sender_display_name, m.kind, coalesce(m.body, ''), coalesce(m.emote, ''), m.created_at
		from chat_messages m
		join chat_conversations c on c.id=m.conversation_id
		where m.conversation_id = $1
		order by created_at asc
		limit $2
	`, entityid.Derive("conversation", conversationID), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	messages := []ChatMessage{}
	for rows.Next() {
		var message ChatMessage
		var kind string
		var emote string
		if err := rows.Scan(&message.ID, &message.ConversationID, &message.SenderUserID, &message.SenderDisplayName, &kind, &message.Body, &emote, &message.CreatedAt); err != nil {
			return nil, err
		}
		message.Kind = contracts.ChatMessageKind(kind)
		message.Emote = contracts.ChatEmote(emote)
		messages = append(messages, message)
	}
	return messages, rows.Err()
}

func (s *pgStore) GetActiveChatRestriction(userID string) (ChatRestriction, bool, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return ChatRestriction{}, false, nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	var restriction ChatRestriction
	err := s.pool.QueryRow(ctx, `
		select
			action_type,
			reason_code,
			coalesce(reason_note, ''),
			coalesce(ends_at, '0001-01-01 00:00:00+00'::timestamptz)
		from enforcement_actions
		where target_user_id = $1
			and action_type in ('chat_mute', 'temporary_ban', 'permanent_ban')
			and starts_at <= now()
			and (ends_at is null or ends_at > now())
			and revoked_at is null
		order by
			case action_type
				when 'permanent_ban' then 0
				when 'temporary_ban' then 1
				else 2
			end,
			created_at desc,
			id desc
		limit 1
	`, userID).Scan(&restriction.ActionType, &restriction.ReasonCode, &restriction.ReasonNote, &restriction.EndsAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ChatRestriction{}, false, nil
	}
	if err != nil {
		return ChatRestriction{}, false, err
	}
	return restriction, true, nil
}
