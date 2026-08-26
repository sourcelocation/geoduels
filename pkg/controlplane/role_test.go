package controlplane

import "testing"

func TestParseRole(t *testing.T) {
	for _, tc := range []struct {
		in    string
		want  Role
		valid bool
	}{{"", RoleServe, true}, {"serve", RoleServe, true}, {"MATCHMAKER", RoleMatchmaker, true}, {"jobs", RoleJobs, true}, {"all", "", false}} {
		got, err := ParseRole(tc.in)
		if (err == nil) != tc.valid || got != tc.want {
			t.Errorf("ParseRole(%q) = %q, %v", tc.in, got, err)
		}
	}
}
