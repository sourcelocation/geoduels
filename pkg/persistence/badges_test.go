package persistence

import "testing"

func TestBadgeFromPartsUsesStableLevelAndExtraState(t *testing.T) {
	legacy := badgeFromParts(badgeCodeLegacyTopFinish, 1, 7, true)
	if legacy.ID != "geoduels-v1-top-finish" || legacy.Kind != "legacy_top_finish" || legacy.Extra != 7 {
		t.Fatalf("legacy badge = %+v", legacy)
	}
	for count, wantLevel := range map[int16]int16{1: 1, 2: 2, 3: 3, 4: 3, 99: 3} {
		got := topFinishLevel(count)
		if got != wantLevel {
			t.Fatalf("topFinishLevel(%d) = %d, want %d", count, got, wantLevel)
		}
		top := badgeFromParts(badgeCodeTopFinish, got, count, true)
		if top.ID != "top-finish" || top.Level != int(wantLevel) || top.MaxLevel != 3 || top.Extra != int(count) {
			t.Fatalf("top finish badge for count %d = %+v", count, top)
		}
		wantImage := map[int16]string{
			1: "/badges/top-finish-1-badge.v1.png",
			2: "/badges/top-finish-2-badge.v1.png",
			3: "/badges/top-finish-3-badge.v1.png",
		}[wantLevel]
		if top.ImageURL != wantImage {
			t.Fatalf("top finish image for count %d = %q, want %q", count, top.ImageURL, wantImage)
		}
	}
}

func TestBadgeRefRejectsRetiredLegacyAward(t *testing.T) {
	if code, ok := badgeRefFromID("geoduels-v1-top-finish"); !ok || code != badgeCodeLegacyTopFinish {
		t.Fatalf("legacy badge ref = %d, %v", code, ok)
	}
	if code, ok := badgeRefFromID("top-finish"); !ok || code != badgeCodeTopFinish {
		t.Fatalf("top-finish badge ref = %d, %v", code, ok)
	}
}

func TestAdminBadgeCatalogExcludesSystemDerivedBadges(t *testing.T) {
	var store DB
	badges := store.ListAdminGrantableBadges()
	if len(badges) == 0 {
		t.Fatal("expected at least one manually grantable badge")
	}
	seen := map[string]bool{}
	for _, badge := range badges {
		seen[badge.ID] = true
	}
	for _, excluded := range []string{"geoduels-team", "discord-server-member", "elo-1000", "elo-1500", "elo-2000", "geoduels-v1-top-finish", "top-finish"} {
		if seen[excluded] {
			t.Fatalf("system-derived badge %q should not be grantable", excluded)
		}
	}
	if !seen["event-winner-2026"] {
		t.Fatal("event winner badge should be grantable")
	}
	for _, expected := range []string{"event-winner-2026", "speedrunner", "supporter"} {
		if !seen[expected] {
			t.Fatalf("manual award badge %q should be grantable", expected)
		}
	}
	for i := 1; i < len(badges); i++ {
		if badges[i-1].ID >= badges[i].ID {
			t.Fatalf("badge catalog is not stably ordered: %q before %q", badges[i-1].ID, badges[i].ID)
		}
	}
}
