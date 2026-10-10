package social

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"geoduels/internal/storekit"
	"geoduels/pkg/entityid"
	db "geoduels/pkg/persistence/sqlc/db"
)

// PGStore persists social data using the pool, or the transaction supplied
// by WithinTx. The original store is never mutated when starting a transaction.
type PGStore struct {
	pool *pgxpool.Pool
	tx   pgx.Tx
}

func NewPGStore(pool *pgxpool.Pool) *PGStore { return &PGStore{pool: pool} }

// q selects the transaction when present and the pool otherwise.
func (s *PGStore) q() *db.Queries {
	if s.tx != nil {
		return db.New(s.tx)
	}
	return db.New(s.pool)
}

// WithinTx runs fn with a copy of the store bound to one transaction.
func (s *PGStore) WithinTx(ctx context.Context, fn func(Store) error) error {
	return storekit.WithTx(ctx, s.pool, func(tx pgx.Tx) error {
		bound := *s
		bound.tx = tx
		return fn(&bound)
	})
}

var _ Store = (*PGStore)(nil)

func (s *PGStore) requireTx() (pgx.Tx, error) {
	if s.tx == nil {
		return nil, errors.New("social store: operation requires a transaction")
	}
	return s.tx, nil
}

func (s *PGStore) GetSocialSettings(ctx context.Context, userID string) (SocialSettings, error) {
	id, err := storekit.ProfileUUID(userID)
	if err != nil {
		return SocialSettings{}, err
	}
	row, err := s.q().GetSocialSettings(ctx, id)
	settings := SocialSettings{Discoverable: row.SocialDiscoverable, PresenceVisible: row.SocialPresenceVisible, RequestsEnabled: row.SocialRequestsEnabled, PartyInvitesEnabled: row.SocialPartyInvitesEnabled}
	return settings, err
}

func (s *PGStore) UpdateSocialSettings(ctx context.Context, userID string, settings SocialSettings) (SocialSettings, error) {
	id, err := storekit.ProfileUUID(userID)
	if err != nil {
		return settings, err
	}
	row, err := s.q().UpdateSocialSettings(ctx, db.UpdateSocialSettingsParams{ID: id, SocialDiscoverable: settings.Discoverable, SocialPresenceVisible: settings.PresenceVisible, SocialRequestsEnabled: settings.RequestsEnabled, SocialPartyInvitesEnabled: settings.PartyInvitesEnabled})
	settings = SocialSettings{Discoverable: row.SocialDiscoverable, PresenceVisible: row.SocialPresenceVisible, RequestsEnabled: row.SocialRequestsEnabled, PartyInvitesEnabled: row.SocialPartyInvitesEnabled}
	return settings, err
}

func (s *PGStore) GetSocialAccount(ctx context.Context, userID string) (SocialAccount, error) {
	id, err := storekit.ProfileUUID(userID)
	if err != nil {
		return SocialAccount{}, err
	}
	row, err := s.q().GetSocialAccount(ctx, id)
	if err != nil {
		return SocialAccount{}, err
	}
	return SocialAccount{IsGuest: !row.HasIdentity, RequestsEnabled: row.SocialRequestsEnabled, PartyInvitesEnabled: row.SocialPartyInvitesEnabled}, nil
}

func (s *PGStore) Relationship(ctx context.Context, userID, targetID string) (RelationshipState, string, error) {
	if userID == targetID {
		return RelationshipNone, "", nil
	}
	viewerUUID, err := storekit.ProfileUUID(userID)
	if err != nil {
		return RelationshipNone, "", err
	}
	targetUUID, err := storekit.ProfileUUID(targetID)
	if err != nil {
		return RelationshipNone, "", err
	}
	row, err := s.q().Relationship(ctx, db.RelationshipParams{BlockerUserID: viewerUUID, BlockedUserID: targetUUID})
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
	if row.RequestID.Valid {
		requestID := storekit.UUIDVal(row.RequestID)
		if storekit.UUIDVal(row.SenderID) == userID {
			return RelationshipOutgoing, requestID, nil
		}
		return RelationshipIncoming, requestID, nil
	}
	return RelationshipNone, "", nil
}

func (s *PGStore) ListFriends(ctx context.Context, userID string, limit int) ([]CompactPlayer, error) {
	limit = BoundedLimit(limit, FriendsListLimit, 500)
	id, err := storekit.ProfileUUID(userID)
	if err != nil {
		return nil, err
	}
	seasonID, err := storekit.ActiveSeasonID(ctx, s.pool)
	if err != nil {
		return nil, err
	}
	rows, err := s.q().ListFriends(ctx, db.ListFriendsParams{UserIDLow: id, SeasonID: seasonID, RowLimit: int32(limit)})
	if err != nil {
		return nil, err
	}
	out := make([]CompactPlayer, 0, len(rows))
	for _, row := range rows {
		p := CompactPlayer{UserID: storekit.UUIDVal(row.UserID), DisplayName: row.DisplayName, AvatarURL: row.AvatarUrl, MMR: int(row.Mmr)}
		if row.LastSeenAt.Valid {
			value := row.LastSeenAt.Time
			p.LastSeenAt = &value
		}
		p.Relationship = RelationshipFriends
		out = append(out, p)
	}
	return out, nil
}

func (s *PGStore) ListFriendRequests(ctx context.Context, userID, direction string, limit int) ([]FriendRequest, error) {
	limit = BoundedLimit(limit, FriendRequestsLimit, 100)
	id, err := storekit.ProfileUUID(userID)
	if err != nil {
		return nil, err
	}
	seasonID, err := storekit.ActiveSeasonID(ctx, s.pool)
	if err != nil {
		return nil, err
	}
	q := s.q()
	out := []FriendRequest{}
	if direction == "outgoing" {
		rows, err := q.ListOutgoingFriendRequests(ctx, db.ListOutgoingFriendRequestsParams{SeasonID: seasonID, SenderUserID: id, RowLimit: int32(limit)})
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			item := FriendRequest{ID: storekit.UUIDVal(row.RequestID), Direction: direction, CreatedAt: row.CreatedAt.Time, ExpiresAt: row.ExpiresAt.Time}
			item.Player = CompactPlayer{UserID: storekit.UUIDVal(row.UserID), DisplayName: fmt.Sprint(row.DisplayName), AvatarURL: row.AvatarUrl, MMR: int(row.Mmr), RequestID: storekit.UUIDVal(row.RequestID), Relationship: RelationshipOutgoing}
			if row.LastSeenAt.Valid {
				value := row.LastSeenAt.Time
				item.Player.LastSeenAt = &value
			}
			out = append(out, item)
		}
	} else {
		rows, err := q.ListIncomingFriendRequests(ctx, db.ListIncomingFriendRequestsParams{SeasonID: seasonID, RecipientUserID: id, RowLimit: int32(limit)})
		if err != nil {
			return nil, err
		}
		for _, row := range rows {
			item := FriendRequest{ID: storekit.UUIDVal(row.RequestID), Direction: direction, CreatedAt: row.CreatedAt.Time, ExpiresAt: row.ExpiresAt.Time}
			item.Player = CompactPlayer{UserID: storekit.UUIDVal(row.UserID), DisplayName: fmt.Sprint(row.DisplayName), AvatarURL: row.AvatarUrl, MMR: int(row.Mmr), RequestID: storekit.UUIDVal(row.RequestID), Relationship: RelationshipIncoming}
			if row.LastSeenAt.Valid {
				value := row.LastSeenAt.Time
				item.Player.LastSeenAt = &value
			}
			out = append(out, item)
		}
	}
	return out, nil
}

func (s *PGStore) SearchSocialPlayers(ctx context.Context, userID, query string, limit int) ([]CompactPlayer, error) {
	query = strings.TrimSpace(query)
	if len([]rune(query)) < 2 {
		return []CompactPlayer{}, nil
	}
	limit = BoundedLimit(limit, SearchLimit, SearchMaxLimit)
	id, err := storekit.ProfileUUID(userID)
	if err != nil {
		return nil, err
	}
	seasonID, err := storekit.ActiveSeasonID(ctx, s.pool)
	if err != nil {
		return nil, err
	}
	rows, err := s.q().SearchSocialPlayers(ctx, db.SearchSocialPlayersParams{SeasonID: seasonID, SelfUserID: id, Query: query, RowLimit: int32(limit)})
	if err != nil {
		return nil, err
	}
	out := make([]CompactPlayer, 0, len(rows))
	for _, row := range rows {
		p := CompactPlayer{UserID: storekit.UUIDVal(row.UserID), DisplayName: row.DisplayName, AvatarURL: row.AvatarUrl, MMR: int(row.Mmr)}
		if row.LastSeenAt.Valid {
			value := row.LastSeenAt.Time
			p.LastSeenAt = &value
		}
		p.Relationship, p.RequestID, _ = s.Relationship(ctx, userID, p.UserID)
		out = append(out, p)
	}
	return out, nil
}

func (s *PGStore) ListRecentPlayers(ctx context.Context, userID string, limit int) ([]CompactPlayer, error) {
	limit = BoundedLimit(limit, RecentPlayersLimit, RecentPlayersLimit)
	id, err := storekit.ProfileUUID(userID)
	if err != nil {
		return nil, err
	}
	seasonID, err := storekit.ActiveSeasonID(ctx, s.pool)
	if err != nil {
		return nil, err
	}
	rows, err := s.q().ListRecentPlayers(ctx, db.ListRecentPlayersParams{SeasonID: seasonID, SelfUserID: id, RowLimit: int32(limit)})
	if err != nil {
		return nil, err
	}
	out := make([]CompactPlayer, 0, len(rows))
	for _, row := range rows {
		p := CompactPlayer{UserID: storekit.UUIDVal(row.UserID), DisplayName: row.DisplayName, AvatarURL: row.AvatarUrl, MMR: int(row.Mmr)}
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

func (s *PGStore) CountFriends(ctx context.Context, userID string) (int, error) {
	userUUID, err := storekit.ProfileUUID(userID)
	if err != nil {
		return 0, err
	}
	count, err := s.q().CountFriends(ctx, userUUID)
	return int(count), err
}

func (s *PGStore) SendAllowed(ctx context.Context, userID, targetID string) (bool, error) {
	userUUID, err := storekit.ProfileUUID(userID)
	if err != nil {
		return false, err
	}
	targetUUID, err := storekit.ProfileUUID(targetID)
	if err != nil {
		return false, err
	}
	allowed, err := s.q().CanSendFriendRequest(ctx, db.CanSendFriendRequestParams{BlockerUserID: userUUID, ID: targetUUID})
	if err != nil {
		return false, err
	}
	return allowed.Bool, nil
}

func (s *PGStore) CrossedRequest(ctx context.Context, senderID, recipientID string) (string, bool, error) {
	senderUUID, err := storekit.ProfileUUID(senderID)
	if err != nil {
		return "", false, err
	}
	recipientUUID, err := storekit.ProfileUUID(recipientID)
	if err != nil {
		return "", false, err
	}
	id, err := s.q().FindCrossedFriendRequest(ctx, db.FindCrossedFriendRequestParams{RecipientUserID: recipientUUID, SenderUserID: senderUUID})
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return storekit.UUIDVal(id), true, nil
}

func (s *PGStore) InsertFriendRequest(ctx context.Context, senderID, recipientID string, expiresAt time.Time) (FriendRequest, error) {
	senderUUID, err := storekit.ProfileUUID(senderID)
	if err != nil {
		return FriendRequest{}, err
	}
	recipientUUID, err := storekit.ProfileUUID(recipientID)
	if err != nil {
		return FriendRequest{}, err
	}
	requestUUID, err := storekit.ProfileUUID(entityid.New())
	if err != nil {
		return FriendRequest{}, err
	}
	row, err := s.q().UpsertFriendRequest(ctx, db.UpsertFriendRequestParams{ID: requestUUID, SenderUserID: senderUUID, RecipientUserID: recipientUUID, ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true}})
	if err != nil {
		return FriendRequest{}, err
	}
	return FriendRequest{ID: storekit.UUIDVal(row.ID), Direction: "outgoing", CreatedAt: row.CreatedAt.Time, ExpiresAt: row.ExpiresAt.Time}, nil
}

func (s *PGStore) AcceptFriendRequest(ctx context.Context, requestID, recipientID string) error {
	tx, err := s.requireTx()
	if err != nil {
		return err
	}
	return acceptFriendRequestTx(ctx, tx, requestID, recipientID)
}

func (s *PGStore) NotifyUser(ctx context.Context, userID, notificationType, dedupeKey string, payload map[string]any, actorID string) error {
	tx, err := s.requireTx()
	if err != nil {
		return err
	}
	// A notification about something that expires, such as a request, ends with it.
	expiresAt, _ := time.Parse(time.RFC3339, fmt.Sprint(payload["expiresAt"]))
	_, err = storekit.UpsertUserNotificationTx(ctx, tx, userID, notificationType, dedupeKey, payload, actorID, expiresAt)
	return err
}

func acceptFriendRequestTx(ctx context.Context, tx pgx.Tx, requestID, recipientID string) error {
	requestUUID, err := storekit.ProfileUUID(requestID)
	if err != nil {
		return ErrNotFound
	}
	recipientUUID, err := storekit.ProfileUUID(recipientID)
	if err != nil {
		return ErrNotFound
	}
	q := db.New(tx)
	senderID, err := q.AcceptFriendRequest(ctx, db.AcceptFriendRequestParams{ID: requestUUID, RecipientUserID: recipientUUID})
	if err != nil {
		return ErrNotFound
	}
	return q.InsertFriendship(ctx, db.InsertFriendshipParams{UserA: senderID, UserB: recipientUUID, CreatedFromRequestID: requestUUID})
}

func (s *PGStore) FriendRequestSender(ctx context.Context, requestID, recipientID string) (string, error) {
	userUUID, err := storekit.ProfileUUID(recipientID)
	if err != nil {
		return "", ErrNotFound
	}
	requestUUID, err := storekit.ProfileUUID(requestID)
	if err != nil {
		return "", ErrNotFound
	}
	senderUUID, err := s.q().FriendRequestSender(ctx, db.FriendRequestSenderParams{ID: requestUUID, RecipientUserID: userUUID})
	if err != nil {
		return "", ErrNotFound
	}
	return storekit.UUIDVal(senderUUID), nil
}

func (s *PGStore) DeclineFriendRequest(ctx context.Context, requestID, recipientID string) error {
	userUUID, err := storekit.ProfileUUID(recipientID)
	if err != nil {
		return ErrNotFound
	}
	requestUUID, err := storekit.ProfileUUID(requestID)
	if err != nil {
		return ErrNotFound
	}
	affected, err := s.q().DeclineFriendRequest(ctx, db.DeclineFriendRequestParams{ID: requestUUID, RecipientUserID: userUUID})
	if err != nil || affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PGStore) CancelFriendRequest(ctx context.Context, requestID, senderID string) error {
	userUUID, err := storekit.ProfileUUID(senderID)
	if err != nil {
		return ErrNotFound
	}
	requestUUID, err := storekit.ProfileUUID(requestID)
	if err != nil {
		return ErrNotFound
	}
	affected, err := s.q().CancelFriendRequest(ctx, db.CancelFriendRequestParams{ID: requestUUID, SenderUserID: userUUID})
	if err != nil || affected == 0 {
		return ErrNotFound
	}
	return nil
}

func (s *PGStore) MarkFriendRequestNotificationRead(ctx context.Context, requestID string) error {
	return s.q().MarkFriendRequestNotificationRead(ctx, storekit.IngestText(requestID))
}

func (s *PGStore) RemoveFriend(ctx context.Context, userID, targetID string) error {
	userUUID, err := storekit.ProfileUUID(userID)
	if err != nil {
		return err
	}
	targetUUID, err := storekit.ProfileUUID(targetID)
	if err != nil {
		return err
	}
	return s.q().RemoveFriend(ctx, db.RemoveFriendParams{UserA: userUUID, UserB: targetUUID})
}

func (s *PGStore) AddUserBlock(ctx context.Context, userID, targetID string) error {
	userUUID, err := storekit.ProfileUUID(userID)
	if err != nil {
		return err
	}
	targetUUID, err := storekit.ProfileUUID(targetID)
	if err != nil {
		return err
	}
	return s.q().AddUserBlock(ctx, db.AddUserBlockParams{BlockerUserID: userUUID, BlockedUserID: targetUUID})
}

func (s *PGStore) RemoveUserBlock(ctx context.Context, userID, targetID string) error {
	userUUID, err := storekit.ProfileUUID(userID)
	if err != nil {
		return err
	}
	targetUUID, err := storekit.ProfileUUID(targetID)
	if err != nil {
		return err
	}
	return s.q().RemoveUserBlock(ctx, db.RemoveUserBlockParams{BlockerUserID: userUUID, BlockedUserID: targetUUID})
}

func (s *PGStore) CancelPairFriendRequests(ctx context.Context, userID, targetID string) error {
	userUUID, err := storekit.ProfileUUID(userID)
	if err != nil {
		return err
	}
	targetUUID, err := storekit.ProfileUUID(targetID)
	if err != nil {
		return err
	}
	return s.q().CancelPairFriendRequests(ctx, db.CancelPairFriendRequestsParams{UserA: userUUID, UserB: targetUUID})
}

func (s *PGStore) RevokeFriendCodes(ctx context.Context, userID string) error {
	userUUID, err := storekit.ProfileUUID(userID)
	if err != nil {
		return err
	}
	return s.q().RevokeFriendCodes(ctx, userUUID)
}

func (s *PGStore) InsertFriendCode(ctx context.Context, userID, code string, expiresAt time.Time) error {
	userUUID, err := storekit.ProfileUUID(userID)
	if err != nil {
		return err
	}
	return s.q().InsertFriendCode(ctx, db.InsertFriendCodeParams{Code: code, UserID: userUUID, ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true}})
}

func (s *PGStore) ResolveFriendCode(ctx context.Context, userID, code string) (CompactPlayer, error) {
	code = strings.ToUpper(strings.TrimSpace(code))
	userUUID, err := storekit.ProfileUUID(userID)
	if err != nil {
		return CompactPlayer{}, ErrNotFound
	}
	seasonID, err := storekit.ActiveSeasonID(ctx, s.pool)
	if err != nil {
		return CompactPlayer{}, ErrNotFound
	}
	row, err := s.q().ResolveFriendCode(ctx, db.ResolveFriendCodeParams{SeasonID: seasonID, Code: code, SelfUserID: userUUID})
	if err != nil {
		return CompactPlayer{}, ErrNotFound
	}
	p := CompactPlayer{UserID: storekit.UUIDVal(row.UserID), DisplayName: row.DisplayName, AvatarURL: row.AvatarUrl, MMR: int(row.Mmr)}
	if row.LastSeenAt.Valid {
		value := row.LastSeenAt.Time
		p.LastSeenAt = &value
	}
	p.Relationship, p.RequestID, _ = s.Relationship(ctx, userID, p.UserID)
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

const friendCodeAlphabet = "23456789ABCDEFGHJKMNPQRSTUVWXYZ"

func (s *PGStore) InvitationEligibility(ctx context.Context, partyID, inviterID, recipientID string) (PartyInvitation, error) {
	partyUUID, err := storekit.ProfileUUID(partyID)
	if err != nil {
		return PartyInvitation{}, ErrBlocked
	}
	inviterUUID, err := storekit.ProfileUUID(inviterID)
	if err != nil {
		return PartyInvitation{}, ErrBlocked
	}
	recipientUUID, err := storekit.ProfileUUID(recipientID)
	if err != nil {
		return PartyInvitation{}, ErrBlocked
	}
	eligibility, err := s.q().PartyInvitationEligibility(ctx, db.PartyInvitationEligibilityParams{PartyID: partyUUID, InviterUserID: inviterUUID, RecipientUserID: recipientUUID})
	if err != nil {
		return PartyInvitation{}, ErrBlocked
	}
	return PartyInvitation{PartyID: partyID, InviteCode: eligibility.InviteCode, Mode: string(eligibility.Mode), MemberCount: int(eligibility.MemberCount)}, nil
}

func (s *PGStore) PendingPartyInvitation(ctx context.Context, partyID, recipientID string) (PartyInvitation, bool, error) {
	partyUUID, err := storekit.ProfileUUID(partyID)
	if err != nil {
		return PartyInvitation{}, false, nil
	}
	recipientUUID, err := storekit.ProfileUUID(recipientID)
	if err != nil {
		return PartyInvitation{}, false, nil
	}
	row, err := s.q().GetPendingPartyInvitation(ctx, db.GetPendingPartyInvitationParams{PartyID: partyUUID, RecipientUserID: recipientUUID})
	if errors.Is(err, pgx.ErrNoRows) {
		return PartyInvitation{}, false, nil
	}
	if err != nil {
		return PartyInvitation{}, false, err
	}
	return PartyInvitation{ID: storekit.UUIDVal(row.ID), PartyID: partyID, CreatedAt: row.CreatedAt.Time, ExpiresAt: row.ExpiresAt.Time}, true, nil
}

func (s *PGStore) UpsertPartyInvitation(ctx context.Context, partyID, inviterID, recipientID string, expiresAt, createdAt time.Time) (PartyInvitation, error) {
	partyUUID, err := storekit.ProfileUUID(partyID)
	if err != nil {
		return PartyInvitation{}, ErrBlocked
	}
	inviterUUID, err := storekit.ProfileUUID(inviterID)
	if err != nil {
		return PartyInvitation{}, ErrBlocked
	}
	recipientUUID, err := storekit.ProfileUUID(recipientID)
	if err != nil {
		return PartyInvitation{}, ErrBlocked
	}
	invitationUUID, err := storekit.ProfileUUID(entityid.New())
	if err != nil {
		return PartyInvitation{}, err
	}
	row, err := s.q().UpsertPartyInvitation(ctx, db.UpsertPartyInvitationParams{
		ID: invitationUUID, PartyID: partyUUID, InviterUserID: inviterUUID, RecipientUserID: recipientUUID,
		ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: true}, CreatedAt: pgtype.Timestamptz{Time: createdAt, Valid: true},
	})
	if err != nil {
		return PartyInvitation{}, err
	}
	return PartyInvitation{ID: storekit.UUIDVal(row.ID), PartyID: partyID, CreatedAt: row.CreatedAt.Time, ExpiresAt: row.ExpiresAt.Time}, nil
}

func (s *PGStore) MarkPartyInvitationNotificationRead(ctx context.Context, userID, invitationID string) error {
	userUUID, err := storekit.ProfileUUID(userID)
	if err != nil {
		return err
	}
	return s.q().MarkPartyInvitationNotificationRead(ctx, db.MarkPartyInvitationNotificationReadParams{UserID: userUUID, InvitationID: storekit.IngestText(invitationID)})
}

func (s *PGStore) ListPartyInviteStatus(ctx context.Context, inviterID, partyID string) (map[string]CompactPartyInvite, error) {
	out := map[string]CompactPartyInvite{}
	partyUUID, err := storekit.ProfileUUID(partyID)
	if err != nil {
		return out, nil
	}
	inviterUUID, err := storekit.ProfileUUID(inviterID)
	if err != nil {
		return out, nil
	}
	rows, err := s.q().ListOutgoingPartyInvitations(ctx, db.ListOutgoingPartyInvitationsParams{
		InviterUserID: inviterUUID, PartyID: partyUUID,
	})
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		out[storekit.UUIDVal(row.RecipientUserID)] = CompactPartyInvite{ID: storekit.UUIDVal(row.ID), CreatedAt: row.CreatedAt.Time, ExpiresAt: row.ExpiresAt.Time}
	}
	return out, nil
}

func (s *PGStore) ListPartyInvitations(ctx context.Context, userID string, limit int) ([]PartyInvitation, error) {
	limit = BoundedLimit(limit, 10, 50)
	userUUID, err := storekit.ProfileUUID(userID)
	if err != nil {
		return nil, err
	}
	rows, err := s.q().ListPartyInvitations(ctx, db.ListPartyInvitationsParams{RecipientUserID: userUUID, Limit: int32(limit)})
	if err != nil {
		return nil, err
	}
	out := make([]PartyInvitation, 0, len(rows))
	for _, row := range rows {
		item := PartyInvitation{ID: storekit.UUIDVal(row.InvitationID), PartyID: storekit.UUIDVal(row.PartyID), InviteCode: row.InviteCode, Mode: string(row.Mode), MemberCount: int(row.MemberCount), CreatedAt: row.CreatedAt.Time, ExpiresAt: row.ExpiresAt.Time}
		item.Inviter = CompactPlayer{UserID: storekit.UUIDVal(row.InviterID), DisplayName: row.DisplayName, AvatarURL: row.AvatarUrl}
		out = append(out, item)
	}
	return out, nil
}

func (s *PGStore) RespondPartyInvitation(ctx context.Context, userID, invitationID, response string) (PartyInvitation, error) {
	status := "declined"
	if response == "accept" {
		status = "accepted"
	}
	userUUID, err := storekit.ProfileUUID(userID)
	if err != nil {
		return PartyInvitation{}, ErrNotFound
	}
	invitationUUID, err := storekit.ProfileUUID(invitationID)
	if err != nil {
		return PartyInvitation{}, ErrNotFound
	}
	q := s.q()
	row, err := q.RespondPartyInvitation(ctx, db.RespondPartyInvitationParams{ID: invitationUUID, RecipientUserID: userUUID, Status: db.GdSocialRequestStatus(status)})
	if err != nil {
		return PartyInvitation{}, ErrNotFound
	}
	return PartyInvitation{ID: storekit.UUIDVal(row.InvitationID), PartyID: storekit.UUIDVal(row.PartyID), InviteCode: row.InviteCode, Mode: string(row.Mode), ExpiresAt: row.ExpiresAt.Time}, nil
}
