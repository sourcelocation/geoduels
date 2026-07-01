package matchlaunch

import (
	"testing"

	"geoduels/pkg/coordinator"
)

func TestAssignedPayloadRejectsUserOutsideMatchRoster(t *testing.T) {
	payload, ok, err := (Launcher{}).AssignedPayload("late-user", coordinator.Assignment{
		MatchID:     "match-1",
		PublicRoute: "node-1",
		Players:     []string{"player-1", "player-2"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if ok || payload.MatchID != "" {
		t.Fatalf("late lobby member received assignment payload: %+v", payload)
	}
}
