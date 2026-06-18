package rules

import (
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

// Node-scoped reports build a reduced snapshot. They must preserve infra probe
// evidence; otherwise the node report falsely fires "*_probe_required_when_installed"
// even when the collector already has a valid probe for that node.
func TestEvaluateForNode_PreservesInfraProbeEvidence(t *testing.T) {
	const nodeID = "node-1"

	snap := &collector.Snapshot{
		Nodes: []*cluster_controllerpb.NodeRecord{
			{NodeId: nodeID},
		},
		NodeHealths: map[string]*cluster_controllerpb.NodeHealth{
			nodeID: {
				InstalledVersions: map[string]string{"etcd": "3.5.14"},
			},
		},
		Inventories: map[string]*node_agentpb.Inventory{
			nodeID: {
				Units: []*node_agentpb.UnitStatus{
					{Name: "globular-etcd.service"},
				},
			},
		},
		InfraProbes: map[string]*node_agentpb.GetInfraProbeResponse{
			nodeID: {
				Results: []*cluster_controllerpb.InfraProbeResult{
					{
						Component: "etcd",
						Violations: []*cluster_controllerpb.InfraViolation{
							{
								Id:       "etcd.config_valid",
								Severity: "ERROR",
								Message:  "initial-cluster-token mismatch",
								Evidence: "rendered=old desired=new",
							},
						},
					},
				},
			},
		},
	}

	r := &Registry{
		invariants: []Invariant{
			etcdInfraConfigValid{},
			etcdInfraProbeRequiredWhenInstalled{},
		},
	}

	findings := r.EvaluateForNode(snap, nodeID)
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %#v", len(findings), findings)
	}
	if findings[0].InvariantID != "etcd.config_valid" {
		t.Fatalf("invariant_id = %q, want %q", findings[0].InvariantID, "etcd.config_valid")
	}
	if findings[0].Severity != cluster_doctorpb.Severity_SEVERITY_ERROR {
		t.Fatalf("severity = %v, want %v", findings[0].Severity, cluster_doctorpb.Severity_SEVERITY_ERROR)
	}
}
