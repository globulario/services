package main

// repository_source_resolver_test.go — acceptance tests for the RepositorySource
// chain and MaterializeArtifactToLocal.
//
// All tests are etcd-free: sources and policy are injected directly via
// resolveFromSources / MaterializeArtifactToLocal.
//
// Scenarios:
//   A  Local hit               — LOCAL_POSIX has blob; no other source consulted
//   B  Local miss → upstream   — local miss; stub upstream returns Reader; materialized
//   C  Checksum mismatch       — upstream returns wrong bytes; CHECKSUM_MISMATCH; continues
//   D  All sources miss        — every source returns MISS; error with diagnostics
//   E  RequireChecksum gate    — no sha256 in request + non-local source blocked
//   F  Source unavailable      — Health()=false; UNAVAILABLE recorded; continues
//   G  Materialize fast-path   — candidate has LocalPath; stat-only, no write
//   H  Materialize with Reader — streaming write, sha256 verified, file appears on disk
//   I  Materialize bad sha256  — Reader returns wrong bytes; errChecksumMismatch

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"io/fs"
	"os"
	"testing"

	"github.com/globulario/services/golang/storage_backend"
)

// ── stub RepositorySource ─────────────────────────────────────────────────────

type stubSource struct {
	name      string
	typ       string
	priority  int
	available bool
	healthMsg string
	candidate *ArtifactCandidate // nil → ErrArtifactNotFound
	openErr   error
}

func (s *stubSource) Name() string  { return s.name }
func (s *stubSource) Type() string  { return s.typ }
func (s *stubSource) Priority() int { return s.priority }

func (s *stubSource) Health(_ context.Context) SourceHealth {
	return SourceHealth{Available: s.available, Reason: s.healthMsg}
}

func (s *stubSource) Open(_ context.Context, _ ArtifactRequest) (*ArtifactCandidate, error) {
	if s.openErr != nil {
		return nil, s.openErr
	}
	if s.candidate == nil {
		return nil, ErrArtifactNotFound
	}
	return s.candidate, nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func testReq() ArtifactRequest {
	return ArtifactRequest{
		PublisherID: "test@example.com",
		Name:        "widget",
		Version:     "1.2.3",
		Platform:    "linux_amd64",
		BuildNumber: 7,
		BuildID:     "build-uuid-7",
	}
}

func sha256Of(data []byte) string {
	h := sha256.New()
	h.Write(data)
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

func newResolverServer(t *testing.T) *server {
	t.Helper()
	dir := t.TempDir()
	local := storage_backend.NewOSStorage(dir)
	srv := &server{Root: dir}
	srv.storage = local
	srv.localStorage = local
	return srv
}

func permissivePolicy() SourcePolicy {
	return SourcePolicy{
		Enabled:         true,
		RequireChecksum: false,
	}
}

// ── Scenario A: local hit ─────────────────────────────────────────────────────

func TestResolver_A_LocalHit(t *testing.T) {
	srv := newResolverServer(t)
	ctx := context.Background()

	// Write a real file into the local store so stat succeeds.
	data := []byte("widget binary content")
	req := testReq()
	req.Sha256 = sha256Of(data)
	ref := artifactRequestToRef(req)
	key := artifactKeyWithBuild(ref, req.BuildNumber)
	binKey := binaryStorageKey(key)
	_ = srv.localStorage.MkdirAll(ctx, artifactsDir, 0o755)
	_, _ = srv.localStorage.WriteFileAtomic(ctx, binKey, bytes.NewReader(data), req.Sha256, int64(len(data)))

	localSrc := &stubSource{
		name:      "local-posix",
		typ:       "LOCAL_POSIX",
		priority:  10,
		available: true,
		candidate: &ArtifactCandidate{
			SourceName: "local-posix",
			SourceType: "LOCAL_POSIX",
			LocalPath:  srv.localStorage.LocalPath(binKey),
			SizeBytes:  int64(len(data)),
			Authority:  true,
		},
	}
	upstreamSrc := &stubSource{
		name:      "upstream",
		typ:       "UPSTREAM",
		priority:  30,
		available: true,
	} // candidate nil → ErrArtifactNotFound — should never be consulted

	result, err := srv.resolveFromSources(ctx, req, []RepositorySource{localSrc, upstreamSrc}, permissivePolicy())
	if err != nil {
		t.Fatalf("expected hit, got error: %v", err)
	}
	if result.SourceName != "local-posix" {
		t.Errorf("expected local-posix source, got %q", result.SourceName)
	}
	if len(result.Diagnostics) != 1 {
		t.Errorf("expected 1 attempt (local), got %d", len(result.Diagnostics))
	}
	if result.Diagnostics[0].Status != "HIT" {
		t.Errorf("expected HIT, got %q", result.Diagnostics[0].Status)
	}
}

// ── Scenario B: local miss → upstream hit ────────────────────────────────────

func TestResolver_B_LocalMissThenUpstreamHit(t *testing.T) {
	srv := newResolverServer(t)
	ctx := context.Background()

	data := []byte("upstream artifact bytes")
	req := testReq()
	req.Sha256 = sha256Of(data)

	localSrc := &stubSource{
		name: "local-posix", typ: "LOCAL_POSIX", priority: 10, available: true,
		// nil candidate → ErrArtifactNotFound
	}
	upstreamSrc := &stubSource{
		name: "github", typ: "UPSTREAM", priority: 30, available: true,
		candidate: &ArtifactCandidate{
			SourceName: "github",
			SourceType: "UPSTREAM",
			Reader:     io.NopCloser(bytes.NewReader(data)),
			SizeBytes:  int64(len(data)),
			Sha256:     req.Sha256,
		},
	}

	result, err := srv.resolveFromSources(ctx, req, []RepositorySource{localSrc, upstreamSrc}, permissivePolicy())
	if err != nil {
		t.Fatalf("expected hit, got error: %v", err)
	}
	if result.SourceName != "github" {
		t.Errorf("expected github source, got %q", result.SourceName)
	}
	if result.LocalPath == "" {
		t.Error("expected non-empty LocalPath after materialization")
	}
	// File must exist on disk.
	if _, statErr := os.Stat(result.LocalPath); statErr != nil {
		t.Errorf("materialized file not on disk: %v", statErr)
	}
	// Diagnostics: local=MISS, upstream=HIT.
	if len(result.Diagnostics) != 2 {
		t.Fatalf("expected 2 diagnostics, got %d", len(result.Diagnostics))
	}
	if result.Diagnostics[0].Status != "MISS" {
		t.Errorf("local attempt: expected MISS, got %q", result.Diagnostics[0].Status)
	}
	if result.Diagnostics[1].Status != "HIT" {
		t.Errorf("upstream attempt: expected HIT, got %q", result.Diagnostics[1].Status)
	}
}

// ── Scenario C: checksum mismatch → continues to next source ─────────────────

func TestResolver_C_ChecksumMismatch(t *testing.T) {
	srv := newResolverServer(t)
	ctx := context.Background()

	good := []byte("correct bytes")
	bad := []byte("wrong bytes here!")
	req := testReq()
	req.Sha256 = sha256Of(good) // we expect good sha256

	badSrc := &stubSource{
		name: "bad-upstream", typ: "UPSTREAM", priority: 30, available: true,
		candidate: &ArtifactCandidate{
			SourceName: "bad-upstream",
			SourceType: "UPSTREAM",
			Reader:     io.NopCloser(bytes.NewReader(bad)), // wrong content
			Sha256:     req.Sha256,
		},
	}
	goodSrc := &stubSource{
		name: "good-upstream", typ: "UPSTREAM", priority: 40, available: true,
		candidate: &ArtifactCandidate{
			SourceName: "good-upstream",
			SourceType: "UPSTREAM",
			Reader:     io.NopCloser(bytes.NewReader(good)),
			Sha256:     req.Sha256,
		},
	}

	result, err := srv.resolveFromSources(ctx, req, []RepositorySource{badSrc, goodSrc}, permissivePolicy())
	if err != nil {
		t.Fatalf("expected eventual hit, got error: %v", err)
	}
	if result.SourceName != "good-upstream" {
		t.Errorf("expected good-upstream, got %q", result.SourceName)
	}
	if len(result.Diagnostics) != 2 {
		t.Fatalf("expected 2 diagnostics, got %d", len(result.Diagnostics))
	}
	if result.Diagnostics[0].Status != "CHECKSUM_MISMATCH" {
		t.Errorf("bad-upstream: expected CHECKSUM_MISMATCH, got %q", result.Diagnostics[0].Status)
	}
	if result.Diagnostics[1].Status != "HIT" {
		t.Errorf("good-upstream: expected HIT, got %q", result.Diagnostics[1].Status)
	}
}

// ── Scenario D: all sources miss ─────────────────────────────────────────────

func TestResolver_D_AllSourcesMiss(t *testing.T) {
	srv := newResolverServer(t)
	ctx := context.Background()
	req := testReq()

	sources := []RepositorySource{
		&stubSource{name: "local", typ: "LOCAL_POSIX", priority: 10, available: true},
		&stubSource{name: "upstream", typ: "UPSTREAM", priority: 30, available: true},
	}

	_, err := srv.resolveFromSources(ctx, req, sources, permissivePolicy())
	if err == nil {
		t.Fatal("expected error when all sources miss")
	}
	// Error should mention the artifact identity.
	if !containsSubstring(err.Error(), req.Name) {
		t.Errorf("error does not mention artifact name: %v", err)
	}
}

// ── Scenario E: RequireChecksum blocks non-local without sha256 ───────────────

func TestResolver_E_RequireChecksumBlocksNonLocal(t *testing.T) {
	srv := newResolverServer(t)
	ctx := context.Background()

	req := testReq()
	req.Sha256 = "" // no checksum

	policy := SourcePolicy{Enabled: true, RequireChecksum: true}

	localSrc := &stubSource{name: "local", typ: "LOCAL_POSIX", priority: 10, available: true}
	upstreamSrc := &stubSource{
		name: "upstream", typ: "UPSTREAM", priority: 30, available: true,
		candidate: &ArtifactCandidate{
			SourceName: "upstream", SourceType: "UPSTREAM",
			Reader: io.NopCloser(bytes.NewReader([]byte("data"))),
		},
	}

	_, err := srv.resolveFromSources(ctx, req, []RepositorySource{localSrc, upstreamSrc}, policy)
	if err == nil {
		t.Fatal("expected error: no sha256 + RequireChecksum should block")
	}
	// The upstream attempt should be UNAVAILABLE due to the checksum policy.
	// (local returns MISS, upstream is blocked by policy, so no hit possible)
}

// ── Scenario F: source unavailable ───────────────────────────────────────────

func TestResolver_F_SourceUnavailable(t *testing.T) {
	srv := newResolverServer(t)
	ctx := context.Background()

	data := []byte("bytes")
	req := testReq()
	req.Sha256 = sha256Of(data)

	downSrc := &stubSource{
		name: "flaky", typ: "UPSTREAM", priority: 30,
		available: false, healthMsg: "connection timeout",
	}
	goodSrc := &stubSource{
		name: "backup", typ: "UPSTREAM", priority: 40, available: true,
		candidate: &ArtifactCandidate{
			SourceName: "backup", SourceType: "UPSTREAM",
			Reader: io.NopCloser(bytes.NewReader(data)), Sha256: req.Sha256,
		},
	}

	result, err := srv.resolveFromSources(ctx, req, []RepositorySource{downSrc, goodSrc}, permissivePolicy())
	if err != nil {
		t.Fatalf("expected hit from backup source, got: %v", err)
	}
	if result.SourceName != "backup" {
		t.Errorf("expected backup source, got %q", result.SourceName)
	}
	if result.Diagnostics[0].Status != "UNAVAILABLE" {
		t.Errorf("flaky: expected UNAVAILABLE, got %q", result.Diagnostics[0].Status)
	}
}

// ── Scenario G: MaterializeArtifactToLocal fast path (LocalPath set) ─────────

func TestMaterialize_G_FastPathLocalPath(t *testing.T) {
	srv := newResolverServer(t)
	ctx := context.Background()

	data := []byte("preexisting binary")
	req := testReq()
	req.Sha256 = sha256Of(data)
	req.SizeBytes = int64(len(data))

	// Write the file into local CAS first.
	ref := artifactRequestToRef(req)
	key := artifactKeyWithBuild(ref, req.BuildNumber)
	binKey := binaryStorageKey(key)
	_ = srv.localStorage.MkdirAll(ctx, artifactsDir, 0o755)
	_, _ = srv.localStorage.WriteFileAtomic(ctx, binKey, bytes.NewReader(data), req.Sha256, req.SizeBytes)
	localPath := srv.localStorage.LocalPath(binKey)

	candidate := &ArtifactCandidate{
		SourceName: "local-posix",
		SourceType: "LOCAL_POSIX",
		LocalPath:  localPath,
		SizeBytes:  int64(len(data)),
		Authority:  true,
	}

	result, err := srv.MaterializeArtifactToLocal(ctx, req, candidate)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.LocalPath != localPath {
		t.Errorf("expected LocalPath=%q, got %q", localPath, result.LocalPath)
	}
	if result.SizeBytes != int64(len(data)) {
		t.Errorf("expected SizeBytes=%d, got %d", len(data), result.SizeBytes)
	}
}

// ── Scenario H: MaterializeArtifactToLocal streaming write ───────────────────

func TestMaterialize_H_StreamingWrite(t *testing.T) {
	srv := newResolverServer(t)
	ctx := context.Background()

	data := []byte("artifact content for streaming test")
	req := testReq()
	req.Sha256 = sha256Of(data)
	req.SizeBytes = int64(len(data))

	candidate := &ArtifactCandidate{
		SourceName: "upstream",
		SourceType: "UPSTREAM",
		Reader:     io.NopCloser(bytes.NewReader(data)),
		SizeBytes:  int64(len(data)),
		Sha256:     req.Sha256,
	}

	result, err := srv.MaterializeArtifactToLocal(ctx, req, candidate)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Sha256 != req.Sha256 {
		t.Errorf("sha256 mismatch: got %q", result.Sha256)
	}
	if result.SizeBytes != int64(len(data)) {
		t.Errorf("size mismatch: got %d", result.SizeBytes)
	}
	// File must exist on disk with correct content.
	written, readErr := os.ReadFile(result.LocalPath)
	if readErr != nil {
		t.Fatalf("cannot read materialized file: %v", readErr)
	}
	if !bytes.Equal(written, data) {
		t.Errorf("materialized content mismatch")
	}
}

// ── Scenario I: MaterializeArtifactToLocal checksum mismatch ─────────────────

func TestMaterialize_I_ChecksumMismatch(t *testing.T) {
	srv := newResolverServer(t)
	ctx := context.Background()

	expected := []byte("good content")
	actual := []byte("tampered!!!")
	req := testReq()
	req.Sha256 = sha256Of(expected)
	req.SizeBytes = int64(len(expected))

	candidate := &ArtifactCandidate{
		SourceName: "upstream",
		SourceType: "UPSTREAM",
		Reader:     io.NopCloser(bytes.NewReader(actual)), // wrong bytes
		Sha256:     req.Sha256,
	}

	_, err := srv.MaterializeArtifactToLocal(ctx, req, candidate)
	if !errors.Is(err, errChecksumMismatch) {
		t.Fatalf("expected errChecksumMismatch, got: %v", err)
	}

	// Temp file must not be left behind.
	ref := artifactRequestToRef(req)
	key := artifactKeyWithBuild(ref, req.BuildNumber)
	binKey := binaryStorageKey(key)
	_, statErr := srv.localStorage.Stat(ctx, binKey)
	if !errors.Is(statErr, fs.ErrNotExist) {
		t.Errorf("expected blob absent after mismatch, stat returned: %v", statErr)
	}
}
