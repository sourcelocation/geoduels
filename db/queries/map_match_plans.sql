-- name: IncrementMapPlay :exec
update maps set play_count=play_count+1,updated_at=now() where id=$1;

-- name: InsertMatchRoundPlan :exec
insert into match_round_plans(match_id,round_index,map_id,lat,lng,country,pano_id,heading,pitch) values($1,$2,$3,$4,$5,$6,$7,$8,$9) on conflict(match_id,round_index) do nothing;

-- name: ListMatchRoundPlans :many
select round_index,lat,lng,coalesce(country,'') AS country,pano_id,heading,pitch,map_id AS map_id from match_round_plans where match_id=sqlc.arg(match_id) order by round_index;

-- name: MapDisplayName :one
select display_name from maps where id=$1;

-- name: SelectPlanRowsGE :many
select lat_e7 AS lat_e7,lng_e7 AS lng_e7,coalesce(country,'') AS country,pano_id,heading_cdeg AS heading_cdeg,pitch_cdeg AS pitch_cdeg from locations where map_storage_id=(select storage_id from maps where id=sqlc.arg(map_id)) and rand_key_i >= sqlc.arg(min_rand_key) order by rand_key_i asc limit sqlc.arg(row_limit);

-- name: SelectPlanRowsLT :many
select lat_e7 AS lat_e7,lng_e7 AS lng_e7,coalesce(country,'') AS country,pano_id,heading_cdeg AS heading_cdeg,pitch_cdeg AS pitch_cdeg from locations where map_storage_id=(select storage_id from maps where id=sqlc.arg(map_id)) and rand_key_i < sqlc.arg(max_rand_key) order by rand_key_i asc limit sqlc.arg(row_limit);

-- name: SelectedMap :one
select owner_user_id AS owner_user_id,visibility,status,display_name,location_count from maps where id=sqlc.arg(map_id) and archived_at is null for share;
