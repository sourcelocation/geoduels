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
	if got := strings.Count(source, "computePlayRegionBoundsE7(parsed)"); got != 2 {
		t.Fatalf("bounds computation on ingest = %d, want 2 (create/replace and official import)", got)
	}
	if !strings.Contains(source, "auto_zoom_play_region=$9") {
		t.Fatal("UpdateCustomMap must persist auto_zoom_play_region")
	}
}

func TestWrappedLngBoundsE7(t *testing.T) {
	t.Run("single point", func(t *testing.T) {
		start, end := wrappedLngBoundsE7([]int32{500})
		if start != 500 || end != 500 {
			t.Fatalf("single point bounds = %d..%d, want 500..500", start, end)
		}
	})

	t.Run("non-crossing region keeps min<=max", func(t *testing.T) {
		start, end := wrappedLngBoundsE7([]int32{300_000_000, 400_000_000, 350_000_000})
		if start != 300_000_000 || end != 400_000_000 {
			t.Fatalf("non-crossing bounds = %d..%d, want 300000000..400000000", start, end)
		}
	})

	t.Run("antimeridian-crossing region wraps with start>end", func(t *testing.T) {
		// Longitudes 177,178,179 and -179,-178 (a narrow band across +/-180).
		start, end := wrappedLngBoundsE7([]int32{1_770_000_000, 1_790_000_000, -1_790_000_000, 1_780_000_000, -1_780_000_000})
		if start != 1_770_000_000 || end != -1_780_000_000 {
			t.Fatalf("crossing bounds = %d..%d, want 1770000000..-1780000000", start, end)
		}
		if start <= end {
			t.Fatal("crossing interval must have start > end")
		}
	})
}

func TestComputePlayRegionBoundsE7(t *testing.T) {
	rows := []mapRow{
		{LatE7: 100, LngE7: 300_000_000},
		{LatE7: -50, LngE7: 400_000_000},
		{LatE7: 250, LngE7: 350_000_000},
	}
	minLat, maxLat, minLng, maxLng := computePlayRegionBoundsE7(rows)
	if minLat != -50 || maxLat != 250 {
		t.Fatalf("latitude bounds = %d..%d, want -50..250", minLat, maxLat)
	}
	if minLng != 300_000_000 || maxLng != 400_000_000 {
		t.Fatalf("longitude bounds = %d..%d, want 300000000..400000000", minLng, maxLng)
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
