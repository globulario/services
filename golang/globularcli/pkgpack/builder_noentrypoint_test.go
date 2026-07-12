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

// TestBuildPackage_NoEntrypoint_BinarySha256Identity_CarriesDeclaredChecksum:
// a binary-less package that DECLARES a binary_sha256 identity must carry that
// pinned checksum into the manifest verbatim. The build never sees the fetched
// binary, so the declared value IS the canonical identity — the node-agent
// re-hashes the installed binary against it (never recomputes a missing one).
func TestBuildPackage_NoEntrypoint_BinarySha256Identity_CarriesDeclaredChecksum(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "bin"), 0755)
	specDir := filepath.Join(root, "specs")
	os.MkdirAll(specDir, 0755)
	specPath := filepath.Join(specDir, "wrapper_cmd.yaml")
	pinned := "sha256:ff41753634b20c869ef6a32a20863521b33d4186ac0d6a49379ab48a48395ee7"
	spec := "metadata:\n" +
		"  name: wrapper-cmd\n" +
		"  kind: command\n" +
		"  entrypoint: none\n" +
		"  identity:\n" +
		"    proof: binary_sha256\n" +
		"    installed_path: /usr/local/bin/wrapper-cmd\n" +
		"    checksum: \"" + pinned + "\"\n" +
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
	results, err := BuildPackages(BuildOptions{
		Root: root, SpecPath: specPath, Version: "0.0.1",
		Platform: runtime.GOOS + "_" + runtime.GOARCH, OutDir: t.TempDir(),
		Publisher: "test@example.com", SkipMissingConfig: true, SkipMissingSystemd: true,
	})
	if err != nil || len(results) != 1 || results[0].Err != nil {
		t.Fatalf("build: err=%v results=%+v", err, results)
	}
	mf, err := readEntryFromTgz(results[0].OutputPath, "package.json")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(mf), pinned) {
		t.Errorf("binary_sha256 identity must carry declared checksum %q into manifest:\n%s", pinned, string(mf))
	}
}

// TestBuildPackage_NoEntrypoint_VersionIdentity_NoChecksum: a version-proved noop
// package (vendor tree/symlink, .deb, OS-repo) must NOT synthesize a binary
// checksum — the version is the identity; the manifest stays checksum-free.
func TestBuildPackage_NoEntrypoint_VersionIdentity_NoChecksum(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "bin"), 0755)
	specDir := filepath.Join(root, "specs")
	os.MkdirAll(specDir, 0755)
	specPath := filepath.Join(specDir, "osdaemon_service.yaml")
	spec := "metadata:\n" +
		"  name: osdaemon\n" +
		"  kind: infrastructure\n" +
		"  entrypoint: none\n" +
		"  identity:\n" +
		"    proof: version\n" +
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
	results, err := BuildPackages(BuildOptions{
		Root: root, SpecPath: specPath, Version: "0.0.1",
		Platform: runtime.GOOS + "_" + runtime.GOARCH, OutDir: t.TempDir(),
		Publisher: "test@example.com", SkipMissingConfig: true, SkipMissingSystemd: true,
	})
	if err != nil || len(results) != 1 || results[0].Err != nil {
		t.Fatalf("build: err=%v results=%+v", err, results)
	}
	mf, err := readEntryFromTgz(results[0].OutputPath, "package.json")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(mf), "entrypoint_checksum") {
		t.Errorf("version-mode identity must not carry entrypoint_checksum:\n%s", string(mf))
	}
}
