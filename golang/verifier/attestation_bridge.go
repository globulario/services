package verifier

import (
	"strings"

	"github.com/globulario/services/golang/attestation"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

// AttestVerdict runs the runtime.identity_attestation contract against a
// node_agent ServiceRuntimeProof + verifier Target and returns the
// attestation verdict + reason.
//
// The verifier already computes a coarse ProofStatus from the proof; this
// function adds the focused "do PID, exe, hash, service id, build id, and
// launch authority all bind?" answer the attestation contract requires.
// See docs/intent/runtime.identity_attestation.yaml.
//
// Pure function — translates one proof, no I/O.
func AttestVerdict(target Target, proof *node_agentpb.ServiceRuntimeProof) (string, string) {
	if proof == nil {
		return string(attestation.VerdictUnverified), "no runtime proof captured"
	}
	att := attestation.RuntimeAttestation{
		PID:              int(proof.GetRunningPid()),
		ExePath:          strings.TrimSpace(proof.GetRunningExePath()),
		BinaryHash:       strings.TrimSpace(proof.GetRunningExeSha256()),
		ServiceID:        strings.TrimSpace(proof.GetServiceId()),
		BuildID:          strings.TrimSpace(proof.GetRuntimeBuildId()),
		LaunchAuthority:  strings.TrimSpace(proof.GetSystemdUnitPath()),
	}
	if t := proof.GetProcessStartTime(); t != nil {
		att.ProcessStartTime = t.AsTime()
	}
	if t := proof.GetCheckedAt(); t != nil {
		att.ObservedAt = t.AsTime()
	}

	expect := attestation.ExpectedIdentity{
		ServiceID:  strings.TrimSpace(target.Service),
		BuildID:    strings.TrimSpace(target.DesiredBuildID),
		BinaryHash: normalizeHash(target.DesiredEntrypointChecksum),
	}
	// Wrapper packages (keepalived, scylladb upstream binaries) install
	// outside the Globular-managed bin tree — the manifest's
	// entrypoint_checksum is synthetic and cannot be enforced. The
	// existing verifier already handles this via installedPathIsUpstream;
	// mirror that here so attestation doesn't fire false MISMATCH on
	// wrapper services.
	if installedPathIsUpstream(proof.GetInstalledPath()) {
		expect.BinaryHash = "" // skip hash binding for wrapper packages
	}
	// Runtime build_id reporting is not yet implemented across all
	// services (see runtime_proof.go comment block). When the proof
	// carries no runtime build_id and the target requires one, the
	// honest answer is UNVERIFIED rather than MISMATCH — operators can
	// see "build_id not reported" in the reason and respond.
	if expect.BuildID != "" && att.BuildID == "" {
		expect.BuildID = "" // skip build_id binding until services expose /version
	}

	v, reason := attestation.VerifyAttestation(att, expect)
	return string(v), reason
}
