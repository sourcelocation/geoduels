package persistence

import (
	"context"
	"errors"
	"geoduels/pkg/contracts"
	"geoduels/pkg/entityid"
	db "geoduels/pkg/persistence/sqlc/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"strings"
	"time"
)

// Team values are written by the generated query using nullif($10, '')::gd_team_id.

func chatUUID(s string) pgtype.UUID { var u pgtype.UUID; _ = u.Scan(s); return u }

func chatUUIDErr(s string) (pgtype.UUID, error) {
	var u pgtype.UUID
	if err := u.Scan(s); err != nil {
		return pgtype.UUID{}, err
	}
	if !u.Valid {
		return pgtype.UUID{}, pgx.ErrNoRows
	}
	return u, nil
}
func chatText(s string) pgtype.Text { return pgtype.Text{String: s, Valid: s != ""} }
func chatStr(v interface{}) string {
	if v == nil {
		return ""
	}
	return v.(string)
}
func (s *DB) RecordChatMessage(cid, scope, scopeID string, m ChatMessage) error {
	cid, scope, scopeID = strings.TrimSpace(cid), strings.TrimSpace(scope), strings.TrimSpace(scopeID)
	if cid == "" || scope == "" || scopeID == "" {
		return errors.New("conversation scope required")
	}
	ctx, c := context.WithTimeout(context.Background(), 4*time.Second)
	defer c()
	t := m.CreatedAt
	if t.IsZero() {
		t = time.Now()
	}
	sid := entityid.Derive("conversation", cid)
	if e := s.db.EnsureConversation(ctx, db.EnsureConversationParams{ConversationID: chatUUID(sid), ScopeKind: db.GdChatScope(scope), ScopeID: chatUUID(scopeID)}); e != nil {
		return e
	}
	match := pgtype.UUID{}
	team := db.NullGdTeamID{}
	if m.Audience == contracts.ChatAudienceTeam {
		var err error
		match, err = nullableSessionUUID(m.MatchID)
		if err != nil {
			return err
		}
		team = db.NullGdTeamID{GdTeamID: db.GdTeamID(m.TeamID), Valid: m.TeamID != ""}
	}
	return s.db.InsertMessage(ctx, db.InsertMessageParams{MessageID: chatUUID(m.ID), ConversationID: chatUUID(sid), TeamMatchID: match, SenderUserID: chatUUID(m.SenderUserID), SenderDisplayName: m.SenderDisplayName, Kind: db.GdChatKind(m.Kind), Body: chatText(m.Body), Emote: chatText(string(m.Emote)), Audience: db.GdChatAudience(m.Audience), TeamID: team, CreatedAt: pgtype.Timestamptz{Time: t, Valid: true}})
}
func (s *DB) ListChatMessages(id string, n int) ([]ChatMessage, error) {
	return s.listChatMessages(id, "", n)
}
func (s *DB) ListChatMessagesForUser(id, u string, n int) ([]ChatMessage, error) {
	return s.listChatMessages(id, strings.TrimSpace(u), n)
}
func (s *DB) listChatMessages(id, u string, n int) ([]ChatMessage, error) {
	id = strings.TrimSpace(id)
	if id == "" {
		return nil, nil
	}
	if n <= 0 || n > 500 {
		n = 200
	}
	ctx, c := context.WithTimeout(context.Background(), 4*time.Second)
	defer c()
	rs, e := s.db.ListMessages(ctx, db.ListMessagesParams{ConversationID: chatUUID(entityid.Derive("conversation", id)), ViewerUserID: u, RowLimit: int32(n)})
	if e != nil {
		return nil, e
	}
	out := make([]ChatMessage, 0, len(rs))
	for _, r := range rs {
		teamID := ""
		if r.TeamID.Valid {
			teamID = string(r.TeamID.GdTeamID)
		}
		out = append(out, ChatMessage{ID: r.ID.String(), ConversationID: chatStr(r.ConversationID), MatchID: uuidVal(r.MatchID), SenderUserID: r.SenderUserID.String(), SenderDisplayName: r.SenderDisplayName, Kind: contracts.ChatMessageKind(r.Kind), Body: r.Body, Emote: contracts.ChatEmote(r.Emote), Audience: contracts.ChatAudience(r.Audience), TeamID: teamID, CreatedAt: r.CreatedAt.Time})
	}
	return out, nil
}
func (s *DB) ActivePartyChatTeam(p, u string) (string, string, bool, error) {
	ctx, c := context.WithTimeout(context.Background(), 4*time.Second)
	defer c()
	r, e := s.db.ActivePartyChatTeam(ctx, db.ActivePartyChatTeamParams{PartyID: chatUUID(p), UserID: chatUUID(u)})
	if errors.Is(e, pgx.ErrNoRows) {
		return "", "", false, nil
	}
	if e != nil {
		return "", "", false, e
	}
	t := chatStr(r.TeamID)
	return uuidVal(r.MatchID), t, t != "", nil
}
func (s *DB) ChatTeamForMatch(m, u string) (string, bool, error) {
	ctx, c := context.WithTimeout(context.Background(), 4*time.Second)
	defer c()
	v, e := s.db.ChatTeamForMatch(ctx, db.ChatTeamForMatchParams{MatchID: chatUUID(m), UserID: chatUUID(u)})
	if errors.Is(e, pgx.ErrNoRows) {
		return "", false, nil
	}
	if e != nil {
		return "", false, e
	}
	t := chatStr(v)
	return t, t != "", nil
}
func (s *DB) GetActiveChatRestriction(u string) (ChatRestriction, bool, error) {
	if strings.TrimSpace(u) == "" {
		return ChatRestriction{}, false, nil
	}
	ctx, c := context.WithTimeout(context.Background(), 3*time.Second)
	defer c()
	r, e := s.db.ActiveRestriction(ctx, chatUUID(u))
	if errors.Is(e, pgx.ErrNoRows) {
		return ChatRestriction{}, false, nil
	}
	if e != nil {
		return ChatRestriction{}, false, e
	}
	return ChatRestriction{ActionType: r.ActionType, ReasonCode: r.ReasonCode, ReasonNote: chatStr(r.ReasonNote), EndsAt: r.EndsAt.(pgtype.Timestamptz).Time}, true, nil
}
