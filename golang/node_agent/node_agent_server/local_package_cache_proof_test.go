package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func withLocalPackageProofDirs(t *testing.T) (cacheDir, binDir string) {
	t.Helper()
	cacheDir = t.TempDir()
	binDir = t.TempDir()

	oldDirs := localPackageDirs
	oldBin := globularBinDir
	localPackageDirs = []string{cacheDir}
	globularBinDir = binDir
	t.Cleanup(func() {
		localPackageDirs = oldDirs
		globularBinDir = oldBin
	})
	return cacheDir, binDir
}

func writeProofPackage(t *testing.T, path, manifest string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("create package: %v", err)
	}
	defer f.Close()

	gz := gzip.NewWriter(f)
	defer gz.Close()
	tr := tar.NewWriter(gz)
	defer tr.Close()

	data := []byte(manifest)
	if err := tr.WriteHeader(&tar.Header{Name: "package.json", Mode: 0o644, Size: int64(len(data))}); err != nil {
		t.Fatalf("write package header: %v", err)
	}
	if _, err := tr.Write(data); err != nil {
		t.Fatalf("write package manifest: %v", err)
	}
}

func hexSHA256(data []byte) string {
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum[:])
}

func TestLocalPackageCacheProofsRequireManifestEntrypointMatch(t *testing.T) {
	cacheDir, binDir := withLocalPackageProofDirs(t)
	binary := []byte("alertmanager-binary")
	binPath := filepath.Join(binDir, "alertmanager")
	if err := os.WriteFile(binPath, binary, 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}
	checksum := hexSHA256(binary)
	manifest := fmt.Sprintf(`{
  "type": "infrastructure",
  "name": "alertmanager",
  "version": "0.28.1",
  "platform": "linux_amd64",
  "publisher": "core@globular.io",
  "entrypoint": "bin/alertmanager",
  "entrypoint_checksum": "sha256:%s",
  "build_number": 7,
  "build_id": "build-alertmanager"
}`, checksum)
	writeProofPackage(t, filepath.Join(cacheDir, "alertmanager_0.28.1_linux_amd64.tgz"), manifest)

	proofs := localPackageCacheProofs("linux_amd64")
	if len(proofs) != 1 {
		t.Fatalf("proof count = %d, want 1 (%+v)", len(proofs), proofs)
	}
	proof := proofs[0]
	if proof.Name != "alertmanager" || proof.Kind != "INFRASTRUCTURE" || proof.Version != "0.28.1" {
		t.Fatalf("proof identity = %+v", proof)
	}
	if proof.ManifestEntrypointChecksum != checksum || proof.DiskEntrypointChecksum != checksum {
		t.Fatalf("proof checksums = manifest %q disk %q, want %q", proof.ManifestEntrypointChecksum, proof.DiskEntrypointChecksum, checksum)
	}
}

func TestLocalPackageCacheProofsRejectChecksumMismatch(t *testing.T) {
	cacheDir, binDir := withLocalPackageProofDirs(t)
	if err := os.WriteFile(filepath.Join(binDir, "alertmanager"), []byte("actual"), 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}
	manifest := fmt.Sprintf(`{
  "type": "infrastructure",
  "name": "alertmanager",
  "version": "0.28.1",
  "platform": "linux_amd64",
  "entrypoint": "bin/alertmanager",
  "entrypoint_checksum": "sha256:%s"
}`, hexSHA256([]byte("different")))
	writeProofPackage(t, filepath.Join(cacheDir, "alertmanager_0.28.1_linux_amd64.tgz"), manifest)

	if proofs := localPackageCacheProofs("linux_amd64"); len(proofs) != 0 {
		t.Fatalf("mismatched package produced proofs: %+v", proofs)
	}
}

func TestLocalPackageCacheProofsUseRegistryKindWhenManifestTypeMissing(t *testing.T) {
	cacheDir, binDir := withLocalPackageProofDirs(t)
	binary := []byte("mc-binary")
	if err := os.WriteFile(filepath.Join(binDir, "mc"), binary, 0o755); err != nil {
		t.Fatalf("write binary: %v", err)
	}
	checksum := hexSHA256(binary)
	manifest := fmt.Sprintf(`{
  "name": "mc",
  "version": "RELEASE.2025-08-13T08-35-41Z",
  "platform": "linux_amd64",
  "entrypoint": "bin/mc",
  "entrypoint_checksum": "sha256:%s"
}`, checksum)
	writeProofPackage(t, filepath.Join(cacheDir, "mc_RELEASE.2025-08-13T08-35-41Z_linux_amd64.tgz"), manifest)

	proofs := localPackageCacheProofs("linux_amd64")
	if len(proofs) != 1 {
		t.Fatalf("proof count = %d, want 1 (%+v)", len(proofs), proofs)
	}
	if proofs[0].Kind != "COMMAND" {
		t.Fatalf("kind = %q, want COMMAND", proofs[0].Kind)
	}
}
