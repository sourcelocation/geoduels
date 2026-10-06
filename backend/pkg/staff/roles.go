// Package staff defines the shared staff vocabulary and explicit permissions.
package staff

import "errors"

type Role string

const (
	Admin     Role = "admin"
	Judge     Role = "judge"
	Moderator Role = "moderator"
	Lanista   Role = "lanista"
)

var ErrForbidden = errors.New("staff permission required")
var ErrInvalidRole = errors.New("unsupported staff role")

func Parse(value string) (Role, error) {
	r := Role(value)
	switch r {
	case Admin, Judge, Moderator, Lanista:
		return r, nil
	}
	return "", ErrInvalidRole
}

type Roles []Role

func (r Roles) Has(role Role) bool {
	for _, v := range r {
		if v == role {
			return true
		}
	}
	return false
}
func FromStrings(values []string) Roles {
	out := make(Roles, 0, len(values))
	for _, v := range values {
		if r, e := Parse(v); e == nil {
			out = append(out, r)
		}
	}
	return out
}

// Capability is a coarse permission a staff action requires. Roles map to
// capabilities in one place so new roles never require touching transports or
// use cases.
type Capability uint64

const (
	CapManageAccess Capability = 1 << iota
	CapManageConfig
	CapManageContent
	CapManageMaps
	CapReviewReports
	CapEnforceBans
	CapCurate
	CapManageEvents
)

// roleCapabilities is the single source of truth for role/permission mapping.
// Roles stay independent: admin grants administrative capabilities only, judge
// grants review/enforcement, moderator grant curation, lanista grants events.
var roleCapabilities = map[Role]Capability{
	Admin:     CapManageAccess | CapManageConfig | CapManageContent | CapManageMaps,
	Judge:     CapReviewReports | CapEnforceBans,
	Moderator: CapCurate,
	Lanista:   CapManageEvents,
}

// Capabilities returns the union of capabilities granted by these roles.
func (r Roles) Capabilities() Capability {
	var out Capability
	for _, role := range r {
		out |= roleCapabilities[role]
	}
	return out
}

type Actor struct {
	ID     string
	Roles  Roles
	Banned bool
}

// SystemActor represents trusted internal work with no interactive identity.
func SystemActor() Actor { return Actor{ID: "", Roles: Roles{}} }

func (a Actor) Can(capability Capability) bool {
	return a.ID != "" && !a.Banned && a.Roles.Capabilities()&capability != 0
}

func (a Actor) RequireCap(capability Capability) error {
	if !a.Can(capability) {
		return ErrForbidden
	}
	return nil
}

// RequireAny allows the actor when it holds at least one of capabilities.
func (a Actor) RequireAny(capabilities ...Capability) error {
	for _, capability := range capabilities {
		if a.Can(capability) {
			return nil
		}
	}
	return ErrForbidden
}

func (a Actor) Require(role Role) error {
	if a.ID == "" || a.Banned || !a.Roles.Has(role) {
		return ErrForbidden
	}
	return nil
}
