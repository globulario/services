package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/config"
	globular_service "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/installed_state"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"github.com/globulario/services/golang/workflow"
	"github.com/globulario/services/golang/workflow/engine"
	"github.com/globulario/services/golang/workflow/v1alpha1"
	"github.com/globulario/services/golang/workflow/workflowpb"
	"github.com/google/uuid"
)

// RunPackageReleaseWorkflow executes the release.apply.package workflow to
// roll out any package (SERVICE, INFRASTRUCTURE, WORKLOAD, COMMAND) across
// candidate nodes. The controller orchestrates; per-node steps call
// node-agents via gRPC.
func (srv *server) RunPackageReleaseWorkflow(ctx context.Context, releaseID, releaseName, pkgName, pkgKind, version, desiredHash string, candidateNodes []string) (*engine.Run, error) {
	defPath := resolveWorkflowDefinition("release.apply.package")
	if defPath == "" {
		return nil, fmt.Errorf("release.apply.package.yaml not found")
	}

	loader := v1alpha1.NewLoader()
	def, err := loader.LoadFile(defPath)
	if err != nil {
		return nil, fmt.Errorf("load workflow definition %s: %w", defPath, err)
	}

	router := engine.NewRouter()

	// Wire release controller actions with real implementations.
	engine.RegisterReleaseControllerActions(router, srv.buildReleaseControllerConfig(releaseName, pkgKind))

	// Wire node-agent actions — each callback resolves the node's agent
	// endpoint from the workflow's per-item inputs and calls via gRPC.
	engine.RegisterNodeDirectApplyActions(router, srv.buildNodeDirectApplyConfig())

	var wfRunID string // set after reportRunStart, captured by OnStepDone closure
	workflowName := def.Metadata.Name
	eng := &engine.Engine{
		Router: router,
		OnStepDone: func(run *engine.Run, step *engine.StepState) {
			elapsed := time.Duration(0)
			if !step.StartedAt.IsZero() && !step.FinishedAt.IsZero() {
				elapsed = step.FinishedAt.Sub(step.StartedAt)
			}
			log.Printf("release-workflow: step %s → %s (%s)",
				step.ID, step.Status, elapsed.Round(time.Millisecond))
			// Report step failures to the workflow service (fires event for ai-watcher).
			if step.Status == engine.StepFailed && wfRunID != "" {
				srv.reportStepFailed(wfRunID, step.ID, step.Error)
			}
			// Record per-step outcome for AI diagnostics.
			if srv.workflowRec != nil {
				srv.workflowRec.RecordStepOutcome(ctx, workflowName, step.ID,
					engineStepStatusToPB(step.Status),
					step.StartedAt, step.FinishedAt,
					"", step.Error)
			}
		},
	}

	nodesAny := make([]any, len(candidateNodes))
	for i, n := range candidateNodes {
		nodesAny[i] = n
	}

	inputs := map[string]any{
		"cluster_id":       srv.cfg.ClusterDomain,
		"release_id":       releaseID,
		"release_name":     releaseName,
		"package_name":     pkgName,
		"package_kind":     pkgKind,
		"resolved_version": version,
		"desired_hash":     desiredHash,
		"candidate_nodes":  nodesAny,
	}

	log.Printf("release-workflow: starting %s for release %s (%s:%s@%s) across %d nodes",
		def.Metadata.Name, releaseName, pkgKind, pkgName, version, len(candidateNodes))

	// Persist workflow run via recorder — release workflows are user-initiated
	// and rare, so full run detail is valuable for drill-down.
	componentKind := workflow.KindService
	if strings.EqualFold(pkgKind, "INFRASTRUCTURE") {
		componentKind = workflow.KindInfra
	}
	if srv.workflowRec != nil {
		wfRunID = srv.workflowRec.StartRun(ctx, &workflow.RunParams{
			ComponentName:    pkgName,
			ComponentKind:    componentKind,
			ComponentVersion: version,
			ReleaseKind:      "ServiceRelease",
			ReleaseObjectID:  releaseID,
			TriggerReason:    workflowpb.TriggerReason_TRIGGER_REASON_DESIRED_DRIFT,
			CorrelationID:    releaseID,
			WorkflowName:     workflowName,
		})
	}
	// Also publish legacy event for ai-watcher compatibility.
	srv.reportRunStart(pkgName, pkgKind, version, releaseID, len(candidateNodes))

	start := time.Now()
	run, err := eng.Execute(ctx, def, inputs)
	elapsed := time.Since(start)

	if err != nil {
		log.Printf("release-workflow: %s FAILED after %s: %v",
			releaseName, elapsed.Round(time.Millisecond), err)
		if wfRunID != "" && srv.workflowRec != nil {
			srv.workflowRec.FinishRun(ctx, wfRunID, workflow.Failed,
				fmt.Sprintf("%s FAILED after %s", releaseName, elapsed.Round(time.Millisecond)),
				err.Error(), workflowpb.FailureClass_FAILURE_CLASS_UNKNOWN)
		}
		srv.reportRunDone(wfRunID, pkgName, true,
			fmt.Sprintf("%s FAILED after %s: %v", releaseName, elapsed.Round(time.Millisecond), err))
	} else {
		succeeded := 0
		for _, st := range run.Steps {
			if st.Status == engine.StepSucceeded {
				succeeded++
			}
		}
		log.Printf("release-workflow: %s SUCCEEDED in %s (%d/%d steps)",
			releaseName, elapsed.Round(time.Millisecond), succeeded, len(run.Steps))
		if wfRunID != "" && srv.workflowRec != nil {
			srv.workflowRec.FinishRun(ctx, wfRunID, workflow.Succeeded,
				fmt.Sprintf("%s@%s SUCCEEDED in %s (%d/%d steps)",
					pkgName, version, elapsed.Round(time.Millisecond), succeeded, len(run.Steps)),
				"", workflow.NoFailure)
		}
		srv.reportRunDone(wfRunID, pkgName, false,
			fmt.Sprintf("%s@%s SUCCEEDED in %s (%d/%d steps)",
				pkgName, version, elapsed.Round(time.Millisecond), succeeded, len(run.Steps)))
	}

	return run, err
}

// RunInfraReleaseWorkflow executes the infrastructure-specific release workflow.
// Delegates to RunPackageReleaseWorkflow with kind=INFRASTRUCTURE.
func (srv *server) RunInfraReleaseWorkflow(ctx context.Context, releaseID, releaseName, pkgName, version, desiredHash string, candidateNodes []string) (*engine.Run, error) {
	return srv.RunPackageReleaseWorkflow(ctx, releaseID, releaseName, pkgName, "INFRASTRUCTURE", version, desiredHash, candidateNodes)
}

// RunRemovePackageWorkflow executes the release.remove.package workflow
// to uninstall a package from all target nodes.
func (srv *server) RunRemovePackageWorkflow(ctx context.Context, releaseID, pkgName, pkgKind string, candidateNodes []string) (*engine.Run, error) {
	defPath := resolveWorkflowDefinition("release.remove.package")
	if defPath == "" {
		return nil, fmt.Errorf("release.remove.package.yaml not found")
	}

	loader := v1alpha1.NewLoader()
	def, err := loader.LoadFile(defPath)
	if err != nil {
		return nil, fmt.Errorf("load workflow definition %s: %w", defPath, err)
	}

	router := engine.NewRouter()
	// releaseName == releaseID for remove workflow
	engine.RegisterReleaseControllerActions(router, srv.buildReleaseControllerConfig(releaseID, pkgKind))
	engine.RegisterNodeDirectApplyActions(router, srv.buildNodeDirectApplyConfig())

	var rmRunID string
	eng := &engine.Engine{
		Router: router,
		OnStepDone: func(run *engine.Run, step *engine.StepState) {
			elapsed := time.Duration(0)
			if !step.StartedAt.IsZero() && !step.FinishedAt.IsZero() {
				elapsed = step.FinishedAt.Sub(step.StartedAt)
			}
			log.Printf("remove-workflow: step %s → %s (%s)",
				step.ID, step.Status, elapsed.Round(time.Millisecond))
			if step.Status == engine.StepFailed && rmRunID != "" {
				srv.reportStepFailed(rmRunID, step.ID, step.Error)
			}
		},
	}

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

	log.Printf("remove-workflow: starting removal of %s (%s) across %d nodes",
		pkgName, pkgKind, len(candidateNodes))

	componentKind := workflow.KindService
	if strings.EqualFold(pkgKind, "INFRASTRUCTURE") {
		componentKind = workflow.KindInfra
	}
	if srv.workflowRec != nil {
		rmRunID = srv.workflowRec.StartRun(ctx, &workflow.RunParams{
			ComponentName:   pkgName,
			ComponentKind:   componentKind,
			ReleaseKind:     "ServiceRelease",
			ReleaseObjectID: releaseID,
			TriggerReason:   workflowpb.TriggerReason_TRIGGER_REASON_MANUAL,
			CorrelationID:   releaseID,
			WorkflowName:    "release.remove.package",
		})
	}
	srv.reportRunStart(pkgName, pkgKind, "", releaseID, len(candidateNodes))

	start := time.Now()
	run, err := eng.Execute(ctx, def, inputs)
	elapsed := time.Since(start)

	if err != nil {
		log.Printf("remove-workflow: %s FAILED after %s: %v",
			pkgName, elapsed.Round(time.Millisecond), err)
		if rmRunID != "" && srv.workflowRec != nil {
			srv.workflowRec.FinishRun(ctx, rmRunID, workflow.Failed,
				fmt.Sprintf("remove %s FAILED", pkgName), err.Error(), workflowpb.FailureClass_FAILURE_CLASS_UNKNOWN)
		}
		srv.reportRunDone(rmRunID, pkgName, true,
			fmt.Sprintf("remove %s FAILED: %v", pkgName, err))
	} else {
		log.Printf("remove-workflow: %s SUCCEEDED in %s",
			pkgName, elapsed.Round(time.Millisecond))
		if rmRunID != "" && srv.workflowRec != nil {
			srv.workflowRec.FinishRun(ctx, rmRunID, workflow.Succeeded,
				fmt.Sprintf("remove %s SUCCEEDED in %s", pkgName, elapsed.Round(time.Millisecond)),
				"", workflow.NoFailure)
		}
		srv.reportRunDone(rmRunID, pkgName, false,
			fmt.Sprintf("remove %s SUCCEEDED in %s", pkgName, elapsed.Round(time.Millisecond)))
	}

	return run, err
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

// patchReleasePhase updates the phase of a release resource (Service or Infrastructure).
func (srv *server) patchReleasePhase(ctx context.Context, resourceType, releaseName, newPhase, reason string) error {
	obj, _, err := srv.resources.Get(ctx, resourceType, releaseName)
	if err != nil || obj == nil {
		return fmt.Errorf("get %s %s: %w", resourceType, releaseName, err)
	}
	nowMs := time.Now().UnixMilli()

	switch rel := obj.(type) {
	case *cluster_controllerpb.ServiceRelease:
		if rel.Status == nil {
			rel.Status = &cluster_controllerpb.ServiceReleaseStatus{}
		}
		prev := rel.Status.Phase
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
// given node on the release.
func (srv *server) patchReleaseNodeStatus(ctx context.Context, resourceType, releaseName, nodeID string, update func(*cluster_controllerpb.NodeReleaseStatus)) error {
	obj, _, err := srv.resources.Get(ctx, resourceType, releaseName)
	if err != nil || obj == nil {
		return fmt.Errorf("get %s %s: %w", resourceType, releaseName, err)
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
			return srv.patchReleasePhase(ctx, resourceType, releaseName, cluster_controllerpb.ReleasePhaseFailed, reason)
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
			return srv.patchReleaseNodeStatus(ctx, resourceType, releaseName, nodeID, func(n *cluster_controllerpb.NodeReleaseStatus) {
				n.Phase = cluster_controllerpb.ReleasePhaseAvailable
				n.InstalledVersion = version
				n.UpdatedUnixMs = time.Now().UnixMilli()
				n.ErrorMessage = ""
			})
		},
		MarkNodeFailed: func(ctx context.Context, releaseID, nodeID, reason string) error {
			log.Printf("release-workflow: node %s FAILED for %s: %s", nodeID, releaseName, reason)
			return srv.patchReleaseNodeStatus(ctx, resourceType, releaseName, nodeID, func(n *cluster_controllerpb.NodeReleaseStatus) {
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
			// Check aggregate for any failures — if any node failed, mark release DEGRADED.
			// Otherwise, transition to AVAILABLE.
			finalPhase := cluster_controllerpb.ReleasePhaseAvailable
			if status, ok := aggregate["status"].(string); ok && status != "ok" {
				finalPhase = cluster_controllerpb.ReleasePhaseDegraded
			}
			return srv.patchReleasePhase(ctx, resourceType, releaseName, finalPhase, "")
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
}

// selectReleaseTargets filters candidate nodes: only include nodes that are
// bootstrap-ready and have the package's required profiles.
func (srv *server) selectReleaseTargets(ctx context.Context, candidates []any, pkgName, pkgKind, desiredHash string) ([]any, error) {
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
		if !isInfra && !bootstrapPhaseReady(node.BootstrapPhase) {
			log.Printf("release-workflow: skip node %s (bootstrap_phase=%s)", nodeID, node.BootstrapPhase)
			continue
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
		if pkg != nil {
			// The desired hash is a synthetic release hash (sha256 of metadata
			// like "core@globular.io/dns=0.0.1"), while the installed checksum
			// is the real file content hash ("sha256:abcdef..."). These are
			// different hash domains and cannot be compared directly.
			//
			// Recompute the synthetic hash from the installed version+publisher
			// and compare that to the desired hash. If they match, the node has
			// the correct version installed.
			installedVersion := pkg.GetVersion()
			installedBuild := pkg.GetBuildNumber()
			publisher := pkg.GetPublisherId()
			if publisher == "" {
				publisher = "core@globular.io"
			}
			var computedHash string
			if isInfra {
				computedHash = ComputeInfrastructureDesiredHash(publisher, pkgName, installedVersion, installedBuild)
			} else {
				computedHash = ComputeReleaseDesiredHash(publisher, pkgName, installedVersion, installedBuild, nil)
			}
			if desiredHash == "" || computedHash == desiredHash {
				log.Printf("release-workflow: skip node %s for %s (already installed v=%s)",
					ec.nodeID, pkgName, installedVersion)
				continue
			}
			log.Printf("release-workflow: node %s needs update for %s (installed_v=%s computed=%s desired=%s)",
				ec.nodeID, pkgName, installedVersion, computedHash, desiredHash)
		} else {
			log.Printf("release-workflow: node %s has no installed record for %s/%s", ec.nodeID, ec.installedKind, pkgName)
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
		InstallPackage: func(ctx context.Context, name, version, kind string) error {
			nc, _ := engine.GetNodeContext(ctx)
			nodeID, endpoint := nc.NodeID, nc.AgentEndpoint
			if endpoint == "" {
				return fmt.Errorf("no agent endpoint for node %s", nodeID)
			}

			log.Printf("release-workflow: installing %s@%s (%s) on node %s via %s", name, version, kind, nodeID, endpoint)
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

			conn, err := srv.dialNodeAgent(endpoint)
			if err != nil {
				return fmt.Errorf("connect to node %s: %w", nodeID, err)
			}
			defer conn.Close()

			unit := "globular-" + name + ".service"
			client := node_agentpb.NewNodeAgentServiceClient(conn)
			_, err = client.ControlService(ctx, &node_agentpb.ControlServiceRequest{
				Unit:   unit,
				Action: "restart",
			})
			if err != nil {
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

			conn, err := srv.dialNodeAgent(endpoint)
			if err != nil {
				return fmt.Errorf("connect to node %s: %w", nodeID, err)
			}
			defer conn.Close()

			unit := "globular-" + name + ".service"
			client := node_agentpb.NewNodeAgentServiceClient(conn)
			_, err = client.ControlService(ctx, &node_agentpb.ControlServiceRequest{
				Unit:   unit,
				Action: "restart",
			})
			if err != nil {
				return fmt.Errorf("restart %s on node %s: %w", unit, nodeID, err)
			}
			return nil
		},

		VerifyPackageRuntime: func(ctx context.Context, name, healthCheck string) error {
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

// --------------------------------------------------------------------------
// Workflow definition resolver
// --------------------------------------------------------------------------

var fetchControllerDefsDone int32 // atomic: 1 = successfully fetched

// resolveWorkflowDefinition finds a workflow YAML by name.
// On first miss it attempts to fetch all definitions from MinIO.
// Unlike sync.Once, retries on failure so transient MinIO unavailability
// during startup doesn't permanently prevent workflow resolution.
func resolveWorkflowDefinition(name string) string {
	candidates := []string{
		"/var/lib/globular/workflows/" + name + ".yaml",
		"/usr/lib/globular/workflows/" + name + ".yaml",
		"/tmp/" + name + ".yaml",
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	// Not found on disk — try fetching from MinIO (retry until success).
	if atomic.LoadInt32(&fetchControllerDefsDone) == 0 {
		fetchWorkflowDefsFromMinIO()
	}

	// Retry after fetch.
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

func fetchWorkflowDefsFromMinIO() {
	destDir := "/var/lib/globular/workflows"
	os.MkdirAll(destDir, 0o755)
	knownDefs := []string{
		"day0.bootstrap.yaml",
		"node.bootstrap.yaml",
		"node.join.yaml",
		"node.repair.yaml",
		"cluster.reconcile.yaml",
		"release.apply.package.yaml",
		"release.apply.infrastructure.yaml",
		"release.remove.package.yaml",
	}
	fetched := 0
	for _, defName := range knownDefs {
		key := "workflows/" + defName
		data, err := config.GetClusterConfig(key)
		if err != nil {
			log.Printf("workflow-resolver: fetch %s: %v", key, err)
			continue
		}
		if data == nil {
			continue
		}
		dest := filepath.Join(destDir, defName)
		if err := os.WriteFile(dest, data, 0o644); err != nil {
			log.Printf("workflow-resolver: write %s: %v", dest, err)
			continue
		}
		fetched++
	}
	if fetched > 0 {
		log.Printf("workflow-resolver: fetched %d workflow definitions from MinIO", fetched)
		atomic.StoreInt32(&fetchControllerDefsDone, 1)
	}
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
		"run_id":      runID,
		"release_id":  releaseID,
		"package":     pkgName,
		"kind":        pkgKind,
		"version":     version,
		"node_count":  nodeCount,
		"cluster":     srv.cfg.ClusterDomain,
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

