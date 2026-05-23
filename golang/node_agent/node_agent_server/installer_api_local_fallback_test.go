package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindLocalPackageAnyVersion_SelectsLatestLexicographic(t *testing.T) {
	dir := t.TempDir()
	files := []string{
		"envoy_1.34.0_linux_amd64.tgz",
		"envoy_1.35.3_linux_amd64.tgz",
		"envoy_1.35.1_linux_amd64.tgz",
	}
	for _, f := range files {
		if err := os.WriteFile(filepath.Join(dir, f), []byte("x"), 0o644); err != nil {
			t.Fatalf("write %s: %v", f, err)
		}
	}

	got := findLocalPackageAnyVersion(dir, "envoy", "linux_amd64")
	want := filepath.Join(dir, "envoy_1.35.3_linux_amd64.tgz")
	if got != want {
		t.Fatalf("findLocalPackageAnyVersion = %q, want %q", got, want)
	}
}

func TestFindLocalPackageAnyVersion_NoMatch(t *testing.T) {
	dir := t.TempDir()
	if got := findLocalPackageAnyVersion(dir, "envoy", "linux_amd64"); got != "" {
		t.Fatalf("expected empty result, got %q", got)
	}
}

