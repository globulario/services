package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/globulario/services/golang/runtimedirs"
)

func mustMkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func mustWrite(t *testing.T, path, content string) {
	t.Helper()
	mustMkdir(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func dirExists(t *testing.T, path string) bool {
	t.Helper()
	fi, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
		t.Fatalf("stat %s: %v", path, err)
	}
	return fi.IsDir()
}

// Case 1: an empty legacy alias dir is removed; the canonical dir is untouched.
func TestCanonicalizeRuntimeDirs_EmptyLegacyRemoved(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, "cluster_doctor")
	canon := filepath.Join(root, "cluster-doctor")
	mustMkdir(t, legacy)
	mustMkdir(t, canon)

	CanonicalizeRuntimeDirsOnce(root)

	if dirExists(t, legacy) {
		t.Errorf("legacy dir %s should have been removed", legacy)
	}
	if !dirExists(t, canon) {
		t.Errorf("canonical dir %s should still exist", canon)
	}
}

// Case 2: a legacy dir with contents migrates into the canonical dir (created
// if missing) and the now-empty legacy dir is removed.
func TestCanonicalizeRuntimeDirs_ContentsMigratedLegacyRemoved(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, "ai_memory")
	canon := filepath.Join(root, "ai-memory")
	mustWrite(t, filepath.Join(legacy, "state.json"), `{"node":"x"}`)
	// canonical intentionally absent — the sweep must create it.

	CanonicalizeRuntimeDirsOnce(root)

	if dirExists(t, legacy) {
		t.Errorf("legacy dir %s should have been removed after migration", legacy)
	}
	got, err := os.ReadFile(filepath.Join(canon, "state.json"))
	if err != nil {
		t.Fatalf("expected migrated file in canonical dir: %v", err)
	}
	if string(got) != `{"node":"x"}` {
		t.Errorf("migrated content mismatch: got %q", string(got))
	}
}

// Case 3: when only the canonical dir exists, the sweep is a no-op and never
// fabricates a legacy dir.
func TestCanonicalizeRuntimeDirs_NoopWhenOnlyCanonical(t *testing.T) {
	root := t.TempDir()
	canon := filepath.Join(root, "node-agent")
	mustWrite(t, filepath.Join(canon, "state.json"), `{"keep":true}`)

	CanonicalizeRuntimeDirsOnce(root)

	if !dirExists(t, canon) {
		t.Errorf("canonical dir %s should still exist", canon)
	}
	if got, err := os.ReadFile(filepath.Join(canon, "state.json")); err != nil || string(got) != `{"keep":true}` {
		t.Errorf("canonical content must be untouched: got %q err %v", string(got), err)
	}
	for _, legacy := range runtimedirs.LegacyRuntimeAliases("node-agent") {
		if dirExists(t, filepath.Join(root, legacy)) {
			t.Errorf("sweep must not create legacy dir %s", legacy)
		}
	}
}

// Case 4: a conflicting entry is never overwritten, and the legacy dir is left
// intact for operator review.
func TestCanonicalizeRuntimeDirs_ConflictPreservesBoth(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, "ai_memory")
	canon := filepath.Join(root, "ai-memory")
	mustWrite(t, filepath.Join(legacy, "state.json"), "LEGACY")
	mustWrite(t, filepath.Join(canon, "state.json"), "CANONICAL")

	CanonicalizeRuntimeDirsOnce(root)

	// Canonical file must NOT be overwritten.
	got, err := os.ReadFile(filepath.Join(canon, "state.json"))
	if err != nil || string(got) != "CANONICAL" {
		t.Errorf("canonical file must not be overwritten: got %q err %v", string(got), err)
	}
	// Legacy dir + its conflicting file must be left intact.
	if !dirExists(t, legacy) {
		t.Errorf("legacy dir %s must be preserved on conflict", legacy)
	}
	if lg, err := os.ReadFile(filepath.Join(legacy, "state.json")); err != nil || string(lg) != "LEGACY" {
		t.Errorf("legacy file must be preserved on conflict: got %q err %v", string(lg), err)
	}
}

// Case 5: the sweep is driven by the SHARED runtimedirs alias map, not a local
// duplicate list. Seed an empty legacy dir for every known alias and assert all
// are canonicalized away.
func TestCanonicalizeRuntimeDirs_UsesSharedAliasMap(t *testing.T) {
	root := t.TempDir()
	pairs := runtimedirs.CanonicalToLegacy()
	if len(pairs) == 0 {
		t.Fatal("shared alias map is empty — extraction broken")
	}
	for canonical, legacies := range pairs {
		mustMkdir(t, filepath.Join(root, canonical))
		for _, legacy := range legacies {
			mustMkdir(t, filepath.Join(root, legacy))
		}
	}

	CanonicalizeRuntimeDirsOnce(root)

	for canonical, legacies := range pairs {
		for _, legacy := range legacies {
			if dirExists(t, filepath.Join(root, legacy)) {
				t.Errorf("legacy alias %s (canonical %s) should have been removed via the shared map", legacy, canonical)
			}
		}
		if !dirExists(t, filepath.Join(root, canonical)) {
			t.Errorf("canonical dir %s should remain", canonical)
		}
	}
}

// Idempotency: running twice is safe and leaves the same end state.
func TestCanonicalizeRuntimeDirs_Idempotent(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, "ai_executor")
	mustWrite(t, filepath.Join(legacy, "f.txt"), "data")

	CanonicalizeRuntimeDirsOnce(root)
	CanonicalizeRuntimeDirsOnce(root) // second run must not error or change state

	if dirExists(t, legacy) {
		t.Errorf("legacy dir %s should remain removed after second sweep", legacy)
	}
	if got, err := os.ReadFile(filepath.Join(root, "ai-executor", "f.txt")); err != nil || string(got) != "data" {
		t.Errorf("canonical content must survive idempotent re-run: got %q err %v", string(got), err)
	}
}
