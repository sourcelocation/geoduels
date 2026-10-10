package main

import (
	"context"
	"log"
	"sync"
	"time"

	"geoduels/pkg/maintenance"
)

// gameplayDrain keeps the original drain start across maintenance and SIGTERM.
// Maintenance alone must not exit the process or make replacement pods unready.
type gameplayDrain struct {
	mu        sync.Mutex
	startedAt time.Time
	stopping  bool
}

func (d *gameplayDrain) setMaintenance(status maintenance.Status, now time.Time) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.stopping {
		return
	}
	if status.Phase == maintenance.PhaseActive || status.PlayBlocked() {
		if d.startedAt.IsZero() {
			d.startedAt = now
		}
	} else {
		d.startedAt = time.Time{}
	}
}

func (d *gameplayDrain) startShutdown(now time.Time) time.Time {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.stopping = true
	if d.startedAt.IsZero() {
		d.startedAt = now
	}
	return d.startedAt
}

func (d *gameplayDrain) isDraining() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return !d.startedAt.IsZero()
}

func (d *gameplayDrain) isStopping() bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.stopping
}

func (g *gameplayNode) refreshMaintenanceDrain() {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	status, err := g.readMaintenance(ctx)
	if err != nil {
		// Keep the last known drain state if the database is briefly unavailable.
		log.Printf("node maintenance read failed: %v", err)
		return
	}
	g.drain.setMaintenance(status, time.Now())
}
