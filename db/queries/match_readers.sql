-- name: GetFinalMatchSnapshot :one
select replay_zstd, coalesce(replay_codec, 0)::int AS replay_codec, coalesce(replay_uncompressed_bytes, 0) AS replay_uncompressed_bytes, replay_sha256, replay_json::text AS replay_json
from match_history where match_id = $1 and (replay_expires_at is null or replay_expires_at > now()) limit 1;

-- name: ListPlayerMatchHistory :many
select h.match_id, h.mode, h.started_at, h.ended_at, coalesce(h.winner_user_id::text, '') AS winner_user_id,
case when h.mode = 'singleplayer' then 'completed' when h.winner_user_id is null then 'draw' when h.winner_user_id = p.user_id then 'win' else 'loss' end AS outcome,
coalesce(h.ranked, false) and h.mode = 'duel' AS ranked, coalesce(p.final_ranked_delta, 0) AS ranked_delta, coalesce(p.total_score, 0) AS total_score, coalesce(opponent.user_id, '') AS opponent_user_id, coalesce(opponent.display_name, '') AS opponent_display_name
from match_players p join match_history h on h.match_id = p.match_id
left join lateral (select op.user_id::text user_id, coalesce(nullif(op.display_name, ''), nullif(u.display_name, ''), op.user_id::text) display_name from match_players op left join users u on u.id = op.user_id where op.match_id = p.match_id and op.user_id <> p.user_id order by op.total_score desc, op.user_id limit 1) opponent on true
where p.user_id = $1 and h.mode = 'duel' and coalesce(h.ranked, false) and (p.ended_at, p.match_id) < (sqlc.arg(cursor_ended_at), sqlc.arg(cursor_match_id)::uuid)
order by p.ended_at desc, p.match_id desc limit $2;

-- name: ListPlayerMatchHistoryBasic :many
select h.match_id, h.mode, h.started_at, h.ended_at, coalesce(h.winner_user_id::text, '') AS winner_user_id, case when h.mode = 'singleplayer' then 'completed' when h.winner_user_id is null then 'draw' when h.winner_user_id = p.user_id then 'win' else 'loss' end AS outcome, coalesce(h.ranked, false) and h.mode = 'duel' AS ranked, coalesce(p.final_ranked_delta, 0) AS ranked_delta, coalesce(p.total_score, 0) AS total_score, coalesce(opponent.user_id, '') AS opponent_user_id, coalesce(opponent.display_name, '') AS opponent_display_name
from match_players p join match_history h on h.match_id = p.match_id left join lateral (select op.user_id::text user_id, coalesce(nullif(op.display_name, ''), nullif(u.display_name, ''), op.user_id::text) display_name from match_players op left join users u on u.id = op.user_id where op.match_id = p.match_id and op.user_id <> p.user_id order by op.total_score desc, op.user_id limit 1) opponent on true where p.user_id = $1 order by p.ended_at desc, p.match_id desc limit $2;

-- name: ListPlayerMatchHistoryBefore :many
select h.match_id,h.mode,h.started_at,h.ended_at,coalesce(h.winner_user_id::text,'') AS winner_user_id,case when h.mode='singleplayer' then 'completed' when h.winner_user_id is null then 'draw' when h.winner_user_id=p.user_id then 'win' else 'loss' end AS outcome,coalesce(h.ranked,false) and h.mode='duel' AS ranked,coalesce(p.final_ranked_delta,0) AS ranked_delta,coalesce(p.total_score,0) AS total_score,coalesce(opponent.user_id,'') AS opponent_user_id,coalesce(opponent.display_name,'') AS opponent_display_name from match_players p join match_history h on h.match_id=p.match_id left join lateral (select op.user_id::text user_id,coalesce(nullif(op.display_name,''),nullif(u.display_name,''),op.user_id::text) display_name from match_players op left join users u on u.id=op.user_id where op.match_id=p.match_id and op.user_id<>p.user_id order by op.total_score desc,op.user_id limit 1) opponent on true where p.user_id=$1 and (p.ended_at,p.match_id)<(sqlc.arg(cursor_ended_at),sqlc.arg(cursor_match_id)::uuid) order by p.ended_at desc,p.match_id desc limit $2;

-- name: ListPlayerMatchHistoryRanked :many
select h.match_id, h.mode, h.started_at, h.ended_at, coalesce(h.winner_user_id::text, '') AS winner_user_id, case when h.mode = 'singleplayer' then 'completed' when h.winner_user_id is null then 'draw' when h.winner_user_id = p.user_id then 'win' else 'loss' end AS outcome, coalesce(h.ranked, false) and h.mode = 'duel' AS ranked, coalesce(p.final_ranked_delta, 0) AS ranked_delta, coalesce(p.total_score, 0) AS total_score, coalesce(opponent.user_id, '') AS opponent_user_id, coalesce(opponent.display_name, '') AS opponent_display_name from match_players p join match_history h on h.match_id=p.match_id left join lateral (select op.user_id::text user_id, coalesce(nullif(op.display_name,''),nullif(u.display_name,''),op.user_id::text) display_name from match_players op left join users u on u.id=op.user_id where op.match_id=p.match_id and op.user_id<>p.user_id order by op.total_score desc,op.user_id limit 1) opponent on true where p.user_id=$1 and h.mode='duel' and coalesce(h.ranked,false) order by p.ended_at desc,p.match_id desc limit $2;

-- name: PlayerParticipatedInMatch :one
select exists(select 1 from match_players where user_id=$1 and match_id=$2);
