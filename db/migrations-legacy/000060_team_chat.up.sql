alter table chat_messages
  add column if not exists audience text not null default 'all',
  add column if not exists team_id text,
  add column if not exists team_match_id uuid references match_sessions(match_id) on delete cascade;

alter table chat_messages
  drop constraint if exists chat_messages_audience_check,
  drop constraint if exists chat_messages_team_audience_check;

alter table chat_messages
  add constraint chat_messages_audience_check check (audience in ('all', 'team')),
  add constraint chat_messages_team_audience_check check (
    (audience = 'all' and team_id is null and team_match_id is null) or
    (audience = 'team' and team_match_id is not null and team_id in ('a', 'b'))
  );

create index if not exists idx_chat_messages_team_history
on chat_messages(conversation_id, team_match_id, team_id, created_at)
where audience = 'team';
