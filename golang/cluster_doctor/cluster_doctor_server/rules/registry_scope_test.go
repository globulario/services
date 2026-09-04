package rules

import (
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
)

// scopeProbe records how many nodes the snapshot it was handed contained.
type scopeProbe struct {
	scope    string
	seen     *int
	sawNodes *[]string
}

func (p scopeProbe) ID() string       { return "test.scope_probe_" + p.scope }
func (p scopeProbe) Category() string { return "test" }
func (p scopeProbe) Scope() string    { return p.scope }
func (p scopeProbe) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	*p.seen = len(snap.Nodes)
	ids := make([]string, 0, len(snap.Nodes))
	for _, n := range snap.Nodes {
		ids = append(ids, n.GetNodeId())
	}
	*p.sawNodes = ids
	return nil
}

// TestEvaluateForNode_ClusterScopedRulesSeeFullNodeSet locks in the fix for the
// 2026-08-19 false-CRITICAL regression.
//
// EvaluateForNode builds a snapshot whose Nodes list is filtered down to the one
// node being reported on. Node-scoped rules want that view. Cluster-scoped rules
// do NOT: they reason about cluster-wide membership, so a filtered view makes
// every other node look absent (no NodeRecord, no inventory) and they count those
// nodes as down.
//
// Concretely, objectstore.write_quorum_lost fired CRITICAL
// ("active_drives=1 < write_quorum=3") against a fully healthy 4-drive MinIO pool,
// naming a different "sole survivor" on every node's report, while
// known_down_nodes was empty — nothing had actually been observed down. The
// verdict was built entirely from nodes missing from a deliberately narrowed
// snapshot.
//
// Contract: a cluster-scoped rule must receive the same node set from
// EvaluateForNode that it would receive from EvaluateCluster.
func TestEvaluateForNode_ClusterScopedRulesSeeFullNodeSet(t *testing.T) {
	snap := &collector.Snapshot{
		Nodes: []*cluster_controllerpb.NodeRecord{
			{NodeId: "node-a"},
			{NodeId: "node-b"},
			{NodeId: "node-c"},
		},
	}

	var nodeSeen, clusterSeen int
	var nodeIDs, clusterIDs []string

	r := &Registry{invariants: []Invariant{
		scopeProbe{scope: "node", seen: &nodeSeen, sawNodes: &nodeIDs},
		scopeProbe{scope: "cluster", seen: &clusterSeen, sawNodes: &clusterIDs},
	}}

	r.EvaluateForNode(snap, "node-b")

	// Node-scoped: exactly the requested node.
	if nodeSeen != 1 {
		t.Errorf("node-scoped rule saw %d node(s) %v, want 1 (the requested node)", nodeSeen, nodeIDs)
	}
	if len(nodeIDs) == 1 && nodeIDs[0] != "node-b" {
		t.Errorf("node-scoped rule saw %q, want \"node-b\"", nodeIDs[0])
	}

	// Cluster-scoped: the full set. If this drops back to 1, cluster rules are
	// again reasoning over a truncated membership view and will emit verdicts
	// derived from absent data instead of observed state.
	if clusterSeen != len(snap.Nodes) {
		t.Errorf("cluster-scoped rule saw %d node(s) %v, want %d (full node set) — "+
			"cluster rules must not be evaluated against the node-filtered snapshot",
			clusterSeen, clusterIDs, len(snap.Nodes))
	}
}
