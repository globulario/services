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
	"sync"
	"time"

	"github.com/globulario/services/golang/backup_manager/backup_managerpb"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// runningJobs tracks cancelable running jobs.
type runningJobs struct {
	mu   sync.Mutex
	jobs map[string]context.CancelFunc
}

func newRunningJobs() *runningJobs {
	return &runningJobs{jobs: make(map[string]context.CancelFunc)}
}

func (r *runningJobs) add(jobID string, cancel context.CancelFunc) {
	r.mu.Lock()
	r.jobs[jobID] = cancel
	r.mu.Unlock()
}

func (r *runningJobs) remove(jobID string) {
	r.mu.Lock()
	delete(r.jobs, jobID)
	r.mu.Unlock()
}

func (r *runningJobs) cancel(jobID string) bool {
	r.mu.Lock()
	cancel, ok := r.jobs[jobID]
	r.mu.Unlock()
	if ok {
		cancel()
		return true
	}
	return false
}

func (r *runningJobs) count() int {
	r.mu.Lock()
	n := len(r.jobs)
	r.mu.Unlock()
	return n
}

// RunBackup starts a backup job asynchronously and returns the job ID.
func (srv *server) RunBackup(ctx context.Context, rqst *backup_managerpb.RunBackupRequest) (*backup_managerpb.RunBackupResponse, error) {
	if rqst.FailIfRunning && srv.active.count() >= srv.MaxConcurrentJobs {
		return nil, status.Error(codes.AlreadyExists, "maximum concurrent backup jobs reached")
	}

	plan := rqst.Plan
	if plan == nil {
		plan = defaultPlan()
	}
	if plan.Name == "" {
		plan.Name = "cluster-backup"
	}

	jobID := Utility.RandomUUID()
	now := time.Now().UnixMilli()

	job := &backup_managerpb.BackupJob{
		JobId:         jobID,
		PlanName:      plan.Name,
		State:         backup_managerpb.BackupJobState_BACKUP_JOB_QUEUED,
		JobType:       backup_managerpb.BackupJobType_BACKUP_JOB_TYPE_BACKUP,
		CreatedUnixMs: now,
		Plan:          plan,
		Message:       "queued",
	}

	if err := srv.store.SaveJob(job); err != nil {
		return nil, status.Errorf(codes.Internal, "save job: %v", err)
	}

	slog.Info("backup job created", "job_id", jobID, "plan", plan.Name)
	metricsJobsTotal.WithLabelValues("queued").Inc()

	go srv.executeJob(job, rqst.Mode, rqst.Scope, rqst.Labels)

	return &backup_managerpb.RunBackupResponse{JobId: jobID}, nil
}

// GetBackupJob returns the status of a specific job.
func (srv *server) GetBackupJob(ctx context.Context, rqst *backup_managerpb.GetBackupJobRequest) (*backup_managerpb.GetBackupJobResponse, error) {
	job, err := srv.store.GetJob(rqst.JobId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "job %s not found", rqst.JobId)
	}
	return &backup_managerpb.GetBackupJobResponse{Job: job}, nil
}

// ListBackupJobs returns job history with optional filters.
func (srv *server) ListBackupJobs(ctx context.Context, rqst *backup_managerpb.ListBackupJobsRequest) (*backup_managerpb.ListBackupJobsResponse, error) {
	jobs, total, err := srv.store.ListJobs(rqst.State, rqst.PlanName, rqst.Limit, rqst.Offset)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list jobs: %v", err)
	}
	return &backup_managerpb.ListBackupJobsResponse{Jobs: jobs, Total: total}, nil
}

// ListBackups returns completed backup artifacts.
func (srv *server) ListBackups(ctx context.Context, rqst *backup_managerpb.ListBackupsRequest) (*backup_managerpb.ListBackupsResponse, error) {
	arts, total, err := srv.store.ListArtifacts(rqst.PlanName, rqst.Mode, rqst.QualityState, rqst.Limit, rqst.Offset)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list backups: %v", err)
	}
	return &backup_managerpb.ListBackupsResponse{Backups: arts, Total: total}, nil
}

// GetBackup returns a single backup artifact.
// It enriches the artifact with evidence from disk if not already persisted in the manifest.
func (srv *server) GetBackup(ctx context.Context, rqst *backup_managerpb.GetBackupRequest) (*backup_managerpb.GetBackupResponse, error) {
	art, err := srv.store.GetArtifact(rqst.BackupId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "backup %s not found", rqst.BackupId)
	}
	srv.enrichArtifactEvidence(art)
	return &backup_managerpb.GetBackupResponse{Backup: art}, nil
}

// enrichArtifactEvidence loads evidence from disk reports if the artifact
// doesn't already have them persisted (backward compatibility for older artifacts).
func (srv *server) enrichArtifactEvidence(art *backup_managerpb.BackupArtifact) {
	capsuleDir := srv.CapsuleDir(art.BackupId)
	reportsDir := filepath.Join(capsuleDir, "reports")

	// Validation report
	if art.ValidationReport == nil {
		data, err := os.ReadFile(filepath.Join(reportsDir, "validate.json"))
		if err == nil {
			var rpt struct {
				Valid            bool                     `json:"valid"`
				ValidatedAtUnix  int64                    `json:"validated_at_unix"`
				Issues           []struct {
					Severity string `json:"severity"`
					Code     string `json:"code"`
					Message  string `json:"message"`
				} `json:"issues"`
				ReplicationChecks []struct {
					Destination  string   `json:"destination"`
					Ok           bool     `json:"ok"`
					MissingFiles []string `json:"missing_files"`
					Error        string   `json:"error"`
				} `json:"replication_checks"`
			}
			if json.Unmarshal(data, &rpt) == nil {
				vr := &backup_managerpb.ValidationReport{
					Valid:             rpt.Valid,
					ValidatedAtUnixMs: rpt.ValidatedAtUnix,
				}
				for _, iss := range rpt.Issues {
					vr.Issues = append(vr.Issues, &backup_managerpb.ValidationIssue{
						Severity: parseSeverity(iss.Severity),
						Code:     iss.Code,
						Message:  iss.Message,
					})
				}
				for _, rc := range rpt.ReplicationChecks {
					vr.ReplicationChecks = append(vr.ReplicationChecks, &backup_managerpb.ReplicationValidation{
						DestinationName: rc.Destination,
						Ok:              rc.Ok,
						MissingFiles:    rc.MissingFiles,
						ErrorMessage:    rc.Error,
					})
				}
				art.ValidationReport = vr
			}
		}
	}

	// Restore-test report
	if art.RestoreTestReport == nil {
		data, err := os.ReadFile(filepath.Join(reportsDir, "restore-test.json"))
		if err == nil {
			var rpt struct {
				BackupID string `json:"backup_id"`
				Level    string `json:"level"`
				Passed   bool   `json:"passed"`
				Started  int64  `json:"started"`
				Finished int64  `json:"finished"`
				Checks   []struct {
					Provider string `json:"provider"`
					Ok       bool   `json:"ok"`
					Summary  string `json:"summary"`
					Error    string `json:"error"`
				} `json:"checks"`
			}
			if json.Unmarshal(data, &rpt) == nil {
				rt := &backup_managerpb.RestoreTestReport{
					BackupId:       rpt.BackupID,
					Passed:         rpt.Passed,
					StartedUnixMs:  rpt.Started,
					FinishedUnixMs: rpt.Finished,
				}
				// Parse level string back to enum
				switch rpt.Level {
				case "RESTORE_TEST_LIGHT":
					rt.Level = backup_managerpb.RestoreTestLevel_RESTORE_TEST_LIGHT
				case "RESTORE_TEST_HEAVY":
					rt.Level = backup_managerpb.RestoreTestLevel_RESTORE_TEST_HEAVY
				}
				for _, c := range rpt.Checks {
					rt.Checks = append(rt.Checks, &backup_managerpb.RestoreTestCheck{
						Provider:     c.Provider,
						Ok:           c.Ok,
						Summary:      c.Summary,
						ErrorMessage: c.Error,
					})
				}
				art.RestoreTestReport = rt
			}
		}
	}

	// Node coverage
	if len(art.NodeCoverage) == 0 {
		data, err := os.ReadFile(filepath.Join(capsuleDir, "meta", "coverage.json"))
		if err == nil {
			var coverages []struct {
				Provider string `json:"provider"`
				Nodes    []struct {
					NodeID   string `json:"node_id"`
					Hostname string `json:"hostname"`
					Ok       bool   `json:"ok"`
					Error    string `json:"error"`
				} `json:"nodes"`
			}
			if json.Unmarshal(data, &coverages) == nil {
				for _, cov := range coverages {
					report := &backup_managerpb.NodeCoverageReport{
						Provider: cov.Provider,
						Total:    uint32(len(cov.Nodes)),
					}
					for _, n := range cov.Nodes {
						report.Entries = append(report.Entries, &backup_managerpb.NodeCoverageReportEntry{
							NodeId:       n.NodeID,
							Hostname:     n.Hostname,
							Ok:           n.Ok,
							ErrorMessage: n.Error,
						})
						if n.Ok {
							report.Succeeded++
						} else {
							report.Failed++
						}
					}
					art.NodeCoverage = append(art.NodeCoverage, report)
				}
			}
		}
	}
}

func parseSeverity(s string) backup_managerpb.BackupSeverity {
	switch s {
	case "BACKUP_SEVERITY_INFO":
		return backup_managerpb.BackupSeverity_BACKUP_SEVERITY_INFO
	case "BACKUP_SEVERITY_WARN":
		return backup_managerpb.BackupSeverity_BACKUP_SEVERITY_WARN
	case "BACKUP_SEVERITY_ERROR":
		return backup_managerpb.BackupSeverity_BACKUP_SEVERITY_ERROR
	default:
		return backup_managerpb.BackupSeverity_BACKUP_SEVERITY_UNSPECIFIED
	}
}

// DeleteBackup removes a backup artifact and optionally cleans up provider data.
func (srv *server) DeleteBackup(ctx context.Context, rqst *backup_managerpb.DeleteBackupRequest) (*backup_managerpb.DeleteBackupResponse, error) {
	art, err := srv.store.GetArtifact(rqst.BackupId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "backup %s not found", rqst.BackupId)
	}

	var provResults []*backup_managerpb.DeleteResult
	var repResults []*backup_managerpb.DeleteResult

	if rqst.DeleteProviderArtifacts {
		// Provider-specific cleanup (gated by config)
		for _, pr := range art.ProviderResults {
			if pr.State != backup_managerpb.BackupJobState_BACKUP_JOB_SUCCEEDED {
				continue
			}
			switch pr.Type {
			case backup_managerpb.BackupProviderType_BACKUP_PROVIDER_RESTIC:
				if snapID, ok := pr.Outputs["snapshot_id"]; ok && snapID != "" {
					if srv.AllowResticPruneOnDelete {
						srv.cleanupResticSnapshot(snapID, pr.Outputs)
						provResults = append(provResults, &backup_managerpb.DeleteResult{
							Target: "restic:" + snapID, Ok: true, Message: "forget/prune requested",
						})
					} else {
						provResults = append(provResults, &backup_managerpb.DeleteResult{
							Target: "restic:" + snapID, Ok: false, Message: "skipped: AllowResticPruneOnDelete=false",
						})
					}
				}
			}
		}

		// Delete replicated capsule copies (gated by config)
		if srv.AllowRemoteDelete {
			for _, dest := range srv.resolveDestinations(nil) {
				if dest.Primary && dest.Type == "local" {
					continue
				}
				if err := srv.deleteReplicatedCapsule(rqst.BackupId, dest); err != nil {
					slog.Warn("failed to delete replicated capsule", "dest", dest.Name, "error", err)
					repResults = append(repResults, &backup_managerpb.DeleteResult{
						Target: dest.Name, Ok: false, Message: err.Error(),
					})
				} else {
					repResults = append(repResults, &backup_managerpb.DeleteResult{
						Target: dest.Name, Ok: true, Message: "deleted",
					})
				}
			}
		}
	}

	// Delete local capsule
	if err := srv.store.DeleteArtifact(rqst.BackupId); err != nil {
		return nil, status.Errorf(codes.Internal, "delete backup: %v", err)
	}
	metricsArtifactsTotal.Dec()

	slog.Info("backup deleted", "backup_id", rqst.BackupId)
	return &backup_managerpb.DeleteBackupResponse{
		Deleted:            true,
		Message:            "deleted",
		ProviderResults:    provResults,
		ReplicationResults: repResults,
	}, nil
}

// ValidateBackup checks integrity of a backup artifact.
func (srv *server) ValidateBackup(ctx context.Context, rqst *backup_managerpb.ValidateBackupRequest) (*backup_managerpb.ValidateBackupResponse, error) {
	valid, issues := srv.store.ValidateArtifact(rqst.BackupId, rqst.Deep)

	var repChecks []*backup_managerpb.ReplicationValidation

	if rqst.Deep {
		providerIssues := srv.validateProviders(rqst.BackupId)
		issues = append(issues, providerIssues...)
		for _, iss := range providerIssues {
			if iss.Severity == backup_managerpb.BackupSeverity_BACKUP_SEVERITY_ERROR {
				valid = false
			}
		}

		// Verify replication targets
		repChecks = srv.validateReplications(rqst.BackupId)
		for _, rc := range repChecks {
			if !rc.Ok {
				valid = false
				issues = append(issues, &backup_managerpb.ValidationIssue{
					Severity: backup_managerpb.BackupSeverity_BACKUP_SEVERITY_ERROR,
					Code:     "REPLICATION_INCOMPLETE",
					Message:  fmt.Sprintf("destination %s: %s", rc.DestinationName, rc.ErrorMessage),
				})
			}
		}
	}

	// Track metrics
	if valid {
		metricsValidationsTotal.WithLabelValues("valid").Inc()
	} else {
		metricsValidationsTotal.WithLabelValues("invalid").Inc()
	}

	// Write validate report if deep
	if rqst.Deep {
		srv.writeValidateReport(rqst.BackupId, valid, issues, repChecks)

		// Persist validation evidence into the artifact
		art, err := srv.store.GetArtifact(rqst.BackupId)
		if err == nil {
			art.ValidationReport = &backup_managerpb.ValidationReport{
				Valid:             valid,
				ValidatedAtUnixMs: time.Now().UnixMilli(),
				Issues:            issues,
				ReplicationChecks: repChecks,
			}
			// Upgrade quality state on successful deep validation
			if valid && art.QualityState != backup_managerpb.QualityState_QUALITY_PROMOTED &&
				art.QualityState != backup_managerpb.QualityState_QUALITY_RESTORE_TESTED {
				art.QualityState = backup_managerpb.QualityState_QUALITY_VALIDATED
			}
			_ = srv.store.SaveArtifact(art)
		}
	}

	return &backup_managerpb.ValidateBackupResponse{
		Valid:             valid,
		Issues:            issues,
		ReplicationChecks: repChecks,
	}, nil
}

// CancelBackupJob cancels a running backup job.
func (srv *server) CancelBackupJob(ctx context.Context, rqst *backup_managerpb.CancelBackupJobRequest) (*backup_managerpb.CancelBackupJobResponse, error) {
	if srv.active.cancel(rqst.JobId) {
		slog.Info("backup job canceled", "job_id", rqst.JobId)
		return &backup_managerpb.CancelBackupJobResponse{Canceled: true, Message: "cancel signal sent"}, nil
	}

	// Check if job exists but is not running
	job, err := srv.store.GetJob(rqst.JobId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "job %s not found", rqst.JobId)
	}
	if job.State == backup_managerpb.BackupJobState_BACKUP_JOB_SUCCEEDED ||
		job.State == backup_managerpb.BackupJobState_BACKUP_JOB_FAILED ||
		job.State == backup_managerpb.BackupJobState_BACKUP_JOB_CANCELED {
		return &backup_managerpb.CancelBackupJobResponse{Canceled: false, Message: fmt.Sprintf("job already in terminal state: %s", job.State)}, nil
	}

	return &backup_managerpb.CancelBackupJobResponse{Canceled: false, Message: "job not found in active set"}, nil
}

// DeleteBackupJob removes a backup job record and optionally its artifacts.
func (srv *server) DeleteBackupJob(ctx context.Context, rqst *backup_managerpb.DeleteBackupJobRequest) (*backup_managerpb.DeleteBackupJobResponse, error) {
	job, err := srv.store.GetJob(rqst.JobId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "job %s not found", rqst.JobId)
	}

	// Don't allow deleting running jobs — cancel first
	if job.State == backup_managerpb.BackupJobState_BACKUP_JOB_RUNNING ||
		job.State == backup_managerpb.BackupJobState_BACKUP_JOB_QUEUED {
		return nil, status.Errorf(codes.FailedPrecondition, "cannot delete a running/pending job — cancel it first")
	}

	// Optionally delete the associated backup artifact
	if rqst.DeleteArtifacts && job.BackupId != "" {
		if _, delErr := srv.DeleteBackup(ctx, &backup_managerpb.DeleteBackupRequest{
			BackupId:                job.BackupId,
			DeleteProviderArtifacts: true,
		}); delErr != nil {
			slog.Warn("failed to delete backup artifact for job", "job_id", rqst.JobId, "backup_id", job.BackupId, "error", delErr)
		}
	}

	if err := srv.store.DeleteJob(rqst.JobId); err != nil {
		return nil, status.Errorf(codes.Internal, "delete job: %v", err)
	}

	slog.Info("backup job deleted", "job_id", rqst.JobId)
	return &backup_managerpb.DeleteBackupJobResponse{
		Deleted: true,
		Message: fmt.Sprintf("job %s deleted", rqst.JobId),
	}, nil
}

// RestorePlan generates a read-only preview of what a restore would do.
func (srv *server) RestorePlan(ctx context.Context, rqst *backup_managerpb.RestorePlanRequest) (*backup_managerpb.RestorePlanResponse, error) {
	art, err := srv.store.GetArtifact(rqst.BackupId)
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "backup %s not found", rqst.BackupId)
	}

	var steps []*backup_managerpb.RestoreStep
	var warnings []*backup_managerpb.ValidationIssue
	order := uint32(1)

	for _, pr := range art.ProviderResults {
		if pr.State != backup_managerpb.BackupJobState_BACKUP_JOB_SUCCEEDED {
			continue
		}
		switch pr.Type {
		case backup_managerpb.BackupProviderType_BACKUP_PROVIDER_ETCD:
			if rqst.IncludeEtcd {
				steps = append(steps, &backup_managerpb.RestoreStep{
					Order:   order,
					Title:   "Restore etcd snapshot",
					Details: "Stop etcd, restore snapshot, restart etcd. Requires cluster downtime.",
				})
				order++
				warnings = append(warnings, &backup_managerpb.ValidationIssue{
					Severity: backup_managerpb.BackupSeverity_BACKUP_SEVERITY_WARN,
					Code:     "DOWNTIME_REQUIRED",
					Message:  "etcd restore requires full cluster downtime",
				})
			}
		case backup_managerpb.BackupProviderType_BACKUP_PROVIDER_RESTIC:
			if rqst.IncludeConfig {
				steps = append(steps, &backup_managerpb.RestoreStep{
					Order:   order,
					Title:   "Restore configuration files",
					Details: "Restore filesystem config/state from restic snapshot.",
				})
				order++
			}
		case backup_managerpb.BackupProviderType_BACKUP_PROVIDER_MINIO:
			if rqst.IncludeMinio {
				steps = append(steps, &backup_managerpb.RestoreStep{
					Order:   order,
					Title:   "Restore MinIO object data",
					Details: "Sync backed-up objects back into the MinIO source bucket using rclone. This overwrites existing objects in the source bucket with the backed-up versions.",
				})
				order++
			}
		case backup_managerpb.BackupProviderType_BACKUP_PROVIDER_SCYLLA:
			if rqst.IncludeScylla {
				steps = append(steps, &backup_managerpb.RestoreStep{
					Order:   order,
					Title:   "Restore ScyllaDB",
					Details: "Restore ScyllaDB from snapshot via scylla-manager.",
				})
				order++
				warnings = append(warnings, &backup_managerpb.ValidationIssue{
					Severity: backup_managerpb.BackupSeverity_BACKUP_SEVERITY_WARN,
					Code:     "DOWNTIME_REQUIRED",
					Message:  "ScyllaDB restore may require cluster downtime",
				})
			}
		}
	}

	if len(steps) == 0 {
		steps = append(steps, &backup_managerpb.RestoreStep{
			Order:   1,
			Title:   "No restore actions selected",
			Details: "Enable at least one provider (etcd, config, minio, scylla) to generate a restore plan.",
		})
	}

	return &backup_managerpb.RestorePlanResponse{
		BackupId:          rqst.BackupId,
		Steps:             steps,
		Warnings:          warnings,
		ConfirmationToken: srv.generateConfirmationToken(rqst.BackupId),
	}, nil
}

// --- Job execution (async) ---

func (srv *server) executeJob(job *backup_managerpb.BackupJob, mode backup_managerpb.BackupMode, scope *backup_managerpb.BackupScope, labels map[string]string) {
	// Acquire semaphore slot
	srv.sem <- struct{}{}
	defer func() { <-srv.sem }()

	ctx, cancel := context.WithCancel(context.Background())
	srv.active.add(job.JobId, cancel)
	defer func() {
		cancel()
		srv.active.remove(job.JobId)
	}()

	metricsRunning.Set(float64(srv.active.count()))
	defer func() { metricsRunning.Set(float64(srv.active.count() - 1)) }()

	job.State = backup_managerpb.BackupJobState_BACKUP_JOB_RUNNING
	job.StartedUnixMs = time.Now().UnixMilli()
	job.Message = "running"
	_ = srv.store.SaveJob(job)

	slog.Info("backup job started", "job_id", job.JobId, "plan", job.PlanName, "mode", mode)

	// Generate backup ID early so providers can write into the capsule
	backupID := Utility.RandomUUID()

	// --- Phase 1: Distributed lock for CLUSTER mode ---
	var clusterLock *ClusterLock
	if mode == backup_managerpb.BackupMode_BACKUP_MODE_CLUSTER {
		var lockErr error
		clusterLock, lockErr = srv.AcquireClusterLock(ctx, job.JobId, backupID)
		if lockErr != nil {
			job.State = backup_managerpb.BackupJobState_BACKUP_JOB_FAILED
			job.Message = fmt.Sprintf("cluster lock: %v", lockErr)
			job.FinishedUnixMs = time.Now().UnixMilli()
			_ = srv.store.SaveJob(job)
			metricsJobsTotal.WithLabelValues("failed").Inc()
			slog.Warn("cluster lock acquisition failed", "job_id", job.JobId, "error", lockErr)
			return
		}
		defer clusterLock.Release()
	}

	// Create capsule root
	if err := srv.EnsureCapsuleDir(backupID); err != nil {
		job.State = backup_managerpb.BackupJobState_BACKUP_JOB_FAILED
		job.Message = fmt.Sprintf("create capsule dir: %v", err)
		job.FinishedUnixMs = time.Now().UnixMilli()
		_ = srv.store.SaveJob(job)
		metricsJobsTotal.WithLabelValues("failed").Inc()
		return
	}

	// --- Phase 2: Topology snapshot (CLUSTER mode) ---
	var clusterInfo *backup_managerpb.ClusterInfo
	var topologyNodes []TopologyNode
	if mode == backup_managerpb.BackupMode_BACKUP_MODE_CLUSTER {
		var topoErr error
		clusterInfo, topoErr = srv.captureTopology(backupID)
		if topoErr != nil {
			slog.Warn("topology capture failed (non-fatal)", "job_id", job.JobId, "error", topoErr)
			clusterInfo = &backup_managerpb.ClusterInfo{Domain: srv.Domain}
		}
		// Also save the discovered nodes for fan-out
		topologyNodes = srv.discoverNodes()
	} else {
		clusterInfo = &backup_managerpb.ClusterInfo{Domain: srv.Domain}
	}

	// Resolve providers via scope/mode/plan/config (with availability checks)
	resolved, err := ResolveProviders(mode, scope, job.Plan.Providers, srv)
	if err != nil {
		job.State = backup_managerpb.BackupJobState_BACKUP_JOB_FAILED
		job.Message = fmt.Sprintf("provider resolution failed: %v", err)
		job.FinishedUnixMs = time.Now().UnixMilli()
		_ = srv.store.SaveJob(job)
		metricsJobsTotal.WithLabelValues("failed").Inc()
		slog.Warn("provider resolution failed", "job_id", job.JobId, "error", err)
		return
	}
	providers := resolved.Specs
	resolvedNames := resolved.Names

	if len(resolved.Skipped) > 0 {
		var skippedNames []string
		for _, s := range resolved.Skipped {
			skippedNames = append(skippedNames, s.Name+"("+s.Reason+")")
		}
		slog.Info("providers skipped", "job_id", job.JobId, "skipped", skippedNames)
	}

	slog.Info("resolved providers", "job_id", job.JobId, "providers", resolvedNames, "mode", mode)

	// --- Phase 3: Validate hook coverage (CLUSTER mode) ---
	var hookSummary *backup_managerpb.HookSummary
	if mode == backup_managerpb.BackupMode_BACKUP_MODE_CLUSTER {
		hookSummary = &backup_managerpb.HookSummary{}

		// Validate coverage before running hooks
		targets := srv.resolveHookTargets(scope)
		if err := srv.validateHookCoverage(targets); err != nil {
			job.State = backup_managerpb.BackupJobState_BACKUP_JOB_FAILED
			job.Message = fmt.Sprintf("hook coverage validation failed: %v", err)
			job.FinishedUnixMs = time.Now().UnixMilli()
			_ = srv.store.SaveJob(job)
			metricsJobsTotal.WithLabelValues("failed").Inc()
			slog.Warn("hook coverage validation failed", "job_id", job.JobId, "error", err)
			return
		}

		// Quiesce hooks: PrepareBackup
		prepareResults := srv.runPrepareHooks(ctx, backupID, mode, scope, labels)
		hookSummary.Prepare = prepareResults

		if anyHookFailed(prepareResults) && srv.HookStrict {
			job.State = backup_managerpb.BackupJobState_BACKUP_JOB_FAILED
			job.Message = "aborted: prepare hook failed (HookStrict=true)"
			job.FinishedUnixMs = time.Now().UnixMilli()
			_ = srv.store.SaveJob(job)
			metricsJobsTotal.WithLabelValues("failed").Inc()

			finalizeResults := srv.runFinalizeHooks(ctx, backupID, mode, scope, labels, false)
			hookSummary.Finalize = finalizeResults
			slog.Warn("backup aborted by strict hook policy", "job_id", job.JobId)
			return
		}
	}

	// --- Phase 4 & 5: Run providers with scope classification ---
	var results []*backup_managerpb.BackupProviderResult
	var coverages []*NodeCoverage
	allOk := true
	totalProviders := 0
	for _, s := range providers {
		if s.Enabled {
			totalProviders++
		}
	}
	providerIdx := 0

	for _, spec := range providers {
		if !spec.Enabled {
			continue
		}

		// Check cancellation
		if ctx.Err() != nil {
			job.State = backup_managerpb.BackupJobState_BACKUP_JOB_CANCELED
			job.Message = "canceled by user"
			job.FinishedUnixMs = time.Now().UnixMilli()
			job.Results = results
			_ = srv.store.SaveJob(job)
			metricsJobsTotal.WithLabelValues("canceled").Inc()
			slog.Info("backup job canceled", "job_id", job.JobId)

			if hookSummary != nil {
				hookSummary.Finalize = srv.runFinalizeHooks(ctx, backupID, mode, scope, labels, false)
			}
			return
		}

		name := providerName(spec.Type)
		providerIdx++

		// Update job progress before running provider
		job.Message = fmt.Sprintf("running %s (%d/%d)", name, providerIdx, totalProviders)
		job.Results = results
		_ = srv.store.SaveJob(job)

		// Phase 5: Classify provider scope
		if mode == backup_managerpb.BackupMode_BACKUP_MODE_CLUSTER && providerScope(spec.Type) == ProviderScopeNode {
			// Phase 4: Fan-out to all nodes
			result, coverage := srv.runProviderOnAllNodes(ctx, spec, backupID, topologyNodes)
			results = append(results, result)
			coverages = append(coverages, coverage)
			if result.State == backup_managerpb.BackupJobState_BACKUP_JOB_FAILED {
				allOk = false
			}
		} else {
			// Cluster-scoped or SERVICE mode: run once locally
			cc, err := srv.NewCapsuleContext(backupID, name)
			if err != nil {
				results = append(results, failResult(spec.Type, fmt.Sprintf("create capsule context: %v", err), nil))
				allOk = false
				continue
			}
			result := srv.runProvider(ctx, spec, cc)
			results = append(results, result)
			if result.State == backup_managerpb.BackupJobState_BACKUP_JOB_FAILED {
				allOk = false
			}
		}

		// Save intermediate results so the UI can see completed providers
		job.Results = results
		_ = srv.store.SaveJob(job)
	}

	// Quiesce hooks: FinalizeBackup (CLUSTER mode only, always runs)
	if hookSummary != nil {
		finalizeResults := srv.runFinalizeHooks(ctx, backupID, mode, scope, labels, allOk)
		hookSummary.Finalize = finalizeResults
	}

	// --- Phase 6: Write coverage metadata ---
	if len(coverages) > 0 {
		if err := srv.writeCoverage(backupID, coverages); err != nil {
			slog.Warn("failed to write coverage", "job_id", job.JobId, "error", err)
		}
	}

	job.Results = results
	job.FinishedUnixMs = time.Now().UnixMilli()
	durationSec := float64(job.FinishedUnixMs-job.StartedUnixMs) / 1000.0

	if allOk {
		job.State = backup_managerpb.BackupJobState_BACKUP_JOB_SUCCEEDED
		job.BackupId = backupID
		job.Message = "completed successfully"

		// Compress capsule if configured
		var archivePath string
		if srv.CompressCapsule {
			var compErr error
			archivePath, compErr = compressCapsule(srv.CapsuleDir(backupID))
			if compErr != nil {
				slog.Warn("capsule compression failed (continuing without)", "backup_id", backupID, "error", compErr)
			} else {
				slog.Info("capsule compressed", "backup_id", backupID, "archive", archivePath)
			}
		}
		_ = archivePath // archive is available alongside the capsule dir for replication

		// Write a preliminary manifest before replication so that the capsule
		// directory contains manifest.json + manifest.sha256 when copied to
		// remote destinations. The manifest is updated again after replication
		// with the final replication results.
		manifestSHAPre := computeCapsuleSHA(srv.CapsuleDir(backupID))
		preArt := &backup_managerpb.BackupArtifact{
			BackupId:        backupID,
			CreatedUnixMs:   time.Now().UnixMilli(),
			Location:        srv.CapsuleDir(backupID),
			PlanName:        job.PlanName,
			Domain:          srv.Domain,
			ProviderResults: results,
			SchemaVersion:   2,
			ManifestSha256:  manifestSHAPre,
		}
		if err := srv.store.SaveArtifact(preArt); err != nil {
			slog.Warn("failed to write pre-replication manifest", "backup_id", backupID, "error", err)
		}

		// Replicate capsule to configured destinations
		job.Message = "replicating to destinations"
		_ = srv.store.SaveJob(job)
		repResults := srv.replicateToDestinations(backupID, job.Plan)
		job.Replications = repResults

		// Collect location paths from successful replications
		var locations []string
		locations = append(locations, srv.CapsuleDir(backupID))
		for _, r := range repResults {
			if r.State == backup_managerpb.BackupJobState_BACKUP_JOB_SUCCEEDED && r.DestinationPath != srv.DataDir {
				locations = append(locations, r.DestinationPath)
			}
		}

		// Default mode to SERVICE if unspecified
		if mode == backup_managerpb.BackupMode_BACKUP_MODE_UNSPECIFIED {
			mode = backup_managerpb.BackupMode_BACKUP_MODE_SERVICE
		}

		// Build the executed scope (what actually ran)
		executedScope := &backup_managerpb.BackupScope{
			Providers: resolvedNames,
		}
		if scope != nil {
			executedScope.Services = scope.Services
		}

		// Compute total bytes from provider results
		var totalBytes uint64
		for _, r := range results {
			totalBytes += r.BytesWritten
		}

		// Compute total replication bytes
		var repTotalBytes uint64
		for _, r := range repResults {
			repTotalBytes += r.BytesWritten
		}

		// Compute manifest SHA from capsule directory
		manifestSHA := computeCapsuleSHA(srv.CapsuleDir(backupID))

		// Created by: extract from labels or default
		createdBy := ""
		if labels != nil {
			createdBy = labels["created_by"]
		}

		// Build node coverage proto from internal coverages
		var nodeCovProtos []*backup_managerpb.NodeCoverageReport
		for _, cov := range coverages {
			report := &backup_managerpb.NodeCoverageReport{
				Provider: cov.Provider,
				Total:    uint32(len(cov.Nodes)),
			}
			for _, entry := range cov.Nodes {
				report.Entries = append(report.Entries, &backup_managerpb.NodeCoverageReportEntry{
					NodeId:       entry.NodeID,
					Hostname:     entry.Hostname,
					Ok:           entry.Ok,
					ErrorMessage: entry.Error,
				})
				if entry.Ok {
					report.Succeeded++
				} else {
					report.Failed++
				}
			}
			nodeCovProtos = append(nodeCovProtos, report)
		}

		// Create artifact
		art := &backup_managerpb.BackupArtifact{
			BackupId:         backupID,
			CreatedUnixMs:    time.Now().UnixMilli(),
			CompletedUnixMs:  job.FinishedUnixMs,
			Location:         srv.CapsuleDir(backupID),
			Locations:        locations,
			PlanName:         job.PlanName,
			Domain:           srv.Domain,
			CreatedBy:        createdBy,
			ProviderResults:  results,
			Replications:     repResults,
			SchemaVersion:    2,
			Mode:             mode,
			Scope:            executedScope,
			Labels:           labels,
			QualityState:     backup_managerpb.QualityState_QUALITY_UNVERIFIED,
			SkippedProviders: resolved.Skipped,
			Cluster:          clusterInfo,
			Hooks:            hookSummary,
			TotalBytes:       totalBytes,
			ManifestSha256:   manifestSHA,
			NodeCoverage:     nodeCovProtos,
		}
		if err := srv.store.SaveArtifact(art); err != nil {
			slog.Error("failed to save artifact", "job_id", job.JobId, "err", err)
		}
		srv.updateRecoverySeedAfterBackup(art)
		metricsArtifactsTotal.Inc()
		metricsLastSuccess.SetToCurrentTime()
		metricsLastDuration.Set(durationSec)
		metricsJobsTotal.WithLabelValues("succeeded").Inc()
		slog.Info("backup job succeeded", "job_id", job.JobId, "backup_id", backupID, "duration_s", durationSec, "providers", resolvedNames)
	} else {
		job.State = backup_managerpb.BackupJobState_BACKUP_JOB_FAILED
		job.Message = "one or more providers failed"
		metricsLastDuration.Set(durationSec)
		metricsJobsTotal.WithLabelValues("failed").Inc()
		slog.Warn("backup job failed", "job_id", job.JobId, "duration_s", durationSec)
	}

	_ = srv.store.SaveJob(job)
}

func defaultPlan() *backup_managerpb.BackupPlan {
	return &backup_managerpb.BackupPlan{
		Name: "cluster-backup",
		Providers: []*backup_managerpb.BackupProviderSpec{
			{Type: backup_managerpb.BackupProviderType_BACKUP_PROVIDER_ETCD, Enabled: true},
			{Type: backup_managerpb.BackupProviderType_BACKUP_PROVIDER_RESTIC, Enabled: true},
		},
	}
}

// --- Provider-specific validation ---

func (srv *server) validateProviders(backupID string) []*backup_managerpb.ValidationIssue {
	var issues []*backup_managerpb.ValidationIssue
	capsuleDir := srv.CapsuleDir(backupID)

	art, err := srv.store.GetArtifact(backupID)
	if err != nil {
		return issues
	}

	for _, pr := range art.ProviderResults {
		if pr.State != backup_managerpb.BackupJobState_BACKUP_JOB_SUCCEEDED {
			continue
		}
		name := providerName(pr.Type)

		switch pr.Type {
		case backup_managerpb.BackupProviderType_BACKUP_PROVIDER_ETCD:
			// Check snapshot file exists in capsule
			snapshotPath := filepath.Join(capsuleDir, "payload", "etcd", "etcd-snapshot.db")
			if !fileExists(snapshotPath) {
				issues = append(issues, &backup_managerpb.ValidationIssue{
					Severity: backup_managerpb.BackupSeverity_BACKUP_SEVERITY_ERROR,
					Code:     "ETCD_SNAPSHOT_MISSING",
					Message:  "etcd snapshot file missing from capsule",
				})
				continue
			}
			// Verify snapshot with etcdctl
			_, stderr, err := runCmd("etcdctl", "snapshot", "status", snapshotPath)
			if err != nil {
				issues = append(issues, &backup_managerpb.ValidationIssue{
					Severity: backup_managerpb.BackupSeverity_BACKUP_SEVERITY_ERROR,
					Code:     "ETCD_SNAPSHOT_CORRUPT",
					Message:  fmt.Sprintf("etcd snapshot verification failed: %s", stderr),
				})
			}

		case backup_managerpb.BackupProviderType_BACKUP_PROVIDER_RESTIC:
			snapID := pr.Outputs["snapshot_id"]
			if snapID == "" {
				issues = append(issues, &backup_managerpb.ValidationIssue{
					Severity: backup_managerpb.BackupSeverity_BACKUP_SEVERITY_WARN,
					Code:     "RESTIC_SNAPSHOT_ID_MISSING",
					Message:  "no snapshot_id recorded for restic provider (backup created before snapshot tracking was added)",
				})
				continue
			}
			repo := pr.Outputs["repo_path"]
			if repo == "" {
				repo = srv.ResticRepo
			}
			// Verify exact snapshot_id exists by parsing restic JSON output
			cmd := exec.CommandContext(context.Background(), "restic", "snapshots", "--json", "--repo", repo)
			cmd.Env = append(os.Environ(), "RESTIC_REPOSITORY="+repo, "RESTIC_PASSWORD="+srv.ResticPassword)
			snapOut, err := cmd.CombinedOutput()
			if err != nil {
				issues = append(issues, &backup_managerpb.ValidationIssue{
					Severity: backup_managerpb.BackupSeverity_BACKUP_SEVERITY_ERROR,
					Code:     "RESTIC_REPO_UNREACHABLE",
					Message:  fmt.Sprintf("cannot access restic repo: %v", err),
				})
			} else if !resticSnapshotExists(snapOut, snapID) {
				issues = append(issues, &backup_managerpb.ValidationIssue{
					Severity: backup_managerpb.BackupSeverity_BACKUP_SEVERITY_ERROR,
					Code:     "RESTIC_SNAPSHOT_NOT_FOUND",
					Message:  fmt.Sprintf("snapshot %s not found in restic repo %s", snapID, repo),
				})
			}

		case backup_managerpb.BackupProviderType_BACKUP_PROVIDER_SCYLLA:
			taskID := pr.Outputs["task_id"]
			if taskID == "" {
				issues = append(issues, &backup_managerpb.ValidationIssue{
					Severity: backup_managerpb.BackupSeverity_BACKUP_SEVERITY_WARN,
					Code:     "SCYLLA_TASK_ID_MISSING",
					Message:  "no task_id recorded for scylla provider",
				})
				continue
			}
			// Check task status
			scyllaArgs := []string{"task", "progress", taskID, "--cluster", srv.ScyllaCluster}
			if srv.ScyllaManagerAPI != "" && srv.ScyllaManagerAPI != "http://127.0.0.1:5080" {
				scyllaArgs = append(scyllaArgs, "--api-url", srv.ScyllaManagerAPI)
			}
			stdout, stderr, err := runCmd("sctool", scyllaArgs...)
			if err != nil {
				detail := strings.TrimSpace(stderr)
				if detail == "" {
					detail = err.Error()
				}
				issues = append(issues, &backup_managerpb.ValidationIssue{
					Severity: backup_managerpb.BackupSeverity_BACKUP_SEVERITY_ERROR,
					Code:     "SCYLLA_TASK_FAILED",
					Message:  fmt.Sprintf("scylla backup task check failed: %s", detail),
				})
			} else if !containsAny(stdout, "DONE", "SUCCESS") {
				issues = append(issues, &backup_managerpb.ValidationIssue{
					Severity: backup_managerpb.BackupSeverity_BACKUP_SEVERITY_WARN,
					Code:     "SCYLLA_TASK_INCOMPLETE",
					Message:  "scylla backup task may not be complete",
				})
			}
		default:
			_ = name // provider dir exists check could go here
		}
	}

	return issues
}

// --- Provider-specific cleanup ---

func (srv *server) cleanupResticSnapshot(snapshotID string, outputs map[string]string) {
	repo := srv.ResticRepo
	if r, ok := outputs["repo_path"]; ok && r != "" {
		repo = r
	}
	password := srv.ResticPassword

	cmd := exec.CommandContext(context.Background(), "restic", "forget", "--prune", snapshotID, "--repo", repo)
	cmd.Env = append(os.Environ(), "RESTIC_REPOSITORY="+repo, "RESTIC_PASSWORD="+password)
	if out, err := cmd.CombinedOutput(); err != nil {
		slog.Warn("restic forget failed", "snapshot", snapshotID, "error", err, "output", string(out))
	} else {
		slog.Info("restic forget completed", "snapshot", snapshotID)
	}
}

func (srv *server) deleteReplicatedCapsule(backupID string, dest DestinationConfig) error {
	switch dest.Type {
	case "local", "nfs":
		capsulePath := filepath.Join(dest.Path, "artifacts", backupID)
		return os.RemoveAll(capsulePath)
	case "minio", "s3":
		remotePath := fmt.Sprintf(":s3:%s/artifacts/%s", dest.Path, backupID)
		args := []string{"purge", remotePath}
		if dest.Type == "minio" {
			args = append(args, "--s3-provider", "Minio", "--s3-endpoint", dest.Options["endpoint"])
			if ak := dest.Options["access_key"]; ak != "" {
				args = append(args, "--s3-access-key-id", ak)
			}
			if sk := dest.Options["secret_key"]; sk != "" {
				args = append(args, "--s3-secret-access-key", sk)
			}
			// Skip TLS verification for internal MinIO with self-signed certs
			if strings.HasPrefix(dest.Options["endpoint"], "https") {
				args = append(args, "--no-check-certificate")
			}
		}
		_, stderr, err := runCmd("rclone", args...)
		if err != nil {
			return fmt.Errorf("rclone purge: %s: %w", stderr, err)
		}
		return nil
	case "rclone":
		remotePath := fmt.Sprintf("%s/artifacts/%s", dest.Path, backupID)
		_, stderr, err := runCmd("rclone", "purge", remotePath)
		if err != nil {
			return fmt.Errorf("rclone purge: %s: %w", stderr, err)
		}
		return nil
	}
	return nil
}

// --- Validation report ---

func (srv *server) writeValidateReport(backupID string, valid bool, issues []*backup_managerpb.ValidationIssue, repChecks []*backup_managerpb.ReplicationValidation) {
	capsuleDir := srv.CapsuleDir(backupID)
	reportsDir := filepath.Join(capsuleDir, "reports")
	_ = os.MkdirAll(reportsDir, 0755)

	report := map[string]interface{}{
		"backup_id":          backupID,
		"valid":              valid,
		"validated_at_unix":  time.Now().UnixMilli(),
		"issues":             make([]map[string]string, 0),
		"replication_checks": make([]map[string]interface{}, 0),
	}

	for _, iss := range issues {
		report["issues"] = append(report["issues"].([]map[string]string), map[string]string{
			"severity": iss.Severity.String(),
			"code":     iss.Code,
			"message":  iss.Message,
		})
	}
	for _, rc := range repChecks {
		report["replication_checks"] = append(report["replication_checks"].([]map[string]interface{}), map[string]interface{}{
			"destination":   rc.DestinationName,
			"ok":            rc.Ok,
			"missing_files": rc.MissingFiles,
			"error":         rc.ErrorMessage,
		})
	}

	data, _ := json.MarshalIndent(report, "", "  ")
	_ = os.WriteFile(filepath.Join(reportsDir, "validate.json"), data, 0644)
}

// --- Replication verification ---

func (srv *server) validateReplications(backupID string) []*backup_managerpb.ReplicationValidation {
	art, err := srv.store.GetArtifact(backupID)
	if err != nil {
		return nil
	}

	var results []*backup_managerpb.ReplicationValidation

	// Expected files that must be present in every capsule copy
	requiredFiles := []string{"manifest.json", "manifest.sha256"}

	// Collect payload files from provider results
	for _, pr := range art.ProviderResults {
		for _, pf := range pr.PayloadFiles {
			requiredFiles = append(requiredFiles, pf)
		}
	}

	for _, rep := range art.Replications {
		if rep.State != backup_managerpb.BackupJobState_BACKUP_JOB_SUCCEEDED {
			continue
		}

		rv := &backup_managerpb.ReplicationValidation{
			DestinationName: rep.DestinationName,
		}

		switch rep.DestinationType {
		case backup_managerpb.BackupDestinationType_BACKUP_DESTINATION_LOCAL,
			backup_managerpb.BackupDestinationType_BACKUP_DESTINATION_NFS:
			rv.Ok, rv.MissingFiles = verifyLocalCapsule(
				filepath.Join(rep.DestinationPath, "artifacts", backupID), requiredFiles)

		case backup_managerpb.BackupDestinationType_BACKUP_DESTINATION_MINIO,
			backup_managerpb.BackupDestinationType_BACKUP_DESTINATION_S3,
			backup_managerpb.BackupDestinationType_BACKUP_DESTINATION_RCLONE:
			remotePath := fmt.Sprintf("%s/artifacts/%s", rep.DestinationPath, backupID)
			if rep.DestinationType == backup_managerpb.BackupDestinationType_BACKUP_DESTINATION_MINIO ||
				rep.DestinationType == backup_managerpb.BackupDestinationType_BACKUP_DESTINATION_S3 {
				remotePath = fmt.Sprintf(":s3:%s/artifacts/%s", rep.DestinationPath, backupID)
			}
			extraArgs := srv.rcloneArgsForDest(rep.DestinationName, rep.DestinationType)
			rv.Ok, rv.MissingFiles, rv.ErrorMessage = verifyRemoteCapsule(remotePath, requiredFiles, extraArgs)
		}

		if !rv.Ok && rv.ErrorMessage == "" {
			rv.ErrorMessage = fmt.Sprintf("missing files: %v", rv.MissingFiles)
		}

		results = append(results, rv)
	}

	return results
}

func verifyLocalCapsule(capsulePath string, requiredFiles []string) (bool, []string) {
	var missing []string
	for _, f := range requiredFiles {
		p := filepath.Join(capsulePath, f)
		if !fileOrDirExists(p) {
			missing = append(missing, f)
		}
	}
	return len(missing) == 0, missing
}

func verifyRemoteCapsule(remotePath string, requiredFiles []string, extraArgs []string) (bool, []string, string) {
	// Use rclone lsf to list remote files
	args := append([]string{"lsf", remotePath, "--recursive"}, extraArgs...)
	stdout, stderr, err := runCmd("rclone", args...)
	if err != nil {
		return false, nil, fmt.Sprintf("rclone lsf failed: %s: %v", strings.TrimSpace(stderr), err)
	}

	remoteFiles := make(map[string]bool)
	for _, line := range strings.Split(stdout, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			remoteFiles[line] = true
		}
	}

	var missing []string
	for _, f := range requiredFiles {
		if !remoteFiles[f] {
			missing = append(missing, f)
		}
	}
	return len(missing) == 0, missing, ""
}

// --- Helpers ---

// resticSnapshotExists parses restic snapshots JSON output and checks for an exact ID match.
// Restic returns an array of objects with "id" and "short_id" fields.
func resticSnapshotExists(jsonData []byte, snapID string) bool {
	var snapshots []struct {
		ID      string `json:"id"`
		ShortID string `json:"short_id"`
	}
	if err := json.Unmarshal(jsonData, &snapshots); err != nil {
		// Fallback: if JSON parsing fails, do substring match as last resort
		return strings.Contains(string(jsonData), snapID)
	}
	for _, s := range snapshots {
		if s.ID == snapID || s.ShortID == snapID {
			return true
		}
		// Also match prefix (restic short IDs are 8-char prefixes)
		if len(snapID) >= 8 && strings.HasPrefix(s.ID, snapID) {
			return true
		}
	}
	return false
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}
