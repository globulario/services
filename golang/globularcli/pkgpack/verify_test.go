package pkgpack

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeTGZ creates a .tar.gz at path with the given entries.
func writeTGZ(t *testing.T, path string, entries map[string][]byte) {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	for name, content := range entries {
		tw.WriteHeader(&tar.Header{
			Name: name,
			Size: int64(len(content)),
			Mode: 0o644,
		})
		tw.Write(content)
	}

	tw.Close()
	gw.Close()
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

func sha256Hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

func TestVerifyTGZ_ApplicationPackage(t *testing.T) {
	dir := t.TempDir()
	tgzPath := filepath.Join(dir, "app.tgz")

	manifest := Manifest{
		Type:    "application",
		Name:    "webadmin",
		Version: "1.0.0",
		Platform: "linux_amd64",
		Publisher: "core@globular.io",
	}
	mdata, _ := json.Marshal(manifest)

	writeTGZ(t, tgzPath, map[string][]byte{
		"package.json": mdata,
		"index.html":   []byte("<html>hello</html>"),
		"css/style.css": []byte("body {}"),
	})

	summary, err := VerifyTGZ(tgzPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.Type != "application" {
		t.Errorf("Type = %q, want application", summary.Type)
	}
	if summary.Name != "webadmin" {
		t.Errorf("Name = %q, want webadmin", summary.Name)
	}
}

func TestVerifyTGZ_ApplicationNoBinOK(t *testing.T) {
	dir := t.TempDir()
	tgzPath := filepath.Join(dir, "app.tgz")

	manifest := Manifest{Type: "application", Name: "myapp", Version: "2.0.0", Platform: "linux_amd64", Publisher: "core@globular.io"}
	mdata, _ := json.Marshal(manifest)

	// Application with no bin/ or specs/ — should be valid.
	writeTGZ(t, tgzPath, map[string][]byte{
		"package.json": mdata,
		"index.html":   []byte("<html></html>"),
	})

	_, err := VerifyTGZ(tgzPath)
	if err != nil {
		t.Fatalf("application without bin/ should be valid: %v", err)
	}
}

func TestVerifyTGZ_ApplicationEmptyContent(t *testing.T) {
	dir := t.TempDir()
	tgzPath := filepath.Join(dir, "app.tgz")

	manifest := Manifest{Type: "application", Name: "empty", Version: "1.0.0", Platform: "linux_amd64", Publisher: "core@globular.io"}
	mdata, _ := json.Marshal(manifest)

	// Only package.json, no content files.
	writeTGZ(t, tgzPath, map[string][]byte{
		"package.json": mdata,
	})

	_, err := VerifyTGZ(tgzPath)
	if err == nil {
		t.Fatal("expected error for application with no content files")
	}
}

func TestVerifyTGZ_ServiceStillRequiresBin(t *testing.T) {
	dir := t.TempDir()
	tgzPath := filepath.Join(dir, "svc.tgz")

	manifest := Manifest{Name: "gateway", Version: "1.0.0", Platform: "linux_amd64", Publisher: "core@globular.io"} // Type defaults to "service"
	mdata, _ := json.Marshal(manifest)

	// Service package with no bin/ should fail.
	writeTGZ(t, tgzPath, map[string][]byte{
		"package.json": mdata,
		"index.html":   []byte("nope"),
	})

	_, err := VerifyTGZ(tgzPath)
	if err == nil {
		t.Fatal("expected error for service without bin/")
	}
}

func TestVerifyTGZ_InfrastructurePackage(t *testing.T) {
	dir := t.TempDir()
	tgzPath := filepath.Join(dir, "etcd.tgz")

	manifest := Manifest{Type: "infrastructure", Name: "etcd", Version: "3.5.14", Platform: "linux_amd64", Publisher: "core@globular.io"}
	mdata, _ := json.Marshal(manifest)

	// Infrastructure: bin/ required, specs/ NOT required, systemd/ and config/ optional.
	writeTGZ(t, tgzPath, map[string][]byte{
		"package.json":              mdata,
		"bin/etcd":                  []byte("binary"),
		"systemd/globular-etcd.service": []byte("[Unit]\nDescription=etcd"),
		"config/etcd.yaml":          []byte("data-dir: /var/lib/etcd"),
	})

	summary, err := VerifyTGZ(tgzPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.Type != "infrastructure" {
		t.Errorf("Type = %q, want infrastructure", summary.Type)
	}
	if summary.SystemdCount != 1 {
		t.Errorf("SystemdCount = %d, want 1", summary.SystemdCount)
	}
	if summary.ConfigCount != 1 {
		t.Errorf("ConfigCount = %d, want 1", summary.ConfigCount)
	}
}

func TestVerifyTGZ_InfrastructureNoBin(t *testing.T) {
	dir := t.TempDir()
	tgzPath := filepath.Join(dir, "bad.tgz")

	manifest := Manifest{Type: "infrastructure", Name: "etcd", Version: "3.5.14", Platform: "linux_amd64", Publisher: "core@globular.io"}
	mdata, _ := json.Marshal(manifest)

	// Infrastructure without bin/ should fail.
	writeTGZ(t, tgzPath, map[string][]byte{
		"package.json":   mdata,
		"config/etcd.yaml": []byte("config"),
	})

	_, err := VerifyTGZ(tgzPath)
	if err == nil {
		t.Fatal("expected error for infrastructure without bin/")
	}
}

func TestVerifyTGZ_TypeDefault(t *testing.T) {
	dir := t.TempDir()
	tgzPath := filepath.Join(dir, "svc.tgz")

	manifest := Manifest{Name: "gateway", Version: "1.0.0", Platform: "linux_amd64", Publisher: "core@globular.io"} // empty Type → "service"
	mdata, _ := json.Marshal(manifest)

	writeTGZ(t, tgzPath, map[string][]byte{
		"package.json":         mdata,
		"bin/gateway_server":   []byte("binary"),
		"specs/gateway.yaml":   []byte("spec"),
	})

	summary, err := VerifyTGZ(tgzPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if summary.Type != "service" {
		t.Errorf("Type = %q, want service (default)", summary.Type)
	}
}

// TestVerifyTGZ_AwarenessBundleShapeRejected pins the boundary between the
// service-package publish path and the awareness-bundle publish path. An
// awareness bundle ships manifest.json (not package.json) and has no
// bin/specs layout, so VerifyTGZ MUST reject it — the bundle has its own
// validator in golang/globularcli/awareness_bundle_publish.go. Without
// this rejection, someone could run `globular pkg publish` on an
// awareness bundle and the service path would attempt to register it as
// a SERVICE artifact with no entrypoint, corrupting the catalog.
func TestVerifyTGZ_AwarenessBundleShapeRejected(t *testing.T) {
	dir := t.TempDir()
	tgzPath := filepath.Join(dir, "awareness-bundle.tar.gz")

	// Awareness manifest shape — not package.json.
	awareness := map[string]string{
		"name":     "globular-awareness-bundle",
		"kind":     "AWARENESS_BUNDLE",
		"version":  "0.0.1",
		"build_id": "abc",
	}
	mdata, _ := json.Marshal(awareness)

	writeTGZ(t, tgzPath, map[string][]byte{
		"manifest.json": mdata,
		"graph.json":    []byte(`{"version":1}`),
	})

	if _, err := VerifyTGZ(tgzPath); err == nil {
		t.Fatal("VerifyTGZ should reject an awareness bundle (no package.json) — use `awareness bundle publish` instead")
	}
}

func TestVerifyTGZ_RejectsEmbeddedBuildTokenInVersion(t *testing.T) {
	dir := t.TempDir()
	tgzPath := filepath.Join(dir, "badver.tgz")

	manifest := Manifest{Name: "dns", Version: "1.2.3+b325", Platform: "linux_amd64", Publisher: "core@globular.io"}
	mdata, _ := json.Marshal(manifest)

	writeTGZ(t, tgzPath, map[string][]byte{
		"package.json":       mdata,
		"bin/dns_server":     []byte("binary"),
		"specs/dns.yaml":     []byte("steps: []"),
		"config/dns/config":  []byte("x"),
	})

	_, err := VerifyTGZ(tgzPath)
	if err == nil || !strings.Contains(err.Error(), "embeds a build token") {
		t.Fatalf("expected embedded build token error, got %v", err)
	}
}

func TestVerifyTGZ_RejectsEntrypointChecksumMismatch(t *testing.T) {
	dir := t.TempDir()
	tgzPath := filepath.Join(dir, "badsha.tgz")
	payload := []byte("binary-v1")

	manifest := Manifest{
		Name:               "dns",
		Version:            "1.2.3",
		Platform:           "linux_amd64",
		Publisher:          "core@globular.io",
		Entrypoint:         "bin/dns_server",
		EntrypointChecksum: "sha256:" + strings.Repeat("a", 64),
	}
	mdata, _ := json.Marshal(manifest)

	writeTGZ(t, tgzPath, map[string][]byte{
		"package.json":   mdata,
		"bin/dns_server": payload,
		"specs/dns.yaml": []byte("steps: []"),
	})

	_, err := VerifyTGZ(tgzPath)
	if err == nil || !strings.Contains(err.Error(), "entrypoint_checksum mismatch") {
		t.Fatalf("expected entrypoint checksum mismatch, got %v", err)
	}
}

func TestVerifyTGZ_AcceptsEntrypointChecksumMatch(t *testing.T) {
	dir := t.TempDir()
	tgzPath := filepath.Join(dir, "goodsha.tgz")
	payload := []byte("binary-v1")

	manifest := Manifest{
		Name:               "dns",
		Version:            "1.2.3",
		Platform:           "linux_amd64",
		Publisher:          "core@globular.io",
		Entrypoint:         "bin/dns_server",
		EntrypointChecksum: "sha256:" + sha256Hex(payload),
	}
	mdata, _ := json.Marshal(manifest)

	writeTGZ(t, tgzPath, map[string][]byte{
		"package.json":   mdata,
		"bin/dns_server": payload,
		"specs/dns.yaml": []byte("steps: []"),
	})

	if _, err := VerifyTGZ(tgzPath); err != nil {
		t.Fatalf("expected checksum match to pass, got %v", err)
	}
}

func TestVerifyTGZ_RejectsDuplicateSystemdSingletonDirective(t *testing.T) {
	dir := t.TempDir()
	tgzPath := filepath.Join(dir, "badsystemd.tgz")

	unit := `[Unit]
Description=dns
[Service]
Type=simple
Type=forking
ExecStart=/usr/lib/globular/bin/dns_server
`
	manifest := Manifest{
		Type:      "infrastructure",
		Name:      "dns",
		Version:   "1.2.3",
		Platform:  "linux_amd64",
		Publisher: "core@globular.io",
	}
	mdata, _ := json.Marshal(manifest)

	writeTGZ(t, tgzPath, map[string][]byte{
		"package.json":                mdata,
		"bin/dns_server":              []byte("binary"),
		"systemd/globular-dns.service": []byte(unit),
	})

	_, err := VerifyTGZ(tgzPath)
	if err == nil || !strings.Contains(err.Error(), "duplicate singleton directive") {
		t.Fatalf("expected duplicate systemd directive error, got %v", err)
	}
}

// writeTGZWithModes is like writeTGZ but lets callers override the tar header
// mode for specific entries. Used to exercise the scripts/-must-be-executable
// publish guard in verify.go without relying on the default writer's 0o644.
func writeTGZWithModes(t *testing.T, path string, entries map[string][]byte, modes map[string]int64) {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	for name, content := range entries {
		mode := int64(0o644)
		if m, ok := modes[name]; ok {
			mode = m
		}
		_ = tw.WriteHeader(&tar.Header{
			Name: name,
			Size: int64(len(content)),
			Mode: mode,
		})
		_, _ = tw.Write(content)
	}

	_ = tw.Close()
	_ = gw.Close()
	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Regression guard for the mcp Day-0 publish failure observed 2026-06-04:
// packages/metadata/mcp/scripts/post-install.sh shipped at mode 0o664, CI's
// shutil.copytree preserved the source mode, and the tarball reached the
// publish-time validator which rejected it with "script ... is not executable".
// VerifyTGZ must reject any file under scripts/ that lacks an execute bit.
func TestVerifyTGZ_ScriptsNonExecutable_Rejected(t *testing.T) {
	dir := t.TempDir()
	tgzPath := filepath.Join(dir, "svc.tgz")

	manifest := Manifest{
		Type:                "service",
		Name:                "demo",
		Version:             "1.0.0",
		Platform:            "linux_amd64",
		Publisher:           "core@globular.io",
		EntrypointChecksum: "sha256:" + sha256Hex([]byte("entry")),
	}
	mdata, _ := json.Marshal(manifest)

	writeTGZ(t, tgzPath, map[string][]byte{
		"package.json":            mdata,
		"bin/demo":                []byte("entry"),
		"scripts/post-install.sh": []byte("#!/bin/sh\necho hi\n"),
	})

	_, err := VerifyTGZ(tgzPath)
	if err == nil {
		t.Fatal("expected VerifyTGZ to reject non-executable script, got nil")
	}
	if !strings.Contains(err.Error(), "is not executable") {
		t.Fatalf("expected error containing 'is not executable', got %v", err)
	}
	if !strings.Contains(err.Error(), "scripts/post-install.sh") {
		t.Fatalf("expected error to name the offending script, got %v", err)
	}
}

// Pinned counterpart: a script entry at any executable mode (e.g. 0o755)
// must pass the validator. The pkgpack builder normalizes to 0o755 via
// entryMode; this test confirms the validator side accepts that mode.
// Uses an infrastructure-typed package to keep the test scoped to the
// scripts/-mode rule (spec/ presence has its own test surface).
func TestVerifyTGZ_ScriptsExecutable_Accepted(t *testing.T) {
	dir := t.TempDir()
	tgzPath := filepath.Join(dir, "infra.tgz")

	manifest := Manifest{
		Type:      "infrastructure",
		Name:      "demo",
		Version:   "1.0.0",
		Platform:  "linux_amd64",
		Publisher: "core@globular.io",
	}
	mdata, _ := json.Marshal(manifest)

	writeTGZWithModes(t, tgzPath, map[string][]byte{
		"package.json":            mdata,
		"bin/demo":                []byte("binary"),
		"scripts/post-install.sh": []byte("#!/bin/sh\necho hi\n"),
	}, map[string]int64{
		"scripts/post-install.sh": 0o755,
	})

	summary, err := VerifyTGZ(tgzPath)
	if err != nil {
		t.Fatalf("VerifyTGZ rejected an executable script: %v", err)
	}
	if summary.Name != "demo" {
		t.Errorf("Name = %q, want demo", summary.Name)
	}
	if summary.ScriptsCount != 1 {
		t.Errorf("ScriptsCount = %d, want 1", summary.ScriptsCount)
	}
}
