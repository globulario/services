package rules

import (
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
)

// Registry holds all registered invariants and evaluates them against a Snapshot.
type Registry struct {
	invariants []Invariant
	cfg        Config
}

// NewRegistry builds the default invariant registry with all v1 rules.
func NewRegistry(cfg Config) *Registry {
	r := &Registry{cfg: cfg}
	r.invariants = []Invariant{
		// Implementable invariants (available RPC data)
		nodeReachable{},
		nodeInventoryComplete{},
		nodePlanSuccess{},
		nodeUnitFilesPresent{},
		nodeUnitsRunning{},
		clusterServicesDrift{},
		clusterNetworkDrift{},
	}
	// Append PENDING stubs
	r.invariants = append(r.invariants, pendingInvariants()...)
	return r
}

// EvaluateAll runs all invariants against the snapshot and returns all findings.
func (r *Registry) EvaluateAll(snap *collector.Snapshot) []Finding {
	var all []Finding
	for _, inv := range r.invariants {
		all = append(all, inv.Evaluate(snap, r.cfg)...)
	}
	return all
}

// EvaluateForNode runs all node-scoped invariants for the given node id.
func (r *Registry) EvaluateForNode(snap *collector.Snapshot, nodeID string) []Finding {
	// Build a single-node snapshot view.
	nodesnap := &collector.Snapshot{
		SnapshotID:     snap.SnapshotID,
		GeneratedAt:    snap.GeneratedAt,
		DataSources:    snap.DataSources,
		DataIncomplete: snap.DataIncomplete,
		DataErrors:     snap.DataErrors,
		NodeHealths:    snap.NodeHealths,
		Inventories:    snap.Inventories,
		PlanStatuses:   snap.PlanStatuses,
		NodePlans:      snap.NodePlans,
	}
	// Filter Nodes to just the requested one.
	for _, n := range snap.Nodes {
		if n.GetNodeId() == nodeID {
			nodesnap.Nodes = append(nodesnap.Nodes, n)
			break
		}
	}

	var all []Finding
	for _, inv := range r.invariants {
		if inv.Scope() == "node" || inv.Scope() == "cluster" {
			all = append(all, inv.Evaluate(nodesnap, r.cfg)...)
		}
	}
	return all
}

// FindByID looks up a cached finding by its finding_id across all findings.
func FindByID(findings []Finding, findingID string) (Finding, bool) {
	for _, f := range findings {
		if f.FindingID == findingID {
			return f, true
		}
	}
	return Finding{}, false
}
