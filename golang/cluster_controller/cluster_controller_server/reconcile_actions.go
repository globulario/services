package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/globulario/services/golang/installed_state"
	"github.com/globulario/services/golang/workflow/engine"
	"github.com/globulario/services/golang/workflow/v1alpha1"
	"github.com/globulario/services/golang/workflow/workflowpb"
)

// engineStepStatusToPB maps engine.StepStatus to the proto StepStatus enum.
// Used when recording per-step outcomes to the workflow service.
func engineStepStatusToPB(s engine.StepStatus) workflowpb.StepStatus {
	switch s {
	case engine.StepPending:
		return workflowpb.StepStatus_STEP_STATUS_PENDING
	case engine.StepRunning:
		return workflowpb.StepStatus_STEP_STATUS_RUNNING
	case engine.StepSucceeded:
		return workflowpb.StepStatus_STEP_STATUS_SUCCEEDED
	case engine.StepFailed:
		return workflowpb.StepStatus_STEP_STATUS_FAILED
	case engine.StepSkipped:
		return workflowpb.StepStatus_STEP_STATUS_SKIPPED
	}
	return workflowpb.StepStatus_STEP_STATUS_UNKNOWN
}

// buildReconcileControllerConfig returns the ReconcileControllerConfig that
// wires cluster.reconcile workflow actions to real controller state.
func (srv *server) buildReconcileControllerConfig() engine.ReconcileControllerConfig {
	return engine.ReconcileControllerConfig{
		AdvanceInfraJoins: srv.reconcileAdvanceInfraJoins,
		ScanDrift:         srv.reconcileScanDrift,
		ClassifyDrift:     srv.reconcileClassifyDrift,
		FinalizeClean:     srv.reconcileFinalizeClean,
		MarkItemStarted:   srv.reconcileMarkItemStarted,
		ChooseWorkflow:    srv.reconcileChooseWorkflow,
		MarkItemTerminal:  srv.reconcileMarkItemTerminal,
		MarkItemFailed:    srv.reconcileMarkItemFailed,
		AggregateResults:  srv.reconcileAggregateResults,
		Finalize:          srv.reconcileFinalize,
		MarkFailed:        srv.reconcileMarkFailed,
		EmitCompleted:     srv.reconcileEmitCompleted,
	}
}

// reconcileAdvanceInfraJoins drives the ScyllaDB/etcd/MinIO join-phase state
// machines and recovers any stuck bootstrap workflows. This is the explicit
// "orchestration" step — it advances nodes through their infrastructure join
// phases. Drift scanning happens in a separate step (scan_drift).
func (srv *server) reconcileAdvanceInfraJoins(ctx context.Context, clusterID string) error {
	srv.lock("reconcileAdvanceInfraJoins:snapshot")
	nodes := make([]*nodeState, 0, len(srv.state.Nodes))
	for _, node := range srv.state.Nodes {
		nodes = append(nodes, node)
	}
	srv.unlock()

	// Drive ScyllaDB join phases.
	if srv.scyllaMembers != nil {
		if dirty := srv.scyllaMembers.reconcileScyllaJoinPhases(ctx, nodes); dirty {
			srv.lock("reconcileAdvanceInfraJoins:scylla-persist")
			_ = srv.persistStateLocked(false)
			srv.unlock()
		}
	}

	// Drive etcd join phases.
	if srv.etcdMembers != nil {
		if dirty := srv.etcdMembers.reconcileEtcdJoinPhases(ctx, nodes); dirty {
			srv.lock("reconcileAdvanceInfraJoins:etcd-persist")
			_ = srv.persistStateLocked(false)
			srv.unlock()
		}
	}

	// Drive MinIO pool join phases.
	if srv.minioPoolMgr != nil {
		srv.lock("reconcileAdvanceInfraJoins:minio-snapshot")
		state := srv.state
		srv.unlock()
		if dirty := srv.minioPoolMgr.reconcileMinioJoinPhases(nodes, state); dirty {
			srv.lock("reconcileAdvanceInfraJoins:minio-persist")
			_ = srv.persistStateLocked(false)
			srv.unlock()
		}
	}

	// Recover bootstrap workflows that were interrupted by a controller restart.
	srv.recoverStuckBootstrapWorkflows(nodes, time.Now())

	log.Printf("reconcile-workflow: advance_infra_joins completed for %d nodes", len(nodes))
	return nil
}

// reconcileScanDrift scans the cluster for drift items that need remediation.
// It does NOT drive infra join state machines — that's the job of the
// preceding advance_infra_joins step. Returns the list of drift items.
func (srv *server) reconcileScanDrift(ctx context.Context, clusterID, scope string, includeNodes []any) ([]any, error) {
	srv.lock("reconcileScanDrift:snapshot")
	nodes := make([]*nodeState, 0, len(srv.state.Nodes))
	for _, node := range srv.state.Nodes {
		nodes = append(nodes, node)
	}
	srv.unlock()

	// Build the set of nodes to include (empty = all).
	includeSet := make(map[string]bool)
	for _, n := range includeNodes {
		includeSet[fmt.Sprint(n)] = true
	}

	var driftItems []any

	for _, node := range nodes {
		if node == nil || node.NodeID == "" {
			continue
		}
		if len(includeSet) > 0 && !includeSet[node.NodeID] {
			continue
		}
		// Only scan nodes that are past bootstrap.
		if !bootstrapPhaseReady(node.BootstrapPhase) {
			continue
		}

		// Probe infra health on nodes where join is verified.
		if node.AgentEndpoint != "" {
			if nodeHasProfile(&memberNode{Profiles: node.Profiles}, profilesForEtcd) && node.EtcdJoinPhase == EtcdJoinVerified {
				if !srv.probeEtcdHealth(ctx, node.AgentEndpoint) {
					driftItems = append(driftItems, map[string]any{
						"type":      "infra_unhealthy",
						"node_id":   node.NodeID,
						"component": "etcd",
						"endpoint":  node.AgentEndpoint,
						"hostname":  node.Identity.Hostname,
					})
				}
			}
			if nodeHasScyllaProfile(node) && node.ScyllaJoinPhase == ScyllaJoinVerified {
				if !srv.probeScyllaHealth(ctx, node.AgentEndpoint) {
					driftItems = append(driftItems, map[string]any{
						"type":      "infra_unhealthy",
						"node_id":   node.NodeID,
						"component": "scylladb",
						"endpoint":  node.AgentEndpoint,
						"hostname":  node.Identity.Hostname,
					})
				}
			}
			if nodeHasMinioProfile(node) && node.MinioJoinPhase == MinioJoinVerified {
				if !srv.probeMinioHealth(ctx, node.AgentEndpoint) {
					driftItems = append(driftItems, map[string]any{
						"type":      "infra_unhealthy",
						"node_id":   node.NodeID,
						"component": "minio",
						"endpoint":  node.AgentEndpoint,
						"hostname":  node.Identity.Hostname,
					})
				}
			}
		}

		// Check for version drift and missing packages against desired state.
		desiredCanon, _, err := srv.loadDesiredServices(ctx)
		if err != nil {
			log.Printf("reconcile-workflow: scan_drift: load desired services failed: %v", err)
			continue
		}

		// Scope desired to this node's resolved intent.
		intent, _ := ResolveNodeIntent(node.NodeID, node.Profiles, node.Units)
		desiredCanon = FilterDesiredByIntent(desiredCanon, intent)

		for svc, desiredVer := range desiredCanon {
			// Check installed state from etcd.
			pkg, err := installed_state.GetInstalledPackage(ctx, node.NodeID, "SERVICE", svc)
			if err != nil || pkg == nil {
				// Also try INFRASTRUCTURE kind.
				pkg, err = installed_state.GetInstalledPackage(ctx, node.NodeID, "INFRASTRUCTURE", svc)
			}
			if err != nil || pkg == nil {
				driftItems = append(driftItems, map[string]any{
					"type":            "missing_package",
					"node_id":         node.NodeID,
					"package_name":    svc,
					"desired_version": desiredVer,
					"hostname":        node.Identity.Hostname,
				})
				continue
			}
			if pkg.GetVersion() != desiredVer {
				driftItems = append(driftItems, map[string]any{
					"type":              "version_drift",
					"node_id":           node.NodeID,
					"package_name":      svc,
					"desired_version":   desiredVer,
					"installed_version": pkg.GetVersion(),
					"hostname":          node.Identity.Hostname,
				})
			}
		}

		// Check for unmanaged packages (installed but not desired).
		allInstalled, err := installed_state.ListAllNodes(ctx, "SERVICE", "")
		if err == nil {
			for _, pkg := range allInstalled {
				if pkg.GetNodeId() != node.NodeID {
					continue
				}
				canon := canonicalServiceName(pkg.GetName())
				if _, desired := desiredCanon[canon]; !desired && canon != "" {
					driftItems = append(driftItems, map[string]any{
						"type":         "unmanaged_package",
						"node_id":      node.NodeID,
						"package_name": canon,
						"version":      pkg.GetVersion(),
						"hostname":     node.Identity.Hostname,
					})
				}
			}
		}
	}

	log.Printf("reconcile-workflow: scan_drift found %d drift items across %d nodes", len(driftItems), len(nodes))
	return driftItems, nil
}

// reconcileClassifyDrift categorizes drift items by severity and type.
func (srv *server) reconcileClassifyDrift(ctx context.Context, driftReport []any, maxRemediations int) ([]any, error) {
	if len(driftReport) == 0 {
		return nil, nil
	}

	// Priority order: infra_unhealthy > missing_package > version_drift > unmanaged_package
	priority := map[string]int{
		"infra_unhealthy":   0,
		"missing_package":   1,
		"version_drift":     2,
		"unmanaged_package": 3,
	}

	// Sort by priority (stable relative order within same priority).
	type scored struct {
		item  map[string]any
		score int
	}
	var items []scored
	for _, raw := range driftReport {
		item, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		t := fmt.Sprint(item["type"])
		p, ok := priority[t]
		if !ok {
			p = 99
		}
		items = append(items, scored{item: item, score: p})
	}

	// Stable sort by priority.
	for i := 1; i < len(items); i++ {
		for j := i; j > 0 && items[j].score < items[j-1].score; j-- {
			items[j], items[j-1] = items[j-1], items[j]
		}
	}

	// Cap at maxRemediations.
	if maxRemediations > 0 && len(items) > maxRemediations {
		items = items[:maxRemediations]
	}

	result := make([]any, len(items))
	currentRefs := make(map[string]map[string]bool) // drift_type → entity_ref set
	for i, s := range items {
		s.item["priority"] = s.score
		result[i] = s.item

		// Record observation for AI diagnostics.
		dType := fmt.Sprint(s.item["type"])
		eRef := driftEntityRef(s.item)
		if dType != "" && eRef != "" {
			if currentRefs[dType] == nil {
				currentRefs[dType] = make(map[string]bool)
			}
			currentRefs[dType][eRef] = true
			if srv.workflowRec != nil {
				srv.workflowRec.RecordDriftObservation(ctx, dType, eRef, "", "")
			}
		}
	}

	// Opportunistic cleanup: any previously-tracked drift item NOT in the
	// current scan has been resolved and should be cleared. Fire in background
	// so classify_drift isn't delayed by telemetry bookkeeping.
	if srv.workflowRec != nil {
		go srv.clearResolvedDrift(context.Background(), currentRefs)
	}

	log.Printf("reconcile-workflow: classify_drift selected %d remediation items (max=%d)", len(result), maxRemediations)
	return result, nil
}

// driftEntityRef builds a stable identifier for a drift item so the telemetry
// layer can track its lifetime across reconcile cycles.
func driftEntityRef(item map[string]any) string {
	pkg := fmt.Sprint(item["package_name"])
	node := fmt.Sprint(item["node_id"])
	switch {
	case pkg != "" && node != "":
		return pkg + "@" + node
	case pkg != "":
		return pkg
	case node != "":
		return node
	}
	return ""
}

// clearResolvedDrift removes drift_unresolved rows for entities that no longer
// appear in the current drift scan. Runs in background on each classify_drift.
func (srv *server) clearResolvedDrift(ctx context.Context, current map[string]map[string]bool) {
	// Not currently read-capable from the recorder (no ListDriftUnresolved
	// client helper). Defer full implementation — stale rows will age out via
	// operator action or explicit ClearDriftObservation from remediation.
	_ = current
}

// reconcileFinalizeClean runs when no drift is found.
func (srv *server) reconcileFinalizeClean(ctx context.Context, clusterID string) error {
	log.Printf("reconcile-workflow: cluster %s is clean — no drift detected", clusterID)
	srv.emitClusterEvent("cluster.reconcile.clean", map[string]interface{}{
		"severity":   "INFO",
		"cluster_id": clusterID,
		"message":    "No drift detected",
	})
	return nil
}

// reconcileMarkItemStarted logs the start of a remediation item.
func (srv *server) reconcileMarkItemStarted(ctx context.Context, item map[string]any) error {
	log.Printf("reconcile-workflow: starting remediation: type=%s node=%s pkg=%s",
		item["type"], item["node_id"], item["package_name"])
	return nil
}

// reconcileChooseWorkflow selects the appropriate child workflow for a drift item.
func (srv *server) reconcileChooseWorkflow(ctx context.Context, item map[string]any) (map[string]any, error) {
	driftType := fmt.Sprint(item["type"])
	nodeID := fmt.Sprint(item["node_id"])
	pkgName := fmt.Sprint(item["package_name"])
	desiredVersion := fmt.Sprint(item["desired_version"])

	switch driftType {
	case "missing_package", "version_drift":
		kind := "SERVICE"
		if catalogEntry := CatalogByName(pkgName); catalogEntry != nil && catalogEntry.Kind == KindInfrastructure {
			kind = "INFRASTRUCTURE"
		}
		return map[string]any{
			"workflow_name": "release.apply.package",
			"inputs": map[string]any{
				"cluster_id":       srv.cfg.ClusterDomain,
				"release_id":       fmt.Sprintf("reconcile-%s-%s", nodeID, pkgName),
				"release_name":     fmt.Sprintf("reconcile-%s", pkgName),
				"package_name":     pkgName,
				"package_kind":     kind,
				"resolved_version": desiredVersion,
				"desired_hash":     "",
				"candidate_nodes":  []any{nodeID},
			},
		}, nil

	case "infra_unhealthy":
		component := fmt.Sprint(item["component"])
		log.Printf("reconcile-workflow: infra_unhealthy — %s on node %s (remediation deferred)", component, nodeID)
		// For now, just log. In the future this could trigger node.repair.
		return map[string]any{
			"workflow_name": "noop",
			"inputs": map[string]any{
				"reason":    fmt.Sprintf("infra_unhealthy: %s on %s", component, nodeID),
				"node_id":   nodeID,
				"component": component,
			},
		}, nil

	case "unmanaged_package":
		if !srv.enableServiceRemoval {
			log.Printf("reconcile-workflow: unmanaged package %s on %s — removal disabled", pkgName, nodeID)
			return map[string]any{
				"workflow_name": "noop",
				"inputs": map[string]any{
					"reason":       fmt.Sprintf("unmanaged: %s on %s (removal disabled)", pkgName, nodeID),
					"node_id":      nodeID,
					"package_name": pkgName,
				},
			}, nil
		}
		return map[string]any{
			"workflow_name": "release.remove.package",
			"inputs": map[string]any{
				"cluster_id":      srv.cfg.ClusterDomain,
				"release_id":      fmt.Sprintf("remove-%s-%s", nodeID, pkgName),
				"package_name":    pkgName,
				"package_kind":    "SERVICE",
				"candidate_nodes": []any{nodeID},
			},
		}, nil

	default:
		return map[string]any{
			"workflow_name": "noop",
			"inputs": map[string]any{
				"reason": fmt.Sprintf("unknown drift type: %s", driftType),
			},
		}, nil
	}
}

// reconcileMarkItemTerminal records the outcome of a child remediation.
func (srv *server) reconcileMarkItemTerminal(ctx context.Context, item, childResult map[string]any) error {
	status := "unknown"
	if childResult != nil {
		status = fmt.Sprint(childResult["status"])
	}
	log.Printf("reconcile-workflow: item terminal: type=%s node=%s pkg=%s child_status=%s",
		item["type"], item["node_id"], item["package_name"], status)
	// Clear the drift observation if remediation succeeded — the next scan
	// will re-observe it if it's still there.
	if status == "SUCCEEDED" && srv.workflowRec != nil {
		dType := fmt.Sprint(item["type"])
		eRef := driftEntityRef(item)
		if dType != "" && eRef != "" {
			srv.workflowRec.ClearDriftObservation(ctx, dType, eRef)
		}
	}
	return nil
}

// reconcileMarkItemFailed records a failed remediation item.
func (srv *server) reconcileMarkItemFailed(ctx context.Context, item map[string]any) error {
	log.Printf("reconcile-workflow: item FAILED: type=%s node=%s pkg=%s",
		item["type"], item["node_id"], item["package_name"])
	srv.emitClusterEvent("cluster.reconcile.item_failed", map[string]interface{}{
		"severity":     "WARN",
		"node_id":      fmt.Sprint(item["node_id"]),
		"package_name": fmt.Sprint(item["package_name"]),
		"drift_type":   fmt.Sprint(item["type"]),
	})
	return nil
}

// reconcileAggregateResults aggregates all remediation outcomes.
func (srv *server) reconcileAggregateResults(ctx context.Context) (map[string]any, error) {
	// In the future this could collect per-item results from outputs.
	// For now, return a simple status summary.
	return map[string]any{
		"status": "completed",
	}, nil
}

// reconcileFinalize finalizes the reconcile pass.
func (srv *server) reconcileFinalize(ctx context.Context, aggregate map[string]any) error {
	status := "unknown"
	if aggregate != nil {
		status = fmt.Sprint(aggregate["status"])
	}
	log.Printf("reconcile-workflow: finalized (status=%s)", status)
	srv.emitClusterEvent("cluster.reconcile.finalized", map[string]interface{}{
		"severity": "INFO",
		"status":   status,
	})
	return nil
}

// reconcileMarkFailed records a top-level reconcile failure.
func (srv *server) reconcileMarkFailed(ctx context.Context) error {
	log.Printf("reconcile-workflow: FAILED (top-level)")
	srv.emitClusterEvent("cluster.reconcile.failed", map[string]interface{}{
		"severity": "ERROR",
		"message":  "Cluster reconcile workflow failed",
	})
	return nil
}

// reconcileEmitCompleted emits a top-level completion event.
func (srv *server) reconcileEmitCompleted(ctx context.Context) error {
	log.Printf("reconcile-workflow: completed")
	srv.emitClusterEvent("cluster.reconcile.completed", map[string]interface{}{
		"severity": "INFO",
		"message":  "Cluster reconcile workflow completed",
	})
	return nil
}

// RunClusterReconcileWorkflow executes the cluster.reconcile workflow to
// detect drift and dispatch remediation workflows. This replaces the
// direct ScyllaDB/MinIO join-phase calls that were in reconcileNodes().
func (srv *server) RunClusterReconcileWorkflow(ctx context.Context) (*engine.Run, error) {
	defPath := resolveWorkflowDefinition("cluster.reconcile")
	if defPath == "" {
		return nil, fmt.Errorf("cluster.reconcile.yaml not found")
	}

	loader := v1alpha1.NewLoader()
	def, err := loader.LoadFile(defPath)
	if err != nil {
		return nil, fmt.Errorf("load workflow definition %s: %w", defPath, err)
	}

	router := engine.NewRouter()

	// Wire reconcile controller actions.
	engine.RegisterReconcileControllerActions(router, srv.buildReconcileControllerConfig())

	// Wire workflow-service actions for child workflow dispatch.
	engine.RegisterWorkflowServiceActions(router, engine.WorkflowServiceConfig{
		StartChild: func(ctx context.Context, workflowName string, inputs map[string]any) (string, error) {
			// For "noop" workflows, just return immediately.
			if workflowName == "noop" {
				reason := ""
				if inputs != nil {
					reason = fmt.Sprint(inputs["reason"])
				}
				log.Printf("reconcile-workflow: noop child: %s", reason)
				return "noop-run", nil
			}

			// Delegate to the real workflow runner.
			if workflowName == "release.apply.package" {
				releaseID := fmt.Sprint(inputs["release_id"])
				releaseName := fmt.Sprint(inputs["release_name"])
				pkgName := fmt.Sprint(inputs["package_name"])
				pkgKind := fmt.Sprint(inputs["package_kind"])
				version := fmt.Sprint(inputs["resolved_version"])
				desiredHash := fmt.Sprint(inputs["desired_hash"])
				candidates, _ := inputs["candidate_nodes"].([]any)
				candidateStrs := make([]string, len(candidates))
				for i, c := range candidates {
					candidateStrs[i] = fmt.Sprint(c)
				}

				run, err := srv.RunPackageReleaseWorkflow(ctx, releaseID, releaseName, pkgName, pkgKind, version, desiredHash, candidateStrs)
				if err != nil {
					return "", err
				}
				return run.ID, nil
			}

			if workflowName == "release.remove.package" {
				releaseID := fmt.Sprint(inputs["release_id"])
				pkgName := fmt.Sprint(inputs["package_name"])
				pkgKind := fmt.Sprint(inputs["package_kind"])
				candidates, _ := inputs["candidate_nodes"].([]any)
				candidateStrs := make([]string, len(candidates))
				for i, c := range candidates {
					candidateStrs[i] = fmt.Sprint(c)
				}

				run, err := srv.RunRemovePackageWorkflow(ctx, releaseID, pkgName, pkgKind, candidateStrs)
				if err != nil {
					return "", err
				}
				return run.ID, nil
			}

			return "", fmt.Errorf("unknown child workflow: %s", workflowName)
		},
		WaitChildTerminal: func(ctx context.Context, childRunID string) (map[string]any, error) {
			// Child workflows run synchronously in RunPackageReleaseWorkflow,
			// so by the time StartChild returns, the run is already terminal.
			if strings.HasPrefix(childRunID, "noop") {
				return map[string]any{"status": "SUCCEEDED", "run_id": childRunID}, nil
			}
			return map[string]any{"status": "SUCCEEDED", "run_id": childRunID}, nil
		},
	})

	// Condition evaluator for len() expressions.
	evalCond := func(ctx context.Context, expr string, inputs, outputs map[string]any) (bool, error) {
		if strings.Contains(expr, "len(remediation_items)") {
			items, _ := outputs["remediation_items"].([]any)
			if strings.Contains(expr, "== 0") {
				return len(items) == 0, nil
			}
			if strings.Contains(expr, "> 0") {
				return len(items) > 0, nil
			}
		}
		return true, nil
	}

	eng := &engine.Engine{
		Router:   router,
		EvalCond: evalCond,
		OnStepDone: func(run *engine.Run, step *engine.StepState) {
			log.Printf("reconcile-workflow: step %s -> %s", step.ID, step.Status)
			if srv.workflowRec != nil {
				srv.workflowRec.RecordStepOutcome(ctx, "cluster.reconcile", step.ID,
					engineStepStatusToPB(step.Status),
					step.StartedAt, step.FinishedAt,
					"", step.Error)
			}
		},
	}

	inputs := map[string]any{
		"cluster_id": srv.cfg.ClusterDomain,
		"scope":      "cluster",
	}

	log.Printf("reconcile-workflow: starting cluster.reconcile")
	startedAt := time.Now()
	run, err := eng.Execute(ctx, def, inputs)
	finishedAt := time.Now()

	// Summary-only persistence: cluster.reconcile fires every 30s, so writing
	// a full run + steps per cycle would inflate the workflow_runs table. We
	// record only the outcome into workflow_run_summaries (bounded O(1) rows).
	runID := ""
	if run != nil {
		runID = run.ID
	}
	outcomeStatus := workflowpb.RunStatus_RUN_STATUS_SUCCEEDED
	failureReason := ""
	if err != nil {
		outcomeStatus = workflowpb.RunStatus_RUN_STATUS_FAILED
		failureReason = err.Error()
		log.Printf("reconcile-workflow: cluster.reconcile FAILED: %v", err)
	} else {
		log.Printf("reconcile-workflow: cluster.reconcile completed")
	}
	if srv.workflowRec != nil {
		srv.workflowRec.RecordOutcome(ctx, "cluster.reconcile", runID,
			outcomeStatus, startedAt, finishedAt, failureReason)
	}

	return run, err
}
