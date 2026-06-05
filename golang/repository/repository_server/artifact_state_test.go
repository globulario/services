package main

// artifact_state_test.go — Phase A release-blocking tests for the
// repository pipeline state machine.

import (
	"context"
	"strings"
	"sync"
	"testing"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
	"github.com/globulario/services/golang/repository/upstream"
)

// transitionRecorder collects the sequence of state transitions observed for
// a given artifact key. Thread-safe so tests can inspect the slice safely.
type transitionRecorder struct {
	mu      sync.Mutex
	entries []recordedTransition
}

type recordedTransition struct {
	ArtifactKey   string
	From          ArtifactPipelineState
	To            ArtifactPipelineState
	Reason        string
	WorkflowRunID string
}

func (r *transitionRecorder) hook(key string, from, to ArtifactPipelineState, reason, runID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.entries = append(r.entries, recordedTransition{key, from, to, reason, runID})
}

func (r *transitionRecorder) statesFor(key string) []ArtifactPipelineState {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []ArtifactPipelineState
	for _, e := range r.entries {
		if e.ArtifactKey == key {
			out = append(out, e.To)
		}
	}
	return out
}

// installRecorder wires a transitionRecorder onto a server and returns it.
func installRecorder(t *testing.T, srv *server) *transitionRecorder {
	t.Helper()
	r := &transitionRecorder{}
	srv.artifactStateHook = r.hook
	return r
}

// ── State machine unit tests ───────────────────────────────────────────────

func TestIsTransitionAllowed_HappyPathChain(t *testing.T) {
	chain := []ArtifactPipelineState{
		PipelineUnspecified,
		PipelineDiscovered,
		PipelineDownloading,
		PipelineBlobWritten,
		PipelineBlobVerified,
		PipelineManifestWritten,
		PipelineLedgerWritten,
		PipelinePublished,
	}
	for i := 0; i < len(chain)-1; i++ {
		if !IsTransitionAllowed(chain[i], chain[i+1]) {
			t.Errorf("expected %s → %s to be allowed", chain[i], chain[i+1])
		}
	}
}

func TestIsTransitionAllowed_RepairChain(t *testing.T) {
	for _, from := range []ArtifactPipelineState{
		PipelineBrokenMissingBlob, PipelineBrokenChecksumMismatch,
	} {
		if !IsTransitionAllowed(from, PipelineDownloading) {
			t.Errorf("repair: %s → DOWNLOADING must be allowed", from)
		}
	}
}

func TestIsTransitionAllowed_PublishedDegrades(t *testing.T) {
	for _, to := range []ArtifactPipelineState{
		PipelineBrokenMissingBlob, PipelineBrokenChecksumMismatch,
		PipelineQuarantined, PipelineRevoked,
	} {
		if !IsTransitionAllowed(PipelinePublished, to) {
			t.Errorf("PUBLISHED → %s must be allowed", to)
		}
	}
}

func TestIsTransitionAllowed_RevokedTerminal(t *testing.T) {
	for _, to := range []ArtifactPipelineState{
		PipelinePublished, PipelineDownloading, PipelineQuarantined,
	} {
		if IsTransitionAllowed(PipelineRevoked, to) {
			t.Errorf("REVOKED is terminal; %s must NOT be allowed", to)
		}
	}
	// Self-transition is allowed for retry safety.
	if !IsTransitionAllowed(PipelineRevoked, PipelineRevoked) {
		t.Error("self-transition must always be allowed")
	}
}

func TestIsTransitionAllowed_PublishedBackToDownloadingForbidden(t *testing.T) {
	if IsTransitionAllowed(PipelinePublished, PipelineDownloading) {
		t.Fatal("PUBLISHED must not jump straight to DOWNLOADING — must go via BROKEN_X")
	}
}

func TestTransitionArtifactState_IdempotentSelfTransition(t *testing.T) {
	srv := newTestServer(t)
	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io", Name: "echo",
		Version: "1.0.0", Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
	}
	key := artifactKeyWithBuild(ref, 1)
	fields := ArtifactStateFields{Name: "echo", Version: "1.0.0", Platform: "linux_amd64"}

	if err := srv.transitionArtifactState(context.Background(), key, PipelineDiscovered, "first", "", fields); err != nil {
		t.Fatalf("first transition: %v", err)
	}
	if err := srv.transitionArtifactState(context.Background(), key, PipelineDiscovered, "retry", "", fields); err != nil {
		t.Fatalf("idempotent retry must be allowed: %v", err)
	}
	if got := srv.readArtifactState(context.Background(), key); got != PipelineDiscovered {
		t.Fatalf("state after idempotent retry: got %s, want DISCOVERED", got)
	}
}

func TestTransitionArtifactState_RejectsIllegalTransition(t *testing.T) {
	srv := newTestServer(t)
	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io", Name: "echo",
		Version: "1.0.0", Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
	}
	key := artifactKeyWithBuild(ref, 1)
	fields := ArtifactStateFields{}

	// Walk to PUBLISHED legally.
	steps := []ArtifactPipelineState{
		PipelineDiscovered, PipelineDownloading, PipelineBlobWritten,
		PipelineBlobVerified, PipelineManifestWritten, PipelineLedgerWritten,
		PipelinePublished,
	}
	for _, s := range steps {
		if err := srv.transitionArtifactState(context.Background(), key, s, "walk", "", fields); err != nil {
			t.Fatalf("transition to %s: %v", s, err)
		}
	}
	// PUBLISHED → DISCOVERED is not a legal edge.
	err := srv.transitionArtifactState(context.Background(), key, PipelineDiscovered, "illegal", "", fields)
	if err == nil {
		t.Fatal("expected illegal-transition error PUBLISHED → DISCOVERED")
	}
	if !strings.Contains(err.Error(), "illegal transition") {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := srv.readArtifactState(context.Background(), key); got != PipelinePublished {
		t.Fatalf("illegal transition must not mutate state; got %s", got)
	}
}

// ── End-to-end sync state-machine tests ────────────────────────────────────

// TestArtifactStateHappyPathSync: brand-new artifact synced from upstream
// must progress through every pipeline state and end at PUBLISHED.
func TestArtifactStateHappyPathSync(t *testing.T) {
	root, _ := createLocalDirSource(t, "v1.0.84", "echo", "1.0.84")

	srv := newTestServer(t)
	rec := installRecorder(t, srv)
	ctx := context.Background()

	provider, _ := upstream.NewSource(upstream.TypeLocalDir)
	opts := upstream.SourceOpts{
		LocalRoot:         root,
		IndexPathTemplate: "releases/{tag}/release-index.json",
	}
	indexData, err := provider.GetReleaseIndex(ctx, opts, "v1.0.84")
	if err != nil {
		t.Fatalf("get index: %v", err)
	}
	idx, err := parseReleaseIndex(indexData)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	src := &repopb.UpstreamSource{Name: "test-source", Enabled: true}
	result := srv.processSyncEntry(ctx, idx.Packages[0], src, provider, opts, "v1.0.84", false, "")
	if result.Status != repopb.UpstreamSyncStatus_SYNC_IMPORTED {
		t.Fatalf("expected SYNC_IMPORTED, got %s: %s", result.Status, result.Detail)
	}

	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io", Name: "echo",
		Version: "1.0.84", Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
	}
	key := artifactKeyWithBuild(ref, 1)

	got := rec.statesFor(key)
	want := []ArtifactPipelineState{
		PipelineDiscovered, PipelineDownloading, PipelineBlobWritten,
		PipelineBlobVerified, PipelineManifestWritten, PipelineLedgerWritten,
		PipelinePublished,
	}
	if len(got) != len(want) {
		t.Fatalf("wrong number of transitions: got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("transition %d: got %s, want %s (full sequence: %v)", i, got[i], want[i], got)
		}
	}
	if final := srv.readArtifactState(ctx, key); final != PipelinePublished {
		t.Fatalf("final state must be PUBLISHED, got %s", final)
	}
}

// TestSyncSkipsOnlyWhenPublishedAndBlobVerified: ledger + manifest + blob
// + matching digest + state PUBLISHED → SYNC_SKIPPED, state stays PUBLISHED.
func TestSyncSkipsOnlyWhenPublishedAndBlobVerified(t *testing.T) {
	root, expectedDigest := createLocalDirSource(t, "v1.0.84", "echo", "1.0.84")
	pkgContent := []byte("fake-package-binary-content-for-echo")

	srv := newTestServer(t)
	ctx := context.Background()

	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io", Name: "echo",
		Version: "1.0.84", Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
	}
	seedPublishedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref: ref, BuildNumber: 1, BuildId: "e2e-1",
		Checksum: expectedDigest, SizeBytes: int64(len(pkgContent)),
	})
	key := artifactKeyWithBuild(ref, 1)
	binKey := binaryStorageKey(key)
	if err := srv.Storage().WriteFile(ctx, binKey, pkgContent, 0o644); err != nil {
		t.Fatalf("rewrite blob: %v", err)
	}

	// Pre-stamp PUBLISHED so the skip path sees a coherent state — this is
	// the post-backfill / steady-state shape.
	fields := ArtifactStateFields{
		BlobKey: binKey, Checksum: expectedDigest,
		SizeBytes: int64(len(pkgContent)), BuildID: "e2e-1", BuildNumber: 1,
		PublisherID: "core@globular.io", Name: "echo",
		Version: "1.0.84", Platform: "linux_amd64",
	}
	if err := srv.transitionArtifactState(ctx, key, PipelinePublished, "test_pre_publish", "", fields); err != nil {
		t.Fatalf("pre-publish: %v", err)
	}

	rec := installRecorder(t, srv)
	provider, _ := upstream.NewSource(upstream.TypeLocalDir)
	opts := upstream.SourceOpts{LocalRoot: root, IndexPathTemplate: "releases/{tag}/release-index.json"}
	indexData, _ := provider.GetReleaseIndex(ctx, opts, "v1.0.84")
	idx, _ := parseReleaseIndex(indexData)

	src := &repopb.UpstreamSource{Name: "test-source", Enabled: true}
	result := srv.processSyncEntry(ctx, idx.Packages[0], src, provider, opts, "v1.0.84", false, "")

	if result.Status != repopb.UpstreamSyncStatus_SYNC_SKIPPED {
		t.Fatalf("expected SYNC_SKIPPED, got %s: %s", result.Status, result.Detail)
	}
	if !strings.Contains(result.Detail, "blob verified") {
		t.Fatalf("detail should include 'blob verified'; got %q", result.Detail)
	}
	if got := srv.readArtifactState(ctx, key); got != PipelinePublished {
		t.Fatalf("state after skip: got %s, want PUBLISHED", got)
	}
	// Skip path emits an idempotent PUBLISHED → PUBLISHED self-transition.
	for _, e := range rec.statesFor(key) {
		if e != PipelinePublished {
			t.Fatalf("skip path must not introduce other states; got %s in sequence", e)
		}
	}
}

// TestPublishedMetadataMissingBlobBecomesBrokenAndRepairs: the original
// production bug, now expressed as state transitions.
func TestPublishedMetadataMissingBlobBecomesBrokenAndRepairs(t *testing.T) {
	root, expectedDigest := createLocalDirSource(t, "v1.0.84", "echo", "1.0.84")
	pkgContent := []byte("fake-package-binary-content-for-echo")

	srv := newTestServer(t)
	ctx := context.Background()

	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io", Name: "echo",
		Version: "1.0.84", Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
	}
	seedPublishedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref: ref, BuildNumber: 1, BuildId: "e2e-1",
		Checksum: expectedDigest, SizeBytes: int64(len(pkgContent)),
	})
	key := artifactKeyWithBuild(ref, 1)
	binKey := binaryStorageKey(key)

	// Stamp PUBLISHED, then delete the blob — the production bug shape.
	fields := ArtifactStateFields{
		BlobKey: binKey, Checksum: expectedDigest,
		SizeBytes: int64(len(pkgContent)), BuildID: "e2e-1", BuildNumber: 1,
		PublisherID: "core@globular.io", Name: "echo",
		Version: "1.0.84", Platform: "linux_amd64",
	}
	if err := srv.transitionArtifactState(ctx, key, PipelinePublished, "test_pre_publish", "", fields); err != nil {
		t.Fatalf("pre-publish: %v", err)
	}
	if err := srv.Storage().Remove(ctx, binKey); err != nil {
		t.Fatalf("simulate missing blob: %v", err)
	}

	rec := installRecorder(t, srv)
	provider, _ := upstream.NewSource(upstream.TypeLocalDir)
	opts := upstream.SourceOpts{LocalRoot: root, IndexPathTemplate: "releases/{tag}/release-index.json"}
	indexData, _ := provider.GetReleaseIndex(ctx, opts, "v1.0.84")
	idx, _ := parseReleaseIndex(indexData)

	src := &repopb.UpstreamSource{Name: "test-source", Enabled: true}
	result := srv.processSyncEntry(ctx, idx.Packages[0], src, provider, opts, "v1.0.84", false, "")

	if result.Status != repopb.UpstreamSyncStatus_SYNC_IMPORTED {
		t.Fatalf("expected SYNC_IMPORTED, got %s: %s", result.Status, result.Detail)
	}
	if result.Action != "repair_blob" {
		t.Fatalf("action: got %q, want repair_blob", result.Action)
	}
	if final := srv.readArtifactState(ctx, key); final != PipelinePublished {
		t.Fatalf("final state: got %s, want PUBLISHED", final)
	}
	// Sequence must include BROKEN_MISSING_BLOB before re-PUBLISHED.
	seen := rec.statesFor(key)
	if !contains(seen, PipelineBrokenMissingBlob) {
		t.Fatalf("expected BROKEN_MISSING_BLOB in sequence; got %v", seen)
	}
	if !contains(seen, PipelinePublished) {
		t.Fatalf("expected final PUBLISHED in sequence; got %v", seen)
	}
	// DownloadArtifact would now succeed — verify blob exists.
	if _, err := srv.Storage().Stat(ctx, binKey); err != nil {
		t.Fatalf("blob must be present after repair: %v", err)
	}
}

// TestPublishedMetadataChecksumMismatchBecomesBrokenAndRepairs: corrupted
// blob (size mismatch) on a previously PUBLISHED row.
func TestPublishedMetadataChecksumMismatchBecomesBrokenAndRepairs(t *testing.T) {
	root, expectedDigest := createLocalDirSource(t, "v1.0.84", "echo", "1.0.84")
	pkgContent := []byte("fake-package-binary-content-for-echo")

	srv := newTestServer(t)
	ctx := context.Background()

	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io", Name: "echo",
		Version: "1.0.84", Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
	}
	seedPublishedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref: ref, BuildNumber: 1, BuildId: "e2e-1",
		Checksum: expectedDigest, SizeBytes: int64(len(pkgContent)),
	})
	key := artifactKeyWithBuild(ref, 1)
	binKey := binaryStorageKey(key)

	fields := ArtifactStateFields{
		BlobKey: binKey, Checksum: expectedDigest,
		SizeBytes: int64(len(pkgContent)), BuildID: "e2e-1", BuildNumber: 1,
		PublisherID: "core@globular.io", Name: "echo",
		Version: "1.0.84", Platform: "linux_amd64",
	}
	if err := srv.transitionArtifactState(ctx, key, PipelinePublished, "test_pre_publish", "", fields); err != nil {
		t.Fatalf("pre-publish: %v", err)
	}
	corrupt := []byte("short")
	if int64(len(corrupt)) == int64(len(pkgContent)) {
		t.Fatal("setup: corrupt content must differ in length")
	}
	if err := srv.Storage().WriteFile(ctx, binKey, corrupt, 0o644); err != nil {
		t.Fatalf("corrupt blob: %v", err)
	}

	rec := installRecorder(t, srv)
	provider, _ := upstream.NewSource(upstream.TypeLocalDir)
	opts := upstream.SourceOpts{LocalRoot: root, IndexPathTemplate: "releases/{tag}/release-index.json"}
	indexData, _ := provider.GetReleaseIndex(ctx, opts, "v1.0.84")
	idx, _ := parseReleaseIndex(indexData)

	src := &repopb.UpstreamSource{Name: "test-source", Enabled: true}
	result := srv.processSyncEntry(ctx, idx.Packages[0], src, provider, opts, "v1.0.84", false, "")

	if result.Status != repopb.UpstreamSyncStatus_SYNC_IMPORTED {
		t.Fatalf("expected SYNC_IMPORTED, got %s: %s", result.Status, result.Detail)
	}
	if result.Action != "repair_blob" {
		t.Fatalf("action: got %q, want repair_blob", result.Action)
	}
	if final := srv.readArtifactState(ctx, key); final != PipelinePublished {
		t.Fatalf("final state: got %s, want PUBLISHED", final)
	}
	seen := rec.statesFor(key)
	if !contains(seen, PipelineBrokenChecksumMismatch) {
		t.Fatalf("expected BROKEN_CHECKSUM_MISMATCH in sequence; got %v", seen)
	}
	if !contains(seen, PipelinePublished) {
		t.Fatalf("expected final PUBLISHED in sequence; got %v", seen)
	}
}

// ── Backfill tests ─────────────────────────────────────────────────────────
//
// The backfill helper calls srv.scylla.ListManifests; in the test server
// srv.scylla is nil, so we exercise the per-row classification directly via
// the artifactBlobStatus + transition path. This keeps the test independent
// of a real ScyllaDB cluster while still exercising the classification rule.

// classifyOneForBackfill is the same per-row classification the backfill
// helper uses — extracted so tests can drive it without a Scylla session.
func (srv *server) classifyOneForBackfill(ctx context.Context, ref *repopb.ArtifactRef, buildNumber int64, ledgerSize int64, publishState string) ArtifactPipelineState {
	key := artifactKeyWithBuild(ref, buildNumber)
	fields := ArtifactStateFields{
		BlobKey:     binaryStorageKey(key),
		SizeBytes:   ledgerSize,
		BuildNumber: buildNumber,
		PublisherID: ref.GetPublisherId(),
		Name:        ref.GetName(),
		Version:     ref.GetVersion(),
		Platform:    ref.GetPlatform(),
	}
	switch publishState {
	case repopb.PublishState_QUARANTINED.String():
		_ = srv.transitionArtifactState(ctx, key, PipelineQuarantined, "backfill_test", "", fields)
		return PipelineQuarantined
	case repopb.PublishState_REVOKED.String():
		_ = srv.transitionArtifactState(ctx, key, PipelineRevoked, "backfill_test", "", fields)
		return PipelineRevoked
	case repopb.PublishState_CORRUPTED.String():
		_ = srv.transitionArtifactState(ctx, key, PipelineBrokenChecksumMismatch, "backfill_test", "", fields)
		return PipelineBrokenChecksumMismatch
	}
	present, reason := srv.artifactBlobStatus(ctx, ref, buildNumber, ledgerSize)
	switch {
	case present:
		_ = srv.transitionArtifactState(ctx, key, PipelinePublished, "backfill_test", "", fields)
		return PipelinePublished
	case reason == "missing_blob":
		srv.markPipelineMissingBlob(ctx, key, "backfill_test", "", fields)
		return PipelineBrokenMissingBlob
	case reason == "size_mismatch":
		srv.markPipelineBrokenChecksum(ctx, key, "backfill_test", "", fields)
		return PipelineBrokenChecksumMismatch
	default:
		return PipelineUnspecified
	}
}

func TestBackfillArtifactState(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	// Row A: valid blob → should classify as PUBLISHED.
	refA := &repopb.ArtifactRef{
		PublisherId: "core@globular.io", Name: "echo",
		Version: "1.0.0", Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
	}
	seedPublishedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref: refA, BuildNumber: 1, BuildId: "a-1",
		Checksum: "sha256:abc", SizeBytes: 100, // size matches what seedPublishedArtifact writes
	})

	// Row B: missing blob → should classify as BROKEN_MISSING_BLOB.
	refB := &repopb.ArtifactRef{
		PublisherId: "core@globular.io", Name: "rbac",
		Version: "1.0.0", Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
	}
	seedPublishedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref: refB, BuildNumber: 1, BuildId: "b-1",
		Checksum: "sha256:def", SizeBytes: 100,
	})
	bKey := artifactKeyWithBuild(refB, 1)
	if err := srv.Storage().Remove(ctx, binaryStorageKey(bKey)); err != nil {
		t.Fatalf("remove blob B: %v", err)
	}

	// Row C: blob present but wrong size → BROKEN_CHECKSUM_MISMATCH.
	refC := &repopb.ArtifactRef{
		PublisherId: "core@globular.io", Name: "gateway",
		Version: "1.0.0", Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
	}
	seedPublishedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref: refC, BuildNumber: 1, BuildId: "c-1",
		Checksum: "sha256:ghi", SizeBytes: 100,
	})
	cKey := artifactKeyWithBuild(refC, 1)
	if err := srv.Storage().WriteFile(ctx, binaryStorageKey(cKey), []byte("short"), 0o644); err != nil {
		t.Fatalf("rewrite blob C: %v", err)
	}

	stateA := srv.classifyOneForBackfill(ctx, refA, 1, 100, repopb.PublishState_PUBLISHED.String())
	stateB := srv.classifyOneForBackfill(ctx, refB, 1, 100, repopb.PublishState_PUBLISHED.String())
	stateC := srv.classifyOneForBackfill(ctx, refC, 1, 100, repopb.PublishState_PUBLISHED.String())

	if stateA != PipelinePublished {
		t.Errorf("row A: got %s, want PUBLISHED", stateA)
	}
	if stateB != PipelineBrokenMissingBlob {
		t.Errorf("row B: got %s, want BROKEN_MISSING_BLOB", stateB)
	}
	if stateC != PipelineBrokenChecksumMismatch {
		t.Errorf("row C: got %s, want BROKEN_CHECKSUM_MISMATCH", stateC)
	}

	// Re-running classification on row A is idempotent — state stays PUBLISHED.
	stateAretry := srv.classifyOneForBackfill(ctx, refA, 1, 100, repopb.PublishState_PUBLISHED.String())
	if stateAretry != PipelinePublished {
		t.Errorf("row A retry: got %s, want PUBLISHED", stateAretry)
	}
}

// contains is a small helper to check membership in a state slice.
func contains(haystack []ArtifactPipelineState, needle ArtifactPipelineState) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}

// TestPipelineState_IsKnown pins meta.fallback_must_degrade_semantics for
// the artifact state read path. The PipelineUnknown sentinel exists so
// callers that gate destructive transitions (rollback eligibility, broken
// checksum downgrade) can distinguish "Scylla unreachable" from a legitimate
// empty/legacy row — both used to return PipelineUnspecified, letting a
// REVOKED artifact silently pass the terminal-state gate during an outage.
//
// IsKnown() must return true for every state EXCEPT PipelineUnknown.
func TestPipelineState_IsKnown(t *testing.T) {
	for _, s := range []ArtifactPipelineState{
		PipelineUnspecified, // legitimate legacy empty
		PipelineDiscovered,
		PipelineDownloading,
		PipelineBlobWritten,
		PipelineBlobVerified,
		PipelineManifestWritten,
		PipelineLedgerWritten,
		PipelinePublished,
		PipelineQuarantined,
		PipelineRevoked,
		PipelineBrokenMissingBlob,
		PipelineBrokenChecksumMismatch,
	} {
		if !s.IsKnown() {
			t.Errorf("%q.IsKnown() = false, want true (only PipelineUnknown is unknown)", s)
		}
	}
	if PipelineUnknown.IsKnown() {
		t.Errorf("PipelineUnknown.IsKnown() = true, want false")
	}
}
