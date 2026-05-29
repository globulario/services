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

// KindPath returns the path to the kind sidecar file for a given service name.
// Written at install time alongside the version marker.
// Format: /var/lib/globular/services/<name>/kind
func KindPath(serviceName string) string {
	name := sanitize(serviceName)
	if name == "" {
		name = "unknown"
	}
	return filepath.Join(baseDir, name, "kind")
}

// EntrypointPath returns the path to the entrypoint sidecar file for a given
// service name. The file holds the binary filename declared by the package
// manifest's `entrypoint` field, so verifier/drift logic can resolve the
// installed binary path without inferring it from the package name.
//
// Project T (commit reserved): the pre-fix installer inferred the binary
// path with strings.ReplaceAll(name, "-", "_") (SERVICE) or raw name
// (INFRASTRUCTURE). Both fail for packages whose name uses hyphens but
// whose entrypoint uses underscores (e.g. scylla-manager → scylla_manager).
// The sidecar is the source of truth; the inferred logic is the fallback
// for legacy installs that pre-date the sidecar.
//
// Format: /var/lib/globular/services/<name>/entrypoint
func EntrypointPath(serviceName string) string {
	name := sanitize(serviceName)
	if name == "" {
		name = "unknown"
	}
	return filepath.Join(baseDir, name, "entrypoint")
}

// WriteEntrypoint persists the manifest-declared entrypoint binary filename
// for a package alongside the version marker. The stored value is JUST the
// binary filename (e.g. "scylla_manager"); a "bin/" or "./bin/" prefix is
// stripped because callers join the result with the canonical bin dir.
//
// Safe to call multiple times; last write wins. Returns nil and writes
// nothing when entrypoint is empty.
func WriteEntrypoint(serviceName, entrypoint string) error {
	entrypoint = strings.TrimSpace(entrypoint)
	if entrypoint == "" {
		return nil
	}
	entrypoint = strings.TrimPrefix(entrypoint, "./")
	entrypoint = strings.TrimPrefix(entrypoint, "bin/")
	if entrypoint == "" {
		return nil
	}
	path := EntrypointPath(serviceName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(entrypoint+"\n"), 0o644)
}

// ReadEntrypoint returns the persisted entrypoint binary filename for a
// package, or "" if no sidecar exists. Callers should treat "" as "fall
// back to the legacy inferred path."
func ReadEntrypoint(serviceName string) string {
	data, err := os.ReadFile(EntrypointPath(serviceName))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(data))
}

// WriteKind persists the package kind ("SERVICE", "INFRASTRUCTURE", "COMMAND",
// "APPLICATION") alongside the version marker so offline/Phase-1 reads know the
// kind without an etcd query. Safe to call multiple times; last write wins.
func WriteKind(serviceName, kind string) error {
	kind = strings.ToUpper(strings.TrimSpace(kind))
	if kind == "" {
		return nil
	}
	path := KindPath(serviceName)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(kind+"\n"), 0o644)
}

// ReadKind returns the persisted kind for a package, or "" if no sidecar exists.
func ReadKind(serviceName string) string {
	data, err := os.ReadFile(KindPath(serviceName))
	if err != nil {
		return ""
	}
	return strings.ToUpper(strings.TrimSpace(string(data)))
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
