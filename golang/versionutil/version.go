package versionutil

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var baseDir = "/var/lib/globular/services"

// SetBaseDir overrides the default base directory for version markers. Mainly used for tests.
func SetBaseDir(dir string) {
	if strings.TrimSpace(dir) == "" {
		return
	}
	baseDir = dir
}

// BaseDir returns the current base directory for marker files.
func BaseDir() string {
	return baseDir
}

// MarkerPath returns the path to the version marker file for a given service name.
// The canonical path uses hyphens (e.g. "cluster-controller"). If the canonical
// path does not exist on disk but a legacy underscore variant does (e.g.
// "cluster_controller"), the legacy path is returned instead for backward compat.
func MarkerPath(serviceName string) string {
	name := sanitize(serviceName)
	if name == "" {
		name = "unknown"
	}
	canonical := filepath.Join(baseDir, name, "version")
	if _, err := os.Stat(canonical); err == nil {
		return canonical
	}
	// Check legacy underscore variant.
	legacy := strings.ReplaceAll(name, "-", "_")
	if legacy != name {
		legacyPath := filepath.Join(baseDir, legacy, "version")
		if _, err := os.Stat(legacyPath); err == nil {
			return legacyPath
		}
	}
	// Neither exists; return the canonical path for new writes.
	return canonical
}

func sanitize(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	if n == "" {
		return ""
	}
	// Canonical form uses hyphens, not underscores.
	n = strings.ReplaceAll(n, "_", "-")
	re := regexp.MustCompile(`[^a-z0-9._-]+`)
	n = re.ReplaceAllString(n, "-")
	return n
}
