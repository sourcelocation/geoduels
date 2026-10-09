// Package db holds GeoDuels' schema: the goose migrations in migrations/, which cmd/migrate applies.
// sqlc reads the same files for the queries in queries/.
package db

import "embed"

// Migrations are the goose migrations, each with an Up and a Down section.
//
//go:embed migrations/*.sql
var Migrations embed.FS
