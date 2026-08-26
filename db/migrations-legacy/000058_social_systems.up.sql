alter table users
  add column if not exists last_seen_at timestamptz,
  add column if not exists social_discoverable boolean not null default true,
  add column if not exists social_presence_visible boolean not null default true,
  add column if not exists social_requests_enabled boolean not null default true,
  add column if not exists social_party_invites_enabled boolean not null default true;

create table if not exists friend_requests (
  id uuid primary key,
  sender_user_id uuid not null references users(id) on delete cascade,
  recipient_user_id uuid not null references users(id) on delete cascade,
  status text not null default 'pending',
  created_at timestamptz not null default now(),
  responded_at timestamptz,
  expires_at timestamptz not null,
  constraint friend_requests_distinct_users check (sender_user_id <> recipient_user_id),
  constraint friend_requests_status_check check (status in ('pending', 'accepted', 'declined', 'cancelled', 'expired'))
);

create unique index if not exists idx_friend_requests_active_pair
on friend_requests(least(sender_user_id, recipient_user_id), greatest(sender_user_id, recipient_user_id))
where status = 'pending';

create index if not exists idx_friend_requests_recipient_pending
on friend_requests(recipient_user_id, created_at desc)
where status = 'pending';

create index if not exists idx_friend_requests_sender_pending
on friend_requests(sender_user_id, created_at desc)
where status = 'pending';

create table if not exists friendships (
  user_id_low uuid not null references users(id) on delete cascade,
  user_id_high uuid not null references users(id) on delete cascade,
  created_from_request_id uuid references friend_requests(id) on delete set null,
  created_at timestamptz not null default now(),
  primary key (user_id_low, user_id_high),
  constraint friendships_canonical_pair check (user_id_low < user_id_high)
);

create index if not exists idx_friendships_high on friendships(user_id_high, created_at desc);

create table if not exists user_blocks (
  blocker_user_id uuid not null references users(id) on delete cascade,
  blocked_user_id uuid not null references users(id) on delete cascade,
  created_at timestamptz not null default now(),
  primary key (blocker_user_id, blocked_user_id),
  constraint user_blocks_distinct_users check (blocker_user_id <> blocked_user_id)
);

create index if not exists idx_user_blocks_target on user_blocks(blocked_user_id);

create table if not exists friend_codes (
  code char(6) primary key,
  user_id uuid not null references users(id) on delete cascade,
  created_at timestamptz not null default now(),
  expires_at timestamptz not null,
  revoked_at timestamptz
);

create unique index if not exists idx_friend_codes_active_user
on friend_codes(user_id)
where revoked_at is null;

create table if not exists party_invitations (
  id uuid primary key,
  party_id uuid not null references parties(id) on delete cascade,
  inviter_user_id uuid not null references users(id) on delete cascade,
  recipient_user_id uuid not null references users(id) on delete cascade,
  status text not null default 'pending',
  created_at timestamptz not null default now(),
  expires_at timestamptz not null,
  responded_at timestamptz,
  constraint party_invitations_distinct_users check (inviter_user_id <> recipient_user_id),
  constraint party_invitations_status_check check (status in ('pending', 'accepted', 'declined', 'cancelled', 'expired'))
);

create unique index if not exists idx_party_invitations_active
on party_invitations(party_id, recipient_user_id)
where status = 'pending';

create index if not exists idx_party_invitations_recipient
on party_invitations(recipient_user_id, created_at desc)
where status = 'pending';

alter table user_notifications
  add column if not exists category text not null default 'system',
  add column if not exists actor_user_id uuid references users(id) on delete set null,
  add column if not exists entity_kind text,
  add column if not exists entity_id text,
  add column if not exists archived_at timestamptz,
  add column if not exists expires_at timestamptz;

create index if not exists idx_user_notifications_inbox
on user_notifications(user_id, created_at desc, id desc)
where archived_at is null;

create table if not exists user_event_sequences (
  user_id uuid primary key references users(id) on delete cascade,
  sequence bigint not null default 0
);

create table if not exists user_events (
  user_id uuid not null references users(id) on delete cascade,
  sequence bigint not null,
  type text not null,
  payload_json jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now(),
  primary key (user_id, sequence)
);

create index if not exists idx_user_events_created_at on user_events(created_at);
