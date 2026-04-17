package main

// release_ledger.go — Per-package release ledger.
//
// The release ledger is the persistent record of all PUBLISHED releases for a
// package. It provides:
//
//   - O(1) latest-release lookup (no directory scanning)
//   - Monotonic version enforcement (new RELEASED version must be > latest)
//   - Deterministic version → build_id resolution
//
// Storage: ScyllaDB table `repository.release_ledger` (distributed, consistent).
// Fallback: MinIO JSON file at `ledger/{publisher}%{name}.json` when ScyllaDB
// is unavailable.
//
// The ledger is written on every successful promote-to-PUBLISHED transition
// and read by the release resolver for latest-version queries.

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
	"github.com/globulario/services/golang/versionutil"
)

// Ensure repopb is used (for migration).
var _ = repopb.PublishState_PUBLISHED

// releaseLedgerEntry represents a single published release in the ledger.
type releaseLedgerEntry struct {
	Version    string `json:"version"`
	BuildID    string `json:"build_id"`
	Digest     string `json:"digest"`
	Platform   string `json:"platform"`
	SizeBytes  int64  `json:"size_bytes"`
	ReleasedAt string `json:"released_at"`
}

// releaseLedger is the per-package release history.
type releaseLedger struct {
	Publisher     string                `json:"publisher"`
	Name          string                `json:"name"`
	LatestVersion string                `json:"latest_version"`
	LatestBuildID string                `json:"latest_build_id"`
	Releases      []*releaseLedgerEntry `json:"releases"`
}

// ledgerStorageKey returns the MinIO key for a package's ledger.
func ledgerStorageKey(publisher, name string) string {
	return fmt.Sprintf("ledger/%s%%%s.json", publisher, name)
}

// ── Ledger read/write ───────────────────────────────────────────────────

// readLedger loads the release ledger for a package. Returns nil if no ledger
// exists (package has never been PUBLISHED).
func (srv *server) readLedger(ctx context.Context, publisher, name string) *releaseLedger {
	// Try ScyllaDB first.
	if srv.scylla != nil {
		if ledger := srv.readLedgerFromScylla(ctx, publisher, name); ledger != nil {
			return ledger
		}
	}

	// Fallback: MinIO.
	key := ledgerStorageKey(publisher, name)
	data, err := srv.Storage().ReadFile(ctx, key)
	if err != nil {
		return nil
	}
	var ledger releaseLedger
	if err := json.Unmarshal(data, &ledger); err != nil {
		slog.Warn("ledger: corrupt JSON", "publisher", publisher, "name", name, "err", err)
		return nil
	}
	return &ledger
}

// writeLedger persists the release ledger. Writes to both ScyllaDB and MinIO.
func (srv *server) writeLedger(ctx context.Context, ledger *releaseLedger) error {
	data, err := json.MarshalIndent(ledger, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal ledger: %w", err)
	}

	// Write to MinIO (primary storage for ledger).
	key := ledgerStorageKey(ledger.Publisher, ledger.Name)
	if err := srv.Storage().WriteFile(ctx, key, data, 0o644); err != nil {
		return fmt.Errorf("write ledger to storage: %w", err)
	}

	// Write to ScyllaDB (distributed lookup).
	if srv.scylla != nil {
		srv.writeLedgerToScylla(ctx, ledger, data)
	}

	return nil
}

// ── ScyllaDB ledger operations ──────────────────────────────────────────

func (srv *server) readLedgerFromScylla(ctx context.Context, publisher, name string) *releaseLedger {
	if srv.scylla == nil {
		return nil
	}
	row, err := srv.scylla.GetManifest(ctx, fmt.Sprintf("ledger/%s/%s", publisher, name))
	if err != nil || row == nil {
		return nil
	}
	var ledger releaseLedger
	if err := json.Unmarshal(row.ManifestJSON, &ledger); err != nil {
		return nil
	}
	return &ledger
}

func (srv *server) writeLedgerToScylla(ctx context.Context, ledger *releaseLedger, data []byte) {
	if srv.scylla == nil {
		return
	}
	row := manifestRow{
		ArtifactKey:  fmt.Sprintf("ledger/%s/%s", ledger.Publisher, ledger.Name),
		ManifestJSON: data,
		PublishState: "LEDGER",
		PublisherID:  ledger.Publisher,
		Name:         ledger.Name,
		Version:      ledger.LatestVersion,
		BuildNumber:  0,
		CreatedAt:    time.Now(),
	}
	if err := srv.scylla.PutManifest(ctx, row); err != nil {
		slog.Warn("ledger: scylla write failed (non-fatal)", "name", ledger.Name, "err", err)
	}
}

// ── Ledger operations ───────────────────────────────────────────────────

// ledgerMu serializes ledger writes to prevent concurrent modification.
var ledgerMu sync.Mutex

// appendToLedger adds a new release entry to the package's ledger.
// Called after successful promote-to-PUBLISHED.
// Returns an error if the version is not monotonically increasing.
func (srv *server) appendToLedger(ctx context.Context, publisher, name, version, buildID, digest, platform string, sizeBytes int64) error {
	ledgerMu.Lock()
	defer ledgerMu.Unlock()

	ledger := srv.readLedger(ctx, publisher, name)
	if ledger == nil {
		ledger = &releaseLedger{
			Publisher: publisher,
			Name:      name,
		}
	}

	// Monotonic version enforcement: new version must be > latest.
	// Exception: same version is allowed (multiple platforms, or re-promote).
	if ledger.LatestVersion != "" && version != ledger.LatestVersion {
		cmp, err := versionutil.Compare(version, ledger.LatestVersion)
		if err == nil && cmp < 0 {
			return fmt.Errorf("non-monotonic version: %s < latest %s for %s/%s",
				version, ledger.LatestVersion, publisher, name)
		}
	}

	// Check for duplicate entry (idempotent re-promote).
	for _, r := range ledger.Releases {
		if r.BuildID == buildID {
			return nil // already in ledger
		}
	}

	entry := &releaseLedgerEntry{
		Version:    version,
		BuildID:    buildID,
		Digest:     digest,
		Platform:   platform,
		SizeBytes:  sizeBytes,
		ReleasedAt: time.Now().UTC().Format(time.RFC3339),
	}
	ledger.Releases = append(ledger.Releases, entry)

	// Update latest if this version is newer.
	if ledger.LatestVersion == "" {
		ledger.LatestVersion = version
		ledger.LatestBuildID = buildID
	} else {
		cmp, err := versionutil.Compare(version, ledger.LatestVersion)
		if err == nil && cmp >= 0 {
			ledger.LatestVersion = version
			ledger.LatestBuildID = buildID
		}
	}

	return srv.writeLedger(ctx, ledger)
}

// getLatestRelease returns the latest PUBLISHED build_id for a package on a
// specific platform. Returns ("", "") if no release exists.
func (srv *server) getLatestRelease(ctx context.Context, publisher, name, platform string) (version, buildID string) {
	ledger := srv.readLedger(ctx, publisher, name)
	if ledger == nil {
		return "", ""
	}

	// Walk releases in reverse (newest first) to find the latest for this platform.
	for i := len(ledger.Releases) - 1; i >= 0; i-- {
		r := ledger.Releases[i]
		if r.Platform == platform || platform == "" {
			return r.Version, r.BuildID
		}
	}

	return "", ""
}

// ── Ledger migration ────────────────────────────────────────────────────

const ledgerMigrationMarker = "ledger/.migration-complete"

// MigrateReleaseLedger builds the release ledger from existing PUBLISHED
// artifacts. Idempotent — skips if marker exists.
func (srv *server) MigrateReleaseLedger(ctx context.Context) {
	if _, err := srv.Storage().ReadFile(ctx, ledgerMigrationMarker); err == nil {
		slog.Debug("release ledger migration already complete")
		return
	}

	entries, err := srv.Storage().ReadDir(ctx, artifactsDir)
	if err != nil {
		slog.Debug("no artifacts directory, skipping ledger migration")
		return
	}

	built := 0
	for _, e := range entries {
		fname := e.Name()
		if !strings.HasSuffix(fname, ".manifest.json") {
			continue
		}
		key := strings.TrimSuffix(fname, ".manifest.json")
		_, state, m, readErr := srv.readManifestAndStateByKey(ctx, key)
		if readErr != nil || m == nil {
			continue
		}
		if state != repopb.PublishState_PUBLISHED {
			continue
		}
		ref := m.GetRef()
		if ref == nil {
			continue
		}
		buildID := m.GetBuildId()
		if buildID == "" {
			continue // can't add to ledger without build_id
		}

		err := srv.appendToLedger(ctx, ref.GetPublisherId(), ref.GetName(),
			ref.GetVersion(), buildID, m.GetChecksum(),
			ref.GetPlatform(), m.GetSizeBytes())
		if err != nil {
			slog.Warn("ledger migration: skip", "key", key, "err", err)
			continue
		}
		built++
	}

	// Write marker.
	marker, _ := json.Marshal(map[string]any{
		"migrated_at": time.Now().UTC().Format(time.RFC3339),
		"entries":     built,
	})
	_ = srv.Storage().WriteFile(ctx, ledgerMigrationMarker, marker, 0o644)

	slog.Info("release ledger migration complete", "entries", built)
}
