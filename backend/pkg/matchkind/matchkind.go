// Package matchkind is the one description of each kind of match. A match's kind is fixed when it
// is created; everything else about it (which engine runs it, whether it is rated, which maps it
// plays, who may be in it, what happens when a player starts something else) is looked up here.
package matchkind

import (
	"fmt"
	"time"

	"geoduels/pkg/contracts"
)

type Kind = contracts.MatchKind

const (
	RankedDuel  = contracts.KindRankedDuel
	PrivateDuel = contracts.KindPrivateDuel
	TeamDuel    = contracts.KindTeamDuel
	FreeForAll  = contracts.KindFreeForAll
	Solo        = contracts.KindSolo
)

// Origin is how a match came to be.
type Origin string

const (
	OriginQueue  Origin = "queue"
	OriginParty  Origin = "party"
	OriginDirect Origin = "direct"
)

// Engine is what runs a match on a gameplay node.
type Engine string

const (
	// EngineVersus runs players against each other: duels, team duels and free-for-alls.
	EngineVersus Engine = "versus"
	// EngineSolo runs one player through rounds they advance themselves.
	EngineSolo Engine = "solo"
)

// Maps says where a match's map comes from.
type Maps int

const (
	// MapsRotation plays the configured map for the ruleset, whatever was asked for.
	MapsRotation Maps = iota
	// MapsChosen plays the map in the match's config, falling back to the rotation.
	MapsChosen
)

type Spec struct {
	Kind   Kind
	Engine Engine
	// Mode is the engine's layout: duel, team_duel, free_for_all or singleplayer.
	Mode contracts.MatchMode
	// Rated matches change rating, ranked stats and badges, and are checked by the risk engine.
	Rated bool
	// Stats matches count toward games played and wins.
	Stats      bool
	MinPlayers int
	MaxPlayers int
	// Teams matches need players on both teams a and b.
	Teams bool
	Maps  Maps
	// Replaceable matches end when their player starts another match.
	Replaceable bool
	// IdleEnd ends a match nobody played for that long: solo as forfeited, the others as a draw.
	IdleEnd     time.Duration
	Origin      Origin
	GuestsAllow bool
}

// idleEnd is how long a match may go without anyone playing it before it ends.
const idleEnd = 15 * time.Minute

var specs = map[Kind]Spec{
	RankedDuel: {
		Kind: RankedDuel, Engine: EngineVersus, Mode: contracts.ModeDuel, Rated: true, Stats: true,
		MinPlayers: 2, MaxPlayers: 2, Maps: MapsRotation, IdleEnd: idleEnd, Origin: OriginQueue,
	},
	PrivateDuel: {
		Kind: PrivateDuel, Engine: EngineVersus, Mode: contracts.ModeDuel, Stats: true,
		MinPlayers: 2, MaxPlayers: 2, Maps: MapsChosen, IdleEnd: idleEnd, Origin: OriginParty, GuestsAllow: true,
	},
	TeamDuel: {
		Kind: TeamDuel, Engine: EngineVersus, Mode: contracts.ModeTeamDuel, Teams: true,
		MinPlayers: contracts.MinPartyMembers, MaxPlayers: contracts.MaxPartyMembers, Maps: MapsChosen, IdleEnd: idleEnd, Origin: OriginParty, GuestsAllow: true,
	},
	FreeForAll: {
		Kind: FreeForAll, Engine: EngineVersus, Mode: contracts.ModeFreeForAll,
		MinPlayers: contracts.MinPartyMembers, MaxPlayers: contracts.MaxPartyMembers, Maps: MapsChosen, IdleEnd: idleEnd, Origin: OriginParty, GuestsAllow: true,
	},
	Solo: {
		Kind: Solo, Engine: EngineSolo, Mode: contracts.ModeSingleplayer,
		MinPlayers: 1, MaxPlayers: 1, Maps: MapsChosen, Replaceable: true, IdleEnd: idleEnd, Origin: OriginDirect, GuestsAllow: true,
	},
}

// All lists every kind.
func All() []Kind {
	return []Kind{RankedDuel, PrivateDuel, TeamDuel, FreeForAll, Solo}
}

// Lookup returns a kind's spec.
func Lookup(kind Kind) (Spec, bool) {
	spec, ok := specs[kind]
	return spec, ok
}

// Of returns a kind's spec, or an empty one that allows nothing.
func Of(kind Kind) Spec {
	return specs[kind]
}

// ForParty is the kind a party plays in its mode.
func ForParty(mode contracts.MatchMode) (Kind, bool) {
	switch mode {
	case contracts.ModeDuel:
		return PrivateDuel, true
	case contracts.ModeTeamDuel:
		return TeamDuel, true
	case contracts.ModeFreeForAll:
		return FreeForAll, true
	default:
		return "", false
	}
}

// FromHistory is the kind of a match recorded before kinds existed, from its history row.
func FromHistory(mode contracts.MatchMode, ranked bool) Kind {
	switch mode {
	case contracts.ModeSingleplayer:
		return Solo
	case contracts.ModeTeamDuel:
		return TeamDuel
	case contracts.ModeFreeForAll:
		return FreeForAll
	default:
		if ranked {
			return RankedDuel
		}
		return PrivateDuel
	}
}

// Seat is one player's place in a match.
type Seat struct {
	UserID string
	Team   string
}

// CheckSeats reports whether these players can play a match of this kind.
func (s Spec) CheckSeats(seats []Seat) error {
	if s.Kind == "" {
		return fmt.Errorf("unknown match kind")
	}
	if len(seats) < s.MinPlayers || len(seats) > s.MaxPlayers {
		if s.MinPlayers == s.MaxPlayers {
			return fmt.Errorf("%s needs exactly %d players", s.Kind, s.MinPlayers)
		}
		return fmt.Errorf("%s needs %d to %d players", s.Kind, s.MinPlayers, s.MaxPlayers)
	}
	seen := map[string]bool{}
	teams := map[string]int{}
	for _, seat := range seats {
		if seat.UserID == "" {
			return fmt.Errorf("player id required")
		}
		if seen[seat.UserID] {
			return fmt.Errorf("player %s is seated twice", seat.UserID)
		}
		seen[seat.UserID] = true
		if s.Teams {
			if seat.Team != "a" && seat.Team != "b" {
				return fmt.Errorf("team must be a or b")
			}
			teams[seat.Team]++
		} else if seat.Team != "" {
			return fmt.Errorf("%s has no teams", s.Kind)
		}
	}
	if s.Teams && (teams["a"] == 0 || teams["b"] == 0) {
		return fmt.Errorf("team duel requires players on both teams")
	}
	return nil
}
