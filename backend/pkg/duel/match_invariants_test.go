package duel_test

import (
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"reflect"
	"testing"
	"time"

	"geoduels/pkg/contracts"
	"geoduels/pkg/duel"
	"geoduels/pkg/gameplay"
)

// Independently defined gameplay constants (not imported from the engine).
const (
	invariantStartingHP = 6000
	invariantMaxRounds  = 20
	invariantFFARounds  = 5
)

var invariantSeeds = []int64{1, 7, 42, 1337, 90210}

type simMode struct {
	name    string
	mode    contracts.MatchMode
	kind    contracts.MatchKind
	players []string
	teams   map[string]string
}

// checkInvariants asserts independently defined gameplay invariants against a
// fresh authoritative snapshot taken after every applied command.
func checkInvariants(t *testing.T, tag string, snap *contracts.MatchSnapshot, s simMode, prevHP map[string]int, endedSeen *bool) {
	t.Helper()
	if len(snap.Players) != len(s.players) {
		t.Fatalf("%s: player set changed: got %d players, want %d", tag, len(snap.Players), len(s.players))
	}
	for _, id := range s.players {
		p, ok := snap.Players[id]
		if !ok {
			t.Fatalf("%s: participant %q missing from snapshot", tag, id)
		}
		if p.HP < 0 || p.HP > invariantStartingHP {
			t.Fatalf("%s: player %q HP %d outside [0,%d]", tag, id, p.HP, invariantStartingHP)
		}
		if prev, seen := prevHP[id]; seen && p.HP > prev {
			t.Fatalf("%s: player %q HP increased from %d to %d", tag, id, prev, p.HP)
		}
		prevHP[id] = p.HP
		if p.TotalScore < 0 || p.TotalScore > invariantMaxRounds*gameplay.MaxScore {
			t.Fatalf("%s: player %q total score %d out of bounds", tag, id, p.TotalScore)
		}
		if p.DamageMultiplier < 1 {
			t.Fatalf("%s: player %q damage multiplier %v below 1", tag, id, p.DamageMultiplier)
		}
	}

	if snap.Mode != s.mode {
		t.Fatalf("%s: mode changed from %v to %v", tag, s.mode, snap.Mode)
	}

	// Round bookkeeping: results are sequential, bounded, and consistent.
	if len(snap.RoundResults) > invariantMaxRounds {
		t.Fatalf("%s: %d round results exceeds the round cap", tag, len(snap.RoundResults))
	}
	for i, result := range snap.RoundResults {
		if result.RoundNumber != i+1 {
			t.Fatalf("%s: round result %d has round number %d", tag, i, result.RoundNumber)
		}
		for id, pr := range result.Players {
			if pr.Score < 0 || pr.Score > gameplay.MaxScore {
				t.Fatalf("%s: round %d player %q score %d out of bounds", tag, result.RoundNumber, id, pr.Score)
			}
			if math.IsNaN(pr.DistanceKm) || pr.DistanceKm < 0 {
				t.Fatalf("%s: round %d player %q distance %v invalid", tag, result.RoundNumber, id, pr.DistanceKm)
			}
		}
	}
	if len(snap.RoundResults) > 0 {
		last := snap.RoundResults[len(snap.RoundResults)-1]
		if snap.LastRoundResult == nil || snap.LastRoundResult.RoundID != last.RoundID {
			t.Fatalf("%s: lastRoundResult does not match the newest round result", tag)
		}
	}

	// A live match has a current round except during the result intermission;
	// a finished match never does.
	if snap.State == contracts.MatchLive && snap.CurrentRound == nil && snap.Phase != contracts.PhaseRoundResult {
		t.Fatalf("%s: live match without a current round (phase %v)", tag, snap.Phase)
	}
	if snap.State == contracts.MatchEnded {
		if snap.CurrentRound != nil {
			t.Fatalf("%s: ended match still exposes a current round", tag)
		}
		if snap.Phase != contracts.PhaseEnded || snap.RoundPhase != contracts.RoundPhaseEnded {
			t.Fatalf("%s: ended match reports phase %v/%v", tag, snap.Phase, snap.RoundPhase)
		}
		*endedSeen = true
	}
	if snap.CurrentRound != nil {
		if snap.CurrentRound.RoundNumber < len(snap.RoundResults) || snap.CurrentRound.RoundNumber > len(snap.RoundResults)+1 {
			t.Fatalf("%s: current round %d incompatible with %d resolved rounds", tag, snap.CurrentRound.RoundNumber, len(snap.RoundResults))
		}
	}

	// Mode-specific structure.
	if s.mode == contracts.ModeDuel && snap.Teams != nil {
		t.Fatalf("%s: plain duel exposed team state", tag)
	}
	if s.mode == contracts.ModeTeamDuel {
		if len(snap.Teams) != 2 {
			t.Fatalf("%s: team duel has %d teams, want 2", tag, len(snap.Teams))
		}
		for _, team := range snap.Teams {
			if team.HP < 0 || team.HP > invariantStartingHP {
				t.Fatalf("%s: team %q HP %d outside [0,%d]", tag, team.TeamID, team.HP, invariantStartingHP)
			}
			if snap.State == contracts.MatchLive {
				for _, member := range team.Players {
					if snap.Players[member].HP != team.HP {
						t.Fatalf("%s: member %q HP %d diverges from team %q HP %d", tag, member, snap.Players[member].HP, team.TeamID, team.HP)
					}
				}
			}
		}
	}
	if s.mode == contracts.ModeFreeForAll {
		if snap.Teams != nil {
			t.Fatalf("%s: free-for-all exposed team state", tag)
		}
		if len(snap.RoundResults) > invariantFFARounds {
			t.Fatalf("%s: free-for-all played %d rounds, cap is %d", tag, len(snap.RoundResults), invariantFFARounds)
		}
	}

	// Terminal states are absorbing: nothing may revive an ended match.
	if *endedSeen && snap.State != contracts.MatchEnded {
		t.Fatalf("%s: match left the ended state", tag)
	}
}

func applyCommand(t *testing.T, eng *duel.Engine, m *simMode, rng *rand.Rand, now *time.Time) {
	t.Helper()
	matchID := "sim"
	action := rng.Intn(100)
	switch {
	case action < 55: // guess
		userID := m.players[rng.Intn(len(m.players))]
		snap, _ := eng.GetSnapshot(matchID)
		if snap.State != contracts.MatchLive || snap.CurrentRound == nil {
			return
		}
		lat := rng.Float64()*180 - 90
		lng := rng.Float64()*360 - 180
		eng.SubmitGuess(contracts.GuessPayload{
			UserID: userID, MatchID: matchID, RoundID: snap.CurrentRound.RoundID,
			Lat: lat, Lng: lng, Finalize: rng.Intn(100) < 30,
		})
	case action < 80: // tick
		eng.Tick()
	case action < 90: // disconnect
		userID := m.players[rng.Intn(len(m.players))]
		eng.MarkDisconnected(matchID, userID)
	case action < 98: // resume
		userID := m.players[rng.Intn(len(m.players))]
		eng.MarkResumed(matchID, userID)
	default: // rare forfeit
		userID := m.players[rng.Intn(len(m.players))]
		eng.Forfeit(matchID, userID)
	}
	*now = now.Add(time.Duration(rng.Intn(8)+1) * time.Second)
}

func TestMatchInvariantsUnderGeneratedSequences(t *testing.T) {
	modes := []simMode{
		{name: "duel", mode: contracts.ModeDuel, kind: contracts.KindRankedDuel, players: []string{"p1", "p2"}},
		{name: "team_duel", mode: contracts.ModeTeamDuel, kind: contracts.KindTeamDuel, players: []string{"p1", "p2", "p3", "p4"}, teams: map[string]string{"p1": "a", "p3": "a", "p2": "b", "p4": "b"}},
		{name: "free_for_all", mode: contracts.ModeFreeForAll, kind: contracts.KindFreeForAll, players: []string{"p1", "p2", "p3"}},
	}
	for _, s := range modes {
		for _, seed := range invariantSeeds {
			t.Run(fmt.Sprintf("%s/seed%d", s.name, seed), func(t *testing.T) {
				rng := rand.New(rand.NewSource(seed))
				base := time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)
				now := base
				eng := duel.NewWithClock(func(string, int) (contracts.LocationPoint, error) {
					return contracts.LocationPoint{Lat: rng.Float64()*140 - 70, Lng: rng.Float64()*360 - 180, Country: "xx"}, nil
				}, func() time.Time { return now })
				if _, err := eng.CreateMatchWithOptions("sim", s.players, nil, duel.MatchOptions{
					Kind: s.kind, Teams: s.teams,
					Config: contracts.MatchConfig{RoundTimerMode: contracts.RoundTimerFixed, RoundTimeLimitMS: 45_000},
				}); err != nil {
					t.Fatalf("create match: %v", err)
				}
				prevHP := map[string]int{}
				endedSeen := false
				accepted := 0

				// Non-participants never pass authorization, and rejected
				// commands change nothing.
				rejectIntruder := func() {
					snapBefore, _ := eng.GetSnapshot("sim")
					beforeRaw, _ := json.Marshal(snapBefore)
					if _, err := eng.SubmitGuess(contracts.GuessPayload{UserID: "intruder", MatchID: "sim", RoundID: "intruder-round", Lat: 1, Lng: 1, Finalize: true}); err == nil {
						t.Fatal("intruder guess accepted")
					}
					if _, err := eng.MarkDisconnected("sim", "intruder"); err == nil {
						t.Fatal("intruder disconnect accepted")
					}
					if _, err := eng.MarkResumed("sim", "intruder"); err == nil {
						t.Fatal("intruder resume accepted")
					}
					if snapBefore.State == contracts.MatchLive {
						if _, err := eng.Forfeit("sim", "intruder"); err == nil {
							t.Fatal("intruder forfeit accepted")
						}
					}
					snapAfter, _ := eng.GetSnapshot("sim")
					afterRaw, _ := json.Marshal(snapAfter)
					if !reflect.DeepEqual(beforeRaw, afterRaw) {
						t.Fatal("rejected intruder commands mutated the match")
					}
				}

				for step := 0; step < 600; step++ {
					applyCommand(t, eng, &s, rng, &now)
					accepted++
					snap, err := eng.GetSnapshot("sim")
					if err != nil {
						t.Fatalf("step %d: snapshot: %v", step, err)
					}
					checkInvariants(t, fmt.Sprintf("step %d", step), snap, s, prevHP, &endedSeen)
					if step == 10 {
						rejectIntruder()
					}
					if endedSeen {
						// Keep hammering the ended match; it must stay ended.
						eng.SubmitGuess(contracts.GuessPayload{UserID: s.players[0], MatchID: "sim", RoundID: "sim:r99", Lat: 1, Lng: 1, Finalize: true})
						eng.Tick()
						snap, err := eng.GetSnapshot("sim")
						if err != nil {
							t.Fatalf("post-terminal snapshot: %v", err)
						}
						checkInvariants(t, fmt.Sprintf("post-terminal step %d", step), snap, s, prevHP, &endedSeen)
					}
				}
				if !endedSeen {
					t.Fatal("generated sequence never reached a terminal state")
				}
				if len(prevHP) == 0 || accepted < 100 {
					t.Fatalf("sequence barely exercised the engine (%d commands)", accepted)
				}
			})
		}
	}
}

// Abandoning a match ends it as a draw, and a removed match is gone from the engine.
func TestAbandonAndRemove(t *testing.T) {
	eng := duel.New(func(string, int) (contracts.LocationPoint, error) {
		return contracts.LocationPoint{Lat: 1, Lng: 2, Country: "xx"}, nil
	})
	if _, err := eng.CreateMatchWithOptions("m", []string{"a", "b"}, nil, duel.MatchOptions{Kind: contracts.KindRankedDuel}); err != nil {
		t.Fatal(err)
	}
	snap, err := eng.Abandon("m")
	if err != nil {
		t.Fatal(err)
	}
	if snap.State != contracts.MatchEnded {
		t.Fatalf("state = %s, want ended", snap.State)
	}
	for id, player := range snap.Players {
		if player.HP != 0 {
			t.Errorf("%s has %d HP after abandon, want a draw at zero", id, player.HP)
		}
	}
	eng.Remove("m")
	if _, err := eng.GetSnapshot("m"); err == nil {
		t.Fatal("removed match still in the engine")
	}
}
