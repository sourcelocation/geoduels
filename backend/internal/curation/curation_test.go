package curation

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"
)

// fakeStore stages changes in a copy and keeps them only when the callback
// succeeds. Unused Store methods panic via the embedded nil interface.
type fakeStore struct {
	Store
	cycle            Cycle
	winner, trending Winner
	badgeErr         error
	awarded          []string
	awardSources     []string
	cycleAwards      []time.Time
}

func (f *fakeStore) WithinTx(_ context.Context, fn func(Store) error) error {
	tx := *f
	tx.awarded = slices.Clone(f.awarded)
	tx.awardSources, tx.cycleAwards = slices.Clone(f.awardSources), slices.Clone(f.cycleAwards)
	if err := fn(&tx); err != nil {
		return err
	}
	*f = tx
	return nil
}
func (f *fakeStore) LockCycle(context.Context) (Cycle, error) { return f.cycle, nil }
func (f *fakeStore) Winner(context.Context, time.Time) (Winner, bool, error) {
	return f.winner, f.winner.MapID != "", nil
}
func (f *fakeStore) TrendingMap(context.Context) (Winner, bool, error) {
	return f.trending, f.trending.MapID != "", nil
}
func (f *fakeStore) SaveAward(_ context.Context, start, _ time.Time, _ Winner, source string) error {
	f.cycleAwards = append(f.cycleAwards, start)
	f.awardSources = append(f.awardSources, source)
	return nil
}
func (f *fakeStore) AdvanceCycle(_ context.Context, cycle Cycle, _ time.Time) error {
	f.cycle = cycle
	return nil
}
func (f *fakeStore) AwardBadge(_ context.Context, id, badge string) error {
	if f.badgeErr != nil {
		return f.badgeErr
	}
	f.awarded = append(f.awarded, id+":"+badge)
	return nil
}

func TestCurationClosesOneCycleAcrossDowntime(t *testing.T) {
	for _, source := range []string{"nomination", "trending", "none"} {
		t.Run(source, func(t *testing.T) {
			f := &fakeStore{}
			start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
			f.cycle = Cycle{StartsAt: start, ClosesAt: start.Add(Window)}
			winner := Winner{MapID: "map1", CreatorID: "creator"}
			if source == "nomination" {
				f.winner = winner
				f.trending = Winner{MapID: "other"}
			}
			if source == "trending" {
				f.trending = winner
			}
			now := start.Add(3*Window + time.Hour)
			svc := NewService(f)
			svc.clock = func() time.Time { return now }
			if err := svc.RunSweep(context.Background()); err != nil {
				t.Fatal(err)
			}
			wantAwards := 1
			if source == "none" {
				wantAwards = 0
			}
			if len(f.cycleAwards) != wantAwards || len(f.awarded) != wantAwards {
				t.Fatalf("awards=%v badges=%v", f.cycleAwards, f.awarded)
			}
			if wantAwards == 1 && (f.awardSources[0] != source || !f.cycleAwards[0].Equal(start)) {
				t.Fatal("wrong winner source or cycle")
			}
			if f.cycle.StartsAt.After(now) || !f.cycle.ClosesAt.After(now) {
				t.Fatalf("invalid next cycle: %+v", f.cycle)
			}
			if err := svc.RunSweep(context.Background()); err != nil {
				t.Fatal(err)
			}
			if len(f.cycleAwards) != wantAwards {
				t.Fatal("sweep repeated award")
			}
		})
	}
}

func TestCurationBadgeFailureLeavesCycleUnchanged(t *testing.T) {
	f := &fakeStore{}
	f.cycle = Cycle{StartsAt: time.Now().Add(-2 * Window), ClosesAt: time.Now().Add(-Window)}
	original := f.cycle
	f.winner = Winner{MapID: "map1", CreatorID: "creator"}
	f.badgeErr = errors.New("badge failed")
	if err := NewService(f).RunSweep(context.Background()); !errors.Is(err, f.badgeErr) {
		t.Fatal(err)
	}
	if f.cycle != original || len(f.cycleAwards) != 0 {
		t.Fatal("failed award advanced the cycle")
	}
}
