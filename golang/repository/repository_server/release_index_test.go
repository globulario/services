package main

import (
	"strings"
	"testing"
)

func validEntry() *releaseIndexEntry {
	return &releaseIndexEntry{
		Name:          "echo",
		Kind:          "SERVICE",
		Publisher:     "core@globular.io",
		Version:       "1.0.53",
		BuildID:       "67",
		Platform:      "linux_amd64",
		Filename:      "echo-1.0.53-linux_amd64.tgz",
		PackageDigest: "sha256:abcdef0123456789abcdef0123456789abcdef0123456789abcdef0123456789",
		AssetURL:      "https://example.com/echo.tgz",
		ReleaseTag:    "v1.0.53",
	}
}

func validIndex() *releaseIndex {
	return &releaseIndex{
		SchemaVersion:   SchemaVersionV1,
		ReleaseTag:      "v1.0.53",
		GlobularVersion: "1.0.53",
		Publisher:       "core@globular.io",
		Packages:        []*releaseIndexEntry{validEntry()},
	}
}

func TestValidateReleaseIndex_Valid(t *testing.T) {
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
	idx.SchemaVersion = ""
	err := ValidateReleaseIndex(idx)
	if err == nil || !strings.Contains(err.Error(), "schema_version") {
		t.Fatalf("expected schema_version error, got: %v", err)
	}
}

func TestValidateReleaseIndex_WrongSchemaVersion(t *testing.T) {
	idx := validIndex()
	idx.SchemaVersion = "globular.repository.index/v99"
	err := ValidateReleaseIndex(idx)
	if err == nil || !strings.Contains(err.Error(), "unsupported") {
		t.Fatalf("expected unsupported schema error, got: %v", err)
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
	err := ValidateReleaseIndex(idx)
	if err == nil || !strings.Contains(err.Error(), "package_digest is required") {
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
	if idx.SchemaVersion != SchemaVersionV1 {
		t.Fatalf("expected schema %s, got %s", SchemaVersionV1, idx.SchemaVersion)
	}
	if err := ValidateReleaseIndex(idx); err != nil {
		t.Fatalf("generated index should be valid: %v", err)
	}
}
