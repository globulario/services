// @awareness namespace=globular.platform
// @awareness component=platform_controller.workflow_release.stale_callback_generation_guard
// @awareness file_role=regression_tests_for_workflow_callback_generation_guard
// @awareness protects=globular.platform:failure_mode.workflow.stale_release_callback_overwrites_generation
// @awareness risk=high
package main

import (
	"context"
	"fmt"
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

// bumpReleaseGenerationTo advances Meta.Generation by mutating the spec until
// the desired generation is reached. The MemStore manages Generation
// internally and only increments it when the spec hash changes — direct
// Meta.Generation writes are overwritten by the store. The Version field is a
// convenient unique-per-bump knob because it has no behavioural effect on
// patchReleasePhaseGuarded.
func bumpReleaseGenerationTo(t *testing.T, srv *server, name string, target int64) {
	t.Helper()
	for i := int64(0); i < 50; i++ {
		obj, _, err := srv.resources.Get(context.Background(), "ServiceRelease", name)
		if err != nil {
			t.Fatalf("bump Get: %v", err)
		}
		rel := obj.(*cluster_controllerpb.ServiceRelease)
		if rel.Meta != nil && rel.Meta.Generation >= target {
			return
		}
		if rel.Spec == nil {
			rel.Spec = &cluster_controllerpb.ServiceReleaseSpec{}
		}
		rel.Spec.Version = fmt.Sprintf("bump-%d", i)
		if _, err := srv.resources.Apply(context.Background(), "ServiceRelease", rel); err != nil {
			t.Fatalf("bump Apply: %v", err)
		}
	}
	t.Fatalf("bumpReleaseGenerationTo: did not reach generation %d after 50 spec bumps", target)
}

// TestWorkflowReleaseCallbackRejectsStaleGeneration pins the generation-guard
// contract for patchReleasePhaseGuarded.
//
// Failure mode workflow.stale_release_callback_overwrites_generation: a
// workflow completion callback that fires after the release generation has
// advanced (e.g. a retry started a new run) must not overwrite the current
// phase. The guard compares the workflow's dispatchGeneration against the
// release's current Meta.Generation. If current > expected, the write is
// silently skipped — the stale callback is dropped.
//
// Three properties asserted:
//
//  1. STALE REJECT — expectedGeneration=1, current=3 → write skipped, phase
//     unchanged, no error returned (silent drop is the contract).
//  2. CURRENT ACCEPT — expectedGeneration=current → write applied.
//  3. AHEAD-OF-STORE ACCEPT — expectedGeneration > current → write applied
//     (the guard rejects only callbacks STRICTLY OLDER than current, not
//     callbacks ahead; ahead means the in-memory state hasn't been
//     refreshed yet, which is fine to commit).
//
// expectedGeneration=0 is the disable sentinel and is covered by every
// existing test that calls these functions with 0; this test asserts the
// non-zero behaviour the failure_mode names.
func TestWorkflowReleaseCallbackRejectsStaleGeneration(t *testing.T) {
	srv := hashTestServer(t)
	seedServiceRelease(t, srv,
		hashSchemaTestReleaseName, hashSchemaTestVersion,
		hashSchemaTestBinarySHA, hashSchemaTestReleaseIdentity,
		hashSchemaTestBuildID)

	// Drive the seeded release to Generation=3 via legitimate spec bumps
	// (the MemStore overwrites direct Meta.Generation writes). Target=3
	// gives room to test stale (1), current (3), and ahead (5).
	bumpReleaseGenerationTo(t, srv, hashSchemaTestReleaseName, 3)
	obj, _, err := srv.resources.Get(context.Background(), "ServiceRelease", hashSchemaTestReleaseName)
	if err != nil {
		t.Fatalf("post-bump Get: %v", err)
	}
	rel := obj.(*cluster_controllerpb.ServiceRelease)
	if rel.Meta.Generation < 3 {
		t.Fatalf("bump did not raise generation; got %d", rel.Meta.Generation)
	}
	currentGen := rel.Meta.Generation

	// Property 1: STALE REJECT — workflow generation predates current.
	staleGen := currentGen - 2
	if err := srv.patchReleasePhaseGuarded(context.Background(),
		"ServiceRelease", hashSchemaTestReleaseName,
		cluster_controllerpb.ReleasePhaseFailed, "stale callback message", staleGen); err != nil {
		t.Fatalf("stale patchReleasePhaseGuarded returned error %v — must drop silently", err)
	}
	got, _, _ := srv.resources.Get(context.Background(), "ServiceRelease", hashSchemaTestReleaseName)
	rel = got.(*cluster_controllerpb.ServiceRelease)
	if rel.Status.Phase == cluster_controllerpb.ReleasePhaseFailed {
		t.Fatalf("stale callback (gen=%d, current=%d) overwrote phase to %q — generation guard failed",
			staleGen, currentGen, rel.Status.Phase)
	}
	if rel.Status.Message == "stale callback message" {
		t.Fatalf("stale callback wrote its reason into Status.Message — write was not skipped")
	}

	// Property 2: CURRENT ACCEPT — workflow generation matches current.
	if err := srv.patchReleasePhaseGuarded(context.Background(),
		"ServiceRelease", hashSchemaTestReleaseName,
		cluster_controllerpb.ReleasePhaseFailed, "current callback", currentGen); err != nil {
		t.Fatalf("current patchReleasePhaseGuarded: %v", err)
	}
	got, _, _ = srv.resources.Get(context.Background(), "ServiceRelease", hashSchemaTestReleaseName)
	rel = got.(*cluster_controllerpb.ServiceRelease)
	if rel.Status.Phase != cluster_controllerpb.ReleasePhaseFailed {
		t.Fatalf("current callback (gen=%d, current=%d) did not apply — phase=%q",
			currentGen, currentGen, rel.Status.Phase)
	}
	if rel.Status.Message != "current callback" {
		t.Fatalf("current callback Message = %q, want %q", rel.Status.Message, "current callback")
	}

	// Reset for property 3: roll the phase back so we can detect the next
	// write. Use the disable sentinel (generation=0) to bypass the guard
	// on this housekeeping write only.
	if err := srv.patchReleasePhaseGuarded(context.Background(),
		"ServiceRelease", hashSchemaTestReleaseName,
		cluster_controllerpb.ReleasePhaseResolved, "reset", 0); err != nil {
		t.Fatalf("reset phase: %v", err)
	}

	// Property 3: AHEAD-OF-STORE ACCEPT — workflow generation is newer
	// than the store's view. The guard rejects only callbacks strictly
	// OLDER than current. current > ahead is false → write proceeds.
	aheadGen := currentGen + 2
	if err := srv.patchReleasePhaseGuarded(context.Background(),
		"ServiceRelease", hashSchemaTestReleaseName,
		cluster_controllerpb.ReleasePhaseAvailable, "ahead callback", aheadGen); err != nil {
		t.Fatalf("ahead patchReleasePhaseGuarded: %v", err)
	}
	got, _, _ = srv.resources.Get(context.Background(), "ServiceRelease", hashSchemaTestReleaseName)
	rel = got.(*cluster_controllerpb.ServiceRelease)
	if rel.Status.Phase != cluster_controllerpb.ReleasePhaseAvailable {
		t.Fatalf("ahead callback (gen=%d, current=%d) was rejected — phase=%q (guard is too strict)",
			aheadGen, currentGen, rel.Status.Phase)
	}
}

// TestWorkflowReleaseCallbackRejectsStaleGeneration_NodeStatus pins the same
// generation-guard contract on patchReleaseNodeStatusGuarded — the per-node
// callback path. The race in
// workflow.stale_release_callback_overwrites_generation can also corrupt
// per-node state via this writer.
func TestWorkflowReleaseCallbackRejectsStaleGeneration_NodeStatus(t *testing.T) {
	srv := hashTestServer(t)
	seedServiceRelease(t, srv,
		hashSchemaTestReleaseName, hashSchemaTestVersion,
		hashSchemaTestBinarySHA, hashSchemaTestReleaseIdentity,
		hashSchemaTestBuildID)

	// Drive generation up via legitimate spec bumps (MemStore overwrites
	// direct Meta.Generation writes).
	bumpReleaseGenerationTo(t, srv, hashSchemaTestReleaseName, 3)
	obj, _, err := srv.resources.Get(context.Background(), "ServiceRelease", hashSchemaTestReleaseName)
	if err != nil {
		t.Fatalf("post-bump Get: %v", err)
	}
	rel := obj.(*cluster_controllerpb.ServiceRelease)
	currentGen := rel.Meta.Generation

	// First write at current generation (accept): seeds a node entry we
	// can detect.
	if err := srv.patchReleaseNodeStatusGuarded(context.Background(),
		"ServiceRelease", hashSchemaTestReleaseName, hashSchemaTestNodeID, currentGen,
		func(n *cluster_controllerpb.NodeReleaseStatus) {
			n.InstalledHash = "binary-sha-from-current-callback"
			n.ProofStatus = RolloutProofInstalledVerified
		}); err != nil {
		t.Fatalf("current node patch: %v", err)
	}

	// Stale callback predating current attempts to overwrite the node
	// entry.
	staleGen := currentGen - 2
	if err := srv.patchReleaseNodeStatusGuarded(context.Background(),
		"ServiceRelease", hashSchemaTestReleaseName, hashSchemaTestNodeID, staleGen,
		func(n *cluster_controllerpb.NodeReleaseStatus) {
			n.InstalledHash = "stale-callback-hash-must-not-appear"
			n.ProofStatus = RolloutProofMismatch
		}); err != nil {
		t.Fatalf("stale node patch returned error %v — must drop silently", err)
	}

	// Verify the stale write did not land.
	got, _, _ := srv.resources.Get(context.Background(), "ServiceRelease", hashSchemaTestReleaseName)
	rel = got.(*cluster_controllerpb.ServiceRelease)
	if len(rel.Status.Nodes) != 1 {
		t.Fatalf("expected 1 node entry, got %d", len(rel.Status.Nodes))
	}
	n := rel.Status.Nodes[0]
	if n.InstalledHash != "binary-sha-from-current-callback" {
		t.Fatalf("stale node callback overwrote InstalledHash: got %q, want %q",
			n.InstalledHash, "binary-sha-from-current-callback")
	}
	if n.ProofStatus != RolloutProofInstalledVerified {
		t.Fatalf("stale node callback overwrote ProofStatus: got %q, want %q",
			n.ProofStatus, RolloutProofInstalledVerified)
	}
}
