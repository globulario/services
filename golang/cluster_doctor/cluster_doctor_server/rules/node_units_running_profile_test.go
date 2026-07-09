package rules

// node_units_running_profile_test.go — Regression for profile-aware
// expected-unit evaluation. Expected service state follows the node's
// ASSIGNED PROFILES (controller-owned placement intent), never a hardcoded
// every-node assumption. Scar: after a core-only day-1 join (globule-nuc,
// 2026-07-08), the doctor reported scylla-server.service "inactive (expected
// active)" on a node that must not run ScyllaDB at all — the same
// local-scylla assumption that deadlocked needs_scylla services via their
// systemd ExecStartPre gate. See
// docs/design/package-identity-single-authority.md §7 and
// ops.always.profile.authority-model.

import (
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

func TestNodeUnitsRunning_ScyllaSilentOnNonStorageNode(t *testing.T) {
	snap := &collector.Snapshot{
		Nodes: []*cluster_controllerpb.NodeRecord{{NodeId: "n1", Profiles: []string{"core"}}},
		Inventories: map[string]*node_agentpb.Inventory{"n1": {Units: []*node_agentpb.UnitStatus{
			{Name: "scylla-server.service", State: "inactive"},
			{Name: "globular-scylla-manager.service", State: "inactive"},
			{Name: "globular-scylla-manager-agent.service", State: "inactive"},
		}}},
	}
	findings := (nodeUnitsRunning{}).Evaluate(snap, testConfig())
	for _, f := range findings {
		if f.InvariantID == "node.systemd.units_running" {
			t.Errorf("storage-plane unit inactive on a core-only node must be silent (inactive BY DESIGN), got finding: %+v", f)
		}
	}
}

func TestNodeUnitsRunning_ScyllaFiresOnStorageNode(t *testing.T) {
	snap := &collector.Snapshot{
		Nodes: []*cluster_controllerpb.NodeRecord{{NodeId: "n1", Profiles: []string{"core", "storage"}}},
		Inventories: map[string]*node_agentpb.Inventory{"n1": {Units: []*node_agentpb.UnitStatus{
			{Name: "scylla-server.service", State: "inactive"},
		}}},
	}
	findings := (nodeUnitsRunning{}).Evaluate(snap, testConfig())
	hit := false
	for _, f := range findings {
		if f.InvariantID == "node.systemd.units_running" && f.EntityRef == "n1/scylla-server.service" {
			hit = true
		}
	}
	if !hit {
		t.Fatal("scylla-server inactive on a STORAGE node is a real problem and must surface a finding")
	}
}

func TestNodeUnitsRunning_NonStorageSuppressionIsNarrow(t *testing.T) {
	// The suppression must be narrow: a non-storage-plane unit on the same
	// core-only node still fires. Guards against an over-broad future patch.
	snap := &collector.Snapshot{
		Nodes: []*cluster_controllerpb.NodeRecord{{NodeId: "n1", Profiles: []string{"core"}}},
		Inventories: map[string]*node_agentpb.Inventory{"n1": {Units: []*node_agentpb.UnitStatus{
			{Name: "scylla-server.service", State: "inactive"},
			{Name: "globular-dns.service", State: "failed"},
		}}},
	}
	findings := (nodeUnitsRunning{}).Evaluate(snap, testConfig())
	sawDNS := false
	for _, f := range findings {
		if f.EntityRef == "n1/scylla-server.service" {
			t.Errorf("scylla-server must be suppressed on core-only node; got %+v", f)
		}
		if f.EntityRef == "n1/globular-dns.service" {
			sawDNS = true
		}
	}
	if !sawDNS {
		t.Fatal("globular-dns.service failed on a core node must still fire")
	}
}
