-- name: InsertModerationLog :one

INSERT INTO moderation_log(subject_user_id, actor_user_id, action, reason, expires_at, metadata)
VALUES(
    sqlc.arg('subject_user_id')::uuid,
    NULLIF(sqlc.arg('actor_user_id'), '')::uuid,
    sqlc.arg('action'),
    NULLIF(sqlc.arg('reason'), ''),
    sqlc.arg('expires_at'),
    convert_from(sqlc.arg('metadata'), 'UTF8')::jsonb
)
RETURNING id;

-- name: ListModerationLog :many
SELECT l.id AS log_id, l.subject_user_id, coalesce(nullif(subject.display_name, ''), l.subject_user_id::text, '') AS subject_display_name, l.actor_user_id, coalesce(nullif(actor.display_name, ''), l.actor_user_id::text, '') AS actor_display_name, l.action, coalesce(l.reason, '') AS reason, l.expires_at, l.signal_ids, l.metadata, l.created_at
FROM moderation_log l LEFT JOIN users subject ON subject.id=l.subject_user_id LEFT JOIN users actor ON actor.id=l.actor_user_id
WHERE (sqlc.narg(subject_user_id)::uuid IS NULL OR l.subject_user_id=sqlc.narg(subject_user_id)::uuid) ORDER BY l.created_at DESC,l.id DESC LIMIT sqlc.arg(row_limit);
