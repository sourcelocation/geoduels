package social

import (
	"context"
	"errors"
	"slices"
	"testing"
	"time"
)

type memoryStore struct {
	Store
	account                   SocialAccount
	allowed                   bool
	friendCount               int
	crossedID                 string
	requests                  []FriendRequest
	accepted                  []string
	blocked, friends, pending bool
	removeErr, commitErr      error
	invitation                PartyInvitation
	invitationExists          bool
	invitationWrites          int
}

func (m *memoryStore) WithinTx(_ context.Context, fn func(Store) error) error {
	tx := *m
	tx.requests = slices.Clone(m.requests)
	tx.accepted = slices.Clone(m.accepted)
	if err := fn(&tx); err != nil {
		return err
	}
	if m.commitErr != nil {
		return m.commitErr
	}
	*m = tx
	return nil
}
func (m *memoryStore) GetSocialAccount(context.Context, string) (SocialAccount, error) {
	return m.account, nil
}
func (m *memoryStore) SendAllowed(context.Context, string, string) (bool, error) {
	return m.allowed, nil
}
func (m *memoryStore) CountFriends(context.Context, string) (int, error) { return m.friendCount, nil }
func (m *memoryStore) CrossedRequest(context.Context, string, string) (string, bool, error) {
	return m.crossedID, m.crossedID != "", nil
}
func (m *memoryStore) AcceptFriendRequest(_ context.Context, id, _ string) error {
	m.accepted = append(m.accepted, id)
	m.friends = true
	return nil
}
func (m *memoryStore) InsertFriendRequest(_ context.Context, _, _ string, expires time.Time) (FriendRequest, error) {
	request := FriendRequest{ID: "request", ExpiresAt: expires}
	m.requests = append(m.requests, request)
	return request, nil
}
func (m *memoryStore) TellFriendshipAccepted(context.Context, string, string, string) error {
	return nil
}
func (m *memoryStore) AddUserBlock(context.Context, string, string) error {
	m.blocked = true
	return nil
}
func (m *memoryStore) RemoveUserBlock(context.Context, string, string) error {
	m.blocked = false
	return nil
}
func (m *memoryStore) RemoveFriend(context.Context, string, string) error {
	if m.removeErr != nil {
		return m.removeErr
	}
	m.friends = false
	return nil
}
func (m *memoryStore) CancelPairFriendRequests(context.Context, string, string) error {
	m.pending = false
	return nil
}
func (m *memoryStore) InvitationEligibility(context.Context, string, string, string) (PartyInvitation, error) {
	return PartyInvitation{InviteCode: "code", Mode: "duel", MemberCount: 2}, nil
}
func (m *memoryStore) PendingPartyInvitation(context.Context, string, string) (PartyInvitation, bool, error) {
	return m.invitation, m.invitationExists, nil
}
func (m *memoryStore) UpsertPartyInvitation(_ context.Context, party, _, _ string, expires, created time.Time) (PartyInvitation, error) {
	m.invitation = PartyInvitation{ID: "invitation", PartyID: party, ExpiresAt: expires, CreatedAt: created}
	m.invitationExists = true
	m.invitationWrites++
	return m.invitation, nil
}
func socialStore() *memoryStore {
	return &memoryStore{account: SocialAccount{RequestsEnabled: true}, allowed: true}
}

func TestFriendRequestEligibilityAndExpiry(t *testing.T) {
	now := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	for _, tc := range []struct {
		name                           string
		guest, disabled, blocked, full bool
		want                           error
	}{
		{name: "allowed"}, {name: "guest", guest: true, want: ErrRegistrationRequired}, {name: "disabled", disabled: true, want: ErrBlocked}, {name: "blocked", blocked: true, want: ErrBlocked}, {name: "limit", full: true, want: ErrLimit},
	} {
		t.Run(tc.name, func(t *testing.T) {
			store := socialStore()
			store.account.IsGuest = tc.guest
			store.account.RequestsEnabled = !tc.disabled
			store.allowed = !tc.blocked
			if tc.full {
				store.friendCount = FriendLimit
			}
			svc := NewService(store)
			svc.clock = func() time.Time { return now }
			request, err := svc.SendFriendRequest(context.Background(), "sender", "recipient")
			if !errors.Is(err, tc.want) {
				t.Fatalf("expected %v, got %v", tc.want, err)
			}
			if tc.want != nil {
				if len(store.requests) != 0 {
					t.Fatal("rejected request persisted")
				}
				return
			}
			if !request.ExpiresAt.Equal(now.Add(FriendRequestTTL)) || len(store.requests) != 1 {
				t.Fatalf("incomplete request: %+v", request)
			}
		})
	}
}

func TestCrossedRequestBecomesFriendship(t *testing.T) {
	store := socialStore()
	store.crossedID = "incoming"
	request, err := NewService(store).SendFriendRequest(context.Background(), "sender", "recipient")
	if err != nil {
		t.Fatal(err)
	}
	if request.ID != "incoming" || request.Direction != "incoming" || !store.friends || !slices.Equal(store.accepted, []string{"incoming"}) || len(store.requests) != 0 {
		t.Fatal("crossed request was not accepted")
	}
}

func TestBlockCascadeAndFailureAtomicity(t *testing.T) {
	for _, fail := range []bool{false, true} {
		store := socialStore()
		store.friends = true
		store.pending = true
		if fail {
			store.removeErr = errors.New("remove failed")
		}
		err := NewService(store).SetUserBlock(context.Background(), "a", "b", true)
		if fail {
			if !errors.Is(err, store.removeErr) || store.blocked || !store.friends || !store.pending {
				t.Fatal("failed block leaked changes")
			}
		} else if err != nil || !store.blocked || store.friends || store.pending {
			t.Fatalf("incomplete block: %v", err)
		}
	}
}

func TestInvitationResendWindow(t *testing.T) {
	now := time.Date(2026, 9, 28, 12, 0, 0, 0, time.UTC)
	store := socialStore()
	svc := NewService(store)
	svc.clock = func() time.Time { return now }
	first, err := svc.CreatePartyInvitation(context.Background(), "party", "a", "b", 0)
	if err != nil {
		t.Fatal(err)
	}
	if !first.ExpiresAt.Equal(now.Add(DefaultPartyInviteTTL)) {
		t.Fatal("wrong default expiry")
	}
	now = now.Add(PartyInviteResendAfter - time.Nanosecond)
	repeat, err := svc.CreatePartyInvitation(context.Background(), "party", "a", "b", 0)
	if err != nil {
		t.Fatal(err)
	}
	if !repeat.CreatedAt.Equal(first.CreatedAt) || store.invitationWrites != 1 {
		t.Fatal("resend inside cooldown created a new invitation")
	}
	now = now.Add(time.Nanosecond)
	refreshed, err := svc.CreatePartyInvitation(context.Background(), "party", "a", "b", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if !refreshed.CreatedAt.Equal(now) || !refreshed.ExpiresAt.Equal(now.Add(time.Minute)) || store.invitationWrites != 2 {
		t.Fatal("invitation was not refreshed at cooldown boundary")
	}
}

func TestInvitationCommitFailureReturnsNoResult(t *testing.T) {
	store := socialStore()
	store.commitErr = errors.New("commit failed")
	result, err := NewService(store).CreatePartyInvitation(context.Background(), "party", "a", "b", 0)
	if !errors.Is(err, store.commitErr) || result.ID != "" || store.invitationExists {
		t.Fatalf("result=%+v err=%v", result, err)
	}
}
