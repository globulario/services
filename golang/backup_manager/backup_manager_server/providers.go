package main

import (
	"bytes"
	"context"
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

	// Snapshot goes into the capsule payload dir
	snapshotFile := filepath.Join(cc.PayloadDir, "etcd-snapshot.db")
	outputs["snapshot_path"] = snapshotFile

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

	// Set required etcdctl environment variables
	etcdEnv := append(os.Environ(),
		"ETCDCTL_API=3",
		"ETCDCTL_ENDPOINTS="+endpoints,
	)
	if fileExists(cacert) {
		etcdEnv = append(etcdEnv, "ETCDCTL_CACERT="+cacert)
	}
	if fileExists(cert) {
		etcdEnv = append(etcdEnv, "ETCDCTL_CERT="+cert)
	}
	if fileExists(key) {
		etcdEnv = append(etcdEnv, "ETCDCTL_KEY="+key)
	}

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
		return failResult(spec.Type, fmt.Sprintf("etcdctl snapshot save failed: %v", err), outputs)
	}

	// Get snapshot file size
	if info, statErr := os.Stat(snapshotFile); statErr == nil {
		outputs["snapshot_bytes"] = fmt.Sprintf("%d", info.Size())
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

	args := append([]string{"backup", "--json"}, validPaths...)
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
		outputs["exit_error"] = err.Error()
		return failResult(spec.Type, fmt.Sprintf("restic backup failed: %v", err), outputs)
	}

	// Get latest snapshot ID
	snapCmd := exec.CommandContext(ctx, "restic", "snapshots", "--latest", "1", "--json")
	snapCmd.Env = append(os.Environ(), env...)
	snapOut, snapErr := snapCmd.CombinedOutput()
	if snapErr == nil {
		snapJSON := strings.TrimSpace(string(snapOut))
		outputs["latest_snapshot"] = snapJSON
		_ = CapsuleWriteFile(cc.ProviderDir, "snapshot.json", []byte(snapJSON))

		// Extract snapshot ID from JSON (simple parse)
		if idx := strings.Index(snapJSON, `"short_id":"`); idx >= 0 {
			rest := snapJSON[idx+len(`"short_id":"`):]
			if end := strings.Index(rest, `"`); end >= 0 {
				outputs["snapshot_id"] = rest[:end]
			}
		}
	}

	outputs["repo_path"] = repo
	outputs["paths_included"] = strings.Join(validPaths, ",")

	result := successResult(spec.Type, "restic backup completed", outputs)
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
	location := optOrDefault(spec, "location", srv.ScyllaLocation)

	if cluster == "" {
		return failResult(spec.Type, "scylla cluster name not configured (set ScyllaCluster in config or pass 'cluster' option)", outputs)
	}

	if location == "" {
		return failResult(spec.Type, "scylla backup location not configured (set ScyllaLocation in config or pass 'location' option)", outputs)
	}

	outputs["cluster"] = cluster
	outputs["location"] = location
	outputs["api_url"] = apiURL

	args := []string{
		"backup",
		"--cluster", cluster,
		"--location", location,
		"--api-url", apiURL,
	}

	slog.Info("running sctool backup", "cluster", cluster, "location", location)
	cmd := exec.CommandContext(ctx, "sctool", args...)
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf
	err := cmd.Run()

	stdoutStr := strings.TrimSpace(outBuf.String())
	stderrStr := strings.TrimSpace(errBuf.String())
	if stdoutStr != "" {
		outputs["stdout"] = stdoutStr
	}
	if stderrStr != "" {
		outputs["stderr"] = stderrStr
	}

	if err != nil {
		outputs["exit_error"] = err.Error()
		_ = CapsuleWriteFile(cc.ProviderDir, "log.txt", []byte(stdoutStr+"\n"+stderrStr))
		return failResult(spec.Type, fmt.Sprintf("sctool backup failed: %v", err), outputs)
	}

	// Extract task ID from sctool output if present
	for _, line := range strings.Split(stdoutStr, "\n") {
		if strings.Contains(line, "task") || strings.Contains(line, "backup/") {
			outputs["task_id"] = strings.TrimSpace(line)
			break
		}
	}

	// Write task info into capsule
	taskJSON := fmt.Sprintf(`{"cluster":%q,"location":%q,"task_id":%q,"output":%q}`,
		cluster, location, outputs["task_id"], stdoutStr)
	_ = CapsuleWriteFile(cc.ProviderDir, "task.json", []byte(taskJSON))
	_ = CapsuleWriteFile(cc.ProviderDir, "log.txt", []byte(stdoutStr+"\n"+stderrStr))

	result := successResult(spec.Type, "scylla-manager backup triggered", outputs)
	result.OutputFiles = []string{"provider/scylla/task.json", "provider/scylla/log.txt"}
	result.RestoreInputs = map[string]string{
		"cluster":  cluster,
		"location": location,
		"task_id":  outputs["task_id"],
	}
	return result
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
