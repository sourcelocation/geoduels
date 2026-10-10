package contracts_test

import (
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"geoduels/pkg/contracts"
	"geoduels/pkg/duel"
	"geoduels/pkg/singleplayer"
)

// permittedTopLevelKeys is the privacy allowlist for serialized client
// snapshots. Anything outside this set is a leak.
var permittedTopLevelKeys = map[string]bool{
	"matchId": true, "mode": true, "kind": true, "config": true,
	"state": true, "phase": true, "roundPhase": true,
	"phaseStartedAt": true, "phaseEndsAt": true,
	"currentRound": true, "lastRoundResult": true, "roundResults": true,
	"roundMsLeft": true, "players": true, "teams": true,
	"self": true, "team": true, "ratingPreview": true,
	"eventSequence": true, "serverUnixMs": true, "graceWindowSec": true,
}

type scenario struct {
	name          string
	mode          contracts.MatchMode
	kind          contracts.MatchKind
	players       []string
	teams         map[string]string
	useSoloEngine bool
}

// guessFor returns distinct, recoverable coordinates per player and round so
// any leak is attributable.
func guessFor(playerID string, round int) (float64, float64) {
	index := 0
	for i, c := range playerID {
		index += int(c) << (i % 4)
	}
	return float64(10*(round%9)+index%9) + 0.5, float64(index%170) - 85.0
}

type harness struct {
	snap    *contracts.MatchSnapshot
	players []string
}

// runScenario drives a real production engine through a round lifecycle with a
// controlled clock and captures an authoritative snapshot at each phase.
func runScenario(t *testing.T, s scenario) map[string]harness {
	t.Helper()
	base := time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)
	var nowMs int64
	clock := func() time.Time { return base.Add(time.Duration(nowMs) * time.Millisecond) }
	advance := func(d time.Duration) { nowMs += d.Milliseconds() }
	phaseSnaps := map[string]harness{}
	locationProvider := func(string, int) (contracts.LocationPoint, error) {
		return contracts.LocationPoint{Lat: 52.52, Lng: 13.405, Country: "de"}, nil
	}
	captureFrom := func(get func() (*contracts.MatchSnapshot, error)) func(string) {
		return func(name string) {
			snap, err := get()
			if err != nil {
				t.Fatalf("%s: snapshot: %v", name, err)
			}
			phaseSnaps[name] = harness{snap: snap, players: s.players}
		}
	}

	if s.useSoloEngine {
		solo := singleplayer.NewWithClock(locationProvider, clock)
		player := s.players[0]
		if _, err := solo.CreateMatch("m1", s.players, nil); err != nil {
			t.Fatalf("create solo match: %v", err)
		}
		capture := captureFrom(func() (*contracts.MatchSnapshot, error) { return solo.GetSnapshot("m1") })
		capture("intro")
		advance(5 * time.Second)
		capture("live_no_guess")
		lat, lng := guessFor(player, 1)
		solo.SubmitGuess(contracts.GuessPayload{UserID: player, MatchID: "m1", RoundID: "m1:r1", Lat: lat, Lng: lng, Finalize: true})
		capture("result")
		advance(1 * time.Second)
		solo.AdvanceRound("m1", player)
		capture("next_round_intro")
		advance(5 * time.Second)
		lat2, lng2 := guessFor(player, 2)
		solo.SubmitGuess(contracts.GuessPayload{UserID: player, MatchID: "m1", RoundID: "m1:r2", Lat: lat2, Lng: lng2, Finalize: true})
		solo.Forfeit("m1", player)
		capture("ended")
		return phaseSnaps
	}

	eng := duel.NewWithClock(locationProvider, clock)
	capture := captureFrom(func() (*contracts.MatchSnapshot, error) { return eng.GetSnapshot("m1") })
	if _, err := eng.CreateMatchWithOptions("m1", s.players, nil, duel.MatchOptions{Kind: s.kind, Teams: s.teams}); err != nil {
		t.Fatalf("create match: %v", err)
	}
	currentRoundID := func() string {
		snap, _ := eng.GetSnapshot("m1")
		if snap.CurrentRound == nil {
			return ""
		}
		return snap.CurrentRound.RoundID
	}
	submitAll := func(round int, finalize bool) {
		roundID := currentRoundID()
		for _, p := range s.players {
			lat, lng := guessFor(p, round)
			eng.SubmitGuess(contracts.GuessPayload{UserID: p, MatchID: "m1", RoundID: roundID, Lat: lat, Lng: lng, Finalize: finalize})
		}
	}
	capture("intro")
	advance(5 * time.Second)
	capture("live_no_guess")
	submitAll(1, true)
	capture("result")
	advance(7 * time.Second) // past the 6s intermission
	eng.Tick()
	capture("next_round_intro")
	advance(5 * time.Second)
	submitAll(2, false)
	capture("live_with_guesses")
	eng.Forfeit("m1", s.players[0])
	capture("ended")
	return phaseSnaps
}

func serialize(t *testing.T, snap *contracts.MatchSnapshot, userID string) map[string]any {
	t.Helper()
	client := contracts.ClientSnapshotForPlayer(snap, userID)
	if client == nil {
		t.Fatalf("serializer returned nil for user %q", userID)
	}
	raw, err := json.Marshal(client)
	if err != nil {
		t.Fatalf("marshal client snapshot: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("decode client snapshot: %v", err)
	}
	return decoded
}

func assertPrivacyInvariants(t *testing.T, snap *contracts.MatchSnapshot, h harness, userID string) {
	t.Helper()
	decoded := serialize(t, snap, userID)

	for key := range decoded {
		if !permittedTopLevelKeys[key] {
			t.Fatalf("user %q: top-level key %q is not on the privacy allowlist", userID, key)
		}
	}

	// The live round's answer location must never be serialized.
	if current, ok := decoded["currentRound"].(map[string]any); ok {
		loc, _ := current["location"].(map[string]any)
		for _, forbidden := range []string{"lat", "lng", "country"} {
			if _, present := loc[forbidden]; present {
				t.Fatalf("user %q: current round location leaks %q", userID, forbidden)
			}
		}
	}

	// Authoritative per-player guess coordinates must never be serialized.
	if players, ok := decoded["players"].(map[string]any); ok {
		for id, rawPlayer := range players {
			player, _ := rawPlayer.(map[string]any)
			for _, forbidden := range []string{"lastGuessLat", "lastGuessLng", "hasGuess"} {
				if _, present := player[forbidden]; present {
					t.Fatalf("user %q: player %q state leaks %q", userID, id, forbidden)
				}
			}
		}
	}

	// A round still being played must never appear as a resolved result.
	if current, ok := decoded["currentRound"].(map[string]any); ok {
		if last, ok := decoded["lastRoundResult"].(map[string]any); ok {
			if last["roundId"] == current["roundId"] {
				t.Fatalf("user %q: in-progress round %v is disclosed as a resolved result", userID, current["roundId"])
			}
		}
	}

	// Self visibility: only the recipient gets a self block, and only their own
	// in-progress guess, and only while the round is actually live.
	if self, hasSelf := decoded["self"].(map[string]any); hasSelf {
		if self["userId"] != userID {
			t.Fatalf("user %q: self block belongs to %v", userID, self["userId"])
		}
		guess, hasGuess := self["currentGuess"]
		expectedVisible := snap.Phase == contracts.PhaseLive &&
			snap.RoundPhase == contracts.RoundPhaseLive &&
			snap.Players[userID].HasGuess
		if hasGuess != expectedVisible {
			t.Fatalf("user %q: self currentGuess visibility = %v, want %v", userID, hasGuess, expectedVisible)
		}
		if hasGuess {
			point, _ := guess.(map[string]any)
			wantLat, wantLng := guessFor(userID, roundNumberOf(snap))
			if point["lat"] != wantLat || point["lng"] != wantLng {
				t.Fatalf("user %q: self guess %+v does not match own submitted guess (%v,%v)", userID, point, wantLat, wantLng)
			}
		}
	}

	// Teammate visibility: team guesses exist only for team duels in a live
	// round, only for the recipient's own team, and never for the recipient or
	// an opponent.
	if team, hasTeam := decoded["team"].(map[string]any); hasTeam {
		if snap.Mode != contracts.ModeTeamDuel || snap.Phase != contracts.PhaseLive || snap.RoundPhase != contracts.RoundPhaseLive {
			t.Fatalf("user %q: team guesses exposed outside a live team-duel round", userID)
		}
		selfTeam := snap.Players[userID].TeamID
		if selfTeam == "" {
			t.Fatalf("user %q: team block present for teamless recipient", userID)
		}
		guesses, _ := team["guesses"].(map[string]any)
		for id := range guesses {
			if id == userID {
				t.Fatalf("user %q: team guesses include self", userID)
			}
			if snap.Players[id].TeamID != selfTeam {
				t.Fatalf("user %q: team guesses include non-teammate %q", userID, id)
			}
			wantLat, wantLng := guessFor(id, roundNumberOf(snap))
			point, _ := guesses[id].(map[string]any)
			if point["lat"] != wantLat || point["lng"] != wantLng {
				t.Fatalf("user %q: teammate %q guess %+v does not match the authoritative guess", userID, id, point)
			}
		}
	} else if snap.Mode == contracts.ModeTeamDuel && snap.Phase == contracts.PhaseLive && snap.RoundPhase == contracts.RoundPhaseLive {
		for id, p := range snap.Players {
			if id != userID && p.TeamID == snap.Players[userID].TeamID && p.HasGuess {
				t.Fatalf("user %q: teammate %q has a guess but no team block was serialized", userID, id)
			}
		}
	}
}

func roundNumberOf(snap *contracts.MatchSnapshot) int {
	if snap.CurrentRound != nil {
		return snap.CurrentRound.RoundNumber
	}
	if snap.LastRoundResult != nil {
		return snap.LastRoundResult.RoundNumber
	}
	return 1
}

func TestSnapshotPrivacyAcrossModesAndPhases(t *testing.T) {
	scenarios := []scenario{
		{name: "duel", mode: contracts.ModeDuel, kind: contracts.KindRankedDuel, players: []string{"alice", "bob"}},
		{name: "team_duel", mode: contracts.ModeTeamDuel, kind: contracts.KindTeamDuel, players: []string{"alice", "bob", "carol", "dave"}, teams: map[string]string{"alice": "a", "carol": "a", "bob": "b", "dave": "b"}},
		{name: "free_for_all", mode: contracts.ModeFreeForAll, kind: contracts.KindFreeForAll, players: []string{"alice", "bob", "carol"}},
		{name: "singleplayer", useSoloEngine: true, players: []string{"alice"}},
	}
	for _, s := range scenarios {
		t.Run(s.name, func(t *testing.T) {
			phases := runScenario(t, s)
			for phase, h := range phases {
				t.Run(phase, func(t *testing.T) {
					for _, participant := range h.players {
						assertPrivacyInvariants(t, h.snap, h, participant)
					}
					// A user who is not in the match gets no self or team block.
					decoded := serialize(t, h.snap, "outsider")
					if _, present := decoded["self"]; present {
						t.Fatal("outsider received a self block")
					}
					if _, present := decoded["team"]; present {
						t.Fatal("outsider received a team block")
					}
				})
			}
		})
	}
}

func TestSerializationDoesNotMutateAuthoritativeState(t *testing.T) {
	base := time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)
	var nowMs int64
	clock := func() time.Time { return base.Add(time.Duration(nowMs) * time.Millisecond) }
	prov := func(string, int) (contracts.LocationPoint, error) {
		return contracts.LocationPoint{Lat: 48.85, Lng: 2.35, Country: "fr"}, nil
	}
	eng := duel.NewWithClock(prov, clock)
	teams := map[string]string{"alice": "a", "bob": "a", "carol": "b", "dave": "b"}
	cfg := contracts.MatchConfig{RoundTimerMode: contracts.RoundTimerFixed, RoundTimeLimitMS: 45_000}
	if _, err := eng.CreateMatchWithOptions("m1", []string{"alice", "bob", "carol", "dave"}, nil, duel.MatchOptions{Kind: contracts.KindTeamDuel, Teams: teams, Config: cfg}); err != nil {
		t.Fatalf("create match: %v", err)
	}
	nowMs += 5000
	for _, p := range []string{"alice", "bob", "carol", "dave"} {
		lat, lng := guessFor(p, 1)
		eng.SubmitGuess(contracts.GuessPayload{UserID: p, MatchID: "m1", RoundID: "m1:r1", Lat: lat, Lng: lng, Finalize: false})
	}

	before, err := eng.GetSnapshot("m1")
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	beforeRaw, err := json.Marshal(before)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	for _, userID := range []string{"alice", "bob", "carol", "dave", "outsider"} {
		contracts.ClientSnapshotForPlayer(before, userID)
	}

	after, err := eng.GetSnapshot("m1")
	if err != nil {
		t.Fatalf("snapshot after serialization: %v", err)
	}
	afterRaw, err := json.Marshal(after)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !reflect.DeepEqual(beforeRaw, afterRaw) {
		t.Fatal("serializing for recipients mutated the authoritative snapshot")
	}

	// In-progress guesses must survive untouched and still drive round
	// resolution afterwards.
	nowMs += 60_000
	eng.Tick()
	resolved, err := eng.GetSnapshot("m1")
	if err != nil {
		t.Fatalf("snapshot after resolution: %v", err)
	}
	if resolved.LastRoundResult == nil {
		t.Fatal("round did not resolve after serialization")
	}
	for id, result := range resolved.LastRoundResult.Players {
		wantLat, wantLng := guessFor(id, 1)
		if result.Lat != wantLat || result.Lng != wantLng {
			t.Fatalf("player %q resolved guess %+v lost its submitted coordinates", id, result)
		}
	}
}
