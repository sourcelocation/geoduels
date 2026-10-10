// Package ratelimit counts hits in fixed windows kept in Postgres, shared by every process.
package ratelimit

import (
	"context"
	"time"

	db "geoduels/pkg/persistence/sqlc/db"
)

// Hit counts one hit against the key's window. It reports whether the hit was within limit and,
// when it was not, how long until the window ends. A limit of zero or less allows everything.
func Hit(ctx context.Context, q db.DBTX, key string, limit int, window time.Duration) (bool, time.Duration, error) {
	if limit <= 0 {
		return true, 0, nil
	}
	row, err := db.New(q).HitRateLimit(ctx, db.HitRateLimitParams{Key: key, WindowSeconds: window.Seconds()})
	if err != nil {
		return false, 0, err
	}
	if int(row.Hits) <= limit {
		return true, 0, nil
	}
	return false, max(time.Until(row.WindowStartedAt.Time.Add(window)), time.Second), nil
}
