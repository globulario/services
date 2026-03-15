package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/plan/planpb"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (srv *server) GetClusterHealth(ctx context.Context, req *cluster_controllerpb.GetClusterHealthRequest) (*cluster_controllerpb.GetClusterHealthResponse, error) {
	srv.lock("cluster-health")
	defer srv.unlock()

	resp := &cluster_controllerpb.GetClusterHealthResponse{
		TotalNodes: int32(len(srv.state.Nodes)),
	}

	now := time.Now()
	healthyThreshold := 2 * time.Minute // Node is healthy if seen within this time

	for _, node := range srv.state.Nodes {
		nodeHealth := &cluster_controllerpb.NodeHealthStatus{
			NodeId:    node.NodeID,
			Hostname:  node.Identity.Hostname,
			LastError: node.LastError,
			LastSeen:  timestamppb.New(node.LastSeen),
		}

		// Determine node health status
		timeSinceSeen := now.Sub(node.LastSeen)
		isHealthy := (node.Status == "healthy" || node.Status == "ready" || node.Status == "converging")
		switch {
		case isHealthy && timeSinceSeen < healthyThreshold:
			nodeHealth.Status = "healthy"
			resp.HealthyNodes++
		case node.Status == "unhealthy" || node.Status == "degraded" || node.LastError != "":
			nodeHealth.Status = "unhealthy"
			nodeHealth.FailedChecks = 1
			if node.LastError != "" {
				nodeHealth.LastError = node.LastError
			}
			resp.UnhealthyNodes++
		case timeSinceSeen >= healthyThreshold:
			nodeHealth.Status = "unknown"
			nodeHealth.LastError = fmt.Sprintf("not seen for %v", timeSinceSeen.Round(time.Second))
			resp.UnknownNodes++
		default:
			nodeHealth.Status = "unknown"
			resp.UnknownNodes++
		}

		resp.NodeHealth = append(resp.NodeHealth, nodeHealth)
	}

	// Determine overall cluster status
	switch {
	case resp.TotalNodes == 0:
		resp.Status = "unhealthy"
	case resp.UnhealthyNodes == 0 && resp.UnknownNodes == 0:
		resp.Status = "healthy"
	case resp.HealthyNodes > 0:
		resp.Status = "degraded"
	default:
		resp.Status = "unhealthy"
	}

	return resp, nil
}

func (srv *server) GetClusterHealthV1(ctx context.Context, _ *cluster_controllerpb.GetClusterHealthV1Request) (*cluster_controllerpb.GetClusterHealthV1Response, error) {
	if srv.resources == nil {
		return nil, status.Error(codes.FailedPrecondition, "resource store unavailable")
	}
	if srv.planStore == nil || srv.kv == nil {
		return nil, status.Error(codes.FailedPrecondition, "plan store or kv unavailable")
	}
	desiredNetObj, err := srv.loadDesiredNetwork(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "load desired network: %v", err)
	}
	specHash := ""
	if desiredNetObj != nil && desiredNetObj.Spec != nil {
		hash, _ := hashDesiredNetwork(&cluster_controllerpb.DesiredNetwork{
			Domain:           desiredNetObj.Spec.GetClusterDomain(),
			Protocol:         desiredNetObj.Spec.GetProtocol(),
			PortHttp:         desiredNetObj.Spec.GetPortHttp(),
			PortHttps:        desiredNetObj.Spec.GetPortHttps(),
			AlternateDomains: append([]string(nil), desiredNetObj.Spec.GetAlternateDomains()...),
			AcmeEnabled:      desiredNetObj.Spec.GetAcmeEnabled(),
			AdminEmail:       desiredNetObj.Spec.GetAdminEmail(),
		})
		specHash = hash
	}
	desiredCanon, _, err := srv.loadDesiredServices(ctx)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "load desired services: %v", err)
	}
	// Keep a SERVICE-only copy for the privileged-apply check below.
	// Infrastructure packages are managed by bootstrap, not the convergence
	// loop, so they must not trigger PLAN_AWAITING_PRIVILEGED_APPLY.
	serviceOnlyDesired := make(map[string]string, len(desiredCanon))
	for k, v := range desiredCanon {
		serviceOnlyDesired[k] = v
	}
	// Merge InfrastructureRelease entries so infrastructure daemons
	// (etcd, minio, prometheus, etc.) appear in the health summary
	// alongside gRPC services. Without this they show as "unmanaged".
	if srv.resources != nil {
		if items, _, err := srv.resources.List(ctx, "InfrastructureRelease", ""); err == nil {
			for _, obj := range items {
				if rel, ok := obj.(*cluster_controllerpb.InfrastructureRelease); ok && rel.Spec != nil {
					canon := canonicalServiceName(rel.Spec.Component)
					if canon == "" && rel.Meta != nil {
						canon = canonicalServiceName(rel.Meta.Name)
					}
					if canon != "" {
						if _, exists := desiredCanon[canon]; !exists {
							desiredCanon[canon] = rel.Spec.Version
						}
					}
				}
			}
		}
	}
	srv.lock("health:snapshot")
	nodes := make([]*nodeState, 0, len(srv.state.Nodes))
	for _, n := range srv.state.Nodes {
		nodes = append(nodes, n)
	}
	srv.unlock()

	var nodeHealths []*cluster_controllerpb.NodeHealth
	serviceCounts := make(map[string]int)
	serviceAtDesired := make(map[string]int)
	serviceUpgrading := make(map[string]int)

	for _, node := range nodes {
		if node == nil {
			continue
		}
		appliedNet, _ := srv.getNodeAppliedHash(ctx, node.NodeID)
		filtered := filterVersionsForNode(desiredCanon, node)
		desiredSvcHash := stableServiceDesiredHash(filtered)
		appliedSvcHash, _ := srv.getNodeAppliedServiceHash(ctx, node.NodeID)
		plan, _ := srv.planStore.GetCurrentPlan(ctx, node.NodeID)
		status, _ := srv.planStore.GetStatus(ctx, node.NodeID)
		phase := ""
		if status != nil {
			phase = status.GetState().String()
		}
		lastErr := ""
		if status != nil {
			lastErr = status.GetErrorMessage()
		}
		// Determine whether the node can perform privileged operations.
		canPriv := false
		if node.Capabilities != nil {
			canPriv = node.Capabilities.CanApplyPrivileged
		}

		// Only show PLAN_AWAITING_PRIVILEGED_APPLY when at least one desired
		// SERVICE-kind package is genuinely missing or at the wrong version
		// AND the node cannot self-apply. Infrastructure packages (envoy,
		// etcd, minio, etc.) are managed by bootstrap and must not trigger
		// this state — they don't flow through the convergence loop.
		svcOnlyFiltered := filterVersionsForNode(serviceOnlyDesired, node)
		if desiredSvcHash != "" {
			hasMissing := false
			for svc, desiredVer := range svcOnlyFiltered {
				installedVer := ""
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
				if installedVer != desiredVer {
					hasMissing = true
					break
				}
			}
			if hasMissing && !canPriv {
				isActive := status != nil &&
					(status.GetState() == planpb.PlanState_PLAN_RUNNING ||
						status.GetState() == planpb.PlanState_PLAN_ROLLING_BACK)
				if !isActive {
					phase = planpb.PlanState_PLAN_AWAITING_PRIVILEGED_APPLY.String()
				}
			}
			// Stamp the applied service hash when all desired services are
			// already installed at the correct version but the hash was never
			// written (e.g. services installed externally via bootstrap/CLI).
			if !hasMissing && appliedSvcHash != desiredSvcHash && len(svcOnlyFiltered) > 0 && len(node.InstalledVersions) > 0 {
				if err := srv.putNodeAppliedServiceHash(ctx, node.NodeID, desiredSvcHash); err != nil {
					log.Printf("health: stamp applied service hash for %s: %v", node.NodeID, err)
				} else {
					log.Printf("health: external install detected for node %s — all %d services converged, stamped applied hash", node.NodeID, len(svcOnlyFiltered))
					appliedSvcHash = desiredSvcHash
				}
			}
		}

		nodeHealths = append(nodeHealths, &cluster_controllerpb.NodeHealth{
			NodeId:              node.NodeID,
			DesiredNetworkHash:  specHash,
			AppliedNetworkHash:  appliedNet,
			DesiredServicesHash: desiredSvcHash,
			AppliedServicesHash: appliedSvcHash,
			CurrentPlanId: func() string {
				if plan != nil {
					return plan.GetPlanId()
				} else {
					return ""
				}
			}(),
			CurrentPlanGeneration: func() uint64 {
				if plan != nil {
					return plan.GetGeneration()
				} else {
					return 0
				}
			}(),
			CurrentPlanPhase:    phase,
			LastError:           lastErr,
			CanApplyPrivileged:  canPriv,
			InstalledVersions:   node.InstalledVersions,
		})

		for svc, desiredVer := range filtered {
			serviceCounts[svc]++
			// Per-service convergence: compare installed version against
			// the desired version for THIS service, not the global hash.
			// Use canonicalized fuzzy matching because InstalledVersions
			// keys may include publisher prefix ("pub/service") or use
			// different casing/separators than the desired-state key.
			installedVer := ""
			if v, ok := node.InstalledVersions[svc]; ok {
				installedVer = v
			} else {
				// Fuzzy match: strip publisher prefix and canonicalize.
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
			}
			if installedVer == desiredVer {
				serviceAtDesired[svc]++
			}
			if status != nil && plan != nil && plan.GetDesiredHash() != "" && plan.GetDesiredHash() == desiredSvcHash {
				if status.GetState() == planpb.PlanState_PLAN_RUNNING || status.GetState() == planpb.PlanState_PLAN_PENDING {
					serviceUpgrading[svc]++
				}
			}
		}
	}

	var summaries []*cluster_controllerpb.ServiceSummary
	for svc, ver := range desiredCanon {
		total := int32(serviceCounts[svc])
		at := int32(serviceAtDesired[svc])
		up := int32(serviceUpgrading[svc])
		summaries = append(summaries, &cluster_controllerpb.ServiceSummary{
			ServiceName:    svc,
			DesiredVersion: ver,
			NodesAtDesired: at,
			NodesTotal:     total,
			Upgrading:      up,
		})
	}

	return &cluster_controllerpb.GetClusterHealthV1Response{
		Nodes:    nodeHealths,
		Services: summaries,
	}, nil
}

func (srv *server) GetNodeHealthDetailV1(ctx context.Context, req *cluster_controllerpb.GetNodeHealthDetailV1Request) (*cluster_controllerpb.GetNodeHealthDetailV1Response, error) {
	if req == nil || req.GetNodeId() == "" {
		return nil, status.Error(codes.InvalidArgument, "node_id is required")
	}
	nodeID := req.GetNodeId()

	srv.lock("health-detail:snapshot")
	node := srv.state.Nodes[nodeID]
	srv.unlock()

	if node == nil {
		return nil, status.Errorf(codes.NotFound, "node %q not found", nodeID)
	}

	var checks []*cluster_controllerpb.NodeHealthCheck

	// 1. Heartbeat check
	heartbeatOK := !node.LastSeen.IsZero() && time.Since(node.LastSeen) < unhealthyThreshold
	hbReason := ""
	if !heartbeatOK {
		if node.LastSeen.IsZero() {
			hbReason = "never seen"
		} else {
			hbReason = fmt.Sprintf("last seen %s ago", time.Since(node.LastSeen).Truncate(time.Second))
		}
	}
	checks = append(checks, &cluster_controllerpb.NodeHealthCheck{
		Subsystem: "heartbeat",
		Ok:        heartbeatOK,
		Reason:    hbReason,
	})

	// 2. Unit checks — compare required units from plan vs reported unit states
	plan, _ := srv.computeNodePlan(node)
	required := requiredUnitsFromPlan(plan)
	unitStates := make(map[string]string, len(node.Units))
	for _, u := range node.Units {
		if u.Name != "" {
			unitStates[strings.ToLower(u.Name)] = strings.ToLower(u.State)
		}
	}
	for unit := range required {
		unitOK := false
		reason := ""
		st, found := unitStates[strings.ToLower(unit)]
		if !found {
			reason = "unit not reported by node"
		} else if st != "active" {
			reason = fmt.Sprintf("state is %q", st)
		} else {
			unitOK = true
		}
		checks = append(checks, &cluster_controllerpb.NodeHealthCheck{
			Subsystem: "unit:" + unit,
			Ok:        unitOK,
			Reason:    reason,
		})
	}

	// 3. Inventory check
	checks = append(checks, &cluster_controllerpb.NodeHealthCheck{
		Subsystem: "inventory",
		Ok:        node.InventoryComplete,
		Reason: func() string {
			if !node.InventoryComplete {
				return "inventory scan not yet complete"
			}
			return ""
		}(),
	})

	// 4. Version checks — compare installed vs desired
	desiredCanon, _, _ := srv.loadDesiredServices(ctx)
	filtered := filterVersionsForNode(desiredCanon, node)
	for svc, desiredVer := range filtered {
		installedVer, found := node.InstalledVersions[svc]
		ok := found && installedVer == desiredVer
		reason := ""
		if !found {
			reason = fmt.Sprintf("not installed (desired %s)", desiredVer)
		} else if installedVer != desiredVer {
			reason = fmt.Sprintf("installed %s, desired %s", installedVer, desiredVer)
		}
		checks = append(checks, &cluster_controllerpb.NodeHealthCheck{
			Subsystem: "version:" + svc,
			Ok:        ok,
			Reason:    reason,
		})
	}

	// Overall status from existing evaluator, overridden to unhealthy if heartbeat fails.
	overallStatus, _ := srv.evaluateNodeStatus(node, node.Units)
	if !heartbeatOK {
		overallStatus = "unhealthy"
	}
	allOK := true
	for _, c := range checks {
		if !c.Ok {
			allOK = false
			break
		}
	}

	canPriv := false
	privReason := ""
	if node.Capabilities != nil {
		canPriv = node.Capabilities.CanApplyPrivileged
		privReason = node.Capabilities.PrivilegeReason
	}

	return &cluster_controllerpb.GetNodeHealthDetailV1Response{
		NodeId:             nodeID,
		OverallStatus:      overallStatus,
		Healthy:            allOK,
		Checks:             checks,
		LastError:          node.LastError,
		CanApplyPrivileged: canPriv,
		InventoryComplete:  node.InventoryComplete,
		LastSeen:           timestamppb.New(node.LastSeen),
		PrivilegeReason:    privReason,
	}, nil
}

func (srv *server) monitorNodeHealth(ctx context.Context) {
	now := time.Now()

	srv.lock("health-monitor:snapshot")
	nodes := make([]*nodeState, 0, len(srv.state.Nodes))
	for _, node := range srv.state.Nodes {
		nodes = append(nodes, node)
	}
	srv.unlock()

	var stateDirty bool

	for _, node := range nodes {
		timeSinceSeen := now.Sub(node.LastSeen)

		srv.lock("health-monitor:check")
		currentNode := srv.state.Nodes[node.NodeID]
		if currentNode == nil {
			srv.unlock()
			continue
		}

		previousStatus := currentNode.Status

		// Check if node is unhealthy
		if timeSinceSeen > unhealthyThreshold {
			currentNode.FailedHealthChecks++

			if currentNode.Status != "unhealthy" {
				currentNode.Status = "unhealthy"
				currentNode.MarkedUnhealthySince = now
				currentNode.LastError = fmt.Sprintf("no contact for %v", timeSinceSeen.Round(time.Second))
				log.Printf("node %s marked unhealthy: %s", node.NodeID, currentNode.LastError)
				stateDirty = true
			}

			// Attempt recovery if needed
			shouldRecover := currentNode.RecoveryAttempts < maxRecoveryAttempts &&
				(currentNode.LastRecoveryAttempt.IsZero() || now.Sub(currentNode.LastRecoveryAttempt) > recoveryAttemptInterval)

			if shouldRecover && node.AgentEndpoint != "" {
				currentNode.LastRecoveryAttempt = now
				currentNode.RecoveryAttempts++
				stateDirty = true
				log.Printf("attempting recovery for node %s (attempt %d/%d)", node.NodeID, currentNode.RecoveryAttempts, maxRecoveryAttempts)
				srv.unlock()

				// Attempt to reconnect and redispatch plan
				if err := srv.attemptNodeRecovery(ctx, node); err != nil {
					log.Printf("recovery attempt for node %s failed: %v", node.NodeID, err)
					srv.lock("health-monitor:recovery-failed")
					if n := srv.state.Nodes[node.NodeID]; n != nil {
						n.LastError = fmt.Sprintf("recovery failed: %v", err)
					}
					srv.unlock()
				} else {
					log.Printf("recovery attempt for node %s initiated successfully", node.NodeID)
				}
				continue
			}
		} else if currentNode.Status == "unhealthy" && previousStatus == "unhealthy" {
			// Node came back online - reset recovery counters
			currentNode.Status = "healthy"
			currentNode.FailedHealthChecks = 0
			currentNode.RecoveryAttempts = 0
			currentNode.MarkedUnhealthySince = time.Time{}
			currentNode.LastError = ""
			log.Printf("node %s recovered and marked healthy", node.NodeID)
			stateDirty = true
		}
		srv.unlock()
	}

	if stateDirty {
		srv.lock("health-monitor:persist")
		func() {
			defer srv.unlock()
			if err := srv.persistStateLocked(true); err != nil {
				log.Printf("health monitor: persist state: %v", err)
			}
		}()
	}
}

func (srv *server) attemptNodeRecovery(ctx context.Context, node *nodeState) error {
	if node.AgentEndpoint == "" {
		return fmt.Errorf("no agent endpoint for node %s", node.NodeID)
	}

	// Close any existing connection to force reconnection
	srv.closeAgentClient(node.AgentEndpoint)

	// Get fresh agent client
	client, err := srv.getAgentClient(ctx, node.AgentEndpoint)
	if err != nil {
		return fmt.Errorf("connect to agent: %w", err)
	}

	// Try to get inventory to verify connectivity
	_, err = client.GetInventory(ctx)
	if err != nil {
		return fmt.Errorf("get inventory: %w", err)
	}

	// If we can connect, dispatch the current plan
	plan, planErr := srv.computeNodePlan(node)
	if planErr != nil {
		return fmt.Errorf("compute plan: %w", planErr)
	}
	if plan == nil || (len(plan.GetUnitActions()) == 0 && len(plan.GetRenderedConfig()) == 0) {
		// No plan needed, just mark as recovered
		return nil
	}

	opID := uuid.NewString()
	srv.broadcastOperationEvent(srv.newOperationEvent(opID, node.NodeID, cluster_controllerpb.OperationPhase_OP_QUEUED, "recovery: plan queued", 0, false, ""))

	if err := srv.dispatchPlan(ctx, node, plan, opID); err != nil {
		srv.broadcastOperationEvent(srv.newOperationEvent(opID, node.NodeID, cluster_controllerpb.OperationPhase_OP_FAILED, "recovery: plan failed", 0, true, err.Error()))
		return fmt.Errorf("dispatch plan: %w", err)
	}

	// Phase 4b: store pending rendered config hashes on recovery dispatch.
	// Promoted to RenderedConfigHashes only after agent reports apply success.
	if len(plan.GetRenderedConfig()) > 0 {
		srv.lock("recovery-rendered-config-hashes")
		node.PendingRenderedConfigHashes = HashRenderedConfigs(plan.GetRenderedConfig())
		srv.unlock()
	}
	srv.broadcastOperationEvent(srv.newOperationEvent(opID, node.NodeID, cluster_controllerpb.OperationPhase_OP_RUNNING, "recovery: plan dispatched", 25, false, ""))
	return nil
}

func (srv *server) evaluateNodeStatus(node *nodeState, units []unitStatusRecord) (string, string) {
	if node == nil {
		return "degraded", "missing node record"
	}
	plan, _ := srv.computeNodePlan(node)
	required := requiredUnitsFromPlan(plan)
	if len(required) == 0 {
		return "ready", ""
	}
	unitStates := make(map[string]string, len(units))
	for _, u := range units {
		if u.Name == "" {
			continue
		}
		unitStates[strings.ToLower(u.Name)] = strings.ToLower(u.State)
	}
	var missing []string
	var notActive []string
	for unit := range required {
		state, ok := unitStates[strings.ToLower(unit)]
		if !ok {
			missing = append(missing, fmt.Sprintf("%s missing", unit))
			continue
		}
		if state != "active" {
			if state == "" {
				state = "unknown"
			}
			notActive = append(notActive, fmt.Sprintf("%s is %s", unit, state))
		}
	}
	if len(missing) > 0 || len(notActive) > 0 {
		reason := strings.Join(append(missing, notActive...), "; ")
		if node.ReportedAt.IsZero() || time.Since(node.ReportedAt) < statusGracePeriod {
			return "converging", reason
		}
		return "degraded", reason
	}
	return "ready", ""
}

func (srv *server) startHealthMonitorLoop(ctx context.Context) {
	ticker := time.NewTicker(healthCheckInterval)
	safeGo("health-monitor", func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if !srv.isLeader() {
					continue
				}
				srv.monitorNodeHealth(ctx)
			}
		}
	})
}
