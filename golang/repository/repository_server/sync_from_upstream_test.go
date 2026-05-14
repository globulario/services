package main

import (
	"context"
	"encoding/json"
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

// TestProcessSyncEntrySkipsExistingDigestSameBuildNumber verifies that the
// true idempotent skip works when both digest AND build_number match.
func TestProcessSyncEntrySkipsExistingDigestSameBuildNumber(t *testing.T) {
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
		BuildNumber: 67,
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
		"",
	)

	if result.GetStatus() != repopb.UpstreamSyncStatus_SYNC_SKIPPED {
		t.Fatalf("expected SYNC_SKIPPED (same build_number), got %s: %s", result.GetStatus().String(), result.GetDetail())
	}
	if result.GetDetail() == "" {
		t.Fatal("expected detail explaining the existing artifact")
	}
}

// TestProcessSyncEntryDedupesDifferentBuildNumber verifies that when a local
// artifact exists with the same digest but a different build_number, sync
// dedupes to the canonical local artifact and skips duplicate import.
func TestProcessSyncEntryDedupesDifferentBuildNumber(t *testing.T) {
	srv := newTestServer(t)
	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io",
		Name:        "workflow",
		Version:     "1.0.53",
		Platform:    "linux_amd64",
		Kind:        repopb.ArtifactKind_SERVICE,
	}
	// Simulate local bootstrap publish: build_number=1, same bytes as upstream.
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
			BuildNumber:   67, // upstream has build_number=67, local has build_number=1
			Platform:      "linux_amd64",
			PackageDigest: "sha256:same-content",
			AssetURL:      "https://example.invalid/workflow.tgz",
		},
		&repopb.UpstreamSource{Name: "test-source"},
		prov, pOpts,
		"v1.0.53",
		false,
		"",
	)

	if result.GetStatus() != repopb.UpstreamSyncStatus_SYNC_SKIPPED {
		t.Fatalf("expected SYNC_SKIPPED for dedupe case, got %s: %s", result.GetStatus().String(), result.GetDetail())
	}
	if !strings.Contains(result.GetDetail(), "deduped") {
		t.Fatalf("expected dedupe detail, got: %s", result.GetDetail())
	}

	aliasKey := aliasStorageKey(ref, "v1.0.53", 67)
	raw, err := srv.Storage().ReadFile(context.Background(), aliasKey)
	if err != nil {
		t.Fatalf("expected alias file %q, got err: %v", aliasKey, err)
	}
	var alias releaseBuildAliasRecord
	if err := json.Unmarshal(raw, &alias); err != nil {
		t.Fatalf("unmarshal alias: %v", err)
	}
	if alias.CanonicalBuildID != "019d0001-0000-7000-8000-000000000001" {
		t.Fatalf("canonical_build_id=%q, want existing build id", alias.CanonicalBuildID)
	}
	if alias.BuildNumber != 67 || alias.ReleaseTag != "v1.0.53" {
		t.Fatalf("unexpected alias locator: release=%q build_number=%d", alias.ReleaseTag, alias.BuildNumber)
	}
}

func TestProcessSyncEntryRejectsOnAliasConflict(t *testing.T) {
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
		BuildId:     "canonical-a",
		Checksum:    "sha256:same-content",
		SizeBytes:   100,
	})
	// Pre-existing conflicting alias for the same release/build locator.
	if err := srv.ensureReleaseBuildAlias(context.Background(), ref, "v1.0.53", 67, "upstream-67", "canonical-b", "sha256:same-content", "v1.0.53", "test-source"); err != nil {
		t.Fatalf("seed alias: %v", err)
	}

	prov, pOpts := testProvider()
	result := srv.processSyncEntry(
		context.Background(),
		&releaseIndexEntry{
			Name:          "workflow",
			Publisher:     "core@globular.io",
			Version:       "1.0.53",
			BuildID:       "upstream-67",
			BuildNumber:   67,
			Platform:      "linux_amd64",
			PackageDigest: "sha256:same-content",
			AssetURL:      "https://example.invalid/workflow.tgz",
		},
		&repopb.UpstreamSource{Name: "test-source"},
		prov, pOpts,
		"v1.0.53",
		false,
		"",
	)
	if result.GetStatus() != repopb.UpstreamSyncStatus_SYNC_REJECTED {
		t.Fatalf("expected SYNC_REJECTED on alias conflict, got %s: %s", result.GetStatus().String(), result.GetDetail())
	}
}

func TestImportUpstreamArtifact_IdempotentSkipPersistsAlias(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()
	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io",
		Name:        "workflow",
		Version:     "1.0.53",
		Platform:    "linux_amd64",
		Kind:        repopb.ArtifactKind_SERVICE,
	}
	seedPublishedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref:         ref,
		BuildNumber: 67,
		BuildId:     "canonical-67",
		Checksum:    "sha256:same-content",
		SizeBytes:   100,
	})

	n := &normalizedEntry{
		Publisher: "core@globular.io",
		Name:      "workflow",
		Version:   "1.0.53",
		Platform:  "linux_amd64",
		BuildID:   "upstream-67",
		BuildNumber: 67,
		Digest:    "sha256:same-content",
		OriginRelease: "v1.0.53",
	}
	err := srv.importUpstreamArtifact(
		ctx,
		n,
		[]byte("unused-for-idempotent-path"),
		"sha256:same-content",
		&repopb.UpstreamSource{Name: "test-source"},
		"v1.0.53",
		ArtifactStateFields{},
		"",
	)
	if err != nil {
		t.Fatalf("importUpstreamArtifact: %v", err)
	}
	alias, err := srv.loadReleaseBuildAlias(ctx, ref, "v1.0.53", 67)
	if err != nil {
		t.Fatalf("load alias: %v", err)
	}
	if alias == nil {
		t.Fatal("expected alias record")
	}
	if alias.CanonicalBuildID != "canonical-67" {
		t.Fatalf("canonical_build_id=%q, want canonical-67", alias.CanonicalBuildID)
	}
}

func TestProcessSyncEntrySkipsWhenSameBuildIDAlreadyAtHigherBuildNumber(t *testing.T) {
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
		BuildNumber: 204,
		BuildId:     "upstream:same-id",
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
			BuildID:       "upstream:same-id",
			BuildNumber:   1,
			Platform:      "linux_amd64",
			PackageDigest: "sha256:same-content",
			AssetURL:      "https://example.invalid/workflow.tgz",
		},
		&repopb.UpstreamSource{Name: "test-source"},
		prov, pOpts,
		"v1.0.53",
		false,
		"",
	)
	if result.GetStatus() != repopb.UpstreamSyncStatus_SYNC_SKIPPED {
		t.Fatalf("expected SYNC_SKIPPED, got %s (%s)", result.GetStatus().String(), result.GetDetail())
	}
}

func TestProcessSyncEntryRejectsSameBuildIDDifferentDigest(t *testing.T) {
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
		BuildNumber: 50,
		BuildId:     "upstream:same-id",
		Checksum:    "sha256:local",
		SizeBytes:   100,
	})

	prov, pOpts := testProvider()
	result := srv.processSyncEntry(
		context.Background(),
		&releaseIndexEntry{
			Name:          "workflow",
			Publisher:     "core@globular.io",
			Version:       "1.0.53",
			BuildID:       "upstream:same-id",
			BuildNumber:   204,
			Platform:      "linux_amd64",
			PackageDigest: "sha256:upstream",
			AssetURL:      "https://example.invalid/workflow.tgz",
		},
		&repopb.UpstreamSource{Name: "test-source"},
		prov, pOpts,
		"v1.0.53",
		false,
		"",
	)
	if result.GetStatus() != repopb.UpstreamSyncStatus_SYNC_REJECTED {
		t.Fatalf("expected SYNC_REJECTED, got %s (%s)", result.GetStatus().String(), result.GetDetail())
	}
	if result.GetAction() != "conflict" {
		t.Fatalf("expected conflict action, got %q", result.GetAction())
	}
}

// TestProcessSyncEntryDedupesWhenSameBuildIDHasLowerBuildNumber: when the
// local repo holds an artifact at build_number=1 with build_id=X checksum=Y,
// and upstream advertises the same build_id+checksum at build_number=204, the
// correct response is dedupe + alias — NOT a second import row. The doc's
// conflict matrix (same identity + same checksum -> dedupe + alias) overrides
// the older "import at higher build_number" intuition because build_number is
// a locator, not canonical identity. The alias preserves the upstream
// build_number reference so callers using (release_tag, build_number) can
// still resolve to the canonical local build.
func TestProcessSyncEntryDedupesWhenSameBuildIDHasLowerBuildNumber(t *testing.T) {
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
		BuildId:     "upstream:same-id",
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
			BuildID:       "upstream:same-id",
			BuildNumber:   204,
			Platform:      "linux_amd64",
			PackageDigest: "sha256:same-content",
			AssetURL:      "https://example.invalid/workflow.tgz",
		},
		&repopb.UpstreamSource{Name: "test-source"},
		prov, pOpts,
		"v1.0.53",
		false,
		"",
	)
	if result.GetStatus() != repopb.UpstreamSyncStatus_SYNC_SKIPPED {
		t.Fatalf("expected SYNC_SKIPPED (dedupe), got %s: %s",
			result.GetStatus(), result.GetDetail())
	}
	if !strings.Contains(result.GetDetail(), "deduped") {
		t.Errorf("expected dedupe detail, got: %s", result.GetDetail())
	}
	// Alias for upstream build_number=204 must resolve back to the existing
	// canonical local build_id.
	alias, err := srv.loadReleaseBuildAlias(context.Background(), ref, "v1.0.53", 204)
	if err != nil {
		t.Fatalf("loadReleaseBuildAlias: %v", err)
	}
	if alias == nil {
		t.Fatal("expected alias for (v1.0.53, 204) pointing to canonical local build")
	}
	if alias.CanonicalBuildID != "upstream:same-id" {
		t.Errorf("alias.CanonicalBuildID = %q, want %q", alias.CanonicalBuildID, "upstream:same-id")
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

func TestNormalizeReleaseEntry_NumericBuildIDReplaced(t *testing.T) {
	entry := &releaseIndexEntry{
		Name: "sidekick", Version: "1.1.0", Platform: "linux_amd64",
		BuildID: "105",
		PackageDigest: "sha256:1111111111111111111111111111111111111111111111111111111111111111",
		AssetURL: "https://example.com/sidekick.tgz",
	}
	n := normalizeReleaseEntry(entry, &repopb.UpstreamSource{})
	if n.BuildID == "105" {
		t.Fatal("numeric build_id must be replaced to avoid cross-package collisions")
	}
	if !strings.HasPrefix(n.BuildID, "upstream:") {
		t.Fatalf("expected derived upstream build_id, got %q", n.BuildID)
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
	result := srv.processSyncEntry(context.Background(), entry, src, prov, pOpts, "v1.0.0", true, "")

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
	result := srv.processSyncEntry(context.Background(), entry, src, prov, pOpts, "v1.0.0", true, "")
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
	result := srv.processSyncEntry(context.Background(), entry, src, prov, pOpts, "v1.0.0", true, "")
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
		Version: "1.0.82", BuildNumber: 9, BuildID: "bid-9",
		Platform:         "linux_amd64",
		PackageDigest:    "sha256:0000000000000000000000000000000000000000000000000000000000000000",
		AssetURL:         "https://example.com/v1.0.82/gateway.tgz",
		ReleaseTag:       "v1.0.84",
		OriginRelease:    "v1.0.82",
		ChangedInRelease: &unchanged,
	}
	src := &repopb.UpstreamSource{Name: "test-source"}

	prov, pOpts := testProvider()
		result := srv.processSyncEntry(context.Background(), entry, src, prov, pOpts, "v1.0.84", true, "")
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
		result := srv.processSyncEntry(context.Background(), entry, src, prov, pOpts, "v1.0.84", true, "")
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
	result := srv.processSyncEntry(context.Background(), entry, src, prov, pOpts, "v1.0.85", false, "")
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
		Ref: ref, BuildNumber: 9, BuildId: "bid-9",
		Checksum: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		SizeBytes: 100,
	})

	entry := &releaseIndexEntry{
		Name: "gateway", Kind: "SERVICE", Publisher: "core@globular.io",
		Version: "1.0.82", BuildNumber: 9, BuildID: "bid-9",
		Platform:      "linux_amd64",
		PackageDigest: "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
		AssetURL:      "https://example.com/v1.0.82/gateway.tgz",
		ReleaseTag:    "v1.0.85",
	}
	src := &repopb.UpstreamSource{Name: "test-source"}
	prov, pOpts := testProvider()
	result := srv.processSyncEntry(context.Background(), entry, src, prov, pOpts, "v1.0.85", false, "")
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
	result := srv.processSyncEntry(context.Background(), entry, src, prov, pOpts, "v1.0.84", true, "")
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
	result := srv.processSyncEntry(context.Background(), entry, src, prov, pOpts, "v1.0.84", true, "")
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

// ── Missing-blob repair regression tests ────────────────────────────────────
//
// The bug these tests exist to prevent: ScyllaDB / release-ledger metadata
// said an artifact was PUBLISHED, but MinIO had been recreated empty after a
// migration to distributed mode. Repo sync skipped the artifact because its
// digest matched, then DownloadArtifact failed with "specified key does not
// exist". After the fix, every skip path requires the exact binary blob to
// exist in object storage; missing or wrong-size blobs trigger a re-import.

// TestSyncFromUpstream_LedgerMatchButBlobMissing_Reimports is the
// release-blocking regression test for the missing-blob bug. Metadata,
// ledger row, and digest all match — but the binary blob has been deleted
// from object storage. Sync MUST re-import, not skip.
func TestSyncFromUpstream_LedgerMatchButBlobMissing_Reimports(t *testing.T) {
	root, expectedDigest := createLocalDirSource(t, "v1.0.84", "echo", "1.0.84")
	pkgContent := []byte("fake-package-binary-content-for-echo")

	srv := newTestServer(t)
	ctx := context.Background()

	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io",
		Name:        "echo",
		Version:     "1.0.84",
		Platform:    "linux_amd64",
		Kind:        repopb.ArtifactKind_SERVICE,
	}
	// Seed the repository as if the artifact had previously been published:
	// manifest exists, ledger exists, blob exists. Same publisher/name/
	// version/build_id/digest as the upstream entry that is about to be
	// synced.
	seedPublishedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref:         ref,
		BuildNumber: 1,
		BuildId:     "e2e-1",
		Checksum:    expectedDigest,
		SizeBytes:   int64(len(pkgContent)),
	})

	// Simulate the production incident: MinIO was recreated empty after the
	// move to distributed mode, so the metadata lingers but the .bin object
	// is gone. We delete the seeded blob to reproduce that exact state.
	key := artifactKeyWithBuild(ref, 1)
	binKey := binaryStorageKey(key)
	if err := srv.Storage().Remove(ctx, binKey); err != nil {
		t.Fatalf("simulate missing blob: %v", err)
	}
	if _, err := srv.Storage().Stat(ctx, binKey); err == nil {
		t.Fatal("setup invariant: blob should be absent after Remove")
	}

	provider, err := upstream.NewSource(upstream.TypeLocalDir)
	if err != nil {
		t.Fatalf("create provider: %v", err)
	}
	opts := upstream.SourceOpts{
		LocalRoot:         root,
		IndexPathTemplate: "releases/{tag}/release-index.json",
	}
	indexData, err := provider.GetReleaseIndex(ctx, opts, "v1.0.84")
	if err != nil {
		t.Fatalf("get release index: %v", err)
	}
	idx, err := parseReleaseIndex(indexData)
	if err != nil {
		t.Fatalf("parse release index: %v", err)
	}
	if len(idx.Packages) != 1 {
		t.Fatalf("expected 1 package in index, got %d", len(idx.Packages))
	}

	src := &repopb.UpstreamSource{Name: "test-source", Enabled: true}
	result := srv.processSyncEntry(ctx, idx.Packages[0], src, provider, opts, "v1.0.84", false, "")

	// Must NOT be reported as skipped — that was the original bug.
	if result.Status == repopb.UpstreamSyncStatus_SYNC_SKIPPED ||
		result.Status == repopb.UpstreamSyncStatus_SYNC_WOULD_SKIP {
		t.Fatalf("missing blob must not be reported as skipped; got %s: %s",
			result.Status, result.Detail)
	}
	if result.Status != repopb.UpstreamSyncStatus_SYNC_IMPORTED {
		t.Fatalf("expected SYNC_IMPORTED, got %s: %s", result.Status, result.Detail)
	}
	if result.Action != "repair_blob" {
		t.Fatalf("expected action=repair_blob, got %q (detail=%q)", result.Action, result.Detail)
	}
	if !strings.Contains(result.Detail, "blob") {
		t.Fatalf("repair detail should mention the blob, got %q", result.Detail)
	}

	// Storage now contains the exact binary blob the import was supposed to
	// create — and DownloadArtifact (which Stats this same key) would
	// succeed against it.
	fi, statErr := srv.Storage().Stat(ctx, binKey)
	if statErr != nil {
		t.Fatalf("blob should be present after repair: %v", statErr)
	}
	if fi.Size() != int64(len(pkgContent)) {
		t.Fatalf("blob size mismatch after repair: got %d, want %d", fi.Size(), len(pkgContent))
	}
	got, err := srv.Storage().ReadFile(ctx, binKey)
	if err != nil {
		t.Fatalf("read repaired blob: %v", err)
	}
	if string(got) != string(pkgContent) {
		t.Fatal("repaired blob content does not match upstream package content")
	}
}

// TestSyncFromUpstream_LedgerMatchAndBlobPresent_Skips proves the inverse
// of the golden test: when metadata, ledger, digest, AND the blob are all
// present and consistent, sync correctly skips with up_to_date.
func TestSyncFromUpstream_LedgerMatchAndBlobPresent_Skips(t *testing.T) {
	root, expectedDigest := createLocalDirSource(t, "v1.0.84", "echo", "1.0.84")
	pkgContent := []byte("fake-package-binary-content-for-echo")

	srv := newTestServer(t)
	ctx := context.Background()

	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io",
		Name:        "echo",
		Version:     "1.0.84",
		Platform:    "linux_amd64",
		Kind:        repopb.ArtifactKind_SERVICE,
	}
	seedPublishedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref:         ref,
		BuildNumber: 1,
		BuildId:     "e2e-1",
		Checksum:    expectedDigest,
		SizeBytes:   int64(len(pkgContent)),
	})

	// Ensure the seeded blob's content actually matches the digest the
	// release index claims, so the ledger/blob state is fully consistent.
	key := artifactKeyWithBuild(ref, 1)
	binKey := binaryStorageKey(key)
	if err := srv.Storage().WriteFile(ctx, binKey, pkgContent, 0o644); err != nil {
		t.Fatalf("rewrite blob with matching content: %v", err)
	}

	provider, _ := upstream.NewSource(upstream.TypeLocalDir)
	opts := upstream.SourceOpts{
		LocalRoot:         root,
		IndexPathTemplate: "releases/{tag}/release-index.json",
	}
	indexData, err := provider.GetReleaseIndex(ctx, opts, "v1.0.84")
	if err != nil {
		t.Fatalf("get release index: %v", err)
	}
	idx, err := parseReleaseIndex(indexData)
	if err != nil {
		t.Fatalf("parse release index: %v", err)
	}

	src := &repopb.UpstreamSource{Name: "test-source", Enabled: true}
	result := srv.processSyncEntry(ctx, idx.Packages[0], src, provider, opts, "v1.0.84", false, "")

	if result.Status != repopb.UpstreamSyncStatus_SYNC_SKIPPED {
		t.Fatalf("expected SYNC_SKIPPED, got %s: %s", result.Status, result.Detail)
	}
	if result.Action != "up_to_date" {
		t.Fatalf("expected action=up_to_date, got %q", result.Action)
	}
	if !strings.Contains(result.Detail, "blob verified") {
		t.Fatalf("skip detail should include 'blob verified', got %q", result.Detail)
	}
}

// TestSyncFromUpstream_MetadataExistsButBlobSizeMismatch_Reimports covers
// the corruption case: metadata, ledger, and the binary all exist, but the
// binary has the wrong size (truncated upload, partial recovery, etc.).
// The size_mismatch must be treated as a damaged blob and re-imported.
func TestSyncFromUpstream_MetadataExistsButBlobSizeMismatch_Reimports(t *testing.T) {
	root, expectedDigest := createLocalDirSource(t, "v1.0.84", "echo", "1.0.84")
	pkgContent := []byte("fake-package-binary-content-for-echo")

	srv := newTestServer(t)
	ctx := context.Background()

	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io",
		Name:        "echo",
		Version:     "1.0.84",
		Platform:    "linux_amd64",
		Kind:        repopb.ArtifactKind_SERVICE,
	}
	seedPublishedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref:         ref,
		BuildNumber: 1,
		BuildId:     "e2e-1",
		Checksum:    expectedDigest,
		SizeBytes:   int64(len(pkgContent)),
	})

	// Replace the blob with content of a different size — the ledger says N
	// bytes, but the blob is now M != N bytes. Mimics a partial restore.
	key := artifactKeyWithBuild(ref, 1)
	binKey := binaryStorageKey(key)
	corrupt := []byte("short")
	if int64(len(corrupt)) == int64(len(pkgContent)) {
		t.Fatal("test setup: corrupt content must differ in size from real content")
	}
	if err := srv.Storage().WriteFile(ctx, binKey, corrupt, 0o644); err != nil {
		t.Fatalf("write corrupt blob: %v", err)
	}

	provider, _ := upstream.NewSource(upstream.TypeLocalDir)
	opts := upstream.SourceOpts{
		LocalRoot:         root,
		IndexPathTemplate: "releases/{tag}/release-index.json",
	}
	indexData, err := provider.GetReleaseIndex(ctx, opts, "v1.0.84")
	if err != nil {
		t.Fatalf("get release index: %v", err)
	}
	idx, err := parseReleaseIndex(indexData)
	if err != nil {
		t.Fatalf("parse release index: %v", err)
	}

	src := &repopb.UpstreamSource{Name: "test-source", Enabled: true}
	result := srv.processSyncEntry(ctx, idx.Packages[0], src, provider, opts, "v1.0.84", false, "")

	if result.Status == repopb.UpstreamSyncStatus_SYNC_SKIPPED {
		t.Fatalf("size-mismatched blob must not be skipped; detail=%q", result.Detail)
	}
	if result.Status != repopb.UpstreamSyncStatus_SYNC_IMPORTED {
		t.Fatalf("expected SYNC_IMPORTED, got %s: %s", result.Status, result.Detail)
	}
	if result.Action != "repair_blob" {
		t.Fatalf("expected action=repair_blob, got %q", result.Action)
	}

	// After repair the blob's size matches the upstream content again.
	fi, statErr := srv.Storage().Stat(ctx, binKey)
	if statErr != nil {
		t.Fatalf("blob should be present after repair: %v", statErr)
	}
	if fi.Size() != int64(len(pkgContent)) {
		t.Fatalf("blob size after repair: got %d, want %d", fi.Size(), len(pkgContent))
	}
}

// TestDigestEqual_NormalizesSha256Prefix verifies the canonical digest
// comparison helper across mixed-case, whitespace, and prefix variations.
func TestDigestEqual_NormalizesSha256Prefix(t *testing.T) {
	cases := []struct {
		name string
		a, b string
		want bool
	}{
		{"upper-prefixed equals lower-bare", "sha256:ABC", "abc", true},
		{"whitespace and prefix", " abc ", "sha256:abc", true},
		{"prefix on both sides", "sha256:abc", "SHA256:ABC", true},
		{"empty a", "", "abc", false},
		{"empty b", "abc", "", false},
		{"both empty", "", "", false},
		{"prefix-only is empty", "sha256:", "abc", false},
		{"different hex", "sha256:abc", "sha256:def", false},
		{"raw vs prefixed different", "abc", "sha256:def", false},
	}
	for _, tc := range cases {
		got := digestEqual(tc.a, tc.b)
		if got != tc.want {
			t.Errorf("%s: digestEqual(%q, %q) = %v, want %v",
				tc.name, tc.a, tc.b, got, tc.want)
		}
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

// ── Integration scenarios from globular_repository_package_identity_fix ────
//
// These mirror the named scenarios in the design doc so the conflict-matrix
// expectations are locked at the sync entry point. They use processSyncEntry
// directly because the conflict gates fire before any network/download path,
// so a no-op provider is sufficient.

// Scenario A: duplicate upstream package.
//
//   local:    repository@0.3.4 linux_amd64 build_id=A build_number=100 checksum=X
//   upstream: repository@0.3.4 linux_amd64 build_id=B build_number=101 checksum=X
//
// Expected: one canonical artifact, alias for build_number=101 -> canonical
// build_id A, no second artifact row, sync reports SYNC_SKIPPED action=up_to_date.
func TestScenarioA_DuplicateUpstreamPackage_DedupesWithAlias(t *testing.T) {
	srv := newTestServer(t)

	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io", Name: "repository",
		Version: "0.3.4", Platform: "linux_amd64",
		Kind: repopb.ArtifactKind_SERVICE,
	}
	const sameChecksum = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	const localBuildID = "01JLOCALBUILDIDA"
	seedPublishedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref: ref, BuildNumber: 100, BuildId: localBuildID,
		Checksum: sameChecksum, SizeBytes: 11,
	})

	unchanged := false
	entry := &releaseIndexEntry{
		Name: "repository", Kind: "SERVICE", Publisher: "core@globular.io",
		Version: "0.3.4", BuildNumber: 101, BuildID: "01JUPSTREAMBUILDIDB",
		Platform:         "linux_amd64",
		PackageDigest:    sameChecksum,
		AssetURL:         "https://example.invalid/repository.tgz",
		ReleaseTag:       "v1.2.32",
		OriginRelease:    "v1.2.32",
		ChangedInRelease: &unchanged,
	}
	src := &repopb.UpstreamSource{Name: "test-source"}
	prov, pOpts := testProvider()
	result := srv.processSyncEntry(context.Background(), entry, src, prov, pOpts, "v1.2.32", false, "")

	if result.Status != repopb.UpstreamSyncStatus_SYNC_SKIPPED {
		t.Fatalf("expected SYNC_SKIPPED, got %s: %s", result.Status, result.Detail)
	}
	if result.Action != "up_to_date" {
		t.Errorf("expected action=up_to_date, got %q", result.Action)
	}
	// Alias for the upstream locator must resolve to the canonical local build_id.
	alias, err := srv.loadReleaseBuildAlias(context.Background(), ref, "v1.2.32", 101)
	if err != nil {
		t.Fatalf("loadReleaseBuildAlias: %v", err)
	}
	if alias == nil {
		t.Fatal("expected alias for (v1.2.32, 101) -> canonical local build")
	}
	if alias.CanonicalBuildID != localBuildID {
		t.Errorf("alias.CanonicalBuildID = %q, want %q", alias.CanonicalBuildID, localBuildID)
	}
}

// Scenario B: malicious/confused upstream build_id reuse.
//
//   local:    build_id=A checksum=X
//   upstream: build_id=A checksum=Y
//
// Expected: reject, quarantine the incoming metadata, local artifact unchanged.
func TestScenarioB_UpstreamBuildIDReuseRejected(t *testing.T) {
	srv := newTestServer(t)

	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io", Name: "repository",
		Version: "0.3.4", Platform: "linux_amd64",
		Kind: repopb.ArtifactKind_SERVICE,
	}
	const localBuildID = "01JBUILDIDA"
	const localChecksum = "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	const conflictChecksum = "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	seedPublishedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref: ref, BuildNumber: 100, BuildId: localBuildID,
		Checksum: localChecksum, SizeBytes: 17,
	})

	entry := &releaseIndexEntry{
		Name: "repository", Kind: "SERVICE", Publisher: "core@globular.io",
		Version: "0.3.4", BuildNumber: 101, BuildID: localBuildID, // same build_id
		Platform:      "linux_amd64",
		PackageDigest: conflictChecksum,
		AssetURL:      "https://example.invalid/repository-evil.tgz",
		ReleaseTag:    "v1.2.32",
	}
	src := &repopb.UpstreamSource{Name: "test-source"}
	prov, pOpts := testProvider()
	result := srv.processSyncEntry(context.Background(), entry, src, prov, pOpts, "v1.2.32", false, "")

	if result.Status != repopb.UpstreamSyncStatus_SYNC_REJECTED {
		t.Fatalf("expected SYNC_REJECTED, got %s: %s", result.Status, result.Detail)
	}
	if result.Action != "conflict" {
		t.Errorf("expected action=conflict, got %q", result.Action)
	}
	if !strings.Contains(result.Detail, "build_id conflict") {
		t.Errorf("expected build_id conflict detail, got: %s", result.Detail)
	}
	// Local artifact must be unchanged -- checksum stays at localChecksum.
	manifestResp, err := srv.GetArtifactManifest(context.Background(),
		&repopb.GetArtifactManifestRequest{Ref: ref, BuildNumber: 100})
	if err != nil {
		t.Fatalf("get local manifest: %v", err)
	}
	if manifestResp.GetManifest().GetChecksum() != localChecksum {
		t.Errorf("local checksum mutated: got %q, want %q",
			manifestResp.GetManifest().GetChecksum(), localChecksum)
	}
	// No alias was created for the rejected upstream locator.
	alias, err := srv.loadReleaseBuildAlias(context.Background(), ref, "v1.2.32", 101)
	if err != nil {
		t.Fatalf("loadReleaseBuildAlias: %v", err)
	}
	if alias != nil {
		t.Errorf("unexpected alias for rejected upstream entry: %+v", alias)
	}
}

// Scenario C: same version, real new build.
//
//   local:    repository@0.3.4 build_id=A checksum=X build_number=100
//   upstream: repository@0.3.4 build_id=B checksum=Y build_number=101
//
// Expected: both builds allowed (different build_id AND different checksum =
// distinct artifact identities); a version-only resolution then fails as
// ambiguous because the resolver requires a build_id pin.
func TestScenarioC_SameVersionRealNewBuild_AmbiguousResolution(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io", Name: "repository",
		Version: "0.3.4", Platform: "linux_amd64",
		Kind: repopb.ArtifactKind_SERVICE,
	}
	seedPublishedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref: ref, BuildNumber: 100, BuildId: "01JBUILDIDA",
		Checksum:  "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		SizeBytes: 13,
		Channel:   repopb.ArtifactChannel_STABLE,
	})
	seedPublishedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref: ref, BuildNumber: 101, BuildId: "01JBUILDIDB",
		Checksum:  "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		SizeBytes: 13,
		Channel:   repopb.ArtifactChannel_STABLE,
	})

	// Sanity: both manifests retrievable at their own build_id.
	for _, want := range []struct {
		buildID  string
		expectCS string
	}{
		{"01JBUILDIDA", "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
		{"01JBUILDIDB", "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
	} {
		resp, err := srv.ResolveArtifact(ctx, &repopb.ResolveArtifactRequest{
			Name: "repository", Platform: "linux_amd64",
			BuildId: want.buildID, PublisherId: "core@globular.io",
		})
		if err != nil {
			t.Fatalf("resolve build_id=%s: %v", want.buildID, err)
		}
		if got := resp.GetManifest().GetChecksum(); got != want.expectCS {
			t.Errorf("build_id=%s checksum=%q want %q", want.buildID, got, want.expectCS)
		}
	}

	// Version-only resolution must refuse to pick a winner.
	_, err := srv.ResolveArtifact(ctx, &repopb.ResolveArtifactRequest{
		Name: "repository", Platform: "linux_amd64",
		Version: "0.3.4", PublisherId: "core@globular.io",
	})
	if err == nil {
		t.Fatal("expected ambiguous resolution error, got nil")
	}
	if !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("expected error to mention 'ambiguous', got: %v", err)
	}
}

// Scenario D: release-index platform pin (resolver-side invariants only).
//
//   A previously-imported artifact at build_id=A checksum=X must always be
//   reachable by build_id-pinned resolve, regardless of which release tag
//   later referenced it. The sync-side half of Scenario D (sync re-sees the
//   same pin and produces SYNC_SKIPPED + persists the alias) is blocked on
//   the same platform-normalization fix described above and is intentionally
//   not covered here — see also TestProcessSyncEntryDedupesDifferentBuildNumber.
func TestScenarioD_ReleaseIndexPin_ResolveByBuildIDIsDeterministic(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io", Name: "repository",
		Version: "0.3.4", Platform: "linux_amd64",
		Kind: repopb.ArtifactKind_SERVICE,
	}
	const pinnedBuildID = "01JPINNEDBUILDID"
	const pinnedChecksum = "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc"

	seedPublishedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref: ref, BuildNumber: 171, BuildId: pinnedBuildID,
		Checksum:  pinnedChecksum,
		SizeBytes: 19,
		Channel:   repopb.ArtifactChannel_STABLE,
	})

	resp, err := srv.ResolveArtifact(ctx, &repopb.ResolveArtifactRequest{
		Name: "repository", Platform: "linux_amd64",
		BuildId:     pinnedBuildID,
		PublisherId: "core@globular.io",
	})
	if err != nil {
		t.Fatalf("ResolveArtifact build_id=%s: %v", pinnedBuildID, err)
	}
	if resp.GetManifest().GetBuildId() != pinnedBuildID {
		t.Errorf("resolved build_id = %q, want %q", resp.GetManifest().GetBuildId(), pinnedBuildID)
	}
	if got := resp.GetManifest().GetChecksum(); got != pinnedChecksum {
		t.Errorf("resolved checksum = %q, want %q", got, pinnedChecksum)
	}
	if resp.GetResolutionSource() != "exact-build_id" {
		t.Errorf("resolution_source = %q, want exact-build_id", resp.GetResolutionSource())
	}
}
