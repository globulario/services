// Package attestation defines RuntimeAttestation — the bound proof that a
// running process is who it claims to be. Collection lives in node_agent's
// runtime_proof.go; this package owns the type and the verification rules
// that turn collected evidence into a yes/no answer.
//
// The contract: a running PID is not trustworthy by name alone. Trust
// requires that the PID, on-disk executable path, binary hash, service
// identity, build id, and launch authority (systemd unit or other launcher)
// all bind to the same expected target — and that the attestation itself
// was observed after the process started. See
// docs/intent/runtime.identity_attestation.yaml.
// @awareness namespace=globular.platform
// @awareness component=platform_attestation
// @awareness file_role=runtime_attestation_type_definitions
// @awareness implements=globular.platform:intent.runtime.identity.requires_proof
// @awareness risk=critical
package attestation

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// RuntimeAttestation is the bound observation of a running process.
type RuntimeAttestation struct {
	// PID of the running process (kernel pid, e.g. /proc/<pid>/).
	PID int

	// ExePath is the absolute path of the binary the process is executing
	// (typically /proc/<pid>/exe resolved). May differ from the systemd
	// unit's ExecStart if the process re-exec'd a different binary.
	ExePath string

	// BinaryHash is the sha256 of the bytes ExePath currently points to.
	BinaryHash string

	// ServiceID is the cluster-wide service identifier (e.g. "echo_server").
	ServiceID string

	// BuildID is the build identifier the service reports at runtime,
	// typically via a /version probe or embedded in the binary at build.
	BuildID string

	// LaunchAuthority names who launched the process. For Globular today
	// this is the systemd unit path; future executors (initd, container
	// runtime) populate their own identifier. Empty when the process is
	// an orphan (no known launcher).
	LaunchAuthority string

	// ObservedAt is when the attestation was collected. MUST be ≥ the
	// process start time — see VerifyAttestation.
	ObservedAt time.Time

	// ProcessStartTime is the kernel-reported start of PID (e.g. from
	// /proc/<pid> ctime). MUST be ≤ ObservedAt.
	ProcessStartTime time.Time
}

// ExpectedIdentity is what the caller believes the running process should
// be. Mismatch on any field rejects the attestation.
type ExpectedIdentity struct {
	ServiceID  string
	BuildID    string // optional — when empty, build id check is skipped (legacy services without /version)
	BinaryHash string // optional — when empty, checksum check is skipped (e.g. wrapper packages)
}

// Verdict is the outcome of VerifyAttestation.
type Verdict string

const (
	// VerdictTrusted — every binding matched and the attestation was
	// observed after the process started.
	VerdictTrusted Verdict = "TRUSTED"

	// VerdictUnverified — missing required fields (no PID, no exe path,
	// no service id). The process exists but we cannot bind it.
	VerdictUnverified Verdict = "UNVERIFIED"

	// VerdictMismatch — a binding did not match the expected identity.
	// The process is something other than what we expected.
	VerdictMismatch Verdict = "MISMATCH"

	// VerdictOrphan — no LaunchAuthority recorded. The process exists
	// but no known launcher claims it.
	VerdictOrphan Verdict = "ORPHAN"

	// VerdictStaleObservation — ObservedAt predates ProcessStartTime,
	// which is impossible for an honest observer. The attestation is
	// either replayed from a previous PID generation or forged.
	VerdictStaleObservation Verdict = "STALE_OBSERVATION"
)

// VerifyAttestation enforces the binding contract. Returns a Verdict and
// (when not Trusted) a human-readable reason. Callers MUST treat any
// non-Trusted verdict as a non-actionable identity claim — do not honor
// the running process as that service.
func VerifyAttestation(att RuntimeAttestation, expect ExpectedIdentity) (Verdict, string) {
	if att.PID <= 0 || strings.TrimSpace(att.ServiceID) == "" || strings.TrimSpace(att.ExePath) == "" {
		return VerdictUnverified, "attestation missing pid, exe path, or service id"
	}
	if att.ObservedAt.IsZero() || att.ProcessStartTime.IsZero() {
		return VerdictUnverified, "attestation missing observed_at or process_start_time"
	}
	// Honest observer rule: you can't observe a process before it started.
	if att.ObservedAt.Before(att.ProcessStartTime) {
		return VerdictStaleObservation,
			fmt.Sprintf("observed_at %s predates process_start_time %s — attestation is replayed or forged",
				att.ObservedAt.Format(time.RFC3339Nano),
				att.ProcessStartTime.Format(time.RFC3339Nano))
	}
	if strings.TrimSpace(att.LaunchAuthority) == "" {
		return VerdictOrphan,
			fmt.Sprintf("pid %d (%s) has no launch authority — orphan process", att.PID, att.ExePath)
	}
	// Bindings.
	if got, want := att.ServiceID, strings.TrimSpace(expect.ServiceID); got != want {
		return VerdictMismatch,
			fmt.Sprintf("service_id mismatch: attestation=%q expected=%q", got, want)
	}
	if expect.BinaryHash != "" {
		if got := strings.TrimSpace(att.BinaryHash); got == "" {
			return VerdictUnverified, "binary hash not measured"
		} else if !equalHashes(got, expect.BinaryHash) {
			return VerdictMismatch,
				fmt.Sprintf("binary_hash mismatch: attestation=%s expected=%s", got, expect.BinaryHash)
		}
	}
	if expect.BuildID != "" {
		if got := strings.TrimSpace(att.BuildID); got == "" {
			return VerdictUnverified, "build_id not reported by service"
		} else if got != expect.BuildID {
			return VerdictMismatch,
				fmt.Sprintf("build_id mismatch: attestation=%q expected=%q", got, expect.BuildID)
		}
	}
	return VerdictTrusted, ""
}

// equalHashes compares two sha256 hashes case-insensitively after stripping
// any "sha256:" prefix on either side.
func equalHashes(a, b string) bool {
	return strings.EqualFold(stripHashPrefix(a), stripHashPrefix(b))
}

func stripHashPrefix(s string) string {
	s = strings.TrimSpace(s)
	if strings.HasPrefix(strings.ToLower(s), "sha256:") {
		return s[len("sha256:"):]
	}
	return s
}

// ErrUnverified is a sentinel returned by helpers that prefer to fail an
// operation rather than report a verdict.
var ErrUnverified = errors.New("runtime attestation unverified")

// MustTrust converts a verdict into an error suitable for guarding
// privileged operations. Returns nil only when verdict is Trusted.
func MustTrust(v Verdict, reason string) error {
	if v == VerdictTrusted {
		return nil
	}
	return fmt.Errorf("%w: verdict=%s reason=%s", ErrUnverified, v, reason)
}
