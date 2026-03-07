package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/globulario/services/golang/backup_manager/backup_managerpb"
)

// runProvider executes a backup provider, writing outputs into the capsule.
func (srv *server) runProvider(ctx context.Context, spec *backup_managerpb.BackupProviderSpec, cc *CapsuleContext) *backup_managerpb.BackupProviderResult {
	start := time.Now().UnixMilli()
	name := providerName(spec.Type)

	// Apply provider-level timeout
	timeoutSec := int(spec.TimeoutSeconds)
	if timeoutSec <= 0 {
		timeoutSec = srv.ProviderTimeoutSeconds
	}
	if timeoutSec > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
		defer cancel()
	}

	slog.Info("running backup provider", "provider", name, "backup_id", cc.BackupID, "timeout_s", timeoutSec)

	var result *backup_managerpb.BackupProviderResult

	switch spec.Type {
	case backup_managerpb.BackupProviderType_BACKUP_PROVIDER_ETCD:
		result = srv.runEtcdBackup(ctx, spec, cc)
	case backup_managerpb.BackupProviderType_BACKUP_PROVIDER_RESTIC:
		result = srv.runResticBackup(ctx, spec, cc)
	case backup_managerpb.BackupProviderType_BACKUP_PROVIDER_MINIO:
		result = srv.runMinioBackup(ctx, spec, cc)
	case backup_managerpb.BackupProviderType_BACKUP_PROVIDER_SCYLLA:
		result = srv.runScyllaBackup(ctx, spec, cc)
	default:
		result = &backup_managerpb.BackupProviderResult{
			Type:         spec.Type,
			Enabled:      true,
			State:        backup_managerpb.BackupJobState_BACKUP_JOB_FAILED,
			Severity:     backup_managerpb.BackupSeverity_BACKUP_SEVERITY_ERROR,
			Summary:      fmt.Sprintf("unknown provider type: %v", spec.Type),
			ErrorMessage: "unsupported provider",
			Outputs:      make(map[string]string),
		}
	}

	result.StartedUnixMs = start
	result.FinishedUnixMs = time.Now().UnixMilli()

	dur := result.FinishedUnixMs - start
	if result.State == backup_managerpb.BackupJobState_BACKUP_JOB_SUCCEEDED {
		slog.Info("backup provider completed", "provider", name, "duration_ms", dur)
	} else {
		slog.Warn("backup provider failed", "provider", name, "duration_ms", dur, "error", result.ErrorMessage)
	}

	return result
}

// optOrDefault returns spec.Options[key] if set, otherwise fallback.
func optOrDefault(spec *backup_managerpb.BackupProviderSpec, key, fallback string) string {
	if spec.Options != nil {
		if v, ok := spec.Options[key]; ok && v != "" {
			return v
		}
	}
	return fallback
}

// runCmd executes a command and returns stdout, stderr, and error.
func runCmd(name string, args ...string) (stdout, stderr string, err error) {
	cmd := exec.Command(name, args...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err = cmd.Run()
	return outBuf.String(), errBuf.String(), err
}

// runCmdCtx executes a command with context cancellation support.
func runCmdCtx(ctx context.Context, name string, args ...string) (stdout, stderr string, err error) {
	cmd := exec.CommandContext(ctx, name, args...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err = cmd.Run()
	return outBuf.String(), errBuf.String(), err
}

// --- ETCD PROVIDER ---
// Writes snapshot into capsule: payload/etcd/etcd-snapshot.db
// Writes verification output: provider/etcd/status.txt

func (srv *server) runEtcdBackup(ctx context.Context, spec *backup_managerpb.BackupProviderSpec, cc *CapsuleContext) *backup_managerpb.BackupProviderResult {
	outputs := make(map[string]string)
	outputs["method"] = "etcdctl snapshot save"

	endpoints := optOrDefault(spec, "endpoints", srv.EtcdEndpoints)
	cacert := optOrDefault(spec, "cacert", srv.EtcdCACert)
	cert := optOrDefault(spec, "cert", srv.EtcdCert)
	key := optOrDefault(spec, "key", srv.EtcdKey)

	// Try ca.crt fallback if ca.pem is not readable
	if !fileExists(cacert) {
		alt := filepath.Join(filepath.Dir(cacert), "ca.crt")
		if fileExists(alt) {
			cacert = alt
		}
	}

	// If TLS certs exist and the endpoint has no scheme, prefix with https://
	// so etcdctl uses TLS instead of plaintext.
	hasTLS := fileExists(cacert)
	if hasTLS && !strings.Contains(endpoints, "://") {
		parts := strings.Split(endpoints, ",")
		for i, ep := range parts {
			ep = strings.TrimSpace(ep)
			if !strings.Contains(ep, "://") {
				parts[i] = "https://" + ep
			}
		}
		endpoints = strings.Join(parts, ",")
	}

	// Snapshot goes into the capsule payload dir
	snapshotFile := filepath.Join(cc.PayloadDir, "etcd-snapshot.db")
	outputs["snapshot_path"] = snapshotFile
	outputs["endpoints"] = endpoints

	args := []string{
		"snapshot", "save", snapshotFile,
		"--endpoints", endpoints,
	}

	if fileExists(cacert) {
		args = append(args, "--cacert", cacert)
	}
	if fileExists(cert) {
		args = append(args, "--cert", cert)
	}
	if fileExists(key) {
		args = append(args, "--key", key)
	}

	// Set ETCDCTL_API only; do NOT set ETCDCTL_ENDPOINTS/CACERT/CERT/KEY
	// because etcdctl fatally errors when both a flag and its env var are set.
	etcdEnv := append(os.Environ(), "ETCDCTL_API=3")

	slog.Info("running etcdctl", "args", args)
	saveCmd := exec.CommandContext(ctx, "etcdctl", args...)
	saveCmd.Env = etcdEnv
	var saveBuf, saveErrBuf bytes.Buffer
	saveCmd.Stdout = &saveBuf
	saveCmd.Stderr = &saveErrBuf
	err := saveCmd.Run()
	stdout := saveBuf.String()
	stderr := saveErrBuf.String()

	outputs["stdout"] = strings.TrimSpace(stdout)
	if stderr != "" {
		outputs["stderr"] = strings.TrimSpace(stderr)
	}

	if err != nil {
		outputs["exit_error"] = err.Error()
		// Write log even on failure
		_ = CapsuleWriteFile(cc.ProviderDir, "log.txt", []byte(stdout+"\n"+stderr))
		detail := strings.TrimSpace(stderr)
		if detail == "" {
			detail = err.Error()
		}
		return failResult(spec.Type, fmt.Sprintf("etcdctl snapshot save failed: %s", detail), outputs)
	}

	// Get snapshot file size
	var snapshotBytes uint64
	if info, statErr := os.Stat(snapshotFile); statErr == nil {
		snapshotBytes = uint64(info.Size())
		outputs["snapshot_bytes"] = fmt.Sprintf("%d", snapshotBytes)
	}

	// Verify the snapshot and write status to provider dir
	verifyArgs := []string{"snapshot", "status", snapshotFile, "--write-out", "table"}
	verifyCmd := exec.CommandContext(ctx, "etcdctl", verifyArgs...)
	verifyCmd.Env = etcdEnv
	var verifyBuf bytes.Buffer
	verifyCmd.Stdout = &verifyBuf
	verifyCmd.Stderr = &verifyBuf
	verifyErr := verifyCmd.Run()
	verifyOut := verifyBuf.String()
	if verifyErr == nil {
		outputs["verify_status"] = strings.TrimSpace(verifyOut)
		_ = CapsuleWriteFile(cc.ProviderDir, "status.txt", []byte(verifyOut))
	}

	// Write combined log
	_ = CapsuleWriteFile(cc.ProviderDir, "log.txt", []byte(stdout+"\n"+stderr+"\n"+verifyOut))

	result := successResult(spec.Type, "etcd snapshot saved", outputs)
	result.BytesWritten = snapshotBytes
	result.PayloadFiles = []string{"payload/etcd/etcd-snapshot.db"}
	result.OutputFiles = []string{"provider/etcd/status.txt", "provider/etcd/log.txt"}
	result.RestoreInputs = map[string]string{
		"snapshot_path": "payload/etcd/etcd-snapshot.db",
		"data_dir":      "/var/lib/etcd",
	}
	return result
}

// --- RESTIC PROVIDER ---
// Writes run output: provider/restic/run.json, provider/restic/log.txt
// Records snapshot_id in outputs.

func (srv *server) runResticBackup(ctx context.Context, spec *backup_managerpb.BackupProviderSpec, cc *CapsuleContext) *backup_managerpb.BackupProviderResult {
	outputs := make(map[string]string)
	outputs["method"] = "restic backup"

	repo := optOrDefault(spec, "repo", srv.ResticRepo)
	password := optOrDefault(spec, "password", srv.ResticPassword)
	pathsStr := optOrDefault(spec, "paths", srv.ResticPaths)

	if repo == "" {
		return failResult(spec.Type, "restic repo not configured", outputs)
	}

	outputs["repo"] = repo

	env := []string{
		"RESTIC_REPOSITORY=" + repo,
		"RESTIC_PASSWORD=" + password,
	}

	// Initialize repo if it doesn't exist
	initCmd := exec.CommandContext(ctx, "restic", "init")
	initCmd.Env = append(os.Environ(), env...)
	initOut, _ := initCmd.CombinedOutput()
	initMsg := strings.TrimSpace(string(initOut))
	if !strings.Contains(initMsg, "created") && !strings.Contains(initMsg, "already") {
		slog.Debug("restic init output", "output", initMsg)
	}

	// Build backup paths
	paths := strings.Split(pathsStr, ",")
	var validPaths []string
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p != "" && fileOrDirExists(p) {
			validPaths = append(validPaths, p)
		}
	}

	if len(validPaths) == 0 {
		return failResult(spec.Type, "no valid backup paths found", outputs)
	}

	outputs["paths"] = strings.Join(validPaths, ",")

	// Exclude the restic repo itself and backup artifacts to avoid
	// backing up transient temp files that disappear during the run.
	args := []string{"backup", "--json",
		"--exclude", repo,
		"--exclude", srv.DataDir + "/artifacts",
	}
	args = append(args, validPaths...)
	cmd := exec.CommandContext(ctx, "restic", args...)
	cmd.Env = append(os.Environ(), env...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	slog.Info("running restic backup", "paths", validPaths, "repo", repo)
	err := cmd.Run()

	stdoutStr := strings.TrimSpace(outBuf.String())
	stderrStr := strings.TrimSpace(errBuf.String())
	outputs["stdout"] = stdoutStr
	if stderrStr != "" {
		outputs["stderr"] = stderrStr
	}

	// Write run output into capsule
	_ = CapsuleWriteFile(cc.ProviderDir, "run.json", []byte(stdoutStr))
	_ = CapsuleWriteFile(cc.ProviderDir, "log.txt", []byte(stderrStr))

	if err != nil {
		// Restic exit code 3 = warnings (e.g. a file vanished during backup)
		// but the snapshot was still created successfully. Treat as success.
		exitCode := 0
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		}
		if exitCode == 3 {
			slog.Warn("restic backup completed with warnings", "stderr", stderrStr)
			outputs["warnings"] = stderrStr
		} else {
			outputs["exit_error"] = err.Error()
			detail := strings.TrimSpace(stderrStr)
			if detail == "" {
				detail = err.Error()
			}
			return failResult(spec.Type, fmt.Sprintf("restic backup failed: %s", detail), outputs)
		}
	}

	// Get latest snapshot ID
	snapCmd := exec.CommandContext(ctx, "restic", "snapshots", "--latest", "1", "--json", "--repo", repo)
	snapCmd.Env = append(os.Environ(), env...)
	snapOut, snapErr := snapCmd.CombinedOutput()
	if snapErr != nil {
		slog.Warn("restic snapshots command failed", "error", snapErr, "output", strings.TrimSpace(string(snapOut)))
	} else {
		snapJSON := strings.TrimSpace(string(snapOut))
		outputs["latest_snapshot"] = snapJSON
		_ = CapsuleWriteFile(cc.ProviderDir, "snapshot.json", []byte(snapJSON))

		// Parse snapshot ID from JSON array
		var snapshots []struct {
			ID      string `json:"id"`
			ShortID string `json:"short_id"`
		}
		if err := json.Unmarshal([]byte(snapJSON), &snapshots); err != nil {
			slog.Warn("failed to parse restic snapshot JSON", "error", err, "json", snapJSON[:min(len(snapJSON), 200)])
		} else if len(snapshots) > 0 {
			if snapshots[0].ShortID != "" {
				outputs["snapshot_id"] = snapshots[0].ShortID
			} else if snapshots[0].ID != "" {
				outputs["snapshot_id"] = snapshots[0].ID[:8]
			}
			slog.Info("restic snapshot recorded", "snapshot_id", outputs["snapshot_id"])
		} else {
			slog.Warn("restic snapshots returned empty array", "json", snapJSON)
		}
	}

	outputs["repo_path"] = repo
	outputs["paths_included"] = strings.Join(validPaths, ",")

	// Parse restic JSON output for bytes stats
	var resticBytes uint64
	for _, line := range strings.Split(stdoutStr, "\n") {
		line = strings.TrimSpace(line)
		if strings.Contains(line, `"message_type":"summary"`) {
			var summary struct {
				TotalBytesProcessed uint64 `json:"total_bytes_processed"`
				DataAdded           uint64 `json:"data_added"`
			}
			if json.Unmarshal([]byte(line), &summary) == nil {
				resticBytes = summary.TotalBytesProcessed
				if resticBytes == 0 {
					resticBytes = summary.DataAdded
				}
				outputs["total_bytes_processed"] = fmt.Sprintf("%d", summary.TotalBytesProcessed)
				outputs["data_added"] = fmt.Sprintf("%d", summary.DataAdded)
			}
		}
	}

	result := successResult(spec.Type, "restic backup completed", outputs)
	result.BytesWritten = resticBytes
	result.OutputFiles = []string{"provider/restic/run.json", "provider/restic/log.txt", "provider/restic/snapshot.json"}
	result.RestoreInputs = map[string]string{
		"snapshot_id": outputs["snapshot_id"],
		"repo":        repo,
		"target":      "/",
	}
	return result
}

// --- MINIO PROVIDER ---
// Writes logs to provider/rclone/log.txt

func (srv *server) runMinioBackup(ctx context.Context, spec *backup_managerpb.BackupProviderSpec, cc *CapsuleContext) *backup_managerpb.BackupProviderResult {
	outputs := make(map[string]string)
	outputs["method"] = "rclone sync"

	remote := optOrDefault(spec, "remote", srv.RcloneRemote)
	source := optOrDefault(spec, "source", srv.RcloneSource)

	if remote == "" {
		return failResult(spec.Type, "rclone remote not configured (set RcloneRemote in config or pass 'remote' option)", outputs)
	}

	if !fileOrDirExists(source) {
		return failResult(spec.Type, fmt.Sprintf("source path does not exist: %s", source), outputs)
	}

	outputs["source"] = source
	outputs["remote"] = remote

	args := []string{
		"sync", source, remote,
		"--stats-one-line",
		"--stats", "0",
		"-v",
	}

	slog.Info("running rclone sync", "source", source, "remote", remote)
	cmd := exec.CommandContext(ctx, "rclone", args...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()

	combined := strings.TrimSpace(outBuf.String() + "\n" + errBuf.String())
	if combined != "" {
		outputs["output"] = combined
	}

	// Write log into capsule
	_ = CapsuleWriteFile(cc.ProviderDir, "log.txt", []byte(combined))

	if err != nil {
		outputs["exit_error"] = err.Error()
		return failResult(spec.Type, fmt.Sprintf("rclone sync failed: %v", err), outputs)
	}

	result := successResult(spec.Type, "rclone sync completed", outputs)
	result.OutputFiles = []string{"provider/minio/log.txt"}
	result.RestoreInputs = map[string]string{
		"remote": remote,
		"source": source,
	}
	return result
}

// --- SCYLLA PROVIDER ---
// Writes task info to provider/scylla/task.json

func (srv *server) runScyllaBackup(ctx context.Context, spec *backup_managerpb.BackupProviderSpec, cc *CapsuleContext) *backup_managerpb.BackupProviderResult {
	outputs := make(map[string]string)
	outputs["method"] = "sctool backup"

	apiURL := optOrDefault(spec, "api_url", srv.ScyllaManagerAPI)
	cluster := optOrDefault(spec, "cluster", srv.ScyllaCluster)

	if cluster == "" {
		return failResult(spec.Type, "scylla cluster name not configured (set ScyllaCluster in config or pass 'cluster' option)", outputs)
	}

	// Derive backup locations from configured destinations.
	locations := srv.scyllaLocations()
	if override := optOrDefault(spec, "location", srv.ScyllaLocation); override != "" {
		locations = []string{override}
	}
	if len(locations) == 0 {
		return failResult(spec.Type, "no ScyllaDB-compatible backup destinations configured (sctool requires S3/GCS/Azure — local paths are not supported)", outputs)
	}

	outputs["cluster"] = cluster
	outputs["locations"] = strings.Join(locations, ",")
	outputs["api_url"] = apiURL

	// Try to find an existing backup task and start it, rather than creating
	// a new one each time (which causes "another task is running" errors).
	taskID := srv.findExistingScyllaBackupTask(cluster, apiURL)

	var stdoutStr, stderrStr string

	if taskID != "" {
		// Start the existing task
		slog.Info("starting existing sctool backup task", "task_id", taskID, "cluster", cluster)
		outputs["reused_task"] = "true"

		startArgs := []string{"start", "--cluster", cluster, taskID}
		if apiURL != "" && apiURL != "http://127.0.0.1:5080" {
			startArgs = append(startArgs, "--api-url", apiURL)
		}
		stdout, stderr, err := runCmdCtx(ctx, "sctool", startArgs...)
		stdoutStr = strings.TrimSpace(stdout)
		stderrStr = strings.TrimSpace(stderr)

		if err != nil {
			// If start fails (e.g. already running), fall through to create new
			slog.Warn("sctool start failed, will create new task", "task_id", taskID, "error", err, "stderr", stderrStr)
			taskID = ""
		}
	}

	if taskID == "" {
		// Create a new backup task
		args := []string{"backup", "--cluster", cluster}
		if apiURL != "" && apiURL != "http://127.0.0.1:5080" {
			args = append(args, "--api-url", apiURL)
		}
		for _, loc := range locations {
			args = append(args, "--location", loc)
		}

		slog.Info("running sctool backup", "cluster", cluster, "locations", strings.Join(locations, ","))
		cmd := exec.CommandContext(ctx, "sctool", args...)
		var outBuf, errBuf bytes.Buffer
		cmd.Stdout = &outBuf
		cmd.Stderr = &errBuf
		err := cmd.Run()

		stdoutStr = strings.TrimSpace(outBuf.String())
		stderrStr = strings.TrimSpace(errBuf.String())

		if err != nil {
			outputs["exit_error"] = err.Error()
			if stdoutStr != "" {
				outputs["stdout"] = stdoutStr
			}
			if stderrStr != "" {
				outputs["stderr"] = stderrStr
			}
			_ = CapsuleWriteFile(cc.ProviderDir, "log.txt", []byte(stdoutStr+"\n"+stderrStr))
			detail := stderrStr
			if detail == "" {
				detail = stdoutStr
			}
			if detail != "" {
				slog.Warn("sctool backup stderr", "output", detail)
			}
			return failResult(spec.Type, fmt.Sprintf("sctool backup failed: %v", err), outputs)
		}

		// Extract task ID from sctool output
		for _, line := range strings.Split(stdoutStr, "\n") {
			if strings.Contains(line, "backup/") {
				taskID = strings.TrimSpace(line)
				break
			}
		}
	}

	if stdoutStr != "" {
		outputs["stdout"] = stdoutStr
	}
	if stderrStr != "" {
		outputs["stderr"] = stderrStr
	}
	outputs["task_id"] = taskID

	locJoin := strings.Join(locations, ",")

	// Poll sctool task progress until complete (or context cancelled)
	var scyllaBytes uint64
	pollStatus := ""
	if taskID != "" {
		progressArgs := []string{"task", "progress", taskID, "--cluster", cluster}
		if apiURL != "" && apiURL != "http://127.0.0.1:5080" {
			progressArgs = append(progressArgs, "--api-url", apiURL)
		}

		pollCount := 0
		for {
			select {
			case <-ctx.Done():
				pollStatus = "context cancelled (timeout)"
				slog.Warn("scylla backup poll timed out", "task_id", taskID, "polls", pollCount)
				goto done
			case <-time.After(10 * time.Second):
			}
			pollCount++

			pOut, _, pErr := runCmdCtx(ctx, "sctool", progressArgs...)
			if pErr != nil {
				slog.Warn("sctool progress poll failed", "error", pErr)
				pollStatus = "poll error: " + pErr.Error()
				goto done
			}

			outputs["progress"] = pOut

			// Check the Status: line specifically (not other lines that might contain DONE)
			status := extractStatusLine(pOut)
			slog.Info("scylla backup poll", "task_id", taskID, "status", status, "poll", pollCount)

			if strings.Contains(status, "DONE") || strings.Contains(status, "ERROR") || strings.Contains(status, "ABORTED") {
				scyllaBytes = parseScyllaBytes(pOut)
				if strings.Contains(status, "ERROR") || strings.Contains(status, "ABORTED") {
					pollStatus = "task failed"
				} else {
					pollStatus = "completed"
				}
				break
			}
		}
	}
done:
	outputs["poll_status"] = pollStatus

	// Write task info into capsule
	taskJSON := fmt.Sprintf(`{"cluster":%q,"locations":%q,"task_id":%q,"output":%q}`,
		cluster, locJoin, taskID, stdoutStr)
	_ = CapsuleWriteFile(cc.ProviderDir, "task.json", []byte(taskJSON))
	_ = CapsuleWriteFile(cc.ProviderDir, "log.txt", []byte(stdoutStr+"\n"+stderrStr))

	summary := "scylla-manager backup triggered"
	if pollStatus == "completed" {
		summary = "scylla-manager backup completed"
	} else if pollStatus == "task failed" {
		// Extract detailed cause from sctool progress output
		cause := extractCauseLine(outputs["progress"])
		if cause != "" {
			summary = "scylla backup failed: " + cause
		} else {
			summary = "scylla-manager backup task failed"
		}
	}

	if pollStatus == "task failed" || strings.HasPrefix(pollStatus, "poll error") || pollStatus == "context cancelled (timeout)" {
		result := failResult(spec.Type, summary, outputs)
		result.BytesWritten = scyllaBytes
		result.OutputFiles = []string{"provider/scylla/task.json", "provider/scylla/log.txt"}
		return result
	}

	result := successResult(spec.Type, summary, outputs)
	result.BytesWritten = scyllaBytes
	result.OutputFiles = []string{"provider/scylla/task.json", "provider/scylla/log.txt"}
	result.RestoreInputs = map[string]string{
		"cluster":   cluster,
		"locations": locJoin,
		"task_id":   taskID,
	}
	return result
}

// findExistingScyllaBackupTask finds an existing backup task for the cluster
// that can be restarted, avoiding "another task is running" conflicts.
func (srv *server) findExistingScyllaBackupTask(cluster, apiURL string) string {
	args := []string{"tasks", "--cluster", cluster}
	if apiURL != "" && apiURL != "http://127.0.0.1:5080" {
		args = append(args, "--api-url", apiURL)
	}
	stdout, _, err := runCmd("sctool", args...)
	if err != nil {
		return ""
	}
	// Find the first backup task ID (format: backup/<uuid>)
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		for _, field := range strings.Fields(line) {
			if strings.HasPrefix(field, "backup/") && !strings.Contains(field, "repair") {
				return field
			}
		}
	}
	return ""
}

// extractStatusLine extracts the Status value from sctool progress output.
// Looks for a line like "Status:		DONE" or "Status:		RUNNING (uploading data)".
func extractStatusLine(output string) string {
	for _, line := range strings.Split(output, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Status:") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "Status:"))
		}
	}
	return ""
}

// extractCauseLine extracts the Cause section from sctool progress output.
// Looks for lines after "Cause:" and collects them until the next header or empty line.
// Example output:
//
//	Cause:		snapshot
//	 10.0.0.63: not enough disk space
func extractCauseLine(output string) string {
	lines := strings.Split(output, "\n")
	inCause := false
	var parts []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "Cause:") {
			inCause = true
			after := strings.TrimSpace(strings.TrimPrefix(trimmed, "Cause:"))
			if after != "" {
				parts = append(parts, after)
			}
			continue
		}
		if inCause {
			// Stop at next top-level header (e.g. "Start time:", "Duration:") or empty line
			if trimmed == "" {
				break
			}
			// A known header line ends the cause section
			if strings.HasPrefix(trimmed, "Start time:") || strings.HasPrefix(trimmed, "End time:") ||
				strings.HasPrefix(trimmed, "Duration:") || strings.HasPrefix(trimmed, "Progress:") ||
				strings.HasPrefix(trimmed, "Snapshot Tag:") || strings.HasPrefix(trimmed, "Datacenters:") {
				break
			}
			parts = append(parts, trimmed)
		}
	}
	return strings.Join(parts, " — ")
}

// scyllaLocations derives sctool --location values from configured Destinations.
// sctool requires locations in the format <provider>:<path> (e.g. s3:my-bucket).
// Local/NFS destinations are NOT supported by sctool and are skipped.
func (srv *server) scyllaLocations() []string {
	var locs []string
	for _, d := range srv.Destinations {
		switch d.Type {
		case "s3", "minio":
			locs = append(locs, "s3:"+d.Path)
		case "gcs":
			locs = append(locs, "gcs:"+d.Path)
		case "azure":
			locs = append(locs, "azure:"+d.Path)
		// local/nfs: sctool does not support filesystem paths, skip
		}
	}
	return locs
}

// --- Helpers ---

func successResult(provType backup_managerpb.BackupProviderType, summary string, outputs map[string]string) *backup_managerpb.BackupProviderResult {
	return &backup_managerpb.BackupProviderResult{
		Type:     provType,
		Enabled:  true,
		State:    backup_managerpb.BackupJobState_BACKUP_JOB_SUCCEEDED,
		Severity: backup_managerpb.BackupSeverity_BACKUP_SEVERITY_INFO,
		Summary:  summary,
		Outputs:  outputs,
	}
}

func failResult(provType backup_managerpb.BackupProviderType, errMsg string, outputs map[string]string) *backup_managerpb.BackupProviderResult {
	return &backup_managerpb.BackupProviderResult{
		Type:         provType,
		Enabled:      true,
		State:        backup_managerpb.BackupJobState_BACKUP_JOB_FAILED,
		Severity:     backup_managerpb.BackupSeverity_BACKUP_SEVERITY_ERROR,
		Summary:      errMsg,
		ErrorMessage: errMsg,
		Outputs:      outputs,
	}
}

func providerName(t backup_managerpb.BackupProviderType) string {
	switch t {
	case backup_managerpb.BackupProviderType_BACKUP_PROVIDER_ETCD:
		return "etcd"
	case backup_managerpb.BackupProviderType_BACKUP_PROVIDER_RESTIC:
		return "restic"
	case backup_managerpb.BackupProviderType_BACKUP_PROVIDER_MINIO:
		return "minio"
	case backup_managerpb.BackupProviderType_BACKUP_PROVIDER_SCYLLA:
		return "scylla"
	default:
		return "unknown"
	}
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func fileOrDirExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// parseScyllaBytes extracts transferred bytes from sctool progress output.
// It looks for size values like "123.45MiB", "1.2GiB", "456KiB", "789B" in the output.
func parseScyllaBytes(output string) uint64 {
	var maxBytes uint64
	for _, line := range strings.Split(output, "\n") {
		// Look for size patterns in the progress output
		for _, field := range strings.Fields(line) {
			b := parseSizeField(field)
			if b > maxBytes {
				maxBytes = b
			}
		}
	}
	return maxBytes
}

// parseSizeField parses a human-readable size like "123.45MiB" into bytes.
func parseSizeField(s string) uint64 {
	s = strings.TrimSpace(s)
	multipliers := []struct {
		suffix string
		mult   float64
	}{
		{"TiB", 1024 * 1024 * 1024 * 1024},
		{"GiB", 1024 * 1024 * 1024},
		{"MiB", 1024 * 1024},
		{"KiB", 1024},
		{"TB", 1000 * 1000 * 1000 * 1000},
		{"GB", 1000 * 1000 * 1000},
		{"MB", 1000 * 1000},
		{"KB", 1000},
		{"B", 1},
	}
	for _, m := range multipliers {
		if strings.HasSuffix(s, m.suffix) {
			numStr := strings.TrimSuffix(s, m.suffix)
			var val float64
			if _, err := fmt.Sscanf(numStr, "%f", &val); err == nil && val > 0 {
				return uint64(val * m.mult)
			}
		}
	}
	return 0
}

func containsProvider(list []string, name string) bool {
	for _, v := range list {
		if strings.EqualFold(v, name) {
			return true
		}
	}
	return false
}
