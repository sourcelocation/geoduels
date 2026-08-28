package persistence

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"geoduels/pkg/entityid"
	db "geoduels/pkg/persistence/sqlc/db"
)

var (
	ErrSocialNotFound = errors.New("social resource not found")
	ErrSocialBlocked  = errors.New("social action unavailable")
	ErrSocialLimit    = errors.New("social limit reached")
)

const friendCodeAlphabet = "23456789ABCDEFGHJKMNPQRSTUVWXYZ"

type SocialRepository interface {
	GetSocialAccount(userID string) (isGuest, requestsEnabled, invitesEnabled bool, err error)
	GetSocialSettings(userID string) (SocialSettings, error)
	UpdateSocialSettings(userID string, settings SocialSettings) (SocialSettings, error)
	Relationship(userID, targetID string) (RelationshipState, string, error)
	ListFriends(userID string, limit int) ([]CompactPlayer, error)
	ListFriendRequests(userID string, direction string, limit int) ([]FriendRequest, error)
	SearchSocialPlayers(userID, query string, limit int) ([]CompactPlayer, error)
	ListRecentPlayers(userID string, limit int) ([]CompactPlayer, error)
	SendFriendRequest(userID, targetID string) (FriendRequest, error)
	RespondFriendRequest(userID, requestID, response string) error
	RemoveFriend(userID, targetID string) error
	SetUserBlock(userID, targetID string, blocked bool) error
	CreateFriendCode(userID string, ttl time.Duration) (FriendCode, error)
	ResolveFriendCode(userID, code string) (CompactPlayer, error)
	CreatePartyInvitation(partyID, inviterID, recipientID string, ttl time.Duration) (PartyInvitation, error)
	RespondPartyInvitation(userID, invitationID, response string) (PartyInvitation, error)
	ListPartyInvitations(userID string, limit int) ([]PartyInvitation, error)
	TouchLastSeen(userID string, seenAt time.Time) error
	AppendUserEvent(userID, eventType string, payload any) (int64, error)
	ListUserEvents(userID string, after int64, limit int) ([]UserEvent, error)
}

func (s *DB) GetSocialSettings(userID string) (SocialSettings, error) {
	id, err := profileUUID(userID)
	if err != nil {
		return SocialSettings{}, err
	}
	row, err := s.db.GetSocialSettings(context.Background(), id)
	settings := SocialSettings{Discoverable: row.SocialDiscoverable, PresenceVisible: row.SocialPresenceVisible, RequestsEnabled: row.SocialRequestsEnabled, PartyInvitesEnabled: row.SocialPartyInvitesEnabled}
	return settings, err
}

func (s *DB) UpdateSocialSettings(userID string, settings SocialSettings) (SocialSettings, error) {
	id, err := profileUUID(userID)
	if err != nil {
		return settings, err
	}
	row, err := s.db.UpdateSocialSettings(context.Background(), db.UpdateSocialSettingsParams{ID: id, SocialDiscoverable: settings.Discoverable, SocialPresenceVisible: settings.PresenceVisible, SocialRequestsEnabled: settings.RequestsEnabled, SocialPartyInvitesEnabled: settings.PartyInvitesEnabled})
	settings = SocialSettings{Discoverable: row.SocialDiscoverable, PresenceVisible: row.SocialPresenceVisible, RequestsEnabled: row.SocialRequestsEnabled, PartyInvitesEnabled: row.SocialPartyInvitesEnabled}
	return settings, err
}

func (s *DB) GetSocialAccount(userID string) (bool, bool, bool, error) {
	id, err := profileUUID(userID)
	if err != nil {
		return false, false, false, err
	}
	row, err := s.db.GetSocialAccount(context.Background(), id)
	return string(row.AccountType) == "guest", row.SocialRequestsEnabled, row.SocialPartyInvitesEnabled, err
}

func (s *DB) Relationship(userID, targetID string) (RelationshipState, string, error) {
	if userID == targetID {
		return RelationshipNone, "", nil
	}
	viewerUUID, err := profileUUID(userID)
	if err != nil {
		return RelationshipNone, "", err
	}
	targetUUID, err := profileUUID(targetID)
	if err != nil {
		return RelationshipNone, "", err
	}
	row, err := db.New(s.pool).Relationship(context.Background(), db.RelationshipParams{BlockerUserID: viewerUUID, BlockedUserID: targetUUID})
	if err != nil {
		return RelationshipNone, "", err
	}
	if row.BlockedByViewer {
		return RelationshipBlocked, "", nil
	}
	if row.BlockedByTarget {
		return RelationshipNone, "", nil
	}
	if row.Friends {
		return RelationshipFriends, "", nil
	}
	if row.RequestID != "" {
		if row.SenderID == userID {
			return RelationshipOutgoing, row.RequestID, nil
		}
		return RelationshipIncoming, row.RequestID, nil
	}
	return RelationshipNone, "", nil
}

func (s *DB) ListFriends(userID string, limit int) ([]CompactPlayer, error) {
	limit = boundedSocialLimit(limit, 100, 500)
	id, err := profileUUID(userID)
	if err != nil {
		return nil, err
	}
	rows, err := db.New(s.pool).ListFriends(context.Background(), db.ListFriendsParams{UserIDLow: id, Limit: int32(limit)})
	if err != nil {
		return nil, err
	}
	out := make([]CompactPlayer, 0, len(rows))
	for _, row := range rows {
		p := CompactPlayer{UserID: row.UserID, DisplayName: row.DisplayName, AvatarURL: row.AvatarUrl, MMR: int(row.Mmr)}
		if row.LastSeenAt.Valid {
			value := row.LastSeenAt.Time
			p.LastSeenAt = &value
		}
		p.Relationship = RelationshipFriends
		out = append(out, p)
	}
	return out, nil
}

func (s *DB) ListFriendRequests(userID, direction string, limit int) ([]FriendRequest, error) {
	limit = boundedSocialLimit(limit, 20, 100)
	id, err := profileUUID(userID)
	if err != nil {
		return nil, err
	}
	q := db.New(s.pool)
	out := []FriendRequest{}
	if direction == "outgoing" {
		rows, err := q.ListOutgoingFriendRequests(context.Background(), db.ListOutgoingFriendRequestsParams{SenderUserID: id, Limit: int32(limit)})
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			item := FriendRequest{ID: row.RequestID, Direction: direction, CreatedAt: row.CreatedAt.Time, ExpiresAt: row.ExpiresAt.Time}
			item.Player = CompactPlayer{UserID: row.UserID, DisplayName: fmt.Sprint(row.DisplayName), AvatarURL: row.AvatarUrl, MMR: int(row.Mmr), RequestID: row.RequestID, Relationship: RelationshipOutgoing}
			if row.LastSeenAt.Valid {
				value := row.LastSeenAt.Time
				item.Player.LastSeenAt = &value
			}
			out = append(out, item)
		}
	} else {
		rows, err := q.ListIncomingFriendRequests(context.Background(), db.ListIncomingFriendRequestsParams{RecipientUserID: id, Limit: int32(limit)})
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			item := FriendRequest{ID: row.RequestID, Direction: direction, CreatedAt: row.CreatedAt.Time, ExpiresAt: row.ExpiresAt.Time}
			item.Player = CompactPlayer{UserID: row.UserID, DisplayName: fmt.Sprint(row.DisplayName), AvatarURL: row.AvatarUrl, MMR: int(row.Mmr), RequestID: row.RequestID, Relationship: RelationshipIncoming}
			if row.LastSeenAt.Valid {
				value := row.LastSeenAt.Time
				item.Player.LastSeenAt = &value
			}
			out = append(out, item)
		}
	}
	return out, nil
}

func (s *DB) SearchSocialPlayers(userID, query string, limit int) ([]CompactPlayer, error) {
	query = strings.TrimSpace(query)
	if len([]rune(query)) < 2 {
		return []CompactPlayer{}, nil
	}
	limit = boundedSocialLimit(limit, 10, 20)
	id, err := profileUUID(userID)
	if err != nil {
		return nil, err
	}
	rows, err := db.New(s.pool).SearchSocialPlayers(context.Background(), db.SearchSocialPlayersParams{ID: id, Lower: query, Limit: int32(limit)})
	if err != nil {
		return nil, err
	}
	out := make([]CompactPlayer, 0, len(rows))
	for _, row := range rows {
		p := CompactPlayer{UserID: row.UserID, DisplayName: row.DisplayName, AvatarURL: row.AvatarUrl, MMR: int(row.Mmr)}
		if row.LastSeenAt.Valid {
			value := row.LastSeenAt.Time
			p.LastSeenAt = &value
		}
		p.Relationship, p.RequestID, _ = s.Relationship(userID, p.UserID)
		out = append(out, p)
	}
	return out, nil
}

func (s *DB) ListRecentPlayers(userID string, limit int) ([]CompactPlayer, error) {
	limit = boundedSocialLimit(limit, 3, 3)
	id, err := profileUUID(userID)
	if err != nil {
		return nil, err
	}
	rows, err := db.New(s.pool).ListRecentPlayers(context.Background(), db.ListRecentPlayersParams{Column1: id, Limit: int32(limit)})
	if err != nil {
		return nil, err
	}
	out := make([]CompactPlayer, 0, len(rows))
	for _, row := range rows {
		p := CompactPlayer{UserID: row.UserID, DisplayName: row.DisplayName, AvatarURL: row.AvatarUrl, MMR: int(row.Mmr)}
		if row.LastSeenAt.Valid {
			value := row.LastSeenAt.Time
			p.LastSeenAt = &value
		}
		if row.SharedAt.Valid {
			value := row.SharedAt.Time
			p.SharedMatchAt = &value
		}
		p.Relationship = RelationshipNone
		out = append(out, p)
	}
	return out, nil
}

func (s *DB) SendFriendRequest(userID, targetID string) (FriendRequest, error) {
	if userID == targetID {
		return FriendRequest{}, ErrSocialBlocked
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return FriendRequest{}, err
	}
	defer tx.Rollback(ctx)
	userUUID, err := profileUUID(userID)
	if err != nil {
		return FriendRequest{}, err
	}
	targetUUID, err := profileUUID(targetID)
	if err != nil {
		return FriendRequest{}, err
	}
	q := db.New(tx)
	allowed, err := q.CanSendFriendRequest(ctx, db.CanSendFriendRequestParams{BlockerUserID: userUUID, ID: targetUUID})
	if err != nil || !allowed.Bool {
		return FriendRequest{}, ErrSocialBlocked
	}
	friendCount, err := q.CountFriends(ctx, userUUID)
	if err != nil {
		return FriendRequest{}, err
	}
	if friendCount >= 500 {
		return FriendRequest{}, ErrSocialLimit
	}
	crossedID, err := q.FindCrossedFriendRequest(ctx, db.FindCrossedFriendRequestParams{RecipientUserID: userUUID, SenderUserID: targetUUID})
	if err == nil {
		if err := acceptFriendRequestTx(ctx, tx, crossedID, userID); err != nil {
			return FriendRequest{}, err
		}
		_ = appendUserEventTx(ctx, tx, targetID, "friendship.created", map[string]any{"userId": userID})
		_ = appendUserEventTx(ctx, tx, userID, "friendship.created", map[string]any{"userId": targetID})
		return FriendRequest{ID: crossedID, Direction: "incoming"}, tx.Commit(ctx)
	}
	id := entityid.New()
	expiresAt := time.Now().Add(30 * 24 * time.Hour)
	var createdAt time.Time
	requestUUID, err := profileUUID(id)
	if err != nil {
		return FriendRequest{}, err
	}
	row, err := q.UpsertFriendRequest(ctx, db.UpsertFriendRequestParams{ID: requestUUID, SenderUserID: userUUID, RecipientUserID: targetUUID, ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true}})
	if err != nil {
		return FriendRequest{}, err
	}
	id, createdAt, expiresAt = row.ID, row.CreatedAt.Time, row.ExpiresAt.Time
	var notificationID int64
	_ = upsertUserNotification(ctx, tx, targetID, "friend_request_received", "friend_request:"+id,
		map[string]any{"requestId": id, "actorUserId": userID}, &notificationID)
	_ = appendUserEventTx(ctx, tx, targetID, "friend_request.created", map[string]any{"requestId": id})
	if err := tx.Commit(ctx); err != nil {
		return FriendRequest{}, err
	}
	return FriendRequest{ID: id, Direction: "outgoing", CreatedAt: createdAt, ExpiresAt: expiresAt}, nil
}

func acceptFriendRequestTx(ctx context.Context, tx pgx.Tx, requestID, recipientID string) error {
	requestUUID, err := profileUUID(requestID)
	if err != nil {
		return ErrSocialNotFound
	}
	recipientUUID, err := profileUUID(recipientID)
	if err != nil {
		return ErrSocialNotFound
	}
	q := db.New(tx)
	senderID, err := q.AcceptFriendRequest(ctx, db.AcceptFriendRequestParams{ID: requestUUID, RecipientUserID: recipientUUID})
	if err != nil {
		return ErrSocialNotFound
	}
	senderUUID, err := profileUUID(senderID)
	if err != nil {
		return err
	}
	return q.InsertFriendship(ctx, db.InsertFriendshipParams{Column1: senderUUID, Column2: recipientUUID, CreatedFromRequestID: requestUUID})
}

func (s *DB) RespondFriendRequest(userID, requestID, response string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	userUUID, err := profileUUID(userID)
	if err != nil {
		return err
	}
	requestUUID, err := profileUUID(requestID)
	if err != nil {
		return ErrSocialNotFound
	}
	q := db.New(tx)
	var otherID string
	if response == "accept" {
		otherID, err = q.FriendRequestSender(ctx, db.FriendRequestSenderParams{ID: requestUUID, RecipientUserID: userUUID})
		if err != nil {
			return ErrSocialNotFound
		}
		if err := acceptFriendRequestTx(ctx, tx, requestID, userID); err != nil {
			return err
		}
		var notificationID int64
		_ = upsertUserNotification(ctx, tx, otherID, "friendship_accepted", "friendship_accepted:"+requestID,
			map[string]any{"actorUserId": userID}, &notificationID)
		_ = appendUserEventTx(ctx, tx, otherID, "friendship.created", map[string]any{"userId": userID})
		_ = appendUserEventTx(ctx, tx, userID, "friendship.created", map[string]any{"userId": otherID})
	} else {
		var affected int64
		if response == "cancel" {
			affected, err = q.CancelFriendRequest(ctx, db.CancelFriendRequestParams{ID: requestUUID, SenderUserID: userUUID})
		} else {
			affected, err = q.DeclineFriendRequest(ctx, db.DeclineFriendRequestParams{ID: requestUUID, RecipientUserID: userUUID})
		}
		if err != nil || affected == 0 {
			return ErrSocialNotFound
		}
	}
	_ = q.MarkFriendRequestNotificationRead(ctx, db.MarkFriendRequestNotificationReadParams{UserID: userUUID, Column2: ingestText(requestID)})
	return tx.Commit(ctx)
}

func (s *DB) RemoveFriend(userID, targetID string) error {
	userUUID, err := profileUUID(userID)
	if err != nil {
		return err
	}
	targetUUID, err := profileUUID(targetID)
	if err != nil {
		return err
	}
	return db.New(s.pool).RemoveFriend(context.Background(), db.RemoveFriendParams{Column1: userUUID, Column2: targetUUID})
}

func (s *DB) SetUserBlock(userID, targetID string, blocked bool) error {
	if userID == targetID {
		return ErrSocialBlocked
	}
	ctx := context.Background()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	userUUID, err := profileUUID(userID)
	if err != nil {
		return err
	}
	targetUUID, err := profileUUID(targetID)
	if err != nil {
		return err
	}
	q := db.New(tx)
	if blocked {
		if err := q.AddUserBlock(ctx, db.AddUserBlockParams{BlockerUserID: userUUID, BlockedUserID: targetUUID}); err != nil {
			return err
		}
		_ = q.RemoveFriend(ctx, db.RemoveFriendParams{Column1: userUUID, Column2: targetUUID})
		_ = q.CancelPairFriendRequests(ctx, db.CancelPairFriendRequestsParams{Column1: userUUID, Column2: targetUUID})
	} else {
		err = q.RemoveUserBlock(ctx, db.RemoveUserBlockParams{BlockerUserID: userUUID, BlockedUserID: targetUUID})
		if err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *DB) CreateFriendCode(userID string, ttl time.Duration) (FriendCode, error) {
	if ttl <= 0 {
		ttl = 7 * 24 * time.Hour
	}
	ctx := context.Background()
	userUUID, err := profileUUID(userID)
	if err != nil {
		return FriendCode{}, err
	}
	for attempt := 0; attempt < 8; attempt++ {
		code, err := randomFriendCode()
		if err != nil {
			return FriendCode{}, err
		}
		expiresAt := time.Now().Add(ttl)
		tx, err := s.pool.Begin(ctx)
		if err != nil {
			return FriendCode{}, err
		}
		q := db.New(tx)
		_ = q.RevokeFriendCodes(ctx, userUUID)
		err = q.InsertFriendCode(ctx, db.InsertFriendCodeParams{Code: code, UserID: userUUID, ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true}})
		if err == nil {
			err = tx.Commit(ctx)
			return FriendCode{Code: code, ExpiresAt: expiresAt}, err
		}
		_ = tx.Rollback(ctx)
	}
	return FriendCode{}, errors.New("could not allocate friend code")
}

func (s *DB) ResolveFriendCode(userID, code string) (CompactPlayer, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	userUUID, err := profileUUID(userID)
	if err != nil {
		return CompactPlayer{}, ErrSocialNotFound
	}
	row, err := db.New(s.pool).ResolveFriendCode(context.Background(), db.ResolveFriendCodeParams{Code: code, ID: userUUID})
	if err != nil {
		return CompactPlayer{}, ErrSocialNotFound
	}
	p := CompactPlayer{UserID: row.UserID, DisplayName: row.DisplayName, AvatarURL: row.AvatarUrl, MMR: int(row.Mmr)}
	if row.LastSeenAt.Valid {
		value := row.LastSeenAt.Time
		p.LastSeenAt = &value
	}
	p.Relationship, p.RequestID, _ = s.Relationship(userID, p.UserID)
	return p, nil
}

func randomFriendCode() (string, error) {
	buf := make([]byte, 6)
	random := make([]byte, 6)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	for i := range buf {
		buf[i] = friendCodeAlphabet[int(random[i])%len(friendCodeAlphabet)]
	}
	return string(buf), nil
}

func (s *DB) CreatePartyInvitation(partyID, inviterID, recipientID string, ttl time.Duration) (PartyInvitation, error) {
	if ttl <= 0 {
		ttl = 20 * time.Minute
	}
	ctx := context.Background()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return PartyInvitation{}, err
	}
	defer tx.Rollback(ctx)
	partyUUID, err := profileUUID(partyID)
	if err != nil {
		return PartyInvitation{}, ErrSocialBlocked
	}
	inviterUUID, err := profileUUID(inviterID)
	if err != nil {
		return PartyInvitation{}, ErrSocialBlocked
	}
	recipientUUID, err := profileUUID(recipientID)
	if err != nil {
		return PartyInvitation{}, ErrSocialBlocked
	}
	q := db.New(tx)
	eligibility, err := q.PartyInvitationEligibility(ctx, db.PartyInvitationEligibilityParams{ID: partyUUID, UserID: inviterUUID, Column3: recipientUUID})
	if err != nil {
		return PartyInvitation{}, ErrSocialBlocked
	}
	id := entityid.New()
	expiresAt := time.Now().Add(ttl)
	invitationUUID, err := profileUUID(id)
	if err != nil {
		return PartyInvitation{}, err
	}
	row, err := q.UpsertPartyInvitation(ctx, db.UpsertPartyInvitationParams{ID: invitationUUID, PartyID: partyUUID, InviterUserID: inviterUUID, RecipientUserID: recipientUUID, ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true}})
	if err != nil {
		return PartyInvitation{}, err
	}
	id, expiresAt = row.ID, row.ExpiresAt.Time
	var notificationID int64
	_ = upsertUserNotification(ctx, tx, recipientID, "party_invitation_received", "party_invitation:"+id,
		map[string]any{"invitationId": id, "actorUserId": inviterID}, &notificationID)
	_ = appendUserEventTx(ctx, tx, recipientID, "party_invitation.created", map[string]any{"invitationId": id})
	if err := tx.Commit(ctx); err != nil {
		return PartyInvitation{}, err
	}
	return PartyInvitation{ID: id, PartyID: partyID, InviteCode: eligibility.InviteCode, Mode: string(eligibility.Mode), MemberCount: int(eligibility.MemberCount), ExpiresAt: expiresAt}, nil
}

func (s *DB) ListPartyInvitations(userID string, limit int) ([]PartyInvitation, error) {
	limit = boundedSocialLimit(limit, 10, 50)
	userUUID, err := profileUUID(userID)
	if err != nil {
		return nil, err
	}
	rows, err := db.New(s.pool).ListPartyInvitations(context.Background(), db.ListPartyInvitationsParams{RecipientUserID: userUUID, Limit: int32(limit)})
	if err != nil {
		return nil, err
	}
	out := make([]PartyInvitation, 0, len(rows))
	for _, row := range rows {
		item := PartyInvitation{ID: row.InvitationID, PartyID: row.PartyID, InviteCode: row.InviteCode, Mode: string(row.Mode), MemberCount: int(row.MemberCount), CreatedAt: row.CreatedAt.Time, ExpiresAt: row.ExpiresAt.Time}
		item.Inviter = CompactPlayer{UserID: row.InviterID, DisplayName: row.DisplayName, AvatarURL: row.AvatarUrl}
		out = append(out, item)
	}
	return out, nil
}

func (s *DB) RespondPartyInvitation(userID, invitationID, response string) (PartyInvitation, error) {
	status := "declined"
	if response == "accept" {
		status = "accepted"
	}
	userUUID, err := profileUUID(userID)
	if err != nil {
		return PartyInvitation{}, ErrSocialNotFound
	}
	invitationUUID, err := profileUUID(invitationID)
	if err != nil {
		return PartyInvitation{}, ErrSocialNotFound
	}
	q := db.New(s.pool)
	row, err := q.RespondPartyInvitation(context.Background(), db.RespondPartyInvitationParams{ID: invitationUUID, RecipientUserID: userUUID, Status: db.GdSocialRequestStatus(status)})
	if err != nil {
		return PartyInvitation{}, ErrSocialNotFound
	}
	_ = q.MarkPartyInvitationNotificationRead(context.Background(), db.MarkPartyInvitationNotificationReadParams{UserID: userUUID, Column2: ingestText(invitationID)})
	return PartyInvitation{ID: row.PiID, PartyID: row.PID, InviteCode: row.InviteCode, Mode: string(row.Mode), ExpiresAt: row.ExpiresAt.Time}, nil
}

func (s *DB) TouchLastSeen(userID string, seenAt time.Time) error {
	id, err := profileUUID(userID)
	if err != nil {
		return err
	}
	return s.db.TouchLastSeen(context.Background(), db.TouchLastSeenParams{ID: id, LastSeenAt: pgtype.Timestamptz{Time: seenAt, Valid: true}})
}

func (s *DB) AppendUserEvent(userID, eventType string, payload any) (int64, error) {
	ctx := context.Background()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx)
	sequence := int64(0)
	if err := appendUserEventTxScan(ctx, tx, userID, eventType, payload, &sequence); err != nil {
		return 0, err
	}
	return sequence, tx.Commit(ctx)
}

func appendUserEventTx(ctx context.Context, tx pgx.Tx, userID, eventType string, payload any) error {
	return appendUserEventTxScan(ctx, tx, userID, eventType, payload, new(int64))
}

func appendUserEventTxScan(ctx context.Context, tx pgx.Tx, userID, eventType string, payload any, sequence *int64) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	userUUID, err := profileUUID(userID)
	if err != nil {
		return err
	}
	q := db.New(tx)
	value, err := q.NextUserEventSequence(ctx, userUUID)
	if err != nil {
		return err
	}
	*sequence = value
	return q.InsertUserEvent(ctx, db.InsertUserEventParams{UserID: userUUID, Sequence: value, Type: db.GdUserEventType(eventType), Column4: body})
}

func (s *DB) ListUserEvents(userID string, after int64, limit int) ([]UserEvent, error) {
	limit = boundedSocialLimit(limit, 100, 500)
	userUUID, err := profileUUID(userID)
	if err != nil {
		return nil, err
	}
	rows, err := db.New(s.pool).ListUserEvents(context.Background(), db.ListUserEventsParams{UserID: userUUID, Sequence: after, Limit: int32(limit)})
	if err != nil {
		return nil, err
	}
	out := make([]UserEvent, 0, len(rows))
	for _, row := range rows {
		event := UserEvent{Sequence: row.Sequence, Type: string(row.Type), Payload: []byte(row.Payload), OccurredAt: row.CreatedAt.Time}
		out = append(out, event)
	}
	return out, nil
}

func boundedSocialLimit(limit, fallback, maximum int) int {
	if limit <= 0 {
		return fallback
	}
	if limit > maximum {
		return maximum
	}
	return limit
}
