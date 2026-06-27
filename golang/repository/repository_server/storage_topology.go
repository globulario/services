package main

// storage_topology.go — ledger-to-CAS consistency scan.
//
// Packages live ONLY in the local POSIX CAS (the blob mirror was removed and
// MinIO is never used for packages — operator decision 2026-06-12). The former
// MinIO-endpoint topology validation and MinIO authority sentinel are gone with
// it; what remains is an operator-invoked consistency scan that compares the
// Scylla ledger (ground truth for artifact existence) against the local CAS
// (ground truth for blob presence). It returns counts of expected, present,
// missing, and checksum-mismatched artifacts, and drives doctor invariants.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"time"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// StorageConsistencyReport is the result of a full ledger-vs-CAS comparison.
type StorageConsistencyReport struct {
	// TotalInLedger is the number of PUBLISHED artifacts in the Scylla ledger.
	TotalInLedger int
	// Present is the count of artifacts whose blobs are confirmed present in the CAS.
	Present int
	// Missing is the count of artifacts whose blobs are absent from the CAS.
	Missing int
	// ChecksumMismatch is the count of artifacts present in the CAS but with
	// a different checksum than the ledger declares.
	ChecksumMismatch int
	// MissingKeys lists the artifact keys with absent blobs (for operator use).
	MissingKeys []string
	// MismatchKeys lists the artifact keys with checksum mismatches.
	MismatchKeys []string
	// Degraded is true when Missing > 0 or ChecksumMismatch > 0.
	Degraded bool
	// ScannedAt is when the scan completed.
	ScannedAt time.Time
}

// runStorageConsistencyScan compares the Scylla ledger against the local POSIX
// CAS and returns a report of artifacts that are known-to-ledger but missing or
// corrupt on disk.
//
// verifyChecksums=false: Stat-only check (fast, safe for routine health).
// verifyChecksums=true: full SHA-256 re-computation (slow, for deep integrity).
func (srv *server) runStorageConsistencyScan(ctx context.Context, verifyChecksums bool) (*StorageConsistencyReport, error) {
	if srv.scylla == nil {
		return nil, status.Error(codes.FailedPrecondition,
			"consistency scan requires ScyllaDB — ledger not available")
	}
	if srv.storage == nil {
		return nil, status.Error(codes.FailedPrecondition,
			"consistency scan requires local storage — backend not initialized")
	}

	rows, err := srv.scylla.ListManifests(ctx)
	if err != nil {
		return nil, fmt.Errorf("consistency scan: list ledger: %w", err)
	}

	report := &StorageConsistencyReport{ScannedAt: time.Now()}

	for _, row := range rows {
		// Only check PUBLISHED artifacts — other states are work-in-progress.
		if row.PublishState != repopb.PublishState_PUBLISHED.String() {
			continue
		}
		report.TotalInLedger++
		binKey := binaryStorageKey(row.ArtifactKey)

		fi, statErr := srv.storage.Stat(ctx, binKey)
		if statErr != nil {
			report.Missing++
			report.MissingKeys = append(report.MissingKeys, row.ArtifactKey)
			slog.Warn("consistency scan: blob missing from local CAS",
				"artifact_key", row.ArtifactKey, "binary_path", binKey)
			continue
		}

		// Size integrity — fast, always checked.
		if row.SizeBytes > 0 && fi.Size() != row.SizeBytes {
			report.ChecksumMismatch++
			report.MismatchKeys = append(report.MismatchKeys, row.ArtifactKey)
			slog.Warn("consistency scan: blob size mismatch",
				"artifact_key", row.ArtifactKey,
				"ledger_size", row.SizeBytes, "cas_size", fi.Size())
			continue
		}

		// Optional full SHA-256 re-verification.
		if verifyChecksums && row.Checksum != "" {
			blobData, readErr := srv.storage.ReadFile(ctx, binKey)
			if readErr != nil {
				report.Missing++
				report.MissingKeys = append(report.MissingKeys, row.ArtifactKey)
				continue
			}
			h := sha256.Sum256(blobData)
			actual := "sha256:" + hex.EncodeToString(h[:])
			if !digestEqual(actual, row.Checksum) {
				report.ChecksumMismatch++
				report.MismatchKeys = append(report.MismatchKeys, row.ArtifactKey)
				slog.Warn("consistency scan: checksum mismatch",
					"artifact_key", row.ArtifactKey,
					"ledger_checksum", row.Checksum, "actual_checksum", actual)
				continue
			}
		}
		report.Present++
	}

	report.Degraded = report.Missing > 0 || report.ChecksumMismatch > 0
	slog.Info("storage consistency scan complete",
		"total_in_ledger", report.TotalInLedger,
		"present", report.Present,
		"missing", report.Missing,
		"checksum_mismatch", report.ChecksumMismatch,
		"degraded", report.Degraded,
		"verify_checksums", verifyChecksums)
	return report, nil
}
