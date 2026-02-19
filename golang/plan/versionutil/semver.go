package versionutil

import (
	"fmt"
	"strings"

	"github.com/coreos/go-semver/semver"
)

// PickLatestSemver returns the highest semantic version from the input slice.
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
	var bestOriginal string

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
			if strings.HasPrefix(strings.TrimSpace(raw), "v") {
				bestOriginal = "v" + parsed.String()
			} else {
				bestOriginal = parsed.String()
			}
		}
	}

	if best == nil {
		return "", fmt.Errorf("no valid semantic versions found")
	}
	return bestOriginal, nil
}
