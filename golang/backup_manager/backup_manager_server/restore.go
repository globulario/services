package main

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/globulario/services/golang/backup_manager/backup_managerpb"
	"github.com/globulario/services/golang/node_agent/node_agentpb"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// --- Confirmation token store with TTL ---

const tokenTTL = 10 * time.Minute

type tokenStore struct {
	mu     sync.Mutex
	tokens map[string]time.Time // token -> expiry
}

func newTokenStore() *tokenStore {
	ts := &tokenStore{tokens: make(map[string]time.Time)}
	go ts.cleanup()
	return ts
}

func (ts *tokenStore) generate(backupID string) string {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	raw := fmt.Sprintf("%s:%d:%x", backupID, time.Now().UnixNano(), sha256.Sum256([]byte(backupID+time.Now().String())))
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(raw)))
	token := hash[:16]
	ts.tokens[token] = time.Now().Add(tokenTTL)
	return token
}

func (ts *tokenStore) validate(token string) bool {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	expiry, ok := ts.tokens[token]
	if !ok {
		return false
	}
	if time.Now().After(expiry) {
		delete(ts.tokens, token)
		return false
	}
	// Single use: consume on validation
	delete(ts.tokens, token)
	return true
}

func (ts *tokenStore) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		ts.mu.Lock()
		now := time.Now()
		for k, v := range ts.tokens {
			if now.After(v) {
				delete(ts.tokens, k)
			}
		}
		ts.mu.Unlock()
	}
}

// --- RestoreBackup RPC ---

// RestoreBackup executes a restore from a backup artifact.
func (srv *server) RestoreBackup(ctx context.Context, rqst *backup_managerpb.RestoreBackupRequest) (*backup_managerpb.RestoreBackupResponse, error) {
	art, err := srv.store.GetArtifact(rqst.BackupId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "backup %s not found", rqst.BackupId)
	}

	// Schema backward compatibility check
	if art.SchemaVersion == 0 {
		return nil, status.Error(codes.FailedPrecondition, "unsupported manifest schema (v0/missing); only schema_version >= 1 supports restore")
	}

	// Safety gate: require confirmation token or force
	if !rqst.Force && rqst.ConfirmationToken == "" {
		return nil, status.Error(codes.FailedPrecondition, "restore requires confirmation_token from RestorePlan or force=true")
	}

	// Validate confirmation token (TTL-based)
	if rqst.ConfirmationToken != "" && !rqst.Force {
		if !srv.tokens.validate(rqst.ConfirmationToken) {
			return nil, status.Error(codes.FailedPrecondition, "invalid or expired confirmation_token; generate a fresh one via RestorePlan")
		}
	}

	// Safety gate: check for active jobs unless forced
	if !rqst.Force && srv.active.count() > 0 {
		return nil, status.Error(codes.FailedPrecondition, "another job is running; use force=true to override")
	}

	// Generate restore plan steps
	planResp := srv.buildRestorePlan(art, rqst)

	// Dry run: validate + plan but don't execute
	if rqst.DryRun {
		// For restic: verify snapshot exists
		for _, pr := range art.ProviderResults {
			if pr.Type == backup_managerpb.BackupProviderType_BACKUP_PROVIDER_RESTIC && rqst.IncludeConfig {
				snapID := pr.Outputs["snapshot_id"]
				if snapID != "" {
					repo := pr.Outputs["repo_path"]
					if repo == "" {
						repo = srv.ResticRepo
					}
					cmd := exec.CommandContext(ctx, "restic", "snapshots", "--json", "--repo", repo)
					cmd.Env = appendResticEnv(repo, srv.ResticPassword)
					out, err := cmd.CombinedOutput()
					if err != nil || !strings.Contains(string(out), snapID) {
						planResp.Warnings = append(planResp.Warnings, &backup_managerpb.ValidationIssue{
							Severity: backup_managerpb.BackupSeverity_BACKUP_SEVERITY_ERROR,
							Code:     "RESTIC_SNAPSHOT_NOT_FOUND",
							Message:  fmt.Sprintf("snapshot %s not found in repo %s", snapID, repo),
						})
					}
				}
			}
		}

		return &backup_managerpb.RestoreBackupResponse{
			DryRun:   true,
			Steps:    planResp.Steps,
			Warnings: planResp.Warnings,
		}, nil
	}

	// Create restore job
	jobID := Utility.RandomUUID()
	now := time.Now().UnixMilli()

	job := &backup_managerpb.BackupJob{
		JobId:         jobID,
		PlanName:      "restore:" + rqst.BackupId,
		State:         backup_managerpb.BackupJobState_BACKUP_JOB_QUEUED,
		CreatedUnixMs: now,
		BackupId:      rqst.BackupId,
		Message:       "restore queued",
		JobType:       backup_managerpb.BackupJobType_BACKUP_JOB_TYPE_RESTORE,
	}

	if err := srv.store.SaveJob(job); err != nil {
		return nil, status.Errorf(codes.Internal, "save restore job: %v", err)
	}

	slog.Info("restore job created", "job_id", jobID, "backup_id", rqst.BackupId)
	metricsJobsTotal.WithLabelValues("restore_queued").Inc()

	go srv.executeRestore(job, art, rqst)

	return &backup_managerpb.RestoreBackupResponse{
		JobId:    jobID,
		Steps:    planResp.Steps,
		Warnings: planResp.Warnings,
	}, nil
}

// executeRestore runs the restore process asynchronously.
func (srv *server) executeRestore(job *backup_managerpb.BackupJob, art *backup_managerpb.BackupArtifact, rqst *backup_managerpb.RestoreBackupRequest) {
	srv.sem <- struct{}{}
	defer func() { <-srv.sem }()

	ctx, cancel := context.WithCancel(context.Background())
	srv.active.add(job.JobId, cancel)
	defer func() {
		cancel()
		srv.active.remove(job.JobId)
	}()

	metricsRunning.Inc()
	defer metricsRunning.Dec()

	job.State = backup_managerpb.BackupJobState_BACKUP_JOB_RUNNING
	job.StartedUnixMs = time.Now().UnixMilli()
	job.Message = "restoring"
	if err := srv.store.SaveJob(job); err != nil {
		slog.Error("failed to save restore job state", "job_id", job.JobId, "state", "RUNNING", "err", err)
	}

	capsuleDir := srv.CapsuleDir(art.BackupId)

	// If capsule is missing locally, try to fetch from remote
	if !fileOrDirExists(capsuleDir) {
		if err := srv.FetchCapsuleFromRemote(art.BackupId, art); err != nil {
			job.State = backup_managerpb.BackupJobState_BACKUP_JOB_FAILED
			job.Message = fmt.Sprintf("capsule missing and remote fetch failed: %v", err)
			job.FinishedUnixMs = time.Now().UnixMilli()
			if saveErr := srv.store.SaveJob(job); saveErr != nil {
		slog.Error("failed to save restore job state", "job_id", job.JobId, "err", saveErr)
	}
			metricsJobsTotal.WithLabelValues("restore_failed").Inc()
			return
		}
	}

	var results []*backup_managerpb.BackupProviderResult
	allOk := true

	for _, pr := range art.ProviderResults {
		if pr.State != backup_managerpb.BackupJobState_BACKUP_JOB_SUCCEEDED {
			continue
		}

		if ctx.Err() != nil {
			job.State = backup_managerpb.BackupJobState_BACKUP_JOB_CANCELED
			job.Message = "restore canceled"
			job.FinishedUnixMs = time.Now().UnixMilli()
			job.Results = results
			if saveErr := srv.store.SaveJob(job); saveErr != nil {
		slog.Error("failed to save restore job state", "job_id", job.JobId, "err", saveErr)
	}
			metricsJobsTotal.WithLabelValues("restore_canceled").Inc()
			return
		}

		name := providerName(pr.Type)

		// Build restore options from the artifact's provider outputs/inputs
		var opts map[string]string
		var include bool

		switch pr.Type {
		case backup_managerpb.BackupProviderType_BACKUP_PROVIDER_ETCD:
			include = rqst.IncludeEtcd
			opts = srv.buildEtcdRestoreOpts(capsuleDir, pr)
		case backup_managerpb.BackupProviderType_BACKUP_PROVIDER_RESTIC:
			include = rqst.IncludeConfig
			target := "/"
			if rqst.TargetNode != "" {
				target = rqst.TargetNode
			}
			opts = srv.buildResticRestoreOpts(pr, target)
		case backup_managerpb.BackupProviderType_BACKUP_PROVIDER_MINIO:
			include = rqst.IncludeMinio
			opts = srv.buildMinioRestoreOpts(pr)
		case backup_managerpb.BackupProviderType_BACKUP_PROVIDER_SCYLLA:
			include = rqst.IncludeScylla
			opts = srv.buildScyllaRestoreOpts(pr)
		}

		if !include || opts == nil {
			continue
		}

		// Dispatch restore to the local node-agent
		result := srv.restoreViaNodeAgent(ctx, pr.Type, name, art.BackupId, opts, rqst.Force)

		// Write per-provider restore log into capsule
		logPath := filepath.Join(capsuleDir, "provider", name, "restore.log")
		_ = os.MkdirAll(filepath.Dir(logPath), 0755)
		logData := fmt.Sprintf("state=%s\nsummary=%s\nerror=%s\noutputs=%v\n",
			result.State, result.Summary, result.ErrorMessage, result.Outputs)
		_ = os.WriteFile(logPath, []byte(logData), 0644)

		results = append(results, result)
		if result.State == backup_managerpb.BackupJobState_BACKUP_JOB_FAILED {
			allOk = false
		}
	}

	job.Results = results
	job.FinishedUnixMs = time.Now().UnixMilli()

	// Always restart services after restore — even if some providers failed.
	// The etcd restore alone changes the cluster ID and stales LastSeen timestamps,
	// causing "node unreachable" until controller and node-agent are restarted.
	// gRPC services also need to re-register their ports in the restored etcd.
	srv.restartAllServices(ctx)

	// Reseed RBAC after restore — role bindings may have been wiped by the etcd restore.
	srv.reseedRBAC(ctx)

	// Schedule a delayed self-restart so the backup-manager reloads the
	// restored job store from disk. We can't restart immediately because
	// we need to finish writing the restore job result first.
	go func() {
		time.Sleep(5 * time.Second)
		slog.Info("restore: scheduling backup-manager self-restart to reload job store")
		_ = exec.CommandContext(context.Background(), "sudo", "systemctl", "restart", "globular-backup-manager.service").Run()
	}()

	if allOk {
		job.State = backup_managerpb.BackupJobState_BACKUP_JOB_SUCCEEDED
		job.Message = "restore completed successfully"
		metricsJobsTotal.WithLabelValues("restore_succeeded").Inc()
		slog.Info("restore succeeded", "job_id", job.JobId, "backup_id", art.BackupId)
	} else {
		job.State = backup_managerpb.BackupJobState_BACKUP_JOB_FAILED
		job.Message = "one or more providers failed to restore"
		metricsJobsTotal.WithLabelValues("restore_failed").Inc()
		slog.Warn("restore failed", "job_id", job.JobId, "backup_id", art.BackupId)
	}

	if err := srv.store.SaveJob(job); err != nil {
		slog.Error("failed to save final restore job state", "job_id", job.JobId, "state", job.State, "err", err)
	}

	// Write restore report into capsule
	srv.writeRestoreReport(capsuleDir, job)
}

// --- Option builders (resolve provider-specific options from artifact) ---

func (srv *server) buildEtcdRestoreOpts(capsuleDir string, pr *backup_managerpb.BackupProviderResult) map[string]string {
	snapshotPath := filepath.Join(capsuleDir, "payload", "etcd", "etcd-snapshot.db")
	// Node-agent knows the correct data-dir (/var/lib/globular/etcd);
	// only pass snapshot_path — don't forward stale data_dir from old artifacts.
	return map[string]string{"snapshot_path": snapshotPath}
}

func (srv *server) buildResticRestoreOpts(pr *backup_managerpb.BackupProviderResult, target string) map[string]string {
	snapID := pr.Outputs["snapshot_id"]
	if snapID == "" {
		snapID = pr.RestoreInputs["snapshot_id"]
	}
	repo := pr.Outputs["repo_path"]
	if repo == "" {
		repo = pr.RestoreInputs["repo"]
	}
	if repo == "" {
		repo = srv.ResticRepo
	}
	if t := pr.RestoreInputs["target"]; t != "" && target == "/" {
		target = t
	}
	return map[string]string{
		"snapshot_id": snapID,
		"repo":        repo,
		"password":    srv.ResticPassword,
		"target":      target,
	}
}

func (srv *server) buildMinioRestoreOpts(pr *backup_managerpb.BackupProviderResult) map[string]string {
	remote := pr.Outputs["remote"]
	if remote == "" {
		remote = pr.RestoreInputs["remote"]
	}
	source := pr.Outputs["source"]
	if source == "" {
		source = pr.RestoreInputs["source"]
	}
	return map[string]string{"remote": remote, "source": source}
}

func (srv *server) buildScyllaRestoreOpts(pr *backup_managerpb.BackupProviderResult) map[string]string {
	cluster := pr.Outputs["cluster"]
	if cluster == "" {
		cluster = pr.RestoreInputs["cluster"]
	}
	if cluster == "" {
		cluster = srv.ScyllaCluster
	}

	var locations []string
	if l := pr.Outputs["locations"]; l != "" {
		locations = strings.Split(l, ",")
	} else if l := pr.RestoreInputs["locations"]; l != "" {
		locations = strings.Split(l, ",")
	} else if l := pr.Outputs["location"]; l != "" {
		locations = []string{l}
	} else if l := pr.RestoreInputs["location"]; l != "" {
		locations = []string{l}
	}
	if len(locations) == 0 {
		if srv.ScyllaLocation != "" {
			locations = []string{srv.ScyllaLocation}
		} else {
			locations = srv.scyllaLocations()
		}
	}

	snapshotTag := pr.Outputs["snapshot_tag"]
	if snapshotTag == "" {
		snapshotTag = pr.RestoreInputs["snapshot_tag"]
	}
	if snapshotTag == "" {
		snapshotTag = srv.extractScyllaSnapshotTag(cluster, srv.ScyllaManagerAPI, locations)
	}

	return map[string]string{
		"cluster":      cluster,
		"locations":    strings.Join(locations, ","),
		"snapshot_tag": snapshotTag,
		"api_url":      srv.ScyllaManagerAPI,
	}
}

// --- Node-agent restore dispatch ---

// restoreViaNodeAgent dispatches a restore to the local node-agent via gRPC.
func (srv *server) restoreViaNodeAgent(
	ctx context.Context,
	provType backup_managerpb.BackupProviderType,
	provName, backupID string,
	opts map[string]string,
	force bool,
) *backup_managerpb.BackupProviderResult {

	start := time.Now().UnixMilli()

	endpoint := fmt.Sprintf("127.0.0.1:%d", nodeAgentDefaultPort)
	slog.Info("dispatching restore to node-agent", "provider", provName, "endpoint", endpoint)

	conn, err := srv.dialNodeAgent(ctx, endpoint)
	if err != nil {
		return restoreFailResult(provType,
			fmt.Sprintf("dial node-agent for restore: %v", err),
			err.Error(), nil, start)
	}
	defer conn.Close()

	client := node_agentpb.NewNodeAgentServiceClient(conn)

	spec := &node_agentpb.RestoreProviderSpec{
		Provider:       provName,
		Options:        opts,
		TimeoutSeconds: uint32(srv.ProviderTimeoutSeconds),
		Force:          force,
	}

	runResp, err := client.RunRestoreProvider(ctx, &node_agentpb.RunRestoreProviderRequest{
		BackupId: backupID,
		Spec:     spec,
		NodeId:   srv.Id,
	})
	if err != nil {
		return restoreFailResult(provType,
			fmt.Sprintf("RunRestoreProvider RPC failed: %v", err),
			err.Error(), nil, start)
	}

	taskID := runResp.TaskId
	slog.Info("restore task started on node-agent", "provider", provName, "task_id", taskID)

	// Poll until done
	result, err := srv.pollRestoreTask(ctx, client, taskID, provName)
	if err != nil {
		return restoreFailResult(provType,
			fmt.Sprintf("poll restore task: %v", err),
			err.Error(), nil, start)
	}

	// Convert node-agent result to backup-manager result
	state := backup_managerpb.BackupJobState_BACKUP_JOB_SUCCEEDED
	severity := backup_managerpb.BackupSeverity_BACKUP_SEVERITY_INFO
	if !result.Ok {
		state = backup_managerpb.BackupJobState_BACKUP_JOB_FAILED
		severity = backup_managerpb.BackupSeverity_BACKUP_SEVERITY_ERROR
	}

	return &backup_managerpb.BackupProviderResult{
		Type:           provType,
		Enabled:        true,
		State:          state,
		Severity:       severity,
		Summary:        result.Summary,
		ErrorMessage:   result.ErrorMessage,
		Outputs:        result.Outputs,
		BytesWritten:   result.BytesWritten,
		StartedUnixMs:  result.StartedUnixMs,
		FinishedUnixMs: result.FinishedUnixMs,
	}
}

// pollRestoreTask polls GetRestoreTaskResult until done or context expires.
func (srv *server) pollRestoreTask(
	ctx context.Context,
	client node_agentpb.NodeAgentServiceClient,
	taskID, provName string,
) (*node_agentpb.BackupProviderResult, error) {
	ticker := time.NewTicker(nodeAgentPollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil, fmt.Errorf("context cancelled waiting for restore task %s", taskID)
		case <-ticker.C:
			resp, err := client.GetRestoreTaskResult(ctx, &node_agentpb.GetRestoreTaskResultRequest{
				TaskId: taskID,
			})
			if err != nil {
				slog.Warn("poll restore task error (will retry)",
					"task_id", taskID, "provider", provName, "error", err)
				continue
			}
			if resp.Result != nil && resp.Result.Done {
				slog.Info("restore task completed",
					"task_id", taskID, "provider", provName, "ok", resp.Result.Ok)
				return resp.Result, nil
			}
		}
	}
}

// --- FetchCapsuleFromRemote ---

// FetchCapsuleFromRemote downloads a capsule from the first available replication target.
func (srv *server) FetchCapsuleFromRemote(backupID string, art *backup_managerpb.BackupArtifact) error {
	capsuleDir := srv.CapsuleDir(backupID)

	for _, rep := range art.Replications {
		if rep.State != backup_managerpb.BackupJobState_BACKUP_JOB_SUCCEEDED {
			continue
		}

		slog.Info("fetching capsule from remote", "backup_id", backupID, "source", rep.DestinationName)

		var err error
		switch rep.DestinationType {
		case backup_managerpb.BackupDestinationType_BACKUP_DESTINATION_LOCAL,
			backup_managerpb.BackupDestinationType_BACKUP_DESTINATION_NFS:
			srcPath := filepath.Join(rep.DestinationPath, "artifacts", backupID)
			if fileOrDirExists(srcPath) {
				_, stderr, cpErr := runCmd("cp", "-a", srcPath, capsuleDir)
				if cpErr != nil {
					err = fmt.Errorf("cp: %s: %w", stderr, cpErr)
				}
			} else {
				err = fmt.Errorf("remote path does not exist: %s", srcPath)
			}

		case backup_managerpb.BackupDestinationType_BACKUP_DESTINATION_MINIO,
			backup_managerpb.BackupDestinationType_BACKUP_DESTINATION_S3:
			remotePath := fmt.Sprintf(":s3:%s/artifacts/%s", rep.DestinationPath, backupID)
			args := []string{"sync", remotePath, capsuleDir}
			// Look up destination config for MinIO credentials
			dest := srv.findDestinationByName(rep.DestinationName)
			if dest != nil && dest.Type == "minio" {
				endpoint := dest.Options["endpoint"]
				if endpoint != "" {
					args = append(args, "--s3-provider", "Minio", "--s3-endpoint", endpoint, "--s3-env-auth=false")
				}
				if ak := dest.Options["access_key"]; ak != "" {
					args = append(args, "--s3-access-key-id", ak)
				}
				if sk := dest.Options["secret_key"]; sk != "" {
					args = append(args, "--s3-secret-access-key", sk)
				}
				if strings.HasPrefix(endpoint, "https") {
					args = append(args, "--no-check-certificate")
				}
			}
			_, stderr, syncErr := runCmd("rclone", args...)
			if syncErr != nil {
				err = fmt.Errorf("rclone: %s: %w", stderr, syncErr)
			}

		case backup_managerpb.BackupDestinationType_BACKUP_DESTINATION_RCLONE:
			remotePath := fmt.Sprintf("%s/artifacts/%s", rep.DestinationPath, backupID)
			_, stderr, syncErr := runCmd("rclone", "sync", remotePath, capsuleDir)
			if syncErr != nil {
				err = fmt.Errorf("rclone: %s: %w", stderr, syncErr)
			}
		}

		if err != nil {
			slog.Warn("fetch from remote failed", "source", rep.DestinationName, "error", err)
			continue
		}

		// Verify we got manifest
		if fileExists(filepath.Join(capsuleDir, "manifest.json")) {
			slog.Info("capsule fetched from remote", "backup_id", backupID, "source", rep.DestinationName)
			return nil
		}
	}

	return fmt.Errorf("no replication target has a valid capsule for %s", backupID)
}

// --- Restore report ---

func (srv *server) writeRestoreReport(capsuleDir string, job *backup_managerpb.BackupJob) {
	report := map[string]interface{}{
		"job_id":     job.JobId,
		"backup_id":  job.BackupId,
		"state":      job.State.String(),
		"message":    job.Message,
		"started":    job.StartedUnixMs,
		"finished":   job.FinishedUnixMs,
		"providers":  make([]map[string]interface{}, 0),
	}

	for _, r := range job.Results {
		report["providers"] = append(report["providers"].([]map[string]interface{}), map[string]interface{}{
			"type":    providerName(r.Type),
			"state":   r.State.String(),
			"summary": r.Summary,
			"error":   r.ErrorMessage,
			"outputs": r.Outputs,
		})
	}

	data, _ := json.MarshalIndent(report, "", "  ")
	reportsDir := filepath.Join(capsuleDir, "reports")
	_ = os.MkdirAll(reportsDir, 0755)
	_ = os.WriteFile(filepath.Join(reportsDir, "restore.json"), data, 0644)
}

// --- Helpers ---

func (srv *server) buildRestorePlan(art *backup_managerpb.BackupArtifact, rqst *backup_managerpb.RestoreBackupRequest) *backup_managerpb.RestorePlanResponse {
	planRqst := &backup_managerpb.RestorePlanRequest{
		BackupId:            rqst.BackupId,
		IncludeEtcd:         rqst.IncludeEtcd,
		IncludeConfig:       rqst.IncludeConfig,
		IncludeMinio:        rqst.IncludeMinio,
		IncludeScylla:       rqst.IncludeScylla,
		IncludeServiceData:  rqst.IncludeServiceData,
		ServiceDataServices: rqst.ServiceDataServices,
	}
	resp, _ := srv.RestorePlan(context.Background(), planRqst)
	return resp
}

// serviceDataForRestore filters service data entries for restore based on policy and service filter.
// Classification policy:
//   - AUTHORITATIVE: always restored
//   - REBUILDABLE: optional, controlled by RestoreRebuildableServiceData config
//   - CACHE: never restored
func (srv *server) serviceDataForRestore(entries []*backup_managerpb.ServiceDataEntry, services []string) []*backup_managerpb.ServiceDataEntry {
	serviceFilter := make(map[string]bool)
	for _, s := range services {
		serviceFilter[s] = true
	}

	var result []*backup_managerpb.ServiceDataEntry
	for _, e := range entries {
		// Filter by specific services if requested
		if len(serviceFilter) > 0 && !serviceFilter[e.ServiceName] {
			continue
		}
		// CACHE datasets are never restored
		if e.DataClass == "CACHE" {
			continue
		}
		// Skip REBUILDABLE unless RestoreRebuildableServiceData is enabled
		if e.DataClass == "REBUILDABLE" && !srv.RestoreRebuildableServiceData {
			slog.Info("restore: skipping REBUILDABLE entry", "service", e.ServiceName, "name", e.DatasetName)
			continue
		}
		result = append(result, e)
	}
	return result
}

// generateConfirmationToken creates a TTL-based token via the server's token store.
func (srv *server) generateConfirmationToken(backupID string) string {
	return srv.tokens.generate(backupID)
}

func appendResticEnv(repo, password string) []string {
	return append(os.Environ(), "RESTIC_REPOSITORY="+repo, "RESTIC_PASSWORD="+password)
}

func restoreFailResult(provType backup_managerpb.BackupProviderType, summary, errMsg string, outputs map[string]string, start int64) *backup_managerpb.BackupProviderResult {
	if errMsg == "" {
		errMsg = summary
	}
	return &backup_managerpb.BackupProviderResult{
		Type: provType, Enabled: true,
		State:         backup_managerpb.BackupJobState_BACKUP_JOB_FAILED,
		Severity:      backup_managerpb.BackupSeverity_BACKUP_SEVERITY_ERROR,
		Summary:       summary,
		ErrorMessage:  errMsg,
		Outputs:       outputs,
		StartedUnixMs: start, FinishedUnixMs: time.Now().UnixMilli(),
	}
}

// restartAllServices restarts all gRPC service systemd units so they
// re-register in etcd with their current ports. Infrastructure services
// (etcd, envoy, xds, gateway, minio) are skipped — they don't register
// in etcd and restarting them could disrupt the restore flow itself.
//
// After restarting gRPC services, the cluster controller and node agent
// are restarted last (in that order) so the controller is ready to
// accept heartbeats when the node agent comes back up. Without this,
// restored etcd state contains stale LastSeen timestamps and the
// controller marks nodes as unreachable.
func (srv *server) restartAllServices(ctx context.Context) {
	// List all globular service units
	out, err := exec.CommandContext(ctx, "systemctl", "list-units", "globular-*", "--no-legend", "--no-pager", "--plain").Output()
	if err != nil {
		slog.Warn("restartAllServices: failed to list units", "err", err)
		return
	}

	// Phase 1: skip infra + control-plane services.
	// Node agent and controller are restarted in phase 2 after all
	// gRPC services are back, so heartbeats land on a ready controller.
	skip := map[string]bool{
		"globular-etcd.service":                 true,
		"globular-envoy.service":                true,
		"globular-xds.service":                  true,
		"globular-gateway.service":              true,
		"globular-minio.service":                true,
		"globular-node-agent.service":           true, // restarted in phase 2
		"globular-cluster-controller.service":   true, // restarted in phase 2
		"globular-backup-manager.service":        true, // self — cannot restart ourselves
		"globular-prometheus.service":            true,
		"globular-node-exporter.service":         true,
		"globular-scylla-manager.service":        true,
		"globular-scylla-manager-agent.service":  true,
	}

	var restarted []string
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		unit := fields[0]
		if !strings.HasPrefix(unit, "globular-") || !strings.HasSuffix(unit, ".service") {
			continue
		}
		if skip[unit] {
			continue
		}
		slog.Info("restartAllServices: restarting", "unit", unit)
		if err := exec.CommandContext(ctx, "sudo", "systemctl", "restart", unit).Run(); err != nil {
			slog.Warn("restartAllServices: restart failed", "unit", unit, "err", err)
		} else {
			restarted = append(restarted, unit)
		}
	}
	slog.Info("restartAllServices: phase 1 done (gRPC services)", "restarted", restarted)

	// Phase 2: restart controller first, then node agent.
	// Controller must be listening before node agent sends its first heartbeat,
	// otherwise the heartbeat is lost and the node stays "unreachable" until
	// the next 30-second tick.

	// 2a. Restart controller
	if isSystemdActive("globular-cluster-controller.service") {
		slog.Info("restartAllServices: restarting cluster-controller")
		if err := exec.CommandContext(ctx, "sudo", "systemctl", "restart", "globular-cluster-controller.service").Run(); err != nil {
			slog.Warn("restartAllServices: controller restart failed", "err", err)
		} else {
			restarted = append(restarted, "globular-cluster-controller.service")
		}
		// Wait for controller to be ready (port 12000), up to 15 seconds.
		for i := 0; i < 30; i++ {
			if isPortOpen("127.0.0.1", 12000) {
				slog.Info("restartAllServices: controller ready on :12000")
				break
			}
			time.Sleep(500 * time.Millisecond)
		}
	}

	// 2b. Reset node-agent plan generation file. After an etcd restore, plan
	// generation numbers in etcd are from an older snapshot while the node-agent's
	// on-disk high water mark is from the current timeline. New plans arrive with
	// generation <= lastApplied, causing the node-agent to reject them as "replay"
	// and quarantine the plan. Deleting the file resets the counter so all new
	// plans are accepted.
	genFile := "/var/lib/globular/node-agent/last-generation"
	if err := os.Remove(genFile); err == nil {
		slog.Info("restartAllServices: reset node-agent generation file", "path", genFile)
	}

	// 2c. Restart node agent — controller is now ready to accept heartbeats.
	if isSystemdActive("globular-node-agent.service") {
		slog.Info("restartAllServices: restarting node-agent")
		if err := exec.CommandContext(ctx, "sudo", "systemctl", "restart", "globular-node-agent.service").Run(); err != nil {
			slog.Warn("restartAllServices: node-agent restart failed", "err", err)
		} else {
			restarted = append(restarted, "globular-node-agent.service")
		}
	}
	slog.Info("restartAllServices: phase 2 done (control-plane)", "restarted", restarted)
}

func isPortOpen(host string, port int) bool {
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:%d", host, port), 500*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

func isSystemdActive(unit string) bool {
	out, err := exec.Command("systemctl", "is-active", unit).Output()
	if err != nil {
		return false
	}
	return strings.TrimSpace(string(out)) == "active"
}

// reseedRBAC waits for the RBAC service to become healthy after a restore,
// then reseeds SA role bindings and cluster roles that may have been wiped.
func (srv *server) reseedRBAC(ctx context.Context) {
	const rbacPort = 10000

	// Wait up to 30 seconds for RBAC service to become reachable.
	slog.Info("reseedRBAC: waiting for RBAC service")
	ready := false
	for i := 0; i < 60; i++ {
		if isPortOpen("127.0.0.1", rbacPort) {
			ready = true
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	if !ready {
		slog.Warn("reseedRBAC: RBAC service not reachable after 30s, skipping reseed")
		return
	}

	// Give the RBAC service a moment to finish startup after port is open.
	time.Sleep(2 * time.Second)

	// Use the globular CLI seed command via exec for simplicity and reuse.
	slog.Info("reseedRBAC: running globular rbac seed")
	out, err := exec.CommandContext(ctx, "globular", "rbac", "seed", "--insecure").CombinedOutput()
	if err != nil {
		slog.Warn("reseedRBAC: seed command failed", "err", err, "output", string(out))
	} else {
		slog.Info("reseedRBAC: seed complete", "output", string(out))
	}
}

// findDestinationByName looks up a configured destination by its name.
func (srv *server) findDestinationByName(name string) *DestinationConfig {
	for i := range srv.Destinations {
		if srv.Destinations[i].Name == name {
			return &srv.Destinations[i]
		}
	}
	return nil
}
