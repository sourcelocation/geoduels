package persistence

import (
	"context"
	"strings"

	"github.com/jackc/pgx/v5"
)

type mapIdentityQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func resolveMapIdentity(ctx context.Context, q mapIdentityQuerier, identifier string) (string, string, error) {
	identifier = strings.TrimSpace(identifier)
	var id, key string
	err := q.QueryRow(ctx, `
		select m.id::text,coalesce((
			select a.alias from map_aliases a where a.map_id=m.id order by a.created_at,a.alias limit 1
		),m.id::text)
		from maps m
		where m.id::text=$1
		   or exists(select 1 from map_aliases a where a.map_id=m.id and a.alias=$1)
		limit 1
	`, identifier).Scan(&id, &key)
	return id, key, err
}
