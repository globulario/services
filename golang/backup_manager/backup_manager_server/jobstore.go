// @awareness namespace=globular.platform
// @awareness component=platform_backup
// @awareness file_role=backup_job_state_store
// @awareness implements=globular.platform:intent.backup.provider_results_are_explicit
// @awareness risk=high
package main

import (
	"context"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/globulario/services/golang/backup_manager/backup_managerpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/protobuf/encoding/protojson"
)

const (
	etcdJobPrefix = "/globular/backup/jobs/"
	etcdArtPrefix = "/globular/backup/artifacts/"
	etcdTimeout   = 5 * time.Second
)

// jobStore persists BackupJob and BackupArtifact records in etcd.
// All nodes share the same data, so round-robin routing is transparent.
type jobStore struct {
	dataDir    string // local filesystem for capsule validation only
	etcdNewFn  func() (*clientv3.Client, error)
}

var jsonOpts = protojson.MarshalOptions{Indent: "  "}

func newJobStore(dataDir string, etcdClientFn func() (*clientv3.Client, error)) (*jobStore, error) {
	// Ensure local dirs still exist for capsule validation
	for _, d := range []string{
		filepath.Join(dataDir, "artifacts"),
	} {
		if err := os.MkdirAll(d, 0755); err != nil {
			return nil, fmt.Errorf("create dir %s: %w", d, err)
		}
	}
	return &jobStore{
		dataDir:   dataDir,
		etcdNewFn: etcdClientFn,
	}, nil
}

// etcd returns a short-lived etcd client. Caller must close it.
func (s *jobStore) etcd() (*clientv3.Client, error) {
	return s.etcdNewFn()
}

// --- Jobs ---

func (s *jobStore) SaveJob(job *backup_managerpb.BackupJob) error {
	data, err := jsonOpts.Marshal(job)
	if err != nil {
		return fmt.Errorf("marshal job: %w", err)
	}
	cli, err := s.etcd()
	if err != nil {
		return fmt.Errorf("etcd client: %w", err)
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), etcdTimeout)
	defer cancel()

	_, err = cli.Put(ctx, etcdJobPrefix+job.JobId, string(data))
	return err
}

func (s *jobStore) GetJob(jobID string) (*backup_managerpb.BackupJob, error) {
	cli, err := s.etcd()
	if err != nil {
		return nil, fmt.Errorf("etcd client: %w", err)
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), etcdTimeout)
	defer cancel()

	resp, err := cli.Get(ctx, etcdJobPrefix+jobID)
	if err != nil {
		return nil, err
	}
	if len(resp.Kvs) == 0 {
		return nil, fmt.Errorf("job %s not found", jobID)
	}

	job := &backup_managerpb.BackupJob{}
	if err := protojson.Unmarshal(resp.Kvs[0].Value, job); err != nil {
		return nil, fmt.Errorf("unmarshal job: %w", err)
	}
	return job, nil
}

func (s *jobStore) ListJobs(state backup_managerpb.BackupJobState, planName string, limit, offset uint32) ([]*backup_managerpb.BackupJob, uint32, error) {
	cli, err := s.etcd()
	if err != nil {
		return nil, 0, fmt.Errorf("etcd client: %w", err)
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), etcdTimeout)
	defer cancel()

	resp, err := cli.Get(ctx, etcdJobPrefix, clientv3.WithPrefix())
	if err != nil {
		return nil, 0, err
	}

	var all []*backup_managerpb.BackupJob
	for _, kv := range resp.Kvs {
		job := &backup_managerpb.BackupJob{}
		if err := protojson.Unmarshal(kv.Value, job); err != nil {
			slog.Warn("skipping corrupt job in etcd", "key", string(kv.Key), "error", err)
			continue
		}
		if state != backup_managerpb.BackupJobState_BACKUP_JOB_STATE_UNSPECIFIED && job.State != state {
			continue
		}
		if planName != "" && job.PlanName != planName {
			continue
		}
		all = append(all, job)
	}

	sort.Slice(all, func(i, j int) bool {
		return all[i].CreatedUnixMs > all[j].CreatedUnixMs
	})

	total := uint32(len(all))
	if offset > 0 && int(offset) < len(all) {
		all = all[offset:]
	} else if int(offset) >= len(all) {
		all = nil
	}
	if limit > 0 && int(limit) < len(all) {
		all = all[:limit]
	}
	return all, total, nil
}

func (s *jobStore) DeleteJob(jobID string) error {
	cli, err := s.etcd()
	if err != nil {
		return fmt.Errorf("etcd client: %w", err)
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), etcdTimeout)
	defer cancel()

	_, err = cli.Delete(ctx, etcdJobPrefix+jobID)
	return err
}

// --- Artifacts ---

func (s *jobStore) SaveArtifact(art *backup_managerpb.BackupArtifact) error {
	data, err := jsonOpts.Marshal(art)
	if err != nil {
		return fmt.Errorf("marshal artifact: %w", err)
	}
	cli, err := s.etcd()
	if err != nil {
		return fmt.Errorf("etcd client: %w", err)
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), etcdTimeout)
	defer cancel()

	_, err = cli.Put(ctx, etcdArtPrefix+art.BackupId, string(data))
	if err != nil {
		return err
	}

	// Also write local manifest for capsule validation
	dir := filepath.Join(s.dataDir, "artifacts", art.BackupId)
	if err := os.MkdirAll(dir, 0755); err != nil {
		slog.Warn("could not create local artifact dir", "error", err)
		return nil // etcd write succeeded, local is best-effort
	}
	manifestPath := filepath.Join(dir, "manifest.json")
	if err := os.WriteFile(manifestPath, data, 0644); err != nil {
		slog.Warn("could not write local manifest", "error", err)
		return nil
	}
	checksum := fmt.Sprintf("%x", sha256.Sum256(data))
	_ = os.WriteFile(filepath.Join(dir, "manifest.sha256"), []byte(checksum+"\n"), 0644)
	return nil
}

func (s *jobStore) GetArtifact(backupID string) (*backup_managerpb.BackupArtifact, error) {
	cli, err := s.etcd()
	if err != nil {
		return nil, fmt.Errorf("etcd client: %w", err)
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), etcdTimeout)
	defer cancel()

	resp, err := cli.Get(ctx, etcdArtPrefix+backupID)
	if err != nil {
		return nil, err
	}
	if len(resp.Kvs) == 0 {
		return nil, fmt.Errorf("artifact %s not found", backupID)
	}

	art := &backup_managerpb.BackupArtifact{}
	if err := protojson.Unmarshal(resp.Kvs[0].Value, art); err != nil {
		return nil, fmt.Errorf("unmarshal artifact: %w", err)
	}
	return art, nil
}

func (s *jobStore) ListArtifacts(planName string, mode backup_managerpb.BackupMode, qualityState backup_managerpb.QualityState, limit, offset uint32) ([]*backup_managerpb.BackupArtifact, uint32, error) {
	cli, err := s.etcd()
	if err != nil {
		return nil, 0, fmt.Errorf("etcd client: %w", err)
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), etcdTimeout)
	defer cancel()

	resp, err := cli.Get(ctx, etcdArtPrefix, clientv3.WithPrefix())
	if err != nil {
		return nil, 0, err
	}

	var all []*backup_managerpb.BackupArtifact
	for _, kv := range resp.Kvs {
		art := &backup_managerpb.BackupArtifact{}
		if err := protojson.Unmarshal(kv.Value, art); err != nil {
			slog.Warn("skipping corrupt artifact in etcd", "key", string(kv.Key), "error", err)
			continue
		}
		if planName != "" && art.PlanName != planName {
			continue
		}
		if mode != backup_managerpb.BackupMode_BACKUP_MODE_UNSPECIFIED && art.Mode != mode {
			continue
		}
		if qualityState != backup_managerpb.QualityState_QUALITY_STATE_UNSPECIFIED && art.QualityState != qualityState {
			continue
		}
		all = append(all, art)
	}

	sort.Slice(all, func(i, j int) bool {
		return all[i].CreatedUnixMs > all[j].CreatedUnixMs
	})

	total := uint32(len(all))
	if offset > 0 && int(offset) < len(all) {
		all = all[offset:]
	} else if int(offset) >= len(all) {
		all = nil
	}
	if limit > 0 && int(limit) < len(all) {
		all = all[:limit]
	}
	return all, total, nil
}

func (s *jobStore) DeleteArtifact(backupID string) error {
	cli, err := s.etcd()
	if err != nil {
		return fmt.Errorf("etcd client: %w", err)
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), etcdTimeout)
	defer cancel()

	_, err = cli.Delete(ctx, etcdArtPrefix+backupID)
	if err != nil {
		return err
	}

	// Also remove local capsule data
	_ = os.RemoveAll(filepath.Join(s.dataDir, "artifacts", backupID))
	return nil
}

// ValidateArtifact checks manifest integrity using local capsule files.
func (s *jobStore) ValidateArtifact(backupID string, deep bool) (bool, []*backup_managerpb.ValidationIssue) {
	var issues []*backup_managerpb.ValidationIssue
	artDir := filepath.Join(s.dataDir, "artifacts", backupID)
	manifestPath := filepath.Join(artDir, "manifest.json")

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		// Try fetching from etcd and writing locally first
		art, etcdErr := s.GetArtifact(backupID)
		if etcdErr != nil {
			issues = append(issues, &backup_managerpb.ValidationIssue{
				Severity: backup_managerpb.BackupSeverity_BACKUP_SEVERITY_ERROR,
				Code:     "MANIFEST_MISSING",
				Message:  fmt.Sprintf("cannot read manifest locally or from etcd: %v / %v", err, etcdErr),
			})
			return false, issues
		}
		// Write it locally for validation
		_ = os.MkdirAll(artDir, 0755)
		data, _ = jsonOpts.Marshal(art)
		_ = os.WriteFile(manifestPath, data, 0644)
	}

	art := &backup_managerpb.BackupArtifact{}
	if err := protojson.Unmarshal(data, art); err != nil {
		issues = append(issues, &backup_managerpb.ValidationIssue{
			Severity: backup_managerpb.BackupSeverity_BACKUP_SEVERITY_ERROR,
			Code:     "MANIFEST_CORRUPT",
			Message:  fmt.Sprintf("cannot parse manifest: %v", err),
		})
		return false, issues
	}

	if deep {
		expected, err := os.ReadFile(filepath.Join(artDir, "manifest.sha256"))
		if err != nil {
			issues = append(issues, &backup_managerpb.ValidationIssue{
				Severity: backup_managerpb.BackupSeverity_BACKUP_SEVERITY_ERROR,
				Code:     "CHECKSUM_MISSING",
				Message:  "manifest.sha256 file not found",
			})
		} else {
			actual := fmt.Sprintf("%x", sha256.Sum256(data))
			want := strings.TrimSpace(string(expected))
			if actual != want {
				issues = append(issues, &backup_managerpb.ValidationIssue{
					Severity: backup_managerpb.BackupSeverity_BACKUP_SEVERITY_ERROR,
					Code:     "CHECKSUM_MISMATCH",
					Message:  fmt.Sprintf("expected %s, got %s", want, actual),
				})
			}
		}

		providerDir := filepath.Join(artDir, "provider")
		payloadDir := filepath.Join(artDir, "payload")
		if _, err := os.Stat(providerDir); os.IsNotExist(err) {
			issues = append(issues, &backup_managerpb.ValidationIssue{
				Severity: backup_managerpb.BackupSeverity_BACKUP_SEVERITY_WARN,
				Code:     "CAPSULE_INCOMPLETE",
				Message:  "provider/ directory missing from capsule",
			})
		}
		if _, err := os.Stat(payloadDir); os.IsNotExist(err) {
			issues = append(issues, &backup_managerpb.ValidationIssue{
				Severity: backup_managerpb.BackupSeverity_BACKUP_SEVERITY_WARN,
				Code:     "CAPSULE_INCOMPLETE",
				Message:  "payload/ directory missing from capsule",
			})
		}
	}

	if len(art.ProviderResults) == 0 {
		issues = append(issues, &backup_managerpb.ValidationIssue{
			Severity: backup_managerpb.BackupSeverity_BACKUP_SEVERITY_WARN,
			Code:     "NO_PROVIDERS",
			Message:  "no provider results in manifest",
		})
	}

	valid := true
	for _, iss := range issues {
		if iss.Severity == backup_managerpb.BackupSeverity_BACKUP_SEVERITY_ERROR {
			valid = false
			break
		}
	}
	return valid, issues
}
