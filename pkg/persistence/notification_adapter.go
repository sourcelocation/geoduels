package persistence

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	db "geoduels/pkg/persistence/sqlc/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

type notificationTxAdapter struct{ queries *db.Queries }

func newNotificationTxAdapter(tx pgx.Tx) notificationTxAdapter {
	return notificationTxAdapter{queries: db.New(tx)}
}

func uuidValue(value string) (pgtype.UUID, error) {
	var id pgtype.UUID
	if err := id.Scan(strings.TrimSpace(value)); err != nil {
		return id, err
	}
	return id, nil
}

func (a notificationTxAdapter) upsert(ctx context.Context, userID, typ, dedupe string, payload any, id *int64) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	u, err := uuidValue(userID)
	if err != nil {
		return err
	}
	row, err := a.queries.UpsertUserNotification(ctx, db.UpsertUserNotificationParams{UserID: u, NotificationType: db.GdNotificationType(typ), DedupeKey: dedupe, Payload: body})
	if err != nil {
		return err
	}
	*id = row
	return nil
}

func (a notificationTxAdapter) reporters(ctx context.Context, subject string) ([]string, error) {
	u, err := uuidValue(subject)
	if err != nil {
		return nil, err
	}
	return a.queries.ListReporters(ctx, u)
}
func (a notificationTxAdapter) enqueue(ctx context.Context, typ, dedupe string, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	return a.queries.EnqueueNotificationOutbox(ctx, db.EnqueueNotificationOutboxParams{NotificationType: db.GdNotificationOutboxType(typ), DedupeKey: dedupe, Payload: body})
}

func notificationItem(row db.ClaimPendingNotificationRow) NotificationOutboxItem {
	return NotificationOutboxItem{ID: row.ID, Type: string(row.Type), PayloadJSON: json.RawMessage(row.PayloadJson), Attempts: int(row.Attempts)}
}
func requireNotificationID(id int64) (int64, error) {
	if id <= 0 {
		return 0, fmt.Errorf("notification id required")
	}
	return id, nil
}
func timestamptz(t time.Time) pgtype.Timestamptz { return pgtype.Timestamptz{Time: t, Valid: true} }
