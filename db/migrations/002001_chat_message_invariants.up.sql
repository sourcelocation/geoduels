-- Keep the durable chat invariant in step with the accepted wire emotes.
-- Public match messages intentionally have no team authorization context.
alter table chat_messages
  drop constraint if exists chat_messages_text_body_check;

alter table chat_messages
  add constraint chat_messages_text_body_check check (
    (kind = 'text'::gd_chat_kind and body is not null and length(body) > 0 and emote is null)
    or (kind = 'emote'::gd_chat_kind and emote in ('skull'::text, 'sob'::text, 'thinking'::text, 'sunglasses'::text, 'wave'::text) and body is null)
  );
