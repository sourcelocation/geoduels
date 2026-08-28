-- name: DailyUserCommented :one
select commented from map_daily_users where map_id=sqlc.arg(map_id)::uuid and day=current_date and user_id=sqlc.arg(user_id)::uuid;

-- name: DailyUserFavorited :one
select favorited from map_daily_users where map_id=sqlc.arg(map_id)::uuid and day=current_date and user_id=sqlc.arg(user_id)::uuid;

-- name: DailyUserPlayed :one
select played from map_daily_users where map_id=sqlc.arg(map_id)::uuid and day=current_date and user_id=sqlc.arg(user_id)::uuid;

-- name: IncrementDailyCommented :exec
insert into map_stats_daily(map_id,day,comments,unique_commenters) values(sqlc.arg(map_id)::uuid,current_date,1,sqlc.arg(increment)) on conflict(map_id,day) do update set comments=map_stats_daily.comments+1,unique_commenters=map_stats_daily.unique_commenters+sqlc.arg(increment);

-- name: IncrementDailyFavorited :exec
insert into map_stats_daily(map_id,day,favorites,unique_favoriters) values(sqlc.arg(map_id)::uuid,current_date,1,sqlc.arg(increment)) on conflict(map_id,day) do update set favorites=map_stats_daily.favorites+1,unique_favoriters=map_stats_daily.unique_favoriters+sqlc.arg(increment);

-- name: IncrementDailyPlayed :exec
insert into map_stats_daily(map_id,day,plays,unique_players) values(sqlc.arg(map_id)::uuid,current_date,1,sqlc.arg(increment)) on conflict(map_id,day) do update set plays=map_stats_daily.plays+1,unique_players=map_stats_daily.unique_players+sqlc.arg(increment);

-- name: IncrementMapComment :exec
update maps set comment_count=comment_count+1, updated_at=now() where id=sqlc.arg(map_id)::uuid;

-- name: IncrementMapFavorite :exec
update maps set favorite_count=favorite_count+1, updated_at=now() where id=sqlc.arg(map_id)::uuid;

-- name: TrendingInputs :one
select coalesce(sum(unique_players)*3+sum(unique_favoriters)*8+sum(unique_commenters)*2,0)::float8 AS weighted_score, coalesce(sum(unique_players)+sum(unique_favoriters)+sum(unique_commenters),0)::float8 AS activity_total, (select published_at from maps where id=sqlc.arg(map_id)::uuid) AS published_at from map_stats_daily where map_id=sqlc.arg(map_id)::uuid and day >= current_date - sqlc.arg(window_days)::int;

-- name: UpdateTrendingScore :exec
update maps set trending_score=sqlc.arg(trending_score) where id=sqlc.arg(map_id)::uuid;

-- name: UpsertDailyCommented :exec
insert into map_daily_users(map_id,day,user_id,commented) values(sqlc.arg(map_id)::uuid,current_date,sqlc.arg(user_id)::uuid,true) on conflict(map_id,day,user_id) do update set commented=true;

-- name: UpsertDailyFavorited :exec
insert into map_daily_users(map_id,day,user_id,favorited) values(sqlc.arg(map_id)::uuid,current_date,sqlc.arg(user_id)::uuid,true) on conflict(map_id,day,user_id) do update set favorited=true;

-- name: UpsertDailyPlayed :exec
insert into map_daily_users(map_id,day,user_id,played) values(sqlc.arg(map_id)::uuid,current_date,sqlc.arg(user_id)::uuid,true) on conflict(map_id,day,user_id) do update set played=true;
