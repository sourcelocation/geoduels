package persistence

import (
	"context"
	"database/sql"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

func incrementMapFavoriteStats(ctx context.Context, tx pgx.Tx, mapID, userID string) error {
	if _, err := tx.Exec(ctx, `update maps set favorite_count=favorite_count+1, updated_at=now() where id=$1`, mapID); err != nil {
		return err
	}
	return markMapDailyUser(ctx, tx, mapID, userID, "favorited")
}

func incrementMapCommentStats(ctx context.Context, tx pgx.Tx, mapID, userID string) error {
	if _, err := tx.Exec(ctx, `update maps set comment_count=comment_count+1, updated_at=now() where id=$1`, mapID); err != nil {
		return err
	}
	return markMapDailyUser(ctx, tx, mapID, userID, "commented")
}

func markMapDailyUser(ctx context.Context, tx pgx.Tx, mapID, userID, field string) error {
	field = strings.TrimSpace(field)
	if field != "played" && field != "favorited" && field != "commented" {
		return errors.New("invalid map stat field")
	}
	var already bool
	if err := tx.QueryRow(ctx, `select `+field+` from map_daily_users where map_id=$1 and day=current_date and user_id=$2`, mapID, userID).Scan(&already); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if _, err := tx.Exec(ctx, `
		insert into map_daily_users(map_id, day, user_id, `+field+`)
		values($1, current_date, $2, true)
		on conflict(map_id, day, user_id) do update set `+field+`=true
	`, mapID, userID); err != nil {
		return err
	}
	rawField := map[string]string{"played": "plays", "favorited": "favorites", "commented": "comments"}[field]
	uniqueField := map[string]string{"played": "unique_players", "favorited": "unique_favoriters", "commented": "unique_commenters"}[field]
	uniqueIncrement := 0
	if !already {
		uniqueIncrement = 1
	}
	if _, err := tx.Exec(ctx, `
		insert into map_stats_daily(map_id, day, `+rawField+`, `+uniqueField+`)
		values($1, current_date, 1, $2)
		on conflict(map_id, day) do update set `+rawField+`=map_stats_daily.`+rawField+`+1, `+uniqueField+`=map_stats_daily.`+uniqueField+`+$2
	`, mapID, uniqueIncrement); err != nil {
		return err
	}
	return refreshMapTrendingScore(ctx, tx, mapID)
}

func refreshMapTrendingScore(ctx context.Context, tx pgx.Tx, mapID string) error {
	var base, interactions float64
	var published sql.NullTime
	if err := tx.QueryRow(ctx, `
		select
			coalesce(sum(unique_players) * 3 + sum(unique_favoriters) * 8 + sum(unique_commenters) * 2, 0)::float8,
			coalesce(sum(unique_players) + sum(unique_favoriters) + sum(unique_commenters), 0)::float8,
			(select published_at from maps where id=$1)
		from map_stats_daily
		where map_id=$1 and day >= current_date - $2::int
	`, mapID, mapTrendingWindowDays).Scan(&base, &interactions, &published); err != nil {
		return err
	}
	score := base
	if published.Valid && interactions >= 3 {
		age := time.Since(published.Time)
		if age < 0 {
			age = 0
		}
		decay := 1 - math.Min(age.Hours()/(24*7), 1)
		score = base * (1 + 19*decay)
	}
	_, err := tx.Exec(ctx, `update maps set trending_score=$2 where id=$1`, mapID, score)
	return err
}
