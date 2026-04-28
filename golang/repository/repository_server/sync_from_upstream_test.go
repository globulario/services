package main

import (
	"context"
	"strings"
	"testing"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

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
		"v1.0.53",
		false,
		"",
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
