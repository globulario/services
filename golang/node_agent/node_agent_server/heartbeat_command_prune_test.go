package main

import (
	"os"
	"path/filepath"
	"testing"
)

// The heartbeat COMMAND prune deletes a stale installed-state record only when
// the globular-managed binary is truly gone. "Truly gone" MUST be judged by the
// globular install roots only — never $PATH. The bug this guards: ffmpeg/yt-dlp
// were uninstalled from /usr/lib/globular/bin but a system apt binary
// (/usr/bin/ffmpeg) still sits in $PATH; if the disk-truth check consulted
// $PATH, the stale COMMAND record would never be pruned and cluster-doctor would
// report placement.installed_package_orphaned forever.

func TestGlobularCommandBinaryExists_GlobularRootOnly(t *testing.T) {
	binDir := t.TempDir()
	prev := globularBinDir
	globularBinDir = binDir
	t.Cleanup(func() { globularBinDir = prev })

	// A globular-managed command present under the globular root → installed.
	if err := os.WriteFile(filepath.Join(binDir, "mytool"), []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	if !globularCommandBinaryExists("mytool") {
		t.Error("command present under the globular root must count as installed")
	}

	// The "-cmd" artifact suffix is stripped before probing the root.
	if !globularCommandBinaryExists("mytool-cmd") {
		t.Error("'-cmd' suffix must be stripped before the globular-root probe")
	}
}

func TestGlobularCommandBinaryExists_IgnoresSystemPath(t *testing.T) {
	binDir := t.TempDir()
	prev := globularBinDir
	globularBinDir = binDir
	t.Cleanup(func() { globularBinDir = prev })

	// A binary that exists ONLY on $PATH (a system/apt install), NOT under any
	// globular root — mirrors apt's /usr/bin/ffmpeg after the globular command
	// was uninstalled. It must NOT be treated as the globular-managed command,
	// otherwise the stale installed-state record is never pruned.
	pathDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(pathDir, "ffmpeg"), []byte("x"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", pathDir)

	if globularCommandBinaryExists("ffmpeg") {
		t.Error("a system binary on $PATH must NOT count as a globular-managed command — the prune would never fire (placement.installed_package_orphaned would persist)")
	}
}
