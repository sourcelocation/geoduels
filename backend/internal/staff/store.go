package staff

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"geoduels/internal/audit"
	"geoduels/internal/storekit"
	"geoduels/pkg/contracts"
	db "geoduels/pkg/persistence/sqlc/db"
)

// Store persists role grants. WithinTx supplies a store whose writes commit
// together, or roll back when the callback returns an error.
type Store interface {
	WithinTx(ctx context.Context, fn func(Store) error) error
	LockUser(ctx context.Context, userID string) error
	UserRoles(ctx context.Context, userID string) ([]string, error)
	GrantRole(ctx context.Context, userID, role, actorID, reason string) error
	RevokeRole(ctx context.Context, userID, role string) error
	ListRoleGrants(ctx context.Context) ([]contracts.UserRoleGrant, error)
	RecordAudit(ctx context.Context, entry audit.Entry) (int64, error)
	AwardTeamBadge(ctx context.Context, userID string) error
	RemoveTeamBadge(ctx context.Context, userID string) error
}

type PGStore struct {
	pool *pgxpool.Pool
	tx   pgx.Tx
}

func NewPGStore(pool *pgxpool.Pool) *PGStore {
	return &PGStore{pool: pool}
}

// q selects the transaction when present and the pool otherwise.
func (a *PGStore) q() *db.Queries {
	if a.tx != nil {
		return db.New(a.tx)
	}
	return db.New(a.pool)
}

func (a *PGStore) requireTx() (pgx.Tx, error) {
	if a.tx == nil {
		return nil, errors.New("staff store: operation requires a transaction")
	}
	return a.tx, nil
}

// WithinTx runs fn against a copy bound to the current transaction.
func (a *PGStore) WithinTx(ctx context.Context, fn func(Store) error) error {
	return storekit.WithTx(ctx, a.pool, func(tx pgx.Tx) error {
		bound := *a
		bound.tx = tx
		return fn(&bound)
	})
}

func (a *PGStore) RecordAudit(ctx context.Context, entry audit.Entry) (int64, error) {
	tx, err := a.requireTx()
	if err != nil {
		return 0, err
	}
	return audit.Record(ctx, tx, entry)
}

var _ Store = (*PGStore)(nil)

func mapNoRows(err error) error {
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	return err
}
