package persistence

import (
	"context"

	db "geoduels/pkg/persistence/sqlc/db"
)

func (s *DB) ListLeaderboard(ctx context.Context, mode, seasonID string, limit, offset int) ([]LeaderboardEntry, error) {
	if mode == "" {
		mode = modeDuel
	}
	if limit <= 0 {
		limit = 100
	}
	if limit > 200 {
		limit = 200
	}
	if offset < 0 {
		offset = 0
	}
	if seasonID == "" {
		var err error
		seasonID, err = s.db.GetActiveSeasonID(ctx)
		if err != nil {
			return nil, err
		}
	}
	rows, err := s.db.ListLeaderboard(ctx, db.ListLeaderboardParams{Mode: db.GdMatchMode(mode), SeasonID: seasonID, LimitCount: int32(limit), OffsetCount: int32(offset)})
	if err != nil {
		return nil, err
	}
	entries := make([]LeaderboardEntry, 0, limit)
	for _, row := range rows {
		entries = append(entries, LeaderboardEntry{Rank: int(row.Rank), UserID: row.RUserID, DisplayName: row.DisplayName.String, AvatarURL: row.AvatarUrl, MMR: int(row.Mmr), GamesPlayed: int(row.GamesPlayed), Wins: int(row.Wins)})
	}
	return entries, nil
}

func (s *DB) GetLeaderboardOverview(ctx context.Context, userID, mode, seasonID string, limit int) (LeaderboardOverview, error) {
	if mode == "" {
		mode = modeDuel
	}
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}
	entries, err := s.ListLeaderboard(ctx, mode, seasonID, limit, 0)
	if err != nil {
		return LeaderboardOverview{}, err
	}
	if seasonID == "" {
		seasonID, err = s.db.GetActiveSeasonID(ctx)
		if err != nil {
			return LeaderboardOverview{}, err
		}
	}
	totals, err := s.db.GetLeaderboardTotals(ctx, db.GetLeaderboardTotalsParams{UserID: userID, Mode: db.GdMatchMode(mode), SeasonID: seasonID})
	if err != nil {
		return LeaderboardOverview{}, err
	}
	return LeaderboardOverview{Mode: mode, SeasonID: seasonID, SelfRank: int(totals.SelfRank), TotalPlayers: int(totals.TotalPlayers), Entries: entries}, nil
}
