package main

import (
	"reflect"
	"testing"
	"time"
)

func TestReconcilePlan(t *testing.T) {
	now := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	held := map[string]time.Time{
		"running":   now.Add(-time.Minute),
		"replaced":  now.Add(-time.Minute),
		"recording": now.Add(-time.Minute),
		"just-took": now.Add(-time.Second),
	}
	finalizing := map[string]bool{"recording": true}
	open := map[string]bool{"running": true, "lost": true}
	gone, missing := reconcilePlan(held, finalizing, open, now)
	// A match still being picked up or recorded is not let go; one the database gave away is.
	if want := []string{"replaced"}; !reflect.DeepEqual(gone, want) {
		t.Errorf("gone = %v, want %v", gone, want)
	}
	// A match the database says the node runs, but the node does not have, is lost.
	if want := []string{"lost"}; !reflect.DeepEqual(missing, want) {
		t.Errorf("missing = %v, want %v", missing, want)
	}
}
