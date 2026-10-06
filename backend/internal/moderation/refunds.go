package moderation

import (
	"context"
	"strings"
	"time"

	"geoduels/internal/rating"
)

// RatingState is the ranked state used by refund policy.
type RatingState = rating.State

// PlanRefund computes the MMR returned to a cheater's victim for one match.
//
// It simulates the victim winning that match against the cheater, then caps the
// gain at the MMR the victim actually lost, then clamps to the ranked bounds.
// Returning ok=false means no refund should be issued (no gain, or the victim
// is already capped).
func PlanRefund(current, cheater RatingState, originalDelta int, now time.Time) (before, after, delta int, ok bool) {
	victimWin, _ := rating.CalculateDuelUpdates(current, cheater, "p1", now)
	gain := victimWin.Delta
	if gain <= 0 {
		return 0, 0, 0, false
	}
	originalLoss := -originalDelta
	if gain > originalLoss {
		gain = originalLoss
	}
	before = current.MMR
	after = rating.ClampRankedMMR(before + gain)
	delta = after - before
	if delta <= 0 {
		return 0, 0, 0, false
	}
	return before, after, delta, true
}

// ShouldBanRegistrationIP reports whether a cheating ban should also block new
// signups from the cheater's registration IP: only when the IP is known and at
// least one other recent cheater shared it.
func ShouldBanRegistrationIP(registrationIP string, relatedCheater bool) bool {
	return strings.TrimSpace(registrationIP) != "" && relatedCheater
}

// RefundCandidate contains match facts, independent of their database encoding.
type RefundCandidate struct {
	MatchID, UserID string
	CheaterMMR      int
	CheaterRD       float64
	OriginalDelta   int
}

type RefundAward struct {
	RefundCandidate
	CheaterID, SeasonID, Reason string
	Before, After, Delta        int
}

func (s *Service) refundVictims(ctx context.Context, store Store, cheaterID, reason string) (EloRefundSummary, error) {
	seasonID, err := store.ActiveSeasonID(ctx)
	if err != nil {
		return EloRefundSummary{}, err
	}
	candidates, err := store.RefundCandidates(ctx, cheaterID)
	if err != nil {
		return EloRefundSummary{}, err
	}
	var summary EloRefundSummary
	for _, candidate := range candidates {
		if candidate.OriginalDelta >= 0 {
			continue
		}
		current, found, err := store.LockRefundRating(ctx, candidate.UserID, seasonID)
		if err != nil {
			return EloRefundSummary{}, err
		}
		if !found {
			continue
		}
		now := s.now()
		before, after, delta, ok := PlanRefund(current, RatingState{MMR: candidate.CheaterMMR, RD: candidate.CheaterRD, UpdatedAt: now}, candidate.OriginalDelta, now)
		if !ok {
			continue
		}
		saved, err := store.SaveRefund(ctx, RefundAward{RefundCandidate: candidate, CheaterID: cheaterID, SeasonID: seasonID, Reason: reason, Before: before, After: after, Delta: delta})
		if err != nil {
			return EloRefundSummary{}, err
		}
		if saved {
			summary.RefundsIssued++
			summary.TotalRefunded += delta
		}
	}
	return summary, nil
}

func pardonCutoff(now time.Time, olderThan time.Duration) time.Time {
	if olderThan <= 0 {
		olderThan = 7 * 24 * time.Hour
	}
	return now.Add(-olderThan)
}
