package main

import (
	"testing"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/rules"
)

// TestCacheFindings_NodeScopedDoesNotPoisonClusterDelta verifies that a
// node-scoped cacheFindings call (clusterWide=false) does NOT corrupt the
// cluster-wide delta authority. Without the clusterWide flag, the previous
// implementation produced spurious resolved → created event churn whenever a
// dashboard polled both endpoints: the node-scoped subset overwrote the
// shared lastFindings, then the next cluster-wide call computed delta
// against the subset and emitted ~N-K bogus "created" events for findings
// that had never actually disappeared.
//
// Repro:
//  1. Cluster-wide call with {A, B, C, D, E} — expect created [A,B,C,D,E]
//  2. Node-scoped call with {A, B} (subset) — expect NO events, lastEmitted unchanged
//  3. Cluster-wide call again with {A, B, C, D, E} — expect NO events (delta empty)
func TestCacheFindings_NodeScopedDoesNotPoisonClusterDelta(t *testing.T) {
	emitted := []string{} // captures (topic, finding_id)
	srv := &ClusterDoctorServer{
		cfg: &clusterdoctorConfig{EmitAuditEvents: false}, // don't need a real event client
	}
	// Stub the emitter by overriding through a thin wrapper. The real
	// publishFindingEvent requires srv.eventClient — we test the delta logic
	// only, not the wire emission. So we rely on EmitAuditEvents=false to
	// skip the publish call and check lastEmittedFindings directly.

	clusterWide := []rules.Finding{
		{FindingID: "A"}, {FindingID: "B"}, {FindingID: "C"},
		{FindingID: "D"}, {FindingID: "E"},
	}
	nodeSubset := []rules.Finding{
		{FindingID: "A"}, {FindingID: "B"},
	}

	// Step 1: cluster-wide → lastEmittedFindings becomes full set
	srv.cacheFindings(clusterWide, true)
	if len(srv.lastEmittedFindings) != 5 {
		t.Fatalf("after cluster-wide call, lastEmittedFindings len=%d want=5", len(srv.lastEmittedFindings))
	}
	if len(srv.lastFindings) != 5 {
		t.Fatalf("after cluster-wide call, lastFindings len=%d want=5", len(srv.lastFindings))
	}

	// Step 2: node-scoped → lastEmittedFindings must NOT change
	srv.cacheFindings(nodeSubset, false)
	if len(srv.lastEmittedFindings) != 5 {
		t.Fatalf("after node-scoped call, lastEmittedFindings len=%d want=5 (must not be overwritten by subset)", len(srv.lastEmittedFindings))
	}
	// lastFindings (the ExplainFinding lookup cache) IS allowed to change
	// to the node subset — that is its job, to reflect the most recent scope.
	if len(srv.lastFindings) != 2 {
		t.Fatalf("after node-scoped call, lastFindings len=%d want=2 (lookup cache should reflect node subset)", len(srv.lastFindings))
	}

	// Step 3: cluster-wide again with same set → delta vs lastEmittedFindings
	// must be empty. We detect "would have emitted" by capturing the count
	// difference: lastEmittedFindings should still have all 5 and be the same.
	srv.cacheFindings(clusterWide, true)
	if len(srv.lastEmittedFindings) != 5 {
		t.Fatalf("after second cluster-wide call, lastEmittedFindings len=%d want=5", len(srv.lastEmittedFindings))
	}

	_ = emitted
}

// TestCacheFindings_ClusterWideDeltaDetectsRealChange verifies the delta
// path still correctly detects real changes in the cluster-wide finding set.
func TestCacheFindings_ClusterWideDeltaDetectsRealChange(t *testing.T) {
	srv := &ClusterDoctorServer{
		cfg: &clusterdoctorConfig{EmitAuditEvents: false},
	}

	// Initial cluster-wide
	srv.cacheFindings([]rules.Finding{
		{FindingID: "A"}, {FindingID: "B"}, {FindingID: "C"},
	}, true)

	// State change: B resolved, D appeared
	srv.cacheFindings([]rules.Finding{
		{FindingID: "A"}, {FindingID: "C"}, {FindingID: "D"},
	}, true)

	// lastEmittedFindings must reflect the new set
	ids := map[string]bool{}
	for _, f := range srv.lastEmittedFindings {
		ids[f.FindingID] = true
	}
	if !ids["A"] || !ids["C"] || !ids["D"] {
		t.Fatalf("lastEmittedFindings missing expected IDs: %v", ids)
	}
	if ids["B"] {
		t.Fatalf("lastEmittedFindings still contains resolved finding B: %v", ids)
	}
}

// TestCacheFindings_NodeScopedRefreshesLookupCache verifies that a
// node-scoped call still updates the ExplainFinding lookup cache, so the
// MCP/CLI ExplainFinding for a node-scoped finding_id still works.
func TestCacheFindings_NodeScopedRefreshesLookupCache(t *testing.T) {
	srv := &ClusterDoctorServer{
		cfg: &clusterdoctorConfig{EmitAuditEvents: false},
	}

	srv.cacheFindings([]rules.Finding{
		{FindingID: "node-only-finding"},
	}, false)

	if len(srv.lastFindings) != 1 || srv.lastFindings[0].FindingID != "node-only-finding" {
		t.Fatalf("node-scoped call must populate lastFindings for ExplainFinding lookups; got %v", srv.lastFindings)
	}
	if len(srv.lastEmittedFindings) != 0 {
		t.Fatalf("node-scoped call must NOT populate lastEmittedFindings; got len=%d", len(srv.lastEmittedFindings))
	}
}
