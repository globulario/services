package main

import (
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
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// RunRestoreTest runs a sandbox restore to prove a backup can be restored.
func (srv *server) RunRestoreTest(ctx context.Context, rqst *backup_managerpb.RunRestoreTestRequest) (*backup_managerpb.RunRestoreTestResponse, error) {
	backupID := rqst.BackupId

	// If no backup_id, pick the latest UNVERIFIED or VALIDATED backup
	if backupID == "" {
		arts, _, err := srv.store.ListArtifacts("", 0, 0)
		if err != nil {
			return nil, status.Errorf(codes.Internal, "list artifacts: %v", err)
		}
		for _, a := range arts {
			if a.QualityState == backup_managerpb.QualityState_QUALITY_UNVERIFIED ||
				a.QualityState == backup_managerpb.QualityState_QUALITY_VALIDATED ||
				a.QualityState == backup_managerpb.QualityState_QUALITY_STATE_UNSPECIFIED {
				backupID = a.BackupId
				break
			}
		}
		if backupID == "" {
			return nil, status.Error(codes.NotFound, "no eligible backup found for restore test")
		}
	}

	art, err := srv.store.GetArtifact(backupID)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "backup %s not found", backupID)
	}

	level := rqst.Level
	if level == backup_managerpb.RestoreTestLevel_RESTORE_TEST_LEVEL_UNSPECIFIED {
		level = backup_managerpb.RestoreTestLevel_RESTORE_TEST_LIGHT
	}

	// Create a restore-test job
	jobID := Utility.RandomUUID()
	job := &backup_managerpb.BackupJob{
		JobId:         jobID,
		PlanName:      "restore-test:" + backupID,
		State:         backup_managerpb.BackupJobState_BACKUP_JOB_QUEUED,
		CreatedUnixMs: time.Now().UnixMilli(),
		BackupId:      backupID,
		Message:       "restore test queued",
		JobType:       backup_managerpb.BackupJobType_BACKUP_JOB_TYPE_RESTORE,
	}

	if err := srv.store.SaveJob(job); err != nil {
		return nil, status.Errorf(codes.Internal, "save job: %v", err)
	}

	slog.Info("restore test job created", "job_id", jobID, "backup_id", backupID, "level", level)

	go srv.executeRestoreTest(job, art, level, rqst.TargetRoot)

	return &backup_managerpb.RunRestoreTestResponse{
		JobId:    jobID,
		BackupId: backupID,
		Level:    level,
	}, nil
}

// executeRestoreTest runs the restore test asynchronously.
func (srv *server) executeRestoreTest(job *backup_managerpb.BackupJob, art *backup_managerpb.BackupArtifact, level backup_managerpb.RestoreTestLevel, targetRoot string) {
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
	job.Message = "running restore test"
	_ = srv.store.SaveJob(job)

	capsuleDir := srv.CapsuleDir(art.BackupId)

	// Ensure capsule exists locally
	if !fileOrDirExists(capsuleDir) {
		if err := srv.FetchCapsuleFromRemote(art.BackupId, art); err != nil {
			job.State = backup_managerpb.BackupJobState_BACKUP_JOB_FAILED
			job.Message = fmt.Sprintf("capsule missing and fetch failed: %v", err)
			job.FinishedUnixMs = time.Now().UnixMilli()
			_ = srv.store.SaveJob(job)
			return
		}
	}

	// Default sandbox root
	if targetRoot == "" {
		targetRoot = filepath.Join(srv.DataDir, "restore-tests", art.BackupId)
	}
	_ = os.MkdirAll(targetRoot, 0755)

	report := &backup_managerpb.RestoreTestReport{
		BackupId:       art.BackupId,
		Level:          level,
		Passed:         true,
		StartedUnixMs:  time.Now().UnixMilli(),
	}

	// Build set of executed providers from artifact scope
	executedProviders := make(map[string]bool)
	if art.Scope != nil && len(art.Scope.Providers) > 0 {
		for _, p := range art.Scope.Providers {
			executedProviders[p] = true
		}
	}

	for _, pr := range art.ProviderResults {
		if pr.State != backup_managerpb.BackupJobState_BACKUP_JOB_SUCCEEDED {
			continue
		}
		if ctx.Err() != nil {
			break
		}

		// Only test providers that were part of the executed scope
		name := providerName(pr.Type)
		if len(executedProviders) > 0 && !executedProviders[name] {
			continue
		}

		var check *backup_managerpb.RestoreTestCheck

		switch pr.Type {
		case backup_managerpb.BackupProviderType_BACKUP_PROVIDER_ETCD:
			check = srv.restoreTestEtcd(ctx, capsuleDir, pr, level, targetRoot)
		case backup_managerpb.BackupProviderType_BACKUP_PROVIDER_RESTIC:
			check = srv.restoreTestRestic(ctx, pr, level, targetRoot)
		case backup_managerpb.BackupProviderType_BACKUP_PROVIDER_SCYLLA:
			check = srv.restoreTestScylla(ctx, pr, level)
		default:
			check = &backup_managerpb.RestoreTestCheck{
				Provider: name,
				Ok:       true,
				Summary:  "no restore test available for this provider",
			}
		}

		if check != nil {
			report.Checks = append(report.Checks, check)
			if !check.Ok {
				report.Passed = false
			}
		}
	}

	report.FinishedUnixMs = time.Now().UnixMilli()

	// Write restore-test report
	srv.writeRestoreTestReport(capsuleDir, report)

	// Update quality state if passed
	if report.Passed {
		if art.QualityState != backup_managerpb.QualityState_QUALITY_PROMOTED {
			art.QualityState = backup_managerpb.QualityState_QUALITY_RESTORE_TESTED
			_ = srv.store.SaveArtifact(art)
		}
		job.State = backup_managerpb.BackupJobState_BACKUP_JOB_SUCCEEDED
		job.Message = "restore test passed"
		metricsJobsTotal.WithLabelValues("restore_test_succeeded").Inc()
		metricsRestoreTestedTotal.Inc()
		slog.Info("restore test passed", "job_id", job.JobId, "backup_id", art.BackupId)
	} else {
		job.State = backup_managerpb.BackupJobState_BACKUP_JOB_FAILED
		job.Message = "restore test failed"
		metricsJobsTotal.WithLabelValues("restore_test_failed").Inc()
		slog.Warn("restore test failed", "job_id", job.JobId, "backup_id", art.BackupId)
	}

	job.FinishedUnixMs = time.Now().UnixMilli()
	_ = srv.store.SaveJob(job)
}

// --- Provider-specific restore test checks ---

func (srv *server) restoreTestEtcd(ctx context.Context, capsuleDir string, pr *backup_managerpb.BackupProviderResult, level backup_managerpb.RestoreTestLevel, targetRoot string) *backup_managerpb.RestoreTestCheck {
	snapshotPath := filepath.Join(capsuleDir, "payload", "etcd", "etcd-snapshot.db")
	if !fileExists(snapshotPath) {
		return &backup_managerpb.RestoreTestCheck{
			Provider: "etcd", Ok: false,
			Summary:      "etcd snapshot file missing from capsule",
			ErrorMessage: "missing: " + snapshotPath,
		}
	}

	// LIGHT: etcdctl snapshot status
	stdout, stderr, err := runCmdCtx(ctx, "etcdctl", "snapshot", "status", snapshotPath)
	if err != nil {
		return &backup_managerpb.RestoreTestCheck{
			Provider: "etcd", Ok: false,
			Summary:      "etcdctl snapshot status failed",
			ErrorMessage: fmt.Sprintf("%s: %v", strings.TrimSpace(stderr), err),
		}
	}

	if level == backup_managerpb.RestoreTestLevel_RESTORE_TEST_HEAVY {
		// HEAVY: restore into sandbox data-dir
		sandboxDir := filepath.Join(targetRoot, "etcd-data")
		args := []string{"snapshot", "restore", snapshotPath, "--data-dir", sandboxDir}
		_, stderr, err := runCmdCtx(ctx, "etcdctl", args...)
		if err != nil {
			return &backup_managerpb.RestoreTestCheck{
				Provider: "etcd", Ok: false,
				Summary:      "etcdctl snapshot restore (sandbox) failed",
				ErrorMessage: fmt.Sprintf("%s: %v", strings.TrimSpace(stderr), err),
			}
		}
		// Cleanup sandbox
		_ = os.RemoveAll(sandboxDir)
	}

	return &backup_managerpb.RestoreTestCheck{
		Provider: "etcd", Ok: true,
		Summary: fmt.Sprintf("etcd snapshot valid (%s)", strings.TrimSpace(stdout)),
	}
}

func (srv *server) restoreTestRestic(ctx context.Context, pr *backup_managerpb.BackupProviderResult, level backup_managerpb.RestoreTestLevel, targetRoot string) *backup_managerpb.RestoreTestCheck {
	snapID := pr.Outputs["snapshot_id"]
	if snapID == "" {
		return &backup_managerpb.RestoreTestCheck{
			Provider: "restic", Ok: false,
			Summary:      "no snapshot_id recorded",
			ErrorMessage: "missing snapshot_id in provider outputs",
		}
	}

	repo := pr.Outputs["repo_path"]
	if repo == "" {
		repo = srv.ResticRepo
	}

	// LIGHT: verify snapshot exists via JSON
	cmd := exec.CommandContext(ctx, "restic", "snapshots", "--json", "--repo", repo)
	cmd.Env = appendResticEnv(repo, srv.ResticPassword)
	snapOut, err := cmd.CombinedOutput()
	if err != nil {
		return &backup_managerpb.RestoreTestCheck{
			Provider: "restic", Ok: false,
			Summary:      "cannot access restic repo",
			ErrorMessage: fmt.Sprintf("%v: %s", err, strings.TrimSpace(string(snapOut))),
		}
	}
	if !resticSnapshotExists(snapOut, snapID) {
		return &backup_managerpb.RestoreTestCheck{
			Provider: "restic", Ok: false,
			Summary:      fmt.Sprintf("snapshot %s not found in repo", snapID),
			ErrorMessage: "snapshot not found",
		}
	}

	if level == backup_managerpb.RestoreTestLevel_RESTORE_TEST_HEAVY {
		// HEAVY: restore into sandbox directory and verify files exist
		sandboxDir := filepath.Join(targetRoot, "restic-restore")
		restoreCmd := exec.CommandContext(ctx, "restic", "restore", snapID, "--target", sandboxDir, "--repo", repo)
		restoreCmd.Env = appendResticEnv(repo, srv.ResticPassword)
		out, err := restoreCmd.CombinedOutput()
		if err != nil {
			return &backup_managerpb.RestoreTestCheck{
				Provider: "restic", Ok: false,
				Summary:      "restic restore (sandbox) failed",
				ErrorMessage: fmt.Sprintf("%v: %s", err, strings.TrimSpace(string(out))),
			}
		}

		// Verify at least one file was restored
		entries, _ := os.ReadDir(sandboxDir)
		if len(entries) == 0 {
			return &backup_managerpb.RestoreTestCheck{
				Provider: "restic", Ok: false,
				Summary:      "sandbox restore produced no files",
				ErrorMessage: "empty restore directory",
			}
		}

		// Cleanup sandbox
		_ = os.RemoveAll(sandboxDir)
	}

	return &backup_managerpb.RestoreTestCheck{
		Provider: "restic", Ok: true,
		Summary: fmt.Sprintf("snapshot %s verified in repo", snapID),
	}
}

func (srv *server) restoreTestScylla(ctx context.Context, pr *backup_managerpb.BackupProviderResult, level backup_managerpb.RestoreTestLevel) *backup_managerpb.RestoreTestCheck {
	taskID := pr.Outputs["task_id"]
	if taskID == "" {
		return &backup_managerpb.RestoreTestCheck{
			Provider: "scylla", Ok: false,
			Summary:      "no task_id recorded",
			ErrorMessage: "missing task_id in provider outputs",
		}
	}

	// LIGHT: confirm task success
	stdout, _, err := runCmdCtx(ctx, "sctool", "task", "progress", taskID, "--cluster", srv.ScyllaCluster, "--api-url", srv.ScyllaManagerAPI)
	if err != nil {
		return &backup_managerpb.RestoreTestCheck{
			Provider: "scylla", Ok: false,
			Summary:      "scylla task check failed",
			ErrorMessage: err.Error(),
		}
	}

	if !containsAny(stdout, "DONE", "SUCCESS") {
		return &backup_managerpb.RestoreTestCheck{
			Provider: "scylla", Ok: false,
			Summary:      "scylla backup task may not be complete",
			ErrorMessage: "task not in DONE/SUCCESS state",
		}
	}

	// HEAVY: not supported for scylla without dry-run/sandbox
	if level == backup_managerpb.RestoreTestLevel_RESTORE_TEST_HEAVY {
		return &backup_managerpb.RestoreTestCheck{
			Provider: "scylla", Ok: true,
			Summary: "scylla heavy restore test not supported (sandbox not available); light check passed",
		}
	}

	return &backup_managerpb.RestoreTestCheck{
		Provider: "scylla", Ok: true,
		Summary: "scylla backup task confirmed complete",
	}
}

// --- Promote / Demote ---

// PromoteBackup marks a backup as PROMOTED, protecting it from retention.
func (srv *server) PromoteBackup(ctx context.Context, rqst *backup_managerpb.PromoteBackupRequest) (*backup_managerpb.PromoteBackupResponse, error) {
	art, err := srv.store.GetArtifact(rqst.BackupId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "backup %s not found", rqst.BackupId)
	}

	art.QualityState = backup_managerpb.QualityState_QUALITY_PROMOTED
	if err := srv.store.SaveArtifact(art); err != nil {
		return nil, status.Errorf(codes.Internal, "save artifact: %v", err)
	}

	slog.Info("backup promoted", "backup_id", rqst.BackupId)
	return &backup_managerpb.PromoteBackupResponse{
		Ok:           true,
		QualityState: art.QualityState,
		Message:      "backup promoted; protected from retention",
	}, nil
}

// DemoteBackup removes PROMOTED status, making backup eligible for retention.
func (srv *server) DemoteBackup(ctx context.Context, rqst *backup_managerpb.DemoteBackupRequest) (*backup_managerpb.DemoteBackupResponse, error) {
	art, err := srv.store.GetArtifact(rqst.BackupId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "backup %s not found", rqst.BackupId)
	}

	if art.QualityState != backup_managerpb.QualityState_QUALITY_PROMOTED {
		return &backup_managerpb.DemoteBackupResponse{
			Ok:           false,
			QualityState: art.QualityState,
			Message:      "backup is not promoted",
		}, nil
	}

	// Revert to the highest earned state
	art.QualityState = backup_managerpb.QualityState_QUALITY_RESTORE_TESTED
	// Check if there's actually a restore test report
	capsuleDir := srv.CapsuleDir(rqst.BackupId)
	if !fileExists(filepath.Join(capsuleDir, "reports", "restore-test.json")) {
		if fileExists(filepath.Join(capsuleDir, "reports", "validate.json")) {
			art.QualityState = backup_managerpb.QualityState_QUALITY_VALIDATED
		} else {
			art.QualityState = backup_managerpb.QualityState_QUALITY_UNVERIFIED
		}
	}

	if err := srv.store.SaveArtifact(art); err != nil {
		return nil, status.Errorf(codes.Internal, "save artifact: %v", err)
	}

	slog.Info("backup demoted", "backup_id", rqst.BackupId, "new_state", art.QualityState)
	return &backup_managerpb.DemoteBackupResponse{
		Ok:           true,
		QualityState: art.QualityState,
		Message:      "backup demoted; now eligible for retention",
	}, nil
}

// --- Restore test report ---

func (srv *server) writeRestoreTestReport(capsuleDir string, report *backup_managerpb.RestoreTestReport) {
	reportsDir := filepath.Join(capsuleDir, "reports")
	_ = os.MkdirAll(reportsDir, 0755)

	rpt := map[string]interface{}{
		"backup_id":  report.BackupId,
		"level":      report.Level.String(),
		"passed":     report.Passed,
		"started":    report.StartedUnixMs,
		"finished":   report.FinishedUnixMs,
		"checks":     make([]map[string]interface{}, 0),
	}

	for _, c := range report.Checks {
		rpt["checks"] = append(rpt["checks"].([]map[string]interface{}), map[string]interface{}{
			"provider": c.Provider,
			"ok":       c.Ok,
			"summary":  c.Summary,
			"error":    c.ErrorMessage,
		})
	}

	data, _ := json.MarshalIndent(rpt, "", "  ")
	_ = os.WriteFile(filepath.Join(reportsDir, "restore-test.json"), data, 0644)
}
