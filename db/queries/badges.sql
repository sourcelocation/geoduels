-- name: ClaimDiscordSync :one
WITH candidate AS (SELECT d.id FROM discord_sync_outbox d WHERE d.processed_at IS NULL AND d.next_attempt_at <= $1 ORDER BY d.next_attempt_at,d.id LIMIT 1 FOR UPDATE SKIP LOCKED) UPDATE discord_sync_outbox o SET attempts=o.attempts+1,next_attempt_at=$2,last_error=NULL FROM candidate WHERE o.id=candidate.id RETURNING o.id,o.action,o.discord_user_id,o.attempts;

-- name: ClaimDonation :one
UPDATE support_donation_refs SET completed_at=COALESCE(completed_at,now()) WHERE ref=$1 RETURNING user_id::text;

-- name: ClearTeamBadgeSelection :exec
UPDATE users SET selected_badge_code=NULL WHERE id=$1::uuid AND selected_badge_code=$2;

-- name: DeleteTeamBadge :exec
DELETE FROM user_badges WHERE user_id=$1::uuid AND badge_code=$2;

-- name: FindClaimedUser :one
SELECT id::text FROM users WHERE nickname_claimed_at IS NOT NULL AND lower(display_name)=lower($1);

-- name: FindDiscordIdentity :one
SELECT user_id::text FROM user_identities WHERE provider=$1 AND provider_user_id=$2 LIMIT 1;

-- name: GetBadge :one
SELECT level, COALESCE(extra,0)::smallint AS extra FROM user_badges WHERE user_id=$1::uuid AND badge_code=$2;

-- name: InsertBadge :exec
INSERT INTO user_badges(user_id,badge_code,level,extra,awarded_at,updated_at) VALUES ($1::uuid,$2,$3,NULLIF($4,0),now(),now());

-- name: InsertBadgeGrantLog :exec
INSERT INTO moderation_log(subject_user_id,actor_user_id,action,reason,metadata) VALUES ($1::uuid,NULLIF($2,'')::uuid,'badge_granted',NULL,jsonb_build_object('badgeId',$3::text,'source','admin'));

-- name: InsertDonationRef :exec
INSERT INTO support_donation_refs(ref,user_id) VALUES($1,$2::uuid);

-- name: InsertModerationBadgeGrant :exec
INSERT INTO moderation_log(subject_user_id,actor_user_id,action,reason,metadata) VALUES($1::uuid,NULLIF($2,'')::uuid,'badge_granted',NULL,jsonb_build_object('badgeId',$3::text,'source','admin'));

-- name: LockBadge :one
SELECT level, COALESCE(extra, 0)::smallint AS extra FROM user_badges WHERE user_id = $1::uuid AND badge_code = $2 FOR UPDATE;

-- name: LockTopFinish :one
SELECT COALESCE(extra,0)::smallint + 1 AS count FROM user_badges WHERE user_id=$1::uuid AND badge_code=$2 FOR UPDATE;

-- name: LoginBadgeInfo :one
SELECT COALESCE(u.account_type='guest',false), COALESCE(u.is_admin,false) OR COALESCE(u.is_moderator,false), COALESCE(r.mmr,$3)::int FROM users u LEFT JOIN ranks r ON r.user_id=u.id AND r.mode=$2 AND r.season_id=$4 WHERE u.id=$1::uuid;

-- name: LoginDiscordSyncInfo :one
SELECT ui.user_id::text, ui.provider_user_id, COALESCE(max(CASE ub.badge_code WHEN $3 THEN 2000 WHEN $4 THEN 1500 WHEN $5 THEN 1000 ELSE 0 END),0)::int AS highest_elo_badge_mmr FROM user_identities ui JOIN users u ON u.id=ui.user_id LEFT JOIN user_badges ub ON ub.user_id=ui.user_id AND ub.badge_code IN ($3,$4,$5) WHERE ui.provider=$1 AND ui.provider_user_id=$2 AND COALESCE(u.account_type,'registered')<>'guest' AND u.deleted_at IS NULL GROUP BY ui.user_id,ui.provider_user_id;

-- name: MarkDiscordSyncFailed :exec
UPDATE discord_sync_outbox SET next_attempt_at=$2,last_error=NULLIF($3,'') WHERE id=$1 AND processed_at IS NULL;

-- name: MarkDiscordSyncProcessed :exec
UPDATE discord_sync_outbox SET processed_at=now(),last_error=NULL WHERE id=$1;

-- name: UpdateBadge :exec
UPDATE user_badges SET level=$3, extra=NULLIF($4,0), updated_at=now() WHERE user_id=$1::uuid AND badge_code=$2;
