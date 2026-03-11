package pkgpack

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"encoding/json"
	"os"
	"path/filepath"
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

func TestVerifyTGZ_ApplicationPackage(t *testing.T) {
	dir := t.TempDir()
	tgzPath := filepath.Join(dir, "app.tgz")

	manifest := Manifest{
		Type:    "application",
		Name:    "webadmin",
		Version: "1.0.0",
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

	manifest := Manifest{Type: "application", Name: "myapp", Version: "2.0.0"}
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

	manifest := Manifest{Type: "application", Name: "empty", Version: "1.0.0"}
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

	manifest := Manifest{Name: "gateway", Version: "1.0.0"} // Type defaults to "service"
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

	manifest := Manifest{Type: "infrastructure", Name: "etcd", Version: "3.5.14"}
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

	manifest := Manifest{Type: "infrastructure", Name: "etcd", Version: "3.5.14"}
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

	manifest := Manifest{Name: "gateway", Version: "1.0.0"} // empty Type → "service"
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
