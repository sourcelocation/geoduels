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
-- Plans are written with their match, so a plan without one belongs to a match already cleaned up.
DELETE FROM match_round_plans
WHERE ctid IN (
    SELECT p.ctid FROM match_round_plans p
    WHERE NOT EXISTS (SELECT 1 FROM match_sessions s WHERE s.match_id = p.match_id)
    LIMIT $1
);

-- name: DeleteMatchSessions :execresult
-- Matches over for an hour. Interrupted ones are ended by the API's sweep.
DELETE FROM match_sessions
WHERE match_id IN (
    SELECT match_id FROM match_sessions
    WHERE ended_at < now() - interval '1 hour'
    ORDER BY ended_at
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

-- name: DeleteStaleLeases :execresult
-- Leases expired for a week: nothing stamped with their tokens is still open.
DELETE FROM control_plane_leases
WHERE name IN (
    SELECT name FROM control_plane_leases
    WHERE expires_at < now() - interval '7 days'
    LIMIT $1
);

-- name: DeleteRateLimitWindows :execresult
-- Windows no limit counts any more; the longest is a day.
DELETE FROM rate_limit_windows
WHERE key IN (
    SELECT key FROM rate_limit_windows
    WHERE window_started_at < now() - interval '2 days'
    LIMIT $1
);

-- name: DeletePresence :execresult
-- Players not seen for a day; anyone online refreshes their row every half minute.
DELETE FROM presence
WHERE user_id IN (
    SELECT user_id FROM presence
    WHERE seen_at < now() - interval '1 day'
    LIMIT $1
);

-- name: ExpireParties :execrows
-- Parties past their lifetime close, unless their match is still going.
UPDATE parties SET state = 'expired', updated_at = now()
WHERE state = 'open' AND expires_at < now()
  AND NOT EXISTS (SELECT 1 FROM match_sessions ms WHERE ms.match_id = parties.last_match_id AND ms.ended_at IS NULL);
