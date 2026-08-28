-- name: CreateChangelogPost :one
INSERT INTO changelog_posts(slug,title,markdown,published,updated_at) VALUES($1,$2,$3,$4,now()) RETURNING id,slug,title,markdown,published,created_at,updated_at;

-- name: EnqueueDiscordSyncAll :exec
INSERT INTO discord_sync_outbox(action, discord_user_id)
SELECT $1, provider_user_id
FROM user_identities
WHERE provider = $2
ON CONFLICT (action, discord_user_id) WHERE processed_at IS NULL DO UPDATE SET
  next_attempt_at = least(discord_sync_outbox.next_attempt_at, excluded.next_attempt_at),
  last_error = NULL;

-- name: GetChangelogPost :one
SELECT id,slug,title,markdown,published,created_at,updated_at FROM changelog_posts WHERE slug=sqlc.arg(slug) AND (sqlc.arg(include_unpublished)::boolean=false OR published=true);

-- name: GetSetting :one
SELECT value_json::text FROM site_settings WHERE key=$1;

-- name: ListChangelogPosts :many
SELECT id,slug,title,markdown,published,created_at,updated_at FROM changelog_posts WHERE ($1::boolean OR published=true) ORDER BY updated_at DESC,id DESC;

-- name: SetSetting :exec
INSERT INTO site_settings(key,value_json,updated_at) VALUES(sqlc.arg(setting_key),sqlc.arg(value_json)::jsonb,now()) ON CONFLICT(key) DO UPDATE SET value_json=excluded.value_json,updated_at=now();

-- name: UpdateChangelogPost :one
UPDATE changelog_posts SET slug=$2,title=$3,markdown=$4,published=$5,updated_at=now() WHERE id=$1 RETURNING id,slug,title,markdown,published,created_at,updated_at;
