package contracts

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestClientSnapshotForPlayerKeepsLiveGuessesOutOfSharedPlayers(t *testing.T) {
	snap := &MatchSnapshot{
		Phase:      PhaseLive,
		RoundPhase: RoundPhaseLive,
		Players: map[string]PlayerState{
			"u1": {UserID: "u1", LastGuessLat: 10.5, LastGuessLng: 20.5, HasGuess: true},
			"u2": {UserID: "u2", LastGuessLat: -30.5, LastGuessLng: 40.5, HasGuess: true},
		},
	}

	client := ClientSnapshotForPlayer(snap, "u1")

	if client.Self == nil || client.Self.CurrentGuess == nil {
		t.Fatalf("expected own guess in private self state")
	}
	if got := *client.Self.CurrentGuess; got.Lat != 10.5 || got.Lng != 20.5 {
		t.Fatalf("expected own guess in private self state, got lat=%v lng=%v", got.Lat, got.Lng)
	}

	encoded, err := json.Marshal(client)
	if err != nil {
		t.Fatalf("marshal client snapshot: %v", err)
	}
	payload := string(encoded)
	if strings.Contains(payload, "lastGuessLat") || strings.Contains(payload, "lastGuessLng") {
		t.Fatalf("expected shared players payload to omit live guess fields, got %s", payload)
	}
	if strings.Contains(payload, "-30.5") || strings.Contains(payload, "40.5") {
		t.Fatalf("expected opponent live guess coordinates to be absent, got %s", payload)
	}
	if got := snap.Players["u2"]; got.LastGuessLat != -30.5 || got.LastGuessLng != 40.5 {
		t.Fatalf("expected original snapshot to remain unchanged, got lat=%v lng=%v", got.LastGuessLat, got.LastGuessLng)
	}
}

func TestClientSnapshotForPlayerRevealsGuessesInRoundResults(t *testing.T) {
	snap := &MatchSnapshot{
		Phase:      PhaseRoundResult,
		RoundPhase: RoundPhaseResult,
		LastRoundResult: &RoundResult{
			RoundID:        "round-1",
			RoundNumber:    1,
			ActualLocation: LocationPoint{Lat: 1, Lng: 2},
			Players: map[string]RoundPlayerResult{
				"u1": {UserID: "u1", Lat: 10.5, Lng: 20.5},
				"u2": {UserID: "u2", Lat: -30.5, Lng: 40.5},
			},
		},
		Players: map[string]PlayerState{
			"u1": {UserID: "u1", LastGuessLat: 10.5, LastGuessLng: 20.5, HasGuess: true},
			"u2": {UserID: "u2", LastGuessLat: -30.5, LastGuessLng: 40.5, HasGuess: true},
		},
	}

	client := ClientSnapshotForPlayer(snap, "u1")
	encoded, err := json.Marshal(client)
	if err != nil {
		t.Fatalf("marshal client snapshot: %v", err)
	}
	payload := string(encoded)
	if !strings.Contains(payload, "-30.5") || !strings.Contains(payload, "40.5") {
		t.Fatalf("expected opponent guess coordinates in revealed round result, got %s", payload)
	}
	if client.Self != nil && client.Self.CurrentGuess != nil {
		t.Fatalf("expected live self guess to be omitted outside live round")
	}
}

func TestClientSnapshotForPlayerKeepsZeroCoordinateSelfGuess(t *testing.T) {
	snap := &MatchSnapshot{
		Phase:      PhaseLive,
		RoundPhase: RoundPhaseLive,
		Players: map[string]PlayerState{
			"u1": {UserID: "u1", LastGuessLat: 0, LastGuessLng: 0, HasGuess: true},
		},
	}

	client := ClientSnapshotForPlayer(snap, "u1")

	if client.Self == nil || client.Self.CurrentGuess == nil {
		t.Fatalf("expected zero-coordinate own guess in private self state")
	}
	if got := *client.Self.CurrentGuess; got.Lat != 0 || got.Lng != 0 {
		t.Fatalf("expected zero-coordinate own guess, got lat=%v lng=%v", got.Lat, got.Lng)
	}
}

func TestClientSnapshotForPlayerRedactsLiveRoundLocationCoordinates(t *testing.T) {
	snap := &MatchSnapshot{
		Phase:      PhaseLive,
		RoundPhase: RoundPhaseLive,
		CurrentRound: &RoundState{
			RoundID:     "r1",
			RoundNumber: 1,
			Location: LocationPoint{
				Lat:    51.5074,
				Lng:    -0.1278,
				PanoID: ptrString("pano-abc"),
			},
		},
		Players: map[string]PlayerState{
			"u1": {UserID: "u1"},
		},
	}

	client := ClientSnapshotForPlayer(snap, "u1")
	if client == nil || client.CurrentRound == nil {
		t.Fatalf("expected current round in client snapshot")
	}
	if client.CurrentRound.Location.Lat != 0 || client.CurrentRound.Location.Lng != 0 {
		t.Fatalf("expected live round coordinates to be redacted, got lat=%v lng=%v", client.CurrentRound.Location.Lat, client.CurrentRound.Location.Lng)
	}
	if client.CurrentRound.Location.PanoID == nil || *client.CurrentRound.Location.PanoID != "pano-abc" {
		t.Fatalf("expected pano id to remain available")
	}
	if snap.CurrentRound.Location.Lat != 51.5074 || snap.CurrentRound.Location.Lng != -0.1278 {
		t.Fatalf("expected original snapshot to remain unchanged")
	}
}

func ptrString(v string) *string { return &v }
