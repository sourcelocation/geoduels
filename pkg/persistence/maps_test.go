package persistence

import (
	"os"
	"strings"
	"testing"
)

func TestDecodeMapRowsRejectsDuplicatesAndInvalidLocations(t *testing.T) {
	rows, digest, rejected, err := decodeMapRows(strings.NewReader(`[
		{"lat":10,"lng":20,"panoId":"one"},
		{"lat":10,"lng":20,"panoId":"two"},
		{"lat":11,"lng":21,"panoId":"one"},
		{"lat":91,"lng":0},
		{"lat":12,"lng":22},
		{"lat":13,"lng":23},
		{"lat":14,"lng":24},
		{"lat":15,"lng":25}
	]`), absoluteMaxMapLocations)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 5 || rejected != 3 {
		t.Fatalf("rows=%d rejected=%d", len(rows), rejected)
	}
	if len(digest) != 64 {
		t.Fatalf("expected sha256 digest, got %q", digest)
	}
}

func TestDecodeMapRowsAcceptsMapMakingExport(t *testing.T) {
	rows, _, rejected, err := decodeMapRows(strings.NewReader(`{
		"name":"test",
		"customCoordinates":[
			{"lat":-0.15796175210211638,"lng":37.7503211982208,"heading":88.26937616435896,"pitch":-6.581217542806854,"zoom":0,"panoId":null,"countryCode":"KE","stateCode":null,"extra":{"panoId":"3P5a5OtPyfh9ByBzuHANrg","panoDate":"2022-02"}},
			{"lat":6.428595321124186,"lng":-1.4132091930831705,"heading":60.50836,"pitch":0,"zoom":0,"panoId":null,"countryCode":"GH","stateCode":null,"extra":{"panoId":"Jd2sr079nrj3lhQ9Ab_kEw","panoDate":"2016-04"}},
			{"lat":6.5462559963431595,"lng":0.25167556026892035,"heading":40.375347,"pitch":0,"zoom":0,"panoId":null,"countryCode":null,"stateCode":null,"extra":{"panoId":"b184OQ0GzootvmVPP6CuAg","panoDate":"2025-02"}},
			{"lat":6.2225031210101065,"lng":-1.383655971662562,"heading":268.6127,"pitch":0,"zoom":0,"panoId":null,"countryCode":null,"stateCode":null,"extra":{"panoId":"cQ5UcYcmtkqA3oVIvOR8jA","panoDate":"2016-04"}},
			{"lat":8.555462932718319,"lng":-2.2134015863200154,"heading":151.81796,"pitch":0,"zoom":0,"panoId":null,"countryCode":null,"stateCode":null,"extra":{"panoId":"exGCRe5MBhjvC1PBBUzWLg","panoDate":"2025-02"}}
		]
	}`), absoluteMaxMapLocations)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 5 || rejected != 0 {
		t.Fatalf("rows=%d rejected=%d", len(rows), rejected)
	}
	if rows[0].PanoID == nil || *rows[0].PanoID != "3P5a5OtPyfh9ByBzuHANrg" {
		t.Fatalf("nested pano id not imported: %#v", rows[0].PanoID)
	}
	if rows[0].Country != "KE" {
		t.Fatalf("country code not imported: %q", rows[0].Country)
	}
	if rows[0].Heading == nil || *rows[0].Heading != 88.26937616435896 {
		t.Fatalf("heading not imported: %#v", rows[0].Heading)
	}
	if rows[0].Pitch == nil || *rows[0].Pitch != -6.581217542806854 {
		t.Fatalf("pitch not imported: %#v", rows[0].Pitch)
	}
}

func TestParseMapRowsAcceptsMapMakingExport(t *testing.T) {
	rows, err := parseMapRows([]byte(`{
		"name":"test",
		"customCoordinates":[
			{"lat":10,"lng":20,"heading":88,"pitch":-6,"panoId":null,"countryCode":"KE","extra":{"panoId":"nested-pano"}}
		]
	}`))
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 {
		t.Fatalf("rows=%d", len(rows))
	}
	if rows[0].PanoID == nil || *rows[0].PanoID != "nested-pano" {
		t.Fatalf("nested pano id not imported: %#v", rows[0].PanoID)
	}
	if rows[0].Country != "KE" {
		t.Fatalf("country code not imported: %q", rows[0].Country)
	}
}

func TestDecodeMapRowsRequiresArray(t *testing.T) {
	if _, _, _, err := decodeMapRows(strings.NewReader(`{"lat":10,"lng":20}`), absoluteMaxMapLocations); err == nil {
		t.Fatal("expected non-array map to fail")
	}
}

func TestDecodeMapRowsEnforcesDynamicTierLimit(t *testing.T) {
	if _, _, _, err := decodeMapRows(strings.NewReader(`[
		{"lat":10,"lng":20},
		{"lat":11,"lng":21}
	]`), 1); err == nil || !strings.Contains(err.Error(), "map limit is 1 locations") {
		t.Fatalf("expected dynamic location limit error, got %v", err)
	}
}

func TestDeterministicPivotIsStable(t *testing.T) {
	a := deterministicPivot("match", "revision")
	b := deterministicPivot("match", "revision")
	if a != b || a < 0 || a > 16_777_215 {
		t.Fatalf("invalid pivot %v %v", a, b)
	}
}

func TestMapSearchPatternNormalizesAndEscapes(t *testing.T) {
	got := mapSearchPattern("  source   50%_\\  ")
	if got != `%source 50\%\_\\%` {
		t.Fatalf("unexpected search pattern %q", got)
	}
	if mapSearchPattern("   ") != "" {
		t.Fatal("blank search should not produce a pattern")
	}
}

func TestMapSearchPatternCapsLongTermsByRune(t *testing.T) {
	got := mapSearchPattern(strings.Repeat("ø", 81))
	if len([]rune(strings.Trim(got, "%"))) != 80 {
		t.Fatalf("expected 80 runes, got %d in %q", len([]rune(strings.Trim(got, "%"))), got)
	}
}

func TestMapCommentsQueryCastsUUIDsForStringScanning(t *testing.T) {
	body, err := os.ReadFile("../../db/queries/map_comments.sql")
	if err != nil {
		t.Fatal(err)
	}
	source := string(body)
	for _, expression := range []string{"c.id::text", "c.map_id::text", "c.user_id::text"} {
		if !strings.Contains(source, expression) {
			t.Fatalf("map comments query must cast %s before scanning into strings", expression)
		}
	}
	compact := strings.ReplaceAll(source, " ", "")
	if !strings.Contains(compact, "coalesce(c.user_id=nullif(sqlc.arg(viewer_user_id),'')::uuidorsqlc.arg(can_moderate)::boolean,false)") {
		t.Fatal("map comments query must normalize anonymous can-delete results to false")
	}
}

func TestArchiveCustomMapAdminOverrideIncludesOwnerlessMaps(t *testing.T) {
	body, err := os.ReadFile("../../db/queries/map_ingest.sql")
	if err != nil {
		t.Fatal(err)
	}
	source := string(body)
	if strings.Contains(source, "allow_any)::boolean and m.owner_user_id is not null") || strings.Contains(source, "allow_any)::boolean and owner_user_id is not null") {
		t.Fatal("admin map deletion override must include ownerless maps")
	}
	if got := strings.Count(source, "owner_user_id=sqlc.arg(user_id) or sqlc.arg(allow_any)::boolean"); got != 2 {
		t.Fatalf("admin map deletion predicates = %d, want 2", got)
	}
}
