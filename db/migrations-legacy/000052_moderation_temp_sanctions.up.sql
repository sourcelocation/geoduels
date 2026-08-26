alter table users
  add column if not exists ban_expires_at timestamptz;

alter table moderation_verdicts
  drop constraint if exists moderation_verdicts_verdict_check,
  drop constraint if exists moderation_verdicts_enforcement_action_check;

alter table moderation_verdicts
  add constraint moderation_verdicts_verdict_check
    check (verdict in ('confirmed', 'dismissed', 'inconclusive', 'watch', 'duplicate', 'abusive_report')),
  add constraint moderation_verdicts_enforcement_action_check
    check (enforcement_action is null or enforcement_action in ('warning', 'temporary_ban', 'permanent_ban', 'chat_mute', 'report_mute', 'unban', 'refund'));

alter table enforcement_actions
  drop constraint if exists enforcement_actions_type_check;

alter table enforcement_actions
  add constraint enforcement_actions_type_check
    check (action_type in ('warning', 'temporary_ban', 'permanent_ban', 'chat_mute', 'report_mute', 'unban', 'refund'));
