package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/globulario/services/golang/backup_manager/backup_managerpb"
)

// RecoveryInputs captures every parameter needed to restore a cluster from this backup.
// Serialised to recovery-capsule/restore-inputs.json inside every artifact capsule.
type RecoveryInputs struct {
	BackupID  string    `json:"backup_id"`
	CreatedAt time.Time `json:"created_at"`
	Domain    string    `json:"domain"`

	Etcd   *EtcdRestoreInputs   `json:"etcd,omitempty"`
	Restic *ResticRestoreInputs `json:"restic,omitempty"`
	Scylla *ScyllaRestoreInputs `json:"scylla,omitempty"`
	Minio  *MinioRestoreInputs  `json:"minio,omitempty"`
}

// EtcdRestoreInputs describes the etcd snapshot location and target data directory.
type EtcdRestoreInputs struct {
	// SnapshotFile is relative to the artifact root (e.g. "payload/etcd/etcd-snapshot.db").
	SnapshotFile string `json:"snapshot_file"`
	DataDir      string `json:"data_dir"`
	Endpoints    string `json:"endpoints,omitempty"`
}

// ResticRestoreInputs describes the restic repository and snapshot to restore from.
// The restic password is NOT stored here. The restore script reads RESTIC_PASSWORD from
// the environment or from the password file hint.
type ResticRestoreInputs struct {
	Repo            string `json:"repo"`
	SnapshotID      string `json:"snapshot_id"`
	Paths           string `json:"paths"`
	PasswordFileHint string `json:"password_file_hint"`
}

// ScyllaRestoreInputs describes the sctool restore parameters.
type ScyllaRestoreInputs struct {
	Cluster     string `json:"cluster"`
	SnapshotTag string `json:"snapshot_tag"`
	Locations   string `json:"locations"`
	TaskID      string `json:"task_id,omitempty"`
	APIURL      string `json:"api_url,omitempty"`
}

// MinioRestoreInputs describes where MinIO stores its data and its agent config path.
type MinioRestoreInputs struct {
	// Endpoint as stored in service config (host:port without scheme).
	Endpoint string `json:"endpoint"`
	// DataPath is the directory restic restores MinIO objects to.
	// Typically /var/lib/globular/minio/data.
	DataPath string `json:"data_path"`
	// AgentConfig is the path to scylla-manager-agent.yaml on each node.
	AgentConfig string `json:"agent_config"`
}

// generateRecoveryCapsule creates the recovery-capsule/ directory inside the artifact.
// It is called after all backup providers have succeeded, before the capsule is sealed
// and replicated. The result is included in every destination the capsule is synced to.
//
// The directory layout produced:
//
//	recovery-capsule/
//	  restore-inputs.json          machine-readable restore parameters
//	  restore.sh                   full-cluster restore orchestrator (all phases)
//	  phase1-restore-files.sh      restore /var/lib/globular via restic (incl. MinIO data)
//	  phase2-bootstrap-minio.sh    (re)start MinIO once files are restored
//	  phase3-restore-etcd.sh       restore etcd from the embedded snapshot
//	  phase4-restore-scylla.sh     restore ScyllaDB via sctool (needs MinIO running)
//	  README.md                    human-readable recovery guide
//
// The etcd snapshot is NOT copied; it already lives at payload/etcd/etcd-snapshot.db
// relative to the artifact root. All scripts address it via that relative path.
func (srv *server) generateRecoveryCapsule(
	_ context.Context,
	backupID string,
	results []*backup_managerpb.BackupProviderResult,
) error {
	capsuleDir := srv.CapsuleDir(backupID)
	recoveryDir := filepath.Join(capsuleDir, "recovery-capsule")

	if err := os.MkdirAll(recoveryDir, 0755); err != nil {
		return fmt.Errorf("create recovery-capsule dir: %w", err)
	}

	inputs := srv.buildRecoveryInputs(backupID, results)

	// Write restore-inputs.json
	data, err := json.MarshalIndent(inputs, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal recovery inputs: %w", err)
	}
	if err := os.WriteFile(filepath.Join(recoveryDir, "restore-inputs.json"), data, 0644); err != nil {
		return fmt.Errorf("write restore-inputs.json: %w", err)
	}

	scripts := []struct {
		name string
		fn   func(io.Writer, *RecoveryInputs)
	}{
		{"restore.sh", writeMainRestoreScript},
		{"phase1-restore-files.sh", writePhase1Script},
		{"phase2-bootstrap-minio.sh", writePhase2Script},
		{"phase3-restore-etcd.sh", writePhase3Script},
		{"phase4-restore-scylla.sh", writePhase4Script},
	}

	for _, s := range scripts {
		path := filepath.Join(recoveryDir, s.name)
		f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0755)
		if err != nil {
			slog.Warn("failed to create recovery script", "script", s.name, "error", err)
			continue
		}
		s.fn(f, inputs)
		f.Close()
	}

	if f, err := os.Create(filepath.Join(recoveryDir, "README.md")); err == nil {
		writeRecoveryREADME(f, inputs)
		f.Close()
	}

	slog.Info("recovery capsule generated", "backup_id", backupID, "dir", recoveryDir)
	return nil
}

func (srv *server) buildRecoveryInputs(
	backupID string,
	results []*backup_managerpb.BackupProviderResult,
) *RecoveryInputs {
	ri := &RecoveryInputs{
		BackupID:  backupID,
		CreatedAt: time.Now().UTC(),
		Domain:    srv.Domain,
		Minio: &MinioRestoreInputs{
			Endpoint:    srv.MinioEndpoint,
			DataPath:    srv.RcloneSource,
			AgentConfig: "/var/lib/globular/scylla-manager-agent/scylla-manager-agent.yaml",
		},
	}
	if ri.Minio.DataPath == "" {
		ri.Minio.DataPath = "/var/lib/globular/minio/data"
	}

	for _, r := range results {
		if r.State != backup_managerpb.BackupJobState_BACKUP_JOB_SUCCEEDED {
			continue
		}
		switch r.Type {
		case backup_managerpb.BackupProviderType_BACKUP_PROVIDER_ETCD:
			snapshotFile := r.RestoreInputs["snapshot_path"]
			if snapshotFile == "" {
				snapshotFile = "payload/etcd/etcd-snapshot.db"
			}
			ri.Etcd = &EtcdRestoreInputs{
				SnapshotFile: snapshotFile,
				DataDir:      r.RestoreInputs["data_dir"],
				Endpoints:    r.Outputs["endpoints"],
			}

		case backup_managerpb.BackupProviderType_BACKUP_PROVIDER_RESTIC:
			ri.Restic = &ResticRestoreInputs{
				Repo:            srv.ResticRepo,
				SnapshotID:      r.Outputs["snapshot_id"],
				Paths:           srv.ResticPaths,
				PasswordFileHint: "/var/lib/globular/backups/restic.password",
			}

		case backup_managerpb.BackupProviderType_BACKUP_PROVIDER_SCYLLA:
			ri.Scylla = &ScyllaRestoreInputs{
				Cluster:     r.RestoreInputs["cluster"],
				SnapshotTag: r.RestoreInputs["snapshot_tag"],
				Locations:   r.RestoreInputs["locations"],
				TaskID:      r.RestoreInputs["task_id"],
				APIURL:      r.Outputs["api_url"],
			}
		}
	}
	return ri
}

// ---------------------------------------------------------------------------
// Script generators
// ---------------------------------------------------------------------------

func writeMainRestoreScript(w io.Writer, ri *RecoveryInputs) {
	lines := []string{
		"#!/bin/bash",
		"# ============================================================",
		"# Globular Full-Cluster Recovery Orchestrator",
		"# Backup : " + ri.BackupID,
		"# Domain : " + ri.Domain,
		"# Created: " + ri.CreatedAt.Format(time.RFC3339),
		"# ============================================================",
		"#",
		"# USAGE",
		"#   sudo ./restore.sh [--phase 1|2|3|4] [--dry-run]",
		"#              [--restic-password <pw>]",
		"#",
		"# Without --phase all phases run in order:",
		"#   Phase 1  Restore /var/lib/globular from restic (includes MinIO data)",
		"#   Phase 2  Bootstrap MinIO (restart service with restored data)",
		"#   Phase 3  Restore etcd cluster state from snapshot",
		"#   Phase 4  Restore ScyllaDB from snapshot in MinIO (via sctool)",
		"#",
		"# Recovery chain (MinIO unavailable scenario):",
		"#   restic restores /var/lib/globular → MinIO data back on disk",
		"#   → MinIO starts → scylla backup accessible → sctool restore → etcd restore",
		"# ============================================================",
		"",
		"set -euo pipefail",
		"",
		`SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"`,
		`ARTIFACT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"`,
		"DRY_RUN=false",
		`PHASE_FILTER=""`,
		"",
		`while [[ $# -gt 0 ]]; do`,
		`  case "$1" in`,
		`    --dry-run)           DRY_RUN=true ;;`,
		`    --phase)             PHASE_FILTER="$2"; shift ;;`,
		`    --restic-password)   export RESTIC_PASSWORD="$2"; shift ;;`,
		`    *) echo "Unknown option: $1" >&2; exit 1 ;;`,
		`  esac`,
		`  shift`,
		`done`,
		"",
		`log() { echo "[$(date +%H:%M:%S)] $*"; }`,
		`die() { echo "ERROR: $*" >&2; exit 1; }`,
		"",
		`[[ $EUID -eq 0 ]] || die "Run as root (or with sudo)"`,
		"",
		"# Resolve restic password",
		`if [[ -z "${RESTIC_PASSWORD:-}" ]]; then`,
		`  PWFILE="` + ri.resticPasswordFile() + `"`,
		`  if [[ -f "$PWFILE" ]]; then`,
		`    export RESTIC_PASSWORD="$(cat "$PWFILE")"`,
		`  else`,
		`    read -r -s -p "Restic repository password: " RESTIC_PASSWORD; echo`,
		`    export RESTIC_PASSWORD`,
		`  fi`,
		`fi`,
		"",
		`run_phase() {`,
		`  local n="$1" desc="$2" script="$3"`,
		`  [[ -n "$PHASE_FILTER" && "$PHASE_FILTER" != "$n" ]] && return 0`,
		`  log "=== Phase $n: $desc ==="`,
		`  if $DRY_RUN; then`,
		`    log "[dry-run] would execute: $SCRIPT_DIR/$script"`,
		`  else`,
		`    bash "$SCRIPT_DIR/$script"`,
		`  fi`,
		`  log "Phase $n complete."`,
		`}`,
		"",
		`run_phase 1 "Restore files from restic (includes MinIO object data)" "phase1-restore-files.sh"`,
		`run_phase 2 "Bootstrap MinIO service"                                "phase2-bootstrap-minio.sh"`,
		`run_phase 3 "Restore etcd cluster state"                             "phase3-restore-etcd.sh"`,
		`run_phase 4 "Restore ScyllaDB"                                       "phase4-restore-scylla.sh"`,
		"",
		`log ""`,
		`log "Recovery complete."`,
		`log "Start the cluster controller to bring services online:"`,
		`log "  sudo systemctl start globular-cluster-controller.service"`,
	}
	fmt.Fprintln(w, strings.Join(lines, "\n"))
}

func writePhase1Script(w io.Writer, ri *RecoveryInputs) {
	resticRepo := ""
	snapshotID := "latest"
	resticPaths := "/var/lib/globular"
	if ri.Restic != nil {
		resticRepo = ri.Restic.Repo
		if ri.Restic.SnapshotID != "" {
			snapshotID = ri.Restic.SnapshotID
		}
		if ri.Restic.Paths != "" {
			resticPaths = ri.Restic.Paths
		}
	}

	lines := []string{
		"#!/bin/bash",
		"# Phase 1 — Restore files from restic",
		"# Restores " + resticPaths + " including MinIO object data.",
		"# After this phase MinIO data is back on disk but the service is not yet started.",
		"",
		"set -euo pipefail",
		`log() { echo "[phase1] $*"; }`,
		"",
		"RESTIC_REPO=" + shellQuote(resticRepo),
		"SNAPSHOT_ID=" + shellQuote(snapshotID),
		"",
		`[[ -n "${RESTIC_PASSWORD:-}" ]] || { echo "RESTIC_PASSWORD not set" >&2; exit 1; }`,
		"",
		"# Stop services that write to the paths we are about to restore.",
		"# Ignore errors if services are already stopped.",
		`for svc in globular-cluster-controller globular-node-agent globular-minio; do`,
		`  systemctl stop "$svc.service" 2>/dev/null || true`,
		`done`,
		"",
		`log "Verifying restic repository at $RESTIC_REPO ..."`,
		`restic -r "$RESTIC_REPO" snapshots "$SNAPSHOT_ID" --no-lock 2>&1 | tail -3`,
		"",
		`log "Restoring snapshot $SNAPSHOT_ID to / ..."`,
		`restic -r "$RESTIC_REPO" restore "$SNAPSHOT_ID" --target / \`,
	}

	// Build --include flags for each path
	for _, p := range strings.Split(resticPaths, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			lines = append(lines, `  --include `+shellQuote(p)+` \`)
		}
	}
	lines = append(lines,
		`  --overwrite always`,
		"",
		`log "File restore complete."`,
		`log "MinIO data is at: `+ri.minioDataPath()+`"`,
	)
	fmt.Fprintln(w, strings.Join(lines, "\n"))
}

func writePhase2Script(w io.Writer, ri *RecoveryInputs) {
	dataPath := ri.minioDataPath()
	endpoint := ""
	if ri.Minio != nil {
		endpoint = ri.Minio.Endpoint
	}

	lines := []string{
		"#!/bin/bash",
		"# Phase 2 — Bootstrap MinIO",
		"# MinIO object data has been restored by Phase 1.",
		"# This phase (re)starts MinIO and waits for it to be ready.",
		"# Once MinIO is running, the ScyllaDB backup is accessible in the globular-backups bucket.",
		"",
		"set -euo pipefail",
		`log() { echo "[phase2] $*"; }`,
		"",
		`log "Checking MinIO data directory ..."`,
		`[[ -d "` + dataPath + `" ]] || { echo "MinIO data dir not found: ` + dataPath + `" >&2; exit 1; }`,
		`ls "` + dataPath + `" | head -5`,
		"",
		`log "Starting MinIO ..."`,
		`systemctl start globular-minio.service`,
		"",
		`log "Waiting for MinIO to become ready (endpoint: ` + endpoint + `) ..."`,
		`TIMEOUT=60`,
		`START=$(date +%s)`,
		`while true; do`,
		`  if curl -sk "https://` + endpoint + `/minio/health/live" -o /dev/null -w "%{http_code}" 2>/dev/null | grep -q "^200$"; then`,
		`    break`,
		`  fi`,
		`  ELAPSED=$(( $(date +%s) - START ))`,
		`  [[ $ELAPSED -lt $TIMEOUT ]] || { echo "MinIO did not become ready within ${TIMEOUT}s" >&2; exit 1; }`,
		`  sleep 3`,
		`done`,
		"",
		`log "MinIO is ready."`,
		`log "Verify the globular-backups bucket is accessible before running Phase 4."`,
	}
	fmt.Fprintln(w, strings.Join(lines, "\n"))
}

func writePhase3Script(w io.Writer, ri *RecoveryInputs) {
	snapshotFile := "payload/etcd/etcd-snapshot.db"
	dataDir := "/var/lib/globular/etcd"
	if ri.Etcd != nil {
		if ri.Etcd.SnapshotFile != "" {
			snapshotFile = ri.Etcd.SnapshotFile
		}
		if ri.Etcd.DataDir != "" {
			dataDir = ri.Etcd.DataDir
		}
	}

	lines := []string{
		"#!/bin/bash",
		"# Phase 3 — Restore etcd",
		"# Restores the etcd cluster state from the embedded snapshot.",
		"# This recreates all Globular desired-state, service configs, RBAC rules, etc.",
		"",
		"set -euo pipefail",
		`log() { echo "[phase3] $*"; }`,
		`SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"`,
		`ARTIFACT_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"`,
		"",
		"SNAPSHOT=$ARTIFACT_DIR/" + snapshotFile,
		"DATA_DIR=" + shellQuote(dataDir),
		"",
		`[[ -f "$SNAPSHOT" ]] || { echo "etcd snapshot not found: $SNAPSHOT" >&2; exit 1; }`,
		"",
		`log "Verifying snapshot integrity ..."`,
		`etcdutl snapshot status "$SNAPSHOT" --write-out=table`,
		"",
		`log "Stopping etcd ..."`,
		`systemctl stop globular-etcd.service 2>/dev/null || true`,
		"",
		`log "Moving old data directory out of the way ..."`,
		`BACKUP_DATADIR="${DATA_DIR}.pre-restore.$(date +%Y%m%d%H%M%S)"`,
		`[[ -d "$DATA_DIR" ]] && mv "$DATA_DIR" "$BACKUP_DATADIR"`,
		"",
		`log "Restoring snapshot to $DATA_DIR ..."`,
		`etcdutl snapshot restore "$SNAPSHOT" --data-dir "$DATA_DIR"`,
		"",
		`log "Setting ownership ..."`,
		`chown -R globular:globular "$DATA_DIR" 2>/dev/null || chown -R etcd:etcd "$DATA_DIR" 2>/dev/null || true`,
		"",
		`log "Starting etcd ..."`,
		`systemctl start globular-etcd.service`,
		"",
		`log "Waiting for etcd to be ready ..."`,
		`TIMEOUT=30`,
		`START=$(date +%s)`,
		`while ! etcdctl endpoint health 2>/dev/null; do`,
		`  ELAPSED=$(( $(date +%s) - START ))`,
		`  [[ $ELAPSED -lt $TIMEOUT ]] || { echo "etcd did not become ready" >&2; exit 1; }`,
		`  sleep 2`,
		`done`,
		"",
		`log "etcd restored successfully."`,
	}
	fmt.Fprintln(w, strings.Join(lines, "\n"))
}

func writePhase4Script(w io.Writer, ri *RecoveryInputs) {
	if ri.Scylla == nil {
		lines := []string{
			"#!/bin/bash",
			"# Phase 4 — Restore ScyllaDB",
			"# No ScyllaDB backup was captured in this backup — skipping.",
			`echo "No ScyllaDB restore inputs found. Nothing to do."`,
		}
		fmt.Fprintln(w, strings.Join(lines, "\n"))
		return
	}

	sc := ri.Scylla

	// Build sctool API args
	apiArgs := ""
	if sc.APIURL != "" {
		apiArgs += " --api-url " + shellQuote(normalizeScyllaAPIURL(sc.APIURL))
	}

	lines := []string{
		"#!/bin/bash",
		"# Phase 4 — Restore ScyllaDB",
		"# Uses sctool to restore ScyllaDB from the backup in MinIO.",
		"# MinIO MUST be running before this phase (Phase 2).",
		"# ScyllaDB nodes MUST be running and registered in scylla-manager.",
		"",
		"set -euo pipefail",
		`log() { echo "[phase4] $*"; }`,
		"",
		"CLUSTER=" + shellQuote(sc.Cluster),
		"SNAPSHOT_TAG=" + shellQuote(sc.SnapshotTag),
		"LOCATIONS=" + shellQuote(sc.Locations),
		"",
		`log "Starting ScyllaDB nodes ..."`,
		`systemctl start globular-scylla.service 2>/dev/null || true`,
		"",
		`log "Waiting for ScyllaDB nodes to be healthy ..."`,
		`sleep 10`,
		`nodetool status 2>/dev/null || true`,
		"",
		`log "Restoring cluster $CLUSTER from snapshot $SNAPSHOT_TAG ..."`,
		`log "Locations: $LOCATIONS"`,
		"",
		"sctool restore" + apiArgs + " \\",
		"  --cluster $CLUSTER \\",
		"  --location $LOCATIONS \\",
		"  --snapshot-tag $SNAPSHOT_TAG \\",
		"  --restore-tables",
		"",
		`log "ScyllaDB restore initiated. Monitor progress with:"`,
		`log "  sctool task list --cluster $CLUSTER` + apiArgs + `"`,
	}
	fmt.Fprintln(w, strings.Join(lines, "\n"))
}

func writeRecoveryREADME(w io.Writer, ri *RecoveryInputs) {
	var sb strings.Builder

	sb.WriteString("# Globular Recovery Capsule\n\n")
	sb.WriteString("**Backup ID**: `" + ri.BackupID + "`  \n")
	sb.WriteString("**Created**: " + ri.CreatedAt.Format(time.RFC3339) + "  \n")
	sb.WriteString("**Domain**: " + ri.Domain + "  \n\n")

	sb.WriteString("## What is this?\n\n")
	sb.WriteString("This directory contains everything needed to recover a Globular cluster\n")
	sb.WriteString("from scratch — including the case where MinIO (object storage) is unavailable.\n\n")

	sb.WriteString("## Contents\n\n")
	sb.WriteString("| File | Purpose |\n")
	sb.WriteString("|------|---------|\n")
	sb.WriteString("| `restore-inputs.json` | Machine-readable restore parameters |\n")
	sb.WriteString("| `restore.sh` | Full-cluster restore orchestrator |\n")
	sb.WriteString("| `phase1-restore-files.sh` | Restore `/var/lib/globular` via restic (includes MinIO data) |\n")
	sb.WriteString("| `phase2-bootstrap-minio.sh` | (Re)start MinIO after files are restored |\n")
	sb.WriteString("| `phase3-restore-etcd.sh` | Restore etcd state from embedded snapshot |\n")
	sb.WriteString("| `phase4-restore-scylla.sh` | Restore ScyllaDB from MinIO via sctool |\n")
	sb.WriteString("| `../payload/etcd/etcd-snapshot.db` | etcd snapshot (referenced by phase3) |\n\n")

	sb.WriteString("## Recovery scenarios\n\n")

	sb.WriteString("### 1. Full disaster — cluster gone, only this capsule survives\n\n")
	sb.WriteString("Requirements: `restic` binary + restic repository at `" + ri.resticRepo() + "`\n\n")
	sb.WriteString("```bash\n")
	sb.WriteString("# 1. Install Globular binaries on fresh hardware\n")
	sb.WriteString("# 2. Copy this capsule (including ../payload/) to the new node\n")
	sb.WriteString("# 3. Copy restic repo to " + ri.resticRepo() + "\n")
	sb.WriteString("sudo RESTIC_PASSWORD=<password> ./restore.sh\n")
	sb.WriteString("sudo systemctl start globular-cluster-controller.service\n")
	sb.WriteString("```\n\n")

	sb.WriteString("### 2. MinIO gone — restic repo survives on local disk\n\n")
	sb.WriteString("The restic backup covers `" + ri.resticPaths() + "`, which includes\n")
	sb.WriteString("MinIO object data at `" + ri.minioDataPath() + "`.\n")
	sb.WriteString("Restoring restic brings MinIO back, making the ScyllaDB backup accessible.\n\n")
	sb.WriteString("```bash\n")
	sb.WriteString("sudo RESTIC_PASSWORD=<password> ./restore.sh\n")
	sb.WriteString("```\n\n")

	sb.WriteString("### 3. etcd only — cluster misconfigured / etcd data lost\n\n")
	sb.WriteString("```bash\n")
	sb.WriteString("sudo ./restore.sh --phase 3\n")
	sb.WriteString("```\n\n")

	sb.WriteString("### 4. ScyllaDB data loss — MinIO and cluster are healthy\n\n")
	sb.WriteString("```bash\n")
	sb.WriteString("sudo ./restore.sh --phase 4\n")
	sb.WriteString("```\n\n")

	sb.WriteString("## Backup data locations\n\n")

	if ri.Etcd != nil {
		sb.WriteString("### etcd\n\n")
		sb.WriteString("- Snapshot: `" + ri.Etcd.SnapshotFile + "` (relative to artifact root)\n")
		sb.WriteString("- Restore target: `" + ri.Etcd.DataDir + "`\n\n")
	}

	if ri.Restic != nil {
		sb.WriteString("### Files (restic)\n\n")
		sb.WriteString("- Repository: `" + ri.Restic.Repo + "`\n")
		sb.WriteString("- Snapshot ID: `" + ri.Restic.SnapshotID + "`\n")
		sb.WriteString("- Paths covered: `" + ri.Restic.Paths + "`\n")
		sb.WriteString("- **Includes MinIO data** at `" + ri.minioDataPath() + "`\n\n")
	}

	if ri.Scylla != nil {
		sb.WriteString("### ScyllaDB\n\n")
		sb.WriteString("- Cluster: `" + ri.Scylla.Cluster + "`\n")
		sb.WriteString("- Snapshot tag: `" + ri.Scylla.SnapshotTag + "`\n")
		sb.WriteString("- Location: `" + ri.Scylla.Locations + "`\n")
		if ri.Scylla.APIURL != "" {
			sb.WriteString("- sctool API: `" + ri.Scylla.APIURL + "`\n")
		}
		sb.WriteString("\n")
	}

	if ri.Minio != nil {
		sb.WriteString("### MinIO\n\n")
		sb.WriteString("- Endpoint: `" + ri.Minio.Endpoint + "`\n")
		sb.WriteString("- Data directory: `" + ri.Minio.DataPath + "`\n")
		sb.WriteString("- Agent config: `" + ri.Minio.AgentConfig + "`\n\n")
	}

	sb.WriteString("## Recovery chain (MinIO unavailable)\n\n")
	sb.WriteString("```\n")
	sb.WriteString("restic repo → restore /var/lib/globular → MinIO data on disk\n")
	sb.WriteString("  → start MinIO → scylla backup accessible in globular-backups\n")
	sb.WriteString("  → restore etcd  → cluster desired-state restored\n")
	sb.WriteString("  → sctool restore → ScyllaDB data restored\n")
	sb.WriteString("  → start globular-cluster-controller → services converge\n")
	sb.WriteString("```\n")

	fmt.Fprint(w, sb.String())
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func shellQuote(s string) string {
	if s == "" {
		return `""`
	}
	return `"` + strings.ReplaceAll(s, `"`, `\"`) + `"`
}

func (ri *RecoveryInputs) resticPasswordFile() string {
	if ri.Restic != nil && ri.Restic.PasswordFileHint != "" {
		return ri.Restic.PasswordFileHint
	}
	return "/var/lib/globular/backups/restic.password"
}

func (ri *RecoveryInputs) resticRepo() string {
	if ri.Restic != nil {
		return ri.Restic.Repo
	}
	return "/var/backups/globular/restic"
}

func (ri *RecoveryInputs) resticPaths() string {
	if ri.Restic != nil && ri.Restic.Paths != "" {
		return ri.Restic.Paths
	}
	return "/var/lib/globular"
}

func (ri *RecoveryInputs) minioDataPath() string {
	if ri.Minio != nil && ri.Minio.DataPath != "" {
		return ri.Minio.DataPath
	}
	return "/var/lib/globular/minio/data"
}
