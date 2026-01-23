package versionutil

import (
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
func MarkerPath(serviceName string) string {
	name := sanitize(serviceName)
	if name == "" {
		name = "unknown"
	}
	return filepath.Join(baseDir, name, "version")
}

func sanitize(name string) string {
	n := strings.ToLower(strings.TrimSpace(name))
	if n == "" {
		return ""
	}
	re := regexp.MustCompile(`[^a-z0-9._-]+`)
	n = re.ReplaceAllString(n, "-")
	return n
}
