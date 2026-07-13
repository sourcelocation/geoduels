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
		&item.AutoZoomPlayRegion,
		&item.RankedMoving,
		&item.RankedNMPZ,
		&item.DefaultMoving,
		&item.DefaultNMPZ,
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

func (s *pgStore) mapCountryStats(ctx context.Context, mapID string) ([]contracts.MapCountryStat, error) {
	if strings.TrimSpace(mapID) == "" {
		return nil, nil
	}
	rows, err := s.pool.Query(ctx, `
		select country, location_count
		from map_country_stats
		where map_id=$1
		order by location_count desc, country asc
		limit 64
	`, strings.TrimSpace(mapID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []contracts.MapCountryStat{}
	for rows.Next() {
		var item contracts.MapCountryStat
		if err := rows.Scan(&item.Country, &item.LocationCount); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
