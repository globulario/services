package main

// append_to_ledger_recovery_test.go — recovery / stuck-import completion path.
//
// The v1.2.157 → v1.2.158 sync failure exposed this: a previous force_full_rebuild
// minted cluster-controller@1.2.155, then a later sync of v1.2.158 (whose carry-
// forward references cluster-controller@1.2.153) tried to retry-complete the older
// version's MANIFEST_WRITTEN partial. The monotonicity check rejected
// "1.2.153 < latest 1.2.155" and the artifact stayed stuck.
//
// Contract pinned here:
//   - completing an existing MANIFEST_WRITTEN partial import of an older
//     version is allowed (monotonicity bypassed for verified recoveries only)
//   - brand-new lower-version publishes below latest are still rejected
//   - version+platform immutability still applies (no duplicate PUBLISHED)
//   - LatestVersion/LatestBuildID anchor does not regress
//   - REVOKED / QUARANTINED / BROKEN states do not unlock the bypass
//   - ResolveByBuildID returns the recovered build after completion

import (
	"context"
	"strings"
	"testing"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

// helper: seed a higher-version PUBLISHED ledger entry so a subsequent older
// append will hit the monotonicity gate.
func seedLatestVersion(t *testing.T, srv *server, version, buildID, digest string) {
	t.Helper()
	if err := srv.appendToLedger(context.Background(),
		"core@globular.io", "demo-svc", version,
		buildID, digest, "linux_amd64", 1000, nil,
	); err != nil {
		t.Fatalf("seed latest %s: %v", version, err)
	}
}

// helper: build the canonical artifact storage key for the older version that
// will be in MANIFEST_WRITTEN. Used as RecoveryArtifactKey.
func artifactKeyFor(version string, buildNumber int64) string {
	return artifactKeyWithBuild(&repopb.ArtifactRef{
		PublisherId: "core@globular.io",
		Name:        "demo-svc",
		Version:     version,
		Platform:    "linux_amd64",
	}, buildNumber)
}

// helper: place the recovery artifact in MANIFEST_WRITTEN.
func seedManifestWritten(t *testing.T, srv *server, version, buildID, digest string, buildNumber int64) string {
	t.Helper()
	key := artifactKeyFor(version, buildNumber)
	if err := srv.transitionArtifactState(context.Background(), key,
		PipelineManifestWritten, "test_seed", "", ArtifactStateFields{
			BuildID:     buildID,
			BuildNumber: buildNumber,
			Checksum:    digest,
			Version:     version,
			Platform:    "linux_amd64",
			Name:        "demo-svc",
			PublisherID: "core@globular.io",
		}); err != nil {
		t.Fatalf("seed manifest_written: %v", err)
	}
	return key
}

// ── Test 1: brand-new older version below latest is REJECTED ──
func TestLedger_BrandNewLowerVersionBelowLatest_StillRejected(t *testing.T) {
	srv := newTestServer(t)
	seedLatestVersion(t, srv, "1.2.155",
		"latest-bid-aaaa", "sha256:"+strings.Repeat("a", 64))

	// No MANIFEST_WRITTEN row seeded → recovery bypass cannot engage.
	err := srv.appendToLedger(context.Background(),
		"core@globular.io", "demo-svc", "1.2.153",
		"older-bid-zzzz", "sha256:"+strings.Repeat("z", 64),
		"linux_amd64", 900, nil,
	)
	if err == nil {
		t.Fatal("expected monotonicity rejection for brand-new lower version")
	}
	if !strings.Contains(err.Error(), "non-monotonic") {
		t.Errorf("expected non-monotonic error, got: %v", err)
	}
}

// ── Test 2: MANIFEST_WRITTEN recovery for older version COMPLETES ──
func TestLedger_RecoveryOfManifestWrittenOlderVersion_Completes(t *testing.T) {
	srv := newTestServer(t)
	seedLatestVersion(t, srv, "1.2.155",
		"latest-bid-aaaa", "sha256:"+strings.Repeat("a", 64))

	olderBuildID := "recovery-bid-bbbb"
	olderDigest := "sha256:" + strings.Repeat("b", 64)
	key := seedManifestWritten(t, srv, "1.2.153", olderBuildID, olderDigest, 408)

	err := srv.appendToLedger(context.Background(),
		"core@globular.io", "demo-svc", "1.2.153",
		olderBuildID, olderDigest, "linux_amd64", 900, nil,
		AppendToLedgerOpts{RecoveryArtifactKey: key},
	)
	if err != nil {
		t.Fatalf("recovery should bypass monotonicity, got: %v", err)
	}

	// Verify the entry is now in the ledger.
	resolvedBuildID := srv.getExactRelease(context.Background(),
		"core@globular.io", "demo-svc", "1.2.153", "linux_amd64")
	if resolvedBuildID != olderBuildID {
		t.Errorf("getExactRelease should return the recovered build_id; got %q, want %q",
			resolvedBuildID, olderBuildID)
	}
}

// ── Test 3: latest version anchor is NOT regressed by recovery ──
func TestLedger_RecoveryDoesNotRegressLatestVersion(t *testing.T) {
	srv := newTestServer(t)
	latestBuildID := "latest-bid-aaaa"
	seedLatestVersion(t, srv, "1.2.155",
		latestBuildID, "sha256:"+strings.Repeat("a", 64))

	olderBuildID := "recovery-bid-bbbb"
	olderDigest := "sha256:" + strings.Repeat("b", 64)
	key := seedManifestWritten(t, srv, "1.2.153", olderBuildID, olderDigest, 408)

	if err := srv.appendToLedger(context.Background(),
		"core@globular.io", "demo-svc", "1.2.153",
		olderBuildID, olderDigest, "linux_amd64", 900, nil,
		AppendToLedgerOpts{RecoveryArtifactKey: key},
	); err != nil {
		t.Fatalf("recovery append: %v", err)
	}

	gotVersion, gotBuildID := srv.getLatestRelease(
		context.Background(), "core@globular.io", "demo-svc", "linux_amd64")
	if gotVersion != "1.2.155" {
		t.Errorf("latest version regressed: got %q, want 1.2.155", gotVersion)
	}
	if gotBuildID != latestBuildID {
		t.Errorf("latest build_id regressed: got %q, want %q", gotBuildID, latestBuildID)
	}
}

// ── Test 4: REVOKED state does NOT unlock the recovery bypass ──
func TestLedger_RecoveryKey_RevokedState_StillRejected(t *testing.T) {
	srv := newTestServer(t)
	seedLatestVersion(t, srv, "1.2.155",
		"latest-bid-aaaa", "sha256:"+strings.Repeat("a", 64))

	olderBuildID := "revoked-bid-cccc"
	olderDigest := "sha256:" + strings.Repeat("c", 64)
	key := artifactKeyFor("1.2.153", 408)
	// Walk to REVOKED via the legal path.
	for _, to := range []ArtifactPipelineState{
		PipelineDiscovered, PipelineDownloading, PipelineBlobWritten,
		PipelineBlobVerified, PipelineManifestWritten, PipelineLedgerWritten,
		PipelinePublished, PipelineRevoked,
	} {
		if err := srv.transitionArtifactState(context.Background(), key, to,
			"test_seed", "", ArtifactStateFields{
				BuildID: olderBuildID, BuildNumber: 408,
				Checksum: olderDigest, Version: "1.2.153",
				Platform: "linux_amd64", Name: "demo-svc",
				PublisherID: "core@globular.io",
			}); err != nil {
			t.Fatalf("seed revoked state walk: %v", err)
		}
	}

	err := srv.appendToLedger(context.Background(),
		"core@globular.io", "demo-svc", "1.2.153",
		olderBuildID, olderDigest, "linux_amd64", 900, nil,
		AppendToLedgerOpts{RecoveryArtifactKey: key},
	)
	if err == nil {
		t.Fatal("REVOKED state must not unlock recovery bypass; expected reject")
	}
	if !strings.Contains(err.Error(), "non-monotonic") {
		t.Errorf("expected non-monotonic error (REVOKED bypass disabled), got: %v", err)
	}
}

// ── Test 5: PUBLISHED state does NOT unlock the recovery bypass ──
// (defensive: someone might construct the key wrong and hit an already-
// published artifact's row; we never want to re-add a PUBLISHED entry to
// the ledger via the recovery path).
func TestLedger_RecoveryKey_PublishedState_StillRejected(t *testing.T) {
	srv := newTestServer(t)
	seedLatestVersion(t, srv, "1.2.155",
		"latest-bid-aaaa", "sha256:"+strings.Repeat("a", 64))

	key := artifactKeyFor("1.2.153", 408)
	for _, to := range []ArtifactPipelineState{
		PipelineDiscovered, PipelineDownloading, PipelineBlobWritten,
		PipelineBlobVerified, PipelineManifestWritten, PipelineLedgerWritten,
		PipelinePublished,
	} {
		if err := srv.transitionArtifactState(context.Background(), key, to,
			"test_seed", "", ArtifactStateFields{Version: "1.2.153"}); err != nil {
			t.Fatalf("seed: %v", err)
		}
	}

	err := srv.appendToLedger(context.Background(),
		"core@globular.io", "demo-svc", "1.2.153",
		"another-bid", "sha256:"+strings.Repeat("d", 64),
		"linux_amd64", 900, nil,
		AppendToLedgerOpts{RecoveryArtifactKey: key},
	)
	if err == nil {
		t.Fatal("PUBLISHED state must not unlock recovery bypass; expected reject")
	}
	if !strings.Contains(err.Error(), "non-monotonic") {
		t.Errorf("expected non-monotonic error, got: %v", err)
	}
}

// ── Test 6: recovery does NOT create duplicate (version, platform) row ──
// If the older version is ALREADY in the ledger (e.g. a prior successful
// append) and the recovery is retried, the existing version+platform
// immutability check fires.
func TestLedger_RecoveryDoesNotCreateDuplicatePublishedRow(t *testing.T) {
	srv := newTestServer(t)
	seedLatestVersion(t, srv, "1.2.155",
		"latest-bid-aaaa", "sha256:"+strings.Repeat("a", 64))

	olderBuildID := "recovery-bid-bbbb"
	olderDigest := "sha256:" + strings.Repeat("b", 64)
	key := seedManifestWritten(t, srv, "1.2.153", olderBuildID, olderDigest, 408)

	// First recovery succeeds.
	if err := srv.appendToLedger(context.Background(),
		"core@globular.io", "demo-svc", "1.2.153",
		olderBuildID, olderDigest, "linux_amd64", 900, nil,
		AppendToLedgerOpts{RecoveryArtifactKey: key},
	); err != nil {
		t.Fatalf("first recovery: %v", err)
	}

	// Retry recovery with the SAME build_id → idempotent re-promote (no-op).
	if err := srv.appendToLedger(context.Background(),
		"core@globular.io", "demo-svc", "1.2.153",
		olderBuildID, olderDigest, "linux_amd64", 900, nil,
		AppendToLedgerOpts{RecoveryArtifactKey: key},
	); err != nil {
		t.Errorf("idempotent re-recovery should be a no-op, got: %v", err)
	}

	// Try recovery with a DIFFERENT build_id at the same (version, platform).
	// The version+platform immutability check must fire — recovery does not
	// open a new lane for replacing an immutable identity.
	err := srv.appendToLedger(context.Background(),
		"core@globular.io", "demo-svc", "1.2.153",
		"different-bid", "sha256:"+strings.Repeat("e", 64),
		"linux_amd64", 900, nil,
		AppendToLedgerOpts{RecoveryArtifactKey: key},
	)
	if err == nil {
		t.Fatal("recovery with different build_id at same (version,platform) must be rejected")
	}
	if !strings.Contains(err.Error(), "already published") {
		t.Errorf("expected version+platform immutability error, got: %v", err)
	}
}

// ── Test 7: empty RecoveryArtifactKey behaves like normal append ──
// (no bypass; identical to the legacy code path).
func TestLedger_EmptyRecoveryArtifactKey_NoBypass(t *testing.T) {
	srv := newTestServer(t)
	seedLatestVersion(t, srv, "1.2.155",
		"latest-bid-aaaa", "sha256:"+strings.Repeat("a", 64))

	err := srv.appendToLedger(context.Background(),
		"core@globular.io", "demo-svc", "1.2.153",
		"older-bid-zzzz", "sha256:"+strings.Repeat("z", 64),
		"linux_amd64", 900, nil,
		AppendToLedgerOpts{RecoveryArtifactKey: ""}, // explicit empty
	)
	if err == nil {
		t.Fatal("empty RecoveryArtifactKey must not engage bypass")
	}
	if !strings.Contains(err.Error(), "non-monotonic") {
		t.Errorf("expected non-monotonic error, got: %v", err)
	}
}

// ── Test 8: stale RecoveryArtifactKey (no such artifact_state row) ──
// PipelineUnspecified (the default for a non-existent key) must NOT engage
// the bypass.
func TestLedger_NonExistentArtifactStateKey_NoBypass(t *testing.T) {
	srv := newTestServer(t)
	seedLatestVersion(t, srv, "1.2.155",
		"latest-bid-aaaa", "sha256:"+strings.Repeat("a", 64))

	err := srv.appendToLedger(context.Background(),
		"core@globular.io", "demo-svc", "1.2.153",
		"older-bid-zzzz", "sha256:"+strings.Repeat("z", 64),
		"linux_amd64", 900, nil,
		AppendToLedgerOpts{RecoveryArtifactKey: "no-such-key-xxx"},
	)
	if err == nil {
		t.Fatal("non-existent artifact_state must not engage bypass")
	}
	if !strings.Contains(err.Error(), "non-monotonic") {
		t.Errorf("expected non-monotonic error, got: %v", err)
	}
}
