-- +goose Up
-- The match lifecycle moves out of Redis. This release ships in a maintenance window: every pod of
-- the previous release is stopped before it runs, so nothing still reads the columns it drops.
--
-- A match's kind is its one description (pkg/matchkind). The gameplay node that runs a match picks
-- it up and stamps it with its id and its lease's fencing token; the node's lease in
-- control_plane_leases is what keeps its matches live, so a match needs no lease of its own.

ALTER TYPE public.gd_match_preset RENAME TO gd_match_kind;
ALTER TABLE public.match_sessions RENAME COLUMN preset_id TO kind;

-- Matches still open from the previous release ended with it.
UPDATE public.match_sessions SET ended_at = now() WHERE ended_at IS NULL;

ALTER TABLE public.match_sessions
    DROP COLUMN mode,
    DROP COLUMN ranked,
    DROP COLUMN source_kind,
    DROP COLUMN state,
    DROP COLUMN source_party_invite_code,
    DROP COLUMN public_route,
    DROP COLUMN lease_expires_at,
    DROP COLUMN updated_at,
    ADD COLUMN node_url text,
    -- What the node needs to run the match: players with their profiles, teams, season.
    ADD COLUMN spec_json jsonb DEFAULT '{}'::jsonb NOT NULL,
    ADD COLUMN outcome text,
    ADD CONSTRAINT match_sessions_outcome_check CHECK (outcome = ANY (ARRAY['finished'::text, 'replaced'::text, 'aborted'::text, 'interrupted'::text]));

UPDATE public.match_sessions SET outcome = 'interrupted' WHERE outcome IS NULL;

DROP TYPE public.gd_match_session_state;

DROP FUNCTION public.gd_match_status(timestamptz, timestamptz);

-- A match is ended once ended_at is set; starting until a node picks it up, which it must within
-- 15 seconds; live while that node's lease holds with the token the match was stamped with; and
-- interrupted otherwise. Nothing has to notice a node dying for its matches to stop being live.
-- +goose StatementBegin
CREATE FUNCTION public.gd_match_status(p_ended_at timestamptz, p_node_id text, p_node_epoch bigint, p_created_at timestamptz) RETURNS text
LANGUAGE sql STABLE PARALLEL SAFE AS
$$ SELECT CASE
     WHEN p_ended_at IS NOT NULL THEN 'ended'
     WHEN p_node_id IS NULL THEN CASE WHEN p_created_at > now() - interval '15 seconds' THEN 'starting' ELSE 'interrupted' END
     WHEN EXISTS (
       SELECT 1 FROM public.control_plane_leases l
       WHERE l.name = 'gameplay-node:' || p_node_id AND l.fencing_token = p_node_epoch AND l.expires_at > now()
     ) THEN 'live'
     ELSE 'interrupted'
   END $$;
-- +goose StatementEnd

-- Matches waiting for a node, oldest first.
CREATE INDEX idx_match_sessions_waiting ON public.match_sessions USING btree (created_at)
  WHERE (node_id IS NULL AND ended_at IS NULL);
-- A node's open matches, for its reconcile.
CREATE INDEX idx_match_sessions_open_node ON public.match_sessions USING btree (node_id)
  WHERE (ended_at IS NULL);
-- Open matches, for the sweep that ends interrupted ones.
CREATE INDEX idx_match_sessions_open ON public.match_sessions USING btree (created_at)
  WHERE (ended_at IS NULL);
CREATE INDEX idx_match_sessions_ended ON public.match_sessions USING btree (ended_at)
  WHERE (ended_at IS NOT NULL);

-- A player is in at most one unfinished match: their seat in it is active until it ends.
ALTER TABLE public.match_participants ADD COLUMN active boolean DEFAULT false NOT NULL;
CREATE UNIQUE INDEX match_participants_one_active ON public.match_participants USING btree (user_id) WHERE active;

DROP TABLE public.runtime_matches;
DROP TYPE public.gd_runtime_state;

-- A party is in a match while its last one is live; nothing stores that. The enum keeps 'in_match'
-- and 'started' because Postgres cannot drop enum values; nothing writes them any more.
UPDATE public.parties
SET state = 'open', last_match_id = coalesce(active_match_id, started_match_id, last_match_id)
WHERE state IN ('in_match', 'started');
ALTER TABLE public.parties DROP COLUMN active_match_id, DROP COLUMN started_match_id;

-- When a member last had the party open, for its presence.
ALTER TABLE public.party_members ADD COLUMN seen_at timestamp with time zone;

-- Players looking for a ranked duel. Losing it in a crash only means queueing again.
CREATE UNLOGGED TABLE public.queue_tickets (
    user_id uuid NOT NULL REFERENCES public.users(id) ON DELETE CASCADE,
    variant text NOT NULL,
    mmr integer NOT NULL,
    joined_at timestamp with time zone DEFAULT now() NOT NULL,
    seen_at timestamp with time zone DEFAULT now() NOT NULL,
    PRIMARY KEY (user_id, variant)
);

-- Fixed-window counters for rate limits. Losing them in a crash only resets the windows.
CREATE UNLOGGED TABLE public.rate_limit_windows (
    key text PRIMARY KEY,
    window_started_at timestamp with time zone NOT NULL,
    hits integer NOT NULL
);

-- Who is online: seen by an open socket within the presence window. Kept apart from users so the
-- frequent writes touch one narrow row; losing it in a crash only shows everyone offline briefly.
CREATE UNLOGGED TABLE public.presence (
    user_id uuid PRIMARY KEY REFERENCES public.users(id) ON DELETE CASCADE,
    seen_at timestamp with time zone NOT NULL
);
CREATE INDEX idx_presence_seen_at ON public.presence USING btree (seen_at);

-- +goose Down
-- Presence, queue tickets, rate-limit windows, party presence and the matches' kinds-only description go;
-- matches open at the time end, and parties come back open.
DROP TABLE public.presence;
DROP TABLE public.rate_limit_windows;
DROP TABLE public.queue_tickets;
ALTER TABLE public.party_members DROP COLUMN seen_at;
ALTER TABLE public.parties ADD COLUMN active_match_id uuid, ADD COLUMN started_match_id uuid;
CREATE INDEX idx_parties_active_match ON public.parties USING btree (active_match_id);
CREATE INDEX idx_parties_started_match ON public.parties USING btree (started_match_id);

CREATE TYPE public.gd_runtime_state AS ENUM ('live', 'ended');
CREATE TABLE public.runtime_matches (
    id uuid NOT NULL PRIMARY KEY,
    state public.gd_runtime_state NOT NULL,
    owner_epoch bigint DEFAULT 0 NOT NULL,
    started_at timestamp with time zone DEFAULT now() NOT NULL,
    ended_at timestamp with time zone
)
WITH (autovacuum_vacuum_scale_factor='0.01', autovacuum_analyze_scale_factor='0.01');

DROP INDEX public.match_participants_one_active;
ALTER TABLE public.match_participants DROP COLUMN active;

DROP INDEX public.idx_match_sessions_ended;
DROP INDEX public.idx_match_sessions_open;
DROP INDEX public.idx_match_sessions_open_node;
DROP INDEX public.idx_match_sessions_waiting;

DROP FUNCTION public.gd_match_status(timestamptz, text, bigint, timestamptz);
-- +goose StatementBegin
CREATE FUNCTION public.gd_match_status(p_ended_at timestamptz, p_lease_expires_at timestamptz) RETURNS text
LANGUAGE sql STABLE PARALLEL SAFE AS
$$ SELECT CASE
     WHEN p_ended_at IS NOT NULL THEN 'ended'
     WHEN p_lease_expires_at > now() THEN 'live'
     ELSE 'interrupted'
   END $$;
-- +goose StatementEnd

UPDATE public.match_sessions SET ended_at = now() WHERE ended_at IS NULL;
CREATE TYPE public.gd_match_session_state AS ENUM ('waiting', 'live', 'ended');
ALTER TABLE public.match_sessions
    DROP CONSTRAINT match_sessions_outcome_check,
    DROP COLUMN outcome,
    DROP COLUMN spec_json,
    DROP COLUMN node_url,
    ADD COLUMN updated_at timestamp with time zone DEFAULT now() NOT NULL,
    ADD COLUMN lease_expires_at timestamp with time zone,
    ADD COLUMN public_route text,
    ADD COLUMN source_party_invite_code text,
    ADD COLUMN state public.gd_match_session_state DEFAULT 'ended'::public.gd_match_session_state NOT NULL,
    ADD COLUMN source_kind public.gd_match_source DEFAULT 'queue'::public.gd_match_source NOT NULL,
    ADD COLUMN ranked boolean DEFAULT false NOT NULL,
    ADD COLUMN mode public.gd_match_mode DEFAULT 'duel'::public.gd_match_mode NOT NULL;
ALTER TABLE public.match_sessions RENAME COLUMN kind TO preset_id;
ALTER TYPE public.gd_match_kind RENAME TO gd_match_preset;
UPDATE public.match_sessions SET
    mode = CASE preset_id WHEN 'solo' THEN 'singleplayer'::public.gd_match_mode WHEN 'team_duel' THEN 'team_duel'::public.gd_match_mode
        WHEN 'free_for_all' THEN 'free_for_all'::public.gd_match_mode ELSE 'duel'::public.gd_match_mode END,
    ranked = preset_id = 'ranked_duel',
    source_kind = CASE WHEN preset_id = 'solo' THEN 'solo'::public.gd_match_source WHEN preset_id = 'ranked_duel' THEN 'queue'::public.gd_match_source
        ELSE 'party'::public.gd_match_source END;
CREATE INDEX idx_match_sessions_expired_lease ON public.match_sessions USING btree (lease_expires_at, match_id) WHERE (state = 'live'::public.gd_match_session_state);
CREATE INDEX idx_match_sessions_state ON public.match_sessions USING btree (state, updated_at DESC);
CREATE INDEX idx_match_sessions_unended_lease ON public.match_sessions USING btree (lease_expires_at) WHERE (ended_at IS NULL);
