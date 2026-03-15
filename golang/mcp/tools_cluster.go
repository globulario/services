package main

import (
	"context"
	"fmt"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"google.golang.org/protobuf/types/known/emptypb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func registerClusterTools(s *server) {

	// ── cluster_get_info ────────────────────────────────────────────────
	s.register(toolDef{
		Name:        "cluster_get_info",
		Description: "Returns basic cluster identity: cluster ID, domain, and creation time. Use this first to confirm which cluster you are connected to before running other diagnostics.",
		InputSchema: inputSchema{Type: "object"},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		conn, err := s.clients.get(ctx, controllerEndpoint())
		if err != nil {
			return nil, err
		}
		client := cluster_controllerpb.NewClusterControllerServiceClient(conn)

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		info, err := client.GetClusterInfo(callCtx, &timestamppb.Timestamp{})
		if err != nil {
			return nil, fmt.Errorf("GetClusterInfo: %w", err)
		}

		createdAt := ""
		if info.GetCreatedAt() != nil {
			createdAt = fmtTimestamp(info.GetCreatedAt().GetSeconds(), info.GetCreatedAt().GetNanos())
		}

		return map[string]interface{}{
			"cluster_id":     info.GetClusterId(),
			"cluster_domain": info.GetClusterDomain(),
			"created_at":     createdAt,
		}, nil
	})

	// ── cluster_get_health ──────────────────────────────────────────────
	s.register(toolDef{
		Name:        "cluster_get_health",
		Description: "Returns a cluster-wide health summary: overall status, per-node convergence state (desired vs applied hash), plan phase, and service rollout progress. Use this to quickly assess whether all nodes are converged or if any are drifted/unhealthy.",
		InputSchema: inputSchema{Type: "object"},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		conn, err := s.clients.get(ctx, controllerEndpoint())
		if err != nil {
			return nil, err
		}
		client := cluster_controllerpb.NewClusterControllerServiceClient(conn)

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		resp, err := client.GetClusterHealthV1(callCtx, &cluster_controllerpb.GetClusterHealthV1Request{})
		if err != nil {
			return nil, fmt.Errorf("GetClusterHealthV1: %w", err)
		}

		// Normalize nodes.
		healthyCount := 0
		unhealthyCount := 0
		nodes := make([]map[string]interface{}, 0, len(resp.GetNodes()))
		for _, n := range resp.GetNodes() {
			status := "healthy"
			if n.GetLastError() != "" || n.GetDesiredServicesHash() != n.GetAppliedServicesHash() {
				status = "unhealthy"
				unhealthyCount++
			} else {
				healthyCount++
			}

			// Build health checks from installed versions.
			healthChecks := make([]map[string]interface{}, 0)
			for svc, ver := range n.GetInstalledVersions() {
				healthChecks = append(healthChecks, map[string]interface{}{
					"service": svc,
					"version": ver,
				})
			}

			nodes = append(nodes, map[string]interface{}{
				"node_id":              n.GetNodeId(),
				"status":              status,
				"desired_hash":        n.GetDesiredServicesHash(),
				"applied_hash":        n.GetAppliedServicesHash(),
				"current_plan_phase":  n.GetCurrentPlanPhase(),
				"last_error":          n.GetLastError(),
				"can_apply_privileged": n.GetCanApplyPrivileged(),
				"health_checks":       healthChecks,
			})
		}

		overallStatus := "healthy"
		if unhealthyCount > 0 {
			if unhealthyCount == len(resp.GetNodes()) {
				overallStatus = "critical"
			} else {
				overallStatus = "degraded"
			}
		}

		return map[string]interface{}{
			"overall_status":  overallStatus,
			"node_count":      len(resp.GetNodes()),
			"healthy_count":   healthyCount,
			"unhealthy_count": unhealthyCount,
			"nodes":           nodes,
		}, nil
	})

	// ── cluster_list_nodes ──────────────────────────────────────────────
	s.register(toolDef{
		Name:        "cluster_list_nodes",
		Description: "Lists all registered cluster nodes with their identity (hostname, IPs), status, assigned profiles, and agent endpoint. Use this to see the full node inventory or find a specific node_id for detailed queries.",
		InputSchema: inputSchema{Type: "object"},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		conn, err := s.clients.get(ctx, controllerEndpoint())
		if err != nil {
			return nil, err
		}
		client := cluster_controllerpb.NewClusterControllerServiceClient(conn)

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		resp, err := client.ListNodes(callCtx, &cluster_controllerpb.ListNodesRequest{})
		if err != nil {
			return nil, fmt.Errorf("ListNodes: %w", err)
		}

		nodes := make([]map[string]interface{}, 0, len(resp.GetNodes()))
		for _, n := range resp.GetNodes() {
			hostname := ""
			var ips []string
			var capabilities map[string]interface{}

			if id := n.GetIdentity(); id != nil {
				hostname = id.GetHostname()
				ips = id.GetIps()
			}

			if caps := n.GetCapabilities(); caps != nil {
				capabilities = map[string]interface{}{
					"cpu_count":            caps.GetCpuCount(),
					"ram":                  fmtBytes(caps.GetRamBytes()),
					"disk":                 fmtBytes(caps.GetDiskBytes()),
					"disk_free":            fmtBytes(caps.GetDiskFreeBytes()),
					"can_apply_privileged": caps.GetCanApplyPrivileged(),
				}
			}

			lastSeen := ""
			if n.GetLastSeen() != nil {
				lastSeen = fmtTimestamp(n.GetLastSeen().GetSeconds(), n.GetLastSeen().GetNanos())
			}

			nodes = append(nodes, map[string]interface{}{
				"node_id":        n.GetNodeId(),
				"hostname":       hostname,
				"ips":            ips,
				"status":         n.GetStatus(),
				"profiles":       n.GetProfiles(),
				"agent_endpoint": n.GetAgentEndpoint(),
				"last_seen":      lastSeen,
				"capabilities":   capabilities,
			})
		}

		return map[string]interface{}{
			"nodes": nodes,
		}, nil
	})

	// ── cluster_get_node_health_detail ───────────────────────────────────
	s.register(toolDef{
		Name:        "cluster_get_node_health_detail",
		Description: "Returns detailed health for a single node: overall status, individual subsystem checks (heartbeat, units, versions, inventory), last error, and privilege capability. Use this to drill into why a specific node is unhealthy.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"node_id": {Type: "string", Description: "The node ID to inspect (from cluster_list_nodes or cluster_get_health)"},
			},
			Required: []string{"node_id"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		nodeID := getStr(args, "node_id")
		if nodeID == "" {
			return nil, fmt.Errorf("node_id is required")
		}

		conn, err := s.clients.get(ctx, controllerEndpoint())
		if err != nil {
			return nil, err
		}
		client := cluster_controllerpb.NewClusterControllerServiceClient(conn)

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		resp, err := client.GetNodeHealthDetailV1(callCtx, &cluster_controllerpb.GetNodeHealthDetailV1Request{
			NodeId: nodeID,
		})
		if err != nil {
			return nil, fmt.Errorf("GetNodeHealthDetailV1: %w", err)
		}

		checks := make([]map[string]interface{}, 0, len(resp.GetChecks()))
		for _, c := range resp.GetChecks() {
			checks = append(checks, map[string]interface{}{
				"subsystem": c.GetSubsystem(),
				"ok":        c.GetOk(),
				"reason":    c.GetReason(),
			})
		}

		lastSeenAgo := "never"
		if resp.GetLastSeen() != nil {
			t := resp.GetLastSeen().AsTime()
			lastSeenAgo = ago(t)
		}

		return map[string]interface{}{
			"node_id":              resp.GetNodeId(),
			"overall_status":      resp.GetOverallStatus(),
			"healthy":             resp.GetHealthy(),
			"checks":              checks,
			"last_error":          resp.GetLastError(),
			"can_apply_privileged": resp.GetCanApplyPrivileged(),
			"inventory_complete":  resp.GetInventoryComplete(),
			"last_seen_ago":       lastSeenAgo,
		}, nil
	})

	// ── cluster_get_node_plan ───────────────────────────────────────────
	s.register(toolDef{
		Name:        "cluster_get_node_plan",
		Description: "Returns the current pending convergence plan for a node, including plan ID, generation, reason, expiry, and spec details. Use this to understand what changes are queued for a node or confirm there is no pending plan.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"node_id": {Type: "string", Description: "The node ID to get the plan for"},
			},
			Required: []string{"node_id"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		nodeID := getStr(args, "node_id")
		if nodeID == "" {
			return nil, fmt.Errorf("node_id is required")
		}

		conn, err := s.clients.get(ctx, controllerEndpoint())
		if err != nil {
			return nil, err
		}
		client := cluster_controllerpb.NewClusterControllerServiceClient(conn)

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		resp, err := client.GetNodePlanV1(callCtx, &cluster_controllerpb.GetNodePlanV1Request{
			NodeId: nodeID,
		})
		if err != nil {
			return nil, fmt.Errorf("GetNodePlanV1: %w", err)
		}

		plan := resp.GetPlan()
		if plan == nil {
			return map[string]interface{}{
				"node_id": nodeID,
				"status":  "no pending plan",
			}, nil
		}

		return map[string]interface{}{
			"node_id":        plan.GetNodeId(),
			"plan_id":        plan.GetPlanId(),
			"generation":     plan.GetGeneration(),
			"created_at":     fmtTime(int64(plan.GetCreatedUnixMs())),
			"expires_at":     fmtTime(int64(plan.GetExpiresUnixMs())),
			"issued_by":      plan.GetIssuedBy(),
			"reason":         plan.GetReason(),
			"desired_hash":   plan.GetDesiredHash(),
			"locks":          plan.GetLocks(),
			"api_version":    plan.GetApiVersion(),
			"kind":           plan.GetKind(),
		}, nil
	})

	// ── cluster_get_desired_state ───────────────────────────────────────
	s.register(toolDef{
		Name:        "cluster_get_desired_state",
		Description: "Returns the full desired-state manifest: all services with their target versions, platforms, and build numbers, plus the current revision. Use this to see what the cluster should converge to and compare against actual installed state.",
		InputSchema: inputSchema{Type: "object"},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		conn, err := s.clients.get(ctx, controllerEndpoint())
		if err != nil {
			return nil, err
		}
		client := cluster_controllerpb.NewClusterControllerServiceClient(conn)

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		state, err := client.GetDesiredState(callCtx, &emptypb.Empty{})
		if err != nil {
			return nil, fmt.Errorf("GetDesiredState: %w", err)
		}

		services := make([]map[string]interface{}, 0, len(state.GetServices()))
		for _, svc := range state.GetServices() {
			services = append(services, map[string]interface{}{
				"service_id":   svc.GetServiceId(),
				"version":      svc.GetVersion(),
				"platform":     svc.GetPlatform(),
				"build_number": svc.GetBuildNumber(),
			})
		}

		return map[string]interface{}{
			"services": services,
			"revision": state.GetRevision(),
		}, nil
	})
}
