create table if not exists ranked_stats (
  user_id text not null references users(id),
  mode text not null,
  season_id text not null,
  games_played integer not null default 0,
  wins integer not null default 0,
  updated_at timestamptz not null default now(),
  primary key (user_id, mode, season_id)
);

insert into ranked_stats (user_id, mode, season_id, games_played, wins)
select r.user_id, r.mode, r.season_id, 0, 0
from ranks r
on conflict (user_id, mode, season_id) do nothing;
