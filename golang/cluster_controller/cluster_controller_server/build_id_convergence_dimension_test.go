package main

// build_id_convergence_dimension_test.go — D3: build_id is an INDEPENDENT
// convergence dimension (thread & require). No hash-schema change.
//
// classifyPackageConvergence enforces build_id when present, and — for
// build-backed artifacts (requireBuildID) — refuses a missing desired build_id
// rather than silently skipping identity. Legitimately build-less records may
// skip the dimension. Invariants: package.identity_tuple_must_be_unique,
// desired.build_id_immutable_after_resolution.

import (
	"strings"
	"testing"
	"time"

	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

// Test 1: same version + same hash, DIFFERENT build_id → BuildIDOK=false,
// RepairRequired=true. "Same version" must not hide "different build".
func TestBuildIDDimension_DifferentBuildIDNotConverged(t *testing.T) {
	pc := classifyPackageConvergence(
		&nodeState{}, "echo", "SERVICE",
		"1.0.0", "", // version, hash ("" → HashOK)
		"build-A", "", // desiredBuildID present, no entrypoint
		true, // requireBuildID (enforced regardless when build_id present)
		&node_agentpb.InstalledPackage{Version: "1.0.0", BuildId: "build-B"},
		time.Now(),
	)
	if pc.BuildIDOK {
		t.Fatal("a different installed build_id must NOT be BuildIDOK")
	}
	if !pc.RepairRequired || !strings.Contains(pc.Reason, "build_id") {
		t.Fatalf("expected build_id repair; got ok=%v repair=%v reason=%q", pc.BuildIDOK, pc.RepairRequired, pc.Reason)
	}
}

// Test 2: build-backed (requireBuildID=true) with EMPTY desired build_id →
// "missing desired build identity", non-converged (no silent skip).
func TestBuildIDDimension_BuildBackedMissingBuildIDFailsClosed(t *testing.T) {
	pc := classifyPackageConvergence(
		&nodeState{}, "echo", "SERVICE",
		"1.0.0", "", "", "", // version, hash, EMPTY build_id, entrypoint
		true, // requireBuildID — build-backed
		&node_agentpb.InstalledPackage{Version: "1.0.0", BuildId: "build-X"},
		time.Now(),
	)
	if pc.BuildIDOK {
		t.Fatal("missing build_id on a build-backed artifact must NOT be BuildIDOK")
	}
	if !pc.RepairRequired || !strings.Contains(pc.Reason, "missing desired build identity") {
		t.Fatalf("expected missing-build-identity repair; got ok=%v reason=%q", pc.BuildIDOK, pc.Reason)
	}
}

// Test 3: build-less (requireBuildID=false) with empty desired build_id → the
// dimension is skipped (BuildIDOK). No over-enforcement of runtime-only/command
// records.
func TestBuildIDDimension_BuildLessMissingBuildIDSkips(t *testing.T) {
	pc := classifyPackageConvergence(
		&nodeState{}, "rclone", "COMMAND",
		"1.2.3", "", "", "", // version, hash, EMPTY build_id, entrypoint
		false, // requireBuildID=false — legitimately build-less
		&node_agentpb.InstalledPackage{Version: "1.2.3"},
		time.Now(),
	)
	if !pc.BuildIDOK {
		t.Fatalf("build-less record with empty build_id must skip the dimension (BuildIDOK); reason=%q", pc.Reason)
	}
}

// Test 3b: build_id present and MATCHING → BuildIDOK (the normal converged path),
// regardless of requireBuildID.
func TestBuildIDDimension_MatchingBuildIDConverges(t *testing.T) {
	pc := classifyPackageConvergence(
		&nodeState{}, "echo", "SERVICE",
		"1.0.0", "", "build-A", "",
		true,
		&node_agentpb.InstalledPackage{Version: "1.0.0", BuildId: "build-A"},
		time.Now(),
	)
	if !pc.BuildIDOK {
		t.Fatalf("matching build_id must be BuildIDOK; reason=%q", pc.Reason)
	}
}
