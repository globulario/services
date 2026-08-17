package collector

// verification_write_dedup_test.go
//
// persistVerificationResults re-Put every verdict on every collection sweep,
// unconditionally. A verdict only changes when the thing it describes changes,
// so in a steady cluster nearly every sweep rewrote the same bytes: on
// 2026-08-16 this path measured ~7.3 writes/sec across 232 keys — roughly
// 20 MB/hour of MVCC history and the second largest contributor to etcd
// reaching its 2 GiB backend quota (NOSPACE on 4 of 5 members, control plane
// read-only). Two samples of the same key 20s apart were byte-identical.
//
// These tests pin the skip contract. They deliberately do NOT bless the write
// itself: cluster-doctor is observer-only
// (invariant cluster_doctor.observer_only_never_writes_etcd, critical) and this
// Put remains a known allowlisted violation whose migration target is typed
// verdict persistence on cluster_controller. Deduplication makes the violation
// cheap; it does not make it lawful.

import "testing"

func TestVerificationWriteDedup_UnknownKeyAlwaysWrites(t *testing.T) {
	c := &Collector{}
	if c.shouldSkipVerificationWrite("/globular/verification/runtime/n1/svc", []byte(`{"ok":true}`)) {
		t.Fatal("a key we have never written must not be skipped — absence of a record " +
			"is not evidence that etcd already holds these bytes")
	}
}

func TestVerificationWriteDedup_IdenticalPayloadSkipped(t *testing.T) {
	c := &Collector{}
	key := "/globular/verification/runtime/n1/svc"
	payload := []byte(`{"ok":true,"proof":"abc"}`)

	c.recordVerificationWrite(key, payload)
	if !c.shouldSkipVerificationWrite(key, payload) {
		t.Error("identical payload must be skipped — this is the ~20 MB/hour that filled etcd")
	}
}

func TestVerificationWriteDedup_ChangedPayloadWrites(t *testing.T) {
	c := &Collector{}
	key := "/globular/verification/runtime/n1/svc"

	c.recordVerificationWrite(key, []byte(`{"ok":true}`))
	if c.shouldSkipVerificationWrite(key, []byte(`{"ok":false}`)) {
		t.Error("a changed verdict must be written — suppressing it would leave etcd " +
			"asserting a proof that no longer holds")
	}
}

// Per-key isolation: one service's verdict changing must not suppress or force
// another's.
func TestVerificationWriteDedup_IsolatedPerKey(t *testing.T) {
	c := &Collector{}
	a := "/globular/verification/runtime/n1/svc-a"
	b := "/globular/verification/runtime/n1/svc-b"
	payload := []byte(`{"ok":true}`)

	c.recordVerificationWrite(a, payload)
	if !c.shouldSkipVerificationWrite(a, payload) {
		t.Error("key a should skip")
	}
	if c.shouldSkipVerificationWrite(b, payload) {
		t.Error("key b was never written and must not inherit key a's record")
	}
}

// A failed Put must leave no record, so the next sweep retries. The production
// path enforces this by calling recordVerificationWrite only after a successful
// Put; this pins the consequence.
func TestVerificationWriteDedup_UnrecordedWriteIsRetried(t *testing.T) {
	c := &Collector{}
	key := "/globular/verification/runtime/n1/svc"
	payload := []byte(`{"ok":true}`)

	// Simulate: Put failed, so nothing was recorded.
	if c.shouldSkipVerificationWrite(key, payload) {
		t.Fatal("after a failed write the next sweep must retry, not assume the value landed")
	}
}

func TestVerificationWriteDedup_PruneDropsDisappearedKeys(t *testing.T) {
	c := &Collector{}
	gone := "/globular/verification/runtime/removed-node/svc"
	kept := "/globular/verification/runtime/n1/svc"
	payload := []byte(`{"ok":true}`)

	c.recordVerificationWrite(gone, payload)
	c.recordVerificationWrite(kept, payload)

	c.pruneVerificationWriteHashes(map[string]struct{}{kept: {}})

	if c.shouldSkipVerificationWrite(gone, payload) {
		t.Error("a key no longer produced must be forgotten, so a node that returns " +
			"is written afresh rather than silently skipped")
	}
	if !c.shouldSkipVerificationWrite(kept, payload) {
		t.Error("a still-live key must keep its record across a prune")
	}
}
