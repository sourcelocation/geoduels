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

	db "geoduels/pkg/persistence/sqlc/db"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	defaultMonthlySeasonResetDay = 1
	seasonResetHourUTC           = 21
)

var rankedSeasonIDPattern = regexp.MustCompile(`^s(\d+)(?:\.\d+)?$`)

func (s *DB) GetRankedSeasonSettings() (RankedSeasonSettings, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	settings, err := rankedSeasonSettingsTx(ctx, s.pool)
	if err != nil {
		return RankedSeasonSettings{}, err
	}
	return settingsWithNextReset(settings, time.Now().UTC()), nil
}

func (s *DB) SetRankedSeasonResetRule(monthlyResetDay int) (RankedSeasonSettings, error) {
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

func (s *DB) RunDueRankedSeasonReset(now time.Time) (RankedSeasonResetResult, bool, error) {
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
	seeded, err := advanceRankedSeasonTx(ctx, db.New(tx), tx, settings.ActiveSeasonID, nextSeasonID)
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

func advanceRankedSeasonTx(ctx context.Context, q *db.Queries, tx pgx.Tx, previousSeasonID, nextSeasonID string) (int, error) {
	if strings.TrimSpace(previousSeasonID) == "" || strings.TrimSpace(nextSeasonID) == "" {
		return 0, errors.New("season id required")
	}
	if previousSeasonID == nextSeasonID {
		return 0, errors.New("season is already active")
	}
	// Finalize the outgoing season once, while the season settings row is locked.
	// This is the authoritative trigger for the repeatable top-finish badge.
	finishers, err := q.ListRankedSeasonFinishers(ctx, db.ListRankedSeasonFinishersParams{Mode: modeDuel, SeasonID: previousSeasonID})
	if err != nil {
		return 0, err
	}
	for _, userID := range finishers {
		if _, err := awardTopFinishTx(ctx, tx, userID.String()); err != nil {
			return 0, err
		}
	}
	seeded, err := q.SeedRankedSeasonRanks(ctx, db.SeedRankedSeasonRanksParams{Mode: modeDuel, SeasonID: nextSeasonID, Mmr: initialMMR, Rd: initialRatingRD})
	if err != nil {
		return 0, err
	}
	if err := q.SeedRankedSeasonStats(ctx, db.SeedRankedSeasonStatsParams{Mode: modeDuel, SeasonID: nextSeasonID}); err != nil {
		return 0, err
	}
	return int(seeded), nil
}

func (s *DB) activeSeasonID(ctx context.Context) (string, error) {
	return activeSeasonIDTx(ctx, s.pool)
}

func activeSeasonIDTx(ctx context.Context, source any) (string, error) {
	q := seasonQueries(source)
	settings, err := rankedSeasonSettingsTx(ctx, q)
	if err != nil {
		return "", err
	}
	return settings.ActiveSeasonID, nil
}

func rankedSeasonSettingsTx(ctx context.Context, source any) (RankedSeasonSettings, error) {
	q := seasonQueries(source)
	row, err := q.GetRankedSeasonSettings(ctx)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return normalizeRankedSeasonSettings(RankedSeasonSettings{}), nil
		}
		return RankedSeasonSettings{}, err
	}
	raw := row
	var settings RankedSeasonSettings
	if err := json.Unmarshal([]byte(raw), &settings); err != nil {
		return normalizeRankedSeasonSettings(RankedSeasonSettings{}), nil
	}
	return normalizeRankedSeasonSettings(settings), nil
}

func seasonQueries(source any) *db.Queries {
	if q, ok := source.(*db.Queries); ok {
		return q
	}
	if tx, ok := source.(pgx.Tx); ok {
		return db.New(tx)
	}
	if p, ok := source.(*pgxpool.Pool); ok {
		return db.New(p)
	}
	return nil
}

func rankedSeasonSettingsForUpdateTx(ctx context.Context, tx pgx.Tx) (RankedSeasonSettings, error) {
	defaultSettings := normalizeRankedSeasonSettings(RankedSeasonSettings{})
	payload, err := json.Marshal(defaultSettings)
	if err != nil {
		return RankedSeasonSettings{}, err
	}
	q := db.New(tx)
	if err := q.EnsureRankedSeasonSettings(ctx, payload); err != nil {
		return RankedSeasonSettings{}, err
	}
	row, err := q.GetRankedSeasonSettingsForUpdate(ctx)
	if err != nil {
		return RankedSeasonSettings{}, err
	}
	raw := row
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
	return db.New(tx).WriteRankedSeasonSettings(ctx, payload)
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
