-- name: GetMatchSessionReturnTarget :one
SELECT COALESCE(return_target_kind, 'home') AS return_target_kind,
       return_target_map_id,
       return_target_party_id
FROM match_sessions WHERE match_id = $1;

-- name: GetMatchSessionStatus :one
-- starting, live, ended or interrupted (gd_match_status).
SELECT gd_match_status(ended_at, node_id, node_epoch, created_at)::text AS status
FROM match_sessions WHERE match_id = $1;

-- name: GetMatchSession :one
SELECT match_id, kind, gd_match_status(ended_at, node_id, node_epoch, created_at)::text AS status,
       coalesce(node_url, '') AS node_url, config_json, source_party_id,
       COALESCE(return_target_kind, 'home') AS return_target_kind, return_target_map_id, return_target_party_id,
       created_at, ended_at, coalesce(outcome, '') AS outcome
FROM match_sessions WHERE match_id = $1;

-- name: ListMatchSeats :many
SELECT user_id, coalesce(team_id, '') AS team_id, display_name, avatar_url, active
FROM match_participants WHERE match_id = $1
ORDER BY user_id;

-- name: LockActiveSeats :many
-- The unfinished matches these players are seated in, locked until the caller's transaction ends.
SELECT mp.user_id, mp.match_id, ms.kind, gd_match_status(ms.ended_at, ms.node_id, ms.node_epoch, ms.created_at)::text AS status
FROM match_participants mp
JOIN match_sessions ms ON ms.match_id = mp.match_id
WHERE mp.user_id = ANY(sqlc.arg(user_ids)::uuid[]) AND mp.active
ORDER BY mp.user_id
FOR UPDATE OF mp, ms;

-- name: EndMatch :execrows
-- Ends an open match with an outcome and frees its players' seats, also when it had already ended.
WITH ended AS (
    UPDATE match_sessions s
    SET ended_at = now(), outcome = sqlc.arg(outcome)
    WHERE s.match_id = sqlc.arg(match_id) AND s.ended_at IS NULL
    RETURNING s.match_id
), released AS (
    UPDATE match_participants mp SET active = false
    WHERE mp.match_id = sqlc.arg(match_id) AND mp.active
)
SELECT ended.match_id FROM ended;

-- name: EndInterruptedMatches :execrows
-- Ends the matches whose node is gone or that no node picked up, freeing their players' seats.
WITH ended AS (
    UPDATE match_sessions s
    SET ended_at = now(), outcome = 'interrupted'
    WHERE s.ended_at IS NULL
      AND gd_match_status(s.ended_at, s.node_id, s.node_epoch, s.created_at) = 'interrupted'
    RETURNING s.match_id
), released AS (
    UPDATE match_participants mp SET active = false
    FROM ended WHERE mp.match_id = ended.match_id AND mp.active
)
SELECT ended.match_id FROM ended;

-- name: InsertMatchSession :exec
INSERT INTO match_sessions(
    match_id, kind, source_party_id, config_json, map_id, spec_json,
    return_target_kind, return_target_map_id, return_target_party_id
)
VALUES(
    sqlc.arg(match_id), sqlc.arg(kind), sqlc.narg(source_party_id),
    convert_from(sqlc.arg(config_json), 'UTF8')::jsonb, sqlc.narg(map_id),
    convert_from(sqlc.arg(spec_json), 'UTF8')::jsonb,
    sqlc.arg(return_target_kind), sqlc.narg(return_target_map_id), sqlc.narg(return_target_party_id)
);

-- name: InsertMatchSeat :execrows
-- Seats a player in a new match. No row is written when the player is already in another one.
INSERT INTO match_participants(match_id, user_id, team_id, display_name, avatar_url, joined_party_at, active)
VALUES(sqlc.arg(match_id), sqlc.arg(user_id), sqlc.narg(team_id), sqlc.arg(display_name), sqlc.arg(avatar_url), sqlc.narg(joined_party_at), true)
ON CONFLICT (user_id) WHERE active DO NOTHING;

-- name: ParticipantJoinedAt :one
SELECT joined_at FROM party_members WHERE party_id = $1 AND user_id = $2;

-- name: PickUpWaitingMatch :one
-- The oldest match no node has picked up yet, locked for this node.
SELECT match_id, kind, config_json, spec_json
FROM match_sessions
WHERE node_id IS NULL AND ended_at IS NULL AND created_at > now() - interval '15 seconds'
ORDER BY created_at
LIMIT 1
FOR UPDATE SKIP LOCKED;

-- name: PlaceMatch :exec
UPDATE match_sessions
SET node_id = sqlc.arg(node_id), node_epoch = sqlc.arg(node_epoch), node_url = sqlc.arg(node_url), started_at = now()
WHERE match_id = sqlc.arg(match_id) AND node_id IS NULL AND ended_at IS NULL;

-- name: ListNodeOpenMatches :many
-- The matches this node process runs that have not ended.
SELECT match_id FROM match_sessions
WHERE node_id = sqlc.arg(node_id) AND node_epoch = sqlc.arg(node_epoch) AND ended_at IS NULL;

-- name: GetActiveMatchForUser :one
-- The match the player is in now, starting or live.
SELECT ms.match_id, ms.kind, gd_match_status(ms.ended_at, ms.node_id, ms.node_epoch, ms.created_at)::text AS status
FROM match_participants mp
JOIN match_sessions ms ON ms.match_id = mp.match_id
WHERE mp.user_id = $1 AND mp.active
  AND gd_match_status(ms.ended_at, ms.node_id, ms.node_epoch, ms.created_at) IN ('starting', 'live');

-- name: ListUsersInLiveMatches :many
SELECT mp.user_id
FROM match_participants mp
JOIN match_sessions ms ON ms.match_id = mp.match_id
WHERE mp.user_id = ANY(sqlc.arg(user_ids)::uuid[]) AND mp.active
  AND gd_match_status(ms.ended_at, ms.node_id, ms.node_epoch, ms.created_at) IN ('starting', 'live');
