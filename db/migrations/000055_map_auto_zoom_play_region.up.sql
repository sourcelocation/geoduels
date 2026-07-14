alter table maps add column if not exists auto_zoom_play_region boolean not null default false;
alter table maps add column if not exists bounds_min_lat_e7 integer;
alter table maps add column if not exists bounds_max_lat_e7 integer;
alter table maps add column if not exists bounds_min_lng_e7 integer;
alter table maps add column if not exists bounds_max_lng_e7 integer;

-- Latitude bounds are a plain min/max.
update maps m
set bounds_min_lat_e7 = b.min_lat,
    bounds_max_lat_e7 = b.max_lat
from (
  select map_storage_id, min(lat_e7) as min_lat, max(lat_e7) as max_lat
  from locations
  group by map_storage_id
) b
where b.map_storage_id = m.storage_id;

-- Longitude bounds use the shortest circular interval (the complement of the
-- largest empty gap between adjacent longitudes) so antimeridian-crossing maps
-- produce a narrow region. Stored as [min_lng_e7, max_lng_e7] traversed east;
-- min_lng_e7 > max_lng_e7 when the interval crosses the antimeridian.
with ordered as (
  select map_storage_id, lng_e7,
         lead(lng_e7) over (partition by map_storage_id order by lng_e7) as next_lng,
         min(lng_e7) over (partition by map_storage_id) as min_lng,
         max(lng_e7) over (partition by map_storage_id) as max_lng
  from (select distinct map_storage_id, lng_e7 from locations) d
),
gaps as (
  select map_storage_id, (next_lng::bigint - lng_e7) as gap, next_lng as start_lng, lng_e7 as end_lng
  from ordered
  where next_lng is not null
  union all
  select map_storage_id, (min_lng::bigint + 3600000000 - max_lng) as gap, min_lng as start_lng, max_lng as end_lng
  from ordered
  where next_lng is null
),
best as (
  select distinct on (map_storage_id) map_storage_id, start_lng, end_lng
  from gaps
  order by map_storage_id, gap desc, start_lng
)
update maps m
set bounds_min_lng_e7 = b.start_lng,
    bounds_max_lng_e7 = b.end_lng
from best b
where b.map_storage_id = m.storage_id;
