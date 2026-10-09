// Package storekit holds the small SQL helpers shared by feature stores so
// each feature package can own its PostgreSQL access without duplicating
// id/season/notification plumbing.
package storekit

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	db "geoduels/pkg/persistence/sqlc/db"
)

// DefaultSeasonID is used when the ranked season settings row has no active
// season configured yet.
const DefaultSeasonID = "s2"

// ProfileUUID parses a public entity ID into the UUID form used by SQL.
func ProfileUUID(v string) (pgtype.UUID, error) {
	var u pgtype.UUID
	if err := u.Scan(v); err != nil {
		return u, err
	}
	return u, nil
}

func MustUUID(v string) pgtype.UUID {
	u, _ := ProfileUUID(v)
	return u
}

func UUIDVal(value pgtype.UUID) string {
	if !value.Valid {
		return ""
	}
	return value.String()
}

func IngestText(value string) pgtype.Text {
	return pgtype.Text{String: value, Valid: true}
}

func Timestamptz(t time.Time) pgtype.Timestamptz {
	return pgtype.Timestamptz{Time: t, Valid: true}
}

// QueriesFor adapts a pool, transaction, or Queries value into *db.Queries.
// WithTx runs fn inside a pool transaction.
func WithTx(ctx context.Context, pool *pgxpool.Pool, fn func(pgx.Tx) error) error {
	if pool == nil {
		return errors.New("no pool for transaction")
	}
	tx, err := pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func TextVal(value any) string {
	if value == nil {
		return ""
	}
	return fmt.Sprint(value)
}

func Nullable(v string) any {
	if v == "" {
		return nil
	}
	return v
}

func UUIDList(values []string) []pgtype.UUID {
	out := make([]pgtype.UUID, 0, len(values))
	for _, v := range values {
		out = append(out, MustUUID(v))
	}
	return out
}

func OptionalUUID(value string) (pgtype.UUID, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return pgtype.UUID{}, nil
	}
	return ProfileUUID(value)
}

func QueriesFor(source any) *db.Queries {
	if q, ok := source.(*db.Queries); ok {
		return q
	}
	if tx, ok := source.(pgx.Tx); ok {
		return db.New(tx)
	}
	if p, ok := source.(*pgxpool.Pool); ok {
		return db.New(p)
	}
	return nil
}

// ActiveSeasonID resolves the active ranked season, falling back to the
// default when settings are absent or unreadable.
func ActiveSeasonID(ctx context.Context, source any) (string, error) {
	q := QueriesFor(source)
	if q == nil {
		return "", errors.New("no query source for active season lookup")
	}
	raw, err := q.GetRankedSeasonSettings(ctx)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return DefaultSeasonID, nil
		}
		return "", err
	}
	var settings struct {
		ActiveSeasonID string `json:"activeSeasonId"`
	}
	if json.Unmarshal(raw, &settings) != nil || strings.TrimSpace(settings.ActiveSeasonID) == "" {
		return DefaultSeasonID, nil
	}
	return settings.ActiveSeasonID, nil
}

// UpsertUserNotificationTx writes a user notification inside the caller's
// transaction and returns its id. Writing the same dedupe key again announces
// it again. actorUserID is stored on the row and resolved to a display name at
// read time; a zero expiresAt never expires.
func UpsertUserNotificationTx(ctx context.Context, tx pgx.Tx, userID, notificationType, dedupeKey string, payload any, actorUserID string, expiresAt time.Time) (int64, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}
	var u pgtype.UUID
	if err := u.Scan(strings.TrimSpace(userID)); err != nil {
		return 0, err
	}
	actor, err := optionalProfileUUID(actorUserID)
	if err != nil {
		return 0, err
	}
	return db.New(tx).UpsertUserNotification(ctx, db.UpsertUserNotificationParams{
		UserID: u, Type: db.GdNotificationType(notificationType), DedupeKey: dedupeKey, PayloadJson: body, ActorUserID: actor,
		ExpiresAt: pgtype.Timestamptz{Time: expiresAt, Valid: !expiresAt.IsZero()},
	})
}

func optionalProfileUUID(value string) (pgtype.UUID, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return pgtype.UUID{}, nil
	}
	return ProfileUUID(value)
}
