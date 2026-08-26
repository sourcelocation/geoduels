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

func TestClientSnapshotForPlayerIncludesIndividualDamageMultipliers(t *testing.T) {
	snap := &MatchSnapshot{
		Players: map[string]PlayerState{
			"u1": {UserID: "u1", DamageMultiplier: 2},
			"u2": {UserID: "u2", DamageMultiplier: 1.5},
		},
	}

	client := ClientSnapshotForPlayer(snap, "u1")

	if got := client.Players["u1"].DamageMultiplier; got != 2 {
		t.Fatalf("own DamageMultiplier = %v, want 2", got)
	}
	if got := client.Players["u2"].DamageMultiplier; got != 1.5 {
		t.Fatalf("opponent DamageMultiplier = %v, want 1.5", got)
	}
	encoded, err := json.Marshal(client)
	if err != nil {
		t.Fatalf("marshal client snapshot: %v", err)
	}
	if payload := string(encoded); !strings.Contains(payload, `"damageMultiplier":2`) || !strings.Contains(payload, `"damageMultiplier":1.5`) {
		t.Fatalf("expected individual damage multipliers in client payload, got %s", payload)
	}
}

func TestClientSnapshotForPlayerSharesOnlyTeammateGuesses(t *testing.T) {
	snap := &MatchSnapshot{Mode: ModeTeamDuel, Phase: PhaseLive, RoundPhase: RoundPhaseLive, Players: map[string]PlayerState{
		"self":  {UserID: "self", TeamID: "a"},
		"mate":  {UserID: "mate", TeamID: "a", HasGuess: true, LastGuessLat: 12, LastGuessLng: 34},
		"enemy": {UserID: "enemy", TeamID: "b", HasGuess: true, LastGuessLat: 56, LastGuessLng: 78},
	}}
	client := ClientSnapshotForPlayer(snap, "self")
	if client.Team == nil || client.Team.Guesses["mate"].Lat != 12 {
		t.Fatalf("expected teammate guess, got %#v", client.Team)
	}
	if _, ok := client.Team.Guesses["enemy"]; ok {
		t.Fatal("opponent guess leaked into private team state")
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
	if config.MultiplierMode != MultiplierShared {
		t.Fatalf("MultiplierMode = %q, want %q", config.MultiplierMode, MultiplierShared)
	}
}

func TestNormalizeMatchReturnTargetDoesNotInferFromMatchConfig(t *testing.T) {
	target := NormalizeMatchReturnTarget(nil)
	if target.Kind != MatchReturnHome {
		t.Fatalf("nil target kind = %q, want home", target.Kind)
	}
	mapTarget := NormalizeMatchReturnTarget(&MatchReturnTarget{Kind: MatchReturnMap, MapID: "map-1"})
	if mapTarget.Kind != MatchReturnMap || mapTarget.MapID != "map-1" {
		t.Fatalf("map target = %+v", mapTarget)
	}
	invalid := NormalizeMatchReturnTarget(&MatchReturnTarget{Kind: MatchReturnMap})
	if invalid.Kind != MatchReturnHome {
		t.Fatalf("invalid map target kind = %q, want home", invalid.Kind)
	}
}

func TestNormalizeMatchConfigKeepsIndividualMultipliers(t *testing.T) {
	config := NormalizeMatchConfig(MatchConfig{MultiplierMode: MultiplierIndividual})
	if config.MultiplierMode != MultiplierIndividual {
		t.Fatalf("MultiplierMode = %q, want %q", config.MultiplierMode, MultiplierIndividual)
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
