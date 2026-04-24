package main

// storage_topology.go — MinIO storage topology validation, authority sentinel,
// and ledger-to-MinIO consistency scan.
//
// Three concerns, one file:
//
//  1. Phase 2 — Topology validation: At startup, reject configurations that
//     would silently route artifact reads to empty MinIO stores. In
//     standalone_authority mode, the authority_minio_endpoint MUST be a specific
//     node FQDN — never a round-robin DNS name like minio.globular.internal.
//
//  2. Phase 3 — Authority sentinel: Before serving artifact RPCs, verify that
//     the MinIO endpoint we are pointed at is actually the authority's store.
//     A sentinel object (artifacts/.repository-health/storage-authority.json)
//     is written by the authority at startup and checked by all instances.
//     A missing sentinel means the endpoint is likely an empty non-authority
//     MinIO — we refuse to serve rather than return false NotFound.
//
//  3. Phase 4 — Consistency scan: An operator-invoked function that compares the
//     Scylla ledger (ground truth for artifact existence) against the authority
//     MinIO (ground truth for blob presence). Returns counts of expected, present,
//     missing, and checksum-mismatched artifacts. Drives doctor invariants.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	repopb "github.com/globulario/services/golang/repository/repositorypb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ─── Phase 2: Topology Validation ─────────────────────────────────────────

// ErrStorageTopologyInvalid is returned by validateStorageTopology when the
// configured MinIO endpoint is unsafe for the declared storage mode.
type ErrStorageTopologyInvalid struct {
	Reason string
}

func (e ErrStorageTopologyInvalid) Error() string {
	return "REPOSITORY_STORAGE_TOPOLOGY_INVALID: " + e.Reason
}

// validateStorageTopology checks that the repository's MinIO endpoint is safe
// for the configured storage mode. Called at startup before serving RPCs.
//
// Rules enforced:
//   - Loopback addresses forbidden for remote storage endpoints.
//   - In standalone_authority mode: endpoint must NOT be a round-robin DNS name
//     (minio.globular.internal routes ~50% of reads to empty non-authority stores).
//   - If RepositoryStorageConfig is absent from etcd the cluster is likely
//     bootstrapping — lenient validation only (loopback check).
func (srv *server) validateStorageTopology(ctx context.Context) error {
	if srv.MinioConfig == nil || srv.MinioConfig.Endpoint == "" {
		// Storage not configured — caught by requireHealthy() at RPC time.
		return nil
	}

	// Always enforce: no loopback for remote addresses.
	if strings.Contains(srv.MinioConfig.Endpoint, "127.0.0.1") ||
		strings.Contains(srv.MinioConfig.Endpoint, "localhost") {
		return ErrStorageTopologyInvalid{
			Reason: fmt.Sprintf("MinIO endpoint %q uses loopback — forbidden for distributed storage",
				srv.MinioConfig.Endpoint),
		}
	}

	storageCfg, err := config.LoadRepositoryStorageConfig()
	if err != nil {
		// Storage config not in etcd yet — cluster may be bootstrapping.
		// Warn but don't block — the authority key alone is sufficient during bootstrap.
		slog.Warn("storage topology config not in etcd — skipping strict validation (cluster may be bootstrapping)",
			"err", err)
		return nil
	}

	// Strict validation for standalone_authority mode.
	if storageCfg.Mode == config.StorageModeStandaloneAuthority {
		roundRobinNames := []string{"minio.globular.internal"}
		for _, rr := range roundRobinNames {
			if strings.Contains(srv.MinioConfig.Endpoint, rr) {
				return ErrStorageTopologyInvalid{
					Reason: fmt.Sprintf("endpoint %q is a round-robin DNS name forbidden in standalone_authority mode — "+
						"use a specific node FQDN (e.g. globule-ryzen.globular.internal:9000) to avoid split-brain reads",
						srv.MinioConfig.Endpoint),
				}
			}
		}
		// Warn when our endpoint differs from the declared authority endpoint.
		if storageCfg.AuthorityMinioEndpoint != "" &&
			srv.MinioConfig.Endpoint != storageCfg.AuthorityMinioEndpoint {
			slog.Warn("minio endpoint differs from declared authority — non-authority instance will proxy artifact operations",
				"configured", srv.MinioConfig.Endpoint,
				"authority", storageCfg.AuthorityMinioEndpoint,
				"authority_node", storageCfg.AuthorityNodeID)
		}
	}

	slog.Info("storage topology validated",
		"mode", storageCfg.Mode,
		"endpoint", srv.MinioConfig.Endpoint,
		"authority_node", storageCfg.AuthorityNodeID)
	return nil
}

// ─── Phase 3: Authority Sentinel ──────────────────────────────────────────

const (
	sentinelPath    = "artifacts/.repository-health/storage-authority.json"
	sentinelTimeout = 10 * time.Second
)

// storageAuthoritySentinel is the content of the sentinel object written to
// MinIO by the authority node at startup. Non-authority nodes verify it exists
// before serving artifact RPCs.
type storageAuthoritySentinel struct {
	NodeID    string `json:"node_id"`
	WrittenAt int64  `json:"written_at"`
	Bucket    string `json:"bucket"`
	Checksum  string `json:"checksum,omitempty"`
}

// writeAuthoritySentinel writes the authority sentinel object to MinIO.
// Called by the authority node at startup, after storage is initialised.
func (srv *server) writeAuthoritySentinel(ctx context.Context, nodeID string) error {
	if srv.storage == nil {
		return fmt.Errorf("authority sentinel: storage not initialized")
	}
	sentinel := storageAuthoritySentinel{
		NodeID:    nodeID,
		WrittenAt: time.Now().Unix(),
		Bucket:    srv.MinioConfig.Bucket,
	}
	// Compute checksum over deterministic fields.
	raw, err := json.Marshal(storageAuthoritySentinel{
		NodeID: sentinel.NodeID,
		Bucket: sentinel.Bucket,
	})
	if err != nil {
		return fmt.Errorf("authority sentinel: marshal for checksum: %w", err)
	}
	h := sha256.Sum256(raw)
	sentinel.Checksum = hex.EncodeToString(h[:])

	data, err := json.Marshal(sentinel)
	if err != nil {
		return fmt.Errorf("authority sentinel: marshal: %w", err)
	}

	wCtx, cancel := context.WithTimeout(ctx, sentinelTimeout)
	defer cancel()
	if err := srv.storage.WriteFile(wCtx, sentinelPath, data, 0o644); err != nil {
		return fmt.Errorf("authority sentinel: write to minio: %w", err)
	}
	slog.Info("authority sentinel written to MinIO",
		"path", sentinelPath,
		"node_id", nodeID,
		"bucket", sentinel.Bucket)
	return nil
}

// verifyAuthoritySentinel reads and validates the sentinel from MinIO.
// Returns nil if the sentinel is present and valid.
//
// A missing sentinel means the MinIO endpoint is likely an empty non-authority
// store. The caller should fail rather than serve false NotFound responses.
//
// expectedNodeID may be "" — node identity is not verified when empty.
func (srv *server) verifyAuthoritySentinel(ctx context.Context, expectedNodeID string) error {
	if srv.storage == nil {
		return fmt.Errorf("authority sentinel: storage not initialized")
	}
	rCtx, cancel := context.WithTimeout(ctx, sentinelTimeout)
	defer cancel()

	data, err := srv.storage.ReadFile(rCtx, sentinelPath)
	if err != nil {
		return fmt.Errorf("authority sentinel missing at %q — "+
			"this MinIO endpoint may not be the authority store; "+
			"set /globular/repository/authority to the correct node: %w",
			sentinelPath, err)
	}

	var sentinel storageAuthoritySentinel
	if err := json.Unmarshal(data, &sentinel); err != nil {
		return fmt.Errorf("authority sentinel corrupt (cannot parse JSON at %q): %w", sentinelPath, err)
	}
	if sentinel.NodeID == "" {
		return fmt.Errorf("authority sentinel: node_id is empty — sentinel at %q is invalid", sentinelPath)
	}
	if expectedNodeID != "" && sentinel.NodeID != expectedNodeID {
		return fmt.Errorf("authority sentinel: node_id mismatch — sentinel says %q, expected %q; "+
			"this MinIO is not the authority store", sentinel.NodeID, expectedNodeID)
	}
	slog.Debug("authority sentinel verified",
		"path", sentinelPath,
		"node_id", sentinel.NodeID,
		"written_at", time.Unix(sentinel.WrittenAt, 0).Format(time.RFC3339))
	return nil
}

// ─── Phase 4: Ledger-to-MinIO Consistency Scan ────────────────────────────

// StorageConsistencyReport is the result of a full ledger-vs-MinIO comparison.
type StorageConsistencyReport struct {
	// TotalInLedger is the number of PUBLISHED artifacts in the Scylla ledger.
	TotalInLedger int
	// Present is the count of artifacts whose blobs are confirmed present in MinIO.
	Present int
	// Missing is the count of artifacts whose blobs are absent from MinIO.
	Missing int
	// ChecksumMismatch is the count of artifacts present in MinIO but with
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

// runStorageConsistencyScan compares the Scylla ledger against the authority
// MinIO and returns a report of artifacts that are known-to-ledger but
// missing or corrupt in MinIO.
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
			"consistency scan requires MinIO storage — backend not initialized")
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
			slog.Warn("consistency scan: blob missing from MinIO",
				"artifact_key", row.ArtifactKey, "binary_path", binKey)
			continue
		}

		// Size integrity — fast, always checked.
		if row.SizeBytes > 0 && fi.Size() != row.SizeBytes {
			report.ChecksumMismatch++
			report.MismatchKeys = append(report.MismatchKeys, row.ArtifactKey)
			slog.Warn("consistency scan: blob size mismatch",
				"artifact_key", row.ArtifactKey,
				"ledger_size", row.SizeBytes, "minio_size", fi.Size())
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
			actual := hex.EncodeToString(h[:])
			if actual != row.Checksum {
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
