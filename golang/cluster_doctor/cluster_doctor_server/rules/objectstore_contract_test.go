package rules

import (
	"testing"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/config"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

// ── objectstoreContractMissing ────────────────────────────────────────────────

func TestContractMissing_NoDesiredState_NoMinIO_NoFinding(t *testing.T) {
	// No desired state and no MinIO running → silent (pre-formation).
	snap := &collector.Snapshot{
		Nodes:       []*cluster_controllerpb.NodeRecord{{NodeId: "node-1"}},
		Inventories: map[string]*node_agentpb.Inventory{},
	}
	findings := objectstoreContractMissing{}.Evaluate(snap, Config{})
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings when no minio running, got %d", len(findings))
	}
}

func TestContractMissing_DesiredStatePresent_NoFinding(t *testing.T) {
	// Desired state exists → no finding regardless of MinIO status.
	snap := threeNodePoolSnap()
	findings := objectstoreContractMissing{}.Evaluate(snap, Config{})
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings when desired state present, got %d", len(findings))
	}
}

func TestContractMissing_NoDesiredState_MinIOActive_Critical(t *testing.T) {
	// No desired state but MinIO is active → CRITICAL.
	snap := &collector.Snapshot{
		ObjectStoreDesired: nil,
		Nodes: []*cluster_controllerpb.NodeRecord{
			{NodeId: "node-1"},
		},
		Inventories: map[string]*node_agentpb.Inventory{
			"node-1": minioActiveInventory(),
		},
	}
	findings := objectstoreContractMissing{}.Evaluate(snap, Config{})
	if len(findings) != 1 {
		t.Fatalf("expected 1 CRITICAL finding, got %d", len(findings))
	}
	if findings[0].Severity != cluster_doctorpb.Severity_SEVERITY_CRITICAL {
		t.Errorf("expected CRITICAL severity, got %v", findings[0].Severity)
	}
	if findings[0].InvariantID != "objectstore.minio.contract_missing" {
		t.Errorf("unexpected invariant ID: %s", findings[0].InvariantID)
	}
}

func TestContractMissing_NoDesiredState_MinIOInactive_NoFinding(t *testing.T) {
	// No desired state and MinIO is inactive → no finding (not yet deployed).
	snap := &collector.Snapshot{
		ObjectStoreDesired: nil,
		Nodes: []*cluster_controllerpb.NodeRecord{
			{NodeId: "node-1"},
		},
		Inventories: map[string]*node_agentpb.Inventory{
			"node-1": minioInactiveInventory("inactive"),
		},
	}
	findings := objectstoreContractMissing{}.Evaluate(snap, Config{})
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings for inactive MinIO, got %d", len(findings))
	}
}

// ── objectstoreDestructiveGuard ───────────────────────────────────────────────

func TestDestructiveGuard_Converged_NoFinding(t *testing.T) {
	// Applied == desired → no pending change → no finding.
	snap := threeNodePoolSnap()
	snap.AppliedStateFingerprint = config.RenderStateFingerprint(snap.ObjectStoreDesired)
	findings := objectstoreDestructiveGuard{}.Evaluate(snap, Config{})
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings when fully converged, got %d: %v", len(findings), findings[0].Summary)
	}
}

func TestDestructiveGuard_NonDestructiveBump_NoFinding(t *testing.T) {
	// Generation bumped but fingerprint unchanged (e.g. credential rotation) → no finding.
	snap := threeNodePoolSnap()
	// Same fingerprint as desired but applied gen is older.
	snap.ObjectStoreDesired.Generation = 4
	snap.ObjectStoreAppliedGeneration = 3
	snap.AppliedStateFingerprint = config.RenderStateFingerprint(snap.ObjectStoreDesired)
	findings := objectstoreDestructiveGuard{}.Evaluate(snap, Config{})
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings for non-destructive bump, got %d: %v", len(findings), findings[0].Summary)
	}
}

func TestDestructiveGuard_FingerprintChange_NoTransition_Critical(t *testing.T) {
	// Fingerprint changed and no transition record → CRITICAL.
	snap := threeNodePoolSnap()
	snap.ObjectStoreDesired.Generation = 4
	snap.ObjectStoreAppliedGeneration = 3
	snap.AppliedStateFingerprint = "stale-fingerprint-000000"
	snap.DesiredTopologyTransition = nil

	findings := objectstoreDestructiveGuard{}.Evaluate(snap, Config{})
	if len(findings) != 1 {
		t.Fatalf("expected 1 CRITICAL finding, got %d", len(findings))
	}
	if findings[0].Severity != cluster_doctorpb.Severity_SEVERITY_CRITICAL {
		t.Errorf("expected CRITICAL, got %v", findings[0].Severity)
	}
	if findings[0].InvariantID != "objectstore.minio.destructive_guard" {
		t.Errorf("unexpected invariant ID: %s", findings[0].InvariantID)
	}
}

func TestDestructiveGuard_FingerprintChange_ApprovedTransition_NoFinding(t *testing.T) {
	// Fingerprint changed but transition record is approved → safe.
	snap := threeNodePoolSnap()
	snap.ObjectStoreDesired.Generation = 4
	snap.ObjectStoreAppliedGeneration = 3
	snap.AppliedStateFingerprint = "stale-fingerprint-000000"
	snap.DesiredTopologyTransition = &config.TopologyTransition{
		Generation:    4,
		IsDestructive: true,
		Approved:      true,
		AffectedNodes: []string{"10.0.0.63", "10.0.0.8", "10.0.0.20"},
		CreatedAt:     time.Now(),
	}

	findings := objectstoreDestructiveGuard{}.Evaluate(snap, Config{})
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings when transition is approved, got %d: %v", len(findings), findings[0].Summary)
	}
}

func TestDestructiveGuard_FingerprintChange_UnapprovedTransition_Warn(t *testing.T) {
	// Transition record exists but Approved=false → WARN.
	snap := threeNodePoolSnap()
	snap.ObjectStoreDesired.Generation = 4
	snap.ObjectStoreAppliedGeneration = 3
	snap.AppliedStateFingerprint = "stale-fingerprint-000000"
	snap.DesiredTopologyTransition = &config.TopologyTransition{
		Generation:    4,
		IsDestructive: true,
		Approved:      false,
		Reasons:       []string{"fingerprint change"},
		CreatedAt:     time.Now(),
	}

	findings := objectstoreDestructiveGuard{}.Evaluate(snap, Config{})
	if len(findings) != 1 {
		t.Fatalf("expected 1 WARN finding, got %d", len(findings))
	}
	if findings[0].Severity != cluster_doctorpb.Severity_SEVERITY_WARN {
		t.Errorf("expected WARN, got %v", findings[0].Severity)
	}
}

func TestDestructiveGuard_FirstDistributed_NoTransition_Critical(t *testing.T) {
	// First distributed topology (never applied) and no transition record → CRITICAL.
	desired := &config.ObjectStoreDesiredState{
		Mode:          config.ObjectStoreModeDistributed,
		Generation:    1,
		Endpoint:      "10.0.0.63:9000",
		Nodes:         []string{"10.0.0.63", "10.0.0.8", "10.0.0.20"},
		DrivesPerNode: 1,
	}
	snap := &collector.Snapshot{
		ObjectStoreDesired:           desired,
		ObjectStoreAppliedGeneration: 0, // never applied
		AppliedStateFingerprint:      "",
		DesiredTopologyTransition:    nil,
		Nodes:                        []*cluster_controllerpb.NodeRecord{},
		Inventories:                  map[string]*node_agentpb.Inventory{},
		NodeRenderedGenerations:      map[string]int64{},
		NodeRenderedFingerprints:     map[string]string{},
	}

	findings := objectstoreDestructiveGuard{}.Evaluate(snap, Config{})
	if len(findings) != 1 {
		t.Fatalf("expected 1 CRITICAL finding for first distributed topology, got %d", len(findings))
	}
	if findings[0].Severity != cluster_doctorpb.Severity_SEVERITY_CRITICAL {
		t.Errorf("expected CRITICAL, got %v", findings[0].Severity)
	}
}

func TestDestructiveGuard_FirstDistributed_ApprovedTransition_NoFinding(t *testing.T) {
	// First distributed topology but with an approved transition → safe.
	desired := &config.ObjectStoreDesiredState{
		Mode:          config.ObjectStoreModeDistributed,
		Generation:    1,
		Endpoint:      "10.0.0.63:9000",
		Nodes:         []string{"10.0.0.63", "10.0.0.8", "10.0.0.20"},
		DrivesPerNode: 1,
	}
	snap := &collector.Snapshot{
		ObjectStoreDesired:           desired,
		ObjectStoreAppliedGeneration: 0,
		AppliedStateFingerprint:      "",
		DesiredTopologyTransition: &config.TopologyTransition{
			Generation:    1,
			IsDestructive: true,
			Approved:      true,
			CreatedAt:     time.Now(),
		},
		Nodes:                    []*cluster_controllerpb.NodeRecord{},
		Inventories:              map[string]*node_agentpb.Inventory{},
		NodeRenderedGenerations:  map[string]int64{},
		NodeRenderedFingerprints: map[string]string{},
	}

	findings := objectstoreDestructiveGuard{}.Evaluate(snap, Config{})
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings for approved first-distributed transition, got %d: %v", len(findings), findings[0].Summary)
	}
}
