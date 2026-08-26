create table if not exists user_preferences (
  user_id uuid primary key references users(id) on delete cascade,
  schema_version integer not null default 1,
  preferences_json jsonb not null default '{}'::jsonb,
  revision bigint not null default 0,
  updated_at timestamptz not null default now(),
  constraint user_preferences_schema_version_check check (schema_version > 0),
  constraint user_preferences_revision_check check (revision >= 0)
);

