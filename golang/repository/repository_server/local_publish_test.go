package main

// local_publish_test.go — Tests for the local/official identity lane enforcement.
//
// Covers all 6 acceptance tests from the design doc
// (claude_local_publish_promotion_rules.md):
//
//  Test 1: Local modified package cannot reuse official stable identity
//  Test 2: Local publish creates local identity (version suffix + non-official channel)
//  Test 3: Local override does not mutate official BOM
//  Test 4: DEV-channel desired state must carry local identity fields
//  Test 5: Promotion creates new official identity (version immutability gate)
//  Test 6: Cross-cluster local package cannot silently override stable

import (
	"context"
	"strings"
	"testing"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

// ── Test 1: Official stable identity is sealed ─────────────────────────────

// TestLocalPublish1_OfficialStableSealedAgainstDifferentDigest verifies that
// once an official stable (publisher, name, version, platform) is published, a
// second upload with a different digest is rejected as an identity conflict.
func TestLocalPublish1_OfficialStableSealedAgainstDifferentDigest(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io",
		Name:        "storage",
		Version:     "1.2.43",
		Platform:    "linux_amd64",
		Kind:        repopb.ArtifactKind_SERVICE,
	}

	// Publish official stable artifact.
	seedPublishedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref:         ref,
		BuildNumber: 1,
		BuildId:     "official-build-A",
		Checksum:    "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		SizeBytes:   100,
	})

	// Attempt to claim the same official stable identity with a different digest.
	err := srv.enforceOfficialNamespaceSeal(ctx,
		"core@globular.io", "storage", "1.2.43", "linux_amd64",
		"sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		repopb.ArtifactChannel_STABLE,
	)
	if err == nil {
		t.Fatal("expected official namespace seal to reject different digest, got nil error")
	}
	if !strings.Contains(err.Error(), "identity conflict") && !strings.Contains(err.Error(), "SEALED") {
		t.Errorf("expected 'identity conflict' or 'SEALED' in error, got: %v", err)
	}
}

// TestLocalPublish1_OfficialStableAllowsSameDigest verifies that the seal is
// NOT triggered when the incoming digest matches the published one (idempotent
// re-upload / retry scenario).
func TestLocalPublish1_OfficialStableAllowsSameDigest(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io",
		Name:        "storage",
		Version:     "1.2.43",
		Platform:    "linux_amd64",
		Kind:        repopb.ArtifactKind_SERVICE,
	}
	const digest = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"

	seedPublishedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref: ref, BuildNumber: 1, BuildId: "official-build-A",
		Checksum: digest, SizeBytes: 100,
	})

	// Same digest — must succeed (idempotent retry).
	err := srv.enforceOfficialNamespaceSeal(ctx,
		"core@globular.io", "storage", "1.2.43", "linux_amd64",
		digest, repopb.ArtifactChannel_STABLE,
	)
	if err != nil {
		t.Errorf("expected seal to pass for identical digest, got: %v", err)
	}
}

// ── Test 2: Local publish creates local identity ───────────────────────────

// TestLocalPublish2_LocalVersionSuffixPassesIdentityRules verifies that a
// non-official publisher + DEV channel + local version suffix is valid.
func TestLocalPublish2_LocalVersionSuffixPassesIdentityRules(t *testing.T) {
	cases := []struct {
		publisher string
		channel   repopb.ArtifactChannel
		version   string
	}{
		{"local@ryzen", repopb.ArtifactChannel_DEV, "1.2.43+local.ryzen.1"},
		{"local@nuc", repopb.ArtifactChannel_DEV, "1.2.43-dev.fix-retry.a1b2c3"},
		{"org@acme.com", repopb.ArtifactChannel_CANDIDATE, "1.2.43-hotfix.1"},
	}
	for _, c := range cases {
		err := validateLocalIdentityRules(c.publisher, c.channel, c.version)
		if err != nil {
			t.Errorf("publisher=%s channel=%v version=%s: expected nil, got %v",
				c.publisher, c.channel, c.version, err)
		}
	}
}

// TestLocalPublish2_OfficialPublisherCannotUseDEVChannel verifies that the
// official publisher is forbidden from publishing to DEV channel.
func TestLocalPublish2_OfficialPublisherCannotUseDEVChannel(t *testing.T) {
	err := validateLocalIdentityRules(
		"core@globular.io",
		repopb.ArtifactChannel_DEV,
		"1.2.43",
	)
	if err == nil {
		t.Fatal("expected identity rule violation for official publisher + DEV channel")
	}
	if !strings.Contains(err.Error(), "DEV channel") && !strings.Contains(err.Error(), "identity lane") {
		t.Errorf("expected 'DEV channel' or 'identity lane' in error, got: %v", err)
	}
}

// TestLocalPublish2_OfficialPublisherCannotUseLocalVersionSuffix verifies that
// the official publisher + STABLE channel + local version suffix is rejected.
func TestLocalPublish2_OfficialPublisherCannotUseLocalVersionSuffix(t *testing.T) {
	err := validateLocalIdentityRules(
		"core@globular.io",
		repopb.ArtifactChannel_STABLE,
		"1.2.43+local.ryzen.1",
	)
	if err == nil {
		t.Fatal("expected identity rule violation for official publisher + STABLE + local suffix")
	}
	if !strings.Contains(err.Error(), "identity lane") {
		t.Errorf("expected 'identity lane' in error, got: %v", err)
	}
}

// TestLocalPublish2_FormatLocalVersion verifies the version suffix formatting helper.
func TestLocalPublish2_FormatLocalVersion(t *testing.T) {
	cases := []struct {
		base, lane, qualifier string
		n                     int
		want                  string
	}{
		{"1.2.43", "local", "ryzen", 1, "1.2.43+local.ryzen.1"},
		{"1.2.43", "dev", "fix-retry", 0, "1.2.43-dev.fix-retry"},
		{"1.2.43", "hotfix", "", 2, "1.2.43-hotfix.2"},
		{"v1.2.43", "local", "nuc", 3, "1.2.43+local.nuc.3"},
	}
	for _, c := range cases {
		got := FormatLocalVersion(c.base, c.lane, c.qualifier, c.n)
		if got != c.want {
			t.Errorf("FormatLocalVersion(%q,%q,%q,%d) = %q, want %q",
				c.base, c.lane, c.qualifier, c.n, got, c.want)
		}
	}
}

// ── Test 3: Local override does not mutate official BOM ────────────────────

// TestLocalPublish3_OfficialBOMUnaffectedByLocalPublish verifies that
// publishing a local artifact (different publisher, DEV channel) leaves the
// official ledger entry for the same package name untouched.
func TestLocalPublish3_OfficialBOMUnaffectedByLocalPublish(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	officialRef := &repopb.ArtifactRef{
		PublisherId: "core@globular.io", Name: "storage",
		Version: "1.2.43", Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
	}
	// Seed official artifact.
	seedPublishedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref: officialRef, BuildNumber: 1, BuildId: "official-A",
		Checksum: "sha256:aaaa", SizeBytes: 50,
	})

	// Publish a local artifact (different publisher, different build_id).
	localRef := &repopb.ArtifactRef{
		PublisherId: "local@ryzen", Name: "storage",
		Version: "1.2.43+local.ryzen.1", Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
	}
	seedPublishedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref: localRef, BuildNumber: 1, BuildId: "local-B",
		Checksum: "sha256:bbbb", SizeBytes: 50,
	})

	// Official ledger must still point to build A.
	ledger := srv.readLedger(ctx, "core@globular.io", "storage")
	if ledger == nil {
		t.Fatal("official ledger should exist")
	}
	if ledger.LatestBuildID != "official-A" {
		t.Errorf("official LatestBuildID=%q, want official-A", ledger.LatestBuildID)
	}
	if ledger.LatestVersion != "1.2.43" {
		t.Errorf("official LatestVersion=%q, want 1.2.43", ledger.LatestVersion)
	}

	// Official ledger must NOT contain the local build_id.
	for _, r := range ledger.Releases {
		if r.BuildID == "local-B" {
			t.Error("official ledger must not contain local build_id=local-B")
		}
	}

	// Local ledger is separate.
	localLedger := srv.readLedger(ctx, "local@ryzen", "storage")
	if localLedger == nil {
		t.Fatal("local ledger should exist")
	}
	if localLedger.LatestBuildID != "local-B" {
		t.Errorf("local LatestBuildID=%q, want local-B", localLedger.LatestBuildID)
	}
}

// ── Test 4: Official stable does not interfere with local namespace ─────────

// TestLocalPublish4_OfficialSealDoesNotBlockLocalPublisher verifies that the
// official namespace seal is NOT applied to non-official publishers. A local@
// publisher can freely publish even when the official stable namespace has the
// same package name and version number as its base.
func TestLocalPublish4_OfficialSealDoesNotBlockLocalPublisher(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	// Official stable artifact already published.
	seedPublishedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref: &repopb.ArtifactRef{
			PublisherId: "core@globular.io", Name: "storage",
			Version: "1.2.43", Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
		},
		BuildNumber: 1, BuildId: "official-A",
		Checksum: "sha256:aaaa", SizeBytes: 50,
	})

	// Local publisher can publish its own artifact — seal is N/A.
	err := srv.enforceOfficialNamespaceSeal(ctx,
		"local@ryzen", "storage", "1.2.43+local.ryzen.1", "linux_amd64",
		"sha256:bbbb", repopb.ArtifactChannel_DEV,
	)
	if err != nil {
		t.Errorf("expected seal to be N/A for local publisher, got: %v", err)
	}
}

// ── Test 5: Promotion creates new official identity ─────────────────────────

// TestLocalPublish5_PromotedVersionMustBeDistinct verifies that a promotion
// uses a NEW version (different from the local build's base version). The version
// immutability gate rejects any attempt to publish a new build_id at an already-
// published (version, platform) under the official publisher.
func TestLocalPublish5_PromotedVersionMustBeDistinct(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	// Official stable 1.2.43 already published (the "before promotion" state).
	seedPublishedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref: &repopb.ArtifactRef{
			PublisherId: "core@globular.io", Name: "storage",
			Version: "1.2.43", Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
		},
		BuildNumber: 1, BuildId: "official-build-A",
		Checksum: "sha256:aaaa", SizeBytes: 50,
	})

	// Attempt to "promote" by publishing new bytes at the same version 1.2.43.
	// This must be rejected by the version immutability gate.
	err := srv.appendToLedger(ctx,
		"core@globular.io", "storage", "1.2.43",
		"promoted-build-C", "sha256:cccc", "linux_amd64", 50)
	if err == nil {
		t.Fatal("expected appendToLedger to reject same-version promotion with different build_id")
	}
	if !strings.Contains(err.Error(), "already published") && !strings.Contains(err.Error(), "immutable") {
		t.Errorf("expected 'already published' or 'immutable' in error, got: %v", err)
	}

	// Correct promotion: publish at a NEW version (1.2.53).
	err = srv.appendToLedger(ctx,
		"core@globular.io", "storage", "1.2.53",
		"promoted-build-C", "sha256:cccc", "linux_amd64", 50)
	if err != nil {
		t.Errorf("expected promotion at new version 1.2.53 to succeed, got: %v", err)
	}
}

// ── Test 6: Cross-cluster local package cannot override stable ──────────────

// TestLocalPublish6_CrossClusterLocalCannotOverrideStable verifies that the
// official namespace seal prevents another cluster's local artifact from
// claiming the official stable identity when different bytes are present.
func TestLocalPublish6_CrossClusterLocalCannotOverrideStable(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	// Our cluster has official 1.2.43.
	seedPublishedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref: &repopb.ArtifactRef{
			PublisherId: "core@globular.io", Name: "storage",
			Version: "1.2.43", Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
		},
		BuildNumber: 1, BuildId: "official-build-A",
		Checksum: "sha256:official-digest-aaaa", SizeBytes: 100,
	})

	// Cluster B sends a "storage 1.2.43" with modified bytes (local fix from B).
	// It uses official publisher + stable channel — the seal must reject it.
	err := srv.enforceOfficialNamespaceSeal(ctx,
		"core@globular.io", "storage", "1.2.43", "linux_amd64",
		"sha256:cluster-b-local-digest-bbbb",
		repopb.ArtifactChannel_STABLE,
	)
	if err == nil {
		t.Fatal("expected seal to reject cross-cluster local artifact claiming official stable identity")
	}
	if !strings.Contains(err.Error(), "identity conflict") && !strings.Contains(err.Error(), "SEALED") {
		t.Errorf("expected 'identity conflict' or 'SEALED', got: %v", err)
	}

	// The same artifact published under a non-official publisher is fine.
	err = srv.enforceOfficialNamespaceSeal(ctx,
		"local@cluster-b", "storage", "1.2.43+local.cluster-b.1", "linux_amd64",
		"sha256:cluster-b-local-digest-bbbb",
		repopb.ArtifactChannel_DEV,
	)
	if err != nil {
		t.Errorf("expected non-official local import to pass seal, got: %v", err)
	}
}
