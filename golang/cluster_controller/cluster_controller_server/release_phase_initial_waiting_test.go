package main

import (
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

// TestInitialPhaseCanEnterWaiting is the regression guard for the
// scylladb-not-in-repository CRIT: a fresh InfrastructureRelease (empty status,
// phase "") whose artifact is not yet published must be allowed to transition
// "" -> WAITING (the publish-wait backoff), not fail with
// `invalid phase transition "" → "WAITING"`. release_pipeline.go patches
// straight to WAITING on ErrNoPublishedArtifact, and a fresh release is still at
// phase "" at that point.
func TestInitialPhaseCanEnterWaiting(t *testing.T) {
	if err := advancePhase("", cluster_controllerpb.ReleasePhaseWaiting); err != nil {
		t.Fatalf(`advancePhase("", WAITING) must be allowed for a fresh release whose artifact is not yet published, got: %v`, err)
	}

	// And the documented recovery loop out of WAITING must remain valid.
	if err := advancePhase(cluster_controllerpb.ReleasePhaseWaiting, cluster_controllerpb.ReleasePhasePending); err != nil {
		t.Fatalf("WAITING -> PENDING (retry after backoff) must stay valid, got: %v", err)
	}
	if err := advancePhase(cluster_controllerpb.ReleasePhaseWaiting, cluster_controllerpb.ReleasePhaseResolved); err != nil {
		t.Fatalf("WAITING -> RESOLVED (artifact appeared) must stay valid, got: %v", err)
	}
}
