-- name: ApplyEloRefund :exec
UPDATE ranks
SET mmr = $4, updated_at = now()
WHERE user_id = $1 AND mode = $2 AND season_id = $3;

-- name: BanUser :execrows
UPDATE users
SET banned_at = sqlc.arg('banned_at'),
    ban_reason = sqlc.arg('ban_reason'),
    ban_expires_at = NULL
WHERE id = sqlc.arg('user_id')::uuid;

-- name: BanUserForCheating :execrows
UPDATE users
SET banned_at = COALESCE(banned_at, now()),
    ban_reason = $2,
    ban_expires_at = NULL
WHERE id = $1;

-- name: BanUserOAuthIdentities :exec
INSERT INTO oauth_identity_bans(provider, provider_user_id, banned_user_id, reason, created_by, created_at, revoked_at)
SELECT provider, provider_user_id, sqlc.arg(banned_user_id), NULLIF(sqlc.arg(reason)::text, ''), NULLIF(sqlc.arg(created_by)::text, '')::uuid, now(), NULL
FROM (
    SELECT provider, provider_user_id
    FROM user_identity_history
    WHERE user_id = sqlc.arg(banned_user_id)
    UNION
    SELECT provider, provider_user_id
    FROM user_identities
    WHERE user_id = sqlc.arg(banned_user_id)
) identities
ON CONFLICT (provider, provider_user_id) DO UPDATE SET
    banned_user_id = excluded.banned_user_id,
    reason = excluded.reason,
    created_by = excluded.created_by,
    created_at = now(),
    revoked_at = NULL;

-- name: ClearChatMute :execrows
UPDATE users
SET chat_muted_at = NULL, chat_mute_reason = NULL, chat_mute_expires_at = NULL
WHERE id = $1;

-- name: ClearReportMute :execrows
UPDATE users
SET report_muted_at = NULL, report_mute_reason = NULL, report_mute_expires_at = NULL
WHERE id = $1;

-- name: ClearReporterMute :execrows
UPDATE users
SET report_muted_at = NULL, report_mute_reason = NULL, report_mute_expires_at = NULL
WHERE id = $1;

-- name: GetPlayerRiskContext :one
SELECT
    COALESCE(r.mmr, sqlc.arg(default_rating))::int AS rating,
    COALESCE(rs.games_played, 0)::int AS ranked_games
FROM users u
LEFT JOIN ranks r ON r.user_id = u.id AND r.mode = sqlc.arg(mode) AND r.season_id = sqlc.arg(season_id)
LEFT JOIN ranked_stats rs ON rs.user_id = u.id AND rs.mode = sqlc.arg(mode) AND rs.season_id = sqlc.arg(season_id)
WHERE u.id = sqlc.arg(user_id);

-- name: GetUserRegistrationIP :one
SELECT COALESCE(registration_ip_address, '') AS registration_ip
FROM users
WHERE id = $1;

-- name: HasRelatedCheaterFromIP :one
SELECT exists(
    SELECT 1
    FROM users
    WHERE id <> $1
        AND registration_ip_address = $2
        AND banned_at >= now() - interval '7 days'
        AND (
            lower(COALESCE(ban_reason, '')) LIKE '%cheat%'
            OR lower(COALESCE(ban_reason, '')) LIKE 'auto_%'
        )
) AS related_cheater;

-- name: InsertEloRefund :execrows
INSERT INTO elo_refunds(
    user_id, match_id, cheater_user_id, original_delta, refund_delta,
    victim_mmr_before, victim_mmr_after, computed_refund_delta, reason, created_by_reason
)
VALUES($1, $2, $3, $4, $5, $6, $7, $5, 'cheating_verdict', $8)
ON CONFLICT (user_id, match_id, cheater_user_id) DO NOTHING;

-- name: InsertIPSignupBan :exec
INSERT INTO ip_signup_bans(ip_address, reason, created_by, created_at, revoked_at)
VALUES($1, NULLIF(sqlc.arg(reason)::text, ''), NULLIF(sqlc.arg(created_by)::text, '')::uuid, now(), NULL)
ON CONFLICT (ip_address) DO UPDATE SET
    reason = excluded.reason,
    created_by = excluded.created_by,
    created_at = now(),
    revoked_at = NULL;

-- name: InsertModerationLog :one

INSERT INTO moderation_log(subject_user_id, actor_user_id, action, reason, expires_at, metadata)
VALUES(
    sqlc.arg('subject_user_id')::uuid,
    NULLIF(sqlc.arg('actor_user_id'), '')::uuid,
    sqlc.arg('action'),
    NULLIF(sqlc.arg('reason'), ''),
    sqlc.arg('expires_at'),
    sqlc.arg('metadata')::jsonb
)
RETURNING id;

-- name: IsIPSignupBanned :one
SELECT exists(
    SELECT 1 FROM ip_signup_bans
    WHERE ip_address = $1 AND revoked_at IS NULL
) AS banned;

-- name: ListActiveIPSignupBans :many
SELECT id, ip_address, COALESCE(reason, '') AS reason, COALESCE(created_by::text, '') AS created_by, created_at
FROM ip_signup_bans
WHERE revoked_at IS NULL
ORDER BY created_at DESC
LIMIT $1;

-- name: ListCheaterRefundCandidates :many
WITH candidate_matches AS (
    SELECT
        h.match_id,
        h.ended_at,
        h.winner_user_id,
        opponent.user_id AS opponent_user_id,
        cheater.mmr AS cheater_mmr,
        COALESCE(cheater.rating_rd, sqlc.arg(default_rating_rd)::double precision) AS cheater_rd,
        opponent.final_ranked_delta AS original_delta
    FROM match_history h
    JOIN match_players cheater ON cheater.match_id = h.match_id AND cheater.user_id = $1
    JOIN match_players opponent ON opponent.match_id = h.match_id AND opponent.user_id <> $1
    LEFT JOIN parties l ON l.active_match_id = h.match_id
        OR l.started_match_id = h.match_id
        OR l.last_match_id = h.match_id
    WHERE h.mode = $2
        AND h.winner_user_id = $1
        AND (sqlc.narg(since)::timestamptz IS NULL OR h.ended_at >= sqlc.narg(since))
        AND h.ranked
        AND l.id IS NULL
)
SELECT
    match_id::text AS match_id,
    opponent_user_id::text AS opponent_user_id,
    cheater_mmr,
    cheater_rd,
    COALESCE(original_delta, 0) AS original_delta
FROM candidate_matches
WHERE original_delta IS NOT NULL
ORDER BY ended_at ASC, match_id ASC;

-- name: ListCommunityPardonCandidates :many
SELECT u2.id::text AS user_id,
       COALESCE(max(ml.created_at) FILTER (WHERE ml.action IN ('permanent_ban', 'temporary_ban')), u2.banned_at) AS sanction_started_at
FROM users u2
LEFT JOIN moderation_log ml ON ml.subject_user_id = u2.id AND ml.action IN ('permanent_ban', 'temporary_ban', 'unban')
WHERE u2.banned_at IS NOT NULL
  AND (u2.ban_expires_at IS NULL OR u2.ban_expires_at > now())
GROUP BY u2.id
HAVING COALESCE(max(ml.created_at) FILTER (WHERE ml.action IN ('permanent_ban', 'temporary_ban')), u2.banned_at) < $1
   AND (max(ml.created_at) FILTER (WHERE ml.action IN ('permanent_ban', 'temporary_ban')) IS NULL
        OR max(ml.created_at) FILTER (WHERE ml.action = 'unban') IS NULL
        OR max(ml.created_at) FILTER (WHERE ml.action = 'unban') < max(ml.created_at) FILTER (WHERE ml.action IN ('permanent_ban', 'temporary_ban')));

-- name: ListMatchGuessPlayers :many
SELECT DISTINCT user_id::text AS user_id
FROM ranked_guess_events
WHERE match_id = $1;

-- name: ListRecentGuessEvents :many
SELECT match_id::text AS match_id, round_number, score, guess_ms, evidence, occurred_at
FROM ranked_guess_events
WHERE user_id = $1
ORDER BY occurred_at DESC, round_number DESC
LIMIT 50;

-- name: LockOpponentRating :one
SELECT mmr, rd, updated_at
FROM ranks
WHERE user_id = $1 AND mode = $2 AND season_id = $3
FOR UPDATE;

-- name: PardonBannedPlayers :many
WITH candidates AS (
    SELECT u2.id
    FROM users u2
    LEFT JOIN moderation_log ml ON ml.subject_user_id = u2.id AND ml.action IN ('permanent_ban', 'temporary_ban', 'unban')
    WHERE u2.banned_at IS NOT NULL
      AND (u2.ban_expires_at IS NULL OR u2.ban_expires_at > now())
    GROUP BY u2.id
    HAVING COALESCE(max(ml.created_at) FILTER (WHERE ml.action IN ('permanent_ban', 'temporary_ban')), u2.banned_at) < $1
       AND (max(ml.created_at) FILTER (WHERE ml.action IN ('permanent_ban', 'temporary_ban')) IS NULL
            OR max(ml.created_at) FILTER (WHERE ml.action = 'unban') IS NULL
            OR max(ml.created_at) FILTER (WHERE ml.action = 'unban') < max(ml.created_at) FILTER (WHERE ml.action IN ('permanent_ban', 'temporary_ban')))
)
UPDATE users u
SET banned_at = NULL, ban_reason = NULL, ban_expires_at = NULL
FROM candidates c
WHERE u.id = c.id
RETURNING u.id::text AS user_id;

-- name: RevokeIPSignupBan :exec
UPDATE ip_signup_bans
SET revoked_at = COALESCE(revoked_at, now())
WHERE ip_address = $1;

-- name: RevokeOAuthIdentityBans :execrows
UPDATE oauth_identity_bans
SET revoked_at = COALESCE(revoked_at, now())
WHERE banned_user_id = $1 AND revoked_at IS NULL;

-- name: SetChatMute :execrows
UPDATE users
SET chat_muted_at = now(), chat_mute_reason = NULLIF(sqlc.arg(reason)::text, ''), chat_mute_expires_at = sqlc.arg(chat_mute_expires_at)
WHERE id = sqlc.arg(user_id);

-- name: SetEloRefundNotification :exec
UPDATE elo_refunds
SET notification_id = $4
WHERE user_id = $1 AND match_id = $2 AND cheater_user_id = $3;

-- name: SetReportMute :execrows
UPDATE users
SET report_muted_at = now(), report_mute_reason = NULLIF(sqlc.arg(reason)::text, ''), report_mute_expires_at = sqlc.arg(report_mute_expires_at)
WHERE id = sqlc.arg(user_id);
