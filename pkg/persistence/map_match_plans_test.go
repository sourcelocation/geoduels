package persistence

import "testing"

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
