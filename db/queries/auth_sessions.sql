-- name: CreateAuthSession :one
INSERT INTO auth_sessions(id,user_id,refresh_token_hash,expires_at,created_at,last_used_at,user_agent,ip_address) VALUES($1,$2,$3,$4,now(),now(),$5,$6) RETURNING id,user_id,refresh_token_hash,expires_at,created_at,last_used_at,revoked_at,coalesce(user_agent,''),coalesce(ip_address,'');

-- name: GetAuthSessionByRefreshToken :one
SELECT id,user_id,refresh_token_hash,expires_at,created_at,last_used_at,revoked_at,coalesce(user_agent,''),coalesce(ip_address,'') FROM auth_sessions WHERE refresh_token_hash=$1;

-- name: RevokeAuthSession :exec
UPDATE auth_sessions SET revoked_at=coalesce(revoked_at,now()) WHERE id=$1;

-- name: RevokeAuthSessionsForUser :exec
UPDATE auth_sessions SET revoked_at=coalesce(revoked_at,now()) WHERE user_id=$1 AND revoked_at IS NULL;

-- name: RotateAuthSession :one
UPDATE auth_sessions SET refresh_token_hash=sqlc.arg(next_refresh_token_hash),expires_at=sqlc.arg(expires_at),last_used_at=sqlc.arg(last_used_at) WHERE id=sqlc.arg(session_id) AND refresh_token_hash=sqlc.arg(current_refresh_token_hash) AND revoked_at IS NULL RETURNING id,user_id,refresh_token_hash,expires_at,created_at,last_used_at,revoked_at,coalesce(user_agent,''),coalesce(ip_address,'');

-- name: SetRegistrationIP :exec
UPDATE users SET registration_ip_address=coalesce(nullif(trim(registration_ip_address),''),sqlc.arg(registration_ip_address)) WHERE id=sqlc.arg(user_id);
