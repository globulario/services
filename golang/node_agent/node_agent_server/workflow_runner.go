// @awareness namespace=globular.platform
// @awareness component=platform_node_agent.workflow
// @awareness file_role=workflow_step_runner_and_poller
// @awareness implements=globular.platform:intent.node_agent.is_executor_not_cluster_brain
// @awareness risk=high
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"runtime"
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/installed_state"
	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/actions"
	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/ingress"
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
	if inputs != nil {
		if addr, ok := inputs["repository_address"].(string); ok {
			repoAddr = strings.TrimSpace(addr)
		}
	}
	if repoAddr == "" {
		repoAddr = srv.discoverRepositoryAddr()
	}

	engine.RegisterNodeAgentActions(router, engine.NodeAgentConfig{
		NodeID: srv.nodeID,
		FetchAndInstall: func(ctx context.Context, pkg engine.PackageRef) error {
			if pkg.Name == "keepalived" && !srv.shouldInstallKeepalived(ctx) {
				log.Printf("workflow-runner: skipping keepalived (ingress disabled or node is not a VIP participant)")
				return nil
			}
			// "discovery" is a retired package from older node.join definitions.
			// Keep execution compatible with stale on-node workflow files until
			// workflow package refresh lands everywhere.
			if pkg.Name == "discovery" {
				log.Printf("workflow-runner: skipping deprecated package %s from stale workflow definition", pkg.Name)
				return nil
			}
			// Fast path: skip if already installed and the unit is active.
			// The local join workflow has no version context, so any installed
			// version is acceptable.
			existing, _ := installed_state.GetInstalledPackage(ctx, srv.nodeID, pkg.Kind, pkg.Name)
			if skipIfAlreadyInstalled(ctx, pkg.Name, existing, supervisor.IsActive) {
				return nil
			}
			// Runtime fast path: if installed-state is missing/stale but the unit
			// is already active, treat the package as satisfied for Day-1 join.
			if unit := packageUnit(pkg.Name); unit != "" {
				if active, _ := supervisor.IsActive(ctx, unit); active {
					log.Printf("workflow-runner: %s unit %s already active (installed-state missing/stale), skipping reinstall", pkg.Name, unit)
					return nil
				}
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
					// Day-1 fallback: use the active release BOM package version
					// when repository latest-resolution fails. This avoids
					// version="" -> 0.0.0-dev sentinel installs, and matches
					// Day-0 staged artifacts like envoy_<ver>_linux_amd64.tgz.
					bomVersion, bomErr := resolveVersionFromReleaseIndexFunc(pkg.Name)
					if bomErr == nil && strings.TrimSpace(bomVersion) != "" {
						version = bomVersion
						log.Printf("workflow-runner: resolve latest manifest for %s failed (%v); using release-index version %s", pkg.Name, err, version)
					} else {
						log.Printf("workflow-runner: resolve latest manifest for %s failed (%v); release-index fallback unavailable (%v)", pkg.Name, err, bomErr)
					}
				}
			}
			if err := srv.InstallPackage(ctx, pkg.Name, pkg.Kind, repoAddr, version, buildID, checksum); err != nil {
				return err
			}
			// Start the unit after a successful first-install. install_payload only
			// extracts binary + unit file; it does not start the service. Without this
			// step, SERVICE packages installed during the join workflow stay disabled
			// and inactive after the join completes because the controller reconciler
			// sees installed==desired and dispatches no further actions.
			//
			// minio and sidekick are excluded: minio requires MinIO cluster membership
			// before it can start, and sidekick is its sidecar. Both are activated by
			// a dedicated storage-join workflow once membership is established.
			if unit := packageUnit(pkg.Name); unit != "" && pkg.Name != "minio" && pkg.Name != "sidekick" {
				if startErr := supervisor.Start(ctx, unit); startErr != nil {
					log.Printf("workflow-runner: %s post-install start failed (non-fatal): %v", pkg.Name, startErr)
				} else {
					log.Printf("workflow-runner: %s started after install", pkg.Name)
				}
			}
			return nil
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

func (srv *NodeAgentServer) shouldInstallKeepalived(ctx context.Context) bool {
	etcdClient, err := config.GetEtcdClient()
	if err != nil || etcdClient == nil {
		// Fail open for bootstrap: if etcd is unavailable, keep legacy behavior.
		return true
	}
	readCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	resp, err := etcdClient.Get(readCtx, "/globular/ingress/v1/spec")
	if err != nil || resp == nil || len(resp.Kvs) == 0 {
		return true
	}
	var spec ingress.Spec
	if err := json.Unmarshal(resp.Kvs[0].Value, &spec); err != nil {
		return true
	}
	if spec.Mode != ingress.ModeVIPFailover || spec.VIPFailover == nil {
		return false
	}
	for _, participant := range spec.VIPFailover.Participants {
		if strings.TrimSpace(participant) == strings.TrimSpace(srv.nodeID) {
			return true
		}
	}
	return false
}

// resolveLatestManifestFunc is the implementation used by FetchAndInstall for
// first-time installs. Overridable in tests via the variable below.
var resolveLatestManifestFunc = resolveLatestManifest
var resolveVersionFromReleaseIndexFunc = resolveVersionFromReleaseIndex

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

	// Primary path: resolve latest STABLE artifact identity through the
	// repository resolver. This is the authoritative "latest published"
	// contract for first-install Day-1 paths.
	resolved, rerr := repo.ResolveArtifact(withAgentAuth(ctx), &repositorypb.ResolveArtifactRequest{
		PublisherId: defaultPublisherID,
		Name:        pkgName,
		Kind:        kind,
		Platform:    platform,
		Channel:     repositorypb.ArtifactChannel_STABLE,
	})
	if rerr == nil {
		m := resolved.GetManifest()
		if m != nil && m.GetRef() != nil && strings.TrimSpace(m.GetRef().GetVersion()) != "" {
			return m.GetRef().GetVersion(), m.GetBuildId(), m.GetChecksum(), nil
		}
	}

	// Compatibility fallback: some older repository paths still rely on the
	// manifest getter with an empty version to imply "latest".
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
		if rerr != nil {
			return "", "", "", fmt.Errorf("ResolveArtifact %s: %v; GetArtifactManifest %s: %w", pkgName, rerr, pkgName, err)
		}
		return "", "", "", fmt.Errorf("GetArtifactManifest %s: %w", pkgName, err)
	}
	m := resp.GetManifest()
	if m == nil {
		return "", "", "", fmt.Errorf("GetArtifactManifest %s: empty manifest", pkgName)
	}
	return m.GetRef().GetVersion(), m.GetBuildId(), m.GetChecksum(), nil
}

const releaseIndexPath = "/var/lib/globular/release-index.json"

type releaseIndexDoc struct {
	Packages []struct {
		Name    string `json:"name"`
		Version string `json:"version"`
	} `json:"packages"`
}

// resolveVersionFromReleaseIndex reads the active BOM and returns the package
// version for pkgName. It is a Day-1 degraded-path fallback when repository
// latest-manifest resolution is unavailable.
func resolveVersionFromReleaseIndex(pkgName string) (string, error) {
	raw, err := os.ReadFile(releaseIndexPath)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", releaseIndexPath, err)
	}
	var idx releaseIndexDoc
	if err := json.Unmarshal(raw, &idx); err != nil {
		return "", fmt.Errorf("parse %s: %w", releaseIndexPath, err)
	}
	target := strings.TrimSpace(pkgName)
	for _, p := range idx.Packages {
		if strings.TrimSpace(p.Name) == target {
			v := strings.TrimSpace(p.Version)
			if v == "" {
				return "", fmt.Errorf("package %s has empty version in %s", target, releaseIndexPath)
			}
			return v, nil
		}
	}
	return "", fmt.Errorf("package %s not found in %s", target, releaseIndexPath)
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
