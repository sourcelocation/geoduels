package main

import (
	"context"
	"log"
	"time"

	"geoduels/pkg/social"
)

const lastSeenWriteInterval = 5 * time.Minute

type lastSeenWriter interface {
	TouchLastSeen(ctx context.Context, userID string, seenAt time.Time) error
}

func (a *api) touchViewerPresence(ctx context.Context, userID string) {
	if userID == "" {
		return
	}
	if a.coord != nil {
		_ = a.coord.TouchPresence(ctx, userID)
	}
	a.scheduleLastSeenWrite(ctx, userID, time.Now().UTC())
	if a.live != nil {
		a.live.notePresence(ctx, userID)
	}
}

func (a *api) scheduleLastSeenWrite(ctx context.Context, userID string, seenAt time.Time) {
	writer := a.lastSeen
	if writer == nil {
		writer = a.db
	}
	if writer == nil {
		return
	}
	if a.redis != nil {
		acquired, err := a.redis.SetNX(ctx, lastSeenWriteKey(userID), seenAt.Format(time.RFC3339Nano), lastSeenWriteInterval).Result()
		if err != nil || !acquired {
			return
		}
	}
	go func() {
		writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer cancel()
		if err := writer.TouchLastSeen(writeCtx, userID, seenAt); err != nil {
			log.Printf("last seen write failed for %s: %v", userID, err)
			if a.redis != nil {
				cleanupCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
				defer cancel()
				_ = a.redis.Del(cleanupCtx, lastSeenWriteKey(userID)).Err()
			}
		}
	}()
}

func lastSeenWriteKey(userID string) string {
	return "rt:presence:last-seen-write:" + userID
}

func (a *api) applySocialPresence(ctx context.Context, players []social.CompactPlayer) {
	if len(players) == 0 {
		return
	}
	ids := make([]string, 0, len(players))
	for _, player := range players {
		if player.LastSeenAt == nil {
			continue
		}
		ids = append(ids, player.UserID)
	}
	present := map[string]bool{}
	if a.coord != nil && len(ids) > 0 {
		if next, err := a.coord.PresentUsers(ctx, ids); err == nil {
			present = next
		}
	}
	for i := range players {
		if players[i].LastSeenAt == nil {
			continue
		}
		if !present[players[i].UserID] {
			players[i].PresenceStatus = "offline"
			continue
		}
		players[i].PresenceStatus = "online"
		if a.coord != nil {
			if assigned, ok, err := a.coord.GetAssignmentByUser(ctx, players[i].UserID); err == nil && ok && assigned.MatchID != "" {
				players[i].Activity = "in_match"
			}
		}
	}
}
