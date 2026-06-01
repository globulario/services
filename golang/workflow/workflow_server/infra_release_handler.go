package main

// infra_release_handler.go — infrastructure desired-hash schema validation for
// the workflow server.
//
// When the controller dispatches a release.apply.package workflow for an
// INFRASTRUCTURE package, the inputs must carry desired_hash computed by
// ComputeInfrastructureDesiredHash (schema: infra:<pub>/<comp>=<ver>+b:<build>;).
// Using ResolvedArtifactDigest (raw artifact blob SHA256) as desired_hash is a
// schema violation that causes the node agent's post-install stamp to mismatch
// what the controller expects, creating a perpetual redispatch loop.
//
// This file provides a pure-function validator that the executor can call when
// routing INFRASTRUCTURE install workflows, and serves as the audit boundary
// that enforces the hash schema on the workflow-execution path.
//
// Invariant: infra.desired_hash_consistency

import "strings"

// infraDesiredHashValid returns true when desired_hash for an INFRASTRUCTURE
// workflow is valid: it is either empty (no constraint yet) or differs from the
// raw artifact digest. Equal hashes signal a schema violation — the controller
// used ResolvedArtifactDigest instead of ComputeInfrastructureDesiredHash.
//
//globular:enforces infra.desired_hash_consistency
//globular:expects_hash_schema infra_desired_hash
//globular:risk convergence.hash_mismatch_loop
func infraDesiredHashValid(desiredHash, resolvedArtifactDigest string) bool {
	if desiredHash == "" {
		return true // no hash required yet
	}
	dh := normalizeInfraHash(desiredHash)
	ra := normalizeInfraHash(resolvedArtifactDigest)
	// If desired_hash == artifact_digest the wrong schema was used.
	return ra == "" || dh != ra
}

// infraWorkflowInputsValid validates that an INFRASTRUCTURE workflow's inputs
// carry the convergence hash, not the raw artifact digest. Returns false if the
// inputs contain a schema violation that would cause a convergence mismatch loop.
func infraWorkflowInputsValid(inputs map[string]any) bool {
	pkgKind, _ := inputs["package_kind"].(string)
	if strings.ToUpper(pkgKind) != "INFRASTRUCTURE" {
		return true // only validate INFRASTRUCTURE workflows
	}
	desiredHash, _ := inputs["desired_hash"].(string)
	artifactDigest, _ := inputs["resolved_artifact_digest"].(string)
	return infraDesiredHashValid(desiredHash, artifactDigest)
}

func normalizeInfraHash(h string) string {
	return strings.ToLower(strings.TrimSpace(strings.TrimPrefix(h, "sha256:")))
}
