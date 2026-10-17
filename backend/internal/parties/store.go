package parties

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"geoduels/internal/storekit"
	"geoduels/pkg/gameplay"
	"geoduels/pkg/matchkind"
	db "geoduels/pkg/persistence/sqlc/db"
)

// MapResolver resolves the default/party map. Implemented by maps.PGStore.
type MapResolver interface {
	ResolveGameplayMapID(mode matchkind.Mode, ruleset gameplay.Ruleset, requestedMapID string) (string, error)
}

// PGStore persists parties data using the pool, or the transaction supplied
// by WithinTx. The original store is never mutated when starting a transaction.
type PGStore struct {
	pool *pgxpool.Pool
	tx   pgx.Tx
	maps MapResolver
}

func NewPGStore(pool *pgxpool.Pool, maps MapResolver) *PGStore {
	return &PGStore{pool: pool, maps: maps}
}

// q selects the transaction when present and the pool otherwise.
func (s *PGStore) q() *db.Queries {
	if s.tx != nil {
		return db.New(s.tx)
	}
	return db.New(s.pool)
}

func (s *PGStore) requireTx() (pgx.Tx, error) {
	if s.tx == nil {
		return nil, errors.New("parties store: operation requires a transaction")
	}
	return s.tx, nil
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

func (s *PGStore) ResolveGameplayMapID(mode matchkind.Mode, ruleset gameplay.Ruleset, requestedMapID string) (string, error) {
	return s.maps.ResolveGameplayMapID(mode, ruleset, requestedMapID)
}
