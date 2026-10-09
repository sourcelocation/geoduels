// Command migrate brings the database to the schema this release needs: GeoDuels' goose
// migrations (db/migrations), then River's. Every workload that uses the database runs it before
// starting (an init container), so a new pod never meets an older schema. A table-based lock lets
// one run at a time, and it works through the transaction-mode pooler like every other connection.
//
//	migrate          apply what is pending
//	migrate down     roll back the latest migration
//	migrate status   list the migrations and whether each is applied
//
// It connects with the services' settings (POSTGRES_URL, POSTGRES_PGBOUNCER, POSTGRES_MAX_CONNS).
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/database"
	"github.com/pressly/goose/v3/lock"
	"github.com/riverqueue/river/riverdriver/riverpgxv5"
	"github.com/riverqueue/river/rivermigrate"

	"geoduels/db"
	"geoduels/pkg/persistence"
)

// baselineVersion is the v2 schema. Older databases must first reach it with the v2.0.1 release.
const baselineVersion = 2000

func main() {
	command := "up"
	if len(os.Args) > 1 {
		command = os.Args[1]
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	store, err := persistence.NewFromEnv()
	if err != nil {
		log.Fatalf("migrate: %v", err)
	}
	defer store.Close()
	if err := run(ctx, store.Pool(), command); err != nil {
		log.Fatalf("migrate %s: %v", command, err)
	}
}

func run(ctx context.Context, pool *pgxpool.Pool, command string) error {
	sqlDB := stdlib.OpenDBFromPool(pool)
	defer sqlDB.Close()
	migrations, err := fs.Sub(db.Migrations, "migrations")
	if err != nil {
		return err
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, sqlDB, migrations)
	if err != nil {
		return err
	}
	if command == "status" {
		return printStatus(ctx, provider)
	}
	if command != "up" && command != "down" {
		return fmt.Errorf("unknown command %q: use up, down or status", command)
	}

	locker, err := lock.NewPostgresTableLocker()
	if err != nil {
		return err
	}
	if err := locker.Lock(ctx, sqlDB); err != nil {
		return fmt.Errorf("lock: %w", err)
	}
	defer func() {
		if err := locker.Unlock(context.WithoutCancel(ctx), sqlDB); err != nil {
			log.Printf("migrate: unlock: %v", err)
		}
	}()

	if command == "down" {
		result, err := provider.Down(ctx)
		if err != nil {
			return err
		}
		log.Printf("rolled back %s", result.Source.Path)
		return nil
	}
	if err := adoptGolangMigrate(ctx, sqlDB, provider); err != nil {
		return fmt.Errorf("take over from golang-migrate: %w", err)
	}
	results, err := provider.Up(ctx)
	if err != nil {
		return err
	}
	for _, result := range results {
		log.Printf("applied %s in %s", result.Source.Path, result.Duration)
	}
	migrator, err := rivermigrate.New(riverpgxv5.New(pool), nil)
	if err != nil {
		return err
	}
	river, err := migrator.Migrate(ctx, rivermigrate.DirectionUp, &rivermigrate.MigrateOpts{})
	if err != nil {
		return fmt.Errorf("river: %w", err)
	}
	for _, version := range river.Versions {
		log.Printf("applied river migration %d", version.Version)
	}
	return nil
}

// adoptGolangMigrate takes over a database that golang-migrate managed (public.schema_migrations):
// the versions it applied are recorded in goose's table, and its own table goes.
func adoptGolangMigrate(ctx context.Context, sqlDB *sql.DB, provider *goose.Provider) error {
	var legacy, adopted bool
	err := sqlDB.QueryRowContext(ctx, `SELECT
		to_regclass('public.schema_migrations') IS NOT NULL,
		to_regclass('public.`+goose.DefaultTablename+`') IS NOT NULL`).Scan(&legacy, &adopted)
	if err != nil || !legacy || adopted {
		return err
	}
	var version int64
	var dirty bool
	if err := sqlDB.QueryRowContext(ctx, "SELECT version, dirty FROM public.schema_migrations").Scan(&version, &dirty); err != nil {
		return err
	}
	if dirty {
		return fmt.Errorf("golang-migrate left version %d half-applied; repair it by hand first", version)
	}
	if version < baselineVersion {
		return fmt.Errorf("the database is on the v1 schema (version %d): migrate it with the v2.0.1 release first", version)
	}

	store, err := database.NewStore(goose.DialectPostgres, goose.DefaultTablename)
	if err != nil {
		return err
	}
	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // a no-op once committed
	if err := store.CreateVersionTable(ctx, tx); err != nil {
		return err
	}
	if err := store.Insert(ctx, tx, database.InsertRequest{Version: 0}); err != nil {
		return err
	}
	adoptedCount := 0
	for _, source := range provider.ListSources() {
		if source.Version > version {
			continue
		}
		if err := store.Insert(ctx, tx, database.InsertRequest{Version: source.Version}); err != nil {
			return err
		}
		adoptedCount++
	}
	if _, err := tx.ExecContext(ctx, "DROP TABLE public.schema_migrations"); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	log.Printf("took over %d migrations from golang-migrate, up to version %d", adoptedCount, version)
	return nil
}

func printStatus(ctx context.Context, provider *goose.Provider) error {
	statuses, err := provider.Status(ctx)
	if err != nil {
		return err
	}
	for _, status := range statuses {
		applied := "pending"
		if status.State == goose.StateApplied {
			applied = "applied " + status.AppliedAt.Format("2006-01-02 15:04")
		}
		fmt.Printf("%-40s %s\n", status.Source.Path, applied)
	}
	if len(statuses) == 0 {
		return errors.New("no migrations embedded")
	}
	return nil
}
