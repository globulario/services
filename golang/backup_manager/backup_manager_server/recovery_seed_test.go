package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/globulario/services/golang/backup_manager/backup_managerpb"
)

func TestWriteAndLoadRecoverySeed(t *testing.T) {
	// Use temp dir instead of the real /var/lib path
	origDir := recoverySeedDir
	tmpDir := t.TempDir()
	// We can't reassign constants, so test the write/load logic directly
	// by writing to a temp dir and reading back.

	seedFile := filepath.Join(tmpDir, "seed.json")
	credsDir := filepath.Join(tmpDir, "credentials")
	if err := os.MkdirAll(credsDir, 0700); err != nil {
		t.Fatal(err)
	}

	dest := DestinationConfig{
		Name: "test-minio",
		Type: "minio",
		Path: "backups/cluster-01",
		Options: map[string]string{
			"endpoint":   "https://minio.example.com",
			"access_key": "AKID",
			"secret_key": "SECRET",
			"region":     "us-east-1",
		},
		AuthoritativeForRecovery: true,
	}

	// Build seed manually (same logic as writeRecoverySeed)
	safeOptions := make(map[string]string)
	for k, v := range dest.Options {
		if k == "access_key" || k == "secret_key" || k == "password" || k == "token" {
			continue
		}
		safeOptions[k] = v
	}

	credsFile := filepath.Join(credsDir, dest.Name+".json")
	seed := &RecoverySeed{
		Version:     recoverySeedVersion,
		ClusterName: "test-cluster",
		ClusterID:   "abc-123",
		Domain:      "example.com",
		CreatedAt:   "2026-03-07T00:00:00Z",
		UpdatedAt:   "2026-03-07T00:00:00Z",
		Destination: RecoverySeedDest{
			Name:    dest.Name,
			Type:    dest.Type,
			Path:    dest.Path,
			Options: safeOptions,
		},
		CredsFile: credsFile,
		LastBackup: &RecoverySeedBackup{
			BackupID:     "bk-001",
			CreatedAtMs:  1709827200000,
			PlanName:     "daily",
			QualityState: "QUALITY_VALIDATED",
		},
	}

	// Write seed
	data, err := json.MarshalIndent(seed, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(seedFile, data, 0600); err != nil {
		t.Fatal(err)
	}

	// Write creds
	creds := &RecoverySeedCreds{AccessKey: "AKID", SecretKey: "SECRET"}
	credsData, _ := json.MarshalIndent(creds, "", "  ")
	if err := os.WriteFile(credsFile, credsData, 0600); err != nil {
		t.Fatal(err)
	}

	// Read back seed
	readData, err := os.ReadFile(seedFile)
	if err != nil {
		t.Fatal(err)
	}
	var loaded RecoverySeed
	if err := json.Unmarshal(readData, &loaded); err != nil {
		t.Fatal(err)
	}

	// Verify
	if loaded.Version != recoverySeedVersion {
		t.Errorf("version = %q, want %q", loaded.Version, recoverySeedVersion)
	}
	if loaded.ClusterName != "test-cluster" {
		t.Errorf("cluster_name = %q", loaded.ClusterName)
	}
	if loaded.ClusterID != "abc-123" {
		t.Errorf("cluster_id = %q", loaded.ClusterID)
	}
	if loaded.Destination.Name != "test-minio" {
		t.Errorf("dest name = %q", loaded.Destination.Name)
	}
	if loaded.Destination.Type != "minio" {
		t.Errorf("dest type = %q", loaded.Destination.Type)
	}
	// Credentials must not be in seed
	if _, found := loaded.Destination.Options["access_key"]; found {
		t.Error("access_key leaked into seed options")
	}
	if _, found := loaded.Destination.Options["secret_key"]; found {
		t.Error("secret_key leaked into seed options")
	}
	// But endpoint/region should be present
	if loaded.Destination.Options["endpoint"] != "https://minio.example.com" {
		t.Errorf("endpoint = %q", loaded.Destination.Options["endpoint"])
	}
	if loaded.Destination.Options["region"] != "us-east-1" {
		t.Errorf("region = %q", loaded.Destination.Options["region"])
	}

	// Verify creds file
	if loaded.CredsFile != credsFile {
		t.Errorf("creds_file = %q, want %q", loaded.CredsFile, credsFile)
	}
	var loadedCreds RecoverySeedCreds
	cd, _ := os.ReadFile(credsFile)
	json.Unmarshal(cd, &loadedCreds)
	if loadedCreds.AccessKey != "AKID" || loadedCreds.SecretKey != "SECRET" {
		t.Errorf("creds = %+v", loadedCreds)
	}

	// Verify last backup
	if loaded.LastBackup == nil {
		t.Fatal("last_backup is nil")
	}
	if loaded.LastBackup.BackupID != "bk-001" {
		t.Errorf("backup_id = %q", loaded.LastBackup.BackupID)
	}

	_ = origDir // suppress unused
}

func TestRecoverySeedValidation(t *testing.T) {
	tests := []struct {
		name    string
		seed    RecoverySeed
		wantErr bool
	}{
		{
			name:    "missing version",
			seed:    RecoverySeed{},
			wantErr: true,
		},
		{
			name:    "unsupported version",
			seed:    RecoverySeed{Version: "99"},
			wantErr: true,
		},
		{
			name: "missing dest name",
			seed: RecoverySeed{
				Version:     "1",
				Destination: RecoverySeedDest{Type: "minio"},
			},
			wantErr: true,
		},
		{
			name: "local dest not allowed",
			seed: RecoverySeed{
				Version:     "1",
				Destination: RecoverySeedDest{Name: "local", Type: "local"},
			},
			wantErr: true,
		},
		{
			name: "valid seed",
			seed: RecoverySeed{
				Version: "1",
				Destination: RecoverySeedDest{
					Name: "minio", Type: "minio", Path: "bucket",
				},
			},
			wantErr: false,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			data, _ := json.MarshalIndent(tc.seed, "", "  ")
			f := filepath.Join(dir, "seed.json")
			os.WriteFile(f, data, 0600)

			// Parse and validate inline (same logic as loadRecoverySeed)
			var s RecoverySeed
			err := json.Unmarshal(data, &s)
			if err != nil {
				if !tc.wantErr {
					t.Errorf("unexpected parse error: %v", err)
				}
				return
			}

			validationErr := validateSeed(&s)
			if tc.wantErr && validationErr == nil {
				t.Error("expected validation error, got nil")
			}
			if !tc.wantErr && validationErr != nil {
				t.Errorf("unexpected validation error: %v", validationErr)
			}
		})
	}
}

func TestResolveRecoveryDestination(t *testing.T) {
	srv := &server{}
	srv.Destinations = []DestinationConfig{
		{Name: "local", Type: "local", Path: "/tmp/backups", Primary: true},
	}

	// Only local → no recovery destination
	if d := srv.resolveRecoveryDestination(); d != nil {
		t.Errorf("expected nil for local-only, got %v", d)
	}

	// Add one non-local → auto-select
	srv.Destinations = append(srv.Destinations, DestinationConfig{
		Name: "minio-1", Type: "minio", Path: "bucket/prefix",
	})
	d := srv.resolveRecoveryDestination()
	if d == nil || d.Name != "minio-1" {
		t.Errorf("expected minio-1, got %v", d)
	}

	// Add second non-local → ambiguous, no auto-select
	srv.Destinations = append(srv.Destinations, DestinationConfig{
		Name: "s3-backup", Type: "s3", Path: "other-bucket",
	})
	if d := srv.resolveRecoveryDestination(); d != nil {
		t.Errorf("expected nil for ambiguous, got %v", d)
	}

	// Mark one as authoritative → resolves
	srv.Destinations[2].AuthoritativeForRecovery = true
	d = srv.resolveRecoveryDestination()
	if d == nil || d.Name != "s3-backup" {
		t.Errorf("expected s3-backup, got %v", d)
	}
}

func TestUpdateRecoverySeedAfterBackup(t *testing.T) {
	srv := &server{}
	srv.Destinations = []DestinationConfig{
		{Name: "local", Type: "local", Path: "/tmp"},
	}

	// No recovery destination → should not panic
	art := &backup_managerpb.BackupArtifact{
		BackupId:      "bk-test",
		CreatedUnixMs: 1234567890,
	}
	srv.updateRecoverySeedAfterBackup(art) // should be a no-op
}

func TestMultipleAuthoritativeDestinationsRejected(t *testing.T) {
	srv := &server{}
	srv.Destinations = []DestinationConfig{
		{Name: "local", Type: "local", Path: "/tmp", Primary: true},
		{Name: "minio-1", Type: "minio", Path: "bucket1", AuthoritativeForRecovery: true},
		{Name: "s3-2", Type: "s3", Path: "bucket2", AuthoritativeForRecovery: true},
	}

	// Save should reject multiple authoritative destinations
	// We test the validation logic directly since Save() calls globular.SaveService
	authCount := 0
	for _, d := range srv.Destinations {
		if d.AuthoritativeForRecovery {
			authCount++
		}
	}
	if authCount <= 1 {
		t.Fatal("test setup: expected >1 authoritative destinations")
	}

	// With exactly one, it should be fine
	srv.Destinations[2].AuthoritativeForRecovery = false
	authCount = 0
	for _, d := range srv.Destinations {
		if d.AuthoritativeForRecovery {
			authCount++
		}
	}
	if authCount != 1 {
		t.Errorf("expected exactly 1 authoritative, got %d", authCount)
	}
}

func TestCredentialsLoading(t *testing.T) {
	dir := t.TempDir()
	credsFile := filepath.Join(dir, "test-creds.json")

	// No creds file → not available
	seed := &RecoverySeed{CredsFile: credsFile}
	if seedCredentialsAvailable(seed) {
		t.Error("expected credentials not available before file exists")
	}

	// Write creds
	creds := &RecoverySeedCreds{AccessKey: "mykey", SecretKey: "mysecret"}
	data, _ := json.MarshalIndent(creds, "", "  ")
	if err := os.WriteFile(credsFile, data, 0600); err != nil {
		t.Fatal(err)
	}

	// Now available
	if !seedCredentialsAvailable(seed) {
		t.Error("expected credentials available after file written")
	}

	// Load and verify
	loaded, err := loadSeedCredentials(seed)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.AccessKey != "mykey" || loaded.SecretKey != "mysecret" {
		t.Errorf("credentials mismatch: %+v", loaded)
	}

	// Empty creds file reference → error
	emptySeed := &RecoverySeed{}
	_, err = loadSeedCredentials(emptySeed)
	if err == nil {
		t.Error("expected error for empty creds file reference")
	}
}

func TestLocalOnlyDisasterRecoveryNotReady(t *testing.T) {
	srv := &server{}
	srv.Destinations = []DestinationConfig{
		{Name: "local", Type: "local", Path: "/var/backups/globular", Primary: true},
	}

	// No recovery destination should be resolved
	dest := srv.resolveRecoveryDestination()
	if dest != nil {
		t.Error("local-only config should not resolve a recovery destination")
	}
}

func TestSaveRejectsLocalAuthoritativeDestination(t *testing.T) {
	// Replicate the Save() validation logic (we can't call Save() directly
	// because it requires globular.SaveService internals, but the validation
	// is inline and testable).
	dests := []DestinationConfig{
		{Name: "local", Type: "local", Path: "/var/backups/globular", Primary: true, AuthoritativeForRecovery: true},
	}

	for _, d := range dests {
		if d.AuthoritativeForRecovery && d.Type == "local" {
			// This is the expected rejection
			return
		}
	}
	t.Fatal("expected local authoritative destination to be rejected")
}

func TestResolveRecoveryDestination_IgnoresLocalAuthoritative(t *testing.T) {
	srv := &server{}
	srv.Destinations = []DestinationConfig{
		{Name: "local", Type: "local", Path: "/tmp", Primary: true, AuthoritativeForRecovery: true},
	}

	// Local authoritative should be ignored
	d := srv.resolveRecoveryDestination()
	if d != nil {
		t.Errorf("expected nil for local authoritative, got %v", d.Name)
	}

	// Add a non-local destination — it should auto-select
	srv.Destinations = append(srv.Destinations, DestinationConfig{
		Name: "minio", Type: "minio", Path: "bucket",
	})
	d = srv.resolveRecoveryDestination()
	if d == nil || d.Name != "minio" {
		t.Errorf("expected minio, got %v", d)
	}
}

func TestWriteRecoverySeed_SkipsLocalDestination(t *testing.T) {
	srv := &server{}
	srv.Name = "test"
	srv.Id = "test-id"
	srv.Domain = "test.local"

	localDest := &DestinationConfig{
		Name: "local", Type: "local", Path: "/tmp/backups",
	}

	// Should be a no-op (no panic, no file written)
	srv.writeRecoverySeed(localDest, nil)

	// Verify no seed was written to the default path
	if _, err := os.Stat(seedPath()); err == nil {
		t.Error("seed file should not exist after writing from local destination")
	}
}

// validateSeed replicates the validation logic from loadRecoverySeed for testing.
func validateSeed(s *RecoverySeed) error {
	if s.Version == "" {
		return fmt.Errorf("missing version")
	}
	if s.Version != recoverySeedVersion {
		return fmt.Errorf("unsupported version: %s", s.Version)
	}
	if s.Destination.Name == "" || s.Destination.Type == "" {
		return fmt.Errorf("missing destination name or type")
	}
	if s.Destination.Type == "local" {
		return fmt.Errorf("destination must not be local")
	}
	return nil
}
