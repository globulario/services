package main

// awareness_bundle_cmd_test.go — P1-5 acceptance tests.
//
// These tests pin the contract that the awareness bundle build copies every
// docs/awareness/*.yaml knowledge file (not just an allow-listed subset)
// into the output bundle. They guard against a refactor that adds a glob
// filter, accidentally excluding a load-bearing file like
// detector_mapping.yaml — without which consumers cannot rebuild
// detector → failure_mode edges and coverage silently degrades.

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// criticalAwarenessFiles is the set of knowledge files whose absence from
// the bundle would silently break a documented graph behavior. Treat this
// list as load-bearing — adding to it requires understanding why the file
// is required, not just "it exists in docs/awareness/".
var criticalAwarenessFiles = []string{
	"detector_mapping.yaml",           // P1-5: detector → failure_mode edges
	"failure_modes.yaml",              // every coverage check joins on this
	"invariants.yaml",                 // forbidden_fix / required_test edges
	"context_aliases.yaml",            // preflight alias matching
	"design_patterns.yaml",            // mitigates edges for coverage
	"awareness_self_invariants.yaml",  // assurance bootstraps from this
	"fix_cases.yaml",                  // fix-ledger lookups
}

// TestCollectDocsAwarenessEntries_IncludesCriticalFiles verifies the helper
// the bundle build uses returns every load-bearing knowledge file. New
// critical files added to criticalAwarenessFiles must show up here; a
// glob/filter refactor that excludes any of them breaks this test.
func TestCollectDocsAwarenessEntries_IncludesCriticalFiles(t *testing.T) {
	dir := t.TempDir()
	for _, name := range criticalAwarenessFiles {
		writeFile(t, filepath.Join(dir, name), "stub: true\n")
	}
	// Also seed a nested file to confirm recursive walk.
	nestedDir := filepath.Join(dir, "failuregraph_seeds")
	if err := os.MkdirAll(nestedDir, 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}
	writeFile(t, filepath.Join(nestedDir, "etcd.yaml"), "stub: true\n")

	entries, err := collectDocsAwarenessEntries(dir)
	if err != nil {
		t.Fatalf("collectDocsAwarenessEntries: %v", err)
	}
	arcPaths := arcPathSet(entries)
	for _, name := range criticalAwarenessFiles {
		want := "docs/" + name
		if !arcPaths[want] {
			t.Errorf("bundle entries missing %q\n  got: %v", want, sortedArcPaths(arcPaths))
		}
	}
	if !arcPaths["docs/failuregraph_seeds/etcd.yaml"] {
		t.Errorf("nested file not collected; got: %v", sortedArcPaths(arcPaths))
	}
}

// TestCollectDocsAwarenessEntries_RealRepoIncludesDetectorMapping is the
// integration leg of the P1-5 contract: when run against the real repo's
// docs/awareness/ directory, the helper must return detector_mapping.yaml.
// A repo-level deletion or rename of the file fails this test loudly
// before a release ships a bundle missing it.
func TestCollectDocsAwarenessEntries_RealRepoIncludesDetectorMapping(t *testing.T) {
	docsDir := findRepoDocsAwareness(t)
	entries, err := collectDocsAwarenessEntries(docsDir)
	if err != nil {
		t.Fatalf("collectDocsAwarenessEntries(%s): %v", docsDir, err)
	}
	arcPaths := arcPathSet(entries)
	if !arcPaths["docs/detector_mapping.yaml"] {
		t.Fatalf("real repo's docs/awareness/ is missing detector_mapping.yaml\n"+
			"  arcPaths: %v", sortedArcPaths(arcPaths))
	}
}

// TestCollectDocsAwarenessEntries_MissingDirReturnsEmpty mirrors the
// RunE caller's "warn and continue" semantics — a missing docs/awareness/
// directory is not a fatal error, the helper just returns no entries.
func TestCollectDocsAwarenessEntries_MissingDirReturnsEmpty(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "does-not-exist")
	entries, err := collectDocsAwarenessEntries(dir)
	if err != nil {
		t.Errorf("missing dir should not error, got: %v", err)
	}
	if len(entries) != 0 {
		t.Errorf("missing dir should return empty, got: %v", entries)
	}
	if entries == nil {
		t.Error("missing dir should return empty (non-nil) slice for consistent caller handling")
	}
}

// TestCollectDocsAwarenessEntries_ArcPathsUseForwardSlash protects against
// path-separator drift on Windows builds; the tar inside the bundle must
// use forward slashes regardless of the host OS, because consumers untar
// it on Linux nodes.
func TestCollectDocsAwarenessEntries_ArcPathsUseForwardSlash(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "contracts")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	writeFile(t, filepath.Join(nested, "a.yaml"), "x: 1\n")

	entries, err := collectDocsAwarenessEntries(dir)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	for _, e := range entries {
		if strings.Contains(e.arcPath, `\`) {
			t.Errorf("arcPath contains backslash: %q", e.arcPath)
		}
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func arcPathSet(entries []bundleFileEntry) map[string]bool {
	out := make(map[string]bool, len(entries))
	for _, e := range entries {
		out[e.arcPath] = true
	}
	return out
}

func sortedArcPaths(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// findRepoDocsAwareness walks up from the test file's directory until it
// finds docs/awareness/. Skips the test if it can't (which makes the
// integration leg non-fatal in unusual build environments).
func findRepoDocsAwareness(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Skipf("getwd: %v", err)
	}
	for {
		candidate := filepath.Join(dir, "docs", "awareness")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Skip("docs/awareness not found above test working directory")
		}
		dir = parent
	}
}
