package moderation

import (
	"testing"
	"time"

	"geoduels/internal/rating"
)

func TestPlanRefundCapsAtOriginalLoss(t *testing.T) {
	now := time.Now()
	current := rating.State{MMR: 1000, RD: rating.InitialRatingRD, UpdatedAt: now}
	cheater := rating.State{MMR: 1400, RD: rating.InitialRatingRD, UpdatedAt: now}

	before, after, delta, ok := PlanRefund(current, cheater, -5, now)
	if !ok {
		t.Fatal("expected a refund")
	}
	if before != 1000 || after != before+delta {
		t.Fatalf("inconsistent bounds before=%d after=%d delta=%d", before, after, delta)
	}
	if delta > 5 || delta <= 0 {
		t.Fatalf("refund must be positive and capped at the 5-point loss, got %d", delta)
	}
}

func TestPlanRefundSkipsWhenNoLoss(t *testing.T) {
	now := time.Now()
	current := rating.State{MMR: 1000, RD: rating.InitialRatingRD, UpdatedAt: now}
	cheater := rating.State{MMR: 600, RD: rating.InitialRatingRD, UpdatedAt: now}
	if _, _, _, ok := PlanRefund(current, cheater, 0, now); ok {
		t.Fatal("a non-loss must not produce a refund")
	}
}

func TestPlanRefundSkipsWhenOpponentStronger(t *testing.T) {
	now := time.Now()
	// A much stronger current rating than the cheater's snapshot yields no
	// simulated win for the victim, so nothing should be refunded.
	current := rating.State{MMR: 2500, RD: rating.MinimumRatingRD, UpdatedAt: now}
	cheater := rating.State{MMR: 200, RD: rating.MaximumRatingRD, UpdatedAt: now}
	if _, _, _, ok := PlanRefund(current, cheater, -20, now); ok {
		t.Fatal("no positive gain must not produce a refund")
	}
}

func TestShouldBanRegistrationIP(t *testing.T) {
	if !ShouldBanRegistrationIP("203.0.113.7", true) {
		t.Fatal("known IP with a related cheater must be banned")
	}
	if ShouldBanRegistrationIP("", true) {
		t.Fatal("missing IP must not be banned")
	}
	if ShouldBanRegistrationIP("203.0.113.7", false) {
		t.Fatal("IP without a related cheater must not be banned")
	}
}
