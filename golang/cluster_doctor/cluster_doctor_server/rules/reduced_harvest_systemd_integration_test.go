package rules

import (
	"context"
	"errors"
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

func systemdFinding(findings []Finding) (Finding, bool) {
	for _, f := range findings {
		if f.InvariantID == "node.systemd.units_running" {
			return f, true
		}
	}
	return Finding{}, false
}

func TestReducedHarvestSystemd_OtherNodeDialFailureDoesNotVetoTarget(t *testing.T) {
	snap := &collector.Snapshot{
		DataIncomplete: true,
		DataErrors: []collector.DataError{
			{Service: "node_agent@n2", RPC: "dial", Err: errors.New("node n2 has no agent_endpoint")},
		},
		Nodes: []*cluster_controllerpb.NodeRecord{
			{NodeId: "n1"},
			{NodeId: "n2"},
		},
		Inventories: map[string]*node_agentpb.Inventory{
			"n1": {Units: []*node_agentpb.UnitStatus{{Name: "globular-torrent.service", State: "inactive"}}},
		},
	}
	registry := &Registry{
		invariants: []Invariant{nodeUnitsRunning{}},
		cfg:        testConfig(),
	}
	finding, ok := systemdFinding(registry.EvaluateAll(snap))
	if !ok {
		t.Fatal("expected node.systemd.units_running finding for n1")
	}
	if finding.InvariantStatus != cluster_doctorpb.InvariantStatus_INVARIANT_FAIL {
		t.Fatalf("status = %s, want FAIL: n2 dial failure is unrelated to n1 GetInventory evidence", finding.InvariantStatus)
	}
	if finding.CheckError != "" {
		t.Fatalf("CheckError = %q, want empty", finding.CheckError)
	}

	dispatcher := &evidenceClosureDispatcher{}
	healer := &Healer{Dispatcher: dispatcher, PolicyLookup: autoPolicy}
	report := healer.Evaluate(context.Background(), []Finding{finding})
	if dispatcher.calls != 1 || report.AutoFixed != 1 {
		t.Fatalf("governed target remediation was vetoed: calls=%d AutoFixed=%d", dispatcher.calls, report.AutoFixed)
	}
}

func TestReducedHarvestSystemd_TargetInventoryFailureRemainsFailClosed(t *testing.T) {
	// Keep a stale inventory entry deliberately. The registry must not let it
	// remain a conclusive FAIL when this sweep's target GetInventory call failed.
	snap := &collector.Snapshot{
		DataIncomplete: true,
		DataErrors: []collector.DataError{
			{Service: "node_agent@n1", RPC: "GetInventory", Err: errors.New("context deadline exceeded")},
		},
		Nodes: []*cluster_controllerpb.NodeRecord{{NodeId: "n1"}},
		Inventories: map[string]*node_agentpb.Inventory{
			"n1": {Units: []*node_agentpb.UnitStatus{{Name: "globular-torrent.service", State: "inactive"}}},
		},
	}
	registry := &Registry{
		invariants: []Invariant{nodeUnitsRunning{}},
		cfg:        testConfig(),
	}
	finding, ok := systemdFinding(registry.EvaluateAll(snap))
	if !ok {
		t.Fatal("expected diagnostic finding to remain visible")
	}
	if finding.InvariantStatus != cluster_doctorpb.InvariantStatus_INVARIANT_UNKNOWN {
		t.Fatalf("status = %s, want UNKNOWN when target inventory collection failed", finding.InvariantStatus)
	}
	if finding.CheckError == "" {
		t.Fatal("target evidence failure must carry CheckError")
	}

	dispatcher := &evidenceClosureDispatcher{}
	healer := &Healer{Dispatcher: dispatcher, PolicyLookup: autoPolicy}
	healer.Evaluate(context.Background(), []Finding{finding})
	if dispatcher.calls != 0 {
		t.Fatalf("dispatcher calls = %d, want 0: target inventory failure must fail closed", dispatcher.calls)
	}
}
