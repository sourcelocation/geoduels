package profiles

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"geoduels/internal/storekit"
	db "geoduels/pkg/persistence/sqlc/db"
)

// TouchLastSeen records when the player was last seen, as their profile shows it.
func TouchLastSeen(ctx context.Context, q db.DBTX, userID string, seenAt time.Time) error {
	id, err := storekit.ProfileUUID(userID)
	if err != nil {
		return err
	}
	return db.New(q).TouchLastSeen(ctx, db.TouchLastSeenParams{ID: id, LastSeenAt: pgtype.Timestamptz{Time: seenAt, Valid: true}})
}
