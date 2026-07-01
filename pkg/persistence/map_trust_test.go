package persistence

import "testing"

func TestAutomaticMapCreatorTier(t *testing.T) {
	tests := []struct {
		name       string
		age        int
		favorites  int
		maps       int
		restricted bool
		want       int
	}{
		{name: "new creator stays base", age: 13, favorites: 100, maps: 5, want: mapCreatorTierBase},
		{name: "trusted threshold", age: 14, favorites: 25, maps: 1, want: mapCreatorTierTrusted},
		{name: "established needs two maps", age: 30, favorites: 100, maps: 1, want: mapCreatorTierTrusted},
		{name: "established threshold", age: 30, favorites: 100, maps: 2, want: mapCreatorTierEstablished},
		{name: "moderation forces base", age: 90, favorites: 500, maps: 20, restricted: true, want: mapCreatorTierBase},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := automaticMapCreatorTier(test.age, test.favorites, test.maps, test.restricted); got != test.want {
				t.Fatalf("tier = %d, want %d", got, test.want)
			}
		})
	}
}

func TestMapCreatorLimits(t *testing.T) {
	base := limitsForMapCreatorTier(mapCreatorTierBase)
	if base.maxMaps != 10 || base.maxActiveLocations != 200_000 || base.maxUploadedLocationsHour != 300_000 {
		t.Fatalf("unexpected base limits: %+v", base)
	}
	established := limitsForMapCreatorTier(mapCreatorTierEstablished)
	if established.maxMaps != 100 || established.maxActiveLocations != 1_000_000 || established.maxUploadsPerDay != 30 {
		t.Fatalf("unexpected established limits: %+v", established)
	}
}
