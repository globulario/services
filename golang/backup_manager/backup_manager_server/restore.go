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
	_ = srv.store.SaveJob(job)

	capsuleDir := srv.CapsuleDir(art.BackupId)

	// If capsule is missing locally, try to fetch from remote
	if !fileOrDirExists(capsuleDir) {
		if err := srv.FetchCapsuleFromRemote(art.BackupId, art); err != nil {
			job.State = backup_managerpb.BackupJobState_BACKUP_JOB_FAILED
			job.Message = fmt.Sprintf("capsule missing and remote fetch failed: %v", err)
			job.FinishedUnixMs = time.Now().UnixMilli()
			_ = srv.store.SaveJob(job)
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
			_ = srv.store.SaveJob(job)
			metricsJobsTotal.WithLabelValues("restore_canceled").Inc()
			return
		}

		var result *backup_managerpb.BackupProviderResult
		name := providerName(pr.Type)

		switch pr.Type {
		case backup_managerpb.BackupProviderType_BACKUP_PROVIDER_ETCD:
			if rqst.IncludeEtcd {
				result = srv.restoreEtcd(ctx, capsuleDir, pr, rqst.Force)
			}
		case backup_managerpb.BackupProviderType_BACKUP_PROVIDER_RESTIC:
			if rqst.IncludeConfig {
				target := "/"
				if rqst.TargetNode != "" {
					target = rqst.TargetNode
				}
				result = srv.restoreRestic(ctx, pr, target)
			}
		case backup_managerpb.BackupProviderType_BACKUP_PROVIDER_MINIO:
			if rqst.IncludeMinio {
				result = srv.restoreMinio(ctx, pr)
			}
		case backup_managerpb.BackupProviderType_BACKUP_PROVIDER_SCYLLA:
			if rqst.IncludeScylla {
				result = srv.restoreScylla(ctx, pr)
			}
		}

		if result != nil {
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
	}

	job.Results = results
	job.FinishedUnixMs = time.Now().UnixMilli()

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

	_ = srv.store.SaveJob(job)

	// Write restore report into capsule
	srv.writeRestoreReport(capsuleDir, job)
}

// --- Provider restore implementations ---

func (srv *server) restoreEtcd(ctx context.Context, capsuleDir string, pr *backup_managerpb.BackupProviderResult, force bool) *backup_managerpb.BackupProviderResult {
	outputs := make(map[string]string)
	start := time.Now().UnixMilli()

	snapshotPath := filepath.Join(capsuleDir, "payload", "etcd", "etcd-snapshot.db")
	if !fileExists(snapshotPath) {
		return restoreFailResult(pr.Type, "etcd snapshot file not found in capsule",
			fmt.Sprintf("missing: %s", snapshotPath), outputs, start)
	}

	// Safety: check if etcd is running (port 2379 or systemd)
	if !force {
		if isPortOpen("127.0.0.1", 2379) {
			return restoreFailResult(pr.Type, "etcd appears to be running on port 2379; stop etcd first or use force=true",
				"ETCD_RUNNING", outputs, start)
		}
		// Also check systemd
		if isSystemdActive("etcd.service") {
			return restoreFailResult(pr.Type, "etcd.service is active; stop it first or use force=true",
				"ETCD_SERVICE_ACTIVE", outputs, start)
		}
	}

	dataDir := "/var/lib/etcd"
	if d, ok := pr.RestoreInputs["data_dir"]; ok && d != "" {
		dataDir = d
	}

	args := []string{
		"snapshot", "restore", snapshotPath,
		"--data-dir", dataDir + ".restore",
	}

	slog.Info("restoring etcd snapshot", "snapshot", snapshotPath, "data_dir", dataDir)
	stdout, stderr, err := runCmdCtx(ctx, "etcdctl", args...)
	outputs["stdout"] = strings.TrimSpace(stdout)
	outputs["stderr"] = strings.TrimSpace(stderr)

	if err != nil {
		return restoreFailResult(pr.Type, fmt.Sprintf("etcdctl snapshot restore failed: %v", err),
			err.Error(), outputs, start)
	}

	outputs["restored_data_dir"] = dataDir + ".restore"
	outputs["note"] = "Restored to .restore suffix. Move to actual data-dir and restart etcd to complete."

	return &backup_managerpb.BackupProviderResult{
		Type: pr.Type, Enabled: true,
		State:         backup_managerpb.BackupJobState_BACKUP_JOB_SUCCEEDED,
		Severity:      backup_managerpb.BackupSeverity_BACKUP_SEVERITY_INFO,
		Summary:       "etcd snapshot restored (move data-dir and restart etcd to complete)",
		Outputs:       outputs,
		StartedUnixMs: start, FinishedUnixMs: time.Now().UnixMilli(),
	}
}

func (srv *server) restoreRestic(ctx context.Context, pr *backup_managerpb.BackupProviderResult, target string) *backup_managerpb.BackupProviderResult {
	outputs := make(map[string]string)
	start := time.Now().UnixMilli()

	snapID := pr.Outputs["snapshot_id"]
	if snapID == "" {
		if id, ok := pr.RestoreInputs["snapshot_id"]; ok {
			snapID = id
		}
	}
	if snapID == "" {
		return restoreFailResult(pr.Type, "no snapshot_id recorded; cannot restore", "", outputs, start)
	}

	repo := pr.Outputs["repo_path"]
	if repo == "" {
		if r, ok := pr.RestoreInputs["repo"]; ok {
			repo = r
		}
	}
	if repo == "" {
		repo = srv.ResticRepo
	}
	password := srv.ResticPassword

	if t, ok := pr.RestoreInputs["target"]; ok && t != "" && target == "/" {
		target = t
	}

	args := []string{"restore", snapID, "--target", target, "--repo", repo}

	cmd := exec.CommandContext(ctx, "restic", args...)
	cmd.Env = appendResticEnv(repo, password)

	slog.Info("restoring restic snapshot", "snapshot", snapID, "target", target)
	out, err := cmd.CombinedOutput()
	outputs["output"] = strings.TrimSpace(string(out))
	outputs["snapshot_id"] = snapID
	outputs["target"] = target

	if err != nil {
		return restoreFailResult(pr.Type, fmt.Sprintf("restic restore failed: %v", err),
			err.Error(), outputs, start)
	}

	return &backup_managerpb.BackupProviderResult{
		Type: pr.Type, Enabled: true,
		State:         backup_managerpb.BackupJobState_BACKUP_JOB_SUCCEEDED,
		Severity:      backup_managerpb.BackupSeverity_BACKUP_SEVERITY_INFO,
		Summary:       fmt.Sprintf("restic snapshot %s restored to %s", snapID, target),
		Outputs:       outputs,
		StartedUnixMs: start, FinishedUnixMs: time.Now().UnixMilli(),
	}
}

func (srv *server) restoreMinio(ctx context.Context, pr *backup_managerpb.BackupProviderResult) *backup_managerpb.BackupProviderResult {
	outputs := make(map[string]string)
	start := time.Now().UnixMilli()

	remote := pr.Outputs["remote"]
	source := pr.Outputs["source"]
	if remote == "" {
		if r, ok := pr.RestoreInputs["remote"]; ok {
			remote = r
		}
	}
	if source == "" {
		if s, ok := pr.RestoreInputs["source"]; ok {
			source = s
		}
	}
	if remote == "" || source == "" {
		return restoreFailResult(pr.Type, "missing remote/source in provider outputs; cannot restore", "", outputs, start)
	}

	// Reverse sync: remote -> local source
	args := []string{"sync", remote, source, "--stats-one-line", "-v"}

	slog.Info("restoring minio/rclone data", "remote", remote, "target", source)
	cmd := exec.CommandContext(ctx, "rclone", args...)
	out, err := cmd.CombinedOutput()
	outputs["output"] = strings.TrimSpace(string(out))

	if err != nil {
		return restoreFailResult(pr.Type, fmt.Sprintf("rclone restore failed: %v", err),
			err.Error(), outputs, start)
	}

	return &backup_managerpb.BackupProviderResult{
		Type: pr.Type, Enabled: true,
		State:         backup_managerpb.BackupJobState_BACKUP_JOB_SUCCEEDED,
		Severity:      backup_managerpb.BackupSeverity_BACKUP_SEVERITY_INFO,
		Summary:       fmt.Sprintf("rclone data restored from %s to %s", remote, source),
		Outputs:       outputs,
		StartedUnixMs: start, FinishedUnixMs: time.Now().UnixMilli(),
	}
}

func (srv *server) restoreScylla(ctx context.Context, pr *backup_managerpb.BackupProviderResult) *backup_managerpb.BackupProviderResult {
	outputs := make(map[string]string)
	start := time.Now().UnixMilli()

	cluster := pr.Outputs["cluster"]
	if cluster == "" {
		if c, ok := pr.RestoreInputs["cluster"]; ok {
			cluster = c
		}
	}
	if cluster == "" {
		cluster = srv.ScyllaCluster
	}

	// Derive locations: try restore inputs first, then configured destinations
	var locations []string
	if l := pr.Outputs["locations"]; l != "" {
		locations = strings.Split(l, ",")
	} else if l := pr.RestoreInputs["locations"]; l != "" {
		locations = strings.Split(l, ",")
	} else if l := pr.Outputs["location"]; l != "" {
		// backwards compat with old capsules
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

	if cluster == "" || len(locations) == 0 {
		return restoreFailResult(pr.Type, "scylla cluster/location not configured; cannot restore", "", outputs, start)
	}

	args := []string{
		"restore",
		"--cluster", cluster,
		"--api-url", srv.ScyllaManagerAPI,
	}
	for _, loc := range locations {
		args = append(args, "--location", loc)
	}

	slog.Info("restoring scylladb", "cluster", cluster, "locations", strings.Join(locations, ","))
	cmd := exec.CommandContext(ctx, "sctool", args...)
	out, err := cmd.CombinedOutput()
	outputs["output"] = strings.TrimSpace(string(out))

	if err != nil {
		return restoreFailResult(pr.Type, fmt.Sprintf("sctool restore failed: %v", err),
			err.Error(), outputs, start)
	}

	return &backup_managerpb.BackupProviderResult{
		Type: pr.Type, Enabled: true,
		State:         backup_managerpb.BackupJobState_BACKUP_JOB_SUCCEEDED,
		Severity:      backup_managerpb.BackupSeverity_BACKUP_SEVERITY_INFO,
		Summary:       "scylladb restore triggered via sctool",
		Outputs:       outputs,
		StartedUnixMs: start, FinishedUnixMs: time.Now().UnixMilli(),
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
		BackupId:      rqst.BackupId,
		IncludeEtcd:   rqst.IncludeEtcd,
		IncludeConfig: rqst.IncludeConfig,
		IncludeMinio:  rqst.IncludeMinio,
		IncludeScylla: rqst.IncludeScylla,
	}
	resp, _ := srv.RestorePlan(context.Background(), planRqst)
	return resp
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

// findDestinationByName looks up a configured destination by its name.
func (srv *server) findDestinationByName(name string) *DestinationConfig {
	for i := range srv.Destinations {
		if srv.Destinations[i].Name == name {
			return &srv.Destinations[i]
		}
	}
	return nil
}
