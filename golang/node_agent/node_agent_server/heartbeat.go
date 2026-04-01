package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/installed_state"
	"github.com/globulario/services/golang/versionutil"
	"github.com/globulario/services/golang/repository/repository_client"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
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
	// Initial sync: populate installed-state etcd records for packages
	// installed by the Day-0 installer (which doesn't go through plan execution).
	srv.syncInstalledStateToEtcd(ctx)

	heartbeat := time.NewTicker(30 * time.Second)
	defer heartbeat.Stop()

	// Re-sync installed state every 5 minutes so late-arriving packages
	// (e.g. repository not ready at first boot) eventually land in etcd.
	syncTicker := time.NewTicker(5 * time.Minute)
	defer syncTicker.Stop()

	for {
		if err := srv.reportStatus(ctx); err != nil {
			log.Printf("node heartbeat failed: %v", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-syncTicker.C:
			srv.syncInstalledStateToEtcd(ctx)
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
			kind := "SERVICE"
			if isDay0JoinInfra(name) {
				kind = "INFRASTRUCTURE"
			}
			existing, _ := installed_state.GetInstalledPackage(ctx, srv.nodeID, kind, name)
			if existing != nil {
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

	if synced > 0 {
		log.Printf("nodeagent: synced %d installed-state records to etcd", synced)
	}
}

// syncRepoArtifactsToEtcd queries the repository for all published artifacts
// and enriches existing installed-state records with correct version and kind
// from the repo. It does NOT create new records for packages that weren't
// discovered locally by Phase 1 — only Phase 1 (systemd/markers/config) knows
// what's actually installed on this machine. Phase 2 only:
//   - Updates version for existing records that have the fallback "0.0.1"
//   - Corrects the kind (e.g. SERVICE → INFRASTRUCTURE) for misclassified records
//   - Creates records ONLY for INFRASTRUCTURE/COMMAND packages that have a
//     matching systemd unit running (verified via systemctl)
func (srv *NodeAgentServer) syncRepoArtifactsToEtcd(ctx context.Context, now int64, platform string, synced *int) {
	repoAddr := strings.TrimSpace(os.Getenv("REPOSITORY_ADDRESS"))
	if repoAddr == "" {
		repoAddr = discoverServiceAddr(10008) // repository.PackageRepository default port
	}

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

		if existing != nil && existing.GetVersion() != "0.0.1" {
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

func (srv *NodeAgentServer) reportStatus(ctx context.Context) error {
	if srv.controllerEndpoint == "" {
		return nil
	}
	if srv.nodeID == "" {
		return nil
	}
	identity := buildNodeIdentity()
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

	// Phase 1: local discovery (systemd units, version markers, config files).
	for key, info := range installed {
		statusReq.InstalledVersions[key.String()] = info.Version
	}

	// Phase 2: merge from etcd installed_state registry (canonical source of
	// truth). This covers packages synced by syncRepoArtifactsToEtcd (Phase 2)
	// which may not have local markers or config files (e.g. infrastructure
	// packages installed by the spec-based installer on Day-0).
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
				// etcd version takes precedence over local 0.0.1 fallback.
				if existing, ok := statusReq.InstalledVersions[canon]; !ok || existing == "0.0.1" {
					statusReq.InstalledVersions[canon] = pkg.GetVersion()
				}
			}
		}
	}

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
	addr := strings.TrimSpace(msg[idx+len(marker):])
	addr = strings.Trim(addr, ")")
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
		srv.controllerEndpoint = addr
		if srv.controllerClientOverride != nil {
			srv.controllerClient = srv.controllerClientOverride(addr)
		} else {
			srv.resetControllerClient()
			if errEnsure := srv.ensureControllerClient(ctx); errEnsure != nil {
				return err
			}
		}
		return send()
	}
	return nil
}

func (srv *NodeAgentServer) ensureControllerClient(ctx context.Context) error {
	if srv.controllerEndpoint == "" {
		return errors.New("controller endpoint is not configured")
	}
	opts, err := srv.controllerDialOptions()
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
	conn, err := dialer(dialCtx, srv.controllerEndpoint, opts...)
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

func (srv *NodeAgentServer) controllerDialOptions() ([]grpc.DialOption, error) {
	if srv.controllerEndpoint == "" {
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
	serverName := srv.controllerSNI
	if serverName == "" {
		if host, _, err := net.SplitHostPort(srv.controllerEndpoint); err == nil {
			serverName = host
		} else {
			serverName = srv.controllerEndpoint
		}
	}
	if serverName != "" {
		tlsConfig.ServerName = serverName
	}
	opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(&tlsConfig)))
	return opts, nil
}
