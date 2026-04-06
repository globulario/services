package main

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	repositorypb "github.com/globulario/services/golang/repository/repositorypb"
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

		// Fetch repository catalog (4th layer) for full status vocabulary.
		repoArtifacts := make(map[string]bool) // lowercase name → exists
		if conn, err := s.clients.get(outerCtx, repositoryEndpoint()); err == nil {
			repoClient := repositorypb.NewPackageRepositoryClient(conn)
			repoCtx, repoCancel := context.WithTimeout(authCtx(outerCtx), 5*time.Second)
			defer repoCancel()
			if resp, err := repoClient.ListArtifacts(repoCtx, &repositorypb.ListArtifactsRequest{}); err == nil {
				for _, a := range resp.GetArtifacts() {
					if ref := a.GetRef(); ref != nil {
						repoArtifacts[strings.ToLower(strings.ReplaceAll(ref.GetName(), "_", "-"))] = true
					}
				}
			}
		}

		// If a specific node is requested, do per-service diff for that node
		if nodeID != "" {
			return convergenceForNode(nodeID, desiredMap, repoArtifacts, healthNodes, installedPkgs, errors), nil
		}

		// Cluster-wide: per-node convergence summary using frozen 7-status vocabulary.
		// See CLAUDE.md for vocabulary: Installed, Planned, Available, Drifted, Unmanaged, Missing in repo, Orphaned.
		nodeSummaries := make([]map[string]interface{}, 0, len(healthNodes))
		for _, n := range healthNodes {
			installed := 0 // desired == installed, converged
			planned := 0   // desired set, not yet installed
			drifted := 0   // installed version differs from desired
			unmanaged := 0 // installed without desired-state entry
			missingInRepo := 0 // desired/installed but artifact not in repository

			instVersions := n.GetInstalledVersions()
			seen := make(map[string]bool)

			for sid, desiredVer := range desiredMap {
				installedVer, ok := instVersions[sid]
				if !ok {
					// Desired but not installed
					planned++
				} else if installedVer != desiredVer {
					drifted++
				} else {
					// Check repository layer
					if len(repoArtifacts) > 0 && !repoArtifacts[strings.ToLower(strings.ReplaceAll(sid, "_", "-"))] {
						missingInRepo++
					} else {
						installed++
					}
				}
				seen[sid] = true
			}

			for sid := range instVersions {
				if !seen[sid] {
					unmanaged++
				}
			}

			hashMatch := n.GetDesiredServicesHash() == n.GetAppliedServicesHash()
			nodeSummaries = append(nodeSummaries, map[string]interface{}{
				"node_id":         n.GetNodeId(),
				"hash_match":      hashMatch,
				"installed":       installed,
				"planned":         planned,
				"drifted":         drifted,
				"unmanaged":       unmanaged,
				"missing_in_repo": missingInRepo,
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

		// Plan goroutines removed — plan system deleted.

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

		// Plan processing removed — plan system deleted.
		result["pending_plan"] = "plan system removed"
		result["execution"] = "plan system removed"
		result["diagnosis"] = computeDiagnosis(result, health)
		result["errors"] = errors

		return result, nil
	})
}

// convergenceForNode computes per-service convergence detail for a specific node.
// Uses the frozen 7-status vocabulary from CLAUDE.md.
func convergenceForNode(nodeID string, desiredMap map[string]string, repoArtifacts map[string]bool, healthNodes []*cluster_controllerpb.NodeHealth, installedPkgs []*node_agentpb.InstalledPackage, errors []string) map[string]interface{} {
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

	// Frozen 7-status vocabulary counters.
	installedCount := 0  // desired == installed, converged
	plannedCount := 0    // desired set, not yet installed
	driftedCount := 0    // installed version differs from desired
	unmanagedCount := 0  // installed without desired-state entry
	missingInRepoCount := 0 // desired/installed but not in repository

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
					entry["status"] = "installed"
					installedCount++
				} else {
					entry["status"] = "drifted"
					entry["drift"] = fmt.Sprintf("desired %s, installed %s", desiredVer, healthVer)
					driftedCount++
				}
			} else {
				entry["status"] = "planned"
				entry["installed_version"] = ""
				plannedCount++
			}
		} else {
			instVer, _ := inst["version"].(string)
			entry["installed_version"] = instVer
			entry["kind"], _ = inst["kind"].(string)
			entry["checksum"], _ = inst["checksum"].(string)

			if instVer == desiredVer {
				// Check repository layer
				if len(repoArtifacts) > 0 && !repoArtifacts[strings.ToLower(strings.ReplaceAll(sid, "_", "-"))] {
					entry["status"] = "missing_in_repo"
					missingInRepoCount++
				} else {
					entry["status"] = "installed"
					installedCount++
				}
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
		"installed":       installedCount,
		"planned":         plannedCount,
		"drifted":         driftedCount,
		"unmanaged":       unmanagedCount,
		"missing_in_repo": missingInRepoCount,
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

func computeDiagnosis(_ map[string]interface{}, health rpcResult[*cluster_controllerpb.GetNodeHealthDetailV1Response]) string {
	if health.err != nil {
		return "Cannot diagnose: health data unavailable. Check controller connectivity."
	}
	if health.resp != nil && !health.resp.GetCanApplyPrivileged() {
		reason := health.resp.GetPrivilegeReason()
		if reason == "" {
			reason = "euid is not 0, no systemd write access"
		}
		return fmt.Sprintf("BLOCKED: Node lacks privilege. Reason: %s.", reason)
	}
	if health.resp != nil && health.resp.GetLastError() != "" {
		return fmt.Sprintf("UNHEALTHY: %s", health.resp.GetLastError())
	}
	if health.resp != nil && health.resp.GetHealthy() {
		return "CONVERGED: Node is healthy. Workflow-native release pipeline handles convergence."
	}
	return "UNKNOWN: Insufficient data."
}
