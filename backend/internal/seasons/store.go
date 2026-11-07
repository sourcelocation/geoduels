package seasons

import (
	"context"
	"geoduels/internal/jobs"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"geoduels/internal/storekit"
	db "geoduels/pkg/persistence/sqlc/db"
)

// PGStore owns PostgreSQL access for this feature.
type PGStore struct {
	pool   *pgxpool.Pool
	db     *db.Queries
	teller jobs.Teller
}

func NewPGStore(pool *pgxpool.Pool, teller jobs.Teller) *PGStore {
	return &PGStore{pool: pool, db: db.New(pool), teller: teller}
}

func (s *PGStore) withTx(ctx context.Context, fn func(pgx.Tx) error) error {
	return storekit.WithTx(ctx, s.pool, fn)
}
