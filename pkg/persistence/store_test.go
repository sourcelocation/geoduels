package persistence

import (
	"os"
	"strings"
	"testing"
)

func TestFinalizeMatchUsesTypedSetBasedStatUpdates(t *testing.T) {
	body, err := os.ReadFile("matches_write.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(body)
	if strings.Contains(source, "case when user_id = $2") {
		t.Fatal("winner must not be passed as a UUID sentinel")
	}
	if got := strings.Count(source, "result.won::integer"); got != 2 {
		t.Fatalf("set-based boolean win expressions = %d, want 2", got)
	}
	if !strings.Contains(source, "final_ranked_delta = result.rating_after - result.rating_before") {
		t.Fatal("set-based final ranked delta update is missing")
	}
}

func TestRecordMatchHistoryCastsReplayExpirationParameters(t *testing.T) {
	body, err := os.ReadFile("matches_write.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(body)
	if strings.Contains(source, "$4 + make_interval(days => $16)") {
		t.Fatal("replay expiration must cast pgx timestamp and integer parameters")
	}
	if !strings.Contains(source, "$4::timestamptz + make_interval(days => $16::integer)") {
		t.Fatal("typed replay expiration expression is missing")
	}
}

func TestProfileHistoryStatsCountDuelsOnly(t *testing.T) {
	body, err := os.ReadFile("profiles.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(body)
	if got := strings.Count(source, "and h.mode = 'duel'"); got < 2 {
		t.Fatalf("profile history_stats duel filters = %d, want at least 2", got)
	}
}

func TestRecordMatchHistoryNormalizesSingleplayerSource(t *testing.T) {
	body, err := os.ReadFile("matches_write.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(body)
	if !strings.Contains(source, "snap.Mode == contracts.ModeSingleplayer") {
		t.Fatal("singleplayer match history must be detected explicitly")
	}
	if !strings.Contains(source, `sourceKind = "solo"`) {
		t.Fatal("singleplayer match history must use solo source_kind")
	}
}

func TestRiskEngineSignalsDoNotCreateLegacyCases(t *testing.T) {
	body, err := os.ReadFile("moderation_enforcement.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(body)
	if strings.Contains(source, "createAutoDetectionCase") {
		t.Fatal("risk-engine ingestion must not create legacy moderation cases")
	}
	if strings.Contains(source, "upsertIncidentForSignal") {
		t.Fatal("risk-engine ingestion must not create incident projections")
	}
	if !strings.Contains(source, "enqueueSignalNotification") {
		t.Fatal("queued risk-engine signals must notify directly")
	}
}

func TestCheatingBanRefundQueryUsesCompactRatingColumns(t *testing.T) {
	body, err := os.ReadFile("moderation_enforcement.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(body)
	if strings.Contains(source, "snapshot_json->'ratingPreview'") {
		t.Fatal("cheating-ban refund query must not depend on retained replay JSON")
	}
	if !strings.Contains(source, "opponent.final_ranked_delta as original_delta") {
		t.Fatal("cheating-ban refund query must use compact persisted rating deltas")
	}
}

func TestNullableTeamEnumsAreCastBeforeEmptyStringFallback(t *testing.T) {
	for _, name := range []string{"parties.go", "chat.go"} {
		body, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		source := string(body)
		if strings.Contains(source, "coalesce(m.team_id, '')") ||
			strings.Contains(source, "coalesce(mp.team_id, '')") ||
			strings.Contains(source, "coalesce(team_id, '')") ||
			strings.Contains(source, "mp.team_id = m.team_id") {
			t.Fatalf("%s contains an untyped or cross-type gd_team_id expression", name)
		}
	}
}

func TestTextTeamExpressionsAreCastBeforeEnumWrites(t *testing.T) {
	checks := map[string][]string{
		"parties.go": {
			"then 'a'::gd_team_id",
			"else 'b'::gd_team_id",
		},
		"chat.go": {
			"nullif($10, '')::gd_team_id",
		},
	}
	for name, expectedExpressions := range checks {
		body, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		source := string(body)
		for _, expected := range expectedExpressions {
			if !strings.Contains(source, expected) {
				t.Fatalf("%s is missing typed team expression %q", name, expected)
			}
		}
	}
}

func TestComputedPartyRolesUseEnumValues(t *testing.T) {
	body, err := os.ReadFile("parties.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(body)
	for _, expected := range []string{
		"then 'owner'::gd_party_role",
		"else 'member'::gd_party_role",
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("computed party role is missing enum expression %q", expected)
		}
	}
}

func TestEnteringTeamDuelShufflesMembersIntoBalancedTeams(t *testing.T) {
	body, err := os.ReadFile("parties.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(body)
	for _, expected := range []string{
		"currentMode != string(contracts.ModeTeamDuel) && mode == contracts.ModeTeamDuel",
		"row_number() over (order by random())",
		"shuffled.position % 2 = 1",
		"'a'::gd_team_id",
		"'b'::gd_team_id",
	} {
		if !strings.Contains(source, expected) {
			t.Fatalf("team-duel shuffle is missing %q", expected)
		}
	}
}
