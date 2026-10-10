package main

import (
	"context"
	"sync"
	"time"

	"geoduels/internal/social"
)

// A connected player is recorded as online every half minute, well within the presence window, and
// their "last seen" on their profile every five minutes.
const (
	presenceWriteEvery = 30 * time.Second
	lastSeenWriteEvery = 5 * time.Minute
)

// throttle remembers when this process last did something for each player.
type throttle struct {
	mu   sync.Mutex
	last map[string]time.Time
}

func (t *throttle) due(userID string, now time.Time, every time.Duration) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.last == nil {
		t.last = map[string]time.Time{}
	}
	if now.Sub(t.last[userID]) < every {
		return false
	}
	t.last[userID] = now
	for id, at := range t.last {
		if now.Sub(at) > 2*every {
			delete(t.last, id)
		}
	}
	return true
}

// presenceThrottle paces the two presence writes.
type presenceThrottle struct {
	online, lastSeen throttle
}

// touchViewerPresence records that a player is here: any open socket of theirs calls it. Friends see
// it when their friends list next refreshes.
func (a *api) touchViewerPresence(ctx context.Context, userID string) {
	if userID == "" || a.presence == nil {
		return
	}
	now := time.Now().UTC()
	online := a.presenceWrites.online.due(userID, now, presenceWriteEvery)
	lastSeen := a.presenceWrites.lastSeen.due(userID, now, lastSeenWriteEvery)
	if !online && !lastSeen {
		return
	}
	go func() {
		writeCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer cancel()
		if online {
			_ = a.presence.TouchPresence(writeCtx, userID)
		}
		if lastSeen {
			_ = a.presence.TouchLastSeen(writeCtx, userID, now)
		}
	}()
}

func (a *api) applySocialPresence(ctx context.Context, players []social.CompactPlayer) {
	if len(players) == 0 || a.presence == nil {
		return
	}
	ids := make([]string, 0, len(players))
	for _, player := range players {
		if player.LastSeenAt != nil {
			ids = append(ids, player.UserID)
		}
	}
	online, err := a.presence.Online(ctx, ids)
	if err != nil {
		return
	}
	playing, err := a.matchStore.UsersInLiveMatches(ctx, ids)
	if err != nil {
		playing = map[string]bool{}
	}
	for i := range players {
		if players[i].LastSeenAt == nil {
			continue
		}
		if !online[players[i].UserID] {
			players[i].PresenceStatus = "offline"
			continue
		}
		players[i].PresenceStatus = "online"
		if playing[players[i].UserID] {
			players[i].Activity = "in_match"
		}
	}
}
