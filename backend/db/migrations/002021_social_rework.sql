-- +goose Up
-- Who is connected is one row per connection, kept by the gateways. A gateway refreshes only its own
-- row, and a gone gateway's connections go with it. Losing all of this in a crash loses nothing:
-- gateways report what they hold on their next heartbeat.
DROP TABLE public.presence;
ALTER TABLE public.party_members DROP COLUMN seen_at;

CREATE UNLOGGED TABLE public.gateways (
    id uuid PRIMARY KEY,
    seen_at timestamp with time zone DEFAULT now() NOT NULL
);

CREATE UNLOGGED TABLE public.live_connections (
    id uuid PRIMARY KEY,
    gateway_id uuid NOT NULL,
    -- Null for a visitor who hasn't started playing.
    user_id uuid,
    connected_at timestamp with time zone DEFAULT now() NOT NULL
);
CREATE INDEX live_connections_user ON public.live_connections (user_id);
CREATE INDEX live_connections_gateway ON public.live_connections (gateway_id);

-- A party's revision grows each time it is sent to its members, so an older copy can't replace a newer one.
ALTER TABLE public.parties ADD COLUMN revision bigint DEFAULT 0 NOT NULL;

-- The inbox: messages with no actions, in the words people read. Friend requests and party
-- invitations are no longer notifications: they appear in the conversation with that person.
DELETE FROM public.user_notifications WHERE type IN ('friend_request_received', 'party_invitation_received');

ALTER TABLE public.user_notifications RENAME TO inbox_messages;
ALTER SEQUENCE public.user_notifications_id_seq RENAME TO inbox_messages_id_seq;
ALTER INDEX public.user_notifications_dedupe_key_key RENAME TO inbox_messages_dedupe_key_key;
ALTER INDEX public.user_notifications_pkey RENAME TO inbox_messages_pkey;
ALTER TABLE public.inbox_messages
    ADD COLUMN title text DEFAULT '' NOT NULL,
    ADD COLUMN body text DEFAULT '' NOT NULL,
    ADD COLUMN image_url text DEFAULT '' NOT NULL,
    ADD COLUMN modal boolean DEFAULT false NOT NULL;

UPDATE public.inbox_messages n SET
    title = CASE n.type::text
        WHEN 'friendship_accepted' THEN coalesce(nullif(a.display_name, '') || ' accepted your friend request', 'Friend request accepted')
        WHEN 'badge_unlocked' THEN 'New badge unlocked'
        WHEN 'mmr_refund' THEN 'Rating refunded'
        WHEN 'account_banned' THEN 'Account suspended'
        WHEN 'account_unbanned' THEN 'Account restriction removed'
        WHEN 'reported_player_banned' THEN 'Report action taken'
        WHEN 'moderation_warning' THEN CASE WHEN n.payload_json ? 'withdrawn' THEN 'A warning was withdrawn' ELSE 'Moderation warning' END
        ELSE 'Notification'
    END,
    body = CASE n.type::text
        WHEN 'badge_unlocked' THEN concat_ws(' ', n.payload_json #>> '{badge,label}', n.payload_json #>> '{badge,description}')
        WHEN 'mmr_refund' THEN '+' || coalesce(n.payload_json ->> 'refundDelta', '0') || ' MMR. A player you lost to was banned for cheating. Your rating has been recalculated.'
        WHEN 'account_banned' THEN coalesce('Reason: ' || nullif(n.payload_json ->> 'reason', ''), 'Your account access has been restricted.')
        WHEN 'reported_player_banned' THEN 'A player you reported was suspended after review. Thanks for keeping GeoDuels fair.'
        WHEN 'moderation_warning' THEN CASE WHEN n.payload_json ? 'withdrawn' THEN '' ELSE coalesce(n.payload_json ->> 'reason', '') END
        ELSE ''
    END,
    image_url = coalesce(n.payload_json #>> '{badge,imageUrl}', ''),
    modal = n.type::text IN ('account_banned', 'moderation_warning') AND NOT n.payload_json ? 'withdrawn'
FROM public.inbox_messages m LEFT JOIN public.users a ON a.id = m.actor_user_id
WHERE m.id = n.id;

ALTER TABLE public.inbox_messages ALTER COLUMN type TYPE text USING type::text;
ALTER TABLE public.inbox_messages RENAME COLUMN type TO kind;
ALTER TABLE public.inbox_messages
    DROP COLUMN payload_json,
    DROP COLUMN category,
    DROP COLUMN actor_user_id,
    DROP COLUMN entity_kind,
    DROP COLUMN entity_id,
    DROP COLUMN archived_at,
    DROP COLUMN expires_at;
DROP TYPE public.gd_notification_type;
DROP TYPE public.gd_notification_category;
DROP TYPE public.gd_notification_entity_kind;

-- Reading a warning is acknowledging it.
ALTER TABLE public.moderation_warnings ADD COLUMN inbox_message_id bigint;
UPDATE public.moderation_warnings w SET inbox_message_id = m.id
FROM public.inbox_messages m WHERE m.dedupe_key = 'warning:' || w.id;
UPDATE public.inbox_messages m SET read_at = coalesce(m.read_at, w.acknowledged_at)
FROM public.moderation_warnings w WHERE w.inbox_message_id = m.id AND w.acknowledged_at IS NOT NULL;
UPDATE public.inbox_messages m SET read_at = NULL, modal = true
FROM public.moderation_warnings w
WHERE w.inbox_message_id = m.id AND w.acknowledged_at IS NULL AND w.withdrawn_at IS NULL;
ALTER TABLE public.moderation_warnings DROP COLUMN acknowledged_at;

-- Direct conversations, and how far each person has read in them.
ALTER TYPE public.gd_chat_scope ADD VALUE IF NOT EXISTS 'direct';
CREATE TABLE public.chat_reads (
    conversation_id uuid NOT NULL,
    user_id uuid NOT NULL REFERENCES public.users (id) ON DELETE CASCADE,
    read_at timestamp with time zone NOT NULL,
    PRIMARY KEY (conversation_id, user_id)
);

-- Nothing has read or written these since the live socket replaced them.
DROP TABLE public.user_event_sequences;
DROP TABLE public.user_events;
DROP TYPE public.gd_user_event_type;

-- +goose Down
-- Inbox words, connection rows and read positions can't be turned back into what they replaced, and
-- PostgreSQL can't remove the 'direct' enum value. This restores the shape only.
CREATE TYPE public.gd_user_event_type AS ENUM ('presence.changed');
CREATE TABLE public.user_event_sequences (user_id uuid PRIMARY KEY, sequence bigint NOT NULL);
CREATE TABLE public.user_events (
    user_id uuid NOT NULL, sequence bigint NOT NULL, type public.gd_user_event_type NOT NULL,
    payload_json jsonb NOT NULL, created_at timestamp with time zone DEFAULT now() NOT NULL,
    PRIMARY KEY (user_id, sequence)
);
DROP TABLE public.chat_reads;
ALTER TABLE public.moderation_warnings ADD COLUMN acknowledged_at timestamp with time zone;
ALTER TABLE public.moderation_warnings DROP COLUMN inbox_message_id;
CREATE TYPE public.gd_notification_type AS ENUM ('friend_request_received', 'friendship_accepted', 'party_invitation_received',
    'badge_unlocked', 'account_banned', 'account_unbanned', 'reported_player_banned', 'mmr_refund', 'moderation_warning');
CREATE TYPE public.gd_notification_category AS ENUM ('system');
CREATE TYPE public.gd_notification_entity_kind AS ENUM ('party');
DELETE FROM public.inbox_messages;
ALTER TABLE public.inbox_messages
    DROP COLUMN title, DROP COLUMN body, DROP COLUMN image_url, DROP COLUMN modal,
    ADD COLUMN payload_json jsonb DEFAULT '{}'::jsonb NOT NULL,
    ADD COLUMN category public.gd_notification_category DEFAULT 'system' NOT NULL,
    ADD COLUMN actor_user_id uuid REFERENCES public.users (id) ON DELETE SET NULL,
    ADD COLUMN entity_kind public.gd_notification_entity_kind,
    ADD COLUMN entity_id text,
    ADD COLUMN archived_at timestamp with time zone,
    ADD COLUMN expires_at timestamp with time zone;
ALTER TABLE public.inbox_messages RENAME COLUMN kind TO type;
ALTER TABLE public.inbox_messages ALTER COLUMN type TYPE public.gd_notification_type USING type::public.gd_notification_type;
ALTER INDEX public.inbox_messages_pkey RENAME TO user_notifications_pkey;
ALTER INDEX public.inbox_messages_dedupe_key_key RENAME TO user_notifications_dedupe_key_key;
ALTER SEQUENCE public.inbox_messages_id_seq RENAME TO user_notifications_id_seq;
ALTER TABLE public.inbox_messages RENAME TO user_notifications;
ALTER TABLE public.parties DROP COLUMN revision;
DROP TABLE public.live_connections;
DROP TABLE public.gateways;
CREATE TABLE public.presence (user_id uuid PRIMARY KEY, seen_at timestamp with time zone NOT NULL);
ALTER TABLE public.party_members ADD COLUMN seen_at timestamp with time zone;
