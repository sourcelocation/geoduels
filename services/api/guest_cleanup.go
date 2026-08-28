package main

import (
	"context"
	"os"
	"strconv"
	"time"

	"github.com/redis/go-redis/v9"

	"geoduels/pkg/observability"
)

const guestCleanupLockKey = "api:guest_cleanup:lock"

var releaseGuestCleanupLockScript = redis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
  return redis.call("DEL", KEYS[1])
end
return 0
`)

func (a *api) runGuestAccountCleanupLoop() {
	interval := a.guestCleanupInterval
	if interval <= 0 {
		return
	}
	a.cleanupGuestAccounts()
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for range ticker.C {
		a.cleanupGuestAccounts()
	}
}

func (a *api) cleanupGuestAccounts() {
	if a.accounts == nil || a.redis == nil || a.guestAccountTTL <= 0 || a.guestCleanupBatchSize <= 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	token := guestCleanupToken()
	ok, err := a.redis.SetNX(ctx, guestCleanupLockKey, token, guestCleanupLockTTL(a.guestCleanupInterval)).Result()
	if err != nil {
		observability.Log("warn", "guest cleanup lock failed", map[string]any{"error": err.Error()})
		return
	}
	if !ok {
		return
	}
	defer func() {
		releaseCtx, releaseCancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer releaseCancel()
		_ = releaseGuestCleanupLockScript.Run(releaseCtx, a.redis, []string{guestCleanupLockKey}, token).Err()
	}()

	total := 0
	for {
		deleted, err := a.accounts.DeleteGuestAccountsOlderThan(a.guestAccountTTL, a.guestCleanupBatchSize)
		if err != nil {
			observability.Log("warn", "guest cleanup failed", map[string]any{"deleted": total, "error": err.Error()})
			return
		}
		total += deleted
		if deleted < a.guestCleanupBatchSize {
			break
		}
	}
	if total > 0 {
		observability.Log("info", "guest cleanup completed", map[string]any{"deleted": total})
	}
}

func guestCleanupLockTTL(interval time.Duration) time.Duration {
	if interval <= 0 || interval > 10*time.Minute {
		return 10 * time.Minute
	}
	if interval < time.Minute {
		return time.Minute
	}
	return interval
}

func guestCleanupToken() string {
	host, _ := os.Hostname()
	return host + ":" + strconv.FormatInt(time.Now().UnixNano(), 10)
}
