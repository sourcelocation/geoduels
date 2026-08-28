package persistence

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"

	db "geoduels/pkg/persistence/sqlc/db"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewFromEnv returns the concrete store; consumers depend on the narrow
// repository interfaces in this package, which *DB satisfies.
func NewFromEnv() (*DB, error) {
	url := os.Getenv("POSTGRES_URL")
	if url == "" {
		return nil, errors.New("POSTGRES_URL is required")
	}
	url = normalizeDBURLForContainer(url)
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	cfg, err := pgxpool.ParseConfig(url)
	if err != nil {
		return nil, err
	}
	if maxConns := getenvInt("POSTGRES_MAX_CONNS", 0); maxConns > 0 {
		cfg.MaxConns = int32(maxConns)
	}
	if strings.EqualFold(os.Getenv("POSTGRES_PGBOUNCER"), "true") {
		// Use the unnamed extended-query flow so transaction-pooled PgBouncer
		// never depends on connection-local prepared statements. Unlike simple
		// protocol, this preserves PostgreSQL's inferred parameter types; that is
		// required for sqlc JSON parameters represented as []byte.
		cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeExec
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &DB{pool: pool, db: db.New(pool)}, nil
}

type DB struct {
	pool *pgxpool.Pool
	db   *db.Queries
}

// withTx keeps transaction ownership inside persistence; callers only provide
// work against the transaction-bound adapters and never receive pgx.Tx.
func (s *DB) withTx(ctx context.Context, fn func(pgx.Tx) error) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *DB) Close() {
	if s.pool != nil {
		s.pool.Close()
	}
}
