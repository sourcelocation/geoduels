create table if not exists discord_sync_outbox (
  id bigserial primary key,
  action text not null,
  discord_user_id text not null,
  attempts integer not null default 0,
  next_attempt_at timestamptz not null default now(),
  processed_at timestamptz,
  last_error text,
  created_at timestamptz not null default now(),
  constraint discord_sync_outbox_action_check check (action in ('sync', 'cleanup_roles'))
);

create unique index if not exists idx_discord_sync_outbox_pending_unique
on discord_sync_outbox(action, discord_user_id)
where processed_at is null;

create index if not exists idx_discord_sync_outbox_pending
on discord_sync_outbox(next_attempt_at, id)
where processed_at is null;
