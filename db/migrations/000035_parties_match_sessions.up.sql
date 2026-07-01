create table if not exists parties (
  id text primary key,
  invite_code text not null unique,
  owner_user_id text not null references users(id) on delete cascade,
  state text not null default 'open',
  mode text not null default 'duel',
  map_scope text not null default 'world',
  active_match_id text,
  started_match_id text,
  last_match_id text,
  config_json jsonb not null default '{"ruleset":"moving","roundTimerMode":"none","pressureTimeLimitMs":15000}'::jsonb,
  map_id text references maps(map_key),
  created_at timestamptz not null default now(),
  expires_at timestamptz not null,
  updated_at timestamptz not null default now(),
  constraint parties_state_check check (state in ('open', 'in_match', 'started', 'closed', 'expired')),
  constraint parties_mode_check check (mode in ('duel', 'team_duel', 'free_for_all'))
);

create index if not exists idx_parties_owner_user_id on parties(owner_user_id);
create index if not exists idx_parties_expires_at on parties(expires_at);
create index if not exists idx_parties_active_match on parties(active_match_id);
create index if not exists idx_parties_started_match on parties(started_match_id);
create index if not exists idx_parties_last_match on parties(last_match_id);

create table if not exists party_members (
  party_id text not null references parties(id) on delete cascade,
  user_id text not null references users(id) on delete cascade,
  role text not null default 'member',
  ready boolean not null default false,
  team_id text,
  joined_at timestamptz not null default now(),
  left_at timestamptz,
  primary key (party_id, user_id),
  constraint party_members_role_check check (role in ('owner', 'member')),
  constraint party_members_team_check check (team_id is null or team_id in ('a', 'b'))
);

create index if not exists idx_party_members_user_id on party_members(user_id);
create index if not exists idx_party_members_active on party_members(party_id, joined_at) where left_at is null;

create table if not exists match_sessions (
  match_id text primary key,
  preset_id text not null,
  mode text not null,
  state text not null default 'live',
  ranked boolean not null default false,
  source_kind text not null default 'queue',
  source_party_id text references parties(id) on delete set null,
  source_party_invite_code text,
  node_id text,
  node_epoch bigint,
  public_route text,
  config_json jsonb not null default '{}'::jsonb,
  map_id text,
  map_revision_id text,
  created_at timestamptz not null default now(),
  started_at timestamptz not null default now(),
  ended_at timestamptz,
  updated_at timestamptz not null default now(),
  constraint match_sessions_state_check check (state in ('waiting', 'live', 'ended'))
);

create index if not exists idx_match_sessions_source_party on match_sessions(source_party_id, started_at desc);
create index if not exists idx_match_sessions_state on match_sessions(state, updated_at desc);

create table if not exists match_participants (
  match_id text not null references match_sessions(match_id) on delete cascade,
  user_id text not null references users(id) on delete cascade,
  team_id text,
  display_name text not null default '',
  avatar_url text not null default '',
  joined_party_at timestamptz,
  primary key (match_id, user_id)
);

create index if not exists idx_match_participants_user on match_participants(user_id, match_id);

alter table match_history
  add column if not exists source_party_id text;

update match_history
set source_party_id = source_lobby_id
where source_party_id is null
  and source_lobby_id is not null;

alter table match_history
  drop column if exists source_lobby_id;

drop table if exists lobby_members;
drop table if exists lobbies;
