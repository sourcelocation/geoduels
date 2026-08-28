-- name: AddMapCommentLike :execrows
INSERT INTO map_comment_likes(comment_id,user_id) VALUES(sqlc.arg(comment_id)::uuid,sqlc.arg(user_id)::uuid) ON CONFLICT DO NOTHING;

-- name: DecrementMapCommentCount :exec
UPDATE maps SET comment_count=greatest(comment_count-1,0), updated_at=now() WHERE id=$1::uuid;

-- name: DecrementMapCommentLike :exec
UPDATE map_comments SET like_count=greatest(like_count-1,0) WHERE id=$1::uuid;

-- name: DeleteMapComment :execrows
UPDATE map_comments SET status=sqlc.arg(status), body=NULL, like_count=0, deleted_by=sqlc.arg(deleted_by)::uuid, deleted_at=now(), updated_at=now() WHERE id=sqlc.arg(comment_id)::uuid AND map_id=sqlc.arg(map_id)::uuid AND status='visible' AND (sqlc.arg(is_moderator) OR user_id=sqlc.arg(deleted_by)::uuid);

-- name: DeleteMapCommentLikes :exec
DELETE FROM map_comment_likes WHERE comment_id=$1::uuid;

-- name: IncrementMapCommentLike :exec
UPDATE map_comments SET like_count=like_count+1 WHERE id=$1::uuid;

-- name: InsertMapComment :exec
INSERT INTO map_comments(id,map_id,parent_id,user_id,body) VALUES(sqlc.arg(comment_id)::uuid,sqlc.arg(map_id)::uuid,NULLIF(sqlc.arg(parent_id),'')::uuid,sqlc.arg(user_id)::uuid,sqlc.arg(body));

-- name: ListMapComments :many
SELECT c.id::text AS id,c.map_id::text AS map_id,coalesce(c.parent_id::text,'') AS parent_id,c.user_id::text AS user_id,coalesce(u.display_name,c.user_id::text) AS user_display_name,coalesce(u.avatar_url,'') AS avatar_url,case when c.status='visible' then c.body else '' end AS body,c.status AS status,coalesce(c.user_id=nullif(sqlc.arg(viewer_user_id),'')::uuid or sqlc.arg(can_moderate)::boolean,false) AS can_delete,c.like_count AS like_count,exists(select 1 from map_comment_likes l where l.comment_id=c.id and l.user_id=NULLIF(sqlc.arg(viewer_user_id),'')::uuid) AS liked,c.created_at AS created_at,c.updated_at AS updated_at FROM map_comments c LEFT JOIN users u ON u.id=c.user_id WHERE c.map_id=sqlc.arg(map_id)::uuid ORDER BY coalesce(c.parent_id,c.id),c.created_at ASC LIMIT 300;

-- name: MapCommentDuplicate :one
SELECT EXISTS(SELECT 1 FROM map_comments WHERE map_id=sqlc.arg(map_id)::uuid AND user_id=sqlc.arg(user_id)::uuid AND parent_id IS NOT DISTINCT FROM NULLIF(sqlc.arg(parent_id),'')::uuid AND body=sqlc.arg(body) AND created_at > now()-interval '2 minutes');

-- name: MapCommentLikeVisible :one
SELECT EXISTS(SELECT 1 FROM map_comments c JOIN maps m ON m.id=c.map_id WHERE c.id=sqlc.arg(comment_id)::uuid AND c.map_id=sqlc.arg(map_id)::uuid AND c.status='visible' AND m.archived_at IS NULL AND (m.owner_user_id IS NULL OR m.official_at IS NOT NULL OR m.owner_user_id=NULLIF(sqlc.arg(viewer_user_id),'')::uuid OR m.visibility IN ('public','unlisted')));

-- name: MapCommentParent :one
SELECT parent_id::text FROM map_comments WHERE id=sqlc.arg(comment_id)::uuid AND map_id=sqlc.arg(map_id)::uuid AND status='visible';

-- name: MapCommentsVisible :one
SELECT EXISTS(SELECT 1 FROM maps m WHERE m.id=sqlc.arg(map_id)::uuid AND m.archived_at IS NULL AND (m.owner_user_id IS NULL OR m.official_at IS NOT NULL OR m.owner_user_id=NULLIF(sqlc.arg(viewer_user_id),'')::uuid OR m.visibility IN ('public','unlisted')));

-- name: RemoveMapCommentLike :execrows
DELETE FROM map_comment_likes WHERE comment_id=sqlc.arg(comment_id)::uuid AND user_id=sqlc.arg(user_id)::uuid;
