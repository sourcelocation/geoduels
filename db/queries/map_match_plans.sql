-- name: IncrementMapPlay :exec
update maps set play_count=play_count+1,updated_at=now() where id=$1;

-- name: InsertMatchRoundPlan :exec
insert into match_round_plans(match_id,round_index,map_id,lat,lng,country,pano_id,heading,pitch) values($1,$2,$3,$4,$5,$6,$7,$8,$9) on conflict(match_id,round_index) do nothing;

-- name: ListMatchRoundPlans :many
select round_index,lat,lng,coalesce(country,''),pano_id,heading,pitch,map_id::text from match_round_plans where match_id=$1 order by round_index;

-- name: MapDisplayName :one
select display_name from maps where id=$1;

-- name: MapIdentity :one
select m.id::text,coalesce((select a.alias from map_aliases a where a.map_id=m.id order by a.created_at,a.alias limit 1),m.id::text) from maps m where m.id::text=$1 or exists(select 1 from map_aliases a where a.map_id=m.id and a.alias=$1) limit 1;

-- name: SelectPlanRowsGE :many
select lat_e7::float8/10000000.0,lng_e7::float8/10000000.0,coalesce(country,''),pano_id,heading_cdeg::float8/100.0,pitch_cdeg::float8/100.0 from locations where map_storage_id=(select storage_id from maps where id=$1) and rand_key_i >= $2 order by rand_key_i asc limit $3;

-- name: SelectPlanRowsLT :many
select lat_e7::float8/10000000.0,lng_e7::float8/10000000.0,coalesce(country,''),pano_id,heading_cdeg::float8/100.0,pitch_cdeg::float8/100.0 from locations where map_storage_id=(select storage_id from maps where id=$1) and rand_key_i < $2 order by rand_key_i asc limit $3;

-- name: SelectedMap :one
select coalesce(owner_user_id::text,''),visibility,status,display_name,location_count from maps where id=$1 and archived_at is null for share;
