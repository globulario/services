package main

// Tests for the user-graph fallback in resolveAwarenessDBPathFor. The system
// install path can be root-owned on a dev machine, which forced operators
// to remember `--db /home/<user>/.globular/awareness/graph.json`. The
// fallback resolves that without touching cluster behaviour.

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestResolveAwarenessDBPath_FallbackWhenSystemMissing pins the case where
// the system install dir doesn't exist at all (a clean dev box). The user
// fallback must be selected and the warning must fire.
func TestResolveAwarenessDBPath_FallbackWhenSystemMissing(t *testing.T) {
	// systemDir intentionally points at a nonexistent path so the resolver
	// can't probe the parent.
	tmp := t.TempDir()
	systemDir := filepath.Join(tmp, "nonexistent-system-dir")
	homeDir := filepath.Join(tmp, "home")
	if err := os.MkdirAll(homeDir, 0o755); err != nil {
		t.Fatal(err)
	}

	var warned string
	got := resolveAwarenessDBPathFor(systemDir, homeDir, "/repo", func(msg string) {
		warned = msg
	})

	want := filepath.Join(homeDir, ".globular", "awareness", "graph.json")
	if got != want {
		t.Errorf("resolved = %q, want %q", got, want)
	}
	if warned == "" {
		t.Fatal("expected fallback warning, got none")
	}
	if !strings.Contains(warned, "user graph") {
		t.Errorf("warning %q does not mention 'user graph'", warned)
	}
	if !strings.Contains(warned, systemDir) {
		t.Errorf("warning %q should name the system path that failed", warned)
	}
	// The user graph directory must have been created so a subsequent
	// graph.Open can write to it.
	if _, err := os.Stat(filepath.Dir(want)); err != nil {
		t.Errorf("user graph dir was not created: %v", err)
	}
}

// TestResolveAwarenessDBPath_SystemAccessibleNoFallback: when the system
// path is fully accessible, no warning is printed and the system path
// wins. This pins that we don't accidentally route operators away from
// the canonical install when they had read+write access all along.
func TestResolveAwarenessDBPath_SystemAccessibleNoFallback(t *testing.T) {
	systemDir := t.TempDir() // exists, current user can read+write
	homeDir := t.TempDir()

	var warned string
	got := resolveAwarenessDBPathFor(systemDir, homeDir, "/repo", func(msg string) {
		warned = msg
	})

	want := filepath.Join(systemDir, "graph.json")
	if got != want {
		t.Errorf("resolved = %q, want %q (system accessible)", got, want)
	}
	if warned != "" {
		t.Errorf("unexpected warning when system graph is accessible: %q", warned)
	}
}

// TestResolveAwarenessDBPath_RepoFallbackWhenNoHome: when neither the
// system path nor a home directory is available, fall back to the
// repo-local .globular/awareness/graph.json. This preserves the original
// dev-without-install behaviour.
func TestResolveAwarenessDBPath_RepoFallbackWhenNoHome(t *testing.T) {
	tmp := t.TempDir()
	systemDir := filepath.Join(tmp, "no-system")

	got := resolveAwarenessDBPathFor(systemDir, "" /* no home */, "/repo", nil /* no warner */)

	want := filepath.Join("/repo", ".globular", "awareness", "graph.json")
	if got != want {
		t.Errorf("resolved = %q, want %q (repo fallback)", got, want)
	}
}

// TestIsUsableAwarenessDB_ReadOnlyFile pins the load-bearing semantic for
// real-world dev boxes: an existing graph.json that the current user can
// read but NOT write must report as not-usable so we fall back. We skip
// when running as root because root bypasses POSIX permission bits.
func TestIsUsableAwarenessDB_ReadOnlyFile(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("requires non-root: root bypasses file permission bits")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "graph.json")
	if err := os.WriteFile(path, []byte("{}"), 0o444); err != nil {
		t.Fatal(err)
	}
	if isUsableAwarenessDB(path) {
		t.Errorf("isUsableAwarenessDB returned true for read-only file; expected false")
	}
}

// TestIsUsableAwarenessDB_WritableFile is the positive case.
func TestIsUsableAwarenessDB_WritableFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "graph.json")
	if err := os.WriteFile(path, []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	if !isUsableAwarenessDB(path) {
		t.Errorf("isUsableAwarenessDB returned false for writable file; expected true")
	}
}
