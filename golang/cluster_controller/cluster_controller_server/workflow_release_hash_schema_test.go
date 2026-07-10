// @awareness namespace=globular.platform
// @awareness component=platform_controller.workflow_release.hash_schemas_must_not_alias
// @awareness file_role=regression_tests_for_workflow_callback_writer_2_no_release_identity_into_installed_hash
// @awareness enforces=globular.platform:invariant.install_package.hash_schemas_must_not_alias
// @awareness protects=globular.platform:failure_mode.node_agent.install_package_aliases_convergence_hash_into_expected_sha256
// @awareness risk=high
package main

import (
	"context"
	"strings"
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_controller/resourcestore"
)

// These regression tests pin the post-fix contract for the workflow callback
// writer of NodeReleaseStatus.InstalledHash (workflow_release.go writer #2):
//
//	InstalledHash must be the binary entrypoint SHA from the release record's
//	resolved manifest, NEVER the workflow's release-identity `hash` parameter
//	produced by ComputeReleaseDesiredHash(publisher, name, version,
//	build_number, config).
//
// The pre-fix bug: MarkNodeSucceeded wrote `n.InstalledHash = hash` where
// `hash` was the workflow input `$.desired_hash`. Per the awareness graph
// invariant `install_package.hash_schemas_must_not_alias`, the install-package
// workflow's two hash inputs (release identity for routing/selection; binary
// SHA for verify) are distinct schemas and must never share a single storage
// slot.
//
// Fix shape (workflow_release.go):
//   1. `MarkNodeSucceeded` now ignores the `hash` parameter for storage
//      (it stays in the log line for forensic visibility).
//   2. Looks up rel.Status.ResolvedEntrypointChecksum via the new
//      `lookupReleaseResolvedEntrypointChecksum` helper.
//   3. Writes the binary SHA into InstalledHash; on empty (legacy artifact)
//      demotes to RolloutProofInventoryClaim — never aliases.
//
// Same fix applied to buildGenericReleaseControllerConfig's MarkNodeSucceeded.

// hashTestServer constructs a minimal server backed by an in-memory resource
// store. Only the fields touched by the workflow callbacks need to be wired.
func hashTestServer(t *testing.T) *server {
	t.Helper()
	srv := newServer(defaultClusterControllerConfig(), "", "", newControllerState(), nil)
	srv.resources = resourcestore.NewMemStore()
	srv.setLeader(true, "leader", "127.0.0.1:1234")
	return srv
}

func seedServiceRelease(t *testing.T, srv *server, releaseName, version, resolvedBinarySHA, desiredHashReleaseIdentity, buildID string) {
	t.Helper()
	rel := &cluster_controllerpb.ServiceRelease{
		Meta: &cluster_controllerpb.ObjectMeta{Name: releaseName, Generation: 1},
		Spec: &cluster_controllerpb.ServiceReleaseSpec{
			PublisherID: "core@globular.io",
			ServiceName: strings.TrimPrefix(releaseName, "core@globular.io/"),
			Version:     version,
		},
		Status: &cluster_controllerpb.ServiceReleaseStatus{
			Phase:                      cluster_controllerpb.ReleasePhaseResolved,
			ResolvedVersion:            version,
			ResolvedBuildID:            buildID,
			ResolvedEntrypointChecksum: resolvedBinarySHA,
			DesiredHash:                desiredHashReleaseIdentity,
		},
	}
	if _, err := srv.resources.Apply(context.Background(), "ServiceRelease", rel); err != nil {
		t.Fatalf("seed ServiceRelease: %v", err)
	}
}

const (
	// Real values from the live 3-node cluster (2026-06-04):
	hashSchemaTestBinarySHA       = "2eb07b676e82c0bb41b4449e4390d489fb4082242a3e53be9200120b41bc1ff5"
	hashSchemaTestReleaseIdentity = "5804f52337edfc3281ae14f876572f961d4840a0aff52dcd078152c0900c00a4"
	hashSchemaTestBuildID         = "019e924c-d699-7068-b03d-d52a0ab545ff"
	hashSchemaTestVersion         = "1.2.160"
	hashSchemaTestReleaseName     = "core@globular.io/cluster-controller"
	hashSchemaTestNodeID          = "eb9a2dac-05b0-52ac-9002-99d8ffd35902"
)

// TestMarkNodeSucceeded_StoresBinarySHA_NotReleaseIdentity is the headline
// regression: a successful workflow callback must put the binary SHA from
// rel.Status.ResolvedEntrypointChecksum into NodeReleaseStatus.InstalledHash,
// not the workflow `hash` parameter (which carries release-identity).
func TestMarkNodeSucceeded_StoresBinarySHA_NotReleaseIdentity(t *testing.T) {
	srv := hashTestServer(t)
	seedServiceRelease(t, srv,
		hashSchemaTestReleaseName, hashSchemaTestVersion,
		hashSchemaTestBinarySHA, hashSchemaTestReleaseIdentity,
		hashSchemaTestBuildID)

	cfg := srv.buildReleaseControllerConfigWithGen(hashSchemaTestReleaseName, "SERVICE", 0)

	// Workflow engine passes release-identity into `hash` — the exact pre-fix
	// bug shape. Post-fix, this value must NOT survive into InstalledHash.
	if err := cfg.MarkNodeSucceeded(context.Background(),
		"release-id-ignored", hashSchemaTestNodeID,
		hashSchemaTestVersion, hashSchemaTestReleaseIdentity); err != nil {
		t.Fatalf("MarkNodeSucceeded: %v", err)
	}

	obj, _, err := srv.resources.Get(context.Background(), "ServiceRelease", hashSchemaTestReleaseName)
	if err != nil {
		t.Fatalf("Get ServiceRelease: %v", err)
	}
	rel := obj.(*cluster_controllerpb.ServiceRelease)
	if rel.Status == nil || len(rel.Status.Nodes) != 1 {
		t.Fatalf("rel.Status.Nodes: want 1 entry, got %+v", rel.Status)
	}
	n := rel.Status.Nodes[0]
	if n.NodeID != hashSchemaTestNodeID {
		t.Fatalf("NodeID = %q, want %q", n.NodeID, hashSchemaTestNodeID)
	}
	if !strings.EqualFold(n.InstalledHash, hashSchemaTestBinarySHA) {
		t.Fatalf("InstalledHash = %q, want binary SHA %q (release-identity %q must NOT survive)",
			n.InstalledHash, hashSchemaTestBinarySHA, hashSchemaTestReleaseIdentity)
	}
	if strings.EqualFold(n.InstalledHash, hashSchemaTestReleaseIdentity) {
		t.Fatalf("CRITICAL: workflow callback stamped release-identity hash %q into InstalledHash — the fix is not active",
			hashSchemaTestReleaseIdentity)
	}
	if n.ProofStatus != RolloutProofInstalledVerified {
		t.Fatalf("ProofStatus = %q, want %q", n.ProofStatus, RolloutProofInstalledVerified)
	}
}

// TestMarkNodeSucceeded_Generic_StoresBinarySHA_NotReleaseIdentity covers
// the buildGenericReleaseControllerConfig path used for orphan-run resumption.
func TestMarkNodeSucceeded_Generic_StoresBinarySHA_NotReleaseIdentity(t *testing.T) {
	srv := hashTestServer(t)
	seedServiceRelease(t, srv,
		hashSchemaTestReleaseName, hashSchemaTestVersion,
		hashSchemaTestBinarySHA, hashSchemaTestReleaseIdentity,
		hashSchemaTestBuildID)

	cfg := srv.buildGenericReleaseControllerConfig()

	if err := cfg.MarkNodeSucceeded(context.Background(),
		hashSchemaTestReleaseName, hashSchemaTestNodeID,
		hashSchemaTestVersion, hashSchemaTestReleaseIdentity); err != nil {
		t.Fatalf("Generic MarkNodeSucceeded: %v", err)
	}

	obj, _, _ := srv.resources.Get(context.Background(), "ServiceRelease", hashSchemaTestReleaseName)
	rel := obj.(*cluster_controllerpb.ServiceRelease)
	if rel.Status == nil || len(rel.Status.Nodes) != 1 {
		t.Fatalf("rel.Status.Nodes: want 1 entry, got %+v", rel.Status)
	}
	if !strings.EqualFold(rel.Status.Nodes[0].InstalledHash, hashSchemaTestBinarySHA) {
		t.Fatalf("Generic InstalledHash = %q, want %q",
			rel.Status.Nodes[0].InstalledHash, hashSchemaTestBinarySHA)
	}
}

// TestMarkNodeSucceeded_EmptyResolvedChecksum_FallsBackToInventoryClaim
// exercises the legacy-artifact / pre-Phase-38 path: when the manifest had no
// entrypoint_checksum, ResolvedEntrypointChecksum is empty. The callback
// MUST demote to RolloutProofInventoryClaim and NOT alias the workflow
// release-identity `hash` into InstalledHash as a "better than nothing"
// fallback — that was the exact pre-fix bug.
func TestMarkNodeSucceeded_EmptyResolvedChecksum_FallsBackToInventoryClaim(t *testing.T) {
	srv := hashTestServer(t)
	// Resolved SHA empty — legacy / pre-bootstrap artifact.
	seedServiceRelease(t, srv,
		hashSchemaTestReleaseName, hashSchemaTestVersion,
		"", hashSchemaTestReleaseIdentity, hashSchemaTestBuildID)

	cfg := srv.buildReleaseControllerConfigWithGen(hashSchemaTestReleaseName, "SERVICE", 0)
	if err := cfg.MarkNodeSucceeded(context.Background(),
		"release-id-ignored", hashSchemaTestNodeID,
		hashSchemaTestVersion, hashSchemaTestReleaseIdentity); err != nil {
		t.Fatalf("MarkNodeSucceeded: %v", err)
	}

	obj, _, _ := srv.resources.Get(context.Background(), "ServiceRelease", hashSchemaTestReleaseName)
	rel := obj.(*cluster_controllerpb.ServiceRelease)
	if rel.Status == nil || len(rel.Status.Nodes) != 1 {
		t.Fatalf("rel.Status.Nodes: want 1 entry, got %+v", rel.Status)
	}
	n := rel.Status.Nodes[0]
	if n.InstalledHash != "" {
		t.Fatalf("InstalledHash should be empty when resolved checksum is empty; got %q (workflow_hash=%q must NOT alias)",
			n.InstalledHash, hashSchemaTestReleaseIdentity)
	}
	if strings.EqualFold(n.InstalledHash, hashSchemaTestReleaseIdentity) {
		t.Fatalf("CRITICAL: empty-resolved path aliased workflow release-identity into InstalledHash")
	}
	if n.ProofStatus != RolloutProofInventoryClaim {
		t.Fatalf("ProofStatus = %q, want %q (no binary proof → inventory_claim)",
			n.ProofStatus, RolloutProofInventoryClaim)
	}
}

// TestLookupReleaseResolvedEntrypointChecksum_ReturnsBinarySHA verifies the
// helper returns the binary SHA from the rel's Status.ResolvedEntrypointChecksum,
// not DesiredHash. This is the lookup the callback uses; if it returned the
// wrong field, the entire fix would silently invert.
func TestLookupReleaseResolvedEntrypointChecksum_ReturnsBinarySHA(t *testing.T) {
	srv := hashTestServer(t)
	seedServiceRelease(t, srv,
		hashSchemaTestReleaseName, hashSchemaTestVersion,
		hashSchemaTestBinarySHA, hashSchemaTestReleaseIdentity,
		hashSchemaTestBuildID)

	got := srv.lookupReleaseResolvedEntrypointChecksum(context.Background(),
		"ServiceRelease", hashSchemaTestReleaseName)

	if !strings.EqualFold(got, hashSchemaTestBinarySHA) {
		t.Fatalf("lookup = %q, want binary SHA %q", got, hashSchemaTestBinarySHA)
	}
	if strings.EqualFold(got, hashSchemaTestReleaseIdentity) {
		t.Fatalf("CRITICAL: helper returned the release-identity hash %q — it must source from Status.ResolvedEntrypointChecksum, not Status.DesiredHash",
			hashSchemaTestReleaseIdentity)
	}
}

func TestLookupServiceReleaseConvergenceIdentityIncludesEntrypointChecksum(t *testing.T) {
	srv := hashTestServer(t)
	const (
		binarySHA       = "241c1702f0ed1c0dba31339abaab422906a4295cc92640f5b832c131ee385767"
		convergenceHash = "cfd6de59b3ecdb00f5b8430d1d8cc5cc6c00e8e88468dfb96d47f2fbe9425212"
		buildID         = "019f0000-1111-7222-8333-444455556666"
	)
	rel := &cluster_controllerpb.InfrastructureRelease{
		Meta: &cluster_controllerpb.ObjectMeta{Name: defaultPublisherID() + "/envoy"},
		Spec: &cluster_controllerpb.InfrastructureReleaseSpec{
			PublisherID: defaultPublisherID(),
			Component:   "envoy",
			Version:     "1.35.3",
			BuildNumber: 7,
		},
		Status: &cluster_controllerpb.InfrastructureReleaseStatus{
			ResolvedBuildID:            buildID,
			ResolvedBuildNumber:        8,
			ResolvedEntrypointChecksum: binarySHA,
			DesiredHash:                convergenceHash,
		},
	}
	if _, err := srv.resources.Apply(context.Background(), "InfrastructureRelease", rel); err != nil {
		t.Fatalf("seed InfrastructureRelease: %v", err)
	}

	got := srv.lookupServiceReleaseConvergenceIdentity(context.Background(), "envoy")
	if got.resolvedBuildID != buildID {
		t.Fatalf("resolvedBuildID = %q, want %q", got.resolvedBuildID, buildID)
	}
	if got.desiredHash != convergenceHash {
		t.Fatalf("desiredHash = %q, want convergence hash %q", got.desiredHash, convergenceHash)
	}
	if got.resolvedEntrypointChecksum != binarySHA {
		t.Fatalf("resolvedEntrypointChecksum = %q, want binary SHA %q", got.resolvedEntrypointChecksum, binarySHA)
	}
	if got.resolvedBuildNumber != 8 {
		t.Fatalf("resolvedBuildNumber = %d, want resolved build number 8", got.resolvedBuildNumber)
	}
	if got.desiredHash == got.resolvedEntrypointChecksum {
		t.Fatalf("desired hash and entrypoint checksum must remain distinct schemas")
	}
}

// TestLookupReleaseResolvedEntrypointChecksum_MissingRelease_ReturnsEmpty
// pins the safe-by-default behaviour: a missing or unreadable release record
// returns "", which the caller maps to inventory_claim.
func TestLookupReleaseResolvedEntrypointChecksum_MissingRelease_ReturnsEmpty(t *testing.T) {
	srv := hashTestServer(t)
	got := srv.lookupReleaseResolvedEntrypointChecksum(context.Background(),
		"ServiceRelease", "core@globular.io/never-existed")
	if got != "" {
		t.Fatalf("missing release lookup returned %q, want empty", got)
	}
}

// TestCallbackAfterConvergenceScan_DoesNotReintroduceFalseDrift simulates the
// ordering hazard: a prior convergence-scan write (release_pipeline.go:1320)
// put the binary SHA into NodeReleaseStatus.InstalledHash; then a workflow
// callback fires (potentially racing). The callback's write must also be a
// binary SHA — both writers must agree on the same hash kind, so neither
// "wins" with a different schema.
func TestCallbackAfterConvergenceScan_DoesNotReintroduceFalseDrift(t *testing.T) {
	srv := hashTestServer(t)
	seedServiceRelease(t, srv,
		hashSchemaTestReleaseName, hashSchemaTestVersion,
		hashSchemaTestBinarySHA, hashSchemaTestReleaseIdentity,
		hashSchemaTestBuildID)

	// Step 1: simulate convergence scan stamping binary SHA into InstalledHash.
	if err := srv.patchReleaseNodeStatusGuarded(context.Background(),
		"ServiceRelease", hashSchemaTestReleaseName, hashSchemaTestNodeID, 0,
		func(n *cluster_controllerpb.NodeReleaseStatus) {
			n.InstalledHash = hashSchemaTestBinarySHA
			n.ProofStatus = RolloutProofInstalledVerified
		}); err != nil {
		t.Fatalf("convergence-scan patch: %v", err)
	}

	// Step 2: workflow callback fires AFTER the scan.
	cfg := srv.buildReleaseControllerConfigWithGen(hashSchemaTestReleaseName, "SERVICE", 0)
	if err := cfg.MarkNodeSucceeded(context.Background(),
		"release-id-ignored", hashSchemaTestNodeID,
		hashSchemaTestVersion, hashSchemaTestReleaseIdentity); err != nil {
		t.Fatalf("MarkNodeSucceeded: %v", err)
	}

	// Step 3: assert InstalledHash is STILL the binary SHA — the callback
	// did not overwrite with the workflow's release-identity value.
	obj, _, _ := srv.resources.Get(context.Background(), "ServiceRelease", hashSchemaTestReleaseName)
	rel := obj.(*cluster_controllerpb.ServiceRelease)
	n := rel.Status.Nodes[0]
	if !strings.EqualFold(n.InstalledHash, hashSchemaTestBinarySHA) {
		t.Fatalf("callback-after-scan: InstalledHash = %q, want %q (the workflow `hash` parameter must NOT win this race)",
			n.InstalledHash, hashSchemaTestBinarySHA)
	}
	if strings.EqualFold(n.InstalledHash, hashSchemaTestReleaseIdentity) {
		t.Fatalf("CRITICAL: workflow callback overwrote a correct binary SHA with the release-identity hash")
	}
}

// TestLegacyRecord_WithReleaseIdentityChecksum_StillConvergesViaResolvedField
// proves the fix handles already-poisoned records: a NodeReleaseStatus that
// carries a release-identity InstalledHash from a pre-fix dispatch will be
// repaired by the next workflow callback, which now stamps the correct binary
// SHA from rel.Status.ResolvedEntrypointChecksum.
func TestLegacyRecord_WithReleaseIdentityChecksum_StillConvergesViaResolvedField(t *testing.T) {
	srv := hashTestServer(t)
	seedServiceRelease(t, srv,
		hashSchemaTestReleaseName, hashSchemaTestVersion,
		hashSchemaTestBinarySHA, hashSchemaTestReleaseIdentity,
		hashSchemaTestBuildID)

	// Seed the legacy/pre-fix shape: NodeReleaseStatus.InstalledHash already
	// poisoned with release-identity hash by an older controller binary.
	if err := srv.patchReleaseNodeStatusGuarded(context.Background(),
		"ServiceRelease", hashSchemaTestReleaseName, hashSchemaTestNodeID, 0,
		func(n *cluster_controllerpb.NodeReleaseStatus) {
			n.InstalledHash = hashSchemaTestReleaseIdentity // poisoned
			n.ProofStatus = RolloutProofMismatch
			n.ProofFinding = FindingRolloutInstalledHashMismatch
		}); err != nil {
		t.Fatalf("seed legacy state: %v", err)
	}

	// Run the post-fix callback once.
	cfg := srv.buildReleaseControllerConfigWithGen(hashSchemaTestReleaseName, "SERVICE", 0)
	if err := cfg.MarkNodeSucceeded(context.Background(),
		"release-id-ignored", hashSchemaTestNodeID,
		hashSchemaTestVersion, hashSchemaTestReleaseIdentity); err != nil {
		t.Fatalf("MarkNodeSucceeded: %v", err)
	}

	// The poisoned value must be replaced by the binary SHA.
	obj, _, _ := srv.resources.Get(context.Background(), "ServiceRelease", hashSchemaTestReleaseName)
	rel := obj.(*cluster_controllerpb.ServiceRelease)
	n := rel.Status.Nodes[0]
	if strings.EqualFold(n.InstalledHash, hashSchemaTestReleaseIdentity) {
		t.Fatalf("legacy poisoned hash survived callback — repair did not happen")
	}
	if !strings.EqualFold(n.InstalledHash, hashSchemaTestBinarySHA) {
		t.Fatalf("post-callback InstalledHash = %q, want repaired binary SHA %q",
			n.InstalledHash, hashSchemaTestBinarySHA)
	}
	if n.ProofStatus != RolloutProofInstalledVerified {
		t.Fatalf("ProofStatus = %q, want %q after repair",
			n.ProofStatus, RolloutProofInstalledVerified)
	}
	if n.ProofFinding != "" {
		t.Fatalf("ProofFinding = %q, want empty after repair", n.ProofFinding)
	}
}
