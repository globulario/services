package main

// release_index.go — Canonical schema definition and validation for
// release-index.json, the contract between CI/publish tooling and the
// SyncFromUpstream import pipeline.
//
// Schema versions:
//   - "globular.repository.index/v1" (Phase 1, also accepts legacy integer 1)
//   - "globular.repository.index/v2" (BOM model: package_contract_digest,
//     origin_release, changed_in_release, referenced_releases)
//
// The structs here are the single source of truth. sync_from_upstream.go
// uses them for deserialization; CI tooling uses GenerateReleaseIndex to
// produce them.
//
// Digest model:
//   package_contract_digest — normalized content identity for change detection.
//     Covers: binary, manifest, specs, systemd units, scripts, profiles, deps.
//     Same content packaged twice → same contract digest even if .tgz metadata differs.
//   artifact_sha256 — byte identity of the .tgz archive for download verification.
//     May differ across builds of identical content due to tar/gzip non-reproducibility.
//   entrypoint_checksum — runtime binary/process fingerprint for reverse lookup.

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

const (
	// SchemaVersionV1 is the required schema_version for release index v1.
	SchemaVersionV1 = "globular.repository.index/v1"
	// SchemaVersionV2 adds BOM composition fields.
	SchemaVersionV2 = "globular.repository.index/v2"

	// maxReleaseIndexBytes caps HTTP response for a release index (10 MiB).
	maxReleaseIndexBytes = 10 << 20
	// maxArtifactBytes caps a single artifact download (500 MiB).
	maxArtifactBytes = 500 << 20
)

var releaseTagPattern = regexp.MustCompile(`^v\d+(\.\d+){1,3}([.\-][0-9A-Za-z._\-]+)?$`)

// releaseIndex is the top-level release-index.json document.
type releaseIndex struct {
	// SchemaVersion identifies the index format. Accepts:
	//   "globular.repository.index/v1", "globular.repository.index/v2", or legacy integer 1.
	// Stored as interface{} during unmarshal to handle integer vs string.
	SchemaVersion json.RawMessage `json:"schema_version"`

	// Platform release metadata
	PlatformRelease        string   `json:"platform_release,omitempty"`
	ReleaseTag             string   `json:"release_tag"`
	GlobularVersion        string   `json:"globular_version"`
	Publisher              string   `json:"publisher"`
	PublisherID            string   `json:"publisher_id,omitempty"`
	GeneratedAt            string   `json:"generated_at,omitempty"`
	PackageDigestAlgorithm string   `json:"package_digest_algorithm,omitempty"`
	ReferencedReleases     []string `json:"referenced_releases,omitempty"`
	ForceFullRebuild       bool     `json:"force_full_rebuild,omitempty"`
	ForceFullRebuildReason string   `json:"force_full_rebuild_reason,omitempty"`

	Packages []*releaseIndexEntry `json:"packages"`

	// parsedSchemaVersion is resolved during validation.
	parsedSchemaVersion string
}

// releaseIndexEntry describes one downloadable package within a release.
type releaseIndexEntry struct {
	Name               string `json:"name"`
	Kind               string `json:"kind"`
	Publisher          string `json:"publisher"`
	Version            string `json:"version"`
	BuildNumber        int64  `json:"build_number"`
	BuildID            string `json:"build_id"`
	Channel            string `json:"channel"`
	Platform           string `json:"platform"`
	Filename           string `json:"filename"`

	// Digest model (3-layer):
	//   PackageContractDigest — normalized content identity for change detection.
	//   ArtifactSha256        — byte identity of .tgz for download verification.
	//   PackageDigest         — legacy alias for ArtifactSha256 (v1 compat).
	//   EntrypointChecksum    — runtime binary fingerprint.
	PackageContractDigest string `json:"package_contract_digest,omitempty"`
	ArtifactSha256        string `json:"artifact_sha256,omitempty"`
	PackageDigest         string `json:"package_digest"`
	Checksum              string `json:"checksum,omitempty"` // legacy alias for artifact_sha256
	EntrypointChecksum    string `json:"entrypoint_checksum"`
	PackageManifestSha256 string `json:"package_manifest_sha256,omitempty"`

	AssetURL    string `json:"asset_url"`
	AssetPath   string `json:"asset_path,omitempty"` // relative path for LOCAL_DIR/GIT_INDEX providers
	ReleaseTag  string `json:"release_tag"`
	PublishedAt string `json:"published_at"`

	// BOM composition fields (v2)
	OriginRelease    string `json:"origin_release,omitempty"`
	ChangedInRelease *bool  `json:"changed_in_release,omitempty"` // pointer: explicit true/false/absent

	// Contract enrichment
	Profiles []string          `json:"profiles,omitempty"`
	Provides []string          `json:"provides,omitempty"`
	Requires []string          `json:"requires,omitempty"`
	Defaults map[string]string `json:"defaults,omitempty"`
	HardDeps []string          `json:"hard_deps,omitempty"`
}

// IsChanged returns whether the package was built in the current platform release.
// For v1/legacy entries without the field, defaults to true.
func (e *releaseIndexEntry) IsChanged() bool {
	if e.ChangedInRelease == nil {
		return true // v1 default: all entries are considered changed
	}
	return *e.ChangedInRelease
}

// boolPtr returns a pointer to a bool value.
func boolPtr(v bool) *bool { return &v }

// normalizedEntry holds the fully resolved identity of a release index entry.
type normalizedEntry struct {
	Publisher   string
	Name        string
	Version     string
	Platform    string
	Kind        string
	Channel     string
	BuildNumber int64
	BuildID     string
	Digest      string // canonical digest for download verification (artifact_sha256 or package_digest)
	AssetURL    string
	AssetPath   string // relative path for LOCAL_DIR/GIT_INDEX providers
	Filename    string // archive filename

	EntrypointChecksum    string
	ReleaseTag            string
	PackageContractDigest string
	OriginRelease         string
	ChangedInRelease      bool
	PlatformRelease       string
}

// normalizeReleaseEntry resolves all identity fields from the raw entry + source.
func normalizeReleaseEntry(entry *releaseIndexEntry, src *repopb.UpstreamSource) *normalizedEntry {
	n := &normalizedEntry{
		Publisher:             entry.Publisher,
		Name:                  entry.Name,
		Version:               entry.Version,
		Platform:              NormalizePlatform(entry.Platform),
		Kind:                  entry.Kind,
		AssetURL:              entry.AssetURL,
		AssetPath:             entry.AssetPath,
		Filename:              entry.Filename,
		EntrypointChecksum:    entry.EntrypointChecksum,
		ReleaseTag:            entry.ReleaseTag,
		PackageContractDigest: entry.PackageContractDigest,
		ChangedInRelease:      entry.IsChanged(),
	}

	// Canonical download-verification digest: prefer artifact_sha256, then
	// package_digest, then legacy checksum.
	n.Digest = NormalizeChecksum(entry.ArtifactSha256)
	if n.Digest == "" {
		n.Digest = NormalizeChecksum(entry.PackageDigest)
	}
	if n.Digest == "" {
		n.Digest = NormalizeChecksum(entry.Checksum)
	}

	// Origin release: explicit → release_tag
	n.OriginRelease = entry.OriginRelease
	if n.OriginRelease == "" {
		n.OriginRelease = entry.ReleaseTag
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

	n.BuildNumber = entry.BuildNumber
	n.BuildID = strings.TrimSpace(entry.BuildID)
	if ValidateBuildID(n.BuildID) != nil && !shouldDeriveUpstreamBuildID(n.BuildID) {
		// Invalid upstream build_id format: derive canonical upstream id.
		n.BuildID = ""
	}
	if shouldDeriveUpstreamBuildID(n.BuildID) {
		n.BuildID = deriveUpstreamBuildID(n.Publisher, n.Name, n.Version, n.Platform, n.Digest)
	}
	if n.BuildNumber == 0 {
		n.BuildNumber = deriveBuildNumber(n.BuildID, n.Digest)
	}

	return n
}

// shouldDeriveUpstreamBuildID returns true when an incoming build_id should be
// replaced by a deterministic upstream-derived identity. Numeric-only IDs are
// unsafe because they often mirror build_number and can collide across packages.
func shouldDeriveUpstreamBuildID(buildID string) bool {
	if buildID == "" {
		return true
	}
	for _, r := range buildID {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// deriveBuildNumber produces a deterministic positive build_number from a
// build_id and digest. Result is always >= 1.
func deriveBuildNumber(buildID, digest string) int64 {
	h := sha256.New()
	h.Write([]byte(buildID))
	h.Write([]byte{0})
	h.Write([]byte(digest))
	sum := h.Sum(nil)
	v := int64(binary.BigEndian.Uint64(append([]byte{0}, sum[:7]...)))
	if v <= 0 {
		v = 1
	}
	return v
}

// deriveUpstreamBuildID produces a deterministic build_id from package identity.
func deriveUpstreamBuildID(publisher, name, version, platform, digest string) string {
	h := sha256.New()
	for _, s := range []string{publisher, name, version, platform, digest} {
		h.Write([]byte(s))
		h.Write([]byte{0})
	}
	return "upstream:" + hex.EncodeToString(h.Sum(nil))[:32]
}

// ── Validation ──────────────────────────────────────────────────────────────

// parseSchemaVersion resolves the schema_version field from JSON.
// Accepts: string "globular.repository.index/v1", "globular.repository.index/v2",
// or legacy integer 1 (treated as v1).
func parseSchemaVersion(raw json.RawMessage) (string, error) {
	if len(raw) == 0 {
		return "", fmt.Errorf("schema_version is required")
	}
	// Try string first.
	var s string
	if err := json.Unmarshal(raw, &s); err == nil {
		return s, nil
	}
	// Try integer (legacy CI writes schema_version: 1).
	var n int
	if err := json.Unmarshal(raw, &n); err == nil {
		if n == 1 {
			return SchemaVersionV1, nil
		}
		if n == 2 {
			return SchemaVersionV2, nil
		}
		return "", fmt.Errorf("unsupported integer schema_version %d", n)
	}
	return "", fmt.Errorf("schema_version must be a string or integer (got %s)", string(raw))
}

// ValidateReleaseIndex checks the index for structural correctness.
func ValidateReleaseIndex(idx *releaseIndex) error {
	if idx == nil {
		return fmt.Errorf("release index is nil")
	}

	sv, err := parseSchemaVersion(idx.SchemaVersion)
	if err != nil {
		return err
	}
	switch sv {
	case SchemaVersionV1, SchemaVersionV2:
		// ok
	default:
		return fmt.Errorf("unsupported schema_version %q", sv)
	}
	idx.parsedSchemaVersion = sv

	if idx.ReleaseTag == "" {
		return fmt.Errorf("release_tag is required")
	}
	if !releaseTagPattern.MatchString(idx.ReleaseTag) {
		return fmt.Errorf("release_tag %q is not a version tag (expected vX.Y.Z); BOM was likely authored with a branch ref instead of the release version", idx.ReleaseTag)
	}

	isV2 := sv == SchemaVersionV2
	for i, e := range idx.Packages {
		if err := validateReleaseIndexEntry(e, i, isV2); err != nil {
			return err
		}
	}
	if isV2 {
		if err := validateNoConflictingBuildIDDigest(idx); err != nil {
			return err
		}
	}
	return nil
}

// ValidateReleaseIndexForInstall applies stricter requirements for official
// Day-0/Day-1 install flows where release-index mistakes must fail closed.
func ValidateReleaseIndexForInstall(idx *releaseIndex) error {
	if idx != nil {
		v, _ := parseSchemaVersion(idx.SchemaVersion)
		if v != SchemaVersionV2 {
			v = idx.parsedSchemaVersion
		}
		if v == SchemaVersionV2 {
			for i, e := range idx.Packages {
				if missing := missingInstallPinFields(e); len(missing) > 0 {
					return fmt.Errorf("packages[%d] (%s): repository.identity.release_index_missing_pins: missing required fields: %s",
						i, e.Name, strings.Join(missing, ","))
				}
			}
		}
	}
	if err := ValidateReleaseIndex(idx); err != nil {
		return err
	}
	if idx == nil || idx.parsedSchemaVersion != SchemaVersionV2 {
		return nil
	}
	for i, e := range idx.Packages {
		if _, ok := kindFromArtifactKindString(strings.TrimSpace(e.Kind)); !ok {
			return fmt.Errorf("packages[%d] (%s): kind %q is not supported for install validation", i, e.Name, e.Kind)
		}
		if isNumericOnly(strings.TrimSpace(e.BuildID)) {
			return fmt.Errorf("packages[%d] (%s): numeric-only build_id is not allowed for install validation", i, e.Name)
		}
		if e.BuildNumber <= 0 {
			return fmt.Errorf("packages[%d] (%s): build_number must be > 0 for install validation", i, e.Name)
		}
		// Backward/forward compatibility:
		// - v2 release indexes may provide package_digest (legacy alias) only.
		// - install validation requires an immutable artifact digest either way.
			if strings.TrimSpace(e.ArtifactSha256) == "" {
				legacy := strings.TrimSpace(e.PackageDigest)
				if legacy == "" {
					legacy = strings.TrimSpace(e.Checksum)
				}
				if legacy == "" {
					return fmt.Errorf("packages[%d] (%s): artifact_sha256, package_digest, or checksum is required for install validation", i, e.Name)
				}
				// Normalize in-memory so downstream install paths that read
				// artifact_sha256 continue to work without special-casing.
				e.ArtifactSha256 = legacy
			}
	}
	return nil
}

func missingInstallPinFields(e *releaseIndexEntry) []string {
	var missing []string
	if strings.TrimSpace(e.Name) == "" {
		missing = append(missing, "name")
	}
	if strings.TrimSpace(e.Platform) == "" {
		missing = append(missing, "platform")
	}
	if strings.TrimSpace(e.Version) == "" {
		missing = append(missing, "version")
	}
	if strings.TrimSpace(e.Kind) == "" {
		missing = append(missing, "kind")
	}
	if strings.TrimSpace(e.Publisher) == "" {
		missing = append(missing, "publisher")
	}
	if strings.TrimSpace(e.BuildID) == "" {
		missing = append(missing, "build_id")
	}
	if strings.TrimSpace(e.ArtifactSha256) == "" &&
		strings.TrimSpace(e.PackageDigest) == "" &&
		strings.TrimSpace(e.Checksum) == "" {
		missing = append(missing, "artifact_sha256")
	}
	return missing
}

// IsV2 returns true if the parsed schema version is v2.
func (idx *releaseIndex) IsV2() bool {
	return idx.parsedSchemaVersion == SchemaVersionV2
}

func validateReleaseIndexEntry(e *releaseIndexEntry, idx int, isV2 bool) error {
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
	// At least one artifact locator must be present.
	if e.AssetURL == "" && e.AssetPath == "" && e.Filename == "" {
		return fmt.Errorf("packages[%d] (%s): asset_url, asset_path, or filename is required", idx, e.Name)
	}

	// Digest: at least one of artifact_sha256, package_digest, or checksum must be present.
	digest := e.ArtifactSha256
	if digest == "" {
		digest = e.PackageDigest
	}
	if digest == "" {
		digest = e.Checksum
	}
	if digest == "" {
		return fmt.Errorf("packages[%d] (%s): artifact_sha256, package_digest, or checksum is required", idx, e.Name)
	}
	if !strings.HasPrefix(digest, "sha256:") {
		return fmt.Errorf("packages[%d] (%s): digest must start with \"sha256:\" (got %q)", idx, e.Name, digest)
	}
	hexPart := strings.TrimPrefix(digest, "sha256:")
	if len(hexPart) != 64 {
		return fmt.Errorf("packages[%d] (%s): digest sha256 hex must be 64 chars (got %d)", idx, e.Name, len(hexPart))
	}

	// v2 strict rules
	if isV2 {
		// changed_in_release must be explicit in v2
		if e.ChangedInRelease == nil {
			return fmt.Errorf("packages[%d] (%s): changed_in_release is required in v2 schema", idx, e.Name)
		}
		// unchanged entries require origin_release and asset_url
		if !e.IsChanged() {
			if e.OriginRelease == "" {
				return fmt.Errorf("packages[%d] (%s): origin_release is required when changed_in_release=false", idx, e.Name)
			}
		}
	}

	return nil
}

func isNumericOnly(v string) bool {
	if v == "" {
		return false
	}
	for _, r := range v {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

// validateNoConflictingBuildIDDigest enforces the hard artifact-identity rule:
// same build_id must never map to different digests within a release index.
// Duplicate digests across different build_ids/build_numbers are allowed and
// are deduped/aliased later in import resolution.
func validateNoConflictingBuildIDDigest(idx *releaseIndex) error {
	seen := make(map[string]string) // build_id -> digest
	for _, e := range idx.Packages {
		buildID := strings.TrimSpace(e.BuildID)
		if buildID == "" {
			continue
		}
		digest := strings.ToLower(strings.TrimSpace(e.ArtifactSha256))
		if digest == "" {
			digest = strings.ToLower(strings.TrimSpace(e.PackageDigest))
		}
		if digest == "" {
			continue
		}
		if prev, ok := seen[buildID]; ok && prev != digest {
			return fmt.Errorf("build_id conflict in release index: build_id=%s has multiple digests (%s vs %s)",
				buildID, prev, digest)
		}
		seen[buildID] = digest
	}
	return nil
}

// ── Generation ──────────────────────────────────────────────────────────────

// GenerateReleaseIndex creates a v1 release-index.json document.
func GenerateReleaseIndex(tag, publisher, globularVersion string, entries []*releaseIndexEntry) *releaseIndex {
	sv, _ := json.Marshal(SchemaVersionV1)
	return &releaseIndex{
		SchemaVersion:   sv,
		ReleaseTag:      tag,
		GlobularVersion: globularVersion,
		Publisher:       publisher,
		Packages:        entries,
	}
}

// GenerateReleaseIndexV2 creates a v2 BOM release-index.json document.
func GenerateReleaseIndexV2(tag, platformRelease, publisher string, referencedReleases []string, forceRebuild bool, forceReason string, entries []*releaseIndexEntry) *releaseIndex {
	sv, _ := json.Marshal(SchemaVersionV2)
	return &releaseIndex{
		SchemaVersion:          sv,
		PlatformRelease:        platformRelease,
		ReleaseTag:             tag,
		Publisher:              publisher,
		ReferencedReleases:     referencedReleases,
		ForceFullRebuild:       forceRebuild,
		ForceFullRebuildReason: forceReason,
		Packages:               entries,
	}
}

// ── Contract Digest ─────────────────────────────────────────────────────────

// ComputeContractDigest computes a normalized package contract digest from
// the content components that define the package's install/runtime contract.
// This is independent of tar/gzip metadata — same content always produces
// the same digest regardless of archive creation parameters.
//
// Components hashed (in order):
//   - entrypoint binary checksum
//   - package manifest (package.json) content normalized
//   - spec file content
//   - systemd unit content
//   - profiles, provides, requires, defaults, hard_deps (sorted)
//
// The caller provides pre-computed hashes of file content.
func ComputeContractDigest(components ContractComponents) string {
	h := sha256.New()
	writeField := func(label, value string) {
		h.Write([]byte(label))
		h.Write([]byte{0})
		h.Write([]byte(value))
		h.Write([]byte{0})
	}

	writeField("entrypoint", components.EntrypointChecksum)
	writeField("manifest", components.ManifestSha256)
	writeField("spec", components.SpecSha256)
	writeField("systemd", components.SystemdSha256)

	// Sort and hash list fields for determinism.
	for _, profile := range sortedCopy(components.Profiles) {
		writeField("profile", profile)
	}
	for _, dep := range sortedCopy(components.HardDeps) {
		writeField("hard_dep", dep)
	}
	for _, prov := range sortedCopy(components.Provides) {
		writeField("provides", prov)
	}
	for _, req := range sortedCopy(components.Requires) {
		writeField("requires", req)
	}
	// Defaults: sort by key.
	for _, k := range sortedKeys(components.Defaults) {
		writeField("default:"+k, components.Defaults[k])
	}

	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}

// ContractComponents holds the pre-computed hashes and metadata that define
// a package's install/runtime contract.
type ContractComponents struct {
	EntrypointChecksum string
	ManifestSha256     string            // sha256 of normalized package.json
	SpecSha256         string            // sha256 of spec yaml
	SystemdSha256      string            // sha256 of systemd unit
	Profiles           []string
	HardDeps           []string
	Provides           []string
	Requires           []string
	Defaults           map[string]string
}

// ComputePackageContractDigest computes the package contract digest directly
// from an artifact manifest.
func ComputePackageContractDigest(manifest *repopb.ArtifactManifest) string {
	if manifest == nil {
		return ComputeContractDigest(ContractComponents{})
	}
	hardDeps := make([]string, 0, len(manifest.GetHardDeps()))
	for _, d := range manifest.GetHardDeps() {
		if d == nil {
			continue
		}
		name := strings.TrimSpace(d.GetName())
		if name != "" {
			hardDeps = append(hardDeps, name)
		}
	}
	return ComputeContractDigest(ContractComponents{
		EntrypointChecksum: manifest.GetEntrypointChecksum(),
		ManifestSha256:     manifest.GetChecksum(),
		Profiles:           manifest.GetProfiles(),
		HardDeps:           hardDeps,
		Provides:           manifest.GetProvides(),
		Requires:           manifest.GetRequires(),
		Defaults:           manifest.GetDefaults(),
	})
}

func sortedCopy(s []string) []string {
	out := make([]string, len(s))
	copy(out, s)
	sortStrings(out)
	return out
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sortStrings(keys)
	return keys
}
