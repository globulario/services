package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/globulario/services/golang/config"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// restoreTask tracks an in-progress or completed restore provider execution.
type restoreTask struct {
	mu     sync.Mutex
	result *node_agentpb.BackupProviderResult
}

var (
	restoreTasksMu sync.Mutex
	restoreTasks   = make(map[string]*restoreTask)
)

// RunRestoreProvider starts a restore provider execution asynchronously.
func (s *NodeAgentServer) RunRestoreProvider(ctx context.Context, req *node_agentpb.RunRestoreProviderRequest) (*node_agentpb.RunRestoreProviderResponse, error) {
	if req.Spec == nil || req.Spec.Provider == "" {
		return nil, status.Error(codes.InvalidArgument, "spec.provider is required")
	}
	if req.BackupId == "" {
		return nil, status.Error(codes.InvalidArgument, "backup_id is required")
	}

	taskID := uuid.New().String()

	task := &restoreTask{
		result: &node_agentpb.BackupProviderResult{
			Provider: req.Spec.Provider,
			Done:     false,
		},
	}

	restoreTasksMu.Lock()
	restoreTasks[taskID] = task
	restoreTasksMu.Unlock()

	log.Printf("restore task %s started: provider=%s backup_id=%s node_id=%s",
		taskID, req.Spec.Provider, req.BackupId, req.NodeId)

	go s.executeRestoreProvider(taskID, task, req)

	return &node_agentpb.RunRestoreProviderResponse{TaskId: taskID}, nil
}

// GetRestoreTaskResult returns the current state of a restore task.
func (s *NodeAgentServer) GetRestoreTaskResult(ctx context.Context, req *node_agentpb.GetRestoreTaskResultRequest) (*node_agentpb.GetRestoreTaskResultResponse, error) {
	if req.TaskId == "" {
		return nil, status.Error(codes.InvalidArgument, "task_id is required")
	}

	restoreTasksMu.Lock()
	task, ok := restoreTasks[req.TaskId]
	restoreTasksMu.Unlock()

	if !ok {
		return nil, status.Errorf(codes.NotFound, "restore task %s not found", req.TaskId)
	}

	task.mu.Lock()
	result := task.result
	task.mu.Unlock()

	return &node_agentpb.GetRestoreTaskResultResponse{Result: result}, nil
}

// executeRestoreProvider runs the actual restore work in the background.
func (s *NodeAgentServer) executeRestoreProvider(taskID string, task *restoreTask, req *node_agentpb.RunRestoreProviderRequest) {
	start := time.Now().UnixMilli()

	timeout := time.Duration(req.Spec.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var result *node_agentpb.BackupProviderResult

	switch req.Spec.Provider {
	case "etcd":
		result = s.restoreEtcdProvider(ctx, req)
	case "restic":
		result = s.restoreResticProvider(ctx, req)
	case "scylla":
		result = s.restoreScyllaProvider(ctx, req)
	case "minio":
		result = s.restoreMinioProvider(ctx, req)
	default:
		result = &node_agentpb.BackupProviderResult{
			Provider:     req.Spec.Provider,
			Ok:           false,
			Summary:      fmt.Sprintf("unsupported restore provider: %s", req.Spec.Provider),
			ErrorMessage: "only etcd, restic, scylla are supported",
			Done:         true,
		}
	}

	result.StartedUnixMs = start
	result.FinishedUnixMs = time.Now().UnixMilli()
	result.Done = true

	task.mu.Lock()
	task.result = result
	task.mu.Unlock()

	if result.Ok {
		log.Printf("restore task %s completed: provider=%s", taskID, req.Spec.Provider)
	} else {
		log.Printf("restore task %s failed: provider=%s error=%s", taskID, req.Spec.Provider, result.ErrorMessage)
	}

	// Clean up task after 1 hour
	go func() {
		time.Sleep(1 * time.Hour)
		restoreTasksMu.Lock()
		delete(restoreTasks, taskID)
		restoreTasksMu.Unlock()
	}()
}

// --- etcd restore ---

func (s *NodeAgentServer) restoreEtcdProvider(ctx context.Context, req *node_agentpb.RunRestoreProviderRequest) *node_agentpb.BackupProviderResult {
	outputs := make(map[string]string)
	opts := req.Spec.Options
	if opts == nil {
		opts = make(map[string]string)
	}

	snapshotPath := opts["snapshot_path"]
	if snapshotPath == "" {
		return restoreFail("etcd", "snapshot_path option is required", outputs)
	}
	if _, err := os.Stat(snapshotPath); err != nil {
		return restoreFail("etcd", fmt.Sprintf("snapshot file not found: %s", snapshotPath), outputs)
	}

	// Globular manages etcd: data-dir is owned by globular:globular,
	// so the node-agent (running as globular) can do all file operations
	// directly. Only systemctl stop/start requires sudo.
	dataDir := opts["data_dir"]
	if dataDir == "" {
		dataDir = "/var/lib/globular/etcd"
	}
	serviceName := opts["service_name"]
	if serviceName == "" {
		serviceName = "globular-etcd.service"
	}

	// Stage 1: restore snapshot into a staging directory next to data-dir
	stagingDir := dataDir + ".restore"
	_ = os.RemoveAll(stagingDir)

	log.Printf("etcd restore: restoring snapshot to staging dir %s", stagingDir)
	stdout, stderr, err := runRestore(ctx, "etcdctl", "snapshot", "restore", snapshotPath, "--data-dir", stagingDir)
	outputs["restore_stdout"] = stdout
	outputs["restore_stderr"] = stderr
	if err != nil {
		detail := stderr
		if detail == "" {
			detail = stdout
		}
		return restoreFail("etcd", fmt.Sprintf("etcdctl snapshot restore failed: %s", detail), outputs)
	}

	// Stage 2: stop etcd service (requires sudo)
	log.Printf("etcd restore: stopping %s", serviceName)
	_, stopErr, err := runRestore(ctx, "sudo", "systemctl", "stop", serviceName)
	if err != nil {
		log.Printf("etcd restore: stop warning (may not be running): %s", stopErr)
		outputs["stop_warning"] = stopErr
	}

	// Stage 3: back up current data-dir and swap (no sudo — globular owns it)
	backupDir := dataDir + ".bak"
	if fileExistsNA(dataDir) {
		_ = os.RemoveAll(backupDir)
		log.Printf("etcd restore: backing up %s -> %s", dataDir, backupDir)
		if err := os.Rename(dataDir, backupDir); err != nil {
			outputs["backup_error"] = err.Error()
			_, _, _ = runRestore(ctx, "sudo", "systemctl", "start", serviceName)
			return restoreFail("etcd", fmt.Sprintf("failed to back up data-dir: %s", err), outputs)
		}
		outputs["backed_up_to"] = backupDir
	}

	log.Printf("etcd restore: moving staging %s -> %s", stagingDir, dataDir)
	if err := os.Rename(stagingDir, dataDir); err != nil {
		outputs["move_error"] = err.Error()
		// Rollback: move backup back
		_ = os.Rename(backupDir, dataDir)
		_, _, _ = runRestore(ctx, "sudo", "systemctl", "start", serviceName)
		return restoreFail("etcd", fmt.Sprintf("failed to move restored data: %s", err), outputs)
	}

	// Stage 3b: migrate config file if it was inside the old data-dir.
	// Config now lives at /var/lib/globular/config/etcd.yaml (outside data-dir)
	// but old installs may still have it inside the data-dir.
	configDir := filepath.Dir(dataDir) + "/config"
	configDst := filepath.Join(configDir, "etcd.yaml")
	if !fileExistsNA(configDst) {
		// Try to recover from old backup location
		for _, name := range []string{"etcd.yaml", "etcd.conf.yml"} {
			src := filepath.Join(backupDir, name)
			if fileExistsNA(src) {
				_ = os.MkdirAll(configDir, 0750)
				if data, err := os.ReadFile(src); err == nil {
					if err := os.WriteFile(configDst, data, 0644); err == nil {
						log.Printf("etcd restore: migrated %s from old data-dir to %s", name, configDst)
						outputs["migrated_config"] = configDst
					}
				}
				break
			}
		}
	}

	// Stage 4: start etcd (requires sudo)
	log.Printf("etcd restore: starting %s", serviceName)
	_, startErr, err := runRestore(ctx, "sudo", "systemctl", "start", serviceName)
	if err != nil {
		outputs["start_error"] = startErr
		return restoreFail("etcd", fmt.Sprintf("etcd restored but service failed to start: %s", startErr), outputs)
	}

	// Stage 5: re-populate disk mirror of service configs from restored etcd.
	// This ensures /var/lib/globular/services/*.json reflects the backup state
	// before services restart (e.g., file service public dirs, custom ports).
	time.Sleep(1 * time.Second) // give etcd a moment to accept connections
	if n, errs := config.DumpServiceConfigsToDisk(); n > 0 || len(errs) > 0 {
		log.Printf("etcd restore: dumped %d service configs to disk (errors: %v)", n, errs)
		outputs["service_configs_dumped"] = fmt.Sprintf("%d", n)
		if len(errs) > 0 {
			outputs["service_configs_errors"] = strings.Join(errs, "; ")
		}
	}

	outputs["data_dir"] = dataDir
	var snapshotBytes uint64
	if info, err := os.Stat(snapshotPath); err == nil {
		snapshotBytes = uint64(info.Size())
		outputs["snapshot_bytes"] = fmt.Sprintf("%d", snapshotBytes)
	}
	return &node_agentpb.BackupProviderResult{
		Provider:     "etcd",
		Ok:           true,
		Summary:      "etcd snapshot restored and service restarted",
		BytesWritten: snapshotBytes,
		Outputs:      outputs,
	}
}

// --- restic restore ---

func (s *NodeAgentServer) restoreResticProvider(ctx context.Context, req *node_agentpb.RunRestoreProviderRequest) *node_agentpb.BackupProviderResult {
	outputs := make(map[string]string)
	opts := req.Spec.Options
	if opts == nil {
		opts = make(map[string]string)
	}

	snapshotID := opts["snapshot_id"]
	if snapshotID == "" {
		return restoreFail("restic", "snapshot_id option is required", outputs)
	}

	repo := opts["repo"]
	if repo == "" {
		repo = "/var/backups/globular/restic"
	}
	password := opts["password"]
	if password == "" {
		password = "globular-backup"
	}
	target := opts["target"]
	if target == "" {
		target = "/"
	}

	env := append(os.Environ(),
		"RESTIC_REPOSITORY="+repo,
		"RESTIC_PASSWORD="+password,
	)

	outputs["repo"] = repo
	outputs["snapshot_id"] = snapshotID
	outputs["target"] = target

	// Exclude transient/security state from restores:
	// - backups: backup metadata is managed separately, restoring it causes
	//   zombie jobs and circular backup-of-backups issues
	// - keys/tokens: restoring old crypto keys invalidates all active sessions
	//   and causes "ed25519: verification error" on every authenticated request
	// - pki/tls: restoring old CA/certs breaks every TLS connection because
	//   running services (MinIO, gRPC, Envoy) use certs from the current CA
	// - scylla-manager-agent: auth token and config are regenerated by Day 0;
	//   restoring old tokens breaks sctool authentication, and these files are
	//   owned by scylla (not globular), so restic can't chmod them
	// - scylla-manager: internal SQLite DB with cluster registrations and auth
	//   tokens that must match the current agent; restoring stale tokens from
	//   backup causes HTTP 401 on every sctool command
	// - etcd: restored by its own provider (snapshot); restic overwriting the
	//   freshly-restored member/ directory causes "walpb: crc mismatch" fatal
	excludes := []string{
		"var/backups/globular",
		"var/lib/globular/keys",
		"var/lib/globular/tokens",
		"var/lib/globular/pki",
		"var/lib/globular/config/tls",
		"var/lib/globular/scylla-manager-agent",
		"var/lib/globular/scylla-manager",
		"var/lib/globular/etcd",
	}

	// Stop services whose data directories will be overwritten by restic.
	// Prometheus TSDB requires WAL segments to be sequential — restoring
	// old data on top of a running instance causes fatal corruption.
	stopBeforeRestore := []string{"globular-prometheus.service"}
	for _, svc := range stopBeforeRestore {
		log.Printf("restic restore: stopping %s before restore", svc)
		stopCmd := exec.CommandContext(ctx, "systemctl", "stop", svc)
		if out, err := stopCmd.CombinedOutput(); err != nil {
			log.Printf("restic restore: warning: failed to stop %s: %v (%s)", svc, err, strings.TrimSpace(string(out)))
		}
	}

	log.Printf("restic restore: snapshot=%s target=%s repo=%s", snapshotID, target, repo)
	args := []string{"restore", snapshotID, "--target", target}
	for _, ex := range excludes {
		args = append(args, "--exclude", ex)
	}
	cmd := exec.CommandContext(ctx, "restic", args...)
	cmd.Env = env
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	stdoutStr := strings.TrimSpace(outBuf.String())
	stderrStr := strings.TrimSpace(errBuf.String())
	outputs["stdout"] = stdoutStr
	if stderrStr != "" {
		outputs["stderr"] = stderrStr
	}

	// Restart services that were stopped before restore, regardless of outcome.
	// If a service fails to start (corrupted backup data), clear only the
	// volatile TSDB state (wal/ and chunks_head/) while preserving compacted
	// blocks that contain historical metrics, then retry.
	restartStopped := func() {
		for _, svc := range stopBeforeRestore {
			log.Printf("restic restore: starting %s after restore", svc)
			startCmd := exec.CommandContext(ctx, "systemctl", "start", svc)
			if out, startErr := startCmd.CombinedOutput(); startErr != nil {
				log.Printf("restic restore: %s failed to start: %v (%s)", svc, startErr, strings.TrimSpace(string(out)))

				if svc == "globular-prometheus.service" {
					dataDir := "/var/lib/globular/prometheus/data"
					// Only remove WAL and chunks_head — these are the volatile
					// parts that cause "segments are not sequential" errors.
					// Compacted block directories (01XXXX...) contain the actual
					// historical time-series data and are self-contained.
					for _, sub := range []string{"wal", "chunks_head"} {
						p := filepath.Join(dataDir, sub)
						if _, statErr := os.Stat(p); statErr == nil {
							log.Printf("restic restore: removing corrupted %s", p)
							_ = os.RemoveAll(p)
						}
					}
					// Also remove the lock file so Prometheus can re-acquire it.
					_ = os.Remove(filepath.Join(dataDir, "lock"))
					outputs[svc+"_wal_cleared"] = dataDir

					retryCmd := exec.CommandContext(ctx, "systemctl", "start", svc)
					if retryOut, retryErr := retryCmd.CombinedOutput(); retryErr != nil {
						log.Printf("restic restore: warning: %s still failed after WAL cleanup: %v (%s)", svc, retryErr, strings.TrimSpace(string(retryOut)))
					} else {
						log.Printf("restic restore: %s started with historical blocks preserved", svc)
					}
				}
			}
		}
	}

	if err != nil {
		restartStopped()
		detail := stderrStr
		if detail == "" {
			detail = stdoutStr
		}
		return restoreFail("restic", fmt.Sprintf("restic restore failed: %s", detail), outputs)
	}

	// Parse restored bytes from restic output (e.g. "restored 1234 files, 567.8 MiB")
	var restoredBytes uint64
	combined := stdoutStr + "\n" + stderrStr
	for _, line := range strings.Split(combined, "\n") {
		restoredBytes = parseResticRestoredBytes(line)
		if restoredBytes > 0 {
			outputs["bytes_restored"] = fmt.Sprintf("%d", restoredBytes)
			break
		}
	}

	// Post-restore: verify TLS symlinks in config/tls/ still exist.
	// Restic excludes config/tls but restoring config/ may recreate the
	// parent directory, leaving stale or missing symlinks. Re-create them
	// from the PKI issued certs if needed.
	if repaired := repairTLSSymlinks(); repaired != "" {
		outputs["tls_repair"] = repaired
	}

	restartStopped()

	return &node_agentpb.BackupProviderResult{
		Provider:     "restic",
		Ok:           true,
		Summary:      fmt.Sprintf("restic snapshot %s restored to %s", snapshotID, target),
		BytesWritten: restoredBytes,
		Outputs:      outputs,
	}
}

// repairTLSSymlinks ensures /var/lib/globular/config/tls/ has the expected
// symlinks (server.crt→fullchain.pem, server.key→privkey.pem, ca.crt→ca.pem).
// Returns a human-readable summary of what was repaired, or "" if nothing needed.
func repairTLSSymlinks() string {
	tlsDir := "/var/lib/globular/config/tls"
	pkiCA := "/var/lib/globular/pki/ca.pem"

	// Ensure tls dir exists.
	if err := os.MkdirAll(tlsDir, 0o755); err != nil {
		return fmt.Sprintf("mkdir %s failed: %v", tlsDir, err)
	}

	// Map of expected symlinks: name → possible targets (first existing wins).
	links := map[string][]string{
		"server.crt": {filepath.Join(tlsDir, "fullchain.pem")},
		"server.key": {filepath.Join(tlsDir, "privkey.pem")},
		"ca.crt":     {filepath.Join(tlsDir, "ca.pem"), pkiCA},
	}

	var repaired []string
	for name, targets := range links {
		linkPath := filepath.Join(tlsDir, name)
		// If symlink already exists and its target is valid, skip.
		if dest, err := os.Readlink(linkPath); err == nil {
			if _, err2 := os.Stat(dest); err2 == nil {
				continue // valid symlink
			}
			// Broken symlink — remove and recreate.
			os.Remove(linkPath)
		} else if _, err := os.Stat(linkPath); err == nil {
			continue // regular file exists, don't touch it
		}

		// Find first existing target.
		for _, tgt := range targets {
			if _, err := os.Stat(tgt); err == nil {
				if err := os.Symlink(tgt, linkPath); err == nil {
					repaired = append(repaired, fmt.Sprintf("%s→%s", name, tgt))
				}
				break
			}
		}
	}

	if len(repaired) == 0 {
		return ""
	}
	return fmt.Sprintf("repaired %d symlink(s): %s", len(repaired), strings.Join(repaired, ", "))
}

// --- scylla restore ---

func (s *NodeAgentServer) restoreScyllaProvider(ctx context.Context, req *node_agentpb.RunRestoreProviderRequest) *node_agentpb.BackupProviderResult {
	outputs := make(map[string]string)
	opts := req.Spec.Options
	if opts == nil {
		opts = make(map[string]string)
	}

	cluster := opts["cluster"]
	snapshotTag := opts["snapshot_tag"]
	locations := opts["locations"]
	apiURL := opts["api_url"]

	if cluster == "" || snapshotTag == "" || locations == "" {
		return restoreFail("scylla", "cluster, snapshot_tag, and locations options are required", outputs)
	}

	outputs["cluster"] = cluster
	outputs["snapshot_tag"] = snapshotTag
	outputs["locations"] = locations

	// Deduplicate scylla-manager clusters before restore.
	// After wipe+Day0 cycles, scylla-manager accumulates stale cluster
	// entries with the same name, causing "multiple clusters share the
	// same name" errors on every sctool command that uses --cluster <name>.
	if removed := deduplicateScyllaClusters(ctx, cluster, apiURL); len(removed) > 0 {
		log.Printf("scylla restore: removed %d stale cluster entries: %v", len(removed), removed)
		outputs["dedup_removed"] = strings.Join(removed, ", ")
	}

	// Re-sync scylla-manager's auth token with the running agent's token.
	// After a restore the restic provider excludes scylla-manager-agent config
	// (tokens are regenerated at Day-0), so the agent's current auth_token may
	// differ from what scylla-manager has stored.  Without this sync step,
	// sctool commands fail with "HTTP 401 unauthorized".
	if err := syncScyllaManagerAuthToken(ctx, cluster, apiURL, outputs); err != nil {
		log.Printf("scylla restore: auth token sync warning: %v", err)
	}

	baseArgs := []string{"restore", "--cluster", cluster, "--snapshot-tag", snapshotTag}
	if apiURL != "" && apiURL != "http://127.0.0.1:5080" {
		baseArgs = append(baseArgs, "--api-url", apiURL)
	}
	for _, loc := range strings.Split(locations, ",") {
		loc = strings.TrimSpace(loc)
		if loc != "" {
			baseArgs = append(baseArgs, "--location", loc)
		}
	}

	// Stop all Globular workload services that use ScyllaDB before restore.
	// If left running, they recreate empty keyspaces on startup, racing with
	// schema restore and causing "already exists" errors (keyspace exists but
	// tables are missing).
	stoppedUnits := stopScyllaWorkloadServices(ctx, outputs)
	defer startScyllaWorkloadServices(ctx, stoppedUnits, outputs)

	// Phase 1: restore schema directly via cqlsh.
	//
	// We bypass sctool --restore-schema because it always fails with
	// "Cannot add existing keyspace scylla_manager" — the backup includes
	// scylla_manager schema, but we can't drop it (sctool needs it).
	// sctool rolls back ALL schema changes on error, so user keyspaces
	// are never created.
	//
	// Instead, we download the schema JSON from the backup in MinIO,
	// filter out scylla_manager entries, and apply CQL directly via cqlsh.
	dropUserKeyspaces(ctx, cluster, apiURL, outputs)

	if err := restoreSchemaFromBackup(ctx, snapshotTag, locations, outputs); err != nil {
		return restoreFail("scylla", fmt.Sprintf("direct schema restore failed: %v", err), outputs)
	}

	if !verifyUserKeyspacesExist(ctx, outputs) {
		return restoreFail("scylla", "schema restore completed but no user keyspaces found", outputs)
	}
	log.Printf("scylla restore: schema restored via cqlsh, proceeding to table restore")

	// Start a background goroutine to continuously chown ScyllaDB upload dirs
	// to scylla:scylla while the restore runs. The scylla-manager-agent downloads
	// sstables to <data_dir>/<ks>/<table>/upload/, and ScyllaDB refuses to load
	// files not owned by its euid. This safety net ensures file ownership is
	// correct regardless of which user the agent runs as.
	chownCtx, chownCancel := context.WithCancel(ctx)
	chownDone := make(chan struct{})
	go func() {
		defer close(chownDone)
		watchScyllaUploadOwnership(chownCtx)
	}()

	// Phase 2: restore tables
	tablesArgs := append(append([]string{}, baseArgs...), "--restore-tables")
	log.Printf("scylla restore: restoring tables cluster=%s tag=%s", cluster, snapshotTag)
	tablesOut, tablesErr := exec.CommandContext(ctx, "sctool", tablesArgs...).CombinedOutput()
	tablesOutStr := strings.TrimSpace(string(tablesOut))
	outputs["tables_output"] = tablesOutStr

	if tablesErr != nil {
		detail := tablesOutStr
		if detail == "" {
			detail = tablesErr.Error()
		}
		return restoreFail("scylla", fmt.Sprintf("sctool restore --restore-tables failed: %s", detail), outputs)
	}

	// Poll sctool task progress to get transferred bytes (same as backup side).
	// sctool restore returns a task ID like "restore/xxxxxxxx-..."
	var restoredBytes uint64
	var finalStatus string
	taskID := extractScyllaTaskID(tablesOutStr)
	if taskID != "" {
		outputs["task_id"] = taskID
		progressArgs := []string{"task", "progress", taskID, "--cluster", cluster}
		if apiURL != "" && apiURL != "http://127.0.0.1:5080" {
			progressArgs = append(progressArgs, "--api-url", apiURL)
		}
		for pollCount := 0; pollCount < 180; pollCount++ { // up to 30 min
			select {
			case <-ctx.Done():
				goto pollDone
			case <-time.After(10 * time.Second):
			}
			pOut, pErr := exec.CommandContext(ctx, "sctool", progressArgs...).CombinedOutput()
			if pErr != nil {
				continue
			}
			pStr := string(pOut)
			outputs["progress"] = strings.TrimSpace(pStr)
			statusLine := extractScyllaStatusLine(pStr)
			if strings.Contains(statusLine, "DONE") {
				finalStatus = "DONE"
				restoredBytes = parseScyllaRestoreBytes(pStr)
				break
			}
			if strings.Contains(statusLine, "ERROR") || strings.Contains(statusLine, "ABORTED") {
				finalStatus = "ERROR"
				// Extract the Cause line for a clear error message.
				if cause := extractScyllaCauseLine(pStr); cause != "" {
					outputs["error_cause"] = cause
				}
				break
			}
		}
	}
pollDone:
	chownCancel()
	<-chownDone

	// If sctool task ended in ERROR/ABORTED, report failure.
	if finalStatus == "ERROR" {
		cause := outputs["error_cause"]
		if cause == "" {
			cause = "sctool restore task failed (check sctool task progress)"
		}
		return restoreFail("scylla", fmt.Sprintf("table restore failed: %s", cause), outputs)
	}

	summary := "scylladb tables restored via sctool"
	if outputs["schema_restored"] == "true" {
		summary = "scylladb schema and tables restored via sctool"
	}

	return &node_agentpb.BackupProviderResult{
		Provider:     "scylla",
		Ok:           true,
		Summary:      summary,
		BytesWritten: restoredBytes,
		Outputs:      outputs,
	}
}

// watchScyllaUploadOwnership continuously chowns ScyllaDB's per-table upload
// directories to scylla:scylla while a restore is in progress. ScyllaDB refuses
// to load sstables not owned by its euid (the "scylla" system user). The agent
// downloads sstables to <data_dir>/<ks>/<table>/upload/, and this watcher ensures
// ownership is correct before ScyllaDB's load-and-stream picks them up.
func watchScyllaUploadOwnership(ctx context.Context) {
	dataDir := "/var/lib/scylla/data"
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// Find all "upload" directories and chown any files not owned by scylla.
			out, err := exec.CommandContext(ctx, "sudo", "find", dataDir,
				"-path", "*/upload/*", "-not", "-user", "scylla",
				"-type", "f").CombinedOutput()
			if err != nil || len(strings.TrimSpace(string(out))) == 0 {
				continue
			}
			// Batch chown all non-scylla files in upload dirs.
			exec.CommandContext(ctx, "sudo", "bash", "-c",
				fmt.Sprintf("find %s -path '*/upload/*' -not -user scylla -exec chown scylla:scylla {} +", dataDir),
			).Run()
		}
	}
}

// scyllaListenAddr returns the ScyllaDB listen address from scylla.yaml,
// falling back to common defaults.
func scyllaListenAddr() string {
	data, err := os.ReadFile("/etc/scylla/scylla.yaml")
	if err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "listen_address:") {
				addr := strings.TrimSpace(strings.TrimPrefix(line, "listen_address:"))
				if addr != "" {
					return addr
				}
			}
		}
	}
	// Try common addresses
	for _, addr := range []string{"127.0.0.1", "localhost"} {
		out, err := exec.Command("cqlsh", addr, "9042", "-e", "SELECT now() FROM system.local").CombinedOutput()
		if err == nil && !strings.Contains(string(out), "Connection") {
			return addr
		}
	}
	return "127.0.0.1"
}

// scyllaWorkloadUnits lists systemd units for Globular services that create
// ScyllaDB keyspaces on startup.  We stop them before schema restore to prevent
// a race where they recreate empty keyspaces (without tables) before sctool
// finishes restoring the backed-up schema.
var scyllaWorkloadUnits = []string{
	"globular-persistence.service",
	"globular-resource.service",
	"globular-storage.service",
	"globular-rbac.service",
	"globular-media.service",
	"globular-log.service",
	"globular-file.service",
	"globular-title.service",
	"globular-search.service",
}

// stopScyllaWorkloadServices stops workload services that use ScyllaDB.
// Returns the list of units that were actually running (and stopped).
func stopScyllaWorkloadServices(ctx context.Context, outputs map[string]string) []string {
	var stopped []string
	for _, unit := range scyllaWorkloadUnits {
		// Check if the unit is active before stopping (no sudo needed for is-active)
		if err := exec.CommandContext(ctx, "systemctl", "is-active", "--quiet", unit).Run(); err != nil {
			continue // not running
		}
		log.Printf("scylla restore: stopping %s before schema restore", unit)
		if out, err := exec.CommandContext(ctx, "sudo", "systemctl", "stop", unit).CombinedOutput(); err != nil {
			log.Printf("scylla restore: failed to stop %s: %s", unit, string(out))
		} else {
			stopped = append(stopped, unit)
		}
	}
	if len(stopped) > 0 {
		outputs["stopped_services"] = strings.Join(stopped, ",")
		log.Printf("scylla restore: stopped %d workload services: %s", len(stopped), strings.Join(stopped, ", "))
		// Give services a moment to fully shut down
		select {
		case <-ctx.Done():
		case <-time.After(3 * time.Second):
		}
	}
	return stopped
}

// startScyllaWorkloadServices restarts the units that were stopped.
func startScyllaWorkloadServices(ctx context.Context, units []string, outputs map[string]string) {
	if len(units) == 0 {
		return
	}
	log.Printf("scylla restore: restarting %d workload services", len(units))
	for _, unit := range units {
		if out, err := exec.CommandContext(ctx, "sudo", "systemctl", "start", unit).CombinedOutput(); err != nil {
			log.Printf("scylla restore: failed to restart %s: %s", unit, string(out))
		} else {
			log.Printf("scylla restore: restarted %s", unit)
		}
	}
	outputs["restarted_services"] = strings.Join(units, ",")
}

// verifyUserKeyspacesExist checks that at least one non-system keyspace exists.
func verifyUserKeyspacesExist(ctx context.Context, outputs map[string]string) bool {
	cqlHost := scyllaListenAddr()
	out, err := exec.CommandContext(ctx, "cqlsh", cqlHost, "9042",
		"-e", "DESCRIBE KEYSPACES").CombinedOutput()
	if err != nil {
		log.Printf("scylla restore: failed to verify keyspaces: %s", string(out))
		return false
	}

	systemKS := map[string]bool{
		"system": true, "system_schema": true, "system_auth": true,
		"system_distributed": true, "system_distributed_everywhere": true,
		"system_traces": true, "system_virtual_schema": true,
		"scylla_manager": true,
	}

	var userKS []string
	for _, ks := range strings.Fields(string(out)) {
		ks = strings.TrimSpace(ks)
		if ks == "" || systemKS[ks] || strings.HasPrefix(ks, "system") {
			continue
		}
		userKS = append(userKS, ks)
	}
	if len(userKS) > 0 {
		outputs["verified_keyspaces"] = strings.Join(userKS, ",")
		log.Printf("scylla restore: found %d user keyspaces: %s", len(userKS), strings.Join(userKS, ", "))
		return true
	}
	log.Printf("scylla restore: no user keyspaces found")
	return false
}

// schemaEntry represents one entry in the sctool schema JSON backup file.
type schemaEntry struct {
	Keyspace string `json:"keyspace"`
	Type     string `json:"type"`
	Name     string `json:"name"`
	CQLStmt  string `json:"cql_stmt"`
}

// restoreSchemaFromBackup downloads the schema JSON from the backup in MinIO,
// filters out scylla_manager entries, and applies CQL statements via cqlsh.
// This bypasses sctool --restore-schema which always fails because the backup
// includes scylla_manager schema and that keyspace already exists.
func restoreSchemaFromBackup(ctx context.Context, snapshotTag, locations string, outputs map[string]string) error {
	// Parse the S3 bucket from locations (format: "s3:bucket-name")
	bucket := ""
	for _, loc := range strings.Split(locations, ",") {
		loc = strings.TrimSpace(loc)
		if strings.HasPrefix(loc, "s3:") {
			bucket = strings.TrimPrefix(loc, "s3:")
			break
		}
	}
	if bucket == "" {
		return fmt.Errorf("no s3: location found in locations=%q", locations)
	}

	// Read MinIO credentials from scylla-manager-agent config
	agentCfgPath := "/var/lib/globular/scylla-manager-agent/scylla-manager-agent.yaml"
	minioEndpoint := "127.0.0.1:9000"
	minioAccessKey := "globular"
	minioSecretKey := "globularadmin"
	if data, err := os.ReadFile(agentCfgPath); err == nil {
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "endpoint:") {
				ep := strings.TrimSpace(strings.TrimPrefix(line, "endpoint:"))
				ep = strings.Trim(ep, "\"'")
				ep = strings.TrimPrefix(ep, "https://")
				ep = strings.TrimPrefix(ep, "http://")
				if ep != "" {
					minioEndpoint = ep
				}
			}
			if strings.HasPrefix(line, "access_key_id:") {
				v := strings.TrimSpace(strings.TrimPrefix(line, "access_key_id:"))
				if v != "" {
					minioAccessKey = v
				}
			}
			if strings.HasPrefix(line, "secret_access_key:") {
				v := strings.TrimSpace(strings.TrimPrefix(line, "secret_access_key:"))
				if v != "" {
					minioSecretKey = v
				}
			}
		}
	}

	// Set up mc alias
	log.Printf("scylla restore: setting up mc alias for MinIO at %s", minioEndpoint)
	setupOut, err := exec.CommandContext(ctx, "mc", "alias", "set", "globular-restore",
		"https://"+minioEndpoint, minioAccessKey, minioSecretKey, "--insecure").CombinedOutput()
	if err != nil {
		return fmt.Errorf("mc alias set failed: %s", string(setupOut))
	}

	// Find schema file matching snapshot tag
	log.Printf("scylla restore: searching for schema file with tag %s in bucket %s", snapshotTag, bucket)
	findOut, err := exec.CommandContext(ctx, "mc", "find",
		"globular-restore/"+bucket+"/backup/schema/",
		"--name", "*"+snapshotTag+"*", "--insecure").CombinedOutput()
	if err != nil {
		return fmt.Errorf("mc find failed: %s", string(findOut))
	}

	schemaPath := ""
	for _, line := range strings.Split(strings.TrimSpace(string(findOut)), "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, snapshotTag) && strings.HasSuffix(line, ".json.gz") {
			schemaPath = line
			break
		}
	}
	if schemaPath == "" {
		return fmt.Errorf("no schema file found for tag %s", snapshotTag)
	}
	log.Printf("scylla restore: found schema file: %s", schemaPath)

	// Download to temp file
	tmpDir := os.TempDir()
	gzPath := filepath.Join(tmpDir, "scylla_schema_restore.json.gz")
	defer os.Remove(gzPath)
	cpOut, err := exec.CommandContext(ctx, "mc", "cp", schemaPath, gzPath, "--insecure").CombinedOutput()
	if err != nil {
		return fmt.Errorf("mc cp failed: %s", string(cpOut))
	}

	// Read and decompress
	gzFile, err := os.Open(gzPath)
	if err != nil {
		return fmt.Errorf("open schema gz: %w", err)
	}
	defer gzFile.Close()

	gzReader, err := gzip.NewReader(gzFile)
	if err != nil {
		return fmt.Errorf("gzip reader: %w", err)
	}
	defer gzReader.Close()

	jsonData, err := io.ReadAll(gzReader)
	if err != nil {
		return fmt.Errorf("read schema json: %w", err)
	}

	// Parse JSON
	var entries []schemaEntry
	if err := json.Unmarshal(jsonData, &entries); err != nil {
		return fmt.Errorf("parse schema json: %w", err)
	}

	// Filter: skip scylla_manager, system keyspaces, and roles
	skipKeyspaces := map[string]bool{
		"scylla_manager": true,
		"system":         true, "system_schema": true, "system_auth": true,
		"system_distributed": true, "system_distributed_everywhere": true,
		"system_traces": true, "system_virtual_schema": true,
	}

	var cqlStatements []string
	for _, e := range entries {
		if skipKeyspaces[e.Keyspace] || strings.HasPrefix(e.Keyspace, "system") {
			continue
		}
		if e.Type == "role" {
			continue // skip roles
		}
		cqlStatements = append(cqlStatements, e.CQLStmt)
	}

	if len(cqlStatements) == 0 {
		return fmt.Errorf("no user schema CQL statements found in backup")
	}

	log.Printf("scylla restore: applying %d CQL statements via cqlsh (skipped scylla_manager)", len(cqlStatements))
	outputs["schema_statements"] = fmt.Sprintf("%d", len(cqlStatements))

	// Write CQL to temp file and execute via cqlsh
	cqlHost := scyllaListenAddr()
	cqlPath := filepath.Join(tmpDir, "scylla_schema_restore.cql")
	defer os.Remove(cqlPath)

	cqlContent := strings.Join(cqlStatements, "\n")
	if err := os.WriteFile(cqlPath, []byte(cqlContent), 0600); err != nil {
		return fmt.Errorf("write cql file: %w", err)
	}

	cqlOut, err := exec.CommandContext(ctx, "cqlsh", cqlHost, "9042", "-f", cqlPath).CombinedOutput()
	cqlOutStr := strings.TrimSpace(string(cqlOut))
	if cqlOutStr != "" {
		outputs["cqlsh_output"] = cqlOutStr
		log.Printf("scylla restore: cqlsh output: %s", cqlOutStr)
	}
	if err != nil {
		return fmt.Errorf("cqlsh apply schema failed: %s", cqlOutStr)
	}

	log.Printf("scylla restore: schema applied successfully via cqlsh")
	outputs["schema_restored"] = "direct_cqlsh"

	// Wait for schema to propagate
	select {
	case <-ctx.Done():
		return fmt.Errorf("context cancelled waiting for schema propagation")
	case <-time.After(5 * time.Second):
	}

	return nil
}

// dropUserKeyspaces drops non-system keyspaces so that --restore-schema can
// recreate them cleanly.  On a fresh install the resource service may have
// already created empty keyspaces, causing "Cannot add existing keyspace".
func dropUserKeyspaces(ctx context.Context, cluster, apiURL string, outputs map[string]string) {
	cqlHost := scyllaListenAddr()
	log.Printf("scylla restore: using cqlsh host %s for keyspace cleanup", cqlHost)

	// List keyspaces via cqlsh
	out, err := exec.CommandContext(ctx, "cqlsh", cqlHost, "9042",
		"-e", "DESCRIBE KEYSPACES").CombinedOutput()
	if err != nil {
		log.Printf("scylla restore: failed to list keyspaces for cleanup: %s", string(out))
		return
	}

	systemKS := map[string]bool{
		"system": true, "system_schema": true, "system_auth": true,
		"system_distributed": true, "system_distributed_everywhere": true,
		"system_traces": true, "system_virtual_schema": true,
		"scylla_manager": true,
	}

	var dropped []string
	for _, ks := range strings.Fields(string(out)) {
		ks = strings.TrimSpace(ks)
		if ks == "" || systemKS[ks] || strings.HasPrefix(ks, "system") {
			continue
		}
		log.Printf("scylla restore: dropping keyspace %s before schema restore", ks)
		dropOut, dropErr := exec.CommandContext(ctx, "cqlsh", cqlHost, "9042",
			"-e", fmt.Sprintf("DROP KEYSPACE IF EXISTS %s", ks)).CombinedOutput()
		if dropErr != nil {
			log.Printf("scylla restore: failed to drop keyspace %s: %s", ks, string(dropOut))
		} else {
			dropped = append(dropped, ks)
		}
	}
	if len(dropped) > 0 {
		outputs["dropped_keyspaces"] = strings.Join(dropped, ",")
		log.Printf("scylla restore: dropped %d user keyspaces: %s", len(dropped), strings.Join(dropped, ", "))
		// Wait for schema to settle after drops
		select {
		case <-ctx.Done():
		case <-time.After(5 * time.Second):
		}
	}
}

// pollScyllaTask polls a sctool task until it completes (DONE/ERROR/ABORTED).
// Returns true if the task completed successfully (DONE).
func pollScyllaTask(ctx context.Context, taskID, cluster, apiURL string, outputs map[string]string, label string) bool {
	progressArgs := []string{"task", "progress", taskID, "--cluster", cluster}
	if apiURL != "" && apiURL != "http://127.0.0.1:5080" {
		progressArgs = append(progressArgs, "--api-url", apiURL)
	}
	for pollCount := 0; pollCount < 120; pollCount++ { // up to 20 min
		select {
		case <-ctx.Done():
			return false
		case <-time.After(10 * time.Second):
		}
		pOut, pErr := exec.CommandContext(ctx, "sctool", progressArgs...).CombinedOutput()
		if pErr != nil {
			continue
		}
		pStr := string(pOut)
		outputs[label+"_progress"] = strings.TrimSpace(pStr)
		statusLine := extractScyllaStatusLine(pStr)
		log.Printf("scylla restore: %s task status: %s", label, strings.TrimSpace(statusLine))
		if strings.Contains(statusLine, "DONE") {
			log.Printf("scylla restore: %s task completed", label)
			return true
		}
		if strings.Contains(statusLine, "ERROR") || strings.Contains(statusLine, "ABORTED") {
			return false
		}
	}
	return false
}

// extractScyllaTaskID extracts a task ID from sctool output.
// sctool restore prints lines like "restore/xxxxxxxx-xxxx-..." or just the task UUID.
func extractScyllaTaskID(output string) string {
	for _, line := range strings.Split(output, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "restore/") {
			return line
		}
		// Some versions just print the task ID on a line by itself
		if len(line) >= 36 && strings.Count(line, "-") == 4 {
			return line
		}
	}
	return ""
}

// extractScyllaStatusLine finds the "Status:" line from sctool progress output.
func extractScyllaStatusLine(output string) string {
	for _, line := range strings.Split(output, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "Status:") {
			return line
		}
	}
	return ""
}

// extractScyllaCauseLine finds the "Cause:" line from sctool progress output.
// Example: "Cause:\t\tvalidate free disk space: not enough disk space"
func extractScyllaCauseLine(output string) string {
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Cause:") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "Cause:"))
		}
	}
	return ""
}

// parseScyllaRestoreBytes extracts transferred bytes from sctool progress output.
func parseScyllaRestoreBytes(output string) uint64 {
	var maxBytes uint64
	for _, line := range strings.Split(output, "\n") {
		for _, field := range strings.Fields(line) {
			b := parseScyllaSize(field)
			if b > maxBytes {
				maxBytes = b
			}
		}
	}
	return maxBytes
}

// parseScyllaSize parses a human-readable size like "123.45MiB" into bytes.
func parseScyllaSize(s string) uint64 {
	s = strings.TrimSpace(s)
	type suffix struct {
		s string
		m float64
	}
	for _, sf := range []suffix{
		{"TiB", 1024 * 1024 * 1024 * 1024},
		{"GiB", 1024 * 1024 * 1024},
		{"MiB", 1024 * 1024},
		{"KiB", 1024},
		{"TB", 1e12}, {"GB", 1e9}, {"MB", 1e6}, {"KB", 1e3},
		{"B", 1},
	} {
		if strings.HasSuffix(s, sf.s) {
			numStr := strings.TrimSuffix(s, sf.s)
			var val float64
			if _, err := fmt.Sscanf(numStr, "%f", &val); err == nil && val > 0 {
				return uint64(val * sf.m)
			}
		}
	}
	return 0
}

// deduplicateScyllaClusters removes stale scylla-manager cluster entries
// when multiple clusters share the same name. It parses both "ID" and "Name"
// columns from "sctool cluster list", groups by name, and for duplicates
// keeps the one whose healthchecks are still running (most recent "Next"
// scheduled). Returns the IDs that were deleted.
func deduplicateScyllaClusters(ctx context.Context, targetName, apiURL string) []string {
	args := []string{"cluster", "list"}
	if apiURL != "" && apiURL != "http://127.0.0.1:5080" {
		args = append(args, "--api-url", apiURL)
	}
	out, err := exec.CommandContext(ctx, "sctool", args...).CombinedOutput()
	if err != nil {
		return nil
	}

	entries := parseScyllaClusterListFull(string(out))

	// Group by name, only care about the target name.
	var matching []scyllaClusterEntry
	for _, e := range entries {
		if e.name == targetName {
			matching = append(matching, e)
		}
	}
	if len(matching) <= 1 {
		return nil // no duplicates
	}

	// Find the "active" cluster: the one whose healthchecks have a future
	// "Next" run scheduled. Check each cluster's tasks.
	activeID := ""
	for _, e := range matching {
		taskArgs := []string{"tasks", "-c", e.id}
		if apiURL != "" && apiURL != "http://127.0.0.1:5080" {
			taskArgs = append(taskArgs, "--api-url", apiURL)
		}
		tOut, tErr := exec.CommandContext(ctx, "sctool", taskArgs...).CombinedOutput()
		if tErr != nil {
			continue
		}
		tStr := string(tOut)
		// Active cluster has healthcheck tasks with "Next" times (not empty).
		// Look for healthcheck rows with a non-empty Next column.
		for _, line := range strings.Split(tStr, "\n") {
			if strings.Contains(line, "healthcheck/") && strings.Contains(line, "DONE") {
				// Check if the line has a Next timestamp (column after Status).
				// A stale cluster's healthchecks have no Next time.
				fields := strings.Fields(line)
				// The last field before the end is typically the Next timestamp
				// for active clusters, or empty for stale ones.
				for _, f := range fields {
					// Future timestamps contain the current or next year
					if strings.Contains(f, "Mar") || strings.Contains(f, "Apr") ||
						strings.Contains(f, "May") || strings.Contains(f, "Jun") ||
						strings.Contains(f, "2026") || strings.Contains(f, "2027") {
						// Has a Next scheduled time — this is the active cluster
						activeID = e.id
						break
					}
				}
				if activeID != "" {
					break
				}
			}
		}
		if activeID != "" {
			break
		}
	}

	// If we couldn't determine the active one, keep the last registered (newest).
	if activeID == "" {
		activeID = matching[len(matching)-1].id
	}

	// Delete all duplicates except the active one.
	var removed []string
	for _, e := range matching {
		if e.id == activeID {
			continue
		}
		delArgs := []string{"cluster", "delete", "-c", e.id}
		if apiURL != "" && apiURL != "http://127.0.0.1:5080" {
			delArgs = append(delArgs, "--api-url", apiURL)
		}
		if err := exec.CommandContext(ctx, "sctool", delArgs...).Run(); err != nil {
			log.Printf("deduplicateScyllaClusters: failed to delete cluster %s: %v", e.id, err)
			continue
		}
		removed = append(removed, e.id)
		log.Printf("deduplicateScyllaClusters: deleted stale cluster %s (name=%s)", e.id, e.name)
	}
	return removed
}

type scyllaClusterEntry struct {
	id, name string
}

// syncScyllaManagerAuthToken reads the scylla-manager-agent's current auth_token
// from its config file and updates the scylla-manager cluster entry to use it.
// This fixes "HTTP 401 unauthorized" errors after a restore where the agent's
// token was regenerated but scylla-manager still has the old one.
func syncScyllaManagerAuthToken(ctx context.Context, cluster, apiURL string, outputs map[string]string) error {
	// Read the agent's auth_token from its config file.
	// The file is owned by scylla:globular with mode 0640, and node-agent
	// runs as globular (supplementary group), so we can read it.
	agentCfgPath := "/var/lib/globular/scylla-manager-agent/scylla-manager-agent.yaml"
	data, err := os.ReadFile(agentCfgPath)
	if err != nil {
		// Try sudo cat as fallback — agent config is owned by scylla user.
		out, err2 := exec.CommandContext(ctx, "sudo", "cat", agentCfgPath).Output()
		if err2 != nil {
			return fmt.Errorf("cannot read agent config: %v (direct: %v)", err2, err)
		}
		data = out
	}

	// Parse auth_token from the YAML (simple line parsing to avoid a YAML dependency).
	var token string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "auth_token:") {
			token = strings.TrimSpace(strings.TrimPrefix(line, "auth_token:"))
			// Remove surrounding quotes if present.
			token = strings.Trim(token, "\"'")
			break
		}
	}
	if token == "" {
		return fmt.Errorf("no auth_token found in %s", agentCfgPath)
	}

	// Update the cluster in scylla-manager with the current token.
	args := []string{"cluster", "update", "--cluster", cluster, "--auth-token", token}
	if apiURL != "" && apiURL != "http://127.0.0.1:5080" {
		args = append(args, "--api-url", apiURL)
	}
	out, err := exec.CommandContext(ctx, "sctool", args...).CombinedOutput()
	outStr := strings.TrimSpace(string(out))
	if err != nil {
		outputs["auth_sync_error"] = outStr
		return fmt.Errorf("sctool cluster update failed: %s", outStr)
	}
	log.Printf("scylla restore: auth token synced for cluster %s", cluster)
	outputs["auth_token_synced"] = "true"
	return nil
}

// parseScyllaClusterListFull parses both ID and Name columns from sctool cluster list output.
func parseScyllaClusterListFull(output string) []scyllaClusterEntry {
	var results []scyllaClusterEntry

	lines := strings.Split(output, "\n")
	idColIdx, nameColIdx := -1, -1
	headerLineIdx := -1

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var sep string
		if strings.Contains(line, "│") {
			sep = "│"
		} else if strings.Contains(line, "|") {
			sep = "|"
		} else {
			continue
		}
		parts := strings.Split(line, sep)
		for j, p := range parts {
			col := strings.TrimSpace(p)
			if col == "ID" {
				idColIdx = j
			}
			if col == "Name" {
				nameColIdx = j
			}
		}
		if idColIdx >= 0 && nameColIdx >= 0 {
			headerLineIdx = i
			break
		}
	}
	if headerLineIdx < 0 || idColIdx < 0 || nameColIdx < 0 {
		return nil
	}

	for i := headerLineIdx + 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		// Skip border lines
		isBorder := true
		for _, r := range line {
			if r != '─' && r != '┼' && r != '├' && r != '┤' && r != '╰' && r != '╯' &&
				r != '┴' && r != '-' && r != '+' && r != ' ' {
				isBorder = false
				break
			}
		}
		if isBorder {
			continue
		}

		var sep string
		if strings.Contains(line, "│") {
			sep = "│"
		} else if strings.Contains(line, "|") {
			sep = "|"
		} else {
			continue
		}
		parts := strings.Split(line, sep)
		if idColIdx < len(parts) && nameColIdx < len(parts) {
			id := strings.TrimSpace(parts[idColIdx])
			name := strings.TrimSpace(parts[nameColIdx])
			if id != "" && name != "" {
				results = append(results, scyllaClusterEntry{id: id, name: name})
			}
		}
	}
	return results
}

// --- minio/rclone restore ---

func (s *NodeAgentServer) restoreMinioProvider(ctx context.Context, req *node_agentpb.RunRestoreProviderRequest) *node_agentpb.BackupProviderResult {
	outputs := make(map[string]string)
	opts := req.Spec.Options
	if opts == nil {
		opts = make(map[string]string)
	}

	remote := opts["remote"]
	source := opts["source"]
	if remote == "" || source == "" {
		return restoreFail("minio", "remote and source options are required", outputs)
	}

	outputs["remote"] = remote
	outputs["source"] = source

	log.Printf("minio restore: syncing %s -> %s", remote, source)
	cmd := exec.CommandContext(ctx, "rclone", "sync", remote, source, "--stats-one-line", "-v")
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	err := cmd.Run()
	stdoutStr := strings.TrimSpace(outBuf.String())
	stderrStr := strings.TrimSpace(errBuf.String())
	outputs["stdout"] = stdoutStr
	if stderrStr != "" {
		outputs["stderr"] = stderrStr
	}

	if err != nil {
		detail := stderrStr
		if detail == "" {
			detail = stdoutStr
		}
		return restoreFail("minio", fmt.Sprintf("rclone restore failed: %s", detail), outputs)
	}

	return &node_agentpb.BackupProviderResult{
		Provider: "minio",
		Ok:       true,
		Summary:  fmt.Sprintf("rclone data restored from %s to %s", remote, source),
		Outputs:  outputs,
	}
}

// --- helpers ---

func restoreFail(provider, msg string, outputs map[string]string) *node_agentpb.BackupProviderResult {
	return &node_agentpb.BackupProviderResult{
		Provider:     provider,
		Ok:           false,
		Summary:      msg,
		ErrorMessage: msg,
		Outputs:      outputs,
	}
}

func runRestore(ctx context.Context, name string, args ...string) (stdout, stderr string, err error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err = cmd.Run()
	return strings.TrimSpace(outBuf.String()), strings.TrimSpace(errBuf.String()), err
}

func fileExistsNA(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// parseResticRestoredBytes parses byte count from restic restore output.
// Restic prints lines like "Summary: Restored 1234 files, 567.890 MiB".
func parseResticRestoredBytes(line string) uint64 {
	// Look for size patterns like "567.8 MiB", "1.2 GiB", "456 KiB"
	multipliers := []struct {
		suffix string
		mult   float64
	}{
		{"TiB", 1024 * 1024 * 1024 * 1024},
		{"GiB", 1024 * 1024 * 1024},
		{"MiB", 1024 * 1024},
		{"KiB", 1024},
		{"B", 1},
	}
	for _, m := range multipliers {
		idx := strings.Index(line, m.suffix)
		if idx < 1 {
			continue
		}
		// Walk backwards to find the number
		numEnd := idx
		for numEnd > 0 && line[numEnd-1] == ' ' {
			numEnd--
		}
		numStart := numEnd
		for numStart > 0 && (line[numStart-1] >= '0' && line[numStart-1] <= '9' || line[numStart-1] == '.') {
			numStart--
		}
		if numStart == numEnd {
			continue
		}
		var val float64
		if _, err := fmt.Sscanf(line[numStart:numEnd], "%f", &val); err == nil && val > 0 {
			return uint64(val * m.mult)
		}
	}
	return 0
}
