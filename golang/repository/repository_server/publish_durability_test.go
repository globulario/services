package main

// publish_durability_test.go — promote-to-PUBLISHED must leave the blob durably
// recoverable cluster-wide, not only in the seeding node's local CAS.
//
// A PUBLISHED artifact is a completion obligation: every repository instance
// must be able to obtain the blob. Instances cache-fill their local POSIX CAS
// from the shared MinIO mirror, so the blob MUST be in the mirror. The upload's
// mirror write is best-effort; ensureMirrorDurability closes the race by
// re-pushing the locally-verified blob to the mirror before PUBLISHED. With no
// upstream_import (true for seeded infra packages like etcd) this is the only
// recovery path — a mirror-miss would strand the artifact permanently.

import (
	"context"
	"testing"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
	"github.com/globulario/services/golang/storage_backend"
)

func TestEnsureMirrorDurability_PushesLocalOnlyBlobToMirror(t *testing.T) {
	ctx := context.Background()
	mirror := storage_backend.NewOSStorage(t.TempDir())
	srv := newTestServerWithMirror(t, mirror)

	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io", Name: "etcd",
		Version: "3.5.14", Platform: "linux_amd64", Kind: repopb.ArtifactKind_INFRASTRUCTURE,
	}
	key := artifactKeyWithBuild(ref, 1)
	blobKey := binaryStorageKey(key)

	// Simulate the best-effort mirror write at upload time having failed: the
	// blob is in this instance's local CAS only.
	if err := srv.localStorage.MkdirAll(ctx, artifactsDir, 0o755); err != nil {
		t.Fatalf("mkdir local: %v", err)
	}
	if err := srv.localStorage.WriteFile(ctx, blobKey, []byte("etcd-blob-bytes"), 0o644); err != nil {
		t.Fatalf("write local blob: %v", err)
	}
	if _, err := srv.mirrorStorage.Stat(ctx, blobKey); err == nil {
		t.Fatal("precondition: mirror must NOT have the blob yet")
	}

	srv.ensureMirrorDurability(ctx, key)

	// The blob must now be durably recoverable from the shared mirror.
	if _, err := srv.mirrorStorage.Stat(ctx, blobKey); err != nil {
		t.Fatalf("blob must be durable in mirror after ensure: %v", err)
	}
	got, err := srv.mirrorStorage.ReadFile(ctx, blobKey)
	if err != nil {
		t.Fatalf("read mirror blob: %v", err)
	}
	if string(got) != "etcd-blob-bytes" {
		t.Errorf("mirror blob bytes mismatch: got %q", string(got))
	}
}

func TestEnsureMirrorDurability_NoMirrorIsNoOp(t *testing.T) {
	// Single-node / mirror-optional mode: mirrorStorage is nil. Must not panic
	// and must not block (respects repository.minio_independence).
	srv := newTestServer(t) // mirrorStorage left nil
	srv.ensureMirrorDurability(context.Background(),
		"core@globular.io%etcd%3.5.14%linux_amd64%1")
}

func TestEnsureMirrorDurability_AlreadyInMirrorIsIdempotent(t *testing.T) {
	ctx := context.Background()
	mirror := storage_backend.NewOSStorage(t.TempDir())
	srv := newTestServerWithMirror(t, mirror)

	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io", Name: "etcd",
		Version: "3.5.14", Platform: "linux_amd64", Kind: repopb.ArtifactKind_INFRASTRUCTURE,
	}
	key := artifactKeyWithBuild(ref, 1)
	blobKey := binaryStorageKey(key)

	// Blob already durable in both local and mirror (the common, healthy case).
	for _, st := range []storage_backend.Storage{srv.localStorage, srv.mirrorStorage} {
		_ = st.MkdirAll(ctx, artifactsDir, 0o755)
		if err := st.WriteFile(ctx, blobKey, []byte("etcd-blob-bytes"), 0o644); err != nil {
			t.Fatalf("seed blob: %v", err)
		}
	}

	srv.ensureMirrorDurability(ctx, key) // must be a fast, harmless no-op

	if _, err := srv.mirrorStorage.Stat(ctx, blobKey); err != nil {
		t.Fatalf("mirror blob must remain present: %v", err)
	}
}
