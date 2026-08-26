create table if not exists match_history (
  match_id text primary key,
  mode text not null,
  state text not null,
  started_at timestamptz not null default now(),
  ended_at timestamptz not null default now(),
  winner_user_id text,
  snapshot_json jsonb not null
);

create table if not exists match_players (
  match_id text not null references match_history(match_id) on delete cascade,
  user_id text not null references users(id) on delete cascade,
  display_name text not null,
  mmr integer not null default 0,
  hp integer not null default 0,
  primary key (match_id, user_id)
);
create index if not exists idx_match_players_user_id on match_players(user_id, match_id desc);

create table if not exists match_round_guesses (
  id bigserial primary key,
  match_id text not null references match_history(match_id) on delete cascade,
  round_id text not null,
  round_number integer not null,
  user_id text not null references users(id) on delete cascade,
  lat double precision not null,
  lng double precision not null,
  actual_lat double precision not null,
  actual_lng double precision not null,
  distance_km double precision not null,
  score integer not null,
  guess_unix_ms bigint,
  guess_ms bigint,
  created_at timestamptz not null default now(),
  unique (match_id, round_id, user_id)
);
create index if not exists idx_match_round_guesses_user_id on match_round_guesses(user_id, match_id, round_number);

create table if not exists player_reports (
  id bigserial primary key,
  match_id text not null references match_history(match_id) on delete cascade,
  reporter_user_id text not null references users(id) on delete cascade,
  reported_user_id text not null references users(id) on delete cascade,
  reason text,
  created_at timestamptz not null default now(),
  unique (reporter_user_id, reported_user_id),
  constraint player_reports_no_self_report check (reporter_user_id <> reported_user_id)
);
create index if not exists idx_player_reports_reported on player_reports(reported_user_id, created_at desc);

create table if not exists ip_signup_bans (
  id bigserial primary key,
  ip_address text not null,
  reason text,
  created_by text references users(id) on delete set null,
  created_at timestamptz not null default now(),
  revoked_at timestamptz,
  unique (ip_address)
);
create index if not exists idx_ip_signup_bans_active on ip_signup_bans(ip_address) where revoked_at is null;
