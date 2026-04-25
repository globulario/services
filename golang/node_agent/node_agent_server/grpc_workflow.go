package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/installed_state"
	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/actions"
	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/supervisor"
	"github.com/globulario/services/golang/node_agent/node_agentpb"
	"github.com/globulario/services/golang/versionutil"
	"github.com/globulario/services/golang/workflow/engine"
	"google.golang.org/protobuf/types/known/structpb"
)

func defaultClusterID() string {
	if d, err := config.GetDomain(); err == nil && strings.TrimSpace(d) != "" {
		return strings.TrimSpace(d)
	}
	return "globular.internal"
}

// RunWorkflow implements the gRPC endpoint for workflow execution.
// The controller (or CLI) calls this to trigger a workflow on the node.
func (srv *NodeAgentServer) RunWorkflow(ctx context.Context, req *node_agentpb.RunWorkflowRequest) (*node_agentpb.RunWorkflowResponse, error) {
	name := req.GetWorkflowName()
	if name == "" {
		name = "node.join"
	}

	// Synthetic workflows: simple actions that don't need a YAML definition.
	switch name {
	case "install-package":
		return srv.runInstallPackage(ctx, req)
	case "uninstall-package":
		return srv.runUninstallPackage(ctx, req)
	case "probe-scylla-health":
		return srv.runProbeScyllaHealth(ctx, req)
	case "probe-etcd-health":
		return srv.runProbeEtcdHealth(ctx, req)
	case "probe-minio-health":
		return srv.runProbeMinioHealth(ctx, req)
	case "day0.bootstrap":
		return srv.runDay0Bootstrap(ctx, req)
	}

	// Resolve definition path.
	defPath := req.GetDefinitionPath()
	if defPath == "" {
		defPath = resolveWorkflowPath(name)
	}
	if defPath == "" {
		return nil, fmt.Errorf("workflow definition %q not found", name)
	}

	// Build inputs from request + local state.
	inputs := make(map[string]any)
	for k, v := range req.GetInputs() {
		inputs[k] = v
	}
	// Fill in defaults from local state.
	if _, ok := inputs["cluster_id"]; !ok {
		inputs["cluster_id"] = defaultClusterID()
	}
	if _, ok := inputs["node_id"]; !ok {
		inputs["node_id"] = srv.nodeID
	}
	if _, ok := inputs["node_hostname"]; !ok && srv.state != nil {
		inputs["node_hostname"] = srv.state.NodeName
	}
	if _, ok := inputs["node_ip"]; !ok && srv.state != nil {
		inputs["node_ip"] = srv.state.AdvertiseIP
	}

	log.Printf("grpc-workflow: starting %s (def=%s)", name, defPath)
	start := time.Now()

	run, err := srv.RunWorkflowDefinition(ctx, defPath, inputs)
	elapsed := time.Since(start)

	resp := &node_agentpb.RunWorkflowResponse{
		DurationMs: elapsed.Milliseconds(),
	}

	if run != nil {
		resp.RunId = run.ID
		resp.Status = string(run.Status)
		for _, st := range run.Steps {
			resp.StepsTotal++
			switch st.Status {
			case engine.StepSucceeded:
				resp.StepsSucceeded++
			case engine.StepFailed:
				resp.StepsFailed++
			}
		}
	}

	if err != nil {
		resp.Status = "FAILED"
		resp.Error = err.Error()
	}

	return resp, nil
}

// runInstallPackage handles the synthetic "install-package" workflow.
// The controller sends this when it wants a single package installed on this node.
func (srv *NodeAgentServer) runInstallPackage(ctx context.Context, req *node_agentpb.RunWorkflowRequest) (*node_agentpb.RunWorkflowResponse, error) {
	inputs := req.GetInputs()
	pkgName := inputs["package_name"]
	pkgKind := inputs["kind"]
	if pkgName == "" {
		return nil, fmt.Errorf("install-package: missing package_name input")
	}
	if pkgKind == "" {
		pkgKind = "SERVICE"
	}

	// CRITICAL: Protect ScyllaDB from reinstall while serving CQL.
	// Reinstalling ScyllaDB wipes Raft state and corrupts the cluster.
	if pkgName == "scylladb" {
		if conn, err := net.DialTimeout("tcp", "0.0.0.0:9042", 2*time.Second); err == nil {
			conn.Close()
			log.Printf("grpc-workflow: install-package scylladb SKIPPED — CQL port 9042 is active, protecting Raft state")
			return &node_agentpb.RunWorkflowResponse{
				Status:         "SUCCEEDED",
				StepsTotal:     1,
				StepsSucceeded: 1,
			}, nil
		}
		// Also try the node's own IPs.
		addrs, _ := net.InterfaceAddrs()
		for _, a := range addrs {
			if ipNet, ok := a.(*net.IPNet); ok && !ipNet.IP.IsLoopback() && ipNet.IP.To4() != nil {
				if conn, err := net.DialTimeout("tcp", ipNet.IP.String()+":9042", 2*time.Second); err == nil {
					conn.Close()
					log.Printf("grpc-workflow: install-package scylladb SKIPPED — CQL active on %s:9042, protecting Raft state", ipNet.IP)
					return &node_agentpb.RunWorkflowResponse{
						Status:         "SUCCEEDED",
						StepsTotal:     1,
						StepsSucceeded: 1,
					}, nil
				}
			}
		}
	}

	// Check if already installed at the desired version with runtime proof.
	desiredVersion := inputs["version"]
	buildID := inputs["build_id"]
	if desiredVersion != "" {
		existing, _ := installed_state.GetInstalledPackage(ctx, srv.nodeID, pkgKind, pkgName)
		skipResult, reason := canSkipInstallPackage(
			ctx, pkgName, pkgKind, desiredVersion, buildID, existing,
			supervisor.IsActive, supervisor.IsLoaded,
		)
		switch skipResult {
		case installSkipAllowed:
			log.Printf("grpc-workflow: %s", reason)
			return &node_agentpb.RunWorkflowResponse{
				Status:         "SUCCEEDED",
				StepsTotal:     1,
				StepsSucceeded: 1,
			}, nil

		case installSkipDeniedInactive:
			// Unit is loaded but inactive — try a Start before full reinstall.
			log.Printf("grpc-workflow: %s", reason)
			unit := packageUnit(pkgName)
			if startErr := supervisor.Start(ctx, unit); startErr == nil {
				if waitErr := supervisor.WaitActive(ctx, unit, 30*time.Second); waitErr == nil {
					log.Printf("grpc-workflow: install-package %s: repair via Start succeeded", pkgName)
					srv.syncInstalledStateToEtcd(ctx)
					return &node_agentpb.RunWorkflowResponse{
						Status:         "SUCCEEDED",
						StepsTotal:     1,
						StepsSucceeded: 1,
					}, nil
				}
			}
			log.Printf("grpc-workflow: install-package %s: repair via Start failed, proceeding with full reinstall", pkgName)

		case installSkipDeniedUnitGone:
			log.Printf("grpc-workflow: %s", reason)
			// fall through to full reinstall

		case installSkipDeniedNoRecord, installSkipDeniedVersion:
			log.Printf("grpc-workflow: %s", reason)
			// fall through to full reinstall
		}
	}

	if buildID != "" {
		log.Printf("grpc-workflow: install-package %s (%s) build_id=%s", pkgName, pkgKind, buildID)
	} else {
		log.Printf("grpc-workflow: install-package %s (%s)", pkgName, pkgKind)
	}
	start := time.Now()

	err := srv.InstallPackage(ctx, pkgName, pkgKind, "", desiredVersion, buildID, "")
	elapsed := time.Since(start)

	resp := &node_agentpb.RunWorkflowResponse{
		DurationMs: elapsed.Milliseconds(),
		StepsTotal: 1,
	}
	if err != nil {
		resp.Status = "FAILED"
		resp.Error = err.Error()
		resp.StepsFailed = 1
		log.Printf("grpc-workflow: install-package %s FAILED (%v): %v", pkgName, elapsed, err)
	} else {
		resp.Status = "SUCCEEDED"
		resp.StepsSucceeded = 1
		log.Printf("grpc-workflow: install-package %s SUCCEEDED (%v)", pkgName, elapsed)
		// Sync installed state after successful install.
		srv.syncInstalledStateToEtcd(ctx)
	}
	return resp, nil
}

// runUninstallPackage handles the synthetic "uninstall-package" workflow.
// It stops and removes a package from the node, then clears its installed state
// from etcd. The steps are:
//  1. Stop and disable the systemd unit (SERVICE/INFRASTRUCTURE only)
//  2. Remove the binary, unit file, config directory, and version marker
//  3. Daemon-reload systemd (SERVICE/INFRASTRUCTURE only)
//  4. Delete the installed-state record from etcd
//  5. Remove the service config from etcd
//  6. Sync installed state
func (srv *NodeAgentServer) runUninstallPackage(ctx context.Context, req *node_agentpb.RunWorkflowRequest) (*node_agentpb.RunWorkflowResponse, error) {
	inputs := req.GetInputs()
	pkgName := inputs["package_name"]
	pkgKind := inputs["kind"]
	if pkgName == "" {
		return nil, fmt.Errorf("uninstall-package: missing package_name input")
	}
	if pkgKind == "" {
		pkgKind = "SERVICE"
	}
	pkgKind = strings.ToUpper(pkgKind)

	log.Printf("grpc-workflow: uninstall-package %s (%s)", pkgName, pkgKind)
	start := time.Now()

	const totalSteps int32 = 3 // uninstall files, clear etcd state, sync

	// Step 1: Uninstall files (stop systemd, remove binary/unit/config).
	// Delegate to the registered package.uninstall or handle COMMAND directly.
	var uninstallErr error
	switch pkgKind {
	case "SERVICE", "INFRASTRUCTURE":
		handler := actions.Get("package.uninstall")
		if handler == nil {
			uninstallErr = fmt.Errorf("action package.uninstall not registered")
		} else {
			argsMap := map[string]any{
				"name": pkgName,
				"kind": pkgKind,
			}
			// Allow caller to specify a custom systemd unit name
			// (e.g. "scylla-server.service" instead of "globular-scylladb.service").
			if unit := inputs["unit"]; unit != "" {
				argsMap["unit"] = unit
			}
			args, err := structpb.NewStruct(argsMap)
			if err != nil {
				uninstallErr = fmt.Errorf("build uninstall args: %w", err)
			} else {
				if _, err := handler.Apply(ctx, args); err != nil {
					uninstallErr = fmt.Errorf("uninstall %s: %w", pkgName, err)
				}
			}
		}
	case "COMMAND":
		// Commands have no systemd unit — just remove the binary and markers.
		binDir := "/usr/lib/globular/bin"
		binPath := filepath.Join(binDir, pkgName)
		_ = os.Remove(binPath)

		// Remove version marker.
		markerPath := versionutil.MarkerPath(pkgName)
		_ = os.RemoveAll(filepath.Dir(markerPath))

		log.Printf("grpc-workflow: uninstall-package %s: removed command binary", pkgName)
	default:
		uninstallErr = fmt.Errorf("unsupported package kind %q", pkgKind)
	}

	if uninstallErr != nil {
		elapsed := time.Since(start)
		log.Printf("grpc-workflow: uninstall-package %s FAILED (%v): %v", pkgName, elapsed, uninstallErr)
		return &node_agentpb.RunWorkflowResponse{
			Status:      "FAILED",
			Error:       uninstallErr.Error(),
			DurationMs:  elapsed.Milliseconds(),
			StepsTotal:  totalSteps,
			StepsFailed: 1,
		}, nil
	}

	// Step 2: Clear installed state from etcd.
	if err := installed_state.DeleteInstalledPackage(ctx, srv.nodeID, pkgKind, pkgName); err != nil {
		log.Printf("grpc-workflow: uninstall-package %s: warning: failed to clear installed state: %v", pkgName, err)
		// Non-fatal — the package files are already removed.
	}

	// Also clean up service config from etcd so it no longer appears in admin catalog.
	if err := config.DeleteServiceConfigurationByName(pkgName); err != nil {
		log.Printf("grpc-workflow: uninstall-package %s: warning: failed to clean service config: %v", pkgName, err)
	}

	// Step 3: Sync installed state so the controller sees the change immediately.
	srv.syncInstalledStateToEtcd(ctx)

	elapsed := time.Since(start)
	log.Printf("grpc-workflow: uninstall-package %s SUCCEEDED (%v)", pkgName, elapsed)
	return &node_agentpb.RunWorkflowResponse{
		Status:         "SUCCEEDED",
		DurationMs:     elapsed.Milliseconds(),
		StepsTotal:     totalSteps,
		StepsSucceeded: totalSteps,
	}, nil
}

// runDay0Bootstrap handles the "day0.bootstrap" workflow.
// This uses RunDay0BootstrapWorkflow which wires the installer-specific actions
// (TLS setup, package install, DNS bootstrap, etc.) that are different from
// the generic workflow runner.
func (srv *NodeAgentServer) runDay0Bootstrap(ctx context.Context, req *node_agentpb.RunWorkflowRequest) (*node_agentpb.RunWorkflowResponse, error) {
	defPath := resolveDay0WorkflowPath()
	if defPath == "" {
		// Also try the generic resolver.
		defPath = resolveWorkflowPath("day0.bootstrap")
	}
	if defPath == "" {
		return nil, fmt.Errorf("day0.bootstrap.yaml not found")
	}

	// Build inputs from request + defaults.
	inputs := make(map[string]any)
	for k, v := range req.GetInputs() {
		inputs[k] = v
	}
	if _, ok := inputs["cluster_id"]; !ok {
		inputs["cluster_id"] = defaultClusterID()
	}
	if _, ok := inputs["bootstrap_node_id"]; !ok {
		inputs["bootstrap_node_id"] = srv.nodeID
	}
	if _, ok := inputs["bootstrap_node_hostname"]; !ok && srv.state != nil {
		inputs["bootstrap_node_hostname"] = srv.state.NodeName
	}
	if _, ok := inputs["domain"]; !ok {
		inputs["domain"] = defaultClusterID()
	}
	if _, ok := inputs["repository_address"]; !ok {
		inputs["repository_address"] = ""
	}

	log.Printf("grpc-workflow: starting day0.bootstrap (def=%s)", defPath)
	start := time.Now()

	run, err := srv.RunDay0BootstrapWorkflow(ctx, defPath, inputs)
	elapsed := time.Since(start)

	resp := &node_agentpb.RunWorkflowResponse{
		DurationMs: elapsed.Milliseconds(),
	}
	if run != nil {
		resp.RunId = run.ID
		resp.Status = string(run.Status)
		for _, st := range run.Steps {
			resp.StepsTotal++
			switch st.Status {
			case engine.StepSucceeded:
				resp.StepsSucceeded++
			case engine.StepFailed:
				resp.StepsFailed++
			}
		}
	}
	if err != nil {
		resp.Status = "FAILED"
		resp.Error = err.Error()
	}
	return resp, nil
}

var fetchWorkflowDefsOnce sync.Once

// resolveWorkflowPath finds a workflow YAML by name.
// On first miss it attempts to fetch all definitions from MinIO.
func resolveWorkflowPath(name string) string {
	candidates := []string{
		fmt.Sprintf("/var/lib/globular/workflows/%s.yaml", name),
		fmt.Sprintf("/tmp/%s.yaml", name),
		fmt.Sprintf("/usr/lib/globular/workflows/%s.yaml", name),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	// Not found on disk — try fetching from MinIO (once).
	fetchWorkflowDefsOnce.Do(func() {
		fetchWorkflowDefsFromMinIO()
	})

	// Retry after fetch.
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}

// fetchWorkflowDefsFromMinIO downloads workflow definitions from the
// globular-config bucket in MinIO to /var/lib/globular/workflows/.
func fetchWorkflowDefsFromMinIO() {
	destDir := "/var/lib/globular/workflows"
	os.MkdirAll(destDir, 0o755)

	// List known workflow definitions and fetch each from MinIO.
	knownDefs := []string{
		"day0.bootstrap.yaml",
		"node.bootstrap.yaml",
		"node.join.yaml",
		"node.repair.yaml",
		"cluster.reconcile.yaml",
		"release.apply.package.yaml",
		"release.apply.infrastructure.yaml",
		"release.remove.package.yaml",
	}

	fetched := 0
	for _, name := range knownDefs {
		key := "workflows/" + name
		data, err := config.GetClusterConfig(key)
		if err != nil {
			log.Printf("workflow-resolver: fetch %s from MinIO: %v", key, err)
			continue
		}
		if data == nil {
			continue
		}
		dest := filepath.Join(destDir, name)
		if err := os.WriteFile(dest, data, 0o644); err != nil {
			log.Printf("workflow-resolver: write %s: %v", dest, err)
			continue
		}
		fetched++
	}
	if fetched > 0 {
		log.Printf("workflow-resolver: fetched %d workflow definitions from MinIO to %s", fetched, destDir)
	} else {
		log.Printf("workflow-resolver: no workflow definitions found in MinIO")
	}
}
