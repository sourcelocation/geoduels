package socialpolicy

import "errors"

var (
	ErrRegistrationRequired = errors.New("registration_required")
	ErrUnavailable          = errors.New("social_action_unavailable")
	ErrLimitReached         = errors.New("social_limit_reached")
)

type Account struct {
	IsGuest       bool
	ActionEnabled bool
	Blocked       bool
	TargetExists  bool
	AtLimit       bool
	SameUser      bool
}

// Authorize is the common policy gate for relationship, code, invitation, and
// discovery actions. Persistence still rechecks relational invariants inside
// the mutation transaction to prevent races.
func Authorize(account Account) error {
	switch {
	case account.IsGuest:
		return ErrRegistrationRequired
	case account.SameUser, account.Blocked, !account.TargetExists, !account.ActionEnabled:
		return ErrUnavailable
	case account.AtLimit:
		return ErrLimitReached
	default:
		return nil
	}
}
