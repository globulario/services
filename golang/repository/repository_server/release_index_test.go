package main

import (
	"encoding/json"
	"strings"
	"testing"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

func validEntry() *releaseIndexEntry {
	changed := true
	return &releaseIndexEntry{
		Name:                  "echo",
		Kind:                  "SERVICE",
		Publisher:             "core@globular.io",
		Version:               "1.0.53",
		BuildNumber:           67,
		BuildID:               "67",
		Channel:               "stable",
		Platform:              "linux_amd64",
		Filename:              "echo-1.0.53-linux_amd64.tgz",
		PackageDigest:         "sha256:abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		PackageContractDigest: "sha256:1111111111111111111111111111111111111111111111111111111111111111",
		AssetURL:              "https://example.com/echo.tgz",
		ReleaseTag:            "v1.0.53",
		OriginRelease:         "v1.0.53",
		ChangedInRelease:      &changed,
	}
}

func validIndex() *releaseIndex {
	sv, _ := json.Marshal(SchemaVersionV1)
	return &releaseIndex{
		SchemaVersion:   sv,
		ReleaseTag:      "v1.0.53",
		GlobularVersion: "1.0.53",
		Publisher:       "core@globular.io",
		Packages:        []*releaseIndexEntry{validEntry()},
	}
}

func validV2Index() *releaseIndex {
	sv, _ := json.Marshal(SchemaVersionV2)
	changed := true
	unchanged := false
	return &releaseIndex{
		SchemaVersion:      sv,
		PlatformRelease:    "1.0.84",
		ReleaseTag:         "v1.0.84",
		Publisher:          "core@globular.io",
		ReferencedReleases: []string{"v1.0.82"},
		Packages: []*releaseIndexEntry{
			{
				Name: "repository", Kind: "SERVICE", Publisher: "core@globular.io",
				Version: "1.0.84", BuildNumber: 24, BuildID: "24",
				Platform:              "linux_amd64",
				PackageDigest:         "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				PackageContractDigest: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
				AssetURL:              "https://example.com/v1.0.84/repository.tgz",
				ReleaseTag:            "v1.0.84",
				OriginRelease:         "v1.0.84",
				ChangedInRelease:      &changed,
			},
			{
				Name: "gateway", Kind: "SERVICE", Publisher: "core@globular.io",
				Version: "1.0.82", BuildNumber: 9, BuildID: "9",
				Platform:              "linux_amd64",
				PackageDigest:         "sha256:cccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccccc",
				PackageContractDigest: "sha256:dddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddddd",
				AssetURL:              "https://example.com/v1.0.82/gateway.tgz",
				ReleaseTag:            "v1.0.84",
				OriginRelease:         "v1.0.82",
				ChangedInRelease:      &unchanged,
			},
		},
	}
}

// ── v1 backward compat ──────────────────────────────────────────────────────

func TestValidateReleaseIndex_V1_Valid(t *testing.T) {
	if err := ValidateReleaseIndex(validIndex()); err != nil {
		t.Fatalf("expected valid, got: %v", err)
	}
}

func TestValidateReleaseIndex_Nil(t *testing.T) {
	if err := ValidateReleaseIndex(nil); err == nil {
		t.Fatal("expected error for nil index")
	}
}

func TestValidateReleaseIndex_MissingSchemaVersion(t *testing.T) {
	idx := validIndex()
	idx.SchemaVersion = nil
	err := ValidateReleaseIndex(idx)
	if err == nil || !strings.Contains(err.Error(), "schema_version") {
		t.Fatalf("expected schema_version error, got: %v", err)
	}
}

func TestValidateReleaseIndex_WrongSchemaVersion(t *testing.T) {
	idx := validIndex()
	idx.SchemaVersion, _ = json.Marshal("globular.repository.index/v99")
	err := ValidateReleaseIndex(idx)
	if err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("expected unsupported error, got: %v", err)
	}
}

func TestValidateReleaseIndex_LegacyIntegerSchemaVersion(t *testing.T) {
	idx := validIndex()
	idx.SchemaVersion, _ = json.Marshal(1)
	if err := ValidateReleaseIndex(idx); err != nil {
		t.Fatalf("legacy integer 1 should be accepted: %v", err)
	}
}

func TestValidateReleaseIndex_MissingReleaseTag(t *testing.T) {
	idx := validIndex()
	idx.ReleaseTag = ""
	err := ValidateReleaseIndex(idx)
	if err == nil || !strings.Contains(err.Error(), "release_tag") {
		t.Fatalf("expected release_tag error, got: %v", err)
	}
}

func TestValidateReleaseIndex_EntryMissingName(t *testing.T) {
	idx := validIndex()
	idx.Packages[0].Name = ""
	err := ValidateReleaseIndex(idx)
	if err == nil || !strings.Contains(err.Error(), "name is required") {
		t.Fatalf("expected name error, got: %v", err)
	}
}

func TestValidateReleaseIndex_EntryMissingVersion(t *testing.T) {
	idx := validIndex()
	idx.Packages[0].Version = ""
	err := ValidateReleaseIndex(idx)
	if err == nil || !strings.Contains(err.Error(), "version is required") {
		t.Fatalf("expected version error, got: %v", err)
	}
}

func TestValidateReleaseIndex_EntryMissingDigest(t *testing.T) {
	idx := validIndex()
	idx.Packages[0].PackageDigest = ""
	idx.Packages[0].ArtifactSha256 = ""
	err := ValidateReleaseIndex(idx)
	if err == nil || !strings.Contains(err.Error(), "digest") {
		t.Fatalf("expected digest error, got: %v", err)
	}
}

func TestValidateReleaseIndex_EntryBadDigestPrefix(t *testing.T) {
	idx := validIndex()
	idx.Packages[0].PackageDigest = "md5:abc123"
	err := ValidateReleaseIndex(idx)
	if err == nil || !strings.Contains(err.Error(), "sha256:") {
		t.Fatalf("expected sha256 prefix error, got: %v", err)
	}
}

func TestValidateReleaseIndex_EntryBadDigestLength(t *testing.T) {
	idx := validIndex()
	idx.Packages[0].PackageDigest = "sha256:tooshort"
	err := ValidateReleaseIndex(idx)
	if err == nil || !strings.Contains(err.Error(), "64 chars") {
		t.Fatalf("expected hex length error, got: %v", err)
	}
}

func TestValidateReleaseIndex_EntryMissingAssetURL(t *testing.T) {
	idx := validIndex()
	idx.Packages[0].AssetURL = ""
	err := ValidateReleaseIndex(idx)
	if err == nil || !strings.Contains(err.Error(), "asset_url") {
		t.Fatalf("expected asset_url error, got: %v", err)
	}
}

func TestValidateReleaseIndex_EntryMissingPlatform(t *testing.T) {
	idx := validIndex()
	idx.Packages[0].Platform = ""
	err := ValidateReleaseIndex(idx)
	if err == nil || !strings.Contains(err.Error(), "platform") {
		t.Fatalf("expected platform error, got: %v", err)
	}
}

func TestGenerateReleaseIndex(t *testing.T) {
	e := validEntry()
	idx := GenerateReleaseIndex("v1.0.53", "core@globular.io", "1.0.53", []*releaseIndexEntry{e})
	if err := ValidateReleaseIndex(idx); err != nil {
		t.Fatalf("generated index should be valid: %v", err)
	}
}

// ── v2 BOM model ────────────────────────────────────────────────────────────

func TestValidateV2_WithCompositionFields(t *testing.T) {
	idx := validV2Index()
	if err := ValidateReleaseIndex(idx); err != nil {
		t.Fatalf("v2 BOM index should be valid: %v", err)
	}
	if !idx.IsV2() {
		t.Fatal("expected IsV2() to be true")
	}
}

func TestValidateV2_MixedVersions(t *testing.T) {
	idx := validV2Index()
	if err := ValidateReleaseIndex(idx); err != nil {
		t.Fatalf("mixed-version v2 index should be valid: %v", err)
	}
	if idx.Packages[0].Version == idx.Packages[1].Version {
		t.Fatal("test setup: packages should have different versions")
	}
}

func TestValidateV2_MissingChangedInRelease_Invalid(t *testing.T) {
	idx := validV2Index()
	idx.Packages[0].ChangedInRelease = nil // explicitly absent
	err := ValidateReleaseIndex(idx)
	if err == nil || !strings.Contains(err.Error(), "changed_in_release is required") {
		t.Fatalf("v2 should require explicit changed_in_release, got: %v", err)
	}
}

func TestValidateV2_UnchangedMissingOriginRelease_Invalid(t *testing.T) {
	idx := validV2Index()
	idx.Packages[1].OriginRelease = "" // unchanged but no origin
	err := ValidateReleaseIndex(idx)
	if err == nil || !strings.Contains(err.Error(), "origin_release is required") {
		t.Fatalf("v2 unchanged entry should require origin_release, got: %v", err)
	}
}

func TestValidateV1_MissingChangedInRelease_DefaultsTrue(t *testing.T) {
	idx := validIndex()
	idx.Packages[0].ChangedInRelease = nil // absent in v1
	if err := ValidateReleaseIndex(idx); err != nil {
		t.Fatalf("v1 should accept missing changed_in_release: %v", err)
	}
	if !idx.Packages[0].IsChanged() {
		t.Fatal("v1 entry without changed_in_release should default to true")
	}
}

func TestNormalize_OriginReleaseDefaultsToReleaseTag(t *testing.T) {
	entry := validEntry()
	entry.OriginRelease = "" // absent
	n := normalizeReleaseEntry(entry, &repopb.UpstreamSource{})
	if n.OriginRelease != entry.ReleaseTag {
		t.Fatalf("expected origin_release=%q, got %q", entry.ReleaseTag, n.OriginRelease)
	}
}

func TestNormalize_PreservesExplicitOriginRelease(t *testing.T) {
	entry := validEntry()
	entry.OriginRelease = "v1.0.50"
	entry.ReleaseTag = "v1.0.84"
	n := normalizeReleaseEntry(entry, &repopb.UpstreamSource{})
	if n.OriginRelease != "v1.0.50" {
		t.Fatalf("expected preserved origin_release=v1.0.50, got %q", n.OriginRelease)
	}
}

func TestNormalize_ArtifactSha256PreferredOverPackageDigest(t *testing.T) {
	entry := validEntry()
	entry.ArtifactSha256 = "sha256:eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee"
	entry.PackageDigest = "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff"
	n := normalizeReleaseEntry(entry, &repopb.UpstreamSource{})
	if n.Digest != entry.ArtifactSha256 {
		t.Fatalf("expected artifact_sha256 to win, got %q", n.Digest)
	}
}

// ── Contract digest ─────────────────────────────────────────────────────────

func TestComputeContractDigest_Deterministic(t *testing.T) {
	c := ContractComponents{
		EntrypointChecksum: "sha256:binary1",
		ManifestSha256:     "sha256:manifest1",
		SpecSha256:         "sha256:spec1",
		SystemdSha256:      "sha256:unit1",
		Profiles:           []string{"core", "compute"},
		HardDeps:           []string{"etcd", "scylladb"},
	}
	d1 := ComputeContractDigest(c)
	d2 := ComputeContractDigest(c)
	if d1 != d2 {
		t.Fatalf("same input should produce same digest: %q vs %q", d1, d2)
	}
}

func TestComputeContractDigest_OrderIndependent(t *testing.T) {
	c1 := ContractComponents{
		Profiles: []string{"compute", "core"},
		HardDeps: []string{"scylladb", "etcd"},
	}
	c2 := ContractComponents{
		Profiles: []string{"core", "compute"},
		HardDeps: []string{"etcd", "scylladb"},
	}
	if ComputeContractDigest(c1) != ComputeContractDigest(c2) {
		t.Fatal("order of profiles/deps should not affect contract digest")
	}
}

func TestComputeContractDigest_ManifestChangeMeansChanged(t *testing.T) {
	base := ContractComponents{
		EntrypointChecksum: "sha256:samebinary",
		ManifestSha256:     "sha256:manifest_v1",
	}
	modified := ContractComponents{
		EntrypointChecksum: "sha256:samebinary",
		ManifestSha256:     "sha256:manifest_v2",
	}
	if ComputeContractDigest(base) == ComputeContractDigest(modified) {
		t.Fatal("different manifest should produce different contract digest even with same binary")
	}
}

func TestComputeContractDigest_BinaryChangeMeansChanged(t *testing.T) {
	base := ContractComponents{
		EntrypointChecksum: "sha256:binary_v1",
		ManifestSha256:     "sha256:samemanifest",
	}
	modified := ContractComponents{
		EntrypointChecksum: "sha256:binary_v2",
		ManifestSha256:     "sha256:samemanifest",
	}
	if ComputeContractDigest(base) == ComputeContractDigest(modified) {
		t.Fatal("different binary should produce different contract digest")
	}
}

func TestIsChanged_NilDefaultsTrue(t *testing.T) {
	e := &releaseIndexEntry{}
	if !e.IsChanged() {
		t.Fatal("nil ChangedInRelease should default to true")
	}
}

func TestIsChanged_ExplicitFalse(t *testing.T) {
	f := false
	e := &releaseIndexEntry{ChangedInRelease: &f}
	if e.IsChanged() {
		t.Fatal("explicit false should return false")
	}
}

func TestIsChanged_ExplicitTrue(t *testing.T) {
	tr := true
	e := &releaseIndexEntry{ChangedInRelease: &tr}
	if !e.IsChanged() {
		t.Fatal("explicit true should return true")
	}
}

// ── GenerateV2 ──────────────────────────────────────────────────────────────

func TestGenerateReleaseIndexV2(t *testing.T) {
	changed := true
	e := validEntry()
	e.ChangedInRelease = &changed
	e.OriginRelease = "v1.0.84"
	idx := GenerateReleaseIndexV2("v1.0.84", "1.0.84", "core@globular.io",
		[]string{"v1.0.82"}, false, "", []*releaseIndexEntry{e})
	if err := ValidateReleaseIndex(idx); err != nil {
		t.Fatalf("generated v2 index should be valid: %v", err)
	}
	if !idx.IsV2() {
		t.Fatal("expected v2")
	}
	if len(idx.ReferencedReleases) != 1 || idx.ReferencedReleases[0] != "v1.0.82" {
		t.Fatalf("referenced_releases: %v", idx.ReferencedReleases)
	}
}
