package main

// globular:tested_by desired_hash_consistency

import "testing"

// TestDriftWorkflowUsesDesiredHash verifies that INFRASTRUCTURE drift workflows
// carry the convergence hash (computed by ComputeInfrastructureDesiredHash), not
// the raw artifact digest (ResolvedArtifactDigest). When the two hashes are
// accidentally equal, the controller has violated the infra_desired_hash schema
// and the node agent's post-install stamp will never match the controller's
// expectation — causing a perpetual redispatch loop (the Envoy restart storm,
// 2026-05-06).
//
// Invariant: infra.desired_hash_consistency
func TestDriftWorkflowUsesDesiredHash(t *testing.T) {
	// Simulate the convergence hash produced by ComputeInfrastructureDesiredHash.
	// In production this is SHA256 of "infra:<pub>/<comp>=<ver>+b:<build>;".
	convergenceHash := "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2"
	// Simulate the raw artifact SHA256 from the repository manifest.
	artifactDigest := "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef"

	// Case 1: convergence hash differs from artifact digest — correct.
	if !infraDesiredHashValid(convergenceHash, artifactDigest) {
		t.Error("different hashes should be valid — correct schema use")
	}

	// Case 2: schema violation — desired_hash equals artifact digest.
	// This is the bug: the controller used ResolvedArtifactDigest as desired_hash.
	if infraDesiredHashValid(artifactDigest, artifactDigest) {
		t.Error("desired_hash == artifact_digest is a schema violation — must return false")
	}

	// Case 3: empty desired_hash is always valid (release not yet resolved).
	if !infraDesiredHashValid("", artifactDigest) {
		t.Error("empty desired_hash is always valid")
	}

	// Case 4: empty artifact digest means no comparison — always valid.
	if !infraDesiredHashValid(convergenceHash, "") {
		t.Error("empty artifact_digest means no comparison — should be valid")
	}

	// Case 5: infraWorkflowInputsValid rejects INFRASTRUCTURE inputs with
	// schema violation, but passes SERVICE inputs and correctly hashed INFRA.
	badInfraInputs := map[string]any{
		"package_kind":             "INFRASTRUCTURE",
		"desired_hash":             artifactDigest, // schema violation
		"resolved_artifact_digest": artifactDigest,
	}
	if infraWorkflowInputsValid(badInfraInputs) {
		t.Error("INFRASTRUCTURE workflow with desired_hash == artifact_digest should fail validation")
	}

	goodInfraInputs := map[string]any{
		"package_kind":             "INFRASTRUCTURE",
		"desired_hash":             convergenceHash, // correct schema
		"resolved_artifact_digest": artifactDigest,
	}
	if !infraWorkflowInputsValid(goodInfraInputs) {
		t.Error("INFRASTRUCTURE workflow with convergence hash should pass validation")
	}

	serviceInputs := map[string]any{
		"package_kind": "SERVICE",
		"desired_hash": artifactDigest, // SERVICE schema is different — not validated here
	}
	if !infraWorkflowInputsValid(serviceInputs) {
		t.Error("SERVICE workflow inputs should not be validated by infraWorkflowInputsValid")
	}
}
