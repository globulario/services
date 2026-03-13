package main

import (
	"context"
	"testing"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

// ── marshalManifestWithState / unmarshalManifestWithState ────────────────────

func TestMarshalUnmarshalPublishState(t *testing.T) {
	m := &repopb.ArtifactManifest{
		Ref:      &repopb.ArtifactRef{PublisherId: "glob", Name: "echo", Version: "1.0.0", Platform: "linux_amd64"},
		Checksum: "abc123",
	}

	tests := []struct {
		name  string
		state repopb.PublishState
	}{
		{"unspecified", repopb.PublishState_PUBLISH_STATE_UNSPECIFIED},
		{"staging", repopb.PublishState_STAGING},
		{"verified", repopb.PublishState_VERIFIED},
		{"published", repopb.PublishState_PUBLISHED},
		{"failed", repopb.PublishState_FAILED},
		{"orphaned", repopb.PublishState_ORPHANED},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := marshalManifestWithState(m, tt.state)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
			_, gotState, err := unmarshalManifestWithState(data)
			if err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if gotState != tt.state {
				t.Errorf("state round-trip: got %v, want %v", gotState, tt.state)
			}
		})
	}
}

// ── UploadArtifact → state is VERIFIED ──────────────────────────────────────

func TestUploadArtifact_SetsVerifiedState(t *testing.T) {
	srv := newTestServer(t)
	ref := &repopb.ArtifactRef{
		PublisherId: "glob", Name: "echo", Version: "1.0.0",
		Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
	}

	// Seed an artifact via the storage layer (simulating what UploadArtifact does).
	ctx := context.Background()
	key := artifactKeyWithBuild(ref, 0)
	_ = srv.Storage().MkdirAll(ctx, artifactsDir, 0o755)

	manifest := &repopb.ArtifactManifest{
		Ref:      ref,
		Checksum: "deadbeef",
	}
	data, err := marshalManifestWithState(manifest, repopb.PublishState_VERIFIED)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := srv.Storage().WriteFile(ctx, manifestStorageKey(key), data, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// Read back and verify state.
	readData, err := srv.Storage().ReadFile(ctx, manifestStorageKey(key))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	_, state, err := unmarshalManifestWithState(readData)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if state != repopb.PublishState_VERIFIED {
		t.Errorf("expected VERIFIED, got %v", state)
	}
}

// ── PromoteArtifact ─────────────────────────────────────────────────────────

func TestPromoteArtifact_VerifiedToPublished(t *testing.T) {
	srv := newTestServer(t)
	ref := &repopb.ArtifactRef{
		PublisherId: "glob", Name: "echo", Version: "1.0.0",
		Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
	}
	seedArtifactWithState(t, srv, ref, 0, repopb.PublishState_VERIFIED)

	resp, err := srv.promoteArtifactInternal(context.Background(), ref, 0, repopb.PublishState_PUBLISHED)
	if err != nil {
		t.Fatalf("promote: %v", err)
	}
	if !resp.Result {
		t.Error("expected result=true")
	}
	if resp.PreviousState != repopb.PublishState_VERIFIED {
		t.Errorf("expected previous=VERIFIED, got %v", resp.PreviousState)
	}
	if resp.CurrentState != repopb.PublishState_PUBLISHED {
		t.Errorf("expected current=PUBLISHED, got %v", resp.CurrentState)
	}
}

func TestPromoteArtifact_PublishedIdempotent(t *testing.T) {
	srv := newTestServer(t)
	ref := &repopb.ArtifactRef{
		PublisherId: "glob", Name: "echo", Version: "1.0.0",
		Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
	}
	seedArtifactWithState(t, srv, ref, 0, repopb.PublishState_PUBLISHED)

	resp, err := srv.promoteArtifactInternal(context.Background(), ref, 0, repopb.PublishState_PUBLISHED)
	if err != nil {
		t.Fatalf("promote: %v", err)
	}
	if !resp.Result {
		t.Error("expected result=true for idempotent promotion")
	}
}

func TestPromoteArtifact_VerifiedToOrphaned(t *testing.T) {
	srv := newTestServer(t)
	ref := &repopb.ArtifactRef{
		PublisherId: "glob", Name: "echo", Version: "1.0.0",
		Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
	}
	seedArtifactWithState(t, srv, ref, 0, repopb.PublishState_VERIFIED)

	resp, err := srv.promoteArtifactInternal(context.Background(), ref, 0, repopb.PublishState_ORPHANED)
	if err != nil {
		t.Fatalf("promote: %v", err)
	}
	if resp.CurrentState != repopb.PublishState_ORPHANED {
		t.Errorf("expected ORPHANED, got %v", resp.CurrentState)
	}
}

func TestPromoteArtifact_InvalidTransition(t *testing.T) {
	srv := newTestServer(t)
	ref := &repopb.ArtifactRef{
		PublisherId: "glob", Name: "echo", Version: "1.0.0",
		Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
	}
	seedArtifactWithState(t, srv, ref, 0, repopb.PublishState_ORPHANED)

	// ORPHANED → PUBLISHED should fail.
	_, err := srv.promoteArtifactInternal(context.Background(), ref, 0, repopb.PublishState_PUBLISHED)
	if err == nil {
		t.Fatal("expected error for invalid transition ORPHANED→PUBLISHED")
	}
}

func TestPromoteArtifact_NotFound(t *testing.T) {
	srv := newTestServer(t)
	ref := &repopb.ArtifactRef{
		PublisherId: "glob", Name: "nonexistent", Version: "1.0.0",
		Platform: "linux_amd64",
	}
	_, err := srv.promoteArtifactInternal(context.Background(), ref, 0, repopb.PublishState_PUBLISHED)
	if err == nil {
		t.Fatal("expected NotFound error")
	}
}

func TestPromoteArtifact_AnyStateCanFail(t *testing.T) {
	srv := newTestServer(t)
	ref := &repopb.ArtifactRef{
		PublisherId: "glob", Name: "echo", Version: "1.0.0",
		Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
	}
	seedArtifactWithState(t, srv, ref, 0, repopb.PublishState_PUBLISHED)

	// PUBLISHED → FAILED should succeed (any state can fail).
	resp, err := srv.promoteArtifactInternal(context.Background(), ref, 0, repopb.PublishState_FAILED)
	if err != nil {
		t.Fatalf("promote to FAILED: %v", err)
	}
	if resp.CurrentState != repopb.PublishState_FAILED {
		t.Errorf("expected FAILED, got %v", resp.CurrentState)
	}
}

// ── ValidPromoteTransition ──────────────────────────────────────────────────

func TestValidPromoteTransition(t *testing.T) {
	tests := []struct {
		from, to repopb.PublishState
		valid    bool
	}{
		{repopb.PublishState_PUBLISH_STATE_UNSPECIFIED, repopb.PublishState_VERIFIED, true},
		{repopb.PublishState_STAGING, repopb.PublishState_VERIFIED, true},
		{repopb.PublishState_VERIFIED, repopb.PublishState_PUBLISHED, true},
		{repopb.PublishState_VERIFIED, repopb.PublishState_ORPHANED, true},
		{repopb.PublishState_PUBLISHED, repopb.PublishState_PUBLISHED, true},
		{repopb.PublishState_ORPHANED, repopb.PublishState_PUBLISHED, false},
		{repopb.PublishState_PUBLISHED, repopb.PublishState_VERIFIED, false},
		// Any → FAILED is always valid.
		{repopb.PublishState_VERIFIED, repopb.PublishState_FAILED, true},
		{repopb.PublishState_PUBLISHED, repopb.PublishState_FAILED, true},
		{repopb.PublishState_ORPHANED, repopb.PublishState_FAILED, true},
	}

	for _, tt := range tests {
		got := repopb.ValidPromoteTransition(tt.from, tt.to)
		if got != tt.valid {
			t.Errorf("ValidPromoteTransition(%s, %s) = %v, want %v", tt.from, tt.to, got, tt.valid)
		}
	}
}

// ── test helpers ─────────────────────────────────────────────────────────────

// seedArtifactWithState writes a manifest with an explicit publish state.
func seedArtifactWithState(t *testing.T, srv *server, ref *repopb.ArtifactRef, buildNumber int64, state repopb.PublishState) {
	t.Helper()
	ctx := context.Background()
	key := artifactKeyWithBuild(ref, buildNumber)
	_ = srv.Storage().MkdirAll(ctx, artifactsDir, 0o755)
	m := &repopb.ArtifactManifest{
		Ref:      ref,
		Checksum: "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef",
	}
	data, err := marshalManifestWithState(m, state)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	if err := srv.Storage().WriteFile(ctx, manifestStorageKey(key), data, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := srv.Storage().WriteFile(ctx, binaryStorageKey(key), []byte("fake-binary"), 0o644); err != nil {
		t.Fatalf("write binary: %v", err)
	}
}
