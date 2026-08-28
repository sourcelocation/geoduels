-- name: GetActiveMapCounts :one
SELECT count(*)::int AS current_maps, coalesce(sum(location_count),0)::int AS current_locations
FROM maps WHERE owner_user_id=$1 AND archived_at IS NULL;

-- name: GetMapTrustUser :one
SELECT account_type, created_at, banned_at, ban_expires_at, deleted_at,
       map_creator_tier_override, report_muted_at, report_mute_expires_at
FROM users WHERE id=$1;

-- name: GetMapUploadStats :one
SELECT
  count(*) FILTER (WHERE created_at > now()-interval '1 hour')::int AS hourly_uploads,
  count(*) FILTER (WHERE created_at > now()-interval '1 day')::int AS daily_uploads,
  coalesce(sum(location_count) FILTER (WHERE created_at > now()-interval '1 hour'), 0)::int AS hourly_locations
FROM map_upload_events
WHERE user_id=$1;

-- name: GetOwnedActiveMapLocationCount :one
SELECT location_count
FROM maps
WHERE id=$1 AND owner_user_id=$2 AND archived_at IS NULL;

-- name: GetQualifiedMapFavorites :one
SELECT count(DISTINCT mf.user_id)::int AS qualified_favorites,
       count(DISTINCT mf.map_id)::int AS qualified_maps
FROM map_favorites mf JOIN maps m ON m.id=mf.map_id JOIN users favoriter ON favoriter.id=mf.user_id
WHERE m.owner_user_id=$1 AND mf.user_id<>$1 AND favoriter.account_type='registered'
  AND favoriter.created_at <= now()-interval '7 days'
  AND NOT coalesce(favoriter.banned_at IS NOT NULL AND (favoriter.ban_expires_at IS NULL OR favoriter.ban_expires_at > now()), false)
  AND favoriter.deleted_at IS NULL;

-- name: LockMapUpload :exec
SELECT pg_advisory_xact_lock(hashtext($1));

-- name: SetMapCreatorTierOverride :execrows
UPDATE users SET map_creator_tier_override=$2 WHERE id=$1;

-- name: UpdateMapCreatorTrust :exec
UPDATE users SET map_creator_tier=$2, map_creator_qualified_favorites=$3,
 map_creator_qualified_maps=$4, map_creator_trust_updated_at=now() WHERE id=$1;
