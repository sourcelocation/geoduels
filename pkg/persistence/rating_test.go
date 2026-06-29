package persistence

import (
	"math"
	"testing"
	"time"
)

func TestCalculateDuelRatingUpdatesMovesUncertainPlayersMoreThanEstablishedPlayers(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)

	uncertain1, uncertain2 := CalculateDuelRatingUpdates(
		RatingState{MMR: 1500, RD: initialRatingRD, UpdatedAt: now},
		RatingState{MMR: 1500, RD: initialRatingRD, UpdatedAt: now},
		"p1",
		now,
	)
	established1, established2 := CalculateDuelRatingUpdates(
		RatingState{MMR: 1500, RD: minimumRatingRD, UpdatedAt: now},
		RatingState{MMR: 1500, RD: minimumRatingRD, UpdatedAt: now},
		"p1",
		now,
	)

	if uncertain1.Delta <= established1.Delta {
		t.Fatalf("expected high-RD winner to gain more than established winner, got %d and %d", uncertain1.Delta, established1.Delta)
	}
	if math.Abs(float64(uncertain2.Delta)) <= math.Abs(float64(established2.Delta)) {
		t.Fatalf("expected high-RD loser to lose more than established loser, got %d and %d", uncertain2.Delta, established2.Delta)
	}
	if uncertain1.RD >= initialRatingRD || uncertain2.RD >= initialRatingRD {
		t.Fatalf("expected high-RD players to become more certain, got %.2f and %.2f", uncertain1.RD, uncertain2.RD)
	}
}

func TestCalculateDuelRatingUpdatesFavoriteWinStillMovesEstablishedPlayers(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)

	p1, p2 := CalculateDuelRatingUpdates(
		RatingState{MMR: 1579, RD: minimumRatingRD, UpdatedAt: now},
		RatingState{MMR: 1363, RD: minimumRatingRD, UpdatedAt: now},
		"p1",
		now,
	)

	if p1.Delta < 10 || p1.Delta > 20 {
		t.Fatalf("expected established favorite to gain a moderate amount, got %d", p1.Delta)
	}
	if p2.Delta > -10 || p2.Delta < -20 {
		t.Fatalf("expected established underdog to lose a moderate amount, got %d", p2.Delta)
	}
}

func TestCalculateDuelRatingUpdatesEqualEstablishedWinGainsAboutThirty(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)

	p1, p2 := CalculateDuelRatingUpdates(
		RatingState{MMR: 1500, RD: minimumRatingRD, UpdatedAt: now},
		RatingState{MMR: 1500, RD: minimumRatingRD, UpdatedAt: now},
		"p1",
		now,
	)

	if p1.Delta < 28 || p1.Delta > 32 {
		t.Fatalf("expected equal established winner to gain about 30, got %d", p1.Delta)
	}
	if p2.Delta > -28 || p2.Delta < -32 {
		t.Fatalf("expected equal established loser to lose about 30, got %d", p2.Delta)
	}
}

func TestCalculateDuelRatingUpdatesForgivesLowMMRLosses(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)

	_, loser600 := CalculateDuelRatingUpdates(
		RatingState{MMR: 600, RD: minimumRatingRD, UpdatedAt: now},
		RatingState{MMR: 600, RD: minimumRatingRD, UpdatedAt: now},
		"p1",
		now,
	)
	_, loser900 := CalculateDuelRatingUpdates(
		RatingState{MMR: 900, RD: minimumRatingRD, UpdatedAt: now},
		RatingState{MMR: 900, RD: minimumRatingRD, UpdatedAt: now},
		"p1",
		now,
	)
	_, loser1000 := CalculateDuelRatingUpdates(
		RatingState{MMR: 1000, RD: minimumRatingRD, UpdatedAt: now},
		RatingState{MMR: 1000, RD: minimumRatingRD, UpdatedAt: now},
		"p1",
		now,
	)

	if loser600.Delta > -4 || loser600.Delta < -8 {
		t.Fatalf("expected 600 MMR loser to lose about 20%% of a normal loss, got %d", loser600.Delta)
	}
	if loser900.Delta > -22 || loser900.Delta < -26 {
		t.Fatalf("expected 900 MMR loser to lose about 80%% of a normal loss, got %d", loser900.Delta)
	}
	if loser1000.Delta > -28 || loser1000.Delta < -32 {
		t.Fatalf("expected 1000 MMR loser to take the regular loss, got %d", loser1000.Delta)
	}
}

func TestCalculateDuelRatingUpdatesKeepsMinimumMMRAtFiveHundred(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)

	_, loser := CalculateDuelRatingUpdates(
		RatingState{MMR: initialMMR, RD: minimumRatingRD, UpdatedAt: now},
		RatingState{MMR: initialMMR, RD: minimumRatingRD, UpdatedAt: now},
		"p1",
		now,
	)

	if initialMMR != 500 {
		t.Fatalf("expected initial MMR to be 500, got %d", initialMMR)
	}
	if loser.MMR != minimumRankedMMR {
		t.Fatalf("expected loser to stay clamped at %d, got %d", minimumRankedMMR, loser.MMR)
	}
}

func TestCalculateDuelRatingUpdatesProtectsEstablishedFavoriteAgainstNewUnderdog(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)

	favorite, underdog := CalculateDuelRatingUpdates(
		RatingState{MMR: 1000, RD: minimumRatingRD, UpdatedAt: now},
		RatingState{MMR: 500, RD: initialRatingRD, UpdatedAt: now},
		"p2",
		now,
	)

	if favorite.Delta > -8 || favorite.Delta < -15 {
		t.Fatalf("expected protected favorite loss around 20%% of normal, got %d", favorite.Delta)
	}
	if underdog.Delta != maxDuelMMRDelta {
		t.Fatalf("expected new underdog upset win to remain capped at +%d, got %d", maxDuelMMRDelta, underdog.Delta)
	}
}

func TestCalculateDuelRatingUpdatesRestoresUpsetPenaltyAgainstEstablishedUnderdog(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)

	favorite, _ := CalculateDuelRatingUpdates(
		RatingState{MMR: 1000, RD: minimumRatingRD, UpdatedAt: now},
		RatingState{MMR: 500, RD: minimumRatingRD, UpdatedAt: now},
		"p2",
		now,
	)

	if favorite.Delta != -maxDuelMMRDelta {
		t.Fatalf("expected established underdog upset penalty to cap at -%d, got %d", maxDuelMMRDelta, favorite.Delta)
	}
}

func TestCalculateDuelRatingUpdatesHighRDExpectedLossMovesMoreThanSettledLoss(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)

	_, uncertainLoser := CalculateDuelRatingUpdates(
		RatingState{MMR: 1579, RD: minimumRatingRD, UpdatedAt: now},
		RatingState{MMR: 1363, RD: initialRatingRD, UpdatedAt: now},
		"p1",
		now,
	)
	_, settledLoser := CalculateDuelRatingUpdates(
		RatingState{MMR: 1579, RD: minimumRatingRD, UpdatedAt: now},
		RatingState{MMR: 1363, RD: minimumRatingRD, UpdatedAt: now},
		"p1",
		now,
	)

	if math.Abs(float64(uncertainLoser.Delta)) <= math.Abs(float64(settledLoser.Delta)) {
		t.Fatalf("expected uncertain underdog to move more than settled underdog, got %d and %d", uncertainLoser.Delta, settledLoser.Delta)
	}
	if uncertainLoser.RD >= initialRatingRD {
		t.Fatalf("expected uncertain underdog RD to decrease after a game, got %.2f", uncertainLoser.RD)
	}
}

func TestCalculateDuelRatingUpdatesCapsUpsetDelta(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)

	underdog, favorite := CalculateDuelRatingUpdates(
		RatingState{MMR: 1600, RD: initialRatingRD, UpdatedAt: now},
		RatingState{MMR: 2200, RD: minimumRatingRD, UpdatedAt: now},
		"p1",
		now,
	)

	if underdog.Delta != maxDuelMMRDelta {
		t.Fatalf("expected underdog gain to cap at %d, got %d", maxDuelMMRDelta, underdog.Delta)
	}
	if favorite.Delta < -maxDuelMMRDelta {
		t.Fatalf("expected favorite loss not to exceed cap %d, got %d", maxDuelMMRDelta, favorite.Delta)
	}
}

func TestCalculateDuelRatingUpdatesDrawRewardsLowerRatedPlayer(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)

	p1, p2 := CalculateDuelRatingUpdates(
		RatingState{MMR: 1400, RD: minimumRatingRD, UpdatedAt: now},
		RatingState{MMR: 1600, RD: minimumRatingRD, UpdatedAt: now},
		"",
		now,
	)

	if p1.Delta <= 0 {
		t.Fatalf("expected lower rated player to gain on draw, got %d", p1.Delta)
	}
	if p2.Delta >= 0 {
		t.Fatalf("expected higher rated player to lose on draw, got %d", p2.Delta)
	}
}

func TestInflateRatingRD(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)

	if got := inflateRatingRD(minimumRatingRD, now.AddDate(0, 0, -1), now); got <= minimumRatingRD {
		t.Fatalf("expected inactivity to increase RD, got %.2f", got)
	}
	if got := inflateRatingRD(minimumRatingRD, now.AddDate(-1, 0, 0), now); got != maximumRatingRD {
		t.Fatalf("expected one inactive year from minimum RD to cap at %.2f, got %.2f", maximumRatingRD, got)
	}
	if got := inflateRatingRD(maximumRatingRD, now.AddDate(-1, 0, 0), now); got != maximumRatingRD {
		t.Fatalf("expected RD to stay capped at %.2f, got %.2f", maximumRatingRD, got)
	}
}

func TestRatingRDClampsAfterRepeatedGames(t *testing.T) {
	now := time.Date(2026, 5, 1, 12, 0, 0, 0, time.UTC)
	p1 := RatingState{MMR: 1500, RD: minimumRatingRD, UpdatedAt: now}
	p2 := RatingState{MMR: 1500, RD: minimumRatingRD, UpdatedAt: now}

	for i := 0; i < 100; i++ {
		next1, next2 := CalculateDuelRatingUpdates(p1, p2, "p1", now)
		p1 = RatingState{MMR: next1.MMR, RD: next1.RD, UpdatedAt: now}
		p2 = RatingState{MMR: next2.MMR, RD: next2.RD, UpdatedAt: now}
	}

	if p1.RD < minimumRatingRD || p2.RD < minimumRatingRD {
		t.Fatalf("expected RD not to fall below %.2f, got %.2f and %.2f", minimumRatingRD, p1.RD, p2.RD)
	}
}

func TestClampRankedMMR(t *testing.T) {
	if got := clampRankedMMR(-50); got != minimumRankedMMR {
		t.Fatalf("expected clamp to %d, got %d", minimumRankedMMR, got)
	}
}
