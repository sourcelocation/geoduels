-- name: AcquireUploadLock :exec
select pg_advisory_xact_lock(hashtext($1));

-- name: AddFavorite :execrows
insert into map_favorites(map_id,user_id) values($1,$2) on conflict do nothing;

-- name: ArchiveMap :execrows
update maps set status='archived',archived_at=now(),updated_at=now() where id=sqlc.arg(map_id) and (owner_user_id=sqlc.arg(user_id) or sqlc.arg(allow_any)::boolean) and archived_at is null;

-- name: CreateMap :exec
insert into maps(id,owner_user_id,display_name,description,visibility,status,difficulty,thumbnail_variant,thumbnail_key,location_count,published_at,created_at,updated_at) values(sqlc.arg(map_id),sqlc.arg(owner_user_id),sqlc.arg(display_name),sqlc.arg(description),sqlc.arg(visibility),'processing',sqlc.arg(difficulty),sqlc.arg(thumbnail_variant),sqlc.arg(thumbnail_key),0,case when sqlc.arg(visibility)='public' then now() else null end,now(),now());

-- name: DecrementFavoriteCount :exec
update maps set favorite_count=greatest(favorite_count-1,0),updated_at=now() where id=$1;

-- name: DeleteArchivableMap :execrows
delete from maps m where m.id=sqlc.arg(map_id) and (m.owner_user_id=sqlc.arg(user_id) or sqlc.arg(allow_any)::boolean) and m.archived_at is null and not exists(select 1 from match_round_plans p where p.map_id=m.id) and not exists(select 1 from parties p where p.map_id=m.id);

-- name: DeleteCountryStats :exec
 delete from map_country_stats where map_id=$1;

-- name: DeleteLocations :exec
 delete from locations where map_storage_id=$1;

-- name: FavoriteVisibility :one
select exists(select 1 from maps visible_map where visible_map.id=$1 and visible_map.archived_at is null and (visible_map.owner_user_id is null or visible_map.official_at is not null or visible_map.owner_user_id=$2 or visible_map.visibility in ('public','unlisted'))) as visible, coalesce((select owner_map.owner_user_id::text from maps owner_map where owner_map.id=$1 and owner_map.archived_at is null),'')::text as owner_user_id;

-- name: InsertCountryStats :exec
insert into map_country_stats(map_id,country,location_count) select sqlc.arg(map_id),coalesce(nullif(country,''),'Unknown'),count(*)::int from locations where map_storage_id=sqlc.arg(map_storage_id) group by coalesce(nullif(country,''),'Unknown');

-- name: InsertLocations :exec
insert into locations(map_storage_id,lat_e7,lng_e7,country,pano_id,heading_cdeg,pitch_cdeg,rand_key_i)
select sqlc.arg(map_storage_id), unnest(sqlc.arg(lat_e7)::int[]), unnest(sqlc.arg(lng_e7)::int[]), unnest(sqlc.arg(country)::text[]), unnest(sqlc.arg(pano_id)::text[]), unnest(sqlc.arg(heading_cdeg)::smallint[]), unnest(sqlc.arg(pitch_cdeg)::smallint[]), unnest(sqlc.arg(rand_key_i)::int[]);

-- name: InsertUploadEvent :exec
insert into map_upload_events(user_id,map_id,location_count) values($1,$2,$3);

-- name: LockMapOwner :one
select coalesce(owner_user_id::text,'')::text as owner_user_id from maps where id=$1 and archived_at is null for update;

-- name: MarkMapReady :exec
update maps set status='ready',location_count=sqlc.arg(location_count),content_hash=sqlc.arg(content_hash),rejected_location_count=sqlc.arg(rejected_location_count),updated_at=now() where id=sqlc.arg(map_id);

-- name: PublishMap :execrows
update maps set visibility='public',published_at=coalesce(published_at,now()),updated_at=now() where id=$1 and owner_user_id=$2 and archived_at is null and status='ready';

-- name: RemoveFavorite :execrows
 delete from map_favorites where map_id=$1 and user_id=$2;

-- name: StorageID :one
select storage_id from maps where id=$1;

-- name: UpdateMapDetails :execrows
update maps set display_name=sqlc.arg(display_name),description=sqlc.arg(description),visibility=sqlc.arg(visibility),difficulty=sqlc.arg(difficulty),thumbnail_variant=sqlc.arg(thumbnail_variant),thumbnail_key=sqlc.arg(thumbnail_key),updated_at=now() where id=sqlc.arg(map_id) and owner_user_id=sqlc.arg(owner_user_id) and archived_at is null;

-- name: UpsertAlias :exec
insert into map_aliases(alias,map_id) values($1,$2) on conflict(alias) do update set map_id=excluded.map_id;

-- name: UpsertOfficialMap :exec
insert into maps(id,owner_user_id,display_name,description,visibility,status,difficulty,thumbnail_variant,thumbnail_key,location_count,content_hash,rejected_location_count,published_at,official_at,official_by,official_region_type,official_region_code,created_at,updated_at) values(sqlc.arg(map_id),null,sqlc.arg(display_name),sqlc.arg(description),sqlc.arg(visibility),'processing',sqlc.arg(difficulty),sqlc.arg(thumbnail_variant),sqlc.arg(thumbnail_key),0,sqlc.arg(content_hash),sqlc.arg(rejected_location_count),case when sqlc.arg(visibility)='public' then now() else null end,now(),nullif(sqlc.arg(official_by),'')::uuid,sqlc.arg(official_region_type),sqlc.arg(official_region_code),now(),now()) on conflict(id) do update set owner_user_id=null,display_name=excluded.display_name,description=excluded.description,visibility=excluded.visibility,status='processing',difficulty=excluded.difficulty,thumbnail_variant=excluded.thumbnail_variant,thumbnail_key=excluded.thumbnail_key,content_hash=excluded.content_hash,rejected_location_count=excluded.rejected_location_count,published_at=case when excluded.visibility='public' then coalesce(maps.published_at,now()) else null end,official_at=coalesce(maps.official_at,now()),official_by=excluded.official_by,official_region_type=excluded.official_region_type,official_region_code=excluded.official_region_code,archived_at=null,updated_at=now();
