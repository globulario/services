package main

// release_index.go — Canonical schema definition and validation for
// release-index.json, the contract between CI/publish tooling and the
// SyncFromUpstream import pipeline.
//
// Schema version: globular.repository.index/v1
//
// The structs here are the single source of truth. sync_from_upstream.go
// uses them for deserialization; future CI tooling will use
// GenerateReleaseIndex to produce them.

import (
	"fmt"
	"strings"
)

// SchemaVersionV1 is the required schema_version for release index v1.
const SchemaVersionV1 = "globular.repository.index/v1"

// maxReleaseIndexBytes caps the HTTP response for a release index to
// prevent OOM from a malicious or broken upstream. 10 MiB is generous
// for any realistic index (typical is <100 KiB).
const maxReleaseIndexBytes = 10 << 20 // 10 MiB

// maxArtifactBytes caps a single artifact download. 500 MiB should
// cover any realistic Globular package.
const maxArtifactBytes = 500 << 20 // 500 MiB

// releaseIndex is the top-level release-index.json document.
type releaseIndex struct {
	SchemaVersion   string               `json:"schema_version"`
	ReleaseTag      string               `json:"release_tag"`
	GlobularVersion string               `json:"globular_version"`
	Publisher       string               `json:"publisher"`
	Packages        []*releaseIndexEntry `json:"packages"`
}

// releaseIndexEntry describes one downloadable package within a release.
type releaseIndexEntry struct {
	Name               string `json:"name"`
	Kind               string `json:"kind"`
	Publisher          string `json:"publisher"`
	Version            string `json:"version"`
	BuildID            string `json:"build_id"`
	Platform           string `json:"platform"`
	Filename           string `json:"filename"`
	PackageDigest      string `json:"package_digest"`
	EntrypointChecksum string `json:"entrypoint_checksum"`
	AssetURL           string `json:"asset_url"`
	ReleaseTag         string `json:"release_tag"`
	PublishedAt        string `json:"published_at"`
}

// ValidateReleaseIndex checks the index for structural correctness.
// Returns nil if valid, or an error describing the first violation.
func ValidateReleaseIndex(idx *releaseIndex) error {
	if idx == nil {
		return fmt.Errorf("release index is nil")
	}

	// Schema version is required and must be v1.
	if idx.SchemaVersion == "" {
		return fmt.Errorf("schema_version is required (expected %q)", SchemaVersionV1)
	}
	if idx.SchemaVersion != SchemaVersionV1 {
		return fmt.Errorf("unsupported schema_version %q (expected %q)", idx.SchemaVersion, SchemaVersionV1)
	}

	if idx.ReleaseTag == "" {
		return fmt.Errorf("release_tag is required")
	}

	for i, e := range idx.Packages {
		if err := validateReleaseIndexEntry(e, i); err != nil {
			return err
		}
	}
	return nil
}

// validateReleaseIndexEntry checks a single package entry.
func validateReleaseIndexEntry(e *releaseIndexEntry, idx int) error {
	if e == nil {
		return fmt.Errorf("packages[%d]: entry is nil", idx)
	}
	if e.Name == "" {
		return fmt.Errorf("packages[%d]: name is required", idx)
	}
	if e.Version == "" {
		return fmt.Errorf("packages[%d] (%s): version is required", idx, e.Name)
	}
	if e.Platform == "" {
		return fmt.Errorf("packages[%d] (%s): platform is required", idx, e.Name)
	}
	if e.AssetURL == "" {
		return fmt.Errorf("packages[%d] (%s): asset_url is required", idx, e.Name)
	}
	if e.PackageDigest == "" {
		return fmt.Errorf("packages[%d] (%s): package_digest is required", idx, e.Name)
	}
	if !strings.HasPrefix(e.PackageDigest, "sha256:") {
		return fmt.Errorf("packages[%d] (%s): package_digest must start with \"sha256:\" (got %q)", idx, e.Name, e.PackageDigest)
	}
	hexPart := strings.TrimPrefix(e.PackageDigest, "sha256:")
	if len(hexPart) != 64 {
		return fmt.Errorf("packages[%d] (%s): package_digest sha256 hex must be 64 chars (got %d)", idx, e.Name, len(hexPart))
	}
	return nil
}

// GenerateReleaseIndex creates a release-index.json document from the
// given parameters. Intended for CI/publish tooling — not used by the
// sync pipeline itself.
func GenerateReleaseIndex(tag, publisher, globularVersion string, entries []*releaseIndexEntry) *releaseIndex {
	return &releaseIndex{
		SchemaVersion:   SchemaVersionV1,
		ReleaseTag:      tag,
		GlobularVersion: globularVersion,
		Publisher:       publisher,
		Packages:        entries,
	}
}
