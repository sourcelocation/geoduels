alter table maps add column if not exists auto_zoom_play_region boolean not null default false;
alter table maps add column if not exists bounds_min_lat_e7 integer;
alter table maps add column if not exists bounds_max_lat_e7 integer;
alter table maps add column if not exists bounds_min_lng_e7 integer;
alter table maps add column if not exists bounds_max_lng_e7 integer;

update maps m
set bounds_min_lat_e7 = b.min_lat,
    bounds_max_lat_e7 = b.max_lat,
    bounds_min_lng_e7 = b.min_lng,
    bounds_max_lng_e7 = b.max_lng
from (
  select map_storage_id,
         min(lat_e7) as min_lat,
         max(lat_e7) as max_lat,
         min(lng_e7) as min_lng,
         max(lng_e7) as max_lng
  from locations
  group by map_storage_id
) b
where b.map_storage_id = m.storage_id;
