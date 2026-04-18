package main

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/installed_state"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"github.com/globulario/services/golang/repository/repository_client"
	"github.com/globulario/services/golang/workflow/engine"
	"github.com/globulario/services/golang/versionutil"
)

// Leader-only mutation rule:
//
//   Followers may watch, serve reads, and cache.
//   Followers may NOT resolve releases, dispatch workflows, mutate release
//   phase/generation, or perform repository-backed reconcile decisions.
//
// This is enforced by calling srv.requireLeader(ctx) at the entry point of
// every mutation function. The existing requireLeader (server.go) returns a
// gRPC FailedPrecondition with leader_addr metadata for RPC callers.
// Internal reconcile paths use mustBeLeader() below for a fast boolean check.

// mustBeLeader returns true if this instance is the active leader.
// Use at the top of internal reconcile/mutation functions that are NOT
// gRPC handlers (those should use requireLeader(ctx) for redirect metadata).
func (srv *server) mustBeLeader() bool {
	if srv.isLeader() {
		return true
	}
	reconcileDroppedNotLeader.Inc()
	return false
}

const (
	networkReconcileKey = "network/default"
	serviceKeyPrefix    = "service/"
)

func isNetworkKey(key string) bool {
	return key == networkReconcileKey
}

func serviceNameFromKey(key string) string {
	if len(key) <= len(serviceKeyPrefix) || key[:len(serviceKeyPrefix)] != serviceKeyPrefix {
		return ""
	}
	return key[len(serviceKeyPrefix):]
}

// startControllerRuntime starts watch-based reconcile with a small worker pool.
//
// Design invariant: a converged restart must not dispatch reconcile work for
// converged services. The goal is to suppress disruptive work (workflow
// dispatches, repository resolves, systemd restarts), not to achieve zero
// enqueues — bookkeeping or verification passes are acceptable as long as
// they don't trigger mutations on already-converged state.
//
// Doctrine: a missing or stale release object is not by itself evidence that
// execution is needed. Only unmet convergence (desired != installed on eligible
// nodes) justifies work.
//
// Three distinct gates control the flow of reconcile work:
//
//  1. Admission gate — convergence filter + staggered enqueue suppress work
//     at the source. This is the primary defense against restart storms.
//     "Too much work admitted too early" was the original disease.
//
//  2. Resolve gate — resolveSem (cap 2) limits concurrent repository calls
//     so the PENDING→RESOLVED transition doesn't saturate the repo endpoint.
//
//  3. Execution gate — workflowSem (cap 3) + inflightWorkflows map limit
//     concurrent workflow dispatches to prevent systemd overload on nodes.
func (srv *server) startControllerRuntime(ctx context.Context, workers int) {
	if !reconcileVersionGate() {
		logger.Error("startControllerRuntime: controller version too old — reconciliation disabled",
			"version", Version, "minimum", minSafeReconcileVersion)
		return
	}
	if workers <= 0 {
		workers = 2
	}
	queue := newWorkQueue(128)

	// Install a safe default router so workflow callbacks don't fail with
	// "no handler" after controller restarts. This router uses the live
	// reconcile handlers; per-run routers still take precedence.
	defaultRouter := engine.NewRouter()
	engine.RegisterReconcileControllerActions(defaultRouter, srv.buildReconcileControllerConfig())
	engine.RegisterInvariantActions(defaultRouter, srv.buildInvariantConfig())
	engine.RegisterNodeRepairControllerActions(defaultRouter, srv.buildNodeRepairControllerConfig())
	engine.RegisterNodeRepairAgentActions(defaultRouter, srv.buildNodeRepairAgentConfig())
	engine.RegisterNodeRecoveryControllerActions(defaultRouter, srv.buildNodeRecoveryControllerConfig())
	srv.actorServer.SetDefaultRouter(defaultRouter)

	// Staggered initial enqueue: wait for readiness predicates to pass, then
	// filter out already-converged services and enqueue in small batches.
	// This prevents a restart from becoming a full-cluster apply storm.
	safeGo("staggered-initial-enqueue", func() {
		// Readiness-gated warmup: wait at least 10 seconds, then continue
		// polling until the controller has situational awareness. A timer is
		// a nap; a readiness gate is situational awareness.
		select {
		case <-ctx.Done():
			return
		case <-time.After(10 * time.Second):
		}
		if !srv.waitForReadiness(ctx) {
			return
		}

		queue.Enqueue(networkReconcileKey)

		if srv.resources == nil {
			return
		}

		items, _, err := srv.resources.List(ctx, "ServiceDesiredVersion", "")
		if err != nil {
			return
		}

		// Filter: only enqueue services that are not converged.
		var unconverged []string
		suppressed := 0
		for _, obj := range items {
			sdv, ok := obj.(*cluster_controllerpb.ServiceDesiredVersion)
			if !ok || sdv.Meta == nil || sdv.Spec == nil {
				continue
			}
			canon := canonicalServiceName(sdv.Meta.Name)
			if srv.isServiceConverged(ctx, canon, sdv.Spec.Version, sdv.Spec.BuildNumber, sdv.Spec.BuildID) {
				suppressed++
				continue
			}
			unconverged = append(unconverged, canon)
		}
		convergenceFilterSuppressed.Add(float64(suppressed))

		if len(unconverged) > 0 {
			logger.Info("staggered-initial-enqueue: enqueuing unconverged services",
				"total_desired", len(items), "unconverged", len(unconverged))
		}

		// Enqueue in batches of 5, 2-second gaps.
		const batchSize = 5
		for i, name := range unconverged {
			if ctx.Err() != nil {
				return
			}
			queue.Enqueue(serviceKeyPrefix + name)
			if (i+1)%batchSize == 0 && i+1 < len(unconverged) {
				select {
				case <-ctx.Done():
					return
				case <-time.After(2 * time.Second):
				}
			}
		}
	})

	// watchers
	if srv.resources != nil {
		safeGo("watch-cluster-network", func() {
			ch, err := srv.resources.Watch(ctx, "ClusterNetwork", "default", "")
			if err != nil {
				return
			}
			for evt := range ch {
				_ = evt
				reconcileEnqueueTotal.WithLabelValues("watch").Inc()
				queue.Enqueue(networkReconcileKey)
			}
		})
		safeGo("watch-service-desired", func() {
			ch, err := srv.resources.Watch(ctx, "ServiceDesiredVersion", "", "")
			if err != nil {
				return
			}
			for evt := range ch {
				sdv, ok := evt.Object.(*cluster_controllerpb.ServiceDesiredVersion)
				if !ok || sdv == nil || sdv.Meta == nil {
					continue
				}
				reconcileEnqueueTotal.WithLabelValues("watch").Inc()
				queue.Enqueue(serviceKeyPrefix + canonicalServiceName(sdv.Meta.Name))
			}
		})
		srv.startReleaseReconciler(ctx, queue)
		// Allow ReportNodeStatus to trigger release re-evaluation (e.g. after
		// a node finishes a drift-repair plan and its AppliedServicesHash changes).
		srv.releaseEnqueue = func(releaseName string) {
			queue.Enqueue(releaseKeyPrefix + releaseName)
		}
		srv.infraReleaseEnqueue = func(releaseName string) {
			queue.Enqueue(infraReleaseKeyPrefix + releaseName)
		}
	}
	// Allow SetNodeProfiles to immediately trigger reconciliation.
	srv.enqueueReconcile = func() {
		queue.Enqueue(networkReconcileKey)
	}

	// workers
	for i := 0; i < workers; i++ {
		safeGo(fmt.Sprintf("reconcile-worker-%d", i), func() {
			for {
				key, ok := queue.Get(ctx)
				if !ok {
					return
				}
				if !srv.isLeader() {
					queue.Done(key, fmt.Errorf("not leader"))
					continue
				}
				switch {
				case isNetworkKey(key):
					srv.reconcileNodes(ctx)
				case serviceNameFromKey(key) != "":
					srv.reconcileNodes(ctx)
				case isReleaseKey(key):
					srv.reconcileRelease(ctx, releaseNameFromKey(key))
				case isAppReleaseKey(key):
					srv.reconcileAppRelease(ctx, appReleaseNameFromKey(key))
				case isInfraReleaseKey(key):
					srv.reconcileInfraRelease(ctx, infraReleaseNameFromKey(key))
				default:
					// unknown key, drop
				}
				queue.Done(key, nil)
			}
		})
	}

	// runClusterReconcileIfIdle attempts to start a cluster.reconcile workflow.
	// If a previous run is still active, it marks pending so exactly one
	// follow-up run occurs when the current run finishes. This prevents
	// concurrent reconcile storms while ensuring no work is silently dropped.
	srv.runClusterReconcileIfIdle = func(parentCtx context.Context, source string) {
		// Reconcile circuit breaker: skip if open (too many recent failures).
		if srv.reconcileBreaker != nil {
			if err := srv.reconcileBreaker.Allow(); err != nil {
				logger.Info("reconcile deferred: circuit breaker open", "source", source, "reason", err.Error())
				srv.emitClusterEvent("cluster.reconcile_circuit_open", map[string]interface{}{
					"severity": "CRITICAL",
					"source":   source,
					"message":  err.Error(),
				})
				return
			}
		}

		if !srv.clusterReconcileRunning.CompareAndSwap(false, true) {
			srv.clusterReconcilePending.Store(true)
			clusterReconcileSkippedTotal.WithLabelValues(source).Inc()
			logger.Info("reconcile skipped: previous run still active", "source", source)
			return
		}
		go func() {
			defer srv.clusterReconcileRunning.Store(false)
			for {
				// Clear pending before the run so ticks during execution
				// set it again, guaranteeing exactly one follow-up pass.
				srv.clusterReconcilePending.Store(false)

				rctx, cancel := context.WithTimeout(parentCtx, 10*time.Minute)
				_, err := srv.RunClusterReconcileWorkflow(rctx)
				cancel()

				if err != nil {
					logger.Debug("periodic cluster.reconcile failed", "error", err)
					if srv.reconcileBreaker != nil {
						srv.reconcileBreaker.RecordTimeout()
					}
				} else {
					if srv.reconcileBreaker != nil {
						srv.reconcileBreaker.RecordSuccess()
					}
				}

				// If work arrived while we were running, do one more pass.
				if parentCtx.Err() != nil {
					return
				}
				if !srv.clusterReconcilePending.CompareAndSwap(true, false) {
					return // no pending work
				}
				logger.Info("reconcile: running coalesced follow-up pass")
			}
		}()
	}

	// Periodic cluster.reconcile workflow: drives infrastructure health
	// scans (ScyllaDB, MinIO join phases + probes) and detects package
	// drift. Runs every 30s when leader, replacing the old direct calls
	// in reconcileNodes().
	safeGoTracked("periodic-cluster-reconcile", 30*time.Second, func(h *globular_service.SubsystemHandle) {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if !srv.isLeader() {
					h.Tick()
					continue
				}
				srv.runClusterReconcileIfIdle(ctx, "periodic")
				h.Tick()
			}
		}
	})

	// Periodic bridge: a repair backstop, NOT the primary engine of convergence.
	// Re-creates ServiceRelease objects for desired services that lost their
	// release (e.g. deleted during troubleshooting, or garbage-collected while
	// in REMOVED phase). Without this, a missing release causes the service to
	// stay stuck at "Planned" indefinitely.
	//
	// The primary convergence engine is the watch-driven work queue. The bridge
	// exists only to catch edge cases that watches miss (manual deletion, GC
	// race). It must never become a driver of normal convergence — if it does,
	// that indicates a watch or reconcile bug that should be fixed at the source.
	//
	// Runs every 2 minutes with a 60-second startup delay to avoid contributing
	// to the restart storm.
	safeGoTracked("periodic-release-bridge", 120*time.Second, func(h *globular_service.SubsystemHandle) {
		// Startup delay: let initial enqueue and heartbeats settle first.
		select {
		case <-ctx.Done():
			return
		case <-time.After(60 * time.Second):
		}

		ticker := time.NewTicker(120 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if srv.isLeader() {
					srv.ensureServiceReleasesFromDesired(ctx)
				}
				h.Tick()
			}
		}
	})

	// Drift reconciler: periodically scans desired vs installed per-node
	// and dispatches ApplyPackageRelease for any drift detected.
	// Must obey the same warmup/readiness gate as the release pipeline —
	// drift detection may observe, but must not dispatch/apply until
	// the control plane is ready.
	driftRec := newDriftReconciler(srv, 30*time.Second)
	safeGo("drift-reconciler-gated", func() {
		// Wait for the same readiness predicates the release pipeline uses.
		select {
		case <-ctx.Done():
			return
		case <-time.After(10 * time.Second):
		}
		if !srv.waitForReadiness(ctx) {
			return
		}
		logger.Info("drift-reconciler: readiness gate passed, starting")
		driftRec.Start(ctx)
	})

	// Trigger A: Auto-import desired state from installed services at startup.
	// Waits for at least one node to report installed versions, then checks if
	// desired state is empty. If so, runs importInstalledToDesired to backfill.
	// This closes the gap where Day-0 seed failed or was skipped.
	safeGo("startup-auto-import", func() {
		srv.startupAutoImport(ctx, queue)
	})
}

// startupAutoImport waits for nodes to report installed versions, then
// auto-imports into desired state if it is empty. This is the Trigger A
// from the state alignment design doc.
func (srv *server) startupAutoImport(ctx context.Context, queue *workQueue) {
	// Wait up to 90s for at least one node to report installed versions.
	// Nodes typically report within 30-60s of startup.
	const maxWait = 90 * time.Second
	const pollInterval = 5 * time.Second

	deadline := time.Now().Add(maxWait)
	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			return
		case <-time.After(pollInterval):
		}

		if !srv.isLeader() {
			continue
		}

		// Check canonical installed-state registry first (any package kind).
		if pkgs, err := installed_state.ListAllNodes(ctx, "SERVICE", ""); err == nil && len(pkgs) > 0 {
			break
		}
		if pkgs, err := installed_state.ListAllNodes(ctx, "APPLICATION", ""); err == nil && len(pkgs) > 0 {
			break
		}
		if pkgs, err := installed_state.ListAllNodes(ctx, "INFRASTRUCTURE", ""); err == nil && len(pkgs) > 0 {
			break
		}

		// Fallback: check in-memory node state.
		srv.lock("startupAutoImport:check")
		hasNodes := false
		hasInstalled := false
		for _, node := range srv.state.Nodes {
			hasNodes = true
			if len(node.InstalledVersions) > 0 {
				hasInstalled = true
				break
			}
		}
		srv.unlock()

		if hasInstalled {
			break
		}
		if hasNodes {
			// Nodes exist but haven't reported installed versions yet — keep waiting.
			continue
		}
	}

	if !srv.isLeader() {
		return
	}

	// Check if desired state is already populated.
	if srv.resources == nil {
		return
	}
	svcItems, _, err := srv.resources.List(ctx, "ServiceDesiredVersion", "")
	if err != nil {
		logger.Warn("startupAutoImport: failed to list desired services", "error", err)
		return
	}
	infraItems, _, _ := srv.resources.List(ctx, "InfrastructureRelease", "")

	// Always reconcile: remove desired entries for services no longer in
	// the installed-state registry. This cleans up stale entries from
	// packages that were incorrectly marked as installed previously.
	srv.reconcileDesiredWithInstalled(ctx)

	srv.autoImportDone.Store(true)

	if len(svcItems) > 0 || len(infraItems) > 0 {
		logger.Info("startupAutoImport: desired state already has entries",
			"services", len(svcItems), "infra", len(infraItems))
		return
	}

	// Desired state is empty. Do NOT auto-import from runtime observations —
	// that is an authority inversion (Layer 3 → Layer 2). The operator must
	// explicitly seed desired state via:
	//   globular deploy <package> --bump
	//   or SeedDesiredState RPC (IMPORT_FROM_INSTALLED mode)
	//
	// Auto-importing from runtime was the root cause of phantom service
	// rematerialization: old/stale node-agent reports (fallback 0.1.0 or
	// "unknown" versions) would create desired entries that the reconciler
	// then tried to converge, creating ghost services.
	logger.Warn("startupAutoImport: desired state is EMPTY — operator must seed it explicitly via 'globular deploy' or SeedDesiredState RPC. Auto-import from runtime observations is disabled to prevent phantom rematerialization.")
}

// waitForReadiness polls readiness predicates every 5 seconds until they all
// pass or the context is cancelled. Returns true when ready, false on cancel.
//
// Predicates are split into two tiers:
//
//	Hard (no timeout — must pass, or reconcile never starts):
//	- Controller is leader
//	- Resource store is functional
//
//	Soft (bounded timeout — 60s, then proceed with warning):
//	- At least one node has reported via heartbeat
//	- Installed-state snapshot is readable from etcd
//	- Workflow service reachable
//	- Repository resolution path reachable
//
// The timeout exists for cold starts where no nodes exist yet. But leadership
// and resource store availability are absolute prerequisites — without them,
// reconcile would either mutate on a follower or crash on nil resources.
func (srv *server) waitForReadiness(ctx context.Context) bool {
	const (
		pollInterval = 5 * time.Second
		maxWait      = 60 * time.Second
	)
	deadline := time.Now().Add(maxWait)

	for {
		if ctx.Err() != nil {
			return false
		}

		hardUnmet, softUnmet := srv.checkReadinessTiered(ctx)

		// All predicates pass — ready.
		if len(hardUnmet) == 0 && len(softUnmet) == 0 {
			logger.Info("readiness-gate: all predicates passed")
			return true
		}

		// Hard predicates never timeout. If we're not the leader or the
		// resource store is down, we keep waiting indefinitely.
		if len(hardUnmet) > 0 {
			if time.Now().After(deadline) {
				// Log but do NOT proceed — hard predicates are absolute.
				logger.Warn("readiness-gate: hard predicates still unmet, continuing to wait",
					"hard_unmet", hardUnmet, "soft_unmet", softUnmet)
				// Reset deadline so we don't spam the log every poll.
				deadline = time.Now().Add(maxWait)
			}
			select {
			case <-ctx.Done():
				return false
			case <-time.After(pollInterval):
			}
			continue
		}

		// Only soft predicates remain unmet. Apply bounded timeout.
		if time.Now().After(deadline) {
			logger.Warn("readiness-gate: soft predicates timed out, proceeding (cold start safe)",
				"soft_unmet", softUnmet)
			return true
		}

		select {
		case <-ctx.Done():
			return false
		case <-time.After(pollInterval):
		}
	}
}

// checkReadinessTiered evaluates readiness predicates in two tiers.
// Hard predicates must always pass. Soft predicates can be timed out on cold start.
func (srv *server) checkReadinessTiered(ctx context.Context) (hardUnmet, softUnmet []string) {
	// ── Hard predicates (no timeout) ──

	// 1. Must be leader — followers must never reconcile.
	if !srv.isLeader() {
		hardUnmet = append(hardUnmet, "not_leader")
	}

	// 2. Resource store must be functional — nil resources = crash.
	if srv.resources != nil {
		if _, _, err := srv.resources.List(ctx, "ServiceDesiredVersion", ""); err != nil {
			hardUnmet = append(hardUnmet, "resource_store_unavailable")
		}
	}

	// ── Soft predicates (bounded timeout for cold start) ──

	// 3. At least one node has reported via heartbeat.
	srv.lock("readiness:nodes")
	nodeCount := len(srv.state.Nodes)
	hasHeartbeat := false
	for _, node := range srv.state.Nodes {
		if node != nil && !node.LastSeen.IsZero() {
			hasHeartbeat = true
			break
		}
	}
	srv.unlock()
	if nodeCount == 0 || !hasHeartbeat {
		softUnmet = append(softUnmet, "no_heartbeats")
	}

	// 4. Installed-state snapshot is readable.
	if _, err := installed_state.ListAllNodes(ctx, "SERVICE", ""); err != nil {
		softUnmet = append(softUnmet, "installed_state_unreadable")
	}

	// 5. Workflow service reachable.
	if srv.workflowClient == nil {
		softUnmet = append(softUnmet, "workflow_client_nil")
	}

	// 6. Repository resolution path reachable.
	repo := resolveRepositoryInfo()
	if repo.Address == "" {
		softUnmet = append(softUnmet, "repository_unresolvable")
	}

	// 7. Event client (soft, non-blocking — logged but doesn't block readiness).
	if srv.eventClient == nil {
		logger.Debug("readiness: event client unavailable (non-blocking)")
	}

	return hardUnmet, softUnmet
}

// checkReadiness evaluates all readiness predicates. Returns true if all pass,
// or false with a list of unmet predicate names.
func (srv *server) checkReadiness(ctx context.Context) (bool, []string) {
	var unmet []string

	// 1. Must be leader.
	if !srv.isLeader() {
		unmet = append(unmet, "not_leader")
	}

	// 2. At least one node has reported via heartbeat.
	srv.lock("readiness:nodes")
	nodeCount := len(srv.state.Nodes)
	hasHeartbeat := false
	for _, node := range srv.state.Nodes {
		if node != nil && !node.LastSeen.IsZero() {
			hasHeartbeat = true
			break
		}
	}
	srv.unlock()

	if nodeCount == 0 || !hasHeartbeat {
		unmet = append(unmet, "no_heartbeats")
	}

	// 3. Installed-state snapshot is readable.
	if _, err := installed_state.ListAllNodes(ctx, "SERVICE", ""); err != nil {
		unmet = append(unmet, "installed_state_unreadable")
	}

	// 4. Resource store is functional.
	if srv.resources != nil {
		if _, _, err := srv.resources.List(ctx, "ServiceDesiredVersion", ""); err != nil {
			unmet = append(unmet, "resource_store_unavailable")
		}
	}

	// 5. Workflow service reachable — required for dispatching release workflows.
	if srv.workflowClient == nil {
		unmet = append(unmet, "workflow_client_nil")
	}

	// 6. Repository resolution path reachable — required for resolving artifacts.
	repo := resolveRepositoryInfo()
	if repo.Address == "" {
		unmet = append(unmet, "repository_unresolvable")
	}

	// 7. Event/control-plane path not obviously degraded — check that the
	//    event client was connected at startup. If nil, the event bus is down.
	if srv.eventClient == nil {
		// Non-blocking: event bus unavailability is a soft signal, not a hard gate.
		// We still log it but don't block readiness — reconcile can proceed
		// without event publishing.
		logger.Debug("readiness: event client unavailable (non-blocking)")
	}

	return len(unmet) == 0, unmet
}

// reconcileDesiredWithInstalled removes ServiceDesiredVersion entries that
// no longer have a corresponding installed-state record. This cleans up
// desired entries for packages that were never truly installed (e.g. created
// by a previous bug that treated all repo artifacts as installed).
func (srv *server) reconcileDesiredWithInstalled(ctx context.Context) {
	if srv.resources == nil {
		return
	}

	// Collect installed service names from etcd.
	installedNames := make(map[string]bool)
	if pkgs, err := installed_state.ListAllNodes(ctx, "SERVICE", ""); err == nil {
		for _, pkg := range pkgs {
			canon := canonicalServiceName(pkg.GetName())
			if canon != "" {
				installedNames[canon] = true
			}
		}
	}

	// Check each desired entry against installed state.
	items, _, err := srv.resources.List(ctx, "ServiceDesiredVersion", "")
	if err != nil {
		return
	}
	removed := 0
	corrected := 0
	for _, obj := range items {
		sdv, ok := obj.(*cluster_controllerpb.ServiceDesiredVersion)
		if !ok || sdv.Spec == nil {
			continue
		}
		canon := canonicalServiceName(sdv.Spec.ServiceName)
		if canon == "" && sdv.Meta != nil {
			canon = canonicalServiceName(sdv.Meta.Name)
		}
		if canon == "" {
			continue
		}
		// Remove command-type entries from desired state — commands are
		// one-shot and should not persist as desired services.
		// NOTE: Do NOT remove services that are desired but not yet
		// installed — that is the "Planned" state, and the reconciler
		// needs the desired entry to generate an install plan.
		if strings.HasSuffix(canon, "-cmd") {
			if err := srv.resources.Delete(ctx, "ServiceDesiredVersion", canon); err == nil {
				removed++
			}
			continue
		}

		// NOTE: Auto-correcting desired version from installed was REMOVED.
		// Desired state (Layer 2) must only change via explicit operator action
		// (deploy command, SeedDesiredState RPC). Updating desired from
		// installed is an authority inversion (Layer 3 → Layer 2) and was a
		// vector for phantom service rematerialization.
		//
		// If installed version > desired version, the operator must explicitly
		// bump desired via 'globular deploy <pkg> --bump'.
	}
	if removed > 0 {
		logger.Info("reconcileDesiredWithInstalled: removed stale desired entries", "count", removed)
	}
	if corrected > 0 {
		logger.Info("reconcileDesiredWithInstalled: corrected version regressions", "count", corrected)
	}
}

// reconcileDesiredFromRepository ensures desired state tracks the latest
// published build for each service. This is the system contract: a publish
// action always produces convergence intent, regardless of which script or
// tool performed the publish.
func (srv *server) reconcileDesiredFromRepository(ctx context.Context) {
	if srv.resources == nil {
		return
	}

	items, _, err := srv.resources.List(ctx, "ServiceDesiredVersion", "")
	if err != nil {
		return
	}

	repoAddr := config.ResolveLocalServiceAddr("repository.PackageRepository")
	if repoAddr == "" {
		return
	}
	rc, err := repository_client.NewRepositoryService_Client(repoAddr, "repository.PackageRepository")
	if err != nil {
		return
	}
	defer rc.Close()

	// Fetch all artifacts once and index by canonical name.
	allArts, err := rc.ListArtifacts()
	if err != nil {
		return
	}

	updated := 0
	for _, obj := range items {
		sdv, ok := obj.(*cluster_controllerpb.ServiceDesiredVersion)
		if !ok || sdv.Spec == nil {
			continue
		}
		canon := canonicalServiceName(sdv.Spec.ServiceName)
		if canon == "" {
			continue
		}
		desiredVer := sdv.Spec.Version
		desiredBuild := sdv.Spec.BuildNumber

		// Find the highest build number for the desired version among all artifacts.
		var bestBuild int64
		for _, art := range allArts {
			ref := art.GetRef()
			if ref == nil {
				continue
			}
			artCanon := canonicalServiceName(ref.GetName())
			if artCanon == canon && ref.GetVersion() == desiredVer && art.GetBuildNumber() > bestBuild {
				bestBuild = art.GetBuildNumber()
			}
		}

		if bestBuild > desiredBuild {
			sdv.Spec.BuildNumber = bestBuild
			if sdv.Meta != nil {
				sdv.Meta.Generation++
			}
			if _, err := srv.resources.Apply(ctx, "ServiceDesiredVersion", sdv); err == nil {
				logger.Info("reconcileDesiredFromRepository: updated desired build",
					"service", canon, "version", desiredVer,
					"old_build", desiredBuild, "new_build", bestBuild)
				srv.ensureServiceRelease(ctx, canon, desiredVer, bestBuild)
				updated++
			}
		}
	}
	if updated > 0 {
		logger.Info("reconcileDesiredFromRepository: updated desired builds from repository", "count", updated)
	}
}

const controllerTargetBuildKey = "/globular/system/controller-target-build"

type controllerTargetBuild struct {
	Version     string `json:"version"`
	BuildNumber int64  `json:"build_number"`
	Checksum    string `json:"checksum"`
	SetAt       int64  `json:"set_at"`
	SetBy       string `json:"set_by"`
}

// reconcileControllerSelfUpdate checks if a newer controller build exists
// and coordinates a rolling update: followers first, leader last.
//
// Contract:
//   - /globular/system/controller-target-build is the control key (written by
//     deploy-service.sh or reconcileDesiredFromRepository). It is an intent:
//     "all controller instances should converge to this build."
//   - Followers detect the target, explicitly apply the package via the local
//     node-agent's ApplyPackageRelease RPC, and restart.
//   - The leader resigns ONLY when at least one follower satisfies ALL of:
//     (a) installed version+build matches target (verified via installed_state)
//     (b) heartbeat is fresh (< heartbeatStaleThreshold)
//     (c) node status is not unreachable/blocked
//   - If no safe successor exists, the leader keeps running and logs why.
func (srv *server) reconcileControllerSelfUpdate(ctx context.Context) {
	if srv.etcdClient == nil {
		return
	}

	target, err := srv.readControllerTargetBuild(ctx)
	if err != nil || target == nil || target.Version == "" {
		return
	}

	currentVer := Version
	currentBuild := parseBuildNumber()
	cmp, err := versionutil.CompareFull(currentVer, currentBuild, target.Version, target.BuildNumber)
	if err != nil {
		return
	}

	targetLabel := fmt.Sprintf("%s+%d", target.Version, target.BuildNumber)

	// This instance is already at or ahead of target.
	// If we're a follower, check whether the leader is still old — that's
	// the bootstrap deadlock: new follower ready, old leader holding lease.
	if cmp >= 0 {
		if !srv.isLeader() {
			srv.detectBootstrapHandoff(ctx, target, targetLabel)
		}
		return
	}

	// ── Follower path: explicitly apply the target package locally ──
	if !srv.isLeader() {
		srv.followerSelfApply(ctx, target)
		return
	}

	// ── Leader path: verify safe successor before resigning ──
	selfNodeID := srv.findSelfNodeID()
	safeSuccessors, totalFollowers, blockedReasons := srv.evaluateControllerFollowers(ctx, selfNodeID, target)

	if totalFollowers == 0 {
		logger.Info("controller-self-update: single-node cluster, skipping (release reconciler handles)")
		return
	}

	if safeSuccessors == 0 {
		// Log the specific blocking reasons for each follower.
		for id, reason := range blockedReasons {
			logger.Info("controller-self-update: follower not ready",
				"follower", id, "reason", reason, "target", targetLabel)
		}
		srv.emitClusterEvent("controller.self_update_pending", map[string]interface{}{
			"severity":        "WARNING",
			"action":          "waiting_for_safe_successor",
			"current_version": currentVer,
			"target":          targetLabel,
			"followers_total": totalFollowers,
			"blocked_reasons": blockedReasons,
		})
		return
	}

	// At least one safe successor confirmed — resign leadership.
	logger.Info("controller-self-update: safe successor confirmed, resigning",
		"safe_successors", safeSuccessors, "target", targetLabel)

	srv.emitClusterEvent("controller.self_update", map[string]interface{}{
		"severity":        "INFO",
		"action":          "leader_resigning",
		"current_version": currentVer,
		"target":          targetLabel,
		"safe_successors": safeSuccessors,
	})

	select {
	case srv.resignCh <- struct{}{}:
	default:
	}
}

// detectBootstrapHandoff checks whether the current leader is running an old
// controller binary while this follower is already at the target build. This is
// the first-deployment bootstrap deadlock: the old leader doesn't have the
// self-update protocol and will never resign on its own.
//
// When detected, a strong event is emitted telling the operator (or automation)
// to restart the controller on the leader node so leadership can transfer.
func (srv *server) detectBootstrapHandoff(ctx context.Context, target *controllerTargetBuild, targetLabel string) {
	selfNodeID := srv.findSelfNodeID()

	// Check every other node that has a control-plane profile and a controller
	// installed. Any node running an older controller build is a potential
	// old-leader candidate that needs a restart.
	for id, node := range srv.state.Nodes {
		if id == selfNodeID {
			continue
		}

		// Only check nodes that could be controllers (control-plane profile).
		isControlPlane := false
		for _, p := range node.Profiles {
			if p == "control-plane" {
				isControlPlane = true
				break
			}
		}
		if !isControlPlane {
			continue
		}

		pkg, err := installed_state.GetInstalledPackage(ctx, id, "SERVICE", "cluster-controller")
		if err != nil || pkg == nil {
			continue
		}

		nodeCmp, err := versionutil.CompareFull(pkg.GetVersion(), pkg.GetBuildNumber(), target.Version, target.BuildNumber)
		if err != nil || nodeCmp >= 0 {
			continue // this node is at or ahead of target
		}

		// This control-plane node is behind target. It may be the current leader.
		selfHostname := ""
		if n := srv.state.Nodes[selfNodeID]; n != nil {
			selfHostname = n.Identity.Hostname
		}

		logger.Warn("controller-self-update: BOOTSTRAP HANDOFF REQUIRED",
			"stale_controller_node", id,
			"stale_controller_hostname", node.Identity.Hostname,
			"stale_controller_version", fmt.Sprintf("%s+%d", pkg.GetVersion(), pkg.GetBuildNumber()),
			"target", targetLabel,
			"safe_successor", selfNodeID,
			"safe_successor_hostname", selfHostname,
			"action", "restart controller on stale node to allow self-update",
		)

		srv.emitClusterEvent("controller.bootstrap_handoff_required", map[string]interface{}{
			"severity":                "ERROR",
			"stale_node_id":           id,
			"stale_hostname":          node.Identity.Hostname,
			"stale_version":           fmt.Sprintf("%s+%d", pkg.GetVersion(), pkg.GetBuildNumber()),
			"target_version":          targetLabel,
			"safe_successor_node_id":  selfNodeID,
			"safe_successor_hostname": selfHostname,
			"action":                  "restart_controller_on_stale_node",
			"reason":                  "control-plane node runs old binary without self-update protocol; restart required for first rollout bootstrap",
		})
	}
}

// followerSelfApply is the explicit follower update path. It asks the local
// node-agent to download and install the target controller package via the
// ApplyPackageRelease RPC. This is not "node-agent will handle it somehow" —
// it is a direct, traceable call with specific package coordinates.
func (srv *server) followerSelfApply(ctx context.Context, target *controllerTargetBuild) {
	selfNodeID := srv.findSelfNodeID()
	targetLabel := fmt.Sprintf("%s+%d", target.Version, target.BuildNumber)

	// Check if we already applied this target (installed_state in etcd).
	if selfNodeID != "" {
		pkg, err := installed_state.GetInstalledPackage(ctx, selfNodeID, "SERVICE", "cluster-controller")
		if err == nil && pkg != nil {
			installedCmp, _ := versionutil.CompareFull(
				pkg.GetVersion(), pkg.GetBuildNumber(),
				target.Version, target.BuildNumber,
			)
			if installedCmp >= 0 {
				// Also verify checksum if both are available.
				if target.Checksum == "" || pkg.GetChecksum() == "" || pkg.GetChecksum() == target.Checksum {
					return // already at or ahead of target
				}
				logger.Info("controller-self-update: version+build match but checksum differs, re-applying",
					"installed_checksum", pkg.GetChecksum(), "target_checksum", target.Checksum)
			}
		}
	}

	logger.Info("controller-self-update: follower applying target package",
		"target", targetLabel, "self_node", selfNodeID)

	// Resolve the local node-agent endpoint. The agent runs on the same machine.
	selfNode := srv.state.Nodes[selfNodeID]
	if selfNode == nil || selfNode.AgentEndpoint == "" {
		logger.Warn("controller-self-update: cannot find local node-agent endpoint")
		return
	}

	agent, err := srv.getAgentClient(ctx, selfNode.AgentEndpoint)
	if err != nil {
		logger.Warn("controller-self-update: cannot reach local node-agent",
			"endpoint", selfNode.AgentEndpoint, "err", err)
		return
	}

	// Explicit apply: package name, version, build, optional checksum.
	resp, err := agent.ApplyPackageRelease(ctx, &node_agentpb.ApplyPackageReleaseRequest{
		PackageName:    "cluster-controller",
		PackageKind:    "SERVICE",
		Version:        target.Version,
		BuildNumber:    target.BuildNumber,
		ExpectedSha256: target.Checksum, // strongest signal when available
		Force:          true,            // bypass idempotency — we want this exact build
	})
	if err != nil {
		logger.Warn("controller-self-update: ApplyPackageRelease failed",
			"target", targetLabel, "err", err)
		srv.emitClusterEvent("controller.self_update_apply_failed", map[string]interface{}{
			"severity": "WARNING",
			"node_id":  selfNodeID,
			"target":   targetLabel,
			"error":    err.Error(),
		})
		return
	}

	logger.Info("controller-self-update: follower apply dispatched",
		"target", targetLabel, "response_ok", resp.GetOk(),
		"checksum", resp.GetChecksum())
	// The node-agent will install the package and restart the controller unit.
	// On restart, this function will find cmp >= 0 and return early.
}

// evaluateControllerFollowers checks each follower's readiness to take over
// leadership after a controller self-update. Returns the count of safe
// successors, total followers, and a map of nodeID→reason for non-ready nodes.
func (srv *server) evaluateControllerFollowers(ctx context.Context, selfNodeID string, target *controllerTargetBuild) (safeSuccessors, totalFollowers int, blockedReasons map[string]string) {
	blockedReasons = make(map[string]string)

	for id, node := range srv.state.Nodes {
		if id == selfNodeID {
			continue
		}
		totalFollowers++

		// Predicate 1: node must be reachable (fresh heartbeat).
		if time.Since(node.LastSeen) >= heartbeatStaleThreshold {
			blockedReasons[id] = fmt.Sprintf("stale heartbeat (%s ago)", time.Since(node.LastSeen).Truncate(time.Second))
			continue
		}

		// Predicate 2: node must not be unreachable/blocked.
		if node.Status == "unreachable" || node.Status == "blocked" {
			blockedReasons[id] = fmt.Sprintf("node status is %s", node.Status)
			continue
		}

		// Predicate 3: installed controller version+build must match target.
		// Use installed_state from etcd (source of truth), not just heartbeat InstalledVersions.
		pkg, err := installed_state.GetInstalledPackage(ctx, id, "SERVICE", "cluster-controller")
		if err != nil || pkg == nil {
			blockedReasons[id] = "no installed-state record for cluster-controller"
			continue
		}

		installedCmp, err := versionutil.CompareFull(
			pkg.GetVersion(), pkg.GetBuildNumber(),
			target.Version, target.BuildNumber,
		)
		if err != nil || installedCmp < 0 {
			blockedReasons[id] = fmt.Sprintf("installed %s+%d, target %s+%d",
				pkg.GetVersion(), pkg.GetBuildNumber(), target.Version, target.BuildNumber)
			continue
		}

		// Predicate 4: if target has a checksum, installed must also have one and it must match.
		// No fallback to version+build-only when target checksum is known.
		if target.Checksum != "" {
			if pkg.GetChecksum() == "" {
				blockedReasons[id] = "installed checksum missing, target checksum required"
				continue
			}
			if pkg.GetChecksum() != target.Checksum {
				blockedReasons[id] = fmt.Sprintf("checksum mismatch: installed %s, target %s",
					pkg.GetChecksum(), target.Checksum)
				continue
			}
		}

		// All predicates pass — this follower is a safe successor.
		safeSuccessors++
	}
	return
}

func (srv *server) readControllerTargetBuild(ctx context.Context) (*controllerTargetBuild, error) {
	resp, err := srv.etcdClient.Get(ctx, controllerTargetBuildKey)
	if err != nil || len(resp.Kvs) == 0 {
		return nil, err
	}
	var target controllerTargetBuild
	if err := json.Unmarshal(resp.Kvs[0].Value, &target); err != nil {
		return nil, err
	}
	return &target, nil
}

func (srv *server) findSelfNodeID() string {
	advertiseIP, err := config.GetRoutableIP()
	if err != nil || advertiseIP == "" {
		return ""
	}
	for id, node := range srv.state.Nodes {
		if node.AgentEndpoint != "" && strings.Contains(node.AgentEndpoint, advertiseIP) {
			return id
		}
		for _, ip := range node.Identity.Ips {
			if ip == advertiseIP {
				return id
			}
		}
	}
	return ""
}
