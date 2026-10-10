-- name: Notify :exec
-- Delivered to every listener once the surrounding transaction commits.
SELECT pg_notify(sqlc.arg(channel)::text, sqlc.arg(payload)::text);

-- name: ListenEvents :exec
LISTEN gd_events;
