-- name: GetIdentity :one
SELECT u.id::text AS user_id,coalesce(u.email,ui.email,'') AS email,coalesce(ui.provider_name,'') AS provider_name,coalesce(u.avatar_url,ui.avatar_url,'') AS avatar_url,coalesce(u.account_type='registered' AND u.nickname_claimed_at IS NULL,false) AS needs_nickname,coalesce(nullif(u.display_name,''),ui.provider_name,u.id::text) AS display_name,u.account_type,coalesce(u.is_admin,false) AS is_admin,coalesce(u.is_moderator,false) AS is_moderator,coalesce(u.banned_at IS NOT NULL AND (u.ban_expires_at IS NULL OR u.ban_expires_at>now()),false) AS is_banned,coalesce(u.ban_reason,'') AS ban_reason FROM users u LEFT JOIN LATERAL (SELECT email,provider_name,avatar_url FROM user_identities WHERE user_id=u.id AND provider IN ('discord','google') ORDER BY CASE provider WHEN 'discord' THEN 0 WHEN 'google' THEN 1 ELSE 2 END,created_at ASC LIMIT 1) ui ON true WHERE u.id=sqlc.arg(user_id);

-- name: ListIdentityProviders :many
SELECT provider FROM user_identities WHERE user_id=$1 ORDER BY CASE provider WHEN 'discord' THEN 0 WHEN 'google' THEN 1 ELSE 2 END,provider;

-- name: NicknameTaken :one
SELECT EXISTS(
  SELECT 1 FROM users
  WHERE id <> $1
    AND account_type = 'registered'
    AND nickname_claimed_at IS NOT NULL
    AND lower(display_name) = lower($2)
) AS taken;

-- name: ProviderIdentityBanned :one
SELECT coalesce(reason,'') FROM oauth_identity_bans WHERE provider=$1 AND provider_user_id=$2 AND revoked_at IS NULL LIMIT 1;

-- name: ProviderIdentityExists :one
SELECT EXISTS(SELECT 1 FROM user_identities WHERE provider=$1 AND provider_user_id=$2);

-- name: SetNickname :execrows
UPDATE users
SET display_name = $2,
    nickname_claimed_at = coalesce(nickname_claimed_at, now())
WHERE id = $1
  AND coalesce(account_type, 'registered') <> 'guest';
