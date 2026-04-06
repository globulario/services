package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	backup_managerpb "github.com/globulario/services/golang/backup_manager/backup_managerpb"
	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"google.golang.org/protobuf/types/known/emptypb"
)

func registerComposedTools(s *server) {

	// ── cluster_get_operational_snapshot ─────────────────────────────────
	s.register(toolDef{
		Name:        "cluster_get_operational_snapshot",
		Description: "The highest-value diagnostic tool: returns a single combined view of cluster health, node list, doctor findings, and recent backup jobs. Aggregates data from 4 services in parallel; partial results are returned if any service is unavailable. Start here for a complete cluster overview.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"freshness": {Type: "string", Description: "Doctor data freshness: 'cached' (default, fast) or 'fresh' (forces new scan)"},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		outerCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		result := map[string]interface{}{}
		errors := []string{}
		var mu sync.Mutex
		var wg sync.WaitGroup

		// Shared state for cross-referencing after goroutines complete.
		var healthNodes []*cluster_controllerpb.NodeHealth
		var desiredServices []*cluster_controllerpb.DesiredService

		// 1. Cluster health
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := s.clients.get(outerCtx, controllerEndpoint())
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Sprintf("cluster controller unavailable: %v", err))
				mu.Unlock()
				return
			}
			client := cluster_controllerpb.NewClusterControllerServiceClient(conn)
			callCtx, callCancel := context.WithTimeout(authCtx(outerCtx), 10*time.Second)
			defer callCancel()

			resp, err := client.GetClusterHealthV1(callCtx, &cluster_controllerpb.GetClusterHealthV1Request{})
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Sprintf("GetClusterHealthV1: %v", err))
				mu.Unlock()
				return
			}

			healthyCount := 0
			unhealthyCount := 0
			for _, n := range resp.GetNodes() {
				if n.GetLastError() != "" || n.GetDesiredServicesHash() != n.GetAppliedServicesHash() {
					unhealthyCount++
				} else {
					healthyCount++
				}
			}

			overallStatus := "healthy"
			if unhealthyCount > 0 {
				if unhealthyCount == len(resp.GetNodes()) {
					overallStatus = "critical"
				} else {
					overallStatus = "degraded"
				}
			}

			mu.Lock()
			healthNodes = resp.GetNodes()
			result["health"] = map[string]interface{}{
				"overall_status": overallStatus,
				"node_count":     len(resp.GetNodes()),
				"healthy":        healthyCount,
				"unhealthy":      unhealthyCount,
			}
			mu.Unlock()
		}()

		// 2. Node list
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := s.clients.get(outerCtx, controllerEndpoint())
			if err != nil {
				// Error already captured by health goroutine if controller is down.
				return
			}
			client := cluster_controllerpb.NewClusterControllerServiceClient(conn)
			callCtx, callCancel := context.WithTimeout(authCtx(outerCtx), 10*time.Second)
			defer callCancel()

			resp, err := client.ListNodes(callCtx, &cluster_controllerpb.ListNodesRequest{})
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Sprintf("ListNodes: %v", err))
				mu.Unlock()
				return
			}

			// Also extract cluster identity from first node if available.
			nodes := make([]map[string]interface{}, 0, len(resp.GetNodes()))
			for _, n := range resp.GetNodes() {
				hostname := ""
				if id := n.GetIdentity(); id != nil {
					hostname = id.GetHostname()
				}

				lastSeenAgo := "never"
				if n.GetLastSeen() != nil {
					t := n.GetLastSeen().AsTime()
					lastSeenAgo = ago(t)
				}

				nodes = append(nodes, map[string]interface{}{
					"node_id":       n.GetNodeId(),
					"hostname":      hostname,
					"status":        n.GetStatus(),
					"profiles":      n.GetProfiles(),
					"last_seen_ago": lastSeenAgo,
				})
			}

			mu.Lock()
			result["nodes"] = nodes
			mu.Unlock()
		}()

		// 3. Doctor cluster report
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := s.clients.get(outerCtx, doctorEndpoint())
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Sprintf("cluster doctor unavailable: %v", err))
				mu.Unlock()
				return
			}
			client := cluster_doctorpb.NewClusterDoctorServiceClient(conn)
			callCtx, callCancel := context.WithTimeout(authCtx(outerCtx), 10*time.Second)
			defer callCancel()

			report, err := client.GetClusterReport(callCtx, &cluster_doctorpb.ClusterReportRequest{
				Freshness: freshnessArg(args),
			})
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Sprintf("GetClusterReport: %v", err))
				mu.Unlock()
				return
			}

			topIssues := make([]string, 0)
			for _, f := range report.GetFindings() {
				topIssues = append(topIssues, f.GetSummary())
				if len(topIssues) >= 5 {
					break
				}
			}

			statusName := "unknown"
			switch report.GetOverallStatus() {
			case cluster_doctorpb.ClusterStatus_CLUSTER_HEALTHY:
				statusName = "healthy"
			case cluster_doctorpb.ClusterStatus_CLUSTER_DEGRADED:
				statusName = "degraded"
			case cluster_doctorpb.ClusterStatus_CLUSTER_CRITICAL:
				statusName = "critical"
			}

			mu.Lock()
			result["doctor"] = map[string]interface{}{
				"overall_status": statusName,
				"finding_count":  len(report.GetFindings()),
				"top_issues":     topIssues,
				"freshness":      freshnessPayload(report.GetHeader()),
			}
			mu.Unlock()
		}()

		// 4. Recent backup jobs
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := s.clients.get(outerCtx, backupManagerEndpoint())
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Sprintf("backup manager unavailable: %v", err))
				mu.Unlock()
				return
			}
			client := backup_managerpb.NewBackupManagerServiceClient(conn)
			callCtx, callCancel := context.WithTimeout(authCtx(outerCtx), 10*time.Second)
			defer callCancel()

			resp, err := client.ListBackupJobs(callCtx, &backup_managerpb.ListBackupJobsRequest{
				Limit: 5,
			})
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Sprintf("ListBackupJobs: %v", err))
				mu.Unlock()
				return
			}

			jobs := make([]map[string]interface{}, 0, len(resp.GetJobs()))
			for _, j := range resp.GetJobs() {
				jobs = append(jobs, map[string]interface{}{
					"job_id":      j.GetJobId(),
					"type":        normalizeJobType(j.GetJobType()),
					"state":       normalizeJobState(j.GetState()),
					"finished_at": fmtTime(j.GetFinishedUnixMs()),
				})
			}

			mu.Lock()
			result["recent_jobs"] = jobs
			mu.Unlock()
		}()

		// 5. Desired state (for convergence cross-reference)
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := s.clients.get(outerCtx, controllerEndpoint())
			if err != nil {
				return // error already captured by health goroutine
			}
			client := cluster_controllerpb.NewClusterControllerServiceClient(conn)
			callCtx, callCancel := context.WithTimeout(authCtx(outerCtx), 10*time.Second)
			defer callCancel()

			state, err := client.GetDesiredState(callCtx, &emptypb.Empty{})
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Sprintf("GetDesiredState: %v", err))
				mu.Unlock()
				return
			}
			mu.Lock()
			desiredServices = state.GetServices()
			mu.Unlock()
		}()

		wg.Wait()

		// Cross-reference desired state with health data for convergence summary.
		if desiredServices != nil && healthNodes != nil {
			desiredMap := make(map[string]string, len(desiredServices))
			for _, svc := range desiredServices {
				desiredMap[svc.GetServiceId()] = svc.GetVersion()
			}

			// Merge InfrastructureRelease and ApplicationRelease entries
			// so infra daemons don't inflate the "drifted" count.
			if conn, err := s.clients.get(outerCtx, controllerEndpoint()); err == nil {
				resClient := cluster_controllerpb.NewResourcesServiceClient(conn)
				relCtx, relCancel := context.WithTimeout(authCtx(outerCtx), 5*time.Second)
				defer relCancel()
				if infraResp, err := resClient.ListInfrastructureReleases(relCtx, &cluster_controllerpb.ListInfrastructureReleasesRequest{}); err == nil {
					for _, rel := range infraResp.Items {
						if rel != nil && rel.Spec != nil && rel.Spec.Component != "" {
							if _, exists := desiredMap[rel.Spec.Component]; !exists {
								desiredMap[rel.Spec.Component] = rel.Spec.Version
							}
						}
					}
				}
				if appResp, err := resClient.ListApplicationReleases(relCtx, &cluster_controllerpb.ListApplicationReleasesRequest{}); err == nil {
					for _, rel := range appResp.Items {
						if rel != nil && rel.Spec != nil && rel.Spec.AppName != "" {
							if _, exists := desiredMap[rel.Spec.AppName]; !exists {
								desiredMap[rel.Spec.AppName] = rel.Spec.Version
							}
						}
					}
				}
			}

			convergedNodes := 0
			driftedNodes := 0
			totalServices := len(desiredMap)

			for _, n := range healthNodes {
				nodeDrifted := false
				for sid, desiredVer := range desiredMap {
					installedVer, ok := n.GetInstalledVersions()[sid]
					if !ok || installedVer != desiredVer {
						nodeDrifted = true
						break
					}
				}
				if nodeDrifted {
					driftedNodes++
				} else {
					convergedNodes++
				}
			}

			pct := 0
			if len(healthNodes) > 0 {
				pct = convergedNodes * 100 / len(healthNodes)
			}

			result["convergence"] = map[string]interface{}{
				"desired_services":    totalServices,
				"converged_nodes":     convergedNodes,
				"drifted_nodes":       driftedNodes,
				"convergence_percent": pct,
			}
		}

		result["errors"] = errors

		return result, nil
	})

	// ── cluster_get_node_full_status ────────────────────────────────────
	s.register(toolDef{
		Name:        "cluster_get_node_full_status",
		Description: "Returns a comprehensive status for a single node by aggregating health detail, installed packages, plan execution status, and doctor findings. Queries 4 services in parallel; partial results are returned if any service is unavailable.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"node_id":   {Type: "string", Description: "The node ID to inspect"},
				"freshness": {Type: "string", Description: "Doctor data freshness: 'cached' (default, fast) or 'fresh' (forces new scan)"},
			},
			Required: []string{"node_id"},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		nodeID := getStr(args, "node_id")
		if nodeID == "" {
			return nil, fmt.Errorf("node_id is required")
		}

		outerCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		result := map[string]interface{}{
			"node_id": nodeID,
		}
		errors := []string{}
		var mu sync.Mutex
		var wg sync.WaitGroup

		// 1. Node health detail
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := s.clients.get(outerCtx, controllerEndpoint())
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Sprintf("cluster controller unavailable: %v", err))
				mu.Unlock()
				return
			}
			client := cluster_controllerpb.NewClusterControllerServiceClient(conn)
			callCtx, callCancel := context.WithTimeout(authCtx(outerCtx), 10*time.Second)
			defer callCancel()

			resp, err := client.GetNodeHealthDetailV1(callCtx, &cluster_controllerpb.GetNodeHealthDetailV1Request{
				NodeId: nodeID,
			})
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Sprintf("GetNodeHealthDetailV1: %v", err))
				mu.Unlock()
				return
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

			mu.Lock()
			result["health"] = map[string]interface{}{
				"overall_status":      resp.GetOverallStatus(),
				"healthy":             resp.GetHealthy(),
				"checks":              checks,
				"last_error":          resp.GetLastError(),
				"can_apply_privileged": resp.GetCanApplyPrivileged(),
				"last_seen_ago":       lastSeenAgo,
			}
			mu.Unlock()
		}()

		// 2. Installed packages
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := s.clients.get(outerCtx, nodeAgentEndpoint())
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Sprintf("node agent unavailable: %v", err))
				mu.Unlock()
				return
			}
			client := node_agentpb.NewNodeAgentServiceClient(conn)
			callCtx, callCancel := context.WithTimeout(authCtx(outerCtx), 10*time.Second)
			defer callCancel()

			resp, err := client.ListInstalledPackages(callCtx, &node_agentpb.ListInstalledPackagesRequest{
				NodeId: nodeID,
			})
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Sprintf("ListInstalledPackages: %v", err))
				mu.Unlock()
				return
			}

			pkgs := make([]map[string]interface{}, 0, len(resp.GetPackages()))
			for _, p := range resp.GetPackages() {
				pkgs = append(pkgs, map[string]interface{}{
					"name":    p.GetName(),
					"version": p.GetVersion(),
					"kind":    p.GetKind(),
					"status":  p.GetStatus(),
				})
			}

			mu.Lock()
			result["installed_packages"] = pkgs
			mu.Unlock()
		}()

		// 3. Plan status
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := s.clients.get(outerCtx, nodeAgentEndpoint())
			if err != nil {
				return // error already captured above
			}
			client := node_agentpb.NewNodeAgentServiceClient(conn)
			callCtx, callCancel := context.WithTimeout(authCtx(outerCtx), 10*time.Second)
			defer callCancel()

			// Plan status removed — plan system deleted.
			_ = callCtx
			_ = client
			mu.Lock()
			result["plan_status"] = map[string]interface{}{"status": "plan system removed"}
			mu.Unlock()
		}()

		// 4. Doctor node report
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := s.clients.get(outerCtx, doctorEndpoint())
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Sprintf("cluster doctor unavailable: %v", err))
				mu.Unlock()
				return
			}
			client := cluster_doctorpb.NewClusterDoctorServiceClient(conn)
			callCtx, callCancel := context.WithTimeout(authCtx(outerCtx), 10*time.Second)
			defer callCancel()

			report, err := client.GetNodeReport(callCtx, &cluster_doctorpb.NodeReportRequest{
				NodeId:    nodeID,
				Freshness: freshnessArg(args),
			})
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Sprintf("GetNodeReport: %v", err))
				mu.Unlock()
				return
			}

			findings := make([]map[string]interface{}, 0, len(report.GetFindings()))
			for _, f := range report.GetFindings() {
				findings = append(findings, map[string]interface{}{
					"severity": f.GetSeverity().String(),
					"category": f.GetCategory(),
					"summary":  f.GetSummary(),
				})
			}

			mu.Lock()
			result["doctor"] = map[string]interface{}{
				"reachable":              report.GetReachable(),
				"heartbeat_age_seconds":  report.GetHeartbeatAgeSeconds(),
				"finding_count":          len(report.GetFindings()),
				"findings":               findings,
				"freshness":              freshnessPayload(report.GetHeader()),
			}
			mu.Unlock()
		}()

		wg.Wait()

		result["errors"] = errors

		return result, nil
	})

	// ── backup_get_recovery_posture ─────────────────────────────────────
	s.register(toolDef{
		Name:        "backup_get_recovery_posture",
		Description: "Returns a comprehensive recovery readiness assessment by aggregating recovery seed status, recent backups, retention policy, and schedule configuration. Queries 4 endpoints in parallel for a single-call disaster recovery overview.",
		InputSchema: inputSchema{Type: "object"},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		outerCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		result := map[string]interface{}{}
		errors := []string{}
		var mu sync.Mutex
		var wg sync.WaitGroup

		// 1. Recovery status
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := s.clients.get(outerCtx, backupManagerEndpoint())
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Sprintf("backup manager unavailable: %v", err))
				mu.Unlock()
				return
			}
			client := backup_managerpb.NewBackupManagerServiceClient(conn)
			callCtx, callCancel := context.WithTimeout(authCtx(outerCtx), 10*time.Second)
			defer callCancel()

			resp, err := client.GetRecoveryStatus(callCtx, &backup_managerpb.GetRecoveryStatusRequest{})
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Sprintf("GetRecoveryStatus: %v", err))
				mu.Unlock()
				return
			}

			recovery := map[string]interface{}{
				"seed_present":           resp.GetSeedPresent(),
				"destination_configured": resp.GetDestinationConfigured(),
				"credentials_available":  resp.GetCredentialsAvailable(),
				"seed_matches_config":    resp.GetSeedMatchesConfig(),
				"message":                resp.GetMessage(),
			}

			mu.Lock()
			result["recovery"] = recovery
			mu.Unlock()
		}()

		// 2. Recent backups
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := s.clients.get(outerCtx, backupManagerEndpoint())
			if err != nil {
				return // error already captured above
			}
			client := backup_managerpb.NewBackupManagerServiceClient(conn)
			callCtx, callCancel := context.WithTimeout(authCtx(outerCtx), 10*time.Second)
			defer callCancel()

			resp, err := client.ListBackups(callCtx, &backup_managerpb.ListBackupsRequest{
				Limit: 3,
			})
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Sprintf("ListBackups: %v", err))
				mu.Unlock()
				return
			}

			backups := make([]map[string]interface{}, 0, len(resp.GetBackups()))
			for _, b := range resp.GetBackups() {
				backups = append(backups, map[string]interface{}{
					"backup_id":     b.GetBackupId(),
					"created_at":    fmtTime(b.GetCreatedUnixMs()),
					"total_size":    fmtBytes(b.GetTotalBytes()),
					"quality_state": normalizeQualityState(b.GetQualityState()),
					"mode":          normalizeBackupMode(b.GetMode()),
				})
			}

			mu.Lock()
			result["recent_backups"] = backups
			mu.Unlock()
		}()

		// 3. Retention status
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := s.clients.get(outerCtx, backupManagerEndpoint())
			if err != nil {
				return
			}
			client := backup_managerpb.NewBackupManagerServiceClient(conn)
			callCtx, callCancel := context.WithTimeout(authCtx(outerCtx), 10*time.Second)
			defer callCancel()

			resp, err := client.GetRetentionStatus(callCtx, &backup_managerpb.GetRetentionStatusRequest{})
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Sprintf("GetRetentionStatus: %v", err))
				mu.Unlock()
				return
			}

			retention := map[string]interface{}{
				"current_backup_count": resp.GetCurrentBackupCount(),
				"current_total_size":   fmtBytes(resp.GetCurrentTotalBytes()),
				"oldest_backup_at":     fmtTime(resp.GetOldestBackupUnixMs()),
				"newest_backup_at":     fmtTime(resp.GetNewestBackupUnixMs()),
			}

			if p := resp.GetPolicy(); p != nil {
				retention["policy"] = map[string]interface{}{
					"keep_last_n": p.GetKeepLastN(),
					"keep_days":   p.GetKeepDays(),
				}
			}

			mu.Lock()
			result["retention"] = retention
			mu.Unlock()
		}()

		// 4. Schedule status
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := s.clients.get(outerCtx, backupManagerEndpoint())
			if err != nil {
				return
			}
			client := backup_managerpb.NewBackupManagerServiceClient(conn)
			callCtx, callCancel := context.WithTimeout(authCtx(outerCtx), 10*time.Second)
			defer callCancel()

			resp, err := client.GetScheduleStatus(callCtx, &backup_managerpb.GetScheduleStatusRequest{})
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Sprintf("GetScheduleStatus: %v", err))
				mu.Unlock()
				return
			}

			mu.Lock()
			result["schedule"] = map[string]interface{}{
				"enabled":      resp.GetEnabled(),
				"interval":     resp.GetInterval(),
				"next_fire_at": fmtTime(resp.GetNextFireUnixMs()),
			}
			mu.Unlock()
		}()

		wg.Wait()

		result["errors"] = errors

		return result, nil
	})
}
