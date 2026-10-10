package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"geoduels/internal/storekit"
	"geoduels/pkg/contracts"
	db "geoduels/pkg/persistence/sqlc/db"
)

const queryTimeout = 4 * time.Second

// PGStore owns PostgreSQL access for user notifications.
type PGStore struct {
	pool *pgxpool.Pool
}

func NewPGStore(pool *pgxpool.Pool) *PGStore { return &PGStore{pool: pool} }

func (s *PGStore) q() *db.Queries { return db.New(s.pool) }

func (s *PGStore) ListUserNotifications(ctx context.Context, userID string, limit int) ([]contracts.UserNotification, error) {
	u, err := requireUser(userID)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	rows, err := s.q().ListUserNotifications(ctx, db.ListUserNotificationsParams{UserID: u, Limit: int32(clamp(limit, 20, 50))})
	if err != nil {
		return nil, err
	}
	out := make([]contracts.UserNotification, 0, len(rows))
	for _, row := range rows {
		out = append(out, notification(row.ID, row.Type, row.PayloadJson, row.CreatedAt, row.ActorUserID, row.ActorDisplayName))
	}
	return out, nil
}

func (s *PGStore) ListNotificationInbox(ctx context.Context, userID string, limit int, beforeID int64) ([]contracts.UserNotification, error) {
	u, err := requireUser(userID)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	rows, err := s.q().ListNotificationInbox(ctx, db.ListNotificationInboxParams{UserID: u, BeforeID: beforeID, RowLimit: int32(clamp(limit, 30, 100))})
	if err != nil {
		return nil, err
	}
	out := make([]contracts.UserNotification, 0, len(rows))
	for _, row := range rows {
		item := notification(row.ID, row.Type, row.PayloadJson, row.CreatedAt, row.ActorUserID, row.ActorDisplayName)
		item.Category = string(row.Category)
		if row.ReadAt.Valid {
			value := row.ReadAt.Time
			item.ReadAt = &value
		}
		out = append(out, item)
	}
	return out, nil
}

func (s *PGStore) ListUnseenSince(ctx context.Context, userIDs []string, since time.Time) ([]Announcement, error) {
	ids := make([]pgtype.UUID, 0, len(userIDs))
	for _, userID := range userIDs {
		if u, err := storekit.ProfileUUID(userID); err == nil {
			ids = append(ids, u)
		}
	}
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	rows, err := s.q().ListUnseenNotificationsSince(ctx, db.ListUnseenNotificationsSinceParams{UserIds: ids, Since: pgtype.Timestamptz{Time: since, Valid: true}})
	if err != nil {
		return nil, err
	}
	out := make([]Announcement, 0, len(rows))
	for _, row := range rows {
		out = append(out, Announcement{
			UserID:       storekit.UUIDVal(row.UserID),
			Notification: notification(row.ID, row.Type, row.PayloadJson, row.CreatedAt, row.ActorUserID, row.ActorDisplayName),
		})
	}
	return out, nil
}

func (s *PGStore) MarkUserNotificationRead(ctx context.Context, userID string, notificationID int64) error {
	u, err := requireUser(userID)
	if err != nil {
		return err
	}
	if notificationID <= 0 {
		return errors.New("notification id required")
	}
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	return s.q().MarkUserNotificationRead(ctx, db.MarkUserNotificationReadParams{ID: notificationID, UserID: u})
}

func (s *PGStore) MarkAllUserNotificationsRead(ctx context.Context, userID string) error {
	u, err := requireUser(userID)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, queryTimeout)
	defer cancel()
	return s.q().MarkAllUserNotificationsRead(ctx, u)
}

func notification(id int64, kind db.GdNotificationType, payload []byte, createdAt pgtype.Timestamptz, actor pgtype.UUID, actorName string) contracts.UserNotification {
	return contracts.UserNotification{
		ID: id, Type: string(kind), Payload: json.RawMessage(payload), CreatedAt: createdAt.Time,
		ActorUserID: storekit.UUIDVal(actor), ActorDisplayName: strings.TrimSpace(actorName),
	}
}

func requireUser(userID string) (pgtype.UUID, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return pgtype.UUID{}, errors.New("userID required")
	}
	return storekit.ProfileUUID(userID)
}

func clamp(limit, fallback, max int) int {
	if limit <= 0 {
		return fallback
	}
	return min(limit, max)
}
