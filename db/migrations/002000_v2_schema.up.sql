-- GeoDuels v2 fresh-install schema.
--
-- Version 2000 is the convergence point between a fresh v2 database and an
-- installation upgraded through db/migrations-legacy. This file defines the
-- final v2 schema directly; it must never contain transitional data rewrites.
-- Existing installations below version 2000 must use the legacy migration path.

-- Domain types

CREATE TYPE public.gd_account_type AS ENUM (
    'guest',
    'registered'
);

CREATE TYPE public.gd_chat_audience AS ENUM (
    'all',
    'team'
);

CREATE TYPE public.gd_chat_kind AS ENUM (
    'text',
    'emote'
);

CREATE TYPE public.gd_chat_scope AS ENUM (
    'party',
    'match'
);

CREATE TYPE public.gd_comment_status AS ENUM (
    'visible',
    'deleted',
    'moderated'
);

CREATE TYPE public.gd_discord_sync_action AS ENUM (
    'sync',
    'cleanup_roles'
);

CREATE TYPE public.gd_evidence_strength AS ENUM (
    'weak',
    'limited',
    'substantial',
    'strong'
);

CREATE TYPE public.gd_map_difficulty AS ENUM (
    'easy',
    'normal',
    'hard'
);

CREATE TYPE public.gd_map_status AS ENUM (
    'processing',
    'ready',
    'rejected',
    'archived'
);

CREATE TYPE public.gd_map_visibility AS ENUM (
    'private',
    'unlisted',
    'public'
);

CREATE TYPE public.gd_match_mode AS ENUM (
    'duel',
    'singleplayer',
    'team_duel',
    'free_for_all'
);

CREATE TYPE public.gd_match_preset AS ENUM (
    'ranked_duel',
    'private_duel',
    'team_duel',
    'free_for_all',
    'solo'
);

CREATE TYPE public.gd_match_session_state AS ENUM (
    'waiting',
    'live',
    'ended'
);

CREATE TYPE public.gd_match_source AS ENUM (
    'queue',
    'party',
    'solo'
);

CREATE TYPE public.gd_moderation_log_action AS ENUM (
    'role_granted',
    'role_revoked',
    'permanent_ban',
    'temporary_ban',
    'warning',
    'chat_mute',
    'report_mute',
    'report_unmute',
    'unban',
    'refund',
    'note',
    'assign',
    'status',
    'dismiss',
    'mark_inconclusive',
    'abusive_reports',
    'badge_granted'
);

CREATE TYPE public.gd_moderation_outcome AS ENUM (
    'confirmed',
    'dismissed',
    'inconclusive',
    'abusive_report'
);

CREATE TYPE public.gd_moderation_severity AS ENUM (
    'low',
    'medium',
    'high',
    'critical'
);

CREATE TYPE public.gd_moderation_source AS ENUM (
    'player_report',
    'risk_engine',
    'moderator',
    'system'
);

CREATE TYPE public.gd_notification_category AS ENUM (
    'system',
    'social',
    'moderation',
    'match',
    'map'
);

CREATE TYPE public.gd_notification_entity_kind AS ENUM (
    'friend_request',
    'friendship',
    'party_invitation',
    'badge',
    'match',
    'map',
    'comment',
    'player'
);

CREATE TYPE public.gd_notification_outbox_type AS ENUM (
    'moderation_signal_queued'
);

CREATE TYPE public.gd_notification_type AS ENUM (
    'friend_request_received',
    'friendship_accepted',
    'party_invitation_received',
    'badge_unlocked',
    'account_banned',
    'account_unbanned',
    'reported_player_banned',
    'mmr_refund'
);

CREATE TYPE public.gd_oauth_provider AS ENUM (
    'google',
    'discord'
);

CREATE TYPE public.gd_party_role AS ENUM (
    'owner',
    'member'
);

CREATE TYPE public.gd_party_state AS ENUM (
    'open',
    'in_match',
    'started',
    'closed',
    'expired'
);

CREATE TYPE public.gd_ruleset AS ENUM (
    'moving',
    'no_move',
    'nmpz'
);

CREATE TYPE public.gd_runtime_state AS ENUM (
    'live',
    'ended'
);

CREATE TYPE public.gd_social_request_status AS ENUM (
    'pending',
    'accepted',
    'declined',
    'cancelled',
    'expired'
);

CREATE TYPE public.gd_team_id AS ENUM (
    'a',
    'b'
);

CREATE TYPE public.gd_user_event_type AS ENUM (
    'friendship.created',
    'friend_request.created',
    'party_invitation.created'
);

SET default_tablespace = '';

SET default_table_access_method = heap;

-- Tables and owned sequences

CREATE TABLE public.auth_sessions (
    id uuid NOT NULL,
    user_id uuid NOT NULL,
    refresh_token_hash text NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    last_used_at timestamp with time zone DEFAULT now() NOT NULL,
    revoked_at timestamp with time zone,
    user_agent text,
    ip_address text
)
WITH (autovacuum_vacuum_scale_factor='0.02', autovacuum_analyze_scale_factor='0.01');

CREATE TABLE public.changelog_posts (
    id bigint NOT NULL,
    slug text NOT NULL,
    title text NOT NULL,
    markdown text DEFAULT ''::text NOT NULL,
    published boolean DEFAULT true NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);

CREATE SEQUENCE public.changelog_posts_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE public.changelog_posts_id_seq OWNED BY public.changelog_posts.id;

CREATE TABLE public.chat_conversations (
    id uuid NOT NULL,
    scope_kind public.gd_chat_scope NOT NULL,
    scope_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);

CREATE TABLE public.chat_messages (
    id uuid NOT NULL,
    conversation_id uuid NOT NULL,
    sender_user_id uuid NOT NULL,
    sender_display_name text NOT NULL,
    kind public.gd_chat_kind NOT NULL,
    body text,
    emote text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    audience public.gd_chat_audience DEFAULT 'all'::public.gd_chat_audience NOT NULL,
    team_id public.gd_team_id,
    team_match_id uuid,
    CONSTRAINT chat_messages_audience_check CHECK ((audience = ANY (ARRAY['all'::public.gd_chat_audience, 'team'::public.gd_chat_audience]))),
    CONSTRAINT chat_messages_team_audience_check CHECK ((((audience = 'all'::public.gd_chat_audience) AND (team_id IS NULL) AND (team_match_id IS NULL)) OR ((audience = 'team'::public.gd_chat_audience) AND (team_match_id IS NOT NULL) AND (team_id IS NOT NULL)))),
    CONSTRAINT chat_messages_text_body_check CHECK ((((kind = 'text'::public.gd_chat_kind) AND (body IS NOT NULL) AND (length(body) > 0) AND (emote IS NULL)) OR ((kind = 'emote'::public.gd_chat_kind) AND (emote = ANY (ARRAY['skull'::text, 'sob'::text, 'thinking'::text, 'sunglasses'::text])) AND (body IS NULL))))
)
WITH (autovacuum_vacuum_scale_factor='0.02', autovacuum_analyze_scale_factor='0.01');

CREATE TABLE public.control_plane_leases (
    name text NOT NULL,
    owner_id text NOT NULL,
    fencing_token bigint DEFAULT 1 NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT control_plane_leases_name_check CHECK ((name <> ''::text)),
    CONSTRAINT control_plane_leases_owner_check CHECK ((owner_id <> ''::text))
);

CREATE TABLE public.discord_sync_outbox (
    id bigint NOT NULL,
    action public.gd_discord_sync_action NOT NULL,
    discord_user_id text NOT NULL,
    attempts integer DEFAULT 0 NOT NULL,
    next_attempt_at timestamp with time zone DEFAULT now() NOT NULL,
    processed_at timestamp with time zone,
    last_error text,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);

CREATE SEQUENCE public.discord_sync_outbox_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE public.discord_sync_outbox_id_seq OWNED BY public.discord_sync_outbox.id;

CREATE TABLE public.elo_refunds (
    id bigint NOT NULL,
    user_id uuid NOT NULL,
    match_id uuid NOT NULL,
    cheater_user_id uuid NOT NULL,
    original_delta integer NOT NULL,
    refund_delta integer NOT NULL,
    reason text DEFAULT 'cheating_verdict'::text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    victim_mmr_before integer,
    victim_mmr_after integer,
    computed_refund_delta integer,
    notification_id bigint,
    created_by_reason text,
    CONSTRAINT elo_refunds_positive_refund CHECK ((refund_delta > 0))
);

CREATE SEQUENCE public.elo_refunds_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE public.elo_refunds_id_seq OWNED BY public.elo_refunds.id;

CREATE TABLE public.friend_codes (
    code character(6) NOT NULL,
    user_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    revoked_at timestamp with time zone
);

CREATE TABLE public.friend_requests (
    id uuid NOT NULL,
    sender_user_id uuid NOT NULL,
    recipient_user_id uuid NOT NULL,
    status public.gd_social_request_status DEFAULT 'pending'::public.gd_social_request_status NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    responded_at timestamp with time zone,
    expires_at timestamp with time zone NOT NULL,
    CONSTRAINT friend_requests_distinct_users CHECK ((sender_user_id <> recipient_user_id))
);

CREATE TABLE public.friendships (
    user_id_low uuid NOT NULL,
    user_id_high uuid NOT NULL,
    created_from_request_id uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT friendships_canonical_pair CHECK ((user_id_low < user_id_high))
);

CREATE TABLE public.ip_signup_bans (
    id bigint NOT NULL,
    ip_address text NOT NULL,
    reason text,
    created_by uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    revoked_at timestamp with time zone,
    cidr inet,
    reason_code text,
    expires_at timestamp with time zone
);

CREATE SEQUENCE public.ip_signup_bans_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE public.ip_signup_bans_id_seq OWNED BY public.ip_signup_bans.id;

CREATE TABLE public.locations (
    map_storage_id integer NOT NULL,
    lat_e7 integer NOT NULL,
    lng_e7 integer NOT NULL,
    rand_key_i integer NOT NULL,
    heading_cdeg smallint,
    pitch_cdeg smallint,
    country text,
    pano_id text
);

CREATE TABLE public.map_aliases (
    alias text NOT NULL,
    map_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);

CREATE TABLE public.map_comment_likes (
    comment_id uuid NOT NULL,
    user_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);

CREATE TABLE public.map_comments (
    id uuid NOT NULL,
    parent_id uuid,
    user_id uuid NOT NULL,
    body text,
    status public.gd_comment_status DEFAULT 'visible'::public.gd_comment_status NOT NULL,
    deleted_by uuid,
    deleted_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    like_count integer DEFAULT 0 NOT NULL,
    map_id uuid NOT NULL,
    CONSTRAINT map_comments_like_count_check CHECK ((like_count >= 0)),
    CONSTRAINT map_comments_state_check CHECK ((((status = 'visible'::public.gd_comment_status) AND (body IS NOT NULL) AND ((length(btrim(body)) >= 1) AND (length(btrim(body)) <= 1000)) AND (deleted_by IS NULL) AND (deleted_at IS NULL)) OR ((status = ANY (ARRAY['deleted'::public.gd_comment_status, 'moderated'::public.gd_comment_status])) AND (body IS NULL) AND (deleted_at IS NOT NULL))))
);

CREATE TABLE public.map_country_stats (
    map_id uuid NOT NULL,
    country text NOT NULL,
    location_count integer NOT NULL
);

CREATE TABLE public.map_daily_users (
    day date NOT NULL,
    user_id uuid NOT NULL,
    played boolean DEFAULT false NOT NULL,
    favorited boolean DEFAULT false NOT NULL,
    commented boolean DEFAULT false NOT NULL,
    map_id uuid NOT NULL
);

CREATE TABLE public.map_favorites (
    user_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    map_id uuid NOT NULL
);

CREATE TABLE public.map_stats_daily (
    day date NOT NULL,
    plays integer DEFAULT 0 NOT NULL,
    favorites integer DEFAULT 0 NOT NULL,
    comments integer DEFAULT 0 NOT NULL,
    unique_players integer DEFAULT 0 NOT NULL,
    unique_favoriters integer DEFAULT 0 NOT NULL,
    unique_commenters integer DEFAULT 0 NOT NULL,
    map_id uuid NOT NULL
);

CREATE TABLE public.map_upload_events (
    id bigint NOT NULL,
    user_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    location_count integer DEFAULT 0 NOT NULL,
    map_id uuid,
    CONSTRAINT map_upload_events_location_count_check CHECK ((location_count >= 0))
);

CREATE SEQUENCE public.map_upload_events_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE public.map_upload_events_id_seq OWNED BY public.map_upload_events.id;

CREATE TABLE public.maps (
    display_name text NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    owner_user_id uuid,
    description text DEFAULT ''::text NOT NULL,
    visibility public.gd_map_visibility DEFAULT 'private'::public.gd_map_visibility NOT NULL,
    status public.gd_map_status DEFAULT 'ready'::public.gd_map_status NOT NULL,
    location_count integer DEFAULT 0 NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    archived_at timestamp with time zone,
    difficulty public.gd_map_difficulty DEFAULT 'normal'::public.gd_map_difficulty NOT NULL,
    thumbnail_variant integer DEFAULT 1 NOT NULL,
    published_at timestamp with time zone,
    play_count integer DEFAULT 0 NOT NULL,
    favorite_count integer DEFAULT 0 NOT NULL,
    comment_count integer DEFAULT 0 NOT NULL,
    trending_score double precision DEFAULT 0 NOT NULL,
    official_region_type text DEFAULT ''::text NOT NULL,
    official_region_code text DEFAULT ''::text NOT NULL,
    thumbnail_key text DEFAULT 'generic/variant-1'::text NOT NULL,
    official_at timestamp with time zone,
    official_by uuid,
    id uuid NOT NULL,
    storage_id integer NOT NULL,
    content_hash bytea,
    rejected_location_count integer DEFAULT 0 NOT NULL,
    CONSTRAINT maps_thumbnail_key_check CHECK ((thumbnail_key ~ '^[a-z0-9][a-z0-9-]*/[a-z0-9][a-z0-9-]*$'::text)),
    CONSTRAINT maps_thumbnail_variant_check CHECK (((thumbnail_variant >= 1) AND (thumbnail_variant <= 5)))
);

ALTER TABLE public.maps ALTER COLUMN storage_id ADD GENERATED ALWAYS AS IDENTITY (
    SEQUENCE NAME public.maps_storage_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);

CREATE TABLE public.match_history (
    match_id uuid NOT NULL,
    mode public.gd_match_mode NOT NULL,
    started_at timestamp with time zone DEFAULT now() NOT NULL,
    ended_at timestamp with time zone DEFAULT now() NOT NULL,
    winner_user_id uuid,
    ranked boolean DEFAULT false NOT NULL,
    source_kind public.gd_match_source DEFAULT 'queue'::public.gd_match_source NOT NULL,
    ruleset public.gd_ruleset,
    replay_json jsonb,
    source_party_id uuid,
    replay_zstd bytea,
    replay_codec smallint,
    replay_schema_version smallint,
    replay_uncompressed_bytes integer,
    replay_sha256 bytea,
    replay_expires_at timestamp with time zone,
    round_count smallint DEFAULT 0 NOT NULL,
    map_id uuid,
    CONSTRAINT match_history_replay_codec_check CHECK (((replay_codec IS NULL) OR (replay_codec = 1)))
)
WITH (autovacuum_vacuum_scale_factor='0.02', autovacuum_analyze_scale_factor='0.01');

CREATE TABLE public.match_participants (
    match_id uuid NOT NULL,
    user_id uuid NOT NULL,
    team_id text,
    display_name text DEFAULT ''::text NOT NULL,
    avatar_url text DEFAULT ''::text NOT NULL,
    joined_party_at timestamp with time zone
);

CREATE TABLE public.match_players (
    match_id uuid NOT NULL,
    user_id uuid NOT NULL,
    display_name text NOT NULL,
    mmr integer DEFAULT 0 NOT NULL,
    hp integer DEFAULT 0 NOT NULL,
    rating_rd double precision,
    final_ranked_delta integer,
    rating_before integer,
    rating_after integer,
    total_score integer DEFAULT 0 NOT NULL,
    ended_at timestamp with time zone NOT NULL
);

CREATE TABLE public.match_round_plans (
    match_id uuid NOT NULL,
    round_index integer NOT NULL,
    lat double precision NOT NULL,
    lng double precision NOT NULL,
    country text,
    pano_id text,
    heading double precision,
    pitch double precision,
    map_id uuid NOT NULL,
    CONSTRAINT match_round_plans_round_index_check CHECK ((round_index >= 0))
);

CREATE TABLE public.match_sessions (
    match_id uuid NOT NULL,
    preset_id public.gd_match_preset NOT NULL,
    mode public.gd_match_mode NOT NULL,
    state public.gd_match_session_state DEFAULT 'live'::public.gd_match_session_state NOT NULL,
    ranked boolean DEFAULT false NOT NULL,
    source_kind public.gd_match_source DEFAULT 'queue'::public.gd_match_source NOT NULL,
    source_party_id uuid,
    source_party_invite_code text,
    node_id text,
    node_epoch bigint,
    public_route text,
    config_json jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    started_at timestamp with time zone DEFAULT now() NOT NULL,
    ended_at timestamp with time zone,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    map_id uuid,
    lease_expires_at timestamp with time zone,
    return_target_kind text DEFAULT 'home'::text NOT NULL,
    return_target_map_id uuid,
    return_target_party_id uuid,
    CONSTRAINT match_sessions_return_target_kind_check CHECK ((return_target_kind = ANY (ARRAY['home'::text, 'map'::text, 'party'::text])))
);

CREATE TABLE public.moderation_log (
    id bigint NOT NULL,
    subject_user_id uuid,
    actor_user_id uuid,
    action public.gd_moderation_log_action NOT NULL,
    reason text,
    expires_at timestamp with time zone,
    signal_ids bigint[] DEFAULT '{}'::bigint[] NOT NULL,
    metadata jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);

CREATE SEQUENCE public.moderation_log_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE public.moderation_log_id_seq OWNED BY public.moderation_log.id;

CREATE TABLE public.moderation_signals (
    id bigint NOT NULL,
    subject_user_id uuid NOT NULL,
    signal_type text NOT NULL,
    source public.gd_moderation_source NOT NULL,
    severity public.gd_moderation_severity NOT NULL,
    evidence_strength public.gd_evidence_strength NOT NULL,
    detector_key text,
    detector_version text,
    reason_code text NOT NULL,
    score double precision DEFAULT 0 NOT NULL,
    recommended_queue boolean DEFAULT false NOT NULL,
    reporter_user_id uuid,
    match_id uuid,
    payload_json jsonb DEFAULT '{}'::jsonb NOT NULL,
    occurred_at timestamp with time zone DEFAULT now() NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    reviewed_at timestamp with time zone,
    reviewed_by uuid,
    outcome public.gd_moderation_outcome,
    CONSTRAINT moderation_signals_no_self_report CHECK (((reporter_user_id IS NULL) OR (reporter_user_id <> subject_user_id)))
);

CREATE SEQUENCE public.moderation_signals_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE public.moderation_signals_id_seq OWNED BY public.moderation_signals.id;

CREATE TABLE public.notification_outbox (
    id bigint NOT NULL,
    type public.gd_notification_outbox_type NOT NULL,
    dedupe_key text NOT NULL,
    payload_json jsonb NOT NULL,
    attempts integer DEFAULT 0 NOT NULL,
    next_attempt_at timestamp with time zone DEFAULT now() NOT NULL,
    sent_at timestamp with time zone,
    last_error text,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);

CREATE SEQUENCE public.notification_outbox_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE public.notification_outbox_id_seq OWNED BY public.notification_outbox.id;

CREATE TABLE public.oauth_identity_bans (
    provider public.gd_oauth_provider NOT NULL,
    provider_user_id text NOT NULL,
    banned_user_id uuid,
    reason text,
    created_by uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    revoked_at timestamp with time zone
);

CREATE TABLE public.parties (
    id uuid NOT NULL,
    invite_code text NOT NULL,
    owner_user_id uuid NOT NULL,
    state public.gd_party_state DEFAULT 'open'::public.gd_party_state NOT NULL,
    mode public.gd_match_mode DEFAULT 'duel'::public.gd_match_mode NOT NULL,
    map_scope text DEFAULT 'world'::text NOT NULL,
    active_match_id uuid,
    started_match_id uuid,
    last_match_id uuid,
    config_json jsonb DEFAULT '{"ruleset": "moving", "roundTimerMode": "none", "pressureTimeLimitMs": 15000}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    map_id uuid
);

CREATE TABLE public.party_invitations (
    id uuid NOT NULL,
    party_id uuid NOT NULL,
    inviter_user_id uuid NOT NULL,
    recipient_user_id uuid NOT NULL,
    status public.gd_social_request_status DEFAULT 'pending'::public.gd_social_request_status NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    expires_at timestamp with time zone NOT NULL,
    responded_at timestamp with time zone,
    CONSTRAINT party_invitations_distinct_users CHECK ((inviter_user_id <> recipient_user_id))
);

CREATE TABLE public.party_members (
    party_id uuid NOT NULL,
    user_id uuid NOT NULL,
    role public.gd_party_role DEFAULT 'member'::public.gd_party_role NOT NULL,
    ready boolean DEFAULT false NOT NULL,
    team_id public.gd_team_id,
    joined_at timestamp with time zone DEFAULT now() NOT NULL,
    left_at timestamp with time zone
);

CREATE TABLE public.player_map_bests (
    user_id uuid NOT NULL,
    map_id uuid NOT NULL,
    ruleset smallint NOT NULL,
    best_score smallint NOT NULL,
    match_id uuid NOT NULL,
    achieved_at timestamp with time zone NOT NULL,
    CONSTRAINT player_map_bests_best_score_check CHECK (((best_score >= 0) AND (best_score <= 25000))),
    CONSTRAINT player_map_bests_ruleset_check CHECK ((ruleset = ANY (ARRAY[0, 1])))
);

CREATE TABLE public.ranked_guess_events (
    user_id uuid NOT NULL,
    match_id uuid NOT NULL,
    round_number smallint NOT NULL,
    score smallint NOT NULL,
    guess_ms integer NOT NULL,
    evidence real DEFAULT 0 NOT NULL,
    occurred_at timestamp with time zone NOT NULL
)
WITH (autovacuum_vacuum_scale_factor='0.02', autovacuum_analyze_scale_factor='0.01');

CREATE TABLE public.ranked_stats (
    user_id uuid NOT NULL,
    mode public.gd_match_mode NOT NULL,
    season_id text NOT NULL,
    games_played integer DEFAULT 0 NOT NULL,
    wins integer DEFAULT 0 NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);

CREATE TABLE public.ranks (
    user_id uuid NOT NULL,
    mode public.gd_match_mode NOT NULL,
    mmr integer DEFAULT 500 NOT NULL,
    season_id text NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    rd double precision DEFAULT 350 NOT NULL
);

CREATE TABLE public.runtime_matches (
    id uuid NOT NULL,
    state public.gd_runtime_state NOT NULL,
    owner_epoch bigint DEFAULT 0 NOT NULL,
    started_at timestamp with time zone DEFAULT now() NOT NULL,
    ended_at timestamp with time zone
)
WITH (autovacuum_vacuum_scale_factor='0.01', autovacuum_analyze_scale_factor='0.01');

CREATE TABLE public.site_settings (
    key text NOT NULL,
    value_json jsonb NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);

CREATE TABLE public.support_donation_refs (
    ref text NOT NULL,
    user_id uuid NOT NULL,
    completed_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);

CREATE TABLE public.user_badges (
    user_id uuid NOT NULL,
    awarded_at timestamp with time zone DEFAULT now() NOT NULL,
    badge_code smallint NOT NULL,
    level smallint DEFAULT 1 NOT NULL,
    extra smallint,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT user_badges_level_check CHECK (level >= 1),
    CONSTRAINT user_badges_extra_check CHECK (extra IS NULL OR extra >= 0)
);

CREATE TABLE public.user_blocks (
    blocker_user_id uuid NOT NULL,
    blocked_user_id uuid NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT user_blocks_distinct_users CHECK ((blocker_user_id <> blocked_user_id))
);

CREATE TABLE public.user_event_sequences (
    user_id uuid NOT NULL,
    sequence bigint DEFAULT 0 NOT NULL
);

CREATE TABLE public.user_events (
    user_id uuid NOT NULL,
    sequence bigint NOT NULL,
    type public.gd_user_event_type NOT NULL,
    payload_json jsonb DEFAULT '{}'::jsonb NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);

CREATE TABLE public.user_identities (
    user_id uuid NOT NULL,
    provider public.gd_oauth_provider NOT NULL,
    provider_user_id text NOT NULL,
    email text,
    provider_name text,
    avatar_url text,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    last_seen_at timestamp with time zone DEFAULT now() NOT NULL
);

CREATE TABLE public.user_identity_history (
    user_id uuid NOT NULL,
    provider public.gd_oauth_provider NOT NULL,
    provider_user_id text NOT NULL,
    email text,
    provider_name text,
    first_seen_at timestamp with time zone DEFAULT now() NOT NULL,
    last_seen_at timestamp with time zone DEFAULT now() NOT NULL,
    deleted_at timestamp with time zone
);

CREATE TABLE public.user_notifications (
    id bigint NOT NULL,
    user_id uuid NOT NULL,
    type public.gd_notification_type NOT NULL,
    dedupe_key text NOT NULL,
    payload_json jsonb DEFAULT '{}'::jsonb NOT NULL,
    read_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    category public.gd_notification_category DEFAULT 'system'::public.gd_notification_category NOT NULL,
    actor_user_id uuid,
    entity_kind public.gd_notification_entity_kind,
    entity_id text,
    archived_at timestamp with time zone,
    expires_at timestamp with time zone
);

CREATE SEQUENCE public.user_notifications_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER SEQUENCE public.user_notifications_id_seq OWNED BY public.user_notifications.id;

CREATE TABLE public.user_preferences (
    user_id uuid NOT NULL,
    schema_version integer DEFAULT 1 NOT NULL,
    preferences_json jsonb DEFAULT '{}'::jsonb NOT NULL,
    revision bigint DEFAULT 0 NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT user_preferences_revision_check CHECK ((revision >= 0)),
    CONSTRAINT user_preferences_schema_version_check CHECK ((schema_version > 0))
);

CREATE TABLE public.user_stats (
    user_id uuid NOT NULL,
    games_played integer DEFAULT 0 NOT NULL,
    wins integer DEFAULT 0 NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);

CREATE TABLE public.users (
    id uuid NOT NULL,
    email text,
    display_name text NOT NULL,
    avatar_url text,
    account_type public.gd_account_type DEFAULT 'registered'::public.gd_account_type NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    is_admin boolean DEFAULT false NOT NULL,
    banned_at timestamp with time zone,
    ban_reason text,
    is_moderator boolean DEFAULT false NOT NULL,
    registration_ip_address text,
    deleted_at timestamp with time zone,
    selected_badge_code smallint,
    nickname_claimed_at timestamp with time zone,
    map_creator_tier smallint DEFAULT 0 NOT NULL,
    map_creator_tier_override smallint,
    map_creator_qualified_favorites integer DEFAULT 0 NOT NULL,
    map_creator_qualified_maps integer DEFAULT 0 NOT NULL,
    map_creator_trust_updated_at timestamp with time zone,
    ban_expires_at timestamp with time zone,
    last_seen_at timestamp with time zone,
    social_discoverable boolean DEFAULT true NOT NULL,
    social_presence_visible boolean DEFAULT true NOT NULL,
    social_requests_enabled boolean DEFAULT true NOT NULL,
    social_party_invites_enabled boolean DEFAULT true NOT NULL,
    chat_muted_at timestamp with time zone,
    chat_mute_reason text,
    chat_mute_expires_at timestamp with time zone,
    report_muted_at timestamp with time zone,
    report_mute_reason text,
    report_mute_expires_at timestamp with time zone,
    CONSTRAINT users_claimed_nickname_format_check CHECK (((nickname_claimed_at IS NULL) OR ((account_type = 'registered'::public.gd_account_type) AND (display_name ~ '^[A-Za-z0-9._]{2,14}$'::text) AND (POSITION(('..'::text) IN (display_name)) = 0) AND (POSITION(('__'::text) IN (display_name)) = 0)))),
    CONSTRAINT users_map_creator_tier_check CHECK (((map_creator_tier >= 0) AND (map_creator_tier <= 2))),
    CONSTRAINT users_map_creator_tier_override_check CHECK (((map_creator_tier_override IS NULL) OR ((map_creator_tier_override >= 0) AND (map_creator_tier_override <= 2))))
);

-- Sequence-backed column defaults

ALTER TABLE ONLY public.changelog_posts ALTER COLUMN id SET DEFAULT nextval('public.changelog_posts_id_seq'::regclass);

ALTER TABLE ONLY public.discord_sync_outbox ALTER COLUMN id SET DEFAULT nextval('public.discord_sync_outbox_id_seq'::regclass);

ALTER TABLE ONLY public.elo_refunds ALTER COLUMN id SET DEFAULT nextval('public.elo_refunds_id_seq'::regclass);

ALTER TABLE ONLY public.ip_signup_bans ALTER COLUMN id SET DEFAULT nextval('public.ip_signup_bans_id_seq'::regclass);

ALTER TABLE ONLY public.map_upload_events ALTER COLUMN id SET DEFAULT nextval('public.map_upload_events_id_seq'::regclass);

ALTER TABLE ONLY public.moderation_log ALTER COLUMN id SET DEFAULT nextval('public.moderation_log_id_seq'::regclass);

ALTER TABLE ONLY public.moderation_signals ALTER COLUMN id SET DEFAULT nextval('public.moderation_signals_id_seq'::regclass);

ALTER TABLE ONLY public.notification_outbox ALTER COLUMN id SET DEFAULT nextval('public.notification_outbox_id_seq'::regclass);

ALTER TABLE ONLY public.user_notifications ALTER COLUMN id SET DEFAULT nextval('public.user_notifications_id_seq'::regclass);

-- Primary keys and unique constraints

ALTER TABLE ONLY public.auth_sessions
    ADD CONSTRAINT auth_sessions_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.auth_sessions
    ADD CONSTRAINT auth_sessions_refresh_token_hash_key UNIQUE (refresh_token_hash);

ALTER TABLE ONLY public.changelog_posts
    ADD CONSTRAINT changelog_posts_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.changelog_posts
    ADD CONSTRAINT changelog_posts_slug_key UNIQUE (slug);

ALTER TABLE ONLY public.chat_conversations
    ADD CONSTRAINT chat_conversations_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.chat_messages
    ADD CONSTRAINT chat_messages_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.control_plane_leases
    ADD CONSTRAINT control_plane_leases_pkey PRIMARY KEY (name);

ALTER TABLE ONLY public.discord_sync_outbox
    ADD CONSTRAINT discord_sync_outbox_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.elo_refunds
    ADD CONSTRAINT elo_refunds_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.elo_refunds
    ADD CONSTRAINT elo_refunds_user_id_match_id_cheater_user_id_key UNIQUE (user_id, match_id, cheater_user_id);

ALTER TABLE ONLY public.friend_codes
    ADD CONSTRAINT friend_codes_pkey PRIMARY KEY (code);

ALTER TABLE ONLY public.friend_requests
    ADD CONSTRAINT friend_requests_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.friendships
    ADD CONSTRAINT friendships_pkey PRIMARY KEY (user_id_low, user_id_high);

ALTER TABLE ONLY public.ip_signup_bans
    ADD CONSTRAINT ip_signup_bans_ip_address_key UNIQUE (ip_address);

ALTER TABLE ONLY public.ip_signup_bans
    ADD CONSTRAINT ip_signup_bans_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.map_aliases
    ADD CONSTRAINT map_aliases_pkey PRIMARY KEY (alias);

ALTER TABLE ONLY public.map_comment_likes
    ADD CONSTRAINT map_comment_likes_pkey PRIMARY KEY (comment_id, user_id);

ALTER TABLE ONLY public.map_comments
    ADD CONSTRAINT map_comments_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.map_country_stats
    ADD CONSTRAINT map_country_stats_pkey PRIMARY KEY (map_id, country);

ALTER TABLE ONLY public.map_daily_users
    ADD CONSTRAINT map_daily_users_pkey PRIMARY KEY (map_id, day, user_id);

ALTER TABLE ONLY public.map_favorites
    ADD CONSTRAINT map_favorites_pkey PRIMARY KEY (map_id, user_id);

ALTER TABLE ONLY public.map_stats_daily
    ADD CONSTRAINT map_stats_daily_pkey PRIMARY KEY (map_id, day);

ALTER TABLE ONLY public.map_upload_events
    ADD CONSTRAINT map_upload_events_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.maps
    ADD CONSTRAINT maps_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.match_history
    ADD CONSTRAINT match_history_pkey PRIMARY KEY (match_id);

ALTER TABLE ONLY public.match_participants
    ADD CONSTRAINT match_participants_pkey PRIMARY KEY (match_id, user_id);

ALTER TABLE ONLY public.match_players
    ADD CONSTRAINT match_players_pkey PRIMARY KEY (match_id, user_id);

ALTER TABLE ONLY public.match_round_plans
    ADD CONSTRAINT match_round_plans_pkey PRIMARY KEY (match_id, round_index);

ALTER TABLE ONLY public.match_sessions
    ADD CONSTRAINT match_sessions_pkey PRIMARY KEY (match_id);

ALTER TABLE ONLY public.moderation_log
    ADD CONSTRAINT moderation_log_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.moderation_signals
    ADD CONSTRAINT moderation_signals_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.notification_outbox
    ADD CONSTRAINT notification_outbox_dedupe_key_key UNIQUE (dedupe_key);

ALTER TABLE ONLY public.notification_outbox
    ADD CONSTRAINT notification_outbox_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.oauth_identity_bans
    ADD CONSTRAINT oauth_identity_bans_pkey PRIMARY KEY (provider, provider_user_id);

ALTER TABLE ONLY public.parties
    ADD CONSTRAINT parties_invite_code_key UNIQUE (invite_code);

ALTER TABLE ONLY public.parties
    ADD CONSTRAINT parties_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.party_invitations
    ADD CONSTRAINT party_invitations_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.party_members
    ADD CONSTRAINT party_members_pkey PRIMARY KEY (party_id, user_id);

ALTER TABLE ONLY public.player_map_bests
    ADD CONSTRAINT player_map_bests_pkey PRIMARY KEY (user_id, map_id, ruleset);

ALTER TABLE ONLY public.ranked_guess_events
    ADD CONSTRAINT ranked_guess_events_pkey PRIMARY KEY (match_id, round_number, user_id);

ALTER TABLE ONLY public.ranked_stats
    ADD CONSTRAINT ranked_stats_pkey PRIMARY KEY (user_id, mode, season_id);

ALTER TABLE ONLY public.ranks
    ADD CONSTRAINT ranks_pkey PRIMARY KEY (user_id, mode, season_id);

ALTER TABLE ONLY public.runtime_matches
    ADD CONSTRAINT runtime_matches_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.site_settings
    ADD CONSTRAINT site_settings_pkey PRIMARY KEY (key);

ALTER TABLE ONLY public.support_donation_refs
    ADD CONSTRAINT support_donation_refs_pkey PRIMARY KEY (ref);

ALTER TABLE ONLY public.user_badges
    ADD CONSTRAINT user_badges_pkey PRIMARY KEY (user_id, badge_code);

ALTER TABLE ONLY public.user_blocks
    ADD CONSTRAINT user_blocks_pkey PRIMARY KEY (blocker_user_id, blocked_user_id);

ALTER TABLE ONLY public.user_event_sequences
    ADD CONSTRAINT user_event_sequences_pkey PRIMARY KEY (user_id);

ALTER TABLE ONLY public.user_events
    ADD CONSTRAINT user_events_pkey PRIMARY KEY (user_id, sequence);

ALTER TABLE ONLY public.user_identities
    ADD CONSTRAINT user_identities_pkey PRIMARY KEY (user_id, provider);

ALTER TABLE ONLY public.user_identities
    ADD CONSTRAINT user_identities_provider_provider_user_id_key UNIQUE (provider, provider_user_id);

ALTER TABLE ONLY public.user_identity_history
    ADD CONSTRAINT user_identity_history_pkey PRIMARY KEY (user_id, provider, provider_user_id);

ALTER TABLE ONLY public.user_notifications
    ADD CONSTRAINT user_notifications_dedupe_key_key UNIQUE (dedupe_key);

ALTER TABLE ONLY public.user_notifications
    ADD CONSTRAINT user_notifications_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.user_preferences
    ADD CONSTRAINT user_preferences_pkey PRIMARY KEY (user_id);

ALTER TABLE ONLY public.user_stats
    ADD CONSTRAINT user_stats_pkey PRIMARY KEY (user_id);

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_email_key UNIQUE (email);

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);

-- Query indexes

CREATE INDEX changelog_posts_published_updated_idx ON public.changelog_posts USING btree (published, updated_at DESC);

CREATE INDEX idx_auth_sessions_expires_at ON public.auth_sessions USING btree (expires_at);

CREATE INDEX idx_auth_sessions_user_id ON public.auth_sessions USING btree (user_id);

CREATE UNIQUE INDEX idx_chat_conversations_scope ON public.chat_conversations USING btree (scope_kind, scope_id);

CREATE INDEX idx_chat_messages_conversation_created ON public.chat_messages USING btree (conversation_id, created_at);

CREATE INDEX idx_chat_messages_team_history ON public.chat_messages USING btree (conversation_id, team_match_id, team_id, created_at) WHERE (audience = 'team'::public.gd_chat_audience);

CREATE INDEX idx_control_plane_leases_expiry ON public.control_plane_leases USING btree (expires_at);

CREATE INDEX idx_discord_sync_outbox_pending ON public.discord_sync_outbox USING btree (next_attempt_at, id) WHERE (processed_at IS NULL);

CREATE UNIQUE INDEX idx_discord_sync_outbox_pending_unique ON public.discord_sync_outbox USING btree (action, discord_user_id) WHERE (processed_at IS NULL);

CREATE INDEX idx_elo_refunds_cheater_created ON public.elo_refunds USING btree (cheater_user_id, created_at DESC);

CREATE INDEX idx_elo_refunds_user_created ON public.elo_refunds USING btree (user_id, created_at DESC);

CREATE UNIQUE INDEX idx_friend_codes_active_user ON public.friend_codes USING btree (user_id) WHERE (revoked_at IS NULL);

CREATE UNIQUE INDEX idx_friend_requests_active_pair ON public.friend_requests USING btree (LEAST(sender_user_id, recipient_user_id), GREATEST(sender_user_id, recipient_user_id)) WHERE (status = 'pending'::public.gd_social_request_status);

CREATE INDEX idx_friend_requests_recipient_pending ON public.friend_requests USING btree (recipient_user_id, created_at DESC) WHERE (status = 'pending'::public.gd_social_request_status);

CREATE INDEX idx_friend_requests_sender_pending ON public.friend_requests USING btree (sender_user_id, created_at DESC) WHERE (status = 'pending'::public.gd_social_request_status);

CREATE INDEX idx_friendships_high ON public.friendships USING btree (user_id_high, created_at DESC);

CREATE INDEX idx_ip_signup_bans_active ON public.ip_signup_bans USING btree (ip_address) WHERE (revoked_at IS NULL);

CREATE INDEX idx_locations_map_rand ON public.locations USING btree (map_storage_id, rand_key_i);

CREATE INDEX idx_map_aliases_map_id ON public.map_aliases USING btree (map_id);

CREATE INDEX idx_map_comment_likes_user_created ON public.map_comment_likes USING btree (user_id, created_at DESC);

CREATE INDEX idx_map_comments_map_created ON public.map_comments USING btree (map_id, created_at);

CREATE INDEX idx_map_comments_user_created ON public.map_comments USING btree (user_id, created_at DESC);

CREATE INDEX idx_map_favorites_user_created ON public.map_favorites USING btree (user_id, created_at DESC);

CREATE INDEX idx_map_upload_events_user_created ON public.map_upload_events USING btree (user_id, created_at DESC);

CREATE INDEX idx_maps_owner_active ON public.maps USING btree (owner_user_id, updated_at DESC) WHERE (archived_at IS NULL);

CREATE INDEX idx_maps_public_new ON public.maps USING btree (published_at DESC) WHERE ((archived_at IS NULL) AND (published_at IS NOT NULL) AND (status = 'ready'::public.gd_map_status));

CREATE INDEX idx_maps_public_popular ON public.maps USING btree (((play_count + (favorite_count * 3))) DESC, published_at DESC) WHERE ((archived_at IS NULL) AND (published_at IS NOT NULL) AND (status = 'ready'::public.gd_map_status));

CREATE INDEX idx_maps_public_trending ON public.maps USING btree (trending_score DESC, published_at DESC) WHERE ((archived_at IS NULL) AND (published_at IS NOT NULL) AND (status = 'ready'::public.gd_map_status));

CREATE INDEX idx_match_history_ended ON public.match_history USING btree (ended_at DESC);

CREATE INDEX idx_match_history_map_ended ON public.match_history USING btree (map_id, ended_at DESC);

CREATE INDEX idx_match_history_replay_cleanup ON public.match_history USING btree (replay_expires_at) WHERE ((replay_zstd IS NOT NULL) OR (replay_json IS NOT NULL));

CREATE INDEX idx_match_participants_user ON public.match_participants USING btree (user_id, match_id);

CREATE INDEX idx_match_players_user_history ON public.match_players USING btree (user_id, ended_at DESC, match_id DESC);

CREATE INDEX idx_match_sessions_expired_lease ON public.match_sessions USING btree (lease_expires_at, match_id) WHERE (state = 'live'::public.gd_match_session_state);

CREATE INDEX idx_match_sessions_return_target_party ON public.match_sessions USING btree (return_target_party_id) WHERE (return_target_party_id IS NOT NULL);

CREATE INDEX idx_match_sessions_source_party ON public.match_sessions USING btree (source_party_id, started_at DESC);

CREATE INDEX idx_match_sessions_state ON public.match_sessions USING btree (state, updated_at DESC);

CREATE INDEX idx_moderation_log_actor_created ON public.moderation_log USING btree (actor_user_id, created_at DESC, id DESC) WHERE (actor_user_id IS NOT NULL);

CREATE INDEX idx_moderation_log_subject_created ON public.moderation_log USING btree (subject_user_id, created_at DESC, id DESC);

CREATE INDEX idx_moderation_signals_detector_dedupe ON public.moderation_signals USING btree (subject_user_id, COALESCE(match_id, '00000000-0000-0000-0000-000000000000'::uuid), COALESCE(detector_key, ''::text), COALESCE(detector_version, ''::text), reason_code) WHERE (source = 'risk_engine'::public.gd_moderation_source);

CREATE INDEX idx_moderation_signals_match ON public.moderation_signals USING btree (match_id, occurred_at DESC) WHERE (match_id IS NOT NULL);

CREATE INDEX idx_moderation_signals_queue ON public.moderation_signals USING btree (recommended_queue, severity, occurred_at DESC);

CREATE INDEX idx_moderation_signals_report_dedupe ON public.moderation_signals USING btree (match_id, reporter_user_id, subject_user_id) WHERE ((source = 'player_report'::public.gd_moderation_source) AND (reporter_user_id IS NOT NULL) AND (match_id IS NOT NULL));

CREATE INDEX idx_moderation_signals_subject_recent ON public.moderation_signals USING btree (subject_user_id, occurred_at DESC);

CREATE INDEX idx_moderation_signals_unreviewed ON public.moderation_signals USING btree (recommended_queue, severity, occurred_at DESC) WHERE (reviewed_at IS NULL);

CREATE INDEX idx_notification_outbox_pending ON public.notification_outbox USING btree (next_attempt_at, id) WHERE (sent_at IS NULL);

CREATE INDEX idx_notification_outbox_sent ON public.notification_outbox USING btree (sent_at) WHERE (sent_at IS NOT NULL);

CREATE INDEX idx_oauth_identity_bans_active ON public.oauth_identity_bans USING btree (provider, provider_user_id) WHERE (revoked_at IS NULL);

CREATE INDEX idx_parties_active_match ON public.parties USING btree (active_match_id);

CREATE INDEX idx_parties_expires_at ON public.parties USING btree (expires_at);

CREATE INDEX idx_parties_last_match ON public.parties USING btree (last_match_id);

CREATE INDEX idx_parties_owner_user_id ON public.parties USING btree (owner_user_id);

CREATE INDEX idx_parties_started_match ON public.parties USING btree (started_match_id);

CREATE INDEX idx_party_invitations_active ON public.party_invitations USING btree (party_id, recipient_user_id) WHERE (status = 'pending'::public.gd_social_request_status);

CREATE INDEX idx_party_invitations_recipient ON public.party_invitations USING btree (recipient_user_id, created_at DESC) WHERE (status = 'pending'::public.gd_social_request_status);

CREATE INDEX idx_party_members_active ON public.party_members USING btree (party_id, joined_at) WHERE (left_at IS NULL);

CREATE INDEX idx_party_members_user_id ON public.party_members USING btree (user_id);

CREATE INDEX idx_player_map_bests_match ON public.player_map_bests USING btree (match_id);

CREATE INDEX idx_ranked_guess_events_user_recent ON public.ranked_guess_events USING btree (user_id, occurred_at DESC, round_number DESC) INCLUDE (score, guess_ms, evidence);

CREATE INDEX idx_ranks_mode_season_mmr_user ON public.ranks USING btree (mode, season_id, mmr DESC, user_id);

CREATE INDEX idx_support_donation_refs_user_created ON public.support_donation_refs USING btree (user_id, created_at DESC);

CREATE INDEX idx_user_badges_user_awarded ON public.user_badges USING btree (user_id, awarded_at DESC);

CREATE INDEX idx_user_badges_user_updated ON public.user_badges USING btree (user_id, updated_at DESC);

CREATE INDEX idx_user_blocks_target ON public.user_blocks USING btree (blocked_user_id);

CREATE INDEX idx_user_events_created_at ON public.user_events USING btree (created_at);

CREATE INDEX idx_user_identities_normalized_email ON public.user_identities USING btree (lower(btrim(email)), user_id) WHERE ((email IS NOT NULL) AND (btrim(email) <> ''::text));

CREATE INDEX idx_user_notifications_inbox ON public.user_notifications USING btree (user_id, created_at DESC, id DESC) WHERE (archived_at IS NULL);

CREATE INDEX idx_user_notifications_read ON public.user_notifications USING btree (read_at, created_at) WHERE (read_at IS NOT NULL);

CREATE INDEX idx_user_notifications_unread ON public.user_notifications USING btree (user_id, created_at DESC) WHERE (read_at IS NULL);

CREATE UNIQUE INDEX idx_users_normalized_email_unique ON public.users USING btree (lower(btrim(email))) WHERE ((email IS NOT NULL) AND (btrim(email) <> ''::text));

CREATE INDEX idx_users_registration_ip_created ON public.users USING btree (registration_ip_address, created_at DESC) WHERE (registration_ip_address IS NOT NULL);

CREATE UNIQUE INDEX maps_id_key ON public.maps USING btree (id);

CREATE UNIQUE INDEX maps_storage_id_key ON public.maps USING btree (storage_id);

CREATE UNIQUE INDEX users_claimed_nickname_unique ON public.users USING btree (lower(display_name)) WHERE ((account_type = 'registered'::public.gd_account_type) AND (nickname_claimed_at IS NOT NULL));

-- Referential integrity

ALTER TABLE ONLY public.auth_sessions
    ADD CONSTRAINT auth_sessions_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.chat_messages
    ADD CONSTRAINT chat_messages_conversation_id_fkey FOREIGN KEY (conversation_id) REFERENCES public.chat_conversations(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.chat_messages
    ADD CONSTRAINT chat_messages_sender_user_id_fkey FOREIGN KEY (sender_user_id) REFERENCES public.users(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.chat_messages
    ADD CONSTRAINT chat_messages_team_match_id_fkey FOREIGN KEY (team_match_id) REFERENCES public.match_sessions(match_id) ON DELETE CASCADE;

ALTER TABLE ONLY public.elo_refunds
    ADD CONSTRAINT elo_refunds_cheater_user_id_fkey FOREIGN KEY (cheater_user_id) REFERENCES public.users(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.elo_refunds
    ADD CONSTRAINT elo_refunds_match_id_fkey FOREIGN KEY (match_id) REFERENCES public.match_history(match_id) ON DELETE CASCADE;

ALTER TABLE ONLY public.elo_refunds
    ADD CONSTRAINT elo_refunds_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.friend_codes
    ADD CONSTRAINT friend_codes_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.friend_requests
    ADD CONSTRAINT friend_requests_recipient_user_id_fkey FOREIGN KEY (recipient_user_id) REFERENCES public.users(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.friend_requests
    ADD CONSTRAINT friend_requests_sender_user_id_fkey FOREIGN KEY (sender_user_id) REFERENCES public.users(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.friendships
    ADD CONSTRAINT friendships_created_from_request_id_fkey FOREIGN KEY (created_from_request_id) REFERENCES public.friend_requests(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.friendships
    ADD CONSTRAINT friendships_user_id_high_fkey FOREIGN KEY (user_id_high) REFERENCES public.users(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.friendships
    ADD CONSTRAINT friendships_user_id_low_fkey FOREIGN KEY (user_id_low) REFERENCES public.users(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.ip_signup_bans
    ADD CONSTRAINT ip_signup_bans_created_by_fkey FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.locations
    ADD CONSTRAINT locations_current_map_storage_id_fkey FOREIGN KEY (map_storage_id) REFERENCES public.maps(storage_id) ON DELETE CASCADE;

ALTER TABLE ONLY public.map_aliases
    ADD CONSTRAINT map_aliases_map_id_fkey FOREIGN KEY (map_id) REFERENCES public.maps(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.map_comment_likes
    ADD CONSTRAINT map_comment_likes_comment_id_fkey FOREIGN KEY (comment_id) REFERENCES public.map_comments(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.map_comment_likes
    ADD CONSTRAINT map_comment_likes_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.map_comments
    ADD CONSTRAINT map_comments_deleted_by_fkey FOREIGN KEY (deleted_by) REFERENCES public.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.map_comments
    ADD CONSTRAINT map_comments_map_id_fkey FOREIGN KEY (map_id) REFERENCES public.maps(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.map_comments
    ADD CONSTRAINT map_comments_parent_id_fkey FOREIGN KEY (parent_id) REFERENCES public.map_comments(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.map_comments
    ADD CONSTRAINT map_comments_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.map_country_stats
    ADD CONSTRAINT map_country_stats_map_id_fkey FOREIGN KEY (map_id) REFERENCES public.maps(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.map_daily_users
    ADD CONSTRAINT map_daily_users_map_id_fkey FOREIGN KEY (map_id) REFERENCES public.maps(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.map_daily_users
    ADD CONSTRAINT map_daily_users_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.map_favorites
    ADD CONSTRAINT map_favorites_map_id_fkey FOREIGN KEY (map_id) REFERENCES public.maps(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.map_favorites
    ADD CONSTRAINT map_favorites_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.map_stats_daily
    ADD CONSTRAINT map_stats_daily_map_id_fkey FOREIGN KEY (map_id) REFERENCES public.maps(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.map_upload_events
    ADD CONSTRAINT map_upload_events_map_id_fkey FOREIGN KEY (map_id) REFERENCES public.maps(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.map_upload_events
    ADD CONSTRAINT map_upload_events_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.maps
    ADD CONSTRAINT maps_official_by_fkey FOREIGN KEY (official_by) REFERENCES public.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.maps
    ADD CONSTRAINT maps_owner_user_id_fkey FOREIGN KEY (owner_user_id) REFERENCES public.users(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.match_history
    ADD CONSTRAINT match_history_map_id_fkey FOREIGN KEY (map_id) REFERENCES public.maps(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.match_participants
    ADD CONSTRAINT match_participants_match_id_fkey FOREIGN KEY (match_id) REFERENCES public.match_sessions(match_id) ON DELETE CASCADE;

ALTER TABLE ONLY public.match_participants
    ADD CONSTRAINT match_participants_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.match_players
    ADD CONSTRAINT match_players_match_id_fkey FOREIGN KEY (match_id) REFERENCES public.match_history(match_id) ON DELETE CASCADE;

ALTER TABLE ONLY public.match_players
    ADD CONSTRAINT match_players_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.match_round_plans
    ADD CONSTRAINT match_round_plans_map_id_fkey FOREIGN KEY (map_id) REFERENCES public.maps(id);

ALTER TABLE ONLY public.match_sessions
    ADD CONSTRAINT match_sessions_map_id_fkey FOREIGN KEY (map_id) REFERENCES public.maps(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.match_sessions
    ADD CONSTRAINT match_sessions_return_target_map_fk FOREIGN KEY (return_target_map_id) REFERENCES public.maps(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.match_sessions
    ADD CONSTRAINT match_sessions_return_target_party_fk FOREIGN KEY (return_target_party_id) REFERENCES public.parties(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.match_sessions
    ADD CONSTRAINT match_sessions_source_party_id_fkey FOREIGN KEY (source_party_id) REFERENCES public.parties(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.moderation_log
    ADD CONSTRAINT moderation_log_actor_user_id_fkey FOREIGN KEY (actor_user_id) REFERENCES public.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.moderation_log
    ADD CONSTRAINT moderation_log_subject_user_id_fkey FOREIGN KEY (subject_user_id) REFERENCES public.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.moderation_signals
    ADD CONSTRAINT moderation_signals_match_id_fkey FOREIGN KEY (match_id) REFERENCES public.match_history(match_id) ON DELETE SET NULL;

ALTER TABLE ONLY public.moderation_signals
    ADD CONSTRAINT moderation_signals_reporter_user_id_fkey FOREIGN KEY (reporter_user_id) REFERENCES public.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.moderation_signals
    ADD CONSTRAINT moderation_signals_reviewed_by_fkey FOREIGN KEY (reviewed_by) REFERENCES public.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.moderation_signals
    ADD CONSTRAINT moderation_signals_subject_user_id_fkey FOREIGN KEY (subject_user_id) REFERENCES public.users(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.oauth_identity_bans
    ADD CONSTRAINT oauth_identity_bans_banned_user_id_fkey FOREIGN KEY (banned_user_id) REFERENCES public.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.oauth_identity_bans
    ADD CONSTRAINT oauth_identity_bans_created_by_fkey FOREIGN KEY (created_by) REFERENCES public.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.parties
    ADD CONSTRAINT parties_map_id_fkey FOREIGN KEY (map_id) REFERENCES public.maps(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.parties
    ADD CONSTRAINT parties_owner_user_id_fkey FOREIGN KEY (owner_user_id) REFERENCES public.users(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.party_invitations
    ADD CONSTRAINT party_invitations_inviter_user_id_fkey FOREIGN KEY (inviter_user_id) REFERENCES public.users(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.party_invitations
    ADD CONSTRAINT party_invitations_party_id_fkey FOREIGN KEY (party_id) REFERENCES public.parties(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.party_invitations
    ADD CONSTRAINT party_invitations_recipient_user_id_fkey FOREIGN KEY (recipient_user_id) REFERENCES public.users(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.party_members
    ADD CONSTRAINT party_members_party_id_fkey FOREIGN KEY (party_id) REFERENCES public.parties(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.party_members
    ADD CONSTRAINT party_members_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.player_map_bests
    ADD CONSTRAINT player_map_bests_map_id_fkey FOREIGN KEY (map_id) REFERENCES public.maps(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.player_map_bests
    ADD CONSTRAINT player_map_bests_match_id_fkey FOREIGN KEY (match_id) REFERENCES public.match_history(match_id) ON DELETE RESTRICT;

ALTER TABLE ONLY public.player_map_bests
    ADD CONSTRAINT player_map_bests_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.ranked_guess_events
    ADD CONSTRAINT ranked_guess_events_match_id_fkey FOREIGN KEY (match_id) REFERENCES public.match_history(match_id) ON DELETE CASCADE;

ALTER TABLE ONLY public.ranked_guess_events
    ADD CONSTRAINT ranked_guess_events_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.ranked_stats
    ADD CONSTRAINT ranked_stats_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id);

ALTER TABLE ONLY public.ranks
    ADD CONSTRAINT ranks_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id);

ALTER TABLE ONLY public.support_donation_refs
    ADD CONSTRAINT support_donation_refs_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.user_badges
    ADD CONSTRAINT user_badges_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.user_blocks
    ADD CONSTRAINT user_blocks_blocked_user_id_fkey FOREIGN KEY (blocked_user_id) REFERENCES public.users(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.user_blocks
    ADD CONSTRAINT user_blocks_blocker_user_id_fkey FOREIGN KEY (blocker_user_id) REFERENCES public.users(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.user_event_sequences
    ADD CONSTRAINT user_event_sequences_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.user_events
    ADD CONSTRAINT user_events_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.user_identities
    ADD CONSTRAINT user_identities_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.user_identity_history
    ADD CONSTRAINT user_identity_history_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.user_notifications
    ADD CONSTRAINT user_notifications_actor_user_id_fkey FOREIGN KEY (actor_user_id) REFERENCES public.users(id) ON DELETE SET NULL;

ALTER TABLE ONLY public.user_notifications
    ADD CONSTRAINT user_notifications_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.user_preferences
    ADD CONSTRAINT user_preferences_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id) ON DELETE CASCADE;

ALTER TABLE ONLY public.user_stats
    ADD CONSTRAINT user_stats_user_id_fkey FOREIGN KEY (user_id) REFERENCES public.users(id);

--
-- Required reference data
--
-- These settings are the same
-- defaults established by the historical chain, expressed directly as final
-- v2 seed data rather than as transitional backfills.
INSERT INTO public.site_settings (key, value_json, updated_at)
VALUES (
    'gameplay_map_settings',
    jsonb_build_object(
        'movingMapId', 'a-source-world',
        'noMoveMapId', 'a-location-world',
        'nmpzMapId', 'a-location-world'
    ),
    now()
);

INSERT INTO public.site_settings (key, value_json, updated_at)
VALUES (
    'ranked_season',
    jsonb_build_object(
        'activeSeasonId', 's2',
        'monthlyResetDay', 1,
        'lastResetAt',
        CASE
            WHEN now() >= ((date_trunc('month', now() AT TIME ZONE 'UTC') + interval '21 hours') AT TIME ZONE 'UTC')
                THEN to_jsonb((date_trunc('month', now() AT TIME ZONE 'UTC') + interval '21 hours') AT TIME ZONE 'UTC')
            ELSE NULL
        END
    ),
    now()
);

INSERT INTO public.site_settings (key, value_json, updated_at)
VALUES (
    'discord_integration',
    jsonb_build_object(
        'guildId', '',
        'joinsChannelId', '',
        'elo1000RoleId', '',
        'elo1500RoleId', '',
        'elo2000RoleId', '',
        'reconcileIntervalMinutes', 15
    ),
    now()
);
