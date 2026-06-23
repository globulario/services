package main

import (
	"os"
	"path/filepath"
	"testing"
)

// Explicit config password wins and is returned verbatim.
func TestResolveResticPassword_ExplicitConfigWins(t *testing.T) {
	srv := &server{ResticPassword: "operator-set", ResticPasswordFile: filepath.Join(t.TempDir(), "pw")}
	if got := srv.resolveResticPassword(); got != "operator-set" {
		t.Fatalf("explicit config password should win, got %q", got)
	}
}

// An existing password file is read (no regeneration).
func TestResolveResticPassword_ReadsExistingFile(t *testing.T) {
	dir := t.TempDir()
	pwFile := filepath.Join(dir, "pw")
	if err := os.WriteFile(pwFile, []byte("from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	srv := &server{ResticPasswordFile: pwFile, ResticRepo: filepath.Join(dir, "restic")}
	if got := srv.resolveResticPassword(); got != "from-file" {
		t.Fatalf("should read password from file, got %q", got)
	}
}

// No file and no repo → generate, persist 0600, return non-empty & stable.
func TestResolveResticPassword_GeneratesAndPersists(t *testing.T) {
	dir := t.TempDir()
	pwFile := filepath.Join(dir, "sub", "pw") // dir must be created
	srv := &server{ResticPasswordFile: pwFile, ResticRepo: filepath.Join(dir, "restic")}

	pw := srv.resolveResticPassword()
	if pw == "" {
		t.Fatal("expected a generated password, got empty")
	}
	if pw == "globular-backup" {
		t.Fatal("must not fall back to the old hardcoded default")
	}
	info, err := os.Stat(pwFile)
	if err != nil {
		t.Fatalf("password file not persisted: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("password file mode = %o, want 0600", perm)
	}
	// A second resolve (fresh srv, password already on disk) returns the same value.
	srv2 := &server{ResticPasswordFile: pwFile, ResticRepo: filepath.Join(dir, "restic")}
	if got := srv2.resolveResticPassword(); got != pw {
		t.Errorf("persisted password not stable: got %q want %q", got, pw)
	}
}

// Existing repo but no known password → refuse (return ""), never invent a
// mismatching one that would silently break the repo.
func TestResolveResticPassword_RefusesForExistingRepoWithoutPassword(t *testing.T) {
	dir := t.TempDir()
	repo := filepath.Join(dir, "restic")
	if err := os.MkdirAll(repo, 0o700); err != nil {
		t.Fatal(err)
	}
	// restic local repos carry a top-level "config" file.
	if err := os.WriteFile(filepath.Join(repo, "config"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	srv := &server{ResticRepo: repo, ResticPasswordFile: filepath.Join(dir, "pw")}
	if got := srv.resolveResticPassword(); got != "" {
		t.Fatalf("must refuse (empty) for an existing repo with no password, got %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "pw")); err == nil {
		t.Error("must not have written a password file when refusing")
	}
}

// repoLooksInitialized: true only for a local dir holding a restic "config".
func TestRepoLooksInitialized(t *testing.T) {
	if repoLooksInitialized("s3:bucket/prefix") {
		t.Error("remote (s3:) repo must not be treated as locally initialized")
	}
	if repoLooksInitialized("") {
		t.Error("empty repo path is not initialized")
	}
	dir := t.TempDir()
	if repoLooksInitialized(dir) {
		t.Error("empty dir (no config file) is not an initialized repo")
	}
	if err := os.WriteFile(filepath.Join(dir, "config"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}
	if !repoLooksInitialized(dir) {
		t.Error("dir with a restic config file should look initialized")
	}
}
