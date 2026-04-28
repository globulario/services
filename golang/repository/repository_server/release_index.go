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
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"strings"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
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
	BuildNumber        int64  `json:"build_number"`         // explicit numeric build number
	BuildID            string `json:"build_id"`             // string build identity
	Channel            string `json:"channel"`              // "stable", "candidate", etc.
	Platform           string `json:"platform"`
	Filename           string `json:"filename"`
	PackageDigest      string `json:"package_digest"`
	EntrypointChecksum string `json:"entrypoint_checksum"`
	AssetURL           string `json:"asset_url"`
	ReleaseTag         string `json:"release_tag"`
	PublishedAt        string `json:"published_at"`
}

// normalizedEntry holds the fully resolved identity of a release index entry.
// Computed once by normalizeReleaseEntry, then used for policy, conflict
// detection, import, and ledger operations.
type normalizedEntry struct {
	Publisher   string
	Name        string
	Version     string
	Platform    string
	Kind        string
	Channel     string
	BuildNumber int64
	BuildID     string
	Digest      string
	AssetURL    string

	EntrypointChecksum string
	ReleaseTag         string
}

// normalizeReleaseEntry resolves all identity fields from the raw entry + source.
// This is the single place where defaults are applied.
func normalizeReleaseEntry(entry *releaseIndexEntry, src *repopb.UpstreamSource) *normalizedEntry {
	n := &normalizedEntry{
		Publisher:          entry.Publisher,
		Name:               entry.Name,
		Version:            entry.Version,
		Platform:           entry.Platform,
		Kind:               entry.Kind,
		Digest:             entry.PackageDigest,
		AssetURL:           entry.AssetURL,
		EntrypointChecksum: entry.EntrypointChecksum,
		ReleaseTag:         entry.ReleaseTag,
	}

	// Publisher fallback chain: entry → source.default_publisher_id → "core@globular.io"
	if n.Publisher == "" {
		if dp := src.GetDefaultPublisherId(); dp != "" {
			n.Publisher = dp
		} else {
			n.Publisher = "core@globular.io"
		}
	}

	// Channel fallback chain: entry → source.channel → "stable"
	n.Channel = entry.Channel
	if n.Channel == "" {
		n.Channel = src.GetChannel()
	}
	if n.Channel == "" {
		n.Channel = "stable"
	}

	// build_number: prefer explicit from entry.
	n.BuildNumber = entry.BuildNumber

	// build_id: preserve upstream, derive if missing.
	n.BuildID = entry.BuildID
	if n.BuildID == "" {
		n.BuildID = deriveUpstreamBuildID(n.Publisher, n.Name, n.Version, n.Platform, n.Digest)
	}

	// If build_number is 0 (missing/default), derive deterministically from build_id
	// to avoid collisions when multiple entries lack build_number.
	if n.BuildNumber == 0 {
		n.BuildNumber = deriveBuildNumber(n.BuildID, n.Digest)
	}

	return n
}

// deriveBuildNumber produces a deterministic positive build_number from a
// build_id and digest. Uses hash to avoid collisions. Result is always >= 1.
func deriveBuildNumber(buildID, digest string) int64 {
	h := sha256.New()
	h.Write([]byte(buildID))
	h.Write([]byte{0})
	h.Write([]byte(digest))
	sum := h.Sum(nil)
	// Take first 7 bytes → positive int64 in range [1, 2^56).
	// Shift right one bit to stay safely positive.
	v := int64(binary.BigEndian.Uint64(append([]byte{0}, sum[:7]...)))
	if v <= 0 {
		v = 1
	}
	return v
}

// deriveUpstreamBuildID produces a deterministic build_id from the package
// identity and content hash. Used only when the upstream release index has no
// build_id. The result is a sha256-based string (not a UUIDv7) that is stable
// across repeated imports of the same artifact.
func deriveUpstreamBuildID(publisher, name, version, platform, digest string) string {
	h := sha256.New()
	for _, s := range []string{publisher, name, version, platform, digest} {
		h.Write([]byte(s))
		h.Write([]byte{0})
	}
	return "upstream:" + hex.EncodeToString(h.Sum(nil))[:32]
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
