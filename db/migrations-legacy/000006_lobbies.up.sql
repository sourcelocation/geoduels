create table if not exists lobbies (
  id text primary key,
  invite_code text not null unique,
  owner_user_id text not null references users(id) on delete cascade,
  state text not null default 'open',
  mode text not null default 'duel',
  map_scope text not null default 'world',
  started_match_id text,
  created_at timestamptz not null default now(),
  expires_at timestamptz not null,
  updated_at timestamptz not null default now(),
  constraint lobbies_state_check check (state in ('open', 'started', 'closed', 'expired')),
  constraint lobbies_mode_check check (mode in ('duel'))
);

create index if not exists idx_lobbies_owner_user_id on lobbies(owner_user_id);
create index if not exists idx_lobbies_expires_at on lobbies(expires_at);

create table if not exists lobby_members (
  lobby_id text not null references lobbies(id) on delete cascade,
  user_id text not null references users(id) on delete cascade,
  role text not null default 'member',
  ready boolean not null default false,
  joined_at timestamptz not null default now(),
  left_at timestamptz,
  primary key (lobby_id, user_id),
  constraint lobby_members_role_check check (role in ('owner', 'member'))
);

create index if not exists idx_lobby_members_user_id on lobby_members(user_id);
