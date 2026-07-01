package persistence

import (
	"context"
	"errors"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

func NewFromEnv() (Store, error) {
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
		cfg.ConnConfig.DefaultQueryExecMode = pgx.QueryExecModeSimpleProtocol
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, err
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, err
	}
	return &pgStore{pool: pool}, nil
}

type pgStore struct {
	pool *pgxpool.Pool
}

func (s *pgStore) Close() {
	if s.pool != nil {
		s.pool.Close()
	}
}
