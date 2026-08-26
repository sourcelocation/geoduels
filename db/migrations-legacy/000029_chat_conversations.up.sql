create table if not exists chat_conversations (
  id text primary key,
  scope_kind text not null,
  scope_id text not null,
  created_at timestamptz not null default now(),
  constraint chat_conversations_scope_kind_check check (scope_kind in ('lobby', 'match'))
);

create unique index if not exists idx_chat_conversations_scope
on chat_conversations(scope_kind, scope_id);

create table if not exists chat_messages (
  id text primary key,
  conversation_id text not null references chat_conversations(id) on delete cascade,
  match_id text,
  sender_user_id text not null references users(id) on delete cascade,
  sender_display_name text not null,
  kind text not null,
  body text,
  emote text,
  moderation_state text not null default 'visible',
  deleted_at timestamptz,
  metadata jsonb not null default '{}'::jsonb,
  created_at timestamptz not null default now(),
  constraint chat_messages_kind_check check (kind in ('text', 'emote')),
  constraint chat_messages_text_body_check check (
    (kind = 'text' and body is not null and length(body) > 0 and emote is null)
    or
    (kind = 'emote' and emote in ('skull', 'sob', 'thinking', 'sunglasses') and body is null)
  )
);

create index if not exists idx_chat_messages_conversation_created
on chat_messages(conversation_id, created_at);

create index if not exists idx_chat_messages_match_created
on chat_messages(match_id, created_at);

create index if not exists idx_chat_messages_sender_created
on chat_messages(sender_user_id, created_at desc);

create index if not exists idx_chat_messages_moderation
on chat_messages(moderation_state, created_at desc);
