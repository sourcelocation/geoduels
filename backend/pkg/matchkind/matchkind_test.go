package matchkind_test

import (
	"testing"

	"geoduels/pkg/contracts"
	"geoduels/pkg/matchkind"
)

// legacyMatch is a match as the code before kinds described it: an engine mode, an unranked flag,
// and whether a party started it. Each start path produced exactly one shape.
type legacyMatch struct {
	name     string
	mode     contracts.MatchMode
	unranked bool
	party    bool
	kind     matchkind.Kind // what the same start path creates now
}

var legacyStarts = []legacyMatch{
	{name: "queue", mode: contracts.ModeDuel, kind: matchkind.RankedDuel},
	{name: "party duel", mode: contracts.ModeDuel, unranked: true, party: true, kind: matchkind.PrivateDuel},
	{name: "party team duel", mode: contracts.ModeTeamDuel, unranked: true, party: true, kind: matchkind.TeamDuel},
	{name: "party free for all", mode: contracts.ModeFreeForAll, unranked: true, party: true, kind: matchkind.FreeForAll},
	{name: "solo", mode: contracts.ModeSingleplayer, kind: matchkind.Solo},
}

// legacyRotation is the old rule for whether a match played the ruleset's rotation map.
func legacyRotation(m legacyMatch) bool {
	return m.mode == contracts.ModeDuel && !m.unranked && !m.party
}

// The rules each place used to apply to whether a match was rated, as they were written, before
// the kind table replaced them.
var legacyRules = map[string]func(m legacyMatch) bool{
	"session ranked column": func(m legacyMatch) bool { return !m.unranked && m.mode == contracts.ModeDuel && !m.party },
	"finalize rating":       func(m legacyMatch) bool { return m.mode == contracts.ModeDuel && !m.unranked && !m.party },
	"history ranked":        func(m legacyMatch) bool { return m.mode == contracts.ModeDuel && !m.unranked && !m.party },
	"risk analysis":         func(m legacyMatch) bool { return m.mode == contracts.ModeDuel && !m.unranked },
	"engine rating preview": func(m legacyMatch) bool { return m.mode == contracts.ModeDuel && !m.unranked },
	"engine disconnect forfeit": func(m legacyMatch) bool {
		return m.mode != contracts.ModeSingleplayer && !m.unranked
	},
}

func legacyPreset(m legacyMatch) matchkind.Kind {
	switch m.mode {
	case contracts.ModeSingleplayer:
		return matchkind.Solo
	case contracts.ModeTeamDuel:
		return matchkind.TeamDuel
	case contracts.ModeFreeForAll:
		return matchkind.FreeForAll
	default:
		if m.party || m.unranked {
			return matchkind.PrivateDuel
		}
		return matchkind.RankedDuel
	}
}

// The kind each start path creates now must say what every old rule said about the match that
// path created then.
func TestKindsAgreeWithTheRulesTheyReplaced(t *testing.T) {
	for _, m := range legacyStarts {
		spec, ok := matchkind.Lookup(m.kind)
		if !ok {
			t.Fatalf("%s: no spec for %s", m.name, m.kind)
		}
		for rule, rated := range legacyRules {
			if rule == "engine disconnect forfeit" && m.mode == contracts.ModeSingleplayer {
				continue
			}
			if got := rated(m); got != spec.Rated {
				t.Errorf("%s: %s said %v, the kind says rated=%v", m.name, rule, got, spec.Rated)
			}
		}
		if got := legacyRotation(m); got != (spec.Maps == matchkind.MapsRotation) {
			t.Errorf("%s: rotation map pool said %v, the kind says %v", m.name, got, spec.Maps == matchkind.MapsRotation)
		}
		if got := legacyPreset(m); got != m.kind {
			t.Errorf("%s: the preset was %s, the kind is %s", m.name, got, m.kind)
		}
		if spec.Mode != m.mode {
			t.Errorf("%s: mode was %s, the kind says %s", m.name, m.mode, spec.Mode)
		}
		// Starting another match never replaced a duel, team duel or free-for-all, only solo.
		if replaceable := m.mode == contracts.ModeSingleplayer; spec.Replaceable != replaceable {
			t.Errorf("%s: replaceable was %v, the kind says %v", m.name, replaceable, spec.Replaceable)
		}
		historyRanked := legacyRules["history ranked"](m)
		if got := matchkind.FromHistory(m.mode, historyRanked); got != m.kind {
			t.Errorf("%s: its history row reads back as %s, want %s", m.name, got, m.kind)
		}
		if m.party {
			if got, ok := matchkind.ForParty(m.mode); !ok || got != m.kind {
				t.Errorf("%s: a party in %s mode plays %s, want %s", m.name, m.mode, got, m.kind)
			}
		}
	}
}

func TestEveryKindIsDescribedConsistently(t *testing.T) {
	for _, kind := range matchkind.All() {
		spec, ok := matchkind.Lookup(kind)
		if !ok || spec.Kind != kind {
			t.Fatalf("%s: spec missing or misnamed", kind)
		}
		if spec.MinPlayers < 1 || spec.MinPlayers > spec.MaxPlayers {
			t.Errorf("%s: players %d..%d", kind, spec.MinPlayers, spec.MaxPlayers)
		}
		if spec.IdleEnd <= 0 {
			t.Errorf("%s: an abandoned match would keep its players seated forever", kind)
		}
		if spec.Rated && (spec.GuestsAllow || spec.Origin != matchkind.OriginQueue) {
			t.Errorf("%s: rated matches come from the queue and seat no guests", kind)
		}
		if spec.Replaceable != (spec.Origin == matchkind.OriginDirect) || (spec.Replaceable && spec.MaxPlayers != 1) {
			t.Errorf("%s: only a match a player starts alone may be replaced", kind)
		}
		if (spec.Engine == matchkind.EngineSolo) != (spec.Mode == contracts.ModeSingleplayer) {
			t.Errorf("%s: engine %s does not run mode %s", kind, spec.Engine, spec.Mode)
		}
		if spec.Origin == matchkind.OriginParty {
			if got, ok := matchkind.ForParty(spec.Mode); !ok || got != kind {
				t.Errorf("%s: a party in %s mode plays %s", kind, spec.Mode, got)
			}
		}
	}
	if _, ok := matchkind.ForParty(contracts.ModeSingleplayer); ok {
		t.Error("a party cannot play solo")
	}
}

func TestCheckSeats(t *testing.T) {
	seats := func(teams ...string) []matchkind.Seat {
		out := make([]matchkind.Seat, len(teams))
		for i, team := range teams {
			out[i] = matchkind.Seat{UserID: string(rune('a' + i)), Team: team}
		}
		return out
	}
	for _, tc := range []struct {
		name  string
		kind  matchkind.Kind
		seats []matchkind.Seat
		ok    bool
	}{
		{"ranked duel", matchkind.RankedDuel, seats("", ""), true},
		{"ranked duel of three", matchkind.RankedDuel, seats("", "", ""), false},
		{"solo", matchkind.Solo, seats(""), true},
		{"solo of two", matchkind.Solo, seats("", ""), false},
		{"team duel", matchkind.TeamDuel, seats("a", "b", "a"), true},
		{"team duel with one side", matchkind.TeamDuel, seats("a", "a"), false},
		{"team duel without teams", matchkind.TeamDuel, seats("", ""), false},
		{"free for all with teams", matchkind.FreeForAll, seats("a", "b"), false},
		{"free for all", matchkind.FreeForAll, seats("", "", ""), true},
		{"seated twice", matchkind.PrivateDuel, []matchkind.Seat{{UserID: "a"}, {UserID: "a"}}, false},
		{"unknown kind", "", seats("", ""), false},
	} {
		if err := matchkind.Of(tc.kind).CheckSeats(tc.seats); (err == nil) != tc.ok {
			t.Errorf("%s: err = %v, want ok=%v", tc.name, err, tc.ok)
		}
	}
}
