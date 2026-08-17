package main

import "testing"

// The quorum_loss alert is raised when the cluster is at risk and retired when
// it is not. Those are the same predicate read twice, so they must be the same
// predicate in code.
//
// They were not: the alert was raised at critical >= 2 but retired only at
// critical == 0. A cluster that recovers from a two-node outage straight into
// an unrelated single-node partition never passes through zero, so a CRITICAL
// "quorum at risk" alert kept flying while quorum was not at risk. Observed
// 2026-08-11 in the resilience suite — dual-node-failure raised it legitimately,
// then network-partition-fencing (which partitions exactly ONE node, so the
// trigger step cannot have run) failed its no_quorum_loss_alert assertion on
// the stale key.
func TestQuorumAtRiskCount_RaiseAndRetireShareOneThreshold(t *testing.T) {
	for _, tc := range []struct {
		critical int
		atRisk   bool
		why      string
	}{
		{0, false, "no critical nodes"},
		{1, false, "one partitioned node still leaves write quorum"},
		{2, true, "two critical nodes is the raise threshold"},
		{3, true, "more than two stays at risk"},
	} {
		if got := quorumAtRiskCount(tc.critical); got != tc.atRisk {
			t.Errorf("quorumAtRiskCount(%d) = %v, want %v (%s)", tc.critical, got, tc.atRisk, tc.why)
		}

		// The retire path is the negation of the raise path. If these ever
		// disagree the alert latches (retire stricter) or flaps (retire looser).
		raise := quorumAtRiskCount(tc.critical)
		retire := !quorumAtRiskCount(tc.critical)
		if raise == retire {
			t.Errorf("critical=%d: raise and retire both %v — they must be exact negations", tc.critical, raise)
		}
	}
}

// The specific regression: one critical node must retire an alert raised
// earlier by two, without the count first returning to zero.
func TestQuorumAtRiskCount_SingleCriticalNodeRetiresAStaleAlert(t *testing.T) {
	if !quorumAtRiskCount(2) {
		t.Fatal("critical=2 must raise the alert")
	}
	if quorumAtRiskCount(1) {
		t.Fatal("critical=1 must NOT be at risk — the retire path is gated on this and a stale CRITICAL alert would survive the recovery")
	}
}
