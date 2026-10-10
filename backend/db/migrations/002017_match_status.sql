-- +goose Up
-- A match's status is derived, never stored. It ended once ended_at is set, which finalization
-- writes with the match's history; it is live while its gameplay node keeps renewing the lease; and
-- it was interrupted once the lease lapsed with no end written (its node is gone). Nothing has to
-- notice a node dying for its matches to stop being live.
-- +goose StatementBegin
CREATE FUNCTION public.gd_match_status(p_ended_at timestamptz, p_lease_expires_at timestamptz) RETURNS text
LANGUAGE sql STABLE PARALLEL SAFE AS
$$ SELECT CASE
     WHEN p_ended_at IS NOT NULL THEN 'ended'
     WHEN p_lease_expires_at > now() THEN 'live'
     ELSE 'interrupted'
   END $$;
-- +goose StatementEnd

-- Matches without an end, by when their lease lapses: parties and cleanup look for lapsed ones.
CREATE INDEX idx_match_sessions_unended_lease ON public.match_sessions USING btree (lease_expires_at)
  WHERE (ended_at IS NULL);

-- Ends that were recorded only in the legacy state columns.
UPDATE public.match_sessions SET ended_at = updated_at
WHERE ended_at IS NULL AND state = 'ended';

UPDATE public.match_sessions ms SET ended_at = rm.ended_at
FROM public.runtime_matches rm
WHERE rm.id = ms.match_id AND rm.state = 'ended' AND rm.ended_at IS NOT NULL AND ms.ended_at IS NULL;

-- +goose Down
-- The backfilled ended_at values stay: they agree with the legacy state columns.
DROP INDEX public.idx_match_sessions_unended_lease;
DROP FUNCTION public.gd_match_status(timestamptz, timestamptz);
