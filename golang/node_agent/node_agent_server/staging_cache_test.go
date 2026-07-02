package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWriteStagedArtifactCache_WritesContentAddressedFileAndLatestAlias(t *testing.T) {
	tmp := t.TempDir()
	oldRoot := stagingRootDir
	stagingRootDir = tmp
	t.Cleanup(func() { stagingRootDir = oldRoot })

	payload := []byte("artifact-bytes-build-a")
	contentPath, err := writeStagedArtifactCache("core@globular.io", "xds", payload)
	if err != nil {
		t.Fatalf("writeStagedArtifactCache: %v", err)
	}
	if _, err := os.Stat(contentPath); err != nil {
		t.Fatalf("content-addressed artifact missing: %v", err)
	}
	if filepath.Base(contentPath) == "latest.artifact" {
		t.Fatalf("content-addressed path must not be latest alias: %q", contentPath)
	}
	latest := latestStagedArtifactPath("core@globular.io", "xds")
	if _, err := os.Lstat(latest); err != nil {
		t.Fatalf("latest alias missing: %v", err)
	}
	got, err := os.ReadFile(latest)
	if err != nil {
		t.Fatalf("read latest alias: %v", err)
	}
	if string(got) != string(payload) {
		t.Fatalf("latest alias bytes differ: got %q want %q", string(got), string(payload))
	}
}
