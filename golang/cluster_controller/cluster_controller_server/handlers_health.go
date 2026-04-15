package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/globular_service"
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
	if srv.kv == nil {
		return nil, status.Error(codes.FailedPrecondition, "kv unavailable")
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
	// (etcd, minio, prometheus, etc.) appear in the hash computation
	// alongside gRPC services.
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

	for _, node := range nodes {
		if node == nil {
			continue
		}
		appliedNet, _ := srv.getNodeAppliedHash(ctx, node.NodeID)
		filtered := filterVersionsForNode(desiredCanon, node)
		desiredSvcHash := stableServiceDesiredHash(filtered)
		appliedSvcHash, _ := srv.getNodeAppliedServiceHash(ctx, node.NodeID)
		canPriv := false
		if node.Capabilities != nil {
			canPriv = node.Capabilities.CanApplyPrivileged
		}

		// Stamp the applied service hash when all desired services are
		// already installed at the correct version but the hash was never
		// written (e.g. services installed externally via bootstrap/CLI).
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
			LastError:           "",
			CanApplyPrivileged:  canPriv,
			InstalledVersions:   node.InstalledVersions,
		})
	}

	// LAW 9: Compute service summaries using pure projection (Desired vs Installed only).
	// No workflow state, no runtime health, no cached counters.
	projections := srv.ComputeClusterProjection(ctx)
	var summaries []*cluster_controllerpb.ServiceSummary
	for _, p := range projections {
		summaries = append(summaries, &cluster_controllerpb.ServiceSummary{
			ServiceName:    p.ServiceName,
			DesiredVersion: p.DesiredVersion,
			NodesAtDesired: int32(p.NodesAtDesired),
			NodesTotal:     int32(p.NodesTotal),
			Kind:           p.Kind,
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
	heartbeatAge := time.Since(node.LastSeen)
	heartbeatOK := !node.LastSeen.IsZero() && heartbeatAge < unhealthyThreshold
	hbReason := ""
	hashIsStale := false
	if !heartbeatOK {
		if node.LastSeen.IsZero() {
			hbReason = "never seen"
		} else if heartbeatAge > heartbeatStaleThreshold {
			hbReason = fmt.Sprintf("unreachable — last seen %s ago, applied hash is stale",
				heartbeatAge.Truncate(time.Second))
			hashIsStale = true
		} else {
			hbReason = fmt.Sprintf("last seen %s ago", heartbeatAge.Truncate(time.Second))
		}
	}
	_ = hashIsStale // used by future hash comparison logic
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

	// 4. Version checks — compare installed vs desired, filtered by node profile.
	desiredCanon, _, _ := srv.loadDesiredServices(ctx)
	filtered := filterVersionsForNode(desiredCanon, node)
	assignedServices := ServicesForProfiles(node.Profiles)
	for svc, desiredVer := range filtered {
		// Skip services not assigned to this node's profiles.
		// Infrastructure/command packages are always checked; only workloads are filtered.
		if comp, ok := catalogIndex[svc]; ok && comp.Kind == KindWorkload && !assignedServices[svc] {
			continue
		}
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

			newStatus := "unhealthy"
		if timeSinceSeen > heartbeatStaleThreshold {
			newStatus = "unreachable"
		}
		if currentNode.Status != newStatus {
				currentNode.Status = newStatus
				currentNode.MarkedUnhealthySince = now
				currentNode.LastError = fmt.Sprintf("no contact for %v", timeSinceSeen.Round(time.Second))
				log.Printf("node %s marked %s: %s", node.NodeID, newStatus, currentNode.LastError)
				srv.emitClusterEvent("cluster.health.degraded", map[string]interface{}{
					"severity":       "WARNING",
					"node_id":        node.NodeID,
					"hostname":       node.Identity.Hostname,
					"reason":         currentNode.LastError,
					"correlation_id": fmt.Sprintf("node:%s", node.NodeID),
				})
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
		} else if (currentNode.Status == "unhealthy" || currentNode.Status == "unreachable") &&
		(previousStatus == "unhealthy" || previousStatus == "unreachable") {
			// Node came back online - reset recovery counters
			currentNode.Status = "healthy"
			currentNode.FailedHealthChecks = 0
			currentNode.RecoveryAttempts = 0
			currentNode.MarkedUnhealthySince = time.Time{}
			currentNode.LastError = ""
			log.Printf("node %s recovered and marked healthy", node.NodeID)
			srv.emitClusterEvent("cluster.health.recovered", map[string]interface{}{
				"severity":       "INFO",
				"node_id":        node.NodeID,
				"hostname":       node.Identity.Hostname,
				"correlation_id": fmt.Sprintf("node:%s", node.NodeID),
			})
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
	// Unreachable: no heartbeat beyond stale threshold.
	if !node.LastSeen.IsZero() && time.Since(node.LastSeen) > heartbeatStaleThreshold {
		return "unreachable", fmt.Sprintf("no heartbeat for %s",
			time.Since(node.LastSeen).Truncate(time.Second))
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
				// Controller self-update runs on all instances (followers and leader).
				srv.reconcileControllerSelfUpdate(ctx)
				if !srv.isLeader() {
					continue
				}
				srv.monitorNodeHealth(ctx)
			}
		}
	})
}

// GetSubsystemHealth returns the health state of all registered background
// subsystems (goroutines) in this controller process.
func (srv *server) GetSubsystemHealth(_ context.Context, _ *cluster_controllerpb.GetControllerSubsystemHealthRequest) (*cluster_controllerpb.GetControllerSubsystemHealthResponse, error) {
	entries := globular_service.SubsystemSnapshot()
	resp := &cluster_controllerpb.GetControllerSubsystemHealthResponse{
		Subsystems: make([]*cluster_controllerpb.ControllerSubsystemHealth, 0, len(entries)),
		Overall:    toControllerSubsystemState(globular_service.SubsystemOverallState()),
	}
	for _, e := range entries {
		sh := &cluster_controllerpb.ControllerSubsystemHealth{
			Name:       e.Name,
			State:      toControllerSubsystemState(e.State),
			LastError:  e.LastError,
			ErrorCount: e.ErrorCount,
			Metadata:   e.Metadata,
		}
		if !e.LastTick.IsZero() {
			sh.LastTick = timestamppb.New(e.LastTick)
		}
		resp.Subsystems = append(resp.Subsystems, sh)
	}
	return resp, nil
}

func toControllerSubsystemState(s globular_service.SubsystemState) cluster_controllerpb.ControllerSubsystemState {
	switch s {
	case globular_service.SubsystemHealthy:
		return cluster_controllerpb.ControllerSubsystemState_CONTROLLER_SUBSYSTEM_STATE_HEALTHY
	case globular_service.SubsystemDegraded:
		return cluster_controllerpb.ControllerSubsystemState_CONTROLLER_SUBSYSTEM_STATE_DEGRADED
	case globular_service.SubsystemFailed:
		return cluster_controllerpb.ControllerSubsystemState_CONTROLLER_SUBSYSTEM_STATE_FAILED
	case globular_service.SubsystemStarting:
		return cluster_controllerpb.ControllerSubsystemState_CONTROLLER_SUBSYSTEM_STATE_STARTING
	case globular_service.SubsystemStopped:
		return cluster_controllerpb.ControllerSubsystemState_CONTROLLER_SUBSYSTEM_STATE_STOPPED
	default:
		return cluster_controllerpb.ControllerSubsystemState_CONTROLLER_SUBSYSTEM_STATE_UNSPECIFIED
	}
}
