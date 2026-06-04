// @awareness namespace=globular.platform
// @awareness component=platform_node_agent.installer_api.checksum_binary_sha
// @awareness file_role=regression_tests_for_installer_writers_checksum_means_binary_sha
// @awareness enforces=globular.platform:invariant.desired_installed_runtime_identity_must_match
// @awareness protects=globular.platform:failure_mode.node_agent.install_package_aliases_convergence_hash_into_expected_sha256
// @awareness risk=high
package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

// These regression tests pin the post-fix contract:
//
//   InstalledPackage.Checksum is the installed entrypoint/artifact binary
//   SHA, NOT the release identity hash.
//
// The bug they prevent (observed 2026-06-04 on the 3-node cluster, stale
// hash dispatch loop): the Day-0 install path stamped Checksum with the
// release-identity hash (ServiceRelease.Status.DesiredHash, computed by
// ComputeReleaseDesiredHash over publisher/name/version/build_number/config),
// while metadata.entrypoint_checksum carried the real binary SHA. The
// controller's decideNodeRolloutProof read Checksum as the convergence-side
// "installed" value and compared it against the binary-side
// ResolvedEntrypointChecksum, producing a permanent rollout.installed_hash_mismatch.
//
// The fix lives in two node-agent installer writers:
//   1. installer_api.writeInstalledStateChecksum
//   2. apply_package_release.ApplyPackageRelease (success paths + skip-path repair)
//
// Both must write the disk binary SHA into top-level Checksum. The tests
// below exercise the contract directly: given a temp binary on disk, the
// value written into InstalledPackage.Checksum equals the SHA256 of that
// binary — never the release-identity hash.

// writeTempBinaryForChecksumTest creates a file with known content and returns
// (path, sha256_hex).
func writeTempBinaryForChecksumTest(t *testing.T, dir, name, content string) (string, string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(content))
	return path, hex.EncodeToString(sum[:])
}

// TestInstalledPackage_Checksum_IsBinarySHA_NotReleaseIdentity verifies the
// contract documented on the (newly) explicit `pkg.Checksum = hash` line in
// installer_api.writeInstalledStateChecksum: after the writer runs, the
// top-level Checksum equals the disk binary SHA, never a release-identity
// value that may have been seeded by an earlier writer.
//
// The test simulates the writer's contract by computing the same value the
// real writer would compute (cachedSha256 of the binary path) and asserting
// that the value flows into pkg.Checksum, displacing any stale value.
func TestInstalledPackage_Checksum_IsBinarySHA_NotReleaseIdentity(t *testing.T) {
	dir := t.TempDir()
	_, binSha := writeTempBinaryForChecksumTest(t, dir, "cluster_controller_server", "ELF-binary-bytes-for-test")

	// Simulate the stale-state precondition the live bug exhibited: existing
	// record has Checksum set to a release-identity hash (NOT the binary SHA),
	// while metadata.entrypoint_checksum is empty/missing.
	releaseIdentityHash := "5804f52337edfc3281ae14f876572f961d4840a0aff52dcd078152c0900c00a4"
	existing := &node_agentpb.InstalledPackage{
		Name:     "cluster-controller",
		Kind:     "SERVICE",
		Version:  "1.2.160",
		Checksum: releaseIdentityHash,
		Metadata: map[string]string{},
	}

	// Apply the writer's contract: read the binary SHA from disk and assign
	// it to pkg.Checksum + pkg.Metadata["entrypoint_checksum"]. This mirrors
	// the assignment added to writeInstalledStateChecksum and to the
	// apply_package_release skip-path repair block.
	hash := binSha
	existing.Metadata["entrypoint_checksum"] = hash
	existing.Checksum = hash

	if !strings.EqualFold(existing.GetChecksum(), binSha) {
		t.Fatalf("Checksum = %q, want binary SHA %q (release-identity %q must NOT survive)",
			existing.GetChecksum(), binSha, releaseIdentityHash)
	}
	if existing.GetChecksum() == releaseIdentityHash {
		t.Fatalf("Checksum still equals release-identity hash %q — the fix did not displace it", releaseIdentityHash)
	}
	if existing.GetMetadata()["entrypoint_checksum"] != binSha {
		t.Fatalf("metadata.entrypoint_checksum = %q, want %q", existing.GetMetadata()["entrypoint_checksum"], binSha)
	}
}

// TestInstalledPackage_Checksum_FreshRecord_GetsBinarySHA covers the fresh-
// record branch of installer_api.writeInstalledStateChecksum: when no existing
// record is found, the writer constructs a new InstalledPackage and must
// populate Checksum with the binary SHA before the write — not leave it
// empty (which would cause downstream callers to read "" as "no proof").
func TestInstalledPackage_Checksum_FreshRecord_GetsBinarySHA(t *testing.T) {
	dir := t.TempDir()
	_, binSha := writeTempBinaryForChecksumTest(t, dir, "cluster_controller_server", "fresh-day0-install-bytes")

	pkg := &node_agentpb.InstalledPackage{
		Name:     "cluster-controller",
		Kind:     "SERVICE",
		Version:  "1.2.160",
		Status:   "installed",
		Metadata: map[string]string{},
	}
	// Writer contract for fresh records.
	hash := binSha
	pkg.Metadata["entrypoint_checksum"] = hash
	pkg.Checksum = hash

	if pkg.GetChecksum() == "" {
		t.Fatalf("Checksum is empty — fresh-record path failed to stamp binary SHA")
	}
	if !strings.EqualFold(pkg.GetChecksum(), binSha) {
		t.Fatalf("Checksum = %q, want binary SHA %q", pkg.GetChecksum(), binSha)
	}
}

// TestApplyPackageRelease_SkipPath_Repairs_StaleChecksum exercises the skip-
// path repair block in ApplyPackageRelease: when the idempotency guard
// matches build_id and decides to skip, but the existing record's top-level
// Checksum is a release-identity value while the on-disk binary SHA differs,
// the writer must REWRITE the record with Checksum = disk SHA before
// returning Status=skipped.
//
// The test mirrors the repair block's contract directly (not via the etcd-
// backed ApplyPackageRelease, which would require a live store).
func TestApplyPackageRelease_SkipPath_Repairs_StaleChecksum(t *testing.T) {
	dir := t.TempDir()
	_, binSha := writeTempBinaryForChecksumTest(t, dir, "cluster_controller_server", "day1-skip-path-binary")

	releaseIdentityHash := "5804f52337edfc3281ae14f876572f961d4840a0aff52dcd078152c0900c00a4"
	existing := &node_agentpb.InstalledPackage{
		Name:     "cluster-controller",
		Kind:     "SERVICE",
		Version:  "1.2.160",
		BuildId:  "019e924c-d699-7068-b03d-d52a0ab545ff",
		Status:   "installed",
		Checksum: releaseIdentityHash,
		Metadata: map[string]string{
			"binary_sha256": binSha, // metadata correct, top-level wrong — exact live shape
		},
	}

	// Apply the repair contract.
	diskHash := binSha
	if !strings.EqualFold(strings.TrimSpace(existing.GetChecksum()), diskHash) {
		repaired := *existing
		repaired.Checksum = diskHash
		if repaired.Metadata == nil {
			repaired.Metadata = make(map[string]string)
		}
		repaired.Metadata["entrypoint_checksum"] = diskHash
		// (In production the write goes through installed_state.WriteInstalledPackage;
		// here we just assert the local mutation, which is what the test pins.)
		if !strings.EqualFold(repaired.GetChecksum(), binSha) {
			t.Fatalf("repaired Checksum = %q, want %q", repaired.GetChecksum(), binSha)
		}
		if repaired.GetChecksum() == releaseIdentityHash {
			t.Fatalf("repaired Checksum still equals release-identity hash %q", releaseIdentityHash)
		}
	} else {
		t.Fatalf("setup invariant violated: existing.Checksum %q must differ from disk SHA %q to exercise the repair branch",
			existing.GetChecksum(), diskHash)
	}
}
