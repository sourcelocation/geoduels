-- name: AddDuelRankedStats :exec
UPDATE ranked_stats stats
SET games_played = stats.games_played + 1,
    wins = stats.wins + result.won::integer,
    updated_at = now()
FROM (
    VALUES
        ($3::uuid, $4::boolean, $5::boolean),
        ($6::uuid, $7::boolean, $8::boolean)
) AS result(user_id, won, apply)
WHERE stats.user_id = result.user_id
  AND stats.mode = $1
  AND stats.season_id = $2
  AND result.apply;

-- name: AddDuelUserStats :exec
UPDATE user_stats stats
SET games_played = stats.games_played + 1,
    wins = stats.wins + result.won::integer,
    updated_at = now()
FROM (
    VALUES
        ($1::uuid, $2::boolean),
        ($3::uuid, $4::boolean)
) AS result(user_id, won)
WHERE stats.user_id = result.user_id;

-- name: CompleteMatchSession :exec
UPDATE match_sessions
SET state = 'ended',
    ended_at = COALESCE(ended_at, now()),
    lease_expires_at = NULL,
    updated_at = now()
WHERE match_id = $1;

-- name: EnsureMatchRankedStats :exec
INSERT INTO ranked_stats (user_id, mode, season_id, games_played, wins)
SELECT input.user_id, $1, $2, 0, 0
FROM jsonb_to_recordset($3::jsonb) AS input(user_id uuid)
ON CONFLICT (user_id, mode, season_id) DO NOTHING;

-- name: EnsureMatchRanks :exec
INSERT INTO ranks (user_id, mode, mmr, season_id)
SELECT input.user_id, $1, $2, $3
FROM jsonb_to_recordset($4::jsonb) AS input(user_id uuid)
ON CONFLICT (user_id, mode, season_id) DO NOTHING;

-- name: EnsureMatchUserStats :exec
INSERT INTO user_stats (user_id, games_played, wins)
SELECT input.user_id, 0, 0
FROM jsonb_to_recordset($1::jsonb) AS input(user_id uuid)
ON CONFLICT (user_id) DO NOTHING;

-- name: EnsureMatchUsers :exec
INSERT INTO users (id, email, display_name, avatar_url, account_type)
SELECT input.user_id, NULL, input.display_name, NULL, 'guest'
FROM jsonb_to_recordset($1::jsonb) AS input(user_id uuid, display_name text)
ON CONFLICT (id) DO NOTHING;

-- name: FindPartyIDByMatchID :one
SELECT id::text AS party_id
FROM parties
WHERE active_match_id = $1 OR started_match_id = $1 OR last_match_id = $1
LIMIT 1;

-- name: GetMatchRoundPlanMapID :one
SELECT map_id::text AS map_id
FROM match_round_plans
WHERE match_id = $1
ORDER BY round_index
LIMIT 1;

-- name: GetMatchSourcePartyID :one
SELECT COALESCE(source_party_id::text, '')::text AS source_party_id
FROM match_sessions WHERE match_id = $1;

-- name: LockDuelRatings :many
SELECT u.id, u.account_type = 'guest' AS is_guest, r.mmr, r.rd, r.updated_at
FROM unnest(ARRAY[$1::uuid, $2::uuid]) WITH ORDINALITY requested(user_id, position)
JOIN users u ON u.id = requested.user_id
JOIN ranks r
  ON r.user_id = u.id
 AND r.mode = $3
 AND r.season_id = $4
ORDER BY requested.position
FOR UPDATE OF r;

-- name: LockMatchSessionState :one

SELECT state FROM match_sessions WHERE match_id = $1 FOR UPDATE;

-- name: MatchBelongsToParty :one
SELECT exists(
    SELECT 1 FROM match_sessions WHERE match_id = $1 AND source_kind = 'party'
) OR exists(
    SELECT 1 FROM parties
    WHERE active_match_id = $1 OR started_match_id = $1 OR last_match_id = $1
) AS private_party_match;

-- name: MatchPlayerPersistedRatings :many
SELECT
    mp.user_id::text AS user_id,
    COALESCE(mp.rating_after, mp.mmr) AS mmr,
    COALESCE(r.rd, mp.rating_rd, $1::double precision) AS rd
FROM match_players mp
LEFT JOIN ranks r
  ON r.user_id = mp.user_id
 AND r.mode = $2
 AND r.season_id = $3
WHERE mp.match_id = $4::uuid
  AND mp.user_id IN (
    SELECT value::uuid FROM jsonb_array_elements_text($5::jsonb)
  );

-- name: RecordRuntimeMatchEnded :exec
INSERT INTO runtime_matches(id, state, owner_epoch, started_at, ended_at)
VALUES($1, $2, $3, now(), now())
ON CONFLICT (id) DO UPDATE SET
    state = excluded.state,
    owner_epoch = excluded.owner_epoch,
    ended_at = now();

-- name: ReopenPartiesAfterMatch :exec
UPDATE parties
SET state = 'open',
    last_match_id = $1,
    active_match_id = NULL,
    started_match_id = NULL,
    updated_at = now()
WHERE active_match_id = $1 OR started_match_id = $1;

-- name: ResetPartyMembersAfterMatch :exec
UPDATE party_members pm
SET ready = false
FROM match_sessions ms
WHERE ms.match_id = $1 AND pm.party_id = ms.source_party_id;

-- name: SetMatchPlayerRatingDeltas :exec
UPDATE match_players players
SET rating_before = result.rating_before,
    rating_after = result.rating_after,
    final_ranked_delta = result.rating_after - result.rating_before
FROM (
    VALUES
        ($2::uuid, $3::integer, $4::integer, $5::boolean),
        ($6::uuid, $7::integer, $8::integer, $9::boolean)
) AS result(user_id, rating_before, rating_after, apply)
WHERE players.match_id = $1::uuid
  AND players.user_id = result.user_id
  AND result.apply;

-- name: UpdateDuelRanks :exec
UPDATE ranks r
SET mmr = next.mmr, rd = next.rd, updated_at = $1
FROM (
    VALUES
        ($4::uuid, $5::integer, $6::double precision, $7::boolean),
        ($8::uuid, $9::integer, $10::double precision, $11::boolean)
) AS next(user_id, mmr, rd, apply)
WHERE r.user_id = next.user_id
  AND r.mode = $2
  AND r.season_id = $3
  AND next.apply;

-- name: UpsertMatchHistory :exec
INSERT INTO match_history(
    match_id, mode, started_at, ended_at, winner_user_id,
    ranked, source_kind, source_party_id, ruleset, map_id,
    replay_zstd, replay_codec, replay_schema_version, replay_uncompressed_bytes,
    replay_sha256, replay_expires_at, round_count
)
VALUES(
    $1::uuid, $2, $3, $4,
    NULLIF($5, '')::uuid,
    $6, $7, NULLIF($8, '')::uuid,
    NULLIF($9, '')::gd_ruleset, NULLIF($10, '')::uuid,
    $11, $12, $13,
    $14, $15,
    $4::timestamptz + make_interval(days => $16::integer),
    $17
)
ON CONFLICT (match_id) DO UPDATE SET
    mode = excluded.mode,
    started_at = excluded.started_at,
    ended_at = excluded.ended_at,
    winner_user_id = excluded.winner_user_id,
    ranked = excluded.ranked,
    source_kind = excluded.source_kind,
    source_party_id = excluded.source_party_id,
    ruleset = excluded.ruleset,
    map_id = excluded.map_id,
    replay_json = NULL,
    replay_zstd = excluded.replay_zstd,
    replay_codec = excluded.replay_codec,
    replay_schema_version = excluded.replay_schema_version,
    replay_uncompressed_bytes = excluded.replay_uncompressed_bytes,
    replay_sha256 = excluded.replay_sha256,
    replay_expires_at = excluded.replay_expires_at,
    round_count = excluded.round_count;

-- name: UpsertMatchPlayers :exec
INSERT INTO match_players(
    match_id, user_id, display_name, mmr, hp, rating_rd, total_score, ended_at
)
SELECT
    $1::uuid, input.user_id, input.display_name, input.mmr, input.hp,
    input.rating_rd, input.total_score, $2
FROM jsonb_to_recordset($3::jsonb) AS input(
    user_id uuid,
    display_name text,
    mmr integer,
    hp integer,
    rating_rd double precision,
    total_score integer
)
ON CONFLICT (match_id, user_id) DO UPDATE SET
    display_name = excluded.display_name,
    mmr = excluded.mmr,
    hp = excluded.hp,
    rating_rd = excluded.rating_rd,
    total_score = excluded.total_score,
    ended_at = excluded.ended_at;

-- name: UpsertPlayerMapBests :exec
INSERT INTO player_map_bests(user_id, map_id, ruleset, best_score, match_id, achieved_at)
SELECT input.user_id, $1::uuid, $2, input.total_score, $3::uuid, $4
FROM jsonb_to_recordset($5::jsonb) AS input(user_id uuid, total_score integer)
ON CONFLICT (user_id, map_id, ruleset) DO UPDATE
SET best_score = excluded.best_score,
    match_id = excluded.match_id,
    achieved_at = excluded.achieved_at
WHERE excluded.best_score > player_map_bests.best_score;

-- name: UpsertRankedGuessEvents :exec
INSERT INTO ranked_guess_events(
    user_id, match_id, round_number, score, guess_ms, evidence, occurred_at
)
SELECT
    input.user_id, $1::uuid, input.round_number, input.score,
    input.guess_ms, input.evidence, input.occurred_at
FROM jsonb_to_recordset($2::jsonb) AS input(
    user_id uuid,
    round_number smallint,
    score smallint,
    guess_ms integer,
    evidence real,
    occurred_at timestamptz
)
ON CONFLICT (match_id, round_number, user_id) DO UPDATE SET
    score = excluded.score,
    guess_ms = excluded.guess_ms,
    evidence = excluded.evidence,
    occurred_at = excluded.occurred_at;
