package substrate

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// Substrate units and paths. These are the fixed Day-0 substrate surface —
// the two units whose accidental stop caused the 2026-07-10 quorum collapse.
const (
	EtcdUnit      = "globular-etcd.service"
	NodeAgentUnit = "globular-node-agent.service"

	EtcdDataDir    = "/var/lib/globular/etcd"
	EtcdConfigPath = "/var/lib/globular/config/etcd.yaml"

	survivorUnit = "globular-etcd-survivor-recovery"
)

// UnitReport describes what rung-1 recovery did for one unit.
type UnitReport struct {
	Unit   string
	Action string // already-active | started | refused | failed
	Detail string
}

// RestartMembers is rung 1: start the local substrate units that are stopped
// but were previously installed and admitted. This is the typed
// EMERGENCY_RESTORE operation — its preconditions make it safe to run
// without cluster quorum:
//
//   - the unit file exists (the unit existed before the incident);
//   - for etcd, the data directory is non-empty (this is an EXISTING member
//     restarting with its own identity — never a fresh bootstrap, which
//     could fork a second cluster);
//   - no configuration mutation, no membership mutation: systemctl start only.
//
// It waits up to wait per unit for the unit to report active.
func RestartMembers(ctx context.Context, wait time.Duration) ([]UnitReport, error) {
	var reports []UnitReport
	for _, unit := range []string{EtcdUnit, NodeAgentUnit} {
		if unitActive(unit) {
			reports = append(reports, UnitReport{Unit: unit, Action: "already-active"})
			continue
		}
		if !unitFileExists(unit) {
			reports = append(reports, UnitReport{Unit: unit, Action: "refused",
				Detail: "unit file not installed — this node never ran it; rung 1 does not install anything"})
			continue
		}
		if unit == EtcdUnit && !dirNonEmpty(EtcdDataDir) {
			reports = append(reports, UnitReport{Unit: unit, Action: "refused",
				Detail: fmt.Sprintf("%s is empty or absent — starting etcd without member data is a fresh bootstrap, not a member restart; use the join or clean+join path instead", EtcdDataDir)})
			continue
		}
		if err := runCmd(ctx, "systemctl", "start", unit); err != nil {
			reports = append(reports, UnitReport{Unit: unit, Action: "failed", Detail: err.Error()})
			continue
		}
		if err := waitUnitActive(ctx, unit, wait); err != nil {
			reports = append(reports, UnitReport{Unit: unit, Action: "failed",
				Detail: fmt.Sprintf("started but not active after %s: %v — check journalctl -u %s", wait, err, unit)})
			continue
		}
		reports = append(reports, UnitReport{Unit: unit, Action: "started"})
	}
	for _, r := range reports {
		if r.Action == "failed" {
			return reports, fmt.Errorf("rung-1 recovery incomplete: %s %s (%s)", r.Unit, r.Action, r.Detail)
		}
	}
	return reports, nil
}

// SurvivorOptions parameterizes rung-2 recovery. The etcd-touching probes are
// injected so the orchestration stays testable and the package keeps a single
// systemd surface.
type SurvivorOptions struct {
	// TakeDump captures a classified dump from the (quorum-less) local member
	// before anything is mutated. Its failure aborts unless Force.
	TakeDump func(ctx context.Context) (string, error)
	// ProbeHealthy returns nil once the local etcd answers a linearizable
	// read — the proof that quorum exists again (single-voter after
	// force-new-cluster).
	ProbeHealthy func(ctx context.Context) error
	// WriteMarker records the recovery receipt once etcd is healthy again.
	WriteMarker func(ctx context.Context) error
	// Force proceeds past a failed pre-mutation dump.
	Force bool
	// BackupDir receives the pre-mutation copy of the etcd data directory.
	// Empty selects the data dir's parent.
	BackupDir string
}

// FromSurvivor is rung 2: this node holds the only surviving copy of the
// coordination state; the rest of the etcd membership is unrecoverable.
// Sequence:
//
//	dump (evidence first) → stop etcd → copy data dir aside →
//	start etcd once with force-new-cluster (drops dead members, keeps ALL
//	data) → prove single-voter quorum → restart the normal unit →
//	write the RESTORED_UNVERIFIED marker.
//
// force-new-cluster mutates raft membership in place; the data-dir copy is
// the manual rollback. Historical raft identity is not preserved — per the
// substrate contract, a fresh coordination cluster seeded from surviving
// state is the goal, not resurrection of the old cluster's ghost.
func FromSurvivor(ctx context.Context, opts SurvivorOptions) error {
	if !unitFileExists(EtcdUnit) {
		return fmt.Errorf("%s is not installed on this node — not a member, cannot be a survivor", EtcdUnit)
	}
	if !dirNonEmpty(EtcdDataDir) {
		return fmt.Errorf("%s is empty or absent — this node has no surviving state; use from-dump on a fresh cluster instead", EtcdDataDir)
	}

	// 1. Evidence first: dump before any mutation.
	if opts.TakeDump != nil {
		if path, err := opts.TakeDump(ctx); err != nil {
			if !opts.Force {
				return fmt.Errorf("pre-recovery dump failed: %w — refusing to mutate the only surviving copy without captured evidence (override requires --force)", err)
			}
			fmt.Fprintf(os.Stderr, "WARN: pre-recovery dump failed (%v) — continuing under --force\n", err)
		} else {
			fmt.Printf("pre-recovery dump written: %s\n", path)
		}
	}

	// 2. Stop the member.
	if err := runCmd(ctx, "systemctl", "stop", EtcdUnit); err != nil {
		return fmt.Errorf("stop %s: %w", EtcdUnit, err)
	}

	// 3. Copy the data dir aside — the rollback for everything after this.
	backupDir := opts.BackupDir
	if backupDir == "" {
		backupDir = filepath.Dir(EtcdDataDir)
	}
	backup := filepath.Join(backupDir, fmt.Sprintf("etcd.survivor-bak.%s", time.Now().UTC().Format("20060102T150405Z")))
	if err := runCmd(ctx, "cp", "-a", EtcdDataDir, backup); err != nil {
		_ = runCmd(ctx, "systemctl", "start", EtcdUnit) // compensate: nothing was mutated yet
		return fmt.Errorf("data-dir backup to %s failed: %w — etcd restarted, nothing mutated", backup, err)
	}
	fmt.Printf("data dir copied to %s (manual rollback point)\n", backup)

	// 4. One-shot force-new-cluster run under a transient unit with the same
	// runtime identity as the real unit.
	cfg, err := forceNewClusterConfig(EtcdConfigPath)
	if err != nil {
		return err
	}
	binPath, user := unitExecPath(EtcdUnit), unitUser(EtcdUnit)
	args := []string{"--collect", "--unit=" + survivorUnit}
	if user != "" {
		args = append(args, "--property=User="+user)
	}
	args = append(args, binPath, "--config-file", cfg)
	if err := runCmd(ctx, "systemd-run", args...); err != nil {
		return fmt.Errorf("start force-new-cluster etcd: %w (data-dir backup: %s)", err, backup)
	}

	// 5. Prove quorum (single voter) with a linearizable read.
	probeCtx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	err = pollUntil(probeCtx, 3*time.Second, opts.ProbeHealthy)
	cancel()
	if err != nil {
		_ = runCmd(ctx, "systemctl", "stop", survivorUnit)
		return fmt.Errorf("force-new-cluster etcd never became healthy: %w — transient unit stopped; membership may already be rewritten, rollback copy at %s", err, backup)
	}

	// 6. Hand back to the normal unit.
	if err := runCmd(ctx, "systemctl", "stop", survivorUnit); err != nil {
		return fmt.Errorf("stop transient unit: %w", err)
	}
	if err := runCmd(ctx, "systemctl", "start", EtcdUnit); err != nil {
		return fmt.Errorf("restart %s after force-new-cluster: %w (rollback copy: %s)", EtcdUnit, err, backup)
	}
	probeCtx, cancel = context.WithTimeout(ctx, time.Minute)
	err = pollUntil(probeCtx, 3*time.Second, opts.ProbeHealthy)
	cancel()
	if err != nil {
		return fmt.Errorf("%s not healthy after force-new-cluster handback: %w (rollback copy: %s)", EtcdUnit, err, backup)
	}

	// 7. Receipt: restored state is evidence, not authority, until verified.
	if opts.WriteMarker != nil {
		if err := opts.WriteMarker(ctx); err != nil {
			return fmt.Errorf("etcd recovered but marker write failed: %w — write it manually before resuming convergence", err)
		}
	}
	return nil
}

// forceNewClusterConfig writes a sibling copy of the etcd config with
// force-new-cluster enabled and returns its path. The original config is
// never modified — the flag must not survive into normal restarts.
func forceNewClusterConfig(srcPath string) (string, error) {
	src, err := os.ReadFile(srcPath)
	if err != nil {
		return "", fmt.Errorf("read %s: %w", srcPath, err)
	}
	out, err := appendForceNewCluster(src)
	if err != nil {
		return "", err
	}
	dst := strings.TrimSuffix(srcPath, ".yaml") + "-force-new-cluster.yaml"
	if err := os.WriteFile(dst, out, 0o640); err != nil {
		return "", fmt.Errorf("write %s: %w", dst, err)
	}
	if st, err := os.Stat(srcPath); err == nil {
		if sys, ok := sysOwner(st); ok {
			_ = os.Chown(dst, sys.uid, sys.gid)
		}
	}
	return dst, nil
}

// appendForceNewCluster is the pure transformation: refuse configs that
// already force a new cluster (double-forcing suggests a botched prior run
// that a human must look at), otherwise append the one-shot flag.
func appendForceNewCluster(src []byte) ([]byte, error) {
	if strings.Contains(string(src), "force-new-cluster") {
		return nil, fmt.Errorf("etcd config already contains force-new-cluster — a prior recovery attempt did not clean up; inspect before retrying")
	}
	out := strings.TrimRight(string(src), "\n") + "\nforce-new-cluster: true\n"
	return []byte(out), nil
}

// ── systemd / process helpers ────────────────────────────────────────────────

func unitActive(unit string) bool {
	return exec.Command("systemctl", "is-active", "--quiet", unit).Run() == nil
}

func unitFileExists(unit string) bool {
	out, err := exec.Command("systemctl", "show", "-p", "FragmentPath", "--value", unit).Output()
	return err == nil && strings.TrimSpace(string(out)) != ""
}

func dirNonEmpty(dir string) bool {
	entries, err := os.ReadDir(dir)
	return err == nil && len(entries) > 0
}

func waitUnitActive(ctx context.Context, unit string, wait time.Duration) error {
	deadline := time.Now().Add(wait)
	for time.Now().Before(deadline) {
		if unitActive(unit) {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(time.Second):
		}
	}
	return fmt.Errorf("unit %s not active", unit)
}

func pollUntil(ctx context.Context, interval time.Duration, probe func(ctx context.Context) error) error {
	if probe == nil {
		return fmt.Errorf("no health probe provided")
	}
	var last error
	for {
		if last = probe(ctx); last == nil {
			return nil
		}
		select {
		case <-ctx.Done():
			return fmt.Errorf("%w (last probe error: %v)", ctx.Err(), last)
		case <-time.After(interval):
		}
	}
}

func runCmd(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("%s %s: %v: %s", name, strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return nil
}

// unitExecPath extracts the binary path from the unit's ExecStart, falling
// back to the canonical install location.
func unitExecPath(unit string) string {
	out, err := exec.Command("systemctl", "show", "-p", "ExecStart", "--value", unit).Output()
	if err == nil {
		// Format: { path=/usr/lib/globular/bin/etcd ; argv[]=... }
		for _, field := range strings.Fields(string(out)) {
			if strings.HasPrefix(field, "path=") {
				return strings.TrimPrefix(field, "path=")
			}
		}
	}
	return "/usr/lib/globular/bin/etcd"
}

// unitUser returns the User= of a unit ("" for root/unset).
func unitUser(unit string) string {
	out, err := exec.Command("systemctl", "show", "-p", "User", "--value", unit).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

type ownerIDs struct{ uid, gid int }

// sysOwner extracts uid/gid so the force-new-cluster config copy carries the
// same ownership as the original (etcd runs unprivileged and must read it).
func sysOwner(st os.FileInfo) (ownerIDs, bool) {
	if sys, ok := st.Sys().(*syscall.Stat_t); ok {
		return ownerIDs{uid: int(sys.Uid), gid: int(sys.Gid)}, true
	}
	return ownerIDs{}, false
}
