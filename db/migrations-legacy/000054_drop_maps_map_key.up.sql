-- Map UUIDs are the canonical identity. Preserve every legacy/public key in
-- the alias table before removing the duplicate value from maps.
insert into map_aliases(alias,map_id)
select map_key,id from maps
where map_key is not null and map_key <> id::text
on conflict(alias) do update set map_id=excluded.map_id;

alter table maps drop constraint if exists maps_map_key_key;
alter table maps drop column if exists map_key;
