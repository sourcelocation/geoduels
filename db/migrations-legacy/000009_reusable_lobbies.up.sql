alter table lobbies
  add column if not exists active_match_id text,
  add column if not exists last_match_id text;

update lobbies
set active_match_id = coalesce(active_match_id, started_match_id)
where state = 'started'
  and started_match_id is not null;

alter table lobbies
  drop constraint if exists lobbies_state_check;

alter table lobbies
  add constraint lobbies_state_check check (state in ('open', 'in_match', 'started', 'closed', 'expired'));

update lobbies
set state = 'in_match'
where state = 'started';

create index if not exists idx_lobbies_active_match_id on lobbies(active_match_id);
create index if not exists idx_lobbies_last_match_id on lobbies(last_match_id);
