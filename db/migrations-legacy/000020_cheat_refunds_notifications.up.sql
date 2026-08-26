alter table users
  add column if not exists registration_ip_address text;

with latest_session_ip as (
  select distinct on (user_id)
    user_id,
    nullif(trim(ip_address), '') as ip_address
  from auth_sessions
  where nullif(trim(ip_address), '') is not null
  order by user_id, last_used_at desc, created_at desc
)
update users u
set registration_ip_address = latest_session_ip.ip_address
from latest_session_ip
where u.id = latest_session_ip.user_id
  and nullif(trim(u.registration_ip_address), '') is null;

create index if not exists idx_users_registration_ip_created
on users(registration_ip_address, created_at desc)
where registration_ip_address is not null;

alter table match_players
  add column if not exists rating_rd double precision,
  add column if not exists ranked_games_played integer not null default 0;

update match_players mp
set rating_rd = nullif(mp_snapshot.player_json->>'ratingRd', '')::double precision,
    ranked_games_played = coalesce(nullif(mp_snapshot.player_json->>'rankedGamesPlayed', '')::integer, ranked_games_played)
from (
  select
    h.match_id,
    players.key as user_id,
    players.value as player_json
  from match_history h
  cross join lateral jsonb_each(h.snapshot_json->'players') as players(key, value)
) mp_snapshot
where mp.match_id = mp_snapshot.match_id
  and mp.user_id = mp_snapshot.user_id
  and mp.rating_rd is null;

alter table elo_refunds
  add column if not exists victim_mmr_before integer,
  add column if not exists victim_mmr_after integer,
  add column if not exists computed_refund_delta integer,
  add column if not exists notification_id bigint,
  add column if not exists created_by_reason text;

create table if not exists user_notifications (
  id bigserial primary key,
  user_id text not null references users(id) on delete cascade,
  type text not null,
  dedupe_key text not null unique,
  payload_json jsonb not null default '{}'::jsonb,
  read_at timestamptz,
  created_at timestamptz not null default now()
);

create index if not exists idx_user_notifications_unread
on user_notifications(user_id, created_at desc)
where read_at is null;

create table if not exists ranked_guess_events (
  id bigserial primary key,
  user_id text not null references users(id) on delete cascade,
  match_id text not null references match_history(match_id) on delete cascade,
  round_id text not null,
  round_number integer not null,
  ruleset text not null default 'moving',
  score integer not null,
  guess_ms bigint not null,
  evidence double precision not null default 0,
  occurred_at timestamptz not null,
  created_at timestamptz not null default now(),
  unique (match_id, round_id, user_id)
);

create index if not exists idx_ranked_guess_events_user_recent
on ranked_guess_events(user_id, occurred_at desc, id desc)
include (score, guess_ms, evidence);

create index if not exists idx_ranked_guess_events_match
on ranked_guess_events(match_id);
