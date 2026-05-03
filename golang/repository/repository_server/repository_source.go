package main

// repository_source.go — RepositorySource abstraction for extensible artifact resolution.
//
// Source chain (priority order):
//   10  LocalPOSIXRepositorySource  — local verified CAS (always present, always first)
//   30  UpstreamRepositorySource    — GitHub / HTTP / LOCAL_DIR provider
//   50  MinIORepositorySource       — optional mirror (informational, never authority)
//
// Law: Install always happens from local verified POSIX storage.
//      No source may write to the local CAS; only MaterializeArtifactToLocal() may.
//      MinIO failure never blocks repository RPCs.

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"time"

	"github.com/globulario/services/golang/storage_backend"
)

// ── Error sentinels ───────────────────────────────────────────────────────────

var (
	ErrArtifactNotFound    = errors.New("artifact not found in source")
	ErrSourceUnavailable   = errors.New("source unavailable")
	ErrChecksumUnknown     = errors.New("source cannot provide checksum for this artifact")
	ErrSourceMisconfigured = errors.New("source is misconfigured")
)

// ── Request / Candidate types ─────────────────────────────────────────────────

// ArtifactRequest describes what artifact the resolver is looking for.
type ArtifactRequest struct {
	PublisherID string
	Name        string
	Version     string
	BuildNumber int64
	BuildID     string
	Platform    string
	Kind        string
	Sha256      string // expected sha256 — used for verification; may be empty
	SizeBytes   int64 // expected size — used for verification; 0 = unknown

	// Optional upstream context (from manifest.UpstreamImport).
	SourceName string
	ReleaseTag string
	AssetURL   string
	AssetPath  string
	Filename   string

	// WorkflowRunID is threaded through to pipeline state transitions so that
	// sync workflows get consistent audit trails. Empty is safe (observability only).
	WorkflowRunID string
}

// ArtifactCandidate is returned by RepositorySource.Open on a hit.
// Exactly one of LocalPath or Reader will be set.
type ArtifactCandidate struct {
	SourceName    string
	SourceType    string
	LocalPath     string        // set for LOCAL_POSIX hits — already on disk, no streaming needed
	Reader        io.ReadCloser // set for remote/mirror sources — caller must Close
	SizeBytes     int64         // 0 = unknown
	Sha256        string        // may be empty when source cannot provide it
	ProvenanceURL string
	Mirror        bool // true → treat as cache, never as authority
	Authority     bool // true → blob is authoritative (local POSIX only)
}

// SourceHealth describes whether a source is available for resolution.
type SourceHealth struct {
	Available bool
	Degraded  bool
	Reason    string
	Detail    map[string]string
}

// ── Interface ─────────────────────────────────────────────────────────────────

// RepositorySource is a read-only artifact source. Sources are composed into a
// priority-ordered chain by the resolver. All writes go through
// MaterializeArtifactToLocal — sources themselves never modify the local CAS.
type RepositorySource interface {
	Name() string
	Type() string
	Priority() int

	// Health returns availability without downloading artifacts.
	Health(ctx context.Context) SourceHealth

	// Open returns a stream or a local path for the artifact.
	// Returns ErrArtifactNotFound when the source doesn't have the artifact.
	// Returns ErrSourceUnavailable when the source is temporarily unreachable.
	// Never panics. Never modifies repository state.
	Open(ctx context.Context, req ArtifactRequest) (*ArtifactCandidate, error)
}

// ── LocalPOSIXRepositorySource ────────────────────────────────────────────────

// LocalPOSIXRepositorySource reads from the local POSIX CAS only.
// It explicitly uses the underlying OSStorage (not ResilientStorage) so that
// a mirror fallback cannot mask a genuine local miss.
type LocalPOSIXRepositorySource struct {
	store *storage_backend.OSStorage
	root  string
}

func newLocalPOSIXSource(store *storage_backend.OSStorage, root string) *LocalPOSIXRepositorySource {
	return &LocalPOSIXRepositorySource{store: store, root: root}
}

func (s *LocalPOSIXRepositorySource) Name() string     { return "local-posix" }
func (s *LocalPOSIXRepositorySource) Type() string     { return "LOCAL_POSIX" }
func (s *LocalPOSIXRepositorySource) Priority() int    { return 10 }

func (s *LocalPOSIXRepositorySource) Health(ctx context.Context) SourceHealth {
	fi, err := s.store.Stat(ctx, ".")
	if err != nil || !fi.IsDir() {
		return SourceHealth{Available: false, Reason: "local CAS root inaccessible"}
	}
	// Write-access probe: attempt a stat on the artifacts dir.
	if _, err := s.store.Stat(ctx, artifactsDir); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return SourceHealth{Available: false, Reason: fmt.Sprintf("artifacts dir inaccessible: %v", err)}
	}
	return SourceHealth{Available: true}
}

func (s *LocalPOSIXRepositorySource) Open(ctx context.Context, req ArtifactRequest) (*ArtifactCandidate, error) {
	ref := artifactRequestToRef(req)
	key := artifactKeyWithBuild(ref, req.BuildNumber)
	binKey := binaryStorageKey(key)

	fi, err := s.store.Stat(ctx, binKey)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, ErrArtifactNotFound
		}
		return nil, fmt.Errorf("%w: stat local blob: %v", ErrSourceUnavailable, err)
	}

	if req.SizeBytes > 0 && fi.Size() != req.SizeBytes {
		// Size mismatch — treat as MISS so the resolver tries upstream.
		return nil, fmt.Errorf("%w: local blob size %d != expected %d (treating as miss — upstream may have correct blob)",
			ErrArtifactNotFound, fi.Size(), req.SizeBytes)
	}

	localPath := s.store.LocalPath(binKey)

	// Verify sha256 when the request carries an expected digest.
	// A correct-size but wrong-content blob is treated as a MISS so the resolver
	// can fall through to upstream and repair the local CAS.
	if req.Sha256 != "" {
		actual, verErr := checksumLocalFile(localPath)
		if verErr != nil {
			return nil, fmt.Errorf("%w: local blob checksum read failed: %v", ErrSourceUnavailable, verErr)
		}
		if !digestEqual(actual, req.Sha256) {
			slog.Warn("local-posix: sha256 mismatch — treating as miss so resolver tries upstream",
				"key", binKey, "expected", req.Sha256, "actual", actual)
			return nil, fmt.Errorf("%w: local blob sha256 %s != expected %s",
				ErrArtifactNotFound, actual, req.Sha256)
		}
	}

	return &ArtifactCandidate{
		SourceName: s.Name(),
		SourceType: s.Type(),
		LocalPath:  localPath,
		SizeBytes:  fi.Size(),
		Sha256:     req.Sha256, // carry through if known
		Authority:  true,
	}, nil
}

// ── MinIORepositorySource ─────────────────────────────────────────────────────

// MinIORepositorySource reads from the optional MinIO mirror.
// Never blocks repository RPCs — any failure is logged and returns ErrSourceUnavailable.
// Never considered authority — Authority is always false.
type MinIORepositorySource struct {
	mirror storage_backend.Storage // the mirror component (may be nil)
}

func newMinIOSource(mirror storage_backend.Storage) *MinIORepositorySource {
	return &MinIORepositorySource{mirror: mirror}
}

func (s *MinIORepositorySource) Name() string  { return "minio-mirror" }
func (s *MinIORepositorySource) Type() string  { return "MINIO_MIRROR" }
func (s *MinIORepositorySource) Priority() int { return 50 }

func (s *MinIORepositorySource) Health(ctx context.Context) SourceHealth {
	if s.mirror == nil {
		return SourceHealth{Available: false, Reason: "no mirror configured"}
	}
	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	if err := s.mirror.Ping(pingCtx); err != nil {
		return SourceHealth{Available: false, Degraded: true, Reason: fmt.Sprintf("ping failed: %v", err)}
	}
	return SourceHealth{Available: true}
}

func (s *MinIORepositorySource) Open(ctx context.Context, req ArtifactRequest) (*ArtifactCandidate, error) {
	if s.mirror == nil {
		return nil, ErrSourceUnavailable
	}

	ref := artifactRequestToRef(req)
	key := artifactKeyWithBuild(ref, req.BuildNumber)
	binKey := binaryStorageKey(key)

	fi, statErr := s.mirror.Stat(ctx, binKey)
	if statErr != nil {
		if errors.Is(statErr, fs.ErrNotExist) {
			return nil, ErrArtifactNotFound
		}
		slog.Warn("repository-source: minio stat failed", "key", binKey, "err", statErr)
		return nil, fmt.Errorf("%w: %v", ErrSourceUnavailable, statErr)
	}

	rc, openErr := s.mirror.Open(ctx, binKey)
	if openErr != nil {
		slog.Warn("repository-source: minio open failed", "key", binKey, "err", openErr)
		return nil, fmt.Errorf("%w: %v", ErrSourceUnavailable, openErr)
	}

	return &ArtifactCandidate{
		SourceName: s.Name(),
		SourceType: s.Type(),
		Reader:     rc,
		SizeBytes:  fi.Size(),
		Mirror:     true,
		Authority:  false,
	}, nil
}
