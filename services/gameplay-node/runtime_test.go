package main

import (
	"testing"

	"geoduels/pkg/contracts"
	"geoduels/pkg/singleplayer"
)

func TestRoundPlanRegistryReturnsPinnedRounds(t *testing.T) {
	registry := newRoundPlanRegistry()
	registry.Set("match", []contracts.PlannedRound{
		{RoundIndex: 0, Location: contracts.LocationPoint{Lat: 1, Lng: 2}},
		{RoundIndex: 1, Location: contracts.LocationPoint{Lat: 3, Lng: 4}},
	})
	point, err := registry.Get("match", 1)
	if err != nil {
		t.Fatal(err)
	}
	if point.Lat != 3 || point.Lng != 4 {
		t.Fatalf("unexpected point %+v", point)
	}
	if _, err := registry.Get("match", 2); err == nil {
		t.Fatal("expected exhausted plan error")
	}
}

func TestSingleplayerRuntimePreservesMatchConfig(t *testing.T) {
	runtime := singleplayerRuntime{
		engine: singleplayer.New(func(matchID string, roundIndex int) (contracts.LocationPoint, error) {
			return contracts.LocationPoint{Lat: 1, Lng: 2, Country: "US"}, nil
		}),
	}
	err := runtime.CreateMatch(
		"solo-config",
		[]string{"u1"},
		map[string]contracts.PlayerProfile{
			"u1": {UserID: "u1", DisplayName: "Solo"},
		},
		false,
		"",
		contracts.MatchConfig{
			Ruleset:     contracts.RulesetNoMove,
			StreetNames: contracts.StreetNamesHidden,
		},
		nil,
	)
	if err != nil {
		t.Fatalf("create match: %v", err)
	}

	snap, err := runtime.GetSnapshot("solo-config")
	if err != nil {
		t.Fatalf("get snapshot: %v", err)
	}
	if snap.Config.Ruleset != contracts.RulesetNoMove {
		t.Fatalf("ruleset = %q, want %q", snap.Config.Ruleset, contracts.RulesetNoMove)
	}
	if snap.Config.StreetNames != contracts.StreetNamesHidden {
		t.Fatalf("street names = %q, want %q", snap.Config.StreetNames, contracts.StreetNamesHidden)
	}
}
