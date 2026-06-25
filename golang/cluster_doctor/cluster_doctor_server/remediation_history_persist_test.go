package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	ai_memorypb "github.com/globulario/services/golang/ai_memory/ai_memorypb"
)

// EX-3b tests. The fake ai-memory client (fakeGateMemory) is defined in
// remediation_gate_persist_test.go and satisfies remediationAuditMemory via its
// Store/Query methods.
//
// Test #7 from the design ("no etcd path") is covered by the existing scanner
// TestClusterDoctor_NoNewEtcdDataWrites (cluster_doctor_etcd_authority_test.go),
// which greps the whole package for `.Put(ctx, "/globular/...")` etcd data writes.
// EX-3b persists via the ai-memory gRPC client (Store), not `.Put`, so it is
// covered there rather than re-asserted weakly here.

// withFreshAuditRing (shared helper in remediation_failure_gate_test.go) swaps in
// an empty audit ring and resets listRemediationAuditsFn for the duration of a
// test, restoring both on cleanup.

// TestPersistRemediationAudit_StoresToAiMemory — an audit record becomes an
// ai-memory entry tagged for the audit ring.
func TestPersistRemediationAudit_StoresToAiMemory(t *testing.T) {
	fake := newFakeGateMemory()
	setRemediationAuditAiMemoryClient(fake)
	t.Cleanup(func() { setRemediationAuditAiMemoryClient(nil) })

	persistRemediationAudit(context.Background(), RemediationAudit{
		AuditID: "rem-1", InvariantID: "inv-a", ActionType: "SYSTEMCTL_RESTART", Timestamp: 1000,
	})
	if n := fake.countWithTag(remediationAuditTagBase); n != 1 {
		t.Fatalf("expected 1 persisted audit memory, got %d", n)
	}
}

// TestWarmLoadRemediationAudits_RepopulatesRing — persisted audits are loaded back
// into an empty in-process ring (the restart/failover recovery path).
func TestWarmLoadRemediationAudits_RepopulatesRing(t *testing.T) {
	fake := newFakeGateMemory()
	setRemediationAuditAiMemoryClient(fake)
	t.Cleanup(func() { setRemediationAuditAiMemoryClient(nil) })
	withFreshAuditRing(t)

	persistRemediationAudit(context.Background(), RemediationAudit{AuditID: "rem-1", ActionType: "A", Timestamp: 1000})
	persistRemediationAudit(context.Background(), RemediationAudit{AuditID: "rem-2", ActionType: "B", Timestamp: 2000})

	if len(remediationAudits.list(0)) != 0 {
		t.Fatalf("precondition: ring should be empty before warm-load (persist stores to ai-memory, not the ring)")
	}
	warmLoadRemediationAudits(context.Background())
	if got := len(remediationAudits.list(0)); got != 2 {
		t.Errorf("warm-load should repopulate the ring with 2 audits, got %d", got)
	}
}

// TestWarmLoad_FilteredFailureCountTripsGate — after warm-load, the failure-rate
// gate counts the recovered failed attempts for the matching key (the whole point:
// failover does not forget recent faceplants).
func TestWarmLoad_FilteredFailureCountTripsGate(t *testing.T) {
	fake := newFakeGateMemory()
	setRemediationAuditAiMemoryClient(fake)
	t.Cleanup(func() { setRemediationAuditAiMemoryClient(nil) })
	withFreshAuditRing(t)

	now := time.Now()
	for i := 0; i < 2; i++ {
		persistRemediationAudit(context.Background(), RemediationAudit{
			AuditID: fmt.Sprintf("rem-f%d", i), InvariantID: "inv-x", EvidenceDigest: "dig-1",
			ActionType: "SYSTEMCTL_RESTART", Timestamp: now.Unix(), Executed: false, Reason: "boom",
		})
	}
	warmLoadRemediationAudits(context.Background())

	got := countRecentFailedActionAttempts(context.Background(), "inv-x", "dig-1", "SYSTEMCTL_RESTART", now.Add(-time.Hour), 500)
	if got != 2 {
		t.Errorf("after warm-load, failure count should be 2, got %d", got)
	}
}

// TestWarmLoad_OldAuditOutsideWindowIgnored — an audit within retention but older
// than the failure-rate query window is warm-loaded yet must not trip the gate.
func TestWarmLoad_OldAuditOutsideWindowIgnored(t *testing.T) {
	fake := newFakeGateMemory()
	setRemediationAuditAiMemoryClient(fake)
	t.Cleanup(func() { setRemediationAuditAiMemoryClient(nil) })
	withFreshAuditRing(t)

	now := time.Now()
	persistRemediationAudit(context.Background(), RemediationAudit{
		AuditID: "rem-old", InvariantID: "inv-x", EvidenceDigest: "dig-1",
		ActionType: "SYSTEMCTL_RESTART", Timestamp: now.Add(-48 * time.Hour).Unix(), Executed: false, Reason: "old",
	})
	warmLoadRemediationAudits(context.Background())

	if len(remediationAudits.list(0)) != 1 {
		t.Fatalf("expected the old audit to be warm-loaded into the ring")
	}
	got := countRecentFailedActionAttempts(context.Background(), "inv-x", "dig-1", "SYSTEMCTL_RESTART", now.Add(-time.Hour), 500)
	if got != 0 {
		t.Errorf("an audit older than the window must not trip the gate, got count %d", got)
	}
}

// TestWarmLoad_MalformedRecordIgnored — a corrupt (non-JSON) record is skipped, the
// ring is not poisoned, and a valid sibling still loads. No panic.
func TestWarmLoad_MalformedRecordIgnored(t *testing.T) {
	fake := newFakeGateMemory()
	setRemediationAuditAiMemoryClient(fake)
	t.Cleanup(func() { setRemediationAuditAiMemoryClient(nil) })
	withFreshAuditRing(t)

	_, _ = fake.Store(context.Background(), &ai_memorypb.StoreRqst{Memory: &ai_memorypb.Memory{
		Project: remediationGateMemoryProject,
		Tags:    []string{remediationAuditTagBase},
		Content: "}{ not json",
	}})
	persistRemediationAudit(context.Background(), RemediationAudit{AuditID: "rem-ok", ActionType: "A", Timestamp: 1000})

	warmLoadRemediationAudits(context.Background()) // must not panic

	got := remediationAudits.list(0)
	if len(got) != 1 || got[0].AuditID != "rem-ok" {
		t.Errorf("warm-load must skip the malformed record and load only the valid one, got %+v", got)
	}
}

// TestRemediationAudit_NilClientNoOp — with no ai-memory client wired, persist and
// warm-load are safe no-ops: the doctor must never become unavailable because its
// audit store is unreachable.
func TestRemediationAudit_NilClientNoOp(t *testing.T) {
	setRemediationAuditAiMemoryClient(nil)
	withFreshAuditRing(t)

	persistRemediationAudit(context.Background(), RemediationAudit{AuditID: "x", Timestamp: 1}) // no panic
	warmLoadRemediationAudits(context.Background())                                             // no panic
	if len(remediationAudits.list(0)) != 0 {
		t.Error("nil client: warm-load must not populate the ring")
	}
}
