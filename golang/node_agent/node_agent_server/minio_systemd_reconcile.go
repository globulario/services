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

// reconcileMinioSystemdConfig ensures that:
//  1. /var/lib/globular/minio/minio.env reflects the current ObjectStoreDesiredState.
//  2. /etc/systemd/system/globular-minio.service.d/distributed.conf is present and
//     correct for distributed/multi-drive mode, or absent for standalone single-drive.
//  3. systemd daemon-reload is run if the override changed.
//  4. The per-node rendered generation is written to etcd so the topology workflow
//     can verify all pool nodes have applied the config before restarting MinIO.
//
// The node agent NEVER restarts MinIO independently. Restart is coordinated by the
// controller's objectstore.minio.apply_topology_generation workflow after all pool
// nodes have written the correct generation.
//
// Called at startup and every syncTicker interval, just like reconcileMinioContract.
func (srv *NodeAgentServer) reconcileMinioSystemdConfig(ctx context.Context) {
	state, err := config.LoadObjectStoreDesiredState(ctx)
	if err != nil {
		// etcd transient — leave whatever is on disk alone.
		return
	}
	if state == nil {
		// No objectstore config yet (pre-pool-formation) — nothing to do.
		return
	}
	if srv.nodeID == "" {
		return
	}

	// Determine this node's IP for rendering the ExecStart line.
	nodeIP := srv.nodeIP()
	if nodeIP == "" {
		log.Printf("minio-systemd: cannot determine node IP — skipping")
		return
	}

	// ── Transition-driven wipe (BEFORE writing env) ───────────────────────────
	// Always check for an approved TopologyTransition record for this generation.
	// This covers ALL destructive topology changes:
	//   - standalone → distributed
	//   - distributed → distributed (node pool change, path change, drive count change)
	// The wipe is idempotent — clearMinioSysForModeChange is a no-op when .minio.sys
	// is already absent, so calling this on every reconcile cycle is safe.
	srv.clearMinioSysIfTransitionApproved(ctx, state, nodeIP)

	// ── Render minio.env ──────────────────────────────────────────────────────

	wantEnv := config.RenderMinioEnv(state)
	if wantEnv == "" {
		return
	}

	// Read existing env for drift detection (log only — etcd desired state wins).
	existingEnv, _ := os.ReadFile(minioEnvFile)

	envChanged, err := atomicWriteIfChanged(minioEnvFile, []byte(wantEnv), 0o640)
	if err != nil {
		log.Printf("minio-systemd: write %s: %v", minioEnvFile, err)
		return
	}
	if envChanged {
		log.Printf("minio-systemd: updated %s (generation=%d)", minioEnvFile, state.Generation)
		// Log a mode-transition hint for operator visibility, but do NOT gate
		// the wipe on this — the transition record (written above) is the gate.
		wasStandalone := !strings.Contains(string(existingEnv), "https://")
		isDistributed := strings.Contains(wantEnv, "https://")
		if wasStandalone && isDistributed {
			log.Printf("minio-systemd: standalone→distributed mode transition detected (gen=%d) — wipe governed by transition record", state.Generation)
		}
	} else if len(existingEnv) > 0 {
		// On-disk content already matches desired — check for fingerprint drift
		// caused by manual edits that were silently overwritten on a prior cycle.
		desiredFP := config.RenderStateFingerprint(state)
		renderedFP, _ := srv.readRenderedFingerprint()
		if renderedFP != "" && renderedFP != desiredFP {
			log.Printf("minio-systemd: DRIFT DETECTED node=%s: rendered fingerprint %s != desired %s (generation=%d); env already matches — checking rendered_generation",
				srv.nodeID, renderedFP[:8], desiredFP[:8], state.Generation)
		}
	}

	// ── Render distributed.conf (systemd override) ────────────────────────────

	wantOverride, needsOverride := config.RenderMinioSystemdOverride(state, nodeIP)

	var overrideChanged bool
	if needsOverride {
		overrideChanged, err = atomicWriteIfChanged(minioOverrideFile, []byte(wantOverride), 0o644)
		if err != nil {
			log.Printf("minio-systemd: write %s: %v", minioOverrideFile, err)
			return
		}
		if overrideChanged {
			log.Printf("minio-systemd: updated %s (generation=%d)", minioOverrideFile, state.Generation)
		}
	} else {
		// Standalone single-drive: remove any stale override left from a previous
		// distributed topology so systemd uses the service's own ExecStart.
		if removed, err := removeIfExists(minioOverrideFile); err != nil {
			log.Printf("minio-systemd: remove stale %s: %v", minioOverrideFile, err)
		} else if removed {
			overrideChanged = true
			log.Printf("minio-systemd: removed stale override (now standalone)")
		}
	}

	// daemon-reload required when the systemd override changed.
	if overrideChanged {
		if err := runDaemonReload(); err != nil {
			log.Printf("minio-systemd: daemon-reload failed: %v", err)
			// Non-fatal: write the rendered generation anyway; the topology
			// workflow will verify MinIO comes up correctly before completing.
		}
	}

	// ── Report rendered generation to etcd ───────────────────────────────────
	// Only update the etcd record when the on-disk content now matches the desired
	// generation (i.e. we wrote the right content, even if it was already present).

	if err := srv.writeRenderedGeneration(ctx, state); err != nil {
		log.Printf("minio-systemd: record rendered generation %d: %v", state.Generation, err)
	}

	_ = envChanged // used implicitly through log above
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
