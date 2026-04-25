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
		nodeUnitFilesPresent{},
		nodeUnitsRunning{},
		installedStateRuntimeMismatch{},
		clusterServicesDrift{},
		clusterNetworkDrift{},
		promRuntime{},
		// Local filesystem checks
		prometheusBearerTokenFile{},
		// Operational diagnostics (multi-node expansion, bootstrap, etcd)
		etcdQuorumHealth{},
		staleNodeDetection{},
		bootstrapPhaseStuck{},
		nodeAgentCrash{},
		// Network diagnostics (multi-IP, WiFi stability)
		nodeMultiIP{},
		// Day 1 join failure diagnostics
		etcdStaleMember{},
		serviceRegistrationGap{},
		// Workflow convergence telemetry (WI20)
		workflowStepFailures{},
		workflowDriftStuck{},
		workflowNoActivity{},
		// MC-4: Blocked workflow runs requiring operator approval
		workflowBlockedRuns{},
		// Artifact identity invariants (cache digest, installed digest,
		// desired/installed build drift). Consumes per-node reports from
		// VerifyPackageIntegrity collected in Snapshot.IntegrityReports.
		artifactIntegrity{},
		// Certificate health invariants: expiry, SAN coverage, chain validity.
		// Consumes per-node GetCertificateStatus collected in Snapshot.CertificateStatus.
		certificateExpiry{},
		certificateSANCoverage{},
		certificateChainValid{},
		// Subsystem health: detects stuck/failed background goroutines.
		// Consumes per-node GetSubsystemHealth collected in Snapshot.SubsystemHealth.
		subsystemStuck{},
		// Objectstore topology invariants: DNS wildcard endpoint, standalone mode
		// in multi-node cluster, unreachable endpoint, missing desired state.
		// Consume ObjectStoreDesired populated from /globular/objectstore/config.
		objectstoreEndpointDNSWildcard{},
		objectstoreStandaloneInCluster{},
		objectstoreEndpointUnreachable{},
		objectstoreNoDesiredState{},
		objectstoreConsumerEndpointDNSWildcard{},
		// PKI health invariants: CA metadata publishing, CA expiry, per-node
		// cert-wrong-CA (issued by rotated CA). Consume CAMetadata populated
		// from /globular/pki/ca and CertificateStatus per node.
		pkiCANotPublished{},
		pkiCAExpiryWarning{},
		pkiNodeCertWrongCA{},
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
