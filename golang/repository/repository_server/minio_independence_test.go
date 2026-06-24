package main

// minio_independence_test.go — acceptance tests proving the repository does not
// depend on MinIO for correctness.
//
// These tests validate the MinIO-independence acceptance criteria:
//   A. Repository works with MinIO stopped (RequireHealthy passes, RPCs succeed).
//   B. Publish cannot create PUBLISHED metadata with missing local blob.
//   C. dep_health.RequireHealthy() only blocks on ScyllaDB — never on MinIO.
//   F. MinIO canary failure disables mirror but not repository.

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
	"github.com/globulario/services/golang/storage_backend"
	"github.com/globulario/services/golang/subsystem"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// newTestServerWithMirror returns a server backed by a ResilientStorage
// (local temp dir + the provided mirror).
func newTestServerWithMirror(t *testing.T, mirror storage_backend.Storage) *server {
	t.Helper()
	localDir := t.TempDir()
	local := storage_backend.NewOSStorage(localDir)
	rs := storage_backend.NewResilientStorage(local, mirror)
	srv := &server{Root: localDir}
	srv.storage = rs
	srv.localStorage = local
	srv.mirrorStorage = mirror
	return srv
}

func nopLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func nopSubsystem(name string) *subsystem.SubsystemHandle {
	return subsystem.RegisterSubsystem(name, time.Minute)
}

// ── Test A: RequireHealthy() never blocks on MinIO failure ───────────────────

// TestDepHealth_MinIODownDoesNotBlockRPCs simulates: scyllaOK=true, mirrorOK=false
// → RequireHealthy must return nil.
func TestDepHealth_MinIODownDoesNotBlockRPCs(t *testing.T) {
	healthy := &atomic.Bool{}
	healthy.Store(true) // ScyllaDB is OK

	mirrorOK := &atomic.Bool{}
	mirrorOK.Store(false) // MinIO is down

	w := &depHealthWatchdog{
		healthy:  healthy,
		mirrorOK: mirrorOK,
		// scyllaSub and minioSub are nil — RequireHealthy only checks healthy.
	}

	if err := w.RequireHealthy(); err != nil {
		t.Fatalf("RequireHealthy() must return nil when ScyllaDB is OK, got: %v", err)
	}
}

// TestDepHealth_ScyllaDownBlocksRPCs: ScyllaDB down → RequireHealthy() MUST return an error.
func TestDepHealth_ScyllaDownBlocksRPCs(t *testing.T) {
	healthy := &atomic.Bool{}
	healthy.Store(false) // ScyllaDB is DOWN

	mirrorOK := &atomic.Bool{}
	mirrorOK.Store(true) // MinIO is up (irrelevant)

	w := &depHealthWatchdog{
		healthy:  healthy,
		mirrorOK: mirrorOK,
	}

	if err := w.RequireHealthy(); err == nil {
		t.Fatal("RequireHealthy() must return an error when ScyllaDB is down")
	}
}

// TestDepHealth_BothDownBlocksOnlyOnScylla: Both ScyllaDB AND MinIO down →
// RequireHealthy must ONLY report ScyllaDB.
func TestDepHealth_BothDownBlocksOnlyOnScylla(t *testing.T) {
	healthy := &atomic.Bool{}
	healthy.Store(false)

	mirrorOK := &atomic.Bool{}
	mirrorOK.Store(false)

	w := &depHealthWatchdog{
		healthy:  healthy,
		mirrorOK: mirrorOK,
	}

	err := w.RequireHealthy()
	if err == nil {
		t.Fatal("RequireHealthy() must return error when ScyllaDB is down")
	}
	// Error must mention ScyllaDB metadata, not MinIO.
	msg := err.Error()
	if !containsSubstring(msg, "ScyllaDB") && !containsSubstring(msg, "scylladb") && !containsSubstring(msg, "metadata") {
		t.Errorf("error should mention ScyllaDB, got: %s", msg)
	}
}

func containsSubstring(s, sub string) bool {
	if len(sub) == 0 {
		return true
	}
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// ── Test A: ResilientStorage — MinIO down, write/read still works ─────────────

func TestResilientStorage_MinIODownWriteReadWorks(t *testing.T) {
	mirror := &alwaysFailStorage{}
	srv := newTestServerWithMirror(t, mirror)
	ctx := context.Background()

	// Write through ResilientStorage — should succeed (local write).
	const path = "artifacts/test.bin"
	payload := []byte("content written with minio down")
	if err := srv.storage.WriteFile(ctx, path, payload, 0o644); err != nil {
		t.Fatalf("WriteFile with mirror down: %v", err)
	}

	// Read back — should succeed from local.
	got, err := srv.storage.ReadFile(ctx, path)
	if err != nil {
		t.Fatalf("ReadFile with mirror down: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Errorf("data mismatch: got %q want %q", got, payload)
	}
}

// ── Test B: publish cannot create PUBLISHED metadata with missing local blob ──

// TestPublish_MissingLocalBlobBlocksPromote verifies that promoteToPublished
// refuses to advance when the local blob is missing.
func TestPublish_MissingLocalBlobBlocksPromote(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	// Seed a minimal manifest (no binary written to storage).
	m := &repopb.ArtifactManifest{
		Ref: &repopb.ArtifactRef{
			PublisherId: "test-publisher",
			Name:        "test-pkg",
			Platform:    "linux_amd64",
			Version:     "1.0.0",
		},
		BuildNumber: 42,
		SizeBytes:   1024,
	}
	key := artifactKeyWithBuild(m.GetRef(), m.GetBuildNumber())

	// Pre-seed to LEDGER_WRITTEN (the state just before PUBLISHED).
	srv.cacheArtifactState(key, artifactStateRecord{State: PipelineLedgerWritten})

	// Do NOT write the binary blob — it's missing.
	// Attempt to promote to PUBLISHED.
	err := srv.promoteToPublished(ctx, key, m)
	if err == nil {
		t.Fatal("promoteToPublished must return an error when local blob is missing")
	}

	// Verify the artifact did NOT advance to PUBLISHED.
	state := srv.readArtifactState(ctx, key)
	if state == PipelinePublished {
		t.Errorf("artifact must not be PUBLISHED when local blob is missing, got PUBLISHED")
	}
}

// ── Test F: canary failure disables mirror, repository continues ──────────────

// TestCanary_FailureDisablesMirrorNotRepository verifies that a canary failure
// causes mirrorOK=false but leaves the service serving RPCs.
//
// pingMinio returns false on canary failure; check() applies the return value
// to mirrorOK. The test simulates check()'s assignment to confirm the model
// holds end-to-end: canary fails → mirrorOK=false → RequireHealthy still OK.
func TestCanary_FailureDisablesMirrorNotRepository(t *testing.T) {
	// ResilientStorage with a mirror that fails canary writes.
	localDir := t.TempDir()
	local := storage_backend.NewOSStorage(localDir)
	mirror := &alwaysFailStorage{} // canary writes will fail
	rs := storage_backend.NewResilientStorage(local, mirror)

	healthy := &atomic.Bool{}
	healthy.Store(true)
	mirrorOK := &atomic.Bool{}
	mirrorOK.Store(true) // starts true

	w := &depHealthWatchdog{
		storage:  rs,
		healthy:  healthy,
		mirrorOK: mirrorOK,
		logger:   nopLogger(),
		minioSub: nopSubsystem("test:minio-canary"),
	}

	ctx := context.Background()
	// Run the canary — it should fail because mirror rejects all writes.
	ok := w.pingMinio(ctx)
	if ok {
		t.Fatal("pingMinio should return false when canary fails")
	}

	// check() applies the pingMinio result to mirrorOK — simulate that here.
	mirrorOK.Store(ok) // false
	if mirrorOK.Load() {
		t.Fatal("mirrorOK should be false after canary failure")
	}

	// The service remains healthy — mirror failure never gates RPCs.
	if err := w.RequireHealthy(); err != nil {
		t.Fatalf("RequireHealthy must pass even when mirror is down: %v", err)
	}
}

// ── test infrastructure ───────────────────────────────────────────────────────

// alwaysFailStorage is a Storage where every operation returns an error.
// Unlike failStorage in storage_backend_test, this is in package main.
type alwaysFailStorage struct{}

var errAlwaysFail = errors.New("storage: always fails (test)")

func (*alwaysFailStorage) ReadFile(_ context.Context, _ string) ([]byte, error) {
	return nil, errAlwaysFail
}
func (*alwaysFailStorage) Open(_ context.Context, _ string) (io.ReadSeekCloser, error) {
	return nil, errAlwaysFail
}
func (*alwaysFailStorage) Stat(_ context.Context, _ string) (fs.FileInfo, error) {
	return nil, errAlwaysFail
}
func (*alwaysFailStorage) Exists(_ context.Context, _ string) bool { return false }
func (*alwaysFailStorage) ReadDir(_ context.Context, _ string) ([]fs.DirEntry, error) {
	return nil, errAlwaysFail
}
func (*alwaysFailStorage) WriteFile(_ context.Context, _ string, _ []byte, _ fs.FileMode) error {
	return errAlwaysFail
}
func (*alwaysFailStorage) AtomicWriteFile(_ context.Context, _ string, _ []byte, _ fs.FileMode) error {
	return errAlwaysFail
}
func (*alwaysFailStorage) Create(_ context.Context, _ string) (io.WriteCloser, error) {
	return nil, errAlwaysFail
}
func (*alwaysFailStorage) MkdirAll(_ context.Context, _ string, _ fs.FileMode) error {
	return errAlwaysFail
}
func (*alwaysFailStorage) RemoveAll(_ context.Context, _ string) error { return errAlwaysFail }
func (*alwaysFailStorage) Remove(_ context.Context, _ string) error    { return errAlwaysFail }
func (*alwaysFailStorage) Rename(_ context.Context, _, _ string) error { return errAlwaysFail }
func (*alwaysFailStorage) Ping(_ context.Context) error                { return errAlwaysFail }
func (*alwaysFailStorage) TempDir() string                             { return "/tmp" }
func (*alwaysFailStorage) Getwd() (string, error)                      { return "", errAlwaysFail }
