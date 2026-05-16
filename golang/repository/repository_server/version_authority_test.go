package main

// version_authority_test.go — Regression tests for the platform-release ≠
// package-version invariant from the version-authority cleanup spec.
//
// Core invariant: platform_release (e.g. 1.2.52) is BOM composition metadata.
// A package's own version (e.g. storage=1.2.43) is its artifact identity and
// must never be overridden by, or conflated with, the platform release.
//
// Additional invariants tested:
//   VA-1: Platform release does not stamp package versions in a V2 BOM.
//   VA-2: normalizeReleaseEntry preserves the per-entry version field.
//   VA-3: Non-PUBLISHED artifacts are excluded from the install-resolution path.
//   VA-4: build_id is install identity; build_number is display-only metadata.
//   VA-5: Unchanged packages carry their origin release version, not platform_release.

import (
	"context"
	"strings"
	"testing"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

// ── VA-1: Platform release does not stamp package versions ─────────────────

// TestVA1_V2BOM_PlatformReleaseIsMetadataNotVersion pins the core invariant
// with the concrete fixture from the version-authority cleanup spec:
//   - platform_release = 1.2.52  (BOM composition identifier)
//   - storage.version  = 1.2.43  (artifact's own semantic version)
//
// Expected: idx.Packages[storage].Version == "1.2.43"
// Forbidden: idx.Packages[storage].Version == "1.2.52"
//
// Without this test a future change that "helpfully" stamps all entries with
// platform_release would silently violate the BOM invariant and cause the
// reconciler to request a non-existent version of unchanged packages.
func TestVA1_V2BOM_PlatformReleaseIsMetadataNotVersion(t *testing.T) {
	unchanged := false
	changed := true

	// Exact fixture from version-authority cleanup spec.
	idx := GenerateReleaseIndexV2("v1.2.52", "1.2.52", "core@globular.io",
		[]string{"v1.2.43"}, false, "", []*releaseIndexEntry{
			{
				Name:             "cluster-controller",
				Kind:             "SERVICE",
				Publisher:        "core@globular.io",
				Version:          "1.2.52", // changed in this release
				BuildNumber:      200,
				BuildID:          "ctrl-build-uuid-001",
				Platform:         "linux_amd64",
				PackageDigest:    "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				AssetURL:         "https://example.com/v1.2.52/cluster-controller.tgz",
				ReleaseTag:       "v1.2.52",
				OriginRelease:    "v1.2.52",
				ChangedInRelease: &changed,
			},
			{
				Name:             "storage",
				Kind:             "SERVICE",
				Publisher:        "core@globular.io",
				Version:          "1.2.43", // unchanged — carries its own version
				BuildNumber:      143,
				BuildID:          "stor-build-uuid-001",
				Platform:         "linux_amd64",
				PackageDigest:    "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				AssetURL:         "https://example.com/v1.2.43/storage.tgz",
				ReleaseTag:       "v1.2.52",
				OriginRelease:    "v1.2.43",
				ChangedInRelease: &unchanged,
			},
		})

	if err := ValidateReleaseIndex(idx); err != nil {
		t.Fatalf("V2 BOM with mixed versions must be valid, got: %v", err)
	}

	if idx.PlatformRelease != "1.2.52" {
		t.Fatalf("platform_release = %q, want 1.2.52", idx.PlatformRelease)
	}

	// Find the storage package.
	var storageEntry *releaseIndexEntry
	for _, p := range idx.Packages {
		if p.Name == "storage" {
			storageEntry = p
			break
		}
	}
	if storageEntry == nil {
		t.Fatal("storage package not found in BOM")
	}

	// The invariant: storage keeps its own version.
	if storageEntry.Version != "1.2.43" {
		t.Errorf("storage.version = %q, want 1.2.43 — platform_release must not stamp package versions",
			storageEntry.Version)
	}

	// The invariant negation: storage version must not equal platform_release.
	if storageEntry.Version == idx.PlatformRelease {
		t.Errorf("storage.version == platform_release (%s) — unchanged package was stamped with platform release",
			storageEntry.Version)
	}

	// The changed package gets platform_release as its version (correct).
	for _, p := range idx.Packages {
		if p.Name == "cluster-controller" {
			if p.Version != idx.PlatformRelease {
				t.Errorf("cluster-controller.version = %q, want %s (platform_release)",
					p.Version, idx.PlatformRelease)
			}
		}
	}
}

// TestVA1_V2BOM_PlatformReleaseFieldIsOnTopLevelOnly verifies that
// platform_release is a single top-level BOM field and is NOT copied into
// individual package entries during generation or validation.
func TestVA1_V2BOM_PlatformReleaseFieldIsOnTopLevelOnly(t *testing.T) {
	changed := true
	idx := GenerateReleaseIndexV2("v1.2.52", "1.2.52", "core@globular.io",
		nil, false, "", []*releaseIndexEntry{
			{
				Name:             "echo",
				Kind:             "SERVICE",
				Publisher:        "core@globular.io",
				Version:          "1.2.52",
				BuildNumber:      1,
				BuildID:          "echo-build-uuid-001",
				Platform:         "linux_amd64",
				PackageDigest:    "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
				AssetURL:         "https://example.com/v1.2.52/echo.tgz",
				ReleaseTag:       "v1.2.52",
				OriginRelease:    "v1.2.52",
				ChangedInRelease: &changed,
			},
		})

	if err := ValidateReleaseIndex(idx); err != nil {
		t.Fatalf("valid v2 BOM: %v", err)
	}

	// The top-level platform_release is set.
	if idx.PlatformRelease == "" {
		t.Error("PlatformRelease should be set on the index")
	}

	// Individual entries have no platform_release field in the struct
	// (they carry Version, OriginRelease — separate concepts).
	// Validate the entry's origin_release is used for provenance, not contamination.
	if idx.Packages[0].OriginRelease != "v1.2.52" {
		t.Errorf("origin_release = %q, want v1.2.52", idx.Packages[0].OriginRelease)
	}
}

// ── VA-2: normalizeReleaseEntry preserves per-entry version ────────────────

// TestVA2_NormalizeEntry_VersionFromEntry verifies that normalizeReleaseEntry
// uses the entry's own Version field, never the enclosing index's PlatformRelease.
// The sync_from_upstream pipeline sets n.PlatformRelease as post-normalization
// metadata (not a version override) — this test documents that contract.
func TestVA2_NormalizeEntry_VersionFromEntry(t *testing.T) {
	entry := &releaseIndexEntry{
		Name:          "storage",
		Kind:          "SERVICE",
		Publisher:     "core@globular.io",
		Version:       "1.2.43", // package's own version
		BuildNumber:   143,
		BuildID:       "upstream-abc123-stor",
		Platform:      "linux_amd64",
		PackageDigest: "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
		AssetURL:      "https://example.com/v1.2.43/storage.tgz",
		ReleaseTag:    "v1.2.52",
		OriginRelease: "v1.2.43",
	}

	src := &repopb.UpstreamSource{}
	n := normalizeReleaseEntry(entry, src)

	// Version must come from entry.Version, not any higher-level platform concept.
	if n.Version != "1.2.43" {
		t.Errorf("normalized version = %q, want 1.2.43 — normalizeReleaseEntry must not contaminate with platform_release",
			n.Version)
	}

	// OriginRelease must come from entry.OriginRelease (not ReleaseTag).
	if n.OriginRelease != "v1.2.43" {
		t.Errorf("normalized origin_release = %q, want v1.2.43", n.OriginRelease)
	}

	// PlatformRelease is left empty by normalizeReleaseEntry — it is set
	// by the sync pipeline after normalization (n.PlatformRelease = releaseTag).
	// This confirms it's metadata assignment, not a version override.
	if n.PlatformRelease != "" {
		t.Errorf("normalizeReleaseEntry must NOT set PlatformRelease, got %q — "+
			"PlatformRelease is assigned post-normalization in sync pipeline", n.PlatformRelease)
	}
}

// TestVA2_NormalizeEntry_OriginReleaseDefaultsToReleaseTagNotPlatformRelease
// confirms origin_release falls back to entry.ReleaseTag when absent,
// NOT to the platform_release. (ReleaseTag is set by CI to the tag the
// artifact was published under; it may equal platform_release for changed
// packages, but must NOT for unchanged packages.)
func TestVA2_NormalizeEntry_OriginReleaseDefaultsToReleaseTagNotPlatformRelease(t *testing.T) {
	entry := &releaseIndexEntry{
		Name:          "dns",
		Kind:          "SERVICE",
		Publisher:     "core@globular.io",
		Version:       "1.2.44",
		BuildNumber:   1,
		BuildID:       "upstream-dns-001",
		Platform:      "linux_amd64",
		PackageDigest: "sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee",
		AssetURL:      "https://example.com/v1.2.44/dns.tgz",
		ReleaseTag:    "v1.2.44", // this is where the artifact actually lives
		OriginRelease: "",        // absent — must default to ReleaseTag
	}

	n := normalizeReleaseEntry(entry, &repopb.UpstreamSource{})

	if n.OriginRelease != "v1.2.44" {
		t.Errorf("origin_release default = %q, want v1.2.44 (entry.ReleaseTag)", n.OriginRelease)
	}
	// Not a version contamination: origin_release is provenance metadata,
	// n.Version is still 1.2.44 (the package's own version).
	if n.Version != "1.2.44" {
		t.Errorf("version = %q, want 1.2.44", n.Version)
	}
}

// ── VA-3: Non-PUBLISHED artifacts excluded from install resolution ──────────

// TestVA3_NonPublishedArtifact_NotInstallable verifies that isRowInstallable
// enforces PUBLISHED-only eligibility for the install path. A STAGING or
// VERIFIED artifact appearing in the ledger must never be selected as an
// install candidate — it hasn't cleared the artifact law gate.
func TestVA3_NonPublishedArtifact_NotInstallable(t *testing.T) {
	publishedStr := repopb.PublishState_PUBLISHED.String()

	tests := []struct {
		publishState  string
		artifactState string
		wantInstall   bool
		desc          string
	}{
		{publishedStr, "", true, "PUBLISHED + empty artifact_state (legacy compat)"},
		{publishedStr, "PUBLISHED", true, "PUBLISHED + PUBLISHED artifact_state"},
		{"STAGING", "", false, "STAGING publish_state must not install"},
		{"VERIFIED", "", false, "VERIFIED publish_state must not install — artifact law gate not cleared"},
		{"BLOB_VERIFIED", "", false, "BLOB_VERIFIED must not install"},
		{"DOWNLOADING", "", false, "DOWNLOADING must not install"},
		{"FAILED", "", false, "FAILED must not install"},
		{publishedStr, "DOWNLOADING", false, "PUBLISHED + DOWNLOADING artifact_state — pipeline incomplete"},
		{publishedStr, "BROKEN_BINARY", false, "PUBLISHED + BROKEN_BINARY must not install"},
		{publishedStr, "QUARANTINED", false, "PUBLISHED + QUARANTINED must not install"},
	}

	for _, tc := range tests {
		row := &manifestRow{
			PublishState:  tc.publishState,
			ArtifactState: tc.artifactState,
		}
		got := isRowInstallable(row)
		if got != tc.wantInstall {
			t.Errorf("isRowInstallable{publish=%s, artifact=%s}: got=%v want=%v — %s",
				tc.publishState, tc.artifactState, got, tc.wantInstall, tc.desc)
		}
	}
}

// TestVA3_VERIFIED_ArtifactExcludedFromResolver ensures that a VERIFIED
// (not yet PUBLISHED) artifact is not returned by ResolveArtifact.
// VERIFIED means download + digest check passed but completePublish hasn't
// run — the artifact law gate is not cleared and it must not reach the reconciler.
func TestVA3_VERIFIED_ArtifactExcludedFromResolver(t *testing.T) {
	srv := newTestServer(t)

	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io",
		Name:        "storage",
		Version:     "1.2.43",
		Platform:    "linux_amd64",
		Kind:        repopb.ArtifactKind_SERVICE,
	}

	// Seed in VERIFIED state — not PUBLISHED.
	key := artifactKeyWithBuild(ref, 143)
	mjson, err := marshalManifestWithState(&repopb.ArtifactManifest{
		Ref:         ref,
		BuildNumber: 143,
		BuildId:     "stor-uuid-va3",
		Checksum:    "sha256:aaaa",
	}, repopb.PublishState_VERIFIED)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := srv.Storage().MkdirAll(context.Background(), artifactsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := srv.Storage().WriteFile(context.Background(), manifestStorageKey(key), mjson, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}

	// ResolveArtifact must not find the VERIFIED artifact.
	_, err = srv.ResolveArtifact(context.Background(), &repopb.ResolveArtifactRequest{
		PublisherId: "core@globular.io",
		Name:        "storage",
		Version:     "1.2.43",
		Platform:    "linux_amd64",
	})
	if err == nil {
		t.Fatal("expected NOT_FOUND for VERIFIED artifact, got nil error")
	}
	errStr := strings.ToLower(err.Error())
	if !strings.Contains(errStr, "not found") && !strings.Contains(errStr, "notfound") {
		t.Errorf("expected not-found error, got: %v", err)
	}
}

// ── VA-4: build_id is install identity; build_number is display-only ────────

// TestVA4_BuildIDIsInstallIdentity_SameVersionTwoBuilds verifies that two
// artifacts at the same version but different build_ids are distinct install
// identities. The convergence decision must be based on build_id, not
// build_number or version alone.
//
// Scenario: CI produces two builds of storage@1.2.43 (e.g. a hotfix rebuild).
// The node has build A installed. Desired state specifies build B by build_id.
// The reconciler must install B, not skip because version is identical.
func TestVA4_BuildIDIsInstallIdentity_SameVersionTwoBuilds(t *testing.T) {
	srv := newTestServer(t)

	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io",
		Name:        "storage",
		Version:     "1.2.43",
		Platform:    "linux_amd64",
		Kind:        repopb.ArtifactKind_SERVICE,
	}

	// Seed build A.
	seedPublishedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref:         ref,
		BuildNumber: 143,
		BuildId:     "stor-build-A-uuid",
		Checksum:    "sha256:aaaa",
		SizeBytes:   100,
	})
	// Seed build B (same version, different build_id).
	seedPublishedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref:         ref,
		BuildNumber: 144,
		BuildId:     "stor-build-B-uuid",
		Checksum:    "sha256:bbbb",
		SizeBytes:   100,
	})

	// Resolve by build_id=A → must return exactly build A.
	respA, err := srv.ResolveArtifact(context.Background(), &repopb.ResolveArtifactRequest{
		PublisherId: "core@globular.io",
		Name:        "storage",
		Version:     "1.2.43",
		Platform:    "linux_amd64",
		BuildId:     "stor-build-A-uuid",
	})
	if err != nil {
		t.Fatalf("resolve build A: %v", err)
	}
	if respA.GetManifest().GetBuildId() != "stor-build-A-uuid" {
		t.Errorf("expected build_id=stor-build-A-uuid, got %q", respA.GetManifest().GetBuildId())
	}

	// Resolve by build_id=B → must return exactly build B.
	respB, err := srv.ResolveArtifact(context.Background(), &repopb.ResolveArtifactRequest{
		PublisherId: "core@globular.io",
		Name:        "storage",
		Version:     "1.2.43",
		Platform:    "linux_amd64",
		BuildId:     "stor-build-B-uuid",
	})
	if err != nil {
		t.Fatalf("resolve build B: %v", err)
	}
	if respB.GetManifest().GetBuildId() != "stor-build-B-uuid" {
		t.Errorf("expected build_id=stor-build-B-uuid, got %q", respB.GetManifest().GetBuildId())
	}

	// The two are distinct installs even though version is identical.
	if respA.GetManifest().GetBuildId() == respB.GetManifest().GetBuildId() {
		t.Error("build A and build B must have different build_ids")
	}
}

// TestVA4_BuildNumberIsDisplayOnly verifies that build_number alone is
// insufficient to distinguish artifact identity. Two builds at the same
// version with different build_numbers are both valid — the identity anchor
// is build_id (UUID), not build_number (integer display counter).
//
// The reconciler must NEVER use build_number as the convergence key.
// INV-4 in invariant_test.go also covers this; this test adds the version
// authority framing (same version, two distinct identities).
func TestVA4_BuildNumberIsDisplayOnly(t *testing.T) {
	srv := newTestServer(t)

	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io",
		Name:        "dns",
		Version:     "1.2.44",
		Platform:    "linux_amd64",
		Kind:        repopb.ArtifactKind_SERVICE,
	}

	// Seed build 1 and build 2 at same version.
	seedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref:         ref,
		BuildNumber: 1,
		BuildId:     "dns-build-uuid-001",
		Checksum:    "sha256:cccc",
	})
	seedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref:         ref,
		BuildNumber: 2,
		BuildId:     "dns-build-uuid-002",
		Checksum:    "sha256:dddd",
	})

	// Fetch by build_number — they return different build_ids (identity).
	m1, err := srv.GetArtifactManifest(context.Background(),
		&repopb.GetArtifactManifestRequest{Ref: ref, BuildNumber: 1})
	if err != nil {
		t.Fatalf("get build 1: %v", err)
	}
	m2, err := srv.GetArtifactManifest(context.Background(),
		&repopb.GetArtifactManifestRequest{Ref: ref, BuildNumber: 2})
	if err != nil {
		t.Fatalf("get build 2: %v", err)
	}

	// Different build_ids despite same version — build_number is display, not identity.
	if m1.GetManifest().GetBuildId() == m2.GetManifest().GetBuildId() {
		t.Error("builds 1 and 2 must have distinct build_ids — build_number is display-only")
	}
	if m1.GetManifest().GetBuildId() != "dns-build-uuid-001" {
		t.Errorf("build 1 id = %q, want dns-build-uuid-001", m1.GetManifest().GetBuildId())
	}
	if m2.GetManifest().GetBuildId() != "dns-build-uuid-002" {
		t.Errorf("build 2 id = %q, want dns-build-uuid-002", m2.GetManifest().GetBuildId())
	}
}

// ── VA-5: Unchanged packages carry origin version across platform upgrades ──

// TestVA5_UnchangedPackageVersionSurvivesPlatformUpgrade simulates a platform
// upgrade from 1.2.43 → 1.2.52 where dns=1.2.44 is unchanged. The new BOM
// must carry dns.version=1.2.44, not dns.version=1.2.52.
//
// This is the root cause of the version authority bug that the cleanup spec
// targets: if the CI pipeline stamps all packages with the platform_release,
// the reconciler requests dns@1.2.52 which doesn't exist, causing Day-1
// convergence failure for unchanged packages.
func TestVA5_UnchangedPackageVersionSurvivesPlatformUpgrade(t *testing.T) {
	changed := true
	unchanged := false

	// BOM for platform release 1.2.52. dns=1.2.44 is unchanged from v1.2.44.
	bom := GenerateReleaseIndexV2("v1.2.52", "1.2.52", "core@globular.io",
		[]string{"v1.2.44"}, false, "", []*releaseIndexEntry{
			{
				Name:             "node-agent",
				Kind:             "INFRA",
				Publisher:        "core@globular.io",
				Version:          "1.2.52",
				BuildNumber:      52,
				BuildID:          "node-agent-build-052",
				Platform:         "linux_amd64",
				PackageDigest:    "sha256:1111111111111111111111111111111111111111111111111111111111111111",
				AssetURL:         "https://example.com/v1.2.52/node-agent.tgz",
				ReleaseTag:       "v1.2.52",
				OriginRelease:    "v1.2.52",
				ChangedInRelease: &changed,
			},
			{
				Name:             "dns",
				Kind:             "SERVICE",
				Publisher:        "core@globular.io",
				Version:          "1.2.44", // unchanged — must not become 1.2.52
				BuildNumber:      44,
				BuildID:          "dns-build-044",
				Platform:         "linux_amd64",
				PackageDigest:    "sha256:2222222222222222222222222222222222222222222222222222222222222222",
				AssetURL:         "https://example.com/v1.2.44/dns.tgz",
				ReleaseTag:       "v1.2.52",
				OriginRelease:    "v1.2.44",
				ChangedInRelease: &unchanged,
			},
		})

	if err := ValidateReleaseIndex(bom); err != nil {
		t.Fatalf("BOM validation: %v", err)
	}

	// dns version must be its own, not the platform version.
	for _, pkg := range bom.Packages {
		if pkg.Name == "dns" {
			if pkg.Version == bom.PlatformRelease {
				t.Errorf("dns.version (%s) == platform_release (%s): "+
					"unchanged package was wrongly stamped with platform release — "+
					"this causes convergence failure (reconciler requests dns@1.2.52 which doesn't exist)",
					pkg.Version, bom.PlatformRelease)
			}
			if pkg.Version != "1.2.44" {
				t.Errorf("dns.version = %q, want 1.2.44", pkg.Version)
			}
			if pkg.OriginRelease != "v1.2.44" {
				t.Errorf("dns.origin_release = %q, want v1.2.44", pkg.OriginRelease)
			}
			if pkg.IsChanged() {
				t.Error("dns.changed_in_release should be false")
			}
		}
	}

	// The node-agent is changed and carries the new platform version.
	for _, pkg := range bom.Packages {
		if pkg.Name == "node-agent" {
			if pkg.Version != bom.PlatformRelease {
				t.Errorf("node-agent.version (%s) should equal platform_release (%s) for changed packages",
					pkg.Version, bom.PlatformRelease)
			}
		}
	}

	// referenced_releases must include the origin tag so nodes can find unchanged artifacts.
	found := false
	for _, r := range bom.ReferencedReleases {
		if r == "v1.2.44" {
			found = true
		}
	}
	if !found {
		t.Errorf("referenced_releases must include v1.2.44 (dns origin), got: %v", bom.ReferencedReleases)
	}
}

// ── VA-6: Build-ID Immutability ────────────────────────────────────────────
//
// Once a (name, version, platform) tuple is PUBLISHED, it is permanently bound
// to its first build_id. Re-publishing under the same version must be rejected.
//
// Without this invariant, the CI pipeline can republish unchanged packages under
// the same version, generating a new build_id. Desired state updates to the new
// build_id; nodes carry the old one → "build drift" warnings that never clear.

// TestVA6_AppendToLedger_SameVersionPlatform_Rejected verifies that
// appendToLedger rejects a second (version, platform) entry with a different
// build_id. This is the core ledger-level enforcement of version immutability.
func TestVA6_AppendToLedger_SameVersionPlatform_Rejected(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	// First publish: workflow@1.2.43 linux_amd64 with build_id "build-aaa"
	if err := srv.appendToLedger(ctx, "core@globular.io", "workflow", "1.2.43", "build-aaa", "sha256:aaa", "linux_amd64", 1000); err != nil {
		t.Fatalf("first appendToLedger: %v", err)
	}

	// Second publish: same version+platform, different build_id — must be rejected.
	err := srv.appendToLedger(ctx, "core@globular.io", "workflow", "1.2.43", "build-bbb", "sha256:bbb", "linux_amd64", 1000)
	if err == nil {
		t.Fatal("expected error re-publishing same (version, platform) with different build_id, got nil")
	}
	if !strings.Contains(err.Error(), "already published") {
		t.Errorf("expected 'already published' error, got: %v", err)
	}
}

// TestVA6_AppendToLedger_SameVersionSameBuildID_Idempotent verifies that
// re-promoting the exact same build_id is a no-op (idempotent re-promote).
func TestVA6_AppendToLedger_SameVersionSameBuildID_Idempotent(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	if err := srv.appendToLedger(ctx, "core@globular.io", "workflow", "1.2.43", "build-aaa", "sha256:aaa", "linux_amd64", 1000); err != nil {
		t.Fatalf("first appendToLedger: %v", err)
	}

	// Re-promote same build_id — must succeed silently.
	if err := srv.appendToLedger(ctx, "core@globular.io", "workflow", "1.2.43", "build-aaa", "sha256:aaa", "linux_amd64", 1000); err != nil {
		t.Errorf("idempotent re-promote must not return an error, got: %v", err)
	}
}

// TestVA6_AppendToLedger_SameVersionDifferentPlatform_Allowed verifies that
// (version, platform_A) and (version, platform_B) can coexist — multi-platform
// builds are legitimate.
func TestVA6_AppendToLedger_SameVersionDifferentPlatform_Allowed(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	if err := srv.appendToLedger(ctx, "core@globular.io", "workflow", "1.2.43", "build-aaa", "sha256:aaa", "linux_amd64", 1000); err != nil {
		t.Fatalf("linux_amd64 appendToLedger: %v", err)
	}
	if err := srv.appendToLedger(ctx, "core@globular.io", "workflow", "1.2.43", "build-bbb", "sha256:bbb", "linux_arm64", 1000); err != nil {
		t.Errorf("linux_arm64 appendToLedger must be allowed for same version, got: %v", err)
	}
}

// TestVA6_GetExactRelease_ReturnsCanonicalBuildID verifies that getExactRelease
// returns the build_id for an exact (name, version, platform) match and ""
// when no match exists.
func TestVA6_GetExactRelease_ReturnsCanonicalBuildID(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	if err := srv.appendToLedger(ctx, "core@globular.io", "dns", "1.2.44", "build-dns-01", "sha256:dns01", "linux_amd64", 500); err != nil {
		t.Fatalf("appendToLedger: %v", err)
	}

	bid := srv.getExactRelease(ctx, "core@globular.io", "dns", "1.2.44", "linux_amd64")
	if bid != "build-dns-01" {
		t.Errorf("getExactRelease = %q, want %q", bid, "build-dns-01")
	}

	// Unknown version must return "".
	if bid := srv.getExactRelease(ctx, "core@globular.io", "dns", "1.2.99", "linux_amd64"); bid != "" {
		t.Errorf("unknown version must return empty, got %q", bid)
	}

	// Unknown package must return "".
	if bid := srv.getExactRelease(ctx, "core@globular.io", "unknownpkg", "1.2.44", "linux_amd64"); bid != "" {
		t.Errorf("unknown package must return empty, got %q", bid)
	}
}

// TestVA6_LatestBuildID_NotOverwrittenBySameVersion verifies that the ledger's
// LatestBuildID is not overwritten when the same version is attempted again
// (the second write is rejected, so LatestBuildID stays bound to the first).
func TestVA6_LatestBuildID_NotOverwrittenBySameVersion(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	if err := srv.appendToLedger(ctx, "core@globular.io", "storage", "1.2.43", "build-orig", "sha256:orig", "linux_amd64", 2000); err != nil {
		t.Fatalf("first appendToLedger: %v", err)
	}

	// Attempt re-publish — must fail.
	_ = srv.appendToLedger(ctx, "core@globular.io", "storage", "1.2.43", "build-new", "sha256:new", "linux_amd64", 2000)

	// LatestBuildID must still be the original.
	_, latestBID := srv.getLatestRelease(ctx, "core@globular.io", "storage", "linux_amd64")
	if latestBID != "build-orig" {
		t.Errorf("LatestBuildID = %q after rejected re-publish, want %q — re-publish must not overwrite canonical build_id", latestBID, "build-orig")
	}
}
