package main

import (
	"context"
	"testing"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

// ── Conformance: full publish lifecycle ───────────────────────────────────────

func TestConformance_FullPublishCycle(t *testing.T) {
	srv := newTestServer(t)
	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io", Name: "echo", Version: "2.0.0",
		Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
	}

	// Step 1: upload artifact → state = VERIFIED.
	seedArtifactWithState(t, srv, ref, 0, repopb.PublishState_VERIFIED)

	// Step 2: read back → confirm VERIFIED.
	key := artifactKeyWithBuild(ref, 0)
	data, err := srv.Storage().ReadFile(context.Background(), manifestStorageKey(key))
	if err != nil {
		t.Fatalf("read after upload: %v", err)
	}
	_, state, err := unmarshalManifestWithState(data)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if state != repopb.PublishState_VERIFIED {
		t.Fatalf("expected VERIFIED after upload, got %v", state)
	}

	// Step 3: promote VERIFIED → PUBLISHED.
	resp, err := srv.promoteArtifactInternal(context.Background(), ref, 0, repopb.PublishState_PUBLISHED)
	if err != nil {
		t.Fatalf("promote to PUBLISHED: %v", err)
	}
	if resp.CurrentState != repopb.PublishState_PUBLISHED {
		t.Fatalf("expected PUBLISHED, got %v", resp.CurrentState)
	}

	// Step 4: read back again → confirm PUBLISHED persisted.
	data, _ = srv.Storage().ReadFile(context.Background(), manifestStorageKey(key))
	_, state, _ = unmarshalManifestWithState(data)
	if state != repopb.PublishState_PUBLISHED {
		t.Fatalf("expected PUBLISHED on re-read, got %v", state)
	}
}

func TestConformance_DescriptorFailure_ArtifactOrphaned(t *testing.T) {
	srv := newTestServer(t)
	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io", Name: "echo", Version: "3.0.0",
		Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
	}

	// Simulate: upload succeeded (VERIFIED), but descriptor failed → promote to ORPHANED.
	seedArtifactWithState(t, srv, ref, 0, repopb.PublishState_VERIFIED)

	resp, err := srv.promoteArtifactInternal(context.Background(), ref, 0, repopb.PublishState_ORPHANED)
	if err != nil {
		t.Fatalf("promote to ORPHANED: %v", err)
	}
	if resp.CurrentState != repopb.PublishState_ORPHANED {
		t.Fatalf("expected ORPHANED, got %v", resp.CurrentState)
	}

	// Orphaned artifact should NOT be promotable to PUBLISHED.
	_, err = srv.promoteArtifactInternal(context.Background(), ref, 0, repopb.PublishState_PUBLISHED)
	if err == nil {
		t.Fatal("expected error promoting ORPHANED → PUBLISHED")
	}
}

func TestConformance_IdempotentRepublish(t *testing.T) {
	srv := newTestServer(t)
	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io", Name: "echo", Version: "4.0.0",
		Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
	}

	// First publish cycle: VERIFIED → PUBLISHED.
	seedArtifactWithState(t, srv, ref, 0, repopb.PublishState_PUBLISHED)

	// Re-promoting PUBLISHED → PUBLISHED should be idempotent.
	resp, err := srv.promoteArtifactInternal(context.Background(), ref, 0, repopb.PublishState_PUBLISHED)
	if err != nil {
		t.Fatalf("idempotent promote: %v", err)
	}
	if !resp.Result {
		t.Error("expected result=true for idempotent promotion")
	}
	if resp.CurrentState != repopb.PublishState_PUBLISHED {
		t.Errorf("expected PUBLISHED, got %v", resp.CurrentState)
	}
}

func TestConformance_UploadFailure_NoManifest(t *testing.T) {
	srv := newTestServer(t)
	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io", Name: "never-uploaded", Version: "1.0.0",
		Platform: "linux_amd64",
	}

	// No seedArtifactWithState → artifact was never uploaded.
	// PromoteArtifact should fail with NotFound.
	_, err := srv.promoteArtifactInternal(context.Background(), ref, 0, repopb.PublishState_PUBLISHED)
	if err == nil {
		t.Fatal("expected error when artifact was never uploaded")
	}
}

func TestConformance_FailStateFromAny(t *testing.T) {
	srv := newTestServer(t)

	states := []repopb.PublishState{
		repopb.PublishState_STAGING,
		repopb.PublishState_VERIFIED,
		repopb.PublishState_PUBLISHED,
		repopb.PublishState_ORPHANED,
	}

	for _, from := range states {
		ref := &repopb.ArtifactRef{
			PublisherId: "core@globular.io", Name: "fail-test", Version: from.String(),
			Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
		}
		seedArtifactWithState(t, srv, ref, 0, from)

		resp, err := srv.promoteArtifactInternal(context.Background(), ref, 0, repopb.PublishState_FAILED)
		if err != nil {
			t.Errorf("promote %v → FAILED: %v", from, err)
			continue
		}
		if resp.CurrentState != repopb.PublishState_FAILED {
			t.Errorf("from %v: expected FAILED, got %v", from, resp.CurrentState)
		}
	}
}
