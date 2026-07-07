package pkgpack

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// TestBuildPackage_NoEntrypoint_EmitsNoneManifestAndNoBinary is the
// noop-elimination regression guard. A spec declaring `entrypoint: none`
// (a binary-less OS-daemon / .deb wrapper) must build a valid package that:
//   - carries manifest entrypoint "none",
//   - carries NO entrypoint_checksum (omitempty → absent),
//   - bundles no bin/<binary> payload,
//   - still passes the builder's own VerifyTGZ (the build returning no error).
//
// This replaces the "noop" sentinel binary: a wrapper package no longer needs
// to ship a fake executable to satisfy the entrypoint verifier.
func TestBuildPackage_NoEntrypoint_EmitsNoneManifestAndNoBinary(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0755); err != nil {
		t.Fatal(err)
	}
	specDir := filepath.Join(root, "specs")
	if err := os.MkdirAll(specDir, 0755); err != nil {
		t.Fatal(err)
	}
	specPath := filepath.Join(specDir, "wrapper_service.yaml")
	// Infrastructure wrapper: an OS daemon, no Globular binary.
	spec := "metadata:\n" +
		"  name: wrapper-svc\n" +
		"  kind: infrastructure\n" +
		"  entrypoint: none\n" +
		"steps:\n" +
		"  - id: install\n" +
		"    type: install_package_payload\n" +
		"    install_bins: false\n" +
		"    install_config: false\n" +
		"    install_spec: false\n" +
		"    install_systemd: false\n"
	if err := os.WriteFile(specPath, []byte(spec), 0644); err != nil {
		t.Fatal(err)
	}

	outDir := t.TempDir()
	platform := runtime.GOOS + "_" + runtime.GOARCH
	results, err := BuildPackages(BuildOptions{
		Root:               root,
		SpecPath:           specPath,
		Version:            "0.0.1",
		Platform:           platform,
		OutDir:             outDir,
		Publisher:          "test@example.com",
		SkipMissingConfig:  true,
		SkipMissingSystemd: true,
	})
	if err != nil {
		t.Fatalf("build no-entrypoint package: %v", err)
	}
	if len(results) != 1 || results[0].Err != nil {
		t.Fatalf("result error: %+v", results)
	}
	pkg := results[0].OutputPath

	mfData, err := readEntryFromTgz(pkg, "package.json")
	if err != nil {
		t.Fatal(err)
	}
	mf := string(mfData)
	if !strings.Contains(mf, `"entrypoint": "none"`) {
		t.Errorf("expected entrypoint \"none\" in manifest, got:\n%s", mf)
	}
	if strings.Contains(mf, "entrypoint_checksum") {
		t.Errorf("no-entrypoint package must not carry entrypoint_checksum, got:\n%s", mf)
	}

	// No regular file under bin/ (an empty bin/ dir entry is acceptable).
	if ok, err := tgzContains(pkg, "bin/noop"); err != nil {
		t.Fatal(err)
	} else if ok {
		t.Error("no-entrypoint package must not bundle the noop sentinel binary")
	}
}
