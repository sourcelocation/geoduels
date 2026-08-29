package persistence

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	"geoduels/pkg/contracts"
	db "geoduels/pkg/persistence/sqlc/db"
)

func (s *DB) UpsertUser(userID, email, displayName string) error {
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

	id, err := profileUUID(userID)
	if err != nil {
		return err
	}
	q := s.db.WithTx(tx)
	if err := q.UpsertUser(ctx, db.UpsertUserParams{ID: id, Email: profileText(nullableEmail), DisplayName: displayName}); err != nil {
		return err
	}
	if err := q.EnsureUserRank(ctx, db.EnsureUserRankParams{UserID: id, Mode: modeDuel, SeasonID: seasonID, Mmr: initialMMR}); err != nil {
		return err
	}
	if err := q.EnsureUserStats(ctx, id); err != nil {
		return err
	}
	if err := q.EnsureRankedStats(ctx, db.EnsureRankedStatsParams{UserID: id, Mode: modeDuel, SeasonID: seasonID}); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *DB) GetProfile(userID string) (Profile, error) {
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
	row, err := s.db.GetProfile(ctx, db.GetProfileParams{
		UserID: userID, Mode: modeDuel, SeasonID: seasonID, DefaultMmr: initialMMR, DefaultRd: initialRatingRD,
	})
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return p, nil
		}
		return p, err
	}
	var selectedBadgeCode int16
	p.DisplayName, p.AvatarURL, p.MMR, p.RatingRD = row.DisplayName.String, row.AvatarUrl, int(row.Mmr), row.RatingRd
	p.GamesPlayed, p.Wins, p.RankedGamesPlayed, p.RankedWins = int(row.GamesPlayed.(int32)), int(row.Wins.(int32)), int(row.RankedGamesPlayed), int(row.RankedWins)
	p.IsGuest, p.IsAdmin, p.IsModerator, p.IsBanned, p.BanReason = row.IsGuest.(bool), row.IsAdmin, row.IsModerator, row.IsBanned.(bool), row.BanReason
	selectedBadgeCode = row.SelectedBadgeCode
	badges, selected, err := s.profileBadges(ctx, userID, badgeIDFromCode(selectedBadgeCode))
	if err != nil {
		return p, err
	}
	p.Badges = badges
	p.SelectedBadge = selected
	return p, nil
}

func (s *DB) GetPublicPlayerProfileByNickname(nickname string) (PublicPlayerProfile, error) {
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
	err = func() error {
		row, e := s.db.GetPublicProfile(ctx, db.GetPublicProfileParams{DisplayName: nickname, Mode: modeDuel, SeasonID: seasonID, DefaultMmr: initialMMR, DefaultRd: initialRatingRD})
		if e != nil {
			return e
		}
		p.UserID, p.DisplayName, p.AvatarURL, p.MMR, p.RatingRD, p.GamesPlayed, p.Wins, p.RankedGamesPlayed, p.RankedWins, selectedBadgeCode = uuidVal(row.UserID), row.DisplayName.String, row.AvatarUrl, int(row.Mmr), row.RatingRd, int(row.GamesPlayed), int(row.Wins), int(row.RankedGamesPlayed), int(row.RankedWins), row.SelectedBadgeCode
		return nil
	}()
	if err != nil {
		return p, err
	}
	settings, err := rankedSeasonSettingsTx(ctx, s.pool)
	if err != nil {
		return p, err
	}
	var seasonStartedAt any
	if settings.LastResetAt != nil {
		seasonStartedAt = *settings.LastResetAt
	}
	id, e := profileUUID(p.UserID)
	if e != nil {
		return p, e
	}
	pos, e := s.db.GetLeaderboardPosition(ctx, db.GetLeaderboardPositionParams{UserID: id, Mode: modeDuel, SeasonID: seasonID})
	if e != nil {
		return p, e
	}
	total, e := s.db.GetLeaderboardTotal(ctx, db.GetLeaderboardTotalParams{Mode: modeDuel, SeasonID: seasonID})
	if e != nil {
		return p, e
	}
	streak, e := s.db.GetBestWinStreak(ctx, db.GetBestWinStreakParams{UserID: id, Since: profileTimestamptz(seasonStartedAt)})
	if e != nil {
		return p, e
	}
	perfect, e := s.db.GetPerfectGuesses(ctx, id)
	if e != nil {
		return p, e
	}
	flawless, e := s.db.GetFlawlessWins(ctx, id)
	if e != nil {
		return p, e
	}
	p.LeaderboardRank, p.LeaderboardTotal, p.BestWinStreak, p.PerfectGuesses, p.FlawlessWins = int(pos), int(total), int(streak), int(perfect), int(flawless)
	badges, selected, err := s.profileBadges(ctx, p.UserID, badgeIDFromCode(selectedBadgeCode))
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

func profileUUID(v string) (pgtype.UUID, error) {
	var u pgtype.UUID
	if err := u.Scan(v); err != nil {
		return u, err
	}
	return u, nil
}

func profileText(v any) pgtype.Text {
	if v == nil {
		return pgtype.Text{}
	}
	return pgtype.Text{String: v.(string), Valid: true}
}

func profileTimestamptz(v any) pgtype.Timestamptz {
	if v == nil {
		return pgtype.Timestamptz{}
	}
	return pgtype.Timestamptz{Time: v.(time.Time), Valid: true}
}

func (s *DB) UpdateSelectedBadge(userID, badgeID string) (Profile, error) {
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
	code, ok := badgeRefFromID(badgeID)
	if badgeID != "" && !ok {
		return Profile{}, errors.New("badge unavailable")
	}
	id, err := profileUUID(userID)
	if err != nil {
		return Profile{}, err
	}
	if err := s.db.UpdateSelectedBadge(ctx, db.UpdateSelectedBadgeParams{UserID: id, BadgeCode: code}); err != nil {
		return Profile{}, err
	}
	return s.GetProfile(userID)
}

func (s *DB) profileBadges(ctx context.Context, userID, selectedBadgeID string) ([]contracts.PlayerBadge, *contracts.PlayerBadge, error) {
	badges := []contracts.PlayerBadge{}
	id, err := profileUUID(userID)
	if err != nil {
		return nil, nil, err
	}
	rows, err := s.db.ListUserBadges(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	owned := map[string]bool{}
	for _, row := range rows {
		badge := badgeFromParts(row.BadgeCode, row.Level, row.Extra, true)
		if badge.ID == "" {
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
