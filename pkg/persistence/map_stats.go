package persistence

import (
	"context"
	"errors"
	db "geoduels/pkg/persistence/sqlc/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"math"
	"time"
)

func mapUUID(s string) (pgtype.UUID, error) { var u pgtype.UUID; return u, u.Scan(s) }
func incrementMapFavoriteStats(c context.Context, t pgx.Tx, m, u string) error {
	q := db.New(t)
	id, e := mapUUID(m)
	if e != nil {
		return e
	}
	if e = q.IncrementMapFavorite(c, id); e != nil {
		return e
	}
	return markMapDailyUser(c, t, m, u, "favorited")
}
func incrementMapCommentStats(c context.Context, t pgx.Tx, m, u string) error {
	q := db.New(t)
	id, e := mapUUID(m)
	if e != nil {
		return e
	}
	if e = q.IncrementMapComment(c, id); e != nil {
		return e
	}
	return markMapDailyUser(c, t, m, u, "commented")
}
func markMapDailyUser(c context.Context, t pgx.Tx, m, u, f string) error {
	q := db.New(t)
	id, e := mapUUID(m)
	if e != nil {
		return e
	}
	uid, e := mapUUID(u)
	if e != nil {
		return e
	}
	var a bool
	switch f {
	case "played":
		a, e = q.DailyUserPlayed(c, db.DailyUserPlayedParams{MapID: id, UserID: uid})
	case "favorited":
		a, e = q.DailyUserFavorited(c, db.DailyUserFavoritedParams{MapID: id, UserID: uid})
	case "commented":
		a, e = q.DailyUserCommented(c, db.DailyUserCommentedParams{MapID: id, UserID: uid})
	default:
		return errors.New("invalid map stat field")
	}
	if e != nil && e != pgx.ErrNoRows {
		return e
	}
	n := int32(0)
	if !a {
		n = 1
	}
	switch f {
	case "played":
		e = q.UpsertDailyPlayed(c, db.UpsertDailyPlayedParams{MapID: id, UserID: uid})
		if e == nil {
			e = q.IncrementDailyPlayed(c, db.IncrementDailyPlayedParams{MapID: id, Increment: n})
		}
	case "favorited":
		e = q.UpsertDailyFavorited(c, db.UpsertDailyFavoritedParams{MapID: id, UserID: uid})
		if e == nil {
			e = q.IncrementDailyFavorited(c, db.IncrementDailyFavoritedParams{MapID: id, Increment: n})
		}
	case "commented":
		e = q.UpsertDailyCommented(c, db.UpsertDailyCommentedParams{MapID: id, UserID: uid})
		if e == nil {
			e = q.IncrementDailyCommented(c, db.IncrementDailyCommentedParams{MapID: id, Increment: n})
		}
	}
	if e != nil {
		return e
	}
	return refreshMapTrendingScore(c, t, m)
}
func refreshMapTrendingScore(c context.Context, t pgx.Tx, m string) error {
	q := db.New(t)
	id, e := mapUUID(m)
	if e != nil {
		return e
	}
	r, e := q.TrendingInputs(c, db.TrendingInputsParams{MapID: id, WindowDays: mapTrendingWindowDays})
	if e != nil {
		return e
	}
	s := r.WeightedScore
	if r.ActivityTotal >= 3 && r.PublishedAt.Valid {
		x := time.Since(r.PublishedAt.Time)
		if x < 0 {
			x = 0
		}
		s *= 1 + 19*(1-math.Min(x.Hours()/(24*7), 1))
	}
	return q.UpdateTrendingScore(c, db.UpdateTrendingScoreParams{MapID: id, TrendingScore: s})
}
