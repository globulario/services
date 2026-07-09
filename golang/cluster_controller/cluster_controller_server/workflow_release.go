package main

import (
	"context"
	"fmt"
	"log"
	"math"
	"strconv"
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	globular_service "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/identity"
	"github.com/globulario/services/golang/installed_state"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"github.com/globulario/services/golang/workflow/engine"
	"github.com/globulario/services/golang/workflow/workflowpb"
	"github.com/google/uuid"
)

const (
	waveStatePending   = "WAVE_PENDING"
	waveStateRunning   = "WAVE_RUNNING"
	waveStateCommitted = "WAVE_COMMITTED"
	waveStateBlocked   = "WAVE_BLOCKED"
)

// RunPackageReleaseWorkflow delegates execution of the release.apply.package
// workflow to the centralized WorkflowService. The controller orchestrates;
// per-node steps call node-agents via gRPC actor callbacks.
//
// resolvedEntrypointChecksum is the BINARY sha256 from the repository
// manifest's entrypoint_checksum. It flows through the workflow inputs into
// the install_package step's `expected_sha256` field and ultimately into
// ApplyPackageReleaseRequest.ExpectedSha256 — the node-agent verify gate
// refuses verified SUCCESS without it. An empty value is permitted only when
// the manifest itself has no checksum (legacy artifacts). NEVER synthesize.
func (srv *server) RunPackageReleaseWorkflow(ctx context.Context, releaseID, releaseName, pkgName, pkgKind, version, desiredHash, resolvedBuildID, resolvedEntrypointChecksum string, resolvedBuildNumber int64, candidateNodes []string, opts ...int64) (*workflowpb.ExecuteWorkflowResponse, error) {
	var dispatchGen int64
	if len(opts) > 0 {
		dispatchGen = opts[0]
	}
	router := engine.NewRouter()
	engine.RegisterReleaseControllerActions(router, srv.buildReleaseControllerConfigWithGen(releaseName, pkgKind, dispatchGen))
	engine.RegisterNodeDirectApplyActions(router, srv.buildNodeDirectApplyConfig())

	nodesAny := make([]any, len(candidateNodes))
	for i, n := range candidateNodes {
		nodesAny[i] = n
	}

	inputs := map[string]any{
		"cluster_id":                  srv.cfg.ClusterDomain,
		"release_id":                  releaseID,
		"release_name":                releaseName,
		"package_name":                pkgName,
		"package_kind":                pkgKind,
		"resolved_version":            version,
		"desired_hash":                desiredHash,
		"resolved_build_id":           resolvedBuildID,            // Phase 2: exact artifact identity
		"resolved_build_number":       resolvedBuildNumber,        // build_number passed to install_package step
		"resolved_entrypoint_checksum": resolvedEntrypointChecksum, // v1.2.119: BINARY sha256 for ExpectedSha256
		"candidate_nodes":             nodesAny,
		"max_parallel_nodes":          maxParallelNodesForKind(pkgKind),
	}

	correlationID := releaseID

	log.Printf("release-workflow: starting release.apply.package for %s (%s:%s@%s) across %d nodes",
		releaseName, pkgKind, pkgName, version, len(candidateNodes))
	parallel := maxParallelNodesForKind(pkgKind)
	if parallel < 1 {
		parallel = 1
	}
	totalWaves := int(math.Ceil(float64(len(candidateNodes)) / float64(parallel)))
	_ = srv.publishWaveState(ctx, releaseName, pkgKind, waveStatePending, parallel, len(candidateNodes), totalWaves, "")
	_ = srv.publishWaveState(ctx, releaseName, pkgKind, waveStateRunning, parallel, len(candidateNodes), totalWaves, "")

	// Publish legacy event for ai-watcher compatibility.
	srv.reportRunStart(pkgName, pkgKind, version, releaseID, len(candidateNodes))

	start := time.Now()
	resp, err := srv.executeWorkflowCentralized(ctx, "release.apply.package", correlationID, inputs, router)
	elapsed := time.Since(start)

	if err != nil {
		_ = srv.publishWaveState(ctx, releaseName, pkgKind, waveStateBlocked, parallel, len(candidateNodes), totalWaves, err.Error())
		log.Printf("release-workflow: %s FAILED after %s: %v",
			releaseName, elapsed.Round(time.Millisecond), err)
		srv.reportRunDone("", pkgName, true,
			fmt.Sprintf("%s FAILED after %s: %v", releaseName, elapsed.Round(time.Millisecond), err))
		return nil, err
	}

	failed := resp.Status != "SUCCEEDED"
	if failed {
		_ = srv.publishWaveState(ctx, releaseName, pkgKind, waveStateBlocked, parallel, len(candidateNodes), totalWaves, resp.Status)
	} else {
		_ = srv.publishWaveState(ctx, releaseName, pkgKind, waveStateCommitted, parallel, len(candidateNodes), totalWaves, "")
	}
	log.Printf("release-workflow: %s finished in %s: %s",
		releaseName, elapsed.Round(time.Millisecond), resp.Status)
	srv.reportRunDone(resp.RunId, pkgName, failed,
		fmt.Sprintf("%s@%s %s in %s", pkgName, version, resp.Status, elapsed.Round(time.Millisecond)))

	return resp, nil
}

// publishWaveState announces wave-level progress on the release record. The
// equality guard skips the etcd Apply when neither Message nor TransitionReason
// changed since the last announce: without it, repeated workflow runs with
// identical wave shapes (e.g. envoy restart-storm re-dispatching the same
// release) rewrite InfrastructureRelease/ServiceRelease records every call.
// On the bloated cluster captured in
// docs/awareness/reports/etcd_bloat_investigation_2026-06-03.md the envoy
// release accumulated ~99K MVCC versions from this writer alone.
func (srv *server) publishWaveState(ctx context.Context, releaseName, pkgKind, state string, maxParallelNodes, totalNodes, totalWaves int, note string) error {
	if isSyntheticReleaseName(releaseName) {
		return nil
	}
	resourceType := releaseResourceType(pkgKind)
	obj, _, err := srv.resources.Get(ctx, resourceType, releaseName)
	if err != nil || obj == nil {
		return err
	}
	baseMsg := fmt.Sprintf("%s max_parallel_nodes=%d total_nodes=%d total_waves=%d", state, maxParallelNodes, totalNodes, totalWaves)
	if note != "" {
		baseMsg += " note=" + note
	}
	switch rel := obj.(type) {
	case *cluster_controllerpb.ServiceRelease:
		if rel.Status == nil {
			rel.Status = &cluster_controllerpb.ServiceReleaseStatus{}
		}
		if rel.Status.Message == baseMsg && rel.Status.TransitionReason == state {
			return nil
		}
		rel.Status.Message = baseMsg
		rel.Status.TransitionReason = state
		rel.Status.LastTransitionUnixMs = time.Now().UnixMilli()
		_, err = srv.resources.Apply(ctx, resourceType, rel)
		return err
	case *cluster_controllerpb.InfrastructureRelease:
		if rel.Status == nil {
			rel.Status = &cluster_controllerpb.InfrastructureReleaseStatus{}
		}
		if rel.Status.Message == baseMsg {
			return nil
		}
		rel.Status.Message = baseMsg
		rel.Status.LastTransitionUnixMs = time.Now().UnixMilli()
		_, err = srv.resources.Apply(ctx, resourceType, rel)
		return err
	default:
		return fmt.Errorf("unexpected release type %T", obj)
	}
}

// RunInfraReleaseWorkflow executes the infrastructure-specific release workflow.
// Delegates to RunPackageReleaseWorkflow with kind=INFRASTRUCTURE.
func (srv *server) RunInfraReleaseWorkflow(ctx context.Context, releaseID, releaseName, pkgName, version, desiredHash, resolvedBuildID, resolvedEntrypointChecksum string, resolvedBuildNumber int64, candidateNodes []string) (*workflowpb.ExecuteWorkflowResponse, error) {
	return srv.RunPackageReleaseWorkflow(ctx, releaseID, releaseName, pkgName, "INFRASTRUCTURE", version, desiredHash, resolvedBuildID, resolvedEntrypointChecksum, resolvedBuildNumber, candidateNodes)
}

// RunRemovePackageWorkflow delegates execution of the release.remove.package
// workflow to the centralized WorkflowService.
func (srv *server) RunRemovePackageWorkflow(ctx context.Context, releaseID, pkgName, pkgKind string, candidateNodes []string) (*workflowpb.ExecuteWorkflowResponse, error) {
	router := engine.NewRouter()
	engine.RegisterReleaseControllerActions(router, srv.buildReleaseControllerConfig(releaseID, pkgKind))
	engine.RegisterNodeDirectApplyActions(router, srv.buildNodeDirectApplyConfig())

	nodesAny := make([]any, len(candidateNodes))
	for i, n := range candidateNodes {
		nodesAny[i] = n
	}

	inputs := map[string]any{
		"cluster_id":      srv.cfg.ClusterDomain,
		"release_id":      releaseID,
		"package_name":    pkgName,
		"package_kind":    pkgKind,
		"candidate_nodes": nodesAny,
	}

	correlationID := releaseID

	log.Printf("remove-workflow: starting removal of %s (%s) across %d nodes",
		pkgName, pkgKind, len(candidateNodes))
	srv.reportRunStart(pkgName, pkgKind, "", releaseID, len(candidateNodes))

	start := time.Now()
	resp, err := srv.executeWorkflowCentralized(ctx, "release.remove.package", correlationID, inputs, router)
	elapsed := time.Since(start)

	if err != nil {
		log.Printf("remove-workflow: %s FAILED after %s: %v",
			pkgName, elapsed.Round(time.Millisecond), err)
		srv.reportRunDone("", pkgName, true,
			fmt.Sprintf("remove %s FAILED: %v", pkgName, err))
		return nil, err
	}

	failed := resp.Status != "SUCCEEDED"
	log.Printf("remove-workflow: %s finished in %s: %s",
		pkgName, elapsed.Round(time.Millisecond), resp.Status)
	srv.reportRunDone(resp.RunId, pkgName, failed,
		fmt.Sprintf("remove %s %s in %s", pkgName, resp.Status, elapsed.Round(time.Millisecond)))

	return resp, nil
}

// --------------------------------------------------------------------------
// Controller action config (runs locally on controller)
// --------------------------------------------------------------------------

// maxParallelNodesForKind returns the maximum number of nodes that may be
// upgraded simultaneously for a given package kind. Infrastructure packages
// use serial rollout (1 node at a time) to preserve quorum — upgrading etcd,
// ScyllaDB, or MinIO in parallel risks dropping below the replication factor
// required for consensus. Service and workload packages use 2 as a safe
// default: one node serves traffic while another upgrades.
func maxParallelNodesForKind(kind string) int {
	if strings.ToUpper(kind) == "INFRASTRUCTURE" {
		return 1
	}
	return 2
}

// mergeSyncInstalledPackage is the pure read-modify-write step of the
// install workflow's sync_installed_state callback. It exists so the
// callback's "do not clobber the install receipt" contract is testable
// without an etcd dependency.
//
// Contract:
//
//   - The convergence committer is allowed to overwrite the cross-
//     validated identity fields (Version, Checksum, BuildId, Kind),
//     because those are exactly what `sync_installed_state` proves
//     across actors.
//
//   - Everything else MUST flow through from `existing` — most
//     critically Metadata, where the canonical install receipt lives
//     (installed_by, unit_file_sha256, binary_sha256, proof_*, …).
//     Pre-fix this callback built a fresh InstalledPackage{} with no
//     Metadata; CommitInstalledPackage marshalled that as-is, which
//     wiped the canonical receipt every install and caused
//     checkUnitHashDrift to fall back to the 4-key legacy_sidecar
//     migration shape. Live regression 2026-06-03, see
//     project_receipt_wipe_in_heartbeat.md.
//
//   - When existing is nil (truly first commit for this row), a
//     fresh package is constructed; metadata is then nil and the
//     install path's later writes will fill it.
func mergeSyncInstalledPackage(existing *node_agentpb.InstalledPackage, nodeID, name, version, hash, kind, buildID string) *node_agentpb.InstalledPackage {
	if existing == nil {
		existing = &node_agentpb.InstalledPackage{
			NodeId: nodeID,
			Name:   name,
			Kind:   kind,
		}
	}
	existing.Version = version
	existing.Checksum = hash
	existing.BuildId = buildID
	if kind != "" {
		existing.Kind = kind
	}
	return existing
}

// releaseResourceType returns the resource type for the release based on pkgKind.
// SERVICE/WORKLOAD/APPLICATION/COMMAND → ServiceRelease
// INFRASTRUCTURE → InfrastructureRelease
func releaseResourceType(pkgKind string) string {
	if strings.ToUpper(pkgKind) == "INFRASTRUCTURE" {
		return "InfrastructureRelease"
	}
	return "ServiceRelease"
}

// hostnameForNode resolves a node_id to a human-readable hostname from
// the in-memory state. Best-effort: returns "" if the node isn't found
// or state is nil. Used to populate workflow run records with a
// hostname operators can recognise at a glance.
func (srv *server) hostnameForNode(nodeID string) string {
	srv.lock("hostnameForNode")
	defer srv.unlock()
	if srv.state == nil {
		return ""
	}
	if n, ok := srv.state.Nodes[nodeID]; ok {
		return n.Identity.Hostname
	}
	return ""
}

// isSyntheticReleaseName reports whether releaseName was generated on the
// fly by the cluster.reconcile drift-dispatch loop (reconcile_actions.go)
// rather than being a persisted ServiceRelease/InfrastructureRelease
// object. Synthetic names carry the "reconcile-" prefix and have no
// backing resource in etcd — they exist only so the child release.apply
// workflow has a stable identifier for its step inputs.
//
// Status patches against synthetic releases are meaningless (there's
// nothing to patch), so the patch helpers treat them as no-ops rather
// than errors. Without this, every drift-triggered child workflow
// dies at its first step with "ServiceRelease reconcile-X: not found".
func isSyntheticReleaseName(releaseName string) bool {
	return strings.HasPrefix(releaseName, "reconcile-")
}

// patchReleasePhase updates the phase of a release resource (Service or Infrastructure).
func (srv *server) patchReleasePhase(ctx context.Context, resourceType, releaseName, newPhase, reason string) error {
	return srv.patchReleasePhaseGuarded(ctx, resourceType, releaseName, newPhase, reason, 0)
}

// patchReleasePhaseGuarded is like patchReleasePhase but skips the write if the
// release's generation has advanced past expectedGeneration (stale callback),
// or if this controller is no longer the leader (post-demotion callback).
func (srv *server) patchReleasePhaseGuarded(ctx context.Context, resourceType, releaseName, newPhase, reason string, expectedGeneration int64) error {
	if !srv.isLeader() {
		log.Printf("release-workflow: skip phase patch %s for %s (no longer leader)", newPhase, releaseName)
		return nil
	}
	if isSyntheticReleaseName(releaseName) {
		// Synthetic release from cluster.reconcile dispatch — nothing
		// to patch. Log so the transition is still observable.
		log.Printf("release-workflow: skip %s phase patch on synthetic release %s (reconcile-dispatch)", newPhase, releaseName)
		return nil
	}
	obj, _, err := srv.resources.Get(ctx, resourceType, releaseName)
	if err != nil {
		return fmt.Errorf("get %s %s: %w", resourceType, releaseName, err)
	}
	if obj == nil {
		return fmt.Errorf("get %s %s: not found", resourceType, releaseName)
	}

	// Generation guard: skip stale callback writes.
	if expectedGeneration > 0 {
		var currentGen int64
		switch rel := obj.(type) {
		case *cluster_controllerpb.ServiceRelease:
			if rel.Meta != nil {
				currentGen = rel.Meta.Generation
			}
		case *cluster_controllerpb.InfrastructureRelease:
			if rel.Meta != nil {
				currentGen = rel.Meta.Generation
			}
		}
		if currentGen > expectedGeneration {
			log.Printf("release-workflow: skip stale phase patch %s for %s (workflow gen=%d, current gen=%d)",
				newPhase, releaseName, expectedGeneration, currentGen)
			return nil
		}
	}

	nowMs := time.Now().UnixMilli()

	switch rel := obj.(type) {
	case *cluster_controllerpb.ServiceRelease:
		if rel.Status == nil {
			rel.Status = &cluster_controllerpb.ServiceReleaseStatus{}
		}
		prev := rel.Status.Phase
		// Idempotency: skip write if phase is already at the target value.
		// Duplicate callbacks must not trigger etcd watchers or reconcile storms.
		if prev == newPhase && reason == "" {
			return nil
		}
		rel.Status.Phase = newPhase
		rel.Status.LastTransitionUnixMs = nowMs
		if reason != "" {
			rel.Status.Message = reason
			rel.Status.TransitionReason = reason
		}
		if newPhase == cluster_controllerpb.ReleasePhaseFailed {
			if block, ok := classifyDeterministicBlock(reason); ok {
				rel.Status.BlockedReason = block.BlockedReason
				rel.Status.NextRetryUnixMs = 0
				rel.Status.TransitionReason = block.BlockedReason
				msg := "deterministic blocked failure; operator/state-change required"
				msg += ": failure_class=" + block.FailureClass + " reason_code=" + block.ReasonCode
				if len(block.UnblockSignals) > 0 {
					msg += " unblock_signals=[" + strings.Join(block.UnblockSignals, ",") + "]"
				}
				if block.MissingLibrary != "" {
					msg += " missing_library=" + block.MissingLibrary
					if block.Provider != "" {
						msg += " provider=" + block.Provider
					}
					if block.ManualAction != "" {
						msg += " manual_action=\"" + block.ManualAction + "\""
					}
				}
				msg += " auto_retry=false retry_after=null"
				rel.Status.Message = msg
			}
		}
		if prev != rel.Status.Phase {
			_ = srv.emitPhaseTransition(releaseName, prev, rel.Status.Phase, reason)
			if srv.workflowRec != nil {
				srv.workflowRec.RecordPhaseTransition(ctx, resourceType, releaseName,
					prev, rel.Status.Phase, reason, callerFunc(2), false)
			}
		}
		_, err = srv.resources.Apply(ctx, resourceType, rel)
		return err
	case *cluster_controllerpb.InfrastructureRelease:
		if rel.Status == nil {
			rel.Status = &cluster_controllerpb.InfrastructureReleaseStatus{}
		}
		prev := rel.Status.Phase
		if prev == newPhase && reason == "" {
			return nil
		}
		rel.Status.Phase = newPhase
		rel.Status.LastTransitionUnixMs = nowMs
		if reason != "" {
			rel.Status.Message = reason
		}
		if prev != rel.Status.Phase && srv.workflowRec != nil {
			srv.workflowRec.RecordPhaseTransition(ctx, resourceType, releaseName,
				prev, rel.Status.Phase, reason, callerFunc(2), false)
		}
		_, err = srv.resources.Apply(ctx, resourceType, rel)
		return err
	}
	return fmt.Errorf("unexpected type %T for %s %s", obj, resourceType, releaseName)
}

// patchReleaseNodeStatus updates (or inserts) a NodeReleaseStatus entry for the
// given node on the release. If expectedGeneration > 0, the write is skipped
// when the release's current generation has advanced (desired state changed
// mid-flight). This prevents stale workflow callbacks from overwriting a
// release that now targets a different version.
func (srv *server) patchReleaseNodeStatus(ctx context.Context, resourceType, releaseName, nodeID string, update func(*cluster_controllerpb.NodeReleaseStatus)) error {
	return srv.patchReleaseNodeStatusGuarded(ctx, resourceType, releaseName, nodeID, 0, update)
}

func (srv *server) patchReleaseNodeStatusGuarded(ctx context.Context, resourceType, releaseName, nodeID string, expectedGeneration int64, update func(*cluster_controllerpb.NodeReleaseStatus)) error {
	if !srv.isLeader() {
		log.Printf("release-workflow: skip node-status patch for %s node=%s (no longer leader)", releaseName, nodeID)
		return nil
	}
	if isSyntheticReleaseName(releaseName) {
		log.Printf("release-workflow: skip node-status patch on synthetic release %s (node=%s)", releaseName, nodeID)
		return nil
	}
	obj, _, err := srv.resources.Get(ctx, resourceType, releaseName)
	if err != nil {
		return fmt.Errorf("get %s %s: %w", resourceType, releaseName, err)
	}
	if obj == nil {
		return fmt.Errorf("get %s %s: not found", resourceType, releaseName)
	}

	// Generation guard: if the release's spec generation has advanced since
	// this workflow was dispatched, skip the write. The callback is from a
	// stale workflow targeting an old version.
	if expectedGeneration > 0 {
		var currentGen int64
		switch rel := obj.(type) {
		case *cluster_controllerpb.ServiceRelease:
			if rel.Meta != nil {
				currentGen = rel.Meta.Generation
			}
		case *cluster_controllerpb.InfrastructureRelease:
			if rel.Meta != nil {
				currentGen = rel.Meta.Generation
			}
		}
		if currentGen > expectedGeneration {
			log.Printf("release-workflow: skip stale node-status patch for %s node=%s (workflow gen=%d, current gen=%d)",
				releaseName, nodeID, expectedGeneration, currentGen)
			return nil
		}
	}

	var nodes *[]*cluster_controllerpb.NodeReleaseStatus
	switch rel := obj.(type) {
	case *cluster_controllerpb.ServiceRelease:
		if rel.Status == nil {
			rel.Status = &cluster_controllerpb.ServiceReleaseStatus{}
		}
		nodes = &rel.Status.Nodes
	case *cluster_controllerpb.InfrastructureRelease:
		if rel.Status == nil {
			rel.Status = &cluster_controllerpb.InfrastructureReleaseStatus{}
		}
		nodes = &rel.Status.Nodes
	default:
		return fmt.Errorf("unexpected type %T", obj)
	}

	// Find or insert node entry.
	var entry *cluster_controllerpb.NodeReleaseStatus
	for _, n := range *nodes {
		if n.NodeID == nodeID {
			entry = n
			break
		}
	}
	if entry == nil {
		entry = &cluster_controllerpb.NodeReleaseStatus{NodeID: nodeID}
		*nodes = append(*nodes, entry)
	}
	update(entry)

	_, err = srv.resources.Apply(ctx, resourceType, obj)
	return err
}

// buildReleaseControllerConfig returns a ReleaseControllerConfig with real,
// authoritative state mutations. releaseName is the resource name (Meta.Name)
// and pkgKind determines whether this is a ServiceRelease or InfrastructureRelease.
func (srv *server) buildReleaseControllerConfig(releaseName, pkgKind string) engine.ReleaseControllerConfig {
	return srv.buildReleaseControllerConfigWithGen(releaseName, pkgKind, 0)
}

// buildReleaseControllerConfigWithGen creates the config with a generation
// guard. Callbacks will skip writes if the release's generation has advanced
// past dispatchGeneration, preventing stale workflows from corrupting the
// release projection after desired state changes mid-flight.
func (srv *server) buildReleaseControllerConfigWithGen(releaseName, pkgKind string, dispatchGeneration int64) engine.ReleaseControllerConfig {
	resourceType := releaseResourceType(pkgKind)
	return engine.ReleaseControllerConfig{
		MarkReleaseResolved: func(ctx context.Context, relID string) error {
			log.Printf("release-workflow: mark %s RESOLVED", releaseName)
			return srv.patchReleasePhase(ctx, resourceType, releaseName, cluster_controllerpb.ReleasePhaseResolved, "")
		},
		MarkReleaseApplying: func(ctx context.Context, relID string) error {
			// No internal APPLYING phase write: the release stays in RESOLVED
			// while the workflow executes. Live "is-applying" is derived at
			// the API boundary from workflow run state (see isReleaseApplying).
			log.Printf("release-workflow: execution started for %s (release stays RESOLVED)", releaseName)
			return nil
		},
		MarkReleaseFailed: func(ctx context.Context, relID, reason string) error {
			log.Printf("release-workflow: mark %s FAILED: %s", releaseName, reason)
			return srv.patchReleasePhaseGuarded(ctx, resourceType, releaseName, cluster_controllerpb.ReleasePhaseFailed, reason, dispatchGeneration)
		},
		RecheckConvergence: func(ctx context.Context, relID string) error {
			log.Printf("release-workflow: recheck convergence for %s", releaseName)
			if srv.enqueueReconcile != nil {
				srv.enqueueReconcile()
			}
			return nil
		},
		SelectInfraTargets: func(ctx context.Context, candidates []any, pkgName, desiredHash string) ([]any, error) {
			return srv.selectReleaseTargets(ctx, candidates, pkgName, "", desiredHash)
		},
		SelectPackageTargets: func(ctx context.Context, candidates []any, pkgName, pkgKind, desiredHash string) ([]any, error) {
			return srv.selectReleaseTargets(ctx, candidates, pkgName, pkgKind, desiredHash)
		},
		FinalizeNoop: func(ctx context.Context, releaseID string) error {
			log.Printf("release-workflow: %s finalized AVAILABLE (no-op)", releaseName)
			return srv.patchReleasePhase(ctx, resourceType, releaseName, cluster_controllerpb.ReleasePhaseAvailable, "no targets required update")
		},
		MarkNodeStarted: func(ctx context.Context, releaseID, nodeID string) error {
			log.Printf("release-workflow: node %s started for %s", nodeID, releaseName)
			// No per-node APPLYING write: workflow run/step state is the live
			// source of truth for "which node is currently being applied".
			// We only bump the timestamp to record attempt start.
			return srv.patchReleaseNodeStatus(ctx, resourceType, releaseName, nodeID, func(n *cluster_controllerpb.NodeReleaseStatus) {
				n.UpdatedUnixMs = time.Now().UnixMilli()
			})
		},
		MarkNodeSucceeded: func(ctx context.Context, releaseID, nodeID, version, hash string) error {
			// CRITICAL — invariant install_package.hash_schemas_must_not_alias:
			// the `hash` parameter is the workflow input $.desired_hash, which
			// is the release-identity hash produced by ComputeReleaseDesiredHash
			// (publisher, name, version, build_number, config) — NOT a binary
			// SHA. NodeReleaseStatus.InstalledHash is declared on the proto as
			// "Phase 4: artifact sha256 verified at apply time" (binary SHA);
			// storing the release-identity here aliases two distinct hash
			// schemas into the same field and produces a permanent
			// rollout.installed_hash_mismatch when decideNodeRolloutProof later
			// compares it against ResolvedEntrypointChecksum.
			//
			// Authoritative source of the binary SHA at success-callback time:
			// rel.Status.ResolvedEntrypointChecksum (the manifest's
			// entrypoint_checksum that flowed through ApplyPackageRelease's
			// ExpectedSha256 — the same value the node-agent's verify gate
			// confirmed against the on-disk binary before marking installed).
			binarySHA := srv.lookupReleaseResolvedEntrypointChecksum(ctx, resourceType, releaseName)
			log.Printf("release-workflow: node %s succeeded for %s (v=%s binary_sha=%s workflow_release_identity=%s)",
				nodeID, releaseName, version, binarySHA, hash)
			return srv.patchReleaseNodeStatusGuarded(ctx, resourceType, releaseName, nodeID, dispatchGeneration, func(n *cluster_controllerpb.NodeReleaseStatus) {
				n.Phase = cluster_controllerpb.ReleasePhaseAvailable
				n.InstalledVersion = version
				n.UpdatedUnixMs = time.Now().UnixMilli()
				n.ErrorMessage = ""
				// Phase 4 hash: write the binary SHA the manifest declared and
				// the node-agent verified. Never the workflow's release-identity
				// `hash` — see comment block above. When the manifest has no
				// entrypoint_checksum (legacy artifacts), demote to inventory
				// claim rather than aliasing in a different-schema value.
				if binarySHA != "" {
					n.InstalledHash = binarySHA
					n.ProofStatus = RolloutProofInstalledVerified
					n.ProofFinding = ""
				} else {
					n.ProofStatus = RolloutProofInventoryClaim
					n.ProofFinding = ""
				}
			})
		},
		MarkNodeFailed: func(ctx context.Context, releaseID, nodeID, reason string) error {
			log.Printf("release-workflow: node %s FAILED for %s: %s", nodeID, releaseName, reason)
			return srv.patchReleaseNodeStatusGuarded(ctx, resourceType, releaseName, nodeID, dispatchGeneration, func(n *cluster_controllerpb.NodeReleaseStatus) {
				n.Phase = cluster_controllerpb.ReleasePhaseFailed
				n.ErrorMessage = reason
				n.UpdatedUnixMs = time.Now().UnixMilli()
			})
		},
		AggregateDirectApply: func(ctx context.Context, releaseID, pkgName string) (map[string]any, error) {
			return map[string]any{"release_id": releaseID, "package_name": pkgName, "status": "ok"}, nil
		},
		FinalizeDirectApply: func(ctx context.Context, releaseID string, aggregate map[string]any) error {
			log.Printf("release-workflow: finalize %s (aggregate=%v)", releaseName, aggregate)
			finalPhase := cluster_controllerpb.ReleasePhaseAvailable
			if status, ok := aggregate["status"].(string); ok && status != "ok" {
				finalPhase = cluster_controllerpb.ReleasePhaseDegraded
			}
			return srv.patchReleasePhaseGuarded(ctx, resourceType, releaseName, finalPhase, "", dispatchGeneration)
		},
	}
}

// buildGenericReleaseControllerConfig returns a ReleaseControllerConfig for the
// defaultRouter used during orphan run resumption. Unlike
// buildReleaseControllerConfigWithGen, closures derive the release name from
// the relID parameter rather than a captured variable, so the config works for
// any release without prior knowledge of its name or package kind.
// dispatchGeneration guards are disabled (0 = accept any generation) because
// the orphan scanner does not know the original dispatch generation.
func (srv *server) buildGenericReleaseControllerConfig() engine.ReleaseControllerConfig {
	return engine.ReleaseControllerConfig{
		MarkReleaseResolved: func(ctx context.Context, relID string) error {
			rt := srv.inferReleaseResourceType(ctx, relID)
			return srv.patchReleasePhase(ctx, rt, relID, cluster_controllerpb.ReleasePhaseResolved, "")
		},
		MarkReleaseApplying: func(ctx context.Context, relID string) error {
			return nil
		},
		MarkReleaseFailed: func(ctx context.Context, relID, reason string) error {
			rt := srv.inferReleaseResourceType(ctx, relID)
			return srv.patchReleasePhaseGuarded(ctx, rt, relID, cluster_controllerpb.ReleasePhaseFailed, reason, 0)
		},
		RecheckConvergence: func(ctx context.Context, relID string) error {
			if srv.enqueueReconcile != nil {
				srv.enqueueReconcile()
			}
			return nil
		},
		SelectInfraTargets: func(ctx context.Context, candidates []any, pkgName, desiredHash string) ([]any, error) {
			return srv.selectReleaseTargets(ctx, candidates, pkgName, "", desiredHash)
		},
		SelectPackageTargets: func(ctx context.Context, candidates []any, pkgName, pkgKind, desiredHash string) ([]any, error) {
			return srv.selectReleaseTargets(ctx, candidates, pkgName, pkgKind, desiredHash)
		},
		FinalizeNoop: func(ctx context.Context, relID string) error {
			rt := srv.inferReleaseResourceType(ctx, relID)
			return srv.patchReleasePhase(ctx, rt, relID, cluster_controllerpb.ReleasePhaseAvailable, "no targets required update")
		},
		MarkNodeStarted: func(ctx context.Context, relID, nodeID string) error {
			rt := srv.inferReleaseResourceType(ctx, relID)
			return srv.patchReleaseNodeStatus(ctx, rt, relID, nodeID, func(n *cluster_controllerpb.NodeReleaseStatus) {
				n.UpdatedUnixMs = time.Now().UnixMilli()
			})
		},
		MarkNodeSucceeded: func(ctx context.Context, relID, nodeID, version, hash string) error {
			rt := srv.inferReleaseResourceType(ctx, relID)
			// install_package.hash_schemas_must_not_alias — the workflow `hash`
			// parameter is the release-identity hash from $.desired_hash. Do not
			// store it in NodeReleaseStatus.InstalledHash, which the proto
			// declares as "artifact sha256 verified at apply time" (binary SHA).
			// Pull the binary SHA from rel.Status.ResolvedEntrypointChecksum,
			// the same manifest-entrypoint value the node-agent's verify gate
			// matched against on-disk bytes at apply time.
			binarySHA := srv.lookupReleaseResolvedEntrypointChecksum(ctx, rt, relID)
			return srv.patchReleaseNodeStatusGuarded(ctx, rt, relID, nodeID, 0, func(n *cluster_controllerpb.NodeReleaseStatus) {
				n.Phase = cluster_controllerpb.ReleasePhaseAvailable
				n.InstalledVersion = version
				n.UpdatedUnixMs = time.Now().UnixMilli()
				n.ErrorMessage = ""
				if binarySHA != "" {
					n.InstalledHash = binarySHA
					n.ProofStatus = RolloutProofInstalledVerified
					n.ProofFinding = ""
				} else {
					n.ProofStatus = RolloutProofInventoryClaim
					n.ProofFinding = ""
				}
			})
		},
		MarkNodeFailed: func(ctx context.Context, relID, nodeID, reason string) error {
			rt := srv.inferReleaseResourceType(ctx, relID)
			return srv.patchReleaseNodeStatusGuarded(ctx, rt, relID, nodeID, 0, func(n *cluster_controllerpb.NodeReleaseStatus) {
				n.Phase = cluster_controllerpb.ReleasePhaseFailed
				n.ErrorMessage = reason
				n.UpdatedUnixMs = time.Now().UnixMilli()
			})
		},
		AggregateDirectApply: func(ctx context.Context, relID, pkgName string) (map[string]any, error) {
			return map[string]any{"release_id": relID, "package_name": pkgName, "status": "ok"}, nil
		},
		FinalizeDirectApply: func(ctx context.Context, relID string, aggregate map[string]any) error {
			rt := srv.inferReleaseResourceType(ctx, relID)
			finalPhase := cluster_controllerpb.ReleasePhaseAvailable
			if status, ok := aggregate["status"].(string); ok && status != "ok" {
				finalPhase = cluster_controllerpb.ReleasePhaseDegraded
			}
			return srv.patchReleasePhaseGuarded(ctx, rt, relID, finalPhase, "", 0)
		},
	}
}

// inferReleaseResourceType checks whether relID is an InfrastructureRelease;
// falls back to ServiceRelease. Used by buildGenericReleaseControllerConfig
// where the package kind is unknown at construction time.
func (srv *server) inferReleaseResourceType(ctx context.Context, relID string) string {
	if obj, _, err := srv.resources.Get(ctx, "InfrastructureRelease", relID); err == nil && obj != nil {
		return "InfrastructureRelease"
	}
	return "ServiceRelease"
}

// lookupReleaseResolvedEntrypointChecksum returns the binary entrypoint SHA
// the controller resolved from the artifact manifest for a release. This is
// the value ApplyPackageRelease's ExpectedSha256 carried into the node-agent
// verify gate, so at MarkNodeSucceeded callback time it equals the on-disk
// binary SHA the node-agent matched before declaring installed.
//
// Returns "" when the release record cannot be loaded or carries no resolved
// value (legacy artifact, pre-bootstrap, etc). The caller MUST treat empty as
// "cannot prove binary identity" and write proof_status = inventory_claim —
// NEVER fall back to the workflow's release-identity hash, which lives in a
// different schema (publisher+name+version+build_number+config) and would
// alias incompatible values into NodeReleaseStatus.InstalledHash. See
// invariant: install_package.hash_schemas_must_not_alias.
func (srv *server) lookupReleaseResolvedEntrypointChecksum(ctx context.Context, resourceType, releaseName string) string {
	if srv == nil || srv.resources == nil {
		return ""
	}
	obj, _, err := srv.resources.Get(ctx, resourceType, releaseName)
	if err != nil || obj == nil {
		return ""
	}
	switch rel := obj.(type) {
	case *cluster_controllerpb.ServiceRelease:
		if rel.Status != nil {
			return strings.TrimSpace(rel.Status.ResolvedEntrypointChecksum)
		}
	case *cluster_controllerpb.InfrastructureRelease:
		if rel.Status != nil {
			return strings.TrimSpace(rel.Status.ResolvedEntrypointChecksum)
		}
	case *cluster_controllerpb.ApplicationRelease:
		if rel.Status != nil {
			return strings.TrimSpace(rel.Status.ResolvedEntrypointChecksum)
		}
	default:
		// Per meta.silence_is_not_valid_for_unexpected: a future
		// release type (e.g. ComputeRelease) silently producing "" here
		// would feed an empty resolved checksum into the dispatch path
		// with no operator-visible signal that the lookup never matched.
		log.Printf("resolvedEntrypointChecksumFor: unknown release type %T for resourceType=%s name=%s",
			rel, resourceType, releaseName)
	}
	return ""
}

// releaseTargetCandidate holds the subset of node state needed for target
// selection, snapshotted under the lock so that etcd I/O can happen without
// holding srv.mu.
type releaseTargetCandidate struct {
	nodeID        string
	agentEndpoint string
	installedKind string
	nodeSnapshot  nodeState
}

// selectReleaseTargets filters candidate nodes: only include nodes that are
// bootstrap-ready and have the package's required profiles.
// selectReleaseTargets's resolvedEntrypointChecksum (the Phase 38 binary
// proof) is looked up server-side from the release-status record at the
// classifyPackageConvergence call site, rather than threaded through every
// caller. This keeps the existing closure signatures stable while still
// catching the false-converged pattern at the workflow level.
func (srv *server) selectReleaseTargets(ctx context.Context, candidates []any, pkgName, pkgKind, desiredHash string, resolvedBuildID ...string) ([]any, error) {
	isInfra := strings.EqualFold(pkgKind, "INFRASTRUCTURE")
	catalogEntry := CatalogByName(pkgName)

	// Phase 1: snapshot node state under the lock — no I/O here.
	srv.lock("selectReleaseTargets")
	var eligible []releaseTargetCandidate
	for _, c := range candidates {
		nodeID := fmt.Sprint(c)
		node := srv.state.Nodes[nodeID]
		if node == nil {
			continue
		}

		// Skip nodes not yet approved — nothing deploys until join workflow starts.
		if node.BootstrapPhase == BootstrapAdmitted || node.BootstrapPhase == "" {
			log.Printf("release-workflow: skip node %s (bootstrap_phase=%s, not yet approved)", nodeID, node.BootstrapPhase)
			continue
		}
		// Workload/service releases skip nodes not yet bootstrap-ready.
		// Infrastructure releases target all nodes (they're what gets nodes ready).
		if !isInfra {
			isControlPlaneCritical := catalogEntry != nil && catalogEntry.ControlPlaneCritical
			if isControlPlaneCritical && !bootstrapInfraReady(node.BootstrapPhase) {
				log.Printf("release-workflow: skip node %s (bootstrap_phase=%s, infra not ready for control-plane-critical)", nodeID, node.BootstrapPhase)
				continue
			} else if !isControlPlaneCritical && !bootstrapPhaseReady(node.BootstrapPhase) {
				log.Printf("release-workflow: skip node %s (bootstrap_phase=%s)", nodeID, node.BootstrapPhase)
				continue
			}
		}

		// Profile filter.
		if catalogEntry != nil && len(catalogEntry.Profiles) > 0 {
			expanded := normalizeProfiles(node.Profiles)
			if !profilesOverlap(catalogEntry.Profiles, expanded) {
				log.Printf("release-workflow: skip node %s (profiles %v don't match %v)", nodeID, expanded, catalogEntry.Profiles)
				continue
			}
		}

		// Skip nodes that are active infrastructure members for this package.
		if isActiveInfraMember(node, pkgName) {
			log.Printf("release-workflow: SKIP node %s — active %s member (protected)", nodeID, pkgName)
			continue
		}

		installedKind := pkgKind
		if installedKind == "" {
			if catalogEntry != nil && catalogEntry.Kind == KindInfrastructure {
				installedKind = "INFRASTRUCTURE"
			} else {
				installedKind = "SERVICE"
			}
		}

		eligible = append(eligible, releaseTargetCandidate{
			nodeID:        nodeID,
			agentEndpoint: node.AgentEndpoint,
			installedKind: installedKind,
			nodeSnapshot:  *node,
		})
	}
	srv.unlock()

	// Resolve the desired build_id ONCE for this package's convergence check.
	//
	// The release-pipeline actor callbacks (SelectInfraTargets / SelectPackageTargets
	// in buildReleaseControllerConfig*) invoke selectReleaseTargets WITHOUT the
	// resolved build_id, so the variadic is empty on that path. classifyPackageConvergence
	// is called below with requireBuildID=true, so an empty desired build_id is (by
	// contract) treated as "missing desired build identity" → RepairRequired on EVERY
	// reconcile tick — even when installed_checksum already equals desired_hash and the
	// installed package carries a build_id. That false drift re-dispatches
	// release.apply.package each tick and restarts the service every pass. Observed live
	// on day-0 (2026-07-06): globular-workflow restarted every ~2min, dropping :10220
	// during each restart, so the controller's own workflow dispatches hit connection-
	// refused and tripped the dispatch circuit breaker (workflow.dispatch_circuit_open
	// CRITICAL + workflow.backend_pressure SUSTAINED).
	//
	// The sibling drift path (reconcileChooseWorkflow) already resolves this from the
	// pinned, immutable release Status via lookupServiceReleaseBuildID; the entrypoint
	// checksum is resolved the same self-lookup way below. Do the same here so the
	// release-pipeline path gets the convergence target it needs. This PRESERVES the
	// fail-closed requireBuildID contract — an explicitly-passed build_id still wins,
	// and a genuinely unresolved Status still yields empty (no regression, same as today).
	// See intent:desired.build_id_immutable_after_resolution.
	wantBuildID := ""
	if len(resolvedBuildID) > 0 {
		wantBuildID = strings.TrimSpace(resolvedBuildID[0])
	}
	if wantBuildID == "" {
		if rbid, _ := srv.lookupServiceReleaseBuildID(ctx, pkgName); strings.TrimSpace(rbid) != "" {
			wantBuildID = strings.TrimSpace(rbid)
		}
	}

	// Phase 2: check installed state via etcd WITHOUT holding srv.mu.
	// Initialize to empty slice (not nil) so that len(selected_targets)==0
	// evaluates correctly in the workflow engine's condition evaluator,
	// which treats nil as -1 (fail-closed) rather than 0.
	targets := []any{}
	for _, ec := range eligible {
		pkg, err := installed_state.GetInstalledPackage(ctx, ec.nodeID, ec.installedKind, pkgName)
		if err != nil {
			// Transient etcd read failure — we have no authoritative
			// view of this node's installed state. Skipping is the
			// scope-explicit choice: "we didn't check" is NOT "not
			// installed". The next reconcile tick will re-check.
			// Without this skip we passed pkg==nil to the classifier
			// which set RepairRequired=true and triggered an
			// unnecessary reinstall dispatch on every transient blip
			// (forbidden.silent_drop_on_partial_fetch +
			// meta.absence_scope_must_be_explicit).
			log.Printf("release-workflow: installed check %s/%s on %s: %v — skipping node this tick (will re-check)",
				ec.installedKind, pkgName, ec.nodeID, err)
			continue
		}
		// wantBuildID is resolved once above (from the explicit variadic arg or,
		// on the actor-callback path, the pinned release Status).
		// Phase 38: look up the resolved entrypoint checksum so the
		// convergence check can detect "buildId matches but the binary
		// on disk is wrong" — the lying-installed_state pattern caught
		// live on globule-ryzen 2026-06-03.
		wantEntrypoint := srv.lookupResolvedEntrypointChecksum(ctx, "core@globular.io", pkgName, ec.installedKind)
		convergence := classifyPackageConvergence(
			&ec.nodeSnapshot,
			pkgName,
			ec.installedKind,
			"",
			desiredHash,
			wantBuildID,
			wantEntrypoint,
			true, // build-backed release verdict — require build_id identity (no silent skip)
			pkg,
			time.Now(),
		)

		if pkg == nil {
			log.Printf("release-workflow: node %s has no installed record for %s/%s", ec.nodeID, ec.installedKind, pkgName)
		} else if !convergence.RepairRequired && (wantBuildID != "" || desiredHash != "") {
			log.Printf("release-workflow: skip node %s for %s (artifact+runtime converged: %s)", ec.nodeID, pkgName, convergence.Reason)
			continue
		} else if !convergence.RepairRequired && wantBuildID == "" && desiredHash == "" {
			log.Printf("release-workflow: skip node %s for %s (no convergence identity, runtime converged)", ec.nodeID, pkgName)
			continue
		} else {
			// Runtime-repair cooldown: don't redeliver same repair continuously.
			if convergence.VersionOK && convergence.HashOK && convergence.BuildIDOK && convergence.RuntimeNeeded && !convergence.RuntimeOK {
				cdKey := runtimeRepairCooldownKey(ec.nodeID, pkgName, ec.installedKind, "", desiredHash, wantBuildID)
				if ok, wait := shouldDispatchRuntimeRepair(cdKey, time.Now()); !ok {
					log.Printf("release-workflow: skip runtime repair for node=%s package=%s (%s), cooldown %s remaining",
						ec.nodeID, pkgName, convergence.Reason, wait.Round(time.Second))
					continue
				}
			}
			if pkg != nil {
				log.Printf("release-workflow: node %s needs update for %s (%s, installed_build_id=%s desired_build_id=%s installed_checksum=%s desired_hash=%s)",
					ec.nodeID, pkgName, convergence.Reason, pkg.GetBuildId(), wantBuildID, pkg.GetChecksum()[:min(16, len(pkg.GetChecksum()))], desiredHash)
			} else {
				log.Printf("release-workflow: node %s needs install for %s (%s)", ec.nodeID, pkgName, convergence.Reason)
			}
		}

		targets = append(targets, map[string]any{
			"node_id":        ec.nodeID,
			"agent_endpoint": ec.agentEndpoint,
		})
	}
	return targets, nil
}

// --------------------------------------------------------------------------
// Node-agent action config (calls node-agent via gRPC)
// --------------------------------------------------------------------------

func (srv *server) buildNodeDirectApplyConfig() engine.NodeDirectApplyConfig {
	return engine.NodeDirectApplyConfig{
		InstallPackage: func(ctx context.Context, name, version, kind, buildID, desiredHash, expectedSha256 string, buildNumber int64) error {
			nc, _ := engine.GetNodeContext(ctx)
			nodeID, endpoint := nc.NodeID, nc.AgentEndpoint
			if endpoint == "" {
				return fmt.Errorf("no agent endpoint for node %s", nodeID)
			}

			log.Printf("release-workflow: installing %s@%s+%d (%s) build_id=%s expected_sha256_present=%t on node %s via %s",
				name, version, buildNumber, kind, buildID, expectedSha256 != "", nodeID, endpoint)
			conn, _, err := srv.dialNodeAgentForNode(nodeID, endpoint)
			if err != nil {
				return fmt.Errorf("connect to node %s: %w", nodeID, err)
			}
			defer conn.Close()

			// HASH SCHEMA — two distinct hashes are propagated to the node-agent:
			//   desired_hash    = convergence identity (sha256 of metadata)
			//   expected_sha256 = BINARY sha256 from repository manifest.entrypoint_checksum
			//
			// expected_sha256 is what the node-agent verify gate compares against
			// the installed binary. It MUST come from manifest authority — never
			// from desired_hash, filename parsing, or runtime observation. See
			// invariant controller.apply_package_release_requires_manifest_checksum.
			client := node_agentpb.NewNodeAgentServiceClient(conn)
			resp, err := client.RunWorkflow(ctx, &node_agentpb.RunWorkflowRequest{
				WorkflowName: "install-package",
				Inputs: map[string]string{
					"package_name":    name,
					"version":         version,
					"kind":            kind,
					"build_id":        buildID,
					"build_number":    strconv.FormatInt(buildNumber, 10),
					"desired_hash":    desiredHash,
					"expected_sha256": expectedSha256,
				},
			})
			if err != nil {
				return fmt.Errorf("install %s on node %s: %w", name, nodeID, err)
			}
			if resp.GetStatus() != "SUCCEEDED" {
				return fmt.Errorf("install %s on node %s: %s", name, nodeID, resp.GetError())
			}
			return nil
		},

		VerifyPackageInstalled: func(ctx context.Context, name, version, hash string) error {
			nc, _ := engine.GetNodeContext(ctx)
			nodeID, endpoint := nc.NodeID, nc.AgentEndpoint
			if endpoint == "" {
				return fmt.Errorf("no agent endpoint for node %s", nodeID)
			}

			conn, _, err := srv.dialNodeAgentForNode(nodeID, endpoint)
			if err != nil {
				return fmt.Errorf("connect to node %s: %w", nodeID, err)
			}
			defer conn.Close()

			client := node_agentpb.NewNodeAgentServiceClient(conn)
			resp, err := client.GetInstalledPackage(ctx, &node_agentpb.GetInstalledPackageRequest{
				NodeId: nodeID,
				Name:   name,
			})
			if err != nil {
				return fmt.Errorf("verify %s on node %s: %w", name, nodeID, err)
			}
			pkg := resp.GetPackage()
			if pkg == nil {
				return fmt.Errorf("verify %s on node %s: package not found", name, nodeID)
			}
			if pkg.GetVersion() != version {
				return fmt.Errorf("verify %s on node %s: installed=%s want=%s", name, nodeID, pkg.GetVersion(), version)
			}
			return nil
		},

		RestartPackageService: func(ctx context.Context, name string) error {
			nc, _ := engine.GetNodeContext(ctx)
			nodeID, endpoint := nc.NodeID, nc.AgentEndpoint
			if endpoint == "" {
				return fmt.Errorf("no agent endpoint for node %s", nodeID)
			}

			unit := "globular-" + name + ".service"
			if err := srv.dedupRestart(ctx, nodeID, endpoint, unit); err != nil {
				return fmt.Errorf("restart %s on node %s: %w", unit, nodeID, err)
			}
			return nil
		},

		MaybeRestartPackage: func(ctx context.Context, name, kind, restartPolicy string) error {
			// COMMAND packages never need restart.
			if strings.EqualFold(kind, "COMMAND") {
				return nil
			}
			if strings.EqualFold(restartPolicy, "never") {
				return nil
			}
			// Skip restart for stateful infrastructure that manages its own lifecycle.
			// Restarting these mid-join kills in-progress Raft/gossip operations.
			// Also skip self-restart (controller would kill its own workflow).
			switch name {
			case "cluster-controller":
				log.Printf("release-workflow: skipping self-restart for cluster-controller")
				return nil
			case "scylladb", "etcd", "minio":
				log.Printf("release-workflow: skipping restart for %s (stateful infrastructure, self-managed lifecycle)", name)
				return nil
			}

			nc, _ := engine.GetNodeContext(ctx)
			nodeID, endpoint := nc.NodeID, nc.AgentEndpoint
			if endpoint == "" {
				return fmt.Errorf("no agent endpoint for node %s", nodeID)
			}

			unit := "globular-" + name + ".service"
			if err := srv.dedupRestart(ctx, nodeID, endpoint, unit); err != nil {
				return fmt.Errorf("restart %s on node %s: %w", unit, nodeID, err)
			}
			return nil
		},

		VerifyPackageRuntime: func(ctx context.Context, name, healthCheck string) error {
			// Skip runtime probes for command/binary-only packages that have no unit.
			if skipRuntimeCheck(name) || strings.TrimSpace(healthCheck) == "" {
				return nil
			}
			// Ingress-gated packages — keepalived is expected to be inactive
			// when the cluster's ingress spec is mode=disabled (Day-0
			// default). Without this gate the workflow's verify_runtime
			// step fails the active-check, defers up to max_defers, and
			// permanently abandons the release.apply.package run — even
			// though the inactive state is correct policy.
			if strings.EqualFold(strings.TrimSpace(name), "keepalived") && srv.ingressIsDisabled(ctx) {
				return nil
			}
			nc, _ := engine.GetNodeContext(ctx)
			nodeID, endpoint := nc.NodeID, nc.AgentEndpoint
			if endpoint == "" {
				return fmt.Errorf("no agent endpoint for node %s", nodeID)
			}

			conn, _, err := srv.dialNodeAgentForNode(nodeID, endpoint)
			if err != nil {
				return fmt.Errorf("connect to node %s: %w", nodeID, err)
			}
			defer conn.Close()

			unit := packageToUnit(name)
			client := node_agentpb.NewNodeAgentServiceClient(conn)
			resp, err := client.ControlService(ctx, &node_agentpb.ControlServiceRequest{
				Unit:   unit,
				Action: "status",
			})
			if err != nil {
				return fmt.Errorf("health check %s on node %s: %w", name, nodeID, err)
			}
			if resp.GetState() != "active" {
				return fmt.Errorf("health check %s on node %s: status=%s (want active)", name, nodeID, resp.GetState())
			}
			return nil
		},

		SyncInstalledPackage: func(ctx context.Context, name, version, hash, kind, buildID string) error {
			nc, _ := engine.GetNodeContext(ctx)
			nodeID := nc.NodeID
			if nodeID == "" {
				return fmt.Errorf("no node ID in context for sync installed state %s", name)
			}
			existing, _ := installed_state.GetInstalledPackage(ctx, nodeID, kind, name)
			pkg := mergeSyncInstalledPackage(existing, nodeID, name, version, hash, kind, buildID)
			return installed_state.CommitInstalledPackage(ctx, pkg)
		},

		// Removal actions
		StopPackageService: func(ctx context.Context, name string) error {
			nc, _ := engine.GetNodeContext(ctx)
			nodeID, endpoint := nc.NodeID, nc.AgentEndpoint
			if endpoint == "" {
				return fmt.Errorf("no agent endpoint for node %s", nodeID)
			}
			conn, _, err := srv.dialNodeAgentForNode(nodeID, endpoint)
			if err != nil {
				return fmt.Errorf("connect to node %s: %w", nodeID, err)
			}
			defer conn.Close()
			unit := "globular-" + name + ".service"
			client := node_agentpb.NewNodeAgentServiceClient(conn)
			_, err = client.ControlService(ctx, &node_agentpb.ControlServiceRequest{
				Unit:   unit,
				Action: "stop",
			})
			if err != nil {
				return fmt.Errorf("stop %s on node %s: %w", unit, nodeID, err)
			}
			return nil
		},

		DisablePackageService: func(ctx context.Context, name string) error {
			nc, _ := engine.GetNodeContext(ctx)
			nodeID, endpoint := nc.NodeID, nc.AgentEndpoint
			if endpoint == "" {
				return fmt.Errorf("no agent endpoint for node %s", nodeID)
			}
			conn, _, err := srv.dialNodeAgentForNode(nodeID, endpoint)
			if err != nil {
				return fmt.Errorf("connect to node %s: %w", nodeID, err)
			}
			defer conn.Close()
			unit := "globular-" + name + ".service"
			client := node_agentpb.NewNodeAgentServiceClient(conn)
			_, err = client.ControlService(ctx, &node_agentpb.ControlServiceRequest{
				Unit:   unit,
				Action: "disable",
			})
			if err != nil {
				return fmt.Errorf("disable %s on node %s: %w", unit, nodeID, err)
			}
			return nil
		},

		UninstallPackage: func(ctx context.Context, name, kind string) error {
			nc, _ := engine.GetNodeContext(ctx)
			nodeID, endpoint := nc.NodeID, nc.AgentEndpoint
			if endpoint == "" {
				return fmt.Errorf("no agent endpoint for node %s", nodeID)
			}
			conn, _, err := srv.dialNodeAgentForNode(nodeID, endpoint)
			if err != nil {
				return fmt.Errorf("connect to node %s: %w", nodeID, err)
			}
			defer conn.Close()
			// Use RunWorkflow to invoke the uninstall action on the node-agent.
			client := node_agentpb.NewNodeAgentServiceClient(conn)
			resp, err := client.RunWorkflow(ctx, &node_agentpb.RunWorkflowRequest{
				WorkflowName: "uninstall-package",
				Inputs: map[string]string{
					"package_name": name,
					"kind":         kind,
				},
			})
			if err != nil {
				return fmt.Errorf("uninstall %s on node %s: %w", name, nodeID, err)
			}
			if resp.GetStatus() != "SUCCEEDED" {
				return fmt.Errorf("uninstall %s on node %s: %s", name, nodeID, resp.GetError())
			}
			return nil
		},

		ClearInstalledPackageState: func(ctx context.Context, name, kind string) error {
			nc, _ := engine.GetNodeContext(ctx)
			nodeID := nc.NodeID
			if nodeID == "" {
				return fmt.Errorf("no node ID in context for clear state %s", name)
			}
			if err := installed_state.DeleteInstalledPackage(ctx, nodeID, kind, name); err != nil {
				return fmt.Errorf("clear state %s on node %s: %w", name, nodeID, err)
			}
			return nil
		},
	}
}

// --------------------------------------------------------------------------
// Package → systemd unit mapping
// --------------------------------------------------------------------------

// packageToUnit returns the systemd unit name for a package.
//
// Delegates to the identity registry so every component (CLI, node-agent,
// controller drift detector, VerifyPackageRuntime) resolves to the same
// unit name. Packages whose upstream unit doesn't follow the
// "globular-{name}.service" convention (e.g. keepalived → keepalived.service,
// scylladb → scylla-server.service) must have an entry in
// golang/identity/identity.go.
func packageToUnit(name string) string {
	if unit := identity.UnitForService(name); unit != "" {
		return unit
	}
	return "globular-" + name + ".service"
}

// ---------------------------------------------------------------------------
// Workflow event publishing — emits events directly to the event service
// (same bus the ai-watcher subscribes to). Fire-and-forget, never blocks.
//
// Events emitted:
//   workflow.release.started   — release workflow begins
//   workflow.release.succeeded — release workflow completed OK
//   workflow.release.failed    — release workflow failed
//   workflow.step.failed       — individual step failure (not every success)
// ---------------------------------------------------------------------------

// reportRunStart publishes a workflow.release.started event.
func (srv *server) reportRunStart(pkgName, pkgKind, version, releaseID string, nodeCount int) string {
	runID := uuid.New().String()
	go globular_service.PublishEvent("workflow.release.started", map[string]interface{}{
		"run_id":     runID,
		"release_id": releaseID,
		"package":    pkgName,
		"kind":       pkgKind,
		"version":    version,
		"node_count": nodeCount,
		"cluster":    srv.cfg.ClusterDomain,
	})
	return runID
}

// reportRunDone publishes workflow.release.succeeded or workflow.release.failed.
func (srv *server) reportRunDone(runID, pkgName string, failed bool, summary string) {
	topic := "workflow.release.succeeded"
	if failed {
		topic = "workflow.release.failed"
	}
	go globular_service.PublishEvent(topic, map[string]interface{}{
		"run_id":  runID,
		"package": pkgName,
		"summary": summary,
		"cluster": srv.cfg.ClusterDomain,
	})
}

// reportStepFailed publishes a workflow.step.failed event.
func (srv *server) reportStepFailed(runID, stepID, errMsg string) {
	go globular_service.PublishEvent("workflow.step.failed", map[string]interface{}{
		"run_id":  runID,
		"step_id": stepID,
		"error":   errMsg,
		"cluster": srv.cfg.ClusterDomain,
	})
}

// skipRuntimeCheck returns true for command-style packages without a long-running unit.
// These are binary-only tools installed to disk; they have no systemd service to probe.
// Packages listed here are treated as "always healthy" from a runtime perspective.
//
// Derived from the catalog so this list stays in sync as new COMMAND packages are added.
func skipRuntimeCheck(name string) bool {
	comp := CatalogByName(strings.ToLower(strings.TrimSpace(name)))
	return comp != nil && comp.Kind == KindCommand
}

// defaultRestartCooldown is the minimum time between two successful restarts
// of the same (node, unit). Chosen to cover the worst-case Envoy CDS+LDS init
// window (~3-5s for 26 dynamic clusters with SDS-wrapped mTLS) with margin,
// while still allowing legitimate retries — a real binary change ships through
// install_package which itself takes longer than this cooldown, so the
// post-install maybe_restart is never blocked.
const defaultRestartCooldown = 10 * time.Second

// restartCooldownWindow returns the per-server cooldown, falling back to the
// default. Test code can shorten this via srv.restartCooldown.
func (srv *server) restartCooldownWindow() time.Duration {
	if srv.restartCooldown > 0 {
		return srv.restartCooldown
	}
	return defaultRestartCooldown
}

// restartNow returns the current wall clock, routed through the test seam.
func (srv *server) restartNow() time.Time {
	if srv.testNow != nil {
		return srv.testNow()
	}
	return time.Now()
}

// dedupRestart guarantees idempotent restart dispatch per (node, unit) by:
//  1. coalescing concurrent in-flight callers (existing semantics — one
//     restart, all callers return when it completes), AND
//  2. suppressing rapid re-dispatch within restartCooldownWindow after a
//     successful restart (Phase 29 — defuses workflow restart-storms that
//     SIGTERM Envoy faster than it can finish CDS+LDS init; see
//     docs/awareness/reports/envoy_lds_cds_wedge.md).
//
// Failure path: a failed restart does NOT write to recentRestarts, so the
// next call is allowed immediately. The caller's own retry/backoff policy
// stays authoritative; this gate only defangs the post-success storm.
//
// Identity gate: a new desired version arrives through the workflow's
// install_package step (which takes >> cooldown), so the post-install
// maybe_restart is never blocked by the prior version's cooldown entry.
func (srv *server) dedupRestart(ctx context.Context, nodeID, endpoint, unit string) error {
	key := nodeID + "::" + unit

	// Step (2) — post-success cooldown. Checked BEFORE the in-flight
	// coalesce so a flurry of arrivals after a recent success returns nil
	// without queueing on an in-flight channel that doesn't exist.
	if v, ok := srv.recentRestarts.Load(key); ok {
		if last, isTime := v.(time.Time); isTime {
			elapsed := srv.restartNow().Sub(last)
			if elapsed < srv.restartCooldownWindow() {
				log.Printf(
					"workflow.restart_suppressed_duplicate node=%s unit=%s elapsed_ms=%d cooldown_ms=%d reason=recent_success",
					nodeID, unit, elapsed.Milliseconds(), srv.restartCooldownWindow().Milliseconds(),
				)
				return nil
			}
			// Beyond cooldown — drop the stale marker so the map doesn't
			// accumulate entries for long-idle units.
			srv.recentRestarts.Delete(key)
		}
	}

	// Step (1) — in-flight coalesce.
	done := make(chan struct{})
	if existing, loaded := srv.inflightRestarts.LoadOrStore(key, done); loaded {
		log.Printf("dedup: skip restart for %s on node %s (already in progress)", unit, nodeID)
		select {
		case <-existing.(chan struct{}):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	// We own this restart. Always clean up the in-flight marker. The
	// recentRestarts marker is written only on success (below) so a failed
	// restart can be retried immediately by the caller's backoff.
	defer func() {
		close(done)
		srv.inflightRestarts.Delete(key)
	}()

	conn, _, err := srv.dialNodeAgentForNode(nodeID, endpoint)
	if err != nil {
		return fmt.Errorf("connect to node %s: %w", nodeID, err)
	}
	defer conn.Close()

	client := node_agentpb.NewNodeAgentServiceClient(conn)
	_, err = client.ControlService(ctx, &node_agentpb.ControlServiceRequest{
		Unit:   unit,
		Action: "restart",
	})
	if err != nil {
		return err
	}

	// Record the successful restart for the cooldown gate. Stored under
	// the same key as the in-flight marker so the next caller's cooldown
	// check finds it on the very first lookup.
	srv.recentRestarts.Store(key, srv.restartNow())
	return nil
}
