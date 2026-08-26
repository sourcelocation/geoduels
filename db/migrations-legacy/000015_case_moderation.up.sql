drop table if exists player_report_reviews cascade;
drop table if exists player_reports cascade;

create table if not exists moderation_cases (
  id bigserial primary key,
  target_user_id text not null references users(id) on delete cascade,
  target_display_name text not null,
  status text not null default 'new',
  priority text not null default 'low',
  score double precision not null default 0,
  report_count integer not null default 0,
  unique_reporter_count integer not null default 0,
  categories jsonb not null default '{}'::jsonb,
  summary text,
  assigned_to text references users(id) on delete set null,
  notification_sent_at timestamptz,
  latest_activity_at timestamptz not null default now(),
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  resolved_at timestamptz,
  resolved_by text references users(id) on delete set null,
  resolution text,
  constraint moderation_cases_status_check check (status in ('new', 'triaged', 'reviewing', 'watching', 'actioned', 'dismissed', 'duplicate')),
  constraint moderation_cases_priority_check check (priority in ('low', 'medium', 'high', 'urgent'))
);

create unique index if not exists idx_moderation_cases_open_target
on moderation_cases(target_user_id)
where status in ('new', 'triaged', 'reviewing', 'watching');

create index if not exists idx_moderation_cases_queue
on moderation_cases(status, priority, latest_activity_at desc);

create table if not exists moderation_reports (
  id bigserial primary key,
  case_id bigint not null references moderation_cases(id) on delete cascade,
  match_id text not null references match_history(match_id) on delete cascade,
  reporter_user_id text not null references users(id) on delete cascade,
  reported_user_id text not null references users(id) on delete cascade,
  category text not null,
  reason text,
  reporter_weight double precision not null default 1,
  created_at timestamptz not null default now(),
  constraint moderation_reports_no_self_report check (reporter_user_id <> reported_user_id),
  constraint moderation_reports_category_check check (category in ('cheating', 'profile', 'harassment', 'boosting', 'other')),
  unique (match_id, reporter_user_id, reported_user_id)
);

create index if not exists idx_moderation_reports_case_created
on moderation_reports(case_id, created_at desc);

create index if not exists idx_moderation_reports_reporter_created
on moderation_reports(reporter_user_id, created_at desc);

create table if not exists moderation_case_events (
  id bigserial primary key,
  case_id bigint not null references moderation_cases(id) on delete cascade,
  actor_user_id text references users(id) on delete set null,
  event_type text not null,
  body text,
  metadata jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now()
);

create index if not exists idx_moderation_case_events_case_created
on moderation_case_events(case_id, created_at desc);

create table if not exists moderation_actions (
  id bigserial primary key,
  case_id bigint not null references moderation_cases(id) on delete cascade,
  actor_user_id text references users(id) on delete set null,
  target_user_id text not null references users(id) on delete cascade,
  action_type text not null,
  reason text,
  metadata jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now(),
  constraint moderation_actions_type_check check (action_type in ('note', 'assign', 'status', 'warn', 'ban', 'report_mute', 'dismiss', 'mark_inconclusive', 'abusive_reports', 'refund'))
);

create table if not exists moderation_reporter_reputation (
  user_id text primary key references users(id) on delete cascade,
  reports_submitted integer not null default 0,
  reports_confirmed integer not null default 0,
  reports_dismissed integer not null default 0,
  reports_inconclusive integer not null default 0,
  reports_abusive integer not null default 0,
  report_weight double precision not null default 1,
  muted_until timestamptz,
  updated_at timestamptz not null default now()
);

create index if not exists idx_moderation_reporter_reputation_muted
on moderation_reporter_reputation(muted_until)
where muted_until is not null;
