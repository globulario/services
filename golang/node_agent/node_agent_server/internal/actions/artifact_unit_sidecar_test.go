package actions

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/protobuf/types/known/structpb"
)

// TestInstallPayload_BundledServiceUnit_WritesSidecar verifies that extracting
// a package tarball that contains a systemd unit in the systemd/ directory
// writes a .sha256 sidecar alongside the installed unit file.
// Guardrail 4: unit definition drift — artifact extraction path.
//
// The hash in the sidecar must match the FINAL installed bytes (post
// template-rendering and normalization), not the raw tarball bytes.
func TestInstallPayload_BundledServiceUnit_WritesSidecar(t *testing.T) {
	dir := t.TempDir()

	// Override install paths to temp dirs.
	origSystemd := ActionSystemdDir
	origState := ActionStateDir
	origBin := ActionBinDir
	origStaging := ActionStagingRoot
	origAllowMissing := AllowMissingSHA256

	systemdDir := filepath.Join(dir, "systemd")
	stateDir := filepath.Join(dir, "state")
	binDir := filepath.Join(dir, "bin")
	stagingDir := filepath.Join(dir, "staging")

	if err := os.MkdirAll(systemdDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		t.Fatal(err)
	}

	ActionSystemdDir = systemdDir
	ActionStateDir = stateDir
	ActionBinDir = binDir
	ActionStagingRoot = stagingDir
	AllowMissingSHA256 = true
	t.Cleanup(func() {
		ActionSystemdDir = origSystemd
		ActionStateDir = origState
		ActionBinDir = origBin
		ActionStagingRoot = origStaging
		AllowMissingSHA256 = origAllowMissing
	})

	// Unit content — no template variables so rendered == raw.
	unitContent := []byte("[Unit]\nDescription=Test\n[Service]\nExecStart=/usr/bin/test\n[Install]\nWantedBy=multi-user.target\n")

	// Build a minimal .tgz with systemd/globular-test.service.
	tgzPath := filepath.Join(dir, "test.tgz")
	if err := buildTestTgz(tgzPath, map[string][]byte{
		"systemd/globular-test.service": unitContent,
	}); err != nil {
		t.Fatalf("build tgz: %v", err)
	}

	args, _ := structpb.NewStruct(map[string]interface{}{
		"service":       "test",
		"artifact_path": tgzPath,
	})
	act := serviceInstallPayloadAction{}
	if _, err := act.Apply(context.Background(), args); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// The installed unit file.
	installed := filepath.Join(systemdDir, "globular-test.service")
	installedData, err := os.ReadFile(installed)
	if err != nil {
		t.Fatalf("installed unit not found: %v", err)
	}

	// The sidecar must exist.
	sidecar := installed + ".sha256"
	sidecarData, err := os.ReadFile(sidecar)
	if err != nil {
		t.Fatalf("sidecar not written: %v", err)
	}

	// Sidecar must match sha256 of the final installed bytes.
	sum := sha256.Sum256(installedData)
	want := hex.EncodeToString(sum[:])
	got := strings.TrimSpace(string(sidecarData))
	if got != want {
		t.Errorf("sidecar hash = %q; want sha256(installed)=%q", got, want)
	}
}

// TestInstallPayload_BundledServiceUnit_SidecarMatchesRendered verifies that
// the sidecar hash matches the RENDERED (template-expanded) content, not the
// raw tarball bytes. This guards against a bug where the sidecar could be
// written before template expansion, causing perpetual false drift.
func TestInstallPayload_BundledServiceUnit_SidecarMatchesRendered(t *testing.T) {
	dir := t.TempDir()

	origSystemd := ActionSystemdDir
	origState := ActionStateDir
	origBin := ActionBinDir
	origStaging := ActionStagingRoot
	origAllowMissing := AllowMissingSHA256

	systemdDir := filepath.Join(dir, "systemd")
	stateDir := filepath.Join(dir, "state")
	binDir := filepath.Join(dir, "bin")

	for _, d := range []string{systemdDir, stateDir, binDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	ActionSystemdDir = systemdDir
	ActionStateDir = stateDir
	ActionBinDir = binDir
	ActionStagingRoot = filepath.Join(dir, "staging")
	AllowMissingSHA256 = true
	t.Cleanup(func() {
		ActionSystemdDir = origSystemd
		ActionStateDir = origState
		ActionBinDir = origBin
		ActionStagingRoot = origStaging
		AllowMissingSHA256 = origAllowMissing
	})

	// Unit with a template variable — renderTemplateVars will expand {{.StateDir}}.
	rawContent := []byte("[Unit]\nDescription=Test\n[Service]\nExecStart={{.StateDir}}/bin/test\n[Install]\nWantedBy=multi-user.target\n")

	tgzPath := filepath.Join(dir, "test.tgz")
	if err := buildTestTgz(tgzPath, map[string][]byte{
		"systemd/globular-tmpl.service": rawContent,
	}); err != nil {
		t.Fatalf("build tgz: %v", err)
	}

	args, _ := structpb.NewStruct(map[string]interface{}{
		"service":       "tmpl",
		"artifact_path": tgzPath,
	})
	act := serviceInstallPayloadAction{}
	if _, err := act.Apply(context.Background(), args); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	installed := filepath.Join(systemdDir, "globular-tmpl.service")
	installedData, err := os.ReadFile(installed)
	if err != nil {
		t.Fatalf("installed unit not found: %v", err)
	}
	sidecar := installed + ".sha256"
	sidecarData, err := os.ReadFile(sidecar)
	if err != nil {
		t.Fatalf("sidecar not written: %v", err)
	}

	// Sidecar must match the final installed (rendered) bytes.
	sum := sha256.Sum256(installedData)
	want := hex.EncodeToString(sum[:])
	got := strings.TrimSpace(string(sidecarData))
	if got != want {
		t.Errorf("sidecar hash = %q; want sha256(rendered)=%q", got, want)
	}

	// The raw content must differ from rendered (template was expanded).
	rawSum := sha256.Sum256(rawContent)
	rawHex := hex.EncodeToString(rawSum[:])
	if got == rawHex {
		t.Error("sidecar matches raw template bytes — was written before rendering (bug)")
	}
}

// buildTestTgz creates a .tgz archive with the given files (tarball-relative path → content).
func buildTestTgz(dest string, files map[string][]byte) error {
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	for name, data := range files {
		if err := tw.WriteHeader(&tar.Header{
			Name: name,
			Mode: 0o644,
			Size: int64(len(data)),
		}); err != nil {
			return err
		}
		if _, err := tw.Write(data); err != nil {
			return err
		}
	}
	return nil
}
