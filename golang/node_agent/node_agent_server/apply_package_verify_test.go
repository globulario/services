package main

// apply_package_verify_test.go — Phase 1 (Diagnostic Honesty Refactor)
//
// Pins the contract of verifyInstalledBinaryHash and its two failure types.
// The Prime Directive: a package is not "installed" until the bytes on disk
// at the deployed binary path hash to the published artifact-manifest digest.
//
// Each test uses a unique package name so each maps to a unique path under
// the per-test globularBinDir override — this dodges cachedSha256's
// (path, modTime, size) cache and keeps tests independent.

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeBinary creates a fake installed binary in the test's bin dir and
// returns its sha256 (lowercase hex, no prefix).
func writeBinary(t *testing.T, dir, name string, payload []byte) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir bin dir: %v", err)
	}
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, payload, 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

// withTempBinDir overrides the package-global globularBinDir for the duration
// of a single test and restores it on cleanup.
func withTempBinDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	old := globularBinDir
	globularBinDir = dir
	t.Cleanup(func() { globularBinDir = old })
	return dir
}

// ─────────────────────────────────────────────────────────────────────────
// Happy paths
// ─────────────────────────────────────────────────────────────────────────

func TestVerifyInstalledBinaryHash_MatchReturnsActualNoError(t *testing.T) {
	dir := withTempBinDir(t)
	hash := writeBinary(t, dir, "match_server", []byte("phase1-match-payload"))

	got, err := verifyInstalledBinaryHash("match", "SERVICE", hash, "build-abc", "op-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != hash {
		t.Fatalf("hash mismatch: got=%s want=%s", got, hash)
	}
}

func TestVerifyInstalledBinaryHash_NormalizesSha256Prefix(t *testing.T) {
	dir := withTempBinDir(t)
	hash := writeBinary(t, dir, "prefix_server", []byte("phase1-prefix-payload"))

	// The repository historically stores hashes as "sha256:<hex>" while the
	// local computation returns plain hex. The gate must compare format-
	// agnostically — proving this prevents a class of false-mismatch alarms.
	got, err := verifyInstalledBinaryHash("prefix", "SERVICE", "SHA256:"+strings.ToUpper(hash), "build-x", "op-x")
	if err != nil {
		t.Fatalf("prefix-normalized expected match, got err: %v", err)
	}
	if got != hash {
		t.Fatalf("hash mismatch: got=%s want=%s", got, hash)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Drift — the headline Phase 1 failure mode: disk binary differs from
// the artifact the controller asked us to install. The apply MUST fail
// rather than write "installed" with a wrong hash.
// ─────────────────────────────────────────────────────────────────────────

func TestVerifyInstalledBinaryHash_MismatchReturnsTypedError(t *testing.T) {
	dir := withTempBinDir(t)
	actualHash := writeBinary(t, dir, "drift_server", []byte("phase1-drift-actual"))
	// Build a wrong expected hash by sha256'ing different bytes.
	wrong := sha256.Sum256([]byte("phase1-drift-EXPECTED-but-wrong"))
	expected := hex.EncodeToString(wrong[:])

	got, err := verifyInstalledBinaryHash("drift", "SERVICE", expected, "build-7", "op-drift")
	if err == nil {
		t.Fatal("expected BinaryHashMismatchError, got nil")
	}
	var mismatch *BinaryHashMismatchError
	if !errors.As(err, &mismatch) {
		t.Fatalf("expected *BinaryHashMismatchError, got %T: %v", err, err)
	}
	if mismatch.Expected != expected {
		t.Errorf("evidence.Expected=%s want=%s", mismatch.Expected, expected)
	}
	if mismatch.Actual != actualHash {
		t.Errorf("evidence.Actual=%s want=%s", mismatch.Actual, actualHash)
	}
	if mismatch.BuildID != "build-7" || mismatch.OperationID != "op-drift" {
		t.Errorf("identity fields not propagated: build_id=%s operation_id=%s",
			mismatch.BuildID, mismatch.OperationID)
	}
	if !strings.Contains(mismatch.Path, "drift_server") {
		t.Errorf("evidence.Path missing binary name: %s", mismatch.Path)
	}
	// The actual hash is also returned to the caller (so installed_state can
	// record what was on disk even when the apply is rejected).
	if got != actualHash {
		t.Errorf("returned hash=%s want=%s", got, actualHash)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Missing binary with proof requested = critical install failure. Pre-fix,
// this path silently wrote installed=true with checksum=expected (the claim),
// even though no binary was on disk.
// ─────────────────────────────────────────────────────────────────────────

func TestVerifyInstalledBinaryHash_MissingBinaryWithExpectedReturnsTypedError(t *testing.T) {
	_ = withTempBinDir(t) // empty bin dir — no binary at the expected path
	expected := strings.Repeat("a", 64)

	got, err := verifyInstalledBinaryHash("ghost", "SERVICE", expected, "build-g", "op-g")
	if err == nil {
		t.Fatal("expected BinaryMissingError, got nil")
	}
	var missing *BinaryMissingError
	if !errors.As(err, &missing) {
		t.Fatalf("expected *BinaryMissingError, got %T: %v", err, err)
	}
	if missing.Expected != expected {
		t.Errorf("evidence.Expected=%s want=%s", missing.Expected, expected)
	}
	if missing.BuildID != "build-g" || missing.OperationID != "op-g" {
		t.Errorf("identity fields not propagated: build_id=%s operation_id=%s",
			missing.BuildID, missing.OperationID)
	}
	if got != "" {
		t.Errorf("returned hash should be empty on missing binary, got %q", got)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Unverified path. Legacy callers that don't yet propagate expected_sha256
// must not break — the gate degrades gracefully: no error, no false claim.
// ─────────────────────────────────────────────────────────────────────────

func TestVerifyInstalledBinaryHash_UnverifiedWithBinaryReturnsHashNoError(t *testing.T) {
	dir := withTempBinDir(t)
	hash := writeBinary(t, dir, "unverified_server", []byte("phase1-unverified-payload"))

	got, err := verifyInstalledBinaryHash("unverified", "SERVICE", "", "", "")
	if err != nil {
		t.Fatalf("unverified path must not error, got: %v", err)
	}
	if got != hash {
		t.Errorf("returned hash=%s want=%s", got, hash)
	}
}

func TestVerifyInstalledBinaryHash_UnverifiedMissingBinaryReturnsEmptyNoError(t *testing.T) {
	_ = withTempBinDir(t)

	got, err := verifyInstalledBinaryHash("absent", "SERVICE", "", "", "")
	if err != nil {
		t.Fatalf("unverified path must not error on missing binary, got: %v", err)
	}
	if got != "" {
		t.Errorf("returned hash=%q want empty (no proof and no binary)", got)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Evidence map pinning. Doctor / the verifier lift these keys into the
// package.installed_binary_hash_mismatch finding evidence section. Renaming
// any of them silently breaks downstream consumers.
// ─────────────────────────────────────────────────────────────────────────

func TestBinaryHashMismatchError_EvidenceMapKeys(t *testing.T) {
	e := &BinaryHashMismatchError{
		Package: "dns", Kind: "SERVICE",
		Path:     "/usr/lib/globular/bin/dns_server",
		Expected: "AAAA", Actual: "BBBB",
		BuildID: "build-1", OperationID: "op-1",
	}
	ev := e.EvidenceMap()
	wantKeys := []string{
		"error", "finding", "installed_path",
		"expected_sha256", "actual_sha256",
		"expected_build_id", "apply_run_id",
	}
	for _, k := range wantKeys {
		if _, ok := ev[k]; !ok {
			t.Errorf("evidence map missing key %q (consumed by doctor lift)", k)
		}
	}
	if ev["finding"] != "package.installed_binary_hash_mismatch" {
		t.Errorf("finding id=%q want package.installed_binary_hash_mismatch", ev["finding"])
	}
	if ev["expected_sha256"] != "AAAA" || ev["actual_sha256"] != "BBBB" {
		t.Errorf("expected/actual not propagated into evidence: %#v", ev)
	}
}

func TestBinaryMissingError_EvidenceMapKeys(t *testing.T) {
	e := &BinaryMissingError{
		Package: "dns", Kind: "SERVICE",
		Path:     "/usr/lib/globular/bin/dns_server",
		Expected: "AAAA",
		BuildID:  "build-1", OperationID: "op-1",
		Underlying: os.ErrNotExist,
	}
	ev := e.EvidenceMap()
	if ev["finding"] != "package.installed_binary_missing" {
		t.Errorf("finding id=%q want package.installed_binary_missing", ev["finding"])
	}
	for _, k := range []string{"error", "installed_path", "expected_sha256", "expected_build_id", "apply_run_id"} {
		if _, ok := ev[k]; !ok {
			t.Errorf("evidence map missing key %q", k)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────
// Status string pinning — doctor / the verifier match on this exact value
// to lift the apply failure into a finding. If this constant changes, those
// consumers must change with it.
// ─────────────────────────────────────────────────────────────────────────

func TestStatusBinaryHashMismatch_ConstantContract(t *testing.T) {
	if StatusBinaryHashMismatch != "failed_binary_hash_mismatch" {
		t.Errorf("StatusBinaryHashMismatch=%q; downstream doctor rule keys off this exact string",
			StatusBinaryHashMismatch)
	}
}

// ─────────────────────────────────────────────────────────────────────────
// COMMAND/SERVICE binary path resolution — verifyInstalledBinaryHash uses
// installedBinaryPath, so confirm the SERVICE → "<name>_server" and
// INFRASTRUCTURE/COMMAND → "<name>" mapping is what the gate observes.
// ─────────────────────────────────────────────────────────────────────────

func TestVerifyInstalledBinaryHash_KindMapsToCorrectPath(t *testing.T) {
	dir := withTempBinDir(t)

	// SERVICE → <name>_server. Write the SERVICE binary only; the gate
	// for an INFRASTRUCTURE/COMMAND name "etcdctl" must not accidentally
	// pick up "etcdctl_server".
	svcHash := writeBinary(t, dir, "kindtest_server", []byte("svc-payload"))
	cliHash := writeBinary(t, dir, "etcdctl", []byte("cli-payload"))

	gotSvc, err := verifyInstalledBinaryHash("kindtest", "SERVICE", svcHash, "", "")
	if err != nil || gotSvc != svcHash {
		t.Errorf("SERVICE path: got=(%s,%v) want=(%s,nil)", gotSvc, err, svcHash)
	}
	gotCli, err := verifyInstalledBinaryHash("etcdctl", "COMMAND", cliHash, "", "")
	if err != nil || gotCli != cliHash {
		t.Errorf("COMMAND path: got=(%s,%v) want=(%s,nil)", gotCli, err, cliHash)
	}
}
