package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"github.com/globulario/services/golang/workflow/engine"
	"github.com/globulario/services/golang/workflow/v1alpha1"
)

// RunPackageReleaseWorkflow executes the release.apply.package workflow to
// roll out any package (SERVICE, INFRASTRUCTURE, WORKLOAD, COMMAND) across
// candidate nodes. The controller orchestrates; per-node steps call
// node-agents via gRPC.
func (srv *server) RunPackageReleaseWorkflow(ctx context.Context, releaseID, releaseName, pkgName, pkgKind, version, desiredHash string, candidateNodes []string) (*engine.Run, error) {
	defPath := resolveWorkflowDefinition("release.apply.package")
	if defPath == "" {
		return nil, fmt.Errorf("release.apply.package.yaml not found")
	}

	loader := v1alpha1.NewLoader()
	def, err := loader.LoadFile(defPath)
	if err != nil {
		return nil, fmt.Errorf("load workflow definition %s: %w", defPath, err)
	}

	router := engine.NewRouter()

	// Wire release controller actions with real implementations.
	engine.RegisterReleaseControllerActions(router, srv.buildReleaseControllerConfig())

	// Wire node-agent actions — each callback resolves the node's agent
	// endpoint from the workflow's per-item inputs and calls via gRPC.
	engine.RegisterNodeDirectApplyActions(router, srv.buildNodeDirectApplyConfig())

	eng := &engine.Engine{
		Router: router,
		OnStepDone: func(run *engine.Run, step *engine.StepState) {
			elapsed := time.Duration(0)
			if !step.StartedAt.IsZero() && !step.FinishedAt.IsZero() {
				elapsed = step.FinishedAt.Sub(step.StartedAt)
			}
			log.Printf("release-workflow: step %s → %s (%s)",
				step.ID, step.Status, elapsed.Round(time.Millisecond))
		},
	}

	nodesAny := make([]any, len(candidateNodes))
	for i, n := range candidateNodes {
		nodesAny[i] = n
	}

	inputs := map[string]any{
		"cluster_id":       srv.cfg.ClusterDomain,
		"release_id":       releaseID,
		"release_name":     releaseName,
		"package_name":     pkgName,
		"package_kind":     pkgKind,
		"resolved_version": version,
		"desired_hash":     desiredHash,
		"candidate_nodes":  nodesAny,
	}

	log.Printf("release-workflow: starting %s for release %s (%s:%s@%s) across %d nodes",
		def.Metadata.Name, releaseName, pkgKind, pkgName, version, len(candidateNodes))

	start := time.Now()
	run, err := eng.Execute(ctx, def, inputs)
	elapsed := time.Since(start)

	if err != nil {
		log.Printf("release-workflow: %s FAILED after %s: %v",
			releaseName, elapsed.Round(time.Millisecond), err)
	} else {
		succeeded := 0
		for _, st := range run.Steps {
			if st.Status == engine.StepSucceeded {
				succeeded++
			}
		}
		log.Printf("release-workflow: %s SUCCEEDED in %s (%d/%d steps)",
			releaseName, elapsed.Round(time.Millisecond), succeeded, len(run.Steps))
	}

	return run, err
}

// RunInfraReleaseWorkflow executes the infrastructure-specific release workflow.
// Delegates to RunPackageReleaseWorkflow with kind=INFRASTRUCTURE.
func (srv *server) RunInfraReleaseWorkflow(ctx context.Context, releaseID, releaseName, pkgName, version, desiredHash string, candidateNodes []string) (*engine.Run, error) {
	return srv.RunPackageReleaseWorkflow(ctx, releaseID, releaseName, pkgName, "INFRASTRUCTURE", version, desiredHash, candidateNodes)
}

// RunRemovePackageWorkflow executes the release.remove.package workflow
// to uninstall a package from all target nodes.
func (srv *server) RunRemovePackageWorkflow(ctx context.Context, releaseID, pkgName, pkgKind string, candidateNodes []string) (*engine.Run, error) {
	defPath := resolveWorkflowDefinition("release.remove.package")
	if defPath == "" {
		return nil, fmt.Errorf("release.remove.package.yaml not found")
	}

	loader := v1alpha1.NewLoader()
	def, err := loader.LoadFile(defPath)
	if err != nil {
		return nil, fmt.Errorf("load workflow definition %s: %w", defPath, err)
	}

	router := engine.NewRouter()
	engine.RegisterReleaseControllerActions(router, srv.buildReleaseControllerConfig())
	engine.RegisterNodeDirectApplyActions(router, srv.buildNodeDirectApplyConfig())

	eng := &engine.Engine{
		Router: router,
		OnStepDone: func(run *engine.Run, step *engine.StepState) {
			elapsed := time.Duration(0)
			if !step.StartedAt.IsZero() && !step.FinishedAt.IsZero() {
				elapsed = step.FinishedAt.Sub(step.StartedAt)
			}
			log.Printf("remove-workflow: step %s → %s (%s)",
				step.ID, step.Status, elapsed.Round(time.Millisecond))
		},
	}

	nodesAny := make([]any, len(candidateNodes))
	for i, n := range candidateNodes {
		nodesAny[i] = n
	}

	inputs := map[string]any{
		"cluster_id":      srv.cfg.ClusterDomain,
		"release_id":      releaseID,
		"package_name":    pkgName,
		"package_kind":    pkgKind,
		"candidate_nodes": nodesAny,
	}

	log.Printf("remove-workflow: starting removal of %s (%s) across %d nodes",
		pkgName, pkgKind, len(candidateNodes))

	start := time.Now()
	run, err := eng.Execute(ctx, def, inputs)
	elapsed := time.Since(start)

	if err != nil {
		log.Printf("remove-workflow: %s FAILED after %s: %v",
			pkgName, elapsed.Round(time.Millisecond), err)
	} else {
		log.Printf("remove-workflow: %s SUCCEEDED in %s",
			pkgName, elapsed.Round(time.Millisecond))
	}

	return run, err
}

// --------------------------------------------------------------------------
// Controller action config (runs locally on controller)
// --------------------------------------------------------------------------

func (srv *server) buildReleaseControllerConfig() engine.ReleaseControllerConfig {
	return engine.ReleaseControllerConfig{
		MarkReleaseResolved: func(ctx context.Context, relID string) error {
			log.Printf("release-workflow: mark %s RESOLVED", relID)
			return nil
		},
		MarkReleaseApplying: func(ctx context.Context, relID string) error {
			log.Printf("release-workflow: mark %s APPLYING", relID)
			return nil
		},
		MarkReleaseFailed: func(ctx context.Context, relID, reason string) error {
			log.Printf("release-workflow: mark %s FAILED: %s", relID, reason)
			return nil
		},
		RecheckConvergence: func(ctx context.Context, relID string) error {
			log.Printf("release-workflow: recheck convergence for %s", relID)
			if srv.enqueueReconcile != nil {
				srv.enqueueReconcile()
			}
			return nil
		},
		SelectInfraTargets: func(ctx context.Context, candidates []any, pkgName, desiredHash string) ([]any, error) {
			return srv.selectReleaseTargets(ctx, candidates, pkgName, "", desiredHash)
		},
		SelectPackageTargets: func(ctx context.Context, candidates []any, pkgName, pkgKind, desiredHash string) ([]any, error) {
			return srv.selectReleaseTargets(ctx, candidates, pkgName, pkgKind, desiredHash)
		},
		FinalizeNoop: func(ctx context.Context, releaseID string) error {
			log.Printf("release-workflow: %s finalized AVAILABLE (no-op)", releaseID)
			return nil
		},
		MarkNodeStarted: func(ctx context.Context, releaseID, nodeID string) error {
			log.Printf("release-workflow: node %s started for %s", nodeID, releaseID)
			return nil
		},
		MarkNodeSucceeded: func(ctx context.Context, releaseID, nodeID, version, hash string) error {
			log.Printf("release-workflow: node %s succeeded for %s (v=%s h=%s)", nodeID, releaseID, version, hash)
			return nil
		},
		MarkNodeFailed: func(ctx context.Context, releaseID, nodeID, reason string) error {
			log.Printf("release-workflow: node %s FAILED for %s: %s", nodeID, releaseID, reason)
			return nil
		},
		AggregateDirectApply: func(ctx context.Context, releaseID, pkgName string) (map[string]any, error) {
			return map[string]any{"release_id": releaseID, "package_name": pkgName, "status": "ok"}, nil
		},
		FinalizeDirectApply: func(ctx context.Context, releaseID string, aggregate map[string]any) error {
			log.Printf("release-workflow: finalize %s (aggregate=%v)", releaseID, aggregate)
			return nil
		},
	}
}

// selectReleaseTargets filters candidate nodes: only include nodes that are
// bootstrap-ready and have the package's required profiles.
func (srv *server) selectReleaseTargets(ctx context.Context, candidates []any, pkgName, pkgKind, desiredHash string) ([]any, error) {
	srv.lock("selectReleaseTargets")
	defer srv.unlock()

	isInfra := strings.EqualFold(pkgKind, "INFRASTRUCTURE")
	catalogEntry := CatalogByName(pkgName)

	var targets []any
	for _, c := range candidates {
		nodeID := fmt.Sprint(c)
		node := srv.state.Nodes[nodeID]
		if node == nil {
			continue
		}

		// Workload/service releases skip nodes not yet bootstrap-ready.
		// Infrastructure releases target all nodes (they're what gets nodes ready).
		if !isInfra && !bootstrapPhaseReady(node.BootstrapPhase) {
			log.Printf("release-workflow: skip node %s (bootstrap_phase=%s)", nodeID, node.BootstrapPhase)
			continue
		}

		// Profile filter.
		if catalogEntry != nil && len(catalogEntry.Profiles) > 0 {
			expanded := normalizeProfiles(node.Profiles)
			if !profilesOverlap(catalogEntry.Profiles, expanded) {
				log.Printf("release-workflow: skip node %s (profiles %v don't match %v)", nodeID, expanded, catalogEntry.Profiles)
				continue
			}
		}

		// TODO: compare installed hash vs desiredHash to skip converged nodes.

		targets = append(targets, map[string]any{
			"node_id":        nodeID,
			"agent_endpoint": node.AgentEndpoint,
		})
	}
	return targets, nil
}

// --------------------------------------------------------------------------
// Node-agent action config (calls node-agent via gRPC)
// --------------------------------------------------------------------------

func (srv *server) buildNodeDirectApplyConfig() engine.NodeDirectApplyConfig {
	return engine.NodeDirectApplyConfig{
		InstallPackage: func(ctx context.Context, name, version, kind string) error {
			nc, _ := engine.GetNodeContext(ctx)
			nodeID, endpoint := nc.NodeID, nc.AgentEndpoint
			if endpoint == "" {
				return fmt.Errorf("no agent endpoint for node %s", nodeID)
			}

			log.Printf("release-workflow: installing %s@%s (%s) on node %s via %s", name, version, kind, nodeID, endpoint)
			conn, err := srv.dialNodeAgent(endpoint)
			if err != nil {
				return fmt.Errorf("connect to node %s: %w", nodeID, err)
			}
			defer conn.Close()

			client := node_agentpb.NewNodeAgentServiceClient(conn)
			resp, err := client.RunWorkflow(ctx, &node_agentpb.RunWorkflowRequest{
				WorkflowName: "install-package",
				Inputs: map[string]string{
					"package_name": name,
					"version":      version,
					"kind":         kind,
				},
			})
			if err != nil {
				return fmt.Errorf("install %s on node %s: %w", name, nodeID, err)
			}
			if resp.GetStatus() != "SUCCEEDED" {
				return fmt.Errorf("install %s on node %s: %s", name, nodeID, resp.GetError())
			}
			return nil
		},

		VerifyPackageInstalled: func(ctx context.Context, name, version, hash string) error {
			nc, _ := engine.GetNodeContext(ctx)
			nodeID, endpoint := nc.NodeID, nc.AgentEndpoint
			if endpoint == "" {
				return fmt.Errorf("no agent endpoint for node %s", nodeID)
			}

			conn, err := srv.dialNodeAgent(endpoint)
			if err != nil {
				return fmt.Errorf("connect to node %s: %w", nodeID, err)
			}
			defer conn.Close()

			client := node_agentpb.NewNodeAgentServiceClient(conn)
			resp, err := client.GetInstalledPackage(ctx, &node_agentpb.GetInstalledPackageRequest{
				NodeId: nodeID,
				Name:   name,
			})
			if err != nil {
				return fmt.Errorf("verify %s on node %s: %w", name, nodeID, err)
			}
			pkg := resp.GetPackage()
			if pkg == nil {
				return fmt.Errorf("verify %s on node %s: package not found", name, nodeID)
			}
			if pkg.GetVersion() != version {
				return fmt.Errorf("verify %s on node %s: installed=%s want=%s", name, nodeID, pkg.GetVersion(), version)
			}
			return nil
		},

		RestartPackageService: func(ctx context.Context, name string) error {
			nc, _ := engine.GetNodeContext(ctx)
			nodeID, endpoint := nc.NodeID, nc.AgentEndpoint
			if endpoint == "" {
				return fmt.Errorf("no agent endpoint for node %s", nodeID)
			}

			conn, err := srv.dialNodeAgent(endpoint)
			if err != nil {
				return fmt.Errorf("connect to node %s: %w", nodeID, err)
			}
			defer conn.Close()

			unit := "globular-" + name + ".service"
			client := node_agentpb.NewNodeAgentServiceClient(conn)
			_, err = client.ControlService(ctx, &node_agentpb.ControlServiceRequest{
				Unit:   unit,
				Action: "restart",
			})
			if err != nil {
				return fmt.Errorf("restart %s on node %s: %w", unit, nodeID, err)
			}
			return nil
		},

		MaybeRestartPackage: func(ctx context.Context, name, kind, restartPolicy string) error {
			// COMMAND packages never need restart.
			if strings.EqualFold(kind, "COMMAND") {
				return nil
			}
			if strings.EqualFold(restartPolicy, "never") {
				return nil
			}

			nc, _ := engine.GetNodeContext(ctx)
			nodeID, endpoint := nc.NodeID, nc.AgentEndpoint
			if endpoint == "" {
				return fmt.Errorf("no agent endpoint for node %s", nodeID)
			}

			conn, err := srv.dialNodeAgent(endpoint)
			if err != nil {
				return fmt.Errorf("connect to node %s: %w", nodeID, err)
			}
			defer conn.Close()

			unit := "globular-" + name + ".service"
			client := node_agentpb.NewNodeAgentServiceClient(conn)
			_, err = client.ControlService(ctx, &node_agentpb.ControlServiceRequest{
				Unit:   unit,
				Action: "restart",
			})
			if err != nil {
				return fmt.Errorf("restart %s on node %s: %w", unit, nodeID, err)
			}
			return nil
		},

		VerifyPackageRuntime: func(ctx context.Context, name, healthCheck string) error {
			nc, _ := engine.GetNodeContext(ctx)
			nodeID, endpoint := nc.NodeID, nc.AgentEndpoint
			if endpoint == "" {
				return fmt.Errorf("no agent endpoint for node %s", nodeID)
			}

			conn, err := srv.dialNodeAgent(endpoint)
			if err != nil {
				return fmt.Errorf("connect to node %s: %w", nodeID, err)
			}
			defer conn.Close()

			unit := "globular-" + name + ".service"
			client := node_agentpb.NewNodeAgentServiceClient(conn)
			resp, err := client.ControlService(ctx, &node_agentpb.ControlServiceRequest{
				Unit:   unit,
				Action: "status",
			})
			if err != nil {
				return fmt.Errorf("health check %s on node %s: %w", name, nodeID, err)
			}
			if resp.GetState() != "active" {
				return fmt.Errorf("health check %s on node %s: status=%s (want active)", name, nodeID, resp.GetState())
			}
			return nil
		},

		SyncInstalledPackage: func(ctx context.Context, name, version, hash string) error {
			nc, _ := engine.GetNodeContext(ctx)
			nodeID, endpoint := nc.NodeID, nc.AgentEndpoint
			if endpoint == "" {
				return fmt.Errorf("no agent endpoint for node %s", nodeID)
			}

			conn, err := srv.dialNodeAgent(endpoint)
			if err != nil {
				return fmt.Errorf("connect to node %s: %w", nodeID, err)
			}
			defer conn.Close()

			client := node_agentpb.NewNodeAgentServiceClient(conn)
			_, err = client.SetInstalledPackage(ctx, &node_agentpb.SetInstalledPackageRequest{
				Package: &node_agentpb.InstalledPackage{
					NodeId:   nodeID,
					Name:     name,
					Version:  version,
					Checksum: hash,
				},
			})
			if err != nil {
				return fmt.Errorf("sync installed state %s on node %s: %w", name, nodeID, err)
			}
			return nil
		},

		// Removal actions
		StopPackageService: func(ctx context.Context, name string) error {
			nc, _ := engine.GetNodeContext(ctx)
			nodeID, endpoint := nc.NodeID, nc.AgentEndpoint
			if endpoint == "" {
				return fmt.Errorf("no agent endpoint for node %s", nodeID)
			}
			conn, err := srv.dialNodeAgent(endpoint)
			if err != nil {
				return fmt.Errorf("connect to node %s: %w", nodeID, err)
			}
			defer conn.Close()
			unit := "globular-" + name + ".service"
			client := node_agentpb.NewNodeAgentServiceClient(conn)
			_, err = client.ControlService(ctx, &node_agentpb.ControlServiceRequest{
				Unit:   unit,
				Action: "stop",
			})
			if err != nil {
				return fmt.Errorf("stop %s on node %s: %w", unit, nodeID, err)
			}
			return nil
		},

		DisablePackageService: func(ctx context.Context, name string) error {
			nc, _ := engine.GetNodeContext(ctx)
			nodeID, endpoint := nc.NodeID, nc.AgentEndpoint
			if endpoint == "" {
				return fmt.Errorf("no agent endpoint for node %s", nodeID)
			}
			conn, err := srv.dialNodeAgent(endpoint)
			if err != nil {
				return fmt.Errorf("connect to node %s: %w", nodeID, err)
			}
			defer conn.Close()
			unit := "globular-" + name + ".service"
			client := node_agentpb.NewNodeAgentServiceClient(conn)
			_, err = client.ControlService(ctx, &node_agentpb.ControlServiceRequest{
				Unit:   unit,
				Action: "disable",
			})
			if err != nil {
				return fmt.Errorf("disable %s on node %s: %w", unit, nodeID, err)
			}
			return nil
		},

		UninstallPackage: func(ctx context.Context, name, kind string) error {
			nc, _ := engine.GetNodeContext(ctx)
			nodeID, endpoint := nc.NodeID, nc.AgentEndpoint
			if endpoint == "" {
				return fmt.Errorf("no agent endpoint for node %s", nodeID)
			}
			conn, err := srv.dialNodeAgent(endpoint)
			if err != nil {
				return fmt.Errorf("connect to node %s: %w", nodeID, err)
			}
			defer conn.Close()
			// Use RunWorkflow to invoke the uninstall action on the node-agent.
			client := node_agentpb.NewNodeAgentServiceClient(conn)
			resp, err := client.RunWorkflow(ctx, &node_agentpb.RunWorkflowRequest{
				WorkflowName: "uninstall-package",
				Inputs: map[string]string{
					"package_name": name,
					"kind":         kind,
				},
			})
			if err != nil {
				return fmt.Errorf("uninstall %s on node %s: %w", name, nodeID, err)
			}
			if resp.GetStatus() != "SUCCEEDED" {
				return fmt.Errorf("uninstall %s on node %s: %s", name, nodeID, resp.GetError())
			}
			return nil
		},

		ClearInstalledPackageState: func(ctx context.Context, name, kind string) error {
			nc, _ := engine.GetNodeContext(ctx)
			nodeID, endpoint := nc.NodeID, nc.AgentEndpoint
			if endpoint == "" {
				return fmt.Errorf("no agent endpoint for node %s", nodeID)
			}
			conn, err := srv.dialNodeAgent(endpoint)
			if err != nil {
				return fmt.Errorf("connect to node %s: %w", nodeID, err)
			}
			defer conn.Close()
			// Clear the installed-state entry by setting an empty package.
			client := node_agentpb.NewNodeAgentServiceClient(conn)
			_, err = client.SetInstalledPackage(ctx, &node_agentpb.SetInstalledPackageRequest{
				Package: &node_agentpb.InstalledPackage{
					NodeId:  nodeID,
					Name:    name,
					Kind:    kind,
					Status:  "removed",
					Version: "",
				},
			})
			if err != nil {
				return fmt.Errorf("clear state %s on node %s: %w", name, nodeID, err)
			}
			return nil
		},
	}
}

// --------------------------------------------------------------------------
// Workflow definition resolver
// --------------------------------------------------------------------------

// resolveWorkflowDefinition finds a workflow YAML by name.
func resolveWorkflowDefinition(name string) string {
	candidates := []string{
		"/var/lib/globular/workflows/" + name + ".yaml",
		"/usr/lib/globular/workflows/" + name + ".yaml",
		"/tmp/" + name + ".yaml",
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

