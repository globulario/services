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

		// Fetch per-service release phases (best-effort).
		resConn, resErr := s.clients.get(ctx, controllerEndpoint())
		var servicePhases []map[string]interface{}
		if resErr == nil {
			resClient := cluster_controllerpb.NewResourcesServiceClient(resConn)
			relCtx, relCancel := context.WithTimeout(authCtx(ctx), 5*time.Second)
			defer relCancel()
			if relResp, err := resClient.ListServiceReleases(relCtx, &cluster_controllerpb.ListServiceReleasesRequest{}); err == nil {
				for _, rel := range relResp.Items {
					phase := ""
					if rel.Status != nil {
						phase = rel.Status.Phase
					}
					name := ""
					if rel.Meta != nil {
						name = rel.Meta.Name
					}
					servicePhases = append(servicePhases, map[string]interface{}{
						"release_name": name,
						"phase":        phase,
					})
				}
			}
		}

		result := map[string]interface{}{
			"overall_status":  overallStatus,
			"node_count":      len(resp.GetNodes()),
			"healthy_count":   healthyCount,
			"unhealthy_count": unhealthyCount,
			"nodes":           nodes,
		}
		if len(servicePhases) > 0 {
			result["service_phases"] = servicePhases
		}
		return result, nil
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

		// Plan system removed — return empty plan info.
		_ = callCtx
		_ = client
		{
			return map[string]interface{}{
				"node_id": nodeID,
				"status":  "no pending plan",
			}, nil
		}

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

	// ── cluster_get_service_workflow_status ────────────────────────────
	s.register(toolDef{
		Name:        "cluster_get_service_workflow_status",
		Description: "Returns release workflow status for services, applications, and infrastructure: phase (PENDING→RESOLVED→AVAILABLE/DEGRADED/FAILED/ROLLED_BACK/REMOVING→REMOVED), workflow_kind (install/upgrade/remove), started_at, transition_reason, per-node status with failed_step, and errors. Live execution state comes from workflow runs (see workflow_list_runs). APPLYING may appear on legacy rows but is no longer written by the workflow engine.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"service_name": {Type: "string", Description: "Optional name filter (e.g. 'authentication'). Omit to list all."},
				"kind":         {Type: "string", Description: "Optional kind filter: 'service', 'application', 'infrastructure'. Omit for all."},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		conn, err := s.clients.get(ctx, controllerEndpoint())
		if err != nil {
			return nil, err
		}
		resClient := cluster_controllerpb.NewResourcesServiceClient(conn)

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 10*time.Second)
		defer cancel()

		filterName := getStr(args, "service_name")
		filterKind := getStr(args, "kind")

		releases := make([]map[string]interface{}, 0)

		// Helper to build node status entries with new fields.
		buildNodeStatuses := func(nodes []*cluster_controllerpb.NodeReleaseStatus) []map[string]interface{} {
			out := make([]map[string]interface{}, 0, len(nodes))
			for _, n := range nodes {
				if n == nil {
					continue
				}
				entry := map[string]interface{}{
					"node_id":           n.NodeID,
					"phase":             n.Phase,
					"installed_version": n.InstalledVersion,
					"error":             n.ErrorMessage,
					"updated":           fmtTime(n.UpdatedUnixMs),
				}
				if n.FailedStepID != "" {
					entry["failed_step"] = n.FailedStepID
				}
				out = append(out, entry)
			}
			return out
		}

		// ServiceReleases
		if filterKind == "" || filterKind == "service" {
			resp, err := resClient.ListServiceReleases(callCtx, &cluster_controllerpb.ListServiceReleasesRequest{})
			if err == nil {
				for _, rel := range resp.Items {
					if rel == nil || rel.Meta == nil {
						continue
					}
					if filterName != "" && (rel.Spec == nil || rel.Spec.ServiceName != filterName) {
						continue
					}
					entry := map[string]interface{}{
						"release_name":  rel.Meta.Name,
						"resource_kind": "ServiceRelease",
					}
					if rel.Spec != nil {
						entry["service_name"] = rel.Spec.ServiceName
						entry["desired_version"] = rel.Spec.Version
						entry["build_number"] = rel.Spec.BuildNumber
						entry["publisher_id"] = rel.Spec.PublisherID
						entry["paused"] = rel.Spec.Paused
						entry["removing"] = rel.Spec.Removing
					}
					if rel.Status != nil {
						entry["phase"] = rel.Status.Phase
						entry["resolved_version"] = rel.Status.ResolvedVersion
						entry["desired_hash"] = rel.Status.DesiredHash
						entry["message"] = rel.Status.Message
						entry["last_transition"] = fmtTime(rel.Status.LastTransitionUnixMs)
						entry["workflow_kind"] = rel.Status.WorkflowKind
						entry["started_at"] = fmtTime(rel.Status.StartedAtUnixMs)
						entry["transition_reason"] = rel.Status.TransitionReason
						entry["nodes"] = buildNodeStatuses(rel.Status.Nodes)
					}
					releases = append(releases, entry)
				}
			}
		}

		// ApplicationReleases
		if filterKind == "" || filterKind == "application" {
			appResp, err := resClient.ListApplicationReleases(callCtx, &cluster_controllerpb.ListApplicationReleasesRequest{})
			if err == nil {
				for _, rel := range appResp.Items {
					if rel == nil || rel.Meta == nil {
						continue
					}
					if filterName != "" && (rel.Spec == nil || rel.Spec.AppName != filterName) {
						continue
					}
					entry := map[string]interface{}{
						"release_name":  rel.Meta.Name,
						"resource_kind": "ApplicationRelease",
					}
					if rel.Spec != nil {
						entry["service_name"] = rel.Spec.AppName
						entry["desired_version"] = rel.Spec.Version
						entry["publisher_id"] = rel.Spec.PublisherID
						entry["removing"] = rel.Spec.Removing
					}
					if rel.Status != nil {
						entry["phase"] = rel.Status.Phase
						entry["resolved_version"] = rel.Status.ResolvedVersion
						entry["desired_hash"] = rel.Status.DesiredHash
						entry["message"] = rel.Status.Message
						entry["last_transition"] = fmtTime(rel.Status.LastTransitionUnixMs)
						entry["workflow_kind"] = rel.Status.WorkflowKind
						entry["started_at"] = fmtTime(rel.Status.StartedAtUnixMs)
						entry["transition_reason"] = rel.Status.TransitionReason
						entry["nodes"] = buildNodeStatuses(rel.Status.Nodes)
					}
					releases = append(releases, entry)
				}
			}
		}

		// InfrastructureReleases
		if filterKind == "" || filterKind == "infrastructure" {
			infraResp, err := resClient.ListInfrastructureReleases(callCtx, &cluster_controllerpb.ListInfrastructureReleasesRequest{})
			if err == nil {
				for _, rel := range infraResp.Items {
					if rel == nil || rel.Meta == nil {
						continue
					}
					if filterName != "" && (rel.Spec == nil || rel.Spec.Component != filterName) {
						continue
					}
					entry := map[string]interface{}{
						"release_name":  rel.Meta.Name,
						"resource_kind": "InfrastructureRelease",
					}
					if rel.Spec != nil {
						entry["service_name"] = rel.Spec.Component
						entry["desired_version"] = rel.Spec.Version
						entry["publisher_id"] = rel.Spec.PublisherID
						entry["removing"] = rel.Spec.Removing
					}
					if rel.Status != nil {
						entry["phase"] = rel.Status.Phase
						entry["resolved_version"] = rel.Status.ResolvedVersion
						entry["desired_hash"] = rel.Status.DesiredHash
						entry["message"] = rel.Status.Message
						entry["last_transition"] = fmtTime(rel.Status.LastTransitionUnixMs)
						entry["workflow_kind"] = rel.Status.WorkflowKind
						entry["started_at"] = fmtTime(rel.Status.StartedAtUnixMs)
						entry["transition_reason"] = rel.Status.TransitionReason
						entry["nodes"] = buildNodeStatuses(rel.Status.Nodes)
					}
					releases = append(releases, entry)
				}
			}
		}

		return map[string]interface{}{
			"releases": releases,
			"count":    len(releases),
		}, nil
	})

	// ── node_resolve ────────────────────────────────────────────────────
	// Phase 1 projection: "who is this node?". Scoped (Clause 5), flat
	// (Clause 11), declares its own freshness (Clause 4). MUST NOT be
	// extended with services/packages/metrics/logs — those belong in
	// separate tools. See docs/architecture/projection-clauses.md.
	s.register(toolDef{
		Name: "node_resolve",
		Description: "Resolves a node's identity from any of: node_id (uuid), hostname, mac, or ip. " +
			"Returns only identity fields — no services, packages, metrics, or health. " +
			"Chain into other tools (cluster_get_node_health_detail, nodeagent_list_installed_packages) " +
			"for those. The response's `source` field tells you whether the answer came from the " +
			"scylla projection or the cluster-controller fallback.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"identifier": {
					Type:        "string",
					Description: "node_id (uuid), hostname, mac (xx:xx:xx:xx:xx:xx), or ip (dotted-quad)",
				},
			},
			Required: []string{"identifier"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		identifier, _ := args["identifier"].(string)
		if identifier == "" {
			return nil, fmt.Errorf("identifier is required")
		}

		conn, err := s.clients.get(ctx, controllerEndpoint())
		if err != nil {
			return nil, err
		}
		client := cluster_controllerpb.NewClusterControllerServiceClient(conn)

		callCtx, cancel := context.WithTimeout(authCtx(ctx), 5*time.Second)
		defer cancel()

		rsp, err := client.ResolveNode(callCtx, &cluster_controllerpb.ResolveNodeRequest{
			Identifier: identifier,
		})
		if err != nil {
			return nil, fmt.Errorf("ResolveNode: %w", err)
		}
		id := rsp.GetIdentity()
		if id == nil {
			return nil, fmt.Errorf("no identity returned for %q", identifier)
		}
		return map[string]interface{}{
			"node_id":     id.GetNodeId(),
			"hostname":    id.GetHostname(),
			"ips":         id.GetIps(),
			"macs":        id.GetMacs(),
			"labels":      id.GetLabels(),
			"source":      id.GetSource(),
			"observed_at": id.GetObservedAt(),
		}, nil
	})
}
