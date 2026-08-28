package main

import (
	"context"

	"geoduels/pkg/leaderboard"
	"geoduels/pkg/persistence"
)

type leaderboardStoreAdapter struct {
	store persistence.LeaderboardRepository
}

func (a leaderboardStoreAdapter) ListLeaderboard(ctx context.Context, mode, season string, limit, offset int) ([]leaderboard.Entry, error) {
	rows, err := a.store.ListLeaderboard(ctx, mode, season, limit, offset)
	out := make([]leaderboard.Entry, 0, len(rows))
	for _, row := range rows {
		out = append(out, leaderboard.Entry{Rank: row.Rank, UserID: row.UserID, DisplayName: row.DisplayName, AvatarURL: row.AvatarURL, MMR: row.MMR, GamesPlayed: row.GamesPlayed, Wins: row.Wins})
	}
	return out, err
}
func (a leaderboardStoreAdapter) GetLeaderboardOverview(ctx context.Context, userID, mode, season string, limit int) (leaderboard.Overview, error) {
	row, err := a.store.GetLeaderboardOverview(ctx, userID, mode, season, limit)
	out := leaderboard.Overview{Mode: row.Mode, SeasonID: row.SeasonID, SelfRank: row.SelfRank, TotalPlayers: row.TotalPlayers}
	for _, e := range row.Entries {
		out.Entries = append(out.Entries, leaderboard.Entry{Rank: e.Rank, UserID: e.UserID, DisplayName: e.DisplayName, AvatarURL: e.AvatarURL, MMR: e.MMR, GamesPlayed: e.GamesPlayed, Wins: e.Wins})
	}
	return out, err
}
func newLeaderboardService(store persistence.LeaderboardRepository) *leaderboard.Service {
	return leaderboard.NewService(leaderboardStoreAdapter{store: store})
}
