// @awareness namespace=globular.platform
// @awareness component=platform_node_agent.install
// @awareness file_role=package_install_and_convergence_evidence_emission
// @awareness implements=globular.platform:intent.installed_state.owned_by_node_agent
// @awareness implements=globular.platform:intent.node_agent.is_executor_not_cluster_brain
// @awareness implements=globular.platform:intent.install.result_requires_durable_commit
// @awareness risk=critical
// @awareness failure_mode=node_agent.side_effect_before_durable_commit
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/installed_state"
	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/actions"
	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/supervisor"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"github.com/globulario/services/golang/versionutil"
)

// writeBinaryUnverifiedInstalledState records an apply that completed without
// a provable binary identity (no expected_sha256 from manifest, or legacy
// caller). The service may be running, but we cannot claim VERIFIED. Doctor /
// verifier lift this into a package.installed_binary_unverified finding so the
// gap is operator-visible rather than masquerading as success.
//
// Returns a degraded response (Ok=false, Status=installed_unverified). Callers
// must NOT treat this as a SUCCESS path even though no error is set.
func (srv *NodeAgentServer) writeBinaryUnverifiedInstalledState(
	ctx context.Context,
	req *node_agentpb.ApplyPackageReleaseRequest,
	name, kind, version, actualHash, reason string,
) *node_agentpb.ApplyPackageReleaseResponse {
	path := installedBinaryPath(name, kind)
	meta := map[string]string{
		"finding":        "package.installed_binary_unverified",
		"installed_path": path,
		"reason":         reason,
		"apply_run_id":   req.GetOperationId(),
	}
	if actualHash != "" {
		meta["actual_sha256"] = actualHash
	}
	if bid := strings.TrimSpace(req.GetBuildId()); bid != "" {
		meta["expected_build_id"] = bid
	}
	_ = installed_state.WriteInstalledPackage(ctx, &node_agentpb.InstalledPackage{
		NodeId:      srv.nodeID,
		Name:        name,
		Version:     version,
		Kind:        kind,
		Status:      StatusBinaryUnverified,
		UpdatedUnix: time.Now().Unix(),
		OperationId: req.GetOperationId(),
		BuildNumber: req.GetBuildNumber(),
		BuildId:     strings.TrimSpace(req.GetBuildId()),
		// InstalledPackage.Checksum is the installed entrypoint/artifact binary
		// SHA, not the release identity hash.
		Checksum: actualHash,
		Metadata: meta,
	})
	log.Printf("apply-package: UNVERIFIED %s/%s@%s — %s (binary hash present but no expected checksum to compare)",
		kind, name, version, reason)
	return &node_agentpb.ApplyPackageReleaseResponse{
		Ok:          false,
		Message:     fmt.Sprintf("installed but unverified: %s", reason),
		PackageName: name,
		Version:     version,
		Status:      StatusBinaryUnverified,
		ErrorDetail: reason,
		OperationId: req.GetOperationId(),
	}
}

// writeBinaryHashMismatchInstalledState records a failed apply when the
// post-install proof gate (Phase 1 of the diagnostic honesty refactor) rejects
// the deployed binary. The status string is consumed by doctor / the future
// verifier to raise the package.installed_binary_hash_mismatch finding;
// evidence (expected vs actual hash, path, build_id) is carried in Metadata
// so the finding can be lifted without re-deriving values.
func (srv *NodeAgentServer) writeBinaryHashMismatchInstalledState(
	ctx context.Context,
	req *node_agentpb.ApplyPackageReleaseRequest,
	name, kind, version string,
	verr error,
) *node_agentpb.ApplyPackageReleaseResponse {
	var pf proofFailure
	if !errors.As(verr, &pf) {
		// Shouldn't happen — caller is supposed to pass a proofFailure.
		return &node_agentpb.ApplyPackageReleaseResponse{
			Ok: false, Message: verr.Error(), PackageName: name, Version: version,
			Status: "failed", ErrorDetail: verr.Error(), OperationId: req.GetOperationId(),
		}
	}
	meta := pf.EvidenceMap()
	_ = installed_state.WriteInstalledPackage(ctx, &node_agentpb.InstalledPackage{
		NodeId:      srv.nodeID,
		Name:        name,
		Version:     version,
		Kind:        kind,
		Status:      StatusBinaryHashMismatch,
		UpdatedUnix: time.Now().Unix(),
		OperationId: req.GetOperationId(),
		BuildNumber: req.GetBuildNumber(),
		BuildId:     strings.TrimSpace(req.GetBuildId()),
		Metadata:    meta,
	})
	log.Printf("apply-package: REJECTED %s/%s@%s — %s",
		kind, name, version, verr.Error())
	return &node_agentpb.ApplyPackageReleaseResponse{
		Ok:          false,
		Message:     fmt.Sprintf("post-install hash verification failed: %v", verr),
		PackageName: name,
		Version:     version,
		Status:      StatusBinaryHashMismatch,
		ErrorDetail: verr.Error(),
		OperationId: req.GetOperationId(),
	}
}

// applyMu prevents concurrent ApplyPackageRelease calls for the same package.
var applyMu sync.Mutex
var embeddedBuildTokenPattern = regexp.MustCompile(`(?i)(?:^|[.+-])b[0-9]+(?:$|[.+-])`)

// installedBinaryPath returns the expected deployed executable path for a package.
//
// Resolution order:
//  1. The manifest-declared entrypoint sidecar
//     (`/var/lib/globular/services/<name>/entrypoint`) written at install time
//     by readArtifactManifestEntrypoint + versionutil.WriteEntrypoint.
//     This is the source of truth — Project T (INC-2026-0020).
//  2. Legacy fallback: infer the path from the package name + kind.
//     SERVICE packages use "<name>_server" (or "<name>"); INFRASTRUCTURE/
//     COMMAND packages use "<name>". Only used when no sidecar exists,
//     which means the package was installed by a pre-Project-T node-agent.
//
// The legacy path computation forced strings.ReplaceAll(name, "-", "_") for
// SERVICE and used the raw name for INFRASTRUCTURE/COMMAND. Both diverge
// from the manifest's actual entrypoint when the package name uses hyphens
// but the binary uses underscores (e.g. scylla-manager → scylla_manager).
// The sidecar removes the divergence; the fallback is preserved only for
// pre-fix installs that have not yet been reinstalled.
func installedBinaryPath(name, kind string) string {
	if entry := versionutil.ReadEntrypoint(name); entry != "" {
		return filepath.Join(globularBinDir, entry)
	}
	if strings.EqualFold(kind, "SERVICE") {
		// Most services follow the {name}_server convention. A few (e.g. mcp)
		// ship a binary with the plain package name. Probe the _server path
		// first; fall back to the plain name when the file doesn't exist so
		// cachedSha256 can still hash the actual installed binary.
		withSuffix := filepath.Join(globularBinDir, strings.ReplaceAll(name, "-", "_")+"_server")
		if _, err := os.Stat(withSuffix); err == nil {
			return withSuffix
		}
		return filepath.Join(globularBinDir, strings.ReplaceAll(name, "-", "_"))
	}
	return filepath.Join(globularBinDir, name)
}

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
	if normalized, err := versionutil.NormalizeExact(version); err != nil {
		return nil, fmt.Errorf("invalid version %q: %w", version, err)
	} else if embeddedBuildTokenPattern.MatchString(strings.ToLower(normalized)) {
		return nil, fmt.Errorf("version %q embeds a build token; use build_number as a plain integer field", version)
	} else {
		version = normalized
	}
	if req.GetBuildNumber() < 0 {
		return nil, fmt.Errorf("invalid build_number %d: must be >= 0", req.GetBuildNumber())
	}
	if platform == "" {
		platform = runtime.GOOS + "_" + runtime.GOARCH
	}

	// Idempotency check: skip if already installed at this exact version+build.
	// Downgrade guard: NEVER install an older version than what is currently
	// installed, unless Force=true OR rollback_mode=true. This is an absolute
	// rule — automatic rollback is forbidden. If a service needs rollback, a
	// human must decide via `globular pkg rollback` (which sets rollback_mode)
	// or `--force`.
	//
	// Rationale: the reconciler can dispatch stale install workflows (e.g. after
	// power loss, ScyllaDB recovery, or repository returning ancient artifacts).
	// Without this guard, the cluster silently reverts to version 0.0.1.
	// Services killed by external events are NOT faulty — they just need time
	// to recover. Rolling them back makes everything worse.
	if !req.GetForce() && !req.GetRollbackMode() {
		existing, _ := installed_state.GetInstalledPackage(ctx, srv.nodeID, kind, name)
		if existing != nil {
			isPartialApply := existing.Status == "partial_apply"

			if existing.Status == "installed" || isPartialApply {
				// Idempotency: if build_id matches exactly AND not partial_apply, skip.
				// A partial_apply record means the binary was replaced out-of-band
				// without going through the official apply path — it MUST be
				// re-applied to restore consistency (binary + state + marker).
				if buildID != "" && existing.GetBuildId() == buildID && !isPartialApply {
					if !buildIDSkipChecksumOK(existing.GetChecksum(), req.GetExpectedSha256()) {
						log.Printf("apply-package: %s/%s@%s build_id matches but binary checksum %s != expected %s — binary replaced out-of-band, reapplying",
							kind, name, version,
							normalizedHash(existing.GetChecksum()),
							normalizedHash(req.GetExpectedSha256()))
						// fall through to reinstall
					} else {
						// Even on the idempotent build_id skip path, re-stamp the
						// canonical install receipt from the live unit/binary before
						// returning. Without this, stale receipts seeded from legacy
						// sidecars or older unit renderings can survive forever when the
						// package is already at the desired build and no reinstall occurs.
						//
						// Keep the top-level Checksum honest too: it is the installed
						// entrypoint/binary SHA, NOT the release identity hash.
						srv.repairInstalledStateOnBuildIDSkip(ctx, existing, kind, name, version)
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
				}

				if isPartialApply {
					log.Printf("apply-package: %s/%s is in partial_apply state (binary replaced without state update) — re-applying to restore consistency",
						kind, name)
				}

				// Downgrade guard: compare versions to prevent automatic rollback.
				// When req.BuildNumber is 0, treat it as "caller has no opinion on
				// build_number" — this happens when the desired spec omits
				// build_number (common for infrastructure releases). In that case
				// compare versions only; if versions match, accept the request
				// (idempotent reinstall) instead of rejecting as +N → +0 downgrade.
				reqBuildNumber := req.GetBuildNumber()
				if reqBuildNumber == 0 && version == existing.GetVersion() {
					// Same version, no build_number opinion — allow.
				} else {
					cmp, cmpErr := versionutil.CompareFull(
						version, reqBuildNumber,
						existing.GetVersion(), existing.GetBuildNumber(),
					)
					if cmpErr == nil && cmp < 0 {
						msg := fmt.Sprintf("refuse to downgrade %s/%s from %s+%d to %s+%d — automatic rollback is forbidden (use Force=true for manual rollback)",
							kind, name, existing.GetVersion(), existing.GetBuildNumber(), version, reqBuildNumber)
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
	}

	// Capture the previously-installed revision before mutation so the
	// post-success hook can pick the right action label (install / upgrade /
	// rollback) for the InstalledPackageRevision row.
	previousInstalled, _ := installed_state.GetInstalledPackage(ctx, srv.nodeID, kind, name)

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

	// Phase F-final pre-install config policy gate. Runs BEFORE InstallPackage
	// mutates anything. Returns a snapshot of declared-config file state so
	// the post-success hook can emit accurate PRESERVED/REPLACED receipts;
	// returns an error when a FAIL_ON_LOCAL_MODIFICATION conflict is detected
	// (a CONFLICT receipt has already been recorded). In that case we abort.
	publisherID := strings.TrimSpace(req.GetPublisher())
	if publisherID == "" {
		publisherID = defaultPublisherID
	}
	preInstallPkg := &node_agentpb.InstalledPackage{
		Name: name, Version: version, Kind: kind, Platform: platform,
		BuildNumber: req.GetBuildNumber(), BuildId: buildID,
	}
	configSnap, configPolicyObservable, configErr := srv.applyConfigPolicyPreInstall(ctx, repoAddr, publisherID, preInstallPkg, req.GetWorkflowRunId())
	if !configPolicyObservable {
		// Repository unreachable or manifest fetch failed — we don't know
		// whether FAIL_ON_LOCAL_MODIFICATION configs would have blocked
		// this apply. Today we fail-OPEN (apply proceeds) to preserve the
		// pre-existing behaviour during transient repo flap; the
		// distinguished log line is the regression budget for promoting
		// this to fail-CLOSED once we have telemetry on how often it
		// fires. (meta.authority_must_express_uncertainty,
		// forbidden.error_absorbed_into_empty_map.)
		log.Printf("apply-package: config policy unobservable for %s@%s — apply proceeds without FAIL_ON_LOCAL_MODIFICATION gate (next install retries)", name, version)
	}
	if configErr != nil {
		log.Printf("apply-package: BLOCKED by config policy: %v", configErr)
		_ = installed_state.WriteInstalledPackage(ctx, &node_agentpb.InstalledPackage{
			NodeId: srv.nodeID, Name: name, Version: version, Kind: kind,
			Status: "blocked_config_conflict", UpdatedUnix: time.Now().Unix(),
			OperationId: operationID, BuildNumber: req.GetBuildNumber(),
			Metadata: map[string]string{"error": configErr.Error()},
		})
		return &node_agentpb.ApplyPackageReleaseResponse{
			Ok:          false,
			Message:     fmt.Sprintf("config policy blocked apply: %v", configErr),
			PackageName: name,
			Version:     version,
			Status:      "blocked_config_conflict",
			ErrorDetail: configErr.Error(),
			OperationId: operationID,
		}, nil
	}

	// Mark as updating in installed-state. Preserve existing metadata
	// (especially receipt fields like unit_file_sha256) so that if the
	// install fails after this point, the heartbeat can still compare
	// the on-disk unit file against the previous receipt instead of
	// falling to sidecar migration and reporting false unit_file_drift.
	now := time.Now().Unix()
	updatingPkg := &node_agentpb.InstalledPackage{
		NodeId:      srv.nodeID,
		Name:        name,
		Version:     version,
		Kind:        kind,
		Status:      "updating",
		UpdatedUnix: now,
		OperationId: operationID,
		BuildNumber: req.GetBuildNumber(),
		BuildId:     buildID,
	}
	if previousInstalled != nil {
		PreserveInstallReceiptMetadata(previousInstalled, updatingPkg)
	}
	_ = installed_state.WriteInstalledPackage(ctx, updatingPkg)

	// Use the existing InstallPackage method which handles:
	// - Fetching from repository (with fallback to local packages)
	// - Extracting and installing payload/infrastructure
	// - systemd daemon-reload
	// - Writing version markers
	//
	// Identity propagation (root-cause fix, see todo Task 1):
	// build_id + expected_sha256 MUST flow end-to-end so the fetch layer
	// can validate cached bytes. Dropping either field here was the cause of
	// the "stale cache reinstall" incident.
	if err := srv.InstallPackage(ctx, name, kind, repoAddr, version,
		buildID, req.GetExpectedSha256()); err != nil {
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

	// ── COMMAND-kind packages: no service to restart ────────────────────
	// COMMAND packages are CLI binaries (etcdctl, yt-dlp, mc, sctool,
	// sha256sum, restic, claude, etc.). They have no systemd unit, no
	// HealthCheckUnit, and no daemon to enable/restart. Attempting
	// `systemctl enable globular-etcdctl.service` returns exit 5 (unit
	// not found) → the workflow ends RUN_STATUS_FAILED → every Day-1
	// join produces ~5-7 spurious failures even though the binaries
	// were placed correctly.
	//
	// Issue: #2 in the services repo.
	//
	// For COMMAND kind, the install IS the convergence boundary —
	// `srv.InstallPackage` has already extracted the binary, pinned it,
	// and written the version marker. We commit installed-state as
	// "installed" and return success without touching systemctl.
	if kind == "COMMAND" {
		log.Printf("apply-package: kind=COMMAND name=%s — skipping systemctl (no service for CLI package)", name)

		// Phase 1 proof gate: the install path produced a binary; before we
		// declare it "installed", verify the bytes on disk hash to the
		// expected artifact-manifest digest. Mismatch / missing → fail apply
		// and write structured evidence to installed_state.
		actualHash, verdict, verr := verifyInstalledBinaryHashStrict(name, kind, req.GetExpectedSha256(), buildID, operationID)
		if verr != nil {
			return srv.writeBinaryHashMismatchInstalledState(ctx, req, name, kind, version, verr), nil
		}
		if verdict == BinaryUnverified {
			return srv.writeBinaryUnverifiedInstalledState(ctx, req, name, kind, version, actualHash,
				"no expected_sha256 provided in apply request — cannot prove installed binary matches the desired artifact"), nil
		}

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
			// InstalledPackage.Checksum is the installed entrypoint/artifact binary
			// SHA, not the release identity hash. actualHash comes from
			// cachedSha256(installedBinaryPath(...)) via verifyInstalledBinaryHashStrict.
			Checksum: actualHash,
		}
		if actualHash != "" {
			if pkg.Metadata == nil {
				pkg.Metadata = make(map[string]string)
			}
			pkg.Metadata["entrypoint_checksum"] = actualHash
		}
		// Stamp the canonical install receipt (see docs/architecture/
		// retire-systemd-sidecars.md). installed_state.metadata is the
		// sole authority for expected unit/binary content; sidecars are
		// legacy. Best-effort: missing receipt surfaces as fail-closed
		// at heartbeat, which is correct.
		stampReceiptForInstalledPackage(pkg, "node-agent.apply_package_release.command", installedBinaryPath(name, kind))
		if err := installed_state.WriteInstalledPackage(ctx, pkg); err != nil {
			log.Printf("apply-package: WARNING installed_state write failed for COMMAND %s@%s: %v — skipping post-install side effects", name, version, err)
		} else {
			// Record the installed revision in the repository's history and emit
			// config receipts — same hook the service-restart path uses. The
			// rollback workflow consumes this; without it COMMAND installs would
			// never appear in Provenance / Audit screens.
			srv.recordRevisionAndReceipts(ctx, repoAddr, req, pkg, previousInstalled, configSnap)
		}
		return &node_agentpb.ApplyPackageReleaseResponse{
			Ok:          true,
			Message:     fmt.Sprintf("installed %s/%s@%s (COMMAND — no service to restart)", kind, name, version),
			PackageName: name,
			Version:     version,
			Status:      "installed",
			OperationId: operationID,
			BuildId:     buildID,
		}, nil
	}

	// Restart the service and verify it is running before reporting success.
	// installed-state is written AFTER the service is confirmed active — never before.
	// This is the convergence truth boundary: OK=true means the service IS running.
	unit := "globular-" + strings.ReplaceAll(name, "_", "-") + ".service"
	log.Printf("apply-package: restarting %s", unit)

	// ── MinIO topology gate ─────────────────────────────────────────────
	// The minio binary may be installed on any storage-profile node, but
	// globular-minio.service must not start until the node is admitted into
	// ObjectStoreDesiredState.Nodes via apply-topology.
	// Skip Enable+Restart here; reconcileMinioSystemdConfig (syncTicker)
	// will start MinIO once the pool state admits this node.
	// If etcd is unavailable (poolErr != nil), fall through: the topology
	// gate in reconcileMinioSystemdConfig will stop MinIO on the next cycle.
	if name == "minio" {
		nodeIP := srv.nodeIP()
		poolState, poolErr := config.LoadObjectStoreDesiredState(ctx)
		if poolErr == nil && !nodeIPInPool(nodeIP, poolState) {
			log.Printf("apply-package: minio installed on non-member node %s (ip=%s) — skipping service start (held_not_in_topology)", srv.nodeID, nodeIP)
			_ = installed_state.WriteInstalledPackage(ctx, &node_agentpb.InstalledPackage{
				NodeId:      srv.nodeID,
				Name:        name,
				Version:     version,
				Kind:        kind,
				Status:      "installed_held",
				UpdatedUnix: time.Now().Unix(),
				OperationId: operationID,
				BuildNumber: req.GetBuildNumber(),
				BuildId:     buildID,
				Metadata:    map[string]string{"held_reason": "not_in_objectstore_pool"},
			})
			return &node_agentpb.ApplyPackageReleaseResponse{
				Ok:          true,
				Message:     fmt.Sprintf("minio installed on %s (ip=%s) but service held — not in ObjectStoreDesiredState.Nodes (run apply-topology to admit)", srv.nodeID, nodeIP),
				PackageName: name,
				Version:     version,
				Status:      "installed_held",
				OperationId: operationID,
			}, nil
		}
	}

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

	// INFRASTRUCTURE packages whose spec declares install_systemd=false (e.g.
	// etcdctl, mc, rclone, restic, sctool, sha256sum, yt-dlp) have no unit
	// file. Attempting Enable+Restart returns exit 5 (unit not found) and
	// marks the package failed even though the binary was placed correctly.
	// Check for the unit file first; if absent, the install boundary is the
	// binary itself — report success without touching systemctl.
	unitPath := "/etc/systemd/system/" + unit

	// A SERVICE is a long-running daemon and MUST have a systemd unit. If the
	// unit file is missing the service was never fully provisioned (or its unit
	// was removed): installPayload only writes units a package bundles under
	// systemd/, but service-dist packages ship the unit *inline* in the spec's
	// install_services step, so nothing was written. Reporting this as a
	// "binary-only" success leaves the daemon permanently down and makes the
	// reconciler reinstall forever (installed_state matches, unit gone — the
	// installSkipDeniedUnitGone loop). Recover by running the package's bundled
	// spec through the installer engine (writes+enables+starts the unit), then
	// fall through to the normal enable/restart/verify path below. If recovery
	// cannot produce a unit, fail loudly — never report a SERVICE installed with
	// no runnable unit. (install.join_path_must_complete_install_contract,
	// meta.half_done_must_not_look_done,
	// meta.failure_response_must_contract_not_amplify)
	if _, statErr := os.Stat(unitPath); os.IsNotExist(statErr) && isServiceKind(kind) {
		if rerr := srv.recreateServiceUnitFromSpec(ctx, name, version); rerr != nil {
			return srv.writeServiceUnitUnrecoverableInstalledState(ctx, req, name, kind, version, unitPath, rerr), nil
		}
		if _, st2 := os.Stat(unitPath); os.IsNotExist(st2) {
			return srv.writeServiceUnitUnrecoverableInstalledState(ctx, req, name, kind, version, unitPath,
				fmt.Errorf("no install_services unit found in the bundled package spec")), nil
		}
		log.Printf("apply-package: recreated missing unit %s from package spec — proceeding to enable/restart/verify", unit)
	}

	// Genuine binary-only packages have no systemd unit (INFRASTRUCTURE whose
	// spec sets install_systemd=false: etcdctl, mc, rclone, restic, sctool,
	// sha256sum, yt-dlp). A SERVICE never reaches this branch — the block above
	// either recreated its unit or failed loudly.
	if _, statErr := os.Stat(unitPath); os.IsNotExist(statErr) {
		log.Printf("apply-package: %s has no systemd unit — binary-only package, skipping restart", unit)

		// Phase 1 proof gate: see COMMAND branch above. Same contract for
		// binary-only packages — disk hash must match the manifest before
		// we mark installed.
		actualHash, verdict, verr := verifyInstalledBinaryHashStrict(name, kind, req.GetExpectedSha256(), buildID, operationID)
		if verr != nil {
			return srv.writeBinaryHashMismatchInstalledState(ctx, req, name, kind, version, verr), nil
		}
		if verdict == BinaryUnverified {
			return srv.writeBinaryUnverifiedInstalledState(ctx, req, name, kind, version, actualHash,
				"no expected_sha256 provided in apply request — cannot prove installed binary matches the desired artifact"), nil
		}

		binaryOnlyPkg := &node_agentpb.InstalledPackage{
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
			// InstalledPackage.Checksum is the installed entrypoint/artifact binary
			// SHA, not the release identity hash. actualHash comes from
			// cachedSha256(installedBinaryPath(...)) via verifyInstalledBinaryHashStrict.
			Checksum: actualHash,
		}
		if actualHash != "" {
			if binaryOnlyPkg.Metadata == nil {
				binaryOnlyPkg.Metadata = make(map[string]string)
			}
			binaryOnlyPkg.Metadata["entrypoint_checksum"] = actualHash
		}
		stampReceiptForInstalledPackage(binaryOnlyPkg, "node-agent.apply_package_release.binary_only", installedBinaryPath(name, kind))
		if err := installed_state.WriteInstalledPackage(ctx, binaryOnlyPkg); err != nil {
			log.Printf("apply-package: WARNING installed_state write failed for binary-only %s@%s: %v — skipping post-install side effects", name, version, err)
		} else {
			srv.recordRevisionAndReceipts(ctx, repoAddr, req, binaryOnlyPkg, previousInstalled, configSnap)
		}
		return &node_agentpb.ApplyPackageReleaseResponse{
			Ok:          true,
			Message:     fmt.Sprintf("installed %s/%s@%s (binary-only — no systemd unit)", kind, name, version),
			PackageName: name,
			Version:     version,
			Status:      "installed",
			OperationId: operationID,
			BuildId:     buildID,
		}, nil
	}

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
	// Phase 1 proof gate (diagnostic honesty): systemd-active is a CLAIM,
	// not proof. The bytes on disk at the deployed binary path must hash
	// to the expected artifact-manifest digest before we declare installed.
	// Mismatch / missing → fail apply and write structured evidence; do NOT
	// mark installed.
	actualHash, verdict, verr := verifyInstalledBinaryHashStrict(name, kind, req.GetExpectedSha256(), buildID, operationID)
	if verr != nil {
		return srv.writeBinaryHashMismatchInstalledState(ctx, req, name, kind, version, verr), nil
	}
	if verdict == BinaryUnverified {
		return srv.writeBinaryUnverifiedInstalledState(ctx, req, name, kind, version, actualHash,
			"no expected_sha256 provided in apply request — cannot prove installed binary matches the desired artifact"), nil
	}

	// Write installed-state ONLY after the service is confirmed active AND
	// the post-install hash is proved (or, in the unverified path, computed).
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
		// InstalledPackage.Checksum is the installed entrypoint/artifact binary
		// SHA, not the release identity hash. actualHash comes from
		// cachedSha256(installedBinaryPath(...)) via verifyInstalledBinaryHashStrict.
		Checksum: actualHash,
	}
	if actualHash != "" {
		if pkg.Metadata == nil {
			pkg.Metadata = make(map[string]string)
		}
		pkg.Metadata["entrypoint_checksum"] = actualHash
		log.Printf("apply-package: stored entrypoint_checksum for %s: %s", name, actualHash[:16])
	}
	// Canonical install path: stamp the receipt before committing the
	// installed_state record. See docs/architecture/retire-systemd-sidecars.md.
	srv.stampCanonicalReceiptForInstalledPackage(ctx, pkg, "node-agent.apply_package_release.service", installedBinaryPath(name, kind))
	if err := installed_state.WriteInstalledPackage(ctx, pkg); err != nil {
		// The installed-state record is the durable commit. Side effects
		// (revision history, config receipts) must not run if the commit
		// fails — otherwise the repository says "installed at vX" while
		// etcd still says "updating". Log and return success for the
		// binary install (it IS on disk) but skip the side effects.
		// See meta.state_mutations_must_be_durably_committed_before_side_effects.
		log.Printf("apply-package: WARNING installed_state write failed for %s/%s@%s: %v — skipping post-install side effects", kind, name, version, err)
	} else {
		// Phase F post-success hook: record the installed revision in the
		// repository's history table and emit one config-receipt per declared
		// config file. Both calls are best-effort and never block the apply
		// response. The rollback workflow consumes RecordInstalledRevision; the
		// `pkg config conflicts` CLI consumes RecordConfigReceipt.
		srv.recordRevisionAndReceipts(ctx, repoAddr, req, pkg, previousInstalled, configSnap)
	}

	// Tombstone any stale INFRASTRUCTURE record when the package is installed as
	// SERVICE. Services that were originally deployed via Day-0 bootstrap carry a
	// legacy INFRASTRUCTURE record that never gets updated by the release pipeline.
	// If left in place it silently overrides the correct SERVICE version in the
	// heartbeat Phase 2 etcd scan (INFRA ran after SERVICE in the old loop order).
	if kind == "SERVICE" {
		if staleInfra, _ := installed_state.GetInstalledPackage(ctx, srv.nodeID, "INFRASTRUCTURE", name); staleInfra != nil {
			if err := installed_state.DeleteInstalledPackage(ctx, srv.nodeID, "INFRASTRUCTURE", name); err == nil {
				log.Printf("apply-package: removed stale INFRASTRUCTURE record for SERVICE %s (was %s)", name, staleInfra.GetVersion())
			}
		}
	}

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

// repairInstalledStateOnBuildIDSkip refreshes the installed-state binary
// identity and canonical install receipt when ApplyPackageRelease short-circuits
// on an exact build_id match.
//
// This path is best-effort and metadata-only. It exists to heal stale
// installed_state rows left by older installers/day-0 scripts whose unit
// receipts no longer match the on-disk unit content. Without this refresh,
// unit_file_drift can persist forever on nodes that are already at the desired
// build and therefore never take the reinstall path.
func (srv *NodeAgentServer) repairInstalledStateOnBuildIDSkip(ctx context.Context, existing *node_agentpb.InstalledPackage, kind, name, version string) {
	if existing == nil {
		return
	}
	diskHash, hashErr := cachedSha256(installedBinaryPath(name, kind))
	if hashErr != nil || diskHash == "" {
		// No hashable binary means no safe binary-identity repair; still try the
		// skip-path receipt restamp so unit receipts can converge from disk.
		srv.restampReceiptOnInstallSkip(ctx, name, kind, version, existing.GetBuildId())
		return
	}

	if !strings.EqualFold(strings.TrimSpace(existing.GetChecksum()), diskHash) {
		repaired := *existing
		repaired.Checksum = diskHash
		repaired.UpdatedUnix = time.Now().Unix()
		if repaired.Metadata == nil {
			repaired.Metadata = make(map[string]string)
		}
		repaired.Metadata["entrypoint_checksum"] = diskHash
		log.Printf("apply-package: %s/%s@%s build_id matches, repairing top-level Checksum %s → %s (binary SHA from disk)",
			kind, name, version,
			normalizedHash(existing.GetChecksum()),
			normalizedHash(diskHash))
		if werr := installed_state.WriteInstalledPackage(ctx, &repaired); werr != nil {
			log.Printf("apply-package: %s/%s@%s Checksum repair write failed: %v (non-fatal)", kind, name, version, werr)
		}
	}

	// Always refresh the canonical install receipt from the live disk view,
	// even when the top-level binary checksum was already correct.
	srv.restampReceiptOnInstallSkip(ctx, name, kind, version, existing.GetBuildId())
}
