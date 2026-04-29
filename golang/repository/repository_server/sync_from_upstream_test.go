package main

import (
	"context"
	"strings"
	"testing"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
	"github.com/globulario/services/golang/repository/upstream"
)

// testProvider returns a no-op provider and empty opts for unit tests that
// exercise policy/conflict/skip paths and never reach actual artifact download.
func testProvider() (upstream.ReleaseSource, upstream.SourceOpts) {
	src, _ := upstream.NewSource(upstream.TypeHTTPIndex)
	return src, upstream.SourceOpts{}
}

func TestProcessSyncEntrySkipsExistingDigestWithDifferentBuildNumber(t *testing.T) {
	srv := newTestServer(t)
	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io",
		Name:        "workflow",
		Version:     "1.0.53",
		Platform:    "linux_amd64",
		Kind:        repopb.ArtifactKind_SERVICE,
	}
	seedPublishedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref:         ref,
		BuildNumber: 1,
		BuildId:     "019d0001-0000-7000-8000-000000000001",
		Checksum:    "sha256:same-content",
		SizeBytes:   100,
	})

	prov, pOpts := testProvider()
	result := srv.processSyncEntry(
		context.Background(),
		&releaseIndexEntry{
			Name:          "workflow",
			Publisher:     "core@globular.io",
			Version:       "1.0.53",
			BuildID:       "67",
			BuildNumber:   67,
			Platform:      "linux_amd64",
			PackageDigest: "sha256:same-content",
			AssetURL:      "https://example.invalid/workflow.tgz",
		},
		&repopb.UpstreamSource{Name: "test-source"},
		prov, pOpts,
		"v1.0.53",
		false,
	)

	if result.GetStatus() != repopb.UpstreamSyncStatus_SYNC_SKIPPED {
		t.Fatalf("expected SYNC_SKIPPED, got %s: %s", result.GetStatus().String(), result.GetDetail())
	}
	if result.GetDetail() == "" {
		t.Fatal("expected detail explaining the existing artifact")
	}
}

// ── Policy rejection tests ──────────────────────────────────────────────────

func TestCheckImportPolicy_AllowedPublishers(t *testing.T) {
	n := &normalizedEntry{Name: "echo", Kind: "SERVICE", Publisher: "evil@attacker.io", Channel: "stable"}
	src := &repopb.UpstreamSource{AllowedPublishers: []string{"trusted@globular.io"}}

	reason, rejected := checkImportPolicy(n, src)
	if !rejected {
		t.Fatal("expected rejection for disallowed publisher")
	}
	if !strings.Contains(reason, "allowed_publishers") {
		t.Fatalf("expected allowed_publishers in reason, got: %s", reason)
	}

	n.Publisher = "trusted@globular.io"
	_, rejected = checkImportPolicy(n, src)
	if rejected {
		t.Fatal("trusted publisher should not be rejected")
	}
}

func TestCheckImportPolicy_AllowedKinds(t *testing.T) {
	n := &normalizedEntry{Name: "echo", Kind: "APPLICATION", Channel: "stable"}
	src := &repopb.UpstreamSource{AllowedKinds: []string{"SERVICE", "INFRASTRUCTURE"}}

	reason, rejected := checkImportPolicy(n, src)
	if !rejected {
		t.Fatal("expected rejection for disallowed kind")
	}
	if !strings.Contains(reason, "allowed_kinds") {
		t.Fatalf("expected allowed_kinds in reason, got: %s", reason)
	}

	n.Kind = "SERVICE"
	_, rejected = checkImportPolicy(n, src)
	if rejected {
		t.Fatal("SERVICE kind should be allowed")
	}
}

func TestCheckImportPolicy_AllowedChannels_EntryChannel(t *testing.T) {
	// Policy checks the normalized entry channel (from entry.Channel), not source.Channel.
	n := &normalizedEntry{Name: "echo", Kind: "SERVICE", Channel: "candidate"}
	src := &repopb.UpstreamSource{AllowedChannels: []string{"stable"}}

	reason, rejected := checkImportPolicy(n, src)
	if !rejected {
		t.Fatal("expected rejection for candidate when only stable allowed")
	}
	if !strings.Contains(reason, "allowed_channels") {
		t.Fatalf("expected allowed_channels in reason, got: %s", reason)
	}

	// stable channel should pass
	n.Channel = "stable"
	_, rejected = checkImportPolicy(n, src)
	if rejected {
		t.Fatal("stable channel should be allowed")
	}
}

func TestCheckImportPolicy_ChannelDefaultsToStable(t *testing.T) {
	// Entry with no channel should default to "stable" after normalization.
	entry := &releaseIndexEntry{Name: "echo", Kind: "SERVICE", Version: "1.0.0", Platform: "linux_amd64",
		PackageDigest: "sha256:0000000000000000000000000000000000000000000000000000000000000000",
		AssetURL: "https://example.com/echo.tgz"}
	src := &repopb.UpstreamSource{AllowedChannels: []string{"stable"}}

	n := normalizeReleaseEntry(entry, src)
	if n.Channel != "stable" {
		t.Fatalf("expected default channel 'stable', got %q", n.Channel)
	}

	_, rejected := checkImportPolicy(n, src)
	if rejected {
		t.Fatal("default stable channel should not be rejected when stable is allowed")
	}
}

func TestCheckImportPolicy_RequireChecksum(t *testing.T) {
	n := &normalizedEntry{Name: "echo", Kind: "SERVICE", Channel: "stable", Digest: ""}
	src := &repopb.UpstreamSource{RequireChecksum: true}

	reason, rejected := checkImportPolicy(n, src)
	if !rejected {
		t.Fatal("expected rejection for missing checksum")
	}
	if !strings.Contains(reason, "require_checksum") {
		t.Fatalf("expected require_checksum in reason, got: %s", reason)
	}

	n.Digest = "sha256:abc123"
	_, rejected = checkImportPolicy(n, src)
	if rejected {
		t.Fatal("should not reject when checksum is present")
	}
}

func TestCheckImportPolicy_NoRestrictions(t *testing.T) {
	n := &normalizedEntry{Name: "echo", Kind: "SERVICE", Channel: "stable"}
	src := &repopb.UpstreamSource{}

	_, rejected := checkImportPolicy(n, src)
	if rejected {
		t.Fatal("unrestricted source should not reject anything")
	}
}

// ── Deterministic build ID tests ────────────────────────────────────────────

func TestDeriveUpstreamBuildID_Deterministic(t *testing.T) {
	id1 := deriveUpstreamBuildID("core@globular.io", "echo", "1.0.0", "linux_amd64", "sha256:abc")
	id2 := deriveUpstreamBuildID("core@globular.io", "echo", "1.0.0", "linux_amd64", "sha256:abc")
	if id1 != id2 {
		t.Fatalf("expected deterministic build_id, got %q and %q", id1, id2)
	}
	if !strings.HasPrefix(id1, "upstream:") {
		t.Fatalf("expected upstream: prefix, got %q", id1)
	}
}

func TestDeriveUpstreamBuildID_DifferentInputs(t *testing.T) {
	id1 := deriveUpstreamBuildID("core@globular.io", "echo", "1.0.0", "linux_amd64", "sha256:abc")
	id2 := deriveUpstreamBuildID("core@globular.io", "rbac", "1.0.0", "linux_amd64", "sha256:abc")
	if id1 == id2 {
		t.Fatal("different names should produce different build IDs")
	}

	id3 := deriveUpstreamBuildID("core@globular.io", "echo", "1.0.0", "linux_amd64", "sha256:def")
	if id1 == id3 {
		t.Fatal("different digests should produce different build IDs")
	}
}

// ── build_number from release-index tests ───────────────────────────────────

func TestNormalizeReleaseEntry_ExplicitBuildNumber(t *testing.T) {
	entry := &releaseIndexEntry{
		Name: "echo", Version: "1.0.0", Platform: "linux_amd64",
		BuildNumber: 42, BuildID: "run-42",
		PackageDigest: "sha256:0000000000000000000000000000000000000000000000000000000000000000",
		AssetURL: "https://example.com/echo.tgz",
	}
	n := normalizeReleaseEntry(entry, &repopb.UpstreamSource{})
	if n.BuildNumber != 42 {
		t.Fatalf("expected build_number=42, got %d", n.BuildNumber)
	}
	if n.BuildID != "run-42" {
		t.Fatalf("expected build_id=run-42, got %q", n.BuildID)
	}
}

func TestNormalizeReleaseEntry_MissingBuildNumberDerived(t *testing.T) {
	entry := &releaseIndexEntry{
		Name: "echo", Version: "1.0.0", Platform: "linux_amd64",
		PackageDigest: "sha256:0000000000000000000000000000000000000000000000000000000000000000",
		AssetURL: "https://example.com/echo.tgz",
	}
	n := normalizeReleaseEntry(entry, &repopb.UpstreamSource{})
	if n.BuildNumber <= 0 {
		t.Fatalf("expected positive derived build_number, got %d", n.BuildNumber)
	}
	if n.BuildID == "" {
		t.Fatal("expected derived build_id")
	}
}

func TestNormalizeReleaseEntry_TwoMissingBuildIDsNoCollision(t *testing.T) {
	entry1 := &releaseIndexEntry{
		Name: "echo", Version: "1.0.0", Platform: "linux_amd64",
		PackageDigest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		AssetURL: "https://example.com/echo.tgz",
	}
	entry2 := &releaseIndexEntry{
		Name: "rbac", Version: "1.0.0", Platform: "linux_amd64",
		PackageDigest: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		AssetURL: "https://example.com/rbac.tgz",
	}
	src := &repopb.UpstreamSource{}
	n1 := normalizeReleaseEntry(entry1, src)
	n2 := normalizeReleaseEntry(entry2, src)
	if n1.BuildID == n2.BuildID {
		t.Fatal("two different packages with missing build_id should not have the same derived build_id")
	}
	if n1.BuildNumber == n2.BuildNumber {
		t.Fatalf("two different packages should not collide on build_number: %d", n1.BuildNumber)
	}
}

func TestNormalizeReleaseEntry_NonNumericBuildID(t *testing.T) {
	entry := &releaseIndexEntry{
		Name: "echo", Version: "1.0.0", Platform: "linux_amd64",
		BuildID: "ci-run-abc-123",
		PackageDigest: "sha256:0000000000000000000000000000000000000000000000000000000000000000",
		AssetURL: "https://example.com/echo.tgz",
	}
	n := normalizeReleaseEntry(entry, &repopb.UpstreamSource{})
	if n.BuildID != "ci-run-abc-123" {
		t.Fatalf("expected preserved build_id, got %q", n.BuildID)
	}
	// build_number should be derived since 0 was the default (no explicit build_number)
	if n.BuildNumber <= 0 {
		t.Fatalf("expected positive derived build_number for non-numeric build_id, got %d", n.BuildNumber)
	}
}

func TestDeriveBuildNumber_AlwaysPositive(t *testing.T) {
	for _, input := range []string{"a", "b", "c", "build-100", "upstream:abc123"} {
		bn := deriveBuildNumber(input, "sha256:test")
		if bn <= 0 {
			t.Fatalf("deriveBuildNumber(%q) returned %d, expected > 0", input, bn)
		}
	}
}

// ── Quarantine trust policy tests ───────────────────────────────────────────

func TestImportTargetState_Default(t *testing.T) {
	src := &repopb.UpstreamSource{}
	if s := importTargetState(src); s != repopb.PublishState_PUBLISHED {
		t.Fatalf("expected PUBLISHED, got %s", s)
	}
}

func TestImportTargetState_Import(t *testing.T) {
	src := &repopb.UpstreamSource{TrustPolicy: "import"}
	if s := importTargetState(src); s != repopb.PublishState_PUBLISHED {
		t.Fatalf("expected PUBLISHED, got %s", s)
	}
}

func TestImportTargetState_Quarantine(t *testing.T) {
	src := &repopb.UpstreamSource{TrustPolicy: "quarantine"}
	if s := importTargetState(src); s != repopb.PublishState_QUARANTINED {
		t.Fatalf("expected QUARANTINED, got %s", s)
	}
}

func TestImportTargetState_QuarantineCaseInsensitive(t *testing.T) {
	src := &repopb.UpstreamSource{TrustPolicy: "QUARANTINE"}
	if s := importTargetState(src); s != repopb.PublishState_QUARANTINED {
		t.Fatalf("expected QUARANTINED, got %s", s)
	}
}

// ── Credential redaction tests ──────────────────────────────────────────────

func TestResolveCredentialFromEtcd_RejectsUnsafePrefix(t *testing.T) {
	_, err := resolveCredentialFromEtcd(context.Background(), "/etc/passwd")
	if err == nil {
		t.Fatal("expected error for unsafe prefix")
	}
	if !strings.Contains(err.Error(), "/globular/credentials/") {
		t.Fatalf("expected prefix error, got: %v", err)
	}
}

func TestResolveCredentialFromEtcd_RejectsArbitraryEtcdKey(t *testing.T) {
	_, err := resolveCredentialFromEtcd(context.Background(), "/globular/system/config")
	if err == nil {
		t.Fatal("expected error for key outside credentials prefix")
	}
}

// ── Upstream fallback policy tests ──────────────────────────────────────────

func TestUpstreamFallbackAllowed_NoUpstreamImport(t *testing.T) {
	srv := newTestServer(t)
	manifest := &repopb.ArtifactManifest{
		Ref:      &repopb.ArtifactRef{Name: "echo"},
		Checksum: "sha256:abc",
	}
	if srv.upstreamFallbackAllowed(context.Background(), manifest) {
		t.Fatal("should not allow fallback without upstream_import")
	}
}

func TestUpstreamFallbackAllowed_MissingChecksum(t *testing.T) {
	srv := newTestServer(t)
	manifest := &repopb.ArtifactManifest{
		Ref: &repopb.ArtifactRef{Name: "echo"},
		UpstreamImport: &repopb.UpstreamImportRecord{
			SourceName: "test", AssetUrl: "https://example.com/echo.tgz",
		},
	}
	if srv.upstreamFallbackAllowed(context.Background(), manifest) {
		t.Fatal("should not allow fallback without checksum")
	}
}

// ── NormalizeReleaseEntry channel tests ──────────────────────────────────────

func TestNormalizeReleaseEntry_ChannelFromEntry(t *testing.T) {
	entry := &releaseIndexEntry{
		Name: "echo", Version: "1.0.0", Platform: "linux_amd64",
		Channel: "candidate",
		PackageDigest: "sha256:0000000000000000000000000000000000000000000000000000000000000000",
		AssetURL: "https://example.com/echo.tgz",
	}
	n := normalizeReleaseEntry(entry, &repopb.UpstreamSource{Channel: "stable"})
	if n.Channel != "candidate" {
		t.Fatalf("expected entry channel 'candidate' to win, got %q", n.Channel)
	}
}

func TestNormalizeReleaseEntry_ChannelFallsBackToSource(t *testing.T) {
	entry := &releaseIndexEntry{
		Name: "echo", Version: "1.0.0", Platform: "linux_amd64",
		PackageDigest: "sha256:0000000000000000000000000000000000000000000000000000000000000000",
		AssetURL: "https://example.com/echo.tgz",
	}
	n := normalizeReleaseEntry(entry, &repopb.UpstreamSource{Channel: "candidate"})
	if n.Channel != "candidate" {
		t.Fatalf("expected source channel 'candidate' as fallback, got %q", n.Channel)
	}
}

// ── ContainsFold tests ──────────────────────────────────────────────────────

func TestContainsFold(t *testing.T) {
	if !containsFold([]string{"SERVICE", "INFRASTRUCTURE"}, "service") {
		t.Fatal("expected case-insensitive match")
	}
	if containsFold([]string{"SERVICE"}, "APPLICATION") {
		t.Fatal("should not match different value")
	}
	if containsFold(nil, "anything") {
		t.Fatal("nil slice should not match")
	}
}

// ── Strict --latest semantics tests ─────────────────────────────────────────

func TestSyncFromUpstream_TagAndResolveLatest_InvalidArgument(t *testing.T) {
	srv := newTestServer(t)
	_, err := srv.SyncFromUpstream(context.Background(), &repopb.SyncFromUpstreamRequest{
		SourceName:    "test",
		ReleaseTag:    "v1.0.0",
		ResolveLatest: true,
	})
	if err == nil {
		t.Fatal("expected InvalidArgument when both tag and resolve_latest are set")
	}
	if !strings.Contains(err.Error(), "cannot use both") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSyncFromUpstream_EmptyTagNoResolveLatest_InvalidArgument(t *testing.T) {
	srv := newTestServer(t)
	_, err := srv.SyncFromUpstream(context.Background(), &repopb.SyncFromUpstreamRequest{
		SourceName:    "test",
		ReleaseTag:    "",
		ResolveLatest: false,
	})
	if err == nil {
		t.Fatal("expected InvalidArgument when tag is empty and resolve_latest is false")
	}
	if !strings.Contains(err.Error(), "release_tag is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ── Enriched result tests ───────────────────────────────────────────────────

func TestProcessSyncEntry_PopulatesRichFields(t *testing.T) {
	srv := newTestServer(t)
	entry := &releaseIndexEntry{
		Name: "echo", Kind: "SERVICE", Publisher: "core@globular.io",
		Version: "1.0.0", BuildNumber: 42, BuildID: "42",
		Platform: "linux_amd64", Channel: "stable",
		PackageDigest: "sha256:0000000000000000000000000000000000000000000000000000000000000000",
		AssetURL:      "https://example.com/echo.tgz",
	}
	src := &repopb.UpstreamSource{Name: "test-source"}

	prov, pOpts := testProvider()
	result := srv.processSyncEntry(context.Background(), entry, src, prov, pOpts, "v1.0.0", true)

	if result.Publisher != "core@globular.io" {
		t.Fatalf("publisher: got %q", result.Publisher)
	}
	if result.Kind != "SERVICE" {
		t.Fatalf("kind: got %q", result.Kind)
	}
	if result.Channel != "stable" {
		t.Fatalf("channel: got %q", result.Channel)
	}
	if result.BuildNumber != 42 {
		t.Fatalf("build_number: got %d", result.BuildNumber)
	}
	if !result.ChecksumPresent {
		t.Fatal("checksum_present should be true")
	}
}

func TestProcessSyncEntry_ActionBlocked(t *testing.T) {
	srv := newTestServer(t)
	entry := &releaseIndexEntry{
		Name: "echo", Kind: "APPLICATION", Version: "1.0.0",
		BuildNumber: 1, BuildID: "1", Platform: "linux_amd64",
		PackageDigest: "sha256:0000000000000000000000000000000000000000000000000000000000000000",
		AssetURL:      "https://example.com/echo.tgz",
	}
	src := &repopb.UpstreamSource{
		Name:         "test-source",
		AllowedKinds: []string{"SERVICE"},
	}

	prov, pOpts := testProvider()
	result := srv.processSyncEntry(context.Background(), entry, src, prov, pOpts, "v1.0.0", true)
	if result.Action != "blocked" {
		t.Fatalf("expected action=blocked, got %q", result.Action)
	}
	if result.BlockedReason == "" {
		t.Fatal("expected blocked_reason to be set")
	}
}

func TestProcessSyncEntry_ActionNew(t *testing.T) {
	srv := newTestServer(t)
	entry := &releaseIndexEntry{
		Name: "newpkg", Kind: "SERVICE", Version: "1.0.0",
		BuildNumber: 1, BuildID: "1", Platform: "linux_amd64",
		PackageDigest: "sha256:0000000000000000000000000000000000000000000000000000000000000000",
		AssetURL:      "https://example.com/newpkg.tgz",
	}
	src := &repopb.UpstreamSource{Name: "test-source"}

	prov, pOpts := testProvider()
	result := srv.processSyncEntry(context.Background(), entry, src, prov, pOpts, "v1.0.0", true)
	if result.Action != "new" {
		t.Fatalf("expected action=new, got %q", result.Action)
	}
}

// ── BOM composition tests ───────────────────────────────────────────────────

func TestProcessSyncEntry_UnchangedPackagePreservesVersion(t *testing.T) {
	// An unchanged package from origin release v1.0.82 referenced in platform
	// release v1.0.84 should preserve its original version (1.0.82).
	srv := newTestServer(t)
	unchanged := false
	entry := &releaseIndexEntry{
		Name: "gateway", Kind: "SERVICE", Publisher: "core@globular.io",
		Version: "1.0.82", BuildNumber: 9, BuildID: "9",
		Platform:         "linux_amd64",
		PackageDigest:    "sha256:0000000000000000000000000000000000000000000000000000000000000000",
		AssetURL:         "https://example.com/v1.0.82/gateway.tgz",
		ReleaseTag:       "v1.0.84",
		OriginRelease:    "v1.0.82",
		ChangedInRelease: &unchanged,
	}
	src := &repopb.UpstreamSource{Name: "test-source"}

	prov, pOpts := testProvider()
		result := srv.processSyncEntry(context.Background(), entry, src, prov, pOpts, "v1.0.84", true)
	// Version should be the package version (1.0.82), not the platform release (1.0.84).
	if result.Version != "1.0.82" {
		t.Fatalf("expected package version 1.0.82, got %q", result.Version)
	}
	if result.Action == "blocked" {
		t.Fatalf("unchanged cross-release package should not be blocked: %s", result.BlockedReason)
	}
}

func TestProcessSyncEntry_MixedVersionRelease(t *testing.T) {
	// Platform release v1.0.84 contains repository@1.0.84 (changed) and
	// gateway@1.0.82 (unchanged). Both should process without conflict.
	srv := newTestServer(t)
	changed := true
	unchanged := false

	entries := []*releaseIndexEntry{
		{
			Name: "repository", Kind: "SERVICE", Publisher: "core@globular.io",
			Version: "1.0.84", BuildNumber: 24, BuildID: "24",
			Platform: "linux_amd64",
			PackageDigest: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
			AssetURL:      "https://example.com/v1.0.84/repository.tgz",
			ReleaseTag:    "v1.0.84", OriginRelease: "v1.0.84",
			ChangedInRelease: &changed,
		},
		{
			Name: "gateway", Kind: "SERVICE", Publisher: "core@globular.io",
			Version: "1.0.82", BuildNumber: 9, BuildID: "9",
			Platform: "linux_amd64",
			PackageDigest: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
			AssetURL:      "https://example.com/v1.0.82/gateway.tgz",
			ReleaseTag:    "v1.0.84", OriginRelease: "v1.0.82",
			ChangedInRelease: &unchanged,
		},
	}
	src := &repopb.UpstreamSource{Name: "test-source"}

	for _, entry := range entries {
		prov, pOpts := testProvider()
		result := srv.processSyncEntry(context.Background(), entry, src, prov, pOpts, "v1.0.84", true)
		if result.Status == repopb.UpstreamSyncStatus_SYNC_WOULD_REJECT {
			t.Fatalf("package %s should not be rejected: %s", entry.Name, result.Detail)
		}
		if result.Version != entry.Version {
			t.Fatalf("package %s version: expected %q, got %q", entry.Name, entry.Version, result.Version)
		}
	}
}

func TestSameArtifactMultipleReleases_NoConflict(t *testing.T) {
	// Same package identity + same sha256 referenced by two platform releases
	// is valid (idempotent skip), not a conflict.
	srv := newTestServer(t)
	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io", Name: "gateway",
		Version: "1.0.82", Platform: "linux_amd64",
		Kind: repopb.ArtifactKind_SERVICE,
	}
	seedPublishedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref: ref, BuildNumber: 9, BuildId: "9",
		Checksum: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		SizeBytes: 100,
	})

	// Same artifact referenced from a new platform release v1.0.85.
	unchanged := false
	entry := &releaseIndexEntry{
		Name: "gateway", Kind: "SERVICE", Publisher: "core@globular.io",
		Version: "1.0.82", BuildNumber: 9, BuildID: "9",
		Platform:         "linux_amd64",
		PackageDigest:    "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		AssetURL:         "https://example.com/v1.0.82/gateway.tgz",
		ReleaseTag:       "v1.0.85",
		OriginRelease:    "v1.0.82",
		ChangedInRelease: &unchanged,
	}
	src := &repopb.UpstreamSource{Name: "test-source"}
	prov, pOpts := testProvider()
	result := srv.processSyncEntry(context.Background(), entry, src, prov, pOpts, "v1.0.85", false)
	if result.Status == repopb.UpstreamSyncStatus_SYNC_REJECTED {
		t.Fatalf("same artifact referenced by another release should skip, not reject: %s", result.Detail)
	}
	if result.Action != "up_to_date" {
		t.Fatalf("expected up_to_date, got %q", result.Action)
	}
}

func TestSamePackageIdentityDifferentSha256_Conflict(t *testing.T) {
	// Same (name, version, build_id, platform) but different sha256 = conflict.
	srv := newTestServer(t)
	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io", Name: "gateway",
		Version: "1.0.82", Platform: "linux_amd64",
		Kind: repopb.ArtifactKind_SERVICE,
	}
	seedPublishedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref: ref, BuildNumber: 9, BuildId: "9",
		Checksum: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		SizeBytes: 100,
	})

	entry := &releaseIndexEntry{
		Name: "gateway", Kind: "SERVICE", Publisher: "core@globular.io",
		Version: "1.0.82", BuildNumber: 9, BuildID: "9",
		Platform:      "linux_amd64",
		PackageDigest: "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
		AssetURL:      "https://example.com/v1.0.82/gateway.tgz",
		ReleaseTag:    "v1.0.85",
	}
	src := &repopb.UpstreamSource{Name: "test-source"}
	prov, pOpts := testProvider()
	result := srv.processSyncEntry(context.Background(), entry, src, prov, pOpts, "v1.0.85", false)
	if result.Status != repopb.UpstreamSyncStatus_SYNC_REJECTED {
		t.Fatalf("expected SYNC_REJECTED for different sha256, got %s: %s", result.Status, result.Detail)
	}
	if result.Action != "conflict" {
		t.Fatalf("expected action=conflict, got %q", result.Action)
	}
}

// ── Provider-neutral import path tests ──────────────────────────────────────

func TestProcessSyncEntry_AssetPathOnly_DryRun(t *testing.T) {
	// An entry with only asset_path (no asset_url) should produce a valid
	// dry-run result showing the asset_path in the detail message.
	srv := newTestServer(t)
	entry := &releaseIndexEntry{
		Name: "echo", Kind: "SERVICE", Publisher: "core@globular.io",
		Version: "1.0.84", BuildNumber: 24, BuildID: "24",
		Platform:      "linux_amd64",
		PackageDigest: "sha256:0000000000000000000000000000000000000000000000000000000000000000",
		AssetPath:     "packages/echo_1.0.84_linux_amd64.tgz",
		Filename:      "echo_1.0.84_linux_amd64.tgz",
		ReleaseTag:    "v1.0.84",
		// AssetURL intentionally empty — LOCAL_DIR/GIT_INDEX mode
	}
	src := &repopb.UpstreamSource{Name: "test-source"}
	prov, pOpts := testProvider()
	result := srv.processSyncEntry(context.Background(), entry, src, prov, pOpts, "v1.0.84", true)
	if result.Status != repopb.UpstreamSyncStatus_SYNC_WOULD_IMPORT {
		t.Fatalf("expected WOULD_IMPORT, got %s: %s", result.Status, result.Detail)
	}
	// The detail should show the asset_path since there's no asset_url.
	if !strings.Contains(result.Detail, "packages/echo") {
		t.Fatalf("expected asset_path in detail, got: %s", result.Detail)
	}
}

func TestProcessSyncEntry_NoAssetURLNoAssetPath_DryRun(t *testing.T) {
	// Entry with neither asset_url nor asset_path should still produce a
	// dry-run result (it will fail on actual import but dry-run is safe).
	srv := newTestServer(t)
	entry := &releaseIndexEntry{
		Name: "echo", Kind: "SERVICE", Publisher: "core@globular.io",
		Version: "1.0.84", BuildNumber: 24, BuildID: "24",
		Platform:      "linux_amd64",
		PackageDigest: "sha256:0000000000000000000000000000000000000000000000000000000000000000",
		Filename:      "echo_1.0.84_linux_amd64.tgz",
		ReleaseTag:    "v1.0.84",
		// Both AssetURL and AssetPath empty
	}
	src := &repopb.UpstreamSource{Name: "test-source"}
	prov, pOpts := testProvider()
	result := srv.processSyncEntry(context.Background(), entry, src, prov, pOpts, "v1.0.84", true)
	if result.Status != repopb.UpstreamSyncStatus_SYNC_WOULD_IMPORT {
		t.Fatalf("expected WOULD_IMPORT for dry-run, got %s", result.Status)
	}
}

func TestNormalizedEntry_PopulatesAssetPathAndFilename(t *testing.T) {
	entry := &releaseIndexEntry{
		Name: "echo", Version: "1.0.84", Platform: "linux_amd64",
		AssetPath: "packages/echo.tgz",
		Filename:  "echo_1.0.84.tgz",
		PackageDigest: "sha256:0000000000000000000000000000000000000000000000000000000000000000",
		AssetURL:  "", // intentionally empty
	}
	n := normalizeReleaseEntry(entry, &repopb.UpstreamSource{})
	if n.AssetPath != "packages/echo.tgz" {
		t.Fatalf("expected asset_path propagated, got %q", n.AssetPath)
	}
	if n.Filename != "echo_1.0.84.tgz" {
		t.Fatalf("expected filename propagated, got %q", n.Filename)
	}
}

func TestResolveProvenanceAssetURL_Variants(t *testing.T) {
	tests := []struct {
		name string
		n    *normalizedEntry
		want string
	}{
		{"asset_url wins", &normalizedEntry{AssetURL: "https://example.com/echo.tgz"}, "https://example.com/echo.tgz"},
		{"falls back to path:", &normalizedEntry{AssetPath: "packages/echo.tgz"}, "path:packages/echo.tgz"},
		{"falls back to file:", &normalizedEntry{Filename: "echo.tgz"}, "file:echo.tgz"},
		{"empty", &normalizedEntry{}, ""},
	}
	for _, tt := range tests {
		got := resolveProvenanceAssetURL(tt.n)
		if got != tt.want {
			t.Errorf("%s: got %q, want %q", tt.name, got, tt.want)
		}
	}
}
