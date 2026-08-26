-- Replace closed-vocabulary text columns with PostgreSQL enums.  The labels
-- intentionally remain the public/API values; this is a storage and database
-- validation change, not an API wire-format change.

do $$
begin
  create type gd_account_type as enum ('guest', 'registered');
exception when duplicate_object then null;
end $$;
do $$
begin
  create type gd_oauth_provider as enum ('google', 'discord');
exception when duplicate_object then null;
end $$;
do $$
begin
  create type gd_match_mode as enum ('duel', 'singleplayer', 'team_duel', 'free_for_all');
exception when duplicate_object then null;
end $$;
do $$
begin
  create type gd_ruleset as enum ('moving', 'no_move', 'nmpz');
exception when duplicate_object then null;
end $$;
do $$
begin
  create type gd_map_status as enum ('processing', 'ready', 'rejected', 'archived');
exception when duplicate_object then null;
end $$;
do $$
begin
  create type gd_map_visibility as enum ('private', 'unlisted', 'public');
exception when duplicate_object then null;
end $$;
do $$
begin
  create type gd_map_difficulty as enum ('easy', 'normal', 'hard');
exception when duplicate_object then null;
end $$;
do $$
begin
  create type gd_runtime_state as enum ('live', 'ended');
exception when duplicate_object then null;
end $$;
do $$
begin
  create type gd_party_state as enum ('open', 'in_match', 'started', 'closed', 'expired');
exception when duplicate_object then null;
end $$;
do $$
begin
  create type gd_party_role as enum ('owner', 'member');
exception when duplicate_object then null;
end $$;
do $$
begin
  create type gd_team_id as enum ('a', 'b');
exception when duplicate_object then null;
end $$;
do $$
begin
  create type gd_match_session_state as enum ('waiting', 'live', 'ended');
exception when duplicate_object then null;
end $$;
do $$
begin
  create type gd_match_source as enum ('queue', 'party', 'solo');
exception when duplicate_object then null;
end $$;
do $$
begin
  create type gd_match_preset as enum ('ranked_duel', 'private_duel', 'team_duel', 'free_for_all', 'solo');
exception when duplicate_object then null;
end $$;
do $$
begin
  create type gd_chat_scope as enum ('party', 'match');
exception when duplicate_object then null;
end $$;
do $$
begin
  create type gd_chat_kind as enum ('text', 'emote');
exception when duplicate_object then null;
end $$;
do $$
begin
  create type gd_chat_audience as enum ('all', 'team');
exception when duplicate_object then null;
end $$;
do $$
begin
  create type gd_social_request_status as enum ('pending', 'accepted', 'declined', 'cancelled', 'expired');
exception when duplicate_object then null;
end $$;
do $$
begin
  create type gd_moderation_source as enum ('player_report', 'risk_engine', 'moderator', 'system');
exception when duplicate_object then null;
end $$;
do $$
begin
  create type gd_moderation_severity as enum ('low', 'medium', 'high', 'critical');
exception when duplicate_object then null;
end $$;
do $$
begin
  create type gd_evidence_strength as enum ('weak', 'limited', 'substantial', 'strong');
exception when duplicate_object then null;
end $$;
do $$
begin
  create type gd_moderation_outcome as enum ('confirmed', 'dismissed', 'inconclusive', 'abusive_report');
exception when duplicate_object then null;
end $$;
do $$
begin
  create type gd_comment_status as enum ('visible', 'deleted', 'moderated');
exception when duplicate_object then null;
end $$;
do $$
begin
  create type gd_discord_sync_action as enum ('sync', 'cleanup_roles');
exception when duplicate_object then null;
end $$;
do $$
begin
  create type gd_notification_type as enum (
    'friend_request_received', 'friendship_accepted', 'party_invitation_received',
    'badge_unlocked', 'account_banned', 'account_unbanned',
    'reported_player_banned', 'mmr_refund'
  );
exception when duplicate_object then null;
end $$;
do $$
begin
  create type gd_notification_category as enum ('system', 'social', 'moderation', 'match', 'map');
exception when duplicate_object then null;
end $$;
do $$
begin
  create type gd_notification_entity_kind as enum ('friend_request', 'friendship', 'party_invitation', 'badge', 'match', 'map', 'comment', 'player');
exception when duplicate_object then null;
end $$;
do $$
begin
  create type gd_notification_outbox_type as enum ('moderation_signal_queued');
exception when duplicate_object then null;
end $$;
do $$
begin
  create type gd_user_event_type as enum ('friendship.created', 'friend_request.created', 'party_invitation.created');
exception when duplicate_object then null;
end $$;
do $$
begin
  create type gd_moderation_log_action as enum (
    'role_granted', 'role_revoked', 'permanent_ban', 'temporary_ban',
    'warning', 'chat_mute', 'report_mute', 'report_unmute', 'unban',
    'refund', 'note', 'assign', 'status', 'dismiss', 'mark_inconclusive',
    'abusive_reports', 'signal_attached', 'verdict_submitted', 'badge_granted'
  );
exception when duplicate_object then null;
end $$;

-- Partial indexes and old checks contain text operators.  Drop them before
-- changing the operator type, then recreate them below.
drop index if exists idx_users_nickname_claimed;
drop index if exists idx_users_nickname_registered;
drop index if exists users_claimed_nickname_unique;
drop index if exists idx_maps_owner_active;
drop index if exists idx_maps_public;
drop index if exists idx_maps_browse;
drop index if exists idx_maps_public_new;
drop index if exists idx_maps_public_popular;
drop index if exists idx_maps_public_trending;
drop index if exists idx_map_comments_visible;
drop index if exists idx_friend_requests_active_pair;
drop index if exists idx_friend_requests_recipient_pending;
drop index if exists idx_friend_requests_sender_pending;
drop index if exists idx_party_invitations_active;
drop index if exists idx_party_invitations_recipient;
drop index if exists idx_chat_messages_team_history;
drop index if exists idx_moderation_signals_report_dedupe;
drop index if exists idx_moderation_signals_detector_dedupe;
drop index if exists idx_moderation_signals_unreviewed;
drop index if exists idx_match_sessions_expired_lease;

alter table users drop constraint if exists users_account_type_check;
alter table users drop constraint if exists users_claimed_nickname_format_check;
alter table user_identities drop constraint if exists user_identities_provider_not_guest;
alter table maps drop constraint if exists maps_visibility_check, drop constraint if exists maps_status_check, drop constraint if exists maps_difficulty_check;
alter table map_comments drop constraint if exists map_comments_status_check;
alter table parties drop constraint if exists parties_state_check, drop constraint if exists parties_mode_check;
alter table party_members drop constraint if exists party_members_role_check, drop constraint if exists party_members_team_check;
alter table match_sessions drop constraint if exists match_sessions_state_check;
alter table chat_conversations drop constraint if exists chat_conversations_scope_kind_check;
alter table chat_messages drop constraint if exists chat_messages_kind_check, drop constraint if exists chat_messages_text_body_check, drop constraint if exists chat_messages_audience_check, drop constraint if exists chat_messages_team_audience_check;
alter table friend_requests drop constraint if exists friend_requests_status_check;
alter table party_invitations drop constraint if exists party_invitations_status_check;
alter table moderation_signals drop constraint if exists moderation_signals_source_check, drop constraint if exists moderation_signals_severity_check, drop constraint if exists moderation_signals_evidence_strength_check, drop constraint if exists moderation_signals_outcome_check;
alter table discord_sync_outbox drop constraint if exists discord_sync_outbox_action_check;
alter table map_comments drop constraint if exists map_comments_body_check;

-- Defaults are dropped because a text literal cannot remain attached while its
-- column changes to an enum.  They are restored using typed enum literals.
alter table users alter column account_type drop default;
alter table maps alter column visibility drop default, alter column status drop default, alter column difficulty drop default;
alter table map_comments alter column status drop default;
alter table parties alter column state drop default, alter column mode drop default;
alter table match_history alter column source_kind drop default;
alter table party_members
  alter column role drop default,
  alter column team_id drop default;
alter table match_sessions alter column state drop default, alter column source_kind drop default;
alter table chat_messages
  alter column audience drop default,
  alter column team_id drop default;
alter table friend_requests alter column status drop default;
alter table party_invitations alter column status drop default;
alter table user_notifications alter column category drop default;

alter table users alter column account_type type gd_account_type using account_type::text::gd_account_type;
alter table user_identities alter column provider type gd_oauth_provider using provider::text::gd_oauth_provider;
alter table user_identity_history alter column provider type gd_oauth_provider using provider::text::gd_oauth_provider;
alter table oauth_identity_bans alter column provider type gd_oauth_provider using provider::text::gd_oauth_provider;
alter table ranks alter column mode type gd_match_mode using mode::text::gd_match_mode;
alter table ranked_stats alter column mode type gd_match_mode using mode::text::gd_match_mode;
alter table maps alter column visibility type gd_map_visibility using visibility::text::gd_map_visibility,
  alter column status type gd_map_status using status::text::gd_map_status,
  alter column difficulty type gd_map_difficulty using difficulty::text::gd_map_difficulty;
alter table runtime_matches alter column state type gd_runtime_state using state::text::gd_runtime_state;
alter table parties alter column state type gd_party_state using state::text::gd_party_state,
  alter column mode type gd_match_mode using mode::text::gd_match_mode;
alter table party_members alter column role type gd_party_role using role::text::gd_party_role,
  alter column team_id type gd_team_id using nullif(team_id, '')::text::gd_team_id;
alter table match_history alter column mode type gd_match_mode using mode::text::gd_match_mode,
  alter column source_kind type gd_match_source using source_kind::text::gd_match_source,
  alter column ruleset type gd_ruleset using nullif(ruleset, '')::text::gd_ruleset;
alter table match_sessions alter column preset_id type gd_match_preset using preset_id::text::gd_match_preset;
alter table match_sessions alter column mode type gd_match_mode using mode::text::gd_match_mode;
alter table match_sessions alter column state type gd_match_session_state using state::text::gd_match_session_state;
alter table match_sessions alter column source_kind type gd_match_source using source_kind::text::gd_match_source;
alter table chat_conversations alter column scope_kind type gd_chat_scope using scope_kind::text::gd_chat_scope;
alter table chat_messages alter column kind type gd_chat_kind using kind::text::gd_chat_kind,
  alter column audience type gd_chat_audience using audience::text::gd_chat_audience,
  alter column team_id type gd_team_id using nullif(team_id, '')::text::gd_team_id;
alter table friend_requests alter column status type gd_social_request_status using status::text::gd_social_request_status;
alter table party_invitations alter column status type gd_social_request_status using status::text::gd_social_request_status;
alter table user_notifications alter column type type gd_notification_type using type::text::gd_notification_type,
  alter column category type gd_notification_category using category::text::gd_notification_category,
  alter column entity_kind type gd_notification_entity_kind using nullif(entity_kind, '')::text::gd_notification_entity_kind;
alter table notification_outbox alter column type type gd_notification_outbox_type using type::text::gd_notification_outbox_type;
alter table user_events alter column type type gd_user_event_type using type::text::gd_user_event_type;
alter table moderation_signals alter column source type gd_moderation_source using source::text::gd_moderation_source,
  alter column severity type gd_moderation_severity using severity::text::gd_moderation_severity,
  alter column evidence_strength type gd_evidence_strength using evidence_strength::text::gd_evidence_strength,
  alter column outcome type gd_moderation_outcome using nullif(outcome, '')::text::gd_moderation_outcome;
alter table moderation_log alter column action type gd_moderation_log_action using action::text::gd_moderation_log_action;
alter table map_comments alter column status type gd_comment_status using status::text::gd_comment_status;
alter table discord_sync_outbox alter column action type gd_discord_sync_action using action::text::gd_discord_sync_action;

alter table users alter column account_type set default 'registered'::gd_account_type;
alter table maps alter column visibility set default 'private'::gd_map_visibility,
  alter column status set default 'ready'::gd_map_status,
  alter column difficulty set default 'normal'::gd_map_difficulty;
alter table map_comments alter column status set default 'visible'::gd_comment_status;
alter table parties alter column state set default 'open'::gd_party_state, alter column mode set default 'duel'::gd_match_mode;
alter table party_members alter column role set default 'member'::gd_party_role;
alter table match_sessions alter column state set default 'live'::gd_match_session_state, alter column source_kind set default 'queue'::gd_match_source;
alter table chat_messages alter column audience set default 'all'::gd_chat_audience;
alter table friend_requests alter column status set default 'pending'::gd_social_request_status;
alter table party_invitations alter column status set default 'pending'::gd_social_request_status;
alter table user_notifications alter column category set default 'system'::gd_notification_category;

alter table match_history alter column source_kind set default 'queue'::gd_match_source;

-- Normalize rows written by the old failed/partial tombstone implementation
-- before enforcing the new state invariant.
alter table map_comments alter column body drop not null;
update map_comments
set body = null,
    deleted_at = coalesce(deleted_at, updated_at, now())
where status in ('deleted'::gd_comment_status, 'moderated'::gd_comment_status);

-- A comment tombstone deliberately has no body.  Keeping the row preserves
-- reply topology while preventing the deleted text from being retained.
alter table map_comments add constraint map_comments_state_check check (
  (status = 'visible'::gd_comment_status and body is not null and length(btrim(body)) between 1 and 1000 and deleted_by is null and deleted_at is null)
  or (status in ('deleted'::gd_comment_status, 'moderated'::gd_comment_status) and body is null and deleted_at is not null)
);
alter table users add constraint users_claimed_nickname_format_check check (
  nickname_claimed_at is null or (
    account_type = 'registered'::gd_account_type
    and display_name ~ '^[A-Za-z0-9._]{2,14}$'
    and position('..' in display_name) = 0
    and position('__' in display_name) = 0
  )
);
alter table chat_messages add constraint chat_messages_text_body_check check (
  (kind = 'text'::gd_chat_kind and body is not null and length(body) > 0 and emote is null)
  or (kind = 'emote'::gd_chat_kind and emote in ('skull', 'sob', 'thinking', 'sunglasses') and body is null)
);
alter table chat_messages add constraint chat_messages_audience_check check (audience in ('all'::gd_chat_audience, 'team'::gd_chat_audience));
alter table chat_messages add constraint chat_messages_team_audience_check check (
  (audience = 'all'::gd_chat_audience and team_id is null and team_match_id is null)
  or (audience = 'team'::gd_chat_audience and team_match_id is not null and team_id is not null)
);

create unique index users_claimed_nickname_unique on users(lower(display_name)) where account_type = 'registered'::gd_account_type and nickname_claimed_at is not null;
create index idx_maps_owner_active on maps(owner_user_id, updated_at desc) where archived_at is null;
create index idx_maps_public_new on maps(published_at desc) where archived_at is null and published_at is not null and status = 'ready'::gd_map_status;
create index idx_maps_public_popular on maps((play_count + (favorite_count * 3)) desc, published_at desc) where archived_at is null and published_at is not null and status = 'ready'::gd_map_status;
create index idx_maps_public_trending on maps(trending_score desc, published_at desc) where archived_at is null and published_at is not null and status = 'ready'::gd_map_status;
create unique index idx_friend_requests_active_pair on friend_requests(least(sender_user_id, recipient_user_id), greatest(sender_user_id, recipient_user_id)) where status = 'pending'::gd_social_request_status;
create index idx_friend_requests_recipient_pending on friend_requests(recipient_user_id, created_at desc) where status = 'pending'::gd_social_request_status;
create index idx_friend_requests_sender_pending on friend_requests(sender_user_id, created_at desc) where status = 'pending'::gd_social_request_status;
create index idx_party_invitations_active on party_invitations(party_id, recipient_user_id) where status = 'pending'::gd_social_request_status;
create index idx_party_invitations_recipient on party_invitations(recipient_user_id, created_at desc) where status = 'pending'::gd_social_request_status;
create index idx_chat_messages_team_history on chat_messages(conversation_id, team_match_id, team_id, created_at) where audience = 'team'::gd_chat_audience;
create index idx_moderation_signals_report_dedupe on moderation_signals(match_id, reporter_user_id, subject_user_id) where source = 'player_report'::gd_moderation_source and reporter_user_id is not null and match_id is not null;
create index idx_moderation_signals_detector_dedupe on moderation_signals(subject_user_id, coalesce(match_id, '00000000-0000-0000-0000-000000000000'::uuid), coalesce(detector_key, ''), coalesce(detector_version, ''), reason_code) where source = 'risk_engine'::gd_moderation_source;
create index idx_moderation_signals_unreviewed on moderation_signals(recommended_queue, severity, occurred_at desc) where reviewed_at is null;
create index idx_match_sessions_expired_lease on match_sessions(lease_expires_at, match_id) where state = 'live'::gd_match_session_state;
