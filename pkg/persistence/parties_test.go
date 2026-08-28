package persistence

import (
	"os"
	"strings"
	"testing"
)

func TestPartyMapAccessIncludesPublishedCommunityMaps(t *testing.T) {
	body, err := os.ReadFile("../../db/queries/parties.sql")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "published_at is not null") {
		t.Fatal("party map access must include published community maps")
	}
}

func TestCommunityMapListUsesCurrentPublicVisibility(t *testing.T) {
	if !strings.Contains(communityMapListPredicate, "visibility='public'") {
		t.Fatal("community map listing must require current public visibility")
	}
	if strings.Contains(communityMapListPredicate, "published_at is not null") {
		t.Fatal("community map listing must not rely on historical published state")
	}
}

func TestMapVisibleToUserHonorsCurrentVisibility(t *testing.T) {
	tests := []struct {
		name       string
		owner      string
		accessUser string
		visibility string
		official   bool
		want       bool
	}{
		{name: "official map", owner: "owner", accessUser: "player", visibility: "private", official: true, want: true},
		{name: "owner private map", owner: "owner", accessUser: "owner", visibility: "private", want: true},
		{name: "public map", owner: "owner", accessUser: "player", visibility: "public", want: true},
		{name: "unlisted map", owner: "owner", accessUser: "player", visibility: "unlisted", want: true},
		{name: "private map", owner: "owner", accessUser: "player", visibility: "private", want: false},
	}
	for _, tt := range tests {
		if got := mapVisibleToUser(tt.owner, tt.accessUser, tt.visibility, tt.official); got != tt.want {
			t.Fatalf("%s: mapVisibleToUser(%q, %q, %q, %v) = %v, want %v", tt.name, tt.owner, tt.accessUser, tt.visibility, tt.official, got, tt.want)
		}
	}
}

func TestPartyReadQueryCastsNullableMapID(t *testing.T) {
	body, err := os.ReadFile("../../db/queries/parties.sql")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "COALESCE(l.map_id::text, '')") {
		t.Fatal("party read query must cast nullable uuid map_id before coalescing with text")
	}
}
