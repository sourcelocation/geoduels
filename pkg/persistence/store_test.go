package persistence

import (
	"os"
	"strings"
	"testing"
)

func TestFinalizeMatchFinalRankedDeltaCastsParameters(t *testing.T) {
	body, err := os.ReadFile("matches_write.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(body)
	if strings.Contains(source, "final_ranked_delta = $3 - $2") {
		t.Fatal("final_ranked_delta must cast pgx parameters before subtraction")
	}
	if got := strings.Count(source, "final_ranked_delta = $3::integer - $2::integer"); got != 2 {
		t.Fatalf("typed final_ranked_delta expressions = %d, want 2", got)
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
	if got := strings.Count(source, "and h.mode = 'duel'"); got != 2 {
		t.Fatalf("profile history_stats duel filters = %d, want 2", got)
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
	if !strings.Contains(source, "upsertIncidentForSignal") {
		t.Fatal("risk-engine ingestion must create or update v2 incidents")
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
