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
	// team_match_id is the authorization context for team-only messages. It
	// must remain null for public messages, even when the conversation itself
	// is a match conversation.
	teamMatchID := ""
	teamID := ""
	if message.Audience == contracts.ChatAudienceTeam {
		teamMatchID = message.MatchID
		teamID = message.TeamID
	}
	storageConversationID := entityid.Derive("conversation", conversationID)
	_, err := s.pool.Exec(ctx, `
		insert into chat_conversations (id, scope_kind, scope_id)
		values ($1, $2, $3::uuid)
		on conflict (id) do nothing
	`, storageConversationID, scopeKind, scopeID)
	if err != nil {
		return err
	}
	_, err = s.pool.Exec(ctx, `
		insert into chat_messages (
			id, conversation_id, team_match_id, sender_user_id, sender_display_name, kind, body, emote, audience, team_id, created_at
		)
		values ($1, $2, nullif($3, '')::uuid, $4, $5, $6, $7, $8, $9, nullif($10, '')::gd_team_id, $11)
		on conflict (id) do nothing
	`, message.ID, storageConversationID, teamMatchID, message.SenderUserID, message.SenderDisplayName, string(message.Kind), body, emote, string(message.Audience), teamID, createdAt)
	return err
}

func (s *pgStore) ListChatMessages(conversationID string, limit int) ([]ChatMessage, error) {
	return s.listChatMessages(conversationID, "", limit)
}

func (s *pgStore) ListChatMessagesForUser(conversationID, userID string, limit int) ([]ChatMessage, error) {
	return s.listChatMessages(conversationID, strings.TrimSpace(userID), limit)
}

func (s *pgStore) listChatMessages(conversationID, userID string, limit int) ([]ChatMessage, error) {
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
		select m.id, c.scope_kind || ':' || c.scope_id::text, coalesce(m.team_match_id::text, ''), m.sender_user_id, m.sender_display_name, m.kind, coalesce(m.body, ''), coalesce(m.emote, ''), m.audience, coalesce(m.team_id::text, ''), m.created_at
		from chat_messages m
		join chat_conversations c on c.id=m.conversation_id
		where m.conversation_id = $1
		  and ($2 = '' or m.audience = 'all' or exists (
		    select 1 from match_participants mp
		    where mp.match_id = m.team_match_id and mp.user_id = nullif($2, '')::uuid and mp.team_id::text = m.team_id::text
		  ))
		order by created_at asc
		limit $3
	`, entityid.Derive("conversation", conversationID), userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	messages := []ChatMessage{}
	for rows.Next() {
		var message ChatMessage
		var kind string
		var emote string
		var audience string
		if err := rows.Scan(&message.ID, &message.ConversationID, &message.MatchID, &message.SenderUserID, &message.SenderDisplayName, &kind, &message.Body, &emote, &audience, &message.TeamID, &message.CreatedAt); err != nil {
			return nil, err
		}
		message.Kind = contracts.ChatMessageKind(kind)
		message.Emote = contracts.ChatEmote(emote)
		message.Audience = contracts.ChatAudience(audience)
		messages = append(messages, message)
	}
	return messages, rows.Err()
}

func (s *pgStore) ActivePartyChatTeam(partyID, userID string) (string, string, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	var matchID, teamID string
	err := s.pool.QueryRow(ctx, `
		select ms.match_id::text, coalesce(mp.team_id::text, '')
		from parties p
		join match_sessions ms on ms.match_id = coalesce(p.active_match_id, p.started_match_id)
		join match_participants mp on mp.match_id = ms.match_id and mp.user_id = $2
		where p.id = $1 and ms.mode = 'team_duel' and ms.state = 'live'
	`, partyID, userID).Scan(&matchID, &teamID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", false, nil
	}
	if err != nil {
		return "", "", false, err
	}
	return matchID, teamID, teamID != "", nil
}

func (s *pgStore) ChatTeamForMatch(matchID, userID string) (string, bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	var teamID string
	err := s.pool.QueryRow(ctx, `select coalesce(team_id::text, '') from match_participants where match_id = $1 and user_id = $2`, matchID, userID).Scan(&teamID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return teamID, teamID != "", nil
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
			case when banned_at is not null and (ban_expires_at is null or ban_expires_at > now())
				then case when ban_expires_at is null then 'permanent_ban' else 'temporary_ban' end else 'chat_mute' end,
			case when banned_at is not null and (ban_expires_at is null or ban_expires_at > now()) then 'ban' else 'chat_mute' end,
			case when banned_at is not null and (ban_expires_at is null or ban_expires_at > now()) then coalesce(ban_reason, '') else coalesce(chat_mute_reason, '') end,
			coalesce(case when banned_at is not null and (ban_expires_at is null or ban_expires_at > now()) then ban_expires_at else chat_mute_expires_at end, '0001-01-01 00:00:00+00'::timestamptz)
		from users
		where id = $1 and (
			(banned_at is not null and (ban_expires_at is null or ban_expires_at > now()))
			or (chat_muted_at is not null and (chat_mute_expires_at is null or chat_mute_expires_at > now()))
		)
	`, userID).Scan(&restriction.ActionType, &restriction.ReasonCode, &restriction.ReasonNote, &restriction.EndsAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ChatRestriction{}, false, nil
	}
	if err != nil {
		return ChatRestriction{}, false, err
	}
	return restriction, true, nil
}
