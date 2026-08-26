alter table users
  add column if not exists chat_muted_at timestamptz,
  add column if not exists chat_mute_reason text,
  add column if not exists chat_mute_expires_at timestamptz,
  add column if not exists report_muted_at timestamptz,
  add column if not exists report_mute_reason text,
  add column if not exists report_mute_expires_at timestamptz;

alter table moderation_signals
  add column if not exists reviewed_at timestamptz,
  add column if not exists reviewed_by uuid references users(id) on delete set null,
  add column if not exists outcome text;

alter table moderation_signals
  add constraint moderation_signals_outcome_check
  check (outcome is null or outcome in ('confirmed', 'dismissed', 'inconclusive', 'abusive_report'));

create index idx_moderation_signals_unreviewed
on moderation_signals(recommended_queue, severity, occurred_at desc)
where reviewed_at is null;

create table moderation_log (
  id bigserial primary key,
  subject_user_id uuid references users(id) on delete set null,
  actor_user_id uuid references users(id) on delete set null,
  action text not null,
  reason text,
  expires_at timestamptz,
  signal_ids bigint[] not null default '{}',
  metadata jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now()
);

create index idx_moderation_log_subject_created
on moderation_log(subject_user_id, created_at desc, id desc);

create index idx_moderation_log_actor_created
on moderation_log(actor_user_id, created_at desc, id desc)
where actor_user_id is not null;

-- Preserve the useful history before removing the workflow projections.
insert into moderation_log(subject_user_id, actor_user_id, action, reason, expires_at, metadata, created_at)
select
  i.subject_user_id,
  a.actor_user_id,
  a.event_type,
  coalesce(a.body, a.reason_code),
  null,
  a.metadata || jsonb_build_object('legacyIncidentId', a.incident_id, 'legacyTaskId', a.task_id),
  a.created_at
from moderation_audit_log a
left join moderation_incidents i on i.id = a.incident_id;

insert into moderation_log(subject_user_id, actor_user_id, action, reason, expires_at, metadata, created_at)
select
  e.target_user_id,
  e.actor_user_id,
  e.action_type,
  coalesce(e.reason_note, e.reason_code),
  e.ends_at,
  e.metadata || jsonb_build_object('legacyEnforcementActionId', e.id),
  e.created_at
from enforcement_actions e;

insert into moderation_log(subject_user_id, actor_user_id, action, reason, metadata, created_at)
select user_id, granted_by, 'role_granted', reason, jsonb_build_object('role', role), granted_at
from user_roles;

insert into moderation_log(subject_user_id, actor_user_id, action, reason, metadata, created_at)
select user_id, revoked_by, 'role_revoked', reason, jsonb_build_object('role', role), revoked_at
from user_roles
where revoked_at is not null;

update users u
set
  chat_muted_at = active.starts_at,
  chat_mute_reason = coalesce(active.reason_note, active.reason_code),
  chat_mute_expires_at = active.ends_at
from (
  select distinct on (target_user_id)
    target_user_id, starts_at, reason_note, reason_code, ends_at
  from enforcement_actions
  where action_type = 'chat_mute'
    and revoked_at is null
    and starts_at <= now()
    and (ends_at is null or ends_at > now())
  order by target_user_id, starts_at desc, id desc
) active
where u.id = active.target_user_id;

update users u
set
  report_muted_at = coalesce(r.updated_at, now()),
  report_mute_reason = 'reporting privileges restricted',
  report_mute_expires_at = r.muted_until
from moderation_reporter_state r
where u.id = r.user_id and r.muted_until > now();

alter table ip_signup_bans drop column if exists source_incident_id;

drop table moderation_audit_log;
drop table enforcement_actions;
drop table moderation_verdicts;
drop table moderation_review_tasks;
drop table moderation_incident_signals;
drop table moderation_reporter_state;
drop table moderation_incidents;
drop table user_roles;

drop function if exists greatest_severity(text, text);
drop function if exists greatest_evidence_strength(text, text);
drop function if exists greatest_priority(text, text);
drop function if exists reporter_weight_for_outcome(text);
drop function if exists severity_rank(text);
drop function if exists evidence_strength_rank(text);
drop function if exists priority_rank(text);
