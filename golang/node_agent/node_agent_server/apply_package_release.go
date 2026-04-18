package main

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/globulario/services/golang/installed_state"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/actions"
	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/supervisor"
	"github.com/globulario/services/golang/versionutil"
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
	buildID := strings.TrimSpace(req.GetBuildId()) // Phase 2: exact artifact identity

	if name == "" {
		return nil, fmt.Errorf("package_name is required")
	}
	if kind == "" {
		kind = "SERVICE"
	}
	if kind != "SERVICE" && kind != "INFRASTRUCTURE" && kind != "COMMAND" {
		return nil, fmt.Errorf("package_kind must be SERVICE, INFRASTRUCTURE, or COMMAND, got %q", kind)
	}
	if version == "" {
		return nil, fmt.Errorf("version is required")
	}
	if platform == "" {
		platform = runtime.GOOS + "_" + runtime.GOARCH
	}

	// Idempotency check: skip if already installed at this exact version+build.
	// Downgrade guard: NEVER install an older version than what is currently
	// installed, unless Force=true. This is an absolute rule — automatic rollback
	// is forbidden. If a service needs rollback, a human must decide via the CLI
	// with an explicit --force flag.
	//
	// Rationale: the reconciler can dispatch stale install workflows (e.g. after
	// power loss, ScyllaDB recovery, or repository returning ancient artifacts).
	// Without this guard, the cluster silently reverts to version 0.0.1.
	// Services killed by external events are NOT faulty — they just need time
	// to recover. Rolling them back makes everything worse.
	if !req.GetForce() {
		existing, _ := installed_state.GetInstalledPackage(ctx, srv.nodeID, kind, name)
		if existing != nil {
			isPartialApply := existing.Status == "partial_apply"

			if existing.Status == "installed" || isPartialApply {
				// Idempotency: if build_id matches exactly AND not partial_apply, skip.
				// A partial_apply record means the binary was replaced out-of-band
				// without going through the official apply path — it MUST be
				// re-applied to restore consistency (binary + state + marker).
				if buildID != "" && existing.GetBuildId() == buildID && !isPartialApply {
					log.Printf("apply-package: %s/%s@%s (build %d, build_id=%s) already installed, skipping",
						kind, name, version, req.GetBuildNumber(), buildID)
					return &node_agentpb.ApplyPackageReleaseResponse{
						Ok:          true,
						Message:     "already installed at requested version",
						PackageName: name,
						Version:     version,
						Status:      "skipped",
						OperationId: operationID,
						BuildId:     existing.GetBuildId(),
					}, nil
				}

				if isPartialApply {
					log.Printf("apply-package: %s/%s is in partial_apply state (binary replaced without state update) — re-applying to restore consistency",
						kind, name)
				}

				// Downgrade guard: ALWAYS compare versions, regardless of build_id.
				// A newer version must never be replaced by an older one automatically.
				cmp, cmpErr := versionutil.CompareFull(
					version, req.GetBuildNumber(),
					existing.GetVersion(), existing.GetBuildNumber(),
				)
				if cmpErr == nil && cmp < 0 {
					msg := fmt.Sprintf("refuse to downgrade %s/%s from %s+%d to %s+%d — automatic rollback is forbidden (use Force=true for manual rollback)",
						kind, name, existing.GetVersion(), existing.GetBuildNumber(), version, req.GetBuildNumber())
					log.Printf("apply-package: REJECTED %s", msg)
					return &node_agentpb.ApplyPackageReleaseResponse{
						Ok:          false,
						Message:     msg,
						PackageName: name,
						Version:     version,
						Status:      "rejected",
						ErrorDetail: msg,
						OperationId: operationID,
					}, nil
				}
			}
		}
	}

	// Serialize concurrent applies to prevent conflicts.
	applyMu.Lock()
	defer applyMu.Unlock()

	// Publish guard (Law 8): verify the artifact is PUBLISHED before installing.
	// This is the final safety boundary — even if the controller dispatches an
	// install for a non-PUBLISHED artifact, the node-agent must reject it.
	if repoAddr != "" {
		if err := actions.CheckArtifactPublished(ctx, repoAddr,
			defaultPublisherID, name, version, platform, kind, req.GetBuildNumber()); err != nil {
			log.Printf("apply-package: REJECTED %s/%s@%s — %v", kind, name, version, err)
			return &node_agentpb.ApplyPackageReleaseResponse{
				Ok:          false,
				Message:     fmt.Sprintf("publish guard: artifact not PUBLISHED: %v", err),
				PackageName: name,
				Version:     version,
				Status:      "rejected",
				ErrorDetail: err.Error(),
				OperationId: operationID,
			}, nil
		}
	}

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
		BuildId:     buildID,
	})

	// Use the existing InstallPackage method which handles:
	// - Fetching from repository (with fallback to local packages)
	// - Extracting and installing payload/infrastructure
	// - systemd daemon-reload
	// - Writing version markers
	//
	// Identity propagation (root-cause fix, see todo Task 1):
	// build_number + expected_sha256 MUST flow end-to-end so the fetch layer
	// can validate cached bytes. Dropping either field here was the cause of
	// the "stale cache reinstall" incident.
	if err := srv.InstallPackage(ctx, name, kind, repoAddr, version,
		req.GetBuildNumber(), req.GetExpectedSha256()); err != nil {
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

	// Restart the service and verify it is running before reporting success.
	// installed-state is written AFTER the service is confirmed active — never before.
	// This is the convergence truth boundary: OK=true means the service IS running.
	unit := "globular-" + strings.ReplaceAll(name, "_", "-") + ".service"
	log.Printf("apply-package: restarting %s", unit)

	// ── Self-update edge case ───────────────────────────────────────────
	// When the package being updated IS the node-agent, a synchronous restart
	// would kill this process before the RPC response is sent. Delegate to
	// the external upgrader process which survives our shutdown.
	if name == "node-agent" {
		log.Printf("apply-package: self-update detected — delegating to upgrader")
		upgraderArgs := []string{
			"--unit", unit,
			"--node-id", srv.nodeID,
			"--name", name,
			"--version", version,
			"--build", fmt.Sprintf("%d", req.GetBuildNumber()),
			"--kind", kind,
			"--platform", platform,
			"--operation-id", operationID,
		}
		if req.GetExpectedSha256() != "" {
			upgraderArgs = append(upgraderArgs, "--checksum", req.GetExpectedSha256())
		}
		if buildID != "" {
			upgraderArgs = append(upgraderArgs, "--build-id", buildID)
		}
		if err := supervisor.LaunchUpgrader(upgraderArgs); err != nil {
			errMsg := fmt.Sprintf("launch upgrader failed: %v", err)
			log.Printf("apply-package: %s", errMsg)
			_ = installed_state.WriteInstalledPackage(ctx, &node_agentpb.InstalledPackage{
				NodeId:      srv.nodeID,
				Name:        name,
				Version:     version,
				Kind:        kind,
				Status:      "failed",
				UpdatedUnix: time.Now().Unix(),
				OperationId: operationID,
				BuildNumber: req.GetBuildNumber(),
				Metadata:    map[string]string{"error": errMsg},
			})
			return &node_agentpb.ApplyPackageReleaseResponse{
				Ok:          false,
				Message:     errMsg,
				PackageName: name,
				Version:     version,
				Status:      "failed",
				ErrorDetail: errMsg,
				OperationId: operationID,
			}, nil
		}
		// Upgrader is running — it will restart us, wait for active, and write
		// installed-state. Return success for the install portion; the upgrader
		// owns the restart truth boundary.
		return &node_agentpb.ApplyPackageReleaseResponse{
			Ok:          true,
			Message:     fmt.Sprintf("installed %s/%s@%s, upgrader handling restart", kind, name, version),
			PackageName: name,
			Version:     version,
			Status:      "upgrading",
			OperationId: operationID,
		}, nil
	}

	// ── Normal path: synchronous restart + health verification ──────────
	restartCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Ensure the unit is enabled before restarting. Crash-loop suppression
	// disables units via systemctl disable; without re-enabling here, the
	// unit stays disabled and won't auto-start on reboot.
	if err := supervisor.Enable(restartCtx, unit); err != nil {
		log.Printf("apply-package: enable %s failed (proceeding to restart): %v", unit, err)
	}

	if err := supervisor.Restart(restartCtx, unit); err != nil {
		errMsg := fmt.Sprintf("restart failed for %s: %v", unit, err)
		log.Printf("apply-package: %s", errMsg)
		_ = installed_state.WriteInstalledPackage(ctx, &node_agentpb.InstalledPackage{
			NodeId:      srv.nodeID,
			Name:        name,
			Version:     version,
			Kind:        kind,
			Status:      "failed",
			UpdatedUnix: time.Now().Unix(),
			OperationId: operationID,
			BuildNumber: req.GetBuildNumber(),
			Metadata:    map[string]string{"error": errMsg},
		})
		return &node_agentpb.ApplyPackageReleaseResponse{
			Ok:          false,
			Message:     errMsg,
			PackageName: name,
			Version:     version,
			Status:      "failed",
			ErrorDetail: errMsg,
			OperationId: operationID,
		}, nil
	}

	// Wait for the service to become active (systemd is-active).
	if err := supervisor.WaitActive(restartCtx, unit, 30*time.Second); err != nil {
		errMsg := fmt.Sprintf("service %s did not become active within 30s after restart: %v", unit, err)
		log.Printf("apply-package: %s", errMsg)
		_ = installed_state.WriteInstalledPackage(ctx, &node_agentpb.InstalledPackage{
			NodeId:      srv.nodeID,
			Name:        name,
			Version:     version,
			Kind:        kind,
			Status:      "failed",
			UpdatedUnix: time.Now().Unix(),
			OperationId: operationID,
			BuildNumber: req.GetBuildNumber(),
			Metadata:    map[string]string{"error": errMsg},
		})
		return &node_agentpb.ApplyPackageReleaseResponse{
			Ok:          false,
			Message:     errMsg,
			PackageName: name,
			Version:     version,
			Status:      "failed",
			ErrorDetail: errMsg,
			OperationId: operationID,
		}, nil
	}

	// ── Success: service is running ─────────────────────────────────────
	// Write installed-state ONLY after the service is confirmed active.
	// This is the convergence truth boundary.
	log.Printf("apply-package: %s active after restart — writing installed-state", unit)
	pkg := &node_agentpb.InstalledPackage{
		NodeId:      srv.nodeID,
		Name:        name,
		Version:     version,
		Kind:        kind,
		Status:      "installed",
		UpdatedUnix: time.Now().Unix(),
		OperationId: operationID,
		BuildNumber: req.GetBuildNumber(),
		BuildId:     buildID,
		Platform:    platform,
		Checksum:    req.GetExpectedSha256(),
	}
	// Best-effort: compute the deployed binary's SHA256 so peer nodes can
	// identify manually-copied binaries by checksum lookup.
	binName := strings.ReplaceAll(name, "-", "_") + "_server"
	binPath := filepath.Join(globularBinDir, binName)
	if cksum, err := cachedSha256(binPath); err == nil {
		if pkg.Metadata == nil {
			pkg.Metadata = make(map[string]string)
		}
		pkg.Metadata["entrypoint_checksum"] = cksum
		log.Printf("apply-package: stored entrypoint_checksum for %s: %s", name, cksum[:16])
	}
	_ = installed_state.WriteInstalledPackage(ctx, pkg)

	log.Printf("apply-package: completed %s/%s@%s (running and verified)", kind, name, version)

	return &node_agentpb.ApplyPackageReleaseResponse{
		Ok:          true,
		Message:     fmt.Sprintf("installed %s/%s@%s, service active and verified", kind, name, version),
		PackageName: name,
		Version:     version,
		Status:      "installed",
		OperationId: operationID,
		BuildId:     buildID,
	}, nil
}
