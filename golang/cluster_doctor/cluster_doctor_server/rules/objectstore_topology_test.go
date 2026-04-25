package rules

import (
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/config"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// threeNodePoolSnap builds a Snapshot that represents a healthy 3-node MinIO
// pool in distributed mode. Callers override individual fields to inject faults.
func threeNodePoolSnap() *collector.Snapshot {
	desired := &config.ObjectStoreDesiredState{
		Mode:          config.ObjectStoreModeDistributed,
		Generation:    3,
		Endpoint:      "10.0.0.63:9000",
		Nodes:         []string{"10.0.0.63", "10.0.0.8", "10.0.0.20"},
		DrivesPerNode: 1,
		VolumesHash:   "abc123def456",
	}
	expectedFP := config.RenderStateFingerprint(desired)

	nodes := []*cluster_controllerpb.NodeRecord{
		{NodeId: "node-1", Identity: &cluster_controllerpb.NodeIdentity{Ips: []string{"10.0.0.63"}}},
		{NodeId: "node-2", Identity: &cluster_controllerpb.NodeIdentity{Ips: []string{"10.0.0.8"}}},
		{NodeId: "node-3", Identity: &cluster_controllerpb.NodeIdentity{Ips: []string{"10.0.0.20"}}},
	}

	inventories := map[string]*node_agentpb.Inventory{
		"node-1": minioActiveInventory(),
		"node-2": minioActiveInventory(),
		"node-3": minioActiveInventory(),
	}

	return &collector.Snapshot{
		ObjectStoreDesired:       desired,
		ObjectStoreAppliedGeneration: 3,
		Nodes:                    nodes,
		Inventories:              inventories,
		NodeRenderedGenerations: map[string]int64{
			"node-1": 3, "node-2": 3, "node-3": 3,
		},
		NodeRenderedFingerprints: map[string]string{
			"node-1": expectedFP, "node-2": expectedFP, "node-3": expectedFP,
		},
	}
}

func minioActiveInventory() *node_agentpb.Inventory {
	return &node_agentpb.Inventory{
		Units: []*node_agentpb.UnitStatus{
			{Name: "globular-minio.service", State: "active"},
		},
	}
}

func minioInactiveInventory(state string) *node_agentpb.Inventory {
	return &node_agentpb.Inventory{
		Units: []*node_agentpb.UnitStatus{
			{Name: "globular-minio.service", State: state},
		},
	}
}

// ── topology_consistency ──────────────────────────────────────────────────────

func TestTopologyConsistency_Converged(t *testing.T) {
	snap := threeNodePoolSnap()
	findings := objectstoreMinioTopologyConsistency{}.Evaluate(snap, Config{})
	if len(findings) != 0 {
		t.Fatalf("fully converged pool should produce 0 findings, got %d: %v", len(findings), findings[0].Summary)
	}
}

// TestTopologyConsistency_GenerationLag verifies WARN is emitted when
// applied_generation lags behind desired — workflow not yet completed.
func TestTopologyConsistency_GenerationLag(t *testing.T) {
	snap := threeNodePoolSnap()
	snap.ObjectStoreAppliedGeneration = 2 // lag of 1

	findings := objectstoreMinioTopologyConsistency{}.Evaluate(snap, Config{})
	if len(findings) != 1 {
		t.Fatalf("generation lag should produce 1 finding, got %d", len(findings))
	}
	f := findings[0]
	if f.Severity != cluster_doctorpb.Severity_SEVERITY_WARN {
		t.Errorf("expected WARN severity for generation lag, got %v", f.Severity)
	}
	if f.InvariantID != "objectstore.minio.topology_consistency" {
		t.Errorf("wrong invariant_id: %s", f.InvariantID)
	}
}

// TestTopologyConsistency_NeverApplied verifies CRITICAL is emitted when
// desired mode is distributed but applied_generation is still 0 (workflow
// has never run).
func TestTopologyConsistency_NeverApplied(t *testing.T) {
	snap := threeNodePoolSnap()
	snap.ObjectStoreAppliedGeneration = 0
	snap.ObjectStoreDesired.Mode = config.ObjectStoreModeDistributed

	findings := objectstoreMinioTopologyConsistency{}.Evaluate(snap, Config{})
	if len(findings) != 1 {
		t.Fatalf("never-applied distributed mode should produce 1 finding, got %d", len(findings))
	}
	if findings[0].Severity != cluster_doctorpb.Severity_SEVERITY_CRITICAL {
		t.Errorf("expected CRITICAL severity, got %v", findings[0].Severity)
	}
}

func TestTopologyConsistency_SingleNode_NoFinding(t *testing.T) {
	snap := &collector.Snapshot{
		ObjectStoreDesired: &config.ObjectStoreDesiredState{
			Mode:       config.ObjectStoreModeStandalone,
			Generation: 1,
			Nodes:      []string{"10.0.0.63"},
		},
		ObjectStoreAppliedGeneration: 1,
	}
	findings := objectstoreMinioTopologyConsistency{}.Evaluate(snap, Config{})
	if len(findings) != 0 {
		t.Fatalf("single-node cluster should produce 0 findings, got %d", len(findings))
	}
}

// ── fingerprint_divergence ────────────────────────────────────────────────────

func TestFingerprintDivergence_AllMatch_NoFinding(t *testing.T) {
	snap := threeNodePoolSnap()
	findings := objectstoreMinioFingerprintDivergence{}.Evaluate(snap, Config{})
	if len(findings) != 0 {
		t.Fatalf("matching fingerprints should produce 0 findings, got %d: %v", len(findings), findings[0].Summary)
	}
}

// TestFingerprintDivergence_NodeFingerprintMismatch verifies CRITICAL fires
// when a pool node rendered a different topology fingerprint.
func TestFingerprintDivergence_NodeFingerprintMismatch(t *testing.T) {
	snap := threeNodePoolSnap()
	snap.NodeRenderedFingerprints["node-2"] = "000000000000stale" // wrong fingerprint

	findings := objectstoreMinioFingerprintDivergence{}.Evaluate(snap, Config{})
	if len(findings) != 1 {
		t.Fatalf("fingerprint mismatch should produce 1 finding, got %d", len(findings))
	}
	f := findings[0]
	if f.Severity != cluster_doctorpb.Severity_SEVERITY_CRITICAL {
		t.Errorf("expected CRITICAL, got %v", f.Severity)
	}
	if f.InvariantID != "objectstore.minio.fingerprint_divergence" {
		t.Errorf("wrong invariant_id: %s", f.InvariantID)
	}
}

// TestFingerprintDivergence_NodeMissingFingerprint verifies CRITICAL fires
// when a pool node hasn't written any fingerprint yet (not rendered).
func TestFingerprintDivergence_NodeMissingFingerprint(t *testing.T) {
	snap := threeNodePoolSnap()
	delete(snap.NodeRenderedFingerprints, "node-3")

	findings := objectstoreMinioFingerprintDivergence{}.Evaluate(snap, Config{})
	if len(findings) != 1 {
		t.Fatalf("missing fingerprint should produce 1 finding, got %d", len(findings))
	}
	if findings[0].Severity != cluster_doctorpb.Severity_SEVERITY_CRITICAL {
		t.Errorf("expected CRITICAL, got %v", findings[0].Severity)
	}
}

// TestFingerprintDivergence_SingleNode_NoFinding verifies the invariant is
// silent for single-node pools (standalone mode has no fingerprint agreement requirement).
func TestFingerprintDivergence_SingleNode_NoFinding(t *testing.T) {
	desired := &config.ObjectStoreDesiredState{
		Mode: config.ObjectStoreModeStandalone, Generation: 1, Nodes: []string{"10.0.0.63"},
	}
	snap := &collector.Snapshot{
		ObjectStoreDesired: desired,
		NodeRenderedFingerprints: map[string]string{"node-1": "anyvalue"},
	}
	findings := objectstoreMinioFingerprintDivergence{}.Evaluate(snap, Config{})
	if len(findings) != 0 {
		t.Fatalf("single-node pool should produce 0 findings, got %d", len(findings))
	}
}

// TestFingerprintDivergence_StaleStandaloneAfterJoin verifies that a node
// that renders a standalone-mode fingerprint after a second node joined
// (desired=distributed) causes a CRITICAL finding.
func TestFingerprintDivergence_StaleStandaloneAfterJoin(t *testing.T) {
	// node-2 was wiped and re-rendered standalone mode
	standaloneDesired := &config.ObjectStoreDesiredState{
		Mode:      config.ObjectStoreModeStandalone,
		Generation: 3, // same generation but different topology
		Nodes:     []string{"10.0.0.8"},
	}
	standaloneFP := config.RenderStateFingerprint(standaloneDesired)

	snap := threeNodePoolSnap()
	snap.NodeRenderedFingerprints["node-2"] = standaloneFP

	findings := objectstoreMinioFingerprintDivergence{}.Evaluate(snap, Config{})
	if len(findings) != 1 {
		t.Fatalf("standalone fingerprint in distributed pool should produce 1 finding, got %d", len(findings))
	}
	if findings[0].Severity != cluster_doctorpb.Severity_SEVERITY_CRITICAL {
		t.Errorf("expected CRITICAL for stale standalone config, got %v", findings[0].Severity)
	}
}

// ── post_apply_health ─────────────────────────────────────────────────────────

func TestPostApplyHealth_AllActive_NoFinding(t *testing.T) {
	snap := threeNodePoolSnap()
	findings := objectstoreMinioPostApplyHealth{}.Evaluate(snap, Config{})
	if len(findings) != 0 {
		t.Fatalf("all-active pool should produce 0 findings, got %d: %v", len(findings), findings[0].Summary)
	}
}

// TestPostApplyHealth_NodeNotActive verifies CRITICAL fires when applied_generation
// equals desired but a pool node's MinIO service is not active.
func TestPostApplyHealth_NodeNotActive(t *testing.T) {
	snap := threeNodePoolSnap()
	snap.Inventories["node-2"] = minioInactiveInventory("failed")

	findings := objectstoreMinioPostApplyHealth{}.Evaluate(snap, Config{})
	if len(findings) != 1 {
		t.Fatalf("inactive node should produce 1 finding, got %d", len(findings))
	}
	f := findings[0]
	if f.Severity != cluster_doctorpb.Severity_SEVERITY_CRITICAL {
		t.Errorf("expected CRITICAL, got %v", f.Severity)
	}
	if f.InvariantID != "objectstore.minio.post_apply_health" {
		t.Errorf("wrong invariant_id: %s", f.InvariantID)
	}
}

// TestPostApplyHealth_NotYetApplied_NoFinding verifies the invariant is silent
// when applied_generation < desired (the topology workflow hasn't run yet —
// that's topology_consistency's domain, not post_apply_health).
func TestPostApplyHealth_NotYetApplied_NoFinding(t *testing.T) {
	snap := threeNodePoolSnap()
	snap.ObjectStoreAppliedGeneration = 2 // lag
	snap.Inventories["node-1"] = minioInactiveInventory("inactive")

	findings := objectstoreMinioPostApplyHealth{}.Evaluate(snap, Config{})
	if len(findings) != 0 {
		t.Fatalf("unapplied generation should not fire post_apply_health, got %d", len(findings))
	}
}

// TestPostApplyHealth_NoInventory_Fires verifies that a pool node with no
// inventory data (unreachable) triggers a finding when the workflow has applied.
func TestPostApplyHealth_NoInventory_Fires(t *testing.T) {
	snap := threeNodePoolSnap()
	delete(snap.Inventories, "node-3")

	findings := objectstoreMinioPostApplyHealth{}.Evaluate(snap, Config{})
	if len(findings) != 1 {
		t.Fatalf("no-inventory node should produce 1 finding, got %d", len(findings))
	}
	if findings[0].Severity != cluster_doctorpb.Severity_SEVERITY_CRITICAL {
		t.Errorf("expected CRITICAL, got %v", findings[0].Severity)
	}
}

// TestPostApplyHealth_SingleNode_NoFinding verifies the invariant is silent
// for single-node pools (only distributed topologies are checked).
func TestPostApplyHealth_SingleNode_NoFinding(t *testing.T) {
	snap := &collector.Snapshot{
		ObjectStoreDesired: &config.ObjectStoreDesiredState{
			Mode: config.ObjectStoreModeStandalone, Generation: 1, Nodes: []string{"10.0.0.63"},
		},
		ObjectStoreAppliedGeneration: 1,
	}
	findings := objectstoreMinioPostApplyHealth{}.Evaluate(snap, Config{})
	if len(findings) != 0 {
		t.Fatalf("single-node should produce 0 findings, got %d", len(findings))
	}
}

// ── combined invariant set ────────────────────────────────────────────────────

// TestAllThreeInvariantsPassOnConvergedCluster runs all three topology
// invariants against a fully healthy snapshot and expects no findings.
func TestAllThreeInvariantsPassOnConvergedCluster(t *testing.T) {
	snap := threeNodePoolSnap()
	cfg := Config{}

	invs := []Invariant{
		objectstoreMinioTopologyConsistency{},
		objectstoreMinioFingerprintDivergence{},
		objectstoreMinioPostApplyHealth{},
	}
	for _, inv := range invs {
		findings := inv.Evaluate(snap, cfg)
		if len(findings) != 0 {
			t.Errorf("invariant %s: expected 0 findings on converged cluster, got %d: %s",
				inv.ID(), len(findings), findings[0].Summary)
		}
	}
}

// TestFailedWorkflowVisibleInLastRestartResult checks that the doctor
// post_apply_health invariant fires when applied_generation is at desired
// but MinIO regressed — simulating a failed restart that didn't roll back.
// The last_restart_result would contain status=failed (checked by the CLI/script).
func TestFailedWorkflowVisibleInLastRestartResult(t *testing.T) {
	snap := threeNodePoolSnap()
	// Simulate: workflow ran (applied == desired), but then MinIO crashed on node-1.
	snap.Inventories["node-1"] = minioInactiveInventory("failed")

	findings := objectstoreMinioPostApplyHealth{}.Evaluate(snap, Config{})
	if len(findings) == 0 {
		t.Fatal("post_apply_health should fire when minio crashes post-apply")
	}
	// The finding should point to re-running the topology workflow.
	found := false
	for _, r := range findings[0].Remediation {
		if r.CliCommand != "" && len(r.CliCommand) > 0 {
			found = true
			break
		}
	}
	if !found {
		t.Error("finding should include at least one remediation CLI command")
	}
}
