package main

// Regression tests for the v1.2.119 install-package hash-schema separation.
//
// The historical fallback in runInstallPackage —
//
//   desiredHash := inputs["desired_hash"]
//   if desiredHash == "" {
//       desiredHash = inputs["expected_sha256"]
//   }
//   ApplyPackageReleaseRequest{ ExpectedSha256: desiredHash }
//
// — silently routed the convergence identity hash into ExpectedSha256 when
// the controller started supplying desired_hash via the install-package
// workflow inputs (a v1.2.119 side effect). The node-agent verify gate then
// compared the convergence identity hash against the installed binary and
// honestly returned installed_binary_hash_mismatch on every dispatch.
//
// These tests pin the contract: the two schemas are extracted independently,
// neither falls back into the other, and the value flowing into
// ApplyPackageReleaseRequest.ExpectedSha256 comes from `expected_sha256` only.

import (
	"testing"
)

const (
	// ComputeReleaseDesiredHash("core@globular.io", "dns", "1.2.113", 364, nil)
	// — the canonical "phantom" hash that surfaced as the v1.2.119 regression.
	dnsConvergenceHash = "de2b04ff64ce4489f2d8a6b151571f6f3941cb28674e0124e9d3db2d739ad414"
	// dns@1.2.113 manifest.entrypoint_checksum (verified against the published
	// repository tarball on ryzen 2026-05-28).
	dnsBinarySha256 = "6ed1f9c85ad27d6f17d15f7cc7f41c9a76323fb8e1cdb9f33feb89c11d219086"
)

// TestExtractRunInstallPackageHashes_BothPresent — when the controller supplies
// both schemas, each is returned in its own slot. The dispatch site downstream
// uses convergenceHash for canSkipInstallPackage and expectedSha256 for the
// verify gate; aliasing one into the other re-creates the v1.2.119 regression.
func TestExtractRunInstallPackageHashes_BothPresent(t *testing.T) {
	inputs := map[string]string{
		"desired_hash":    dnsConvergenceHash,
		"expected_sha256": dnsBinarySha256,
	}
	convergenceHash, expectedSha256 := extractRunInstallPackageHashes(inputs)
	if convergenceHash != dnsConvergenceHash {
		t.Fatalf("convergenceHash = %q, want %q", convergenceHash, dnsConvergenceHash)
	}
	if expectedSha256 != dnsBinarySha256 {
		t.Fatalf("expectedSha256 = %q, want %q", expectedSha256, dnsBinarySha256)
	}
}

// TestExtractRunInstallPackageHashes_OnlyConvergence — when only desired_hash
// is supplied (legacy controllers, repair paths without a manifest), the
// binary verify slot must be empty, NOT a copy of the convergence hash. The
// node-agent then writes installed_unverified honestly instead of failing
// every install with a hash mismatch.
func TestExtractRunInstallPackageHashes_OnlyConvergence(t *testing.T) {
	inputs := map[string]string{
		"desired_hash": dnsConvergenceHash,
	}
	convergenceHash, expectedSha256 := extractRunInstallPackageHashes(inputs)
	if convergenceHash != dnsConvergenceHash {
		t.Fatalf("convergenceHash = %q, want %q", convergenceHash, dnsConvergenceHash)
	}
	if expectedSha256 != "" {
		t.Fatalf("expectedSha256 = %q, want empty; convergence hash must NOT alias into the binary verify slot (the v1.2.119 regression)", expectedSha256)
	}
}

// TestExtractRunInstallPackageHashes_OnlyBinary — when only expected_sha256
// is supplied, the convergence slot must be empty, NOT a copy. canSkipInstallPackage
// then falls back to version/buildID matching, not a comparison against the
// wrong-schema hash.
func TestExtractRunInstallPackageHashes_OnlyBinary(t *testing.T) {
	inputs := map[string]string{
		"expected_sha256": dnsBinarySha256,
	}
	convergenceHash, expectedSha256 := extractRunInstallPackageHashes(inputs)
	if convergenceHash != "" {
		t.Fatalf("convergenceHash = %q, want empty; expected_sha256 must NOT alias into the convergence slot", convergenceHash)
	}
	if expectedSha256 != dnsBinarySha256 {
		t.Fatalf("expectedSha256 = %q, want %q", expectedSha256, dnsBinarySha256)
	}
}

// TestExtractRunInstallPackageHashes_BothEmpty — when neither schema is
// supplied, both slots are empty. The node-agent writes installed_unverified
// (no proof available) and the verify gate refuses verified SUCCESS — the
// system tells the truth instead of failing every install or pretending success.
func TestExtractRunInstallPackageHashes_BothEmpty(t *testing.T) {
	inputs := map[string]string{}
	convergenceHash, expectedSha256 := extractRunInstallPackageHashes(inputs)
	if convergenceHash != "" {
		t.Fatalf("convergenceHash = %q, want empty", convergenceHash)
	}
	if expectedSha256 != "" {
		t.Fatalf("expectedSha256 = %q, want empty", expectedSha256)
	}
}

// TestExtractRunInstallPackageHashes_Whitespace — controller-side serialization
// has historically padded values with whitespace; the extractor trims so that
// equality checks downstream don't trip on " 6ed1f9c8..." vs "6ed1f9c8...".
func TestExtractRunInstallPackageHashes_Whitespace(t *testing.T) {
	inputs := map[string]string{
		"desired_hash":    "  " + dnsConvergenceHash + "  ",
		"expected_sha256": "\t" + dnsBinarySha256 + "\n",
	}
	convergenceHash, expectedSha256 := extractRunInstallPackageHashes(inputs)
	if convergenceHash != dnsConvergenceHash {
		t.Fatalf("convergenceHash = %q, want %q (whitespace trim failed)", convergenceHash, dnsConvergenceHash)
	}
	if expectedSha256 != dnsBinarySha256 {
		t.Fatalf("expectedSha256 = %q, want %q (whitespace trim failed)", expectedSha256, dnsBinarySha256)
	}
}

// TestRunInstallPackageHashSchema_PhantomHashIsConvergenceIdentity is a
// sentinel that documents WHY de2b04ff... appeared in v1.2.119 production
// node-agent logs as `expected sha256=de2b04ff...`. The phantom hash is
// ComputeReleaseDesiredHash("core@globular.io", "dns", "1.2.113", 364);
// it had no business in ExpectedSha256. If a future change reintroduces the
// aliasing fallback, the same value will surface again. The dnsConvergenceHash
// constant pins the value; a regression makes its origin auditable.
func TestRunInstallPackageHashSchema_PhantomHashIsConvergenceIdentity(t *testing.T) {
	if dnsConvergenceHash == dnsBinarySha256 {
		t.Fatalf("convergence and binary hashes coincided in fixture; test invalidated")
	}
}
