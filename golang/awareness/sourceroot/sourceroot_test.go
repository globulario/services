package sourceroot

import (
	"os"
	"path/filepath"
	"testing"
)

// TestSourceRoot_NotInGitRepoReturnsAbsent pins the rule that when the
// process is not running inside a git checkout AND no explicit path is
// given, Resolve must return Absent — NOT a cwd-as-root fallback that
// would silently scan an install dir on a production MCP host.
func TestSourceRoot_NotInGitRepoReturnsAbsent(t *testing.T) {
	tmp := t.TempDir()
	prev, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(prev) })
	if err := os.Chdir(tmp); err != nil {
		t.Fatalf("chdir to tmp: %v", err)
	}

	got := Resolve(Options{AllowGitDiscovery: true})
	if got.State != Absent {
		t.Errorf("non-git cwd must yield State=Absent, got %s (path=%q)", got.State, got.Path)
	}
	if got.Path != "" {
		t.Errorf("Absent result must have empty Path, got %q", got.Path)
	}
	if got.IsAvailable() {
		t.Error("IsAvailable() must be false when State=Absent")
	}
}

// TestSourceRoot_ExplicitPathInvalidReturnsInaccessible pins that an
// explicit ExplicitPath that doesn't exist returns Inaccessible — NOT
// a silent fallback to git discovery. The caller's intent ("scan THIS
// path") must not be overridden by ambient state.
func TestSourceRoot_ExplicitPathInvalidReturnsInaccessible(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "definitely-does-not-exist")
	got := Resolve(Options{ExplicitPath: missing})
	if got.State != Inaccessible {
		t.Errorf("missing ExplicitPath must yield State=Inaccessible, got %s", got.State)
	}
	if got.Err == nil {
		t.Error("Inaccessible result must carry a non-nil Err")
	}
	if got.IsAvailable() {
		t.Error("IsAvailable() must be false when State=Inaccessible")
	}
}

// TestSourceRoot_ExplicitPathToFileReturnsWrongContext pins that an
// ExplicitPath that resolves to a regular file (not a directory) is
// classified as WrongContext, distinct from Inaccessible. This is the
// "scanner misconfigured" branch the invariant requires consumers to
// distinguish from "source unavailable."
func TestSourceRoot_ExplicitPathToFileReturnsWrongContext(t *testing.T) {
	f := filepath.Join(t.TempDir(), "regular_file")
	if err := os.WriteFile(f, []byte("hi"), 0o644); err != nil {
		t.Fatalf("create regular file: %v", err)
	}
	got := Resolve(Options{ExplicitPath: f})
	if got.State != WrongContext {
		t.Errorf("file-as-path must yield State=WrongContext, got %s", got.State)
	}
	if got.Reason == "" {
		t.Error("WrongContext must carry a non-empty Reason")
	}
}

// TestSourceRoot_FoundInThisRepo is a positive control: running the
// tests inside the actual repo, Resolve must report Found with a real
// directory.
func TestSourceRoot_FoundInThisRepo(t *testing.T) {
	got := Resolve(DefaultOptions)
	if got.State != Found {
		t.Skipf("test runner is not inside a git checkout (got %s) — positive control N/A here", got.State)
	}
	if !got.IsAvailable() {
		t.Error("IsAvailable() must be true when State=Found")
	}
	if got.Path == "" {
		t.Error("Found result must populate Path")
	}
	info, err := os.Stat(got.Path)
	if err != nil || !info.IsDir() {
		t.Errorf("Found path %q must be a readable directory", got.Path)
	}
}

// TestSourceRoot_NoGitDiscoveryDisabled pins that when AllowGitDiscovery
// is false and no ExplicitPath is given, Resolve returns Absent even
// inside a real git checkout. Strictly-explicit resolution must not be
// overridden by ambient git context.
func TestSourceRoot_NoGitDiscoveryDisabled(t *testing.T) {
	got := Resolve(Options{AllowGitDiscovery: false})
	if got.State != Absent {
		t.Errorf("strictly-explicit resolution with no ExplicitPath must yield Absent, got %s", got.State)
	}
}
