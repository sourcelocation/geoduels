-- name: AddPartyOwner :exec
INSERT INTO party_members (party_id, user_id, role, ready, team_id)
VALUES ($1, $2, 'owner', false, 'a');

-- name: CloseInactiveOpenParties :execrows
UPDATE parties SET state = 'closed', updated_at = now()
WHERE state = 'open' AND id = ANY($1::uuid[])
  AND updated_at < now() - ($2::double precision * interval '1 second');

-- name: CloseParty :exec
UPDATE parties SET state = 'closed', updated_at = now()
WHERE id = $1 AND state = 'open';

-- name: CountActivePartyMembers :one
SELECT count(*) FROM party_members WHERE party_id = $1 AND left_at IS NULL AND user_id <> $2;

-- name: CreateParty :exec
INSERT INTO parties (id, invite_code, owner_user_id, state, mode, map_scope, expires_at, map_id)
VALUES ($1, $2, $3, 'open', $4, $5, $6, $7);

-- name: EndSessionsForEndedRuntimeMatches :exec
UPDATE match_sessions ms
SET state = 'ended',
    ended_at = COALESCE(ms.ended_at, now()),
    updated_at = now()
FROM runtime_matches rm
WHERE rm.id = ms.match_id AND rm.state = $1 AND ms.state <> 'ended';

-- name: ExpireOpenParties :execrows
UPDATE parties SET state = 'expired', updated_at = now()
WHERE state = 'open' AND expires_at < now();

-- name: ExpireParty :exec
UPDATE parties SET state = 'expired', updated_at = now()
WHERE id = $1 AND state = 'open';

-- name: GetPartyOwnerAndState :one
SELECT owner_user_id, state FROM parties WHERE id = $1;

-- name: GetPartySnapshotByID :one
SELECT l.id, l.invite_code, l.owner_user_id, l.state, l.mode, l.map_scope,
       COALESCE(l.active_match_id::text, l.started_match_id::text, '') AS active_match_id,
       COALESCE(l.last_match_id::text, '') AS last_match_id,
       COALESCE(l.started_match_id::text, '') AS started_match_id,
       l.created_at, l.expires_at, l.config_json::text, COALESCE(l.map_id::text, '') AS map_id,
       COALESCE(mp.display_name, ''), COALESCE(mp.location_count, 0)
FROM parties l LEFT JOIN maps mp ON mp.id = l.map_id
WHERE l.id = $1;

-- name: GetPartySnapshotByInviteCode :one
SELECT l.id, l.invite_code, l.owner_user_id, l.state, l.mode, l.map_scope,
       COALESCE(l.active_match_id::text, l.started_match_id::text, '') AS active_match_id,
       COALESCE(l.last_match_id::text, '') AS last_match_id,
       COALESCE(l.started_match_id::text, '') AS started_match_id,
       l.created_at, l.expires_at, l.config_json::text, COALESCE(l.map_id::text, '') AS map_id,
       COALESCE(mp.display_name, ''), COALESCE(mp.location_count, 0)
FROM parties l LEFT JOIN maps mp ON mp.id = l.map_id
WHERE l.invite_code = $1;

-- name: GetPartySnapshotByMatchID :one
SELECT l.id, l.invite_code, l.owner_user_id, l.state, l.mode, l.map_scope,
       COALESCE(l.active_match_id::text, l.started_match_id::text, '') AS active_match_id,
       COALESCE(l.last_match_id::text, '') AS last_match_id,
       COALESCE(l.started_match_id::text, '') AS started_match_id,
       l.created_at, l.expires_at, l.config_json::text, COALESCE(l.map_id::text, '') AS map_id,
       COALESCE(mp.display_name, ''), COALESCE(mp.location_count, 0)
FROM parties l LEFT JOIN maps mp ON mp.id = l.map_id
WHERE l.active_match_id = $1 OR l.last_match_id = $1 OR l.started_match_id = $1;

-- name: GetPartyStateAndExpiry :one
SELECT state, expires_at FROM parties WHERE id = $1;

-- name: GetPartyStateAndOwner :one
SELECT state, owner_user_id FROM parties WHERE id = $1;

-- name: JoinPartyMember :exec
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
    joined_at = CASE WHEN party_members.left_at IS NULL THEN party_members.joined_at ELSE now() END;

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
SELECT m.user_id, u.display_name, COALESCE(u.avatar_url, ''),
       u.account_type = 'guest', COALESCE(u.is_admin, false),
       COALESCE(u.selected_badge_code, 0), COALESCE(m.team_id::text, ''),
       m.role, m.ready, m.joined_at
FROM party_members m JOIN users u ON u.id = m.user_id
WHERE m.party_id = $1 AND m.left_at IS NULL
ORDER BY CASE WHEN m.role = 'owner' THEN 0 ELSE 1 END, m.joined_at;

-- name: LockOpenPartyMode :one
select mode from parties where id=$1 and state='open' for update;

-- name: LockOpenPartyOwner :one
select owner_user_id from parties where id=$1 and state='open' for update;

-- name: MarkPartyInMatch :execrows
UPDATE parties
SET state = 'in_match', active_match_id = $2, started_match_id = $2, updated_at = now()
WHERE id = $1 AND state = 'open';

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

-- name: ReopenEndedParties :execrows
WITH ended AS (
    SELECT l.id, l.active_match_id
    FROM parties l
    JOIN runtime_matches rm ON rm.id = l.active_match_id
    WHERE l.state IN ('in_match', 'started') AND rm.state = $1
),
reopened AS (
    UPDATE parties l
    SET state = 'open',
        last_match_id = ended.active_match_id,
        active_match_id = NULL,
        started_match_id = NULL,
        updated_at = now()
    FROM ended
    WHERE l.id = ended.id
    RETURNING l.id
)
UPDATE party_members m
SET ready = false
FROM reopened
WHERE m.party_id = reopened.id;

-- name: ResetPartyMembersReady :exec
UPDATE party_members SET ready = false WHERE party_id = $1;

-- name: SetPartyConfig :exec
UPDATE parties SET config_json = $2::jsonb, map_id = $3, updated_at = now() WHERE id = $1;

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
WHERE id = $1 AND state IN ('open', 'in_match', 'started');

-- name: TransferPartyOwner :exec
UPDATE parties SET owner_user_id = $2, updated_at = now() WHERE id = $1;
