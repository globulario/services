package pkgpack

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/globulario/services/golang/versionutil"
)

var embeddedBuildTokenPattern = regexp.MustCompile(`(?i)(?:^|[.+-])b[0-9]+(?:$|[.+-])`)

// ValidateVersionBuildSemantics enforces package identity invariants:
// - version must be a valid exact version tag
// - build_number must be a plain non-negative integer
// - build_number must NOT be embedded in version (for example "1.2.3+b12")
func ValidateVersionBuildSemantics(version string, buildNumber int64) error {
	normalized, err := versionutil.NormalizeExact(version)
	if err != nil {
		return fmt.Errorf("invalid version %q: %w", version, err)
	}
	if buildNumber < 0 {
		return fmt.Errorf("invalid build_number %d: must be >= 0", buildNumber)
	}

	lower := strings.ToLower(normalized)
	if embeddedBuildTokenPattern.MatchString(lower) {
		return fmt.Errorf("version %q embeds a build token; use build_number as a plain integer field instead", version)
	}
	return nil
}
