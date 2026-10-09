-- +goose Up
-- The custom outbox tables are replaced by River (internal/jobs). Drop them and
-- the enums they owned.
DROP TABLE IF EXISTS public.notification_outbox;
DROP TABLE IF EXISTS public.discord_sync_outbox;
DROP TYPE IF EXISTS public.gd_notification_outbox_type;
DROP TYPE IF EXISTS public.gd_discord_sync_action;

-- +goose Down
-- The outboxes come back empty; River's jobs aren't moved into them.
CREATE TYPE public.gd_discord_sync_action AS ENUM ('sync', 'cleanup_roles');
CREATE TYPE public.gd_notification_outbox_type AS ENUM ('moderation_signal_queued');
CREATE TABLE public.discord_sync_outbox (
    id bigserial PRIMARY KEY,
    action public.gd_discord_sync_action NOT NULL,
    discord_user_id text NOT NULL,
    attempts integer DEFAULT 0 NOT NULL,
    next_attempt_at timestamp with time zone DEFAULT now() NOT NULL,
    processed_at timestamp with time zone,
    last_error text,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);
CREATE INDEX idx_discord_sync_outbox_pending ON public.discord_sync_outbox USING btree (next_attempt_at, id) WHERE (processed_at IS NULL);
CREATE UNIQUE INDEX idx_discord_sync_outbox_pending_unique ON public.discord_sync_outbox USING btree (action, discord_user_id) WHERE (processed_at IS NULL);
CREATE TABLE public.notification_outbox (
    id bigserial PRIMARY KEY,
    type public.gd_notification_outbox_type NOT NULL,
    dedupe_key text NOT NULL UNIQUE,
    payload_json jsonb NOT NULL,
    attempts integer DEFAULT 0 NOT NULL,
    next_attempt_at timestamp with time zone DEFAULT now() NOT NULL,
    sent_at timestamp with time zone,
    last_error text,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);
CREATE INDEX idx_notification_outbox_pending ON public.notification_outbox USING btree (next_attempt_at, id) WHERE (sent_at IS NULL);
CREATE INDEX idx_notification_outbox_sent ON public.notification_outbox USING btree (sent_at) WHERE (sent_at IS NOT NULL);
