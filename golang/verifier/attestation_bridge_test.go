package verifier

import (
	"strings"
	"testing"
	"time"

	"github.com/globulario/services/golang/attestation"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// TestAttestVerdict_TrustedOnFullBindingMatch — wiring contract test.
// When the proof's PID, exe, binary hash, service id, and launch
// authority all match the target's expectations, AttestVerdict returns
// TRUSTED. This is the happy-path verdict the doctor and operators want
// to see on a healthy service.
func TestAttestVerdict_TrustedOnFullBindingMatch(t *testing.T) {
	start := time.Date(2026, 5, 27, 12, 0, 0, 0, time.UTC)
	proof := &node_agentpb.ServiceRuntimeProof{
		ServiceId:        "echo_server",
		RunningPid:       4711,
		RunningExePath:   "/usr/lib/globular/bin/echo_server",
		RunningExeSha256: "abc123",
		SystemdUnitPath:  "/etc/systemd/system/globular-echo-server.service",
		InstalledPath:    "/usr/lib/globular/bin/echo_server",
		ProcessStartTime: timestamppb.New(start),
		CheckedAt:        timestamppb.New(start.Add(30 * time.Second)),
	}
	target := Target{
		Service:               "echo_server",
		DesiredEntrypointChecksum: "sha256:abc123", // prefix tolerated
	}
	verdict, reason := AttestVerdict(target, proof)
	if verdict != string(attestation.VerdictTrusted) {
		t.Fatalf("verdict: got %q (%s), want TRUSTED", verdict, reason)
	}
}

// TestAttestVerdict_OrphanOnMissingLaunchAuthority — covers the contract
// case where systemd doesn't claim the running PID. AttestVerdict must
// surface this so the verifier carries the explicit ORPHAN signal beyond
// the existing runtime_identity_unproven coarse verdict.
func TestAttestVerdict_OrphanOnMissingLaunchAuthority(t *testing.T) {
	now := time.Now()
	proof := &node_agentpb.ServiceRuntimeProof{
		ServiceId:        "echo_server",
		RunningPid:       9999,
		RunningExePath:   "/tmp/leftover_binary",
		RunningExeSha256: "deadbeef",
		SystemdUnitPath:  "", // ← orphan: no launcher recorded
		InstalledPath:    "/tmp/leftover_binary",
		ProcessStartTime: timestamppb.New(now.Add(-time.Minute)),
		CheckedAt:        timestamppb.New(now),
	}
	target := Target{
		Service:               "echo_server",
		DesiredEntrypointChecksum: "deadbeef",
	}
	verdict, reason := AttestVerdict(target, proof)
	if verdict != string(attestation.VerdictOrphan) {
		t.Fatalf("verdict: got %q (%s), want ORPHAN", verdict, reason)
	}
	if !strings.Contains(reason, "orphan") {
		t.Fatalf("reason must mention orphan, got %q", reason)
	}
}

// TestAttestVerdict_MismatchOnBinaryHashDrift — when the running binary's
// hash differs from the desired entrypoint checksum, the verdict must be
// MISMATCH. This is the case the existing verifier already flags via
// FindingRunningBinaryHashMismatch — attestation surfaces it as a
// distinct typed verdict consumers can match on directly.
func TestAttestVerdict_MismatchOnBinaryHashDrift(t *testing.T) {
	now := time.Now()
	proof := &node_agentpb.ServiceRuntimeProof{
		ServiceId:        "echo_server",
		RunningPid:       1234,
		RunningExePath:   "/usr/lib/globular/bin/echo_server",
		RunningExeSha256: "OLD_HASH",
		SystemdUnitPath:  "/etc/systemd/system/globular-echo-server.service",
		InstalledPath:    "/usr/lib/globular/bin/echo_server",
		ProcessStartTime: timestamppb.New(now.Add(-time.Minute)),
		CheckedAt:        timestamppb.New(now),
	}
	target := Target{
		Service:               "echo_server",
		DesiredEntrypointChecksum: "NEW_HASH",
	}
	verdict, _ := AttestVerdict(target, proof)
	if verdict != string(attestation.VerdictMismatch) {
		t.Fatalf("verdict: got %q, want MISMATCH", verdict)
	}
}

// TestAttestVerdict_SkipsHashCheckForWrapperPackages — wrapper services
// (keepalived, scylladb) install outside /usr/lib/globular/bin and have
// no enforceable manifest checksum. AttestVerdict must skip the hash
// binding for those so the verdict isn't a false MISMATCH.
func TestAttestVerdict_SkipsHashCheckForWrapperPackages(t *testing.T) {
	now := time.Now()
	proof := &node_agentpb.ServiceRuntimeProof{
		ServiceId:        "keepalived",
		RunningPid:       1234,
		RunningExePath:   "/usr/sbin/keepalived",
		RunningExeSha256: "actual_hash",
		SystemdUnitPath:  "/etc/systemd/system/globular-keepalived.service",
		InstalledPath:    "/usr/sbin/keepalived", // ← upstream OS path
		ProcessStartTime: timestamppb.New(now.Add(-time.Minute)),
		CheckedAt:        timestamppb.New(now),
	}
	target := Target{
		Service:               "keepalived",
		DesiredEntrypointChecksum: "synthetic_manifest_checksum_that_would_mismatch",
	}
	verdict, reason := AttestVerdict(target, proof)
	if verdict != string(attestation.VerdictTrusted) {
		t.Fatalf("wrapper-pkg verdict: got %q (%s), want TRUSTED (hash check skipped)", verdict, reason)
	}
}

// TestAttestVerdict_NilProofIsUnverified — defense in depth.
func TestAttestVerdict_NilProofIsUnverified(t *testing.T) {
	verdict, reason := AttestVerdict(Target{Service: "x"}, nil)
	if verdict != string(attestation.VerdictUnverified) {
		t.Fatalf("nil proof: got %q (%s), want UNVERIFIED", verdict, reason)
	}
}
