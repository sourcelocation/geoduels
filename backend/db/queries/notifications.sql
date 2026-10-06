-- name: ListNotificationInbox :many
SELECT n.id,n.type,n.category,n.payload_json,n.read_at,n.created_at,actor.id AS actor_user_id,coalesce(nullif(actor.display_name,''),actor.id::text,'')::text AS actor_display_name
FROM user_notifications n
LEFT JOIN users actor ON actor.id=n.actor_user_id OR (n.actor_user_id IS NULL AND actor.id::text=n.payload_json->>'actorUserId')
WHERE n.user_id=$1 AND n.archived_at IS NULL AND (sqlc.arg(before_id)=0 OR n.id<sqlc.arg(before_id)) AND (n.expires_at IS NULL OR n.expires_at>now())
ORDER BY n.created_at DESC,n.id DESC LIMIT sqlc.arg(row_limit);

-- name: ListUserNotifications :many
SELECT n.id,n.type,n.payload_json,n.created_at,actor.id AS actor_user_id,coalesce(nullif(actor.display_name,''),actor.id::text,'')::text AS actor_display_name
FROM user_notifications n
LEFT JOIN users actor ON actor.id=n.actor_user_id OR (n.actor_user_id IS NULL AND actor.id::text=n.payload_json->>'actorUserId')
WHERE n.user_id=$1 AND n.read_at IS NULL
ORDER BY n.created_at DESC,n.id DESC LIMIT $2;

-- name: MarkAllUserNotificationsRead :exec
UPDATE user_notifications SET read_at=coalesce(read_at,now()) WHERE user_id=$1 AND read_at IS NULL AND archived_at IS NULL;

-- name: MarkUserNotificationRead :exec
UPDATE user_notifications SET read_at=coalesce(read_at,now()) WHERE id=$1 AND user_id=$2;

-- name: UpsertUserNotification :one
WITH inserted AS (
  INSERT INTO user_notifications(user_id,type,dedupe_key,payload_json,actor_user_id)
  VALUES($1,$2,$3,convert_from(sqlc.arg(payload_json), 'UTF8')::jsonb,sqlc.narg(actor_user_id))
  ON CONFLICT(dedupe_key) DO UPDATE SET payload_json=excluded.payload_json, actor_user_id=excluded.actor_user_id
  RETURNING id
) SELECT id FROM inserted;
