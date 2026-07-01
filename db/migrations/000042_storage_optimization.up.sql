-- Stop the largest sources of unbounded database growth while preserving
-- public identifiers and compatibility with legacy replay rows.

alter table match_history
  add column if not exists replay_zstd bytea,
  add column if not exists replay_codec smallint,
  add column if not exists replay_schema_version smallint,
  add column if not exists replay_uncompressed_bytes integer,
  add column if not exists replay_sha256 bytea,
  add column if not exists replay_expires_at timestamptz,
  add column if not exists round_count smallint not null default 0;

alter table match_history
  add constraint match_history_replay_codec_check
  check (replay_codec is null or replay_codec = 1) not valid;

alter table match_players
  add column if not exists total_score integer not null default 0;

-- Preserve compact fields currently derived from replay JSON before either
-- JSON column is pruned.
update match_history
set round_count = coalesce(
  jsonb_array_length(coalesce(replay_json, snapshot_json)->'roundResults'),
  0
)::smallint;

update match_players p
set total_score = coalesce(
  nullif(coalesce(h.replay_json, h.snapshot_json) #>> array['players', p.user_id, 'totalScore'], '')::integer,
  0
)
from match_history h
where h.match_id=p.match_id;

with rating_backfill as (
  select
    p.match_id,
    p.user_id,
    coalesce(p.rating_before, p.mmr) as rating_before,
    case
      when h.winner_user_id=p.user_id
        then nullif(coalesce(h.replay_json,h.snapshot_json)->'ratingPreview'->p.user_id->>'win','')::integer
      when nullif(h.winner_user_id,'') is null
        then nullif(coalesce(h.replay_json,h.snapshot_json)->'ratingPreview'->p.user_id->>'draw','')::integer
      else nullif(coalesce(h.replay_json,h.snapshot_json)->'ratingPreview'->p.user_id->>'lose','')::integer
    end as delta
  from match_players p
  join match_history h on h.match_id=p.match_id
  where h.ranked and p.final_ranked_delta is null
)
update match_players p
set rating_before=b.rating_before,
    final_ranked_delta=b.delta,
    rating_after=b.rating_before+b.delta
from rating_backfill b
where p.match_id=b.match_id
  and p.user_id=b.user_id
  and b.delta is not null;

update match_history
set replay_expires_at = ended_at + interval '30 days';

update match_history
set replay_json = snapshot_json
where replay_json is null and snapshot_json is not null;

update match_history h
set replay_expires_at = null
where exists(select 1 from moderation_reports r where r.match_id=h.match_id)
   or exists(select 1 from moderation_evidence e where e.match_id=h.match_id);

update match_history
set replay_json = null
where replay_expires_at <= now();

alter table match_history
  drop column if exists snapshot_json,
  drop column if exists state;

create index if not exists idx_match_history_replay_cleanup
  on match_history(replay_expires_at)
  where replay_zstd is not null or replay_json is not null;

drop index if exists idx_match_history_ranked_ruleset;

-- Detailed guesses are retained in the compressed replay for 30 days.
drop table if exists match_round_guesses;

-- Keep only the compact permanent ranked-moderation projection.
alter table ranked_guess_events
  drop constraint if exists ranked_guess_events_match_id_round_id_user_id_key;
drop index if exists idx_ranked_guess_events_match;
drop index if exists idx_ranked_guess_events_user_recent;
alter table ranked_guess_events
  alter column round_number type smallint using round_number::smallint,
  alter column score type smallint using score::smallint,
  alter column guess_ms type integer using least(guess_ms, 2147483647)::integer,
  alter column evidence type real using evidence::real;
create unique index if not exists ranked_guess_events_match_round_user_key
  on ranked_guess_events(match_id, round_number, user_id);
create index if not exists idx_ranked_guess_events_user_recent
  on ranked_guess_events(user_id, occurred_at desc, round_number desc)
  include (score, guess_ms, evidence);
alter table ranked_guess_events
  drop constraint if exists ranked_guess_events_pkey,
  drop column if exists id,
  drop column if exists round_id,
  drop column if exists ruleset,
  drop column if exists created_at;
alter table ranked_guess_events
  add constraint ranked_guess_events_pkey
  primary key using index ranked_guess_events_match_round_user_key;

alter table match_players
  drop column if exists ranked_games_played,
  drop column if exists team_id,
  drop column if exists placement;

-- Compact coordinates and deterministic random ordering.
alter table map_revisions
  add column if not exists storage_id integer generated always as identity;
create unique index if not exists map_revisions_storage_id_key
  on map_revisions(storage_id);

-- Match plans already contain immutable round data; remove the location FK
-- before replacing the old location table.
alter table match_round_plans
  drop constraint if exists match_round_plans_location_id_fkey,
  drop column if exists location_id;

-- Build the compact table directly so fixed-width fields are packed first and
-- the migration returns the old heap/index space when the table is swapped.
create table locations_compact (
  revision_storage_id integer not null
    references map_revisions(storage_id) on delete cascade,
  lat_e7 integer not null,
  lng_e7 integer not null,
  rand_key_i integer not null,
  heading_cdeg smallint,
  pitch_cdeg smallint,
  country text,
  pano_id text
);

insert into locations_compact(
  revision_storage_id,lat_e7,lng_e7,rand_key_i,
  heading_cdeg,pitch_cdeg,country,pano_id
)
select
  r.storage_id,
  round(l.lat * 10000000)::integer,
  round(l.lng * 10000000)::integer,
  least(16777215, greatest(0, floor(l.rand_key * 16777216.0)::integer)),
  case
    when l.heading is null then null
    else round((((l.heading + 180.0) - floor((l.heading + 180.0) / 360.0) * 360.0) - 180.0) * 100.0)::smallint
  end,
  case
    when l.pitch is null then null
    else round(greatest(-90.0, least(90.0, l.pitch)) * 100.0)::smallint
  end,
  nullif(l.country,''),
  l.pano_id
from locations l
join map_revisions r on r.id=l.map_revision_id;

drop table locations;
alter table locations_compact rename to locations;
alter table locations
  rename constraint locations_compact_revision_storage_id_fkey
  to locations_revision_storage_id_fkey;
create index idx_locations_revision_rand
  on locations(revision_storage_id,rand_key_i);

-- Remove columns and indexes that have no production read/write path.
drop index if exists idx_chat_messages_match_created;
drop index if exists idx_chat_messages_sender_created;
drop index if exists idx_chat_messages_moderation;
alter table chat_messages
  drop constraint if exists chat_messages_redacted_by_fkey,
  drop column if exists match_id,
  drop column if exists moderation_state,
  drop column if exists deleted_at,
  drop column if exists metadata,
  drop column if exists retained_until,
  drop column if exists redacted_at,
  drop column if exists redacted_by;

alter table runtime_matches
  drop column if exists map_id,
  drop column if exists map_revision_id;

alter table auth_sessions
  drop column if exists ip_redacted_at,
  drop column if exists user_agent_redacted_at;

alter table user_identity_history
  drop column if exists avatar_url;
drop index if exists idx_user_identity_history_provider_user_id;
drop index if exists idx_user_identity_history_email_lower;

drop table if exists runtime_snapshots;

-- More responsive maintenance for append-heavy and frequently cleaned tables.
alter table match_history set (
  autovacuum_vacuum_scale_factor = 0.02,
  autovacuum_analyze_scale_factor = 0.01
);
alter table ranked_guess_events set (
  autovacuum_vacuum_scale_factor = 0.02,
  autovacuum_analyze_scale_factor = 0.01
);
alter table locations set (
  autovacuum_vacuum_scale_factor = 0.02,
  autovacuum_analyze_scale_factor = 0.01
);
alter table chat_messages set (
  autovacuum_vacuum_scale_factor = 0.02,
  autovacuum_analyze_scale_factor = 0.01
);
alter table runtime_matches set (
  autovacuum_vacuum_scale_factor = 0.01,
  autovacuum_analyze_scale_factor = 0.01
);
alter table auth_sessions set (
  autovacuum_vacuum_scale_factor = 0.02,
  autovacuum_analyze_scale_factor = 0.01
);

alter table match_history
  validate constraint match_history_replay_codec_check;
