-- name: CompressReplay :execresult
UPDATE match_history
SET replay_zstd = $2, replay_codec = $3, replay_schema_version = $4,
    replay_uncompressed_bytes = $5, replay_sha256 = $6, replay_json = NULL
WHERE match_id = $1 AND replay_zstd IS NULL;

-- name: DeleteAuthSessions :execresult
DELETE FROM auth_sessions
WHERE id IN (
    SELECT id FROM auth_sessions
    WHERE (expires_at < now() - interval '24 hours') OR (revoked_at < now() - interval '24 hours')
    ORDER BY COALESCE(revoked_at, expires_at)
    LIMIT $1
);

-- name: DeleteChatConversations :execresult
DELETE FROM chat_conversations c
WHERE c.id IN (
    SELECT c2.id FROM chat_conversations c2
    WHERE NOT exists(SELECT 1 FROM chat_messages m WHERE m.conversation_id = c2.id)
    LIMIT $1
);

-- name: DeleteChatMessages :execresult
DELETE FROM chat_messages
WHERE id IN (
    SELECT id FROM chat_messages
    WHERE created_at < now() - interval '7 days'
    ORDER BY created_at
    LIMIT $1
);

-- name: DeleteExpiredReplays :execresult
WITH expired AS (
    SELECT match_id FROM match_history
    WHERE replay_expires_at <= now() AND (replay_zstd IS NOT NULL OR replay_json IS NOT NULL)
    ORDER BY replay_expires_at
    LIMIT $1
    FOR UPDATE SKIP LOCKED
)
UPDATE match_history h
SET replay_zstd = NULL, replay_json = NULL, replay_codec = NULL,
    replay_schema_version = NULL, replay_uncompressed_bytes = NULL, replay_sha256 = NULL
FROM expired e
WHERE h.match_id = e.match_id;

-- name: DeleteMapDailyUsers :execresult
DELETE FROM map_daily_users
WHERE ctid IN (
    SELECT ctid FROM map_daily_users
    WHERE day < current_date - 8
    LIMIT $1
);

-- name: DeleteMapUploadEvents :execresult
DELETE FROM map_upload_events
WHERE id IN (
    SELECT id FROM map_upload_events
    WHERE created_at < now() - interval '24 hours'
    ORDER BY created_at
    LIMIT $1
);

-- name: DeleteMatchPlans :execresult
DELETE FROM match_round_plans
WHERE ctid IN (
    SELECT p.ctid FROM match_round_plans p
    JOIN match_history h ON h.match_id = p.match_id
    WHERE h.ended_at < now() - interval '1 hour'
    LIMIT $1
);

-- name: DeleteMatchSessions :execresult
-- Matches over for an hour: ended, or interrupted (lease lapsed without an end).
DELETE FROM match_sessions
WHERE match_id IN (
    SELECT match_id FROM match_sessions
    WHERE coalesce(ended_at, lease_expires_at) < now() - interval '1 hour'
    ORDER BY coalesce(ended_at, lease_expires_at)
    LIMIT $1
);

-- name: DeleteParties :execresult
DELETE FROM parties
WHERE id IN (
    SELECT id FROM parties
    WHERE state IN ('closed', 'expired') AND updated_at < now() - interval '24 hours'
    ORDER BY updated_at
    LIMIT $1
);

-- name: DeleteUserEvents :execresult
DELETE FROM user_events
WHERE (user_id, sequence) IN (
    SELECT user_id, sequence FROM user_events
    WHERE created_at < now() - interval '7 days'
    ORDER BY created_at
    LIMIT $1
);

-- name: DeleteUserNotifications :execresult
DELETE FROM user_notifications
WHERE id IN (
    SELECT id FROM user_notifications
    WHERE (read_at IS NOT NULL AND read_at < now() - interval '30 days')
       OR (read_at IS NULL AND created_at < now() - interval '90 days')
    ORDER BY created_at
    LIMIT $1
);

-- name: ListLegacyReplays :many
SELECT match_id, replay_json
FROM match_history
WHERE replay_zstd IS NULL AND replay_json IS NOT NULL
  AND (replay_expires_at IS NULL OR replay_expires_at > now())
ORDER BY ended_at DESC
LIMIT $1
FOR UPDATE SKIP LOCKED;

-- name: ListMapCountryStats :many
SELECT country, location_count
FROM map_country_stats
WHERE map_id = $1
ORDER BY location_count DESC, country ASC
LIMIT 64;

-- name: TryAdvisoryLock :one

SELECT pg_try_advisory_xact_lock($1) AS locked;
