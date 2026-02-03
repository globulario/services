package pkgpack

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
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
	_, err := copyConfigDirs([]string{dir1, dir2}, dest)
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

func TestBuildPackagesWithRoot(t *testing.T) {
	payloadRoot := t.TempDir()

	binRoot := filepath.Join(payloadRoot, "bin")
	configRoot := filepath.Join(payloadRoot, "config", "root-service")
	if err := os.MkdirAll(binRoot, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(configRoot, 0755); err != nil {
		t.Fatal(err)
	}

	execPath := filepath.Join(binRoot, "root-service")
	if err := os.WriteFile(execPath, []byte("#!/bin/sh\necho hi\n"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configRoot, "config.yaml"), []byte("x: 1\n"), 0644); err != nil {
		t.Fatal(err)
	}

	specPath := filepath.Join(t.TempDir(), "root_service.yaml")
	spec := "metadata:\n  name: root-service\nsteps:\n  - id: install-root\n    type: install_package_payload\n    install_bins: true\n    install_config: true\n    install_spec: false\n    install_systemd: false\n"
	if err := os.WriteFile(specPath, []byte(spec), 0644); err != nil {
		t.Fatal(err)
	}

	outDir := t.TempDir()
	platform := runtime.GOOS + "_" + runtime.GOARCH
	results, err := BuildPackages(BuildOptions{
		Root:      payloadRoot,
		SpecPath:  specPath,
		Version:   "1.2.3",
		Platform:  platform,
		OutDir:    outDir,
		Publisher: "tester@example.com",
	})
	if err != nil {
		t.Fatalf("build packages: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Err != nil {
		t.Fatalf("result error: %v", results[0].Err)
	}
	if _, err := VerifyTGZ(results[0].OutputPath); err != nil {
		t.Fatalf("verify tgz: %v", err)
	}
}
