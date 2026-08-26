alter table users
  add column if not exists deleted_at timestamptz;

create table if not exists user_identity_history (
  user_id text not null references users(id) on delete cascade,
  provider text not null,
  provider_user_id text not null,
  email text,
  provider_name text,
  avatar_url text,
  first_seen_at timestamptz not null default now(),
  last_seen_at timestamptz not null default now(),
  deleted_at timestamptz,
  primary key (user_id, provider, provider_user_id)
);

create index if not exists idx_user_identity_history_provider_user_id
on user_identity_history(provider, provider_user_id);

create index if not exists idx_user_identity_history_email_lower
on user_identity_history(lower(email))
where email is not null;

create table if not exists oauth_identity_bans (
  provider text not null,
  provider_user_id text not null,
  banned_user_id text references users(id) on delete set null,
  reason text,
  created_by text references users(id) on delete set null,
  created_at timestamptz not null default now(),
  revoked_at timestamptz,
  primary key (provider, provider_user_id)
);

create index if not exists idx_oauth_identity_bans_active
on oauth_identity_bans(provider, provider_user_id)
where revoked_at is null;

insert into user_identity_history(
  user_id,
  provider,
  provider_user_id,
  email,
  provider_name,
  avatar_url,
  first_seen_at,
  last_seen_at,
  deleted_at
)
select
  user_id,
  provider,
  provider_user_id,
  email,
  provider_name,
  avatar_url,
  created_at,
  last_seen_at,
  null
from user_identities
on conflict (user_id, provider, provider_user_id) do update set
  email = excluded.email,
  provider_name = excluded.provider_name,
  avatar_url = excluded.avatar_url,
  last_seen_at = greatest(user_identity_history.last_seen_at, excluded.last_seen_at),
  deleted_at = null;

insert into oauth_identity_bans(provider, provider_user_id, banned_user_id, reason, created_at, revoked_at)
select h.provider, h.provider_user_id, h.user_id, u.ban_reason, coalesce(u.banned_at, now()), null
from user_identity_history h
join users u on u.id = h.user_id
where u.banned_at is not null
on conflict (provider, provider_user_id) do update set
  banned_user_id = excluded.banned_user_id,
  reason = excluded.reason,
  created_at = excluded.created_at,
  revoked_at = null;
