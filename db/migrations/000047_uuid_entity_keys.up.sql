-- Compact entity storage. Application protocols continue to carry canonical
-- UUID strings; browser routes use reversible Crockford Base32 identifiers.
-- This migration rewrites primary/foreign-key indexes and requires a
-- maintenance window.

create or replace function geoduels_entity_uuid(namespace text, value text)
returns uuid
language plpgsql
immutable
strict
as $$
declare
  raw bytea;
  encoded text;
begin
  if value ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$' then
    return value::uuid;
  end if;
  raw := decode(md5(namespace || ':' || value), 'hex');
  raw := set_byte(raw, 6, (get_byte(raw, 6) & 15) | 48);
  raw := set_byte(raw, 8, (get_byte(raw, 8) & 63) | 128);
  encoded := encode(raw, 'hex');
  return (
    substr(encoded, 1, 8) || '-' ||
    substr(encoded, 9, 4) || '-' ||
    substr(encoded, 13, 4) || '-' ||
    substr(encoded, 17, 4) || '-' ||
    substr(encoded, 21, 12)
  )::uuid;
end
$$;

create table if not exists legacy_id_aliases (
  entity_type text not null,
  legacy_id text not null,
  entity_id uuid not null,
  created_at timestamptz not null default now(),
  primary key (entity_type, legacy_id),
  unique (entity_type, entity_id)
);

insert into legacy_id_aliases(entity_type, legacy_id, entity_id)
select 'user', id, geoduels_entity_uuid('user', id)
from users
where id !~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
on conflict do nothing;

insert into legacy_id_aliases(entity_type, legacy_id, entity_id)
select 'session', id, geoduels_entity_uuid('session', id)
from auth_sessions
where id !~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
on conflict do nothing;

insert into legacy_id_aliases(entity_type, legacy_id, entity_id)
select 'match', id, geoduels_entity_uuid('match', id)
from runtime_matches
where id !~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
union
select 'match', match_id, geoduels_entity_uuid('match', match_id)
from match_history
where match_id !~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
union
select 'match', match_id, geoduels_entity_uuid('match', match_id)
from match_sessions
where match_id !~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
on conflict do nothing;

insert into legacy_id_aliases(entity_type, legacy_id, entity_id)
select 'party', id, geoduels_entity_uuid('party', id)
from parties
where id !~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
on conflict do nothing;

insert into legacy_id_aliases(entity_type, legacy_id, entity_id)
select 'comment', id, geoduels_entity_uuid('comment', id)
from map_comments
where id !~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$'
on conflict do nothing;

-- Conversation IDs are derived from their public scope. Preserve that
-- relationship while replacing legacy party/match scope IDs so historical
-- chat remains addressable as "party:<uuid>" or "match:<uuid>".
create temporary table geoduels_chat_conversation_ids on commit drop as
select
  id as legacy_id,
  geoduels_entity_uuid(
    'conversation',
    scope_kind || ':' ||
    case scope_kind
      when 'party' then geoduels_entity_uuid('party', scope_id)::text
      when 'match' then geoduels_entity_uuid('match', scope_id)::text
      else scope_id
    end
  ) as entity_id,
  case scope_kind
    when 'party' then geoduels_entity_uuid('party', scope_id)::text
    when 'match' then geoduels_entity_uuid('match', scope_id)::text
    else scope_id
  end as scope_id
from chat_conversations;

create temporary table geoduels_saved_foreign_keys on commit drop as
select conrelid::regclass::text as table_name, conname, pg_get_constraintdef(oid) as definition
from pg_constraint
where contype='f';

do $$
declare item record;
begin
  for item in select * from geoduels_saved_foreign_keys loop
    execute format('alter table %s drop constraint %I', item.table_name, item.conname);
  end loop;
end
$$;

-- User keys and references.
alter table users alter column id type uuid using geoduels_entity_uuid('user', id);
alter table user_identities alter column user_id type uuid using geoduels_entity_uuid('user', user_id);
alter table auth_sessions
  alter column id type uuid using geoduels_entity_uuid('session', id),
  alter column user_id type uuid using geoduels_entity_uuid('user', user_id);
alter table ranks alter column user_id type uuid using geoduels_entity_uuid('user', user_id);
alter table user_stats alter column user_id type uuid using geoduels_entity_uuid('user', user_id);
alter table ranked_stats alter column user_id type uuid using geoduels_entity_uuid('user', user_id);
alter table maps
  alter column owner_user_id type uuid using geoduels_entity_uuid('user', owner_user_id),
  alter column official_by type uuid using geoduels_entity_uuid('user', official_by);
alter table ip_signup_bans alter column created_by type uuid using geoduels_entity_uuid('user', created_by);
alter table moderation_cases
  alter column target_user_id type uuid using geoduels_entity_uuid('user', target_user_id),
  alter column assigned_to type uuid using geoduels_entity_uuid('user', assigned_to),
  alter column resolved_by type uuid using geoduels_entity_uuid('user', resolved_by);
alter table moderation_case_events alter column actor_user_id type uuid using geoduels_entity_uuid('user', actor_user_id);
alter table moderation_actions
  alter column actor_user_id type uuid using geoduels_entity_uuid('user', actor_user_id),
  alter column target_user_id type uuid using geoduels_entity_uuid('user', target_user_id);
alter table moderation_reporter_reputation alter column user_id type uuid using geoduels_entity_uuid('user', user_id);
alter table user_notifications alter column user_id type uuid using geoduels_entity_uuid('user', user_id);
alter table user_identity_history alter column user_id type uuid using geoduels_entity_uuid('user', user_id);
alter table oauth_identity_bans
  alter column banned_user_id type uuid using geoduels_entity_uuid('user', banned_user_id),
  alter column created_by type uuid using geoduels_entity_uuid('user', created_by);
alter table user_badges alter column user_id type uuid using geoduels_entity_uuid('user', user_id);
alter table support_donation_refs alter column user_id type uuid using geoduels_entity_uuid('user', user_id);
alter table user_roles
  alter column user_id type uuid using geoduels_entity_uuid('user', user_id),
  alter column granted_by type uuid using geoduels_entity_uuid('user', granted_by),
  alter column revoked_by type uuid using geoduels_entity_uuid('user', revoked_by);
alter table moderation_case_log alter column actor_user_id type uuid using geoduels_entity_uuid('user', actor_user_id);
alter table enforcement_actions
  alter column target_user_id type uuid using geoduels_entity_uuid('user', target_user_id),
  alter column actor_user_id type uuid using geoduels_entity_uuid('user', actor_user_id),
  alter column revoked_by type uuid using geoduels_entity_uuid('user', revoked_by);
alter table map_upload_events alter column user_id type uuid using geoduels_entity_uuid('user', user_id);
alter table map_favorites alter column user_id type uuid using geoduels_entity_uuid('user', user_id);
alter table map_daily_users alter column user_id type uuid using geoduels_entity_uuid('user', user_id);

-- Match keys and references.
alter table runtime_matches alter column id type uuid using geoduels_entity_uuid('match', id);
alter table match_history
  alter column match_id type uuid using geoduels_entity_uuid('match', match_id),
  alter column winner_user_id type uuid using geoduels_entity_uuid('user', nullif(winner_user_id, '')),
  alter column source_party_id type uuid using geoduels_entity_uuid('party', nullif(source_party_id, '')),
  alter column map_revision_id type uuid using geoduels_entity_uuid('revision', map_revision_id);
alter table match_players
  alter column match_id type uuid using geoduels_entity_uuid('match', match_id),
  alter column user_id type uuid using geoduels_entity_uuid('user', user_id);
alter table elo_refunds
  alter column match_id type uuid using geoduels_entity_uuid('match', match_id),
  alter column user_id type uuid using geoduels_entity_uuid('user', user_id),
  alter column cheater_user_id type uuid using geoduels_entity_uuid('user', cheater_user_id);
alter table moderation_reports
  alter column match_id type uuid using geoduels_entity_uuid('match', match_id),
  alter column reporter_user_id type uuid using geoduels_entity_uuid('user', reporter_user_id),
  alter column reported_user_id type uuid using geoduels_entity_uuid('user', reported_user_id);
alter table ranked_guess_events
  alter column match_id type uuid using geoduels_entity_uuid('match', match_id),
  alter column user_id type uuid using geoduels_entity_uuid('user', user_id);
alter table match_round_plans alter column match_id type uuid using geoduels_entity_uuid('match', match_id);
alter table moderation_evidence
  alter column match_id type uuid using geoduels_entity_uuid('match', match_id),
  alter column subject_user_id type uuid using geoduels_entity_uuid('user', subject_user_id);

-- Party and session keys.
alter table parties
  alter column id type uuid using geoduels_entity_uuid('party', id),
  alter column owner_user_id type uuid using geoduels_entity_uuid('user', owner_user_id),
  alter column active_match_id type uuid using geoduels_entity_uuid('match', active_match_id),
  alter column started_match_id type uuid using geoduels_entity_uuid('match', started_match_id),
  alter column last_match_id type uuid using geoduels_entity_uuid('match', last_match_id);
alter table party_members
  alter column party_id type uuid using geoduels_entity_uuid('party', party_id),
  alter column user_id type uuid using geoduels_entity_uuid('user', user_id);
alter table match_sessions
  alter column match_id type uuid using geoduels_entity_uuid('match', match_id),
  alter column source_party_id type uuid using geoduels_entity_uuid('party', source_party_id);
alter table match_participants
  alter column match_id type uuid using geoduels_entity_uuid('match', match_id),
  alter column user_id type uuid using geoduels_entity_uuid('user', user_id);

-- Revision, comment, and chat keys.
alter table map_revisions alter column id type uuid using geoduels_entity_uuid('revision', id);
alter table maps alter column active_revision_id type uuid using geoduels_entity_uuid('revision', active_revision_id);
alter table match_round_plans alter column map_revision_id type uuid using geoduels_entity_uuid('revision', map_revision_id);
alter table match_sessions alter column map_revision_id type uuid using geoduels_entity_uuid('revision', map_revision_id);
alter table map_revision_country_stats alter column map_revision_id type uuid using geoduels_entity_uuid('revision', map_revision_id);

alter table map_comments
  alter column id type uuid using geoduels_entity_uuid('comment', id),
  alter column parent_id type uuid using geoduels_entity_uuid('comment', parent_id),
  alter column user_id type uuid using geoduels_entity_uuid('user', user_id),
  alter column deleted_by type uuid using geoduels_entity_uuid('user', deleted_by);
alter table map_comment_likes
  alter column comment_id type uuid using geoduels_entity_uuid('comment', comment_id),
  alter column user_id type uuid using geoduels_entity_uuid('user', user_id);

update chat_messages m
set conversation_id = mapped.entity_id::text
from geoduels_chat_conversation_ids mapped
where m.conversation_id = mapped.legacy_id;

update chat_conversations c
set id = mapped.entity_id::text,
    scope_id = mapped.scope_id
from geoduels_chat_conversation_ids mapped
where c.id = mapped.legacy_id;

alter table chat_conversations alter column id type uuid using id::uuid;
alter table chat_messages
  alter column id type uuid using geoduels_entity_uuid('message', id),
  alter column conversation_id type uuid using conversation_id::uuid,
  alter column sender_user_id type uuid using geoduels_entity_uuid('user', sender_user_id);

do $$
declare item record;
begin
  for item in select * from geoduels_saved_foreign_keys loop
    execute format('alter table %s add constraint %I %s', item.table_name, item.conname, item.definition);
  end loop;
end
$$;

drop function geoduels_entity_uuid(text, text);
