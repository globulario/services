package main

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

var buildIDPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9._:-]{2,127}$`)

// NormalizePlatform canonicalizes platform strings to the underscore form
// used everywhere else in the codebase: the storage key construction
// (artifactKeyWithBuild), the controller's normalizeArtifactPlatform, the
// hardcoded defaults in release_resolver.go, and the manifests written by
// every package.json. Cross-form aliases ("linux/amd64", "linux-amd64",
// " Linux\\AMD64 ") all converge to "linux_amd64".
//
// Before 2026-05-13 this function produced the slash form and broke the
// joined sync path: normalizeReleaseEntry stamped n.Platform = "linux/amd64"
// into the prefix that findExistingArtifactByBuildID and
// findExistingArtifactByDigest then searched for, but the on-disk keys used
// "linux_amd64", so the prefix never matched and every sync re-downloaded.
// Picking the underscore form keeps NormalizePlatform compatible with the
// rest of the system instead of fighting it.
//
// Examples:
//   linux/amd64 -> linux_amd64
//   linux-amd64 -> linux_amd64
//   linux_amd64 -> linux_amd64
func NormalizePlatform(platform string) string {
	p := strings.TrimSpace(strings.ToLower(platform))
	p = strings.ReplaceAll(p, "\\", "_")
	p = strings.ReplaceAll(p, "/", "_")
	p = strings.ReplaceAll(p, "-", "_")
	for strings.Contains(p, "__") {
		p = strings.ReplaceAll(p, "__", "_")
	}
	return strings.Trim(p, "_")
}

// NormalizeChecksum canonicalizes checksum representation.
// If the input is raw 64-hex, it is prefixed with "sha256:".
func NormalizeChecksum(checksum string) string {
	c := strings.TrimSpace(strings.ToLower(checksum))
	if c == "" {
		return ""
	}
	if strings.HasPrefix(c, "sha256:") {
		return c
	}
	if len(c) == 64 {
		isHex := true
		for _, r := range c {
			if !(r >= '0' && r <= '9' || r >= 'a' && r <= 'f') {
				isHex = false
				break
			}
		}
		if isHex {
			return "sha256:" + c
		}
	}
	return c
}

// ValidateBuildID enforces a minimal immutable-id format guard.
func ValidateBuildID(buildID string) error {
	b := strings.TrimSpace(buildID)
	if b == "" {
		return fmt.Errorf("build_id is empty")
	}
	if isNumericOnly(b) {
		return fmt.Errorf("numeric-only build_id is not allowed")
	}
	if !buildIDPattern.MatchString(b) {
		return fmt.Errorf("build_id %q has invalid format", b)
	}
	return nil
}

// CanonicalArtifactKey returns the canonical artifact identity key.
// Format: {publisher}%{name}%{version}%{platform}%{build_id}
func CanonicalArtifactKey(ref *repopb.ArtifactRef, buildID string) string {
	if ref == nil {
		return ""
	}
	bid := strings.TrimSpace(buildID)
	if bid == "" {
		return ""
	}
	return strings.TrimSpace(ref.GetPublisherId()) + "%" +
		strings.TrimSpace(ref.GetName()) + "%" +
		strings.TrimSpace(ref.GetVersion()) + "%" +
		strings.TrimSpace(ref.GetPlatform()) + "%" +
		bid
}

// CanonicalArtifactStorageKeyByBuildNumber returns the transitional storage key
// used by legacy manifest/blob paths.
// Format: {publisher}%{name}%{version}%{platform}%{build_number}
func CanonicalArtifactStorageKeyByBuildNumber(ref *repopb.ArtifactRef, buildNumber int64) string {
	if ref == nil {
		return ""
	}
	return strings.TrimSpace(ref.GetPublisherId()) + "%" +
		strings.TrimSpace(ref.GetName()) + "%" +
		strings.TrimSpace(ref.GetVersion()) + "%" +
		strings.TrimSpace(ref.GetPlatform()) + "%" +
		strconv.FormatInt(buildNumber, 10)
}

// LegacyArtifactIdentityKey returns the legacy pre-build-number key.
// Format: {publisher}%{name}%{version}%{platform}
func LegacyArtifactIdentityKey(ref *repopb.ArtifactRef) string {
	if ref == nil {
		return ""
	}
	return strings.TrimSpace(ref.GetPublisherId()) + "%" +
		strings.TrimSpace(ref.GetName()) + "%" +
		strings.TrimSpace(ref.GetVersion()) + "%" +
		strings.TrimSpace(ref.GetPlatform())
}

// LegacyBuildAliasKey returns a release/build locator key used by alias mapping.
// Format: {publisher}%{name}%{version}%{platform}%{release_tag}%{build_number}
func LegacyBuildAliasKey(ref *repopb.ArtifactRef, buildNumber int64, releaseTag string) string {
	if ref == nil {
		return ""
	}
	rt := strings.TrimSpace(releaseTag)
	if rt == "" || buildNumber <= 0 {
		return ""
	}
	return LegacyArtifactIdentityKey(ref) + "%" + rt + "%" + strconv.FormatInt(buildNumber, 10)
}
