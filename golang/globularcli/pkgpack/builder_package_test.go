package pkgpack

import (
	"archive/tar"
	"compress/gzip"
	"io"
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

// TestBuildPackage_IncludesScripts verifies that scripts from ScriptsRoot are
// embedded in the package with 0755 permissions and listed in the manifest.
func TestBuildPackage_IncludesScripts(t *testing.T) {
	root := t.TempDir()
	binDir := filepath.Join(root, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(binDir, "myscript-svc")
	if err := os.WriteFile(exe, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatal(err)
	}

	// Create scripts directory matching service name
	scriptsRoot := t.TempDir()
	svcScripts := filepath.Join(scriptsRoot, "myscript-svc")
	if err := os.MkdirAll(svcScripts, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(svcScripts, "pre-start.sh"), []byte("#!/bin/bash\necho pre\n"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(svcScripts, "post-install.sh"), []byte("#!/bin/bash\necho post\n"), 0755); err != nil {
		t.Fatal(err)
	}

	specPath := filepath.Join(t.TempDir(), "myscript_svc_service.yaml")
	spec := "metadata:\n  name: myscript-svc\nsteps:\n  - id: install\n    type: install_package_payload\n    install_bins: true\n"
	if err := os.WriteFile(specPath, []byte(spec), 0644); err != nil {
		t.Fatal(err)
	}

	outDir := t.TempDir()
	platform := runtime.GOOS + "_" + runtime.GOARCH
	results, err := BuildPackages(BuildOptions{
		Root:       root,
		SpecPath:   specPath,
		ScriptsDir:        scriptsRoot,
		Version:           "0.0.1",
		Platform:          platform,
		OutDir:            outDir,
		Publisher:         "test@example.com",
		SkipMissingConfig: true,
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if len(results) != 1 || results[0].Err != nil {
		t.Fatalf("result error: %+v", results)
	}
	pkg := results[0].OutputPath

	// Verify scripts/ entries are in archive
	for _, name := range []string{"scripts/pre-start.sh", "scripts/post-install.sh"} {
		if ok, err := tgzContains(pkg, name); err != nil {
			t.Fatal(err)
		} else if !ok {
			t.Fatalf("package missing %s", name)
		}
	}

	// Verify scripts have 0755 permissions
	scriptModes := tgzEntryModes(t, pkg, "scripts/")
	for name, mode := range scriptModes {
		if mode != 0755 {
			t.Errorf("script %s has mode %o, want 0755", name, mode)
		}
	}

	// Verify manifest has scriptsDir
	mfData, err := readEntryFromTgz(pkg, "package.json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(mfData), `"scriptsDir": "scripts"`) {
		t.Fatalf("expected scriptsDir in manifest, got: %s", string(mfData))
	}

	// Verify summary has correct script count
	summary, err := VerifyTGZ(pkg)
	if err != nil {
		t.Fatal(err)
	}
	if summary.ScriptsCount != 2 {
		t.Fatalf("expected 2 scripts, got %d", summary.ScriptsCount)
	}
}

// TestBuildPackage_NoScriptsDir verifies that packages without scripts have no
// scripts/ directory and no scriptsDir in the manifest.
func TestBuildPackage_NoScriptsDir(t *testing.T) {
	root := t.TempDir()
	binDir := filepath.Join(root, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "nosvc"), []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatal(err)
	}

	specPath := filepath.Join(t.TempDir(), "nosvc_service.yaml")
	spec := "metadata:\n  name: nosvc\nsteps:\n  - id: install\n    type: install_package_payload\n    install_bins: true\n"
	if err := os.WriteFile(specPath, []byte(spec), 0644); err != nil {
		t.Fatal(err)
	}

	outDir := t.TempDir()
	platform := runtime.GOOS + "_" + runtime.GOARCH
	results, err := BuildPackages(BuildOptions{
		Root:              root,
		SpecPath:          specPath,
		Version:           "0.0.1",
		Platform:          platform,
		OutDir:            outDir,
		SkipMissingConfig: true,
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if len(results) != 1 || results[0].Err != nil {
		t.Fatalf("result error: %+v", results)
	}
	pkg := results[0].OutputPath

	if ok, err := tgzContainsPrefix(pkg, "scripts/"); err != nil {
		t.Fatal(err)
	} else if ok {
		t.Fatal("package should not contain scripts/")
	}

	mfData, err := readEntryFromTgz(pkg, "package.json")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(mfData), "scriptsDir") {
		t.Fatalf("manifest should not have scriptsDir, got: %s", string(mfData))
	}
}

// TestBuildPackage_ScriptsAutoDiscoverFromRoot verifies scripts are discovered
// from root/scripts/<service>/ when --scripts-dir is not set.
func TestBuildPackage_ScriptsAutoDiscoverFromRoot(t *testing.T) {
	root := t.TempDir()
	binDir := filepath.Join(root, "bin")
	if err := os.MkdirAll(binDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(binDir, "autosvc"), []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatal(err)
	}

	// Scripts at root/scripts/autosvc/
	svcScripts := filepath.Join(root, "scripts", "autosvc")
	if err := os.MkdirAll(svcScripts, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(svcScripts, "post-install.sh"), []byte("#!/bin/bash\necho hi\n"), 0755); err != nil {
		t.Fatal(err)
	}

	specPath := filepath.Join(t.TempDir(), "autosvc_service.yaml")
	spec := "metadata:\n  name: autosvc\nsteps:\n  - id: install\n    type: install_package_payload\n    install_bins: true\n"
	if err := os.WriteFile(specPath, []byte(spec), 0644); err != nil {
		t.Fatal(err)
	}

	outDir := t.TempDir()
	platform := runtime.GOOS + "_" + runtime.GOARCH
	results, err := BuildPackages(BuildOptions{
		Root:              root,
		SpecPath:          specPath,
		Version:           "0.0.1",
		Platform:          platform,
		OutDir:            outDir,
		SkipMissingConfig: true,
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	if len(results) != 1 || results[0].Err != nil {
		t.Fatalf("result error: %+v", results)
	}
	pkg := results[0].OutputPath

	if ok, err := tgzContains(pkg, "scripts/post-install.sh"); err != nil {
		t.Fatal(err)
	} else if !ok {
		t.Fatal("auto-discovered script not in package")
	}

	summary, err := VerifyTGZ(pkg)
	if err != nil {
		t.Fatal(err)
	}
	if summary.ScriptsCount != 1 {
		t.Fatalf("expected 1 script, got %d", summary.ScriptsCount)
	}
}

// tgzEntryModes returns a map of entry name → file mode for entries matching a prefix.
func tgzEntryModes(t *testing.T, tgzPath, prefix string) map[string]int64 {
	t.Helper()
	f, err := os.Open(tgzPath)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)
	modes := make(map[string]int64)
	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(hdr.Name, prefix) && !strings.HasSuffix(hdr.Name, "/") {
			modes[hdr.Name] = hdr.Mode
		}
	}
	return modes
}
