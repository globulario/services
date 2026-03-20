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
	"github.com/globulario/services/golang/plan/planpb"
	"github.com/google/uuid"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func (srv *server) reconcileNodes(ctx context.Context) {
	if !srv.reconcileRunning.CompareAndSwap(false, true) {
		return
	}
	defer srv.reconcileRunning.Store(false)
	if srv.planStore == nil || srv.kv == nil {
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
	srv.unlock()
	now := time.Now()

	// Pre-reconcile: ensure etcd cluster membership matches desired etcd nodes.
	// IMPORTANT: Only add nodes that have etcd installed (unit file present).
	// Adding a member before its etcd is ready to start breaks quorum because
	// etcd counts the unstarted member in its cluster size calculation.
	if srv.etcdMembers != nil {
		membership := srv.snapshotClusterMembership()
		desiredEtcdNodes := filterNodesByProfile(membership, profilesForEtcd)
		// Filter to only nodes that have globular-etcd.service installed.
		var readyEtcdNodes []memberNode
		for _, mn := range desiredEtcdNodes {
			for _, n := range nodes {
				if n != nil && n.NodeID == mn.NodeID && nodeHasEtcdRunning(n) {
					readyEtcdNodes = append(readyEtcdNodes, mn)
					break
				}
			}
		}
		if len(readyEtcdNodes) > 1 { // only expand when >1 ready node (single-node doesn't need member-add)
			if added, err := srv.etcdMembers.reconcileMembers(ctx, readyEtcdNodes); err != nil {
				log.Printf("reconcile: etcd member-add failed: %v", err)
			} else if len(added) > 0 {
				log.Printf("reconcile: registered %d new etcd members: %v", len(added), added)
			}
		}
	}

	for _, node := range nodes {
		if node == nil || node.NodeID == "" {
			continue
		}
		// Validate profiles before any dispatch — unknown profiles block the node.
		actions, profileErr := buildPlanActions(node.Profiles)
		if profileErr != nil {
			node.Status = "blocked"
			node.LastPlanError = profileErr.Error()
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
			node.LastPlanError = ""
			if node.Status == "blocked" {
				node.Status = "converging"
			}
			stateDirty = true
		}

		// Phase 3: Capability gating — desired units must be installed on the node.
		// Hard-gate when inventory_complete=true; soft-gate (warn only) otherwise.
		if len(node.Units) > 0 {
			desiredUnitNames := desiredUnitsFromActions(actions)
			if missing := missingInstalledUnits(desiredUnitNames, node.Units); len(missing) > 0 {
				if node.InventoryComplete {
					// Full inventory reported — hard block.
					node.Status = "blocked"
					node.LastPlanError = fmt.Sprintf("missing unit files: %v", missing)
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
					node.LastPlanError = ""
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
			existingStatus, _ := srv.planStore.GetStatus(ctx, node.NodeID)
			alreadyAwaiting := existingStatus != nil &&
				existingStatus.GetState() == planpb.PlanState_PLAN_AWAITING_PRIVILEGED_APPLY
			if !alreadyAwaiting {
				log.Printf("reconcile: node %s lacks privileged-apply capability, skipping plan dispatch", node.NodeID)
			}
		}

		appliedHash, err := srv.getNodeAppliedHash(ctx, node.NodeID)
		if err != nil {
			log.Printf("reconcile: read applied hash for %s: %v", node.NodeID, err)
			continue
		}
		status, _ := srv.planStore.GetStatus(ctx, node.NodeID)
		currentPlan, _ := srv.planStore.GetCurrentPlan(ctx, node.NodeID)
		meta, _ := srv.getNodePlanMeta(ctx, node.NodeID)
		planHash := ""
		lastEmitMs := int64(0)
		if currentPlan != nil {
			planHash = currentPlan.GetDesiredHash()
			if currentPlan.GetCreatedUnixMs() > 0 {
				lastEmitMs = int64(currentPlan.GetCreatedUnixMs())
			}
		}
		if planHash == "" && meta != nil {
			planHash = meta.DesiredHash
		}
		if lastEmitMs == 0 && meta != nil {
			lastEmitMs = meta.LastEmit
		}
		if specHash != "" && appliedHash != specHash {
			if status != nil && (status.GetState() == planpb.PlanState_PLAN_RUNNING || status.GetState() == planpb.PlanState_PLAN_PENDING) {
				if planHash == specHash && currentPlan != nil && status.GetPlanId() == currentPlan.GetPlanId() && status.GetGeneration() == currentPlan.GetGeneration() {
					if !isPlanStuck(status, lastEmitMs, now) {
						continue
					}
					srv.emitClusterEvent("operation.stalled", map[string]interface{}{
						"severity":       "ERROR",
						"node_id":        node.NodeID,
						"hostname":       node.Identity.Hostname,
						"plan_type":      "network",
						"plan_id":        currentPlan.GetPlanId(),
						"correlation_id": fmt.Sprintf("plan:%s:net", node.NodeID),
					})
				}
			}
			if status != nil && status.GetState() == planpb.PlanState_PLAN_SUCCEEDED {
				if planHash == specHash && currentPlan != nil && status.GetPlanId() == currentPlan.GetPlanId() && status.GetGeneration() == currentPlan.GetGeneration() {
					if err := srv.putNodeAppliedHash(ctx, node.NodeID, specHash); err != nil {
						log.Printf("reconcile: store applied hash for %s: %v", node.NodeID, err)
					}
					if desiredNetworkObj != nil && desiredNetworkObj.Meta != nil && srv.resources != nil {
						_, _ = srv.resources.UpdateStatus(ctx, "ClusterNetwork", "default", &cluster_controllerpb.ObjectStatus{
							ObservedGeneration: desiredNetworkObj.Meta.Generation,
						})
					}
					_ = srv.putNodeFailureCount(ctx, node.NodeID, 0)
					srv.emitClusterEvent("plan_apply_succeeded", map[string]interface{}{
						"severity":       "INFO",
						"node_id":        node.NodeID,
						"hostname":       node.Identity.Hostname,
						"message":        fmt.Sprintf("Network plan succeeded for %s", node.Identity.Hostname),
						"correlation_id": fmt.Sprintf("plan:%s:gen:%d", node.NodeID, currentPlan.GetGeneration()),
					})
					continue
				}
			}
			fails, _ := srv.getNodeFailureCount(ctx, node.NodeID)
			// Desired state changed since last failure — reset failure count so
			// the new config gets a clean attempt without accumulated backoff.
			if planHash != specHash && fails > 0 {
				_ = srv.putNodeFailureCount(ctx, node.NodeID, 0)
				fails = 0
			}
			if status != nil && planHash == specHash && (status.GetState() == planpb.PlanState_PLAN_FAILED || status.GetState() == planpb.PlanState_PLAN_ROLLED_BACK || status.GetState() == planpb.PlanState_PLAN_EXPIRED) {
				srv.emitClusterEvent("plan_apply_failed", map[string]interface{}{
					"severity":       "ERROR",
					"node_id":        node.NodeID,
					"hostname":       node.Identity.Hostname,
					"message":        fmt.Sprintf("Network plan failed for %s (state=%s)", node.Identity.Hostname, status.GetState()),
					"correlation_id": fmt.Sprintf("plan:%s:gen:%d", node.NodeID, status.GetGeneration()),
				})
				delay := backoffDuration(fails)
				if lastEmitMs > 0 && now.Sub(time.UnixMilli(lastEmitMs)) < delay {
					continue
				}
			}

			spec := desiredNetworkToSpec(desiredNet)
			if spec == nil {
				continue
			}
			plan, err := BuildNetworkTransitionPlan(node.NodeID, ClusterDesiredState{
				Network: spec,
			}, NodeObservedState{Units: node.Units})
			if err != nil {
				log.Printf("reconcile: build plan for %s failed: %v", node.NodeID, err)
				continue
			}
			plan.PlanId = uuid.NewString()
			plan.ClusterId = srv.state.ClusterNetworkSpec.GetClusterDomain()
			plan.NodeId = node.NodeID
			plan.Generation = srv.nextPlanGeneration(ctx, node.NodeID)
			plan.DesiredHash = specHash
			if plan.GetCreatedUnixMs() == 0 {
				plan.CreatedUnixMs = uint64(now.UnixMilli())
			}
			plan.IssuedBy = "cluster-controller"

			// Skip network plan dispatch if node lacks privileged-apply capability.
			// Do NOT continue — fall through to services reconciliation below
			// so that external-install detection can stamp the applied hash.
			if !canPriv {
				log.Printf("reconcile: node %s needs privileged apply for network plan (plan_id=%s), skipping network dispatch", node.NodeID, plan.GetPlanId())
				srv.emitClusterEvent("plan_blocked_privileged", map[string]interface{}{
					"severity":       "WARN",
					"node_id":        node.NodeID,
					"hostname":       node.Identity.Hostname,
					"message":        fmt.Sprintf("Node %s cannot apply privileged operations. Run: globular services apply-desired", node.Identity.Hostname),
					"correlation_id": fmt.Sprintf("plan:%s:gen:%d", node.NodeID, plan.GetGeneration()),
				})
			} else {
				if err := srv.signOrAbort(plan); err != nil {
					log.Printf("reconcile: signing aborted for %s: %v", node.NodeID, err)
					continue
				}
				if err := srv.planStore.PutCurrentPlan(ctx, node.NodeID, plan); err != nil {
					log.Printf("reconcile: persist plan for %s: %v", node.NodeID, err)
					continue
				}
				if appendable, ok := srv.planStore.(interface {
					AppendHistory(ctx context.Context, nodeID string, plan *planpb.NodePlan) error
				}); ok {
					_ = appendable.AppendHistory(ctx, node.NodeID, plan)
				}
				newMeta := &planMeta{PlanId: plan.GetPlanId(), Generation: plan.GetGeneration(), DesiredHash: specHash, LastEmit: now.UnixMilli()}
				_ = srv.putNodePlanMeta(ctx, node.NodeID, newMeta)
				if status != nil && (status.GetState() == planpb.PlanState_PLAN_FAILED || status.GetState() == planpb.PlanState_PLAN_ROLLED_BACK || status.GetState() == planpb.PlanState_PLAN_EXPIRED) {
					_ = srv.putNodeFailureCount(ctx, node.NodeID, fails+1)
				}
				log.Printf("reconcile: wrote network plan node=%s plan_id=%s gen=%d", node.NodeID, plan.GetPlanId(), plan.GetGeneration())
				srv.emitClusterEvent("plan_generated", map[string]interface{}{
					"severity":       "INFO",
					"node_id":        node.NodeID,
					"hostname":       node.Identity.Hostname,
					"message":        fmt.Sprintf("Network plan generated for %s", node.Identity.Hostname),
					"correlation_id": fmt.Sprintf("plan:%s:gen:%d", node.NodeID, plan.GetGeneration()),
				})
				srv.emitClusterEvent("plan_apply_started", map[string]interface{}{
					"severity":       "INFO",
					"node_id":        node.NodeID,
					"hostname":       node.Identity.Hostname,
					"message":        fmt.Sprintf("Network plan dispatched for %s", node.Identity.Hostname),
					"correlation_id": fmt.Sprintf("plan:%s:gen:%d", node.NodeID, plan.GetGeneration()),
				})
				continue
			}
		}

		// Services reconciliation
		desiredCanon, desiredObjs, err := srv.loadDesiredServices(ctx)
		if err != nil {
			log.Printf("reconcile: load desired services failed: %v", err)
			desiredCanon = map[string]string{}
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
			if status != nil && status.GetState() == planpb.PlanState_PLAN_SUCCEEDED && planHash == svcHash && currentPlan != nil && status.GetPlanId() == currentPlan.GetPlanId() && status.GetGeneration() == currentPlan.GetGeneration() {
				if err := srv.putNodeAppliedServiceHash(ctx, node.NodeID, svcHash); err != nil {
					log.Printf("reconcile: store applied service hash for %s: %v", node.NodeID, err)
				}
				if srv.resources != nil {
					for _, obj := range desiredObjs {
						if obj != nil && obj.Meta != nil {
							_, _ = srv.resources.UpdateStatus(ctx, "ServiceDesiredVersion", obj.Meta.Name, &cluster_controllerpb.ObjectStatus{
								ObservedGeneration: obj.Meta.Generation,
							})
						}
					}
				}
				srv.emitClusterEvent("plan_apply_succeeded", map[string]interface{}{
					"severity":       "INFO",
					"node_id":        node.NodeID,
					"hostname":       node.Identity.Hostname,
					"message":        fmt.Sprintf("All services at desired state for %s", node.Identity.Hostname),
					"correlation_id": fmt.Sprintf("plan:%s:gen:%d", node.NodeID, currentPlan.GetGeneration()),
				})
			}
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
		if status != nil && (status.GetState() == planpb.PlanState_PLAN_RUNNING || status.GetState() == planpb.PlanState_PLAN_PENDING) {
			if planHash == svcHash && currentPlan != nil && status.GetPlanId() == currentPlan.GetPlanId() && status.GetGeneration() == currentPlan.GetGeneration() {
				if !isPlanStuck(status, lastEmitMs, now) {
					continue
				}
				srv.emitClusterEvent("operation.stalled", map[string]interface{}{
					"severity":       "ERROR",
					"node_id":        node.NodeID,
					"hostname":       node.Identity.Hostname,
					"plan_type":      "service",
					"plan_id":        currentPlan.GetPlanId(),
					"correlation_id": fmt.Sprintf("plan:%s:svc", node.NodeID),
				})
			} else {
				continue
			}
		}
		if status != nil && status.GetState() == planpb.PlanState_PLAN_SUCCEEDED {
			if planHash == svcHash && currentPlan != nil && status.GetPlanId() == currentPlan.GetPlanId() && status.GetGeneration() == currentPlan.GetGeneration() {
				_ = srv.putNodeFailureCountServices(ctx, node.NodeID, 0)
				srv.emitClusterEvent("service_apply_succeeded", map[string]interface{}{
					"severity":       "INFO",
					"node_id":        node.NodeID,
					"hostname":       node.Identity.Hostname,
					"message":        fmt.Sprintf("Service plan succeeded for %s", node.Identity.Hostname),
					"correlation_id": fmt.Sprintf("plan:%s:gen:%d", node.NodeID, currentPlan.GetGeneration()),
				})
				// Don't store appliedSvcHash here — this plan installed only ONE
				// service, but svcHash covers ALL desired services. Storing it
				// would cause the reconciler to skip remaining uninstalled services.
				// Fall through to check if more services need installation.
			}
		}
		failsSvc, _ := srv.getNodeFailureCountServices(ctx, node.NodeID)
		// Desired state changed since last failure — reset failure count.
		if planHash != svcHash && failsSvc > 0 {
			_ = srv.putNodeFailureCountServices(ctx, node.NodeID, 0)
			failsSvc = 0
		}
		if status != nil && planHash == svcHash && (status.GetState() == planpb.PlanState_PLAN_FAILED || status.GetState() == planpb.PlanState_PLAN_ROLLED_BACK || status.GetState() == planpb.PlanState_PLAN_EXPIRED) {
			srv.emitClusterEvent("service_apply_failed", map[string]interface{}{
				"severity":       "ERROR",
				"node_id":        node.NodeID,
				"hostname":       node.Identity.Hostname,
				"message":        fmt.Sprintf("Service plan failed for %s (state=%s)", node.Identity.Hostname, status.GetState()),
				"correlation_id": fmt.Sprintf("plan:%s:gen:%d", node.NodeID, status.GetGeneration()),
			})
			delay := backoffDuration(failsSvc)
			if lastEmitMs > 0 && now.Sub(time.UnixMilli(lastEmitMs)) < delay {
				continue
			}
		}

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
		plan := BuildServiceUpgradePlan(node.NodeID, canonicalServiceName(svcName), version, artifactDigest, desiredBuildNumber)
		if plan != nil {
			mutated, err := op.MutatePlan(ctx, operator.MutateRequest{Service: canonicalServiceName(svcName), NodeID: node.NodeID, Plan: plan, DesiredDomain: desiredNet.GetDomain(), DesiredProtocol: desiredNet.GetProtocol(), ClusterID: srv.state.ClusterId})
			if err != nil {
				log.Printf("reconcile: operator mutate %s on %s failed: %v", svcName, node.NodeID, err)
				continue
			}
			if mutated != nil {
				plan = mutated
			}
		}
		plan.PlanId = uuid.NewString()
		plan.ClusterId = srv.state.ClusterNetworkSpec.GetClusterDomain()
		plan.NodeId = node.NodeID
		plan.Generation = srv.nextPlanGeneration(ctx, node.NodeID)
		plan.DesiredHash = svcHash
		if plan.GetCreatedUnixMs() == 0 {
			plan.CreatedUnixMs = uint64(now.UnixMilli())
		}
		plan.IssuedBy = "cluster-controller"
		if err := srv.signOrAbort(plan); err != nil {
			log.Printf("reconcile: signing aborted for service plan on %s: %v", node.NodeID, err)
			continue
		}
		if err := srv.planStore.PutCurrentPlan(ctx, node.NodeID, plan); err != nil {
			log.Printf("reconcile: persist service plan for %s: %v", node.NodeID, err)
			continue
		}
		if appendable, ok := srv.planStore.(interface {
			AppendHistory(ctx context.Context, nodeID string, plan *planpb.NodePlan) error
		}); ok {
			_ = appendable.AppendHistory(ctx, node.NodeID, plan)
		}
		newMeta := &planMeta{PlanId: plan.GetPlanId(), Generation: plan.GetGeneration(), DesiredHash: svcHash, LastEmit: now.UnixMilli()}
		_ = srv.putNodePlanMeta(ctx, node.NodeID, newMeta)
		if status != nil && planHash == svcHash && (status.GetState() == planpb.PlanState_PLAN_FAILED || status.GetState() == planpb.PlanState_PLAN_ROLLED_BACK || status.GetState() == planpb.PlanState_PLAN_EXPIRED) {
			_ = srv.putNodeFailureCountServices(ctx, node.NodeID, failsSvc+1)
		}
		log.Printf("reconcile: wrote service plan node=%s service=%s plan_id=%s gen=%d", node.NodeID, svcName, plan.GetPlanId(), plan.GetGeneration())
		srv.emitClusterEvent("service_apply_started", map[string]interface{}{
			"severity":       "INFO",
			"node_id":        node.NodeID,
			"hostname":       node.Identity.Hostname,
			"service":        svcName,
			"message":        fmt.Sprintf("Service upgrade plan dispatched for %s on %s", svcName, node.Identity.Hostname),
			"correlation_id": fmt.Sprintf("plan:%s:gen:%d", node.NodeID, plan.GetGeneration()),
		})
	}
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

func isPlanStuck(status *planpb.NodePlanStatus, lastEmitMs int64, now time.Time) bool {
	if status == nil {
		return false
	}
	last := status.GetFinishedUnixMs()
	if last == 0 {
		last = status.GetStartedUnixMs()
	}
	if last == 0 && lastEmitMs > 0 {
		last = uint64(lastEmitMs)
	}
	if last == 0 {
		return false
	}
	return now.Sub(time.UnixMilli(int64(last))) > 10*time.Minute
}

func (srv *server) computeNodePlan(node *nodeState) (*cluster_controllerpb.NodePlan, error) {
	if node == nil {
		return nil, nil
	}
	actionList, err := buildPlanActions(node.Profiles)
	if err != nil {
		return nil, err
	}
	plan := &cluster_controllerpb.NodePlan{
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

func planHash(plan *cluster_controllerpb.NodePlan) string {
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

	ctx := &serviceConfigContext{
		Membership:     membership,
		CurrentNode:    currentMember,
		ClusterID:      membership.ClusterID,
		Domain:         domain,
		ExternalDomain: externalDomain,
		EtcdState:      etcdState,
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
	ctx := &serviceConfigContext{
		Membership:     membership,
		CurrentNode:    currentMember,
		ClusterID:      membership.ClusterID,
		Domain:         domain,
		ExternalDomain: externalDomain,
		EtcdState:      etcdState,
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

func computeNetworkGeneration(spec *cluster_controllerpb.ClusterNetworkSpec) uint64 {
	if spec == nil {
		return 0
	}
	domain := strings.ToLower(strings.TrimSpace(spec.GetClusterDomain()))
	protoStr := strings.ToLower(strings.TrimSpace(spec.GetProtocol()))
	alts := normalizeDomains(spec.GetAlternateDomains())
	sort.Strings(alts)
	builder := strings.Builder{}
	builder.WriteString(domain)
	builder.WriteString("|")
	builder.WriteString(protoStr)
	builder.WriteString("|")
	builder.WriteString(fmt.Sprintf("%d|%d|", spec.GetPortHttp(), spec.GetPortHttps()))
	builder.WriteString(fmt.Sprintf("%t|", spec.GetAcmeEnabled()))
	builder.WriteString(strings.ToLower(strings.TrimSpace(spec.GetAdminEmail())))
	builder.WriteString("|")
	for _, a := range alts {
		builder.WriteString(a)
		builder.WriteString(",")
	}
	sum := sha256.Sum256([]byte(builder.String()))
	var gen uint64
	for i := 0; i < 8; i++ {
		gen = (gen << 8) | uint64(sum[i])
	}
	if gen == 0 {
		gen = 1
	}
	return gen
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

func (srv *server) shouldDispatch(node *nodeState, hash string) bool {
	if node == nil {
		return false
	}
	if node.AgentEndpoint == "" {
		return false
	}
	if hash == "" {
		return false
	}
	if node.LastPlanHash != hash {
		return true
	}
	if node.Status != "ready" {
		return true
	}
	if node.LastPlanError != "" {
		return true
	}
	return false
}

func (srv *server) dispatchPlan(ctx context.Context, node *nodeState, plan *cluster_controllerpb.NodePlan, operationID string) error {
	if plan == nil {
		return fmt.Errorf("node %s plan is empty", node.NodeID)
	}
	client, err := srv.getAgentClient(ctx, node.AgentEndpoint)
	if err != nil {
		return fmt.Errorf("node %s: %w", node.NodeID, err)
	}
	if err := client.ApplyPlan(ctx, plan, operationID); err != nil {
		return fmt.Errorf("node %s apply plan: %w", node.NodeID, err)
	}
	return nil
}
