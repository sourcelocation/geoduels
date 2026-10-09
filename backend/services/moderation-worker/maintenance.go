package main

import (
	"context"
	"time"

	"github.com/riverqueue/river"

	"geoduels/internal/accounts"
	"geoduels/internal/curation"
	"geoduels/internal/jobs"
	"geoduels/internal/moderation"
	"geoduels/internal/seasons"
	"geoduels/internal/storage"
	"geoduels/pkg/observability"
)

// matchAnalyzeWorker runs the integrity detector for a finished match.
type matchAnalyzeWorker struct {
	river.WorkerDefaults[jobs.MatchAnalyzeArgs]
	moderation *moderation.Service
}

func (w *matchAnalyzeWorker) Work(ctx context.Context, job *river.Job[jobs.MatchAnalyzeArgs]) error {
	if w.moderation == nil || job.Args.MatchID == "" {
		return nil
	}
	return w.moderation.EvaluateMatch(ctx, job.Args.MatchID)
}

// guestCleanupWorker deletes stale guest accounts in batches.
type guestCleanupWorker struct {
	river.WorkerDefaults[jobs.GuestCleanupArgs]
	accounts *accounts.Service
	ttl      time.Duration
	batch    int
}

func (w *guestCleanupWorker) Work(context.Context, *river.Job[jobs.GuestCleanupArgs]) error {
	if w.accounts == nil || w.ttl <= 0 || w.batch <= 0 {
		return nil
	}
	total := 0
	for {
		deleted, err := w.accounts.DeleteGuestAccountsOlderThan(w.ttl, w.batch)
		if err != nil {
			return err
		}
		total += deleted
		if deleted < w.batch {
			break
		}
	}
	if total > 0 {
		observability.Log("info", "guest cleanup completed", map[string]any{"deleted": total})
	}
	return nil
}

// storageCleanupWorker prunes storage.
type storageCleanupWorker struct {
	river.WorkerDefaults[jobs.StorageCleanupArgs]
	storage storage.Store
	batch   int
}

func (w *storageCleanupWorker) Work(ctx context.Context, _ *river.Job[jobs.StorageCleanupArgs]) error {
	if w.storage == nil {
		return nil
	}
	result, err := w.storage.CleanupStorage(w.batch)
	if err != nil {
		return err
	}
	if result != (storage.StorageCleanupResult{}) {
		observability.Log("info", "storage cleanup completed", map[string]any{
			"expired_replays":    result.ExpiredReplays,
			"match_sessions":     result.MatchSessions,
			"chat_messages":      result.ChatMessages,
			"auth_sessions":      result.AuthSessions,
			"parties":            result.Parties,
			"map_upload_events":  result.MapUploadEvents,
			"map_daily_users":    result.MapDailyUsers,
			"user_notifications": result.UserNotifications,
		})
	}
	return nil
}

// seasonResetWorker applies a due ranked season reset.
type seasonResetWorker struct {
	river.WorkerDefaults[jobs.SeasonResetArgs]
	seasons seasons.Store
}

func (w *seasonResetWorker) Work(ctx context.Context, _ *river.Job[jobs.SeasonResetArgs]) error {
	if w.seasons == nil {
		return nil
	}
	result, reset, err := w.seasons.RunDueRankedSeasonReset(time.Now().UTC())
	if err != nil {
		return err
	}
	if reset {
		observability.Log("info", "ranked season reset completed", map[string]any{
			"previousSeasonId": result.PreviousSeasonID,
			"activeSeasonId":   result.ActiveSeasonID,
			"playersSeeded":    result.PlayersSeeded,
		})
	}
	return nil
}

// curationSweepWorker closes any due Map-of-the-Week cycle.
type curationSweepWorker struct {
	river.WorkerDefaults[jobs.CurationSweepArgs]
	curation *curation.Service
}

func (w *curationSweepWorker) Work(ctx context.Context, _ *river.Job[jobs.CurationSweepArgs]) error {
	if w.curation == nil {
		return nil
	}
	return w.curation.RunSweep(ctx)
}
