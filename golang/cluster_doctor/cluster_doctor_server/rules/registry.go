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
		nativeDependencyMissing{},
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
		// G9: Per-node, per-package kind mismatch. Fires when the controller's
		// desired kind differs from the repository artifact kind, blocking
		// dispatch indefinitely. Companion to desired.kind_mismatch (aggregate).
		packageKindMismatch{},
		// G10: Controller leader pending self-update. Fires when the leader
		// cannot resign because no follower has reached the target build.
		// Escalates from WARNING to ERROR after pendingUpdateEscalateAfter.
		controllerLeaderPendingUpdate{},
		// G11: Direct observation that the workflow service is unreachable.
		// Distinct from release.blocked_workflow_unavailable (which is metric-
		// derived) — this fires when the doctor collector itself cannot connect.
		workflowServiceReachable{},
		// Artifact identity invariants (cache digest, installed digest,
		// desired/installed build drift). Consumes per-node reports from
		// VerifyPackageIntegrity collected in Snapshot.IntegrityReports.
		artifactIntegrity{},
		// Phase F repository-level invariants. Consumes
		// Snapshot.RepositoryFindings (populated by the collector calling
		// repository.PackageRepository.ListRepositoryFindings).
		repositoryFindings{},
		// Phase 2 hardening: operational mode invariant. Consumes
		// Snapshot.RepositoryOperationalStatus (from GetRepositoryStatus).
		// Fires when the repository is in DEGRADED/READ_ONLY/LOCAL_ONLY mode
		// or is unreachable. Replaces the "cluster.repo.reachable" pending stub.
		repositoryOperationalMode{},
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
		// Topology generation consistency: fires when desired topology generation
		// has not been applied via the objectstore.minio.apply_topology_generation
		// workflow. Consumes ObjectStoreDesired + ObjectStoreAppliedGeneration.
		objectstoreMinioTopologyConsistency{},
		// Fingerprint divergence: CRITICAL when any pool node rendered a
		// different topology than what the desired state specifies.
		// Consumes ObjectStoreDesired + NodeRenderedFingerprints.
		objectstoreMinioFingerprintDivergence{},
		// Post-apply health: CRITICAL when applied_generation == desired but
		// a pool node's globular-minio.service is not active.
		// Detects post-workflow regressions (crash, stale standalone config).
		objectstoreMinioPostApplyHealth{},
		// PKI health invariants: CA metadata publishing, CA expiry, per-node
		// cert-wrong-CA (issued by rotated CA). Consume CAMetadata populated
		// from /globular/pki/ca and CertificateStatus per node.
		pkiCANotPublished{},
		pkiCAExpiryWarning{},
		pkiNodeCertWrongCA{},
		// Disk admission invariants: split-brain standalone, unapproved paths,
		// quorum shape, and existing-data destructive guard.
		// Consume AdmittedDisks + DiskCandidates + ObjectStoreDesired.
		objectstoreMinioStandaloneSplitbrain{},
		objectstoreMinioActiveOnNonMember{},
		objectstoreMinioUnapprovedPath{},
		objectstoreMinioQuorumShape{},
		objectstoreMinioExistingDataGuard{},
		// Physical overlap and write-quorum invariants.
		// Detect NFS/CIFS path sharing between pool nodes (root cause of the
		// ryzen NFS overlap heal deadlock), network mount usage, EC:1 marginal
		// fault tolerance, live write-quorum loss, and format heal deadlock.
		// Consume DiskCandidates + ObjectStoreDesired + unit_state.
		objectstoreDuplicatePhysicalPath{},
		objectstoreNetworkMountUsed{},
		objectstoreZeroWriteFaultTolerance{},
		objectstoreWriteQuorumLost{},
		objectstoreFormatHealDeadlock{},
		// Topology contract invariants:
		//   contract_missing       — MinIO running but no desired state in etcd.
		//   credentials_missing    — contract present but credentials_ready=false.
		//   endpoint_unresolved    — contract present but endpoint_ready=false.
		//   destructive_guard      — destructive topology change pending without
		//                           an approved TopologyTransition record.
		// Consume ObjectStoreDesired + DesiredTopologyTransition + Inventories.
		objectstoreContractMissing{},
		objectstoreCredentialsMissing{},
		objectstoreEndpointUnresolved{},
		objectstoreDestructiveGuard{},
		// Critical-state guardians for ingress/scylla control-plane durability.
		ingressSpecMissing{},
		ingressNodeHoldingLastKnownGood{},
		ingressAmbiguousDisableRejected{},
		scyllaKeyspaceRFPolicyViolation{},
		criticalKeyRegistryPresence{},
		// DNS degraded-mode visibility from /globular/dns/v1/status.
		dnsZoneReloadFailed{},
		dnsServingLastKnownGood{},
		// Reconcile lane status fallback from etcd (when Prometheus unavailable).
		reconcileLaneStatusEtcd{},
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
