package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/globulario/services/golang/versionutil"
)

// Declared-identity-first verification.
//
// The defect: verifyInstalledBinaryHashStrict tested entrypoint=="none" FIRST
// and returned BinaryNotApplicable. A package that explicitly declared
// `identity.proof: binary_sha256` with an `identity.installed_path` — a package
// that asked to be verified — was therefore recorded installed with its binary
// never hashed. Package LAYOUT was overriding an explicit identity CONTRACT.
//
// Precedence is now: declared mode → declared subject → declared expected value
// → observed proof → verdict. Entrypoint layout is consulted only when no
// identity was declared at all.

func writeTempBinary(t *testing.T, content string) (path, sha string) {
	t.Helper()
	dir := t.TempDir()
	path = filepath.Join(dir, "libexample.so.2")
	if err := os.WriteFile(path, []byte(content), 0o755); err != nil {
		t.Fatalf("write temp binary: %v", err)
	}
	sum := sha256.Sum256([]byte(content))
	return path, hex.EncodeToString(sum[:])
}

// 1. entrypoint:none + binary_sha256 + matching file → BinaryVerified.
// This is the case that previously returned NotApplicable without hashing.
func TestIdentityVerify_NoEntrypointDeclaredBinary_Verified(t *testing.T) {
	path, sha := writeTempBinary(t, "real payload bytes")
	got, verdict, err := verifyInstalledBinaryIdentityStrict("libexample", "COMMAND", sha, "bid-1", "op-1",
		declaredPackageIdentity{Proof: identityProofBinarySHA256, InstalledPath: path, Entrypoint: "none"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if verdict != BinaryVerified {
		t.Fatalf("verdict = %v, want %v — entrypoint:none must not exempt a declared binary identity",
			verdict, BinaryVerified)
	}
	if got != sha {
		t.Errorf("hash = %q, want %q", got, sha)
	}
}

// 2. Tampered bytes → BinaryMismatch, with the evidence naming the real path.
func TestIdentityVerify_MismatchNamesDeclaredPath(t *testing.T) {
	path, _ := writeTempBinary(t, "tampered bytes")
	expected := strings.Repeat("ab", 32)
	_, verdict, err := verifyInstalledBinaryIdentityStrict("libexample", "COMMAND", expected, "bid-1", "op-1",
		declaredPackageIdentity{Proof: identityProofBinarySHA256, InstalledPath: path, Entrypoint: "none"})
	if verdict != BinaryMismatch {
		t.Fatalf("verdict = %v, want %v", verdict, BinaryMismatch)
	}
	var mm *BinaryHashMismatchError
	if !asMismatch(err, &mm) {
		t.Fatalf("error %v is not a structured mismatch", err)
	}
	if mm.Path != path {
		t.Errorf("evidence path = %q, want the declared subject %q", mm.Path, path)
	}
	if mm.Expected != expected || mm.Actual == "" {
		t.Errorf("evidence lost expected/actual: %+v", mm)
	}
	if mm.BuildID != "bid-1" || mm.OperationID != "op-1" {
		t.Errorf("evidence lost build/apply identifiers: %+v", mm)
	}
}

// 3. Declared subject absent on disk → BinaryMissing (not NotApplicable).
func TestIdentityVerify_MissingDeclaredFile(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "never-installed.so")
	_, verdict, err := verifyInstalledBinaryIdentityStrict("libexample", "COMMAND", strings.Repeat("cd", 32),
		"bid-1", "op-1",
		declaredPackageIdentity{Proof: identityProofBinarySHA256, InstalledPath: missing, Entrypoint: "none"})
	if verdict != BinaryMissing {
		t.Fatalf("verdict = %v, want %v", verdict, BinaryMissing)
	}
	if err == nil {
		t.Error("missing declared subject must carry structured evidence")
	}
}

// 4./5. A declared binary proof with no usable subject fails closed. It must
// never be NotApplicable: the package asked to be verified and cannot be.
func TestIdentityVerify_DeclaredBinaryWithoutSubjectFailsClosed(t *testing.T) {
	for _, tc := range []struct{ name, path string }{
		{"empty path", ""},
		{"relative path", "usr/lib/x.so"},
		{"traversal", "/usr/../etc/shadow"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, verdict, err := verifyInstalledBinaryIdentityStrict("libexample", "COMMAND",
				strings.Repeat("ef", 32), "bid-1", "op-1",
				declaredPackageIdentity{Proof: identityProofBinarySHA256, InstalledPath: tc.path, Entrypoint: "none"})
			if verdict == BinaryNotApplicable || verdict == BinaryVerified {
				t.Fatalf("verdict = %v — a declared binary identity with no verifiable subject "+
					"must fail closed, never be exempted", verdict)
			}
			if err == nil {
				t.Error("fail-closed must carry an explanation")
			}
		})
	}
}

// 6. Declared binary proof but no expected checksum: never verified success.
func TestIdentityVerify_NoExpectedChecksumIsNotSuccess(t *testing.T) {
	path, _ := writeTempBinary(t, "bytes")
	_, verdict, _ := verifyInstalledBinaryIdentityStrict("libexample", "COMMAND", "", "bid-1", "op-1",
		declaredPackageIdentity{Proof: identityProofBinarySHA256, InstalledPath: path, Entrypoint: "none"})
	if verdict == BinaryVerified || verdict == BinaryNotApplicable {
		t.Fatalf("verdict = %v — missing expected checksum must not read as proven", verdict)
	}
}

// 7. Unknown proof mode fails closed — no fallthrough to layout or success.
func TestIdentityVerify_UnknownProofFailsClosed(t *testing.T) {
	path, sha := writeTempBinary(t, "bytes")
	_, verdict, err := verifyInstalledBinaryIdentityStrict("libexample", "COMMAND", sha, "bid-1", "op-1",
		declaredPackageIdentity{Proof: "gpg_signature", InstalledPath: path, Entrypoint: "none"})
	if verdict == BinaryNotApplicable || verdict == BinaryVerified {
		t.Fatalf("verdict = %v — an unknown declared proof mode must fail closed", verdict)
	}
	if err == nil || !strings.Contains(err.Error(), "unsupported declared identity proof") {
		t.Errorf("error should name the unsupported mode: %v", err)
	}
}

// 8. No declared identity + entrypoint none → NotApplicable. This is the honest
// libnss-resolve contract and must be preserved exactly.
func TestIdentityVerify_NoDeclaredIdentityEntrypointNoneIsNotApplicable(t *testing.T) {
	_, verdict, err := verifyInstalledBinaryIdentityStrict("libnss-resolve", "COMMAND", "", "", "",
		declaredPackageIdentity{Entrypoint: "none"})
	if verdict != BinaryNotApplicable {
		t.Fatalf("verdict = %v, want %v for the declared-no-proof package", verdict, BinaryNotApplicable)
	}
	if err != nil {
		t.Errorf("not-applicable is a legitimate outcome, not an error: %v", err)
	}
}

// 10. version proof does not enter the SHA path.
func TestIdentityVerify_VersionProofSkipsBinaryPath(t *testing.T) {
	path, sha := writeTempBinary(t, "bytes")
	got, verdict, err := verifyInstalledBinaryIdentityStrict("example", "COMMAND", sha, "", "",
		declaredPackageIdentity{Proof: identityProofVersion, InstalledPath: path, Entrypoint: "none"})
	if verdict != BinaryNotApplicable {
		t.Fatalf("verdict = %v — version-proved packages must not run the SHA verifier", verdict)
	}
	if got != "" || err != nil {
		t.Errorf("version proof should produce no hash and no error, got %q / %v", got, err)
	}
}

// The subject selector: declared path wins; layout fallback is available only
// to entrypoint-bearing packages.
func TestIdentityVerificationPath_Selection(t *testing.T) {
	if p, err := identityVerificationPath("x", "COMMAND",
		declaredPackageIdentity{InstalledPath: "/opt/x/bin/x", Entrypoint: "bin/x"}); err != nil || p != "/opt/x/bin/x" {
		t.Errorf("declared path must win: %q / %v", p, err)
	}
	if _, err := identityVerificationPath("x", "COMMAND",
		declaredPackageIdentity{Entrypoint: "none"}); err == nil {
		t.Error("no-entrypoint package with no declared path must have no derivable subject")
	}
	if p, err := identityVerificationPath("x", "COMMAND",
		declaredPackageIdentity{Entrypoint: "bin/x"}); err != nil || p == "" {
		t.Errorf("entrypoint-bearing package should fall back to layout: %q / %v", p, err)
	}
}

// ── installed-path sidecar ────────────────────────────────────────────

func TestIdentityInstalledPathSidecar_RoundTripAndReplacement(t *testing.T) {
	base := t.TempDir()
	versionutil.SetBaseDir(base)
	t.Cleanup(func() { versionutil.SetBaseDir("/var/lib/globular/services") })

	const name = "libexample"
	if versionutil.ReadIdentityInstalledPath(name) != "" {
		t.Fatal("expected no sidecar before any write")
	}

	if err := versionutil.WriteIdentityInstalledPath(name, "/usr/lib/x86_64-linux-gnu/libexample.so.2"); err != nil {
		t.Fatalf("write: %v", err)
	}
	if got := versionutil.ReadIdentityInstalledPath(name); got != "/usr/lib/x86_64-linux-gnu/libexample.so.2" {
		t.Fatalf("round-trip = %q", got)
	}

	// Relative and traversing paths are rejected outright.
	for _, bad := range []string{"relative/x.so", "../escape.so", "/usr/../etc/shadow"} {
		if err := versionutil.WriteIdentityInstalledPath(name, bad); err == nil {
			t.Errorf("write accepted invalid path %q", bad)
		}
	}

	// THE REPLACEMENT LAW: reinstalling an artifact that declares no installed
	// path must CLEAR the sidecar. Inheriting the previous artifact's path
	// would make the verifier hash a binary this package never claimed — a
	// false proof, which is worse than a missing one.
	if err := versionutil.WriteIdentityInstalledPath(name, ""); err != nil {
		t.Fatalf("clearing write: %v", err)
	}
	if got := versionutil.ReadIdentityInstalledPath(name); got != "" {
		t.Fatalf("stale path %q survived a reinstall that declared none", got)
	}
}

func asMismatch(err error, target **BinaryHashMismatchError) bool {
	m, ok := err.(*BinaryHashMismatchError)
	if ok {
		*target = m
	}
	return ok
}

// ── apply-level proof ─────────────────────────────────────────────────
//
// Drives the REAL projection chain the apply path uses — manifest-declared
// values persisted through versionutil sidecars, read back by the production
// verifier via verifyInstalledBinaryHashStrict (the exact function the three
// ApplyPackageRelease branches call), then evaluated with the production
// decision predicate.
//
// Scope stated honestly: the ApplyPackageRelease RPC itself commits through
// installed_state/etcd, which is not available in unit tests, so this exercises
// everything up to and including the verdict the RPC branches on rather than
// the gRPC surface. The mapping asserted below is the code at
// apply_package_release.go: verr != nil takes the mismatch branch (Ok=false),
// BinaryUnverified takes the unverified branch, and only a clean verdict
// reaches Status="installed".
func TestApplyIdentity_DeclaredBinaryProofGovernsInstalledStatus(t *testing.T) {
	base := t.TempDir()
	versionutil.SetBaseDir(base)
	t.Cleanup(func() { versionutil.SetBaseDir("/var/lib/globular/services") })

	const name, kind = "libexample", "COMMAND"
	goodPath, goodSHA := writeTempBinary(t, "the real installed payload")

	// The artifact declared: entrypoint none, binary_sha256, explicit subject.
	persist := func(installedPath string) {
		if err := versionutil.WriteEntrypoint(name, "none"); err != nil {
			t.Fatalf("write entrypoint sidecar: %v", err)
		}
		if err := versionutil.WriteIdentityProof(name, identityProofBinarySHA256); err != nil {
			t.Fatalf("write proof sidecar: %v", err)
		}
		if err := versionutil.WriteIdentityInstalledPath(name, installedPath); err != nil {
			t.Fatalf("write installed-path sidecar: %v", err)
		}
	}

	// statusFor applies the production mapping from apply_package_release.go.
	statusFor := func(expectedSHA string) (status string, ok bool, hash string) {
		h, verdict, verr := verifyInstalledBinaryHashStrict(name, kind, expectedSHA, "bid-1", "op-1",
			versionutil.ReadEntrypoint(name))
		switch {
		case verr != nil:
			return "binary_hash_mismatch", false, h
		case verdict == BinaryUnverified:
			return StatusBinaryUnverified, false, h
		default:
			return "installed", true, h
		}
	}

	t.Run("positive: matching bytes yield a verified install", func(t *testing.T) {
		persist(goodPath)
		status, ok, hash := statusFor(goodSHA)
		if status != "installed" || !ok {
			t.Fatalf("status=%q ok=%v — matching declared bytes must verify", status, ok)
		}
		if hash != goodSHA {
			t.Errorf("recorded hash %q is not the declared subject's hash %q", hash, goodSHA)
		}
		// The receipt must bind the DECLARED subject, not the layout path.
		if got := receiptBinaryPath(name, kind); got != goodPath {
			t.Errorf("receipt bound %q, want the declared installed path %q", got, goodPath)
		}
	})

	t.Run("missing binary: no installed status, evidence retained", func(t *testing.T) {
		persist(filepath.Join(t.TempDir(), "never-installed.so"))
		status, ok, _ := statusFor(goodSHA)
		if ok || status == "installed" {
			t.Fatalf("status=%q ok=%v — a declared subject that is absent must never be installed", status, ok)
		}
		_, _, verr := verifyInstalledBinaryHashStrict(name, kind, goodSHA, "bid-1", "op-1",
			versionutil.ReadEntrypoint(name))
		var miss *BinaryMissingError
		if !asMissing(verr, &miss) {
			t.Fatalf("structured missing-binary evidence not retained: %v", verr)
		}
		if miss.Package != name || miss.Expected == "" {
			t.Errorf("evidence incomplete: %+v", miss)
		}
	})

	t.Run("tampered binary: no installed status, mismatch evidence retained", func(t *testing.T) {
		tamperedPath, _ := writeTempBinary(t, "tampered payload")
		persist(tamperedPath)
		status, ok, _ := statusFor(goodSHA)
		if ok || status == "installed" {
			t.Fatalf("status=%q ok=%v — tampered declared bytes must never be installed", status, ok)
		}
		_, _, verr := verifyInstalledBinaryHashStrict(name, kind, goodSHA, "bid-1", "op-1",
			versionutil.ReadEntrypoint(name))
		var mm *BinaryHashMismatchError
		if !asMismatch(verr, &mm) {
			t.Fatalf("structured mismatch evidence not retained: %v", verr)
		}
		if mm.Path != tamperedPath || mm.Expected != goodSHA || mm.Actual == "" {
			t.Errorf("evidence incomplete or names the wrong path: %+v", mm)
		}
	})

	t.Run("declared proof with no subject: fails closed, not installed", func(t *testing.T) {
		persist("") // artifact declares binary_sha256 but names no path
		status, ok, _ := statusFor(goodSHA)
		if ok || status == "installed" {
			t.Fatalf("status=%q ok=%v — an unfulfillable declared identity must not install", status, ok)
		}
	})

	t.Run("honest no-proof package still installs", func(t *testing.T) {
		const bare = "libnss-resolve"
		if err := versionutil.WriteEntrypoint(bare, "none"); err != nil {
			t.Fatal(err)
		}
		h, verdict, verr := verifyInstalledBinaryHashStrict(bare, kind, "", "", "",
			versionutil.ReadEntrypoint(bare))
		if verdict != BinaryNotApplicable || verr != nil || h != "" {
			t.Fatalf("declared-no-proof package regressed: verdict=%v err=%v hash=%q", verdict, verr, h)
		}
	})
}

func asMissing(err error, target **BinaryMissingError) bool {
	m, ok := err.(*BinaryMissingError)
	if ok {
		*target = m
	}
	return ok
}
