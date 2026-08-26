alter table match_players
  add column if not exists ended_at timestamptz;

update match_players p
set ended_at = h.ended_at
from match_history h
where h.match_id = p.match_id
  and p.ended_at is null;

alter table match_players
  alter column ended_at set not null;

drop index if exists idx_match_players_user_id;
create index idx_match_players_user_history
  on match_players(user_id, ended_at desc, match_id desc);
