package staff

import (
	"context"
	"errors"
	"maps"
	"slices"
	"testing"

	"geoduels/internal/audit"
	pkgstaff "geoduels/pkg/staff"
)

// Unused Store methods deliberately panic via the embedded nil interface.
// Tests supply state for the persistence operations exercised by real services.
type fakeStore struct {
	Store
	calls, locks        int
	commitErr, badgeErr error
	roles               map[string][]string
	audit               []audit.Entry
	awarded, removed    []string
}

func newFakeStore() *fakeStore {
	return &fakeStore{roles: map[string][]string{"u1": {}}}
}

func (f *fakeStore) WithinTx(_ context.Context, fn func(Store) error) error {
	f.calls++
	tx := *f
	tx.roles = maps.Clone(f.roles)
	for id, roles := range tx.roles {
		tx.roles[id] = slices.Clone(roles)
	}
	tx.audit = slices.Clone(f.audit)
	tx.awarded, tx.removed = slices.Clone(f.awarded), slices.Clone(f.removed)
	if err := fn(&tx); err != nil {
		return err
	}
	if f.commitErr != nil {
		return f.commitErr
	}
	*f = tx
	return nil
}

func (f *fakeStore) LockUser(_ context.Context, id string) error {
	f.locks++
	if _, ok := f.roles[id]; !ok {
		return ErrNotFound
	}
	return nil
}
func (f *fakeStore) UserRoles(_ context.Context, id string) ([]string, error) {
	return slices.Clone(f.roles[id]), nil
}
func (f *fakeStore) GrantRole(_ context.Context, id, role, _, _ string) error {
	f.roles[id] = append(f.roles[id], role)
	return nil
}
func (f *fakeStore) RevokeRole(_ context.Context, id, role string) error {
	f.roles[id] = slices.DeleteFunc(f.roles[id], func(r string) bool { return r == role })
	return nil
}
func (f *fakeStore) RecordAudit(_ context.Context, entry audit.Entry) (int64, error) {
	f.audit = append(f.audit, entry)
	return int64(len(f.audit)), nil
}
func (f *fakeStore) AwardTeamBadge(_ context.Context, id string) error {
	if f.badgeErr != nil {
		return f.badgeErr
	}
	f.awarded = append(f.awarded, id)
	return nil
}
func (f *fakeStore) RemoveTeamBadge(_ context.Context, id string) error {
	f.removed = append(f.removed, id)
	return nil
}

func adminActor() pkgstaff.Actor {
	return pkgstaff.Actor{ID: "admin-1", Roles: pkgstaff.Roles{pkgstaff.Admin}}
}
func judgeActor() pkgstaff.Actor {
	return pkgstaff.Actor{ID: "judge-1", Roles: pkgstaff.Roles{pkgstaff.Judge}}
}
func modActor() pkgstaff.Actor {
	return pkgstaff.Actor{ID: "mod-1", Roles: pkgstaff.Roles{pkgstaff.Moderator}}
}

func TestRoleCapabilitiesAreIndependent(t *testing.T) {
	if !adminActor().Can(pkgstaff.CapManageAccess) {
		t.Fatal("admin must manage access")
	}
	if adminActor().Can(pkgstaff.CapReviewReports) {
		t.Fatal("admin must not review reports without the judge role")
	}
	if judgeActor().Can(pkgstaff.CapManageAccess) {
		t.Fatal("judge must not manage access")
	}
	if !judgeActor().Can(pkgstaff.CapReviewReports) {
		t.Fatal("judge must review reports")
	}
	if !modActor().Can(pkgstaff.CapCurate) || modActor().Can(pkgstaff.CapReviewReports) {
		t.Fatal("moderator capabilities misconfigured")
	}
}

func TestUnauthorizedCommandHasNoSideEffects(t *testing.T) {
	f := newFakeStore()
	err := NewService(f).SetRole(context.Background(), judgeActor(), "u1", "admin", "escalate", true)
	if !errors.Is(err, pkgstaff.ErrForbidden) {
		t.Fatalf("expected forbidden, got %v", err)
	}
	if f.calls != 0 {
		t.Fatal("unauthorized command must not open a transaction")
	}
	if f.locks != 0 {
		t.Fatal("unauthorized command must not touch persistence")
	}
}

func TestSetRoleAuditsAndSyncsBadge(t *testing.T) {
	f := newFakeStore()
	svc := NewService(f)
	if err := svc.SetRole(context.Background(), adminActor(), "u1", "judge", "trusted", true); err != nil {
		t.Fatalf("grant failed: %v", err)
	}
	if len(f.audit) != 1 || f.audit[0].Action != audit.ActionRoleGranted {
		t.Fatalf("expected one role_granted audit entry, got %+v", f.audit)
	}
	if len(f.awarded) != 1 {
		t.Fatalf("expected team badge award, got %+v", f.awarded)
	}
	if err := svc.SetRole(context.Background(), adminActor(), "u1", "judge", "again", true); err != nil {
		t.Fatalf("re-grant failed: %v", err)
	}
	if len(f.audit) != 1 {
		t.Fatalf("idempotent grant must not audit, got %+v", f.audit)
	}
	if err := svc.SetRole(context.Background(), adminActor(), "u1", "judge", "", false); err != nil {
		t.Fatalf("revoke failed: %v", err)
	}
	if len(f.removed) != 1 {
		t.Fatalf("expected team badge removal, got %+v", f.removed)
	}
}

func TestRevokeKeepsBadgeWhileRolesRemain(t *testing.T) {
	f := newFakeStore()
	svc := NewService(f)
	if err := svc.SetRole(context.Background(), adminActor(), "u1", "judge", "", true); err != nil {
		t.Fatalf("grant judge: %v", err)
	}
	if err := svc.SetRole(context.Background(), adminActor(), "u1", "admin", "", true); err != nil {
		t.Fatalf("grant admin: %v", err)
	}
	f.removed = nil
	if err := svc.SetRole(context.Background(), adminActor(), "u1", "judge", "", false); err != nil {
		t.Fatalf("revoke judge: %v", err)
	}
	if len(f.removed) != 0 {
		t.Fatalf("badge must remain while other staff roles exist, got %+v", f.removed)
	}
	if err := svc.SetRole(context.Background(), adminActor(), "u1", "admin", "", false); err != nil {
		t.Fatalf("revoke admin: %v", err)
	}
	if len(f.removed) != 1 {
		t.Fatalf("removing the last role must remove the badge, got %+v", f.removed)
	}
}

func TestRoleChangeRollsBackOnBadgeOrCommitFailure(t *testing.T) {
	for _, stage := range []string{"badge", "commit"} {
		t.Run(stage, func(t *testing.T) {
			f := newFakeStore()
			failure := errors.New(stage + " failure")
			if stage == "badge" {
				f.badgeErr = failure
			} else {
				f.commitErr = failure
			}
			err := NewService(f).SetRole(context.Background(), adminActor(), "u1", "judge", "trusted", true)
			if !errors.Is(err, failure) {
				t.Fatalf("expected failure, got %v", err)
			}
			if len(f.roles["u1"]) != 0 || len(f.audit) != 0 || len(f.awarded) != 0 {
				t.Fatal("failed role grant leaked changes")
			}
		})
	}
}
