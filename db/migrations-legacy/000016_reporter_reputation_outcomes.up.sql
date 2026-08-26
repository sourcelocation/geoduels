alter table moderation_reporter_reputation
  add column if not exists reports_inconclusive integer not null default 0,
  add column if not exists reports_abusive integer not null default 0;

alter table moderation_actions
  drop constraint if exists moderation_actions_type_check;

alter table moderation_actions
  add constraint moderation_actions_type_check
  check (action_type in ('note', 'assign', 'status', 'warn', 'ban', 'report_mute', 'dismiss', 'mark_inconclusive', 'abusive_reports', 'refund'));
