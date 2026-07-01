drop index if exists idx_match_players_user_history;

create index if not exists idx_match_players_user_id
  on match_players(user_id, match_id desc);

alter table match_players
  drop column if exists ended_at;
