package main

import (
	"context"
	"encoding/json"
	"net/http"
	"sync"
	"time"

	"geoduels/pkg/maintenance"
)

type globalStatusSnapshot struct {
	OnlinePlayers int                `json:"onlinePlayers"`
	Maintenance   maintenance.Status `json:"maintenance"`
	ObservedAt    time.Time          `json:"observedAt"`
}

type globalStatusHub struct {
	api         *api
	mu          sync.RWMutex
	snapshot    globalStatusSnapshot
	fingerprint string
	subscribers map[chan globalStatusSnapshot]struct{}
	startOnce   sync.Once
}

func newGlobalStatusHub(a *api) *globalStatusHub {
	return &globalStatusHub{
		api: a,
		snapshot: globalStatusSnapshot{
			Maintenance: maintenance.DefaultStatus(),
			ObservedAt:  time.Now(),
		},
		subscribers: make(map[chan globalStatusSnapshot]struct{}),
	}
}

func (h *globalStatusHub) start() {
	if h == nil {
		return
	}
	h.startOnce.Do(func() {
		go func() {
			h.refresh()
			ticker := time.NewTicker(10 * time.Second)
			defer ticker.Stop()
			for range ticker.C {
				h.refresh()
			}
		}()
	})
}

func (h *globalStatusHub) refresh() {
	if h.api.coord == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	online, err := h.api.coord.CountPresentUsers(ctx)
	if err != nil {
		return
	}
	status, err := h.api.maintenanceStatus(ctx)
	if err != nil {
		return
	}
	next := globalStatusSnapshot{
		OnlinePlayers: online,
		Maintenance:   status,
		ObservedAt:    time.Now(),
	}
	body, _ := json.Marshal(struct {
		OnlinePlayers int                `json:"onlinePlayers"`
		Maintenance   maintenance.Status `json:"maintenance"`
	}{online, status})
	fingerprint := string(body)

	h.mu.Lock()
	changed := fingerprint != h.fingerprint
	h.snapshot = next
	h.fingerprint = fingerprint
	if changed {
		for subscriber := range h.subscribers {
			select {
			case subscriber <- next:
			default:
			}
		}
	}
	h.mu.Unlock()
}

func (h *globalStatusHub) current() globalStatusSnapshot {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.snapshot
}

func (h *globalStatusHub) subscribe() (<-chan globalStatusSnapshot, func()) {
	channel := make(chan globalStatusSnapshot, 1)
	h.mu.Lock()
	h.subscribers[channel] = struct{}{}
	channel <- h.snapshot
	h.mu.Unlock()
	return channel, func() {
		h.mu.Lock()
		delete(h.subscribers, channel)
		close(channel)
		h.mu.Unlock()
	}
}

func (a *api) statusHub() *globalStatusHub {
	if a.globalStatus == nil {
		a.globalStatus = newGlobalStatusHub(a)
		a.globalStatus.start()
	}
	return a.globalStatus
}

func (a *api) publicGlobalStatus(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, a.statusHub().current())
}
