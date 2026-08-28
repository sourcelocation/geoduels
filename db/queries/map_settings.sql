-- name: ClearMapOfficial :execrows
UPDATE maps SET official_at=null, official_by=null, updated_at=now() WHERE id=$1::uuid AND archived_at IS NULL;

-- name: GetGameplayMapSettings :one
SELECT value_json::text AS value_json FROM site_settings WHERE key = $1;

-- name: GetReadyMapForShare :one
SELECT status, location_count FROM maps WHERE id=$1::uuid AND archived_at IS NULL FOR SHARE;

-- name: SetGameplayMapSettings :exec
INSERT INTO site_settings(key, value_json, updated_at) VALUES ($1, sqlc.arg(value_json)::jsonb, now()) ON CONFLICT (key) DO UPDATE SET value_json=excluded.value_json, updated_at=now();

-- name: SetMapOfficial :execrows
UPDATE maps SET official_at=coalesce(official_at, now()), official_by=sqlc.arg(official_by)::uuid, published_at=coalesce(published_at, now()), visibility='public', updated_at=now() WHERE id=sqlc.arg(map_id)::uuid AND archived_at IS NULL;
