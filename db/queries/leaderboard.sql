-- name: EnsureRankedSeasonSettings :exec
INSERT INTO site_settings(key, value_json, updated_at)
VALUES ('ranked_season', convert_from($1, 'UTF8')::jsonb, now())
ON CONFLICT (key) DO NOTHING;

-- name: GetActiveSeasonID :one
SELECT (value_json->>'activeSeasonId')::text FROM site_settings WHERE key = 'ranked_season';

-- name: GetLeaderboardTotals :one
WITH ranked AS (
 SELECT r.user_id, row_number() OVER (ORDER BY r.mmr DESC, r.updated_at ASC, r.user_id ASC) AS rank,
        count(*) OVER () AS total_players
 FROM ranks r LEFT JOIN users u ON u.id = r.user_id
 WHERE r.mode = sqlc.arg(mode) AND r.season_id = sqlc.arg(season_id)
   AND coalesce(u.account_type, 'registered') <> 'guest'
   AND NOT coalesce(u.banned_at IS NOT NULL AND (u.ban_expires_at IS NULL OR u.ban_expires_at > now()), false)
)
SELECT coalesce(max(rank) FILTER (WHERE user_id = nullif(sqlc.arg(self_user_id), '')::uuid), 0)::int AS self_rank,
       coalesce(max(total_players), 0)::int AS total_players FROM ranked;

-- name: GetRankedSeasonSettings :one
SELECT value_json
FROM site_settings
WHERE key = 'ranked_season';

-- name: GetRankedSeasonSettingsForUpdate :one
SELECT value_json
FROM site_settings
WHERE key = 'ranked_season'
FOR UPDATE;

-- name: ListLeaderboard :many
SELECT row_number() OVER (ORDER BY r.mmr DESC, r.updated_at ASC, r.user_id ASC) AS rank,
       r.user_id AS user_id,
       coalesce(nullif(u.display_name, r.user_id::text), ui.provider_name, r.user_id::text) AS display_name,
       coalesce(u.avatar_url, ui.avatar_url, '') AS avatar_url,
       r.mmr,
       coalesce(rs.games_played, 0) AS games_played,
       coalesce(rs.wins, 0) AS wins
FROM ranks r
LEFT JOIN users u ON u.id = r.user_id
LEFT JOIN LATERAL (
  SELECT provider_name, avatar_url FROM user_identities
  WHERE user_id = r.user_id AND provider = 'google'
  ORDER BY created_at ASC LIMIT 1
) ui ON true
LEFT JOIN ranked_stats rs ON rs.user_id = r.user_id AND rs.mode = r.mode AND rs.season_id = r.season_id
WHERE r.mode = $1 AND r.season_id = $2
  AND coalesce(u.account_type, 'registered') <> 'guest'
  AND NOT coalesce(u.banned_at IS NOT NULL AND (u.ban_expires_at IS NULL OR u.ban_expires_at > now()), false)
ORDER BY r.mmr DESC, r.updated_at ASC, r.user_id ASC
LIMIT $4 OFFSET $3;

-- name: ListRankedSeasonFinishers :many
WITH ranked AS (
  SELECT r.user_id, row_number() OVER (ORDER BY r.mmr DESC, r.updated_at ASC, r.user_id ASC)::int AS rank
  FROM ranks r JOIN users u ON u.id = r.user_id
  WHERE r.mode = $1 AND r.season_id = $2
    AND coalesce(u.account_type, 'registered') <> 'guest'
    AND NOT coalesce(u.banned_at IS NOT NULL AND (u.ban_expires_at IS NULL OR u.ban_expires_at > now()), false)
)
SELECT user_id FROM ranked WHERE rank BETWEEN 1 AND 100;

-- name: SeedRankedSeasonRanks :execrows
INSERT INTO ranks(user_id, mode, season_id, mmr, rd)
SELECT u.id, $1, $2, $3, $4
FROM users u
WHERE coalesce(u.account_type, 'registered') <> 'guest'
ON CONFLICT (user_id, mode, season_id) DO NOTHING;

-- name: SeedRankedSeasonStats :exec
INSERT INTO ranked_stats(user_id, mode, season_id, games_played, wins)
SELECT u.id, $1, $2, 0, 0
FROM users u
WHERE coalesce(u.account_type, 'registered') <> 'guest'
ON CONFLICT (user_id, mode, season_id) DO NOTHING;

-- name: WriteRankedSeasonSettings :exec
INSERT INTO site_settings(key, value_json, updated_at)
VALUES ('ranked_season', convert_from($1, 'UTF8')::jsonb, now())
ON CONFLICT (key) DO UPDATE SET value_json = excluded.value_json, updated_at = now();
