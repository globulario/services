package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	globular_service "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/installed_state"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"github.com/globulario/services/golang/workflow/engine"
	"github.com/globulario/services/golang/workflow/workflowpb"
	"github.com/google/uuid"
)

// RunPackageReleaseWorkflow delegates execution of the release.apply.package
// workflow to the centralized WorkflowService. The controller orchestrates;
// per-node steps call node-agents via gRPC actor callbacks.
func (srv *server) RunPackageReleaseWorkflow(ctx context.Context, releaseID, releaseName, pkgName, pkgKind, version, desiredHash, resolvedBuildID string, candidateNodes []string, opts ...int64) (*workflowpb.ExecuteWorkflowResponse, error) {
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
		"cluster_id":        srv.cfg.ClusterDomain,
		"release_id":        releaseID,
		"release_name":      releaseName,
		"package_name":      pkgName,
		"package_kind":      pkgKind,
		"resolved_version":  version,
		"desired_hash":      desiredHash,
		"resolved_build_id": resolvedBuildID, // Phase 2: exact artifact identity
		"candidate_nodes":   nodesAny,
	}

	correlationID := releaseID

	log.Printf("release-workflow: starting release.apply.package for %s (%s:%s@%s) across %d nodes",
		releaseName, pkgKind, pkgName, version, len(candidateNodes))

	// Publish legacy event for ai-watcher compatibility.
	srv.reportRunStart(pkgName, pkgKind, version, releaseID, len(candidateNodes))

	start := time.Now()
	resp, err := srv.executeWorkflowCentralized(ctx, "release.apply.package", correlationID, inputs, router)
	elapsed := time.Since(start)

	if err != nil {
		log.Printf("release-workflow: %s FAILED after %s: %v",
			releaseName, elapsed.Round(time.Millisecond), err)
		srv.reportRunDone("", pkgName, true,
			fmt.Sprintf("%s FAILED after %s: %v", releaseName, elapsed.Round(time.Millisecond), err))
		return nil, err
	}

	failed := resp.Status != "SUCCEEDED"
	log.Printf("release-workflow: %s finished in %s: %s",
		releaseName, elapsed.Round(time.Millisecond), resp.Status)
	srv.reportRunDone(resp.RunId, pkgName, failed,
		fmt.Sprintf("%s@%s %s in %s", pkgName, version, resp.Status, elapsed.Round(time.Millisecond)))

	return resp, nil
}

// RunInfraReleaseWorkflow executes the infrastructure-specific release workflow.
// Delegates to RunPackageReleaseWorkflow with kind=INFRASTRUCTURE.
func (srv *server) RunInfraReleaseWorkflow(ctx context.Context, releaseID, releaseName, pkgName, version, desiredHash, resolvedBuildID string, candidateNodes []string) (*workflowpb.ExecuteWorkflowResponse, error) {
	return srv.RunPackageReleaseWorkflow(ctx, releaseID, releaseName, pkgName, "INFRASTRUCTURE", version, desiredHash, resolvedBuildID, candidateNodes)
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
			log.Printf("release-workflow: node %s succeeded for %s (v=%s h=%s)", nodeID, releaseName, version, hash)
			return srv.patchReleaseNodeStatusGuarded(ctx, resourceType, releaseName, nodeID, dispatchGeneration, func(n *cluster_controllerpb.NodeReleaseStatus) {
				n.Phase = cluster_controllerpb.ReleasePhaseAvailable
				n.InstalledVersion = version
				n.UpdatedUnixMs = time.Now().UnixMilli()
				n.ErrorMessage = ""
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

	// Phase 2: check installed state via etcd WITHOUT holding srv.mu.
	var targets []any
	for _, ec := range eligible {
		pkg, err := installed_state.GetInstalledPackage(ctx, ec.nodeID, ec.installedKind, pkgName)
		if err != nil {
			log.Printf("release-workflow: installed check %s/%s on %s: %v", ec.installedKind, pkgName, ec.nodeID, err)
		}
		wantBuildID := ""
		if len(resolvedBuildID) > 0 {
			wantBuildID = resolvedBuildID[0]
		}
		convergence := classifyPackageConvergence(
			&ec.nodeSnapshot,
			pkgName,
			ec.installedKind,
			"",
			desiredHash,
			wantBuildID,
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
		InstallPackage: func(ctx context.Context, name, version, kind, buildID string) error {
			nc, _ := engine.GetNodeContext(ctx)
			nodeID, endpoint := nc.NodeID, nc.AgentEndpoint
			if endpoint == "" {
				return fmt.Errorf("no agent endpoint for node %s", nodeID)
			}

			log.Printf("release-workflow: installing %s@%s (%s) build_id=%s on node %s via %s", name, version, kind, buildID, nodeID, endpoint)
			conn, err := srv.dialNodeAgent(endpoint)
			if err != nil {
				return fmt.Errorf("connect to node %s: %w", nodeID, err)
			}
			defer conn.Close()

			client := node_agentpb.NewNodeAgentServiceClient(conn)
			resp, err := client.RunWorkflow(ctx, &node_agentpb.RunWorkflowRequest{
				WorkflowName: "install-package",
				Inputs: map[string]string{
					"package_name": name,
					"version":      version,
					"kind":         kind,
					"build_id":     buildID,
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

			conn, err := srv.dialNodeAgent(endpoint)
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
			nc, _ := engine.GetNodeContext(ctx)
			nodeID, endpoint := nc.NodeID, nc.AgentEndpoint
			if endpoint == "" {
				return fmt.Errorf("no agent endpoint for node %s", nodeID)
			}

			conn, err := srv.dialNodeAgent(endpoint)
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

		SyncInstalledPackage: func(ctx context.Context, name, version, hash, kind string) error {
			nc, _ := engine.GetNodeContext(ctx)
			nodeID, endpoint := nc.NodeID, nc.AgentEndpoint
			if endpoint == "" {
				return fmt.Errorf("no agent endpoint for node %s", nodeID)
			}

			conn, err := srv.dialNodeAgent(endpoint)
			if err != nil {
				return fmt.Errorf("connect to node %s: %w", nodeID, err)
			}
			defer conn.Close()

			client := node_agentpb.NewNodeAgentServiceClient(conn)
			_, err = client.SetInstalledPackage(ctx, &node_agentpb.SetInstalledPackageRequest{
				Package: &node_agentpb.InstalledPackage{
					NodeId:   nodeID,
					Name:     name,
					Version:  version,
					Checksum: hash,
					Kind:     kind,
				},
			})
			if err != nil {
				return fmt.Errorf("sync installed state %s on node %s: %w", name, nodeID, err)
			}
			return nil
		},

		// Removal actions
		StopPackageService: func(ctx context.Context, name string) error {
			nc, _ := engine.GetNodeContext(ctx)
			nodeID, endpoint := nc.NodeID, nc.AgentEndpoint
			if endpoint == "" {
				return fmt.Errorf("no agent endpoint for node %s", nodeID)
			}
			conn, err := srv.dialNodeAgent(endpoint)
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
			conn, err := srv.dialNodeAgent(endpoint)
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
			conn, err := srv.dialNodeAgent(endpoint)
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
			nodeID, endpoint := nc.NodeID, nc.AgentEndpoint
			if endpoint == "" {
				return fmt.Errorf("no agent endpoint for node %s", nodeID)
			}
			conn, err := srv.dialNodeAgent(endpoint)
			if err != nil {
				return fmt.Errorf("connect to node %s: %w", nodeID, err)
			}
			defer conn.Close()
			// Clear the installed-state entry by setting an empty package.
			client := node_agentpb.NewNodeAgentServiceClient(conn)
			_, err = client.SetInstalledPackage(ctx, &node_agentpb.SetInstalledPackageRequest{
				Package: &node_agentpb.InstalledPackage{
					NodeId:  nodeID,
					Name:    name,
					Kind:    kind,
					Status:  "removed",
					Version: "",
				},
			})
			if err != nil {
				return fmt.Errorf("clear state %s on node %s: %w", name, nodeID, err)
			}
			return nil
		},
	}
}

// --------------------------------------------------------------------------
// Package → systemd unit mapping
// --------------------------------------------------------------------------

// packageUnitOverrides maps package names to their actual systemd unit names
// for packages that don't follow the "globular-{name}.service" convention.
var packageUnitOverrides = map[string]string{
	"scylladb":             "scylla-server.service",
	"scylla-manager":       "globular-scylla-manager.service",
	"scylla-manager-agent": "globular-scylla-manager-agent.service",
}

// packageToUnit returns the systemd unit name for a package.
func packageToUnit(name string) string {
	if unit, ok := packageUnitOverrides[name]; ok {
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
// Expanding this list here also gates serviceHealthyForRelease — a package listed here
// is treated as "always healthy" from a runtime perspective.
func skipRuntimeCheck(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "restic", "rclone", "ffmpeg", "sctool", "mc",
		// Utility binaries published under ServiceRelease (kind=SERVICE, systemd=none).
		// They have no systemd unit so serviceHealthyForRelease would always return
		// false, causing permanent false-DEGRADED drift on every reconcile cycle.
		"etcdctl", "sha256sum", "yt-dlp":
		return true
	}
	return false
}

// dedupRestart ensures only one restart is in progress for a given (node, unit)
// pair. If another goroutine is already restarting the same unit on the same
// node, this call waits for it to complete and returns nil (the restart already
// happened). This prevents systemd start-limit-hit from concurrent workflow
// restart storms.
func (srv *server) dedupRestart(ctx context.Context, nodeID, endpoint, unit string) error {
	key := nodeID + "::" + unit
	done := make(chan struct{})

	if existing, loaded := srv.inflightRestarts.LoadOrStore(key, done); loaded {
		// Another restart is already in progress — wait for it.
		log.Printf("dedup: skip restart for %s on node %s (already in progress)", unit, nodeID)
		select {
		case <-existing.(chan struct{}):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	// We own this restart. Execute and clean up.
	defer func() {
		close(done)
		srv.inflightRestarts.Delete(key)
	}()

	conn, err := srv.dialNodeAgent(endpoint)
	if err != nil {
		return fmt.Errorf("connect to node %s: %w", nodeID, err)
	}
	defer conn.Close()

	client := node_agentpb.NewNodeAgentServiceClient(conn)
	_, err = client.ControlService(ctx, &node_agentpb.ControlServiceRequest{
		Unit:   unit,
		Action: "restart",
	})
	return err
}
