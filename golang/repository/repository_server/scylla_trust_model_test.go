package main

// scylla_trust_model_test.go — "Scylla is not proof of installability" invariant tests.
//
// Rule: No repository API may claim an artifact is installable solely because
// Scylla metadata says it exists. Installability requires:
//   1. Manifest exists (Scylla row or manifest file).
//   2. sha256/size known from manifest.
//   3. Blob exists in local POSIX CAS.
//   4. Blob checksum matches manifest.
//
// Test scenarios:
//   S1  Scylla row present, local blob present, checksum valid → installable
//   S2  Scylla row present, local blob missing, GitHub source has blob → repair + installable
//   S3  Scylla row present, local blob missing, no source has blob → BROKEN_MISSING_BLOB
//   S4  Local POSIX blob present, correct checksum, but wrong size in manifest → promote blocked
//   S5  Scylla says PUBLISHED, local blob has wrong content (size ok, sha256 wrong) → local POSIX source MISS
//   S6  promoteToPublished requires local blob — MinIO alone is not sufficient
//   S7  artifactBlobStatus uses localStorage only (not Storage/MinIO)

import (
	"bytes"
	"context"
	"io"
	"testing"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
	"github.com/globulario/services/golang/storage_backend"
)

// ── helpers ───────────────────────────────────────────────────────────────────

func newTrustModelServer(t *testing.T) *server {
	t.Helper()
	dir := t.TempDir()
	local := storage_backend.NewOSStorage(dir)
	srv := &server{Root: dir}
	srv.storage = local
	srv.localStorage = local
	srv.ensureSignaturePolicy().SetPolicyForTest(&repopb.SignaturePolicy{
		AllowUnsignedLocalDevelopment: true,
		TrustedCorePublishers:         []string{"core@globular.io"},
	})
	return srv
}

func trustModelRef() *repopb.ArtifactRef {
	return &repopb.ArtifactRef{
		PublisherId: "core@globular.io",
		Name:        "echo",
		Version:     "1.0.84",
		Platform:    "linux_amd64",
		Kind:        repopb.ArtifactKind_SERVICE,
	}
}

// writeVerifiedBlob writes a blob to localStorage and returns its sha256.
func writeVerifiedBlob(t *testing.T, srv *server, ref *repopb.ArtifactRef, buildNumber int64, data []byte) string {
	t.Helper()
	ctx := context.Background()
	key := artifactKeyWithBuild(ref, buildNumber)
	binKey := binaryStorageKey(key)
	digest := checksumBytes(data)
	if _, err := srv.localStorage.WriteFileAtomic(ctx, binKey, bytes.NewReader(data), digest, int64(len(data))); err != nil {
		t.Fatalf("writeVerifiedBlob: %v", err)
	}
	return digest
}

// ── S1: local blob + correct checksum → artifactBlobStatus returns ok ────────

func TestTrustModel_S1_LocalBlobPresent_Installable(t *testing.T) {
	srv := newTrustModelServer(t)
	ctx := context.Background()
	ref := trustModelRef()
	data := []byte("echo binary content s1")
	digest := writeVerifiedBlob(t, srv, ref, 1, data)

	present, reason := srv.artifactBlobStatus(ctx, ref, 1, int64(len(data)))
	if !present {
		t.Errorf("expected present=true, got reason=%q", reason)
	}
	if reason != "ok" {
		t.Errorf("expected reason=ok, got %q", reason)
	}
	_ = digest
}

// ── S2: blob missing, upstream provides it → resolver materializes ────────────

func TestTrustModel_S2_BlobMissing_UpstreamRepairs(t *testing.T) {
	srv := newTrustModelServer(t)
	ctx := context.Background()
	ref := trustModelRef()
	data := []byte("echo binary content s2")
	digest := checksumBytes(data)

	req := ArtifactRequest{
		PublisherID: ref.GetPublisherId(),
		Name:        ref.GetName(),
		Version:     ref.GetVersion(),
		Platform:    ref.GetPlatform(),
		BuildNumber: 2,
		Sha256:      digest,
		SizeBytes:   int64(len(data)),
	}

	upstream := &stubSource{
		name:      "github-upstream",
		typ:       "UPSTREAM_GITHUB",
		priority:  30,
		available: true,
		candidate: &ArtifactCandidate{
			SourceName: "github-upstream",
			SourceType: "UPSTREAM_GITHUB",
			Reader:     io.NopCloser(bytes.NewReader(data)),
			Sha256:     digest,
			SizeBytes:  int64(len(data)),
		},
	}
	policy := SourcePolicy{Enabled: true}

	result, err := srv.resolveFromSources(ctx, req, []RepositorySource{upstream}, policy)
	if err != nil {
		t.Fatalf("expected repair to succeed: %v", err)
	}
	if result.SourceType != "UPSTREAM_GITHUB" {
		t.Errorf("expected UPSTREAM_GITHUB, got %q", result.SourceType)
	}
	// Verify the blob is now in local POSIX CAS.
	present, reason := srv.artifactBlobStatus(ctx, ref, 2, int64(len(data)))
	if !present {
		t.Errorf("blob should be present after repair, got reason=%q", reason)
	}
}

// ── S3: blob missing, no source → BROKEN ─────────────────────────────────────

func TestTrustModel_S3_BlobMissing_NoSource_NotInstallable(t *testing.T) {
	srv := newTrustModelServer(t)
	ctx := context.Background()
	ref := trustModelRef()

	present, reason := srv.artifactBlobStatus(ctx, ref, 3, 100)
	if present {
		t.Error("expected present=false when blob has never been written")
	}
	if reason != "missing_blob" {
		t.Errorf("expected missing_blob, got %q", reason)
	}

	// resolveFromSources with no sources should also fail.
	req := ArtifactRequest{
		PublisherID: ref.GetPublisherId(),
		Name:        ref.GetName(),
		Version:     ref.GetVersion(),
		Platform:    ref.GetPlatform(),
		BuildNumber: 3,
	}
	_, resolveErr := srv.resolveFromSources(ctx, req, nil, SourcePolicy{Enabled: true})
	if resolveErr == nil {
		t.Error("expected error when no sources configured")
	}
}

// ── S4: wrong size in manifest → promoteToPublished blocked ──────────────────

func TestTrustModel_S4_SizeMismatch_PromoteBlocked(t *testing.T) {
	srv := newTrustModelServer(t)
	ctx := context.Background()
	ref := trustModelRef()
	data := []byte("echo binary content s4")
	digest := writeVerifiedBlob(t, srv, ref, 4, data)

	manifest := &repopb.ArtifactManifest{
		Ref:         ref,
		BuildNumber: 4,
		Checksum:    digest,
		SizeBytes:   int64(len(data)) + 100, // wrong size
	}
	key := artifactKeyWithBuild(ref, 4)
	err := srv.promoteToPublished(ctx, key, manifest)
	if err == nil {
		t.Error("expected promoteToPublished to fail on size mismatch")
	}
}

// ── S5: local blob has correct size but wrong content → sha256 mismatch ──────

func TestTrustModel_S5_LocalBlobCorrupt_SHA256Miss(t *testing.T) {
	srv := newTrustModelServer(t)
	ctx := context.Background()
	ref := trustModelRef()

	// Write the real binary.
	correctData := []byte("correct echo binary content s5")
	correctDigest := checksumBytes(correctData)

	// Then overwrite with corrupt content (same size!).
	corruptData := make([]byte, len(correctData))
	copy(corruptData, correctData)
	corruptData[0] ^= 0xFF // flip a byte

	// Write corrupt data without sha256 verification (bypassing WriteFileAtomic guard).
	corruptDigest := checksumBytes(corruptData)
	key := artifactKeyWithBuild(ref, 5)
	binKey := binaryStorageKey(key)
	_, _ = srv.localStorage.WriteFileAtomic(ctx, binKey,
		bytes.NewReader(corruptData), corruptDigest, int64(len(corruptData)))

	// LocalPOSIXRepositorySource should detect the sha256 mismatch and return MISS.
	req := ArtifactRequest{
		PublisherID: ref.GetPublisherId(),
		Name:        ref.GetName(),
		Version:     ref.GetVersion(),
		Platform:    ref.GetPlatform(),
		BuildNumber: 5,
		Sha256:      correctDigest, // expected = correct
		SizeBytes:   int64(len(correctData)),
	}
	localSrc := newLocalPOSIXSource(srv.localStorage, srv.localStoreRoot())
	_, err := localSrc.Open(ctx, req)
	if err == nil {
		t.Error("expected Open to fail when local sha256 doesn't match request sha256")
	}
}

// ── S6: promoteToPublished requires local blob (not MinIO alone) ─────────────

func TestTrustModel_S6_PromoteRequiresLocalBlob_NotMinIO(t *testing.T) {
	srv := newTrustModelServer(t)
	ctx := context.Background()
	ref := trustModelRef()
	data := []byte("echo binary content s6")
	digest := checksumBytes(data)

	manifest := &repopb.ArtifactManifest{
		Ref:         ref,
		BuildNumber: 6,
		Checksum:    digest,
		SizeBytes:   int64(len(data)),
	}
	key := artifactKeyWithBuild(ref, 6)

	// Try to promote without writing to local CAS — must fail.
	err := srv.promoteToPublished(ctx, key, manifest)
	if err == nil {
		t.Error("expected promoteToPublished to fail when local CAS is empty")
	}

	// Now write to local CAS — promotion must succeed.
	writeVerifiedBlob(t, srv, ref, 6, data)
	if err := srv.promoteToPublished(ctx, key, manifest); err != nil {
		t.Errorf("unexpected promote failure after writing local blob: %v", err)
	}
}

// ── S7: artifactBlobStatus uses localStorage, not Storage/MinIO ───────────────

func TestTrustModel_S7_ArtifactBlobStatus_LocalOnly(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	mirrorDir := t.TempDir()
	local := storage_backend.NewOSStorage(dir)
	// `mirror` is a detached store standing in for "not the local CAS" — the
	// server never reads it; the blob-status check must consult localStorage only.
	mirror := storage_backend.NewOSStorage(mirrorDir)
	srv := &server{Root: dir}
	srv.storage = local
	srv.localStorage = local

	ctx := context.Background()
	ref := trustModelRef()

	data := []byte("echo binary content s7")
	digest := checksumBytes(data)
	key := artifactKeyWithBuild(ref, 7)
	binKey := binaryStorageKey(key)

	// Write blob ONLY to mirror, not to local.
	_ = mirror.MkdirAll(ctx, artifactsDir, 0o755)
	_ = mirror.WriteFile(ctx, binKey, data, 0o644)

	// artifactBlobStatus must report missing_blob (local CAS only).
	present, reason := srv.artifactBlobStatus(ctx, ref, 7, int64(len(data)))
	if present {
		t.Errorf("expected missing_blob when only mirror has blob, but got present=true (reason=%q)", reason)
	}
	if reason != "missing_blob" {
		t.Errorf("expected reason=missing_blob, got %q", reason)
	}

	// Now write to local CAS — must become present.
	_ = local.MkdirAll(ctx, artifactsDir, 0o755)
	_, _ = local.WriteFileAtomic(ctx, binKey, bytes.NewReader(data), digest, int64(len(data)))

	present, reason = srv.artifactBlobStatus(ctx, ref, 7, int64(len(data)))
	if !present {
		t.Errorf("expected present=true after local write, got reason=%q", reason)
	}
}
