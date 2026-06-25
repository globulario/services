// @awareness namespace=globular.platform
// @awareness component=platform_cluster_doctor.invariant_registry
// @awareness file_role=single_source_for_invariant_evaluation_dispatch
// @awareness implements=globular.platform:intent.doctor.findings_are_operator_language
// @awareness implements=globular.platform:intent.remediation.must_go_through_workflow
// @awareness risk=medium
package rules

// Registry is the only path through which Snapshot data becomes Findings.
// New HealAuto invariants must:
//   1) register here via Register(...)
//   2) have a corresponding policy entry in heal_policy.go's PolicyV1
//      with a non-empty AutoAction
//   3) emit at least one RemediationStep carrying a structured
//      RemediationAction proto (so the gatedDispatcher can route it
//      through ExecuteRemediation)
// Skipping (2) or (3) leaves the healer with a HealAuto disposition that
// never dispatches — the rule appears to work but is silently a no-op.

import (
	"strconv"
	"strings"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"google.golang.org/protobuf/types/known/timestamppb"
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
		releaseBoundaryUnproven{},
		nativeDependencyMissing{},
		clusterServicesDrift{},
		// OT-3: flag when the doctor's own service-config mirror is stale (etcd
		// config fetches failing → GetServicesConfigurations silently serving
		// stale-if-error data), so diagnoses are not made against stale config.
		serviceConfigCacheFresh{},
		// E2: installed package whose node profiles do not authorize it under
		// the catalog placement map — a terminal, non-dispatchable orphan that
		// can never converge (the torrent-orphan class). Operator action required.
		placementInstalledPackageOrphaned{},
		clusterNetworkDrift{},
		promRuntime{},
		// Local filesystem checks
		prometheusBearerTokenFile{},
		artifactFilesystemSafetyLocal{},
		artifactLayoutDriftLocal{},
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
		// Backbone guard: direct observations of gRPC contract regressions
		// (cluster_id propagation, call-depth loops, public probe admission).
		grpcBackboneContract{},
		// Degraded-mode (PR-15): distinguishes a broken gateway route from a
		// down service by comparing the Envoy gateway path against the direct
		// backend port. Consumes Snapshot.GatewayBackendProbes.
		gatewayBackendDivergence{},
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
		// Proactive guard: fires when the desired spec carries mode=disabled
		// without a valid explicit-disable guard (before any node processes it).
		// Complements ingressAmbiguousDisableRejected which fires on node status.
		ingressUnguardedDisableIntent{},
		scyllaKeyspaceRFPolicyViolation{},
		repositoryKeyspaceRFPolicyViolation{},
		// Critical-key registry presence: key is absent from etcd.
		criticalKeyRegistryPresence{},
		// Critical-key ownership completeness: key in live-check list has no
		// declared owner in config.CriticalKeyPolicies. Static check — no etcd.
		criticalKeyOwnershipComplete{},
		// DNS degraded-mode visibility from /globular/dns/v1/status.
		dnsZoneReloadFailed{},
		dnsServingLastKnownGood{},
		// Reconcile lane status fallback from etcd (when Prometheus unavailable).
		reconcileLaneStatusEtcd{},
		// 4-layer integrity rules for the repository/DNS join.
		// repository.desired_build_ids_resolve fires when an active desired
		// build_id has no installable artifact (root cause of the production
		// install-storm).
		// dns.records_match_runtime_health fires when a node would still be
		// included in a profile-derived record despite failing the readiness
		// gate — surfaces unpatched reconcilers and reconciler bugs.
		// fallback.requires_manifest_checksum surfaces weakened checksum
		// policy on upstream sources.
		repositoryDesiredBuildIDsResolve{},
		packageVersionAuthority{},
		dnsRecordsMatchRuntimeHealth{},
		fallbackRequiresManifestChecksum{},
		// WF-DEFER B3: surface workflow correlations that have been
		// auto-abandoned after hitting max_defers. Each is one operator
		// story (release.apply.package for keepalived, etc.) where the
		// underlying blocker has not converged across multiple defer
		// cycles and automatic retry has been suspended.
		workflowCorrelationAbandoned{},
		// Local package identity lane invariants:
		//   local_override_active  — WARN when any artifact in the repository
		//     carries a local/dev/hotfix version suffix (+local., -dev., -hotfix.).
		//   runtime_version_identity_lane — WARN when a node reports a
		//     local/dev/hotfix installed version but no matching active local
		//     override exists for that package/version.
		//   runtime_version_override_divergence — WARN when a node reports a
		//     different local runtime version/build_id than the active override.
		//   official_identity_sealed — ERROR when a checksum mismatch finding
		//     affects an official-publisher (core@globular.io) artifact, indicating
		//     that different bytes were silently stored under an official identity.
		runtimeVersionIdentityLane{},
		runtimeVersionOverrideDivergence{},
		localOverrideActive{},
		localOverrideStale{},
		officialIdentitySealed{},
		// Phase 9 (Diagnostic Honesty Refactor) wire-up. Consumes
		// Snapshot.VerifierResult populated by the collector's
		// runVerification step. Translates every verifier.Finding
		// (per-target + cross-cutting) into a doctor rules.Finding so
		// claim-vs-proof drift surfaces alongside every other invariant.
		runtimeVerification{},
		// Verdict coverage: every installed SERVICE-kind package must have
		// a verifier verdict in the current sweep. Fires when the catch-up
		// pass silently skips a service (e.g. ServiceRelease transiently
		// FAILED). Root cause of INC-2026-0008 (persistence UNVERIFIED
		// after platform-upgrade). Fixed in v1.2.87 by minimalTargetFromInstalled;
		// this invariant is the regression gate.
		verifierVerdictCoverage{},
		// Project O.5: regression gate for the Phase-1 WorkingDirectory
		// outage. Catches any future regression where a
		// `globular-*.service` ships a bare
		// `WorkingDirectory=/var/lib/globular/...` that would crash with
		// status=200/CHDIR if the dir is missing.
		systemdWorkingDirectoryMustBeOptional{},
		// Project S: backup-readiness gate. Fires when scylla-manager is
		// running but no Scylla cluster is registered with it — the
		// "running but unconfigured" state Project R recovered from.
		scyllaManagerClusterRegistered{},
		// Infrastructure truth plane (Phase 1): ScyllaDB config attestation,
		// loopback detection, stalled-join detection, peer convergence, and
		// "installed but no probe" — fed by the node-agent GetInfraProbe RPC.
		scyllaLoopbackForbidden{},
		scyllaConfigValid{},
		scyllaJoinNotStalled{},
		scyllaPeersMatchExpected{},
		scyllaProbeRequiredWhenInstalled{},
		// Infrastructure truth plane (Phase 2): etcd config attestation,
		// loopback detection, stalled-member detection (CORRUPT/critical config),
		// member convergence, and "installed but no probe" — fed by the node-agent
		// GetInfraProbe RPC. Distinct from etcdQuorumHealth/etcdStaleMember, which
		// are Prometheus/etcd-member-derived rather than truth-plane-derived.
		etcdInfraLoopbackForbidden{},
		etcdInfraConfigValid{},
		etcdInfraJoinNotStalled{},
		etcdInfraPeersMatchExpected{},
		etcdInfraProbeRequiredWhenInstalled{},
		// Infrastructure truth plane (Phase 3): MinIO topology attestation
		// (format.json blast-radius guard), loopback detection, split-brain
		// stalled-member detection, live write-quorum loss, and "installed but no
		// probe" — fed by the node-agent GetInfraProbe RPC. Distinct from the
		// objectstore* rules, which derive from desired topology + disk candidates.
		minioInfraLoopbackForbidden{},
		minioInfraTopologyMatchesDesired{},
		minioInfraConfigValid{},
		minioInfraNotStalled{},
		minioInfraWriteQuorum{},
		minioInfraProbeRequiredWhenInstalled{},
		// Infrastructure truth plane (Phase 4): Envoy data-plane bootstrap
		// attestation, the per-node admin-API-derived LDS wedge (CDS progressed but
		// LDS update_attempt==0), listeners-not-active, and "installed but no
		// probe". Diagnostic only — never auto-restarts Envoy (a restart deepens
		// the wedge). The LDS-wedge rule complements the cluster-scope,
		// Prometheus-derived envoyLDSWedge with direct per-node observation.
		envoyInfraConfigValid{},
		envoyInfraLDSProgress{},
		envoyInfraListenersActive{},
		envoyInfraProbeRequiredWhenInstalled{},
		// AI knowledge-base integrity: fires when ai-memory is running but
		// the operational-knowledge seed entries are absent (day-0 deferred
		// seed not yet applied). Auto-heals by seeding from the installed
		// awareness bundle at defaultOpsKnowledgeDir.
		opsKnowledgeSeedDeferred{},
		// awareness-graph RDF store empty: fires when the awareness-graph
		// service is reachable but returns zero triples. The embedded NT seed
		// Install-receipt authority drift (post sidecar retirement). Surfaces
		// the two states produced by node-agent's checkUnitHashDrift after the
		// 4-layer authority fix:
		//   unit_file_drift                      WARN (service still running,
		//                                             release pipeline heals)
		//   installed_state_missing_or_unproven  CRITICAL (no authority anywhere,
		//                                             fail-closed per
		//                                             state.unknown_must_not_default_to_healthy)
		// Legacy "hash_drift" is treated as an alias for unit_file_drift.
		unitReceiptDrift{},
		// Envoy data-plane LDS-wedge detection (Phase 28). Diagnostic only:
		// classifies the (cds.update_success > 0, lds.update_attempt == 0)
		// state as CRITICAL. Pins the invariant
		// envoy.lds_progress_required_for_http_mesh_readiness and detects
		// the failure_mode envoy.lds_update_attempt_zero_despite_cds_progress.
		// Does NOT restart envoy — auto-remediation would deepen the wedge
		// when the root cause is an upstream restart storm.
		envoyLDSWedge{},
	}
	// Append PENDING stubs
	r.invariants = append(r.invariants, pendingInvariants()...)
	return r
}

// EvaluateAll runs all invariants against the snapshot and returns all findings.
//
// When the snapshot is incomplete (snap.DataIncomplete is set because
// at least one sub-fetch errored during collection), every finding
// produced this cycle gets a reduced-harvest annotation: the Summary
// is prefixed with [reduced-harvest], and an Evidence row names the
// missing sources. This is the system-level enforcement of
// meta.harvest_and_yield_are_distinct_availability_dimensions —
// before this wrapper, individual rules silently treated partial
// snapshots as if complete and produced findings that named absence
// as drift when the absence was actually missing-data.
//
// Rules that need stricter behavior (refuse to evaluate at all when
// their specific source errored) call snap.HadError(service, rpc) at
// the top of Evaluate and return early.
func (r *Registry) EvaluateAll(snap *collector.Snapshot) []Finding {
	var all []Finding
	for _, inv := range r.invariants {
		all = append(all, inv.Evaluate(snap, r.cfg)...)
	}
	annotated := annotateForReducedHarvest(all, snap)
	// A rule whose only source errored produces NO finding, so the
	// [reduced-harvest] annotation above has nothing to tag and the masked
	// outage stays invisible. Surface each unavailable source as its own
	// INVARIANT_UNKNOWN finding so "could not see" is never indistinguishable
	// from "healthy".
	return stampEvidenceCollectionTime(append(annotated, snapshotSourceUnavailableFindings(snap)...), snap)
}

// stampEvidenceCollectionTime corrects each finding's evidence Timestamp from the
// rule-evaluation instant (kvEvidence stamps timestamppb.Now()) to the snapshot's
// actual data-collection time, so findingEvidenceTrust can detect staleness. The
// key case is a CACHED snapshot re-evaluated long after collection: every rule
// would otherwise re-stamp "now" and present stale data to the trust gate as
// authoritative (meta.binding_outlives_evidence_until_invalidated; OT-2). This
// re-arms the gate without changing any of the 150+ rule call sites.
//
// Prometheus evidence is dated by the scrape timestamp (snap.PromTS), which
// precedes snapshot generation. The correction is FAIL-SAFE: it only ever moves a
// timestamp BACKWARD (older / more conservative), never forward — so evidence that
// already carries an older real timestamp is left untouched.
func stampEvidenceCollectionTime(findings []Finding, snap *collector.Snapshot) []Finding {
	if snap == nil || snap.GeneratedAt.IsZero() {
		return findings
	}
	for i := range findings {
		for _, ev := range findings[i].Evidence {
			if ev == nil {
				continue
			}
			collected := snap.GeneratedAt
			if ev.GetSourceService() == "prometheus" && !snap.PromTS.IsZero() && snap.PromTS.Before(collected) {
				collected = snap.PromTS
			}
			if ev.Timestamp == nil || ev.Timestamp.AsTime().After(collected) {
				ev.Timestamp = timestamppb.New(collected)
			}
		}
	}
	return findings
}

// snapshotSourceUnavailableFindings emits one INVARIANT_UNKNOWN finding per
// collector sub-fetch that errored this sweep. It is the missing half of
// meta.harvest_and_yield_are_distinct_availability_dimensions: annotateForReducedHarvest
// tags findings that WERE produced, but a rule whose only data source errored
// produces no finding at all, so a real outage is silently masked (the
// FALSE_NEGATIVE class triaged 2026-06-09 — cert expiry, etcd quorum, service
// drift, etc. all going invisible exactly when their upstream is unreachable).
// Making each unavailable source a first-class finding restores the operator's
// "I could not see this" signal.
//
// These carry InvariantStatus=INVARIANT_UNKNOWN and a non-empty CheckError so
// aggregators never count them as FAILs (per the Finding.CheckError contract),
// and Severity=WARN so they surface without implying a confirmed failure.
func snapshotSourceUnavailableFindings(snap *collector.Snapshot) []Finding {
	if snap == nil || !snap.DataIncomplete {
		return nil
	}
	missing := snap.MissingSources()
	if len(missing) == 0 {
		return nil
	}

	// When many sources are unavailable (≥5), this is a systemic condition
	// (e.g. early bootstrap, Prometheus not scraped yet) rather than an
	// isolated outage. Collapse into a single summary finding to avoid a
	// 40+ entry warning storm that obscures actionable findings.
	const collapseThreshold = 5
	if len(missing) >= collapseThreshold {
		return []Finding{{
			FindingID:       FindingID("cluster_doctor.snapshot_source_unavailable", "bulk", strconv.Itoa(len(missing))),
			InvariantID:     "cluster_doctor.snapshot_source_unavailable",
			Severity:        cluster_doctorpb.Severity_SEVERITY_WARN,
			Category:        "observability",
			EntityRef:       "cluster",
			Summary:         strconv.Itoa(len(missing)) + " data sources unavailable this sweep — many checks are indeterminate, NOT healthy. This is expected during early bootstrap or when Prometheus has not scraped yet.",
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_UNKNOWN,
			CheckError:      "collector sub-fetch failed for " + strconv.Itoa(len(missing)) + " sources",
			Evidence: []*cluster_doctorpb.Evidence{
				kvEvidence("cluster_doctor", "snapshot_source_unavailable", map[string]string{
					"missing_sources":       strings.Join(missing, ", "),
					"missing_sources_count": strconv.Itoa(len(missing)),
					"explanation":           "multiple collector sub-fetches errored; rules whose only input is one of these sources emit no finding this sweep, so their verdicts are unknown rather than pass",
				}),
			},
		}}
	}

	var out []Finding
	for _, src := range missing {
		out = append(out, Finding{
			FindingID:       FindingID("cluster_doctor.snapshot_source_unavailable", src, ""),
			InvariantID:     "cluster_doctor.snapshot_source_unavailable",
			Severity:        cluster_doctorpb.Severity_SEVERITY_WARN,
			Category:        "observability",
			EntityRef:       src,
			Summary:         "cluster-doctor could not fetch " + src + " this sweep — checks that depend on it are indeterminate, NOT healthy",
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_UNKNOWN,
			CheckError:      "collector sub-fetch failed for " + src,
			Evidence: []*cluster_doctorpb.Evidence{
				kvEvidence("cluster_doctor", "snapshot_source_unavailable", map[string]string{
					"missing_source": src,
					"explanation":    "the collector's fetch for this source errored; any rule whose only input is this source emits no finding this sweep, so its verdict is unknown rather than pass",
				}),
			},
		})
	}
	return out
}

// annotateForReducedHarvest prepends [reduced-harvest] to each
// finding's Summary and appends an Evidence row naming the missing
// sources, IF the snapshot was incomplete. Returns findings unchanged
// when the snapshot was complete. Safe to call with nil/empty
// findings.
//
// The annotation is structurally visible to every consumer — CLI,
// dashboard, healer-gate, alert pipeline — so operators see "this
// finding was produced under reduced harvest" before treating its
// verdict as authoritative. The Evidence row carries the specific
// "service.rpc" pairs that failed so the operator can investigate
// the missing data alongside the finding.
//
// The function is package-private and operates only on the slice
// returned by Evaluate; rules cannot bypass it because the only
// public dispatch entry is EvaluateAll / EvaluateForNode.
func annotateForReducedHarvest(findings []Finding, snap *collector.Snapshot) []Finding {
	if snap == nil || !snap.DataIncomplete || len(findings) == 0 {
		return findings
	}
	missing := snap.MissingSources()
	missingList := strings.Join(missing, ", ")
	harvestEv := kvEvidence("cluster_doctor", "reduced_harvest", map[string]string{
		"missing_sources":       missingList,
		"missing_sources_count": strconv.Itoa(len(missing)),
		"explanation":           "snapshot data is incomplete because at least one collector sub-fetch errored; this finding's verdict is bounded by the data that was available",
	})
	for i := range findings {
		findings[i].Summary = "[reduced-harvest] " + findings[i].Summary
		findings[i].Evidence = append(findings[i].Evidence, harvestEv)
	}
	return findings
}

// EvaluateForNode runs all node-scoped invariants for the given node id.
func (r *Registry) EvaluateForNode(snap *collector.Snapshot, nodeID string) []Finding {
	// Build a single-node snapshot view. Cluster-scoped fields (critical keys,
	// ingress, objectstore, CA, schema guard) are shared read-only — copy them
	// so invariants that run in both "node" and "cluster" scope see full data.
	nodesnap := &collector.Snapshot{
		SnapshotID:                  snap.SnapshotID,
		GeneratedAt:                 snap.GeneratedAt,
		DataSources:                 snap.DataSources,
		DataIncomplete:              snap.DataIncomplete,
		DataErrors:                  snap.DataErrors,
		NodeHealths:                 snap.NodeHealths,
		Inventories:                 snap.Inventories,
		SubsystemHealth:             snap.SubsystemHealth,
		CertificateStatus:           snap.CertificateStatus,
		IntegrityReports:            snap.IntegrityReports,
		InfraProbes:                 snap.InfraProbes,
		InfraProbeCapabilityMissing: snap.InfraProbeCapabilityMissing,
		RuntimeProofs:               snap.RuntimeProofs,
		NodePackageKinds:            snap.NodePackageKinds,
		NodeDriftAge:                snap.NodeDriftAge,
		NodeRenderedGenerations:     snap.NodeRenderedGenerations,
		NodeRenderedFingerprints:    snap.NodeRenderedFingerprints,
		DiskCandidates:              snap.DiskCandidates,
		// Cluster-scoped state needed by invariants that also run per-node.
		CriticalKeyPresent:           snap.CriticalKeyPresent,
		CriticalKeyQueryError:        snap.CriticalKeyQueryError,
		CriticalKeyPolicyGaps:        snap.CriticalKeyPolicyGaps,
		IngressSpecPresent:           snap.IngressSpecPresent,
		IngressSpecLoadError:         snap.IngressSpecLoadError,
		IngressSpecRaw:               snap.IngressSpecRaw,
		IngressNodeStatus:            snap.IngressNodeStatus,
		ScyllaSchemaGuardStatus:      snap.ScyllaSchemaGuardStatus,
		DNSZoneReloadStatus:          snap.DNSZoneReloadStatus,
		ReconcileLaneStatus:          snap.ReconcileLaneStatus,
		ObjectStoreDesired:           snap.ObjectStoreDesired,
		ObjectStoreDesiredLoadError:  snap.ObjectStoreDesiredLoadError,
		ObjectStoreAppliedGeneration: snap.ObjectStoreAppliedGeneration,
		AppliedStateFingerprint:      snap.AppliedStateFingerprint,
		AppliedVolumesHash:           snap.AppliedVolumesHash,
		DesiredTopologyTransition:    snap.DesiredTopologyTransition,
		AdmittedDisks:                snap.AdmittedDisks,
		CAMetadata:                   snap.CAMetadata,
		DesiredBuildIDIndex:          snap.DesiredBuildIDIndex,
		DesiredVersionIndex:          snap.DesiredVersionIndex,
		RepositoryBuildIDIndex:       snap.RepositoryBuildIDIndex,
		RepositoryVersionIndex:       snap.RepositoryVersionIndex,
		RepositoryFindings:           snap.RepositoryFindings,
		RepositoryOperationalStatus:  snap.RepositoryOperationalStatus,
		RepositoryEndpointMissing:    snap.RepositoryEndpointMissing,
		StepOutcomes:                 snap.StepOutcomes,
		WorkflowSummaries:            snap.WorkflowSummaries,
		DriftUnresolved:              snap.DriftUnresolved,
		BlockedRuns:                  snap.BlockedRuns,
		AbandonedDeferCorrelations:   snap.AbandonedDeferCorrelations,
		PromMetrics:                  snap.PromMetrics,
		PromTS:                       snap.PromTS,
		KindMismatches:               snap.KindMismatches,
		LeaderPendingUpdate:          snap.LeaderPendingUpdate,
		ActiveLocalOverrides:         snap.ActiveLocalOverrides,
		DesiredServiceTargets:        snap.DesiredServiceTargets,
		VerifierResult:               snap.VerifierResult,
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
	annotated := annotateForReducedHarvest(all, nodesnap)
	return stampEvidenceCollectionTime(append(annotated, snapshotSourceUnavailableFindings(nodesnap)...), nodesnap)
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
