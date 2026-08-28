package persistence

import (
	"context"
	"strings"

	db "geoduels/pkg/persistence/sqlc/db"
)

func resolveMapIdentity(ctx context.Context, q db.DBTX, identifier string) (string, string, error) {
	identifier = strings.TrimSpace(identifier)
	row, err := db.New(q).ResolveMapIdentity(ctx, identifier)
	return row.ID, row.Key, err
}
