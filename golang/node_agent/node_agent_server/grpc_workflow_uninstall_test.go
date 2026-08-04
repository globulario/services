package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCommandBinaryPathsIncludesEveryRecoveryLocation(t *testing.T) {
	original := commandBinaryDirs
	commandBinaryDirs = []string{t.TempDir(), t.TempDir()}
	t.Cleanup(func() { commandBinaryDirs = original })

	paths := commandBinaryPaths("yt-dlp")
	if len(paths) != 2 {
		t.Fatalf("commandBinaryPaths() returned %d paths, want 2", len(paths))
	}
	for _, path := range paths {
		if err := os.WriteFile(path, []byte("yt-dlp"), 0o755); err != nil {
			t.Fatalf("write command binary %s: %v", path, err)
		}
	}

	if err := removeCommandBinaries("yt-dlp"); err != nil {
		t.Fatalf("removeCommandBinaries: %v", err)
	}

	for _, dir := range commandBinaryDirs {
		if _, err := os.Stat(filepath.Join(dir, "yt-dlp")); !os.IsNotExist(err) {
			t.Errorf("command binary remains at %s: %v", dir, err)
		}
	}
}
