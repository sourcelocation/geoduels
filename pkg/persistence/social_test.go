package persistence

import (
	"strings"
	"testing"
)

func TestRandomFriendCodeIsShortAndUnambiguous(t *testing.T) {
	for i := 0; i < 100; i++ {
		code, err := randomFriendCode()
		if err != nil {
			t.Fatal(err)
		}
		if len(code) != 6 {
			t.Fatalf("code length = %d, want 6", len(code))
		}
		for _, ambiguous := range "01ILO" {
			if strings.ContainsRune(code, ambiguous) {
				t.Fatalf("code %q contains ambiguous character %q", code, ambiguous)
			}
		}
	}
}
