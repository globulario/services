package pkgpack

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// Ensures packages omit empty config payloads and manifest config_dir is empty.
func TestBuildPackage_OmitsEmptyConfigDir(t *testing.T) {
	root := t.TempDir()
	binDir := filepath.Join(root, "bin")
	specDir := filepath.Join(root, "specs")
	configDir := filepath.Join(root, "config", "empty-svc") // exists but empty
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(specDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(configDir, 0755); err != nil { // empty on purpose
		t.Fatal(err)
	}

	// binary
	exe := filepath.Join(binDir, "empty-svc")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatal(err)
	}

	// spec that references the binary
	specPath := filepath.Join(specDir, "empty_service.yaml")
	spec := "metadata:\n  name: empty-svc\nservice:\n  name: empty-svc\n  exec: empty-svc\nsteps:\n  - id: install-empty-svc\n    type: install_package_payload\n    install_bins: true\n    install_config: false\n    install_spec: false\n    install_systemd: false\n"
	if err := os.WriteFile(specPath, []byte(spec), 0644); err != nil {
		t.Fatal(err)
	}

	outDir := t.TempDir()
	platform := runtime.GOOS + "_" + runtime.GOARCH
	results, err := BuildPackages(BuildOptions{
		Root:      root,
		SpecPath:  specPath,
		Version:   "0.0.1",
		Platform:  platform,
		OutDir:    outDir,
		Publisher: "test@example.com",
	})
	if err != nil {
		t.Fatalf("build packages: %v", err)
	}
	if len(results) != 1 || results[0].Err != nil {
		t.Fatalf("result error: %+v", results)
	}
	pkg := results[0].OutputPath

	// Assert no config/ entries in archive
	for _, prefix := range []string{"config/", "config/empty-svc/"} {
		if ok, err := tgzContainsPrefix(pkg, prefix); err != nil {
			t.Fatal(err)
		} else if ok {
			t.Fatalf("package should not contain %s", prefix)
		}
	}

	// Assert manifest defaults.config_dir empty
	mfData, err := readEntryFromTgz(pkg, "package.json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(mfData), `"configDir": ""`) {
		t.Fatalf("expected configDir empty in manifest, got: %s", string(mfData))
	}
}
