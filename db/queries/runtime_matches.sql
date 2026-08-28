-- name: ExpireStaleRuntimeMatches :exec
update runtime_matches
set state = sqlc.arg(new_state), ended_at = now()
where runtime_matches.state = sqlc.arg(current_state) and exists (select 1 from match_sessions ms where ms.match_id = runtime_matches.id and ms.mode = sqlc.arg(mode))
  and started_at < now() - sqlc.arg(older_than)::interval and ended_at is null;

-- name: GetRuntimeMatch :one
select id, state, owner_epoch, started_at, coalesce(ended_at, '0001-01-01 00:00:00+00'::timestamptz) as ended_at
from runtime_matches where id = $1;

-- name: RecordRuntimeMatchLive :exec
insert into runtime_matches(id, state, owner_epoch, started_at)
values ($1, $2, $3, now())
on conflict (id) do update set state = excluded.state, owner_epoch = excluded.owner_epoch;

-- name: RecordRuntimeMatchTerminal :exec
insert into runtime_matches(id, state, owner_epoch, started_at, ended_at)
values ($1, $2, $3, now(), now())
on conflict (id) do update set state = excluded.state, owner_epoch = excluded.owner_epoch, ended_at = now();
