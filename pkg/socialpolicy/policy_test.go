package socialpolicy

import (
	"errors"
	"testing"
)

func TestAuthorizeUsesSharedSocialRules(t *testing.T) {
	tests := []struct {
		name string
		in   Account
		want error
	}{
		{name: "allowed", in: Account{ActionEnabled: true, TargetExists: true}},
		{name: "guest", in: Account{IsGuest: true, ActionEnabled: true, TargetExists: true}, want: ErrRegistrationRequired},
		{name: "blocked", in: Account{Blocked: true, ActionEnabled: true, TargetExists: true}, want: ErrUnavailable},
		{name: "disabled", in: Account{TargetExists: true}, want: ErrUnavailable},
		{name: "limit", in: Account{ActionEnabled: true, TargetExists: true, AtLimit: true}, want: ErrLimitReached},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := Authorize(tt.in); !errors.Is(err, tt.want) {
				t.Fatalf("Authorize() error = %v, want %v", err, tt.want)
			}
		})
	}
}
