package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"github.com/globulario/services/golang/cluster_controller/cluster_controller_server/operator"
	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func (srv *server) reconcileNodes(ctx context.Context) {
	if !srv.mustBeLeader() {
		return
	}
	if !srv.reconcileRunning.CompareAndSwap(false, true) {
		return
	}
	defer srv.reconcileRunning.Store(false)
	if srv.kv == nil {
		return
	}
	desiredNetworkObj, err := srv.loadDesiredNetwork(ctx)
	if err != nil {
		log.Printf("reconcile: load desired network failed: %v", err)
	}
	var desiredNet *cluster_controllerpb.DesiredNetwork
	specHash := ""
	if desiredNetworkObj != nil {
		desiredNet = &cluster_controllerpb.DesiredNetwork{
			Domain:           desiredNetworkObj.Spec.GetClusterDomain(),
			Protocol:         desiredNetworkObj.Spec.GetProtocol(),
			PortHttp:         desiredNetworkObj.Spec.GetPortHttp(),
			PortHttps:        desiredNetworkObj.Spec.GetPortHttps(),
			AlternateDomains: append([]string(nil), desiredNetworkObj.Spec.GetAlternateDomains()...),
			AcmeEnabled:      desiredNetworkObj.Spec.GetAcmeEnabled(),
			AdminEmail:       desiredNetworkObj.Spec.GetAdminEmail(),
		}
		if h, herr := hashDesiredNetwork(desiredNet); herr == nil {
			specHash = h
		} else {
			log.Printf("reconcile: hash desired network: %v", herr)
		}
	}
	srv.lock("reconcile:snapshot")
	nodes := make([]*nodeState, 0, len(srv.state.Nodes))
	for _, node := range srv.state.Nodes {
		nodes = append(nodes, node)
	}
	stateDirty := srv.cleanupJoinStateLocked(time.Now())
	// Re-seed the configured join token if it was cleaned up or never persisted.
	// This makes the token durable across controller restarts and reconcile cycles.
	if tok := strings.TrimSpace(srv.cfg.JoinToken); tok != "" {
		if existing := srv.state.JoinTokens[tok]; existing == nil || existing.Uses >= existing.MaxUses {
			if srv.state.JoinTokens == nil {
				srv.state.JoinTokens = make(map[string]*joinTokenRecord)
			}
			srv.state.JoinTokens[tok] = &joinTokenRecord{
				Token:     tok,
				ExpiresAt: time.Now().Add(7 * 24 * time.Hour),
				MaxUses:   100,
			}
			stateDirty = true
		}
	}

	// INVARIANT ENFORCEMENT: Run all cluster invariants during the snapshot
	// phase. This is the last line of defense — runs without depending on
	// MinIO or the workflow service.
	if srv.enforceAllInvariantsLocked() {
		stateDirty = true
	}
	workflowRepair := srv.workflowRepairNeeded
	srv.workflowRepairNeeded = nil

	srv.unlock()

	// Repair missing workflows AFTER releasing the lock (does MinIO I/O).
	if len(workflowRepair) > 0 {
		srv.repairMissingWorkflows(ctx, workflowRepair)
	}
	now := time.Now()

	// Pre-reconcile phase 1: drive bootstrap phase state machine.
	// Nodes progress through: admitted → infra_preparing → etcd_joining →
	// etcd_ready → xds_ready → envoy_ready → workload_ready.
	if bootDirty := reconcileBootstrapPhases(nodes, srv); bootDirty {
		stateDirty = true
	}

	// etcd, ScyllaDB, and MinIO join phases are now driven by
	// cluster.reconcile workflow scan_drift action (see reconcile_actions.go).

	for _, node := range nodes {
		if node == nil || node.NodeID == "" {
			continue
		}

		// ── Recovery fencing guard (Invariant 2) ──────────────────────────────
		// A node under active full-reseed recovery must not be touched by the
		// normal reconciler. The recovery workflow owns all installed-state
		// mutations for that node until it completes or fails.
		if srv.isNodeUnderRecovery(ctx, node.NodeID) {
			log.Printf("reconcile: skip node %s — active full-reseed recovery workflow owns this node", node.NodeID)
			continue
		}

		// Bootstrap phase gating: nodes not yet workload_ready get infra-tier-only
		// plans so infrastructure packages (etcd, xds, envoy, etc.) get installed.
		// Workload service plans are blocked until the node reaches workload_ready.
		infraOnly := false
		if !bootstrapPhaseReady(node.BootstrapPhase) {
			infraOnly = true
		}
		// Validate profiles before any dispatch — unknown profiles block the node.
		actions, profileErr := buildPlanActions(node.Profiles)
		if profileErr != nil {
			node.Status = "blocked"
			node.BlockedReason = "unknown_profile"
			node.BlockedDetails = profileErr.Error()
			stateDirty = true
			log.Printf("reconcile: node %s blocked: %v", node.NodeID, profileErr)
			srv.emitClusterEvent("plan_blocked", map[string]interface{}{
				"severity":       "WARN",
				"node_id":        node.NodeID,
				"hostname":       node.Identity.Hostname,
				"message":        fmt.Sprintf("Node %s blocked: unknown profile", node.Identity.Hostname),
				"correlation_id": fmt.Sprintf("plan:%s:gen:0", node.NodeID),
			})
			continue
		}
		// Clear stale unknown_profile block now that profiles are valid.
		if node.BlockedReason == "unknown_profile" {
			node.BlockedReason = ""
			node.BlockedDetails = ""
			if node.Status == "blocked" {
				node.Status = "converging"
			}
			stateDirty = true
		}

		// Bootstrap infra-only filter: during infra_preparing, restrict to
		// infrastructure-tier units only so workload units aren't started early.
		if infraOnly {
			actions = filterActionsByMaxTier(actions, TierInfrastructure)
		}

		// Phase 3: Capability gating — desired units must be installed on the node.
		// Hard-gate when inventory_complete=true; soft-gate (warn only) otherwise.
		if len(node.Units) > 0 {
			desiredUnitNames := desiredUnitsFromActions(actions)
			if missing := missingInstalledUnits(desiredUnitNames, node.Units); len(missing) > 0 {
				if node.InventoryComplete {
					// Full inventory reported — hard block.
					node.Status = "blocked"
					node.BlockedReason = "missing_units"
					node.BlockedDetails = fmt.Sprintf("missing: %s", strings.Join(missing, ", "))
					stateDirty = true
					log.Printf("reconcile: node %s blocked (hard): missing units: %v", node.NodeID, missing)
					srv.emitClusterEvent("plan_blocked", map[string]interface{}{
						"severity":       "WARN",
						"node_id":        node.NodeID,
						"hostname":       node.Identity.Hostname,
						"message":        fmt.Sprintf("Node %s blocked: missing unit files %v", node.Identity.Hostname, missing),
						"correlation_id": fmt.Sprintf("plan:%s:gen:0", node.NodeID),
					})
					continue
				}
				// Inventory not complete — soft mode: warn but allow reconcile to proceed.
				log.Printf("reconcile: node %s soft-warn: possibly missing units (inventory incomplete): %v", node.NodeID, missing)
			} else if node.InventoryComplete {
				// Full inventory present and all units confirmed — clear stale missing_units block.
				if node.BlockedReason == "missing_units" {
					node.BlockedReason = ""
					node.BlockedDetails = ""
					if node.Status == "blocked" {
						node.Status = "converging"
					}
					stateDirty = true
				}
			}
		}

		// Phase 4: Privileged-apply gating — when the node lacks privilege to
		// write systemd units, skip plan dispatch and record the state so the
		// UI can show "Awaiting privileged apply".
		canPriv := node.Capabilities != nil && node.Capabilities.CanApplyPrivileged
		if !canPriv {
			log.Printf("reconcile: node %s lacks privileged-apply capability", node.NodeID)
		}

		appliedHash, err := srv.getNodeAppliedHash(ctx, node.NodeID)
		if err != nil {
			log.Printf("reconcile: read applied hash for %s: %v", node.NodeID, err)
			continue
		}
		// Network reconciliation is now workflow-native; the legacy plan-slot
		// comparison branches have been removed. Drift detection happens in
		// reconcile workflows, not in controller-side imperative logic.
		if specHash != "" && appliedHash != specHash {
			log.Printf("reconcile: network config drift for %s — workflow will converge", node.NodeID)
			continue
		}

		// Day 1 intent resolution: resolve the node's desired component set
		// from its profiles + catalog, then scope desired services accordingly.
		intent, intentErr := ResolveNodeIntent(node.NodeID, node.Profiles, node.Units, node.InstalledVersions)
		if intentErr != nil {
			log.Printf("reconcile: node %s intent resolution failed: %v", node.NodeID, intentErr)
			node.Day1Phase = Day1PackageMetadataInvalid
			node.Day1PhaseReason = intentErr.Error()
			stateDirty = true
			// Non-fatal: fall through with nil intent (backward compat — no filtering).
		} else {
			node.ResolvedIntent = intent
			stateDirty = true
		}

		// Compute Day 1 lifecycle phase from current state.
		d1Phase, d1Reason := ComputeDay1Phase(node)
		if node.Day1Phase != d1Phase || node.Day1PhaseReason != d1Reason {
			node.Day1Phase = d1Phase
			node.Day1PhaseReason = d1Reason
			stateDirty = true
		}

		// Services reconciliation — load desired state then auto-materialize
		// missing infra entries required by the node's resolved intent.
		desiredCanon, desiredObjs, err := srv.loadDesiredServices(ctx)
		if err != nil {
			log.Printf("reconcile: load desired services failed: %v", err)
			desiredCanon = map[string]string{}
		}

		// Infra release retry/enqueue moved outside the per-node loop to avoid
		// re-enqueuing all 16 infra releases once per node. See post-loop block.

		// Day 1 infra materialization: if the node's resolved intent requires
		// components not in desired state, create them now.
		if intent != nil && srv.resources != nil {
			mat := srv.materializeMissingInfraDesired(ctx, intent, desiredCanon)
			if len(mat) > 0 {
				names := make([]string, len(mat))
				for i, m := range mat {
					names[i] = fmt.Sprintf("%s@%s(%s)", m.Component, m.Version, m.Source)
				}
				log.Printf("reconcile: node %s: materialized %d missing desired entries: %s",
					node.NodeID, len(mat), strings.Join(names, ", "))
				intent.MaterializedDesired = mat
				// Re-load desired services to include newly created entries.
				desiredCanon, desiredObjs, err = srv.loadDesiredServices(ctx)
				if err != nil {
					log.Printf("reconcile: reload desired services failed: %v", err)
					desiredCanon = map[string]string{}
				}
				stateDirty = true
			}
		}
		// During bootstrap, restrict to infrastructure-tier services only
		// so workload services aren't installed before the node is ready.
		if infraOnly {
			infraCanon := make(map[string]string)
			for svc, ver := range desiredCanon {
				unit := serviceUnitForCanonical(svc)
				if getUnitTier(unit) <= TierInfrastructure {
					infraCanon[svc] = ver
				}
			}
			desiredCanon = infraCanon
		}

		// Scope desired services to this node's resolved intent (profile-driven).
		desiredCanon = FilterDesiredByIntent(desiredCanon, intent)

		// Gate workloads on runtime dependency health: block services whose
		// local deps (e.g. scylladb for ai-memory) are not yet active.
		var blockedWorkloads []BlockedWorkload
		if !infraOnly {
			desiredCanon, blockedWorkloads = GateDependencies(desiredCanon, node.Units, node.InstalledVersions)
		}

		// Update observability fields from dependency gating.
		if len(blockedWorkloads) > 0 {
			names := make([]string, len(blockedWorkloads))
			for i, bw := range blockedWorkloads {
				names[i] = fmt.Sprintf("%s(%s)", bw.Name, bw.Reason)
			}
			node.BlockedReason = "dependency_not_ready"
			node.BlockedDetails = strings.Join(names, "; ")
			stateDirty = true
			log.Printf("reconcile: node %s: %d workloads gated on deps: %s", node.NodeID, len(blockedWorkloads), node.BlockedDetails)
		} else if node.BlockedReason == "dependency_not_ready" {
			node.BlockedReason = ""
			node.BlockedDetails = ""
			stateDirty = true
		}

		filtered, _ := computeServiceDelta(desiredCanon, node.Units)
		// Removal now flows through the release pipeline (REMOVING → REMOVED).
		// The ad-hoc removal block has been removed.
		svcHash := stableServiceDesiredHash(filtered)
		if svcHash == "" {
			continue
		}
		appliedSvcHash, err := srv.getNodeAppliedServiceHash(ctx, node.NodeID)
		if err != nil {
			log.Printf("reconcile: read applied service hash for %s: %v", node.NodeID, err)
			continue
		}
		if len(filtered) == 0 {
			continue
		}
		if svcHash == appliedSvcHash {
			continue
		}
		// External install detection: if all desired services are reported as
		// installed at the correct version (e.g. via CLI), update the applied
		// hash without requiring a plan to succeed. This handles the case where
		// services were installed outside the plan system.
		if len(node.InstalledVersions) > 0 && len(filtered) > 0 {
			allMatch := true
			for svc, ver := range filtered {
				installedVer := ""
				// InstalledVersions keys are "publisher/service" or just "service"
				for k, v := range node.InstalledVersions {
					parts := strings.SplitN(k, "/", 2)
					candidate := k
					if len(parts) == 2 {
						candidate = parts[1]
					}
					if canonicalServiceName(candidate) == canonicalServiceName(svc) {
						installedVer = v
						break
					}
				}
				if installedVer != ver {
					allMatch = false
					break
				}
			}
			if allMatch {
				log.Printf("reconcile: external install detected node=%s — all %d desired services match installed versions, updating applied hash", node.NodeID, len(filtered))
				if err := srv.putNodeAppliedServiceHash(ctx, node.NodeID, svcHash); err != nil {
					log.Printf("reconcile: store applied service hash for %s: %v", node.NodeID, err)
				}
				_ = srv.putNodeFailureCountServices(ctx, node.NodeID, 0)
				// (EXTERNAL_INSTALL_DETECTED removed — not in required event set)
				continue
			}
		}
		// Service reconciliation is workflow-native via ServiceRelease objects.
		// The legacy plan-slot status inspection has been removed.
		_ = srv.getNodeFailureCountServices

		// Pick the next service that actually needs installation. Skip services
		// already installed at the desired version so we don't loop forever on
		// already-converged services while others remain uninstalled.
		// Also skip services managed by the release reconciler (have a ServiceRelease).
		svcNames := make([]string, 0, len(filtered))
		for name, ver := range filtered {
			// Skip services managed by the release reconciler.
			if srv.resources != nil {
				relKey := defaultPublisherID() + "/" + canonicalServiceName(name)
				if obj, _, _ := srv.resources.Get(ctx, "ServiceRelease", relKey); obj != nil {
					continue
				}
			}
			// Prefer build_id comparison — immune to version string confusion.
			if sdv := desiredObjs[name]; sdv != nil && sdv.Spec != nil && sdv.Spec.BuildID != "" {
				installedBID := node.InstalledBuildIDs[name]
				if installedBID != sdv.Spec.BuildID {
					svcNames = append(svcNames, name)
				}
				continue
			}
			// Fallback to version comparison when build_id not available.
			installedVer := lookupInstalledVersionFromMap(node.InstalledVersions, name)
			if installedVer != ver {
				svcNames = append(svcNames, name)
			}
		}
		if len(svcNames) == 0 {
			// All desired services are installed — store applied hash and move on.
			if err := srv.putNodeAppliedServiceHash(ctx, node.NodeID, svcHash); err != nil {
				log.Printf("reconcile: store applied service hash for %s: %v", node.NodeID, err)
			}
			continue
		}
		sort.Strings(svcNames)
		failsSvc, _ := srv.getNodeFailureCountServices(ctx, node.NodeID)
		svcName := svcNames[int(failsSvc)%len(svcNames)]
		version := filtered[svcName]
		if blockUntil, ok := srv.serviceBlock[svcName]; ok && now.Before(blockUntil) {
			continue
		}
		op := operator.Get(canonicalServiceName(svcName))
		decision, err := op.AdmitPlan(ctx, operator.AdmitRequest{
			Service:        canonicalServiceName(svcName),
			NodeID:         node.NodeID,
			DesiredVersion: version,
			DesiredHash:    svcHash,
		})
		if err != nil {
			log.Printf("reconcile: operator admit %s on %s failed: %v", svcName, node.NodeID, err)
			continue
		}
		if !decision.Allowed {
			if decision.RequeueAfterSeconds > 0 {
				srv.serviceBlock[svcName] = now.Add(time.Duration(decision.RequeueAfterSeconds) * time.Second)
			}
			continue
		}
		// Resolve the artifact digest from the repository so the plan can
		// verify the download. The desired-state hash (svcHash) is NOT an
		// artifact SHA256 — we must look up the actual artifact checksum.
		// When resolution fails (key format mismatch, repo unavailable),
		// pass "skip" to signal the fetcher to download without
		// pre-verification while still computing the hash post-download.
		// Extract build number from desired state if available.
		var desiredBuildNumber int64
		if obj, ok := desiredObjs[svcName]; ok && obj != nil && obj.Spec != nil {
			desiredBuildNumber = obj.Spec.BuildNumber
		}

		artifactDigest := ""
		resolver := &ReleaseResolver{RepositoryAddr: resolveRepositoryInfo().Address}
		resolved, err := resolver.Resolve(ctx, &cluster_controllerpb.ServiceReleaseSpec{
			ServiceName: canonicalServiceName(svcName),
			Version:     version,
			PublisherID: defaultPublisherID(),
			Platform:    srv.getNodePlatform(node.NodeID),
			BuildNumber: desiredBuildNumber,
		})
		if err != nil {
			log.Printf("reconcile: resolve artifact %s@%s: %v (plan will skip digest pre-check)", svcName, version, err)
		} else if resolved != nil {
			artifactDigest = resolved.Digest
			if resolved.BuildNumber > 0 {
				desiredBuildNumber = resolved.BuildNumber
			}
		}
		// Service upgrade handled by workflow-native release pipeline.
		_ = artifactDigest
		_ = desiredBuildNumber
		_ = op
		log.Printf("reconcile: service %s on %s — handled by release pipeline workflow", svcName, node.NodeID)
	}

	// Infra release retry/enqueue: run once after the per-node loop with a
	// cooldown to avoid re-enqueuing all infra releases every 30 seconds.
	if srv.resources != nil {
		srv.infraRetryOnce(ctx)
	}

	// Keep /globular/cluster/scylla/hosts in sync with approved storage nodes.
	// Idempotent: only writes when the list changes.
	srv.publishScyllaHostsIfNeeded(ctx)

	if stateDirty {
		srv.lock("reconcile:persist")
		func() {
			defer srv.unlock()
			if err := srv.persistStateLocked(true); err != nil {
				log.Printf("persist state: %v", err)
			}
		}()
	}
}

// ---------------------------------------------------------------------------
// Day 1 infra materialization — auto-create missing desired-state entries
// ---------------------------------------------------------------------------

// MaterializedInfra records which infra desired-state entries were auto-created.
type MaterializedInfra struct {
	Component string `json:"component"`
	Version   string `json:"version"`
	Source    string `json:"source"` // "installed:<node_id>" | "bootstrap_default"
}

// materializeMissingInfraDesired checks if the node's resolved intent requires
// infra components that have no desired-state entry, and creates them.
//
// Scope: only materializes INFRASTRUCTURE components and workload components
// that are runtime-local-dependencies of already-desired services. Does NOT
// auto-materialize all workloads in the profile — those come from operator
// seed or explicit desired-state commands.
func (srv *server) materializeMissingInfraDesired(ctx context.Context, intent *NodeIntent, desiredCanon map[string]string) []MaterializedInfra {
	if intent == nil || srv.resources == nil {
		return nil
	}

	// Build set of components that are runtime deps of already-desired services.
	runtimeDepsOfDesired := make(map[string]bool)
	for svc := range desiredCanon {
		canon := normalizeComponentName(canonicalServiceName(svc))
		comp := CatalogByName(canon)
		if comp == nil {
			continue
		}
		for _, dep := range comp.RuntimeLocalDependencies {
			runtimeDepsOfDesired[dep] = true
		}
	}

	var materialized []MaterializedInfra

	for _, compName := range intent.ResolvedComponents {
		comp := CatalogByName(compName)
		if comp == nil {
			continue
		}

		// Only materialize: (a) infrastructure components, (b) command packages, or
		// (c) workload components that are runtime deps of already-desired services.
		if comp.Kind != KindInfrastructure && comp.Kind != KindCommand && !runtimeDepsOfDesired[compName] {
			continue
		}

		// Skip day0_join infrastructure — these are managed by dedicated
		// bootstrap/join state machines (e.g. etcd member-add), not the
		// artifact pipeline. Creating InfrastructureRelease for them would
		// cause the node-agent to attempt an artifact-based install.
		if comp.Kind == KindInfrastructure && comp.InstallMode == InstallModeDay0Join {
			log.Printf("materialize-infra: skipping %s (install_mode=day0_join, managed by join logic)", compName)
			continue
		}

		// Check if already in desired state.
		if _, ok := desiredCanon[compName]; ok {
			continue
		}

		// Resolve version: check installed versions across all nodes.
		version, source := srv.resolveInfraVersion(compName)

		if comp.Kind == KindInfrastructure || comp.Kind == KindCommand {
			// Version must be resolved — never create InfrastructureRelease
			// with a fake version. If unresolved, the infra/command is not yet
			// installed anywhere and cannot be materialized.
			if version == "" {
				log.Printf("materialize-infra: skipping %s — version unresolved (not installed on any node)", compName)
				continue
			}

			// Create InfrastructureRelease (used for both infra and command packages).
			relName := defaultPublisherID() + "/" + compName
			existing, _, _ := srv.resources.Get(ctx, "InfrastructureRelease", relName)
			if existing != nil {
				continue // already exists
			}
			obj := &cluster_controllerpb.InfrastructureRelease{
				Meta: &cluster_controllerpb.ObjectMeta{Name: relName},
				Spec: &cluster_controllerpb.InfrastructureReleaseSpec{
					PublisherID: defaultPublisherID(),
					Component:   compName,
					Version:     version,
				},
				Status: &cluster_controllerpb.InfrastructureReleaseStatus{},
			}
			if _, err := srv.resources.Apply(ctx, "InfrastructureRelease", obj); err != nil {
				log.Printf("materialize-infra: failed to create InfrastructureRelease %s: %v", relName, err)
				continue
			}
			log.Printf("materialize-infra: created InfrastructureRelease %s version=%s source=%s", relName, version, source)
			materialized = append(materialized, MaterializedInfra{Component: compName, Version: version, Source: source})
		} else {
			// Workload services: do NOT auto-materialize desired state from
			// installed state. Desired state (Layer 2) must only come from
			// explicit deploy commands, never inferred from Layer 3.
			// The heartbeat already tracks what is installed on each node.
			log.Printf("materialize-infra: skipping workload %s — desired state must come from explicit deploy, not auto-materialization", compName)
		}
	}
	return materialized
}

// resolveInfraVersion determines the version to use for auto-materialized infra.
//
// Resolution order:
//  1. Installed-state registry on existing cluster nodes — prefer a version
//     that is actually running in the cluster.
//  2. Desired infra state / seeded InfrastructureRelease — if installed-state
//     is missing, use the version from the desired release.
//  3. Return ("", "unresolved") with structured reason.
//
// Note: 0.0.1 is a legitimate version for Day-0 built packages (minio, xds,
// gateway, etc.) that don't carry an upstream version. It must not be rejected.
func (srv *server) resolveInfraVersion(componentName string) (version, source string) {
	// Step 1: Check installed-state across all cluster nodes.
	// Reject "unknown" and "" — these are fallback/placeholder versions
	// from nodes that haven't been properly provisioned. Using them would
	// create desired entries with fake versions.
	srv.lock("resolveInfraVersion")
	for nodeID, node := range srv.state.Nodes {
		if node == nil || len(node.InstalledVersions) == 0 {
			continue
		}
		for k, v := range node.InstalledVersions {
			canon := normalizeComponentName(canonicalServiceName(k))
			if canon == componentName && v != "" && v != "unknown" {
				srv.unlock()
				return v, "installed:" + nodeID
			}
		}
	}
	srv.unlock()

	// Step 2: Check existing InfrastructureRelease desired state.
	if srv.resources != nil {
		// Try both key formats: bare name and publisher/name.
		for _, relName := range []string{
			componentName,
			defaultPublisherID() + "/" + componentName,
		} {
			obj, _, err := srv.resources.Get(context.Background(), "InfrastructureRelease", relName)
			if err != nil || obj == nil {
				continue
			}
			if rel, ok := obj.(*cluster_controllerpb.InfrastructureRelease); ok && rel.Spec != nil && rel.Spec.Version != "" {
				return rel.Spec.Version, "desired-release:" + relName
			}
		}
	}

	return "", "unresolved"
}

// infraRetryOnce runs retryFailedInfraReleases + enqueueInfraReleases at most
// once per 60-second window. Prevents the per-reconcile-cycle call from
// flooding the work queue with all 16 infra releases every 30 seconds.
func (srv *server) infraRetryOnce(ctx context.Context) {
	const infraRetryCooldown = 60 * time.Second
	now := time.Now()
	if now.Sub(srv.lastInfraRetry) < infraRetryCooldown {
		return
	}
	srv.lastInfraRetry = now
	srv.retryFailedInfraReleases(ctx)
	srv.enqueueInfraReleases()
}

// retryFailedInfraReleases scans InfrastructureRelease objects and resets any
// in FAILED status back to PENDING, but only if there are unserved nodes that
// still need the package. This avoids blindly resetting all FAILED releases
// which amplifies the reconcile storm.
func (srv *server) retryFailedInfraReleases(ctx context.Context) {
	if !srv.mustBeLeader() {
		return
	}
	items, _, err := srv.resources.List(ctx, "InfrastructureRelease", "")
	if err != nil {
		return
	}
	for _, obj := range items {
		rel, ok := obj.(*cluster_controllerpb.InfrastructureRelease)
		if !ok || rel.Status == nil || rel.Meta == nil {
			continue
		}
		if rel.Status.Phase != cluster_controllerpb.ReleasePhaseFailed {
			continue
		}
		// Respect backoff: only retry if enough time has passed since the
		// last transition. Without this, every reconcile cycle resets FAILED
		// infra releases immediately.
		if rel.Status.LastTransitionUnixMs > 0 {
			elapsed := time.Since(time.UnixMilli(rel.Status.LastTransitionUnixMs))
			if elapsed < releaseRetryBackoff {
				continue
			}
		}
		// Only retry if there are unserved nodes that need this package.
		h := srv.infraReleaseHandle(rel)
		if !srv.hasUnservedNodes(h) {
			continue
		}
		// Bump generation to trigger FAILED → PENDING transition.
		rel.Meta.Generation++
		rel.Status.Phase = cluster_controllerpb.ReleasePhasePending
		rel.Status.Message = "retrying after failure (unserved nodes)"
		rel.Status.TransitionReason = "auto_retry"
		if _, err := srv.resources.Apply(ctx, "InfrastructureRelease", rel); err != nil {
			log.Printf("retryFailedInfraReleases: failed to reset %s: %v", rel.Meta.Name, err)
			continue
		}
		log.Printf("retryFailedInfraReleases: reset %s from FAILED → PENDING (gen=%d)", rel.Meta.Name, rel.Meta.Generation)
	}
}

// enqueueInfraReleases triggers re-processing of InfrastructureRelease objects
// so the release pipeline can detect new unserved nodes and dispatch plans.
//
// Releases are enqueued in priority order from the service catalog:
//   - Critical control plane (etcd, xds, workflow, event, repository) first
//   - Foundational infra (minio, scylladb, gateway, envoy) second
//   - Everything else last
//
// Not all infrastructure has equal blast radius — etcd must converge before
// gateway, and gateway before workloads.
func (srv *server) enqueueInfraReleases() {
	if srv.resources == nil || srv.infraReleaseEnqueue == nil {
		return
	}
	items, _, err := srv.resources.List(context.Background(), "InfrastructureRelease", "")
	if err != nil {
		return
	}

	// Collect releases with their catalog priority for sorting.
	type prioritizedRelease struct {
		name     string
		priority int
	}
	releases := make([]prioritizedRelease, 0, len(items))
	for _, obj := range items {
		rel, ok := obj.(*cluster_controllerpb.InfrastructureRelease)
		if !ok || rel.Meta == nil {
			continue
		}
		pri := 999 // default: lowest priority
		if rel.Spec != nil {
			if entry := CatalogByName(rel.Spec.Component); entry != nil {
				pri = entry.Priority
			}
		}
		releases = append(releases, prioritizedRelease{name: rel.Meta.Name, priority: pri})
	}

	sort.Slice(releases, func(i, j int) bool {
		return releases[i].priority < releases[j].priority
	})

	for _, r := range releases {
		srv.infraReleaseEnqueue(r.name)
	}
}

// enqueueReleasesForConvergingNodes re-enqueues all ServiceRelease objects
// when there are bootstrap-ready nodes that are still converging. This closes
// the Day-1 gap where a new node finishes infra install (hash stabilises) but
// the AVAILABLE ServiceReleases are never re-enqueued to dispatch workloads,
// because the hash-change trigger in ReportNodeStatus only fires for nodes
// already tracked in a release's Status.Nodes.
func (srv *server) enqueueReleasesForConvergingNodes(ctx context.Context) {
	if srv.resources == nil || srv.releaseEnqueue == nil {
		return
	}

	// Check if any bootstrap-ready node is still converging (not "ready").
	// If all nodes are ready, no sweep needed.
	srv.lock("enqueueReleasesForConvergingNodes")
	hasConverging := false
	for _, node := range srv.state.Nodes {
		if bootstrapPhaseReady(node.BootstrapPhase) && node.Status == "converging" {
			hasConverging = true
			break
		}
	}
	srv.unlock()

	if !hasConverging {
		return
	}

	items, _, err := srv.resources.List(ctx, "ServiceRelease", "")
	if err != nil {
		return
	}
	count := 0
	for _, obj := range items {
		rel, ok := obj.(*cluster_controllerpb.ServiceRelease)
		if !ok || rel.Meta == nil {
			continue
		}
		srv.releaseEnqueue(rel.Meta.Name)
		count++
	}
	if count > 0 {
		log.Printf("enqueueReleasesForConvergingNodes: enqueued %d service releases for converging node check", count)
	}
}

func backoffDuration(fails int) time.Duration {
	switch {
	case fails <= 0:
		return 0
	case fails == 1:
		return 5 * time.Second
	case fails == 2:
		return 15 * time.Second
	case fails == 3:
		return 30 * time.Second
	default:
		return 60 * time.Second
	}
}

func (srv *server) computeNodePlan(node *nodeState) (*NodeUnitPlan, error) {
	if node == nil {
		return nil, nil
	}
	actionList, err := buildPlanActions(node.Profiles)
	if err != nil {
		return nil, err
	}
	plan := &NodeUnitPlan{
		NodeId:   node.NodeID,
		Profiles: append([]string(nil), node.Profiles...),
	}
	if len(actionList) > 0 {
		plan.UnitActions = actionList
	}
	if rendered := srv.renderedConfigForNode(node); len(rendered) > 0 {
		plan.RenderedConfig = rendered
		// Phase 4b: inject restart actions for renderers whose output has changed.
		// If a plan is already in flight (PendingRenderedConfigHashes is set), compare
		// against pending so we don't re-dispatch the same restart actions every cycle.
		compareHashes := node.RenderedConfigHashes
		if len(node.PendingRenderedConfigHashes) > 0 {
			compareHashes = node.PendingRenderedConfigHashes
		}
		if restarts := restartActionsForChangedConfigs(compareHashes, rendered); len(restarts) > 0 {
			plan.UnitActions = append(plan.UnitActions, restarts...)
		}
	}
	return plan, nil
}

func planHash(plan *NodeUnitPlan) string {
	if plan == nil {
		return ""
	}
	actions := plan.GetUnitActions()
	rendered := plan.GetRenderedConfig()
	if len(actions) == 0 && len(rendered) == 0 {
		return ""
	}
	h := sha256.New()
	sortedActions := append([]*cluster_controllerpb.UnitAction(nil), actions...)
	sort.Slice(sortedActions, func(i, j int) bool {
		a := sortedActions[i]
		b := sortedActions[j]
		if a == nil && b == nil {
			return false
		}
		if a == nil {
			return true
		}
		if b == nil {
			return false
		}
		if a.GetUnitName() != b.GetUnitName() {
			return a.GetUnitName() < b.GetUnitName()
		}
		return a.GetAction() < b.GetAction()
	})
	for _, action := range sortedActions {
		if action == nil {
			continue
		}
		h.Write([]byte(action.GetUnitName()))
		h.Write([]byte{0})
		h.Write([]byte(action.GetAction()))
		h.Write([]byte{0})
	}
	if len(rendered) > 0 {
		keys := make([]string, 0, len(rendered))
		for key := range rendered {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			h.Write([]byte(key))
			h.Write([]byte{0})
			h.Write([]byte(rendered[key]))
			h.Write([]byte{0})
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}

func (srv *server) clusterNetworkSpec() *cluster_controllerpb.ClusterNetworkSpec {
	srv.lock("unknown")
	spec := srv.state.ClusterNetworkSpec
	srv.unlock()
	if spec == nil {
		return nil
	}
	if clone, ok := proto.Clone(spec).(*cluster_controllerpb.ClusterNetworkSpec); ok {
		return clone
	}
	return nil
}

func (srv *server) renderedConfigForSpec() map[string]string {
	spec := srv.clusterNetworkSpec()
	if spec == nil {
		return nil
	}
	out := make(map[string]string, 4)
	if specJSON, err := protojson.Marshal(spec); err == nil {
		out["cluster.network.spec.json"] = string(specJSON)
	}
	configPayload := map[string]interface{}{
		"Domain":           spec.GetClusterDomain(),
		"Protocol":         spec.GetProtocol(),
		"PortHTTP":         spec.GetPortHttp(),
		"PortHTTPS":        spec.GetPortHttps(),
		"AlternateDomains": spec.GetAlternateDomains(),
		"ACMEEnabled":      spec.GetAcmeEnabled(),
		"AdminEmail":       spec.GetAdminEmail(),
		"ACMEChallenge":    "dns-01",
		"ACMEDNSPreflight": true,
	}
	if cfgJSON, err := json.MarshalIndent(configPayload, "", "  "); err == nil {
		out["/var/lib/globular/network.json"] = string(cfgJSON)
	}
	if gen := srv.networkingGeneration(); gen > 0 {
		out["cluster.network.generation"] = fmt.Sprintf("%d", gen)
	}
	if units := restartUnitsForSpec(spec); len(units) > 0 {
		if b, err := json.Marshal(units); err == nil {
			out["reconcile.restart_units"] = string(b)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (srv *server) snapshotClusterMembership() *clusterMembership {
	srv.lock("snapshot-membership")
	defer srv.unlock()

	membership := &clusterMembership{
		ClusterID: srv.state.ClusterId,
		Nodes:     make([]memberNode, 0, len(srv.state.Nodes)),
	}

	for _, node := range srv.state.Nodes {
		if node == nil {
			continue
		}
		var ip string
		if len(node.Identity.Ips) > 0 {
			ip = node.Identity.Ips[0]
		}
		membership.Nodes = append(membership.Nodes, memberNode{
			NodeID:   node.NodeID,
			Hostname: node.Identity.Hostname,
			IP:       ip,
			Profiles: append([]string(nil), node.Profiles...),
		})
	}

	sort.Slice(membership.Nodes, func(i, j int) bool {
		return membership.Nodes[i].NodeID < membership.Nodes[j].NodeID
	})

	return membership
}

func (srv *server) renderedConfigForNode(node *nodeState) map[string]string {
	out := srv.renderedConfigForSpec()
	if out == nil {
		out = make(map[string]string)
	}

	membership := srv.snapshotClusterMembership()

	var currentMember *memberNode
	for i := range membership.Nodes {
		if membership.Nodes[i].NodeID == node.NodeID {
			currentMember = &membership.Nodes[i]
			break
		}
	}

	if currentMember == nil {
		var ip string
		if len(node.Identity.Ips) > 0 {
			ip = node.Identity.Ips[0]
		}
		currentMember = &memberNode{
			NodeID:   node.NodeID,
			Hostname: node.Identity.Hostname,
			IP:       ip,
			Profiles: node.Profiles,
		}
	}

	domain := ""
	externalDomain := ""
	if spec := srv.clusterNetworkSpec(); spec != nil {
		domain = spec.GetClusterDomain()
		if extDNS := spec.GetExternalDns(); extDNS != nil {
			externalDomain = extDNS.GetDomain()
		}
	}

	// Query live etcd cluster to determine member state for correct initial-cluster-state.
	var etcdState *etcdMemberState
	if srv.etcdMembers != nil {
		if st, err := srv.etcdMembers.snapshotEtcdMembers(context.Background()); err == nil {
			etcdState = st
		}
	}

	// Snapshot MinIO pool state under lock.
	srv.lock("minio-pool-snapshot")
	minioPoolNodes := append([]string(nil), srv.state.MinioPoolNodes...)
	minioCreds := srv.state.MinioCredentials
	minioNodePaths := make(map[string]string, len(srv.state.MinioNodePaths))
	for k, v := range srv.state.MinioNodePaths {
		minioNodePaths[k] = v
	}
	minioDrivesPerNode := srv.state.MinioDrivesPerNode
	srv.unlock()

	ctx := &serviceConfigContext{
		Membership:         membership,
		CurrentNode:        currentMember,
		ClusterID:          membership.ClusterID,
		Domain:             domain,
		ExternalDomain:     externalDomain,
		EtcdState:          etcdState,
		MinioPoolNodes:     minioPoolNodes,
		MinioCredentials:   minioCreds,
		MinioNodePaths:     minioNodePaths,
		MinioDrivesPerNode: minioDrivesPerNode,
	}

	serviceConfigs := renderServiceConfigs(ctx)
	for path, content := range serviceConfigs {
		out[path] = content
	}

	if len(out) == 0 {
		return nil
	}
	return out
}

func (srv *server) renderServiceConfigsForNodeInMembership(node *nodeState, membership *clusterMembership) map[string]string {
	if node == nil || membership == nil {
		return nil
	}
	var currentMember *memberNode
	for i := range membership.Nodes {
		if membership.Nodes[i].NodeID == node.NodeID {
			currentMember = &membership.Nodes[i]
			break
		}
	}
	if currentMember == nil {
		return nil
	}
	domain := ""
	externalDomain := ""
	if spec := srv.clusterNetworkSpec(); spec != nil {
		domain = spec.GetClusterDomain()
		if extDNS := spec.GetExternalDns(); extDNS != nil {
			externalDomain = extDNS.GetDomain()
		}
	}
	var etcdState *etcdMemberState
	if srv.etcdMembers != nil {
		if st, err := srv.etcdMembers.snapshotEtcdMembers(context.Background()); err == nil {
			etcdState = st
		}
	}
	srv.lock("minio-pool-snapshot-preview")
	minioPool := append([]string(nil), srv.state.MinioPoolNodes...)
	minioCr := srv.state.MinioCredentials
	minioNP := make(map[string]string, len(srv.state.MinioNodePaths))
	for k, v := range srv.state.MinioNodePaths {
		minioNP[k] = v
	}
	minioDPN := srv.state.MinioDrivesPerNode
	srv.unlock()

	ctx := &serviceConfigContext{
		Membership:         membership,
		CurrentNode:        currentMember,
		ClusterID:          membership.ClusterID,
		Domain:             domain,
		ExternalDomain:     externalDomain,
		EtcdState:          etcdState,
		MinioPoolNodes:     minioPool,
		MinioCredentials:   minioCr,
		MinioNodePaths:     minioNP,
		MinioDrivesPerNode: minioDPN,
	}
	return renderServiceConfigs(ctx)
}

func (srv *server) networkingGeneration() uint64 {
	srv.lock("state:network-gen")
	gen := srv.state.NetworkingGeneration
	srv.unlock()
	return gen
}

func restartUnitsForSpec(spec *cluster_controllerpb.ClusterNetworkSpec) []string {
	if spec == nil {
		return nil
	}
	units := []string{
		"globular-etcd.service",
		"globular-dns.service",
		"globular-discovery.service",
		"globular-xds.service",
		"globular-envoy.service",
		"globular-gateway.service",
		"globular-minio.service",
		"scylladb.service",
	}
	if spec.GetProtocol() == "https" {
		units = append(units, "globular-storage.service")
	}
	return units
}

func normalizeDomains(domains []string) []string {
	if len(domains) == 0 {
		return nil
	}
	seen := make(map[string]struct{})
	out := make([]string, 0, len(domains))
	for _, v := range domains {
		if v == "" {
			continue
		}
		trimmed := strings.TrimSpace(strings.ToLower(v))
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}


// Deprecated: dispatchPlan is a no-op — ApplyPlan RPC removed.
// Network config and auto-repair should use workflow-native paths.
func (srv *server) dispatchPlan(ctx context.Context, node *nodeState, plan *NodeUnitPlan, operationID string) error {
	log.Printf("dispatchPlan: skipped (plan system removed) node=%s op=%s", node.NodeID, operationID)
	return nil
}

// bootstrapRecoveryGracePeriod is the minimum time a node must be stuck at a
// non-terminal bootstrap phase (with no active workflow) before the recovery
// mechanism re-triggers the join workflow. This prevents re-triggering during
// normal startup where the workflow simply hasn't finished yet.
const bootstrapRecoveryGracePeriod = 2 * time.Minute

// recoverStuckBootstrapWorkflows scans nodes for those stuck at non-terminal
// bootstrap phases with no active join workflow goroutine. This handles the
// case where the controller restarts and kills in-flight triggerJoinWorkflow
// goroutines, leaving nodes stranded.
//
// For each stuck node, it re-triggers go srv.triggerJoinWorkflow() after
// setting BootstrapWorkflowActive = true to prevent double-triggers.
func (srv *server) recoverStuckBootstrapWorkflows(nodes []*nodeState, now time.Time) {
	srv.lock("recoverStuckBootstrapWorkflows")
	defer srv.unlock()

	for _, node := range nodes {
		if node == nil {
			continue
		}

		// Skip terminal phases — these nodes are done or legacy.
		if node.BootstrapPhase == BootstrapNone || node.BootstrapPhase == BootstrapWorkloadReady {
			continue
		}

		// Skip nodes already being driven by a workflow.
		if node.BootstrapWorkflowActive {
			continue
		}

		// Skip nodes that failed — the reconcileBootstrapPhases auto-retry
		// handles those by resetting to admitted.
		if node.BootstrapPhase == BootstrapFailed {
			continue
		}

		// Must have a valid agent endpoint to connect to.
		if node.AgentEndpoint == "" {
			continue
		}

		// CRITICAL: do NOT re-trigger the join workflow on nodes that have
		// already installed infrastructure (storage_joining or later).
		// The join workflow reinstalls packages like ScyllaDB, which wipes
		// their data and destroys Raft identity — causing unrecoverable
		// quorum deadlocks. For these phases, let the bootstrap phase
		// machine handle timeouts and retries without re-running join.
		if node.BootstrapPhase == BootstrapStorageJoining ||
			node.BootstrapPhase == BootstrapEtcdReady ||
			node.BootstrapPhase == BootstrapXdsReady ||
			node.BootstrapPhase == BootstrapEnvoyReady {
			log.Printf("bootstrap-recovery: node %s (%s) at phase %s — skipping join re-trigger (infra already installed)",
				node.NodeID, node.Identity.Hostname, node.BootstrapPhase)
			continue
		}

		// Grace period: don't re-trigger if the node was seen recently.
		// Use LastSeen (last heartbeat time) as the staleness signal.
		// A node that is actively heartbeating but stuck means the workflow
		// goroutine died (controller restart).
		if node.LastSeen.IsZero() || now.Sub(node.LastSeen) < bootstrapRecoveryGracePeriod {
			// Also check BootstrapStartedAt — if the phase was entered recently,
			// the workflow may still be in progress from a fresh trigger.
			if !node.BootstrapStartedAt.IsZero() && now.Sub(node.BootstrapStartedAt) < bootstrapRecoveryGracePeriod {
				continue
			}
			// If LastSeen is recent but BootstrapStartedAt is old, the node is
			// heartbeating but the workflow is gone — proceed with recovery.
			if node.BootstrapStartedAt.IsZero() || now.Sub(node.BootstrapStartedAt) < bootstrapRecoveryGracePeriod {
				continue
			}
		}

		log.Printf("bootstrap-recovery: node %s (%s) stuck at phase %s (last_seen=%s, phase_started=%s) — re-triggering join workflow at %s",
			node.NodeID, node.Identity.Hostname, node.BootstrapPhase,
			node.LastSeen.Format(time.RFC3339), node.BootstrapStartedAt.Format(time.RFC3339),
			node.AgentEndpoint)

		// Set active flag under lock before launching goroutine to prevent
		// double-triggers from concurrent reconcile cycles.
		node.BootstrapWorkflowActive = true
		nodeID := node.NodeID
		endpoint := node.AgentEndpoint
		go srv.triggerJoinWorkflow(nodeID, endpoint)
	}
}
