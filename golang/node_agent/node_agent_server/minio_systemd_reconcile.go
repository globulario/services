package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
)

const (
	minioEnvFile      = "/var/lib/globular/minio/minio.env"
	minioOverrideFile = "/etc/systemd/system/globular-minio.service.d/distributed.conf"
)

// reconcileMinioSystemdConfig is the top-level MinIO local contract enforcer.
// It runs at startup and on every syncTicker interval.
//
// Separation of concerns:
//   - Package installed != runtime authorized. ObjectStoreDesiredState.Nodes is
//     the runtime allow-list; topology changes go through apply-topology workflow.
//   - TopologyTransition authorizes destructive local cleanup (.minio.sys wipe).
//   - Workflow coordinates cluster-level stop/start and health verification.
//   - Node-agent owns local enforcement: hold non-members, wipe on transition,
//     render files, reload daemon, report rendered state.
//   - Node-agent NEVER restarts MinIO independently.
func (srv *NodeAgentServer) reconcileMinioSystemdConfig(ctx context.Context) {
	// 1. Load desired state — bail on transient etcd errors or pre-pool state.
	state, ok := srv.loadMinioDesiredState(ctx)
	if !ok {
		return
	}

	// 2. Resolve this node's IP — required for pool check and file rendering.
	nodeIP, err := srv.resolveMinioNodeIP()
	if err != nil {
		log.Printf("minio-systemd: %v — skipping", err)
		return
	}

	// 3. Enforce runtime membership.
	//    Non-members: stop the service (no data wipe), skip all rendering.
	if allowed := srv.enforceMinioRuntimeMembership(ctx, state, nodeIP); !allowed {
		return
	}

	// 4. Apply approved destructive transition before rendering new config.
	//    TopologyTransition record gates .minio.sys wipe — no local inference.
	srv.applyApprovedMinioTransition(ctx, state, nodeIP)

	// 5. Render minio.env and distributed.conf; returns whether daemon-reload needed.
	daemonReloadNeeded, err := srv.renderMinioSystemdFiles(ctx, state, nodeIP)
	if err != nil {
		log.Printf("minio-systemd: render files: %v", err)
		return
	}

	// 6. Reload daemon when the systemd override changed.
	if daemonReloadNeeded {
		if err := runDaemonReload(); err != nil {
			log.Printf("minio-systemd: daemon-reload failed: %v", err)
			// Non-fatal: record generation anyway; workflow verifies MinIO health.
		}
	}

	// 7. Report rendered generation + fingerprint to etcd.
	//    Workflow reads both to gate the coordinated restart.
	if err := srv.recordMinioRenderedState(ctx, state); err != nil {
		log.Printf("minio-systemd: record rendered generation %d: %v", state.Generation, err)
	}
}

// ── phase functions ───────────────────────────────────────────────────────────

// loadMinioDesiredState returns the current desired state (ok=true) or signals
// that reconciliation should be skipped (ok=false) without logging transient errors.
func (srv *NodeAgentServer) loadMinioDesiredState(ctx context.Context) (*config.ObjectStoreDesiredState, bool) {
	if srv.nodeID == "" {
		return nil, false
	}
	state, err := config.LoadObjectStoreDesiredState(ctx)
	if err != nil {
		return nil, false // etcd transient — leave whatever is on disk alone
	}
	if state == nil {
		return nil, false // pre-pool-formation — nothing to do
	}
	return state, true
}

// resolveMinioNodeIP returns the routable IP for this node.
// Returns an error if the IP cannot be determined.
func (srv *NodeAgentServer) resolveMinioNodeIP() (string, error) {
	ip := srv.nodeIP()
	if ip == "" {
		return "", fmt.Errorf("cannot determine node IP")
	}
	return ip, nil
}

// enforceMinioRuntimeMembership checks pool admission and enforces the hold.
//
//   - Package installed != runtime authorized.
//   - ObjectStoreDesiredState.Nodes is the runtime allow-list.
//
// Returns true when the node is admitted and rendering may proceed.
// Returns false when the node is not in the pool; enforceMinioHeld is called
// to stop the service if active (no data wipe).
func (srv *NodeAgentServer) enforceMinioRuntimeMembership(ctx context.Context, state *config.ObjectStoreDesiredState, nodeIP string) bool {
	if nodeIPInPool(nodeIP, state) {
		return true
	}
	srv.enforceMinioHeld(ctx, nodeIP, state.Generation)
	return false
}

// applyApprovedMinioTransition executes an approved destructive topology cleanup.
//
// TopologyTransition authorizes destructive local cleanup (.minio.sys wipe).
// No wipe happens without an explicit controller-written transition record.
// The wipe is idempotent — a no-op when .minio.sys is already absent.
func (srv *NodeAgentServer) applyApprovedMinioTransition(ctx context.Context, state *config.ObjectStoreDesiredState, nodeIP string) {
	srv.clearMinioSysIfTransitionApproved(ctx, state, nodeIP)
}

// renderMinioSystemdFiles writes minio.env and the distributed.conf override.
// Returns (daemonReloadNeeded, error).
//
// Workflow coordinates cluster-level restart; node-agent owns local file rendering.
func (srv *NodeAgentServer) renderMinioSystemdFiles(ctx context.Context, state *config.ObjectStoreDesiredState, nodeIP string) (bool, error) {
	// ── minio.env ─────────────────────────────────────────────────────────────
	wantEnv := config.RenderMinioEnv(state)
	if wantEnv == "" {
		return false, fmt.Errorf("RenderMinioEnv returned empty (state gen=%d)", state.Generation)
	}

	existingEnv, _ := os.ReadFile(minioEnvFile)

	envChanged, err := atomicWriteIfChanged(minioEnvFile, []byte(wantEnv), 0o640)
	if err != nil {
		return false, fmt.Errorf("write %s: %w", minioEnvFile, err)
	}
	if envChanged {
		log.Printf("minio-systemd: updated %s (generation=%d)", minioEnvFile, state.Generation)
		wasStandalone := !strings.Contains(string(existingEnv), "https://")
		isDistributed := strings.Contains(wantEnv, "https://")
		if wasStandalone && isDistributed {
			log.Printf("minio-systemd: standalone→distributed mode transition detected (gen=%d) — wipe governed by transition record", state.Generation)
		}
	} else if len(existingEnv) > 0 {
		srv.logFingerprintDrift(state)
	}

	// ── distributed.conf (systemd override) ──────────────────────────────────
	wantOverride, needsOverride := config.RenderMinioSystemdOverride(state, nodeIP)
	var overrideChanged bool

	if needsOverride {
		overrideChanged, err = atomicWriteIfChanged(minioOverrideFile, []byte(wantOverride), 0o644)
		if err != nil {
			return false, fmt.Errorf("write %s: %w", minioOverrideFile, err)
		}
		if overrideChanged {
			log.Printf("minio-systemd: updated %s (generation=%d)", minioOverrideFile, state.Generation)
		}
	} else {
		// Standalone: remove any stale override so systemd uses the service's own ExecStart.
		if removed, err := removeIfExists(minioOverrideFile); err != nil {
			log.Printf("minio-systemd: remove stale %s: %v", minioOverrideFile, err)
		} else if removed {
			overrideChanged = true
			log.Printf("minio-systemd: removed stale override (now standalone)")
		}
	}

	return overrideChanged, nil
}

// recordMinioRenderedState writes rendered_generation and state_fingerprint to etcd.
// Workflow reads both to gate the coordinated restart.
func (srv *NodeAgentServer) recordMinioRenderedState(ctx context.Context, state *config.ObjectStoreDesiredState) error {
	return srv.writeRenderedGeneration(ctx, state)
}

// logFingerprintDrift logs a DRIFT DETECTED warning when the in-etcd rendered
// fingerprint diverges from the current desired state fingerprint while the
// on-disk env file already matches. This detects silent overwrites from manual edits.
func (srv *NodeAgentServer) logFingerprintDrift(state *config.ObjectStoreDesiredState) {
	desiredFP := config.RenderStateFingerprint(state)
	renderedFP, _ := srv.readRenderedFingerprint()
	if renderedFP != "" && renderedFP != desiredFP {
		log.Printf("minio-systemd: DRIFT DETECTED node=%s: rendered fingerprint %s != desired %s (generation=%d); env already matches — checking rendered_generation",
			srv.nodeID, renderedFP[:8], desiredFP[:8], state.Generation)
	}
}

// readRenderedFingerprint reads the last rendered state fingerprint for this
// node from etcd. Returns ("", nil) when no fingerprint has been written yet.
func (srv *NodeAgentServer) readRenderedFingerprint() (string, error) {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return "", err
	}
	key := config.EtcdKeyNodeRenderedStateFingerprint(srv.nodeID)
	resp, err := cli.Get(context.Background(), key)
	if err != nil {
		return "", err
	}
	if len(resp.Kvs) == 0 {
		return "", nil
	}
	return string(resp.Kvs[0].Value), nil
}

// writeRenderedGeneration records the last generation this node successfully
// rendered to etcd, together with the state fingerprint. The topology workflow
// reads both to gate the coordinated restart: all pool nodes must have rendered
// the same generation AND the same topology fingerprint before MinIO is restarted.
func (srv *NodeAgentServer) writeRenderedGeneration(ctx context.Context, state *config.ObjectStoreDesiredState) error {
	cli, err := config.GetEtcdClient()
	if err != nil {
		return fmt.Errorf("etcd unavailable: %w", err)
	}

	genKey := config.EtcdKeyNodeRenderedGeneration(srv.nodeID)
	genVal := strconv.FormatInt(state.Generation, 10)
	if _, err := cli.Put(ctx, genKey, genVal); err != nil {
		return fmt.Errorf("put %s: %w", genKey, err)
	}

	fpKey := config.EtcdKeyNodeRenderedStateFingerprint(srv.nodeID)
	fpVal := config.RenderStateFingerprint(state)
	if _, err := cli.Put(ctx, fpKey, fpVal); err != nil {
		return fmt.Errorf("put %s: %w", fpKey, err)
	}
	return nil
}

// nodeIP returns the routable IP of this node. It is used to render the
// ExecStart address in the systemd override.
func (srv *NodeAgentServer) nodeIP() string {
	if srv.state != nil && srv.state.AdvertiseIP != "" {
		return srv.state.AdvertiseIP
	}
	return nodeRoutableIP()
}

// ── file helpers ─────────────────────────────────────────────────────────────

// atomicWriteIfChanged writes content to path via tempfile+rename only when
// the existing file content differs. Returns (true, nil) if a write occurred.
func atomicWriteIfChanged(path string, content []byte, perm os.FileMode) (bool, error) {
	existing, err := os.ReadFile(path)
	if err == nil && string(existing) == string(content) {
		return false, nil // already correct — no write needed
	}

	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return false, err
	}

	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".*")
	if err != nil {
		return false, err
	}
	tmpPath := tmp.Name()
	committed := false
	defer func() {
		if !committed {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		return false, err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return false, err
	}
	if err := tmp.Close(); err != nil {
		return false, err
	}
	if err := os.Chmod(tmpPath, perm); err != nil {
		return false, err
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return false, err
	}
	committed = true
	return true, nil
}

// removeIfExists removes path if it exists. Returns (true, nil) if removed.
func removeIfExists(path string) (bool, error) {
	err := os.Remove(path)
	if os.IsNotExist(err) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// transitionAuthorizesWipe returns (true, reason) when the transition record
// explicitly authorizes wiping .minio.sys on nodeIP for this desired state.
// Returns (false, reason) when the wipe must be skipped.
// This is a pure function (no etcd I/O) so it can be tested without a live cluster.
func transitionAuthorizesWipe(transition *config.TopologyTransition, state *config.ObjectStoreDesiredState, nodeIP string) (bool, string) {
	if transition == nil {
		return false, fmt.Sprintf("no transition record for gen=%d", state.Generation)
	}
	if transition.Generation != state.Generation {
		return false, fmt.Sprintf("transition gen=%d != desired gen=%d (stale record)", transition.Generation, state.Generation)
	}
	if !transition.IsDestructive {
		return false, fmt.Sprintf("transition gen=%d is not destructive", state.Generation)
	}
	if !transition.Approved {
		return false, fmt.Sprintf("transition gen=%d not approved by operator", state.Generation)
	}

	// Node must be in the affected-path wipe plan.
	expectedPath, inPlan := transition.AffectedPaths[nodeIP]
	if !inPlan {
		return false, fmt.Sprintf("node %s not in wipe plan for gen=%d", nodeIP, state.Generation)
	}

	// Cross-check: transition path must match current desired state path.
	if state.NodePaths != nil {
		if desiredPath, ok := state.NodePaths[nodeIP]; ok && desiredPath != expectedPath {
			return false, fmt.Sprintf("path mismatch for node %s: transition=%q desired=%q (stale record?)", nodeIP, expectedPath, desiredPath)
		}
	}

	return true, fmt.Sprintf("approved destructive transition gen=%d node=%s path=%s", state.Generation, nodeIP, expectedPath)
}

// clearMinioSysIfTransitionApproved wipes .minio.sys only when the controller
// has recorded an approved destructive TopologyTransition for the desired
// generation AND this node/path is in the wipe plan. This prevents any local
// inference from triggering a wipe without explicit operator approval.
func (srv *NodeAgentServer) clearMinioSysIfTransitionApproved(ctx context.Context, state *config.ObjectStoreDesiredState, nodeIP string) {
	tCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	transition, err := config.LoadTopologyTransition(tCtx, state.Generation)
	if err != nil {
		log.Printf("minio-transition: could not load transition record gen=%d: %v — skipping .minio.sys wipe", state.Generation, err)
		return
	}

	authorized, reason := transitionAuthorizesWipe(transition, state, nodeIP)
	if !authorized {
		// Only log at non-trivial cases (nil transition on gen=0 is normal startup noise).
		if transition != nil || state.Generation > 0 {
			log.Printf("minio-transition: wipe not authorized — %s", reason)
		}
		return
	}

	log.Printf("minio-transition: %s — proceeding with .minio.sys wipe", reason)
	srv.clearMinioSysForModeChange(state, nodeIP)
}

// clearMinioSysForModeChange removes the .minio.sys directory from every data
// drive on this node. This is necessary when transitioning from standalone to
// distributed mode: standalone MinIO writes format.json with erasure-set
// size=1; distributed mode requires size≥2 and refuses to start on a mismatch.
//
// DATA LOSS NOTE: this wipes MinIO's internal metadata. All objects stored
// while in standalone mode are lost. After a mode transition the operator
// must re-publish all artifacts via `globular pkg publish`.
func (srv *NodeAgentServer) clearMinioSysForModeChange(state *config.ObjectStoreDesiredState, nodeIP string) {
	basePath := "/var/lib/globular/minio"
	if state.NodePaths != nil {
		if p, ok := state.NodePaths[nodeIP]; ok && p != "" {
			basePath = strings.TrimRight(p, "/")
		}
	}

	var dataDirs []string
	if state.DrivesPerNode < 2 {
		dataDirs = []string{filepath.Join(basePath, "data")}
	} else {
		for d := 1; d <= state.DrivesPerNode; d++ {
			dataDirs = append(dataDirs, filepath.Join(basePath, fmt.Sprintf("data%d", d)))
		}
	}

	for _, dir := range dataDirs {
		minioSys := filepath.Join(dir, ".minio.sys")
		if _, err := os.Stat(minioSys); os.IsNotExist(err) {
			continue
		}
		log.Printf("minio-systemd: NOTICE: clearing %s — approved destructive topology transition (objects lost; re-publish required)", minioSys)
		if err := os.RemoveAll(minioSys); err != nil {
			log.Printf("minio-systemd: ERROR: failed to clear %s: %v", minioSys, err)
		} else {
			log.Printf("minio-systemd: cleared %s — MinIO will reinitialise on next start", minioSys)
		}
	}
}

// runDaemonReload runs systemctl daemon-reload to pick up the new override.
func runDaemonReload() error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, "systemctl", "daemon-reload").Run()
}
