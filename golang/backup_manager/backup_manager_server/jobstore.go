package main

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/globulario/services/golang/backup_manager/backup_managerpb"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

// jobStore persists BackupJob and BackupArtifact records as JSON files.
type jobStore struct {
	mu      sync.RWMutex
	dataDir string
	jobsDir string
	artsDir string
	// deleted tracks job IDs that have been deleted during this process lifetime.
	// SaveJob refuses to write jobs in this set, preventing zombie resurrection.
	deleted map[string]struct{}
}

func newJobStore(dataDir string) (*jobStore, error) {
	s := &jobStore{
		dataDir: dataDir,
		jobsDir: filepath.Join(dataDir, "jobs"),
		artsDir: filepath.Join(dataDir, "artifacts"),
		deleted: make(map[string]struct{}),
	}
	for _, d := range []string{s.jobsDir, s.artsDir} {
		if err := os.MkdirAll(d, 0755); err != nil {
			return nil, fmt.Errorf("create dir %s: %w", d, err)
		}
	}
	// Clean up any leftover .tmp files from incomplete atomic writes.
	// These can cause zombie jobs if they later get renamed to .json.
	if entries, err := os.ReadDir(s.jobsDir); err == nil {
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".json.tmp") {
				_ = os.Remove(filepath.Join(s.jobsDir, e.Name()))
			}
		}
	}
	return s, nil
}

var jsonOpts = protojson.MarshalOptions{Indent: "  "}

// writeMsg atomically writes a protobuf message as JSON.
// Writes to a .tmp file first, then renames (atomic on same filesystem).
func writeMsg(path string, msg proto.Message) error {
	data, err := jsonOpts.Marshal(msg)
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func readMsg(path string, msg proto.Message) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return protojson.Unmarshal(data, msg)
}

// --- Jobs ---

func (s *jobStore) SaveJob(job *backup_managerpb.BackupJob) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.deleted[job.JobId]; ok {
		return nil // job was deleted, refuse to recreate it
	}
	return writeMsg(filepath.Join(s.jobsDir, job.JobId+".json"), job)
}

func (s *jobStore) GetJob(jobID string) (*backup_managerpb.BackupJob, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job := &backup_managerpb.BackupJob{}
	if err := readMsg(filepath.Join(s.jobsDir, jobID+".json"), job); err != nil {
		return nil, err
	}
	return job, nil
}

func (s *jobStore) ListJobs(state backup_managerpb.BackupJobState, planName string, limit, offset uint32) ([]*backup_managerpb.BackupJob, uint32, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.jobsDir)
	if err != nil {
		return nil, 0, err
	}

	var all []*backup_managerpb.BackupJob
	for _, e := range entries {
		if e.IsDir() || filepath.Ext(e.Name()) != ".json" {
			continue
		}
		job := &backup_managerpb.BackupJob{}
		if err := readMsg(filepath.Join(s.jobsDir, e.Name()), job); err != nil {
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
	s.mu.Lock()
	defer s.mu.Unlock()
	path := filepath.Join(s.jobsDir, jobID+".json")
	// Remove the main file
	err := os.Remove(path)
	// Also remove any leftover .tmp file from writeMsg
	_ = os.Remove(path + ".tmp")
	// Record deletion so SaveJob won't resurrect this job
	s.deleted[jobID] = struct{}{}
	return err
}

// --- Artifacts ---

func (s *jobStore) SaveArtifact(art *backup_managerpb.BackupArtifact) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	dir := filepath.Join(s.artsDir, art.BackupId)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	manifestPath := filepath.Join(dir, "manifest.json")
	if err := writeMsg(manifestPath, art); err != nil {
		return err
	}
	// Compute checksum on the final written bytes and store separately.
	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return err
	}
	checksum := fmt.Sprintf("%x", sha256.Sum256(data))
	return os.WriteFile(filepath.Join(dir, "manifest.sha256"), []byte(checksum+"\n"), 0644)
}

func (s *jobStore) GetArtifact(backupID string) (*backup_managerpb.BackupArtifact, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	art := &backup_managerpb.BackupArtifact{}
	if err := readMsg(filepath.Join(s.artsDir, backupID, "manifest.json"), art); err != nil {
		return nil, err
	}
	return art, nil
}

func (s *jobStore) ListArtifacts(planName string, mode backup_managerpb.BackupMode, qualityState backup_managerpb.QualityState, limit, offset uint32) ([]*backup_managerpb.BackupArtifact, uint32, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entries, err := os.ReadDir(s.artsDir)
	if err != nil {
		return nil, 0, err
	}

	var all []*backup_managerpb.BackupArtifact
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		art := &backup_managerpb.BackupArtifact{}
		if err := readMsg(filepath.Join(s.artsDir, e.Name(), "manifest.json"), art); err != nil {
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
	s.mu.Lock()
	defer s.mu.Unlock()
	return os.RemoveAll(filepath.Join(s.artsDir, backupID))
}

// ValidateArtifact checks manifest integrity.
// When deep=true, it verifies the manifest.json checksum against manifest.sha256.
func (s *jobStore) ValidateArtifact(backupID string, deep bool) (bool, []*backup_managerpb.ValidationIssue) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var issues []*backup_managerpb.ValidationIssue
	artDir := filepath.Join(s.artsDir, backupID)
	manifestPath := filepath.Join(artDir, "manifest.json")

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		issues = append(issues, &backup_managerpb.ValidationIssue{
			Severity: backup_managerpb.BackupSeverity_BACKUP_SEVERITY_ERROR,
			Code:     "MANIFEST_MISSING",
			Message:  fmt.Sprintf("cannot read manifest: %v", err),
		})
		return false, issues
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

		// Verify capsule directory structure
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
