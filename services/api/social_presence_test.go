package main

import (
	"sync"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"geoduels/pkg/coordinator"
)

type lastSeenTestStore struct {
	testRepositories
	mu     sync.Mutex
	writes []time.Time
	wrote  chan struct{}
}

func (s *lastSeenTestStore) TouchLastSeen(_ string, seenAt time.Time) error {
	s.mu.Lock()
	s.writes = append(s.writes, seenAt)
	s.mu.Unlock()
	select {
	case s.wrote <- struct{}{}:
	default:
	}
	return nil
}

func (s *lastSeenTestStore) writeCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.writes)
}

func TestLastSeenWritesAreAsynchronousAndRedisThrottled(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	store := &lastSeenTestStore{wrote: make(chan struct{}, 2)}
	a := &api{lastSeen: store, redis: rdb}

	a.scheduleLastSeenWrite(t.Context(), "user-1", time.Now().UTC())
	select {
	case <-store.wrote:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for asynchronous last-seen write")
	}
	a.scheduleLastSeenWrite(t.Context(), "user-1", time.Now().UTC())
	time.Sleep(20 * time.Millisecond)
	if got := store.writeCount(); got != 1 {
		t.Fatalf("last-seen writes = %d, want 1", got)
	}
	if ttl := mr.TTL(lastSeenWriteKey("user-1")); ttl <= 0 {
		t.Fatalf("throttle TTL = %s, want positive", ttl)
	}
}

func TestUnchangedPresenceRenewsRedisStateTTL(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	store := &liveSocialStore{}
	a := &api{social: store, redis: rdb, coord: coordinator.NewStore(rdb, time.Minute, time.Hour, time.Hour, time.Second)}
	a.live = newLiveHub(a)

	state := presenceState{Status: "online", LastSeenAt: time.Now().UTC()}
	if err := a.live.writePresenceState(t.Context(), "user-1", state); err != nil {
		t.Fatal(err)
	}
	mr.FastForward(time.Minute)
	a.live.notePresence(t.Context(), "user-1")
	if ttl := mr.TTL(presenceStateKey("user-1")); ttl < 90*time.Second {
		t.Fatalf("presence state TTL = %s, want renewed TTL", ttl)
	}
}
