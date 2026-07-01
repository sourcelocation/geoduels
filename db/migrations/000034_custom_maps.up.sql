alter table maps
  add column if not exists owner_user_id text references users(id) on delete cascade,
  add column if not exists description text not null default '',
  add column if not exists visibility text not null default 'private',
  add column if not exists status text not null default 'ready',
  add column if not exists active_revision_id text references map_revisions(id),
  add column if not exists location_count integer not null default 0,
  add column if not exists updated_at timestamptz not null default now(),
  add column if not exists archived_at timestamptz;

alter table maps drop constraint if exists maps_visibility_check;
alter table maps add constraint maps_visibility_check
  check (visibility in ('private', 'unlisted'));

alter table maps drop constraint if exists maps_status_check;
alter table maps add constraint maps_status_check
  check (status in ('processing', 'ready', 'rejected', 'archived'));

alter table map_revisions
  add column if not exists validation_error text not null default '',
  add column if not exists rejected_count integer not null default 0;

update maps m
set active_revision_id = ma.active_revision_id,
    location_count = coalesce(mr.row_count, 0),
    status = case when ma.active_revision_id is null then 'rejected' else 'ready' end,
    updated_at = coalesce(ma.updated_at, m.created_at)
from map_aliases ma
left join map_revisions mr on mr.id = ma.active_revision_id
where ma.map_key = m.map_key;

drop table if exists map_aliases;

alter table lobbies
  add column if not exists config_json jsonb not null default '{"ruleset":"moving","roundTimerMode":"none","pressureTimeLimitMs":15000}'::jsonb,
  add column if not exists map_id text references maps(map_key);

update lobbies
set map_id = coalesce(map_id, 'a-source-world')
where exists (select 1 from maps where map_key = 'a-source-world');

create table if not exists match_round_plans (
  match_id text not null,
  round_index integer not null check (round_index >= 0),
  map_id text not null references maps(map_key),
  map_revision_id text not null references map_revisions(id),
  location_id bigint references locations(id),
  lat double precision not null,
  lng double precision not null,
  country text,
  pano_id text,
  heading double precision,
  pitch double precision,
  primary key (match_id, round_index)
);

create index if not exists idx_maps_owner_active
  on maps(owner_user_id, updated_at desc)
  where archived_at is null;

create index if not exists idx_map_revisions_map_created
  on map_revisions(map_key, created_at desc);

create index if not exists idx_match_round_plans_revision
  on match_round_plans(map_revision_id);

create table if not exists map_upload_events (
  id bigserial primary key,
  user_id text not null references users(id) on delete cascade,
  map_id text,
  created_at timestamptz not null default now()
);

create index if not exists idx_map_upload_events_user_created
  on map_upload_events(user_id, created_at desc);

alter table runtime_matches
  add column if not exists map_id text,
  add column if not exists map_revision_id text;
