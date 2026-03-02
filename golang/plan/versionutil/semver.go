package versionutil

import (
	"fmt"
	"strings"

	"github.com/coreos/go-semver/semver"
)

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
