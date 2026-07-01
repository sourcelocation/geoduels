alter table chat_conversations
  drop constraint if exists chat_conversations_scope_kind_check;

alter table chat_messages
  drop constraint if exists chat_messages_conversation_id_fkey;

update chat_messages
set conversation_id = 'lobby:' || substring(conversation_id from length('party:') + 1)
where conversation_id like 'party:%';

delete from chat_conversations current
using chat_conversations legacy
where current.scope_kind = 'party'
  and legacy.scope_kind = 'lobby'
  and legacy.id = 'lobby:' || current.scope_id;

update chat_conversations
set
  id = 'lobby:' || scope_id,
  scope_kind = 'lobby'
where scope_kind = 'party';

alter table chat_conversations
  add constraint chat_conversations_scope_kind_check
  check (scope_kind in ('lobby', 'match'));

alter table chat_messages
  add constraint chat_messages_conversation_id_fkey
  foreign key (conversation_id) references chat_conversations(id) on delete cascade;

update match_history
set source_kind = 'lobby'
where source_kind = 'party';

do $$
begin
  if to_regclass('public.match_round_guesses') is not null then
    update match_round_guesses
    set source_kind = 'lobby'
    where source_kind = 'party';
  end if;
end
$$;

update match_sessions
set source_kind = 'lobby'
where source_kind = 'party';
