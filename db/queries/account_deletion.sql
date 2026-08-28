-- name: AnonymizeDeletedUser :one
UPDATE users SET email = null, display_name = 'Deleted player', avatar_url = null, nickname_claimed_at = null, account_type = 'guest', is_admin = false, is_moderator = false, deleted_at = coalesce(deleted_at, now()) WHERE id = $1::uuid RETURNING id;

-- name: ArchiveDeletionIdentities :exec
INSERT INTO user_identity_history(user_id, provider, provider_user_id, email, provider_name, first_seen_at, last_seen_at, deleted_at)
SELECT user_id, provider, provider_user_id, email, provider_name, created_at, last_seen_at, now() FROM user_identities WHERE user_id = $1::uuid
ON CONFLICT (user_id, provider, provider_user_id) DO UPDATE SET email = excluded.email, provider_name = excluded.provider_name, last_seen_at = greatest(user_identity_history.last_seen_at, excluded.last_seen_at), deleted_at = coalesce(user_identity_history.deleted_at, excluded.deleted_at);

-- name: DeleteDeletionIdentities :exec
DELETE FROM user_identities WHERE user_id = $1::uuid;

-- name: DeleteOldGuestAccounts :many
WITH batch AS MATERIALIZED (SELECT id FROM users WHERE account_type = 'guest' AND deleted_at IS NULL AND created_at < now() - (sqlc.arg('ttl_seconds')::double precision * interval '1 second') ORDER BY created_at ASC LIMIT sqlc.arg('account_limit')), del_ranked AS (DELETE FROM ranked_stats USING batch WHERE ranked_stats.user_id = batch.id), del_ranks AS (DELETE FROM ranks USING batch WHERE ranks.user_id = batch.id), del_stats AS (DELETE FROM user_stats USING batch WHERE user_stats.user_id = batch.id) DELETE FROM users USING batch WHERE users.id = batch.id RETURNING users.id;

-- name: GetDeletionUser :one
SELECT coalesce(banned_at is not null and (ban_expires_at is null or ban_expires_at > now()), false) AS is_banned, coalesce(ban_reason, '') AS ban_reason FROM users WHERE id = sqlc.arg(user_id)::uuid;

-- name: ListDeletionDiscordIdentities :many
SELECT provider_user_id FROM user_identities WHERE user_id = sqlc.arg('user_id')::uuid AND provider = sqlc.arg('provider');

-- name: RevokeDeletionSessions :exec
UPDATE auth_sessions SET revoked_at = coalesce(revoked_at, now()) WHERE user_id = $1::uuid AND revoked_at IS NULL;
