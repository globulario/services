package pkgpack

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestCopyConfigDirsCollision(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()

	if err := os.MkdirAll(filepath.Join(dir1, "conf"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir1, "conf", "config.json"), []byte("a"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(filepath.Join(dir2, "conf"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir2, "conf", "config.json"), []byte("b"), 0644); err != nil {
		t.Fatal(err)
	}

	dest := t.TempDir()
	err := copyConfigDirs([]string{dir1, dir2}, dest)
	if err == nil {
		t.Fatalf("expected collision error, got nil")
	}
}

func TestVerifyConfigDirPresent(t *testing.T) {
	staging := t.TempDir()
	if err := os.MkdirAll(filepath.Join(staging, "bin"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staging, "bin", "exec"), []byte("hi"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(staging, "specs"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staging, "specs", "spec.yaml"), []byte("version: 1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	cfgDir := filepath.Join(staging, "config", "svc")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "c.json"), []byte("{}"), 0644); err != nil {
		t.Fatal(err)
	}

	manifest := Manifest{
		Type:       "service",
		Name:       "svc",
		Version:    "1.0.0",
		Platform:   "linux_amd64",
		Publisher:  "core@globular.io",
		Entrypoint: "bin/exec",
		Defaults:   ManifestDefault{ConfigDir: "config/svc", Spec: "specs/spec.yaml"},
	}
	manifestBytes, _ := json.MarshalIndent(manifest, "", "  ")
	manifestBytes = append(manifestBytes, '\n')
	if err := os.WriteFile(filepath.Join(staging, "package.json"), manifestBytes, 0644); err != nil {
		t.Fatal(err)
	}

	out := filepath.Join(t.TempDir(), "pkg.tgz")
	if err := WriteTgz(out, staging); err != nil {
		t.Fatalf("write tgz: %v", err)
	}
	if _, err := VerifyTGZ(out); err != nil {
		t.Fatalf("verify tgz: %v", err)
	}
}
