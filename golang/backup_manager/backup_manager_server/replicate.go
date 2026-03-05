package main

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/globulario/services/golang/backup_manager/backup_managerpb"
)

// replicateToDestinations copies the backup capsule (artifacts/<backup_id>/)
// to all configured destinations.
func (srv *server) replicateToDestinations(backupID string, plan *backup_managerpb.BackupPlan) []*backup_managerpb.ReplicationResult {
	dests := srv.resolveDestinations(plan)
	if len(dests) == 0 {
		return nil
	}

	capsuleDir := srv.CapsuleDir(backupID)
	if !fileOrDirExists(capsuleDir) {
		slog.Warn("capsule dir missing, skipping replication", "backup_id", backupID, "path", capsuleDir)
		return nil
	}

	var results []*backup_managerpb.ReplicationResult

	for _, dest := range dests {
		// Skip primary local — capsule is already there
		if dest.Primary && dest.Type == "local" && dest.Path == srv.DataDir {
			results = append(results, &backup_managerpb.ReplicationResult{
				DestinationName: dest.Name,
				DestinationType: destType(dest.Type),
				DestinationPath: dest.Path,
				State:           backup_managerpb.BackupJobState_BACKUP_JOB_SUCCEEDED,
			})
			continue
		}

		result := srv.replicateOne(backupID, capsuleDir, dest)
		results = append(results, result)
	}

	return results
}

// resolveDestinations merges plan-level destinations with server config.
func (srv *server) resolveDestinations(plan *backup_managerpb.BackupPlan) []DestinationConfig {
	if plan != nil && len(plan.Destinations) > 0 {
		var dests []DestinationConfig
		for _, d := range plan.Destinations {
			dests = append(dests, DestinationConfig{
				Name:    d.Name,
				Type:    destTypeStr(d.Type),
				Path:    d.Path,
				Options: d.Options,
				Primary: d.Primary,
			})
		}
		return dests
	}
	return srv.Destinations
}

// replicateOne copies the capsule to a single destination.
func (srv *server) replicateOne(backupID, capsuleDir string, dest DestinationConfig) *backup_managerpb.ReplicationResult {
	start := time.Now().UnixMilli()

	result := &backup_managerpb.ReplicationResult{
		DestinationName: dest.Name,
		DestinationType: destType(dest.Type),
		DestinationPath: dest.Path,
		StartedUnixMs:   start,
	}

	slog.Info("replicating capsule", "backup_id", backupID, "destination", dest.Name, "type", dest.Type, "path", dest.Path)

	var err error
	switch dest.Type {
	case "local", "nfs":
		err = srv.replicateLocal(backupID, capsuleDir, dest)
	case "minio":
		err = srv.replicateMinio(backupID, capsuleDir, dest)
	case "s3":
		err = srv.replicateS3(backupID, capsuleDir, dest)
	case "rclone":
		err = srv.replicateRclone(backupID, capsuleDir, dest)
	default:
		err = fmt.Errorf("unsupported destination type: %s", dest.Type)
	}

	result.FinishedUnixMs = time.Now().UnixMilli()
	if err != nil {
		result.State = backup_managerpb.BackupJobState_BACKUP_JOB_FAILED
		result.ErrorMessage = err.Error()
		slog.Warn("replication failed", "backup_id", backupID, "destination", dest.Name, "error", err)
	} else {
		result.State = backup_managerpb.BackupJobState_BACKUP_JOB_SUCCEEDED
		slog.Info("replication completed", "backup_id", backupID, "destination", dest.Name, "duration_ms", result.FinishedUnixMs-start)
	}

	return result
}

// replicateLocal copies the capsule to another local/NFS path using cp -a.
func (srv *server) replicateLocal(backupID, capsuleDir string, dest DestinationConfig) error {
	dstDir := filepath.Join(dest.Path, "artifacts", backupID)

	if err := os.MkdirAll(filepath.Dir(dstDir), 0755); err != nil {
		return fmt.Errorf("create destination dir: %w", err)
	}

	_, stderr, err := runCmd("cp", "-a", capsuleDir, dstDir)
	if err != nil {
		return fmt.Errorf("cp capsule: %s: %w", strings.TrimSpace(stderr), err)
	}
	return nil
}

// replicateMinio copies the capsule to a MinIO bucket using rclone.
func (srv *server) replicateMinio(backupID, capsuleDir string, dest DestinationConfig) error {
	endpoint := dest.Options["endpoint"]
	accessKey := dest.Options["access_key"]
	secretKey := dest.Options["secret_key"]
	bucket := dest.Path

	if endpoint == "" || bucket == "" {
		return fmt.Errorf("minio destination requires 'endpoint' and a bucket path")
	}

	remotePath := fmt.Sprintf(":s3:%s/artifacts/%s", bucket, backupID)

	args := []string{
		"sync", capsuleDir, remotePath,
		"--s3-provider", "Minio",
		"--s3-endpoint", endpoint,
		"--s3-env-auth=false",
	}

	if accessKey != "" {
		args = append(args, "--s3-access-key-id", accessKey)
	}
	if secretKey != "" {
		args = append(args, "--s3-secret-access-key", secretKey)
	}
	if dest.Options["no_check_bucket"] == "true" {
		args = append(args, "--s3-no-check-bucket")
	}

	cmd := exec.Command("rclone", args...)
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("rclone to minio: %s: %w", strings.TrimSpace(errBuf.String()), err)
	}
	return nil
}

// replicateS3 copies the capsule to an S3 bucket using rclone.
func (srv *server) replicateS3(backupID, capsuleDir string, dest DestinationConfig) error {
	bucket := dest.Path
	region := dest.Options["region"]
	accessKey := dest.Options["access_key"]
	secretKey := dest.Options["secret_key"]

	if bucket == "" {
		return fmt.Errorf("s3 destination requires a bucket/prefix path")
	}

	remotePath := fmt.Sprintf(":s3:%s/artifacts/%s", bucket, backupID)

	args := []string{"sync", capsuleDir, remotePath, "--s3-provider", "AWS"}

	if region != "" {
		args = append(args, "--s3-region", region)
	}
	if accessKey != "" {
		args = append(args, "--s3-access-key-id", accessKey)
	}
	if secretKey != "" {
		args = append(args, "--s3-secret-access-key", secretKey)
	}

	cmd := exec.Command("rclone", args...)
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("rclone to s3: %s: %w", strings.TrimSpace(errBuf.String()), err)
	}
	return nil
}

// replicateRclone copies the capsule to any rclone remote.
func (srv *server) replicateRclone(backupID, capsuleDir string, dest DestinationConfig) error {
	remote := dest.Path
	if remote == "" {
		return fmt.Errorf("rclone destination requires a remote:path")
	}

	dstPath := fmt.Sprintf("%s/artifacts/%s", remote, backupID)

	cmd := exec.Command("rclone", "sync", capsuleDir, dstPath)
	var errBuf bytes.Buffer
	cmd.Stderr = &errBuf
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("rclone sync: %s: %w", strings.TrimSpace(errBuf.String()), err)
	}
	return nil
}

// destType converts a string destination type to its proto enum.
func destType(t string) backup_managerpb.BackupDestinationType {
	switch t {
	case "local":
		return backup_managerpb.BackupDestinationType_BACKUP_DESTINATION_LOCAL
	case "minio":
		return backup_managerpb.BackupDestinationType_BACKUP_DESTINATION_MINIO
	case "nfs":
		return backup_managerpb.BackupDestinationType_BACKUP_DESTINATION_NFS
	case "s3":
		return backup_managerpb.BackupDestinationType_BACKUP_DESTINATION_S3
	case "rclone":
		return backup_managerpb.BackupDestinationType_BACKUP_DESTINATION_RCLONE
	default:
		return backup_managerpb.BackupDestinationType_BACKUP_DESTINATION_TYPE_UNSPECIFIED
	}
}

// destTypeStr converts a proto enum to its string representation.
func destTypeStr(t backup_managerpb.BackupDestinationType) string {
	switch t {
	case backup_managerpb.BackupDestinationType_BACKUP_DESTINATION_LOCAL:
		return "local"
	case backup_managerpb.BackupDestinationType_BACKUP_DESTINATION_MINIO:
		return "minio"
	case backup_managerpb.BackupDestinationType_BACKUP_DESTINATION_NFS:
		return "nfs"
	case backup_managerpb.BackupDestinationType_BACKUP_DESTINATION_S3:
		return "s3"
	case backup_managerpb.BackupDestinationType_BACKUP_DESTINATION_RCLONE:
		return "rclone"
	default:
		return "unknown"
	}
}
