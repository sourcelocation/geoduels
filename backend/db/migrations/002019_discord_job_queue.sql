-- +goose Up
-- Discord jobs move to a queue of their own, so the Discord worker stops taking the
-- moderation worker's jobs from the default queue (and the other way round). Move
-- the ones still waiting. On a new database River's tables come after this, and
-- there's nothing to move.
-- +goose StatementBegin
DO $$
BEGIN
    IF to_regclass('public.river_job') IS NOT NULL THEN
        UPDATE public.river_job SET queue = 'discord'
        WHERE kind IN ('discord_sync', 'discord_sync_all') AND queue = 'default'
          AND state IN ('available', 'pending', 'retryable', 'scheduled');
    END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DO $$
BEGIN
    IF to_regclass('public.river_job') IS NOT NULL THEN
        UPDATE public.river_job SET queue = 'default'
        WHERE kind IN ('discord_sync', 'discord_sync_all') AND queue = 'discord'
          AND state IN ('available', 'pending', 'retryable', 'scheduled');
    END IF;
END $$;
-- +goose StatementEnd
