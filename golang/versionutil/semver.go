package versionutil

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/coreos/go-semver/semver"
)

var exactTagPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._+:-]*$`)

// Canonical normalizes a version string to canonical SemVer form
// (MAJOR.MINOR.PATCH[-prerelease][+build], no leading "v").
// It trims whitespace, strips a leading "v"/"V" prefix, and parses
// strict semver. Returns an error if the input is not valid semver.
func Canonical(raw string) (string, error) {
	v := strings.TrimSpace(raw)
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	if v == "" {
		return "", fmt.Errorf("empty version string")
	}
	parsed, err := semver.NewVersion(v)
	if err != nil {
		return "", fmt.Errorf("invalid semver %q: %w", raw, err)
	}
	return parsed.String(), nil
}

// IsSemver reports whether raw can be parsed as semantic version after the
// same leading-v normalization used by Canonical.
func IsSemver(raw string) bool {
	_, err := Canonical(raw)
	return err == nil
}

// NormalizeExact preserves upstream-native version tags while still
// canonicalizing real semantic versions. This is for exact artifact identities:
// package versions such as RELEASE.2025-09-07T16-13-09Z and
// n8.1-10-g7f5c90f77e-20260422 are valid package tags even though they are not
// SemVer and cannot participate in bump semantics.
func NormalizeExact(raw string) (string, error) {
	v := strings.TrimSpace(raw)
	if v == "" {
		return "", fmt.Errorf("empty version string")
	}
	if cv, err := Canonical(v); err == nil {
		return cv, nil
	}
	if !exactTagPattern.MatchString(v) {
		return "", fmt.Errorf("invalid exact version tag %q: allowed characters are letters, digits, dot, underscore, plus, colon, and hyphen", raw)
	}
	return v, nil
}

// MustCanonical is like Canonical but panics on invalid input.
// Use only in post-validation contexts where the version is known-good.
func MustCanonical(raw string) string {
	c, err := Canonical(raw)
	if err != nil {
		panic(err)
	}
	return c
}

// Equal returns true if a and b represent the same semantic version
// after canonicalization. Returns false if either is invalid.
func Equal(a, b string) bool {
	ca, errA := Canonical(a)
	cb, errB := Canonical(b)
	if errA != nil || errB != nil {
		return false
	}
	return ca == cb
}

// Compare performs full semver comparison of a and b, returning
// -1, 0, or 1. Both inputs are canonicalized before comparison.
func Compare(a, b string) (int, error) {
	va := strings.TrimSpace(a)
	va = strings.TrimPrefix(va, "v")
	va = strings.TrimPrefix(va, "V")
	vb := strings.TrimSpace(b)
	vb = strings.TrimPrefix(vb, "v")
	vb = strings.TrimPrefix(vb, "V")

	pa, err := semver.NewVersion(va)
	if err != nil {
		return 0, fmt.Errorf("invalid semver %q: %w", a, err)
	}
	pb, err := semver.NewVersion(vb)
	if err != nil {
		return 0, fmt.Errorf("invalid semver %q: %w", b, err)
	}
	return pa.Compare(*pb), nil
}

// CompareFull compares two (version, build_number) tuples.
// Semantic version is compared first; build number breaks ties within the same version.
// Returns -1, 0, or 1.
func CompareFull(verA string, buildA int64, verB string, buildB int64) (int, error) {
	cmp, err := Compare(verA, verB)
	if err != nil {
		return 0, err
	}
	if cmp != 0 {
		return cmp, nil
	}
	// Same semver — compare build numbers.
	switch {
	case buildA < buildB:
		return -1, nil
	case buildA > buildB:
		return 1, nil
	default:
		return 0, nil
	}
}

// EqualFull returns true if both semantic version and build number match.
func EqualFull(verA string, buildA int64, verB string, buildB int64) bool {
	return Equal(verA, verB) && buildA == buildB
}

// PickLatestSemver returns the highest semantic version from the input slice
// in canonical form (no leading "v").
//
// Rules:
//   - Leading "v" is allowed (v1.2.3) and is stripped before parsing.
//   - Versions must conform to semver (MAJOR.MINOR.PATCH with optional pre-release/build).
//   - Invalid entries are ignored; if no valid versions remain, an error is returned.
//   - Standard semver ordering is used: e.g., 1.10.0 > 1.9.0 and 1.0.0 > 1.0.0-alpha.
//
// This function does not mutate the input slice.
func PickLatestSemver(versions []string) (string, error) {
	var best *semver.Version

	for _, raw := range versions {
		v := strings.TrimSpace(raw)
		v = strings.TrimPrefix(v, "v")
		if v == "" {
			continue
		}
		parsed, err := semver.NewVersion(v)
		if err != nil {
			continue // ignore invalid entries
		}
		if best == nil || parsed.Compare(*best) > 0 {
			copy := *parsed
			best = &copy
		}
	}

	if best == nil {
		return "", fmt.Errorf("no valid semantic versions found")
	}
	return best.String(), nil
}
