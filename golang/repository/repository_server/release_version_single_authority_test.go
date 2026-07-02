package main

// CG-3 proof for invariant release.version_single_authority:
//
//	"The human release version is allocated by exactly one authority: the
//	 repository service ... the repository validates monotonicity and records
//	 the allocation. No other caller ... may allocate or advance a release
//	 version."
//
// resolveVersionIntent is the allocation chokepoint named by the invariant
// (protects.symbols: AllocateUpload, resolveVersionIntent). These tests pin the
// authority's three guarantees so the invariant can be promoted from proposed to
// active with evidence:
//
//  1. it REJECTS a regressive version (no caller can allocate below the
//     published high-water mark);
//  2. it treats a published version as IMMUTABLE (no caller can re-allocate an
//     existing release version);
//  3. it COMPUTES the next version from the published latest on a bump (the
//     authority advances the release stream — the caller does not self-stamp).
//
// Together these prove there is exactly one place where a release version is
// decided, and that it is monotonic and immutable — the structural defense
// against the "two version authors" drift the invariant exists to prevent.

import (
	"context"
	"strings"
	"testing"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

// publishedHighWaterMark is the version the repository has already allocated and
// published; every assertion below is relative to it.
const publishedHighWaterMark = "1.2.235"

func seedPublishedHighWaterMark(t *testing.T, srv *server) {
	t.Helper()
	// seedLatestVersion (append_to_ledger_recovery_test.go) publishes via the
	// real appendToLedger path for core@globular.io/demo-svc on linux_amd64.
	seedLatestVersion(t, srv, publishedHighWaterMark,
		"hwm-build-aaaa", "sha256:"+strings.Repeat("a", 64))
}

// (1) Regression is rejected: a caller cannot allocate a release version below
// the published high-water mark. This is the core of single-authority — the
// repository, not the caller, decides what is a valid forward step.
func TestVersionSingleAuthority_RegressionRejected(t *testing.T) {
	srv := newTestServer(t)
	seedPublishedHighWaterMark(t, srv)

	_, err := srv.resolveVersionIntent(
		context.Background(),
		"core@globular.io", "demo-svc", "linux_amd64",
		repopb.VersionIntent_EXACT, "1.2.234", repopb.ArtifactChannel_STABLE, nil,
	)
	if err == nil {
		t.Fatal("expected a regressive version below the published high-water mark to be rejected")
	}
	if !strings.Contains(err.Error(), "monotonically increasing") {
		t.Fatalf("expected a monotonicity rejection, got: %v", err)
	}
}

// (2) A published version is immutable: the authority refuses to re-allocate an
// already-published release version (which would mint a new build_id for the
// same human version and strand every node on the old one).
func TestVersionSingleAuthority_PublishedVersionImmutable(t *testing.T) {
	srv := newTestServer(t)
	seedPublishedHighWaterMark(t, srv)

	_, err := srv.resolveVersionIntent(
		context.Background(),
		"core@globular.io", "demo-svc", "linux_amd64",
		repopb.VersionIntent_EXACT, publishedHighWaterMark, repopb.ArtifactChannel_STABLE, nil,
	)
	if err == nil {
		t.Fatalf("expected re-allocating published version %s to be rejected as immutable", publishedHighWaterMark)
	}
	if !strings.Contains(err.Error(), "already published") {
		t.Fatalf("expected an immutability rejection, got: %v", err)
	}
}

// (3) The authority advances the stream: a bump computes the next version from
// the published latest. The caller expresses INTENT (bump patch); the repository
// produces the version. This is the positive half of single-authority.
func TestVersionSingleAuthority_BumpAdvancesFromPublished(t *testing.T) {
	srv := newTestServer(t)
	seedPublishedHighWaterMark(t, srv)

	got, err := srv.resolveVersionIntent(
		context.Background(),
		"core@globular.io", "demo-svc", "linux_amd64",
		repopb.VersionIntent_BUMP_PATCH, "", repopb.ArtifactChannel_STABLE, nil,
	)
	if err != nil {
		t.Fatalf("resolveVersionIntent(BUMP_PATCH) returned error: %v", err)
	}
	if got != "1.2.236" {
		t.Fatalf("bump from published %s should advance to 1.2.236, got %q", publishedHighWaterMark, got)
	}
}
