package persistence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	defaultMonthlySeasonResetDay = 1
	seasonResetHourUTC           = 21
)

var rankedSeasonIDPattern = regexp.MustCompile(`^s(\d+)(?:\.\d+)?$`)

func (s *pgStore) GetRankedSeasonSettings() (RankedSeasonSettings, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	settings, err := rankedSeasonSettingsTx(ctx, s.pool)
	if err != nil {
		return RankedSeasonSettings{}, err
	}
	return settingsWithNextReset(settings, time.Now().UTC()), nil
}

func (s *pgStore) SetRankedSeasonResetRule(monthlyResetDay int) (RankedSeasonSettings, error) {
	if err := validateMonthlyResetDay(monthlyResetDay); err != nil {
		return RankedSeasonSettings{}, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return RankedSeasonSettings{}, err
	}
	defer tx.Rollback(ctx)
	settings, err := rankedSeasonSettingsForUpdateTx(ctx, tx)
	if err != nil {
		return RankedSeasonSettings{}, err
	}
	settings.MonthlyResetDay = monthlyResetDay
	now := time.Now().UTC()
	currentResetAt := monthlySeasonResetAt(now.Year(), now.Month(), monthlyResetDay)
	if !now.Before(currentResetAt) && (settings.LastResetAt == nil || settings.LastResetAt.Before(currentResetAt)) {
		settings.LastResetAt = &currentResetAt
	}
	if err := writeRankedSeasonSettingsTx(ctx, tx, settings); err != nil {
		return RankedSeasonSettings{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return RankedSeasonSettings{}, err
	}
	return settingsWithNextReset(settings, now), nil
}

func (s *pgStore) RunDueRankedSeasonReset(now time.Time) (RankedSeasonResetResult, bool, error) {
	now = now.UTC()
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return RankedSeasonResetResult{}, false, err
	}
	defer tx.Rollback(ctx)

	settings, err := rankedSeasonSettingsForUpdateTx(ctx, tx)
	if err != nil {
		return RankedSeasonResetResult{}, false, err
	}
	if initializeRankedSeasonResetSchedule(&settings, now) {
		if err := writeRankedSeasonSettingsTx(ctx, tx, settings); err != nil {
			return RankedSeasonResetResult{}, false, err
		}
		if err := tx.Commit(ctx); err != nil {
			return RankedSeasonResetResult{}, false, err
		}
		return RankedSeasonResetResult{}, false, nil
	}
	resetAt := monthlySeasonResetAt(now.Year(), now.Month(), settings.MonthlyResetDay)
	if now.Before(resetAt) || (settings.LastResetAt != nil && !settings.LastResetAt.Before(resetAt)) {
		return RankedSeasonResetResult{}, false, nil
	}
	nextSeasonID, err := nextRankedSeasonID(settings.ActiveSeasonID)
	if err != nil {
		return RankedSeasonResetResult{}, false, err
	}
	seeded, err := advanceRankedSeasonTx(ctx, tx, settings.ActiveSeasonID, nextSeasonID)
	if err != nil {
		return RankedSeasonResetResult{}, false, err
	}
	previousSeasonID := settings.ActiveSeasonID
	settings.ActiveSeasonID = nextSeasonID
	settings.LastResetAt = &resetAt
	if err := writeRankedSeasonSettingsTx(ctx, tx, settings); err != nil {
		return RankedSeasonResetResult{}, false, err
	}
	if err := tx.Commit(ctx); err != nil {
		return RankedSeasonResetResult{}, false, err
	}
	return RankedSeasonResetResult{
		PreviousSeasonID: previousSeasonID,
		ActiveSeasonID:   nextSeasonID,
		PlayersSeeded:    seeded,
		ResetAt:          resetAt.Format(time.RFC3339),
	}, true, nil
}

func advanceRankedSeasonTx(ctx context.Context, tx pgx.Tx, previousSeasonID, nextSeasonID string) (int, error) {
	if strings.TrimSpace(previousSeasonID) == "" || strings.TrimSpace(nextSeasonID) == "" {
		return 0, errors.New("season id required")
	}
	if previousSeasonID == nextSeasonID {
		return 0, errors.New("season is already active")
	}
	seedTag, err := tx.Exec(ctx, `
		insert into ranks(user_id, mode, season_id, mmr, rd)
		select u.id, $1, $2, $3, $4
		from users u
		where coalesce(u.account_type, 'registered') <> 'guest'
		on conflict (user_id, mode, season_id) do nothing
	`, modeDuel, nextSeasonID, initialMMR, initialRatingRD)
	if err != nil {
		return 0, err
	}
	if _, err := tx.Exec(ctx, `
		insert into ranked_stats(user_id, mode, season_id, games_played, wins)
		select u.id, $1, $2, 0, 0
		from users u
		where coalesce(u.account_type, 'registered') <> 'guest'
		on conflict (user_id, mode, season_id) do nothing
	`, modeDuel, nextSeasonID); err != nil {
		return 0, err
	}
	return int(seedTag.RowsAffected()), nil
}

func (s *pgStore) activeSeasonID(ctx context.Context) (string, error) {
	return activeSeasonIDTx(ctx, s.pool)
}

type seasonQuerier interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}

func activeSeasonIDTx(ctx context.Context, q seasonQuerier) (string, error) {
	settings, err := rankedSeasonSettingsTx(ctx, q)
	if err != nil {
		return "", err
	}
	return settings.ActiveSeasonID, nil
}

func rankedSeasonSettingsTx(ctx context.Context, q seasonQuerier) (RankedSeasonSettings, error) {
	var raw string
	err := q.QueryRow(ctx, `
		select value_json::text
		from site_settings
		where key = 'ranked_season'
	`).Scan(&raw)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return normalizeRankedSeasonSettings(RankedSeasonSettings{}), nil
		}
		return RankedSeasonSettings{}, err
	}
	var settings RankedSeasonSettings
	if err := json.Unmarshal([]byte(raw), &settings); err != nil {
		return normalizeRankedSeasonSettings(RankedSeasonSettings{}), nil
	}
	return normalizeRankedSeasonSettings(settings), nil
}

func rankedSeasonSettingsForUpdateTx(ctx context.Context, tx pgx.Tx) (RankedSeasonSettings, error) {
	defaultSettings := normalizeRankedSeasonSettings(RankedSeasonSettings{})
	payload, err := json.Marshal(defaultSettings)
	if err != nil {
		return RankedSeasonSettings{}, err
	}
	if _, err := tx.Exec(ctx, `
		insert into site_settings(key, value_json, updated_at)
		values('ranked_season', $1::jsonb, now())
		on conflict (key) do nothing
	`, string(payload)); err != nil {
		return RankedSeasonSettings{}, err
	}
	var raw string
	if err := tx.QueryRow(ctx, `
		select value_json::text
		from site_settings
		where key = 'ranked_season'
		for update
	`).Scan(&raw); err != nil {
		return RankedSeasonSettings{}, err
	}
	var settings RankedSeasonSettings
	if err := json.Unmarshal([]byte(raw), &settings); err != nil {
		settings = defaultSettings
	}
	return normalizeRankedSeasonSettings(settings), nil
}

func writeRankedSeasonSettingsTx(ctx context.Context, tx pgx.Tx, settings RankedSeasonSettings) error {
	normalized := normalizeRankedSeasonSettings(settings)
	normalized.NextResetAt = nil
	payload, err := json.Marshal(normalized)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `
		insert into site_settings(key, value_json, updated_at)
		values('ranked_season', $1::jsonb, now())
		on conflict (key) do update set
			value_json = excluded.value_json,
			updated_at = now()
	`, string(payload))
	return err
}

func normalizeRankedSeasonSettings(settings RankedSeasonSettings) RankedSeasonSettings {
	settings.ActiveSeasonID = strings.TrimSpace(settings.ActiveSeasonID)
	if settings.ActiveSeasonID == "" {
		settings.ActiveSeasonID = defaultSeasonID
	}
	if validateMonthlyResetDay(settings.MonthlyResetDay) != nil {
		settings.MonthlyResetDay = defaultMonthlySeasonResetDay
	}
	if settings.LastResetAt != nil {
		lastResetAt := settings.LastResetAt.UTC()
		settings.LastResetAt = &lastResetAt
	}
	return settings
}

func settingsWithNextReset(settings RankedSeasonSettings, now time.Time) RankedSeasonSettings {
	settings = normalizeRankedSeasonSettings(settings)
	nextResetAt := nextMonthlySeasonResetAt(now.UTC(), settings.MonthlyResetDay, settings.LastResetAt)
	settings.NextResetAt = &nextResetAt
	return settings
}

func validateMonthlyResetDay(day int) error {
	if day < 1 || day > 28 {
		return errors.New("monthly reset day must be between 1 and 28")
	}
	return nil
}

func monthlySeasonResetAt(year int, month time.Month, day int) time.Time {
	return time.Date(year, month, day, seasonResetHourUTC, 0, 0, 0, time.UTC)
}

func initializeRankedSeasonResetSchedule(settings *RankedSeasonSettings, now time.Time) bool {
	if settings.LastResetAt != nil {
		return false
	}
	initializedAt := now.UTC()
	settings.LastResetAt = &initializedAt
	return true
}

func nextMonthlySeasonResetAt(now time.Time, day int, lastResetAt *time.Time) time.Time {
	current := monthlySeasonResetAt(now.Year(), now.Month(), day)
	if now.Before(current) || (lastResetAt == nil || lastResetAt.Before(current)) {
		return current
	}
	return monthlySeasonResetAt(now.AddDate(0, 1, 0).Year(), now.AddDate(0, 1, 0).Month(), day)
}

func nextRankedSeasonID(seasonID string) (string, error) {
	seasonID = strings.TrimSpace(seasonID)
	matches := rankedSeasonIDPattern.FindStringSubmatch(seasonID)
	if len(matches) != 2 {
		return "", fmt.Errorf("invalid ranked season id %q", seasonID)
	}
	current, err := strconv.Atoi(matches[1])
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("s%d", current+1), nil
}
