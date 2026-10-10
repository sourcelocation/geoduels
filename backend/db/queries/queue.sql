-- name: UpsertQueueTicket :exec
-- Queues the player for a ranked variant; requeueing keeps their place.
INSERT INTO queue_tickets (user_id, variant, mmr)
VALUES (sqlc.arg(user_id), sqlc.arg(variant), sqlc.arg(mmr))
ON CONFLICT (user_id, variant) DO UPDATE SET mmr = excluded.mmr, seen_at = now();

-- name: TouchQueueTickets :execrows
UPDATE queue_tickets SET seen_at = now() WHERE user_id = $1;

-- name: DeleteQueueTickets :exec
DELETE FROM queue_tickets WHERE user_id = $1;

-- name: ListQueueTickets :many
-- Tickets whose player is still connected, oldest first.
SELECT user_id, variant, mmr, joined_at
FROM queue_tickets
WHERE seen_at > now() - (sqlc.arg(fresh_seconds)::double precision * interval '1 second')
ORDER BY joined_at, user_id;

-- name: TakeQueueTickets :many
-- Removes the players' tickets in every variant, returning whose were still there.
DELETE FROM queue_tickets WHERE user_id = ANY(sqlc.arg(user_ids)::uuid[])
RETURNING user_id, variant;

-- name: DeleteStaleQueueTickets :exec
DELETE FROM queue_tickets
WHERE seen_at < now() - (sqlc.arg(fresh_seconds)::double precision * interval '1 second');

-- name: TryMatchmakerLock :one
-- Held by one transaction at a time, until it ends.
SELECT pg_try_advisory_xact_lock(hashtextextended('geoduels:matchmaker', 0)) AS acquired;
