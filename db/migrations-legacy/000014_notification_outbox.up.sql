create table if not exists notification_outbox (
  id bigserial primary key,
  type text not null,
  dedupe_key text not null unique,
  payload_json jsonb not null,
  attempts integer not null default 0,
  next_attempt_at timestamptz not null default now(),
  sent_at timestamptz,
  last_error text,
  created_at timestamptz not null default now()
);

create index if not exists idx_notification_outbox_pending
on notification_outbox(next_attempt_at, id)
where sent_at is null;
