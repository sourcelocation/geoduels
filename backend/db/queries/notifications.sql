-- name: ListNotificationInbox :many
SELECT n.id,n.type,n.category,n.payload_json,n.read_at,n.created_at,actor.id AS actor_user_id,coalesce(nullif(actor.display_name,''),actor.id::text,'')::text AS actor_display_name
FROM user_notifications n
LEFT JOIN users actor ON actor.id=n.actor_user_id
WHERE n.user_id=$1 AND n.archived_at IS NULL AND (sqlc.arg(before_id)=0 OR n.id<sqlc.arg(before_id)) AND (n.expires_at IS NULL OR n.expires_at>now())
ORDER BY n.created_at DESC,n.id DESC LIMIT sqlc.arg(row_limit);

-- name: ListUserNotifications :many
-- The notifications no device has shown yet, oldest first.
SELECT n.id,n.type,n.payload_json,n.created_at,actor.id AS actor_user_id,coalesce(nullif(actor.display_name,''),actor.id::text,'')::text AS actor_display_name
FROM user_notifications n
LEFT JOIN users actor ON actor.id=n.actor_user_id
WHERE n.user_id=$1 AND n.read_at IS NULL AND n.archived_at IS NULL AND (n.expires_at IS NULL OR n.expires_at>now())
ORDER BY n.created_at,n.id LIMIT $2;

-- name: ListUnseenNotificationsSince :many
-- Unshown notifications that any service wrote for these people since a moment.
SELECT n.id,n.user_id,n.type,n.payload_json,n.created_at,actor.id AS actor_user_id,coalesce(nullif(actor.display_name,''),actor.id::text,'')::text AS actor_display_name
FROM user_notifications n
LEFT JOIN users actor ON actor.id=n.actor_user_id
WHERE n.user_id=ANY(sqlc.arg(user_ids)::uuid[]) AND n.read_at IS NULL AND n.created_at>sqlc.arg(since)
  AND n.archived_at IS NULL AND (n.expires_at IS NULL OR n.expires_at>now())
ORDER BY n.created_at,n.id LIMIT 500;

-- name: MarkAllUserNotificationsRead :exec
UPDATE user_notifications SET read_at=coalesce(read_at,now()) WHERE user_id=$1 AND read_at IS NULL AND archived_at IS NULL;

-- name: MarkUserNotificationRead :exec
UPDATE user_notifications SET read_at=coalesce(read_at,now()) WHERE id=$1 AND user_id=$2;

-- name: UpsertUserNotification :one
-- Writing a notification again (a re-sent invitation, a withdrawn warning) announces it again.
WITH inserted AS (
  INSERT INTO user_notifications(user_id,type,dedupe_key,payload_json,actor_user_id,expires_at)
  VALUES($1,$2,$3,convert_from(sqlc.arg(payload_json), 'UTF8')::jsonb,sqlc.narg(actor_user_id),sqlc.narg(expires_at))
  ON CONFLICT(dedupe_key) DO UPDATE SET payload_json=excluded.payload_json, actor_user_id=excluded.actor_user_id, expires_at=excluded.expires_at, read_at=NULL, created_at=now()
  RETURNING id
) SELECT id FROM inserted;
