package main

// resolve_version_repair_test.go — Phase 32.
//
// Pins the version-immutability gate's repair-awareness:
// resolveVersionIntent(EXACT) is the SECOND immutability gate in the
// upload path. The FIRST (enforceOfficialNamespaceSeal) was made
// repair-aware in Phase 31; without this Phase 32 plumbing, the repair
// authorization would pass the seal then be rejected here with
// AlreadyExists — exactly what blocked the node-agent phantom repair
// during Phase 30 deploy.
//
// These tests cover every gate the repair path must enforce at the
// version-immutability layer, mirroring the seal-gate tests in
// repair_authorization_test.go for symmetry.

import (
	"context"
	"strings"
	"testing"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const versionSealedDigest = "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd"

// seedPublishedVersion stores a sealed published artifact for the version
// gate to find. resolveVersionIntent reads from the ledger via
// getExactRelease / getPublishedDigest, both of which are populated by
// seedPublishedArtifact in local_publish_test.go's test seam.
func seedPublishedVersion(t *testing.T, srv *server) {
	t.Helper()
	seedPublishedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref: &repopb.ArtifactRef{
			PublisherId: "core@globular.io",
			Name:        "node-agent",
			Version:     "1.2.143",
			Platform:    "linux_amd64",
			Kind:        repopb.ArtifactKind_SERVICE,
		},
		BuildNumber: 1,
		BuildId:     "phantom-1.2.143-old-bytes",
		Checksum:    versionSealedDigest,
		SizeBytes:   100,
	})
}

func TestResolveVersion_NoRepair_VersionAlreadyPublished_Rejects(t *testing.T) {
	// Regression: default behaviour is unchanged — version already
	// published is rejected with AlreadyExists. This is the gate that
	// protects against the build-drift hazard documented inline.
	srv := newTestServer(t)
	seedPublishedVersion(t, srv)

	_, err := srv.resolveVersionIntent(
		context.Background(),
		"core@globular.io", "node-agent", "linux_amd64",
		repopb.VersionIntent_EXACT, "1.2.143",
		repopb.ArtifactChannel_STABLE, nil,
	)
	if err == nil {
		t.Fatal("expected AlreadyExists, got nil")
	}
	if code := status.Code(err); code != codes.AlreadyExists {
		t.Errorf("code=%v want AlreadyExists", code)
	}
	if !strings.Contains(err.Error(), "--unseal-official") {
		t.Errorf("error must mention --unseal-official so operators discover the repair path; got: %v", err)
	}
}

func TestResolveVersion_RepairValidatesAndAllows(t *testing.T) {
	// The unblock path: when all four gates pass, the version-immutability
	// gate accepts the re-publish. This is exactly what Phase 32 fixes.
	srv := newTestServer(t)
	seedPublishedVersion(t, srv)

	repair := &RepairAuthorization{
		Requested:   true,
		Reason:      "v1.2.131 bytes sealed under v1.2.143 — Path B investigation",
		PriorDigest: versionSealedDigest,
	}
	got, err := srv.resolveVersionIntent(
		context.Background(),
		"core@globular.io", "node-agent", "linux_amd64",
		repopb.VersionIntent_EXACT, "1.2.143",
		repopb.ArtifactChannel_STABLE, repair,
	)
	if err != nil {
		t.Fatalf("expected repair to be authorized at version gate, got: %v", err)
	}
	if got != "1.2.143" {
		t.Fatalf("returned version=%q want 1.2.143", got)
	}
	if !repair.Used {
		t.Error("repair.Used was not set to true — post-success audit will not fire")
	}
	if repair.PriorBuildID != "phantom-1.2.143-old-bytes" {
		t.Errorf("repair.PriorBuildID=%q want phantom-1.2.143-old-bytes", repair.PriorBuildID)
	}
}

func TestResolveVersion_RepairEmptyReason_RejectsInvalidArgument(t *testing.T) {
	// Missing reason at gate 2 → InvalidArgument (same shape as gate 1).
	srv := newTestServer(t)
	seedPublishedVersion(t, srv)

	_, err := srv.resolveVersionIntent(
		context.Background(),
		"core@globular.io", "node-agent", "linux_amd64",
		repopb.VersionIntent_EXACT, "1.2.143",
		repopb.ArtifactChannel_STABLE,
		&RepairAuthorization{Requested: true, Reason: "", PriorDigest: versionSealedDigest},
	)
	if err == nil {
		t.Fatal("expected InvalidArgument for empty reason, got nil")
	}
	if code := status.Code(err); code != codes.InvalidArgument {
		t.Errorf("code=%v want InvalidArgument", code)
	}
}

func TestResolveVersion_RepairWrongPriorDigest_RejectsFailedPrecondition(t *testing.T) {
	// Wrong prior digest at gate 2 → FailedPrecondition (mirrors gate 1).
	// This proves operator-visible symmetry: same error code for same
	// failure shape regardless of which gate fired.
	srv := newTestServer(t)
	seedPublishedVersion(t, srv)

	_, err := srv.resolveVersionIntent(
		context.Background(),
		"core@globular.io", "node-agent", "linux_amd64",
		repopb.VersionIntent_EXACT, "1.2.143",
		repopb.ArtifactChannel_STABLE,
		&RepairAuthorization{
			Requested:   true,
			Reason:      "test",
			PriorDigest: "sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee", // wrong
		},
	)
	if err == nil {
		t.Fatal("expected FailedPrecondition for prior-digest mismatch, got nil")
	}
	if code := status.Code(err); code != codes.FailedPrecondition {
		t.Errorf("code=%v want FailedPrecondition", code)
	}
}

func TestResolveVersion_RepairOnNewVersion_NoOp(t *testing.T) {
	// Repair authorization is meaningless when there's no existing
	// published version to repair — the resolver simply returns the
	// requested version. Repair fields are NOT marked Used in that case.
	srv := newTestServer(t)
	// Note: no seedPublishedVersion — the slot is empty.

	repair := &RepairAuthorization{
		Requested:   true,
		Reason:      "should not be consumed",
		PriorDigest: "sha256:any",
	}
	got, err := srv.resolveVersionIntent(
		context.Background(),
		"core@globular.io", "node-agent", "linux_amd64",
		repopb.VersionIntent_EXACT, "1.2.99",
		repopb.ArtifactChannel_STABLE, repair,
	)
	if err != nil {
		t.Fatalf("first-publish should succeed regardless of repair, got: %v", err)
	}
	if got != "1.2.99" {
		t.Errorf("version=%q want 1.2.99", got)
	}
	if repair.Used {
		t.Error("repair.Used was set but no immutability gate fired — would emit spurious audit")
	}
}

func TestResolveVersion_RepairUsedFlagPropagates(t *testing.T) {
	// Specifically pin that the Used flag survives across the resolver
	// call — the post-success audit in UploadArtifact reads this flag.
	srv := newTestServer(t)
	seedPublishedVersion(t, srv)

	repair := &RepairAuthorization{
		Requested:   true,
		Reason:      "test propagation",
		PriorDigest: versionSealedDigest,
	}
	if _, err := srv.resolveVersionIntent(
		context.Background(),
		"core@globular.io", "node-agent", "linux_amd64",
		repopb.VersionIntent_EXACT, "1.2.143",
		repopb.ArtifactChannel_STABLE, repair,
	); err != nil {
		t.Fatalf("resolveVersionIntent err=%v", err)
	}
	if !repair.Used {
		t.Fatal("repair.Used must be true after gate authorized — post-success audit depends on it")
	}
	if repair.PriorBuildID == "" {
		t.Error("repair.PriorBuildID must be populated for audit completeness")
	}
}

func TestResolveVersion_RepairBumpIntentIgnoresRepair(t *testing.T) {
	// Bump intents are for new-version allocation. Even if repair metadata
	// is passed (e.g. by an oblivious caller), the bump branch must ignore
	// it — repair only ever applies to EXACT version conflicts.
	srv := newTestServer(t)
	seedPublishedVersion(t, srv)

	repair := &RepairAuthorization{Requested: true, Reason: "x", PriorDigest: "y"}
	got, err := srv.resolveVersionIntent(
		context.Background(),
		"core@globular.io", "node-agent", "linux_amd64",
		repopb.VersionIntent_BUMP_PATCH, "",
		repopb.ArtifactChannel_STABLE, repair,
	)
	if err != nil {
		t.Fatalf("bump should succeed, got: %v", err)
	}
	if !strings.HasPrefix(got, "1.") {
		t.Errorf("bumped version=%q does not look like a semver bump", got)
	}
	if repair.Used {
		t.Error("repair.Used set on bump intent — only EXACT should consume repair")
	}
}

func TestResolveVersion_BothGatesShareRepairState(t *testing.T) {
	// Cross-gate guarantee: the seal gate (gate 1) and the version gate
	// (gate 3) operate on the SAME *RepairAuthorization pointer. If the
	// seal gate runs first (the production order in UploadArtifact) and
	// marks Used=true with a PriorBuildID, the version gate observes
	// that state and does not double-populate or contradict.
	srv := newTestServer(t)
	seedPublishedVersion(t, srv)
	ctx := context.Background()

	repair := &RepairAuthorization{
		Requested:   true,
		Reason:      "cross-gate symmetry test",
		PriorDigest: versionSealedDigest,
	}

	// Seal gate first (the production caller does this).
	if err := srv.enforceOfficialNamespaceSeal(ctx,
		"core@globular.io", "node-agent", "1.2.143", "linux_amd64",
		"sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
		repopb.ArtifactChannel_STABLE, repair,
	); err != nil {
		t.Fatalf("seal gate failed: %v", err)
	}
	firstPriorBuildID := repair.PriorBuildID
	if firstPriorBuildID == "" {
		t.Fatal("seal gate did not populate PriorBuildID")
	}

	// Version gate second.
	if _, err := srv.resolveVersionIntent(ctx,
		"core@globular.io", "node-agent", "linux_amd64",
		repopb.VersionIntent_EXACT, "1.2.143",
		repopb.ArtifactChannel_STABLE, repair,
	); err != nil {
		t.Fatalf("version gate failed after seal: %v", err)
	}

	// Both gates saw Used=true; PriorBuildID survives unchanged.
	if !repair.Used {
		t.Error("repair.Used not preserved across gates")
	}
	if repair.PriorBuildID != firstPriorBuildID {
		t.Errorf("PriorBuildID changed across gates: seal=%q version=%q", firstPriorBuildID, repair.PriorBuildID)
	}
}
