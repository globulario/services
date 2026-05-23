package main

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/installed_state"
	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/actions"
	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/supervisor"
	"github.com/globulario/services/golang/node_agent/node_agentpb"
	repositorypb "github.com/globulario/services/golang/repository/repositorypb"
	"github.com/globulario/services/golang/workflow/engine"
	"github.com/globulario/services/golang/workflow/v1alpha1"
)

// RunWorkflowDefinition executes a workflow definition (e.g. node.join) to
// install all packages on this node. This replaces the one-plan-per-cycle
// reconciler with a single workflow run that installs packages in
// tiered parallel batches.
func (srv *NodeAgentServer) RunWorkflowDefinition(ctx context.Context, defPath string, inputs map[string]any) (*engine.Run, error) {
	loader := v1alpha1.NewLoader()
	def, err := loader.LoadFile(defPath)
	if err != nil {
		return nil, fmt.Errorf("load workflow definition %s: %w", defPath, err)
	}

	router := engine.NewRouter()

	// Wire node-agent actions.
	repoAddr := ""
	if srv.state != nil && srv.state.ControllerEndpoint != "" {
		repoAddr = srv.discoverRepositoryAddr()
	}

	engine.RegisterNodeAgentActions(router, engine.NodeAgentConfig{
		NodeID: srv.nodeID,
		FetchAndInstall: func(ctx context.Context, pkg engine.PackageRef) error {
			// Fast path: skip if already installed and the unit is active.
			// The local join workflow has no version context, so any installed
			// version is acceptable.
			existing, _ := installed_state.GetInstalledPackage(ctx, srv.nodeID, pkg.Kind, pkg.Name)
			if skipIfAlreadyInstalled(ctx, pkg.Name, existing, supervisor.IsActive) {
				return nil
			}
			// If installed but inactive, try to start the unit before falling back
			// to a full reinstall. A stopped unit (e.g. envoy after xds_ready reset)
			// often just needs a restart, not a fresh download.
			if existing != nil && strings.EqualFold(existing.GetStatus(), "installed") {
				unit := packageUnit(pkg.Name)
				if unit != "" {
					if startErr := supervisor.Start(ctx, unit); startErr == nil {
						if waitErr := supervisor.WaitActive(ctx, unit, 30*time.Second); waitErr == nil {
							log.Printf("workflow-runner: %s started (was inactive), skipping reinstall", pkg.Name)
							return nil
						}
					}
					log.Printf("workflow-runner: %s start failed — proceeding with reinstall", pkg.Name)
				}
			}
			// Re-install using identity from the installed-state record when
			// available. This allows artifact.fetch to reuse the staged cache
			// file via checksum match rather than failing with "refuse blind
			// reuse" (the 0.0.0-dev sentinel).
			version, buildID, checksum := "", "", ""
			if existing != nil {
				version = existing.GetVersion()
				buildID = existing.GetBuildId()
				checksum = existing.GetChecksum()
			} else if repoAddr != "" {
				// First-install: no etcd record yet. Resolve the latest published
				// manifest so InstallPackage gets a real version instead of the
				// 0.0.0-dev sentinel that causes artifact.fetch to fail.
				if v, b, c, err := resolveLatestManifestFunc(ctx, pkg.Name, pkg.Kind, repoAddr); err == nil {
					version, buildID, checksum = v, b, c
				} else {
					log.Printf("workflow-runner: resolve latest manifest for %s: %v (falling back to empty version)", pkg.Name, err)
				}
			}
			return srv.InstallPackage(ctx, pkg.Name, pkg.Kind, repoAddr, version, buildID, checksum)
		},
		IsServiceActive: func(name string) bool {
			return engine.DefaultIsServiceActive(name)
		},
		SyncInstalledState: func(ctx context.Context) error {
			srv.syncInstalledStateToEtcd(ctx)
			return nil
		},
		ProbeInfraHealth: func(ctx context.Context, probeName string) bool {
			resp, err := srv.RunWorkflow(ctx, &node_agentpb.RunWorkflowRequest{
				WorkflowName: probeName,
			})
			return err == nil && resp.GetStatus() == "SUCCEEDED"
		},
	})
	// Wire read-only verification handlers used by node.join verification blocks
	// and resume-policy verify_effect checks.
	engine.RegisterNodeVerificationActions(router, engine.NodeVerificationConfig{
		VerifyPackagesInstalled: func(ctx context.Context, nodeID string, packages []any) (bool, error) {
			targetNodeID := strings.TrimSpace(nodeID)
			if targetNodeID == "" {
				targetNodeID = strings.TrimSpace(srv.nodeID)
			}
			if targetNodeID == "" {
				return false, fmt.Errorf("node_id is required")
			}
			for _, raw := range packages {
				pkgMap, ok := raw.(map[string]any)
				if !ok {
					return false, fmt.Errorf("invalid package descriptor type %T", raw)
				}
				name := strings.TrimSpace(fmt.Sprint(pkgMap["name"]))
				if name == "" {
					return false, fmt.Errorf("package.name is required")
				}
				kind := strings.ToUpper(strings.TrimSpace(fmt.Sprint(pkgMap["kind"])))
				kinds := []string{kind}
				if kind == "" || kind == "<nil>" {
					// Be permissive for workflows that omit kind in verify blocks.
					kinds = []string{"SERVICE", "INFRASTRUCTURE", "COMMAND", "APPLICATION"}
				}
				foundInstalled := false
				for _, k := range kinds {
					pkg, err := installed_state.GetInstalledPackage(ctx, targetNodeID, k, name)
					if err != nil || pkg == nil {
						continue
					}
					if strings.EqualFold(pkg.GetStatus(), "installed") {
						foundInstalled = true
						break
					}
				}
				if !foundInstalled {
					return false, nil
				}
			}
			return true, nil
		},
		VerifyInstalledStateSynced: func(ctx context.Context, nodeID string) (bool, error) {
			// Best-effort sync: if sync succeeds, effect is present.
			srv.syncInstalledStateToEtcd(ctx)
			return true, nil
		},
	})

	// Wire controller actions (called remotely via the controller client).
	engine.RegisterControllerActions(router, engine.ControllerConfig{
		SetBootstrapPhase: func(ctx context.Context, nodeID, phase string) error {
			if err := srv.ensureControllerClient(ctx); err != nil {
				return fmt.Errorf("controller client: %w", err)
			}
			rpcCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			_, err := srv.controllerClient.SetNodeBootstrapPhase(rpcCtx, &cluster_controllerpb.SetNodeBootstrapPhaseRequest{
				NodeId: nodeID,
				Phase:  phase,
			})
			if err != nil {
				return fmt.Errorf("set bootstrap phase %s for %s: %w", phase, nodeID, err)
			}
			log.Printf("workflow-runner: set phase %s for %s", phase, nodeID)
			return nil
		},
		EmitEvent: func(ctx context.Context, eventType string, data map[string]any) error {
			if err := srv.ensureControllerClient(ctx); err != nil {
				return fmt.Errorf("controller client: %w", err)
			}
			// Convert map[string]any → map[string]string (best-effort).
			strData := make(map[string]string, len(data))
			for k, v := range data {
				strData[k] = fmt.Sprint(v)
			}
			rpcCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			_, err := srv.controllerClient.EmitWorkflowEvent(rpcCtx, &cluster_controllerpb.EmitWorkflowEventRequest{
				EventType: eventType,
				Data:      strData,
			})
			if err != nil {
				return fmt.Errorf("emit event %s: %w", eventType, err)
			}
			return nil
		},
	})
	engine.RegisterControllerVerificationActions(router, engine.ControllerVerificationConfig{
		VerifyBootstrapPhase: func(ctx context.Context, nodeID, expectedPhase string) (bool, error) {
			if err := srv.ensureControllerClient(ctx); err != nil {
				return false, fmt.Errorf("controller client: %w", err)
			}
			rpcCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
			defer cancel()
			resp, err := srv.controllerClient.ListNodes(rpcCtx, &cluster_controllerpb.ListNodesRequest{})
			if err != nil {
				return false, fmt.Errorf("list nodes: %w", err)
			}
			expected := strings.TrimSpace(expectedPhase)
			for _, n := range resp.GetNodes() {
				if strings.TrimSpace(n.GetNodeId()) != strings.TrimSpace(nodeID) {
					continue
				}
				actual := strings.TrimSpace(n.GetMetadata()["bootstrap_phase"])
				return actual == expected, nil
			}
			return false, fmt.Errorf("node %s not found", nodeID)
		},
	})

	// Condition evaluator for contains() expressions.
	evalCond := func(ctx context.Context, expr string, inputs, outputs map[string]any) (bool, error) {
		if strings.HasPrefix(expr, "contains(inputs.node_profiles,") {
			profiles, ok := inputs["node_profiles"].([]any)
			if !ok {
				return false, nil
			}
			parts := strings.SplitN(expr, "'", 3)
			if len(parts) < 2 {
				return false, nil
			}
			target := parts[1]
			for _, p := range profiles {
				if fmt.Sprint(p) == target {
					return true, nil
				}
			}
			return false, nil
		}
		return true, nil
	}

	eng := &engine.Engine{
		Router:   router,
		EvalCond: evalCond,
		OnStepDone: func(run *engine.Run, step *engine.StepState) {
			elapsed := time.Duration(0)
			if !step.StartedAt.IsZero() && !step.FinishedAt.IsZero() {
				elapsed = step.FinishedAt.Sub(step.StartedAt)
			}
			log.Printf("workflow-runner: step %s → %s (%s)",
				step.ID, step.Status, elapsed.Round(time.Millisecond))
		},
	}

	log.Printf("workflow-runner: starting %s for node %s", def.Metadata.Name, srv.nodeID)

	start := time.Now()
	run, err := eng.Execute(ctx, def, inputs)
	elapsed := time.Since(start)

	if err != nil {
		log.Printf("workflow-runner: %s FAILED after %s: %v",
			def.Metadata.Name, elapsed.Round(time.Millisecond), err)
	} else {
		succeeded := 0
		for _, st := range run.Steps {
			if st.Status == engine.StepSucceeded {
				succeeded++
			}
		}
		log.Printf("workflow-runner: %s SUCCEEDED in %s (%d/%d steps)",
			def.Metadata.Name, elapsed.Round(time.Millisecond), succeeded, len(run.Steps))
	}

	return run, err
}

// resolveLatestManifestFunc is the implementation used by FetchAndInstall for
// first-time installs. Overridable in tests via the variable below.
var resolveLatestManifestFunc = resolveLatestManifest

// resolveLatestManifest queries the repository for the latest published manifest
// of pkgName/pkgKind and returns (version, buildID, checksum). Called by the
// workflow-runner FetchAndInstall closure for first-time installs where no
// installed-state record exists, to avoid the 0.0.0-dev sentinel that causes
// artifact.fetch to fail.
func resolveLatestManifest(ctx context.Context, pkgName, pkgKind, repoAddr string) (version, buildID, checksum string, err error) {
	conn, _, err := actions.DialRepository(ctx, repoAddr)
	if err != nil {
		return "", "", "", fmt.Errorf("dial repository: %w", err)
	}
	defer conn.Close()

	platform := runtime.GOOS + "_" + runtime.GOARCH
	kind := mapKindStringToProto(pkgKind)
	repo := repositorypb.NewPackageRepositoryClient(conn)
	resp, err := repo.GetArtifactManifest(withAgentAuth(ctx), &repositorypb.GetArtifactManifestRequest{
		Ref: &repositorypb.ArtifactRef{
			PublisherId: defaultPublisherID,
			Name:        pkgName,
			Platform:    platform,
			Kind:        kind,
			// Version left empty → repository returns the latest published version.
		},
	})
	if err != nil {
		return "", "", "", fmt.Errorf("GetArtifactManifest %s: %w", pkgName, err)
	}
	m := resp.GetManifest()
	if m == nil {
		return "", "", "", fmt.Errorf("GetArtifactManifest %s: empty manifest", pkgName)
	}
	return m.GetRef().GetVersion(), m.GetBuildId(), m.GetChecksum(), nil
}

// skipIfAlreadyInstalled returns true when existing is non-nil with status
// "installed" AND the package's systemd unit is active (or the package is a
// command with no unit). Used by the local join runner's FetchAndInstall fast
// path — the join workflow has no version context, so any installed+running
// version is acceptable. Packages that are installed but inactive still go
// through the full reinstall path.
func skipIfAlreadyInstalled(ctx context.Context, name string, existing *node_agentpb.InstalledPackage, checkActive func(context.Context, string) (bool, error)) bool {
	if existing == nil || !strings.EqualFold(existing.GetStatus(), "installed") {
		return false
	}
	unit := packageUnit(name)
	if unit == "" {
		// Command packages have no unit — installed state is sufficient proof.
		log.Printf("workflow-runner: %s already installed@%s (command), skipping", name, existing.GetVersion())
		return true
	}
	active, _ := checkActive(ctx, unit)
	if active {
		log.Printf("workflow-runner: %s already installed@%s and active, skipping", name, existing.GetVersion())
	}
	return active
}
