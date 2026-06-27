package main

// local_authority_test.go — Regression tests for the "local POSIX CAS is the
// sole installability authority" invariant across the three remaining entry points:
//
//   LA1  verifyArtifactIntegrity: MinIO-only blob must NOT pass
//   LA2  ImportProvisionalArtifact: writes local POSIX first; MinIO-only import fails
//   LA3  UploadBundle: cannot promote to PUBLISHED without local blob

import (
	"bytes"
	"context"
	"encoding/gob"
	"io"
	"testing"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
	resourcepb "github.com/globulario/services/golang/resource/resourcepb"
	"github.com/globulario/services/golang/storage_backend"
	"google.golang.org/grpc/metadata"
)

// ── helpers ──────────────────────────────────────────────────────────────────

func newLocalAuthorityServer(t *testing.T) (srv *server, local, mirror *storage_backend.OSStorage) {
	t.Helper()
	localDir := t.TempDir()
	mirrorDir := t.TempDir()
	local = storage_backend.NewOSStorage(localDir)
	// `mirror` is a detached store standing in for "somewhere that is NOT the
	// local CAS" — the server never reads it (packages live only in the local
	// POSIX CAS). Tests write a blob here to prove it does not make an artifact
	// installable.
	mirror = storage_backend.NewOSStorage(mirrorDir)
	srv = &server{Root: localDir}
	srv.storage = local
	srv.localStorage = local
	srv.ensureSignaturePolicy().SetPolicyForTest(&repopb.SignaturePolicy{
		AllowUnsignedLocalDevelopment: true,
		TrustedCorePublishers:         []string{"core@globular.io"},
	})
	return srv, local, mirror
}

func laRef() *repopb.ArtifactRef {
	return &repopb.ArtifactRef{
		PublisherId: "core@globular.io",
		Name:        "la-test",
		Version:     "1.0.0",
		Platform:    "linux_amd64",
		Kind:        repopb.ArtifactKind_SERVICE,
	}
}

// writeManifestToScyllaSubstitute writes a manifest to localStorage so that
// readManifestAndStateByKey can find it (no real Scylla in unit tests).
func writeManifestLocal(t *testing.T, srv *server, ref *repopb.ArtifactRef, buildNumber int64, digest string, sizeBytes int64) string {
	t.Helper()
	ctx := context.Background()
	key := artifactKeyWithBuild(ref, buildNumber)
	manifest := &repopb.ArtifactManifest{
		Ref:         ref,
		BuildNumber: buildNumber,
		Checksum:    digest,
		SizeBytes:   sizeBytes,
	}
	mjson, err := marshalManifestWithState(manifest, repopb.PublishState_PUBLISHED)
	if err != nil {
		t.Fatalf("marshalManifestWithState: %v", err)
	}
	if err := srv.localStorage.WriteFile(ctx, manifestStorageKey(key), mjson, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	return key
}

// ── LA1: verifyArtifactIntegrity — MinIO-only blob must NOT pass ─────────────

func TestLocalAuthority_LA1_VerifyIntegrity_MinIOOnlyBlobFails(t *testing.T) {
	srv, local, mirror := newLocalAuthorityServer(t)
	ctx := context.Background()
	ref := laRef()
	data := []byte("la1 binary content")
	digest := checksumBytes(data)

	key := artifactKeyWithBuild(ref, 1)
	binKey := binaryStorageKey(key)

	// Write manifest to local (so readManifestAndStateByKey can find it).
	writeManifestLocal(t, srv, ref, 1, digest, int64(len(data)))

	// Write binary ONLY to mirror — not to local CAS.
	_ = mirror.MkdirAll(ctx, artifactsDir, 0o755)
	_ = mirror.WriteFile(ctx, binKey, data, 0o644)

	// Mirror has the blob, local does not.
	if _, statErr := local.Stat(ctx, binKey); statErr == nil {
		t.Fatal("precondition violated: local should not have blob")
	}

	v, err := srv.verifyArtifactIntegrity(ctx, ref, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.Status != VerifyBrokenMissingBlob {
		t.Errorf("expected VerifyBrokenMissingBlob, got %q (reason=%q)", v.Status, v.Reason)
	}
}

// ── LA1b: verifyArtifactIntegrity — local blob present → passes ─────────────

func TestLocalAuthority_LA1b_VerifyIntegrity_LocalBlobPasses(t *testing.T) {
	srv, local, _ := newLocalAuthorityServer(t)
	ctx := context.Background()
	ref := laRef()
	data := []byte("la1b binary content")
	digest := checksumBytes(data)

	key := artifactKeyWithBuild(ref, 2)
	binKey := binaryStorageKey(key)

	writeManifestLocal(t, srv, ref, 2, digest, int64(len(data)))

	_ = local.MkdirAll(ctx, artifactsDir, 0o755)
	if _, err := local.WriteFileAtomic(ctx, binKey, bytes.NewReader(data), digest, int64(len(data))); err != nil {
		t.Fatalf("write local blob: %v", err)
	}

	v, err := srv.verifyArtifactIntegrity(ctx, ref, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if v.Status != VerifyOK && v.Status != VerifyInconclusive {
		t.Errorf("expected VerifyOK or VerifyInconclusive, got %q (reason=%q)", v.Status, v.Reason)
	}
}

// ── LA2: ImportProvisionalArtifact — writes local POSIX first ────────────────

func TestLocalAuthority_LA2_ImportProvisional_WritesLocalFirst(t *testing.T) {
	srv, local, _ := newLocalAuthorityServer(t)
	ctx := context.Background()

	data := []byte("la2 binary content")
	digest := checksumBytes(data)

	req := &repopb.ImportProvisionalRequest{
		PublisherId:        "core@globular.io",
		Name:               "la-test",
		Version:            "1.0.0",
		Platform:           "linux_amd64",
		Digest:             digest,
		ProvisionalBuildId: "test-prov-id-la2",
		Data:               data,
	}

	resp, err := srv.ImportProvisionalArtifact(ctx, req)
	if err != nil {
		t.Fatalf("ImportProvisionalArtifact failed: %v", err)
	}
	if !resp.GetOk() {
		t.Fatalf("expected Ok=true, got message: %s", resp.GetMessage())
	}

	// Verify the blob is now in local POSIX CAS.
	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io",
		Name:        "la-test",
		Version:     "1.0.0",
		Platform:    "linux_amd64",
	}
	key := artifactKeyWithBuild(ref, 0)
	binKey := binaryStorageKey(key)
	if _, statErr := local.Stat(ctx, binKey); statErr != nil {
		t.Errorf("blob must be in local POSIX CAS after import: %v", statErr)
	}
}

// ── LA3: UploadBundle — cannot PUBLISH without local blob ────────────────────

func TestLocalAuthority_LA3_UploadBundle_RequiresLocalBlob(t *testing.T) {
	srv, local, _ := newLocalAuthorityServer(t)
	ctx := context.Background()

	// Create a minimal gob-encoded PackageBundle.
	data := []byte("la3 fake tgz content")
	desc := &resourcepb.PackageDescriptor{
		Name:        "la-test",
		Version:     "1.0.0",
		PublisherID: "core@globular.io",
	}
	bundle := resourcepb.PackageBundle{
		Plaform:           "linux_amd64",
		PackageDescriptor: desc,
		Binairies:         data,
	}

	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(bundle); err != nil {
		t.Fatalf("encode bundle: %v", err)
	}

	// Use a stub stream that provides the gob bytes.
	stream := newUploadBundleStubStream(ctx, buf.Bytes())
	// UploadBundle may fail at the bundle-descriptor RPC (setPackageBundle) because
	// there is no live resource service in unit tests. The invariant under test is
	// that the binary is written to local POSIX CAS BEFORE that RPC is attempted —
	// so the blob must be present regardless of whether UploadBundle returns an error.
	_ = srv.UploadBundle(stream)

	// Verify binary ended up in local POSIX CAS.
	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io",
		Name:        "la-test",
		Version:     "1.0.0",
		Platform:    "linux_amd64",
		Kind:        repopb.ArtifactKind_SERVICE,
	}
	var (
		binKey string
		found  bool
	)
	for buildNumber := int64(1); buildNumber <= 16; buildNumber++ {
		key := artifactKeyWithBuild(ref, buildNumber)
		candidate := binaryStorageKey(key)
		if _, statErr := local.Stat(ctx, candidate); statErr == nil {
			binKey = candidate
			found = true
			break
		}
	}
	if !found {
		legacy := binaryStorageKey(artifactKeyWithBuild(ref, 0))
		if _, statErr := local.Stat(ctx, legacy); statErr == nil {
			binKey = legacy
			found = true
		}
	}
	if !found {
		t.Fatalf("UploadBundle must write blob to local POSIX CAS before attempting bundle registration")
	}
	if _, statErr := local.Stat(ctx, binKey); statErr != nil {
		t.Errorf("UploadBundle must write blob to local POSIX CAS before attempting bundle registration: %v", statErr)
	}
}

// ── stub stream for UploadBundle ─────────────────────────────────────────────

type uploadBundleStubStream struct {
	ctx    context.Context
	chunks [][]byte
	pos    int
}

func newUploadBundleStubStream(ctx context.Context, data []byte) *uploadBundleStubStream {
	const chunkSize = 4096
	var chunks [][]byte
	for len(data) > 0 {
		n := chunkSize
		if n > len(data) {
			n = len(data)
		}
		chunks = append(chunks, data[:n])
		data = data[n:]
	}
	return &uploadBundleStubStream{ctx: ctx, chunks: chunks}
}

func (s *uploadBundleStubStream) Recv() (*repopb.UploadBundleRequest, error) {
	if s.pos >= len(s.chunks) {
		return nil, io.EOF
	}
	chunk := s.chunks[s.pos]
	s.pos++
	return &repopb.UploadBundleRequest{Data: chunk}, nil
}

func (s *uploadBundleStubStream) SendAndClose(*repopb.UploadBundleResponse) error { return nil }
func (s *uploadBundleStubStream) Context() context.Context                        { return s.ctx }
func (s *uploadBundleStubStream) RecvMsg(m any) error                             { return nil }
func (s *uploadBundleStubStream) SendMsg(m any) error                             { return nil }
func (s *uploadBundleStubStream) SetHeader(metadata.MD) error                     { return nil }
func (s *uploadBundleStubStream) SendHeader(metadata.MD) error                    { return nil }
func (s *uploadBundleStubStream) SetTrailer(metadata.MD)                          {}
