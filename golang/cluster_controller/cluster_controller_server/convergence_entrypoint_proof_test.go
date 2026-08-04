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

func TestClassifyPackageConvergence_ChecksumOnlyDriftWithBuildIDAndEntrypointDoesNotRepair(t *testing.T) {
	node := &nodeState{
		LastSeen: time.Now(),
		Units: []unitStatusRecord{
			{Name: "globular-envoy.service", State: "active"},
		},
	}
	const (
		binarySHA       = "241c1702f0ed1c0dba31339abaab422906a4295cc92640f5b832c131ee385767"
		convergenceHash = "cfd6de59b3ecdb00f5b8430d1d8cc5cc6c00e8e88468dfb96d47f2fbe9425212"
		buildID         = "019f0000-1111-7222-8333-444455556666"
	)
	installed := &node_agentpb.InstalledPackage{
		Version:  "1.35.3",
		Checksum: binarySHA,
		BuildId:  buildID,
		Metadata: map[string]string{
			"entrypoint_checksum": binarySHA,
		},
	}
	pc := classifyPackageConvergence(
		node,
		"envoy",
		"INFRASTRUCTURE",
		"1.35.3",
		convergenceHash,
		buildID,
		binarySHA,
		true,
		installed,
		time.Now(),
	)
	if pc.RepairRequired {
		t.Fatalf("checksum-only drift with matching build_id+entrypoint must not repair; reason=%q", pc.Reason)
	}
	if !pc.HashOK || !pc.BuildIDOK || !pc.RuntimeOK {
		t.Fatalf("expected converged despite overloaded Checksum field; hash=%v build=%v runtime=%v reason=%q",
			pc.HashOK, pc.BuildIDOK, pc.RuntimeOK, pc.Reason)
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
