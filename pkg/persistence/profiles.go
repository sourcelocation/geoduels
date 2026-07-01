package persistence

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"geoduels/pkg/contracts"
)

func (s *pgStore) UpsertUser(userID, email, displayName string) error {
	if userID == "" {
		return errors.New("user id required")
	}
	if displayName == "" {
		displayName = userID
	}
	var nullableEmail any
	if email != "" {
		nullableEmail = email
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)
	seasonID, err := activeSeasonIDTx(ctx, tx)
	if err != nil {
		return err
	}

	if _, err := tx.Exec(ctx, `
		insert into users (id, email, display_name, avatar_url, account_type)
		values ($1, $2, $3, null, 'guest')
		on conflict (id) do update set
			email = excluded.email,
			display_name = excluded.display_name
	`, userID, nullableEmail, displayName); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		insert into ranks (user_id, mode, mmr, season_id)
		values ($1, $2, $4, $3)
		on conflict (user_id, mode, season_id) do nothing
	`, userID, modeDuel, seasonID, initialMMR); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		insert into user_stats (user_id, games_played, wins)
		values ($1, 0, 0)
		on conflict (user_id) do nothing
	`, userID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
		insert into ranked_stats (user_id, mode, season_id, games_played, wins)
		values ($1, $2, $3, 0, 0)
		on conflict (user_id, mode, season_id) do nothing
	`, userID, modeDuel, seasonID); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *pgStore) GetProfile(userID string) (Profile, error) {
	p := Profile{UserID: userID, DisplayName: userID, MMR: initialMMR, RatingRD: initialRatingRD}
	if userID == "" {
		return p, errors.New("user id required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	seasonID, err := s.activeSeasonID(ctx)
	if err != nil {
		return p, err
	}
	p.SeasonID = seasonID
	row := s.pool.QueryRow(ctx, `
		select
			coalesce(nullif(u.display_name, seed.user_id::text), ui.provider_name, $1::text) as display_name,
			coalesce(u.avatar_url, ui.avatar_url, '') as avatar_url,
			coalesce(r.mmr, $4) as mmr,
			coalesce(r.rd, $5) as rating_rd,
			greatest(coalesce(us.games_played, 0), coalesce(history_stats.games_played, 0)) as games_played,
			greatest(coalesce(us.wins, 0), coalesce(history_stats.wins, 0)) as wins,
			coalesce(rs.games_played, 0) as ranked_games_played,
				coalesce(rs.wins, 0) as ranked_wins,
				coalesce(u.account_type = 'guest', false) as is_guest,
				coalesce(u.is_admin, false) as is_admin,
				coalesce(u.is_moderator, false) as is_moderator,
				coalesce(u.banned_at is not null and (u.ban_expires_at is null or u.ban_expires_at > now()), false) as is_banned,
				coalesce(u.ban_reason, '') as ban_reason,
				coalesce(u.selected_badge_code, 0) as selected_badge_code,
				coalesce(u.selected_badge_season_id, '') as selected_badge_season_id
		from (select $1::uuid as user_id) seed
		left join users u on u.id = seed.user_id
		left join lateral (
			select provider_name, avatar_url
			from user_identities
			where user_id = seed.user_id and provider = 'google'
			order by created_at asc
			limit 1
		) ui on true
		left join ranks r on r.user_id = seed.user_id and r.mode = $2 and r.season_id = $3
		left join user_stats us on us.user_id = seed.user_id
		left join lateral (
			select
				count(*)::int as games_played,
				count(*) filter (where h.winner_user_id = seed.user_id)::int as wins
			from match_players mp
			join match_history h on h.match_id = mp.match_id
			where mp.user_id = seed.user_id
			  and h.mode = 'duel'
		) history_stats on true
		left join ranked_stats rs on rs.user_id = seed.user_id and rs.mode = $2 and rs.season_id = $3
	`, userID, modeDuel, seasonID, initialMMR, initialRatingRD)
	var selectedBadgeCode int16
	var selectedBadgeSeasonID string
	if err := row.Scan(
		&p.DisplayName,
		&p.AvatarURL,
		&p.MMR,
		&p.RatingRD,
		&p.GamesPlayed,
		&p.Wins,
		&p.RankedGamesPlayed,
		&p.RankedWins,
		&p.IsGuest,
		&p.IsAdmin,
		&p.IsModerator,
		&p.IsBanned,
		&p.BanReason,
		&selectedBadgeCode,
		&selectedBadgeSeasonID,
	); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return p, nil
		}
		return p, err
	}
	badges, selected, err := s.profileBadges(ctx, userID, badgeIDFromParts(selectedBadgeCode, selectedBadgeSeasonID))
	if err != nil {
		return p, err
	}
	p.Badges = badges
	p.SelectedBadge = selected
	return p, nil
}

func (s *pgStore) GetPublicPlayerProfileByNickname(nickname string) (PublicPlayerProfile, error) {
	nickname = strings.TrimSpace(nickname)
	p := PublicPlayerProfile{
		DisplayName: nickname,
		MMR:         initialMMR,
		RatingRD:    initialRatingRD,
	}
	if nickname == "" {
		return p, errors.New("nickname required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	seasonID, err := s.activeSeasonID(ctx)
	if err != nil {
		return p, err
	}
	p.SeasonID = seasonID
	var selectedBadgeCode int16
	var selectedBadgeSeasonID string
	err = s.pool.QueryRow(ctx, `
		select
			u.id::text,
			coalesce(nullif(u.display_name, ''), ui.provider_name, u.id::text),
			coalesce(u.avatar_url, ui.avatar_url, ''),
			coalesce(r.mmr, $4),
			coalesce(r.rd, $5),
			greatest(coalesce(us.games_played, 0), coalesce(history_stats.games_played, 0)),
			greatest(coalesce(us.wins, 0), coalesce(history_stats.wins, 0)),
			coalesce(rs.games_played, 0),
			coalesce(rs.wins, 0),
			coalesce(u.selected_badge_code, 0),
			coalesce(u.selected_badge_season_id, '')
		from users u
		left join lateral (
			select provider_name, avatar_url
			from user_identities
			where user_id = u.id and provider = 'google'
			order by created_at asc
			limit 1
		) ui on true
		left join ranks r on r.user_id = u.id and r.mode = $2 and r.season_id = $3
		left join user_stats us on us.user_id = u.id
		left join lateral (
			select
				count(*)::int as games_played,
				count(*) filter (where h.winner_user_id = u.id)::int as wins
			from match_players mp
			join match_history h on h.match_id = mp.match_id
			where mp.user_id = u.id
			  and h.mode = 'duel'
		) history_stats on true
		left join ranked_stats rs on rs.user_id = u.id and rs.mode = $2 and rs.season_id = $3
		where u.account_type = 'registered'
		  and u.nickname_claimed_at is not null
		  and lower(u.display_name) = lower($1)
	`, nickname, modeDuel, seasonID, initialMMR, initialRatingRD).Scan(
		&p.UserID,
		&p.DisplayName,
		&p.AvatarURL,
		&p.MMR,
		&p.RatingRD,
		&p.GamesPlayed,
		&p.Wins,
		&p.RankedGamesPlayed,
		&p.RankedWins,
		&selectedBadgeCode,
		&selectedBadgeSeasonID,
	)
	if err != nil {
		return p, err
	}
	badges, selected, err := s.profileBadges(ctx, p.UserID, badgeIDFromParts(selectedBadgeCode, selectedBadgeSeasonID))
	if err != nil {
		return p, err
	}
	p.Badges = ownedPlayerBadges(badges)
	p.SelectedBadge = selected
	return p, nil
}

func ownedPlayerBadges(badges []contracts.PlayerBadge) []contracts.PlayerBadge {
	owned := make([]contracts.PlayerBadge, 0, len(badges))
	for _, badge := range badges {
		if badge.Owned {
			owned = append(owned, badge)
		}
	}
	return owned
}

func (s *pgStore) UpdateSelectedBadge(userID, badgeID string) (Profile, error) {
	userID = strings.TrimSpace(userID)
	badgeID = strings.TrimSpace(badgeID)
	if userID == "" {
		return Profile{}, errors.New("user id required")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	badges, _, err := s.profileBadges(ctx, userID, badgeID)
	if err != nil {
		return Profile{}, err
	}
	if badgeID != "" {
		owned := false
		for _, badge := range badges {
			if badge.ID == badgeID && badge.Owned {
				owned = true
				break
			}
		}
		if !owned {
			return Profile{}, errors.New("badge unavailable")
		}
	}
	ref, ok := badgeRefFromID(badgeID)
	if badgeID != "" && !ok {
		return Profile{}, errors.New("badge unavailable")
	}
	if _, err := s.pool.Exec(ctx, `
		update users
		set selected_badge_code = nullif($2, 0),
			selected_badge_season_id = $3
		where id = $1
	`, userID, ref.Code, ref.SeasonID); err != nil {
		return Profile{}, err
	}
	return s.GetProfile(userID)
}

func (s *pgStore) profileBadges(ctx context.Context, userID, selectedBadgeID string) ([]contracts.PlayerBadge, *contracts.PlayerBadge, error) {
	badges := []contracts.PlayerBadge{}
	rows, err := s.pool.Query(ctx, `
		select ub.badge_code, coalesce(ub.badge_season_id, ''), coalesce(ub.rank, 0)
		from user_badges ub
		where ub.user_id = $1
		order by ub.awarded_at desc, ub.badge_code asc, ub.badge_season_id asc
	`, userID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	owned := map[string]bool{}
	for rows.Next() {
		var code int16
		var seasonID string
		var rank int
		if err := rows.Scan(&code, &seasonID, &rank); err != nil {
			return nil, nil, err
		}
		if code == badgeCodeSeasonRank {
			continue
		}
		badge, ok := badgeFromParts(code, seasonID, rank, true)
		if !ok {
			continue
		}
		owned[badge.ID] = true
		badges = append(badges, badge)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, err
	}
	seasonBadges, err := s.earnedSeasonRankBadges(ctx, []string{userID})
	if err != nil {
		return nil, nil, err
	}
	for _, badge := range seasonBadges[userID] {
		if owned[badge.ID] {
			continue
		}
		owned[badge.ID] = true
		badges = append(badges, badge)
	}
	for _, badge := range badgeTemplates() {
		if !owned[badge.ID] {
			badges = append(badges, badge)
		}
	}
	var selected *contracts.PlayerBadge
	for i := range badges {
		if badges[i].ID == selectedBadgeID && badges[i].Owned {
			selected = &badges[i]
			break
		}
	}
	if selected == nil && selectedBadgeID != "" {
		for i := range badges {
			if badges[i].Owned {
				selected = &badges[i]
				break
			}
		}
	}
	return badges, selected, nil
}
