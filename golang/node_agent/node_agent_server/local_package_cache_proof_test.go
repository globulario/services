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

// TestLocalPackageCacheMustNotRedefineBuildIdentity is the regression test for
// the reinstall/restart loop caused by the local-package-cache repair path
// acting as a second writer of build_id.
//
// build_id has exactly one canonical source — the repository that published the
// artifact (identity.has_single_canonical_source_and_is_immutable). A locally
// built bundle mints a DIFFERENT uuid for byte-identical content, so when the
// repair path adopted the cache manifest's id it fought apply-package forever:
//
//	12:04:06  apply-package installs with the desired repository id 019fb141… (v7)
//	12:04:55  repair rewrites it to the cache manifest's 3989bbee… (v4)
//	next pass convergence sees a mismatch → reinstall → restart
//
// Observed live 2026-07-30 on the 5-minute sync ticker (11:22, 11:28, 11:34,
// 11:40, 11:46, 11:52, 11:58, 12:04) across workflow, monitoring, mcp, log,
// event, ai-watcher and ai-executor. Every restart dropped the service's
// listener, so cluster-doctor intermittently reported
// workflow.service_unavailable (connection refused) and went reduced-harvest.
func TestLocalPackageCacheMustNotRedefineBuildIdentity(t *testing.T) {
	const (
		repoID  = "019fb141-50f0-79ff-a616-c0935c008ce5" // repository-issued (v7)
		cacheID = "3989bbee-4c1e-4f3d-9166-2117311b1325" // local bundle build (v4)
	)

	t.Run("never overwrites an existing build_id", func(t *testing.T) {
		if localCacheMayAdoptBuildID(repoID, cacheID) {
			t.Fatal("repair path must NOT redefine an existing build_id from the local cache — that is the reinstall/restart loop")
		}
	})

	t.Run("backfills when the record has none", func(t *testing.T) {
		if !localCacheMayAdoptBuildID("", cacheID) {
			t.Fatal("repair path must backfill build_id when the record has none")
		}
	})

	t.Run("no id to adopt is a no-op", func(t *testing.T) {
		if localCacheMayAdoptBuildID("", "") {
			t.Fatal("must not adopt an empty build_id")
		}
	})

	// The settled-check is the other half: a differing id must NOT re-trigger a
	// rewrite every sync tick once the on-disk bytes already match.
	t.Run("differing id is settled, not a repair trigger", func(t *testing.T) {
		if !localCacheRecordSettled("installed", true, repoID, cacheID) {
			t.Fatal("checksum-matched record with an existing build_id must be settled; treating the mismatch as repairable is what looped")
		}
	})

	t.Run("missing id with a manifest id still needs repair", func(t *testing.T) {
		if localCacheRecordSettled("installed", true, "", cacheID) {
			t.Fatal("record lacking build_id must still be repaired (backfilled)")
		}
	})

	t.Run("checksum mismatch is never settled", func(t *testing.T) {
		if localCacheRecordSettled("installed", false, repoID, cacheID) {
			t.Fatal("a record whose on-disk bytes do not match the manifest must not be treated as settled")
		}
	})
}
