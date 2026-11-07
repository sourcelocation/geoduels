package main

import (
	"context"
	"sync/atomic"
	"time"

	"geoduels/internal/presence"
	"geoduels/pkg/maintenance"
)

const statusRefreshEvery = 10 * time.Second

// statusCache is the online count and maintenance status, read at most this often however many ask.
type statusCache struct {
	api   *api
	value atomic.Pointer[statusEvent]
}

func newStatusCache(a *api) *statusCache {
	cache := &statusCache{api: a}
	cache.value.Store(&statusEvent{Maintenance: maintenance.DefaultStatus()})
	return cache
}

func (s *statusCache) current() statusEvent { return *s.value.Load() }

func (s *statusCache) run() {
	s.refresh()
	ticker := time.NewTicker(statusRefreshEvery)
	defer ticker.Stop()
	for range ticker.C {
		s.refresh()
	}
}

func (s *statusCache) refresh() {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	online, err := presence.Count(ctx, s.api.db.Pool())
	if err != nil {
		return
	}
	status, err := s.api.maintenanceStatus(ctx)
	if err != nil {
		return
	}
	s.value.Store(&statusEvent{OnlinePlayers: online, Maintenance: status})
}
