alter table chat_conversations
  drop constraint if exists chat_conversations_scope_kind_check;

alter table chat_messages
  drop constraint if exists chat_messages_conversation_id_fkey;

update chat_messages
set conversation_id = 'party:' || substring(conversation_id from length('lobby:') + 1)
where conversation_id like 'lobby:%';

delete from chat_conversations legacy
using chat_conversations current
where legacy.scope_kind = 'lobby'
  and current.scope_kind = 'party'
  and current.id = 'party:' || legacy.scope_id;

update chat_conversations
set
  id = 'party:' || scope_id,
  scope_kind = 'party'
where scope_kind = 'lobby';

alter table chat_conversations
  add constraint chat_conversations_scope_kind_check
  check (scope_kind in ('party', 'match'));

alter table chat_messages
  add constraint chat_messages_conversation_id_fkey
  foreign key (conversation_id) references chat_conversations(id) on delete cascade;

update match_history
set source_kind = 'party'
where source_kind = 'lobby';

do $$
begin
  if to_regclass('public.match_round_guesses') is not null then
    update match_round_guesses
    set source_kind = 'party'
    where source_kind = 'lobby';
  end if;
end
$$;

update match_sessions
set source_kind = 'party'
where source_kind = 'lobby';
