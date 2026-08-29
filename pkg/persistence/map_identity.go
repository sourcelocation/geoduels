package persistence

import (
	"context"
	"strings"

	"geoduels/pkg/entityid"
	db "geoduels/pkg/persistence/sqlc/db"
	"github.com/jackc/pgx/v5/pgtype"
)

func resolveMapIdentity(ctx context.Context, q db.DBTX, identifier string) (string, string, error) {
	identifier = strings.TrimSpace(identifier)
	params := db.ResolveMapIdentityParams{}
	if canonicalID, err := entityid.Parse(identifier); err == nil {
		if err := params.MapID.Scan(canonicalID); err != nil {
			return "", "", err
		}
	} else {
		params.Alias = pgtype.Text{String: identifier, Valid: identifier != ""}
	}
	row, err := db.New(q).ResolveMapIdentity(ctx, params)
	return uuidVal(row.ID), textVal(row.Key), err
}
