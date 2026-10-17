package parties

import (
	"context"
	"errors"
	"maps"
	"sort"
	"testing"
	"time"

	"geoduels/pkg/matchkind"
)

type memoryStore struct {
	Store
	snapshot          Party
	expires           time.Time
	members           map[string]MemberRole
	touches, shuffles int
	commitErr         error
}

func (m *memoryStore) WithinTx(_ context.Context, fn func(Store) error) error {
	tx := *m
	tx.members = maps.Clone(m.members)
	if err := fn(&tx); err != nil {
		return err
	}
	if m.commitErr != nil {
		return m.commitErr
	}
	*m = tx
	return nil
}
func (m *memoryStore) GetPartyStateAndExpiry(context.Context, string) (PartyStateExpiry, error) {
	return PartyStateExpiry{State: m.snapshot.State, ExpiresAt: m.expires}, nil
}
func (m *memoryStore) GetPartyStateAndOwner(context.Context, string) (PartyStateOwner, error) {
	return PartyStateOwner{State: m.snapshot.State, OwnerUserID: m.snapshot.OwnerUserID}, nil
}
func (m *memoryStore) CountActivePartyMembers(_ context.Context, _, id string) (int, error) {
	n := len(m.members)
	if _, ok := m.members[id]; ok {
		n--
	}
	return n, nil
}
func (m *memoryStore) GetPartyByID(context.Context, string) (Party, bool, error) {
	return m.snapshot, true, nil
}
func (m *memoryStore) JoinPartyMember(_ context.Context, _, id string, role MemberRole) error {
	m.members[id] = role
	return nil
}
func (m *memoryStore) TouchPartyUpdated(context.Context, string) error { m.touches++; return nil }
func (m *memoryStore) LeavePartyMember(_ context.Context, _, id string) (int64, error) {
	if _, ok := m.members[id]; !ok {
		return 0, nil
	}
	delete(m.members, id)
	return 1, nil
}
func (m *memoryStore) NextPartyOwnerID(context.Context, string) (string, error) {
	ids := []string{}
	for id := range m.members {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	if len(ids) == 0 {
		return "", ErrNotFound
	}
	return ids[0], nil
}
func (m *memoryStore) CloseParty(context.Context, string) error {
	m.snapshot.State = StateClosed
	return nil
}
func (m *memoryStore) PartyMemberActive(_ context.Context, _, id string) (bool, error) {
	_, ok := m.members[id]
	return ok, nil
}
func (m *memoryStore) TransferPartyOwner(_ context.Context, _, id string) error {
	m.snapshot.OwnerUserID = id
	return nil
}
func (m *memoryStore) ReassignPartyRoles(_ context.Context, _, owner string) error {
	for id := range m.members {
		m.members[id] = MemberRoleMember
	}
	m.members[owner] = MemberRoleOwner
	return nil
}
func (m *memoryStore) LockOpenPartyMode(context.Context, string) (matchkind.Mode, error) {
	if m.snapshot.State != StateOpen {
		return "", ErrNotFound
	}
	return m.snapshot.Mode, nil
}
func (m *memoryStore) ShufflePartyTeams(context.Context, string) error { m.shuffles++; return nil }
func (m *memoryStore) SetPartyMode(_ context.Context, _ string, mode matchkind.Mode) error {
	m.snapshot.Mode = mode
	return nil
}

func newPartyStore(now time.Time) *memoryStore {
	return &memoryStore{snapshot: Party{ID: "party", OwnerUserID: "owner", State: StateOpen, Mode: matchkind.ModeDuel}, expires: now.Add(time.Hour), members: map[string]MemberRole{"owner": MemberRoleOwner}}
}

func TestJoinChecksExpiryCapacityAndOwnerRole(t *testing.T) {
	now := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name          string
		expired, full bool
		user          string
		allowed       bool
	}{
		{name: "member", user: "member", allowed: true}, {name: "owner rejoins", user: "owner", allowed: true}, {name: "expired", expired: true, user: "member"}, {name: "full", full: true, user: "member"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := newPartyStore(now)
			if tc.expired {
				store.expires = now.Add(-time.Second)
			}
			if tc.full {
				for i := 1; i < matchkind.MaxPartyMembers; i++ {
					store.members[string(rune('a'+i))] = MemberRoleMember
				}
			}
			original := len(store.members)
			svc := NewService(store)
			svc.clock = func() time.Time { return now }
			_, err := svc.JoinParty("party", tc.user)
			if (err == nil) != tc.allowed {
				t.Fatalf("unexpected error: %v", err)
			}
			if !tc.allowed {
				if len(store.members) != original || store.touches != 0 {
					t.Fatal("rejected join mutated party")
				}
				return
			}
			want := MemberRoleMember
			if tc.user == "owner" {
				want = MemberRoleOwner
			}
			if store.members[tc.user] != want {
				t.Fatal("incorrect member role")
			}
		})
	}
}

func TestLeavingOwnerTransfersOrClosesParty(t *testing.T) {
	for _, other := range []bool{false, true} {
		store := newPartyStore(time.Now())
		if other {
			store.members["next"] = MemberRoleMember
		}
		if _, err := NewService(store).LeaveParty("party", "owner"); err != nil {
			t.Fatal(err)
		}
		if _, exists := store.members["owner"]; exists {
			t.Fatal("owner still present")
		}
		if other {
			if store.snapshot.OwnerUserID != "next" || store.members["next"] != MemberRoleOwner {
				t.Fatal("ownership not transferred")
			}
		} else if store.snapshot.State != StateClosed {
			t.Fatal("empty party not closed")
		}
	}
}

func TestInProgressOwnerCannotLeave(t *testing.T) {
	store := newPartyStore(time.Now())
	store.snapshot.State = StateInMatch
	if _, err := NewService(store).LeaveParty("party", "owner"); err == nil {
		t.Fatal("in-progress owner left")
	}
	if len(store.members) != 1 || store.touches != 0 {
		t.Fatal("rejected leave mutated party")
	}
}

func TestJoinCommitFailureDoesNotPublishMembership(t *testing.T) {
	store := newPartyStore(time.Now())
	store.commitErr = errors.New("commit failed")
	snapshot, err := NewService(store).JoinParty("party", "member")
	if !errors.Is(err, store.commitErr) || snapshot.ID != "" {
		t.Fatalf("snapshot=%+v error=%v", snapshot, err)
	}
	if len(store.members) != 1 || store.touches != 0 {
		t.Fatal("failed join persisted")
	}
}

func TestTeamModeShufflesOnlyWhenEntering(t *testing.T) {
	store := newPartyStore(time.Now())
	svc := NewService(store)
	for range 2 {
		if err := svc.SetPartyMode("party", matchkind.ModeTeamDuel); err != nil {
			t.Fatal(err)
		}
	}
	if store.shuffles != 1 || store.snapshot.Mode != matchkind.ModeTeamDuel {
		t.Fatal("team mode transition repeated shuffle")
	}
}
