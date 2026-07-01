package persistence

import (
	"testing"
	"time"
)

func TestNextRankedSeasonID(t *testing.T) {
	tests := map[string]string{
		"s2":   "s3",
		"s2.5": "s3",
		"s3":   "s4",
	}
	for input, want := range tests {
		got, err := nextRankedSeasonID(input)
		if err != nil {
			t.Fatalf("nextRankedSeasonID(%q) error = %v", input, err)
		}
		if got != want {
			t.Fatalf("nextRankedSeasonID(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestNextRankedSeasonIDRejectsInvalidIDs(t *testing.T) {
	for _, input := range []string{"", "season2", "s", "s2.x"} {
		if got, err := nextRankedSeasonID(input); err == nil {
			t.Fatalf("nextRankedSeasonID(%q) = %q, want error", input, got)
		}
	}
}

func TestNextMonthlySeasonResetAt(t *testing.T) {
	before := time.Date(2026, time.June, 1, 20, 59, 0, 0, time.UTC)
	wantCurrent := time.Date(2026, time.June, 1, 21, 0, 0, 0, time.UTC)
	if got := nextMonthlySeasonResetAt(before, 1, nil); !got.Equal(wantCurrent) {
		t.Fatalf("next reset before due = %s, want %s", got, wantCurrent)
	}

	after := time.Date(2026, time.June, 1, 21, 1, 0, 0, time.UTC)
	if got := nextMonthlySeasonResetAt(after, 1, nil); !got.Equal(wantCurrent) {
		t.Fatalf("next reset after missed due = %s, want %s", got, wantCurrent)
	}

	lastReset := wantCurrent
	wantNext := time.Date(2026, time.July, 1, 21, 0, 0, 0, time.UTC)
	if got := nextMonthlySeasonResetAt(after, 1, &lastReset); !got.Equal(wantNext) {
		t.Fatalf("next reset after completed due = %s, want %s", got, wantNext)
	}
}

func TestInitializeRankedSeasonResetScheduleBeforeMonthlyReset(t *testing.T) {
	now := time.Date(2026, time.June, 1, 20, 59, 0, 0, time.FixedZone("test", 3*60*60))
	settings := RankedSeasonSettings{MonthlyResetDay: 1}

	if initialized := initializeRankedSeasonResetSchedule(&settings, now); !initialized {
		t.Fatal("schedule was not initialized")
	}
	if settings.LastResetAt == nil || !settings.LastResetAt.Equal(now.UTC()) {
		t.Fatalf("last reset = %v, want %s", settings.LastResetAt, now.UTC())
	}

	want := time.Date(2026, time.June, 1, 21, 0, 0, 0, time.UTC)
	if got := nextMonthlySeasonResetAt(now.UTC(), settings.MonthlyResetDay, settings.LastResetAt); !got.Equal(want) {
		t.Fatalf("next reset = %s, want %s", got, want)
	}
}

func TestInitializeRankedSeasonResetScheduleAfterMonthlyReset(t *testing.T) {
	now := time.Date(2026, time.June, 18, 14, 0, 0, 0, time.UTC)
	settings := RankedSeasonSettings{MonthlyResetDay: 1}

	if initialized := initializeRankedSeasonResetSchedule(&settings, now); !initialized {
		t.Fatal("schedule was not initialized")
	}
	if settings.LastResetAt == nil || !settings.LastResetAt.Equal(now) {
		t.Fatalf("last reset = %v, want %s", settings.LastResetAt, now)
	}

	want := time.Date(2026, time.July, 1, 21, 0, 0, 0, time.UTC)
	if got := nextMonthlySeasonResetAt(now, settings.MonthlyResetDay, settings.LastResetAt); !got.Equal(want) {
		t.Fatalf("next reset = %s, want %s", got, want)
	}
}

func TestInitializeRankedSeasonResetSchedulePreservesExistingValue(t *testing.T) {
	existing := time.Date(2026, time.June, 1, 21, 0, 0, 0, time.UTC)
	settings := RankedSeasonSettings{MonthlyResetDay: 1, LastResetAt: &existing}

	if initialized := initializeRankedSeasonResetSchedule(&settings, existing.AddDate(0, 0, 1)); initialized {
		t.Fatal("existing schedule was unexpectedly initialized")
	}
	if settings.LastResetAt == nil || !settings.LastResetAt.Equal(existing) {
		t.Fatalf("last reset = %v, want %s", settings.LastResetAt, existing)
	}
}
