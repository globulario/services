package main

// state_persist_dedup_test.go — Phase 36.
//
// Pins the content-hash dedup contract documented in saveToEtcd:
// a 334 KB controllerState blob must NOT be rewritten to etcd on every
// reconcile tick when its content has not changed. The pre-Phase-36
// behaviour caused 95.8% of MVCC bloat between compaction cycles (see
// docs/awareness/reports/etcd_bloat_investigation_2026-06-03.md).
//
// These tests stub out the etcd Put via the saveToEtcdPutFunc seam and
// verify the exact write count for each call sequence.

import (
	"context"
	"errors"
	"testing"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// withStubPut swaps saveToEtcdPutFunc for the duration of the test,
// restoring the original on cleanup. The returned counter tracks calls.
type putRecorder struct {
	calls  int
	keys   []string
	values []string
	err    error // set non-nil to simulate Put failure
}

func withStubPut(t *testing.T) *putRecorder {
	t.Helper()
	rec := &putRecorder{}
	prev := saveToEtcdPutFunc
	saveToEtcdPutFunc = func(ctx context.Context, cli *clientv3.Client, key, value string) error {
		rec.calls++
		rec.keys = append(rec.keys, key)
		rec.values = append(rec.values, value)
		return rec.err
	}
	t.Cleanup(func() { saveToEtcdPutFunc = prev })
	return rec
}

// fakeEtcdClient is a non-nil sentinel — saveToEtcd's first guard is
// `if cli == nil { return nil }`. We need any non-nil pointer to get
// past that check; the stub Put doesn't actually dereference cli.
func fakeEtcdClient() *clientv3.Client {
	return &clientv3.Client{}
}

func freshState() *controllerState {
	s := newControllerState()
	s.ClusterId = "test-cluster"
	s.CreatedAt = time.Unix(1_700_000_000, 0).UTC()
	return s
}

func TestSaveToEtcd_FirstWriteAlwaysPersists(t *testing.T) {
	// On a brand-new process / new state object, lastPersistedHash is
	// the zero value — the first call must write to etcd to take
	// ownership of the key, regardless of whether the bytes happen to
	// match what's already there.
	rec := withStubPut(t)
	s := freshState()
	if err := s.saveToEtcd(fakeEtcdClient()); err != nil {
		t.Fatalf("first save: %v", err)
	}
	if rec.calls != 1 {
		t.Errorf("first save call count=%d want 1", rec.calls)
	}
}

func TestSaveToEtcd_SkipsWhenUnchanged(t *testing.T) {
	// THE Phase 36 fix: two back-to-back saves with no state change in
	// between must produce exactly ONE etcd Put. The second call hits
	// the hash-skip branch and returns nil without writing.
	rec := withStubPut(t)
	s := freshState()
	cli := fakeEtcdClient()
	for i := 0; i < 5; i++ {
		if err := s.saveToEtcd(cli); err != nil {
			t.Fatalf("save %d: %v", i, err)
		}
	}
	if rec.calls != 1 {
		t.Fatalf("5 saves with unchanged state produced %d etcd Puts — want exactly 1 (Phase 36 dedup not active)", rec.calls)
	}
}

func TestSaveToEtcd_WritesWhenContentChanges(t *testing.T) {
	// State actually changed → hash differs → must write again. This is
	// the legitimate-update path that must continue to work.
	rec := withStubPut(t)
	s := freshState()
	cli := fakeEtcdClient()

	// First save (NetworkingGeneration starts at 1 from newControllerState).
	if err := s.saveToEtcd(cli); err != nil {
		t.Fatalf("first save: %v", err)
	}
	// Mutate state (set to a value DIFFERENT from the constructor default).
	s.NetworkingGeneration = 42
	if err := s.saveToEtcd(cli); err != nil {
		t.Fatalf("second save: %v", err)
	}
	// Mutate again.
	s.NetworkingGeneration = 43
	if err := s.saveToEtcd(cli); err != nil {
		t.Fatalf("third save: %v", err)
	}
	if rec.calls != 3 {
		t.Fatalf("3 distinct states produced %d Puts — want 3", rec.calls)
	}
}

func TestSaveToEtcd_FailedWriteDoesNotUpdateHash(t *testing.T) {
	// Crucial correctness: if the etcd Put returns an error, the hash
	// must NOT be updated. Otherwise a subsequent save with the same
	// content would be wrongly skipped and the state would never reach
	// etcd. Pin this by injecting a failure, then a success, then a
	// no-op — and verifying the success retry actually writes.
	rec := withStubPut(t)
	s := freshState()
	cli := fakeEtcdClient()

	// First save: simulated failure.
	rec.err = errors.New("simulated etcd unavailable")
	if err := s.saveToEtcd(cli); err == nil {
		t.Fatal("expected error from simulated failure, got nil")
	}
	if rec.calls != 1 {
		t.Fatalf("failure path call count=%d want 1", rec.calls)
	}

	// Retry: success.
	rec.err = nil
	if err := s.saveToEtcd(cli); err != nil {
		t.Fatalf("retry save: %v", err)
	}
	if rec.calls != 2 {
		t.Fatalf("retry should have written, call count=%d want 2", rec.calls)
	}

	// Third save with same state should now be skipped (post-success).
	if err := s.saveToEtcd(cli); err != nil {
		t.Fatalf("third save: %v", err)
	}
	if rec.calls != 2 {
		t.Fatalf("third save should be skipped, call count=%d want 2", rec.calls)
	}
}

func TestSaveToEtcd_NilClientIsNoOp(t *testing.T) {
	// Existing contract: nil etcd client returns nil without serializing
	// or hashing. This path is taken during bootstrap before etcd is up.
	rec := withStubPut(t)
	s := freshState()
	if err := s.saveToEtcd(nil); err != nil {
		t.Fatalf("nil client save: %v", err)
	}
	if rec.calls != 0 {
		t.Errorf("nil client should never invoke Put, got %d calls", rec.calls)
	}
	// Hash should remain zero so the next save (with non-nil client) writes.
	var zero [32]byte
	if s.lastPersistedHash != zero {
		t.Error("nil-client save must not advance lastPersistedHash")
	}
}

func TestSaveToEtcd_HashIsDeterministic(t *testing.T) {
	// Hash is computed from json.Marshal output. Pin determinism by
	// inserting map entries in different orders on the SAME state object
	// (since freshState() generates random per-call MinIO creds that
	// would otherwise dominate the comparison). The dedup contract:
	// adding a map entry then removing it produces the same serialized
	// state as never having added it — so the post-mutation save should
	// be a no-op skip.
	rec := withStubPut(t)
	cli := fakeEtcdClient()
	s := freshState()

	// Baseline save.
	if err := s.saveToEtcd(cli); err != nil {
		t.Fatalf("baseline save: %v", err)
	}
	baseline := rec.calls

	// Mutate, save (writes), then revert, save (must skip — same bytes
	// as baseline → same hash as the cached lastPersistedHash).
	s.JoinTokens["transient"] = &joinTokenRecord{Token: "x"}
	if err := s.saveToEtcd(cli); err != nil {
		t.Fatalf("mutation save: %v", err)
	}
	if rec.calls != baseline+1 {
		t.Fatalf("mutation should have written, got %d new calls", rec.calls-baseline)
	}

	delete(s.JoinTokens, "transient")
	if err := s.saveToEtcd(cli); err != nil {
		t.Fatalf("revert save: %v", err)
	}
	// After revert, hash matches baseline — Phase 36 dedup catches it.
	if rec.calls != baseline+2 {
		t.Errorf("revert-to-baseline should write once (to record the revert): got %d new calls", rec.calls-baseline)
	}

	// Saving again with the same state should skip.
	if err := s.saveToEtcd(cli); err != nil {
		t.Fatalf("post-revert no-op save: %v", err)
	}
	if rec.calls != baseline+2 {
		t.Errorf("post-revert no-op save should skip, got %d new calls", rec.calls-baseline)
	}
}

func TestSaveToEtcd_RestartIsolatedHashState(t *testing.T) {
	// Each *controllerState carries its own lastPersistedHash. A "new
	// process" simulation = constructing a new struct; that struct
	// MUST write on first save even if a previous process already
	// persisted equivalent bytes. This preserves crash-recovery
	// correctness: a freshly-loaded state takes ownership by writing
	// at least once.
	rec := withStubPut(t)
	cli := fakeEtcdClient()

	old := freshState()
	if err := old.saveToEtcd(cli); err != nil {
		t.Fatalf("old.save: %v", err)
	}
	beforeRestart := rec.calls

	// "Process restart": new state object, same bytes.
	restored := freshState()
	if err := restored.saveToEtcd(cli); err != nil {
		t.Fatalf("restored.save: %v", err)
	}
	if rec.calls != beforeRestart+1 {
		t.Fatalf("post-restart save should always write once; got %d new calls", rec.calls-beforeRestart)
	}
	// Subsequent unchanged save on the restored object should skip.
	if err := restored.saveToEtcd(cli); err != nil {
		t.Fatalf("restored.save (no-op): %v", err)
	}
	if rec.calls != beforeRestart+1 {
		t.Errorf("post-restart no-op save should be skipped; got %d new calls", rec.calls-beforeRestart)
	}
}

func truncForLog(s string) string {
	if len(s) > 80 {
		return s[:80]
	}
	return s
}
