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

func TestClientSnapshotForPlayerStripsLiveRoundCoordinatesAndKeepsPanoID(t *testing.T) {
	panoID := "pano-123"
	heading := 90.0
	pitch := -5.0
	snap := &MatchSnapshot{
		Phase:      PhaseLive,
		RoundPhase: RoundPhaseLive,
		CurrentRound: &RoundState{
			RoundID:     "round-1",
			RoundNumber: 1,
			Location: LocationPoint{
				Lat:     10.5,
				Lng:     20.5,
				PanoID:  &panoID,
				Heading: &heading,
				Pitch:   &pitch,
			},
		},
		Players: map[string]PlayerState{
			"u1": {UserID: "u1"},
		},
	}

	client := ClientSnapshotForPlayer(snap, "u1")
	encoded, err := json.Marshal(client)
	if err != nil {
		t.Fatalf("marshal client snapshot: %v", err)
	}
	payload := string(encoded)
	if strings.Contains(payload, `"lat":10.5`) || strings.Contains(payload, `"lng":20.5`) {
		t.Fatalf("expected live round coordinates to be stripped, got %s", payload)
	}
	if !strings.Contains(payload, `"panoId":"pano-123"`) {
		t.Fatalf("expected live round pano ID, got %s", payload)
	}
	if !strings.Contains(payload, `"heading":90`) || !strings.Contains(payload, `"pitch":-5`) {
		t.Fatalf("expected live round camera fields, got %s", payload)
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

func TestNormalizeMatchConfigDoesNotChooseMap(t *testing.T) {
	config := NormalizeMatchConfig(MatchConfig{})

	if config.MapID != "" {
		t.Fatalf("MapID = %q, want empty until the owning workflow resolves a map", config.MapID)
	}
	if config.Ruleset != RulesetMoving {
		t.Fatalf("Ruleset = %q, want %q", config.Ruleset, RulesetMoving)
	}
	if config.StreetNames != StreetNamesShown {
		t.Fatalf("StreetNames = %q, want %q", config.StreetNames, StreetNamesShown)
	}
}

func TestNormalizeMatchConfigKeepsNoMoveAndHiddenStreetNames(t *testing.T) {
	config := NormalizeMatchConfig(MatchConfig{
		Ruleset:     RulesetNoMove,
		StreetNames: StreetNamesHidden,
	})

	if config.Ruleset != RulesetNoMove {
		t.Fatalf("Ruleset = %q, want %q", config.Ruleset, RulesetNoMove)
	}
	if config.StreetNames != StreetNamesHidden {
		t.Fatalf("StreetNames = %q, want %q", config.StreetNames, StreetNamesHidden)
	}
}

func TestNormalizeMatchConfigMigratesLegacyMapKey(t *testing.T) {
	config := NormalizeMatchConfig(MatchConfig{MapKey: "legacy-map"})

	if config.MapID != "legacy-map" {
		t.Fatalf("MapID = %q, want legacy-map", config.MapID)
	}
	if config.MapKey != "" {
		t.Fatalf("MapKey = %q, want empty after migration", config.MapKey)
	}
}

func TestNormalizeMatchConfigKeepsAllowedPressureDurations(t *testing.T) {
	for _, pressureTimeLimitMS := range []int64{15_000, 30_000, 60_000, 90_000} {
		config := NormalizeMatchConfig(MatchConfig{PressureTimeLimitMS: pressureTimeLimitMS})

		if config.PressureTimeLimitMS != pressureTimeLimitMS {
			t.Fatalf("PressureTimeLimitMS = %d, want %d", config.PressureTimeLimitMS, pressureTimeLimitMS)
		}
	}
}

func TestNormalizeMatchConfigClearsUnsupportedPressureDurations(t *testing.T) {
	config := NormalizeMatchConfig(MatchConfig{PressureTimeLimitMS: 45_000})

	if config.PressureTimeLimitMS != 0 {
		t.Fatalf("PressureTimeLimitMS = %d, want 0", config.PressureTimeLimitMS)
	}
}
