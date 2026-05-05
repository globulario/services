package rules

import (
	"testing"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func freshNodeRecord(id string) *cluster_controllerpb.NodeRecord {
	return &cluster_controllerpb.NodeRecord{
		NodeId:   id,
		Status:   "ready",
		LastSeen: timestamppb.New(time.Now()),
	}
}

func freshNodeHealth(id string, pkgs map[string]string) *cluster_controllerpb.NodeHealth {
	return &cluster_controllerpb.NodeHealth{
		NodeId:            id,
		InstalledVersions: pkgs,
	}
}

func inventoryWithUnits(units ...*node_agentpb.UnitStatus) *node_agentpb.Inventory {
	return &node_agentpb.Inventory{Units: units}
}

func unit(name, state string) *node_agentpb.UnitStatus {
	return &node_agentpb.UnitStatus{Name: name, State: state}
}

// W03: COMMAND packages must not require systemd units.

func TestInstalledStateRuntimeMismatch_CommandPackage_NoFinding(t *testing.T) {
	snap := &collector.Snapshot{
		Nodes:       []*cluster_controllerpb.NodeRecord{freshNodeRecord("n1")},
		NodeHealths: map[string]*cluster_controllerpb.NodeHealth{"n1": freshNodeHealth("n1", map[string]string{"rclone": "1.65.0"})},
		Inventories: map[string]*node_agentpb.Inventory{"n1": inventoryWithUnits()}, // no units
	}
	findings := (installedStateRuntimeMismatch{}).Evaluate(snap, testConfig())
	if len(findings) != 0 {
		t.Errorf("COMMAND package rclone must not require a systemd unit, got %d findings", len(findings))
	}
}

func TestInstalledStateRuntimeMismatch_DaemonMissingUnit_FindingFired(t *testing.T) {
	snap := &collector.Snapshot{
		Nodes:       []*cluster_controllerpb.NodeRecord{freshNodeRecord("n1")},
		NodeHealths: map[string]*cluster_controllerpb.NodeHealth{"n1": freshNodeHealth("n1", map[string]string{"keepalived": "0.0.1"})},
		Inventories: map[string]*node_agentpb.Inventory{"n1": inventoryWithUnits()}, // no units at all
	}
	findings := (installedStateRuntimeMismatch{}).Evaluate(snap, testConfig())
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for missing keepalived unit, got %d", len(findings))
	}
	if findings[0].InvariantID != "installed_state_runtime_mismatch" {
		t.Errorf("wrong invariant_id: %s", findings[0].InvariantID)
	}
}

func TestInstalledStateRuntimeMismatch_DaemonActiveUnit_NoFinding(t *testing.T) {
	snap := &collector.Snapshot{
		Nodes:       []*cluster_controllerpb.NodeRecord{freshNodeRecord("n1")},
		NodeHealths: map[string]*cluster_controllerpb.NodeHealth{"n1": freshNodeHealth("n1", map[string]string{"keepalived": "0.0.1"})},
		Inventories: map[string]*node_agentpb.Inventory{"n1": inventoryWithUnits(unit("globular-keepalived.service", "active"))},
	}
	findings := (installedStateRuntimeMismatch{}).Evaluate(snap, testConfig())
	if len(findings) != 0 {
		t.Errorf("expected no finding for active keepalived unit, got %d", len(findings))
	}
}

func TestInstalledStateRuntimeMismatch_DaemonFailedUnit_FindingFired(t *testing.T) {
	snap := &collector.Snapshot{
		Nodes:       []*cluster_controllerpb.NodeRecord{freshNodeRecord("n1")},
		NodeHealths: map[string]*cluster_controllerpb.NodeHealth{"n1": freshNodeHealth("n1", map[string]string{"keepalived": "0.0.1"})},
		Inventories: map[string]*node_agentpb.Inventory{"n1": inventoryWithUnits(unit("globular-keepalived.service", "failed"))},
	}
	findings := (installedStateRuntimeMismatch{}).Evaluate(snap, testConfig())
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for failed keepalived unit, got %d", len(findings))
	}
	if findings[0].Severity != cluster_doctorpb.Severity_SEVERITY_WARN {
		t.Errorf("expected WARN for failed unit on ready node, got %v", findings[0].Severity)
	}
}

func TestInstalledStateRuntimeMismatch_UnhealthyNode_IsError(t *testing.T) {
	node := &cluster_controllerpb.NodeRecord{
		NodeId:   "n1",
		Status:   "degraded",
		LastSeen: timestamppb.New(time.Now()),
	}
	snap := &collector.Snapshot{
		Nodes:       []*cluster_controllerpb.NodeRecord{node},
		NodeHealths: map[string]*cluster_controllerpb.NodeHealth{"n1": freshNodeHealth("n1", map[string]string{"mcp": "1.0.0"})},
		Inventories: map[string]*node_agentpb.Inventory{"n1": inventoryWithUnits()},
	}
	findings := (installedStateRuntimeMismatch{}).Evaluate(snap, testConfig())
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding on degraded node, got %d", len(findings))
	}
	if findings[0].Severity != cluster_doctorpb.Severity_SEVERITY_ERROR {
		t.Errorf("expected ERROR for unhealthy node, got %v", findings[0].Severity)
	}
}

func TestInstalledStateRuntimeMismatch_StaleHeartbeat_FindingFired(t *testing.T) {
	staleNode := &cluster_controllerpb.NodeRecord{
		NodeId:   "n1",
		Status:   "ready",
		LastSeen: timestamppb.New(time.Now().Add(-10 * time.Minute)), // 10 min stale
	}
	snap := &collector.Snapshot{
		Nodes:       []*cluster_controllerpb.NodeRecord{staleNode},
		NodeHealths: map[string]*cluster_controllerpb.NodeHealth{"n1": freshNodeHealth("n1", map[string]string{"mcp": "1.0.0"})},
		Inventories: map[string]*node_agentpb.Inventory{"n1": inventoryWithUnits(unit("globular-mcp.service", "active"))},
	}
	findings := (installedStateRuntimeMismatch{}).Evaluate(snap, testConfig())
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for stale heartbeat, got %d", len(findings))
	}
	if findings[0].Summary == "" {
		t.Error("expected non-empty summary for stale finding")
	}
}

func TestInstalledStateRuntimeMismatch_EmptyVersion_NoFinding(t *testing.T) {
	// Packages with empty version should be ignored (no false positives).
	snap := &collector.Snapshot{
		Nodes:       []*cluster_controllerpb.NodeRecord{freshNodeRecord("n1")},
		NodeHealths: map[string]*cluster_controllerpb.NodeHealth{"n1": freshNodeHealth("n1", map[string]string{"mcp": ""})},
		Inventories: map[string]*node_agentpb.Inventory{"n1": inventoryWithUnits()},
	}
	findings := (installedStateRuntimeMismatch{}).Evaluate(snap, testConfig())
	if len(findings) != 0 {
		t.Errorf("expected no finding for empty version, got %d", len(findings))
	}
}

func TestInstalledStateRuntimeMismatch_NoInventory_NoFinding(t *testing.T) {
	// Node without inventory entry: rule should skip gracefully.
	snap := &collector.Snapshot{
		Nodes:       []*cluster_controllerpb.NodeRecord{freshNodeRecord("n1")},
		NodeHealths: map[string]*cluster_controllerpb.NodeHealth{"n1": freshNodeHealth("n1", map[string]string{"mcp": "1.0.0"})},
		Inventories: map[string]*node_agentpb.Inventory{}, // no inventory for n1
	}
	findings := (installedStateRuntimeMismatch{}).Evaluate(snap, testConfig())
	if len(findings) != 0 {
		t.Errorf("expected no finding when inventory is absent, got %d", len(findings))
	}
}

func TestCommandPackage_KnownCommands(t *testing.T) {
	known := []string{"rclone", "restic", "mc", "sctool", "etcdctl", "ffmpeg", "globular-cli"}
	for _, name := range known {
		if !commandPackage(name) {
			t.Errorf("expected %q to be classified as a command package", name)
		}
	}
}

func TestCommandPackage_DaemonIsNotCommand(t *testing.T) {
	daemons := []string{"keepalived", "mcp", "cluster-controller", "node-agent", "scylladb"}
	for _, name := range daemons {
		if commandPackage(name) {
			t.Errorf("expected %q NOT to be classified as a command package", name)
		}
	}
}
