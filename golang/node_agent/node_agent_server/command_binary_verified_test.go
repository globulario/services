package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/globulario/services/golang/repository/repositorypb"
)

// TestCommandBinaryVerified_RejectsUnrelatedPathMatch reproduces the
// orphaned-package resurrection bug: syncRepoArtifactsToEtcd used
// commandBinaryExists (existence-only, via commandBinaryPath's exec.LookPath
// $PATH fallback) to decide whether a COMMAND package was installed. On a
// node with an unrelated, like-named binary already on $PATH (e.g. an OS/apt
// package sharing the command's name — observed live with yt-dlp), that
// fallback returned true for a file that has nothing to do with the
// Globular-packaged artifact, so the loop minted a false installed_state
// record stamped with the repository manifest's own version/checksum.
//
// commandBinaryVerified must require the binary to be found under
// Globular's own managed directories (commandBinaryDirs) AND to hash-match
// the manifest's entrypoint checksum — never trust $PATH presence alone.
func TestCommandBinaryVerified_RejectsUnrelatedPathMatch(t *testing.T) {
	origDirs := commandBinaryDirs
	tmpManaged := t.TempDir()
	commandBinaryDirs = []string{tmpManaged}
	t.Cleanup(func() { commandBinaryDirs = origDirs })

	// Simulate an unrelated binary on $PATH but NOT in a Globular-managed
	// directory (this is what /usr/bin/yt-dlp — the Ubuntu apt package —
	// looked like on the live node: present, findable via PATH, wrong
	// content entirely).
	unrelatedDir := t.TempDir()
	unrelatedPath := filepath.Join(unrelatedDir, "yt-dlp")
	if err := os.WriteFile(unrelatedPath, []byte("#!/bin/sh\necho not-the-real-thing\n"), 0o755); err != nil {
		t.Fatalf("write unrelated binary: %v", err)
	}
	origPath := os.Getenv("PATH")
	t.Cleanup(func() { os.Setenv("PATH", origPath) })
	os.Setenv("PATH", unrelatedDir+string(os.PathListSeparator)+origPath)

	// Sanity: confirm the OLD detector (commandBinaryExists) would have
	// been fooled by this exact setup — pins the bug this test guards
	// against, so a future refactor that reintroduces the LookPath fallback
	// into the verified path gets caught.
	if !commandBinaryExists("yt-dlp") {
		t.Fatalf("setup invalid: commandBinaryExists should find the unrelated PATH binary")
	}

	manifest := &repositorypb.ArtifactManifest{
		EntrypointChecksum: "sha256:91d023f421dffd6fd354d233edb784a05d5dde2c1c585fcb599d5cc23ecb759e",
	}

	if commandBinaryVerified("yt-dlp", manifest) {
		t.Fatalf("commandBinaryVerified must reject an unrelated binary found only via $PATH, not under commandBinaryDirs")
	}
}

// TestCommandBinaryVerified_AcceptsChecksumMatchInManagedDir confirms the
// positive case still works: a binary genuinely placed under a
// Globular-managed directory, whose content hashes to the manifest's
// entrypoint checksum, is accepted.
func TestCommandBinaryVerified_AcceptsChecksumMatchInManagedDir(t *testing.T) {
	origDirs := commandBinaryDirs
	tmpManaged := t.TempDir()
	commandBinaryDirs = []string{tmpManaged}
	t.Cleanup(func() { commandBinaryDirs = origDirs })

	content := []byte("#!/bin/sh\necho the-real-thing\n")
	path := filepath.Join(tmpManaged, "yt-dlp")
	if err := os.WriteFile(path, content, 0o755); err != nil {
		t.Fatalf("write managed binary: %v", err)
	}

	// sha256sum of the exact content above.
	const wantChecksum = "sha256:ba1f57a4dc84f019b6f45738e9b7c05d406a58eb1181c51a76216edb3ad8306b"
	manifest := &repositorypb.ArtifactManifest{EntrypointChecksum: wantChecksum}

	if !commandBinaryVerified("yt-dlp", manifest) {
		t.Fatalf("commandBinaryVerified should accept a checksum-matching binary in a managed dir")
	}
}

// TestCommandBinaryVerified_NoManifestChecksumRejects pins
// meta.fallback_must_degrade_semantics: with no checksum to prove against,
// the function must return false (unverified), never fall back to bare
// existence.
func TestCommandBinaryVerified_NoManifestChecksumRejects(t *testing.T) {
	origDirs := commandBinaryDirs
	tmpManaged := t.TempDir()
	commandBinaryDirs = []string{tmpManaged}
	t.Cleanup(func() { commandBinaryDirs = origDirs })

	path := filepath.Join(tmpManaged, "yt-dlp")
	if err := os.WriteFile(path, []byte("anything"), 0o755); err != nil {
		t.Fatalf("write managed binary: %v", err)
	}

	if commandBinaryVerified("yt-dlp", &repositorypb.ArtifactManifest{}) {
		t.Fatalf("commandBinaryVerified must reject when the manifest carries no checksum to verify against")
	}
}
