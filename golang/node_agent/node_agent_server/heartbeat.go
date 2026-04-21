package main

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/installed_state"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"github.com/globulario/services/golang/repository/repository_client"
	"github.com/globulario/services/golang/versionutil"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func (srv *NodeAgentServer) StartHeartbeat(ctx context.Context) {
	go srv.heartbeatLoop(ctx)

	// Start event publisher — monitors systemd units and publishes
	// state changes to the event service for the ai_watcher.
	ep := newEventPublisher(srv.nodeID)
	go ep.run(ctx)

	// Start event handler — subscribes to operation events (restart, etc.)
	// and executes them with root privileges.
	eh := newEventHandler(srv)
	go eh.run(ctx)
}

func (srv *NodeAgentServer) heartbeatLoop(ctx context.Context) {
	hb := globular_service.RegisterSubsystem("heartbeat", 30*time.Second)
	// Initial sync: populate installed-state etcd records for packages
	// installed by the Day-0 installer (which doesn't go through plan execution).
	srv.syncInstalledStateToEtcd(ctx)
	srv.syncEtcHosts(ctx)
	// Heal /var/lib/globular/objectstore/minio.json if it was clobbered
	// out-of-band; etcd is authoritative. See minio_contract_reconcile.go.
	srv.reconcileMinioContract(ctx)

	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()

	// Re-sync installed state every 5 minutes so late-arriving packages
	// (e.g. repository not ready at first boot) eventually land in etcd.
	syncTicker := time.NewTicker(5 * time.Minute)
	defer syncTicker.Stop()

	// Attempt controller endpoint recovery every 2 minutes when missing.
	rediscoverTicker := time.NewTicker(2 * time.Minute)
	defer rediscoverTicker.Stop()

	// Track whether we've already logged the missing-endpoint state to
	// avoid repeating the same message every 30 seconds.
	loggedMissingEndpoint := false

	for {
		now := time.Now()
		if err := srv.reportStatus(ctx); err != nil {
			srv.consecutiveHeartbeatFail++
			recordHeartbeatFailure(srv.consecutiveHeartbeatFail)
			// After 3 consecutive failures, reset the cached gRPC client
			// so the next cycle dials fresh. This recovers from transient
			// Envoy/mesh outages where the TLS probe or connection dies
			// but the underlying service comes back.
			if srv.consecutiveHeartbeatFail >= 3 {
				srv.resetControllerClient()
			}
			switch {
			case srv.controllerEndpoint == "":
				srv.controllerConnState = ConnStateRediscovering
			case srv.consecutiveHeartbeatFail >= 3:
				srv.controllerConnState = ConnStateUnreachable
			default:
				srv.controllerConnState = ConnStateDegraded
			}
			// Rate-limit: only log missing-endpoint once until it changes.
			if srv.controllerEndpoint == "" {
				if !loggedMissingEndpoint {
					log.Printf("node heartbeat failed (state=%s): %v", srv.controllerConnState, err)
					loggedMissingEndpoint = true
				}
			} else {
				log.Printf("node heartbeat failed (state=%s): %v", srv.controllerConnState, err)
				loggedMissingEndpoint = false
			}
			hb.TickError(err)
		} else {
			srv.controllerConnState = ConnStateConnected
			srv.lastControllerContact = now
			srv.consecutiveHeartbeatFail = 0
			recordHeartbeatSuccess(now)
			loggedMissingEndpoint = false
			hb.Tick()
		}
		setControllerStateGauge(srv.controllerConnState)
		select {
		case <-ctx.Done():
			return
		case <-syncTicker.C:
			srv.syncInstalledStateToEtcd(ctx)
			srv.syncEtcHosts(ctx)
			srv.reconcileMinioContract(ctx)
			srv.importProvisionalPackages(ctx)
		case <-rediscoverTicker.C:
			shouldRediscover := srv.controllerEndpoint == "" ||
				srv.controllerConnState == ConnStateDegraded ||
				srv.controllerConnState == ConnStateUnreachable ||
				srv.consecutiveHeartbeatFail >= 3
			if shouldRediscover {
				if ep := srv.rediscoverControllerEndpoint(); ep != "" && ep != srv.controllerEndpoint {
					old := srv.controllerEndpoint
					srv.controllerEndpoint = ep
					srv.resetControllerClient()
					if srv.state != nil {
						srv.state.ControllerEndpoint = ep
					}
					if err := srv.saveState(); err != nil {
						log.Printf("node-agent: failed to persist rediscovered controller endpoint: %v", err)
					}
					log.Printf("node-agent: controller endpoint refreshed: %s -> %s", old, ep)
					loggedMissingEndpoint = false
				}
			}
		case <-heartbeat.C:
		}
	}
}

// syncInstalledStateToEtcd writes installed-state records to etcd for every
// locally-installed package that doesn't already have a record. This bridges
// the gap between packages installed by the Day-0 installer (which bypasses
// the plan executor) and the installed_state registry that the admin UI and
// controller rely on.
//
// Two sources are reconciled:
//  1. ComputeInstalledServices — discovers SERVICE packages from systemd units,
//     version markers, and config files.
//  2. Repository catalog — discovers APPLICATION and INFRASTRUCTURE packages
//     that were published and are assumed installed on this (bootstrap) node.
func (srv *NodeAgentServer) syncInstalledStateToEtcd(ctx context.Context) {
	if srv.nodeID == "" {
		log.Printf("nodeagent: sync skipped — node ID not yet assigned")
		return
	}

	now := time.Now().Unix()
	platform := runtime.GOOS + "_" + runtime.GOARCH
	synced := 0

	// Phase -1: Adopt services that are running but have no version marker.
	// This covers the legacy Day-0 installer path: binaries deployed to
	// globularBinDir with active systemd units but no version marker written.
	// Without this phase, loadSystemdUnits() marks them RuntimeUnmanaged and
	// IsAuthoritative() filters them out, so etcd never learns they exist.
	// After adoption, Phase 0 and Phase 1 handle them normally.
	srv.adoptRunningUnmanagedServices(ctx)

	// Phase 0: Refresh version markers from deployed binaries. When an
	// operator direct-installs a service binary (e.g. `sudo install`), the
	// version marker at /var/lib/globular/services/<name>/version is not
	// updated, so loadMarkers() would return the stale version and the
	// drift-reconciler would re-roll the repo version on top of the direct
	// install. Probing each binary with --describe gives ground truth.
	srv.refreshMarkersFromBinaries(ctx)

	// Phase 1: Sync packages from local discovery.
	// Day0/join infrastructure (e.g. etcd) is written as INFRASTRUCTURE;
	// all other locally-discovered packages are written as SERVICE.
	installed, _, err := ComputeInstalledServices(ctx)
	if err != nil {
		log.Printf("nodeagent: ComputeInstalledServices failed: %v", err)
	}
	if len(installed) > 0 {
		for key, info := range installed {
			name := canonicalServiceName(key.ServiceName)
			if name == "" {
				continue
			}
			// Only sync authoritative (ManagedInstalled) observations to etcd.
			// Non-authoritative entries (RuntimeUnmanaged, FallbackDiscovered)
			// must not become installed-state records — they would poison the
			// controller's convergence and desired-state import.
			// Defense-in-depth: also reject known bad version strings.
			if !info.IsAuthoritative() {
				continue
			}
			if info.Version == "unknown" || info.Version == "" {
				continue
			}
			kind := "SERVICE"
			if isDay0JoinInfra(name) {
				kind = "INFRASTRUCTURE"
			}
			existing, _ := installed_state.GetInstalledPackage(ctx, srv.nodeID, kind, name)
			if existing != nil {
				// Update existing record if version changed (e.g. after apply-desired).
				if info.Version != "" && info.Version != existing.GetVersion() {
					oldVer := existing.GetVersion()
					existing.Version = info.Version
					existing.UpdatedUnix = now
					existing.Status = "installed"
					if info.PublisherID != "" {
						existing.PublisherId = info.PublisherID
					}
					if err := installed_state.WriteInstalledPackage(ctx, existing); err != nil {
						log.Printf("nodeagent: update installed-state %s/%s to %s: %v", kind, name, info.Version, err)
					} else {
						log.Printf("nodeagent: updated installed-state %s/%s: %s → %s", kind, name, oldVer, info.Version)
						synced++
					}
				}
				continue
			}
			pkg := &node_agentpb.InstalledPackage{
				NodeId:        srv.nodeID,
				Name:          name,
				Version:       info.Version,
				PublisherId:   info.PublisherID,
				Platform:      platform,
				Kind:          kind,
				InstalledUnix: now,
				UpdatedUnix:   now,
				Status:        "installed",
			}
			if err := installed_state.WriteInstalledPackage(ctx, pkg); err != nil {
				log.Printf("nodeagent: sync installed-state %s/%s: %v", kind, name, err)
				continue
			}
			synced++
		}
	}

	// Phase 1.5: Detect partial-apply (binary replaced without state update).
	// If a binary's SHA256 differs from the entrypoint_checksum in etcd but the
	// version/build_id is unchanged, the binary was replaced out-of-band (e.g.
	// manual scp or sudo install) without going through ApplyPackageRelease.
	// Mark the record as PARTIAL_APPLY so the controller doesn't silently
	// treat it as healthy.
	srv.detectPartialApply(ctx, now)

	// Phase 2: Enrich existing records with correct version/kind from repo.
	srv.syncRepoArtifactsToEtcd(ctx, now, platform, &synced)

	// Phase 3: Clean up stale SERVICE records for packages not actually
	// installed on this node. Only remove records where the unit file is
	// gone AND no version marker exists — failed/inactive services with
	// unit files or markers are genuinely installed, just not running.
	if srv.nodeID != "" {
		localNames := make(map[string]bool, len(installed))
		for key := range installed {
			localNames[canonicalServiceName(key.ServiceName)] = true
		}
		if svcPkgs, err := installed_state.ListInstalledPackages(ctx, srv.nodeID, "SERVICE"); err == nil {
			for _, pkg := range svcPkgs {
				name := pkg.GetName()
				if localNames[name] {
					continue // genuinely installed locally
				}
				// Not discovered by loadSystemdUnits/loadMarkers — check if
				// the unit file or version marker still exists on disk.
				unitName := "globular-" + name + ".service"
				unitFileExists := false
				if out, err := exec.CommandContext(ctx, "systemctl", "list-unit-files", unitName, "--no-legend", "--no-pager").Output(); err == nil {
					unitFileExists = strings.TrimSpace(string(out)) != ""
				}
				markerPath := filepath.Join(versionutil.BaseDir(), name, "version")
				_, markerErr := os.Stat(markerPath)
				markerExists := markerErr == nil

				if !unitFileExists && !markerExists {
					// Truly uninstalled — remove stale record.
					_ = installed_state.DeleteInstalledPackage(ctx, srv.nodeID, "SERVICE", name)
					log.Printf("nodeagent: removed stale installed-state SERVICE/%s (no unit file, no version marker)", name)
				}
			}
		}
	}

	// Phase 4: Backfill missing version markers from etcd records.
	// Packages installed by bootstrap, manual deploy, or other tools may
	// exist in etcd installed_state but lack a local version marker. Without
	// the marker, loadMarkers() won't discover them on the next heartbeat,
	// causing the package to appear as "not installed" in health checks.
	if srv.nodeID != "" {
		for _, kind := range []string{"SERVICE", "COMMAND", "INFRASTRUCTURE"} {
			pkgs, err := installed_state.ListInstalledPackages(ctx, srv.nodeID, kind)
			if err != nil {
				continue
			}
			for _, pkg := range pkgs {
				name := pkg.GetName()
				ver := pkg.GetVersion()
				if name == "" || ver == "" {
					continue // skip unknown/fallback versions
				}
				markerPath := versionutil.MarkerPath(name)
				if _, err := os.Stat(markerPath); err == nil {
					continue // marker already exists
				}
				// Create the missing marker directory and file.
				if err := os.MkdirAll(filepath.Dir(markerPath), 0o755); err != nil {
					continue
				}
				if err := os.WriteFile(markerPath, []byte(ver+"\n"), 0o644); err != nil {
					log.Printf("nodeagent: backfill marker %s: %v", markerPath, err)
					continue
				}
				log.Printf("nodeagent: backfilled version marker %s/%s → %s", kind, name, ver)
				synced++
			}
		}
	}

	if synced > 0 {
		log.Printf("nodeagent: synced %d installed-state records to etcd", synced)
	}
}

// globularBinDir is the canonical directory where Globular service binaries
// are deployed. Matches internal/actions.ActionBinDir. Duplicated rather than
// imported to avoid pulling the actions package into the heartbeat loop.
const globularBinDir = "/usr/lib/globular/bin"

// refreshMarkersFromBinaries probes each deployed service binary with
// --describe and rewrites /var/lib/globular/services/<name>/version when the
// binary reports a different version from the marker. This detects
// out-of-band installs (sudo install, manual scp, etc.) that bypass the
// plan executor and would otherwise leave the marker stale, causing the
// drift-reconciler to clobber the direct install with the repo version.
//
// Only probes when binary mtime is newer than marker mtime — so the
// happy path (no direct installs) is a cheap stat per marker with zero
// subprocess execs.
func (srv *NodeAgentServer) refreshMarkersFromBinaries(ctx context.Context) {
	markerRoot := versionutil.BaseDir()
	entries, err := os.ReadDir(markerRoot)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			log.Printf("nodeagent: refresh markers: read %s: %v", markerRoot, err)
		}
		return
	}
	refreshed := 0
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		markerPath := filepath.Join(markerRoot, name, "version")
		markerStat, err := os.Stat(markerPath)
		if err != nil {
			continue // no marker → nothing to refresh
		}

		// Convention: binary name is <name_with_underscores>_server.
		// This matches executableForService() fallback in internal/actions.
		// Services that don't follow the convention (etcd, minio, envoy,
		// scylladb, …) are handled by loadDay0JoinInfra/syncRepoArtifacts.
		binName := strings.ReplaceAll(name, "-", "_") + "_server"
		binPath := filepath.Join(globularBinDir, binName)
		binStat, err := os.Stat(binPath)
		if err != nil {
			continue
		}
		if !binStat.ModTime().After(markerStat.ModTime()) {
			continue // marker is at least as fresh as the binary
		}

		// Binary is newer than marker — probe for ground-truth version.
		// Some binaries (e.g. node_agent itself) do not implement --describe.
		// Silently skip those: a non-zero exit is indistinguishable from
		// "unsupported flag" here, and logging every probe spams the journal.
		probeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		out, err := exec.CommandContext(probeCtx, binPath, "--describe").Output()
		cancel()
		if err != nil {
			// Touch mtime so we don't re-probe the same unsupported binary
			// every heartbeat until it is actually reinstalled.
			_ = os.Chtimes(markerPath, time.Now(), time.Now())
			continue
		}
		// --describe JSON includes {"Version": "0.0.2", ...}.
		// Minimal struct to avoid coupling to the full DescribeMap schema.
		var payload struct {
			Version string `json:"Version"`
		}
		if err := json.Unmarshal(out, &payload); err != nil || strings.TrimSpace(payload.Version) == "" {
			_ = os.Chtimes(markerPath, time.Now(), time.Now())
			continue
		}
		newVer := strings.TrimSpace(payload.Version)
		if cv, err := versionutil.Canonical(newVer); err == nil {
			newVer = cv
		}
		// Compare against current marker content.
		cur, err := os.ReadFile(markerPath)
		if err != nil {
			continue
		}
		curVer := strings.TrimSpace(string(cur))
		if cv, err := versionutil.Canonical(curVer); err == nil {
			curVer = cv
		}
		if curVer == newVer {
			// Touch the marker so the mtime check doesn't re-probe every
			// heartbeat when the binary happens to be newer.
			_ = os.Chtimes(markerPath, time.Now(), time.Now())
			continue
		}
		// NEVER downgrade a marker. If the binary reports an OLDER version
		// than the marker, trust the marker — the binary may simply be a
		// stale leftover (e.g. the Day-0 installer binary), or this probe
		// may have run against an older launcher wrapper. Downgrading was
		// the cause of a prior incident where a service marker silently
		// went from 0.0.2 → 0.0.1 on heartbeat.
		if cmp, cmpErr := versionutil.Compare(newVer, curVer); cmpErr == nil && cmp <= 0 {
			log.Printf("nodeagent: refresh markers: skipping %s — binary reports %s ≤ marker %s (no downgrade)",
				name, newVer, curVer)
			// Touch mtime so we don't re-probe every heartbeat.
			_ = os.Chtimes(markerPath, time.Now(), time.Now())
			continue
		}
		if err := os.WriteFile(markerPath, []byte(newVer+"\n"), 0o644); err != nil {
			log.Printf("nodeagent: refresh markers: write %s: %v", markerPath, err)
			continue
		}
		log.Printf("nodeagent: refreshed version marker %s: %s → %s (direct install detected)",
			name, curVer, newVer)
		refreshed++
	}
	if refreshed > 0 {
		log.Printf("nodeagent: refreshed %d version markers from binary probes", refreshed)
	}
}

// adoptRunningUnmanagedServices creates version markers for Globular service
// binaries that are actively running (systemd unit active) but have no version
// marker file. This is the legacy Day-0 installer gap: old installers deployed
// binaries and systemd units without writing the /var/lib/globular/services/
// <name>/version marker that the managed install path produces.
//
// Without this adoption step, loadSystemdUnits() classifies these services as
// RuntimeUnmanaged and IsAuthoritative() filters them out, so they never appear
// in etcd installed-state and the controller perpetually believes they are not
// installed. Phase 0 (refreshMarkersFromBinaries) then enriches markers from
// binary probes — but only when a marker already exists. This phase fills the
// gap for the very first heartbeat after a legacy Day-0 deploy.
//
// Safeguards:
//   - Only creates markers (never downgrades or removes them).
//   - Only for binaries following the Globular naming convention (*_server).
//   - Only when the corresponding globular-<name>.service is active right now.
//   - Skips infrastructure managed by loadDay0JoinInfra / syncRepoArtifacts.
func (srv *NodeAgentServer) adoptRunningUnmanagedServices(ctx context.Context) {
	// Infrastructure whose version/kind is managed by other discovery paths.
	skipAdopt := map[string]bool{
		"etcd": true, "minio": true, "envoy": true,
		"xds": true, "gateway": true, "mcp": true,
		"node-exporter": true, "prometheus": true,
		"scylla-manager": true, "scylla-manager-agent": true,
		"scylladb": true, "keepalived": true, "sidekick": true,
	}

	entries, err := os.ReadDir(globularBinDir)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			log.Printf("nodeagent: adopt unmanaged: read %s: %v", globularBinDir, err)
		}
		return
	}

	adopted := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), "_server") {
			continue
		}
		// Derive canonical service name from binary name (e.g. "echo_server" → "echo").
		rawName := strings.TrimSuffix(e.Name(), "_server")
		svcName := canonicalServiceName(rawName)
		if svcName == "" || skipAdopt[svcName] {
			continue
		}

		// Skip if a version marker already exists — refreshMarkersFromBinaries
		// handles the update case.
		markerPath := versionutil.MarkerPath(svcName)
		if _, err := os.Stat(markerPath); err == nil {
			continue
		}

		// Only adopt if the unit is currently active — don't create markers
		// for services that are installed but not running (they may be
		// intentionally stopped, or failed).
		unitName := "globular-" + svcName + ".service"
		if err := exec.CommandContext(ctx, "systemctl", "is-active", "--quiet", unitName).Run(); err != nil {
			continue
		}

		// Probe the binary for its self-reported version.
		binPath := filepath.Join(globularBinDir, e.Name())
		probeCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		out, probeErr := exec.CommandContext(probeCtx, binPath, "--describe").Output()
		cancel()
		if probeErr != nil {
			// Binary does not support --describe — skip silently.
			continue
		}
		var payload struct {
			Version string `json:"Version"`
		}
		if err := json.Unmarshal(out, &payload); err != nil {
			continue
		}
		ver := strings.TrimSpace(payload.Version)
		if ver == "" {
			continue
		}
		if cv, err := versionutil.Canonical(ver); err == nil {
			ver = cv
		}

		// Create marker directory and file.
		if err := os.MkdirAll(filepath.Dir(markerPath), 0o755); err != nil {
			log.Printf("nodeagent: adopt unmanaged: mkdir %s: %v", filepath.Dir(markerPath), err)
			continue
		}
		if err := os.WriteFile(markerPath, []byte(ver+"\n"), 0o644); err != nil {
			log.Printf("nodeagent: adopt unmanaged: write marker %s: %v", markerPath, err)
			continue
		}
		log.Printf("nodeagent: adopted running service %s v%s (created version marker from binary probe)", svcName, ver)
		adopted++
	}
	if adopted > 0 {
		log.Printf("nodeagent: adopted %d running services that had no version marker", adopted)
	}
}

// syncRepoArtifactsToEtcd queries the repository for all published artifacts
// and enriches existing installed-state records with correct version and kind
// from the repo. It does NOT create new records for packages that weren't
// discovered locally by Phase 1 — only Phase 1 (systemd/markers/config) knows
// what's actually installed on this machine. Phase 2 only:
//   - Updates version for existing records that have the fallback ""
//   - Corrects the kind (e.g. SERVICE → INFRASTRUCTURE) for misclassified records
//   - Creates records ONLY for INFRASTRUCTURE/COMMAND packages that have a
//     matching systemd unit running (verified via systemctl)
func (srv *NodeAgentServer) syncRepoArtifactsToEtcd(ctx context.Context, now int64, platform string, synced *int) {
	// Resolve repository address from etcd — source of truth for address and port.
	repoAddr := config.ResolveLocalServiceAddr("repository.PackageRepository")

	rc, err := repository_client.NewRepositoryService_Client(repoAddr, "repository.PackageRepository")
	if err != nil {
		log.Printf("nodeagent: sync repo artifacts: connect to repo: %v", err)
		return
	}
	defer rc.Close()

	arts, err := rc.ListArtifacts()
	if err != nil {
		log.Printf("nodeagent: sync repo artifacts: list: %v", err)
		return
	}

	for _, m := range arts {
		ref := m.GetRef()
		if ref == nil || ref.GetName() == "" || ref.GetVersion() == "" {
			continue
		}

		kind := "SERVICE"
		switch ref.GetKind() {
		case 2: // APPLICATION
			kind = "APPLICATION"
		case 3, 4: // AGENT, SUBSYSTEM
			kind = "APPLICATION"
		case 5: // INFRASTRUCTURE
			kind = "INFRASTRUCTURE"
		case 6: // COMMAND
			kind = "COMMAND"
		}

		name := ref.GetName()

		// Check for existing record with the correct kind.
		existing, _ := installed_state.GetInstalledPackage(ctx, srv.nodeID, kind, name)

		// Also check for misclassified SERVICE record (Phase 1 may have
		// written it as SERVICE when the repo says INFRASTRUCTURE/COMMAND).
		var staleService *node_agentpb.InstalledPackage
		if kind != "SERVICE" {
			staleService, _ = installed_state.GetInstalledPackage(ctx, srv.nodeID, "SERVICE", name)
		}

		if existing != nil && existing.GetVersion() != "" {
			// Already has a real version with correct kind — skip.
			continue
		}

		// If there's no existing record and no stale SERVICE record,
		// this artifact is in the repo but NOT installed on this node.
		// Only create new records for INFRASTRUCTURE packages that have
		// a running systemd unit (they were skipped by Phase 1's skipSystemd
		// list but are genuinely installed as daemons).
		if existing == nil && staleService == nil {
			if kind == "SERVICE" || kind == "APPLICATION" {
				// SERVICE/APPLICATION: must have been discovered by Phase 1.
				// If not found, it's not installed on this node — skip.
				continue
			}
			if kind == "COMMAND" {
				// COMMAND packages are standalone binaries — check if the
				// binary exists on disk rather than looking for a systemd unit.
				if !commandBinaryExists(name) {
					continue
				}
			} else {
				// INFRASTRUCTURE: check if a systemd unit is actually running.
				unitName := "globular-" + name + ".service"
				if err := exec.CommandContext(ctx, "systemctl", "is-active", "--quiet", unitName).Run(); err != nil {
					// Not running — not installed on this node.
					continue
				}
			}
		}

		// Clean up the stale SERVICE record if the artifact is really a
		// different kind.
		if staleService != nil {
			_ = installed_state.DeleteInstalledPackage(ctx, srv.nodeID, "SERVICE", name)
			if existing == nil {
				existing = staleService // preserve timestamps
			}
		}

		pkg := &node_agentpb.InstalledPackage{
			NodeId:        srv.nodeID,
			Name:          name,
			Version:       ref.GetVersion(),
			PublisherId:   ref.GetPublisherId(),
			Platform:      platform,
			Kind:          kind,
			Checksum:      m.GetChecksum(),
			BuildNumber:   m.GetBuildNumber(),
			InstalledUnix: now,
			UpdatedUnix:   now,
			Status:        "installed",
		}
		if existing != nil {
			// Preserve original install timestamp when updating.
			pkg.InstalledUnix = existing.GetInstalledUnix()
		}
		if err := installed_state.WriteInstalledPackage(ctx, pkg); err != nil {
			log.Printf("nodeagent: sync installed-state %s/%s: %v", kind, name, err)
			continue
		}
		*synced++
	}
}

// commandBinaryExists checks whether a COMMAND package's binary is installed
// on disk. It strips the "-cmd" suffix (used in artifact names) and probes
// the standard binary locations.
func commandBinaryExists(name string) bool {
	bin := strings.TrimSuffix(name, "-cmd")
	for _, dir := range []string{"/usr/local/bin", "/usr/lib/globular/bin"} {
		if _, err := os.Stat(filepath.Join(dir, bin)); err == nil {
			return true
		}
	}
	// Also check PATH as a fallback
	if _, err := exec.LookPath(bin); err == nil {
		return true
	}
	return false
}

// detectPartialApply checks for binary replacement without state update.
// When a binary is replaced manually (scp, sudo install), the entrypoint_checksum
// in etcd no longer matches the actual binary on disk, but the version/build_id
// still shows the old values. This function detects that mismatch and marks the
// record status as "partial_apply" so the controller and cluster-doctor can flag it.
func (srv *NodeAgentServer) detectPartialApply(ctx context.Context, now int64) {
	if srv.nodeID == "" {
		return
	}
	for _, kind := range []string{"SERVICE", "INFRASTRUCTURE"} {
		pkgs, err := installed_state.ListInstalledPackages(ctx, srv.nodeID, kind)
		if err != nil {
			continue
		}
		for _, pkg := range pkgs {
			name := pkg.GetName()
			if name == "" {
				continue
			}
			// Only check packages that have entrypoint_checksum recorded.
			recordedChecksum := pkg.GetMetadata()["entrypoint_checksum"]
			if recordedChecksum == "" {
				continue
			}
			// Compute current binary checksum.
			binName := strings.ReplaceAll(name, "-", "_") + "_server"
			binPath := filepath.Join(globularBinDir, binName)
			currentChecksum, err := cachedSha256(binPath)
			if err != nil {
				continue // binary not found on disk — not a partial apply
			}
			if currentChecksum == recordedChecksum {
				continue // checksums match — no mismatch
			}
			// Binary changed but installed-state wasn't updated → partial apply.
			if pkg.GetStatus() == "partial_apply" {
				continue // already flagged
			}
			log.Printf("nodeagent: PARTIAL_APPLY detected for %s/%s — binary checksum changed (%s… → %s…) but installed-state was not updated. Manual binary replacement without ApplyPackageRelease.",
				kind, name, recordedChecksum[:16], currentChecksum[:16])
			pkg.Status = "partial_apply"
			pkg.UpdatedUnix = now
			_ = installed_state.WriteInstalledPackage(ctx, pkg)
		}
	}
}

// rediscoverControllerEndpoint attempts to resolve the controller endpoint
// using the same 3-step discovery order as node-agent startup (server.go):
//  1. etcd service registry (config.ResolveServiceAddr)
//  2. DNS form: controller.<clusterDomain>:<port>
//  3. persisted state fallback
func (srv *NodeAgentServer) rediscoverControllerEndpoint() string {
	// Step 1: etcd service registry — same call used at startup.
	if addr := config.ResolveServiceAddr("cluster_controller.ClusterControllerService", ""); addr != "" {
		return addr
	}

	// Step 2: DNS-based discovery — same as startup when clusterDomain is set.
	clusterDomain := ""
	if srv.state != nil {
		clusterDomain = strings.TrimSpace(srv.state.ClusterDomain)
	}
	if clusterDomain != "" {
		controllerPort := "12000"
		if addr := config.ResolveServiceAddr("cluster_controller.ClusterControllerService", ""); addr != "" {
			if _, p, err := net.SplitHostPort(addr); err == nil && p != "" {
				controllerPort = p
			}
		}
		return fmt.Sprintf("controller.%s:%s", clusterDomain, controllerPort)
	}

	// Step 3: persisted state fallback.
	if srv.state != nil && srv.state.ControllerEndpoint != "" {
		return srv.state.ControllerEndpoint
	}

	return ""
}

func (srv *NodeAgentServer) reportStatus(ctx context.Context) error {
	if srv.controllerEndpoint == "" {
		return fmt.Errorf("controller endpoint not configured — heartbeat skipped")
	}
	if srv.nodeID == "" {
		return fmt.Errorf("node ID not assigned — heartbeat skipped")
	}
	identity := srv.buildNodeIdentity()
	units := convertNodeAgentUnits(detectUnits(ctx))
	statusReq := &cluster_controllerpb.NodeStatus{
		NodeId:            srv.nodeID,
		Identity:          identity,
		Ips:               append([]string(nil), identity.GetIps()...),
		Units:             units,
		LastError:         "",
		ReportedAt:        timestamppb.Now(),
		AgentEndpoint:     srv.advertisedAddr,
		InventoryComplete: len(units) > 0,
		Capabilities:      buildNodeCapabilities(),
	}
	installed, appliedHash, err := ComputeInstalledServices(ctx)
	if err != nil {
		log.Printf("nodeagent: compute installed services: %v", err)
	}
	statusReq.InstalledVersions = make(map[string]string)
	statusReq.InstalledBuildIds = make(map[string]string)

	// Phase 1: local discovery (systemd units, version markers, config files).
	// Only report authoritative (ManagedInstalled) observations. Non-authoritative
	// entries (RuntimeUnmanaged, FallbackDiscovered) must not participate in
	// convergence checks — reporting them causes the controller to treat them
	// as installed-state, potentially creating phantom desired entries.
	// Defense-in-depth: also reject "unknown" and "" by string check.
	for key, info := range installed {
		if !info.IsAuthoritative() {
			continue
		}
		if info.Version == "unknown" || info.Version == "" {
			continue // defense-in-depth: reject known bad versions even if source is wrong
		}
		statusReq.InstalledVersions[key.String()] = info.Version
	}

	// Phase 2: etcd installed_state (from repository) overrides local versions.
	// The repository is the source of truth for version numbers.
	if srv.nodeID != "" {
		for _, kind := range []string{"SERVICE", "APPLICATION", "INFRASTRUCTURE"} {
			pkgs, err := installed_state.ListInstalledPackages(ctx, srv.nodeID, kind)
			if err != nil {
				continue
			}
			for _, pkg := range pkgs {
				canon := canonicalServiceName(pkg.GetName())
				if canon == "" || pkg.GetVersion() == "" {
					continue
				}
				statusReq.InstalledVersions[canon] = pkg.GetVersion()
				if bid := pkg.GetBuildId(); bid != "" {
					statusReq.InstalledBuildIds[canon] = bid
				}
			}
		}
	}

	// Phase 2.5: peer checksum lookup for unknown-version services.
	//
	// When Phase 1 finds a binary on disk (systemd unit exists) but has no
	// version marker, and Phase 2 has no etcd record for this node, compute
	// the binary's SHA256 and look for a matching entrypoint_checksum in
	// other nodes' installed_state records. This handles manually-copied
	// binaries (scp between nodes) without touching the repository service.
	//
	// This is best-effort: if etcd is slow or no peer has the checksum,
	// the service stays "unknown" — no harm done.
	if srv.nodeID != "" {
		srv.peerChecksumLookup(ctx, statusReq.InstalledVersions, statusReq.InstalledBuildIds)
	}

	// Phase 3 REMOVED — repository reverse-lookup caused cascading failures.
	// Phase 2.5 (peer checksum via etcd) replaces it safely.

	if len(statusReq.InstalledVersions) > 0 {
		sample := make([]string, 0, 5)
		for k := range statusReq.InstalledVersions {
			if len(sample) < 5 {
				sample = append(sample, k)
			}
		}
		log.Printf("nodeagent: reporting %d installed services (hash=%s, sample=%v)", len(statusReq.InstalledVersions), appliedHash, sample)
	} else {
		log.Printf("nodeagent: no installed services found")
	}
	statusReq.AppliedServicesHash = appliedHash
	return srv.sendStatusWithRetry(ctx, statusReq)
}

// probeTLS moved to config.ProbeTLS — shared by all gRPC dialers.

func leaderAddrFromError(err error) string {
	st, ok := status.FromError(err)
	if !ok {
		return ""
	}
	if st.Code() != codes.FailedPrecondition {
		return ""
	}
	msg := st.Message()
	const marker = "leader_addr="
	idx := strings.Index(msg, marker)
	if idx < 0 {
		return ""
	}
	addr := msg[idx+len(marker):]
	// Stop at first comma, closing paren, or space.
	for i, ch := range addr {
		if ch == ',' || ch == ')' || ch == ' ' {
			addr = addr[:i]
			break
		}
	}
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return ""
	}
	log.Printf("node-agent: leader redirect → %s", addr)
	return addr
}

func (srv *NodeAgentServer) resetControllerClient() {
	srv.controllerConnMu.Lock()
	defer srv.controllerConnMu.Unlock()
	if srv.controllerConn != nil {
		_ = srv.controllerConn.Close()
		srv.controllerConn = nil
	}
	srv.controllerClient = nil
}

func (srv *NodeAgentServer) sendStatusWithRetry(ctx context.Context, statusReq *cluster_controllerpb.NodeStatus) error {
	if statusReq == nil {
		return errors.New("status request is nil")
	}
	if err := srv.ensureControllerClient(ctx); err != nil {
		return err
	}
	send := func() error {
		sendCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
		defer cancel()
		_, err := srv.controllerClient.ReportNodeStatus(sendCtx, &cluster_controllerpb.ReportNodeStatusRequest{
			Status: statusReq,
		})
		return err
	}
	if err := send(); err != nil {
		addr := leaderAddrFromError(err)
		if addr == "" {
			return err
		}
		// Switch to leader and retry once.
		old := srv.controllerEndpoint
		srv.controllerEndpoint = addr
		if srv.controllerClientOverride != nil {
			srv.controllerClient = srv.controllerClientOverride(addr)
		} else {
			srv.resetControllerClient()
			if errEnsure := srv.ensureControllerClient(ctx); errEnsure != nil {
				return err
			}
		}
		if err := send(); err != nil {
			return err
		}
		// Persist the redirected endpoint after a successful retry.
		if srv.state != nil {
			srv.state.ControllerEndpoint = srv.controllerEndpoint
		}
		if err := srv.saveState(); err != nil {
			log.Printf("node-agent: failed to persist redirected controller endpoint (%s -> %s): %v", old, srv.controllerEndpoint, err)
		}
		return nil
	}
	return nil
}

func (srv *NodeAgentServer) ensureControllerClient(ctx context.Context) error {
	if srv.controllerEndpoint == "" {
		return errors.New("controller endpoint is not configured")
	}
	// Canonical resolution: rewrite loopback IP literals to "localhost"
	// so the controller's DNS:localhost SAN verifies, and derive the SNI
	// from the same source. All other dialers use the same helper.
	target := config.ResolveDialTarget(srv.controllerEndpoint)
	opts, err := srv.controllerDialOptions(target)
	if err != nil {
		return err
	}
	srv.controllerConnMu.Lock()
	defer srv.controllerConnMu.Unlock()
	if srv.controllerClient != nil {
		return nil
	}
	dialCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	dialer := srv.controllerDialer
	if dialer == nil {
		dialer = grpc.DialContext
	}
	// Pre-flight TLS check: grpc.WithBlock() swallows all TLS errors
	// (expired cert, wrong CA, SAN mismatch) and returns a generic
	// "context deadline exceeded". Do a raw TLS dial first so any
	// certificate error is surfaced explicitly.
	if !srv.useInsecure {
		if tlsErr := config.ProbeTLS(target.Address); tlsErr != nil {
			return tlsErr
		}
	}
	conn, err := dialer(dialCtx, target.Address, opts...)
	if err != nil {
		return err
	}
	srv.controllerConn = conn
	factory := srv.controllerClientFactory
	if factory == nil {
		factory = cluster_controllerpb.NewClusterControllerServiceClient
	}
	srv.controllerClient = factory(conn)
	return nil
}

func (srv *NodeAgentServer) controllerDialOptions(target config.DialTarget) ([]grpc.DialOption, error) {
	if target.Address == "" {
		return nil, errors.New("controller endpoint is not configured")
	}
	opts := []grpc.DialOption{grpc.WithBlock()}
	if srv.useInsecure {
		// "Insecure" means TLS with skip-verify — NOT plaintext.
		// All services now require TLS; this mode just skips cert validation
		// (useful during Day-0 bootstrap before the CA is fully trusted).
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(&tls.Config{
			InsecureSkipVerify: true,
		})))
		return opts, nil
	}
	// Fall back to the standard cluster CA path if not explicitly configured.
	if srv.controllerCAPath == "" && !srv.controllerUseSystemRoots {
		if caFile := config.GetTLSFile("", "", "ca.crt"); caFile != "" {
			srv.controllerCAPath = caFile
		} else {
			srv.controllerUseSystemRoots = true
		}
	}
	var tlsConfig tls.Config
	if srv.controllerCAPath != "" {
		data, err := os.ReadFile(srv.controllerCAPath)
		if err != nil {
			return nil, fmt.Errorf("read controller ca: %w", err)
		}
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(data) {
			return nil, fmt.Errorf("failed to parse controller ca")
		}
		tlsConfig.RootCAs = pool
	}
	// Explicit override (NODE_AGENT_CONTROLLER_SNI) wins; otherwise use
	// the cert-valid hostname from the shared resolver.
	serverName := srv.controllerSNI
	if serverName == "" {
		serverName = target.ServerName
	}
	if serverName != "" {
		tlsConfig.ServerName = serverName
	}
	opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(&tlsConfig)))
	return opts, nil
}

// syncEtcHosts ensures /etc/hosts contains entries for all cluster nodes.
// This enables hostname-based peer discovery (e.g. ai-executor peer proxying)
// without depending on reverse DNS, which the internal DNS doesn't support.
func (srv *NodeAgentServer) syncEtcHosts(ctx context.Context) {
	if srv.controllerEndpoint == "" {
		return
	}

	// Query the cluster controller for the node list.
	target := config.ResolveDialTarget(srv.controllerEndpoint)
	opts, err := srv.controllerDialOptions(target)
	if err != nil {
		return
	}
	conn, err := grpc.NewClient(target.Address, opts...)
	if err != nil {
		return
	}
	defer conn.Close()

	client := cluster_controllerpb.NewClusterControllerServiceClient(conn)
	callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	resp, err := client.ListNodes(callCtx, &cluster_controllerpb.ListNodesRequest{})
	if err != nil {
		return
	}

	// Build the set of hostname→IP mappings from cluster nodes.
	clusterEntries := make(map[string]string) // ip -> hostname
	for _, node := range resp.GetNodes() {
		hostname := node.GetIdentity().GetHostname()
		ips := node.GetIdentity().GetIps()
		if hostname == "" || len(ips) == 0 {
			continue
		}
		// Use the first non-loopback IP.
		for _, ip := range ips {
			if ip != "127.0.0.1" && ip != "::1" {
				clusterEntries[ip] = hostname
				break
			}
		}
	}
	if len(clusterEntries) == 0 {
		return
	}

	// Read current /etc/hosts and check what's missing.
	hostsFile := "/etc/hosts"
	existing := make(map[string]bool) // IPs already in /etc/hosts
	f, err := os.Open(hostsFile)
	if err != nil {
		return
	}
	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		lines = append(lines, line)
		fields := strings.Fields(line)
		if len(fields) >= 2 && !strings.HasPrefix(strings.TrimSpace(line), "#") {
			existing[fields[0]] = true
		}
	}
	f.Close()

	// Append missing entries.
	var added []string
	// Sort IPs for deterministic order.
	ips := make([]string, 0, len(clusterEntries))
	for ip := range clusterEntries {
		ips = append(ips, ip)
	}
	sort.Strings(ips)

	for _, ip := range ips {
		hostname := clusterEntries[ip]
		if existing[ip] {
			continue
		}
		lines = append(lines, fmt.Sprintf("%s\t%s", ip, hostname))
		added = append(added, hostname)
	}

	if len(added) == 0 {
		return // nothing to add
	}

	// Write back atomically.
	content := strings.Join(lines, "\n") + "\n"
	tmp := hostsFile + ".tmp"
	if err := os.WriteFile(tmp, []byte(content), 0644); err != nil {
		return
	}
	if err := os.Rename(tmp, hostsFile); err != nil {
		os.Remove(tmp)
		return
	}
	log.Printf("nodeagent: added %d node(s) to /etc/hosts: %v", len(added), added)
}
