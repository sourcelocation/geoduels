package matches

import (
	"testing"

	"geoduels/pkg/contracts"
	"geoduels/pkg/matchkind"
)

// A new match closes the player's matches that are over, replaces the replaceable ones still
// running, and is blocked by any other running one, whatever kind the new match is.
func TestSeatOutcome(t *testing.T) {
	statuses := []contracts.MatchSessionStatus{
		contracts.MatchSessionStarting, contracts.MatchSessionLive,
		contracts.MatchSessionEnded, contracts.MatchSessionInterrupted, contracts.MatchSessionMissing,
	}
	for _, kind := range matchkind.All() {
		for _, status := range statuses {
			outcome, blocks := SeatOutcome(status, kind)
			switch {
			case !status.Open():
				if blocks || outcome != OutcomeInterrupted {
					t.Errorf("%s %s: outcome %q blocks=%v, want it closed", status, kind, outcome, blocks)
				}
			case matchkind.Of(kind).Replaceable:
				if blocks || outcome != OutcomeReplaced {
					t.Errorf("%s %s: outcome %q blocks=%v, want it replaced", status, kind, outcome, blocks)
				}
			default:
				if !blocks {
					t.Errorf("%s %s: a running %s match must block a new one", status, kind, kind)
				}
			}
		}
	}
}
