package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"google.golang.org/protobuf/types/known/emptypb"
)

func registerConvergenceTools(s *server) {

	// ── cluster_get_convergence_detail ──────────────────────────────────
	s.register(toolDef{
		Name:        "cluster_get_convergence_detail",
		Description: "Per-service convergence breakdown for a node: compares desired state against installed packages and health data to classify each service as converged, drifted, missing, or unmanaged. Use this to find exactly which services are out of sync and why hash mismatches occur.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"node_id": {Type: "string", Description: "The node ID to inspect. If omitted, returns cluster-wide convergence."},
			},
		},
	}, func(ctx context.Context, args map[string]interface{}) (interface{}, error) {
		nodeID := getStr(args, "node_id")

		outerCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		var (
			desiredServices []map[string]interface{}
			healthNodes     []*cluster_controllerpb.NodeHealth
			installedPkgs   []*node_agentpb.InstalledPackage
			mu              sync.Mutex
			wg              sync.WaitGroup
			errors          []string
		)

		// 1. Desired state
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

			state, err := client.GetDesiredState(callCtx, &emptypb.Empty{})
			if err != nil {
				mu.Lock()
				errors = append(errors, fmt.Sprintf("GetDesiredState: %v", err))
				mu.Unlock()
				return
			}

			svcs := make([]map[string]interface{}, 0, len(state.GetServices()))
			for _, svc := range state.GetServices() {
				svcs = append(svcs, map[string]interface{}{
					"service_id":   svc.GetServiceId(),
					"version":      svc.GetVersion(),
					"platform":     svc.GetPlatform(),
					"build_number": svc.GetBuildNumber(),
				})
			}
			mu.Lock()
			desiredServices = svcs
			mu.Unlock()
		}()

		// 2. Cluster health (per-node installed versions + hashes)
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := s.clients.get(outerCtx, controllerEndpoint())
			if err != nil {
				return // error captured above
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
			mu.Lock()
			healthNodes = resp.GetNodes()
			mu.Unlock()
		}()

		// 3. Installed packages (if node_id specified)
		if nodeID != "" {
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
				mu.Lock()
				installedPkgs = resp.GetPackages()
				mu.Unlock()
			}()
		}

		wg.Wait()

		if desiredServices == nil && healthNodes == nil {
			return map[string]interface{}{
				"errors": errors,
			}, nil
		}

		// Build desired version map: service_id -> version
		desiredMap := make(map[string]string)
		for _, svc := range desiredServices {
			sid, _ := svc["service_id"].(string)
			ver, _ := svc["version"].(string)
			if sid != "" {
				desiredMap[sid] = ver
			}
		}

		// Merge InfrastructureRelease and ApplicationRelease entries so infra
		// daemons and apps don't show as "unmanaged" in convergence views.
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

		// If a specific node is requested, do per-service diff for that node
		if nodeID != "" {
			return convergenceForNode(nodeID, desiredMap, healthNodes, installedPkgs, errors), nil
		}

		// Cluster-wide: per-node convergence summary
		nodeSummaries := make([]map[string]interface{}, 0, len(healthNodes))
		for _, n := range healthNodes {
			converged := 0
			drifted := 0
			missing := 0
			unmanaged := 0

			installed := n.GetInstalledVersions()
			seen := make(map[string]bool)

			for sid, desiredVer := range desiredMap {
				installedVer, ok := installed[sid]
				if !ok {
					missing++
				} else if installedVer != desiredVer {
					drifted++
				} else {
					converged++
				}
				seen[sid] = true
			}

			for sid := range installed {
				if !seen[sid] {
					unmanaged++
				}
			}

			hashMatch := n.GetDesiredServicesHash() == n.GetAppliedServicesHash()
			nodeSummaries = append(nodeSummaries, map[string]interface{}{
				"node_id":    n.GetNodeId(),
				"hash_match": hashMatch,
				"converged":  converged,
				"drifted":    drifted,
				"missing":    missing,
				"unmanaged":  unmanaged,
			})
		}

		return map[string]interface{}{
			"desired_service_count": len(desiredMap),
			"node_count":           len(nodeSummaries),
			"nodes":                nodeSummaries,
			"errors":               errors,
		}, nil
	})

	// ── cluster_get_reconcile_status ────────────────────────────────────
	s.register(toolDef{
		Name:        "cluster_get_reconcile_status",
		Description: "Diagnoses why a node is not converging: checks privilege capability, pending plan state, plan execution progress, and last errors. Returns a human-readable diagnosis explaining what is blocking convergence.",
		InputSchema: inputSchema{
			Type: "object",
			Properties: map[string]propSchema{
				"node_id": {Type: "string", Description: "The node ID to diagnose (required)"},
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

		var (
			health rpcResult[*cluster_controllerpb.GetNodeHealthDetailV1Response]
			plan   rpcResult[*cluster_controllerpb.GetNodePlanV1Response]
			pstat  rpcResult[*node_agentpb.GetPlanStatusV1Response]
			wg     sync.WaitGroup
		)

		// 1. Node health detail
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := s.clients.get(outerCtx, controllerEndpoint())
			if err != nil {
				health.err = err
				return
			}
			client := cluster_controllerpb.NewClusterControllerServiceClient(conn)
			callCtx, callCancel := context.WithTimeout(authCtx(outerCtx), 10*time.Second)
			defer callCancel()
			health.resp, health.err = client.GetNodeHealthDetailV1(callCtx, &cluster_controllerpb.GetNodeHealthDetailV1Request{
				NodeId: nodeID,
			})
		}()

		// 2. Node plan
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := s.clients.get(outerCtx, controllerEndpoint())
			if err != nil {
				plan.err = err
				return
			}
			client := cluster_controllerpb.NewClusterControllerServiceClient(conn)
			callCtx, callCancel := context.WithTimeout(authCtx(outerCtx), 10*time.Second)
			defer callCancel()
			plan.resp, plan.err = client.GetNodePlanV1(callCtx, &cluster_controllerpb.GetNodePlanV1Request{
				NodeId: nodeID,
			})
		}()

		// 3. Plan execution status
		wg.Add(1)
		go func() {
			defer wg.Done()
			conn, err := s.clients.get(outerCtx, nodeAgentEndpoint())
			if err != nil {
				pstat.err = err
				return
			}
			client := node_agentpb.NewNodeAgentServiceClient(conn)
			callCtx, callCancel := context.WithTimeout(authCtx(outerCtx), 10*time.Second)
			defer callCancel()
			pstat.resp, pstat.err = client.GetPlanStatusV1(callCtx, &node_agentpb.GetPlanStatusV1Request{
				NodeId: nodeID,
			})
		}()

		wg.Wait()

		result := map[string]interface{}{
			"node_id": nodeID,
		}
		errors := []string{}

		// Process health
		if health.err != nil {
			errors = append(errors, fmt.Sprintf("GetNodeHealthDetailV1: %v", health.err))
		} else if health.resp != nil {
			checks := make([]map[string]interface{}, 0, len(health.resp.GetChecks()))
			for _, c := range health.resp.GetChecks() {
				checks = append(checks, map[string]interface{}{
					"subsystem": c.GetSubsystem(),
					"ok":        c.GetOk(),
					"reason":    c.GetReason(),
				})
			}

			lastSeenAgo := "never"
			if health.resp.GetLastSeen() != nil {
				lastSeenAgo = ago(health.resp.GetLastSeen().AsTime())
			}

			result["health"] = map[string]interface{}{
				"overall_status":       health.resp.GetOverallStatus(),
				"healthy":              health.resp.GetHealthy(),
				"can_apply_privileged": health.resp.GetCanApplyPrivileged(),
				"privilege_reason":     health.resp.GetPrivilegeReason(),
				"last_error":           health.resp.GetLastError(),
				"last_seen_ago":        lastSeenAgo,
				"checks":               checks,
			}
		}

		// Process plan
		if plan.err != nil {
			errors = append(errors, fmt.Sprintf("GetNodePlanV1: %v", plan.err))
		} else if plan.resp != nil {
			p := plan.resp.GetPlan()
			if p == nil {
				result["pending_plan"] = "none"
			} else {
				expired := false
				if p.GetExpiresUnixMs() > 0 {
					expiresAt := time.UnixMilli(int64(p.GetExpiresUnixMs()))
					expired = time.Now().After(expiresAt)
				}
				result["pending_plan"] = map[string]interface{}{
					"plan_id":      p.GetPlanId(),
					"generation":   p.GetGeneration(),
					"reason":       p.GetReason(),
					"desired_hash": p.GetDesiredHash(),
					"issued_by":    p.GetIssuedBy(),
					"created_at":   fmtTime(int64(p.GetCreatedUnixMs())),
					"expires_at":   fmtTime(int64(p.GetExpiresUnixMs())),
					"expired":      expired,
				}
			}
		}

		// Process plan status
		if pstat.err != nil {
			errors = append(errors, fmt.Sprintf("GetPlanStatusV1: %v", pstat.err))
		} else if pstat.resp != nil {
			st := pstat.resp.GetStatus()
			if st == nil {
				result["execution"] = "no plan executed"
			} else {
				completedSteps := 0
				totalSteps := len(st.GetSteps())
				for _, step := range st.GetSteps() {
					if step.GetState().String() == "STEP_SUCCEEDED" {
						completedSteps++
					}
				}

				result["execution"] = map[string]interface{}{
					"plan_id":         st.GetPlanId(),
					"state":           st.GetState().String(),
					"current_step":    st.GetCurrentStepId(),
					"completed_steps": completedSteps,
					"total_steps":     totalSteps,
					"error_message":   st.GetErrorMessage(),
					"error_step_id":   st.GetErrorStepId(),
					"started_at":      fmtTime(int64(st.GetStartedUnixMs())),
					"finished_at":     fmtTime(int64(st.GetFinishedUnixMs())),
				}
			}
		}

		// Compute diagnosis
		result["diagnosis"] = computeDiagnosis(result, health, plan, pstat)
		result["errors"] = errors

		return result, nil
	})
}

// convergenceForNode computes per-service convergence detail for a specific node.
func convergenceForNode(nodeID string, desiredMap map[string]string, healthNodes []*cluster_controllerpb.NodeHealth, installedPkgs []*node_agentpb.InstalledPackage, errors []string) map[string]interface{} {
	// Build installed map from packages
	installedMap := make(map[string]map[string]interface{})
	for _, p := range installedPkgs {
		installedMap[p.GetName()] = map[string]interface{}{
			"version":  p.GetVersion(),
			"kind":     p.GetKind(),
			"checksum": p.GetChecksum(),
			"status":   p.GetStatus(),
		}
	}

	// Find this node in health data for hash comparison
	var desiredHash, appliedHash string
	var healthInstalledVersions map[string]string
	for _, n := range healthNodes {
		if n.GetNodeId() == nodeID {
			desiredHash = n.GetDesiredServicesHash()
			appliedHash = n.GetAppliedServicesHash()
			healthInstalledVersions = n.GetInstalledVersions()
			break
		}
	}

	// Classify each service
	services := make([]map[string]interface{}, 0)
	seen := make(map[string]bool)

	convergedCount := 0
	driftedCount := 0
	missingCount := 0
	unmanagedCount := 0

	for sid, desiredVer := range desiredMap {
		seen[sid] = true
		entry := map[string]interface{}{
			"service_id":      sid,
			"desired_version": desiredVer,
		}

		inst, hasInstalled := installedMap[sid]
		if !hasInstalled {
			// Also check health-reported versions
			if healthVer, ok := healthInstalledVersions[sid]; ok {
				entry["installed_version"] = healthVer
				if healthVer == desiredVer {
					entry["status"] = "converged"
					convergedCount++
				} else {
					entry["status"] = "drifted"
					entry["drift"] = fmt.Sprintf("desired %s, installed %s", desiredVer, healthVer)
					driftedCount++
				}
			} else {
				entry["status"] = "missing"
				entry["installed_version"] = ""
				missingCount++
			}
		} else {
			instVer, _ := inst["version"].(string)
			entry["installed_version"] = instVer
			entry["kind"], _ = inst["kind"].(string)
			entry["checksum"], _ = inst["checksum"].(string)

			if instVer == desiredVer {
				entry["status"] = "converged"
				convergedCount++
			} else {
				entry["status"] = "drifted"
				entry["drift"] = fmt.Sprintf("desired %s, installed %s", desiredVer, instVer)
				driftedCount++
			}
		}
		services = append(services, entry)
	}

	// Unmanaged: installed but not desired
	for name, inst := range installedMap {
		if !seen[name] {
			instVer, _ := inst["version"].(string)
			kind, _ := inst["kind"].(string)
			services = append(services, map[string]interface{}{
				"service_id":        name,
				"desired_version":   "",
				"installed_version": instVer,
				"kind":              kind,
				"status":            "unmanaged",
			})
			unmanagedCount++
		}
	}

	return map[string]interface{}{
		"node_id":       nodeID,
		"desired_hash":  desiredHash,
		"applied_hash":  appliedHash,
		"hash_match":    desiredHash == appliedHash && desiredHash != "",
		"converged":     convergedCount,
		"drifted":       driftedCount,
		"missing":       missingCount,
		"unmanaged":     unmanagedCount,
		"total_desired": len(desiredMap),
		"services":      services,
		"errors":        errors,
	}
}

// computeDiagnosis generates a human-readable explanation of why convergence
// is or is not progressing for a node.
type rpcResult[T any] struct {
	resp T
	err  error
}

func computeDiagnosis(_ map[string]interface{}, health rpcResult[*cluster_controllerpb.GetNodeHealthDetailV1Response], plan rpcResult[*cluster_controllerpb.GetNodePlanV1Response], pstat rpcResult[*node_agentpb.GetPlanStatusV1Response]) string {
	// Check for errors first
	if health.err != nil && plan.err != nil {
		return "Cannot diagnose: both health and plan data unavailable. Check controller connectivity."
	}

	// Check privilege
	if health.resp != nil && !health.resp.GetCanApplyPrivileged() {
		reason := health.resp.GetPrivilegeReason()
		if reason == "" {
			reason = "euid is not 0, no systemd write access, no sudo access"
		}
		return fmt.Sprintf("BLOCKED: Node lacks privilege to apply plans. Reason: %s. The reconciler will skip this node until it has systemd access.", reason)
	}

	// Check health
	if health.resp != nil && health.resp.GetLastError() != "" {
		return fmt.Sprintf("UNHEALTHY: Last error reported: %s. The reconciler may delay plan dispatch until the node is healthy.", health.resp.GetLastError())
	}

	// Check plan existence
	if plan.resp != nil && plan.resp.GetPlan() == nil {
		if health.resp != nil && health.resp.GetHealthy() {
			return "CONVERGED: No pending plan and node is healthy. The reconciler considers this node up-to-date."
		}
		return "NO PLAN: No convergence plan is pending for this node. The reconciler may not have generated one yet, or the node may already be converged."
	}

	// Check plan expiry
	if plan.resp != nil && plan.resp.GetPlan() != nil {
		p := plan.resp.GetPlan()
		if p.GetExpiresUnixMs() > 0 {
			expiresAt := time.UnixMilli(int64(p.GetExpiresUnixMs()))
			if time.Now().After(expiresAt) {
				return fmt.Sprintf("EXPIRED: Plan %s expired at %s. The reconciler should issue a new plan on the next cycle.", p.GetPlanId(), fmtTime(int64(p.GetExpiresUnixMs())))
			}
		}
	}

	// Check execution status
	if pstat.resp != nil && pstat.resp.GetStatus() != nil {
		st := pstat.resp.GetStatus()
		state := st.GetState().String()

		switch {
		case state == "PLAN_RUNNING" || state == "PLAN_PENDING":
			step := st.GetCurrentStepId()
			if step == "" {
				step = "unknown"
			}
			return fmt.Sprintf("IN PROGRESS: Plan %s is %s, currently at step '%s'.", st.GetPlanId(), state, step)

		case state == "PLAN_FAILED":
			errMsg := st.GetErrorMessage()
			errStep := st.GetErrorStepId()
			if errStep != "" {
				return fmt.Sprintf("FAILED: Plan %s failed at step '%s': %s. The reconciler will retry on the next cycle.", st.GetPlanId(), errStep, errMsg)
			}
			return fmt.Sprintf("FAILED: Plan %s failed: %s. The reconciler will retry on the next cycle.", st.GetPlanId(), errMsg)

		case state == "PLAN_SUCCEEDED":
			return fmt.Sprintf("COMPLETED: Plan %s succeeded at %s. If hashes still mismatch, the health report may not have refreshed yet.", st.GetPlanId(), fmtTime(int64(st.GetFinishedUnixMs())))
		}
	}

	// Plan exists but no execution status
	if plan.resp != nil && plan.resp.GetPlan() != nil {
		return fmt.Sprintf("PENDING: Plan %s is pending but has not started executing. The node agent may not have picked it up yet.", plan.resp.GetPlan().GetPlanId())
	}

	return "UNKNOWN: Insufficient data to determine convergence state."
}
