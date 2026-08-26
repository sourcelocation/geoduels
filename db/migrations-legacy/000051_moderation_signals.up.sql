drop table if exists player_report_reviews cascade;
drop table if exists player_reports cascade;
drop table if exists moderation_case_log cascade;
drop table if exists moderation_evidence cascade;
drop table if exists moderation_actions cascade;
drop table if exists moderation_case_events cascade;
drop table if exists moderation_reports cascade;
drop table if exists moderation_cases cascade;
drop table if exists moderation_reporter_reputation cascade;
drop table if exists moderation_signals cascade;
drop table if exists enforcement_actions cascade;

alter table ip_signup_bans
  drop column if exists source_case_id;

alter table users
  add column if not exists ban_expires_at timestamptz;

create or replace function severity_rank(value text)
returns integer
language sql
immutable
as $$
  select case value
    when 'critical' then 4
    when 'high' then 3
    when 'medium' then 2
    else 1
  end
$$;

create or replace function evidence_strength_rank(value text)
returns integer
language sql
immutable
as $$
  select case value
    when 'strong' then 4
    when 'substantial' then 3
    when 'limited' then 2
    else 1
  end
$$;

create or replace function priority_rank(value text)
returns integer
language sql
immutable
as $$
  select case value
    when 'urgent' then 4
    when 'high' then 3
    when 'medium' then 2
    else 1
  end
$$;

create or replace function greatest_severity(left_value text, right_value text)
returns text
language sql
immutable
as $$
  select case when severity_rank(right_value) > severity_rank(left_value) then right_value else left_value end
$$;

create or replace function greatest_evidence_strength(left_value text, right_value text)
returns text
language sql
immutable
as $$
  select case when evidence_strength_rank(right_value) > evidence_strength_rank(left_value) then right_value else left_value end
$$;

create or replace function greatest_priority(left_value text, right_value text)
returns text
language sql
immutable
as $$
  select case when priority_rank(right_value) > priority_rank(left_value) then right_value else left_value end
$$;

create or replace function reporter_weight_for_outcome(value text)
returns double precision
language sql
immutable
as $$
  select case value
    when 'confirmed' then 1.15
    when 'dismissed' then 0.85
    when 'abusive_report' then 0.25
    else 1.0
  end
$$;

create table moderation_signals (
  id bigserial primary key,
  subject_user_id uuid not null references users(id) on delete cascade,
  signal_type text not null,
  source text not null,
  severity text not null,
  evidence_strength text not null,
  detector_key text,
  detector_version text,
  reason_code text not null,
  score double precision not null default 0,
  recommended_queue boolean not null default false,
  reporter_user_id uuid references users(id) on delete set null,
  match_id uuid references match_history(match_id) on delete set null,
  payload_json jsonb not null default '{}'::jsonb,
  occurred_at timestamptz not null default now(),
  created_at timestamptz not null default now(),
  constraint moderation_signals_source_check check (source in ('player_report', 'risk_engine', 'moderator', 'system')),
  constraint moderation_signals_severity_check check (severity in ('low', 'medium', 'high', 'critical')),
  constraint moderation_signals_evidence_strength_check check (evidence_strength in ('weak', 'limited', 'substantial', 'strong')),
  constraint moderation_signals_no_self_report check (reporter_user_id is null or reporter_user_id <> subject_user_id)
);

create unique index idx_moderation_signals_report_dedupe
on moderation_signals(match_id, reporter_user_id, subject_user_id)
where source = 'player_report' and reporter_user_id is not null and match_id is not null;

create unique index idx_moderation_signals_detector_dedupe
on moderation_signals(subject_user_id, coalesce(match_id, '00000000-0000-0000-0000-000000000000'::uuid), coalesce(detector_key, ''), coalesce(detector_version, ''), reason_code)
where source = 'risk_engine';

create index idx_moderation_signals_subject_recent
on moderation_signals(subject_user_id, occurred_at desc);

create index idx_moderation_signals_queue
on moderation_signals(recommended_queue, severity, occurred_at desc);

create index idx_moderation_signals_match
on moderation_signals(match_id, occurred_at desc)
where match_id is not null;

create table moderation_incidents (
  id bigserial primary key,
  subject_user_id uuid not null references users(id) on delete cascade,
  status text not null default 'open',
  severity text not null default 'low',
  evidence_strength text not null default 'weak',
  reason_code text not null default 'reported_behavior',
  summary text not null default '',
  signal_count integer not null default 0,
  unique_reporter_count integer not null default 0,
  assigned_to uuid references users(id) on delete set null,
  watch_until timestamptz,
  latest_signal_at timestamptz not null default now(),
  resolved_at timestamptz,
  resolved_by uuid references users(id) on delete set null,
  resolution_note text,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  constraint moderation_incidents_status_check check (status in ('open', 'watching', 'actioned', 'dismissed', 'inconclusive', 'duplicate')),
  constraint moderation_incidents_severity_check check (severity in ('low', 'medium', 'high', 'critical')),
  constraint moderation_incidents_evidence_strength_check check (evidence_strength in ('weak', 'limited', 'substantial', 'strong'))
);

create unique index idx_moderation_incidents_open_subject
on moderation_incidents(subject_user_id)
where status in ('open', 'watching');

create index idx_moderation_incidents_queue
on moderation_incidents(status, severity, latest_signal_at desc);

create table moderation_incident_signals (
  incident_id bigint not null references moderation_incidents(id) on delete cascade,
  signal_id bigint not null references moderation_signals(id) on delete cascade,
  created_at timestamptz not null default now(),
  primary key (incident_id, signal_id)
);

create table moderation_review_tasks (
  id bigserial primary key,
  incident_id bigint not null references moderation_incidents(id) on delete cascade,
  status text not null default 'open',
  queue text not null default 'standard',
  priority text not null default 'medium',
  assigned_to uuid references users(id) on delete set null,
  claimed_at timestamptz,
  claim_expires_at timestamptz,
  completed_at timestamptz,
  created_at timestamptz not null default now(),
  updated_at timestamptz not null default now(),
  constraint moderation_review_tasks_status_check check (status in ('open', 'claimed', 'blocked', 'done', 'expired')),
  constraint moderation_review_tasks_queue_check check (queue in ('standard', 'high_risk', 'watchlist')),
  constraint moderation_review_tasks_priority_check check (priority in ('low', 'medium', 'high', 'urgent'))
);

create unique index idx_moderation_review_tasks_open_incident
on moderation_review_tasks(incident_id)
where status in ('open', 'claimed', 'blocked');

create index idx_moderation_review_tasks_queue
on moderation_review_tasks(status, priority, created_at desc);

create index idx_moderation_review_tasks_assignee
on moderation_review_tasks(assigned_to, status, updated_at desc)
where assigned_to is not null;

create table moderation_verdicts (
  id bigserial primary key,
  incident_id bigint not null references moderation_incidents(id) on delete cascade,
  task_id bigint references moderation_review_tasks(id) on delete set null,
  actor_user_id uuid references users(id) on delete set null,
  verdict text not null,
  reason_code text not null,
  note text,
  enforcement_action text,
  metadata jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now(),
  constraint moderation_verdicts_verdict_check check (verdict in ('confirmed', 'dismissed', 'inconclusive', 'watch', 'duplicate', 'abusive_report')),
  constraint moderation_verdicts_enforcement_action_check check (enforcement_action is null or enforcement_action in ('warning', 'temporary_ban', 'permanent_ban', 'chat_mute', 'report_mute', 'unban', 'refund'))
);

create index idx_moderation_verdicts_incident
on moderation_verdicts(incident_id, created_at desc);

create table moderation_audit_log (
  id bigserial primary key,
  incident_id bigint references moderation_incidents(id) on delete cascade,
  task_id bigint references moderation_review_tasks(id) on delete set null,
  actor_user_id uuid references users(id) on delete set null,
  event_type text not null,
  reason_code text,
  body text,
  metadata jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now()
);

create index idx_moderation_audit_log_incident
on moderation_audit_log(incident_id, created_at desc);

create table moderation_reporter_state (
  user_id uuid primary key references users(id) on delete cascade,
  reports_submitted integer not null default 0,
  reports_useful integer not null default 0,
  reports_dismissed integer not null default 0,
  reports_inconclusive integer not null default 0,
  reports_abusive integer not null default 0,
  report_weight double precision not null default 1,
  muted_until timestamptz,
  updated_at timestamptz not null default now()
);

create index idx_moderation_reporter_state_muted
on moderation_reporter_state(muted_until)
where muted_until is not null;

create table enforcement_actions (
  id bigserial primary key,
  target_user_id uuid references users(id) on delete set null,
  actor_user_id uuid references users(id) on delete set null,
  source_incident_id bigint references moderation_incidents(id) on delete set null,
  source_verdict_id bigint references moderation_verdicts(id) on delete set null,
  action_type text not null,
  reason_code text not null,
  reason_note text,
  starts_at timestamptz not null default now(),
  ends_at timestamptz,
  revoked_by uuid references users(id) on delete set null,
  revoked_at timestamptz,
  metadata jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now(),
  constraint enforcement_actions_type_check check (action_type in ('warning', 'temporary_ban', 'permanent_ban', 'chat_mute', 'report_mute', 'unban', 'refund'))
);

create index idx_enforcement_target_active
on enforcement_actions(target_user_id, action_type, starts_at desc)
where revoked_at is null;

create index idx_enforcement_incident
on enforcement_actions(source_incident_id);

alter table ip_signup_bans
  add column if not exists source_incident_id bigint references moderation_incidents(id) on delete set null;
