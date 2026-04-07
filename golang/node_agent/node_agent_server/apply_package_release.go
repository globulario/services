package main

import (
	"context"
	"fmt"
	"log"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/globulario/services/golang/installed_state"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/supervisor"
)

// applyMu prevents concurrent ApplyPackageRelease calls for the same package.
var applyMu sync.Mutex

// ApplyPackageRelease fetches a package from the repository, installs it,
// restarts the targeted service, and updates the installed-state registry.
// This is the reusable primitive for leader-aware control-plane deployments.
//
// Authorization: gated by globular.auth.authz with permission="admin" on
// resource "/node_agent/packages/{package_name}". Only controller workflow
// execution (sa principal) or cluster admins can invoke this RPC.
func (srv *NodeAgentServer) ApplyPackageRelease(ctx context.Context, req *node_agentpb.ApplyPackageReleaseRequest) (*node_agentpb.ApplyPackageReleaseResponse, error) {
	name := strings.TrimSpace(req.GetPackageName())
	kind := strings.ToUpper(strings.TrimSpace(req.GetPackageKind()))
	version := strings.TrimSpace(req.GetVersion())
	repoAddr := strings.TrimSpace(req.GetRepositoryAddr())
	operationID := req.GetOperationId()
	platform := strings.TrimSpace(req.GetPlatform())

	if name == "" {
		return nil, fmt.Errorf("package_name is required")
	}
	if kind == "" {
		kind = "SERVICE"
	}
	if kind != "SERVICE" && kind != "INFRASTRUCTURE" {
		return nil, fmt.Errorf("package_kind must be SERVICE or INFRASTRUCTURE, got %q", kind)
	}
	if version == "" {
		return nil, fmt.Errorf("version is required")
	}
	if platform == "" {
		platform = runtime.GOOS + "_" + runtime.GOARCH
	}

	// Idempotency check: skip if already installed at this version+build (unless force).
	if !req.GetForce() {
		existing, _ := installed_state.GetInstalledPackage(ctx, srv.nodeID, kind, name)
		if existing != nil &&
			existing.Version == version &&
			existing.BuildNumber == req.GetBuildNumber() &&
			existing.Status == "installed" {
			log.Printf("apply-package: %s/%s@%s (build %d) already installed, skipping",
				kind, name, version, req.GetBuildNumber())
			return &node_agentpb.ApplyPackageReleaseResponse{
				Ok:          true,
				Message:     "already installed at requested version",
				PackageName: name,
				Version:     version,
				Status:      "skipped",
				OperationId: operationID,
			}, nil
		}
	}

	// Serialize concurrent applies to prevent conflicts.
	applyMu.Lock()
	defer applyMu.Unlock()

	log.Printf("apply-package: starting %s/%s@%s (build %d, repo=%s, op=%s)",
		kind, name, version, req.GetBuildNumber(), repoAddr, operationID)

	// Mark as updating in installed-state.
	now := time.Now().Unix()
	_ = installed_state.WriteInstalledPackage(ctx, &node_agentpb.InstalledPackage{
		NodeId:      srv.nodeID,
		Name:        name,
		Version:     version,
		Kind:        kind,
		Status:      "updating",
		UpdatedUnix: now,
		OperationId: operationID,
		BuildNumber: req.GetBuildNumber(),
	})

	// Use the existing InstallPackage method which handles:
	// - Fetching from repository (with fallback to local packages)
	// - Extracting and installing payload/infrastructure
	// - systemd daemon-reload
	// - Writing version markers
	if err := srv.InstallPackage(ctx, name, kind, repoAddr, version); err != nil {
		log.Printf("apply-package: install failed for %s/%s@%s: %v", kind, name, version, err)

		// Mark as failed in installed-state.
		_ = installed_state.WriteInstalledPackage(ctx, &node_agentpb.InstalledPackage{
			NodeId:      srv.nodeID,
			Name:        name,
			Version:     version,
			Kind:        kind,
			Status:      "failed",
			UpdatedUnix: time.Now().Unix(),
			OperationId: operationID,
			BuildNumber: req.GetBuildNumber(),
			Metadata:    map[string]string{"error": err.Error()},
		})

		return &node_agentpb.ApplyPackageReleaseResponse{
			Ok:          false,
			Message:     fmt.Sprintf("install failed: %v", err),
			PackageName: name,
			Version:     version,
			Status:      "failed",
			ErrorDetail: err.Error(),
			OperationId: operationID,
		}, nil
	}

	// Restart the service.
	unit := "globular-" + strings.ReplaceAll(name, "_", "-") + ".service"
	log.Printf("apply-package: restarting %s", unit)

	// Mark installed before restart — the restart may kill the caller
	// (e.g. when upgrading the cluster-controller, the workflow engine
	// IS the cluster-controller, and restarting it drops the gRPC stream).
	_ = installed_state.WriteInstalledPackage(ctx, &node_agentpb.InstalledPackage{
		NodeId:      srv.nodeID,
		Name:        name,
		Version:     version,
		Kind:        kind,
		Status:      "installed",
		UpdatedUnix: time.Now().Unix(),
		OperationId: operationID,
		BuildNumber: req.GetBuildNumber(),
		Platform:    platform,
	})

	// Defer the restart so the RPC response reaches the caller before
	// the service process is killed. This is critical for self-deploy
	// scenarios where the caller IS the service being restarted.
	go func() {
		time.Sleep(500 * time.Millisecond) // let the response flush
		restartCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := supervisor.Restart(restartCtx, unit); err != nil {
			log.Printf("apply-package: deferred restart failed for %s: %v", unit, err)
			return
		}
		// Health check after restart.
		healthCtx, hcancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer hcancel()
		for {
			select {
			case <-healthCtx.Done():
				log.Printf("apply-package: %s health check timed out after restart", unit)
				return
			default:
			}
			if active, err := supervisor.IsActive(healthCtx, unit); err == nil && active {
				log.Printf("apply-package: %s healthy after restart", unit)
				return
			}
			time.Sleep(1 * time.Second)
		}
	}()

	log.Printf("apply-package: completed %s/%s@%s (restart deferred)", kind, name, version)

	return &node_agentpb.ApplyPackageReleaseResponse{
		Ok:          true,
		Message:     fmt.Sprintf("installed %s/%s@%s, restart scheduled", kind, name, version),
		PackageName: name,
		Version:     version,
		Status:      "installed",
		OperationId: operationID,
	}, nil
}
