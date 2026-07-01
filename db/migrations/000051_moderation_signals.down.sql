alter table ip_signup_bans
  drop column if exists source_incident_id;

alter table users
  drop column if exists ban_expires_at;

drop table if exists enforcement_actions cascade;
drop table if exists moderation_reporter_state cascade;
drop table if exists moderation_audit_log cascade;
drop table if exists moderation_verdicts cascade;
drop table if exists moderation_review_tasks cascade;
drop table if exists moderation_incident_signals cascade;
drop table if exists moderation_incidents cascade;
drop table if exists moderation_signals cascade;
drop function if exists reporter_weight_for_outcome(text);
drop function if exists greatest_priority(text, text);
drop function if exists greatest_evidence_strength(text, text);
drop function if exists greatest_severity(text, text);
drop function if exists priority_rank(text);
drop function if exists evidence_strength_rank(text);
drop function if exists severity_rank(text);
