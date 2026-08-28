package persistence

import (
	"context"
	"database/sql"
	"strings"
	"time"

	"geoduels/pkg/contracts"
)

type customMapScanner interface {
	Scan(dest ...any) error
}

func scanCustomMap(row customMapScanner) (contracts.CustomMap, error) {
	var item contracts.CustomMap
	var publishedAt time.Time
	var bestScore sql.NullInt32
	var bestMatch string
	var bestAt sql.NullTime
	if err := row.Scan(
		&item.ID,
		&item.MapKey,
		&item.OwnerUserID,
		&item.AuthorName,
		&item.DisplayName,
		&item.Description,
		&item.Visibility,
		&item.Status,
		&item.Difficulty,
		&item.ThumbnailVariant,
		&item.ThumbnailKey,
		&item.LocationCount,
		&item.System,
		&item.Official,
		&publishedAt,
		&item.PlayCount,
		&item.FavoriteCount,
		&item.CommentCount,
		&item.TrendingScore,
		&item.Favorited,
		&item.OfficialRegion,
		&item.ModeMoving,
		&item.ModeNoMove,
		&item.ModeNMPZ,
		&item.CreatedAt,
		&item.UpdatedAt,
		&bestScore,
		&bestMatch,
		&bestAt,
	); err != nil {
		return contracts.CustomMap{}, err
	}
	if !publishedAt.IsZero() && publishedAt.Year() > 1 {
		item.PublishedAt = &publishedAt
	}
	if bestScore.Valid && bestAt.Valid {
		item.PersonalBest = &contracts.MapPersonalBest{Score: int(bestScore.Int32), MatchID: bestMatch, AchievedAt: bestAt.Time}
	}
	return item, nil
}

func (s *DB) mapCountryStats(ctx context.Context, mapID string) ([]contracts.MapCountryStat, error) {
	if strings.TrimSpace(mapID) == "" {
		return nil, nil
	}
	rows, err := s.db.ListMapCountryStats(ctx, chatUUID(strings.TrimSpace(mapID)))
	if err != nil {
		return nil, err
	}
	out := make([]contracts.MapCountryStat, 0, len(rows))
	for _, row := range rows {
		out = append(out, contracts.MapCountryStat{Country: row.Country, LocationCount: int(row.LocationCount)})
	}
	return out, nil
}
