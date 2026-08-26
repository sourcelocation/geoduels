-- Lean admin/moderation rewrite foundation.
-- Keep migrations additive where existing runtime code may still read legacy columns,
-- but remove durable runtime snapshots: gameplay nodes are the live authority.

drop table if exists runtime_snapshots;

create table if not exists user_roles (
  id bigserial primary key,
  user_id text not null references users(id) on delete cascade,
  role text not null,
  granted_by text references users(id) on delete set null,
  granted_at timestamptz not null default now(),
  revoked_by text references users(id) on delete set null,
  revoked_at timestamptz,
  reason text,
  constraint user_roles_role_check check (role in ('admin', 'moderator'))
);

create unique index if not exists idx_user_roles_active
on user_roles(user_id, role)
where revoked_at is null;

insert into user_roles(user_id, role, granted_at, reason)
select id, 'admin', now(), 'backfilled from users.is_admin'
from users
where coalesce(is_admin, false) = true
on conflict do nothing;

insert into user_roles(user_id, role, granted_at, reason)
select id, 'moderator', now(), 'backfilled from users.is_moderator'
from users
where coalesce(is_moderator, false) = true
on conflict do nothing;

alter table moderation_cases
  add column if not exists source text not null default 'report',
  add column if not exists queue text not null default 'active',
  add column if not exists risk_score double precision not null default 0,
  add column if not exists risk_breakdown jsonb not null default '{}'::jsonb,
  add column if not exists confidence double precision not null default 0,
  add column if not exists claimed_at timestamptz,
  add column if not exists claim_expires_at timestamptz,
  add column if not exists escalated_at timestamptz,
  add column if not exists resolution_code text,
  add column if not exists resolution_note text;

update moderation_cases
set
  queue = case when status in ('actioned', 'dismissed', 'duplicate') then 'archive' else 'active' end,
  risk_score = case when risk_score = 0 then score else risk_score end,
  risk_breakdown = case
    when risk_breakdown = '{}'::jsonb then jsonb_build_object('legacyScore', score)
    else risk_breakdown
  end,
  resolution_code = coalesce(resolution_code, nullif(status, ''))
where true;

alter table moderation_cases
  drop constraint if exists moderation_cases_queue_check;

alter table moderation_cases
  alter column queue set default 'intake';

alter table moderation_cases
  add constraint moderation_cases_queue_check check (queue in ('intake', 'active', 'archive'));

update moderation_cases
set queue = case
  when status in ('actioned', 'dismissed', 'duplicate') then 'archive'
  when source = 'auto_detection'
    or assigned_to is not null
    or status in ('reviewing', 'watching')
    or escalated_at is not null
    or risk_score >= 1.5 then 'active'
  else 'intake'
end;

create index if not exists idx_moderation_cases_active_priority
on moderation_cases(priority, latest_activity_at desc)
where queue = 'active' and resolved_at is null;

create index if not exists idx_moderation_cases_assigned_active
on moderation_cases(assigned_to, latest_activity_at desc)
where queue = 'active' and resolved_at is null;

create index if not exists idx_moderation_cases_unclaimed
on moderation_cases(priority, latest_activity_at desc)
where queue = 'active' and assigned_to is null and resolved_at is null;

create index if not exists idx_moderation_cases_archive
on moderation_cases(resolved_at desc, resolution_code)
where queue = 'archive';

create table if not exists moderation_case_log (
  id bigserial primary key,
  case_id bigint not null references moderation_cases(id) on delete cascade,
  actor_user_id text references users(id) on delete set null,
  event_type text not null,
  reason_code text,
  body text,
  metadata jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now()
);

create index if not exists idx_moderation_case_log_case_created
on moderation_case_log(case_id, created_at desc);

create index if not exists idx_moderation_case_log_actor_created
on moderation_case_log(actor_user_id, created_at desc);

insert into moderation_case_log(case_id, actor_user_id, event_type, body, metadata, created_at)
select case_id, actor_user_id, event_type, body, metadata, created_at
from moderation_case_events
on conflict do nothing;

insert into moderation_case_log(case_id, actor_user_id, event_type, body, metadata, created_at)
select case_id, actor_user_id, 'action_' || action_type, reason, metadata, created_at
from moderation_actions
on conflict do nothing;

create table if not exists moderation_evidence (
  id bigserial primary key,
  case_id bigint not null references moderation_cases(id) on delete cascade,
  evidence_type text not null,
  match_id text references match_history(match_id) on delete set null,
  round_id text,
  subject_user_id text references users(id) on delete set null,
  detector_version text,
  rule_id text,
  score double precision not null default 0,
  weight double precision not null default 0,
  occurred_at timestamptz,
  payload_json jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now()
);

create index if not exists idx_moderation_evidence_case
on moderation_evidence(case_id, score desc, created_at desc);

create index if not exists idx_moderation_evidence_subject_recent
on moderation_evidence(subject_user_id, occurred_at desc);

create index if not exists idx_moderation_evidence_detector
on moderation_evidence(detector_version, rule_id, created_at desc);

create table if not exists enforcement_actions (
  id bigserial primary key,
  target_user_id text references users(id) on delete set null,
  actor_user_id text references users(id) on delete set null,
  source_case_id bigint references moderation_cases(id) on delete set null,
  action_type text not null,
  reason_code text not null,
  reason_note text,
  starts_at timestamptz not null default now(),
  expires_at timestamptz,
  revoked_by text references users(id) on delete set null,
  revoked_at timestamptz,
  metadata jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now(),
  constraint enforcement_actions_type_check check (action_type in ('ban', 'unban', 'report_mute', 'ip_block', 'oauth_block', 'warning', 'refund'))
);

create index if not exists idx_enforcement_target_active
on enforcement_actions(target_user_id, action_type, starts_at desc)
where revoked_at is null;

create index if not exists idx_enforcement_case
on enforcement_actions(source_case_id);

insert into enforcement_actions(target_user_id, action_type, reason_code, reason_note, starts_at, created_at)
select id, 'ban', 'legacy_ban', ban_reason, coalesce(banned_at, now()), coalesce(banned_at, now())
from users
where banned_at is not null
on conflict do nothing;

alter table match_history
  add column if not exists ranked boolean not null default false,
  add column if not exists source_kind text not null default 'queue',
  add column if not exists source_lobby_id text,
  add column if not exists map_key text,
  add column if not exists map_revision_id text,
  add column if not exists ruleset text,
  add column if not exists replay_json jsonb;

alter table match_history
  alter column snapshot_json drop not null;

update match_history
set
  ranked = not coalesce((snapshot_json->>'unranked')::boolean, false),
  ruleset = coalesce(nullif(snapshot_json->'config'->>'ruleset', ''), ruleset),
  replay_json = coalesce(replay_json, snapshot_json)
where snapshot_json is not null;

update match_history h
set
  source_kind = 'lobby',
  source_lobby_id = l.id
from lobbies l
where h.match_id in (l.active_match_id, l.started_match_id, l.last_match_id);

create index if not exists idx_match_history_ended
on match_history(ended_at desc);

create index if not exists idx_match_history_source
on match_history(source_kind, source_lobby_id);

create index if not exists idx_match_history_ranked_ruleset
on match_history(ranked, ruleset, ended_at desc);

alter table match_players
  add column if not exists final_ranked_delta integer,
  add column if not exists rating_before integer,
  add column if not exists rating_after integer,
  add column if not exists team_id text,
  add column if not exists placement integer;

alter table match_round_guesses
  add column if not exists ruleset text,
  add column if not exists ranked boolean not null default false,
  add column if not exists source_kind text,
  add column if not exists map_key text,
  add column if not exists map_revision_id text,
  add column if not exists guessed_at timestamptz,
  add column if not exists evidence_excluded boolean not null default false;

update match_round_guesses g
set
  ruleset = coalesce(g.ruleset, h.ruleset),
  ranked = h.ranked,
  source_kind = coalesce(g.source_kind, h.source_kind),
  map_key = coalesce(g.map_key, h.map_key),
  map_revision_id = coalesce(g.map_revision_id, h.map_revision_id),
  guessed_at = coalesce(g.guessed_at, case when g.guess_unix_ms is not null then to_timestamp(g.guess_unix_ms::double precision / 1000.0) else h.ended_at end)
from match_history h
where h.match_id = g.match_id;

create index if not exists idx_match_round_guesses_user_recent
on match_round_guesses(user_id, guessed_at desc);

create index if not exists idx_match_round_guesses_match_round
on match_round_guesses(match_id, round_number);

create index if not exists idx_match_round_guesses_high_score_fast
on match_round_guesses(user_id, guessed_at desc)
where ranked = true and score >= 4900 and guess_ms <= 15000;

alter table auth_sessions
  add column if not exists ip_redacted_at timestamptz,
  add column if not exists user_agent_redacted_at timestamptz;

alter table chat_messages
  add column if not exists retained_until timestamptz,
  add column if not exists redacted_at timestamptz,
  add column if not exists redacted_by text references users(id) on delete set null;

alter table ip_signup_bans
  add column if not exists cidr inet,
  add column if not exists reason_code text,
  add column if not exists expires_at timestamptz,
  add column if not exists source_case_id bigint references moderation_cases(id) on delete set null;

create index if not exists idx_notification_outbox_sent
on notification_outbox(sent_at)
where sent_at is not null;

create index if not exists idx_user_notifications_read
on user_notifications(read_at, created_at)
where read_at is not null;

alter table map_revisions
  add column if not exists pinned boolean not null default false,
  add column if not exists activated_at timestamptz,
  add column if not exists retired_at timestamptz;

alter table lobbies
  add column if not exists archived_at timestamptz;

create index if not exists idx_lobbies_active
on lobbies(updated_at desc)
where state in ('open', 'in_match');

create index if not exists idx_lobbies_archive_cleanup
on lobbies(updated_at)
where state in ('closed', 'expired') and archived_at is null;
