-- name: AddPartyOwner :exec
WITH member AS (
    INSERT INTO party_members (party_id, user_id, role, ready, team_id)
    VALUES ($1, $2, 'owner', false, 'a')
    RETURNING user_id, party_id
)
INSERT INTO current_parties (user_id, party_id)
SELECT user_id, party_id FROM member
ON CONFLICT (user_id) DO UPDATE SET party_id = excluded.party_id;

-- name: CloseInactiveOpenParties :execrows
UPDATE parties SET state = 'closed', updated_at = now()
WHERE state = 'open' AND id = ANY(sqlc.arg(party_ids)::uuid[])
  AND updated_at < now() - (sqlc.arg(inactive_seconds)::double precision * interval '1 second')
  AND NOT EXISTS (SELECT 1 FROM match_sessions ms WHERE ms.match_id = parties.last_match_id AND ms.ended_at IS NULL);

-- name: CloseParty :exec
UPDATE parties SET state = 'closed', updated_at = now()
WHERE id = $1 AND state = 'open';

-- name: CountActivePartyMembers :one
SELECT count(*) FROM party_members WHERE party_id = $1 AND left_at IS NULL AND user_id <> $2;

-- name: CreateParty :exec
INSERT INTO parties (id, invite_code, owner_user_id, state, mode, map_scope, expires_at, map_id)
VALUES ($1, $2, $3, 'open', $4, $5, $6, $7);

-- name: ExpireParty :exec
UPDATE parties SET state = 'expired', updated_at = now()
WHERE id = $1 AND state = 'open';

-- name: GetPartyOwnerAndState :one
SELECT owner_user_id, state FROM parties WHERE id = $1;

-- name: GetPartySnapshotByID :one
SELECT l.id, l.invite_code, l.owner_user_id, l.state, l.mode, l.map_scope,
       l.last_match_id AS last_match_id,
       COALESCE(gd_match_status(ms.ended_at, ms.node_id, ms.node_epoch, ms.created_at), '')::text AS last_match_status,
       l.created_at, l.expires_at, l.config_json, l.map_id AS map_id,
       COALESCE(mp.display_name, ''), COALESCE(mp.location_count, 0)
FROM parties l LEFT JOIN maps mp ON mp.id = l.map_id
LEFT JOIN match_sessions ms ON ms.match_id = l.last_match_id
WHERE l.id = $1;

-- name: GetPartySnapshotByInviteCode :one
SELECT l.id, l.invite_code, l.owner_user_id, l.state, l.mode, l.map_scope,
       l.last_match_id AS last_match_id,
       COALESCE(gd_match_status(ms.ended_at, ms.node_id, ms.node_epoch, ms.created_at), '')::text AS last_match_status,
       l.created_at, l.expires_at, l.config_json, l.map_id AS map_id,
       COALESCE(mp.display_name, ''), COALESCE(mp.location_count, 0)
FROM parties l LEFT JOIN maps mp ON mp.id = l.map_id
LEFT JOIN match_sessions ms ON ms.match_id = l.last_match_id
WHERE l.invite_code = $1;

-- name: GetPartyStateAndExpiry :one
-- An open party is in_match while its last match is starting or live.
SELECT CASE WHEN l.state = 'open' AND gd_match_status(ms.ended_at, ms.node_id, ms.node_epoch, ms.created_at) IN ('starting', 'live') THEN 'in_match' ELSE l.state::text END::text AS state, l.expires_at
FROM parties l LEFT JOIN match_sessions ms ON ms.match_id = l.last_match_id
WHERE l.id = $1;

-- name: GetPartyStateAndOwner :one
SELECT CASE WHEN l.state = 'open' AND gd_match_status(ms.ended_at, ms.node_id, ms.node_epoch, ms.created_at) IN ('starting', 'live') THEN 'in_match' ELSE l.state::text END::text AS state, l.owner_user_id
FROM parties l LEFT JOIN match_sessions ms ON ms.match_id = l.last_match_id
WHERE l.id = $1;

-- name: JoinPartyMember :exec
WITH member AS (
INSERT INTO party_members(party_id, user_id, role, ready, team_id, left_at)
VALUES($1, $2, $3, false, (
    SELECT CASE
        WHEN count(*) FILTER (WHERE team_id = 'a') <= count(*) FILTER (WHERE team_id = 'b') THEN 'a'::gd_team_id
        ELSE 'b'::gd_team_id
    END
    FROM party_members
    WHERE party_id = $1 AND left_at IS NULL
), NULL)
ON CONFLICT (party_id, user_id) DO UPDATE SET
    role = CASE WHEN party_members.role = 'owner' THEN 'owner'::gd_party_role ELSE excluded.role END,
    team_id = COALESCE(party_members.team_id, excluded.team_id),
    left_at = NULL,
    joined_at = CASE WHEN party_members.left_at IS NULL THEN party_members.joined_at ELSE now() END
RETURNING user_id, party_id
)
INSERT INTO current_parties (user_id, party_id)
SELECT user_id, party_id FROM member
ON CONFLICT (user_id) DO UPDATE SET party_id = excluded.party_id;

-- name: KickPartyMember :execrows
UPDATE party_members SET left_at = now(), ready = false
WHERE party_id = $1 AND user_id = $2 AND role <> 'owner' AND left_at IS NULL;

-- name: LeavePartyMember :execrows
UPDATE party_members SET left_at = now(), ready = false
WHERE party_id = $1 AND user_id = $2 AND left_at IS NULL;

-- name: ListOpenPartyIDs :many
SELECT id FROM parties WHERE state = 'open' ORDER BY updated_at;

-- name: ListPartyMemberBadges :many
SELECT ub.user_id, ub.badge_code, COALESCE(ub.level, 1), COALESCE(ub.extra, 0)
FROM user_badges ub
WHERE ub.user_id = ANY($1::uuid[])
ORDER BY ub.user_id ASC, ub.awarded_at DESC, ub.badge_code ASC;

-- name: ListPartyMembers :many
SELECT m.user_id, u.display_name, COALESCE(u.avatar_url, '') AS avatar_url,
       gd_is_guest(u.id) AS is_guest, gd_is_admin(u.id) AS is_admin,
       COALESCE(u.selected_badge_code, 0) AS selected_badge_code, m.team_id AS team_id,
       m.role, m.ready, m.joined_at, m.seen_at,
       EXISTS (
           SELECT 1 FROM parties p JOIN match_participants mp ON mp.match_id = p.last_match_id
           WHERE p.id = m.party_id AND mp.user_id = m.user_id AND mp.active
       ) AS in_active_match
FROM party_members m JOIN users u ON u.id = m.user_id
WHERE m.party_id = $1 AND m.left_at IS NULL
ORDER BY CASE WHEN m.role = 'owner' THEN 0 ELSE 1 END, m.joined_at;

-- name: LockOpenPartyMode :one
select mode from parties where id=$1 and state='open' for update;

-- name: LockOpenPartyOwner :one
select owner_user_id from parties where id=$1 and state='open' for update;

-- name: MarkPartyMatchStarted :execrows
-- The party's match is its last one; it is in that match while the match is live.
WITH started AS (
    UPDATE parties SET last_match_id = $2, updated_at = now()
    WHERE id = $1 AND state = 'open'
    RETURNING id
)
UPDATE party_members m SET ready = false FROM started WHERE m.party_id = started.id;

-- name: NextPartyOwnerID :one
SELECT user_id FROM party_members
WHERE party_id = $1 AND left_at IS NULL
ORDER BY joined_at ASC LIMIT 1;

-- name: PartyMapAccessible :one
select exists(select 1 from maps where id=$1 and archived_at is null and status='ready' and (owner_user_id is null or owner_user_id=$2 or published_at is not null or visibility='unlisted'));

-- name: PartyMemberActive :one
SELECT exists(
    SELECT 1 FROM party_members
    WHERE party_id = $1 AND user_id = $2 AND left_at IS NULL
);

-- name: ReassignPartyRoles :exec
UPDATE party_members
SET role = CASE
    WHEN user_id = $2 THEN 'owner'::gd_party_role
    ELSE 'member'::gd_party_role
END
WHERE party_id = $1 AND left_at IS NULL;

-- name: ResetPartyMembersReady :exec
UPDATE party_members SET ready = false WHERE party_id = $1;

-- name: SetPartyConfig :exec
UPDATE parties SET config_json = convert_from(sqlc.arg(config_json), 'UTF8')::jsonb, map_id = sqlc.arg(map_id), updated_at = now() WHERE id = sqlc.arg(party_id);

-- name: SetPartyMemberTeam :execrows
UPDATE party_members SET team_id = $3
WHERE party_id = $1 AND user_id = $2 AND left_at IS NULL;

-- name: SetPartyMode :exec
UPDATE parties SET mode = $2, updated_at = now() WHERE id = $1;

-- name: ShufflePartyTeams :exec
with shuffled_members as (select user_id,row_number() over(order by random()) position from party_members where party_id=$1 and left_at is null) update party_members m set team_id=case when shuffled.position%2=1 then 'a'::gd_team_id else 'b'::gd_team_id end from shuffled_members shuffled where m.party_id=$1 and m.user_id=shuffled.user_id;

-- name: TouchOpenParty :exec
UPDATE parties SET updated_at = now()
WHERE id = $1 AND state = 'open';

-- name: TouchPartyUpdated :exec
UPDATE parties SET updated_at = now()
WHERE id = $1 AND state = 'open';

-- name: TouchPartyMemberSeen :one
-- Marks the member as having the party open now, returning when they did before.
WITH previous AS (
    SELECT seen_at FROM party_members
    WHERE party_id = $1 AND user_id = $2 AND left_at IS NULL
    FOR UPDATE
)
UPDATE party_members m SET seen_at = now()
FROM previous
WHERE m.party_id = $1 AND m.user_id = $2 AND m.left_at IS NULL
RETURNING previous.seen_at AS previous_seen_at;

-- name: ClearPartyMemberSeen :execrows
UPDATE party_members SET seen_at = NULL
WHERE party_id = $1 AND user_id = $2 AND seen_at IS NOT NULL;

-- name: TransferPartyOwner :exec
UPDATE parties SET owner_user_id = $2, updated_at = now() WHERE id = $1;

-- name: GetCurrentParty :one
SELECT p.id, p.invite_code
FROM current_parties c
JOIN parties p ON p.id = c.party_id
JOIN party_members m ON m.party_id = c.party_id AND m.user_id = c.user_id
WHERE c.user_id = $1 AND m.left_at IS NULL
  AND p.state = 'open'
  AND (p.expires_at > now() OR EXISTS (SELECT 1 FROM match_sessions ms WHERE ms.match_id = p.last_match_id AND ms.ended_at IS NULL));
