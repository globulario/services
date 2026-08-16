package main

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
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

	gz := gzip.NewWriter(f)
	tr := tar.NewWriter(gz)

	data := []byte(manifest)
	if err := tr.WriteHeader(&tar.Header{Name: "package.json", Mode: 0o644, Size: int64(len(data))}); err != nil {
		t.Fatalf("write package header: %v", err)
	}
	if _, err := tr.Write(data); err != nil {
		t.Fatalf("write package manifest: %v", err)
	}
	if err := tr.Close(); err != nil {
		t.Fatalf("close tar writer: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gzip writer: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close package file: %v", err)
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

	// CONTRACT CHANGED 2026-08-13. This case previously asserted the opposite —
	// that the repair path SHOULD backfill build_id when the record has none.
	// That "backfill only" exception was the remaining hole in the same rule the
	// rest of this test enforces, and it produced the second half of the same
	// incident class:
	//
	//	23:49  node-3 records for event / ai-executor / monitoring are written
	//	       from the local cache during etcd instability, each taking its own
	//	       payload-local UUIDv4
	//	       desired (repository, UUIDv7 019ff62a…) had been unchanged in etcd
	//	       since 14:31 — nine hours earlier
	//	→ cluster-doctor release.boundary_unproven, A2=FAILED
	//	  "installed build_id does not match desired build_id"
	//	  against bytes whose checksum matched the manifest exactly
	//
	// An absent build_id is not permission to supply one from a non-authority.
	// Filling it from the cache converts "unknown" into "wrong", and wrong is
	// strictly worse: convergence compares build_id, so a confidently incorrect
	// id reads as drift no reinstall can settle, while an empty one reads as
	// not-yet-proven and resolves on the next authoritative apply.
	//
	// This strengthens rather than weakens the 2026-07-30 protection above:
	// "never adopt" subsumes "never overwrite".
	t.Run("never backfills even when the record has none", func(t *testing.T) {
		if localCacheMayAdoptBuildID("", cacheID) {
			t.Fatal("repair path must NOT supply build_id from the local cache — the cache proves bytes, not identity; an unknown id must stay unknown until an authoritative apply commits one")
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

// TestRepairMustNotMoveTheApplyAnchor pins that the local-package-cache REPAIR
// path never rewrites UpdatedUnix.
//
// UpdatedUnix is the apply anchor: cluster-doctor reads it as last_apply_time
// (apply_time_source=installed_package.updated_unix) and reports
// service.old_pid_after_upgrade for any process that started before it. The
// repair path only ever touches records that already exist (it iterates
// ListInstalledPackages) and installs nothing, so moving the anchor there is
// forbidden_fix:bump_immutable_timestamp_on_observe.
//
// It manufactured exactly that false positive on a clean Day-0 2026-07-30:
// etcd and persistence were both flagged old_pid_after_upgrade while the
// finding's own evidence said running_matches_installed=true — correct
// binaries, but a repair write had moved the anchor to 13:23:42 while the
// processes had legitimately started at 13:18:52.
func TestRepairMustNotMoveTheApplyAnchor(t *testing.T) {
	src, err := os.ReadFile("heartbeat.go")
	if err != nil {
		t.Fatalf("read heartbeat.go: %v", err)
	}
	const fn = "func (srv *NodeAgentServer) repairInstalledStateFromLocalPackageCache"
	i := strings.Index(string(src), fn)
	if i < 0 {
		t.Fatalf("repair function not found — rename? update this test")
	}
	// Bound the scan to this function: next top-level func declaration.
	rest := string(src)[i+len(fn):]
	if j := strings.Index(rest, "\nfunc "); j >= 0 {
		rest = rest[:j]
	}
	if strings.Contains(rest, "UpdatedUnix = now") {
		t.Fatal("repair path must NOT bump UpdatedUnix — it is the apply anchor doctor reads as last_apply_time, and moving it on a metadata-only refresh fabricates service.old_pid_after_upgrade")
	}
}

// TestLocalPackageCacheCreateMustNotStampCacheIdentity covers the CREATE half of
// the same authority rule. localCacheMayAdoptBuildID guards the repair path that
// updates existing records; syncInstalledStateFromLocalPackageCache creates
// records that do not exist yet, and it used to stamp BuildId: proof.BuildID
// directly — bypassing the guard entirely.
//
// That is how node-4's ldap and node-3's event acquired payload-local UUIDv4
// identities on 2026-08-12: an etcd read/write failure made the record look
// absent, the create path ran, and the cache became the identity authority.
//
// The record this path writes declares Status=artifact_present precisely because
// it is NOT claiming a committed install. An identity it cannot prove belongs
// with that: unknown, not borrowed.
func TestLocalPackageCacheCreateMustNotStampCacheIdentity(t *testing.T) {
	const cacheID = "62e09998-5d87-4ae6-8935-29efac0a600c" // observed on node-4/ldap

	// The create path builds its record from a localPackageCacheProof. Assert the
	// proof's build id never reaches the record's identity field by construction:
	// any code that sets BuildId from the proof reintroduces the defect.
	proof := localPackageCacheProof{
		Name:    "ldap",
		Version: "1.2.312",
		Kind:    "SERVICE",
		BuildID: cacheID,
	}
	if localCacheMayAdoptBuildID("", proof.BuildID) {
		t.Fatal("cache-sourced build id must never be adoptable, on create or repair")
	}
}

// TestLocalPackageCacheReceiptShapeIsNotClaimedFromCache pins the provenance
// half. The incomplete receipt observed on node-4/ldap was exactly:
//
//	metadata = [entrypoint_checksum, entrypoint_checksum_disk_observed]
//
// with no unit_file_sha256 and no installed_by, versus the canonical 7-key
// receipt carried by `file` on the same node. checkUnitHashDrift treats that
// shape as installed_state_missing_or_unproven and fails closed, which is
// correct — the fix is that the cache path must not be the writer that produces
// it, not that the doctor should accept it.
//
// This asserts the fail-closed reading is preserved: a receipt lacking
// unit-file provenance must NOT be treated as authoritative.
func TestLocalPackageCacheReceiptShapeIsNotClaimedFromCache(t *testing.T) {
	partial := &node_agentpb.InstalledPackage{
		Name: "ldap", Kind: "SERVICE",
		Metadata: map[string]string{
			"entrypoint_checksum":               "ab01a676",
			"entrypoint_checksum_disk_observed": "ab01a676",
		},
	}
	if got := receiptUnitFileSha256(partial); got != "" {
		t.Fatalf("receiptUnitFileSha256 = %q, want empty — a cache-written receipt carries no unit-file authority", got)
	}

	// The canonical shape, for contrast: `file` on the same node carried
	// unit_file_sha256 and installed_by. Only that shape is authoritative.
	canonical := &node_agentpb.InstalledPackage{
		Name: "file", Kind: "SERVICE",
		Metadata: map[string]string{
			"entrypoint_checksum": "aa",
			"unit_file_sha256":    "bb",
			"installed_by":        "node-agent.apply_package_release",
		},
	}
	if receiptUnitFileSha256(canonical) == "" {
		t.Fatal("canonical receipt must expose unit-file authority — if this fails the predicate itself is broken, not the fixture")
	}
}
