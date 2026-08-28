package main

import (
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

)

type guestCleanupTestStore struct {
	testRepositories
	calls   int
	deleted []int
	ttl     time.Duration
	limit   int
}

func (s *guestCleanupTestStore) DeleteGuestAccountsOlderThan(ttl time.Duration, limit int) (int, error) {
	s.calls++
	s.ttl = ttl
	s.limit = limit
	if len(s.deleted) == 0 {
		return 0, nil
	}
	deleted := s.deleted[0]
	s.deleted = s.deleted[1:]
	return deleted, nil
}

func TestCleanupGuestAccountsDeletesUntilBatchNotFull(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	defer rdb.Close()

	store := &guestCleanupTestStore{deleted: []int{1000, 25}}
	a := &api{
		accounts: store, sessions: store, profiles: store, preferenceStore: store, badges: store, leaderboardStore: store, matchStore: store, moderation: store, admin: store, content: store, seasons: store, gameplayMaps: store, runtimeStore: store, chatStore: store, parties: store, social: store,
		redis:                 rdb,
		guestAccountTTL:       24 * time.Hour,
		guestCleanupInterval:  time.Hour,
		guestCleanupBatchSize: 1000,
	}

	a.cleanupGuestAccounts()

	if store.calls != 2 {
		t.Fatalf("cleanup calls = %d, want 2", store.calls)
	}
	if store.ttl != 24*time.Hour || store.limit != 1000 {
		t.Fatalf("cleanup args ttl=%v limit=%d", store.ttl, store.limit)
	}
}
