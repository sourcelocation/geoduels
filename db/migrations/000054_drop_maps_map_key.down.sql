alter table maps add column if not exists map_key text;

update maps m
set map_key=coalesce((
  select a.alias
  from map_aliases a
  where a.map_id=m.id
  order by a.created_at,a.alias
  limit 1
),m.id::text);

alter table maps alter column map_key set not null;
alter table maps add constraint maps_map_key_key unique(map_key);
