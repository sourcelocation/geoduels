package main

import (
	"time"

	"geoduels/pkg/observability"
	"geoduels/pkg/persistence"
)

func (a *api) runStorageCleanupLoop() {
	if a.storageCleanupInterval <= 0 || a.storageCleanupBatchSize <= 0 {
		return
	}
	a.cleanupStorage()
	ticker := time.NewTicker(a.storageCleanupInterval)
	defer ticker.Stop()
	for range ticker.C {
		a.cleanupStorage()
	}
}

func (a *api) cleanupStorage() {
	maintenance := a.db
	if _, err := maintenance.ReconcileStaleMatchSessions(a.staleMatchGrace, a.storageCleanupBatchSize); err != nil {
		observability.Log("warn", "stale match reconciliation failed", map[string]any{"error": err.Error()})
	}
	result, err := maintenance.CleanupStorage(a.storageCleanupBatchSize)
	if err != nil {
		observability.Log("warn", "storage cleanup failed", map[string]any{"error": err.Error()})
		return
	}
	if result != (persistence.StorageCleanupResult{}) {
		observability.Log("info", "storage cleanup completed", map[string]any{
			"replays_compressed":  result.ReplaysCompressed,
			"expired_replays":     result.ExpiredReplays,
			"runtime_matches":     result.RuntimeMatches,
			"match_sessions":      result.MatchSessions,
			"match_plans":         result.MatchPlans,
			"chat_messages":       result.ChatMessages,
			"chat_conversations":  result.ChatConversations,
			"auth_sessions":       result.AuthSessions,
			"parties":             result.Parties,
			"map_upload_events":   result.MapUploadEvents,
			"map_daily_users":     result.MapDailyUsers,
			"user_notifications":  result.UserNotifications,
			"notification_outbox": result.NotificationOutbox,
			"discord_sync_outbox": result.DiscordSyncOutbox,
		})
	}
}
