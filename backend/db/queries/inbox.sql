-- name: InsertInboxMessage :one
-- Returns nothing when this message was already sent (the same badge unlocked again).
INSERT INTO inbox_messages (user_id, kind, dedupe_key, title, body, image_url, modal)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (dedupe_key) DO NOTHING
RETURNING id;

-- name: ListInboxMessages :many
SELECT id, kind, title, body, image_url, modal, read_at, created_at
FROM inbox_messages
WHERE user_id = $1 AND (sqlc.arg(before_id)::bigint = 0 OR id < sqlc.arg(before_id))
ORDER BY id DESC LIMIT sqlc.arg(row_limit);

-- name: GetInboxMessage :one
SELECT id, kind, title, body, image_url, modal, read_at, created_at
FROM inbox_messages WHERE id = $1 AND user_id = $2;

-- name: CountUnreadInboxMessages :one
SELECT count(*)::int FROM inbox_messages WHERE user_id = $1 AND read_at IS NULL;

-- name: MarkInboxMessageRead :exec
UPDATE inbox_messages SET read_at = coalesce(read_at, now()) WHERE id = $1 AND user_id = $2;

-- name: MarkAllInboxMessagesRead :exec
UPDATE inbox_messages SET read_at = now() WHERE user_id = $1 AND read_at IS NULL;

-- name: DeleteInboxMessage :exec
DELETE FROM inbox_messages WHERE id = $1;
