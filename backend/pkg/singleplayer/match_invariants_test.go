package singleplayer_test

import (
	"fmt"
	"math/rand"
	"testing"
	"time"

	"geoduels/pkg/gameplay"
	"geoduels/pkg/singleplayer"
)

const (
	soloMaxRounds  = 5
	soloStartingHP = 6000
	soloIterations = 40
)

func TestSingleplayerInvariantsUnderGeneratedSequences(t *testing.T) {
	for _, seed := range []int64{3, 11, 77} {
		t.Run(fmt.Sprintf("seed%d", seed), func(t *testing.T) {
			rng := rand.New(rand.NewSource(seed))
			base := time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)
			var nowMs int64
			clock := func() time.Time { return base.Add(time.Duration(nowMs) * time.Millisecond) }
			eng := singleplayer.NewWithClock(func(string, int) (gameplay.LocationPoint, error) {
				return gameplay.LocationPoint{Lat: rng.Float64()*140 - 70, Lng: rng.Float64()*360 - 180, Country: "xx"}, nil
			}, clock)
			if _, err := eng.CreateMatch("solo", []string{"player"}, nil); err != nil {
				t.Fatalf("create session: %v", err)
			}

			endedSeen := false
			progressedRounds := 0
			for step := 0; step < soloIterations; step++ {
				nowMs += int64(rng.Intn(10)+1) * 1000
				snap, err := eng.GetSnapshot("solo")
				if err != nil {
					t.Fatalf("step %d: snapshot: %v", step, err)
				}
				p := snap.Players["player"]
				if p.UserID != "player" {
					t.Fatalf("step %d: unexpected player state", step)
				}
				if len(snap.RoundResults) > soloMaxRounds {
					t.Fatalf("step %d: %d rounds exceeds cap %d", step, len(snap.RoundResults), soloMaxRounds)
				}
				sum := 0
				for i, result := range snap.RoundResults {
					if result.RoundNumber != i+1 {
						t.Fatalf("step %d: result %d has round number %d", step, i, result.RoundNumber)
					}
					pr, ok := result.Players["player"]
					if !ok {
						t.Fatalf("step %d: round %d result missing the player", step, i)
					}
					if pr.Score < 0 || pr.Score > gameplay.MaxScore {
						t.Fatalf("step %d: round %d score %d out of bounds", step, i, pr.Score)
					}
					sum += pr.Score
				}
				if p.TotalScore != sum {
					t.Fatalf("step %d: total score %d does not equal the sum of round scores %d", step, p.TotalScore, sum)
				}
				if snap.State == gameplay.MatchEnded {
					endedSeen = true
					if snap.CurrentRound != nil {
						t.Fatalf("step %d: ended session still exposes a current round", step)
					}
				} else if snap.CurrentRound != nil {
					if snap.CurrentRound.RoundNumber != len(snap.RoundResults)+1 {
						t.Fatalf("step %d: current round %d with %d results", step, snap.CurrentRound.RoundNumber, len(snap.RoundResults))
					}
				}

				// Commands from anyone but the session owner are rejected
				// while the session is live.
				if snap.State == gameplay.MatchLive {
					if _, err := eng.SubmitGuess(gameplay.Guess{UserID: "intruder", MatchID: "solo", RoundID: "any", Lat: 1, Lng: 1, Finalize: true}); err == nil {
						t.Fatalf("step %d: intruder guess accepted", step)
					}
					if _, err := eng.Forfeit("solo", "intruder"); err == nil {
						t.Fatalf("step %d: intruder forfeit accepted", step)
					}
				}

				// Drive forward deterministically enough to make progress.
				current := snap.CurrentRound
				if snap.State == gameplay.MatchLive && current != nil {
					progressedRounds = len(snap.RoundResults) + 1
					if _, err := eng.SubmitGuess(gameplay.Guess{
						UserID: "player", MatchID: "solo", RoundID: current.RoundID,
						Lat: rng.Float64()*180 - 90, Lng: rng.Float64()*360 - 180, Finalize: true,
					}); err != nil {
						t.Fatalf("step %d: finalize: %v", step, err)
					}
					eng.AdvanceRound("solo", "player")
					eng.MarkDisconnected("solo", "player")
					eng.MarkResumed("solo", "player")
				}
			}
			if !endedSeen {
				t.Fatal("sequence never reached the ended state")
			}
			if progressedRounds < soloMaxRounds {
				t.Fatalf("sequence only reached round %d, want all %d", progressedRounds, soloMaxRounds)
			}
		})
	}
}
