-- Core schema for local PostgreSQL
do $$
begin
  if exists (select 1 from pg_available_extensions where name = 'postgis') then
    create extension if not exists postgis;
  end if;
end
$$;

create table if not exists users (
  id text primary key,
  email text unique,
  display_name text not null,
  avatar_url text,
  onboarded_at timestamptz,
  account_type text not null default 'registered',
  created_at timestamptz not null default now()
);

alter table users
  drop constraint if exists users_account_type_check;

alter table users
  add constraint users_account_type_check
  check (account_type in ('guest', 'registered'));

create table if not exists user_identities (
  user_id text not null references users(id) on delete cascade,
  provider text not null,
  provider_user_id text not null,
  email text,
  provider_name text,
  avatar_url text,
  created_at timestamptz not null default now(),
  last_seen_at timestamptz not null default now(),
  primary key (user_id, provider),
  unique (provider, provider_user_id),
  constraint user_identities_provider_not_guest check (provider <> 'guest')
);

create table if not exists auth_sessions (
  id text primary key,
  user_id text not null references users(id) on delete cascade,
  refresh_token_hash text not null unique,
  expires_at timestamptz not null,
  created_at timestamptz not null default now(),
  last_used_at timestamptz not null default now(),
  revoked_at timestamptz,
  user_agent text,
  ip_address text
);
create index if not exists idx_auth_sessions_user_id on auth_sessions(user_id);
create index if not exists idx_auth_sessions_expires_at on auth_sessions(expires_at);

create table if not exists ranks (
  user_id text not null references users(id),
  mode text not null,
  mmr integer not null default 1000,
  season_id text not null,
  updated_at timestamptz not null default now(),
  primary key (user_id, mode, season_id)
);

create table if not exists user_stats (
  user_id text primary key references users(id),
  games_played integer not null default 0,
  wins integer not null default 0,
  updated_at timestamptz not null default now()
);

create table if not exists ranked_stats (
  user_id text not null references users(id),
  mode text not null,
  season_id text not null,
  games_played integer not null default 0,
  wins integer not null default 0,
  updated_at timestamptz not null default now(),
  primary key (user_id, mode, season_id)
);

create table if not exists maps (
  map_key text primary key,
  display_name text not null,
  created_at timestamptz not null default now()
);

create table if not exists map_revisions (
  id text primary key,
  map_key text not null references maps(map_key) on delete cascade,
  content_hash text not null,
  status text not null default 'validated',
  row_count integer not null default 0,
  created_at timestamptz not null default now(),
  unique (map_key, content_hash)
);

create table if not exists map_aliases (
  map_key text primary key references maps(map_key) on delete cascade,
  active_revision_id text references map_revisions(id),
  rollback_revision_id text references map_revisions(id),
  updated_at timestamptz not null default now()
);

create table if not exists locations (
  id bigserial primary key,
  map_revision_id text references map_revisions(id) on delete cascade,
  lat double precision not null,
  lng double precision not null,
  country text,
  pano_id text,
  heading double precision,
  pitch double precision,
  rand_key double precision not null
);

create index if not exists idx_locations_revision_rand on locations(map_revision_id, rand_key);
create index if not exists idx_locations_revision_id on locations(map_revision_id, id);

create table if not exists runtime_matches (
  id text primary key,
  state text not null,
  owner_epoch bigint not null default 0,
  started_at timestamptz not null default now(),
  ended_at timestamptz
);

create table if not exists runtime_snapshots (
  id bigserial primary key,
  match_id text not null,
  seq bigint not null,
  snapshot_json jsonb not null,
  persisted_at timestamptz not null default now()
);
create index if not exists idx_runtime_snapshots_match_seq on runtime_snapshots(match_id, seq desc);

create index if not exists idx_ranks_mode_season_mmr_user
on ranks(mode, season_id, mmr desc, user_id);
