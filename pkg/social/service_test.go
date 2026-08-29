package social

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"testing"
)

type serviceStore struct {
	Store
	contexts []context.Context
	guest    bool
	err      error
	calls    []string
	mu       sync.Mutex
}

func (s *serviceStore) GetSocialAccount(ctx context.Context, _ string) (bool, bool, bool, error) {
	s.recordContext(ctx)
	return s.guest, true, true, s.err
}

func (s *serviceStore) recordContext(ctx context.Context) {
	s.mu.Lock()
	s.contexts = append(s.contexts, ctx)
	s.mu.Unlock()
}

func (s *serviceStore) record(v string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls = append(s.calls, v)
}
func (s *serviceStore) ListFriends(ctx context.Context, _ string, n int) ([]CompactPlayer, error) {
	s.recordContext(ctx)
	s.record("friends:" + itoa(n))
	return nil, s.err
}
func (s *serviceStore) ListFriendRequests(ctx context.Context, _ string, d string, n int) ([]FriendRequest, error) {
	s.recordContext(ctx)
	s.record(d + ":" + itoa(n))
	return nil, s.err
}
func (s *serviceStore) ListRecentPlayers(ctx context.Context, _ string, n int) ([]CompactPlayer, error) {
	s.recordContext(ctx)
	s.record("recent:" + itoa(n))
	return nil, s.err
}
func (s *serviceStore) ListPartyInviteStatus(ctx context.Context, _, _ string) (map[string]CompactPartyInvite, error) {
	s.recordContext(ctx)
	s.record("party")
	return map[string]CompactPartyInvite{}, s.err
}
func itoa(n int) string { return strconv.Itoa(n) }
func (s *serviceStore) snapshot() ([]context.Context, []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]context.Context(nil), s.contexts...), append([]string(nil), s.calls...)
}
func TestAuthorizeForwardsContextAndRejectsGuest(t *testing.T) {
	ctx := context.WithValue(context.Background(), struct{}{}, "x")
	st := &serviceStore{guest: true}
	err := NewService(st).Authorize(ctx, "u")
	if !errors.Is(err, ErrRegistrationRequired) {
		t.Fatalf("err=%v", err)
	}
	contexts, _ := st.snapshot()
	if len(contexts) != 1 || contexts[0] != ctx {
		t.Fatal("context was not forwarded")
	}
}
func TestFriendsPageAggregatesAndLoadsPartyStatus(t *testing.T) {
	ctx := context.Background()
	st := &serviceStore{}
	got, err := NewService(st).FriendsPage(ctx, "u", "p")
	if err != nil {
		t.Fatal(err)
	}
	if got.PartyInvites == nil {
		t.Fatal("party status missing")
	}
	contexts, calls := st.snapshot()
	if len(calls) != 4+1 {
		t.Fatalf("calls=%v", calls)
	}
	if len(contexts) != 5 {
		t.Fatalf("contexts=%d", len(contexts))
	}
	for _, gotCtx := range contexts {
		if gotCtx != ctx {
			t.Fatal("context was not forwarded exactly")
		}
	}
	seen := map[string]bool{}
	for _, c := range calls {
		seen[c] = true
	}
	for _, want := range []string{"friends:100", "incoming:20", "outgoing:20", "recent:3", "party"} {
		if !seen[want] {
			t.Errorf("missing call %q: %v", want, calls)
		}
	}
}
func TestFriendsPageCancellationVisibleToStore(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	st := &serviceStore{}
	_, _ = NewService(st).FriendsPage(ctx, "u", "")
	contexts, _ := st.snapshot()
	if len(contexts) != 4 || ctx.Err() == nil {
		t.Fatal("cancellation not forwarded")
	}
	for _, got := range contexts {
		if got != ctx {
			t.Fatal("wrong context forwarded")
		}
	}
}
func TestFriendsPageReturnsStoreError(t *testing.T) {
	want := errors.New("boom")
	st := &serviceStore{err: want}
	_, err := NewService(st).FriendsPage(context.Background(), "u", "")
	if !errors.Is(err, want) {
		t.Fatalf("err=%v", err)
	}
}
