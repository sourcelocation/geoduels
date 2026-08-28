-- name: AddMapCommentLike :execrows
INSERT INTO map_comment_likes(comment_id,user_id) VALUES($1::uuid,$2::uuid) ON CONFLICT DO NOTHING;

-- name: DecrementMapCommentCount :exec
UPDATE maps SET comment_count=greatest(comment_count-1,0), updated_at=now() WHERE id=$1::uuid;

-- name: DecrementMapCommentLike :exec
UPDATE map_comments SET like_count=greatest(like_count-1,0) WHERE id=$1::uuid;

-- name: DeleteMapComment :execrows
UPDATE map_comments SET status=$5, body=NULL, like_count=0, deleted_by=$1::uuid, deleted_at=now(), updated_at=now() WHERE id=$2::uuid AND map_id=$3::uuid AND status='visible' AND ($4 OR user_id=$1::uuid);

-- name: DeleteMapCommentLikes :exec
DELETE FROM map_comment_likes WHERE comment_id=$1::uuid;

-- name: IncrementMapCommentLike :exec
UPDATE map_comments SET like_count=like_count+1 WHERE id=$1::uuid;

-- name: InsertMapComment :exec
INSERT INTO map_comments(id,map_id,parent_id,user_id,body) VALUES($1::uuid,$2::uuid,NULLIF($3,'')::uuid,$4::uuid,$5);

-- name: ListMapComments :many
SELECT c.id::text AS id,c.map_id::text AS map_id,coalesce(c.parent_id::text,'') AS parent_id,c.user_id::text AS user_id,coalesce(u.display_name,c.user_id::text) AS user_display_name,coalesce(u.avatar_url,'') AS avatar_url,case when c.status='visible' then c.body else '' end AS body,c.status AS status,coalesce(c.user_id=nullif($2,'')::uuid or $3::boolean,false) AS can_delete,c.like_count AS like_count,exists(select 1 from map_comment_likes l where l.comment_id=c.id and l.user_id=NULLIF($2,'')::uuid) AS liked,c.created_at AS created_at,c.updated_at AS updated_at FROM map_comments c LEFT JOIN users u ON u.id=c.user_id WHERE c.map_id=$1::uuid ORDER BY coalesce(c.parent_id,c.id),c.created_at ASC LIMIT 300;

-- name: MapCommentDuplicate :one
SELECT EXISTS(SELECT 1 FROM map_comments WHERE map_id=$1::uuid AND user_id=$2::uuid AND parent_id IS NOT DISTINCT FROM NULLIF($3,'')::uuid AND body=$4 AND created_at > now()-interval '2 minutes');

-- name: MapCommentLikeVisible :one
SELECT EXISTS(SELECT 1 FROM map_comments c JOIN maps m ON m.id=c.map_id WHERE c.id=$1::uuid AND c.map_id=$2::uuid AND c.status='visible' AND m.archived_at IS NULL AND (m.owner_user_id IS NULL OR m.official_at IS NOT NULL OR m.owner_user_id=NULLIF($3,'')::uuid OR m.visibility IN ('public','unlisted')));

-- name: MapCommentParent :one
SELECT parent_id::text FROM map_comments WHERE id=$1::uuid AND map_id=$2::uuid AND status='visible';

-- name: MapCommentsVisible :one
SELECT EXISTS(SELECT 1 FROM maps m WHERE m.id=$1::uuid AND m.archived_at IS NULL AND (m.owner_user_id IS NULL OR m.official_at IS NOT NULL OR m.owner_user_id=NULLIF($2,'')::uuid OR m.visibility IN ('public','unlisted')));

-- name: RemoveMapCommentLike :execrows
DELETE FROM map_comment_likes WHERE comment_id=$1::uuid AND user_id=$2::uuid;
