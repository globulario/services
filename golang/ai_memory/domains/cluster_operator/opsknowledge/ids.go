package opsknowledge

import (
	"path/filepath"
	"regexp"
	"strings"
)

var nonIDChar = regexp.MustCompile(`[^a-z0-9]+`)

// slug normalizes an arbitrary string into a stable, human-readable id segment:
// lower-case, non-alphanumerics collapsed to single underscores, trimmed.
func slug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = nonIDChar.ReplaceAllString(s, "_")
	return strings.Trim(s, "_")
}

// fileSlug derives a slug from a corpus file path's base name (sans extension),
// e.g. ".../service-roles/cluster-controller.yaml" -> "cluster_controller".
func fileSlug(path string) string {
	base := filepath.Base(path)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	return slug(base)
}
