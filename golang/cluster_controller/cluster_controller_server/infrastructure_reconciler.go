// @awareness namespace=globular.platform
// @awareness component=platform_cluster_controller.infrastructure_reconciler
// @awareness file_role=infrastructure_release_buildid_and_desired_hash_resolver_with_no_artifact_digest_substitution
// @awareness enforces=globular.platform:invariant.infra.desired_hash_consistency
// @awareness implements=globular.platform:intent.desired_hash.is_convergence_identity
// @awareness risk=critical
package main

// infrastructure_reconciler.go — DesiredHash MUST come from
// ComputeInfrastructureDesiredHash, NEVER from
// ResolvedArtifactDigest. Substituting the artifact blob SHA256
// produced the Envoy restart storm of 2026-05-06: every reconcile
// loop saw a mismatch because the node agent stamps the
// convergence hash on install, not the artifact digest, so the
// controller-side comparison perpetually re-dispatched the install
// workflow.
//
// If a future change "simplifies" this by always using the
// artifact digest, the convergence loop comes back. The schema
// `infra:<publisher>/<component>=<version>+b:<buildNumber>;` is
// the contract — keep both sides of the comparison generating it
// the same way.

// infrastructure_reconciler.go — convergence hash contract enforcement for
// INFRASTRUCTURE packages.
//
// INFRASTRUCTURE desired hashes must be computed with ComputeInfrastructureDesiredHash
// (schema: infra:<publisherID>/<component>=<version>+b:<buildNumber>;).
// Using ResolvedArtifactDigest (raw artifact blob SHA256) as the desired hash
// produces a permanent convergence mismatch loop — the controller always sees a
// mismatch, perpetually re-dispatching the install workflow (Envoy restart storm,
// 2026-05-06).
//
// The correct flow:
//   1. Controller computes convergenceHash = ComputeInfrastructureDesiredHash(...)
//   2. Stored as InfrastructureRelease.Status.DesiredHash
//   3. lookupServiceReleaseBuildID returns DesiredHash (not ResolvedArtifactDigest)
//   4. Workflow dispatched with desired_hash = DesiredHash
//   5. Node agent stamps pkg.Checksum = desired_hash after install (not binary SHA256)
//   6. classifyPackageConvergence compares installedChecksum == convergenceHash → match
//
// Invariants: infra.desired_hash_consistency, convergence.no_infinite_retry

import (
	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

// infraReleaseBuildID extracts the resolved build_id and desired convergence hash
// from an InfrastructureRelease status. Returns DesiredHash (computed by
// ComputeInfrastructureDesiredHash), NOT ResolvedArtifactDigest. Using the raw
// artifact digest as desired_hash causes a permanent mismatch loop because the
// node agent stamps the convergence hash, not the artifact digest.
//
//globular:enforces infra.desired_hash_consistency
//globular:expects_hash_schema infra_desired_hash
//globular:risk convergence.hash_mismatch_loop
func infraReleaseBuildID(rel *cluster_controllerpb.InfrastructureRelease) (buildID, desiredHash string) {
	if rel == nil || rel.Status == nil {
		return "", ""
	}
	return rel.Status.ResolvedBuildID, rel.Status.DesiredHash
}

// infraHashConverged returns true if the installed package's checksum satisfies
// the infrastructure convergence check. Empty convergenceHash means no check is
// required (release not yet resolved). The node agent must stamp pkg.Checksum
// with the convergence hash (not the artifact binary SHA256) after INFRASTRUCTURE
// install; only then will this function return true.
//
//globular:enforces infra.desired_hash_consistency
//globular:expects_hash_schema infra_desired_hash
func infraHashConverged(installedChecksum, convergenceHash string) bool {
	ch := normalizeDesiredHash(convergenceHash)
	if ch == "" {
		return true // no hash required yet
	}
	return normalizeDesiredHash(installedChecksum) == ch
}

// mustNotUseResolvedArtifactDigest returns true when desiredHash is NOT the raw
// artifact digest — meaning the correct hash schema was used. Returns false when
// the two hashes are equal, which signals a bug: the release pipeline used
// ResolvedArtifactDigest as the convergence hash instead of
// ComputeInfrastructureDesiredHash output.
func mustNotUseResolvedArtifactDigest(desiredHash, resolvedArtifactDigest string) bool {
	if desiredHash == "" || resolvedArtifactDigest == "" {
		return true
	}
	return normalizeDesiredHash(desiredHash) != normalizeDesiredHash(resolvedArtifactDigest)
}
