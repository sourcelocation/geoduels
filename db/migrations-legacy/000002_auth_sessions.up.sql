drop index if exists idx_auth_refresh_tokens_user_id;
drop index if exists idx_auth_refresh_tokens_expires_at;
drop table if exists auth_refresh_tokens;

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
