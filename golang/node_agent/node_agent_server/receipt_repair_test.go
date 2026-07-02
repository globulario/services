package main

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/globulario/services/golang/component_catalog"
	"github.com/globulario/services/golang/node_agent/node_agentpb"
)

func TestRestampInstalledPackageReceipt_PreservesIdentityFields(t *testing.T) {
	dir := t.TempDir()
	unitPath := filepath.Join(dir, "globular-dns.service")
	binPath := filepath.Join(dir, "dns_server")
	if err := os.WriteFile(unitPath, []byte("[Unit]\nDescription=DNS\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(binPath, []byte("binary-bytes"), 0o755); err != nil {
		t.Fatal(err)
	}
	artifact := writeCanonicalArtifact(t, dir, "dns_1.2.257_"+runtime.GOOS+"_"+runtime.GOARCH+".tgz", `version: 1
metadata:
  name: dns
steps:
  - id: install-dns
    type: install_services
    units:
      - name: globular-dns.service
        content: |
          [Unit]
          Description=DNS
          [Service]
          WorkingDirectory={{.StateDir}}/dns
          ExecStart={{.Prefix}}/bin/dns_server --node {{.NodeIP}}
`)
	origLocalDirs := localPackageDirs
	localPackageDirs = []string{dir}
	t.Cleanup(func() { localPackageDirs = origLocalDirs })
	// Keep an artifact available so the restamp path can discover canonical
	// metadata, but write a deliberately different disk unit. The receipt must
	// describe the evidence currently on disk rather than pretending the
	// canonical render is installed.
	srv := &NodeAgentServer{}
	if _, err := renderCanonicalUnitFromArtifactPath(artifact, "globular-dns.service", srv.canonicalUnitRenderInput(context.Background(), "dns", "1.2.257", "SERVICE")); err != nil {
		t.Fatalf("renderCanonicalUnitFromArtifactPath: %v", err)
	}

	pkg := &node_agentpb.InstalledPackage{
		Name:          "dns",
		Kind:          "SERVICE",
		Version:       "1.2.257",
		BuildId:       "build-123",
		BuildNumber:   7,
		Checksum:      "keep-checksum",
		InstalledUnix: 101,
		UpdatedUnix:   202,
		Status:        "installed",
		Metadata: map[string]string{
			receiptKeyUnitFilePath: unitPath,
			receiptKeyBinaryPath:   binPath,
			"other_key":            "preserve-me",
		},
	}

	allowed, reason := packageAuthorizedForReceiptRepair(pkg.GetName(), []string{"core", "control-plane", "storage"})
	if !allowed {
		t.Fatalf("dns should be authorized for receipt repair: %s", reason)
	}

	changed, err := srv.restampInstalledPackageReceipt(context.Background(), pkg)
	if err != nil {
		t.Fatalf("restampInstalledPackageReceipt: %v", err)
	}
	if !changed {
		t.Fatal("expected receipt restamp to change metadata")
	}
	if got := pkg.Metadata[receiptKeyInstalledBy]; got != receiptRepairInstalledBy {
		t.Fatalf("installed_by = %q, want %q", got, receiptRepairInstalledBy)
	}
	if got := pkg.Metadata[receiptKeyUnitFileSha256]; got == "" {
		t.Fatal("unit_file_sha256 missing after restamp")
	} else if got != fileSha256Hex(t, unitPath) {
		t.Fatalf("unit_file_sha256 = %q, want disk hash %q", got, fileSha256Hex(t, unitPath))
	}
	if got := pkg.Metadata[receiptKeyBinarySha256]; got == "" {
		t.Fatal("binary_sha256 missing after restamp")
	}
	if pkg.BuildId != "build-123" {
		t.Fatalf("BuildId changed: %q", pkg.BuildId)
	}
	if pkg.BuildNumber != 7 {
		t.Fatalf("BuildNumber changed: %d", pkg.BuildNumber)
	}
	if pkg.Version != "1.2.257" {
		t.Fatalf("Version changed: %q", pkg.Version)
	}
	if pkg.Checksum != "keep-checksum" {
		t.Fatalf("Checksum changed: %q", pkg.Checksum)
	}
	if pkg.InstalledUnix != 101 {
		t.Fatalf("InstalledUnix changed: %d", pkg.InstalledUnix)
	}
	if pkg.UpdatedUnix != 202 {
		t.Fatalf("UpdatedUnix changed: %d", pkg.UpdatedUnix)
	}
	if pkg.Metadata["other_key"] != "preserve-me" {
		t.Fatalf("unrelated metadata changed: %v", pkg.Metadata)
	}
}

func fileSha256Hex(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return sha256Hex(data)
}

func TestReceiptRepair_RefusesOrphanMediaPackage(t *testing.T) {
	dir := t.TempDir()
	unitPath := filepath.Join(dir, "globular-media.service")
	if err := os.WriteFile(unitPath, []byte("[Unit]\nDescription=Media\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	pkg := &node_agentpb.InstalledPackage{
		Name:        "media",
		Kind:        "SERVICE",
		Version:     "1.2.257",
		BuildId:     "build-456",
		Status:      "installed",
		Metadata:    map[string]string{receiptKeyUnitFilePath: unitPath},
		UpdatedUnix: 33,
	}

	allowed, _ := packageAuthorizedForReceiptRepair(pkg.GetName(), []string{"core", "control-plane", "storage"})
	if allowed {
		t.Fatal("media must be unauthorized on a core/control-plane/storage node")
	}
	if got := pkg.Metadata[receiptKeyUnitFileSha256]; got != "" {
		t.Fatalf("unit_file_sha256 unexpectedly pre-set: %q", got)
	}
	if got := pkg.Metadata[receiptKeyInstalledBy]; got != "" {
		t.Fatalf("installed_by unexpectedly pre-set: %q", got)
	}
}

func TestQuorumProfiles_DoNotInstallOrReceiptMediaPackages(t *testing.T) {
	profiles := []string{"core", "control-plane", "storage"}
	pkgs := component_catalog.PackagesForProfiles(profiles)

	for _, mediaPkg := range []string{"media", "title", "ffmpeg", "yt-dlp", "torrent"} {
		for _, pkg := range pkgs {
			if pkg == mediaPkg {
				t.Fatalf("quorum-only profiles must not install media package %q; got %v", mediaPkg, pkgs)
			}
		}
		allowed, reason := packageAuthorizedForReceiptRepair(mediaPkg, profiles)
		if allowed {
			t.Fatalf("quorum-only profiles must not authorize receipt repair for %q", mediaPkg)
		}
		if reason == "" {
			t.Fatalf("expected authorization denial reason for %q", mediaPkg)
		}
	}
}
