package controlplane

import (
	"context"
	"errors"
	"testing"
	"time"
)

type memoryLeases struct{ held map[string]Lease }

func (m *memoryLeases) Acquire(_ context.Context, name, owner string, _ time.Duration) (Lease, bool, error) {
	if current, ok := m.held[name]; ok && current.Owner != owner {
		return Lease{Name: name, Owner: owner}, false, nil
	}
	lease := Lease{Name: name, Owner: owner, Token: 1}
	m.held[name] = lease
	return lease, true, nil
}
func (m *memoryLeases) Renew(_ context.Context, l Lease, _ time.Duration) (bool, error) {
	current, ok := m.held[l.Name]
	return ok && current == l, nil
}
func (m *memoryLeases) Release(_ context.Context, l Lease) error { delete(m.held, l.Name); return nil }

func TestLeaseStoreConsumerContract(t *testing.T) {
	var store LeaseStore = &memoryLeases{held: map[string]Lease{}}
	first, ok, err := store.Acquire(context.Background(), "matchmaker", "a", time.Second)
	if err != nil || !ok {
		t.Fatalf("acquire: ok=%v err=%v", ok, err)
	}
	standby, ok, err := store.Acquire(context.Background(), "matchmaker", "b", time.Second)
	if err != nil || ok {
		t.Fatalf("second owner acquired: ok=%v err=%v", ok, err)
	}
	if standby.Name != "matchmaker" || standby.Owner != "b" || standby.Token != 0 {
		t.Fatalf("standby lease = %#v, want candidate identity without a token", standby)
	}
	if renewed, err := store.Renew(context.Background(), first, time.Second); err != nil || !renewed {
		t.Fatalf("renew: %v %v", renewed, err)
	}
	if err := store.Release(context.Background(), first); err != nil {
		t.Fatal(err)
	}
}

func TestLeaseNotHeldSentinel(t *testing.T) {
	if !errors.Is(ErrLeaseNotHeld, ErrLeaseNotHeld) {
		t.Fatal("sentinel must be comparable")
	}
}
