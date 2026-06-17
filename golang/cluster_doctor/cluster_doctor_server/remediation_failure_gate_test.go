package main

import (
	"context"
	"testing"
	"time"
)

// withFreshAuditRing isolates the package-level remediation audit ring and
// restores the default ring-backed reader for the duration of a test.
func withFreshAuditRing(t *testing.T) {
	t.Helper()
	prevRing := remediationAudits
	prevFn := listRemediationAuditsFn
	remediationAudits = &remediationAuditRing{maxSize: 500}
	listRemediationAuditsFn = listRemediationAuditsFromRing
	t.Cleanup(func() {
		remediationAudits = prevRing
		listRemediationAuditsFn = prevFn
	})
}

// TestFailureRateGate_CountsFromInProcessRing locks in the AWG re-audit fix:
// the cross-attempt failure-rate breaker reads recent audits from the
// in-process ring (the etcd writer is a no-op for observer-only), so it counts
// real recent failures instead of always returning 0 (dead since v1.2.166).
func TestFailureRateGate_CountsFromInProcessRing(t *testing.T) {
	withFreshAuditRing(t)

	now := time.Now()
	inv, dig, action := "runtime.desired_enabled_not_alive", "sha256:abc", "SYSTEMCTL_RESTART"
	rec := func(a RemediationAudit) { remediationAudits.push(a) }

	rec(RemediationAudit{InvariantID: inv, EvidenceDigest: dig, ActionType: action, Executed: false, Reason: "boom", Timestamp: now.Unix()})
	rec(RemediationAudit{InvariantID: inv, EvidenceDigest: dig, ActionType: action, Executed: false, Reason: "boom", Timestamp: now.Unix()})
	rec(RemediationAudit{InvariantID: inv, EvidenceDigest: dig, ActionType: action, Executed: false, Reason: "boom", Timestamp: now.Unix()})
	rec(RemediationAudit{InvariantID: inv, EvidenceDigest: dig, ActionType: action, Executed: true, Timestamp: now.Unix()})                                       // success — not a failure
	rec(RemediationAudit{InvariantID: inv, EvidenceDigest: dig, ActionType: action, Executed: false, Reason: "boom", DryRun: true, Timestamp: now.Unix()})        // dry-run — excluded
	rec(RemediationAudit{InvariantID: inv, EvidenceDigest: dig, ActionType: action, Executed: false, Reason: "boom", Timestamp: now.Add(-2 * time.Hour).Unix()}) // too old

	n := countRecentFailedActionAttempts(context.Background(), inv, dig, action, now.Add(-time.Hour), 500)
	if n != 3 {
		t.Fatalf("expected 3 recent failed attempts from the ring, got %d (gate is dead if 0)", n)
	}

	// Scope: a different evidence digest must not be counted.
	if n2 := countRecentFailedActionAttempts(context.Background(), inv, "sha256:other", action, now.Add(-time.Hour), 500); n2 != 0 {
		t.Errorf("different evidence digest must not match, got %d", n2)
	}
}

// TestAuditRemediation_PopulatesRing pins the writer side: auditRemediation must
// record into the in-process ring so the failure-rate gate has a live source
// (the etcd persist is a no-op). Audit isolation (TestMain) stubs the etcd
// persist, so this never touches production etcd.
func TestAuditRemediation_PopulatesRing(t *testing.T) {
	withFreshAuditRing(t)

	id := auditRemediation(context.Background(), RemediationAudit{
		InvariantID:    "runtime.desired_enabled_not_alive",
		EvidenceDigest: "sha256:abc",
		ActionType:     "SYSTEMCTL_RESTART",
		Executed:       false,
		Reason:         "boom",
	})
	if id == "" {
		t.Fatal("auditRemediation should return a non-empty audit id")
	}
	got := remediationAudits.list(0)
	if len(got) != 1 {
		t.Fatalf("auditRemediation must record into the ring, got %d entries", len(got))
	}
	if got[0].Timestamp == 0 {
		t.Error("recorded audit should carry a timestamp")
	}
}
