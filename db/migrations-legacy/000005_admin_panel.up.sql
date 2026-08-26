alter table users
  add column if not exists is_admin boolean not null default false,
  add column if not exists banned_at timestamptz,
  add column if not exists ban_reason text;

create table if not exists site_settings (
  key text primary key,
  value_json jsonb not null,
  updated_at timestamptz not null default now()
);
