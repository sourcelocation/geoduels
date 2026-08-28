-- name: ClaimPendingNotification :one
WITH candidate AS (SELECT id FROM notification_outbox WHERE notification_outbox.type=sqlc.arg(notification_type) AND sent_at IS NULL AND notification_outbox.next_attempt_at<=sqlc.arg(now) ORDER BY notification_outbox.next_attempt_at ASC,notification_outbox.id ASC LIMIT 1 FOR UPDATE SKIP LOCKED)
UPDATE notification_outbox n SET attempts=n.attempts+1,next_attempt_at=sqlc.arg(lease_until),last_error=NULL FROM candidate WHERE n.id=candidate.id RETURNING n.id,n.type,n.payload_json,n.attempts;

-- name: EnqueueNotificationOutbox :exec
INSERT INTO notification_outbox(type,dedupe_key,payload_json,next_attempt_at) VALUES($1,$2,convert_from(sqlc.arg(payload_json), 'UTF8')::jsonb,now()) ON CONFLICT(dedupe_key) DO UPDATE SET payload_json=excluded.payload_json,next_attempt_at=now(),sent_at=NULL,last_error=NULL;

-- name: ListNotificationInbox :many
SELECT id,type,category,payload_json,read_at,created_at FROM user_notifications WHERE user_id=$1 AND archived_at IS NULL AND (sqlc.arg(before_id)=0 OR id<sqlc.arg(before_id)) AND (expires_at IS NULL OR expires_at>now()) ORDER BY created_at DESC,id DESC LIMIT sqlc.arg(row_limit);

-- name: ListReporters :many
SELECT DISTINCT reporter_user_id::text FROM moderation_signals WHERE subject_user_id=$1 AND reporter_user_id IS NOT NULL;

-- name: ListUserNotifications :many
SELECT id,type,payload_json,created_at FROM user_notifications WHERE user_id=$1 AND read_at IS NULL ORDER BY created_at DESC,id DESC LIMIT $2;

-- name: MarkAllUserNotificationsRead :exec
UPDATE user_notifications SET read_at=coalesce(read_at,now()) WHERE user_id=$1 AND read_at IS NULL AND archived_at IS NULL;

-- name: MarkNotificationFailed :exec
UPDATE notification_outbox SET next_attempt_at=sqlc.arg(next_attempt_at),last_error=NULLIF(sqlc.arg(last_error),'') WHERE id=sqlc.arg(outbox_id) AND sent_at IS NULL;

-- name: MarkNotificationSent :exec
UPDATE notification_outbox SET sent_at=now(),last_error=NULL WHERE id=$1;

-- name: MarkUserNotificationRead :exec
UPDATE user_notifications SET read_at=coalesce(read_at,now()) WHERE id=$1 AND user_id=$2;

-- name: UpsertUserNotification :one
WITH inserted AS (INSERT INTO user_notifications(user_id,type,dedupe_key,payload_json) VALUES($1,$2,$3,convert_from(sqlc.arg(payload_json), 'UTF8')::jsonb) ON CONFLICT(dedupe_key) DO UPDATE SET payload_json=excluded.payload_json RETURNING id) SELECT id FROM inserted;
