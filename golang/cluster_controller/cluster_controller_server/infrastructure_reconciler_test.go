package main

// globular:tested_by desired_hash_consistency

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

// TestInfraDesiredHashConsistency verifies the three invariants of the
// infrastructure convergence hash contract:
//
//  1. ComputeInfrastructureDesiredHash is stable (same inputs → same output).
//  2. The convergence hash does NOT equal the artifact SHA256. If they were
//     equal, the controller would report convergence before the node agent has
//     installed the correct binary — or vice versa, creating a loop.
//  3. infraHashConverged correctly distinguishes: convergenceHash matches
//     itself (converged) but does NOT match the artifact digest (not converged).
//
// Invariant: infra.desired_hash_consistency
func TestInfraDesiredHashConsistency(t *testing.T) {
	pubID := "globulario"
	component := "envoy"
	version := "1.29.4"
	buildNum := int64(42)

	hash1 := ComputeInfrastructureDesiredHash(pubID, component, version, buildNum)
	hash2 := ComputeInfrastructureDesiredHash(pubID, component, version, buildNum)

	if hash1 == "" {
		t.Fatal("ComputeInfrastructureDesiredHash returned empty string")
	}
	if hash1 != hash2 {
		t.Errorf("hash not stable: first=%s second=%s", hash1, hash2)
	}

	// Simulate the artifact digest: SHA256 of the artifact blob bytes.
	// This is what node_agent historically stamped in pkg.Checksum after install,
	// causing the mismatch loop.
	artifactBlob := []byte("envoy-1.29.4-linux_amd64.tgz-fake-content")
	sum := sha256.Sum256(artifactBlob)
	artifactDigest := hex.EncodeToString(sum[:])

	// The convergence hash MUST differ from the artifact digest.
	// If they're equal, the hash schema was violated.
	if hash1 == artifactDigest {
		t.Error("convergence hash must not equal artifact digest — hash schema violation would cause install loop")
	}

	// infraHashConverged: convergence hash vs itself → converged.
	if !infraHashConverged(hash1, hash1) {
		t.Error("infraHashConverged: convergence hash should satisfy itself")
	}

	// infraHashConverged: artifact digest vs convergence hash → NOT converged.
	// This is the bug scenario: node_agent stamps artifact digest, controller
	// expects convergence hash — mismatch causes the Envoy restart storm.
	if infraHashConverged(artifactDigest, hash1) {
		t.Error("infraHashConverged: artifact digest must NOT satisfy convergence hash check (this is the bug)")
	}

	// infraHashConverged: empty convergenceHash → always passes (not yet published).
	if !infraHashConverged(artifactDigest, "") {
		t.Error("infraHashConverged: empty convergenceHash should always pass")
	}

	// mustNotUseResolvedArtifactDigest: different hashes → valid.
	if !mustNotUseResolvedArtifactDigest(hash1, artifactDigest) {
		t.Error("mustNotUseResolvedArtifactDigest: different hashes are valid (correct schema)")
	}

	// mustNotUseResolvedArtifactDigest: same value → invalid (schema mismatch).
	if mustNotUseResolvedArtifactDigest(artifactDigest, artifactDigest) {
		t.Error("mustNotUseResolvedArtifactDigest: identical hashes indicate schema mismatch — should return false")
	}

	// Verify different components produce different hashes.
	hashOther := ComputeInfrastructureDesiredHash(pubID, "etcd", version, buildNum)
	if hash1 == hashOther {
		t.Error("different components must produce different hashes")
	}
}

// TestLookupServiceReleaseBuildIDUsesDesiredHash verifies that infraReleaseBuildID
// returns InfrastructureRelease.Status.DesiredHash (the convergence hash, computed
// by ComputeInfrastructureDesiredHash), NOT ResolvedArtifactDigest (the raw
// artifact blob SHA256). Using the wrong hash as desired_hash in workflow inputs
// causes a permanent convergence mismatch loop.
//
// Invariant: infra.desired_hash_consistency
func TestLookupServiceReleaseBuildIDUsesDesiredHash(t *testing.T) {
	convergenceHash := ComputeInfrastructureDesiredHash("globulario", "envoy", "1.29.4", 42)
	artifactDigest := "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"

	rel := &cluster_controllerpb.InfrastructureRelease{
		Meta: &cluster_controllerpb.ObjectMeta{Name: "globulario/envoy"},
		Spec: &cluster_controllerpb.InfrastructureReleaseSpec{
			PublisherID: "globulario",
			Component:   "envoy",
			Version:     "1.29.4",
		},
		Status: &cluster_controllerpb.InfrastructureReleaseStatus{
			ResolvedBuildID:        "build-uuid-0042",
			DesiredHash:            convergenceHash,
			ResolvedArtifactDigest: artifactDigest,
		},
	}

	gotBuildID, gotHash := infraReleaseBuildID(rel)

	if gotBuildID != "build-uuid-0042" {
		t.Errorf("expected build_id %q, got %q", "build-uuid-0042", gotBuildID)
	}

	// The returned hash MUST be the convergence hash, not the artifact digest.
	if gotHash != convergenceHash {
		t.Errorf("expected convergence hash %q, got %q (artifact digest would be %q)", convergenceHash, gotHash, artifactDigest)
	}
	if gotHash == artifactDigest {
		t.Errorf("infraReleaseBuildID returned artifact digest instead of convergence hash — this would cause a mismatch loop")
	}

	// Nil release returns empty strings.
	bid, h := infraReleaseBuildID(nil)
	if bid != "" || h != "" {
		t.Error("infraReleaseBuildID(nil) should return empty strings")
	}

	// Release with nil status returns empty strings.
	bid2, h2 := infraReleaseBuildID(&cluster_controllerpb.InfrastructureRelease{})
	if bid2 != "" || h2 != "" {
		t.Error("infraReleaseBuildID with nil status should return empty strings")
	}
}
