package singleplayer

import (
	"testing"

	"geoduels/pkg/contracts"
)

func TestSingleplayerFlowFinalizeAndAdvance(t *testing.T) {
	engine := New(func(matchID string, roundIndex int) (contracts.LocationPoint, error) {
		return contracts.LocationPoint{
			Lat:     float64(roundIndex),
			Lng:     float64(roundIndex),
			Country: "US",
		}, nil
	})
	_, err := engine.CreateMatch("solo-1", []string{"u1"}, map[string]contracts.PlayerProfile{
		"u1": {UserID: "u1", DisplayName: "Solo"},
	})
	if err != nil {
		t.Fatalf("create match: %v", err)
	}

	snap, err := engine.SubmitGuess(contracts.GuessPayload{
		UserID:   "u1",
		MatchID:  "solo-1",
		RoundID:  "solo-1:r1",
		Lat:      0,
		Lng:      0,
		Finalize: true,
	})
	if err != nil {
		t.Fatalf("submit guess: %v", err)
	}
	if snap.Mode != contracts.ModeSingleplayer {
		t.Fatalf("expected singleplayer mode, got %q", snap.Mode)
	}
	if snap.Phase != contracts.PhaseRoundResult {
		t.Fatalf("expected round result phase, got %q", snap.Phase)
	}
	if snap.LastRoundResult == nil {
		t.Fatal("expected round result")
	}
	if len(snap.RoundResults) != 1 {
		t.Fatalf("expected round history length 1, got %d", len(snap.RoundResults))
	}
	if snap.Players["u1"].TotalScore <= 0 {
		t.Fatalf("expected score to accumulate, got %d", snap.Players["u1"].TotalScore)
	}

	snap, err = engine.AdvanceRound("solo-1", "u1")
	if err != nil {
		t.Fatalf("advance round: %v", err)
	}
	if snap.Phase != contracts.PhaseLive {
		t.Fatalf("expected live phase after advance, got %q", snap.Phase)
	}
	if snap.CurrentRound == nil || snap.CurrentRound.RoundNumber != 2 {
		t.Fatalf("expected round 2, got %+v", snap.CurrentRound)
	}
	if len(snap.RoundResults) != 1 {
		t.Fatalf("expected round history to persist after advance, got %d", len(snap.RoundResults))
	}
}

func TestSingleplayerDisconnectResume(t *testing.T) {
	engine := New(func(matchID string, roundIndex int) (contracts.LocationPoint, error) {
		return contracts.LocationPoint{
			Lat:     float64(roundIndex),
			Lng:     float64(roundIndex),
			Country: "US",
		}, nil
	})
	_, err := engine.CreateMatch("solo-disconnect", []string{"u1"}, map[string]contracts.PlayerProfile{
		"u1": {UserID: "u1", DisplayName: "Solo"},
	})
	if err != nil {
		t.Fatalf("create match: %v", err)
	}

	snap, err := engine.MarkDisconnected("solo-disconnect", "u1")
	if err != nil {
		t.Fatalf("disconnect: %v", err)
	}
	if !snap.Players["u1"].Disconnected {
		t.Fatal("expected player to be disconnected")
	}

	snap, err = engine.MarkResumed("solo-disconnect", "u1")
	if err != nil {
		t.Fatalf("resume: %v", err)
	}
	if snap.Players["u1"].Disconnected {
		t.Fatal("expected player to be resumed")
	}
}

func TestSingleplayerSnapshotKeepsConfig(t *testing.T) {
	engine := New(func(matchID string, roundIndex int) (contracts.LocationPoint, error) {
		return contracts.LocationPoint{
			Lat:     float64(roundIndex),
			Lng:     float64(roundIndex),
			Country: "US",
		}, nil
	})
	_, err := engine.CreateMatchWithConfig("solo-nmpz", []string{"u1"}, map[string]contracts.PlayerProfile{
		"u1": {UserID: "u1", DisplayName: "Solo"},
	}, contracts.MatchConfig{Ruleset: contracts.RulesetNMPZ})
	if err != nil {
		t.Fatalf("create match: %v", err)
	}

	snap, err := engine.GetSnapshot("solo-nmpz")
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	if snap.Config.Ruleset != contracts.RulesetNMPZ {
		t.Fatalf("ruleset = %q, want %q", snap.Config.Ruleset, contracts.RulesetNMPZ)
	}
	if snap.Config.MapKey != contracts.MapKeyNMPZ {
		t.Fatalf("map key = %q, want %q", snap.Config.MapKey, contracts.MapKeyNMPZ)
	}
}
