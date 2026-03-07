package main

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"time"

	"github.com/globulario/services/golang/backup_manager/backup_managerpb"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)


// RunRetention executes the retention policy, deleting old backups.
// Retention runs are tracked as jobs for auditability.
func (srv *server) RunRetention(ctx context.Context, rqst *backup_managerpb.RunRetentionRequest) (*backup_managerpb.RunRetentionResponse, error) {
	arts, _, err := srv.store.ListArtifacts("", 0, 0, 0, 0) // all artifacts
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list artifacts: %v", err)
	}

	// Create a tracked job
	jobID := Utility.RandomUUID()
	now := time.Now().UnixMilli()
	job := &backup_managerpb.BackupJob{
		JobId:         jobID,
		PlanName:      "retention",
		State:         backup_managerpb.BackupJobState_BACKUP_JOB_RUNNING,
		JobType:       backup_managerpb.BackupJobType_BACKUP_JOB_TYPE_RETENTION,
		CreatedUnixMs: now,
		StartedUnixMs: now,
	}

	if rqst.DryRun {
		job.Message = "retention dry-run"
	} else {
		job.Message = "applying retention policy"
	}
	_ = srv.store.SaveJob(job)

	if len(arts) == 0 {
		job.State = backup_managerpb.BackupJobState_BACKUP_JOB_SUCCEEDED
		job.FinishedUnixMs = time.Now().UnixMilli()
		job.Message = "no backups to evaluate"
		_ = srv.store.SaveJob(job)
		return &backup_managerpb.RunRetentionResponse{
			DryRun:  rqst.DryRun,
			Message: "no backups to evaluate",
		}, nil
	}

	// Sort by time descending (newest first)
	sort.Slice(arts, func(i, j int) bool {
		return arts[i].CreatedUnixMs > arts[j].CreatedUnixMs
	})

	toDelete, toKeep := srv.evaluateRetention(arts)

	var deletedIDs []string
	var keptIDs []string

	for _, a := range toKeep {
		keptIDs = append(keptIDs, a.BackupId)
	}

	if rqst.DryRun {
		for _, a := range toDelete {
			deletedIDs = append(deletedIDs, a.BackupId)
		}
		job.State = backup_managerpb.BackupJobState_BACKUP_JOB_SUCCEEDED
		job.FinishedUnixMs = time.Now().UnixMilli()
		job.Message = fmt.Sprintf("dry-run: would delete %d backups", len(toDelete))
		_ = srv.store.SaveJob(job)
		return &backup_managerpb.RunRetentionResponse{
			DeletedBackupIds: deletedIDs,
			KeptBackupIds:    keptIDs,
			DryRun:           true,
			Message:          fmt.Sprintf("would delete %d backups", len(toDelete)),
		}, nil
	}

	// Actually delete
	for _, a := range toDelete {
		deleteRqst := &backup_managerpb.DeleteBackupRequest{
			BackupId:                a.BackupId,
			DeleteProviderArtifacts: true,
		}
		if _, err := srv.DeleteBackup(ctx, deleteRqst); err != nil {
			slog.Warn("retention delete failed", "backup_id", a.BackupId, "error", err)
		} else {
			deletedIDs = append(deletedIDs, a.BackupId)
			metricsRetentionDeleted.Inc()
			slog.Info("retention deleted backup", "backup_id", a.BackupId)
		}
	}

	metricsJobsTotal.WithLabelValues("retention").Inc()

	job.State = backup_managerpb.BackupJobState_BACKUP_JOB_SUCCEEDED
	job.FinishedUnixMs = time.Now().UnixMilli()
	job.Message = fmt.Sprintf("deleted %d backups, kept %d", len(deletedIDs), len(keptIDs))
	_ = srv.store.SaveJob(job)

	return &backup_managerpb.RunRetentionResponse{
		DeletedBackupIds: deletedIDs,
		KeptBackupIds:    keptIDs,
		Message:          fmt.Sprintf("deleted %d backups", len(deletedIDs)),
	}, nil
}

// GetRetentionStatus returns the current retention policy and backup stats.
func (srv *server) GetRetentionStatus(ctx context.Context, rqst *backup_managerpb.GetRetentionStatusRequest) (*backup_managerpb.GetRetentionStatusResponse, error) {
	arts, _, err := srv.store.ListArtifacts("", 0, 0, 0, 0)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list artifacts: %v", err)
	}

	resp := &backup_managerpb.GetRetentionStatusResponse{
		Policy: &backup_managerpb.RetentionPolicy{
			KeepLastN:              uint32(srv.RetentionKeepLastN),
			KeepDays:               uint32(srv.RetentionKeepDays),
			MaxTotalBytes:          srv.RetentionMaxTotalBytes,
			MinRestoreTestedToKeep: uint32(srv.MinRestoreTestedToKeep),
		},
		CurrentBackupCount: uint32(len(arts)),
	}

	if len(arts) > 0 {
		// Sort by time
		sort.Slice(arts, func(i, j int) bool {
			return arts[i].CreatedUnixMs < arts[j].CreatedUnixMs
		})
		resp.OldestBackupUnixMs = arts[0].CreatedUnixMs
		resp.NewestBackupUnixMs = arts[len(arts)-1].CreatedUnixMs

		var totalBytes uint64
		for _, a := range arts {
			totalBytes += a.TotalBytes
		}
		resp.CurrentTotalBytes = totalBytes
	}

	return resp, nil
}

// evaluateRetention determines which backups to keep and which to delete.
// Input must be sorted by CreatedUnixMs descending (newest first).
// PROMOTED backups are never deleted. Lower quality states are deleted first.
func (srv *server) evaluateRetention(arts []*backup_managerpb.BackupArtifact) (toDelete, toKeep []*backup_managerpb.BackupArtifact) {
	keepN := srv.RetentionKeepLastN
	keepDays := srv.RetentionKeepDays
	maxBytes := srv.RetentionMaxTotalBytes

	// If no retention policy configured, keep everything
	if keepN <= 0 && keepDays <= 0 && maxBytes == 0 {
		return nil, arts
	}

	cutoff := time.Time{}
	if keepDays > 0 {
		cutoff = time.Now().Add(-time.Duration(keepDays) * 24 * time.Hour)
	}

	var totalBytes uint64
	var candidates []*backup_managerpb.BackupArtifact // potential deletions
	nonPromotedIdx := 0

	for _, a := range arts {
		// Never delete PROMOTED backups
		if a.QualityState == backup_managerpb.QualityState_QUALITY_PROMOTED {
			toKeep = append(toKeep, a)
			totalBytes += a.TotalBytes
			continue
		}

		keep := true

		// Check keep_last_n (counts only non-promoted)
		if keepN > 0 && nonPromotedIdx >= keepN {
			keep = false
		}
		nonPromotedIdx++

		// Check keep_days
		if keep && keepDays > 0 {
			created := time.UnixMilli(a.CreatedUnixMs)
			if created.Before(cutoff) {
				keep = false
			}
		}

		// Check max_total_bytes
		if keep && maxBytes > 0 {
			totalBytes += a.TotalBytes
			if totalBytes > maxBytes {
				keep = false
			}
		}

		if keep {
			toKeep = append(toKeep, a)
		} else {
			candidates = append(candidates, a)
		}
	}

	// Sort candidates: delete lowest quality first
	// UNVERIFIED < VALIDATED < RESTORE_TESTED
	sort.Slice(candidates, func(i, j int) bool {
		return candidates[i].QualityState < candidates[j].QualityState
	})

	// Respect MinRestoreTestedToKeep: if configured, keep some restore-tested backups
	if srv.MinRestoreTestedToKeep > 0 {
		var restoreTestedKept int
		var finalDelete []*backup_managerpb.BackupArtifact
		for _, c := range candidates {
			if c.QualityState >= backup_managerpb.QualityState_QUALITY_RESTORE_TESTED && restoreTestedKept < srv.MinRestoreTestedToKeep {
				toKeep = append(toKeep, c)
				restoreTestedKept++
			} else {
				finalDelete = append(finalDelete, c)
			}
		}
		candidates = finalDelete
	}

	toDelete = candidates
	return toDelete, toKeep
}
