package main

import (
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

// TestPendingIsUniversalReEntry pins the rule the RESOLVED→PENDING edge was
// missing from: PENDING is this machine's re-entry point, and every
// non-terminal phase must be able to return to it. Every sibling already could
// — WAITING ("retry after backoff"), DEFERRED ("retry target selection"),
// AVAILABLE ("drift re-resolve"), FAILED and ROLLED_BACK ("re-apply") — which is
// what makes RESOLVED's absence an omission rather than a rule.
//
// It mattered on the enforcing caller path: release_reconciler.go abandons the
// patch when advancePhase refuses, so a release sitting in RESOLVED could not be
// returned to PENDING and stayed put. Observed 5 times across the 1.2.353
// evidence bundles as "invalid phase transition RESOLVED → PENDING".
//
// Written as a rule over the whole table rather than a single assertion, so a
// future phase added without a re-entry edge fails here instead of stranding
// releases in production.
func TestPendingIsUniversalReEntry(t *testing.T) {
	// Excluded, each for a stated reason — the exclusions are asserted below so
	// this list cannot quietly absorb a real omission:
	//
	//   REMOVED   terminal by design; the only phase with no outgoing edges.
	//   REMOVING  teardown is deliberately one-way. Re-entering at PENDING would
	//             resurrect a release that was being deleted, which is a worse
	//             bug than the one this rule exists to catch.
	//   APPLYING  "a legacy label kept only for API boundary compatibility.
	//             Internal code never writes this phase" — its edges exist only
	//             for old rows, so a re-entry edge would be dead weight.
	excluded := map[string]bool{
		ReleasePhaseRemoved:                       true,
		ReleasePhaseRemoving:                      true,
		cluster_controllerpb.ReleasePhaseApplying: true,
	}

	for phase := range validPhaseTransitions {
		if excluded[phase] || phase == cluster_controllerpb.ReleasePhasePending {
			continue
		}
		if err := advancePhase(phase, cluster_controllerpb.ReleasePhasePending); err != nil {
			t.Errorf("live phase %q cannot re-enter at PENDING: %v", phase, err)
		}
	}
}

// TestRemovingDoesNotResurrect asserts the most important exclusion rather than
// merely listing it. Teardown must stay one-way: a release in REMOVING may only
// reach REMOVED or FAILED.
func TestRemovingDoesNotResurrect(t *testing.T) {
	if err := advancePhase(ReleasePhaseRemoving, cluster_controllerpb.ReleasePhasePending); err == nil {
		t.Fatal("REMOVING must not re-enter at PENDING — that would resurrect a release being deleted")
	}
}

// TestRemovedStaysTerminal is the negative control. Without it, the rule above
// could be satisfied by making every transition legal, which would leave the
// machine constraining nothing. REMOVED must keep having no way out.
func TestRemovedStaysTerminal(t *testing.T) {
	if err := advancePhase(ReleasePhaseRemoved, cluster_controllerpb.ReleasePhasePending); err == nil {
		t.Fatal("REMOVED is terminal and must not gain an outgoing edge to PENDING")
	}
}

// TestResolvedReEntryIsTheFixedCase names the specific stranding that was
// observed, so a revert of the edge fails with the reason attached.
func TestResolvedReEntryIsTheFixedCase(t *testing.T) {
	if err := advancePhase(cluster_controllerpb.ReleasePhaseResolved, cluster_controllerpb.ReleasePhasePending); err != nil {
		t.Fatalf("RESOLVED must be able to re-resolve via PENDING: %v", err)
	}
}
