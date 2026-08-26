-- Maps are durable UUID entities. map_key remains a stable public/legacy
-- alias, while all internal relations use the fixed-width UUID key.

create or replace function geoduels_map_uuid(value text)
returns uuid
language plpgsql
immutable
strict
as $$
declare
  raw bytea;
  encoded text;
begin
  if value ~* '^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$' then
    return value::uuid;
  end if;
  raw := decode(md5('map:' || value), 'hex');
  raw := set_byte(raw,6,(get_byte(raw,6) & 15) | 48);
  raw := set_byte(raw,8,(get_byte(raw,8) & 63) | 128);
  encoded := encode(raw,'hex');
  return (
    substr(encoded,1,8) || '-' ||
    substr(encoded,9,4) || '-' ||
    substr(encoded,13,4) || '-' ||
    substr(encoded,17,4) || '-' ||
    substr(encoded,21,12)
  )::uuid;
end
$$;

alter table maps add column if not exists id uuid;
update maps set id=geoduels_map_uuid(map_key) where id is null;
alter table maps alter column id set not null;
create unique index if not exists maps_id_key on maps(id);
alter table maps add column if not exists storage_id integer generated always as identity;
create unique index if not exists maps_storage_id_key on maps(storage_id);

create table if not exists map_aliases (
  alias text primary key,
  map_id uuid not null references maps(id) on delete cascade,
  created_at timestamptz not null default now()
);
create index if not exists idx_map_aliases_map_id on map_aliases(map_id);
insert into map_aliases(alias,map_id)
select map_key,id from maps
on conflict(alias) do update set map_id=excluded.map_id;

-- Convert compact/current map relations before replacing the maps PK.
alter table map_favorites add column if not exists map_uuid uuid;
update map_favorites f set map_uuid=m.id from maps m where f.map_id=m.map_key and f.map_uuid is null;
alter table map_favorites alter column map_uuid set not null;
alter table map_favorites drop constraint if exists map_favorites_pkey;
alter table map_favorites drop constraint if exists map_favorites_map_id_fkey;
drop index if exists idx_map_favorites_user_created;
alter table map_favorites drop column map_id;
alter table map_favorites rename column map_uuid to map_id;
alter table map_favorites add constraint map_favorites_map_id_fkey foreign key(map_id) references maps(id) on delete cascade;
alter table map_favorites add primary key(map_id,user_id);
create index idx_map_favorites_user_created on map_favorites(user_id,created_at desc);

alter table map_comments add column if not exists map_uuid uuid;
update map_comments c set map_uuid=m.id from maps m where c.map_id=m.map_key and c.map_uuid is null;
alter table map_comments alter column map_uuid set not null;
alter table map_comments drop constraint if exists map_comments_map_id_fkey;
drop index if exists idx_map_comments_map_created;
alter table map_comments drop column map_id;
alter table map_comments rename column map_uuid to map_id;
alter table map_comments add constraint map_comments_map_id_fkey foreign key(map_id) references maps(id) on delete cascade;
create index idx_map_comments_map_created on map_comments(map_id,created_at asc);

alter table map_stats_daily add column if not exists map_uuid uuid;
update map_stats_daily d set map_uuid=m.id from maps m where d.map_id=m.map_key and d.map_uuid is null;
alter table map_stats_daily alter column map_uuid set not null;
alter table map_stats_daily drop constraint if exists map_stats_daily_pkey;
alter table map_stats_daily drop constraint if exists map_stats_daily_map_id_fkey;
alter table map_stats_daily drop column map_id;
alter table map_stats_daily rename column map_uuid to map_id;
alter table map_stats_daily add constraint map_stats_daily_map_id_fkey foreign key(map_id) references maps(id) on delete cascade;
alter table map_stats_daily add primary key(map_id,day);

alter table map_daily_users add column if not exists map_uuid uuid;
update map_daily_users d set map_uuid=m.id from maps m where d.map_id=m.map_key and d.map_uuid is null;
alter table map_daily_users alter column map_uuid set not null;
alter table map_daily_users drop constraint if exists map_daily_users_pkey;
alter table map_daily_users drop constraint if exists map_daily_users_map_id_fkey;
alter table map_daily_users drop column map_id;
alter table map_daily_users rename column map_uuid to map_id;
alter table map_daily_users add constraint map_daily_users_map_id_fkey foreign key(map_id) references maps(id) on delete cascade;
alter table map_daily_users add primary key(map_id,day,user_id);

alter table parties add column if not exists map_uuid uuid;
update parties p set map_uuid=m.id from maps m where p.map_id=m.map_key and p.map_uuid is null;
alter table parties drop constraint if exists parties_map_id_fkey;
alter table parties drop column map_id;
alter table parties rename column map_uuid to map_id;
alter table parties add constraint parties_map_id_fkey foreign key(map_id) references maps(id) on delete set null;

alter table match_sessions add column if not exists map_uuid uuid;
update match_sessions s set map_uuid=m.id from maps m where s.map_id=m.map_key and s.map_uuid is null;
alter table match_sessions drop column if exists map_revision_id;
alter table match_sessions drop column map_id;
alter table match_sessions rename column map_uuid to map_id;
alter table match_sessions add constraint match_sessions_map_id_fkey foreign key(map_id) references maps(id) on delete set null;

alter table match_history add column if not exists map_uuid uuid;
update match_history h set map_uuid=m.id from maps m where h.map_key=m.map_key and h.map_uuid is null;
alter table match_history drop column if exists map_revision_id;
alter table match_history drop column if exists map_key;
alter table match_history rename column map_uuid to map_id;
alter table match_history add constraint match_history_map_id_fkey foreign key(map_id) references maps(id) on delete set null;
create index if not exists idx_match_history_map_ended on match_history(map_id,ended_at desc);

alter table match_round_plans add column if not exists map_uuid uuid;
update match_round_plans p set map_uuid=m.id from maps m where p.map_id=m.map_key and p.map_uuid is null;
alter table match_round_plans alter column map_uuid set not null;
alter table match_round_plans drop constraint if exists match_round_plans_map_id_fkey;
alter table match_round_plans drop constraint if exists match_round_plans_map_revision_id_fkey;
drop index if exists idx_match_round_plans_revision;
alter table match_round_plans drop column if exists map_revision_id;
alter table match_round_plans drop column map_id;
alter table match_round_plans rename column map_uuid to map_id;
alter table match_round_plans add constraint match_round_plans_map_id_fkey foreign key(map_id) references maps(id);

alter table map_upload_events add column if not exists map_uuid uuid;
update map_upload_events e set map_uuid=m.id from maps m where e.map_id=m.map_key and e.map_uuid is null;
alter table map_upload_events drop column map_id;
alter table map_upload_events rename column map_uuid to map_id;
alter table map_upload_events add constraint map_upload_events_map_id_fkey foreign key(map_id) references maps(id) on delete set null;

-- Keep only the active dataset for each map and attach it directly to maps.id.
create table locations_current (
  map_storage_id integer not null references maps(storage_id) on delete cascade,
  lat_e7 integer not null,
  lng_e7 integer not null,
  rand_key_i integer not null,
  heading_cdeg smallint,
  pitch_cdeg smallint,
  country text,
  pano_id text
);
insert into locations_current(map_storage_id,lat_e7,lng_e7,rand_key_i,heading_cdeg,pitch_cdeg,country,pano_id)
select m.storage_id,l.lat_e7,l.lng_e7,l.rand_key_i,l.heading_cdeg,l.pitch_cdeg,l.country,l.pano_id
from maps m
join map_revisions r on r.id=m.active_revision_id
join locations l on l.revision_storage_id=r.storage_id;
drop table locations;
alter table locations_current rename to locations;
create index idx_locations_map_rand on locations(map_storage_id,rand_key_i);

create table map_country_stats (
  map_id uuid not null references maps(id) on delete cascade,
  country text not null,
  location_count integer not null,
  primary key(map_id,country)
);
insert into map_country_stats(map_id,country,location_count)
select m.id,s.country,s.location_count
from maps m
join map_revision_country_stats s on s.map_revision_id=m.active_revision_id;

alter table maps drop constraint if exists maps_active_revision_id_fkey;
alter table maps drop column if exists active_revision_id;
alter table maps add column if not exists content_hash bytea;
alter table maps add column if not exists rejected_location_count integer not null default 0;
drop table map_revision_country_stats;
drop table map_revisions;

alter table maps drop constraint if exists maps_pkey;
alter table maps add primary key(id);
alter table maps add constraint maps_map_key_key unique(map_key);

-- Rewrite configured map identifiers to canonical UUIDs while retaining
-- support for defaults that still arrive as legacy aliases.
update site_settings s
set value_json=jsonb_build_object(
  'rankedMovingMapId', coalesce((select m.id::text from maps m where m.map_key=s.value_json->>'rankedMovingMapId'), s.value_json->>'rankedMovingMapId'),
  'rankedNmpzMapId', coalesce((select m.id::text from maps m where m.map_key=s.value_json->>'rankedNmpzMapId'), s.value_json->>'rankedNmpzMapId'),
  'singleplayerMovingMapId', coalesce((select m.id::text from maps m where m.map_key=s.value_json->>'singleplayerMovingMapId'), s.value_json->>'singleplayerMovingMapId'),
  'singleplayerNmpzMapId', coalesce((select m.id::text from maps m where m.map_key=s.value_json->>'singleplayerNmpzMapId'), s.value_json->>'singleplayerNmpzMapId')
)
where s.key='gameplay_map_settings';

create table player_map_bests (
  user_id uuid not null references users(id) on delete cascade,
  map_id uuid not null references maps(id) on delete cascade,
  ruleset smallint not null check(ruleset in (0,1)),
  best_score smallint not null check(best_score between 0 and 25000),
  match_id uuid not null references match_history(match_id) on delete restrict,
  achieved_at timestamptz not null,
  primary key(user_id,map_id,ruleset)
);
create index idx_player_map_bests_match on player_map_bests(match_id);

insert into player_map_bests(user_id,map_id,ruleset,best_score,match_id,achieved_at)
select distinct on (p.user_id,h.map_id,case when h.ruleset='nmpz' then 1 else 0 end)
  p.user_id,
  h.map_id,
  case when h.ruleset='nmpz' then 1 else 0 end,
  least(25000,greatest(0,p.total_score))::smallint,
  h.match_id,
  h.ended_at
from match_history h
join match_players p on p.match_id=h.match_id
where h.mode='singleplayer'
  and h.round_count=5
  and h.map_id is not null
order by p.user_id,h.map_id,case when h.ruleset='nmpz' then 1 else 0 end,p.total_score desc,h.ended_at asc;

drop function geoduels_map_uuid(text);
