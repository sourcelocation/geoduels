package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func (s *pgStore) ClaimPendingNotification(notificationType string, now time.Time) (NotificationOutboxItem, bool, error) {
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
	row := tx.QueryRow(ctx, `
		with candidate as (
			select id
			from notification_outbox
			where type = $1
				and sent_at is null
				and next_attempt_at <= $2
			order by next_attempt_at asc, id asc
			limit 1
			for update skip locked
		)
		update notification_outbox n
		set attempts = n.attempts + 1,
			next_attempt_at = $3,
			last_error = null
		from candidate
		where n.id = candidate.id
		returning n.id, n.type, n.payload_json::text, n.attempts
	`, notificationType, now, now.Add(5*time.Minute))
	var item NotificationOutboxItem
	var raw string
	if err := row.Scan(&item.ID, &item.Type, &raw, &item.Attempts); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return NotificationOutboxItem{}, false, nil
		}
		return NotificationOutboxItem{}, false, err
	}
	item.PayloadJSON = []byte(raw)
	if err := tx.Commit(ctx); err != nil {
		return NotificationOutboxItem{}, false, err
	}
	return item, true, nil
}

func (s *pgStore) MarkNotificationSent(id int64) error {
	if id <= 0 {
		return errors.New("notification id required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	_, err := s.pool.Exec(ctx, `
		update notification_outbox
		set sent_at = now(),
			last_error = null
		where id = $1
	`, id)
	return err
}

func (s *pgStore) MarkNotificationFailed(id int64, nextAttemptAt time.Time, lastError string) error {
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
	_, err := s.pool.Exec(ctx, `
		update notification_outbox
		set next_attempt_at = $2,
			last_error = nullif($3, '')
		where id = $1
			and sent_at is null
	`, id, nextAttemptAt, lastError)
	return err
}

func upsertUserNotification(ctx context.Context, tx pgx.Tx, userID, notificationType, dedupeKey string, payload any, id *int64) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return tx.QueryRow(ctx, `
		with inserted as (
			insert into user_notifications(user_id, type, dedupe_key, payload_json)
			values($1, $2, $3, $4::jsonb)
			on conflict (dedupe_key) do update set payload_json = excluded.payload_json
			returning id
		)
		select id from inserted
	`, userID, notificationType, dedupeKey, string(body)).Scan(id)
}

func mustJSON(value any) string {
	body, err := json.Marshal(value)
	if err != nil {
		return "null"
	}
	return string(body)
}

func enqueueNotificationOutbox(ctx context.Context, tx pgx.Tx, notificationType, dedupeKey string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		insert into notification_outbox(type, dedupe_key, payload_json, next_attempt_at)
		values($1, $2, $3::jsonb, now())
		on conflict (dedupe_key) do update set
			payload_json = excluded.payload_json,
			next_attempt_at = now(),
			sent_at = null,
			last_error = null
	`, notificationType, dedupeKey, string(body))
	return err
}

func (s *pgStore) ListUserNotifications(userID string, limit int) ([]UserNotification, error) {
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
	rows, err := s.pool.Query(ctx, `
		select id, type, payload_json::text, created_at
		from user_notifications
		where user_id = $1
			and read_at is null
		order by created_at desc, id desc
		limit $2
	`, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []UserNotification{}
	for rows.Next() {
		var item UserNotification
		var raw string
		if err := rows.Scan(&item.ID, &item.Type, &raw, &item.CreatedAt); err != nil {
			return nil, err
		}
		item.Payload = json.RawMessage(raw)
		out = append(out, item)
	}
	return out, rows.Err()
}

func (s *pgStore) MarkUserNotificationRead(userID string, notificationID int64) error {
	userID = strings.TrimSpace(userID)
	if userID == "" || notificationID <= 0 {
		return errors.New("userID and notificationID required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	_, err := s.pool.Exec(ctx, `
		update user_notifications
		set read_at = coalesce(read_at, now())
		where id = $1 and user_id = $2
	`, notificationID, userID)
	return err
}
