package persistence

import (
	"context"
	"time"
)

func (s *pgStore) ListLeaderboard(mode, seasonID string, limit, offset int) ([]LeaderboardEntry, error) {
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

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	if seasonID == "" {
		var err error
		seasonID, err = s.activeSeasonID(ctx)
		if err != nil {
			return nil, err
		}
	}

	rows, err := s.pool.Query(ctx, `
		select
			row_number() over (
				order by r.mmr desc, r.updated_at asc, r.user_id asc
			) as rank,
			r.user_id,
			coalesce(nullif(u.display_name, r.user_id::text), ui.provider_name, r.user_id::text) as display_name,
			coalesce(u.avatar_url, ui.avatar_url, '') as avatar_url,
			r.mmr,
			coalesce(rs.games_played, 0) as games_played,
			coalesce(rs.wins, 0) as wins
		from ranks r
		left join users u on u.id = r.user_id
		left join lateral (
			select provider_name, avatar_url
			from user_identities
			where user_id = r.user_id and provider = 'google'
			order by created_at asc
			limit 1
		) ui on true
		left join ranked_stats rs on rs.user_id = r.user_id and rs.mode = r.mode and rs.season_id = r.season_id
		where r.mode = $1
			and r.season_id = $2
			and coalesce(u.account_type, 'registered') <> 'guest'
			and not coalesce(u.banned_at is not null and (u.ban_expires_at is null or u.ban_expires_at > now()), false)
		order by r.mmr desc, r.updated_at asc, r.user_id asc
		limit $3 offset $4
	`, mode, seasonID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	entries := make([]LeaderboardEntry, 0, limit)
	for rows.Next() {
		var entry LeaderboardEntry
		if err := rows.Scan(
			&entry.Rank,
			&entry.UserID,
			&entry.DisplayName,
			&entry.AvatarURL,
			&entry.MMR,
			&entry.GamesPlayed,
			&entry.Wins,
		); err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return entries, nil
}

func (s *pgStore) GetLeaderboardOverview(userID, mode, seasonID string, limit int) (LeaderboardOverview, error) {
	if mode == "" {
		mode = modeDuel
	}
	if limit <= 0 {
		limit = 10
	}
	if limit > 100 {
		limit = 100
	}

	entries, err := s.ListLeaderboard(mode, seasonID, limit, 0)
	if err != nil {
		return LeaderboardOverview{}, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	if seasonID == "" {
		seasonID, err = s.activeSeasonID(ctx)
		if err != nil {
			return LeaderboardOverview{}, err
		}
	}

	var selfRank, totalPlayers int
	if err := s.pool.QueryRow(ctx, `
		with ranked as (
			select
				r.user_id,
				row_number() over (
					order by r.mmr desc, r.updated_at asc, r.user_id asc
				) as rank,
				count(*) over () as total_players
				from ranks r
				left join users u on u.id = r.user_id
				where r.mode = $1
					and r.season_id = $2
					and coalesce(u.account_type, 'registered') <> 'guest'
					and not coalesce(u.banned_at is not null and (u.ban_expires_at is null or u.ban_expires_at > now()), false)
			)
		select
			coalesce(max(rank) filter (where user_id = nullif($3,'')::uuid), 0) as self_rank,
			coalesce(max(total_players), 0) as total_players
		from ranked
	`, mode, seasonID, userID).Scan(&selfRank, &totalPlayers); err != nil {
		return LeaderboardOverview{}, err
	}

	return LeaderboardOverview{
		Mode:         mode,
		SeasonID:     seasonID,
		SelfRank:     selfRank,
		TotalPlayers: totalPlayers,
		Entries:      entries,
	}, nil
}
