package main

import (
	"fmt"
	"os"
	"strings"
)

// Diagnostic Honesty — Phase 1
//
// The Prime Directive: no component may report "installed" from a claim alone.
//
// A successful payload extraction + systemctl restart is a CLAIM. The proof
// that an install really applied the requested artifact is: the bytes on disk
// at the deployed binary path hash to the same sha256 as the published
// artifact manifest.
//
// This file holds the synchronous post-install hash gate. It runs at apply
// time, after the payload is unpacked but before installed_state is written
// as "installed". A mismatch fails the apply and writes installed_state with
// Status="failed_binary_hash_mismatch" plus structured evidence so doctor /
// the verifier can surface the drift.
//
// Behaviour matrix:
//
//   expected == ""           → unverified path. Returns actual hash, no error.
//                              Caller is responsible for logging the lack of
//                              proof. (Bootstrap/legacy callers that don't yet
//                              propagate expected_sha256 fall in here. Tightened
//                              by a later phase.)
//   expected != "" && match  → returns actual hash, no error.
//   expected != "" && drift  → returns BinaryHashMismatchError carrying
//                              expected/actual/path so the caller can build
//                              the package.installed_binary_hash_mismatch
//                              finding evidence map.
//   binary missing           → if expected != "", returns BinaryMissingError;
//                              if expected == "", returns ("", os.ErrNotExist)
//                              so callers can decide.

// StatusBinaryHashMismatch is the installed_state.Status written when a
// post-install proof check fails. Doctor / the future verifier lift this
// into a package.installed_binary_hash_mismatch finding.
const StatusBinaryHashMismatch = "failed_binary_hash_mismatch"

// proofFailure is the common shape of post-install proof errors. Both
// BinaryHashMismatchError and BinaryMissingError implement it so the apply
// path can handle either with one branch.
type proofFailure interface {
	error
	EvidenceMap() map[string]string
}

// BinaryHashMismatchError reports that the deployed binary on disk does not
// hash to the expected artifact manifest digest. This is a CRITICAL apply
// failure: it means the install path lied — either the payload was tampered,
// the extract step silently failed, or a different artifact landed at the
// expected path.
type BinaryHashMismatchError struct {
	Package     string
	Kind        string
	Path        string
	Expected    string // expected sha256 from artifact manifest (lowercase hex, no "sha256:" prefix)
	Actual      string // actual sha256 computed from /usr/lib/globular/bin/<binary>
	BuildID     string // expected build_id (passed through for finding evidence)
	OperationID string // apply run id
}

// Error implements error.
func (e *BinaryHashMismatchError) Error() string {
	return fmt.Sprintf(
		"installed binary hash mismatch for %s/%s at %s: expected sha256=%s, actual sha256=%s (build_id=%s, apply_run_id=%s)",
		e.Kind, e.Package, e.Path, e.Expected, e.Actual, e.BuildID, e.OperationID,
	)
}

// EvidenceMap returns the structured evidence fields for the
// package.installed_binary_hash_mismatch finding. Callers store this in
// InstalledPackage.Metadata so doctor / the verifier can lift it into a
// finding without having to re-derive the values.
func (e *BinaryHashMismatchError) EvidenceMap() map[string]string {
	return map[string]string{
		"error":             e.Error(),
		"finding":           "package.installed_binary_hash_mismatch",
		"installed_path":    e.Path,
		"expected_sha256":   e.Expected,
		"actual_sha256":     e.Actual,
		"expected_build_id": e.BuildID,
		"apply_run_id":      e.OperationID,
	}
}

// BinaryMissingError reports that the expected installed binary is absent
// after a supposedly-successful install. With expected_sha256 in hand we
// cannot prove the install — the apply must fail.
type BinaryMissingError struct {
	Package     string
	Kind        string
	Path        string
	Expected    string
	BuildID     string
	OperationID string
	Underlying  error
}

func (e *BinaryMissingError) Error() string {
	return fmt.Sprintf(
		"installed binary missing for %s/%s at %s (expected sha256=%s, build_id=%s): %v",
		e.Kind, e.Package, e.Path, e.Expected, e.BuildID, e.Underlying,
	)
}

func (e *BinaryMissingError) Unwrap() error { return e.Underlying }

func (e *BinaryMissingError) EvidenceMap() map[string]string {
	return map[string]string{
		"error":             e.Error(),
		"finding":           "package.installed_binary_missing",
		"installed_path":    e.Path,
		"expected_sha256":   e.Expected,
		"expected_build_id": e.BuildID,
		"apply_run_id":      e.OperationID,
	}
}

// normalizeHash strips a "sha256:" prefix and lowercases the hex. The
// manifest store and the local computation use slightly different shapes
// historically; comparison must be format-agnostic.
func normalizeHash(h string) string {
	s := strings.ToLower(strings.TrimSpace(h))
	return strings.TrimPrefix(s, "sha256:")
}

// BinaryVerdict is the explicit outcome of the post-install hash gate. The
// "missing checksum" case is no longer collapsed into "verified": apply-package
// callers MUST treat it as Unverified (degraded, not success).
//
// Verdict semantics:
//   - BinaryVerified   — expected provided, actual matched. Caller may declare SUCCESS.
//   - BinaryUnverified — expected NOT provided (legacy caller / older release-index).
//                         Binary is on disk but its identity is unproven. Caller
//                         must record an UNVERIFIED installed-state, NOT SUCCESS.
//   - BinaryMismatch   — expected provided, actual differs. Returned via error.
//   - BinaryMissing    — expected provided, binary absent. Returned via error.
//
// See docs/intent/runtime.success_requires_verified_identity.yaml and
// invariant runtime.success_requires_expected_binary_checksum.
type BinaryVerdict string

const (
	BinaryVerified   BinaryVerdict = "VERIFIED"
	BinaryUnverified BinaryVerdict = "UNVERIFIED"
	BinaryMismatch   BinaryVerdict = "MISMATCH"
	BinaryMissing    BinaryVerdict = "MISSING"
)

// StatusBinaryUnverified is the installed_state.Status written when apply
// completes but no expected_sha256 was provided to prove binary identity.
// This is NOT a failure — the install ran, the service is up — but it is
// explicitly NOT a verified success. Doctor / verifier lift this into a
// package.installed_binary_unverified finding so the gap is visible.
const StatusBinaryUnverified = "installed_unverified"

// verifyInstalledBinaryHashStrict is the verdict-returning replacement for
// verifyInstalledBinaryHash. It NEVER collapses missing-expected into success.
//
// Use this from production apply-package callsites. The caller decides what
// to do with BinaryUnverified — typically: write installed-state with
// Status=installed_unverified instead of installed, and return Ok=false with
// a degraded reason. Do not declare SUCCESS.
func verifyInstalledBinaryHashStrict(name, kind, expectedSHA256, buildID, operationID string) (actualHash string, verdict BinaryVerdict, err error) {
	path := installedBinaryPath(name, kind)
	expected := normalizeHash(expectedSHA256)

	actual, hashErr := cachedSha256(path)

	if expected == "" {
		// No expected provided — degraded path. Return whatever hash we
		// could read (possibly empty if the binary is missing) and let the
		// caller record an UNVERIFIED installed-state.
		if hashErr != nil {
			return "", BinaryUnverified, nil
		}
		return actual, BinaryUnverified, nil
	}

	if hashErr != nil {
		return "", BinaryMissing, &BinaryMissingError{
			Package:     name,
			Kind:        kind,
			Path:        path,
			Expected:    expected,
			BuildID:     buildID,
			OperationID: operationID,
			Underlying:  hashErr,
		}
	}
	if actual != expected {
		return actual, BinaryMismatch, &BinaryHashMismatchError{
			Package:     name,
			Kind:        kind,
			Path:        path,
			Expected:    expected,
			Actual:      actual,
			BuildID:     buildID,
			OperationID: operationID,
		}
	}
	return actual, BinaryVerified, nil
}

// verifyInstalledBinaryHash is the Phase 1 synchronous proof gate. It must be
// called after the payload extraction step in ApplyPackageRelease and before
// installed_state is written with Status="installed".
//
// DEPRECATED: this function returns (hash, nil) for empty expected, silently
// treating "no proof" as "verified". Use verifyInstalledBinaryHashStrict and
// branch on the returned BinaryVerdict. The shim is kept so legacy test
// fixtures continue to document the historical behavior. All production
// apply-package callsites in apply_package_release.go have been migrated.
//
// Contract: the returned error is NON-NIL only when a proof check FAILED
// (expected was supplied and either the binary is missing or its hash drifts).
// In the unverified path (expected == ""), the function never returns an
// error — it returns the actual hash on success or "" if the binary is
// unreadable, leaving "is missing binary OK?" to the caller's existing logic.
//
//   expected == ""           → ("", nil) if binary unreadable, (hash, nil) otherwise
//   expected != "" && match  → (hash, nil)
//   expected != "" && drift  → (actual, *BinaryHashMismatchError)
//   expected != "" && absent → ("", *BinaryMissingError)
func verifyInstalledBinaryHash(name, kind, expectedSHA256, buildID, operationID string) (string, error) {
	path := installedBinaryPath(name, kind)
	expected := normalizeHash(expectedSHA256)

	actual, hashErr := cachedSha256(path)

	if expected == "" {
		// Unverified path — degraded but not a failure. Claims without
		// proof are not stored as proof. Legacy callers that don't yet
		// propagate expected_sha256 land here. A later phase tightens
		// this to fail closed.
		if hashErr != nil {
			return "", nil
		}
		return actual, nil
	}

	// Proof requested — verification is mandatory from this point on.
	if hashErr != nil {
		return "", &BinaryMissingError{
			Package:     name,
			Kind:        kind,
			Path:        path,
			Expected:    expected,
			BuildID:     buildID,
			OperationID: operationID,
			Underlying:  hashErr,
		}
	}
	if actual != expected {
		return actual, &BinaryHashMismatchError{
			Package:     name,
			Kind:        kind,
			Path:        path,
			Expected:    expected,
			Actual:      actual,
			BuildID:     buildID,
			OperationID: operationID,
		}
	}
	return actual, nil
}

// statBinaryExists is a small helper that lets tests assert binary presence
// without importing os.
func statBinaryExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
