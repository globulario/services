package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"strings"
	"sync"
	"time"

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

	// Stage 4: start etcd (requires sudo)
	log.Printf("etcd restore: starting %s", serviceName)
	_, startErr, err := runRestore(ctx, "sudo", "systemctl", "start", serviceName)
	if err != nil {
		outputs["start_error"] = startErr
		return restoreFail("etcd", fmt.Sprintf("etcd restored but service failed to start: %s", startErr), outputs)
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
	excludes := []string{
		"var/backups/globular",
		"var/lib/globular/keys",
		"var/lib/globular/tokens",
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

	if err != nil {
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

	return &node_agentpb.BackupProviderResult{
		Provider:     "restic",
		Ok:           true,
		Summary:      fmt.Sprintf("restic snapshot %s restored to %s", snapshotID, target),
		BytesWritten: restoredBytes,
		Outputs:      outputs,
	}
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

	// Phase 1: restore schema (best-effort)
	schemaArgs := append(append([]string{}, baseArgs...), "--restore-schema")
	log.Printf("scylla restore: restoring schema cluster=%s tag=%s", cluster, snapshotTag)
	schemaOut, schemaErr := exec.CommandContext(ctx, "sctool", schemaArgs...).CombinedOutput()
	schemaOutStr := strings.TrimSpace(string(schemaOut))
	outputs["schema_output"] = schemaOutStr
	if schemaErr != nil {
		log.Printf("scylla restore: schema restore failed (may be expected): %s", schemaOutStr)
		outputs["schema_error"] = schemaOutStr
	} else {
		outputs["schema_restored"] = "true"
	}

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
			if strings.Contains(statusLine, "DONE") || strings.Contains(statusLine, "ERROR") || strings.Contains(statusLine, "ABORTED") {
				restoredBytes = parseScyllaRestoreBytes(pStr)
				break
			}
		}
	}
pollDone:

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
