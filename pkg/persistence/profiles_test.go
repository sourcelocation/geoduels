package persistence

import (
	"testing"

	"geoduels/pkg/contracts"
)

func TestOwnedPlayerBadgesExcludesLockedTemplates(t *testing.T) {
	badges := ownedPlayerBadges([]contracts.PlayerBadge{
		{ID: "earned", Owned: true},
		{ID: "locked", Owned: false},
	})
	if len(badges) != 1 || badges[0].ID != "earned" {
		t.Fatalf("owned badges = %+v", badges)
	}
}
