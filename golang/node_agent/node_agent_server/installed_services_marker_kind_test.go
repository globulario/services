package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/globulario/services/golang/versionutil"
)

func withVersionMarkerBaseDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	old := versionutil.BaseDir()
	versionutil.SetBaseDir(dir)
	t.Cleanup(func() { versionutil.SetBaseDir(old) })
	return dir
}

func writeVersionMarker(t *testing.T, root, name, version string) {
	t.Helper()
	dir := filepath.Join(root, name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "version"), []byte(version+"\n"), 0o644); err != nil {
		t.Fatalf("write marker for %s: %v", name, err)
	}
}

func TestLoadMarkersClassifiesCommandFromRegistryWithoutKindSidecar(t *testing.T) {
	root := withVersionMarkerBaseDir(t)
	writeVersionMarker(t, root, "codex", "0.142.3")

	byService := map[string]*InstalledServiceInfo{}
	loadMarkers(context.Background(), byService, func(error) {})

	got := byService["codex"]
	if got == nil {
		t.Fatal("codex marker was not loaded")
	}
	if got.Kind != "COMMAND" {
		t.Fatalf("codex marker kind = %q, want COMMAND", got.Kind)
	}
}

func TestSkipSystemdUnitsDoesNotSkipMCPService(t *testing.T) {
	if skipSystemdUnits["mcp"] {
		t.Fatal("mcp is a SERVICE with a real systemd unit; it must not be skipped by loadSystemdUnits")
	}
}
