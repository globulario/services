package main

// publish_state_authority_test.go — Regression tests for the publish-state
// split-brain fix (Day-1 convergence failure, 2026-04-24).
//
// Invariant: publish_state Scylla column is the SOLE authority for current
// lifecycle state. manifest_json embedded state is immutable historical metadata
// and MUST NOT override the column in any read path.
//
// These tests use fakeLedger to inject explicit column/json disagreement so we
// can prove the column wins in every public API path.

import (
	"context"
	"testing"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

// buildSplitBrainLedger returns a (fakeLedger, key) where:
//   - manifest_json has state = jsonState (written at upload time)
//   - publish_state column has state = colState (written by UpdatePublishState)
func buildSplitBrainLedger(t *testing.T, jsonState, colState repopb.PublishState) (*fakeLedger, string) {
	t.Helper()
	ref := &repopb.ArtifactRef{
		PublisherId: "glob", Name: "echo", Version: "1.0.0",
		Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
	}
	m := &repopb.ArtifactManifest{Ref: ref, Checksum: "deadbeef"}
	mjson, err := marshalManifestWithState(m, jsonState)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	key := artifactKeyWithBuild(ref, 1)
	row := manifestRow{
		ArtifactKey:  key,
		ManifestJSON: mjson,
		PublishState: colState.String(), // authoritative column differs from json
		PublisherID:  ref.GetPublisherId(),
		Name:         ref.GetName(),
		Version:      ref.GetVersion(),
		Platform:     ref.GetPlatform(),
		BuildNumber:  1,
		Checksum:     "deadbeef",
	}
	fl := newFakeLedger()
	fl.rows[key] = &row
	return fl, key
}

// buildServerWithLedger creates a test server wired to the given ledger plus
// a MinIO fake that has the manifest file with jsonState embedded.
func buildServerWithLedger(t *testing.T, fl *fakeLedger, key string, jsonState, colState repopb.PublishState) *server {
	t.Helper()
	srv := newTestServer(t)
	srv.scylla = fl
	srv.listCache = newListCache(fl)

	// Also write the manifest to the MinIO-like storage so the fallback path
	// and direct-read paths find it.
	ctx := context.Background()
	ref := &repopb.ArtifactRef{
		PublisherId: "glob", Name: "echo", Version: "1.0.0",
		Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
	}
	m := &repopb.ArtifactManifest{Ref: ref, Checksum: "deadbeef"}
	mjson, _ := marshalManifestWithState(m, jsonState)
	_ = srv.Storage().MkdirAll(ctx, artifactsDir, 0o755)
	_ = srv.Storage().WriteFile(ctx, manifestStorageKey(key), mjson, 0o644)
	_ = srv.Storage().WriteFile(ctx, binaryStorageKey(key), []byte("blob"), 0o644)
	return srv
}

// ── Test 1: readManifestAndStateByKey uses publish_state column ──────────────

func TestReadManifestUsesPublishStateColumn(t *testing.T) {
	fl, key := buildSplitBrainLedger(t, repopb.PublishState_VERIFIED, repopb.PublishState_PUBLISHED)
	srv := buildServerWithLedger(t, fl, key, repopb.PublishState_VERIFIED, repopb.PublishState_PUBLISHED)

	_, state, m, err := srv.readManifestAndStateByKey(context.Background(), key)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if state != repopb.PublishState_PUBLISHED {
		t.Errorf("state: got %v, want PUBLISHED (column must win over json VERIFIED)", state)
	}
	// Manifest proto field must be stamped too.
	if m.GetPublishState() != repopb.PublishState_PUBLISHED {
		t.Errorf("m.PublishState: got %v, want PUBLISHED", m.GetPublishState())
	}
}

// ── Test 2: GetArtifactVersions uses publish_state column ───────────────────

func TestGetArtifactVersionsUsesPublishStateColumn(t *testing.T) {
	fl, key := buildSplitBrainLedger(t, repopb.PublishState_VERIFIED, repopb.PublishState_PUBLISHED)
	srv := buildServerWithLedger(t, fl, key, repopb.PublishState_VERIFIED, repopb.PublishState_PUBLISHED)

	resp, err := srv.GetArtifactVersions(context.Background(), &repopb.GetArtifactVersionsRequest{
		Name: "echo", PublisherId: "glob",
	})
	if err != nil {
		t.Fatalf("GetArtifactVersions: %v", err)
	}
	if len(resp.GetVersions()) == 0 {
		t.Fatal("expected at least one version")
	}
	got := resp.GetVersions()[0].GetPublishState()
	if got != repopb.PublishState_PUBLISHED {
		t.Errorf("version publish_state: got %v, want PUBLISHED", got)
	}
}

// ── Test 3: ListArtifacts uses publish_state column ──────────────────────────

func TestListArtifactsUsesPublishStateColumn(t *testing.T) {
	fl, key := buildSplitBrainLedger(t, repopb.PublishState_VERIFIED, repopb.PublishState_PUBLISHED)
	srv := buildServerWithLedger(t, fl, key, repopb.PublishState_VERIFIED, repopb.PublishState_PUBLISHED)

	resp, err := srv.ListArtifacts(context.Background(), &repopb.ListArtifactsRequest{})
	if err != nil {
		t.Fatalf("ListArtifacts: %v", err)
	}
	if len(resp.GetArtifacts()) == 0 {
		t.Fatal("expected artifacts in list")
	}
	got := resp.GetArtifacts()[0].GetPublishState()
	if got != repopb.PublishState_PUBLISHED {
		t.Errorf("artifact publish_state in list: got %v, want PUBLISHED", got)
	}
}

// ── Test 4: release resolver candidate filter sees PUBLISHED ─────────────────
// Simulates the controller's getLatestPublished pattern: ListArtifacts then
// filter by a.GetPublishState().

func TestReleaseResolverFindsPublishedDespiteStaleManifestJson(t *testing.T) {
	fl, key := buildSplitBrainLedger(t, repopb.PublishState_VERIFIED, repopb.PublishState_PUBLISHED)
	srv := buildServerWithLedger(t, fl, key, repopb.PublishState_VERIFIED, repopb.PublishState_PUBLISHED)

	resp, err := srv.ListArtifacts(context.Background(), &repopb.ListArtifactsRequest{})
	if err != nil {
		t.Fatalf("ListArtifacts: %v", err)
	}

	var found bool
	for _, a := range resp.GetArtifacts() {
		ps := a.GetPublishState()
		if ps == repopb.PublishState_PUBLISHED || ps == repopb.PublishState_PUBLISH_STATE_UNSPECIFIED {
			if a.GetRef().GetName() == "echo" {
				found = true
			}
		}
	}
	if !found {
		t.Error("release resolver simulation: echo not found as PUBLISHED candidate — controller would report 'no published artifact found'")
	}
}

// ── Test 5: legacy rows (empty publish_state column) fall back to json state ─

func TestPublishStateFallbackForLegacyRows(t *testing.T) {
	// Column is empty — behaves like a pre-column row. State must come from JSON.
	fl, key := buildSplitBrainLedger(t, repopb.PublishState_PUBLISHED, repopb.PublishState_PUBLISH_STATE_UNSPECIFIED)
	// Override column to be empty string (legacy row).
	fl.rows[key].PublishState = ""

	srv := buildServerWithLedger(t, fl, key, repopb.PublishState_PUBLISHED, repopb.PublishState_PUBLISH_STATE_UNSPECIFIED)

	_, state, m, err := srv.readManifestAndStateByKey(context.Background(), key)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if state != repopb.PublishState_PUBLISHED {
		t.Errorf("legacy fallback: got %v, want PUBLISHED (from json)", state)
	}
	if m.GetPublishState() != repopb.PublishState_PUBLISHED {
		t.Errorf("legacy fallback m.PublishState: got %v, want PUBLISHED", m.GetPublishState())
	}
}

// ── Test 6: column wins even when json says PUBLISHED and column says YANKED ─

func TestPublishStateColumnWinsOverManifestJson(t *testing.T) {
	fl, key := buildSplitBrainLedger(t, repopb.PublishState_PUBLISHED, repopb.PublishState_YANKED)
	srv := buildServerWithLedger(t, fl, key, repopb.PublishState_PUBLISHED, repopb.PublishState_YANKED)

	_, state, m, err := srv.readManifestAndStateByKey(context.Background(), key)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if state != repopb.PublishState_YANKED {
		t.Errorf("column should win: got %v, want YANKED", state)
	}
	if m.GetPublishState() != repopb.PublishState_YANKED {
		t.Errorf("m.PublishState should be YANKED, got %v", m.GetPublishState())
	}
}

// ── Test 7: promotion visible without manifest_json rewrite ──────────────────
// Simulates: upload → VERIFIED in both → UpdatePublishState(PUBLISHED) column only
// → read must return PUBLISHED.

func TestPromotionVisibleWithoutManifestRewrite(t *testing.T) {
	ctx := context.Background()
	ref := &repopb.ArtifactRef{
		PublisherId: "glob", Name: "echo", Version: "1.0.0",
		Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
	}
	m := &repopb.ArtifactManifest{Ref: ref, Checksum: "deadbeef"}
	mjson, _ := marshalManifestWithState(m, repopb.PublishState_VERIFIED)
	key := artifactKeyWithBuild(ref, 1)

	// Ledger row: manifest_json has VERIFIED, column also starts VERIFIED.
	fl := newFakeLedger()
	fl.rows[key] = &manifestRow{
		ArtifactKey:  key,
		ManifestJSON: mjson,
		PublishState: repopb.PublishState_VERIFIED.String(),
		Name:         "echo",
	}

	srv := newTestServer(t)
	srv.scylla = fl
	srv.listCache = newListCache(fl)
	_ = srv.Storage().MkdirAll(ctx, artifactsDir, 0o755)
	_ = srv.Storage().WriteFile(ctx, manifestStorageKey(key), mjson, 0o644)
	_ = srv.Storage().WriteFile(ctx, binaryStorageKey(key), []byte("blob"), 0o644)

	// Simulate UpdatePublishState only (no manifest_json rewrite).
	_ = fl.UpdatePublishState(ctx, key, repopb.PublishState_PUBLISHED.String())

	// All read paths must now return PUBLISHED.
	_, state, manifest, err := srv.readManifestAndStateByKey(ctx, key)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if state != repopb.PublishState_PUBLISHED {
		t.Errorf("after UpdatePublishState only: got %v, want PUBLISHED", state)
	}
	if manifest.GetPublishState() != repopb.PublishState_PUBLISHED {
		t.Errorf("m.PublishState after UpdatePublishState: got %v, want PUBLISHED", manifest.GetPublishState())
	}

	// ListArtifacts must also reflect PUBLISHED.
	listResp, err := srv.ListArtifacts(ctx, &repopb.ListArtifactsRequest{})
	if err != nil {
		t.Fatalf("ListArtifacts: %v", err)
	}
	if len(listResp.GetArtifacts()) == 0 {
		t.Fatal("artifact not visible in list after promotion")
	}
	if listResp.GetArtifacts()[0].GetPublishState() != repopb.PublishState_PUBLISHED {
		t.Errorf("ListArtifacts state after column-only promotion: got %v, want PUBLISHED",
			listResp.GetArtifacts()[0].GetPublishState())
	}
}

// ── Test 8: backfillPublishState repairs empty-column rows ──────────────────

func TestBackfillPublishStateRepairsEmptyColumn(t *testing.T) {
	ctx := context.Background()
	ref := &repopb.ArtifactRef{
		PublisherId: "glob", Name: "echo", Version: "1.0.0",
		Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
	}
	m := &repopb.ArtifactManifest{Ref: ref, Checksum: "deadbeef"}
	mjson, _ := marshalManifestWithState(m, repopb.PublishState_PUBLISHED)
	key := artifactKeyWithBuild(ref, 1)

	fl := newFakeLedger()
	fl.rows[key] = &manifestRow{
		ArtifactKey:  key,
		ManifestJSON: mjson,
		PublishState: "", // legacy empty column
		Name:         "echo",
	}

	srv := newTestServer(t)
	srv.scylla = fl

	result := srv.backfillPublishState(ctx)
	if result.RowsBackfilled != 1 {
		t.Errorf("expected 1 backfilled, got %d", result.RowsBackfilled)
	}
	// Column should now be PUBLISHED.
	row, err := fl.GetManifest(ctx, key)
	if err != nil {
		t.Fatalf("get after backfill: %v", err)
	}
	if row.PublishState != repopb.PublishState_PUBLISHED.String() {
		t.Errorf("column after backfill: got %q, want PUBLISHED", row.PublishState)
	}
}

// fakeLedger and helpers are declared in storage_hardening_test.go.
