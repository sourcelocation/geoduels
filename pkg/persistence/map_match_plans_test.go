package persistence

import (
	"os"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5/pgtype"
)

func int4(v int32) pgtype.Int4 {
	return pgtype.Int4{Int32: v, Valid: true}
}

func TestBuildPlayRegionBounds(t *testing.T) {
	t.Run("disabled returns nil", func(t *testing.T) {
		if got := buildPlayRegionBounds(false, int4(1), int4(2), int4(3), int4(4)); got != nil {
			t.Fatalf("expected nil bounds when auto-zoom disabled, got %#v", got)
		}
	})

	t.Run("missing bound returns nil", func(t *testing.T) {
		if got := buildPlayRegionBounds(true, int4(1), pgtype.Int4{}, int4(3), int4(4)); got != nil {
			t.Fatalf("expected nil bounds when a bound is missing, got %#v", got)
		}
	})

	t.Run("converts e7 to degrees", func(t *testing.T) {
		got := buildPlayRegionBounds(true, int4(-123456789), int4(234567890), int4(-1_000_000_000), int4(1_800_000_000))
		if got == nil {
			t.Fatal("expected bounds, got nil")
		}
		if got.MinLat != -12.3456789 || got.MaxLat != 23.456789 {
			t.Fatalf("unexpected latitude bounds: %#v", got)
		}
		if got.MinLng != -100 || got.MaxLng != 180 {
			t.Fatalf("unexpected longitude bounds: %#v", got)
		}
	})
}

func TestPrepareMatchPlanAppliesPlayRegionConfig(t *testing.T) {
	body, err := os.ReadFile("map_match_plans.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(body)
	if got := strings.Count(source, "applyPlayRegionConfig(ctx, tx,"); got != 2 {
		t.Fatalf("applyPlayRegionConfig calls = %d, want 2 (reload and fresh plan paths)", got)
	}
}

func TestMapIngestComputesPlayRegionBounds(t *testing.T) {
	body, err := os.ReadFile("map_ingest.go")
	if err != nil {
		t.Fatal(err)
	}
	source := string(body)
	if got := strings.Count(source, "bounds_min_lat_e7=(select min(lat_e7)"); got != 2 {
		t.Fatalf("bounds computation on ingest = %d, want 2 (create/replace and official import)", got)
	}
	if !strings.Contains(source, "auto_zoom_play_region=$9") {
		t.Fatal("UpdateCustomMap must persist auto_zoom_play_region")
	}
}

func TestSelectedMapAccessible(t *testing.T) {
	tests := []struct {
		name       string
		owner      string
		accessUser string
		visibility string
		want       bool
	}{
		{name: "official map", visibility: "private", want: true},
		{name: "owner private map", owner: "owner", accessUser: "owner", visibility: "private", want: true},
		{name: "other user public map", owner: "owner", accessUser: "player", visibility: "public", want: true},
		{name: "other user unlisted map", owner: "owner", accessUser: "player", visibility: "unlisted", want: true},
		{name: "other user private map", owner: "owner", accessUser: "player", visibility: "private", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := selectedMapAccessible(tt.owner, tt.accessUser, tt.visibility); got != tt.want {
				t.Fatalf("selectedMapAccessible(%q, %q, %q) = %v, want %v", tt.owner, tt.accessUser, tt.visibility, got, tt.want)
			}
		})
	}
}
