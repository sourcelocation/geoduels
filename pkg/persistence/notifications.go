package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	db "geoduels/pkg/persistence/sqlc/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func (s *DB) ClaimPendingNotification(notificationType string, now time.Time) (NotificationOutboxItem, bool, error) {
	notificationType = strings.TrimSpace(notificationType)
	if notificationType == "" {
		return NotificationOutboxItem{}, false, errors.New("notification type required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return NotificationOutboxItem{}, false, err
	}
	defer tx.Rollback(ctx)
	q := db.New(tx)
	var nt pgtype.Timestamptz = timestamptz(now)
	row, scanErr := q.ClaimPendingNotification(ctx, db.ClaimPendingNotificationParams{LeaseUntil: timestamptz(now.Add(5 * time.Minute)), NotificationType: db.GdNotificationOutboxType(notificationType), Now: nt})
	if scanErr != nil {
		if errors.Is(scanErr, pgx.ErrNoRows) {
			return NotificationOutboxItem{}, false, nil
		}
		return NotificationOutboxItem{}, false, scanErr
	}
	if err := tx.Commit(ctx); err != nil {
		return NotificationOutboxItem{}, false, err
	}
	return notificationItem(row), true, nil
}

func (s *DB) MarkNotificationSent(id int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	value, err := requireNotificationID(id)
	if err != nil {
		return err
	}
	return s.db.MarkNotificationSent(ctx, value)
}

func (s *DB) MarkNotificationFailed(id int64, nextAttemptAt time.Time, lastError string) error {
	if id <= 0 {
		return errors.New("notification id required")
	}
	lastError = strings.TrimSpace(lastError)
	if len(lastError) > 1000 {
		lastError = lastError[:1000]
	}
	if nextAttemptAt.IsZero() {
		nextAttemptAt = time.Now().Add(time.Minute)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	value, err := requireNotificationID(id)
	if err != nil {
		return err
	}
	return s.db.MarkNotificationFailed(ctx, db.MarkNotificationFailedParams{NextAttemptAt: timestamptz(nextAttemptAt), LastError: lastError, OutboxID: value})
}

func upsertUserNotification(ctx context.Context, tx pgx.Tx, userID, notificationType, dedupeKey string, payload any, id *int64) error {
	return newNotificationTxAdapter(tx).upsert(ctx, userID, notificationType, dedupeKey, payload, id)
}

func notifyAccountEnforcement(ctx context.Context, tx pgx.Tx, userID, action, reason string, moderationLogID int64, endsAt any) error {
	notificationType := "account_banned"
	if action == "unban" {
		notificationType = "account_unbanned"
	}
	var notificationID int64
	return upsertUserNotification(ctx, tx, userID, notificationType, fmt.Sprintf("%s:%d", notificationType, moderationLogID), map[string]any{
		"reason":          strings.TrimSpace(reason),
		"action":          action,
		"moderationLogId": moderationLogID,
		"endsAt":          endsAt,
	}, &notificationID)
}

func notifyReportersOfBan(ctx context.Context, tx pgx.Tx, subjectUserID, action string, logID int64) error {
	reporterIDs, err := newNotificationTxAdapter(tx).reporters(ctx, subjectUserID)
	if err != nil {
		return err
	}
	for _, reporterID := range reporterIDs {
		var notificationID int64
		if err := upsertUserNotification(ctx, tx, reporterID, "reported_player_banned", fmt.Sprintf("reported_player_banned:%d:%s", logID, reporterID), map[string]any{
			"action":          action,
			"moderationLogId": logID,
		}, &notificationID); err != nil {
			return err
		}
	}
	return nil
}

func mustJSON(value any) string {
	body, err := json.Marshal(value)
	if err != nil {
		return "null"
	}
	return string(body)
}

func enqueueNotificationOutbox(ctx context.Context, tx pgx.Tx, notificationType, dedupeKey string, payload any) error {
	return newNotificationTxAdapter(tx).enqueue(ctx, notificationType, dedupeKey, payload)
}

func (s *DB) ListUserNotifications(userID string, limit int) ([]UserNotification, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, errors.New("userID required")
	}
	if limit <= 0 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	u, err := uuidValue(userID)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.ListUserNotifications(ctx, db.ListUserNotificationsParams{UserID: u, Limit: int32(limit)})
	if err != nil {
		return nil, err
	}
	out := make([]UserNotification, 0, len(rows))
	for _, row := range rows {
		out = append(out, UserNotification{ID: row.ID, Type: string(row.Type), Payload: json.RawMessage(row.PayloadJson), CreatedAt: row.CreatedAt.Time})
	}
	return out, nil
}

func (s *DB) MarkUserNotificationRead(userID string, notificationID int64) error {
	userID = strings.TrimSpace(userID)
	if userID == "" || notificationID <= 0 {
		return errors.New("userID and notificationID required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	u, err := uuidValue(userID)
	if err != nil {
		return err
	}
	id, err := requireNotificationID(notificationID)
	if err != nil {
		return err
	}
	return s.db.MarkUserNotificationRead(ctx, db.MarkUserNotificationReadParams{ID: id, UserID: u})
}

func (s *DB) ListNotificationInbox(userID string, limit int, beforeID int64) ([]UserNotification, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return nil, errors.New("userID required")
	}
	if limit <= 0 {
		limit = 30
	}
	if limit > 100 {
		limit = 100
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	u, err := uuidValue(userID)
	if err != nil {
		return nil, err
	}
	rows, err := s.db.ListNotificationInbox(ctx, db.ListNotificationInboxParams{UserID: u, BeforeID: beforeID, RowLimit: int32(limit)})
	if err != nil {
		return nil, err
	}
	out := make([]UserNotification, 0, len(rows))
	for _, row := range rows {
		item := UserNotification{ID: row.ID, Type: string(row.Type), Category: string(row.Category), Payload: json.RawMessage(row.PayloadJson), CreatedAt: row.CreatedAt.Time}
		if row.ReadAt.Valid {
			value := row.ReadAt.Time
			item.ReadAt = &value
		}
		out = append(out, item)
	}
	return out, nil
}

func (s *DB) MarkAllUserNotificationsRead(userID string) error {
	u, err := uuidValue(userID)
	if err != nil {
		return err
	}
	return s.db.MarkAllUserNotificationsRead(context.Background(), u)
}
