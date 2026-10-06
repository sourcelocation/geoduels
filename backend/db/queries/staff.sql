-- name: GetStaffRoles :many
SELECT role::text FROM user_roles WHERE user_id=$1 ORDER BY role;

-- name: LockStaffUser :one
SELECT id FROM users WHERE id=$1 AND deleted_at IS NULL FOR UPDATE;

-- name: InsertStaffRole :exec
INSERT INTO user_roles(user_id,role,granted_by,reason)
VALUES(sqlc.arg(user_id),sqlc.arg(role)::staff_role,nullif(sqlc.arg(actor_user_id),'')::uuid,sqlc.arg(reason))
ON CONFLICT(user_id,role) DO NOTHING;

-- name: DeleteStaffRole :exec
DELETE FROM user_roles WHERE user_id=sqlc.arg(user_id) AND role=sqlc.arg(role)::staff_role;

-- name: ListUserRoles :many
SELECT u.id, coalesce(nullif(u.display_name,''),u.id::text) AS display_name,
 coalesce(u.email,'') AS email, ur.role::text AS role,
 ur.granted_by AS actor_user_id, ur.granted_at, ur.reason AS last_reason
FROM user_roles ur JOIN users u ON u.id=ur.user_id
WHERE u.deleted_at IS NULL ORDER BY ur.granted_at DESC, u.id, ur.role;
