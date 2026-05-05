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

func TestNativeDependencyMissing_SQLFailedUnit_FindingFired(t *testing.T) {
	snap := &collector.Snapshot{
		Nodes: []*cluster_controllerpb.NodeRecord{{
			NodeId:   "n1",
			Status:   "ready",
			LastSeen: timestamppb.New(time.Now()),
		}},
		NodeHealths: map[string]*cluster_controllerpb.NodeHealth{
			"n1": {NodeId: "n1", InstalledVersions: map[string]string{"sql": "1.2.7"}},
		},
		Inventories: map[string]*node_agentpb.Inventory{
			"n1": {Units: []*node_agentpb.UnitStatus{
				{Name: "globular-sql.service", State: "failed"},
			}},
		},
	}
	findings := (nativeDependencyMissing{}).Evaluate(snap, testConfig())
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for failed sql unit, got %d", len(findings))
	}
	f := findings[0]
	if f.InvariantID != "package.native_dependency_missing" {
		t.Errorf("wrong invariant_id: %s", f.InvariantID)
	}
	if f.Severity != cluster_doctorpb.Severity_SEVERITY_ERROR {
		t.Errorf("expected ERROR severity, got %v", f.Severity)
	}
}

func TestNativeDependencyMissing_SQLActiveUnit_NoFinding(t *testing.T) {
	// Unit is active — no native dep finding even if package is in the known map.
	snap := &collector.Snapshot{
		Nodes: []*cluster_controllerpb.NodeRecord{{
			NodeId:   "n1",
			Status:   "ready",
			LastSeen: timestamppb.New(time.Now()),
		}},
		NodeHealths: map[string]*cluster_controllerpb.NodeHealth{
			"n1": {NodeId: "n1", InstalledVersions: map[string]string{"sql": "1.2.7"}},
		},
		Inventories: map[string]*node_agentpb.Inventory{
			"n1": {Units: []*node_agentpb.UnitStatus{
				{Name: "globular-sql.service", State: "active"},
			}},
		},
	}
	findings := (nativeDependencyMissing{}).Evaluate(snap, testConfig())
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for active sql unit, got %d", len(findings))
	}
}

func TestNativeDependencyMissing_UnknownPackage_NoFinding(t *testing.T) {
	// A package not in knownNativeDeps must never produce a finding.
	snap := &collector.Snapshot{
		Nodes: []*cluster_controllerpb.NodeRecord{{
			NodeId:   "n1",
			Status:   "ready",
			LastSeen: timestamppb.New(time.Now()),
		}},
		NodeHealths: map[string]*cluster_controllerpb.NodeHealth{
			"n1": {NodeId: "n1", InstalledVersions: map[string]string{"dns": "1.2.7"}},
		},
		Inventories: map[string]*node_agentpb.Inventory{
			"n1": {Units: []*node_agentpb.UnitStatus{
				{Name: "globular-dns.service", State: "failed"},
			}},
		},
	}
	findings := (nativeDependencyMissing{}).Evaluate(snap, testConfig())
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for unknown package with failed unit, got %d", len(findings))
	}
}

func TestNativeDependencyMissing_NoInventory_NoFinding(t *testing.T) {
	snap := &collector.Snapshot{
		Nodes: []*cluster_controllerpb.NodeRecord{{
			NodeId:   "n1",
			Status:   "ready",
			LastSeen: timestamppb.New(time.Now()),
		}},
		NodeHealths: map[string]*cluster_controllerpb.NodeHealth{
			"n1": {NodeId: "n1", InstalledVersions: map[string]string{"sql": "1.2.7"}},
		},
		Inventories: map[string]*node_agentpb.Inventory{}, // no inventory for n1
	}
	findings := (nativeDependencyMissing{}).Evaluate(snap, testConfig())
	if len(findings) != 0 {
		t.Errorf("expected 0 findings when inventory absent, got %d", len(findings))
	}
}

func TestNativeDependencyMissing_EmptyVersion_NoFinding(t *testing.T) {
	snap := &collector.Snapshot{
		Nodes: []*cluster_controllerpb.NodeRecord{{
			NodeId:   "n1",
			Status:   "ready",
			LastSeen: timestamppb.New(time.Now()),
		}},
		NodeHealths: map[string]*cluster_controllerpb.NodeHealth{
			"n1": {NodeId: "n1", InstalledVersions: map[string]string{"sql": ""}},
		},
		Inventories: map[string]*node_agentpb.Inventory{
			"n1": {Units: []*node_agentpb.UnitStatus{
				{Name: "globular-sql.service", State: "failed"},
			}},
		},
	}
	findings := (nativeDependencyMissing{}).Evaluate(snap, testConfig())
	if len(findings) != 0 {
		t.Errorf("expected 0 findings for empty version, got %d", len(findings))
	}
}

func TestNativeDependencyMissing_SummaryMentionsLibrary(t *testing.T) {
	snap := &collector.Snapshot{
		Nodes: []*cluster_controllerpb.NodeRecord{{
			NodeId:   "n1",
			Status:   "ready",
			LastSeen: timestamppb.New(time.Now()),
		}},
		NodeHealths: map[string]*cluster_controllerpb.NodeHealth{
			"n1": {NodeId: "n1", InstalledVersions: map[string]string{"sql": "1.2.7"}},
		},
		Inventories: map[string]*node_agentpb.Inventory{
			"n1": {Units: []*node_agentpb.UnitStatus{
				{Name: "globular-sql.service", State: "failed"},
			}},
		},
	}
	findings := (nativeDependencyMissing{}).Evaluate(snap, testConfig())
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if !containsStr(findings[0].Summary, "libodbc.so.2") {
		t.Errorf("expected summary to mention libodbc.so.2, got: %s", findings[0].Summary)
	}
}

func TestNativeDependencyMissing_RemediationMentionsAptInstall(t *testing.T) {
	snap := &collector.Snapshot{
		Nodes: []*cluster_controllerpb.NodeRecord{{
			NodeId:   "n1",
			Status:   "ready",
			LastSeen: timestamppb.New(time.Now()),
		}},
		NodeHealths: map[string]*cluster_controllerpb.NodeHealth{
			"n1": {NodeId: "n1", InstalledVersions: map[string]string{"sql": "1.2.7"}},
		},
		Inventories: map[string]*node_agentpb.Inventory{
			"n1": {Units: []*node_agentpb.UnitStatus{
				{Name: "globular-sql.service", State: "failed"},
			}},
		},
	}
	findings := (nativeDependencyMissing{}).Evaluate(snap, testConfig())
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	if len(findings[0].Remediation) == 0 {
		t.Fatal("expected remediation steps")
	}
	step1 := findings[0].Remediation[0].GetDescription()
	if !containsStr(step1, "unixodbc") {
		t.Errorf("expected remediation step 1 to mention unixodbc, got: %s", step1)
	}
}
