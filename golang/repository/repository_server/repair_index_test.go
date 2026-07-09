package main

// repair_index_test.go — SCAR-3 regression coverage for the owner RepairIndex
// operation (repository.cas_present_index_unknown_is_owner_repairable).
//
// Proves: (1) CAS-present + index-non-installable is DETECTED as repairable;
// (2) dry-run reports actions but mutates nothing; (3) commit reconstitutes the
// index using ONLY the existing blob/sidecar identity (never minted); (4) repair
// REFUSES when identity evidence is missing/contradictory (no upstream fetch, no
// promotion); (5) repair never touches desired state; plus REVOKED/QUARANTINED
// are skipped as policy (never elevated) and already-PUBLISHED is skipped_ok.
//
// Uses the standard repository_server test harness (newTestServer: OS storage,
// scylla=nil, cache-based state).

import (
	"context"
	"os"
	"strings"
	"testing"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

// seedCASArtifact writes a self-consistent CAS record — manifest sidecar + blob
// with a matching size + sha256 — WITHOUT stamping any index/pipeline state. That
// is exactly the "index UNKNOWN/missing while the blob is intact" condition
// RepairIndex targets. Returns the artifact key.
func seedCASArtifact(t *testing.T, srv *server, ref *repopb.ArtifactRef, buildNumber int64, buildID string, content []byte) string {
	t.Helper()
	ctx := context.Background()
	key := artifactKeyWithBuild(ref, buildNumber)
	_ = srv.Storage().MkdirAll(ctx, artifactsDir, 0o755)
	if err := srv.Storage().WriteFile(ctx, binaryStorageKey(key), content, 0o644); err != nil {
		t.Fatalf("write blob: %v", err)
	}
	sum, err := checksumLocalFile(srv.localStorage.LocalPath(binaryStorageKey(key)))
	if err != nil {
		t.Fatalf("checksum blob: %v", err)
	}
	m := &repopb.ArtifactManifest{
		Ref: ref, BuildNumber: buildNumber, BuildId: buildID,
		Checksum: sum, SizeBytes: int64(len(content)),
	}
	mjson, err := marshalManifestWithState(m, repopb.PublishState_PUBLISH_STATE_UNSPECIFIED)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	if err := srv.Storage().WriteFile(ctx, manifestStorageKey(key), mjson, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	return key
}

func mustTransition(t *testing.T, srv *server, key string, to ArtifactPipelineState) {
	t.Helper()
	if err := srv.transitionArtifactState(context.Background(), key, to, "test_setup", "", ArtifactStateFields{
		BlobKey: binaryStorageKey(key),
	}); err != nil {
		t.Fatalf("setup transition → %s: %v", to, err)
	}
}

func repoTestRef() *repopb.ArtifactRef {
	return &repopb.ArtifactRef{
		PublisherId: "core@globular.io", Name: "rbac",
		Version: "1.2.272", Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
	}
}

func TestRepairIndex_ReconstructsFromCasEvidenceOnly(t *testing.T) {
	ctx := context.Background()
	ref := repoTestRef()
	blob := []byte("rbac-1.2.272-binary-bytes")
	const buildID = "build-rbac-1"

	// (1)+(2): detect the divergence and prove dry-run mutates nothing.
	t.Run("detect_and_dryrun_is_side_effect_free", func(t *testing.T) {
		srv := newTestServer(t)
		key := seedCASArtifact(t, srv, ref, 1, buildID, blob)
		if srv.readArtifactState(ctx, key).IsInstallable() {
			t.Fatal("precondition: seeded CAS artifact must start non-installable (index UNKNOWN/missing)")
		}

		rep, err := srv.repairIndexFromCAS(ctx, RepairIndexOptions{DryRun: true})
		if err != nil {
			t.Fatalf("repairIndexFromCAS dry-run: %v", err)
		}
		if rep.Scanned != 1 || rep.Repairable != 1 || rep.Repaired != 0 || rep.Refused != 0 {
			t.Fatalf("dry-run counts: %+v", rep)
		}
		if len(rep.Items) != 1 || rep.Items[0].Action != "would_repair_publish_index" {
			t.Fatalf("dry-run action: %+v", rep.Items)
		}
		// Mutates nothing: state unchanged, still not installable.
		if srv.readArtifactState(ctx, key).IsInstallable() {
			t.Fatal("dry-run must not promote the artifact to installable")
		}
	})

	// (3): commit reconstitutes from evidence; identity is NOT minted.
	t.Run("commit_reconstitutes_identity_from_evidence", func(t *testing.T) {
		srv := newTestServer(t)
		key := seedCASArtifact(t, srv, ref, 1, buildID, blob)
		manifestBefore, _ := srv.Storage().ReadFile(ctx, manifestStorageKey(key))

		rep, err := srv.repairIndexFromCAS(ctx, RepairIndexOptions{DryRun: false})
		if err != nil {
			t.Fatalf("repairIndexFromCAS commit: %v", err)
		}
		if rep.Repaired != 1 || rep.Refused != 0 {
			t.Fatalf("commit counts: %+v", rep)
		}
		it := rep.Items[0]
		if it.Action != "repair_publish_index" {
			t.Fatalf("commit action: %q", it.Action)
		}
		// Index is now installable (state-level authority).
		if !srv.readArtifactState(ctx, key).IsInstallable() {
			t.Fatal("commit must make the artifact installable")
		}
		// Identity came from the sidecar manifest — never minted.
		if it.BuildId != buildID {
			t.Fatalf("build_id must come from the sidecar manifest: got %q want %q", it.BuildId, buildID)
		}
		// The on-disk manifest (identity source) is byte-unchanged.
		manifestAfter, _ := srv.Storage().ReadFile(ctx, manifestStorageKey(key))
		if string(manifestBefore) != string(manifestAfter) {
			t.Fatal("repair must not rewrite the manifest identity on disk")
		}
	})

	// (4a): refuse when the blob is absent — no promotion, no upstream fetch.
	t.Run("refuse_when_blob_missing", func(t *testing.T) {
		srv := newTestServer(t)
		key := seedCASArtifact(t, srv, ref, 1, buildID, blob)
		if err := srv.Storage().Remove(ctx, binaryStorageKey(key)); err != nil {
			t.Fatalf("remove blob: %v", err)
		}
		rep, err := srv.repairIndexFromCAS(ctx, RepairIndexOptions{DryRun: false})
		if err != nil {
			t.Fatalf("repairIndexFromCAS: %v", err)
		}
		if rep.Refused != 1 || rep.Repaired != 0 {
			t.Fatalf("missing-blob counts: %+v", rep)
		}
		if rep.Items[0].Action != "refused" {
			t.Fatalf("missing-blob action: %q", rep.Items[0].Action)
		}
		if srv.readArtifactState(ctx, key).IsInstallable() {
			t.Fatal("refused artifact must not be promoted to installable")
		}
	})

	// (4b): refuse when the blob contradicts the sidecar checksum (same length,
	// different bytes — exercises the checksum branch, not just size).
	t.Run("refuse_when_checksum_contradictory", func(t *testing.T) {
		srv := newTestServer(t)
		key := seedCASArtifact(t, srv, ref, 1, buildID, blob)
		tampered := make([]byte, len(blob))
		for i := range tampered {
			tampered[i] = 'X'
		}
		if err := srv.Storage().WriteFile(ctx, binaryStorageKey(key), tampered, 0o644); err != nil {
			t.Fatalf("tamper blob: %v", err)
		}
		rep, err := srv.repairIndexFromCAS(ctx, RepairIndexOptions{DryRun: false})
		if err != nil {
			t.Fatalf("repairIndexFromCAS: %v", err)
		}
		if rep.Refused != 1 || rep.Repaired != 0 {
			t.Fatalf("checksum-mismatch counts: %+v", rep)
		}
		if rep.Items[0].Action != "refused" {
			t.Fatalf("checksum-mismatch action: %q", rep.Items[0].Action)
		}
	})

	// REVOKED / QUARANTINED are policy states — never auto-elevated.
	t.Run("policy_states_skipped_not_elevated", func(t *testing.T) {
		for _, st := range []ArtifactPipelineState{PipelineRevoked, PipelineQuarantined} {
			srv := newTestServer(t)
			key := seedCASArtifact(t, srv, ref, 1, buildID, blob)
			mustTransition(t, srv, key, st) // legal from Unspecified
			rep, err := srv.repairIndexFromCAS(ctx, RepairIndexOptions{DryRun: false})
			if err != nil {
				t.Fatalf("repairIndexFromCAS: %v", err)
			}
			if rep.SkippedPolicy != 1 || rep.Repaired != 0 {
				t.Fatalf("%s: expected SkippedPolicy=1 Repaired=0, got %+v", st, rep)
			}
			if srv.readArtifactState(ctx, key) != st {
				t.Fatalf("%s must not be elevated by repair, got %s", st, srv.readArtifactState(ctx, key))
			}
		}
	})

	// Already PUBLISHED → skipped_ok (idempotent).
	t.Run("already_published_skipped", func(t *testing.T) {
		srv := newTestServer(t)
		key := seedCASArtifact(t, srv, ref, 1, buildID, blob)
		mustTransition(t, srv, key, PipelinePublished)
		rep, err := srv.repairIndexFromCAS(ctx, RepairIndexOptions{DryRun: false})
		if err != nil {
			t.Fatalf("repairIndexFromCAS: %v", err)
		}
		if rep.SkippedOk != 1 || rep.Repaired != 0 {
			t.Fatalf("expected SkippedOk=1 Repaired=0, got %+v", rep)
		}
	})

	// Filters: a name filter restricts the scan.
	t.Run("name_filter_restricts_scan", func(t *testing.T) {
		srv := newTestServer(t)
		seedCASArtifact(t, srv, ref, 1, buildID, blob)
		other := &repopb.ArtifactRef{PublisherId: "core@globular.io", Name: "dns", Version: "1.2.272", Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE}
		seedCASArtifact(t, srv, other, 1, "build-dns-1", []byte("dns-binary"))
		rep, err := srv.repairIndexFromCAS(ctx, RepairIndexOptions{DryRun: true, NameFilter: "rbac"})
		if err != nil {
			t.Fatalf("repairIndexFromCAS: %v", err)
		}
		if rep.Scanned != 1 || rep.Items[0].Name != "rbac" {
			t.Fatalf("name filter should scan only rbac, got %+v", rep)
		}
	})
}

// Requirement 1 (literal sentinel): the UNKNOWN store-unreachable sentinel is a
// repair CANDIDATE (a divergence, not terminal); PUBLISHED is ok; policy states
// are skip. Pure classifier — no store needed.
func TestRepairIndex_ClassifyState(t *testing.T) {
	cases := map[ArtifactPipelineState]repairIndexCategory{
		PipelineUnknown:                repairCatCandidate,
		PipelineUnspecified:            repairCatCandidate,
		PipelineDownloading:            repairCatCandidate,
		PipelineBrokenChecksumMismatch: repairCatCandidate,
		PipelinePublished:              repairCatPublishedOk,
		PipelineRevoked:                repairCatPolicySkip,
		PipelineQuarantined:            repairCatPolicySkip,
	}
	for state, want := range cases {
		if got := classifyRepairIndexState(state); got != want {
			t.Errorf("classifyRepairIndexState(%s) = %d, want %d", state, got, want)
		}
	}
}

// Requirement 5 (structural): RepairIndex must not read or write the controller's
// desired state — repository owns repository repair. Guard against a future edit
// wiring in a desired/controller dependency.
func TestRepairIndex_DoesNotTouchDesiredState(t *testing.T) {
	src, err := os.ReadFile("artifact_repair_index.go")
	if err != nil {
		t.Fatalf("read source: %v", err)
	}
	for _, banned := range []string{"cluster_controller", "desiredpb", "DesiredServiceClient", "SetDesiredService", "resourcepb"} {
		if strings.Contains(string(src), banned) {
			t.Errorf("repairIndexFromCAS must not reference desired/controller API %q — repository owns repository repair", banned)
		}
	}
}
