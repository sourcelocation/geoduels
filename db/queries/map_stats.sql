-- name: DailyUserCommented :one
select commented from map_daily_users where map_id=$1::uuid and day=current_date and user_id=$2::uuid;

-- name: DailyUserFavorited :one
select favorited from map_daily_users where map_id=$1::uuid and day=current_date and user_id=$2::uuid;

-- name: DailyUserPlayed :one
select played from map_daily_users where map_id=$1::uuid and day=current_date and user_id=$2::uuid;

-- name: IncrementDailyCommented :exec
insert into map_stats_daily(map_id,day,comments,unique_commenters) values($1::uuid,current_date,1,$2) on conflict(map_id,day) do update set comments=map_stats_daily.comments+1,unique_commenters=map_stats_daily.unique_commenters+$2;

-- name: IncrementDailyFavorited :exec
insert into map_stats_daily(map_id,day,favorites,unique_favoriters) values($1::uuid,current_date,1,$2) on conflict(map_id,day) do update set favorites=map_stats_daily.favorites+1,unique_favoriters=map_stats_daily.unique_favoriters+$2;

-- name: IncrementDailyPlayed :exec
insert into map_stats_daily(map_id,day,plays,unique_players) values($1::uuid,current_date,1,$2) on conflict(map_id,day) do update set plays=map_stats_daily.plays+1,unique_players=map_stats_daily.unique_players+$2;

-- name: IncrementMapComment :exec
update maps set comment_count=comment_count+1, updated_at=now() where id=$1::uuid;

-- name: IncrementMapFavorite :exec
update maps set favorite_count=favorite_count+1, updated_at=now() where id=$1::uuid;

-- name: TrendingInputs :one
select coalesce(sum(unique_players)*3+sum(unique_favoriters)*8+sum(unique_commenters)*2,0)::float8, coalesce(sum(unique_players)+sum(unique_favoriters)+sum(unique_commenters),0)::float8, (select published_at from maps where id=$1::uuid) from map_stats_daily where map_id=$1::uuid and day >= current_date - $2::int;

-- name: UpdateTrendingScore :exec
update maps set trending_score=$2 where id=$1::uuid;

-- name: UpsertDailyCommented :exec
insert into map_daily_users(map_id,day,user_id,commented) values($1::uuid,current_date,$2::uuid,true) on conflict(map_id,day,user_id) do update set commented=true;

-- name: UpsertDailyFavorited :exec
insert into map_daily_users(map_id,day,user_id,favorited) values($1::uuid,current_date,$2::uuid,true) on conflict(map_id,day,user_id) do update set favorited=true;

-- name: UpsertDailyPlayed :exec
insert into map_daily_users(map_id,day,user_id,played) values($1::uuid,current_date,$2::uuid,true) on conflict(map_id,day,user_id) do update set played=true;
