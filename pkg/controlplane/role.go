package controlplane

import (
	"fmt"
	"strings"
)

// Role is selected at a composition root. Roles are deliberately separate
// from domain packages so serving HTTP never implicitly starts external work.
type Role string

const (
	RoleServe      Role = "serve"
	RoleMatchmaker Role = "matchmaker"
	RoleJobs       Role = "jobs"
)

func ParseRole(value string) (Role, error) {
	role := Role(strings.ToLower(strings.TrimSpace(value)))
	if role == "" {
		return RoleServe, nil
	}
	switch role {
	case RoleServe, RoleMatchmaker, RoleJobs:
		return role, nil
	default:
		return "", fmt.Errorf("invalid CONTROL_PLANE_ROLE %q (want serve, matchmaker, or jobs)", value)
	}
}
