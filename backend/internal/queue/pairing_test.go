package queue

import (
	"fmt"
	"math/rand"
	"testing"
	"time"
)

var pairingSeeds = []int64{1, 2, 3, 5, 8, 13, 21, 34, 55, 89}

func randomTickets(rng *rand.Rand, now time.Time) []Ticket {
	var tickets []Ticket
	for i := 0; i < 2+rng.Intn(40); i++ {
		userID := fmt.Sprintf("p%02d", i)
		joined := now.Add(-time.Duration(rng.Intn(20_000)) * time.Millisecond)
		mmr := 500 + rng.Intn(1200)
		for _, variant := range Ranked {
			if rng.Intn(3) > 0 {
				tickets = append(tickets, Ticket{UserID: userID, Variant: variant, MMR: mmr, JoinedAt: joined})
			}
		}
	}
	return tickets
}

// Pairings never seats a player twice, only pairs players queued for the variant who both waited a
// second and fall within each other's window, and leaves no two such players unpaired.
func TestPairingsInvariants(t *testing.T) {
	now := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	for _, seed := range pairingSeeds {
		t.Run(fmt.Sprintf("seed%d", seed), func(t *testing.T) {
			tickets := randomTickets(rand.New(rand.NewSource(seed)), now)
			queued := map[string]Ticket{}
			for _, ticket := range tickets {
				queued[ticket.UserID+"/"+ticket.Variant] = ticket
			}
			paired := map[string]bool{}
			for _, pair := range Pairings(tickets, now) {
				for _, ticket := range []Ticket{pair.First, pair.Second} {
					if paired[ticket.UserID] {
						t.Fatalf("%s paired twice", ticket.UserID)
					}
					paired[ticket.UserID] = true
					if _, ok := queued[ticket.UserID+"/"+pair.Variant]; !ok {
						t.Fatalf("%s paired in %s without queueing for it", ticket.UserID, pair.Variant)
					}
					if now.Sub(ticket.JoinedAt) < minMutualWait {
						t.Fatalf("%s paired before waiting a second", ticket.UserID)
					}
				}
				if diff := abs(pair.First.MMR - pair.Second.MMR); diff > window(pair.First, pair.Second, now) {
					t.Fatalf("pair %s/%s is %d apart, outside the window", pair.First.UserID, pair.Second.UserID, diff)
				}
				if pair.Second.JoinedAt.Before(pair.First.JoinedAt) {
					t.Fatalf("pair %s/%s: the longer-waiting player must come first", pair.First.UserID, pair.Second.UserID)
				}
			}
			for _, a := range tickets {
				for _, b := range tickets {
					if a.UserID >= b.UserID || a.Variant != b.Variant || paired[a.UserID] || paired[b.UserID] {
						continue
					}
					if now.Sub(a.JoinedAt) >= minMutualWait && now.Sub(b.JoinedAt) >= minMutualWait && abs(a.MMR-b.MMR) <= window(a, b, now) {
						t.Fatalf("%s and %s could play each other in %s but were left waiting", a.UserID, b.UserID, a.Variant)
					}
				}
			}
		})
	}
}

func TestPairingsWidenWithWaiting(t *testing.T) {
	now := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	pairAfter := func(wait time.Duration, diff int) bool {
		tickets := []Ticket{
			{UserID: "a", Variant: Moving, MMR: 1000, JoinedAt: now.Add(-wait)},
			{UserID: "b", Variant: Moving, MMR: 1000 + diff, JoinedAt: now.Add(-time.Second)},
		}
		return len(Pairings(tickets, now)) == 1
	}
	for _, tc := range []struct {
		wait time.Duration
		diff int
		want bool
	}{
		{time.Second, 150, true},
		{time.Second, 151, false},
		{6 * time.Second, 375, true},
		{6 * time.Second, 376, false},
		{time.Minute, 500, true},
		{time.Minute, 501, false},
		{500 * time.Millisecond, 0, false},
	} {
		if got := pairAfter(tc.wait, tc.diff); got != tc.want {
			t.Errorf("waited %v, %d apart: paired=%v, want %v", tc.wait, tc.diff, got, tc.want)
		}
	}
}

// A player queued for several variants is paired in the first one that has an opponent, and the
// longest-waiting player picks the closest rating.
func TestPairingsOrder(t *testing.T) {
	now := time.Date(2026, 10, 10, 12, 0, 0, 0, time.UTC)
	tickets := []Ticket{
		{UserID: "old", Variant: Moving, MMR: 1000, JoinedAt: now.Add(-10 * time.Second)},
		{UserID: "old", Variant: NoMoveHidden, MMR: 1000, JoinedAt: now.Add(-10 * time.Second)},
		{UserID: "far", Variant: Moving, MMR: 1300, JoinedAt: now.Add(-9 * time.Second)},
		{UserID: "near", Variant: Moving, MMR: 1050, JoinedAt: now.Add(-2 * time.Second)},
		{UserID: "other", Variant: NoMoveHidden, MMR: 1000, JoinedAt: now.Add(-5 * time.Second)},
	}
	pairs := Pairings(tickets, now)
	if len(pairs) != 1 {
		t.Fatalf("pairs = %+v, want one", pairs)
	}
	if got := pairs[0]; got.Variant != Moving || got.First.UserID != "old" || got.Second.UserID != "near" {
		t.Fatalf("pair = %s %s/%s, want moving old/near", got.Variant, got.First.UserID, got.Second.UserID)
	}
}

func TestParseVariants(t *testing.T) {
	for raw, want := range map[string]string{
		"":                      "moving",
		"moving":                "moving",
		"no_move_hidden,moving": "no_move_hidden,moving",
		"nmpz,MOVING, moving":   "moving",
		"unknown":               "moving",
		"moving_hidden,no_move": "moving",
	} {
		got := ""
		for i, variant := range ParseVariants(raw) {
			if i > 0 {
				got += ","
			}
			got += variant
		}
		if got != want {
			t.Errorf("ParseVariants(%q) = %q, want %q", raw, got, want)
		}
	}
}
