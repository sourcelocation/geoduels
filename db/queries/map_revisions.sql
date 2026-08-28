-- name: DeleteMapCountryStats :exec
delete from map_country_stats where map_id=$1;

-- name: DeleteMapLocations :exec
delete from locations where map_storage_id=$1;

-- name: FinalizeMap :exec
update maps set status='ready',location_count=$2,content_hash=$3,rejected_location_count=0,updated_at=now() where id=$1;

-- name: InsertMapCountryStats :exec
insert into map_country_stats(map_id,country,location_count)
select $1,coalesce(nullif(country,''),'Unknown'),count(*)::int from locations where map_storage_id=$2
group by coalesce(nullif(country,''),'Unknown');

-- name: InsertMapLocations :exec
insert into locations(map_storage_id,lat_e7,lng_e7,country,pano_id,heading_cdeg,pitch_cdeg,rand_key_i)
select sqlc.arg(map_storage_id), unnest(sqlc.arg(lat_e7)::int[]), unnest(sqlc.arg(lng_e7)::int[]), unnest(sqlc.arg(country)::text[]), unnest(sqlc.arg(pano_id)::text[]), unnest(sqlc.arg(heading_cdeg)::smallint[]), unnest(sqlc.arg(pitch_cdeg)::smallint[]), unnest(sqlc.arg(rand_key_i)::int[]);

-- name: LockMapStorageID :one
select storage_id from maps where id=$1 for update;

-- name: UpsertMap :exec
insert into maps(id,display_name,status,visibility,location_count,content_hash,created_at,updated_at)
values($1,$2,'processing','public',0,$3,now(),now())
on conflict(id) do update set display_name=excluded.display_name,status='processing',updated_at=now();

-- name: UpsertMapAlias :exec
insert into map_aliases(alias,map_id) values($1,$2) on conflict(alias) do update set map_id=excluded.map_id;
