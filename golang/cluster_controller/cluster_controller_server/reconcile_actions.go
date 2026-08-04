package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/installed_state"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"github.com/globulario/services/golang/workflow/engine"
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
		membership := srv.snapshotClusterMembership()
		if membership != nil {
			if ok, reason := shouldPruneEtcdStaleMembers(time.Now(), nodes); ok {
				desiredEtcdNodes := filterNodesByProfile(membership, profilesForEtcd)
				if err := srv.etcdMembers.removeStaleMembers(ctx, desiredEtcdNodes); err != nil {
					log.Printf("reconcile-workflow: etcd stale-member prune failed: %v", err)
				}
			} else {
				log.Printf("reconcile-workflow: skipping etcd stale-member prune (safety gate): %s", reason)
			}
		}
		if dirty := srv.etcdMembers.reconcileEtcdJoinPhases(ctx, nodes); dirty {
			srv.lock("reconcileAdvanceInfraJoins:etcd-persist")
			_ = srv.persistStateLocked(false)
			srv.unlock()
		}
		// Auto-rejoin nodes that were permanently removed: call MemberAdd and
		// transition them to RejoinInProgress so the node agent wipes their
		// data directory and restarts etcd.
		if dirty := srv.etcdMembers.reconcileEtcdAutoRejoin(ctx, nodes); dirty {
			srv.lock("reconcileAdvanceInfraJoins:etcd-rejoin-persist")
			_ = srv.persistStateLocked(false)
			srv.unlock()
		}
		// Dispatch wipe-etcd-and-rejoin to nodes in RejoinInProgress.
		srv.dispatchEtcdWipeAndRejoin(ctx, nodes)
	}

	// Drive MinIO pool join phases.
	var poolNodes []string
	if srv.minioPoolMgr != nil {
		srv.lock("reconcileAdvanceInfraJoins:minio-snapshot")
		state := srv.state
		poolNodes = append([]string(nil), srv.state.MinioPoolNodes...)
		srv.unlock()
		if dirty := srv.minioPoolMgr.reconcileMinioJoinPhases(nodes, state); dirty {
			srv.lock("reconcileAdvanceInfraJoins:minio-persist")
			_ = srv.persistStateLocked(false)
			srv.unlock()
		}
		// Trigger the coordinated topology workflow when all pool nodes are
		// verified and the applied_generation lags the desired generation.
		srv.maybeRunObjectStoreTopologyWorkflow(ctx)
	}

	// Recover bootstrap workflows that were interrupted by a controller restart.
	srv.recoverStuckBootstrapWorkflows(nodes, time.Now())

	// Advance bootstrap phases that unblocked due to join phase changes above.
	// reconcileBootstrapPhases is normally triggered by reconcileNodes (event-driven),
	// but nodes in storage_joining bypass that trigger (bootstrapPhaseReady=true).
	// Running it here ensures storage_joining → workload_ready fires promptly.
	if bootDirty := reconcileBootstrapPhases(nodes, poolNodes, srv); bootDirty {
		srv.lock("reconcileAdvanceInfraJoins:bootstrap-persist")
		_ = srv.persistStateLocked(false)
		srv.unlock()
	}

	log.Printf("reconcile-workflow: advance_infra_joins completed for %d nodes", len(nodes))
	return nil
}

const etcdPruneSafetyStaleness = 3 * time.Minute

func shouldPruneEtcdStaleMembers(now time.Time, nodes []*nodeState) (bool, string) {
	hasEtcdNode := false
	for _, node := range nodes {
		if node == nil {
			continue
		}
		if !nodeHasProfile(&memberNode{Profiles: node.Profiles}, profilesForEtcd) {
			continue
		}
		hasEtcdNode = true
		if node.LastSeen.IsZero() {
			return false, fmt.Sprintf("node %s has zero LastSeen", node.NodeID)
		}
		if now.Sub(node.LastSeen) > etcdPruneSafetyStaleness {
			return false, fmt.Sprintf("node %s last seen %s ago", node.NodeID, now.Sub(node.LastSeen).Round(time.Second))
		}
		switch node.EtcdJoinPhase {
		case EtcdJoinMemberAdded, EtcdJoinStarted, EtcdJoinRejoinInProgress:
			return false, fmt.Sprintf("node %s in etcd join phase %s", node.NodeID, node.EtcdJoinPhase)
		}
		status := strings.ToLower(strings.TrimSpace(node.Status))
		if status == "offline" || status == "unresponsive" || status == "unknown" {
			return false, fmt.Sprintf("node %s status=%s", node.NodeID, node.Status)
		}
	}
	if !hasEtcdNode {
		return false, "no etcd-profile nodes"
	}
	return true, ""
}

// reconcileScanDrift scans the cluster for drift items that need remediation.
// It does NOT drive infra join state machines — that's the job of the
// preceding advance_infra_joins step. Returns the list of drift items.
func (srv *server) reconcileScanDrift(ctx context.Context, clusterID, scope string, includeNodes []any) ([]any, error) {
	srv.lock("reconcileScanDrift:snapshot")
	nodes := make([]*nodeState, 0, len(srv.state.Nodes))
	activeJoinNodes := make(map[string]bool)
	for _, node := range srv.state.Nodes {
		nodes = append(nodes, node)
		if activeDay1JoinNode(node) {
			activeJoinNodes[node.NodeID] = true
		}
	}
	srv.unlock()

	// Build the set of nodes to include (empty = all).
	includeSet := make(map[string]bool)
	for _, n := range includeNodes {
		includeSet[fmt.Sprint(n)] = true
	}

	// Initialized, never nil: drift_report is a boundary-crossing collection;
	// a nil slice marshals to JSON null and a future len(drift_report) guard
	// would mis-evaluate (see reconcileClassifyDrift). []any{} marshals to [].
	driftItems := []any{}

	for _, node := range nodes {
		if node == nil || node.NodeID == "" {
			continue
		}
		if len(includeSet) > 0 && !includeSet[node.NodeID] {
			continue
		}
		if len(includeSet) == 0 && len(activeJoinNodes) > 0 && !activeJoinNodes[node.NodeID] {
			log.Printf("reconcile-workflow: scan_drift: skipping node %s while Day-1 join active (join isolation)", node.NodeID)
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
		// Merge SERVICE (ServiceDesiredVersion) + INFRASTRUCTURE (InfrastructureRelease)
		// into a single desired map so both kinds participate in drift detection.
		desiredCanon, _, err := srv.loadDesiredServices(ctx)
		if err != nil {
			log.Printf("reconcile-workflow: scan_drift: load desired services failed: %v", err)
			continue
		}
		// Merge infrastructure releases into the same desired map.
		srv.mergeInfraDesiredInto(ctx, desiredCanon)

		// Scope desired to this node's resolved intent.
		// If the node has no profiles, ResolveNodeIntent returns an error and
		// nil intent — FilterDesiredByIntent then no-ops and we drift-scan the
		// full desired set. That's not silent core fallback, just a permissive
		// drift scan; the real "node has no profile" error already surfaces in
		// the main reconciler at reconcile_nodes.go.
		intent, intentErr := ResolveNodeIntent(node.NodeID, node.Profiles, node.Units, node.InstalledVersions)
		if intentErr != nil {
			log.Printf("reconcile-workflow: scan_drift: node %s intent resolution failed: %v (drift scan will not filter)", node.NodeID, intentErr)
		}
		desiredCanon = FilterDesiredByIntent(desiredCanon, intent)

		for svc, desiredVer := range desiredCanon {
			// Check installed state from etcd across ALL legal kinds. The
			// installed_state schema permits SERVICE|INFRASTRUCTURE|COMMAND;
			// a kind this scanner cannot see becomes a permanent
			// missing_package remediation loop (COMMAND packages were
			// invisible here until 2026-07-10 — mc/etcdctl/restic/… cycled
			// as workflow.drift_stuck while installed the whole time).
			pkg, err := installedPackageAnyKind(ctx, node.NodeID, svc)
			if err != nil || pkg == nil {
				// If the node is an active infrastructure member for this package
				// (joined via the Day-0/join flow), the release workflow will
				// short-circuit with 0 targets. Emitting missing_package here
				// creates a permanent remediation loop with no effect.
				if isActiveInfraMember(node, svc) {
					continue
				}
				// objectstore-topology-gated packages (minio/sidekick) are placed
				// by the objectstore admission plane, not by profile-derived desired
				// state. A node can carry the storage profile (ObjectStoreIntent.Member)
				// yet be held pending pool-topology admission; its minio/sidekick is
				// held, not absent. Reporting missing_package there spins a permanent
				// remediation loop the release workflow can never satisfy (0 targets).
				// Scoped narrowly (meta.silence_is_not_valid_for_unexpected): only
				// these gated packages, and only while actually held — a
				// genuinely-missing minio on an active member still drifts below.
				//
				// nodeMinioPlacementIsHeld (not nodeIsExplicitObjectStoreMember) is the
				// gate here on purpose: nodeIsExplicitObjectStoreMember's legacy-mode
				// branch (DesiredObjectStoreMembers == nil) returns true unconditionally,
				// which made this exemption a no-op on any cluster where no topology
				// transition has ever run — every storage-profile Day-1 node's
				// minio/sidekick read as missing forever (failure_mode
				// objectstore.minio.standalone_to_distributed_grow_deadlock).
				if isObjectStoreTopologyGated(svc) && nodeMinioPlacementIsHeld(node) {
					continue
				}
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

		// Check for unmanaged packages (installed but not desired) only when
		// service-removal is enabled. Otherwise these drifts are not actionable
		// (choose_workflow=noop) and become permanent workflow.drift_stuck noise.
		if srv.enableServiceRemoval {
			// Scan both SERVICE and INFRASTRUCTURE kinds.
			for _, kind := range []string{"SERVICE", "INFRASTRUCTURE"} {
				allInstalled, err := installed_state.ListAllNodes(ctx, kind, "")
				if err != nil {
					continue
				}
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
							"kind":         kind,
							"version":      pkg.GetVersion(),
							"hostname":     node.Identity.Hostname,
						})
					}
				}
			}
		}
	}

	if len(includeSet) == 0 && len(activeJoinNodes) > 0 {
		log.Printf("reconcile-workflow: scan_drift found %d drift items across %d nodes (Day-1 join isolation active: %d join node(s))",
			len(driftItems), len(nodes), len(activeJoinNodes))
	} else {
		log.Printf("reconcile-workflow: scan_drift found %d drift items across %d nodes", len(driftItems), len(nodes))
	}
	return driftItems, nil
}

// reconcileClassifyDrift categorizes drift items by severity and type.
func (srv *server) reconcileClassifyDrift(ctx context.Context, driftReport []any, maxRemediations int, coverage []any) ([]any, error) {
	// coverage is the node set this scan examined; empty means a full cluster
	// scan. Cleanup below is restricted to it — see clearResolvedDrift.
	//
	// A nil driftReport means the scan did not produce a report at all (failed
	// or never ran), which is different from an empty report meaning "scanned,
	// nothing drifting". Cleanup must not run on ignorance, so the nil case
	// returns before any clearing. The existing empty-vs-nil distinction at the
	// top of this function relies on the same semantics.
	scanProducedReport := driftReport != nil
	if len(driftReport) == 0 {
		// Return an INITIALIZED empty slice, never nil. remediation_items
		// crosses the actor boundary into the workflow `when` guards
		// (len(remediation_items) == 0 / > 0). A nil slice marshals to JSON
		// null and resolves back as nil, which evalLen deliberately treats as
		// length -1 (fail-closed, undefined != empty) — so BOTH short_circuit
		// and dispatch guards evaluate false and the reconcile foreach fails on
		// a nil collection (the workflow.run.failed storm). An empty []any{}
		// marshals to [] and resolves to length 0, letting short_circuit_clean
		// finalize the no-drift case. Same precedent as selectReleaseTargets.
		// (meta.fallback_must_degrade_semantics; failure_mode
		// ai_executor.repeat_diagnosis_drains_personal_subscription was the
		// downstream symptom of this storm.)
		return []any{}, nil
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

	// OBSERVATION TRUTH vs REMEDIATION SELECTION — these are different sets and
	// must not be conflated.
	//
	// observedAll is EVERY drift condition this scan actually saw. It is built
	// BEFORE the remediation cap, because it answers "does this drift still
	// exist?" — a question the cap has no bearing on.
	//
	// Previously the cap truncated `items` first and observedAll was derived
	// from the truncated slice, so with 75 observations and max_remediations=50
	// rows 51-75 were absent from the "current scan" set. clearResolvedDrift
	// then read that as "no longer drifting" and CLEARED them: unresolved drift
	// was falsely marked resolved purely for sorting below a cap. Because the
	// cap keeps the highest priority first, the rows destroyed were always the
	// lowest-priority ones — and their RecordDriftObservation call was skipped
	// too, so their consecutive-cycle history reset every cycle and persistent
	// low-priority drift could never accumulate toward workflow.drift_stuck.
	observedAll := make(map[string]map[string]bool) // drift_type → entity_ref set
	for _, s := range items {
		dType := fmt.Sprint(s.item["type"])
		eRef := driftEntityRef(s.item)
		if dType == "" || eRef == "" {
			continue
		}
		if observedAll[dType] == nil {
			observedAll[dType] = make(map[string]bool)
		}
		observedAll[dType][eRef] = true
		// Observation history is recorded for EVERY observed row, selected or
		// not: a row that stays drifting must keep advancing its cycle count so
		// workflow.drift_stuck can still fire for drift the cap never reaches.
		if srv.workflowRec != nil {
			// Stamp the workflow this drift type resolves to. The upsert
			// overwrites chosen_workflow each cycle, so passing "" here
			// kept doctor's workflow.drift_stuck findings permanently
			// blank (chosen_workflow=), hiding which remediation looped.
			srv.workflowRec.RecordDriftObservation(ctx, dType, eRef, srv.driftChosenWorkflowName(dType), "")
		}
	}

	// Cap at maxRemediations. This selects what is REMEDIATED this cycle; it
	// never redefines what was OBSERVED.
	selected := items
	if maxRemediations > 0 && len(selected) > maxRemediations {
		selected = selected[:maxRemediations]
	}

	result := make([]any, len(selected))
	for i, s := range selected {
		s.item["priority"] = s.score
		result[i] = s.item
	}

	// Opportunistic cleanup: any previously-tracked drift item NOT in the
	// current scan has been resolved and should be cleared. Fire in background
	// so classify_drift isn't delayed by telemetry bookkeeping.
	if srv.workflowRec != nil && scanProducedReport {
		// Compare against observedAll (complete), never against the capped
		// selection, and only within the coverage this scan actually had.
		coverSet := coverageNodeSet(coverage)
		go srv.clearResolvedDrift(context.Background(), observedAll, coverSet)
		if !srv.enableServiceRemoval {
			// When removal is disabled, unmanaged_package is intentionally
			// non-actionable; proactively clear legacy unresolved rows to avoid
			// persistent workflow.drift_stuck findings.
			go srv.clearUnmanagedDriftObservations(context.Background())
		}
	}

	log.Printf("reconcile-workflow: classify_drift selected %d remediation items (max=%d)", len(result), maxRemediations)
	return result, nil
}

// driftEntityRef builds a stable identifier for a drift item so the telemetry
// layer can track its lifetime across reconcile cycles.
func driftEntityRef(item map[string]any) string {
	pkg, _ := item["package_name"].(string)
	node, _ := item["node_id"].(string)
	// infra_unhealthy items have no package_name — use component instead so
	// etcd/scylladb/minio unhealthy on the SAME node don't collapse onto one
	// no-progress/backoff key (they would otherwise share driftEntityRef and
	// each restart attempt/failure would incorrectly reset or count against
	// a sibling component's counter).
	if pkg == "" {
		if component, ok := item["component"].(string); ok && component != "" {
			pkg = component
		}
	}
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
// coverageNodeSet normalizes the scan's include_nodes into a lookup set.
// An empty result means FULL cluster coverage.
func coverageNodeSet(coverage []any) map[string]bool {
	if len(coverage) == 0 {
		return nil
	}
	out := make(map[string]bool, len(coverage))
	for _, n := range coverage {
		if s := strings.TrimSpace(fmt.Sprint(n)); s != "" {
			out[s] = true
		}
	}
	return out
}

// nodeOfEntityRef extracts the node component of a drift entity ref.
// driftEntityRef builds "pkg@node", "pkg", or "node"; only the "pkg@node" form
// carries an unambiguous node, so anything else is treated as NOT node-scoped
// and is never cleared by a partial scan.
func nodeOfEntityRef(ref string) string {
	if i := strings.LastIndex(ref, "@"); i >= 0 {
		return ref[i+1:]
	}
	return ""
}

func (srv *server) clearResolvedDrift(ctx context.Context, current map[string]map[string]bool, coverage map[string]bool) {
	if srv.workflowClient == nil {
		return
	}
	clusterID := strings.TrimSpace(srv.cfg.ClusterDomain)
	if clusterID == "" {
		return
	}
	resp, err := srv.workflowClient.ListDriftUnresolved(ctx, &workflowpb.ListDriftUnresolvedRequest{
		ClusterId: clusterID,
		MinCycles: 1,
	})
	if err != nil || resp == nil {
		return
	}
	clearResolvedDriftItems(current, coverage, resp.GetItems(), func(driftType, entityRef string) {
		srv.clearDriftObservation(ctx, driftType, entityRef)
	})
}

// clearResolvedDriftItems clears persisted observations that the current scan
// deliberately did not emit. This includes topology-withheld packages: their
// absence is policy, not missing_package drift, so stale generic remediation
// rows must not survive after the scanner learns that eligibility verdict.
// coverage restricts which rows this scan is ENTITLED to resolve. nil means the
// scan covered the whole cluster. For a partial scan, a row whose node was not
// examined has no current observation simply because nobody looked — clearing it
// would convert ignorance into "resolved". Cluster-scoped rows (no node in the
// entity ref) are likewise never cleared by a partial scan.
func clearResolvedDriftItems(current map[string]map[string]bool, coverage map[string]bool, items []*workflowpb.DriftUnresolved, clear func(driftType, entityRef string)) {
	for _, item := range items {
		if item == nil {
			continue
		}
		driftType := strings.TrimSpace(item.GetDriftType())
		entityRef := strings.TrimSpace(item.GetEntityRef())
		if driftType == "" || entityRef == "" {
			continue
		}
		if current[driftType][entityRef] {
			continue
		}
		if len(coverage) > 0 {
			node := nodeOfEntityRef(entityRef)
			if node == "" || !coverage[node] {
				// Outside this scan's coverage: absence proves nothing.
				continue
			}
		}
		clear(driftType, entityRef)
	}
}

// clearUnmanagedDriftObservations clears all unresolved unmanaged_package rows.
// Used only when enableServiceRemoval=false, where unmanaged drift is
// intentionally not remediated and would otherwise stick forever as noop.
func (srv *server) clearUnmanagedDriftObservations(ctx context.Context) {
	if srv.workflowClient == nil {
		return
	}
	clusterID := strings.TrimSpace(srv.cfg.ClusterDomain)
	if clusterID == "" {
		return
	}
	resp, err := srv.workflowClient.ListDriftUnresolved(ctx, &workflowpb.ListDriftUnresolvedRequest{
		ClusterId: clusterID,
		DriftType: "unmanaged_package",
		MinCycles: 1,
	})
	if err != nil || resp == nil {
		return
	}
	for _, item := range resp.GetItems() {
		if item == nil || strings.TrimSpace(item.GetEntityRef()) == "" {
			continue
		}
		_, _ = srv.workflowClient.ClearDriftObservation(ctx, &workflowpb.ClearDriftObservationRequest{
			ClusterId: clusterID,
			DriftType: "unmanaged_package",
			EntityRef: item.GetEntityRef(),
		})
	}
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

type releaseConvergenceIdentity struct {
	resolvedBuildID            string
	desiredHash                string
	resolvedEntrypointChecksum string
	resolvedBuildNumber        int64
}

// lookupServiceReleaseConvergenceIdentity returns the resolved build identity,
// DesiredHash (computed convergence hash, NOT the artifact digest), manifest
// entrypoint checksum, and build number for the given package. DesiredHash is
// what the drift reconciler passes as desired_hash to node-agent workflows, and
// what the convergence-committer stamps into pkg.Checksum. Using artifact digest
// here caused a permanent mismatch: InfrastructureRelease checks pkg.Checksum
// against its own DesiredHash (computed), but the drift path was stamping the raw
// artifact digest — so the two never agreed and the loop never terminated.
//
//globular:enforces infra.desired_hash_consistency
//globular:enforces controller.apply_package_release_requires_manifest_checksum
//globular:expects_hash_schema infra_desired_hash
//globular:expects_hash_schema service_desired_hash
//globular:reads /globular/resources/ServiceRelease
//globular:reads /globular/resources/InfrastructureRelease
//globular:risk convergence.hash_mismatch_loop
func (srv *server) lookupServiceReleaseConvergenceIdentity(ctx context.Context, pkgName string) releaseConvergenceIdentity {
	relKey := defaultPublisherID() + "/" + canonicalServiceName(pkgName)
	// Try ServiceRelease first.
	if obj, _, err := srv.resources.Get(ctx, "ServiceRelease", relKey); err == nil && obj != nil {
		if rel, ok := obj.(*cluster_controllerpb.ServiceRelease); ok {
			if rel.Status != nil {
				var specBuildNumber int64
				if rel.Spec != nil {
					specBuildNumber = rel.Spec.BuildNumber
				}
				return releaseConvergenceIdentity{
					resolvedBuildID:            rel.Status.ResolvedBuildID,
					desiredHash:                rel.Status.DesiredHash,
					resolvedEntrypointChecksum: rel.Status.ResolvedEntrypointChecksum,
					resolvedBuildNumber:        pickBuildNumber(rel.Status.ResolvedBuildNumber, specBuildNumber),
				}
			}
		}
	}
	// Try InfrastructureRelease.
	if obj, _, err := srv.resources.Get(ctx, "InfrastructureRelease", relKey); err == nil && obj != nil {
		if rel, ok := obj.(*cluster_controllerpb.InfrastructureRelease); ok {
			if rel.Status != nil {
				var specBuildNumber int64
				if rel.Spec != nil {
					specBuildNumber = rel.Spec.BuildNumber
				}
				return releaseConvergenceIdentity{
					resolvedBuildID:            rel.Status.ResolvedBuildID,
					desiredHash:                rel.Status.DesiredHash,
					resolvedEntrypointChecksum: rel.Status.ResolvedEntrypointChecksum,
					resolvedBuildNumber:        pickBuildNumber(rel.Status.ResolvedBuildNumber, specBuildNumber),
				}
			}
		}
	}
	return releaseConvergenceIdentity{}
}

// lookupServiceReleaseBuildID preserves the legacy two-value helper for callers
// that only need build_id + convergence hash.
func (srv *server) lookupServiceReleaseBuildID(ctx context.Context, pkgName string) (resolvedBuildID, resolvedHash string) {
	identity := srv.lookupServiceReleaseConvergenceIdentity(ctx, pkgName)
	return identity.resolvedBuildID, identity.desiredHash
}

// driftChosenWorkflowName mirrors reconcileChooseWorkflow's selection for
// telemetry: classify stamps this on the drift observation so doctor's
// workflow.drift_stuck findings name the looping remediation.
func (srv *server) driftChosenWorkflowName(driftType string) string {
	switch driftType {
	case "missing_package", "version_drift":
		return "release.apply.package"
	case "unmanaged_package":
		if srv.enableServiceRemoval {
			return "release.remove.package"
		}
		return "noop"
	default:
		return "noop"
	}
}

// reconcileChooseWorkflow selects the appropriate child workflow for a drift item.
func (srv *server) reconcileChooseWorkflow(ctx context.Context, item map[string]any) (map[string]any, error) {
	driftType := fmt.Sprint(item["type"])
	nodeID := fmt.Sprint(item["node_id"])
	pkgName := fmt.Sprint(item["package_name"])
	desiredVersion := fmt.Sprint(item["desired_version"])

	switch driftType {
	case "missing_package", "version_drift":
		// A drift item that just escalated to FAILED (reconcileNoProgressThreshold
		// consecutive non-resolving remediation attempts) is in a cooldown window:
		// skip dispatch entirely rather than hammering release.apply.package every
		// reconcile cycle for a remediation that keeps not landing (e.g.
		// missing_package with no repository build — see
		// ops.always.package.new-package-must-be-published). The drift scan still
		// runs and the doctor finding stays visible; only the dispatch is paused,
		// and it resumes automatically once the cooldown elapses.
		key := driftType + "|" + driftEntityRef(item)
		if inBackoff, until := srv.reconcileInBackoff(key); inBackoff {
			return map[string]any{
				"workflow_name": "noop",
				"inputs": map[string]any{
					"reason":       fmt.Sprintf("backoff: %s exceeded %d consecutive failed remediation attempts, next attempt after %s", key, reconcileNoProgressThreshold, until.UTC().Format(time.RFC3339)),
					"node_id":      nodeID,
					"package_name": pkgName,
				},
			}, nil
		}

		// Determine the dispatch kind from the component catalog rather than
		// a static name list. COMMAND packages (rclone, restic, sctool, mc,
		// ffmpeg, …) must be tagged as such so the workflow engine skips
		// systemd-unit runtime checks that would never go active.
		kind := "SERVICE"
		if catalogEntry := CatalogByName(pkgName); catalogEntry != nil {
			switch catalogEntry.Kind {
			case KindInfrastructure:
				kind = "INFRASTRUCTURE"
			case KindCommand:
				kind = "COMMAND"
			}
		}
		// Look up the release identity from ServiceRelease/InfrastructureRelease.
		// This threads exact artifact identity and manifest entrypoint proof through
		// the drift-dispatch path so node-agents fetch and verify the correct build.
		identity := srv.lookupServiceReleaseConvergenceIdentity(ctx, pkgName)
		return map[string]any{
			"workflow_name": "release.apply.package",
			"inputs": map[string]any{
				"cluster_id":                   srv.cfg.ClusterDomain,
				"release_id":                   fmt.Sprintf("reconcile-%s-%s-%d", nodeID, pkgName, time.Now().Unix()),
				"release_name":                 fmt.Sprintf("reconcile-%s", pkgName),
				"package_name":                 pkgName,
				"package_kind":                 kind,
				"resolved_version":             desiredVersion,
				"desired_hash":                 identity.desiredHash,
				"resolved_build_id":            identity.resolvedBuildID,
				"resolved_build_number":        identity.resolvedBuildNumber,
				"resolved_entrypoint_checksum": identity.resolvedEntrypointChecksum,
				"candidate_nodes":              []any{nodeID},
			},
		}, nil

	case "infra_unhealthy":
		component := fmt.Sprint(item["component"])
		endpoint := fmt.Sprint(item["endpoint"])
		key := driftType + "|" + driftEntityRef(item)
		if inBackoff, until := srv.reconcileInBackoff(key); inBackoff {
			return map[string]any{
				"workflow_name": "noop",
				"inputs": map[string]any{
					"reason":    fmt.Sprintf("backoff: %s exceeded %d consecutive failed restart attempts, next attempt after %s", key, reconcileNoProgressThreshold, until.UTC().Format(time.RFC3339)),
					"node_id":   nodeID,
					"component": component,
				},
			}, nil
		}
		// Episode identity comes from the drift-history authority, never from
		// this dispatch. Repeated ticks of ONE unresolved episode reconcile to
		// one child run; a recurrence after the drift cleared is a new episode
		// and earns its own run, so both stay independently queryable.
		entityRef := driftEntityRef(item)
		episodeID, epErr := srv.driftEpisodeID(ctx, driftType, entityRef)
		if epErr != nil {
			// Fail closed: no restart run from a guessed identity. The drift is
			// left unresolved (a noop child is a dispatch outcome, not observed
			// convergence, so it cannot clear the row) and a later tick retries.
			log.Printf("reconcile-workflow: infra_unhealthy — %s on node %s: NOT dispatching restart: %v",
				component, nodeID, epErr)
			return map[string]any{
				"workflow_name": "noop",
				"inputs": map[string]any{
					"reason":       fmt.Sprintf("episode_identity_unavailable: %v", epErr),
					"node_id":      nodeID,
					"component":    component,
					"drift_type":   driftType,
					"entity_ref":   entityRef,
					"not_remedied": true,
				},
			}, nil
		}
		log.Printf("reconcile-workflow: infra_unhealthy — %s on node %s: dispatching restart (episode %s)",
			component, nodeID, episodeID)
		return map[string]any{
			"workflow_name": "node.restart_infra_unit",
			"inputs": map[string]any{
				"node_id":          nodeID,
				"component":        component,
				"endpoint":         endpoint,
				"drift_episode_id": episodeID,
				// Descriptive context only. The correlation identity is built
				// from the episode above — the entity reference cannot
				// substitute for it, because it is identical across a
				// resolve-then-recur pair.
				"parent_run_id":  "cluster.reconcile/" + srv.cfg.ClusterDomain,
				"parent_step_id": key,
				"finding_id":     entityRef,
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

// reconcileNoProgressThreshold bounds how many consecutive non-resolving
// terminal remediation attempts (child SUCCEEDED without observed
// installed_state convergence, OR child FAILED outright) may be observed
// before the item is escalated to FAILED and backed off — preventing the
// silent 30s retry-forever loop (convergence.no_infinite_retry / SCAR-2, and
// its FAILED-status counterpart: a permanently-undispatchable-to-completion
// drift, e.g. missing_package with no repository build, was previously
// re-dispatched every reconcile cycle with no terminal state at all —
// meta.silence_is_not_valid_for_unexpected).
const reconcileNoProgressThreshold = 3

// reconcileBackoffCooldown bounds how long a drift item that just escalated to
// FAILED is excluded from further dispatch by reconcileChooseWorkflow. Bounded
// (not permanent) so a transient cause — repository outage, a publish still
// propagating — resolves on its own once the window elapses; no operator
// action required. meta.write_creates_completion_obligation: the dispatch
// attempt now has an actual completion path instead of an unfinished promise.
const reconcileBackoffCooldown = 10 * time.Minute

// observeInstalledPackage reads the L3 installed_state for (nodeID, name) —
// the same any-kind lookup the drift scanner uses. installed_state is
// package-global, so the srv.observeInstalledPkg seam overrides it in tests.
func (srv *server) observeInstalledPackage(ctx context.Context, nodeID, name string) (*node_agentpb.InstalledPackage, error) {
	if srv.observeInstalledPkg != nil {
		return srv.observeInstalledPkg(ctx, nodeID, name)
	}
	return installedPackageAnyKind(ctx, nodeID, name)
}

// installedPackageAnyKind reads the L3 installed record for (nodeID, name)
// across every kind the installed_state schema permits (SERVICE,
// INFRASTRUCTURE, COMMAND). The component catalog's kind, when known, is
// tried first. The drift scanner and the convergence observer MUST share
// this lookup: any kind one of them cannot see either produces phantom
// missing_package drift or blocks SCAR-2 convergence observation forever.
func installedPackageAnyKind(ctx context.Context, nodeID, name string) (*node_agentpb.InstalledPackage, error) {
	kinds := []string{"SERVICE", "INFRASTRUCTURE", "COMMAND"}
	if c := CatalogByName(name); c != nil {
		switch c.Kind {
		case KindInfrastructure:
			kinds = []string{"INFRASTRUCTURE", "SERVICE", "COMMAND"}
		case KindCommand:
			kinds = []string{"COMMAND", "INFRASTRUCTURE", "SERVICE"}
		}
	}
	var lastErr error
	for _, kind := range kinds {
		pkg, err := getInstalledPackageFn(ctx, nodeID, kind, name)
		if err != nil {
			lastErr = err
			continue
		}
		if pkg != nil {
			return pkg, nil
		}
	}
	return nil, lastErr
}

// getInstalledPackageFn is a test seam over the etcd-backed read.
var getInstalledPackageFn = installed_state.GetInstalledPackage

// reconcileItemConverged re-reads installed_state AFTER a child remediation
// returns and reports whether the observed L3 state now matches desired.
// checkable is false for drift types that have no installed-state convergence
// predicate (e.g. unmanaged_package, whose success may legitimately be a
// removal-disabled noop) — those preserve the legacy clear-on-SUCCEEDED path.
// Enforces reconcile.terminal_success_requires_observed_convergence: a child
// workflow reporting SUCCEEDED is a dispatch acknowledgement, not proof the
// node installed the package.
func (srv *server) reconcileItemConverged(ctx context.Context, item map[string]any) (converged, checkable bool) {
	dType := fmt.Sprint(item["type"])
	switch dType {
	case "missing_package", "version_drift":
		nodeID := fmt.Sprint(item["node_id"])
		name := fmt.Sprint(item["package_name"])
		desiredVer := fmt.Sprint(item["desired_version"])
		pkg, err := srv.observeInstalledPackage(ctx, nodeID, name)
		if err != nil || pkg == nil {
			return false, true
		}
		if dType == "missing_package" {
			// Installed at all; when a desired version is known, require the match.
			return desiredVer == "" || pkg.GetVersion() == desiredVer, true
		}
		return pkg.GetVersion() == desiredVer, true // version_drift
	case "infra_unhealthy":
		// Child SUCCEEDED means the restart RPC was dispatched and node-agent
		// reported the systemd action itself succeeded — not proof the
		// service is actually healthy again (same SCAR-2 discipline as
		// missing_package/version_drift: dispatch ack is not install proof).
		// Re-probe the same health check reconcileScanDrift used to raise
		// this drift in the first place before clearing it.
		endpoint := fmt.Sprint(item["endpoint"])
		switch fmt.Sprint(item["component"]) {
		case "etcd":
			return srv.probeEtcdHealth(ctx, endpoint), true
		case "scylladb":
			return srv.probeScyllaHealth(ctx, endpoint), true
		case "minio":
			return srv.probeMinioHealth(ctx, endpoint), true
		default:
			return false, true
		}
	default:
		return false, false
	}
}

// clearDriftObservation clears a recorded drift observation. The srv.clearDriftObsFn
// seam overrides the real (concrete, nil-safe) recorder in tests.
func (srv *server) clearDriftObservation(ctx context.Context, driftType, entityRef string) {
	if srv.clearDriftObsFn != nil {
		srv.clearDriftObsFn(ctx, driftType, entityRef)
		return
	}
	if srv.workflowRec != nil {
		srv.workflowRec.ClearDriftObservation(ctx, driftType, entityRef)
	}
}

func (srv *server) reconcileBumpNoProgress(key string) int {
	srv.reconcileNoProgMu.Lock()
	defer srv.reconcileNoProgMu.Unlock()
	if srv.reconcileNoProgress == nil {
		srv.reconcileNoProgress = make(map[string]int)
	}
	srv.reconcileNoProgress[key]++
	return srv.reconcileNoProgress[key]
}

func (srv *server) reconcileResetNoProgress(key string) {
	srv.reconcileNoProgMu.Lock()
	defer srv.reconcileNoProgMu.Unlock()
	if srv.reconcileNoProgress != nil {
		delete(srv.reconcileNoProgress, key)
	}
}

// reconcileArmBackoff starts a reconcileBackoffCooldown window for key, during
// which reconcileChooseWorkflow will not dispatch a remediation workflow for
// it. Called once an item crosses reconcileNoProgressThreshold.
func (srv *server) reconcileArmBackoff(key string) {
	srv.reconcileNoProgMu.Lock()
	defer srv.reconcileNoProgMu.Unlock()
	if srv.reconcileBackoffUntil == nil {
		srv.reconcileBackoffUntil = make(map[string]time.Time)
	}
	srv.reconcileBackoffUntil[key] = time.Now().Add(reconcileBackoffCooldown)
}

// reconcileInBackoff reports whether key is still within its cooldown window.
// Expired entries are cleared as a side effect so the map does not grow
// unbounded with stale keys.
func (srv *server) reconcileInBackoff(key string) (bool, time.Time) {
	srv.reconcileNoProgMu.Lock()
	defer srv.reconcileNoProgMu.Unlock()
	until, ok := srv.reconcileBackoffUntil[key]
	if !ok {
		return false, time.Time{}
	}
	if !time.Now().Before(until) {
		delete(srv.reconcileBackoffUntil, key)
		return false, time.Time{}
	}
	return true, until
}

// reconcileMarkItemTerminal records the outcome of a child remediation.
//
// SCAR-2 (reconcile.terminal_success_requires_observed_convergence): a child
// workflow reporting SUCCEEDED is a dispatch/finalize acknowledgement, NOT proof
// the node installed the package (installed_state is L3, owned and reported by
// the node-agent). For missing_package/version_drift we therefore re-read
// installed_state before clearing the drift observation; a SUCCEEDED that did not
// produce observed convergence does NOT clear the observation and, after
// reconcileNoProgressThreshold consecutive no-progress passes, escalates to
// FAILED (reason=remediation_no_progress) — never an unbounded silent loop.
//
// Companion follow-up (deliberately OUT of SCAR-2 scope): the upstream cause of
// the false SUCCEEDED is release.apply.package's short_circuit_if_no_targets
// finalizing AVAILABLE when selectReleaseTargets returns 0 targets for a
// NON-convergence reason (bootstrap-phase / profile / active-infra-member); that
// should finalize DEFERRED/BLOCKED, not AVAILABLE (workflow_release.go). The
// observation-gate here defends regardless of that leak.
func (srv *server) reconcileMarkItemTerminal(ctx context.Context, item, childResult map[string]any) error {
	status := "unknown"
	if childResult != nil {
		status = fmt.Sprint(childResult["status"])
	}
	log.Printf("reconcile-workflow: item terminal: type=%s node=%s pkg=%s child_status=%s",
		item["type"], item["node_id"], item["package_name"], status)

	dType := fmt.Sprint(item["type"])
	eRef := driftEntityRef(item)
	key := dType + "|" + eRef

	if status == "SUCCEEDED" {
		converged, checkable := srv.reconcileItemConverged(ctx, item)

		// Observed convergence proven (or the drift type has no observation
		// predicate): clear the drift observation and reset the no-progress counter.
		if !checkable || converged {
			if dType != "" && eRef != "" {
				srv.clearDriftObservation(ctx, dType, eRef)
			}
			srv.reconcileResetNoProgress(key)
			if checkable {
				log.Printf("reconcile-workflow: observed convergence confirmed for %s (%s) — drift cleared", eRef, dType)
			}
			return nil
		}
		// Child reported SUCCEEDED but installed_state does NOT reflect it — falls
		// through to the shared no-progress counting below (SCAR-2).
	}
	// Any other terminal status (FAILED, ERROR, UNKNOWN, ...) also falls through:
	// meta.silence_is_not_valid_for_unexpected. This used to `return nil` here,
	// silently dropping a failed dispatch — a permanently-failing remediation
	// (e.g. missing_package with no repository build) was then re-dispatched every
	// reconcile cycle forever, with no counter, no terminal state, and no visible
	// failure (workflow.drift_stuck climbing consecutive_cycles indefinitely).

	n := srv.reconcileBumpNoProgress(key)
	log.Printf("reconcile-workflow: remediation not resolved for %s (%s) — child_status=%s, no-progress %d/%d",
		eRef, dType, status, n, reconcileNoProgressThreshold)
	if n >= reconcileNoProgressThreshold {
		reason := "remediation_no_progress"
		if status != "SUCCEEDED" {
			reason = fmt.Sprintf("remediation_dispatch_failed: child_status=%s", status)
		}
		failItem := make(map[string]any, len(item)+1)
		for k, v := range item {
			failItem[k] = v
		}
		failItem["reason"] = reason
		_ = srv.reconcileMarkItemFailed(ctx, failItem)
		srv.reconcileResetNoProgress(key) // re-arm: emit a periodic signal, not every tick
		if key != "|" {
			srv.reconcileArmBackoff(key) // stop re-dispatching this item for a cooldown window
		}
	}
	return nil
}

// reconcileMarkItemFailed records a failed remediation item. An optional
// item["reason"] is surfaced in the emitted event (used by the SCAR-2
// no-progress escalation); when absent the event shape is unchanged.
func (srv *server) reconcileMarkItemFailed(ctx context.Context, item map[string]any) error {
	reason := ""
	if r, ok := item["reason"]; ok {
		reason = fmt.Sprint(r)
	}
	log.Printf("reconcile-workflow: item FAILED: type=%s node=%s pkg=%s reason=%s",
		item["type"], item["node_id"], item["package_name"], reason)
	payload := map[string]interface{}{
		"severity":     "WARN",
		"node_id":      fmt.Sprint(item["node_id"]),
		"package_name": fmt.Sprint(item["package_name"]),
		"drift_type":   fmt.Sprint(item["type"]),
	}
	if reason != "" {
		payload["reason"] = reason
	}
	srv.emitEvent("cluster.reconcile.item_failed", payload)
	return nil
}

// emitEvent emits a cluster event. The srv.emitEventFn seam captures events in
// tests; production falls through to emitClusterEvent.
func (srv *server) emitEvent(name string, payload map[string]interface{}) {
	if srv.emitEventFn != nil {
		srv.emitEventFn(name, payload)
		return
	}
	srv.emitClusterEvent(name, payload)
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
	controllerLoopHeartbeatUnix.Set(float64(time.Now().Unix()))
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

// RunClusterReconcileWorkflow delegates execution of the cluster.reconcile
// workflow to the centralized WorkflowService. The workflow detects drift
// and dispatches child remediation workflows (which are also centralized).
func (srv *server) RunClusterReconcileWorkflow(ctx context.Context) (*workflowpb.ExecuteWorkflowResponse, error) {
	router := engine.NewRouter()
	var childResults sync.Map // runID -> map[string]any

	// Wire reconcile controller actions.
	engine.RegisterReconcileControllerActions(router, srv.buildReconcileControllerConfig())

	// Wire workflow-service actions for child workflow dispatch.
	engine.RegisterWorkflowServiceActions(router, engine.WorkflowServiceConfig{
		StartChild: func(ctx context.Context, workflowName string, inputs map[string]any) (string, error) {
			if workflowName == "noop" {
				reason := ""
				if inputs != nil {
					reason = fmt.Sprint(inputs["reason"])
				}
				log.Printf("reconcile-workflow: noop child: %s", reason)
				childResults.Store("noop-run", map[string]any{"status": "SUCCEEDED", "run_id": "noop-run"})
				return "noop-run", nil
			}

			if workflowName == "node.restart_infra_unit" {
				// Durable child run, same shape as the release branches below.
				//
				// This used to call srv.restartInfraUnit inline — the controller
				// dialled node-agent ControlService itself, minted a run id from
				// time.Now().UnixNano() AFTER the mutation, and stored a
				// synthetic status in this process-local map. No Workflow Service
				// run ever existed, so the mutation had no durable identity, every
				// retry allocated a new one, the terminal result vanished on
				// restart, and the returned child id resolved to nothing.
				nodeID := fmt.Sprint(inputs["node_id"])
				component := fmt.Sprint(inputs["component"])
				endpoint := fmt.Sprint(inputs["endpoint"])
				resp, err := srv.RunRestartInfraUnitWorkflow(
					ctx,
					fmt.Sprint(inputs["parent_run_id"]),
					fmt.Sprint(inputs["parent_step_id"]),
					fmt.Sprint(inputs["finding_id"]),
					fmt.Sprint(inputs["drift_episode_id"]),
					nodeID, endpoint, component)
				if err != nil {
					return "", err
				}
				childResults.Store(resp.RunId, map[string]any{
					"status": resp.Status,
					"run_id": resp.RunId,
					"error":  resp.Error,
				})
				return resp.RunId, nil
			}

			if workflowName == "release.apply.package" {
				releaseID := fmt.Sprint(inputs["release_id"])
				releaseName := fmt.Sprint(inputs["release_name"])
				pkgName := fmt.Sprint(inputs["package_name"])
				pkgKind := fmt.Sprint(inputs["package_kind"])
				version := fmt.Sprint(inputs["resolved_version"])
				desiredHash := fmt.Sprint(inputs["desired_hash"])
				resolvedBuildID := fmt.Sprint(inputs["resolved_build_id"])
				// v1.2.119: BINARY hash from manifest.entrypoint_checksum.
				// Empty when parent workflow did not supply it; node-agent then
				// writes installed_unverified honestly.
				resolvedEntrypointChecksum := ""
				if v, ok := inputs["resolved_entrypoint_checksum"].(string); ok {
					resolvedEntrypointChecksum = v
				}
				var resolvedBuildNumber int64
				switch v := inputs["resolved_build_number"].(type) {
				case int64:
					resolvedBuildNumber = v
				case int:
					resolvedBuildNumber = int64(v)
				case float64:
					resolvedBuildNumber = int64(v)
				}
				candidates, _ := inputs["candidate_nodes"].([]any)
				candidateStrs := make([]string, len(candidates))
				for i, c := range candidates {
					candidateStrs[i] = fmt.Sprint(c)
				}

				resp, err := srv.RunPackageReleaseWorkflow(ctx, releaseID, releaseName, pkgName, pkgKind, version, desiredHash, resolvedBuildID, resolvedEntrypointChecksum, resolvedBuildNumber, candidateStrs)
				if err != nil {
					return "", err
				}
				childResults.Store(resp.RunId, map[string]any{
					"status": resp.Status,
					"run_id": resp.RunId,
					"error":  resp.Error,
				})
				return resp.RunId, nil
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

				resp, err := srv.RunRemovePackageWorkflow(ctx, releaseID, pkgName, pkgKind, candidateStrs)
				if err != nil {
					return "", err
				}
				childResults.Store(resp.RunId, map[string]any{
					"status": resp.Status,
					"run_id": resp.RunId,
					"error":  resp.Error,
				})
				return resp.RunId, nil
			}

			return "", fmt.Errorf("unknown child workflow: %s", workflowName)
		},
		WaitChildTerminal: func(ctx context.Context, childRunID string) (map[string]any, error) {
			// Child workflows run synchronously via ExecuteWorkflow,
			// so by the time StartChild returns, the run is already terminal.
			if result, ok := childResults.Load(childRunID); ok {
				if m, ok := result.(map[string]any); ok {
					return m, nil
				}
			}
			return map[string]any{
				"status": "UNKNOWN",
				"run_id": childRunID,
				"error":  "child workflow result was not recorded",
			}, nil
		},
	})

	inputs := map[string]any{
		"cluster_id": srv.cfg.ClusterDomain,
		"scope":      "cluster",
	}

	correlationID := fmt.Sprintf("reconcile:%d", time.Now().UnixMilli())

	log.Printf("reconcile-workflow: starting cluster.reconcile")
	// Mark activity immediately so Prom/Doctor can see liveness even if the
	// workflow fails early.
	workflowActiveRuns.Inc()
	controllerLoopHeartbeatUnix.Set(float64(time.Now().Unix()))
	startedAt := time.Now()
	resp, err := srv.executeWorkflowCentralized(ctx, "cluster.reconcile", correlationID, inputs, router)
	finishedAt := time.Now()
	defer workflowActiveRuns.Dec()

	// Summary-only persistence for the bounded dashboard view.
	runID := ""
	if resp != nil {
		runID = resp.RunId
	}
	outcomeStatus := workflowpb.RunStatus_RUN_STATUS_SUCCEEDED
	failureReason := ""
	if err != nil {
		outcomeStatus = workflowpb.RunStatus_RUN_STATUS_FAILED
		failureReason = err.Error()
		log.Printf("reconcile-workflow: cluster.reconcile FAILED: %v", err)
		// Stamp heartbeat on failure so alerts reset and surfaces show recent activity.
		controllerLoopHeartbeatUnix.Set(float64(time.Now().Unix()))
	} else {
		log.Printf("reconcile-workflow: cluster.reconcile completed")
		// Stamp controller heartbeat so Prometheus alerts can see progress.
		controllerLoopHeartbeatUnix.Set(float64(time.Now().Unix()))
	}
	if srv.workflowRec != nil {
		srv.workflowRec.RecordOutcome(ctx, "cluster.reconcile", runID,
			outcomeStatus, startedAt, finishedAt, failureReason)
	}

	return resp, err
}
