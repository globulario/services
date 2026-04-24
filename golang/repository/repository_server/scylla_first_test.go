package main

// scylla_first_test.go — Tests proving all discovery paths use Scylla even
// when MinIO is empty. Acceptance criteria for the Scylla-first architecture.
//
// Every test uses a non-nil fakeLedger (simulating Scylla available) and an
// empty in-memory storage backend (simulating MinIO empty / not yet synced).
// The invariant: PUBLISHED artifacts must be discoverable from Scylla alone.

import (
	"context"
	"errors"
	"testing"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

// ── helpers ───────────────────────────────────────────────────────────────

// buildScyllaOnlyServer creates a server with:
//   - fakeLedger containing one artifact (column = state)
//   - manifest_json always embeds VERIFIED (split-brain scenario)
//   - empty MinIO storage (nothing in artifactsDir)
func buildScyllaOnlyServer(t *testing.T, ref *repopb.ArtifactRef, buildNum int64, state repopb.PublishState) (*server, string) {
	t.Helper()
	m := &repopb.ArtifactManifest{
		Ref:         ref,
		Checksum:    "deadbeef01deadbeef01deadbeef01deadbeef01deadbeef01deadbeef01dead",
		BuildNumber: buildNum,
	}
	mjson, err := marshalManifestWithState(m, repopb.PublishState_VERIFIED) // json always VERIFIED
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	key := artifactKeyWithBuild(ref, buildNum)

	fl := newFakeLedger()
	fl.rows[key] = &manifestRow{
		ArtifactKey:  key,
		ManifestJSON: mjson,
		PublishState: state.String(), // authoritative column
		PublisherID:  ref.GetPublisherId(),
		Name:         ref.GetName(),
		Version:      ref.GetVersion(),
		Platform:     ref.GetPlatform(),
		BuildNumber:  buildNum,
		Checksum:     "deadbeef01deadbeef01deadbeef01deadbeef01deadbeef01deadbeef01dead",
	}

	srv := newTestServer(t)
	srv.scylla = fl
	// Storage intentionally left empty — no MinIO files written.
	return srv, key
}

// ── Test 1: GetArtifactVersions uses Scylla when MinIO is empty ──────────

func TestGetArtifactVersionsUsesScyllaWhenMinioEmpty(t *testing.T) {
	ref := &repopb.ArtifactRef{
		PublisherId: "glob", Name: "gateway", Version: "1.0.65",
		Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
	}
	srv, _ := buildScyllaOnlyServer(t, ref, 3, repopb.PublishState_PUBLISHED)

	resp, err := srv.GetArtifactVersions(context.Background(), &repopb.GetArtifactVersionsRequest{
		Name: "gateway", PublisherId: "glob",
	})
	if err != nil {
		t.Fatalf("GetArtifactVersions: %v", err)
	}
	if len(resp.GetVersions()) == 0 {
		t.Fatal("expected version from Scylla, MinIO was empty")
	}
	got := resp.GetVersions()[0].GetPublishState()
	if got != repopb.PublishState_PUBLISHED {
		t.Errorf("version publish_state: got %v, want PUBLISHED", got)
	}
}

// ── Test 2: ListArtifacts uses Scylla when MinIO is empty ────────────────

func TestListArtifactsUsesScyllaWhenMinioEmpty(t *testing.T) {
	ref := &repopb.ArtifactRef{
		PublisherId: "glob", Name: "dns", Version: "1.0.65",
		Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
	}
	srv, _ := buildScyllaOnlyServer(t, ref, 2, repopb.PublishState_PUBLISHED)

	resp, err := srv.ListArtifacts(context.Background(), &repopb.ListArtifactsRequest{})
	if err != nil {
		t.Fatalf("ListArtifacts: %v", err)
	}
	if len(resp.GetArtifacts()) == 0 {
		t.Fatal("expected artifact from Scylla, MinIO was empty")
	}
	got := resp.GetArtifacts()[0].GetPublishState()
	if got != repopb.PublishState_PUBLISHED {
		t.Errorf("artifact publish_state in list: got %v, want PUBLISHED", got)
	}
}

// ── Test 3: resolveLatestBuildNumber uses Scylla not MinIO ───────────────

func TestResolveLatestBuildNumberUsesScyllaNotMinio(t *testing.T) {
	ref := &repopb.ArtifactRef{
		PublisherId: "glob", Name: "echo", Version: "1.0.65",
		Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
	}
	srv, _ := buildScyllaOnlyServer(t, ref, 5, repopb.PublishState_PUBLISHED)

	got := srv.resolveLatestBuildNumber(context.Background(), ref)
	if got != 5 {
		t.Errorf("resolveLatestBuildNumber: got %d, want 5 (from Scylla, MinIO empty)", got)
	}
}

// ── Test 4: promoteToPublished fails if Scylla state update fails ────────

func TestPromoteToPublishedFailsIfScyllaStateUpdateFails(t *testing.T) {
	ref := &repopb.ArtifactRef{
		PublisherId: "glob", Name: "echo", Version: "1.0.65",
		Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
	}
	srv := newTestServer(t)

	// Wire an error ledger that always fails UpdatePublishState.
	srv.scylla = errLedger{err: errors.New("scylla write failed")}

	// Write the binary blob so the Stat check passes.
	key := artifactKeyWithBuild(ref, 1)
	ctx := context.Background()
	_ = srv.Storage().MkdirAll(ctx, artifactsDir, 0o755)
	_ = srv.Storage().WriteFile(ctx, binaryStorageKey(key), []byte("fake-binary"), 0o644)

	m := &repopb.ArtifactManifest{Ref: ref, Checksum: "abc", SizeBytes: 0}
	err := srv.promoteToPublished(ctx, key, m)
	if err == nil {
		t.Fatal("expected promoteToPublished to fail when Scylla UpdatePublishState fails")
	}
}

// ── Test 5: release resolver sees PUBLISHED with empty MinIO listing ─────
// Simulates the controller's getLatestPublished pattern:
// call GetArtifactVersions, then filter by a.GetPublishState() == PUBLISHED.

func TestReleaseResolverSeesPublishedWithEmptyMinioListing(t *testing.T) {
	ref := &repopb.ArtifactRef{
		PublisherId: "glob", Name: "node_agent", Version: "1.0.65",
		Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
	}
	srv, _ := buildScyllaOnlyServer(t, ref, 4, repopb.PublishState_PUBLISHED)

	resp, err := srv.GetArtifactVersions(context.Background(), &repopb.GetArtifactVersionsRequest{
		Name: "node_agent", PublisherId: "glob",
	})
	if err != nil {
		t.Fatalf("GetArtifactVersions: %v", err)
	}

	var found bool
	for _, v := range resp.GetVersions() {
		if v.GetPublishState() == repopb.PublishState_PUBLISHED {
			found = true
			break
		}
	}
	if !found {
		t.Error("release resolver: node_agent not found as PUBLISHED candidate — MinIO was empty, Scylla must be the source")
	}
}
