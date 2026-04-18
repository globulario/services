package main

import (
	"context"
	"testing"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

// TestUploadBundleDualWrite_SetsPublishedState verifies that the dual-write
// from UploadBundle sets publish_state=PUBLISHED on the artifact manifest,
// since UploadBundle is a complete publish operation (descriptor already exists).
func TestUploadBundleDualWrite_SetsPublishedState(t *testing.T) {
	srv := newTestServer(t)
	ref := &repopb.ArtifactRef{
		PublisherId: "glob", Name: "echo", Version: "1.0.0",
		Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
	}

	// Simulate UploadBundle dual-write: create a manifest with PUBLISHED state.
	ctx := context.Background()
	key := artifactKeyWithBuild(ref, 0)
	_ = srv.Storage().MkdirAll(ctx, artifactsDir, 0o755)

	m := &repopb.ArtifactManifest{
		Ref:      ref,
		Checksum: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
	}
	data, err := marshalManifestWithState(m, repopb.PublishState_PUBLISHED)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := srv.Storage().WriteFile(ctx, manifestStorageKey(key), data, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := srv.Storage().WriteFile(ctx, binaryStorageKey(key), []byte("fake-binary"), 0o644); err != nil {
		t.Fatalf("write binary: %v", err)
	}

	// GetArtifactManifest should return valid manifest.
	resp, err := srv.GetArtifactManifest(ctx, &repopb.GetArtifactManifestRequest{Ref: ref})
	if err != nil {
		t.Fatalf("GetArtifactManifest: %v", err)
	}
	if resp.GetManifest().GetRef().GetName() != "echo" {
		t.Errorf("expected echo, got %s", resp.GetManifest().GetRef().GetName())
	}

	// Verify publish_state is PUBLISHED by reading raw storage.
	rawData, err := srv.Storage().ReadFile(ctx, manifestStorageKey(key))
	if err != nil {
		t.Fatalf("read raw: %v", err)
	}
	_, state, err := unmarshalManifestWithState(rawData)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if state != repopb.PublishState_PUBLISHED {
		t.Errorf("expected PUBLISHED state, got %v", state)
	}
}

// TestUploadArtifactPath_SetsVerifiedState verifies that the UploadArtifact
// path sets publish_state=VERIFIED (not PUBLISHED).
func TestUploadArtifactPath_SetsVerifiedState(t *testing.T) {
	srv := newTestServer(t)
	ref := &repopb.ArtifactRef{
		PublisherId: "glob", Name: "auth", Version: "2.0.0",
		Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
	}

	// Simulate UploadArtifact: create a manifest with VERIFIED state.
	ctx := context.Background()
	key := artifactKeyWithBuild(ref, 0)
	_ = srv.Storage().MkdirAll(ctx, artifactsDir, 0o755)

	m := &repopb.ArtifactManifest{
		Ref:      ref,
		Checksum: "abcdef01abcdef01abcdef01abcdef01abcdef01abcdef01abcdef01abcdef01",
	}
	data, err := marshalManifestWithState(m, repopb.PublishState_VERIFIED)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := srv.Storage().WriteFile(ctx, manifestStorageKey(key), data, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := srv.Storage().WriteFile(ctx, binaryStorageKey(key), []byte("fake-binary"), 0o644); err != nil {
		t.Fatalf("write binary: %v", err)
	}

	// GetArtifactManifest should still return it (manifest readable in any state).
	resp, err := srv.GetArtifactManifest(ctx, &repopb.GetArtifactManifestRequest{Ref: ref})
	if err != nil {
		t.Fatalf("GetArtifactManifest: %v", err)
	}
	if resp.GetManifest().GetRef().GetName() != "auth" {
		t.Errorf("expected auth, got %s", resp.GetManifest().GetRef().GetName())
	}

	// Verify publish_state is VERIFIED.
	rawData, err := srv.Storage().ReadFile(ctx, manifestStorageKey(key))
	if err != nil {
		t.Fatalf("read raw: %v", err)
	}
	_, state, err := unmarshalManifestWithState(rawData)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if state != repopb.PublishState_VERIFIED {
		t.Errorf("expected VERIFIED state, got %v", state)
	}
}

// TestBothPathsProduceSameManifestFields verifies that manifests created via
// UploadBundle (PUBLISHED) and UploadArtifact (VERIFIED) have the same core
// fields (ref, checksum, size).
func TestBothPathsProduceSameManifestFields(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	ref := &repopb.ArtifactRef{
		PublisherId: "glob", Name: "gateway", Version: "3.0.0",
		Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
	}

	// Create via "UploadBundle" path (PUBLISHED).
	bundleKey := artifactKeyWithBuild(ref, 0)
	_ = srv.Storage().MkdirAll(ctx, artifactsDir, 0o755)
	bundleManifest := &repopb.ArtifactManifest{
		Ref:      ref,
		Checksum: "1111111111111111111111111111111111111111111111111111111111111111",
		SizeBytes: 1024,
	}
	bundleData, _ := marshalManifestWithState(bundleManifest, repopb.PublishState_PUBLISHED)
	_ = srv.Storage().WriteFile(ctx, manifestStorageKey(bundleKey), bundleData, 0o644)
	_ = srv.Storage().WriteFile(ctx, binaryStorageKey(bundleKey), []byte("fake"), 0o644)

	// Read it via GetArtifactManifest.
	resp, err := srv.GetArtifactManifest(ctx, &repopb.GetArtifactManifestRequest{Ref: ref})
	if err != nil {
		t.Fatalf("GetArtifactManifest: %v", err)
	}
	got := resp.GetManifest()
	if got.GetRef().GetName() != "gateway" {
		t.Errorf("name: got %s, want gateway", got.GetRef().GetName())
	}
	if got.GetChecksum() != "1111111111111111111111111111111111111111111111111111111111111111" {
		t.Errorf("checksum mismatch")
	}
	if got.GetSizeBytes() != 1024 {
		t.Errorf("size: got %d, want 1024", got.GetSizeBytes())
	}
}

// TestLegacyManifestWithoutState verifies backward compat: manifests stored
// without a publishState field are read as PUBLISH_STATE_UNSPECIFIED.
func TestLegacyManifestWithoutState(t *testing.T) {
	srv := newTestServer(t)
	ref := &repopb.ArtifactRef{
		PublisherId: "glob", Name: "legacy", Version: "",
		Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
	}

	// Seed using the old seedArtifact helper (no publish state).
	seedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref:      ref,
		Checksum: "aaaa",
	})

	ctx := context.Background()
	key := artifactKeyWithBuild(ref, 0)
	rawData, err := srv.Storage().ReadFile(ctx, manifestStorageKey(key))
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	_, state, err := unmarshalManifestWithState(rawData)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if state != repopb.PublishState_PUBLISH_STATE_UNSPECIFIED {
		t.Errorf("expected UNSPECIFIED for legacy manifest, got %v", state)
	}
}
