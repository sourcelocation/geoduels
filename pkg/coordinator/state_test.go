package coordinator

import (
	"context"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

func TestUnlockMatchRequiresOwnership(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	store := NewStore(rdb, 10*time.Second, 2*time.Hour, 24*time.Hour, 5*time.Second)
	ctx := context.Background()

	locked, err := store.TryLockMatch(ctx, "m-1", "holder-a")
	if err != nil {
		t.Fatalf("try lock: %v", err)
	}
	if !locked {
		t.Fatal("expected lock to be acquired")
	}
	if err := store.UnlockMatch(ctx, "m-1", "holder-b"); err != nil {
		t.Fatalf("unlock with wrong owner: %v", err)
	}

	val, err := rdb.Get(ctx, lockKey("m-1")).Result()
	if err != nil {
		t.Fatalf("get lock after wrong unlock: %v", err)
	}
	if val != "holder-a" {
		t.Fatalf("lock holder = %q", val)
	}

	if err := store.UnlockMatch(ctx, "m-1", "holder-a"); err != nil {
		t.Fatalf("unlock with owner: %v", err)
	}
	if _, err := rdb.Get(ctx, lockKey("m-1")).Result(); err == nil {
		t.Fatal("expected lock to be removed")
	}
}

func TestGetNodeByRouteUsesDirectRouteRecord(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	store := NewStore(rdb, 10*time.Second, 2*time.Hour, 24*time.Hour, 5*time.Second)
	ctx := context.Background()

	if err := store.RegisterNode(ctx, NodeRecord{
		NodeID:      "node-1",
		PublicRoute: "game-1",
		InternalURL: "http://game-1:8081",
	}); err != nil {
		t.Fatalf("register node: %v", err)
	}

	rec, ok, err := store.GetNodeByRoute(ctx, "game-1")
	if err != nil {
		t.Fatalf("get node by route: %v", err)
	}
	if !ok {
		t.Fatal("expected node to be found")
	}
	if rec.NodeID != "node-1" || rec.PublicRoute != "game-1" {
		t.Fatalf("unexpected node record: %+v", rec)
	}
}

func TestPresentUsersUsesCoordinatorPresenceSet(t *testing.T) {
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })
	store := NewStore(rdb, 10*time.Second, 2*time.Hour, 24*time.Hour, 5*time.Second)
	ctx := context.Background()
	if err := store.TouchPresence(ctx, "online-user"); err != nil {
		t.Fatalf("touch: %v", err)
	}
	present, err := store.PresentUsers(ctx, []string{"online-user", "offline-user"})
	if err != nil {
		t.Fatalf("present users: %v", err)
	}
	if !present["online-user"] {
		t.Fatal("expected online-user to be present")
	}
	if present["offline-user"] {
		t.Fatal("did not expect offline-user to be present")
	}
}
