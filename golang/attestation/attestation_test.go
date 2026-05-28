package attestation

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// TestRuntimeAttestationBindsPidHashServiceAndBuild — contract test.
// A trusted attestation must bind PID, exe path, binary hash, service id,
// build id, launch authority, and observed-after-start. Mismatch on any
// binding rejects.
func TestRuntimeAttestationBindsPidHashServiceAndBuild(t *testing.T) {
	start := time.Date(2026, 5, 27, 12, 0, 0, 0, time.UTC)
	good := RuntimeAttestation{
		PID:              4711,
		ExePath:          "/usr/lib/globular/bin/echo_server",
		BinaryHash:       "abcdef0123456789",
		ServiceID:        "echo_server",
		BuildID:          "build-42",
		LaunchAuthority:  "/etc/systemd/system/globular-echo-server.service",
		ProcessStartTime: start,
		ObservedAt:       start.Add(30 * time.Second),
	}
	want := ExpectedIdentity{
		ServiceID:  "echo_server",
		BuildID:    "build-42",
		BinaryHash: "sha256:abcdef0123456789", // case + prefix tolerated
	}
	if v, reason := VerifyAttestation(good, want); v != VerdictTrusted {
		t.Fatalf("good attestation: got %s (%s), want TRUSTED", v, reason)
	}

	// Mismatch on each binding rejects with a Mismatch verdict.
	cases := []struct {
		name   string
		mutate func(*ExpectedIdentity)
		want   Verdict
	}{
		{"wrong service", func(e *ExpectedIdentity) { e.ServiceID = "auth_server" }, VerdictMismatch},
		{"wrong binary hash", func(e *ExpectedIdentity) { e.BinaryHash = "ff" }, VerdictMismatch},
		{"wrong build id", func(e *ExpectedIdentity) { e.BuildID = "build-43" }, VerdictMismatch},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			e := want
			tc.mutate(&e)
			if v, _ := VerifyAttestation(good, e); v != tc.want {
				t.Fatalf("got %s, want %s", v, tc.want)
			}
		})
	}

	// Missing required fields → Unverified (not Trusted, not Mismatch).
	stripped := good
	stripped.ExePath = ""
	if v, _ := VerifyAttestation(stripped, want); v != VerdictUnverified {
		t.Fatalf("missing exe path: got %s, want UNVERIFIED", v)
	}

	// MustTrust converts verdicts to errors with the sentinel.
	if err := MustTrust(VerdictTrusted, ""); err != nil {
		t.Fatalf("MustTrust(TRUSTED) must be nil, got %v", err)
	}
	if err := MustTrust(VerdictMismatch, "x"); !errors.Is(err, ErrUnverified) {
		t.Fatalf("MustTrust(MISMATCH) must wrap ErrUnverified, got %v", err)
	}
}

// TestOrphanProcessIsUntrustedWithoutAttestation — contract test. A
// process with no recorded launch authority (no systemd unit, no container
// runtime) is not honored as the expected service even if every other
// binding lines up.
func TestOrphanProcessIsUntrustedWithoutAttestation(t *testing.T) {
	start := time.Now().Add(-time.Minute)
	orphan := RuntimeAttestation{
		PID:              9999,
		ExePath:          "/tmp/leftover_binary",
		BinaryHash:       "deadbeef",
		ServiceID:        "echo_server",
		LaunchAuthority:  "", // ← no launcher known
		ProcessStartTime: start,
		ObservedAt:       start.Add(30 * time.Second),
	}
	want := ExpectedIdentity{ServiceID: "echo_server", BinaryHash: "deadbeef"}
	v, reason := VerifyAttestation(orphan, want)
	if v != VerdictOrphan {
		t.Fatalf("got %s, want ORPHAN; reason=%s", v, reason)
	}
	if !strings.Contains(reason, "orphan") {
		t.Fatalf("reason must explain orphan status, got %q", reason)
	}
	// And: MustTrust refuses the orphan.
	if err := MustTrust(v, reason); err == nil {
		t.Fatal("MustTrust must refuse orphan verdict")
	}
}

// TestVerifierRejectsAttestationOlderThanProcessStart — contract test.
// ObservedAt < ProcessStartTime is physically impossible for an honest
// observer (you can't see a process before it exists). The attestation
// must be flagged as stale/replayed, never honored as live evidence.
func TestVerifierRejectsAttestationOlderThanProcessStart(t *testing.T) {
	// A previous-PID-generation observation: process restarted at 12:00,
	// but the attestation timestamp is from 11:59. Honest observer can't
	// see the new process before it started.
	procStart := time.Date(2026, 5, 27, 12, 0, 0, 0, time.UTC)
	att := RuntimeAttestation{
		PID:              1234,
		ExePath:          "/usr/lib/globular/bin/echo_server",
		BinaryHash:       "abc",
		ServiceID:        "echo_server",
		LaunchAuthority:  "/etc/systemd/system/echo.service",
		ProcessStartTime: procStart,
		ObservedAt:       procStart.Add(-1 * time.Minute), // ← before start
	}
	want := ExpectedIdentity{ServiceID: "echo_server", BinaryHash: "abc"}
	v, reason := VerifyAttestation(att, want)
	if v != VerdictStaleObservation {
		t.Fatalf("got %s, want STALE_OBSERVATION; reason=%s", v, reason)
	}
	if !strings.Contains(reason, "predates") {
		t.Fatalf("reason must say predates, got %q", reason)
	}
}
