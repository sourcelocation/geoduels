// Package staff manages staff role grants. Other modules authorize their own
// staff operations with pkg/staff capabilities; this module only changes who
// holds which role.
package staff

import (
	"context"
	"errors"
	"slices"

	"geoduels/internal/audit"
	"geoduels/pkg/contracts"
	pkgstaff "geoduels/pkg/staff"
)

// ErrNotFound marks a missing subject.
var ErrNotFound = errors.New("subject not found")

type Service struct {
	store Store
}

func NewService(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) SetRole(ctx context.Context, actor pkgstaff.Actor, userID, role, reason string, grant bool) error {
	if err := actor.RequireCap(pkgstaff.CapManageAccess); err != nil {
		return err
	}
	parsed, err := pkgstaff.Parse(role)
	if err != nil {
		return err
	}
	return s.store.WithinTx(ctx, func(store Store) error {
		return mutateRole(ctx, store, actor, userID, string(parsed), reason, grant)
	})
}

// BootstrapAdmin grants admin unconditionally. The composition root's
// allowlist gates who may reach it.
func (s *Service) BootstrapAdmin(ctx context.Context, userID string) error {
	return s.store.WithinTx(ctx, func(store Store) error {
		return mutateRole(ctx, store, pkgstaff.Actor{ID: userID}, userID, string(pkgstaff.Admin), "Configured admin bootstrap", true)
	})
}

func (s *Service) ListRoleGrants(ctx context.Context, actor pkgstaff.Actor) ([]contracts.UserRoleGrant, error) {
	if err := actor.RequireCap(pkgstaff.CapManageAccess); err != nil {
		return nil, err
	}
	return s.store.ListRoleGrants(ctx)
}

func mutateRole(ctx context.Context, store Store, actor pkgstaff.Actor, userID, role, reason string, grant bool) error {
	if err := store.LockUser(ctx, userID); err != nil {
		return err
	}
	current, err := store.UserRoles(ctx, userID)
	if err != nil {
		return err
	}
	if slices.Contains(current, role) == grant {
		return nil
	}
	action := audit.ActionRoleGranted
	if grant {
		if err := store.GrantRole(ctx, userID, role, actor.ID, reason); err != nil {
			return err
		}
	} else {
		action = audit.ActionRoleRevoked
		if err := store.RevokeRole(ctx, userID, role); err != nil {
			return err
		}
	}
	if _, err := store.RecordAudit(ctx, audit.Entry{ActorID: actor.ID, SubjectID: userID, Action: action, Reason: reason, Metadata: map[string]any{"role": role}}); err != nil {
		return err
	}
	next, err := store.UserRoles(ctx, userID)
	if err != nil {
		return err
	}
	if len(next) > 0 {
		return store.AwardTeamBadge(ctx, userID)
	}
	return store.RemoveTeamBadge(ctx, userID)
}
