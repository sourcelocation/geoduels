package queue

import (
	"sort"
	"time"
)

// The rating window two players must fall within: 150 to start, widening by 75 for every two
// seconds the longer-waiting of them has waited, up to 500. Both must have waited a second, so a
// pairing never forms before either has seen they are queued.
const (
	baseWindow      = 150
	widenEvery      = 2 * time.Second
	widenStep       = 75
	maxWindow       = 500
	minMutualWait   = 1 * time.Second
	pairsPerVariant = 50
)

// Pair is two players who will play each other in a variant. First waited longer.
type Pair struct {
	Variant Variant
	First   Ticket
	Second  Ticket
}

// Pairings decides who plays whom among the waiting tickets. Variants are matched in the order of
// Ranked, and within one the longest-waiting player first, against the closest rating in their
// window. A player paired in one variant is out of all of them.
func Pairings(tickets []Ticket, now time.Time) []Pair {
	byVariant := map[Variant][]Ticket{}
	for _, ticket := range tickets {
		byVariant[ticket.Variant] = append(byVariant[ticket.Variant], ticket)
	}
	for _, waiting := range byVariant {
		sort.SliceStable(waiting, func(i, j int) bool {
			if !waiting[i].JoinedAt.Equal(waiting[j].JoinedAt) {
				return waiting[i].JoinedAt.Before(waiting[j].JoinedAt)
			}
			return waiting[i].UserID < waiting[j].UserID
		})
	}
	paired := map[string]bool{}
	var pairs []Pair
	for _, variant := range Ranked {
		waiting := byVariant[variant]
		matched := 0
		for _, self := range waiting {
			if matched >= pairsPerVariant {
				break
			}
			if paired[self.UserID] || now.Sub(self.JoinedAt) < minMutualWait {
				continue
			}
			best, bestDiff := -1, maxWindow+1
			for i, other := range waiting {
				if other.UserID == self.UserID || paired[other.UserID] || now.Sub(other.JoinedAt) < minMutualWait {
					continue
				}
				diff := abs(other.MMR - self.MMR)
				if diff <= window(self, other, now) && diff < bestDiff {
					best, bestDiff = i, diff
				}
			}
			if best < 0 {
				continue
			}
			paired[self.UserID], paired[waiting[best].UserID] = true, true
			pairs = append(pairs, Pair{Variant: variant, First: self, Second: waiting[best]})
			matched++
		}
	}
	return pairs
}

// window is the rating difference two players may have, from the longer of their waits.
func window(a, b Ticket, now time.Time) int {
	wait := max(now.Sub(a.JoinedAt), now.Sub(b.JoinedAt), 0)
	allowed := baseWindow + int(wait/widenEvery)*widenStep
	return min(allowed, maxWindow)
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
