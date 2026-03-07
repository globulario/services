package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/globulario/services/golang/backup_manager/backup_managerpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	recoverySeedDir     = "/var/lib/globular-recovery"
	recoverySeedFile    = "seed.json"
	recoveryCredsDir    = "credentials"
	recoverySeedVersion = "1"
)

// RecoverySeed is the on-disk schema for /var/lib/globular-recovery/seed.json.
type RecoverySeed struct {
	Version     string                `json:"version"`
	ClusterName string                `json:"cluster_name"`
	ClusterID   string                `json:"cluster_id"`
	Domain      string                `json:"domain"`
	CreatedAt   string                `json:"created_at"`
	UpdatedAt   string                `json:"updated_at"`
	Destination RecoverySeedDest      `json:"recovery_destination"`
	CredsFile   string                `json:"credentials_file"`
	LastBackup  *RecoverySeedBackup   `json:"last_backup,omitempty"`
}

type RecoverySeedDest struct {
	Name    string            `json:"name"`
	Type    string            `json:"type"`
	Path    string            `json:"path"`
	Options map[string]string `json:"options"` // endpoint, region — never credentials
}

type RecoverySeedBackup struct {
	BackupID     string `json:"backup_id"`
	CreatedAtMs  int64  `json:"created_at_ms"`
	PlanName     string `json:"plan_name"`
	QualityState string `json:"quality_state"`
}

type RecoverySeedCreds struct {
	AccessKey string `json:"access_key"`
	SecretKey string `json:"secret_key"`
}

// seedPath returns the full path to seed.json.
func seedPath() string {
	return filepath.Join(recoverySeedDir, recoverySeedFile)
}

// loadRecoverySeed reads and validates the recovery seed from disk.
func loadRecoverySeed() (*RecoverySeed, error) {
	data, err := os.ReadFile(seedPath())
	if err != nil {
		return nil, err
	}

	var seed RecoverySeed
	if err := json.Unmarshal(data, &seed); err != nil {
		return nil, fmt.Errorf("parse recovery seed: %w", err)
	}

	// Validate required fields
	if seed.Version == "" {
		return nil, fmt.Errorf("recovery seed missing version")
	}
	if seed.Version != recoverySeedVersion {
		return nil, fmt.Errorf("unsupported recovery seed version: %s", seed.Version)
	}
	if seed.Destination.Name == "" || seed.Destination.Type == "" {
		return nil, fmt.Errorf("recovery seed missing destination name or type")
	}
	if seed.Destination.Type == "local" {
		return nil, fmt.Errorf("recovery seed destination must not be local")
	}

	return &seed, nil
}

// seedCredentialsAvailable checks if the referenced credentials file exists.
func seedCredentialsAvailable(seed *RecoverySeed) bool {
	if seed.CredsFile == "" {
		return false
	}
	_, err := os.Stat(seed.CredsFile)
	return err == nil
}

// loadSeedCredentials reads credentials from the referenced file.
func loadSeedCredentials(seed *RecoverySeed) (*RecoverySeedCreds, error) {
	if seed.CredsFile == "" {
		return nil, fmt.Errorf("no credentials file referenced")
	}
	data, err := os.ReadFile(seed.CredsFile)
	if err != nil {
		return nil, err
	}
	var creds RecoverySeedCreds
	if err := json.Unmarshal(data, &creds); err != nil {
		return nil, err
	}
	return &creds, nil
}

// resolveRecoveryDestination returns the destination to use for the recovery seed.
// Returns nil if no suitable destination exists. Local destinations are never
// valid as authoritative recovery sources.
func (srv *server) resolveRecoveryDestination() *DestinationConfig {
	// Prefer explicitly marked authoritative destination (must be non-local)
	for i := range srv.Destinations {
		if srv.Destinations[i].AuthoritativeForRecovery && srv.Destinations[i].Type != "local" {
			return &srv.Destinations[i]
		}
	}

	// Auto-select only if exactly one non-local destination exists
	var candidates []*DestinationConfig
	for i := range srv.Destinations {
		if srv.Destinations[i].Type != "local" {
			candidates = append(candidates, &srv.Destinations[i])
		}
	}
	if len(candidates) == 1 {
		return candidates[0]
	}

	return nil
}

// writeRecoverySeed writes/updates the recovery seed to disk.
// Called on config save (destination refs) and after successful backup replication (last_backup).
// Local destinations are never valid as recovery sources; the write is skipped with a warning.
func (srv *server) writeRecoverySeed(dest *DestinationConfig, lastBackup *RecoverySeedBackup) {
	if dest == nil {
		return
	}
	if dest.Type == "local" {
		slog.Warn("refusing to write recovery seed from local destination", "destination", dest.Name)
		return
	}

	// Ensure directory exists with restricted permissions
	if err := os.MkdirAll(recoverySeedDir, 0700); err != nil {
		slog.Warn("cannot create recovery seed directory", "error", err)
		return
	}
	credsDir := filepath.Join(recoverySeedDir, recoveryCredsDir)
	if err := os.MkdirAll(credsDir, 0700); err != nil {
		slog.Warn("cannot create recovery credentials directory", "error", err)
		return
	}

	// Build seed — options without credentials
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
		ClusterName: srv.Name,
		ClusterID:   srv.Id,
		Domain:      srv.Domain,
		UpdatedAt:   time.Now().UTC().Format(time.RFC3339),
		Destination: RecoverySeedDest{
			Name:    dest.Name,
			Type:    dest.Type,
			Path:    dest.Path,
			Options: safeOptions,
		},
		CredsFile:  credsFile,
		LastBackup: lastBackup,
	}

	// Read existing seed to preserve created_at
	existing, err := loadRecoverySeed()
	if err == nil {
		seed.CreatedAt = existing.CreatedAt
		// Preserve last_backup if not being updated
		if seed.LastBackup == nil && existing.LastBackup != nil {
			seed.LastBackup = existing.LastBackup
		}
	} else {
		seed.CreatedAt = seed.UpdatedAt
	}

	// Write seed atomically
	data, _ := json.MarshalIndent(seed, "", "  ")
	tmp := seedPath() + ".tmp"
	if err := os.WriteFile(tmp, data, 0600); err != nil {
		slog.Warn("cannot write recovery seed", "error", err)
		return
	}
	if err := os.Rename(tmp, seedPath()); err != nil {
		slog.Warn("cannot rename recovery seed", "error", err)
		return
	}

	// Write credentials file (separate, strict permissions)
	creds := &RecoverySeedCreds{}
	if ak := dest.Options["access_key"]; ak != "" {
		creds.AccessKey = ak
	}
	if sk := dest.Options["secret_key"]; sk != "" {
		creds.SecretKey = sk
	}
	if creds.AccessKey != "" || creds.SecretKey != "" {
		credsData, _ := json.MarshalIndent(creds, "", "  ")
		credsTmp := credsFile + ".tmp"
		if err := os.WriteFile(credsTmp, credsData, 0600); err != nil {
			slog.Warn("cannot write recovery credentials", "error", err)
			return
		}
		_ = os.Rename(credsTmp, credsFile)
	}

	slog.Info("recovery seed updated", "destination", dest.Name, "has_backup", lastBackup != nil)
}

// updateRecoverySeedAfterBackup updates the seed's last_backup metadata.
// Called after successful replication to the recovery destination.
func (srv *server) updateRecoverySeedAfterBackup(art *backup_managerpb.BackupArtifact) {
	dest := srv.resolveRecoveryDestination()
	if dest == nil {
		return
	}

	// Check if replication to this destination succeeded
	replicated := false
	for _, rep := range art.Replications {
		if rep.DestinationName == dest.Name && rep.State == backup_managerpb.BackupJobState_BACKUP_JOB_SUCCEEDED {
			replicated = true
			break
		}
	}
	if !replicated {
		return
	}

	srv.writeRecoverySeed(dest, &RecoverySeedBackup{
		BackupID:     art.BackupId,
		CreatedAtMs:  art.CreatedUnixMs,
		PlanName:     art.PlanName,
		QualityState: art.QualityState.String(),
	})
}

// updateRecoverySeedOnConfigSave writes destination/credential refs to the seed.
// Called when backup-manager config is saved.
func (srv *server) updateRecoverySeedOnConfigSave() {
	dest := srv.resolveRecoveryDestination()
	if dest == nil {
		return
	}
	srv.writeRecoverySeed(dest, nil)
}

// --- RPC handlers ---

// GetRecoveryStatus returns the current recovery seed state (read-only).
func (srv *server) GetRecoveryStatus(_ context.Context, _ *backup_managerpb.GetRecoveryStatusRequest) (*backup_managerpb.GetRecoveryStatusResponse, error) {
	resp := &backup_managerpb.GetRecoveryStatusResponse{}

	seed, err := loadRecoverySeed()
	if err != nil {
		resp.SeedPresent = false
		resp.Message = fmt.Sprintf("no recovery seed: %v", err)

		// Still check destination config
		dest := srv.resolveRecoveryDestination()
		resp.DestinationConfigured = dest != nil
		return resp, nil
	}

	resp.SeedPresent = true
	resp.SeedVersion = seed.Version
	resp.ClusterName = seed.ClusterName
	resp.ClusterId = seed.ClusterID
	resp.Domain = seed.Domain
	resp.CredentialsAvailable = seedCredentialsAvailable(seed)
	resp.Destination = &backup_managerpb.RecoverySeedDestination{
		Name:    seed.Destination.Name,
		Type:    seed.Destination.Type,
		Path:    seed.Destination.Path,
		Options: seed.Destination.Options,
	}

	if seed.LastBackup != nil {
		resp.LastBackup = &backup_managerpb.RecoverySeedLastBackup{
			BackupId:      seed.LastBackup.BackupID,
			CreatedUnixMs: seed.LastBackup.CreatedAtMs,
			PlanName:      seed.LastBackup.PlanName,
			QualityState:  seed.LastBackup.QualityState,
		}
	}

	// Check if seed matches current config
	dest := srv.resolveRecoveryDestination()
	resp.DestinationConfigured = dest != nil
	if dest != nil {
		resp.SeedMatchesConfig = dest.Name == seed.Destination.Name &&
			dest.Type == seed.Destination.Type &&
			dest.Path == seed.Destination.Path
	}

	return resp, nil
}

// ApplyRecoverySeed applies the recovery seed to restore backup-manager config.
// This is intended for Day 0 bootstrap after a fresh install.
func (srv *server) ApplyRecoverySeed(_ context.Context, rqst *backup_managerpb.ApplyRecoverySeedRequest) (*backup_managerpb.ApplyRecoverySeedResponse, error) {
	seed, err := loadRecoverySeed()
	if err != nil {
		return nil, status.Errorf(codes.NotFound, "no valid recovery seed: %v", err)
	}

	if !seedCredentialsAvailable(seed) {
		return nil, status.Error(codes.FailedPrecondition, "recovery seed credentials file not found")
	}

	// Safety: don't apply if config already has non-local destinations (unless forced)
	if !rqst.Force {
		for _, d := range srv.Destinations {
			if d.Type != "local" {
				return nil, status.Error(codes.AlreadyExists,
					"config already has non-local destinations; use force=true to override")
			}
		}
	}

	// Load credentials
	creds, err := loadSeedCredentials(seed)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "load recovery credentials: %v", err)
	}

	// Build destination config from seed
	opts := make(map[string]string)
	for k, v := range seed.Destination.Options {
		opts[k] = v
	}
	if creds.AccessKey != "" {
		opts["access_key"] = creds.AccessKey
	}
	if creds.SecretKey != "" {
		opts["secret_key"] = creds.SecretKey
	}

	newDest := DestinationConfig{
		Name:                     seed.Destination.Name,
		Type:                     seed.Destination.Type,
		Path:                     seed.Destination.Path,
		Options:                  opts,
		AuthoritativeForRecovery: true,
	}

	// Add to destinations (avoid duplicates)
	found := false
	for i := range srv.Destinations {
		if srv.Destinations[i].Name == newDest.Name {
			srv.Destinations[i] = newDest
			found = true
			break
		}
	}
	if !found {
		srv.Destinations = append(srv.Destinations, newDest)
	}

	slog.Info("recovery seed applied",
		"destination", newDest.Name, "type", newDest.Type, "path", newDest.Path)

	return &backup_managerpb.ApplyRecoverySeedResponse{
		Ok:      true,
		Message: fmt.Sprintf("recovery destination '%s' applied from seed", newDest.Name),
		AppliedDestination: &backup_managerpb.RecoverySeedDestination{
			Name:    newDest.Name,
			Type:    newDest.Type,
			Path:    newDest.Path,
			Options: seed.Destination.Options,
		},
	}, nil
}
