package persistence

import (
	"context"
	"errors"
	"net/url"
	"os"
	"strings"
	"time"

	"geoduels/internal/envcfg"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewFromEnv opens the shared PostgreSQL pool. Feature packages own queries.
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
	if maxConns := envcfg.Int("POSTGRES_MAX_CONNS", 0); maxConns > 0 {
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
	return &DB{pool: pool}, nil
}

type DB struct {
	pool *pgxpool.Pool
}

// Pool exposes the underlying connection pool for feature stores.
func (s *DB) Pool() *pgxpool.Pool { return s.pool }

func (s *DB) Close() {
	if s.pool != nil {
		s.pool.Close()
	}
}

func normalizeDBURLForContainer(dsn string) string {
	if _, err := os.Stat("/.dockerenv"); err != nil {
		return dsn
	}
	u, err := url.Parse(dsn)
	if err != nil {
		return dsn
	}
	if u.Hostname() == "127.0.0.1" || u.Hostname() == "localhost" {
		port := u.Port()
		if port == "" {
			port = "5432"
		}
		u.Host = "host.docker.internal:" + port
		return u.String()
	}
	return dsn
}
