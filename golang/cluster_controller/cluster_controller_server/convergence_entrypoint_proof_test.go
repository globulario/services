package main

// convergence_entrypoint_proof_test.go — Phase 38.
//
// Pins the root-cause fix: classifyPackageConvergence MUST report
// RepairRequired when desired+installed entrypoint_checksum both present
// and differ, even when version/hash/buildId/runtime all match. This
// is the false-converged-lying-installed_state pattern caught live on
// globule-ryzen 2026-06-03.

import (
	"testing"
	"time"

	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

func TestClassifyPackageConvergence_EntrypointChecksumMismatch_RequiresRepair(t *testing.T) {
	// All upstream gates pass (version, hash, buildId, runtime active)
	// — but entrypoint_checksum mismatch means the binary on disk is
	// the OLD bytes. Pre-Phase-38 this returned converged. Now it
	// must return RepairRequired with a specific reason.
	node := &nodeState{
		LastSeen: time.Now(),
		Units: []unitStatusRecord{
			{Name: "globular-node-agent.service", State: "active"},
		},
	}
	installed := &node_agentpb.InstalledPackage{
		Version:  "1.2.143",
		Checksum: "sha256:cafebabe",
		BuildId:  "019e8da6-42a7-7201-b858-4bf26d76e67c",
		Metadata: map[string]string{
			"entrypoint_checksum": "20d5bfff12f4ee2fd25bedaebb95740d80b51137b7345f176977cffea47d35ec", // OLD binary
		},
	}
	pc := classifyPackageConvergence(
		node,
		"node-agent",
		"SERVICE",
		"1.2.143",
		"sha256:cafebabe",
		"019e8da6-42a7-7201-b858-4bf26d76e67c",
		"e9434387f92fd3a19fc399fa8d2a9b2f7097f151d0027b4a7d20cccfe22556c8", // NEW desired
		false,
		installed,
		time.Now(),
	)
	if !pc.RepairRequired {
		t.Fatalf("expected RepairRequired for entrypoint_checksum mismatch, got reason=%q", pc.Reason)
	}
	if !pc.VersionOK || !pc.HashOK || !pc.BuildIDOK {
		t.Errorf("upstream gates should still pass; got version=%v hash=%v buildId=%v", pc.VersionOK, pc.HashOK, pc.BuildIDOK)
	}
}

func TestClassifyPackageConvergence_EntrypointChecksumMatches_Converged(t *testing.T) {
	// Healthy case: all four identity dimensions agree, runtime active.
	node := &nodeState{
		LastSeen: time.Now(),
		Units: []unitStatusRecord{
			{Name: "globular-node-agent.service", State: "active"},
		},
	}
	installed := &node_agentpb.InstalledPackage{
		Version:  "1.2.143",
		Checksum: "sha256:cafebabe",
		BuildId:  "019e8da6-42a7-7201-b858-4bf26d76e67c",
		Metadata: map[string]string{
			"entrypoint_checksum": "e9434387f92fd3a19fc399fa8d2a9b2f7097f151d0027b4a7d20cccfe22556c8",
		},
	}
	pc := classifyPackageConvergence(
		node,
		"node-agent",
		"SERVICE",
		"1.2.143",
		"sha256:cafebabe",
		"019e8da6-42a7-7201-b858-4bf26d76e67c",
		"e9434387f92fd3a19fc399fa8d2a9b2f7097f151d0027b4a7d20cccfe22556c8",
		false,
		installed,
		time.Now(),
	)
	if pc.RepairRequired {
		t.Fatalf("steady-state converged should not require repair; reason=%q", pc.Reason)
	}
	if !pc.RuntimeOK {
		t.Errorf("expected RuntimeOK, got %s", pc.RuntimeState)
	}
}

func TestClassifyPackageConvergence_DesiredEntrypointEmpty_NoOpinion(t *testing.T) {
	// Legacy artifact with no recorded proof on the desired side —
	// classifyPackageConvergence should NOT speculate. The verifier
	// surfaces missing proof via a separate doctor finding.
	node := &nodeState{
		LastSeen: time.Now(),
		Units: []unitStatusRecord{
			{Name: "globular-foo.service", State: "active"},
		},
	}
	installed := &node_agentpb.InstalledPackage{
		Version:  "1.0.0",
		BuildId:  "any",
		Metadata: map[string]string{"entrypoint_checksum": "20d5bfff..."},
	}
	pc := classifyPackageConvergence(
		node,
		"foo",
		"SERVICE",
		"1.0.0",
		"",
		"any",
		"", // desired entrypoint empty — no opinion
		false,
		installed,
		time.Now(),
	)
	if pc.RepairRequired {
		t.Errorf("must not require repair when desired entrypoint is empty; reason=%q", pc.Reason)
	}
}

func TestClassifyPackageConvergence_InstalledEntrypointEmpty_NoOpinion(t *testing.T) {
	// Pre-Phase-37 installed_state with no entrypoint_checksum metadata
	// — cannot compare; fall through to RuntimeOK.
	node := &nodeState{
		LastSeen: time.Now(),
		Units: []unitStatusRecord{
			{Name: "globular-foo.service", State: "active"},
		},
	}
	installed := &node_agentpb.InstalledPackage{
		Version: "1.0.0",
		BuildId: "any",
		// No Metadata at all.
	}
	pc := classifyPackageConvergence(
		node,
		"foo",
		"SERVICE",
		"1.0.0",
		"",
		"any",
		"e9434387f92fd3a19fc399fa8d2a9b2f7097f151d0027b4a7d20cccfe22556c8",
		false,
		installed,
		time.Now(),
	)
	if pc.RepairRequired {
		t.Errorf("must not require repair when installed entrypoint is empty; reason=%q", pc.Reason)
	}
}

// TestInfrastructureReleaseConvergence_ResolvedBuildAndEntrypointMatchIgnoresConvergenceHashMismatch
// pins the gateway/envoy/keepalived reinstall-loop fix (2026-07-04).
//
// Real infra installed_state carries the BINARY sha256 in the Checksum field,
// while an InfrastructureRelease's desired_hash is the compact CONVERGENCE
// identity (publisher+name+version+build_number+config) — a DIFFERENT identity
// domain. The two can never be equal. The OLD classifier compared
// installed.GetChecksum() == desiredHash and hard-failed → RepairRequired every
// cycle → perpetual re-dispatch → gateway/envoy restart churn.
//
// The authoritative convergence identity is build_id
// (invariant:convergence.identity_is_build_id) plus the entrypoint binary proof.
// When BOTH agree, the package IS converged and the desired_hash≠binary_checksum
// mismatch must be IGNORED (advisory HashOK only, never a repair authority)
// — forbidden_fix:convergence_hash_compared_to_binary_checksum.
//
// This test FAILS on the old behavior (old dim-2 returns RepairRequired on the
// hash mismatch) and passes only after the fix.
func TestInfrastructureReleaseConvergence_ResolvedBuildAndEntrypointMatchIgnoresConvergenceHashMismatch(t *testing.T) {
	const (
		binaryChecksum  = "7eaec88a0e2f4b1c9d3e5f6a7b8c9d0e1f2a3b4c5d6e7f8091a2b3c4d5e6f7081" // installed binary == resolved entrypoint
		convergenceHash = "8bc776870102030405060708090a0b0c0d0e0f101112131415161718191a1b1c" // desired_hash (compact convergence identity)
		resolvedBuildID = "019f2f7e-1234-7abc-8def-0123456789ab"                              // installed == resolved
	)
	node := &nodeState{
		LastSeen: time.Now(),
		Units: []unitStatusRecord{
			{Name: "globular-gateway.service", State: "active"},
		},
	}
	installed := &node_agentpb.InstalledPackage{
		Version:  "1.2.269",
		Checksum: binaryChecksum, // installed_state records the BINARY sha256 here
		BuildId:  resolvedBuildID,
		Metadata: map[string]string{"entrypoint_checksum": binaryChecksum},
	}
	pc := classifyPackageConvergence(
		node,
		"gateway",
		"INFRASTRUCTURE",
		"1.2.269",
		convergenceHash, // desiredHash — convergence identity, NOT the binary checksum
		resolvedBuildID, // resolved build_id matches installed
		binaryChecksum,  // desired entrypoint matches installed binary
		true,            // requireBuildID — build-backed infra release
		installed,
		time.Now(),
	)
	if pc.RepairRequired {
		t.Fatalf("build_id + entrypoint both match ⇒ CONVERGED; desired_hash≠binary_checksum must be ignored, got RepairRequired reason=%q", pc.Reason)
	}
	if !pc.BuildIDOK {
		t.Errorf("resolved build_id matches installed ⇒ BuildIDOK must be true")
	}
	if pc.HashOK {
		t.Errorf("desired_hash (convergence) ≠ installed binary checksum ⇒ HashOK must be advisory-false, not manufactured true")
	}
	if !pc.RuntimeOK {
		t.Errorf("runtime active ⇒ RuntimeOK, got %s", pc.RuntimeState)
	}
}

// TestInfrastructureReleaseConvergence_EntrypointMismatchStillRequiresRepair is
// the negative guard: ignoring the desired_hash summary must NOT weaken the
// authoritative binary proof. build_id matches but the on-disk entrypoint binary
// differs ⇒ the node is running the wrong bytes ⇒ RepairRequired.
func TestInfrastructureReleaseConvergence_EntrypointMismatchStillRequiresRepair(t *testing.T) {
	const (
		resolvedBuildID   = "019f2f7e-1234-7abc-8def-0123456789ab"
		installedBinary   = "0000000000000000000000000000000000000000000000000000000000000000"
		desiredEntrypoint = "7eaec88a0e2f4b1c9d3e5f6a7b8c9d0e1f2a3b4c5d6e7f8091a2b3c4d5e6f7081"
		convergenceHash   = "8bc776870102030405060708090a0b0c0d0e0f101112131415161718191a1b1c"
	)
	node := &nodeState{
		LastSeen: time.Now(),
		Units: []unitStatusRecord{
			{Name: "globular-gateway.service", State: "active"},
		},
	}
	installed := &node_agentpb.InstalledPackage{
		Version:  "1.2.269",
		Checksum: installedBinary,
		BuildId:  resolvedBuildID,
		Metadata: map[string]string{"entrypoint_checksum": installedBinary}, // OLD/wrong bytes on disk
	}
	pc := classifyPackageConvergence(
		node,
		"gateway",
		"INFRASTRUCTURE",
		"1.2.269",
		convergenceHash,
		resolvedBuildID,   // build_id matches
		desiredEntrypoint, // but desired entrypoint differs from on-disk binary
		true,
		installed,
		time.Now(),
	)
	if !pc.RepairRequired {
		t.Fatalf("entrypoint binary mismatch must require repair even when build_id matches; reason=%q", pc.Reason)
	}
}

func TestNormalizeEntrypointChecksum_Variants(t *testing.T) {
	cases := []struct{ in, want string }{
		{"sha256:ABCDEF", "abcdef"},
		{"  sha256:abcdef  ", "abcdef"},
		{"ABCDEF", "abcdef"},
		{"", ""},
	}
	for _, c := range cases {
		if got := normalizeEntrypointChecksum(c.in); got != c.want {
			t.Errorf("normalizeEntrypointChecksum(%q)=%q want %q", c.in, got, c.want)
		}
	}
}
