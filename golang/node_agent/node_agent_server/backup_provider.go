package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// backupTask tracks an in-progress or completed backup provider execution.
type backupTask struct {
	mu     sync.Mutex
	result *node_agentpb.BackupProviderResult
}

var (
	backupTasksMu sync.Mutex
	backupTasks   = make(map[string]*backupTask)
)

// RunBackupProvider starts a backup provider execution asynchronously.
func (s *NodeAgentServer) RunBackupProvider(ctx context.Context, req *node_agentpb.RunBackupProviderRequest) (*node_agentpb.RunBackupProviderResponse, error) {
	if req.Spec == nil || req.Spec.Provider == "" {
		return nil, status.Error(codes.InvalidArgument, "spec.provider is required")
	}
	if req.BackupId == "" {
		return nil, status.Error(codes.InvalidArgument, "backup_id is required")
	}

	taskID := uuid.New().String()

	task := &backupTask{
		result: &node_agentpb.BackupProviderResult{
			Provider: req.Spec.Provider,
			Done:     false,
		},
	}

	backupTasksMu.Lock()
	backupTasks[taskID] = task
	backupTasksMu.Unlock()

	log.Printf("backup task %s started: provider=%s backup_id=%s node_id=%s",
		taskID, req.Spec.Provider, req.BackupId, req.NodeId)

	go s.executeBackupProvider(taskID, task, req)

	return &node_agentpb.RunBackupProviderResponse{TaskId: taskID}, nil
}

// GetBackupTaskResult returns the current state of a backup task.
func (s *NodeAgentServer) GetBackupTaskResult(ctx context.Context, req *node_agentpb.GetBackupTaskResultRequest) (*node_agentpb.GetBackupTaskResultResponse, error) {
	if req.TaskId == "" {
		return nil, status.Error(codes.InvalidArgument, "task_id is required")
	}

	backupTasksMu.Lock()
	task, ok := backupTasks[req.TaskId]
	backupTasksMu.Unlock()

	if !ok {
		return nil, status.Errorf(codes.NotFound, "backup task %s not found", req.TaskId)
	}

	task.mu.Lock()
	result := task.result
	task.mu.Unlock()

	return &node_agentpb.GetBackupTaskResultResponse{Result: result}, nil
}

// executeBackupProvider runs the actual provider work in the background.
func (s *NodeAgentServer) executeBackupProvider(taskID string, task *backupTask, req *node_agentpb.RunBackupProviderRequest) {
	start := time.Now().UnixMilli()

	timeout := time.Duration(req.Spec.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	var result *node_agentpb.BackupProviderResult

	switch req.Spec.Provider {
	case "restic":
		result = s.runResticProvider(ctx, req)
	default:
		result = &node_agentpb.BackupProviderResult{
			Provider:     req.Spec.Provider,
			Ok:           false,
			Summary:      fmt.Sprintf("unsupported provider: %s", req.Spec.Provider),
			ErrorMessage: "only restic is supported for per-node execution",
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
		log.Printf("backup task %s completed: provider=%s", taskID, req.Spec.Provider)
	} else {
		log.Printf("backup task %s failed: provider=%s error=%s", taskID, req.Spec.Provider, result.ErrorMessage)
	}

	// Clean up task after 1 hour
	go func() {
		time.Sleep(1 * time.Hour)
		backupTasksMu.Lock()
		delete(backupTasks, taskID)
		backupTasksMu.Unlock()
	}()
}

// runResticProvider runs a restic backup on this node.
func (s *NodeAgentServer) runResticProvider(ctx context.Context, req *node_agentpb.RunBackupProviderRequest) *node_agentpb.BackupProviderResult {
	outputs := make(map[string]string)
	opts := req.Spec.Options
	if opts == nil {
		opts = make(map[string]string)
	}

	repo := opts["repo"]
	if repo == "" {
		repo = "/var/lib/globular/backups/restic"
	}
	password := opts["password"]
	if password == "" {
		password = "globular-backup"
	}
	pathsStr := opts["paths"]
	if pathsStr == "" {
		pathsStr = "/var/lib/globular"
	}

	outputs["repo"] = repo
	outputs["node_id"] = req.NodeId

	env := append(os.Environ(),
		"RESTIC_REPOSITORY="+repo,
		"RESTIC_PASSWORD="+password,
	)

	// Ensure repo is initialized
	initCmd := exec.CommandContext(ctx, "restic", "init")
	initCmd.Env = env
	initOut, _ := initCmd.CombinedOutput()
	initMsg := strings.TrimSpace(string(initOut))
	if !strings.Contains(initMsg, "created") && !strings.Contains(initMsg, "already") {
		log.Printf("restic init: %s", initMsg)
	}

	// Build backup paths
	paths := strings.Split(pathsStr, ",")
	var validPaths []string
	for _, p := range paths {
		p = strings.TrimSpace(p)
		if p != "" {
			if _, err := os.Stat(p); err == nil {
				validPaths = append(validPaths, p)
			}
		}
	}
	if len(validPaths) == 0 {
		return &node_agentpb.BackupProviderResult{
			Provider:     "restic",
			Ok:           false,
			Summary:      "no valid backup paths found",
			ErrorMessage: "no valid backup paths found",
			Outputs:      outputs,
		}
	}

	outputs["paths"] = strings.Join(validPaths, ",")

	// Run backup with per-node tags
	nodeID := req.NodeId
	if nodeID == "" {
		nodeID = s.nodeID
	}
	hostname := nodeID

	args := []string{
		"backup", "--json",
		"--host", hostname,
		"--tag", "globular",
		"--tag", "backup_id:" + req.BackupId,
		"--tag", "node_id:" + nodeID,
	}
	args = append(args, validPaths...)

	cmd := exec.CommandContext(ctx, "restic", args...)
	cmd.Env = env
	var outBuf, errBuf bytes.Buffer
	cmd.Stdout = &outBuf
	cmd.Stderr = &errBuf

	log.Printf("running restic backup on node %s: paths=%v repo=%s", nodeID, validPaths, repo)
	err := cmd.Run()

	stdoutStr := strings.TrimSpace(outBuf.String())
	stderrStr := strings.TrimSpace(errBuf.String())
	outputs["stdout"] = stdoutStr
	if stderrStr != "" {
		outputs["stderr"] = stderrStr
	}

	// Write capsule-compatible output files
	capsuleBase := filepath.Join("/var/lib/globular/backups/artifacts", req.BackupId)
	providerDir := filepath.Join(capsuleBase, "provider", "restic", nodeID)
	payloadDir := filepath.Join(capsuleBase, "payload", "nodes", nodeID, "restic")
	_ = os.MkdirAll(providerDir, 0755)
	_ = os.MkdirAll(payloadDir, 0755)
	_ = os.WriteFile(filepath.Join(providerDir, "run.json"), []byte(stdoutStr), 0644)
	_ = os.WriteFile(filepath.Join(providerDir, "log.txt"), []byte(stderrStr), 0644)

	var outputFiles []string
	outputFiles = append(outputFiles,
		fmt.Sprintf("provider/restic/%s/run.json", nodeID),
		fmt.Sprintf("provider/restic/%s/log.txt", nodeID),
	)

	// Collect artifacts inline for remote transfer
	artifacts := map[string][]byte{
		fmt.Sprintf("provider/restic/%s/run.json", nodeID): []byte(stdoutStr),
		fmt.Sprintf("provider/restic/%s/log.txt", nodeID):  []byte(stderrStr),
	}

	if err != nil {
		outputs["exit_error"] = err.Error()
		return &node_agentpb.BackupProviderResult{
			Provider:     "restic",
			Ok:           false,
			Summary:      fmt.Sprintf("restic backup failed: %v", err),
			ErrorMessage: fmt.Sprintf("restic backup failed: %v", err),
			Outputs:      outputs,
			OutputFiles:  outputFiles,
			Artifacts:    artifacts,
		}
	}

	// Get snapshot ID
	snapCmd := exec.CommandContext(ctx, "restic", "snapshots",
		"--latest", "1", "--json",
		"--host", hostname,
		"--tag", "backup_id:"+req.BackupId,
	)
	snapCmd.Env = env
	snapOut, snapErr := snapCmd.CombinedOutput()
	if snapErr == nil {
		snapJSON := strings.TrimSpace(string(snapOut))
		outputs["latest_snapshot"] = snapJSON
		_ = os.WriteFile(filepath.Join(providerDir, "snapshot.json"), []byte(snapJSON), 0644)
		outputFiles = append(outputFiles, fmt.Sprintf("provider/restic/%s/snapshot.json", nodeID))
		artifacts[fmt.Sprintf("provider/restic/%s/snapshot.json", nodeID)] = []byte(snapJSON)

		// Extract full snapshot ID
		if idx := strings.Index(snapJSON, `"id":"`); idx >= 0 {
			rest := snapJSON[idx+len(`"id":"`):]
			if end := strings.Index(rest, `"`); end >= 0 {
				outputs["snapshot_id"] = rest[:end]
			}
		}
		// Also extract short_id
		if idx := strings.Index(snapJSON, `"short_id":"`); idx >= 0 {
			rest := snapJSON[idx+len(`"short_id":"`):]
			if end := strings.Index(rest, `"`); end >= 0 {
				outputs["short_id"] = rest[:end]
			}
		}
	}

	outputs["repo_path"] = repo
	outputs["paths_included"] = strings.Join(validPaths, ",")
	outputs["host"] = hostname

	return &node_agentpb.BackupProviderResult{
		Provider:    "restic",
		Ok:          true,
		Summary:     fmt.Sprintf("restic backup completed on node %s", nodeID),
		Outputs:     outputs,
		OutputFiles: outputFiles,
		Artifacts:   artifacts,
	}
}
