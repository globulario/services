package main

// repair_authorization_test.go — pins the contract documented in
// repair_authorization.go and the extended enforceOfficialNamespaceSeal in
// local_publish_guard.go. These tests cover every gate the repair path
// must enforce, plus the regression guard that the unauthorized path
// still rejects (Path B's original protection must not regress).

import (
	"context"
	"strings"
	"testing"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

const sealedDigest = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
const incomingDigest = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"

// seedSealedOfficialArtifact stores a sealed official artifact at
// (publisher, name, version, platform) in the test server so subsequent
// seal-check calls find it.
func seedSealedOfficialArtifact(t *testing.T, srv *server) {
	t.Helper()
	seedPublishedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref: &repopb.ArtifactRef{
			PublisherId: "core@globular.io",
			Name:        "storage",
			Version:     "1.2.43",
			Platform:    "linux_amd64",
			Kind:        repopb.ArtifactKind_SERVICE,
		},
		BuildNumber: 1,
		BuildId:     "phantom-build-A",
		Checksum:    sealedDigest,
		SizeBytes:   100,
	})
}

func TestRepair_NoAuthorization_RejectedAsBefore(t *testing.T) {
	// Regression guard: a normal publish (repair=nil) against a sealed
	// official artifact MUST still be rejected with PermissionDenied.
	// This is the original Path B behavior that protects the seal.
	srv := newTestServer(t)
	ctx := context.Background()
	seedSealedOfficialArtifact(t, srv)

	err := srv.enforceOfficialNamespaceSeal(ctx,
		"core@globular.io", "storage", "1.2.43", "linux_amd64",
		incomingDigest, repopb.ArtifactChannel_STABLE, nil,
	)
	if err == nil {
		t.Fatal("expected seal to reject unauthorized overwrite, got nil")
	}
	if code := status.Code(err); code != codes.PermissionDenied {
		t.Errorf("code=%v want PermissionDenied", code)
	}
	if !strings.Contains(err.Error(), "--unseal-official") {
		t.Errorf("error must mention the repair escape hatch so operators discover it; got: %v", err)
	}
}

func TestRepair_RequestedWithoutReason_RejectedInvalidArgument(t *testing.T) {
	// Missing --reason is an explicit operator error, distinct from "the
	// seal blocked you". Return InvalidArgument so it's discoverable as
	// a CLI/operator-side issue, not a permission issue.
	srv := newTestServer(t)
	ctx := context.Background()
	seedSealedOfficialArtifact(t, srv)

	err := srv.enforceOfficialNamespaceSeal(ctx,
		"core@globular.io", "storage", "1.2.43", "linux_amd64",
		incomingDigest, repopb.ArtifactChannel_STABLE,
		&RepairAuthorization{Requested: true, Reason: "", PriorDigest: sealedDigest},
	)
	if err == nil {
		t.Fatal("expected repair to reject empty reason, got nil")
	}
	if code := status.Code(err); code != codes.InvalidArgument {
		t.Errorf("code=%v want InvalidArgument", code)
	}
	if !strings.Contains(err.Error(), "reason") {
		t.Errorf("error must explain that reason was empty; got: %v", err)
	}
}

func TestRepair_PriorDigestMismatch_RejectedFailedPrecondition(t *testing.T) {
	// Wrong --prior-digest = caller has stale state. Return
	// FailedPrecondition so the operator knows the cluster moved under
	// them and they need to re-investigate.
	srv := newTestServer(t)
	ctx := context.Background()
	seedSealedOfficialArtifact(t, srv)

	err := srv.enforceOfficialNamespaceSeal(ctx,
		"core@globular.io", "storage", "1.2.43", "linux_amd64",
		incomingDigest, repopb.ArtifactChannel_STABLE,
		&RepairAuthorization{
			Requested:   true,
			Reason:      "v1.2.131 mislabeled as 1.2.43",
			PriorDigest: "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc", // wrong
		},
	)
	if err == nil {
		t.Fatal("expected repair to reject prior-digest mismatch, got nil")
	}
	if code := status.Code(err); code != codes.FailedPrecondition {
		t.Errorf("code=%v want FailedPrecondition", code)
	}
	if !strings.Contains(err.Error(), "prior-digest mismatch") {
		t.Errorf("error must explain prior-digest mismatch; got: %v", err)
	}
}

func TestRepair_AllGatesPass_Authorized(t *testing.T) {
	// The success path: every gate satisfied → seal returns nil and the
	// upload is allowed to proceed. This is the actual unblock for the
	// Phase 30 phantom recovery scenario.
	srv := newTestServer(t)
	ctx := context.Background()
	seedSealedOfficialArtifact(t, srv)

	err := srv.enforceOfficialNamespaceSeal(ctx,
		"core@globular.io", "storage", "1.2.43", "linux_amd64",
		incomingDigest, repopb.ArtifactChannel_STABLE,
		&RepairAuthorization{
			Requested:   true,
			Reason:      "v1.2.131 mislabeled as 1.2.43 — proven via repository_explain_artifact",
			PriorDigest: sealedDigest,
		},
	)
	if err != nil {
		t.Fatalf("expected repair to be authorized when all gates pass, got: %v", err)
	}
}

func TestRepair_DigestNormalization_AllowsSha256Prefix(t *testing.T) {
	// Operators may pass `--prior-digest sha256:abc…` or bare hex `abc…`;
	// the server normalizes both forms (digestsMatch trims `sha256:`).
	// Pin this so operator UX isn't surprising.
	srv := newTestServer(t)
	ctx := context.Background()
	seedSealedOfficialArtifact(t, srv)

	bareHex := strings.TrimPrefix(sealedDigest, "sha256:")
	err := srv.enforceOfficialNamespaceSeal(ctx,
		"core@globular.io", "storage", "1.2.43", "linux_amd64",
		incomingDigest, repopb.ArtifactChannel_STABLE,
		&RepairAuthorization{
			Requested:   true,
			Reason:      "test bare-hex digest normalization",
			PriorDigest: bareHex, // no sha256: prefix
		},
	)
	if err != nil {
		t.Fatalf("expected bare-hex prior-digest to be accepted, got: %v", err)
	}
}

func TestRepair_NonOfficialPublisher_RepairNoOp(t *testing.T) {
	// Repair authorization is meaningless for non-official publishers
	// because the seal doesn't apply to them in the first place. The
	// check returns nil without consulting repair fields. (No regression
	// from existing isOfficialPublisher gate.)
	srv := newTestServer(t)
	ctx := context.Background()

	err := srv.enforceOfficialNamespaceSeal(ctx,
		"local@cluster-b", "storage", "1.2.43+local.cluster-b.1", "linux_amd64",
		"sha256:doesnotmatter", repopb.ArtifactChannel_DEV,
		&RepairAuthorization{Requested: true, Reason: "n/a", PriorDigest: "n/a"},
	)
	if err != nil {
		t.Fatalf("non-official publisher must be no-op regardless of repair, got: %v", err)
	}
}

func TestGetRepairAuthorization_NoMetadata_ReturnsNil(t *testing.T) {
	// Plain context (no incoming metadata) MUST yield nil — never a
	// half-populated struct that could accidentally be treated as a
	// repair request.
	ctx := context.Background()
	if got := getRepairAuthorization(ctx); got != nil {
		t.Fatalf("expected nil repair authorization for plain ctx, got %+v", got)
	}
}

func TestGetRepairAuthorization_MetadataWithoutUnsealFlag_ReturnsNil(t *testing.T) {
	// Reason/prior-digest present but no unseal=true → not requested.
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		"x-repair-reason", "test",
		"x-repair-prior-digest", "sha256:abc",
	))
	if got := getRepairAuthorization(ctx); got != nil {
		t.Fatalf("expected nil when unseal flag absent, got %+v", got)
	}
}

func TestGetRepairAuthorization_FullTriplet_Parsed(t *testing.T) {
	// Full metadata triplet → struct populated for the seal check.
	ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
		"x-repair-unseal-official", "true",
		"x-repair-reason", "Phase 30 phantom recovery",
		"x-repair-prior-digest", "sha256:abc123",
	))
	got := getRepairAuthorization(ctx)
	if got == nil {
		t.Fatal("expected non-nil RepairAuthorization for full triplet")
	}
	if !got.Requested {
		t.Error("Requested=false; want true")
	}
	if got.Reason != "Phase 30 phantom recovery" {
		t.Errorf("Reason=%q; want %q", got.Reason, "Phase 30 phantom recovery")
	}
	if got.PriorDigest != "sha256:abc123" {
		t.Errorf("PriorDigest=%q; want %q", got.PriorDigest, "sha256:abc123")
	}
}

func TestGetRepairAuthorization_UnsealCaseInsensitive(t *testing.T) {
	// gRPC canonicalizes header values as-given; we accept "True", "TRUE",
	// "true" all the same via EqualFold. Pin this so operator UX doesn't
	// break on uppercase values from shell expansions.
	for _, val := range []string{"true", "True", "TRUE"} {
		ctx := metadata.NewIncomingContext(context.Background(), metadata.Pairs(
			"x-repair-unseal-official", val,
			"x-repair-reason", "test",
			"x-repair-prior-digest", "sha256:abc",
		))
		if got := getRepairAuthorization(ctx); got == nil || !got.Requested {
			t.Errorf("value=%q: expected Requested=true, got %+v", val, got)
		}
	}
}
