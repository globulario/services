package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// withBinDir temporarily overrides the package-level globularBinDir so tests
// can place fake binaries without touching /usr/lib/globular/bin.
func withBinDir(t *testing.T, dir string) {
	t.Helper()
	original := globularBinDir
	globularBinDir = dir
	t.Cleanup(func() {
		globularBinDir = original
	})
	// Cached sha256 entries are keyed by path; clear after the test so a
	// reused path in a later test doesn't see the previous test's hash.
	t.Cleanup(invalidateSha256Cache)
}

// placeBinary writes content at globularBinDir/<filename> with exec perms.
// Returns the lowercase hex sha256 of the written content.
func placeBinary(t *testing.T, filename string, content []byte) string {
	t.Helper()
	path := filepath.Join(globularBinDir, filename)
	if err := os.MkdirAll(globularBinDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, content, 0o755); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	sum := sha256.Sum256(content)
	return hex.EncodeToString(sum[:])
}

// TestVerifyStrict_RunningServiceMatchingChecksum_ReturnsVerified — happy path.
func TestVerifyStrict_RunningServiceMatchingChecksum_ReturnsVerified(t *testing.T) {
	dir := t.TempDir()
	withBinDir(t, dir)
	hash := placeBinary(t, "repository_server", []byte("real-v1.2.116-bytes"))

	actual, verdict, err := verifyInstalledBinaryHashStrict("repository", "SERVICE", hash, "build-1", "op-1", "repository_server")
	if err != nil {
		t.Fatalf("unexpected err: %v", err)
	}
	if verdict != BinaryVerified {
		t.Errorf("verdict = %q, want %q", verdict, BinaryVerified)
	}
	if actual != hash {
		t.Errorf("actual hash mismatch")
	}
}

// TestVerifyStrict_RunningServiceMismatchingChecksum_ReturnsMismatch — the
// regression scenario: binary on disk is v1.2.110 (real content) but the
// release pipeline expected v1.2.115's hash. Pre-fix path declared success;
// new strict path returns BinaryMismatch + error.
func TestVerifyStrict_RunningServiceMismatchingChecksum_ReturnsMismatch(t *testing.T) {
	dir := t.TempDir()
	withBinDir(t, dir)
	_ = placeBinary(t, "repository_server", []byte("v1.2.110-content"))
	wrongExpected := strings.Repeat("a", 64) // 64 hex chars, but won't match the file's hash

	actual, verdict, err := verifyInstalledBinaryHashStrict("repository", "SERVICE", wrongExpected, "build-1", "op-1", "repository_server")
	if err == nil {
		t.Fatalf("expected error for hash mismatch")
	}
	if verdict != BinaryMismatch {
		t.Errorf("verdict = %q, want %q", verdict, BinaryMismatch)
	}
	if actual == "" {
		t.Errorf("actual hash should be populated for evidence even on mismatch")
	}
	var hashErr *BinaryHashMismatchError
	if !errorsAs(err, &hashErr) {
		t.Fatalf("error type = %T, want *BinaryHashMismatchError", err)
	}
}

// TestVerifyStrict_MissingBinary_ReturnsMissing — binary not on disk + expected
// provided. Must return BinaryMissing + error, never SUCCESS.
func TestVerifyStrict_MissingBinary_ReturnsMissing(t *testing.T) {
	dir := t.TempDir()
	withBinDir(t, dir)
	// No binary placed at all.
	expected := strings.Repeat("b", 64)

	_, verdict, err := verifyInstalledBinaryHashStrict("ghost", "SERVICE", expected, "build-g", "op-g", "ghost_server")
	if err == nil {
		t.Fatalf("expected error for missing binary")
	}
	if verdict != BinaryMissing {
		t.Errorf("verdict = %q, want %q", verdict, BinaryMissing)
	}
	var missing *BinaryMissingError
	if !errorsAs(err, &missing) {
		t.Fatalf("error type = %T, want *BinaryMissingError", err)
	}
}

// TestVerifyStrict_MissingExpectedChecksum_ReturnsUnverified — the load-bearing
// behavior change. Before fix: empty expected → (hash, nil) treated as success.
// After fix: empty expected → (hash, BinaryUnverified, nil) — caller must
// route to UNVERIFIED installed-state, not SUCCESS.
func TestVerifyStrict_MissingExpectedChecksum_ReturnsUnverified(t *testing.T) {
	dir := t.TempDir()
	withBinDir(t, dir)
	_ = placeBinary(t, "legacy_server", []byte("some-bytes"))

	actual, verdict, err := verifyInstalledBinaryHashStrict("legacy", "SERVICE", "", "", "", "legacy_server")
	if err != nil {
		t.Fatalf("unverified path must not return an error: %v", err)
	}
	if verdict != BinaryUnverified {
		t.Errorf("verdict = %q, want %q — missing expected_sha256 must be UNVERIFIED, not VERIFIED",
			verdict, BinaryUnverified)
	}
	if actual == "" {
		t.Errorf("hash should be computed even when expected is missing (for evidence)")
	}
}

// TestVerifyStrict_OldBinaryMustNotSatisfyNewExpectedHash — exact regression
// from the v1.2.115 install incident. The local cache held a v1.2.110 binary
// whose hash is X. The release manifest expected v1.2.115's hash Y. Strict
// verify must reject X≠Y as MISMATCH, NOT declare success.
func TestVerifyStrict_OldBinaryMustNotSatisfyNewExpectedHash(t *testing.T) {
	dir := t.TempDir()
	withBinDir(t, dir)
	oldHash := placeBinary(t, "repository_server", []byte("v1.2.110-binary-content-from-stale-local-cache"))
	// The new release (v1.2.116) has a different binary with a different hash.
	newContent := []byte("v1.2.116-binary-with-skip-predicate-fix-and-materialize-step")
	newHashSum := sha256.Sum256(newContent)
	newExpected := hex.EncodeToString(newHashSum[:])
	if oldHash == newExpected {
		t.Fatal("test setup error: old and new content hashed identically")
	}

	_, verdict, err := verifyInstalledBinaryHashStrict("repository", "SERVICE", newExpected, "build-new", "op-upgrade", "repository_server")
	if err == nil {
		t.Fatalf("expected error: old v1.2.110 binary must not satisfy v1.2.116 expected hash")
	}
	if verdict != BinaryMismatch {
		t.Errorf("verdict = %q, want %q", verdict, BinaryMismatch)
	}
}

// TestVerifyStrict_NoEntrypointDeclared_ReturnsNotApplicable — a package that
// explicitly declares entrypoint: none (keepalived, scylladb, …) has no binary
// to hash-verify. The gate is NOT APPLICABLE → clean SUCCESS, no error, even
// with no binary on disk and no expected checksum. This is the noop-sentinel
// replacement: the package no longer ships a fake binary to satisfy the gate.
func TestVerifyStrict_NoEntrypointDeclared_ReturnsNotApplicable(t *testing.T) {
	dir := t.TempDir()
	withBinDir(t, dir)
	// No binary placed — a binary-less wrapper package.

	actual, verdict, err := verifyInstalledBinaryHashStrict("keepalived", "INFRASTRUCTURE", "", "build-k", "op-k", "none")
	if err != nil {
		t.Fatalf("no-entrypoint package must not error: %v", err)
	}
	if verdict != BinaryNotApplicable {
		t.Errorf("verdict = %q, want %q — entrypoint:none must be NOT_APPLICABLE", verdict, BinaryNotApplicable)
	}
	if actual != "" {
		t.Errorf("no-entrypoint package must have empty actual hash, got %q", actual)
	}
}

// TestVerifyStrict_NoEntrypoint_ShortCircuitsExpectedAndMissingBinary — the
// explicit entrypoint:none declaration is trusted and wins even if an expected
// checksum was somehow supplied and the binary is absent. none is the authority,
// not a fallback that a present-expected could override into a REJECT.
func TestVerifyStrict_NoEntrypoint_ShortCircuitsExpectedAndMissingBinary(t *testing.T) {
	dir := t.TempDir()
	withBinDir(t, dir)
	expected := strings.Repeat("c", 64)

	_, verdict, err := verifyInstalledBinaryHashStrict("scylladb", "INFRASTRUCTURE", expected, "build-s", "op-s", "NONE")
	if err != nil {
		t.Fatalf("entrypoint:none must short-circuit, not REJECT: %v", err)
	}
	if verdict != BinaryNotApplicable {
		t.Errorf("verdict = %q, want %q — case-insensitive 'NONE' must be NOT_APPLICABLE", verdict, BinaryNotApplicable)
	}
}

// TestVerifyStrict_EmptyEntrypoint_StaysUnverified — the contract boundary: an
// EMPTY declared entrypoint is NOT the same as "none". It is the ambiguous
// legacy/degraded case and must remain UNVERIFIED, never silently upgraded to
// NOT_APPLICABLE. This is what stops a real, entrypoint-bearing service from
// skipping proof just because its sidecar/checksum was absent.
func TestVerifyStrict_EmptyEntrypoint_StaysUnverified(t *testing.T) {
	dir := t.TempDir()
	withBinDir(t, dir)
	_ = placeBinary(t, "legacy_server", []byte("legacy-bytes"))

	_, verdict, err := verifyInstalledBinaryHashStrict("legacy", "SERVICE", "", "", "", "")
	if err != nil {
		t.Fatalf("empty-entrypoint unverified path must not error: %v", err)
	}
	if verdict != BinaryUnverified {
		t.Errorf("verdict = %q, want %q — empty entrypoint must stay UNVERIFIED, not NOT_APPLICABLE",
			verdict, BinaryUnverified)
	}
}

// errorsAs is a thin wrapper so the test can use errors.As without importing
// the stdlib errors package directly into every test.
func errorsAs(err error, target any) bool {
	switch t := target.(type) {
	case **BinaryHashMismatchError:
		hm, ok := err.(*BinaryHashMismatchError)
		if ok {
			*t = hm
			return true
		}
		return false
	case **BinaryMissingError:
		mm, ok := err.(*BinaryMissingError)
		if ok {
			*t = mm
			return true
		}
		return false
	default:
		return false
	}
}

// invalidateSha256Cache best-effort flushes the in-memory hash cache so a
// path reused across tests doesn't see a stale hash from a prior test's
// content. Implemented as a no-op stub when the cache isn't exposed.
var invalidateSha256Cache = func() {}
