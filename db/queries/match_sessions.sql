-- name: GetMatchSessionReturnTarget :one
SELECT COALESCE(return_target_kind, 'home') AS return_target_kind,
       COALESCE(return_target_map_id::text, '') AS return_target_map_id,
       COALESCE(return_target_party_id::text, '') AS return_target_party_id
FROM match_sessions WHERE match_id = $1;

-- name: GetMatchSessionSource :one
SELECT COALESCE(source_party_id::text, '')::text AS source_party_id,
       COALESCE(source_party_invite_code, '') AS source_party_invite_code
FROM match_sessions WHERE match_id = $1;

-- name: ParticipantJoinedAt :one
SELECT joined_at FROM party_members WHERE party_id = $1 AND user_id = $2;

-- name: RenewMatchSessionLeases :exec
UPDATE match_sessions
SET lease_expires_at = now() + sqlc.arg(ttl)::interval, updated_at = now()
WHERE node_id = sqlc.arg(node_id) AND node_epoch = sqlc.arg(node_epoch) AND state = 'live'
  AND match_id::text = ANY(sqlc.arg(match_ids)::text[]);

-- name: UpsertMatchParticipant :exec
INSERT INTO match_participants(match_id, user_id, team_id, display_name, avatar_url, joined_party_at)
VALUES(sqlc.arg(match_id), sqlc.arg(user_id), NULLIF(sqlc.arg(team_id)::text, ''), sqlc.arg(display_name), sqlc.arg(avatar_url), sqlc.arg(joined_party_at))
ON CONFLICT (match_id, user_id) DO UPDATE SET
    team_id = excluded.team_id,
    display_name = excluded.display_name,
    avatar_url = excluded.avatar_url,
    joined_party_at = COALESCE(match_participants.joined_party_at, excluded.joined_party_at);

-- name: UpsertMatchSession :exec

INSERT INTO match_sessions(
    match_id, preset_id, mode, state, ranked, source_kind,
    source_party_id, source_party_invite_code,
    node_id, node_epoch, public_route,
    config_json, map_id,
    return_target_kind, return_target_map_id, return_target_party_id,
    lease_expires_at, updated_at
)
VALUES(
    sqlc.arg(match_id), sqlc.arg(preset_id), sqlc.arg(mode), 'live', sqlc.arg(ranked), sqlc.arg(source_kind),
    NULLIF(sqlc.arg(source_party_id)::text, '')::uuid, NULLIF(sqlc.arg(source_party_invite_code)::text, ''),
    sqlc.arg(node_id), sqlc.arg(node_epoch), sqlc.arg(public_route),
    sqlc.arg(config_json)::jsonb, NULLIF(sqlc.arg(map_id)::text, '')::uuid,
    NULLIF(sqlc.arg(return_target_kind)::text, ''), NULLIF(sqlc.arg(return_target_map_id)::text, '')::uuid, NULLIF(sqlc.arg(return_target_party_id)::text, '')::uuid,
    now() + sqlc.arg(lease_ttl)::interval, now()
)
ON CONFLICT (match_id) DO UPDATE SET
    preset_id = excluded.preset_id,
    mode = excluded.mode,
    state = CASE WHEN match_sessions.state = 'ended' THEN match_sessions.state ELSE excluded.state END,
    ranked = excluded.ranked,
    source_kind = excluded.source_kind,
    source_party_id = excluded.source_party_id,
    source_party_invite_code = excluded.source_party_invite_code,
    node_id = excluded.node_id,
    node_epoch = excluded.node_epoch,
    public_route = excluded.public_route,
    config_json = excluded.config_json,
    map_id = excluded.map_id,
    return_target_kind = excluded.return_target_kind,
    return_target_map_id = excluded.return_target_map_id,
    return_target_party_id = excluded.return_target_party_id,
    lease_expires_at = CASE WHEN match_sessions.state = 'ended' THEN match_sessions.lease_expires_at ELSE excluded.lease_expires_at END,
    updated_at = now();
