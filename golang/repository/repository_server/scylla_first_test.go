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
	"io"
	"strings"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"

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
	srv.listCache = newListCache(fl)
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

// ── Test 6: promoteToPublished sets artifact_state=PUBLISHED ─────────────
// Regression test for the BLOB_VERIFIED stuck-artifact bug.
//
// Root cause: promoteToPublished called UpdatePublishState (publish_state column)
// but never called transitionArtifactState (artifact_state column). An artifact
// with publish_state=PUBLISHED but artifact_state=BLOB_VERIFIED was treated as
// DesiredBuildIdOrphaned by the node-agent, causing an infinite install-retry
// storm that blocked all convergence cluster-wide.

func TestPromoteToPublished_SetsArtifactStatePublished(t *testing.T) {
	ref := &repopb.ArtifactRef{
		PublisherId: "glob", Name: "echo", Version: "1.0.66",
		Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
	}
	srv, ledger := newLedgerTestServer(t)
	ctx := context.Background()

	key := artifactKeyWithBuild(ref, 1)
	binary := []byte("fake-echo-binary")

	// Seed Scylla row (as upload would — publish_state=VERIFIED, artifact_state
	// unset). syncManifestToScylla writes manifest_json on upload, so a real
	// pre-promote row is never a skeleton; include it or promoteToPublished's
	// skeleton-row guard refuses promotion.
	_ = ledger.PutManifest(ctx, manifestRow{
		ArtifactKey:  key,
		ManifestJSON: []byte("{}"),
		PublishState: repopb.PublishState_VERIFIED.String(),
		PublisherID:  ref.GetPublisherId(),
		Name:         ref.GetName(),
		Version:      ref.GetVersion(),
		Platform:     ref.GetPlatform(),
		BuildNumber:  1,
	})

	// Write the binary blob so promoteToPublished's Stat check passes.
	_ = srv.localStorage.MkdirAll(ctx, artifactsDir, 0o755)
	_ = srv.localStorage.WriteFile(ctx, binaryStorageKey(key), binary, 0o644)

	manifest := &repopb.ArtifactManifest{
		Ref:         ref,
		BuildNumber: 1,
		SizeBytes:   int64(len(binary)),
	}

	if err := srv.promoteToPublished(ctx, key, manifest); err != nil {
		t.Fatalf("promoteToPublished: %v", err)
	}

	// publish_state column must be PUBLISHED.
	row, _ := ledger.GetManifest(ctx, key)
	if row == nil {
		t.Fatal("manifest row not found in ledger")
	}
	if row.PublishState != repopb.PublishState_PUBLISHED.String() {
		t.Errorf("publish_state: got %q, want %q", row.PublishState, repopb.PublishState_PUBLISHED.String())
	}

	// artifact_state must also be PUBLISHED — this is the regression guard.
	// Before the fix, this column stayed at BLOB_VERIFIED (or empty), causing
	// the node-agent's resolveArtifactByBuildID to return DesiredBuildIdOrphaned.
	got := srv.readArtifactState(ctx, key)
	if got != PipelinePublished {
		t.Errorf("artifact_state after promoteToPublished: got %q, want %q — "+
			"node-agent will treat this artifact as DesiredBuildIdOrphaned and loop forever",
			got, PipelinePublished)
	}
}

// ── Test 8: orphan check does not disagree with discovery ────────────────────
// Regression test for the join-order temporal split bug.
//
// After a full promoteToPublished, both the discovery path (publish_state column,
// used by ListArtifacts / buildIDIndexFromManifests) and the install gate
// (artifact_state column, used by isRowInstallable / resolveByBuildID) must agree
// that the artifact is installable. A split-state row (pub=PUBLISHED, art=BLOB_VERIFIED)
// would cause discovery to succeed and the orphan check to fail — producing an
// infinite install-retry storm that blocks all subsequent node joins.
func TestOrphanCheck_DoesNotDisagreeWithDiscovery(t *testing.T) {
	ref := &repopb.ArtifactRef{
		PublisherId: "glob", Name: "workflow", Version: "1.0.66",
		Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
	}
	srv, ledger := newLedgerTestServer(t)
	ctx := context.Background()

	key := artifactKeyWithBuild(ref, 7)
	binary := []byte("fake-workflow-binary")

	// Seed a VERIFIED row (as upload would — manifest_json present, so the
	// pre-promote row is not a skeleton and promoteToPublished's guard allows it).
	_ = ledger.PutManifest(ctx, manifestRow{
		ArtifactKey:  key,
		ManifestJSON: []byte("{}"),
		PublishState: repopb.PublishState_VERIFIED.String(),
		PublisherID:  ref.GetPublisherId(),
		Name:         ref.GetName(),
		Version:      ref.GetVersion(),
		Platform:     ref.GetPlatform(),
		BuildNumber:  7,
	})

	// Write the binary blob.
	_ = srv.localStorage.MkdirAll(ctx, artifactsDir, 0o755)
	_ = srv.localStorage.WriteFile(ctx, binaryStorageKey(key), binary, 0o644)

	manifest := &repopb.ArtifactManifest{
		Ref: ref, BuildNumber: 7, SizeBytes: int64(len(binary)),
	}

	if err := srv.promoteToPublished(ctx, key, manifest); err != nil {
		t.Fatalf("promoteToPublished: %v", err)
	}

	// Discovery path: publish_state must be PUBLISHED.
	row, _ := ledger.GetManifest(ctx, key)
	if row == nil {
		t.Fatal("ledger row not found")
	}
	if row.PublishState != repopb.PublishState_PUBLISHED.String() {
		t.Errorf("discovery (publish_state): got %q, want PUBLISHED", row.PublishState)
	}

	// Install gate: artifact_state must also be PUBLISHED.
	// If it disagrees with publish_state, isRowInstallable rejects the row →
	// resolveByBuildID returns DesiredBuildIdOrphaned → infinite retry storm.
	got := srv.readArtifactState(ctx, key)
	if got != PipelinePublished {
		t.Errorf("install gate (artifact_state): got %q, want PUBLISHED — "+
			"discovery and orphan check disagree; subsequent node joins will partially fail",
			got)
	}

	// Confirm isRowInstallable agrees with both columns.
	if !isRowInstallable(row) {
		t.Errorf("isRowInstallable returned false after promoteToPublished — "+
			"publish_state=%q artifact_state=%q; node-agent will treat this as DesiredBuildIdOrphaned",
			row.PublishState, row.ArtifactState)
	}
}

// TestPromoteToPublished_RefusesSkeletonRow is the F1 regression for INC-2026-0012:
// a row whose manifest_json never landed (interrupted syncManifestToScylla) must
// NOT be promoted to PUBLISHED. Otherwise the publish_state / artifact_state
// UPSERTs in promoteToPublished manufacture a row that reads PUBLISHED with a NULL
// manifest — a half-done row readers can never resolve.
// (repository.artifact_presence_requires_metadata_and_blob / meta.half_done_must_not_look_done.)
func TestPromoteToPublished_RefusesSkeletonRow(t *testing.T) {
	ref := &repopb.ArtifactRef{
		PublisherId: "glob", Name: "echo", Version: "1.0.66",
		Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
	}
	srv, ledger := newLedgerTestServer(t)
	ctx := context.Background()

	key := artifactKeyWithBuild(ref, 1)
	binary := []byte("fake-echo-binary")

	// Seed a SKELETON row: state columns present, manifest_json ABSENT — the exact
	// INC-2026-0012 shape produced when the manifest sync failed but later state
	// UPSERTs succeeded.
	_ = ledger.PutManifest(ctx, manifestRow{
		ArtifactKey:  key,
		PublishState: repopb.PublishState_VERIFIED.String(),
		PublisherID:  ref.GetPublisherId(),
		Name:         ref.GetName(),
		Version:      ref.GetVersion(),
		Platform:     ref.GetPlatform(),
		BuildNumber:  1,
		// ManifestJSON intentionally omitted → skeleton row.
	})

	// Blob IS present, so the rejection is specifically the manifest guard, not
	// the blob-presence check earlier in promoteToPublished.
	_ = srv.localStorage.MkdirAll(ctx, artifactsDir, 0o755)
	_ = srv.localStorage.WriteFile(ctx, binaryStorageKey(key), binary, 0o644)

	manifest := &repopb.ArtifactManifest{Ref: ref, BuildNumber: 1, SizeBytes: int64(len(binary))}

	err := srv.promoteToPublished(ctx, key, manifest)
	if err == nil {
		t.Fatal("promoteToPublished must refuse to promote a skeleton row (manifest_json absent)")
	}
	if !strings.Contains(err.Error(), "skeleton row") {
		t.Errorf("expected skeleton-row rejection, got: %v", err)
	}

	// The row must NOT have been advanced to PUBLISHED.
	row, _ := ledger.GetManifest(ctx, key)
	if row != nil && row.PublishState == repopb.PublishState_PUBLISHED.String() {
		t.Error("skeleton row was promoted to PUBLISHED — F1 guard failed")
	}
}

// TestCompletePublish_RefusesHardDepsWhenCatalogUnavailable is the F4 regression:
// when the published catalog can't be read (Scylla degraded → loadPublishedCatalog
// returns nil), the two cross-artifact law rules (acyclic hard_deps / no
// dep-on-application) cannot run. For an artifact that DECLARES hard_deps those
// rules are the gate, so promotion must be refused rather than degraded open.
// (repository.publish_pipeline_is_ordered / meta.failure_response_must_contract_not_amplify.)
func TestCompletePublish_RefusesHardDepsWhenCatalogUnavailable(t *testing.T) {
	srv, _ := newLedgerTestServer(t)
	// Swap in a ledger whose ListManifests errors → loadPublishedCatalog == nil.
	srv.scylla = errLedger{err: errors.New("scylla unavailable")}
	ctx := context.Background()

	manifest := &repopb.ArtifactManifest{
		Ref: &repopb.ArtifactRef{
			PublisherId: "glob", Name: "echo", Version: "1.0.0",
			Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
		},
		BuildNumber: 1,
		HardDeps:    []*repopb.ArtifactDependencyRef{{}}, // declares a hard dep
	}
	key := artifactKeyWithBuild(manifest.GetRef(), 1)

	err := srv.completePublish(ctx, manifest, key, nil, nil)
	if err == nil {
		t.Fatal("completePublish must refuse a hard_deps artifact when the catalog is unavailable")
	}
	if !strings.Contains(err.Error(), "catalog unavailable") {
		t.Errorf("expected catalog-unavailable rejection, got: %v", err)
	}
}

// updateBinaryStubStream is a stub PackageRepository_UpdateArtifactBinaryServer
// that replays a header + chunk and captures the SendAndClose response.
type updateBinaryStubStream struct {
	ctx      context.Context
	msgs     []*repopb.UpdateArtifactBinaryRequest
	pos      int
	response *repopb.UpdateArtifactBinaryResponse
}

func (s *updateBinaryStubStream) Recv() (*repopb.UpdateArtifactBinaryRequest, error) {
	if s.pos >= len(s.msgs) {
		return nil, io.EOF
	}
	m := s.msgs[s.pos]
	s.pos++
	return m, nil
}
func (s *updateBinaryStubStream) SendAndClose(r *repopb.UpdateArtifactBinaryResponse) error {
	s.response = r
	return nil
}
func (s *updateBinaryStubStream) Context() context.Context    { return s.ctx }
func (s *updateBinaryStubStream) RecvMsg(any) error           { return nil }
func (s *updateBinaryStubStream) SendMsg(any) error           { return nil }
func (s *updateBinaryStubStream) SetHeader(metadata.MD) error { return nil }
func (s *updateBinaryStubStream) SendHeader(metadata.MD) error {
	return nil
}
func (s *updateBinaryStubStream) SetTrailer(metadata.MD) {}

// TestUpdateArtifactBinary_RejectsSameVersionDifferentDigest is the F3 regression:
// delta-deploy of an already-PUBLISHED version with DIFFERENT bytes must be
// rejected up front (AlreadyExists), not stored as a divergent-digest VERIFIED
// phantom and reported as soft "verified" success. Same bytes is an idempotent
// no-op. (repository.artifact.content_immutable_after_publish /
// meta.silence_is_not_valid_for_unexpected.)
func TestUpdateArtifactBinary_RejectsSameVersionDifferentDigest(t *testing.T) {
	srv := newTestServer(t) // scylla nil → storage fallback resolves the seeded build
	ctx := context.Background()
	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io", Name: "echo", Version: "1.0.0",
		Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
	}
	origBytes := []byte("original-echo-binary")
	origDigest := checksumBytes(origBytes)
	seedPublishedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref: ref, BuildNumber: 1, BuildId: "v1",
		Checksum: origDigest, SizeBytes: int64(len(origBytes)),
	})

	mkStream := func(data []byte) *updateBinaryStubStream {
		return &updateBinaryStubStream{ctx: ctx, msgs: []*repopb.UpdateArtifactBinaryRequest{
			{Payload: &repopb.UpdateArtifactBinaryRequest_Header{Header: &repopb.UpdateArtifactBinaryHeader{Ref: ref}}},
			{Payload: &repopb.UpdateArtifactBinaryRequest_Chunk{Chunk: data}},
		}}
	}

	// Different bytes for the already-published version → AlreadyExists, no phantom.
	if err := srv.UpdateArtifactBinary(mkStream([]byte("DIFFERENT-bytes"))); status.Code(err) != codes.AlreadyExists {
		t.Fatalf("expected AlreadyExists for same-version/different-digest, got: %v", err)
	}

	// Same bytes → idempotent published, no error, no new build allocated.
	s := mkStream(origBytes)
	if err := srv.UpdateArtifactBinary(s); err != nil {
		t.Fatalf("idempotent same-digest update must succeed, got: %v", err)
	}
	if s.response == nil || s.response.GetStatus() != "published" {
		t.Errorf("idempotent update must return status=published, got: %+v", s.response)
	}
	if s.response != nil && s.response.GetBuildNumber() != 1 {
		t.Errorf("idempotent update must not allocate a new build, got build %d", s.response.GetBuildNumber())
	}
}

// ── Test 7: promoteToPublished returns error when artifact_state update fails ─
// A failed artifact_state transition must propagate as an error so the operator
// sees failure from 'globular pkg publish' rather than a silent BLOB_VERIFIED.

func TestPromoteToPublished_ReturnsErrorWhenArtifactStateTransitionFails(t *testing.T) {
	ref := &repopb.ArtifactRef{
		PublisherId: "glob", Name: "echo", Version: "1.0.66",
		Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
	}
	ctx := context.Background()

	// errLedger fails UpdatePublishState AND UpdateArtifactState.
	srv := newTestServer(t)
	srv.scylla = errLedger{err: errors.New("scylla unavailable")}

	key := artifactKeyWithBuild(ref, 1)
	binary := []byte("fake-binary")
	_ = srv.localStorage.MkdirAll(ctx, artifactsDir, 0o755)
	_ = srv.localStorage.WriteFile(ctx, binaryStorageKey(key), binary, 0o644)

	manifest := &repopb.ArtifactManifest{
		Ref: ref, BuildNumber: 1, SizeBytes: int64(len(binary)),
	}

	err := srv.promoteToPublished(ctx, key, manifest)
	if err == nil {
		t.Fatal("expected promoteToPublished to fail when Scylla is unavailable")
	}
}
