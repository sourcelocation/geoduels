-- name: InsertWarning :exec
INSERT INTO moderation_warnings(id, user_id, category, previous_nickname, reset_nickname, evidence_message_id)
VALUES ($1, $2, $3, $4, $5, $6);

-- name: ListWarnings :many
SELECT w.*, coalesce(l.reason, '') AS message, l.actor_user_id,
       coalesce(a.display_name, '') AS actor_name, l.created_at
FROM moderation_warnings w JOIN moderation_log l ON l.id = w.id
LEFT JOIN users a ON a.id = l.actor_user_id
WHERE w.user_id = $1
ORDER BY w.id DESC LIMIT 200;

-- name: AcknowledgeWarning :execrows
UPDATE moderation_warnings SET acknowledged_at = coalesce(acknowledged_at, now())
WHERE id = $1 AND user_id = $2 AND withdrawn_at IS NULL;

-- name: WithdrawWarning :execrows
UPDATE moderation_warnings SET withdrawn_at = now(), withdrawn_by = $3
WHERE id = $1 AND user_id = $2 AND withdrawn_at IS NULL;

-- name: WarnedUserExists :one
SELECT EXISTS(SELECT 1 FROM users WHERE id = $1 AND deleted_at IS NULL)::boolean;

-- name: WarningEvidenceBelongsToUser :one
SELECT EXISTS(SELECT 1 FROM chat_messages WHERE id = $1 AND sender_user_id = $2)::boolean;
