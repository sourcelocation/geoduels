drop table if exists legacy_id_aliases;

alter table chat_conversations
  alter column scope_id type uuid using scope_id::uuid;
