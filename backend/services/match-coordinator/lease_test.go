package main

import (
	"context"
	"sync"
	"testing"
	"time"

	"geoduels/pkg/controlplane"
)

// fakeLeases is one lease, held by whoever took it last.
type fakeLeases struct {
	mu       sync.Mutex
	holder   string
	released int
}

func (f *fakeLeases) Acquire(_ context.Context, name, owner string, _ time.Duration) (controlplane.Lease, bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.holder != "" && f.holder != owner {
		return controlplane.Lease{}, false, nil
	}
	f.holder = owner
	return controlplane.Lease{Name: name, Owner: owner, Token: 1}, true, nil
}

func (f *fakeLeases) Renew(_ context.Context, l controlplane.Lease, _ time.Duration) (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.holder == l.Owner, nil
}

func (f *fakeLeases) Release(_ context.Context, l controlplane.Lease) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.holder == l.Owner {
		f.holder = ""
		f.released++
	}
	return nil
}

func TestDrainingHandsTheLeaseToTheOtherReplica(t *testing.T) {
	leases := &fakeLeases{}
	a := &matchCoordinator{leaseStore: leases, lease: controlplane.Lease{Owner: "a"}}
	b := &matchCoordinator{leaseStore: leases, lease: controlplane.Lease{Owner: "b"}}
	a.renewOrTakeLease(time.Second, time.Minute)
	b.renewOrTakeLease(time.Second, time.Minute)
	if !a.isMatchmakerOwner() || b.isMatchmakerOwner() {
		t.Fatal("a should hold the lease, and b wait for it")
	}

	a.draining.Store(true)
	a.handOverLease()
	if a.isMatchmakerOwner() || leases.released != 1 {
		t.Fatal("a draining replica should release the lease at once")
	}
	if a.renewOrTakeLease(time.Second, time.Minute) {
		t.Fatal("a draining replica must not take the lease back")
	}
	b.renewOrTakeLease(time.Second, time.Minute)
	if !b.isMatchmakerOwner() {
		t.Fatal("the other replica should take the lease on its next check")
	}
}

func TestHandOverWaitsForTheTickInProgress(t *testing.T) {
	leases := &fakeLeases{}
	q := &matchCoordinator{leaseStore: leases, lease: controlplane.Lease{Owner: "a"}}
	q.renewOrTakeLease(time.Second, time.Minute)

	q.matchmakerMu.Lock() // a tick in progress
	done := make(chan struct{})
	go func() {
		q.handOverLease()
		close(done)
	}()
	select {
	case <-done:
		t.Fatal("the lease was handed over in the middle of a tick")
	case <-time.After(50 * time.Millisecond):
	}
	q.matchmakerMu.Unlock()
	<-done
	if q.isMatchmakerOwner() {
		t.Fatal("the lease should be handed over once the tick ends")
	}
}
