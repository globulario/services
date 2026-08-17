package main

// blob_seed_runtime_test.go
//
// The CAS seeder ran exactly once, in a startup goroutine. The local CAS is
// per-node — a publish materializes the blob only in the CAS of the instance
// that handled the upload — while the manifest authority is cluster-wide. Every
// other node therefore kept advertising that manifest as PUBLISHED while being
// unable to serve it, and nothing re-ran the seeder to fix it.
//
// Observed 2026-08-16: cluster-controller@1.2.317's blob existed only on
// globule-ryzen and ai-memory@1.2.317's only on globule-dell, so the remaining
// nodes reported repository.identity.missing_blob_for_published_manifest as an
// ERROR — while the staged archives that would have healed them sat unused in
// /var/lib/globular/packages/ on those same nodes.
//
// This pins the property the fix adds: the pass REPEATS. Covered invariant:
// repository.published_missing_blob_with_staged_source_must_self_heal_during_runtime.
// Failure mode closed: repository.mid_life_cas_loss_persists_because_seeder_is_startup_only.

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestSeedLocalCAS_RunsImmediatelyThenRepeats(t *testing.T) {
	var passes atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	srv := &server{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		srv.seedLocalCASEvery(ctx, 5*time.Millisecond, func(context.Context) {
			passes.Add(1)
		})
	}()

	deadline := time.Now().Add(2 * time.Second)
	for passes.Load() < 3 && time.Now().Before(deadline) {
		time.Sleep(2 * time.Millisecond)
	}
	cancel()
	<-done

	if got := passes.Load(); got < 3 {
		t.Fatalf("seeding passes = %d, want >= 3 — the pass must repeat; running once at "+
			"startup is what left nodes advertising blobs they could not serve", got)
	}
}

// The first pass must not wait for the first tick: a node that boots with an
// empty CAS has to heal immediately, not one interval later.
func TestSeedLocalCAS_FirstPassIsImmediate(t *testing.T) {
	var passes atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan struct{})
	srv := &server{}
	go func() {
		defer close(done)
		// An interval far longer than the test: any pass observed can only be
		// the immediate one.
		srv.seedLocalCASEvery(ctx, time.Hour, func(context.Context) { passes.Add(1) })
	}()

	deadline := time.Now().Add(time.Second)
	for passes.Load() == 0 && time.Now().Before(deadline) {
		time.Sleep(2 * time.Millisecond)
	}
	cancel()
	<-done

	if passes.Load() != 1 {
		t.Fatalf("passes = %d, want exactly 1 — the startup pass must run before the first tick", passes.Load())
	}
}

func TestSeedLocalCAS_StopsOnContextCancel(t *testing.T) {
	var passes atomic.Int32
	ctx, cancel := context.WithCancel(context.Background())

	srv := &server{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		srv.seedLocalCASEvery(ctx, time.Millisecond, func(context.Context) { passes.Add(1) })
	}()

	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("seedLocalCASEvery did not return after context cancel — a seeder that " +
			"outlives shutdown keeps touching the CAS during teardown")
	}

	settled := passes.Load()
	time.Sleep(30 * time.Millisecond)
	if passes.Load() != settled {
		t.Errorf("passes kept increasing after cancel (%d -> %d)", settled, passes.Load())
	}
}
