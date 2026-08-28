// Package controlplane contains the small, independently testable pieces used
// by the HTTP, matchmaker, and job composition roots. It deliberately has no
// dependency on a service main package.
package controlplane

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	controlplanequeries "geoduels/pkg/persistence/sqlc/db"
)

var ErrLeaseNotHeld = errors.New("control-plane lease is not held")

// Lease is a fencing lease. Token must be included in durable launch writes
// once those writes are moved to the control-plane schema.
type Lease struct {
	Name  string
	Owner string
	Token int64
}

// LeaseStore is intentionally consumer-sized. It is also the boundary that
// makes a matchmaker role testable without Redis or an HTTP server.
type LeaseStore interface {
	Acquire(context.Context, string, string, time.Duration) (Lease, bool, error)
	Renew(context.Context, Lease, time.Duration) (bool, error)
	Release(context.Context, Lease) error
}

type PostgresLeaseStore struct {
	pool    *pgxpool.Pool
	queries *controlplanequeries.Queries
}

// OpenPostgresLeaseStore is a composition-root helper. Callers own Close on
// the returned pool so process shutdown ordering remains explicit.
func OpenPostgresLeaseStore(ctx context.Context, dsn string) (*PostgresLeaseStore, func(), error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, nil, err
	}
	store, err := NewPostgresLeaseStore(pool)
	if err != nil {
		pool.Close()
		return nil, nil, err
	}
	return store, pool.Close, nil
}

func NewPostgresLeaseStore(pool *pgxpool.Pool) (*PostgresLeaseStore, error) {
	if pool == nil {
		return nil, errors.New("control-plane lease pool is required")
	}
	return &PostgresLeaseStore{pool: pool, queries: controlplanequeries.New(pool)}, nil
}

func (s *PostgresLeaseStore) Acquire(ctx context.Context, name, owner string, ttl time.Duration) (Lease, bool, error) {
	if ttl <= 0 {
		return Lease{}, false, fmt.Errorf("lease ttl must be positive")
	}
	var lease Lease
	lease.Name, lease.Owner = name, owner
	row, err := s.queries.AcquireLease(ctx, controlplanequeries.AcquireLeaseParams{
		Name: name, OwnerID: owner, Ttl: interval(ttl),
	})
	if err != nil {
		// pgx returns ErrNoRows when another non-expired owner holds the row.
		if errors.Is(err, pgx.ErrNoRows) {
			// A standby contender has no fencing token yet, but it must retain its
			// stable owner ID so a later retry can acquire the lease.
			return lease, false, nil
		}
		return Lease{}, false, err
	}
	lease.Token = row.FencingToken
	return lease, true, nil
}

func (s *PostgresLeaseStore) Renew(ctx context.Context, lease Lease, ttl time.Duration) (bool, error) {
	if ttl <= 0 {
		return false, fmt.Errorf("lease ttl must be positive")
	}
	_, err := s.queries.RenewLease(ctx, controlplanequeries.RenewLeaseParams{
		Name: lease.Name, OwnerID: lease.Owner, FencingToken: lease.Token, Ttl: interval(ttl),
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (s *PostgresLeaseStore) Release(ctx context.Context, lease Lease) error {
	return s.queries.ReleaseLease(ctx, controlplanequeries.ReleaseLeaseParams{
		Name: lease.Name, OwnerID: lease.Owner, FencingToken: lease.Token,
	})
}

func interval(value time.Duration) pgtype.Interval {
	return pgtype.Interval{Microseconds: value.Microseconds(), Valid: true}
}
