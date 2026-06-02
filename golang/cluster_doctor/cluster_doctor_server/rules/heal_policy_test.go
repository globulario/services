package rules

import (
	"context"
	"testing"
)

func TestLookupPolicy_KnownInvariant(t *testing.T) {
	r := LookupPolicy("artifact.cache_digest_mismatch")
	if r.Disposition != HealAuto {
		t.Fatalf("expected HealAuto, got %s", r.Disposition)
	}
	if r.AutoAction != "delete_stale_cache" {
		t.Fatalf("expected delete_stale_cache, got %s", r.AutoAction)
	}
}

func TestLookupPolicy_ProposeOnly(t *testing.T) {
	r := LookupPolicy("artifact.installed_digest_mismatch")
	if r.Disposition != HealPropose {
		t.Fatalf("expected HealPropose, got %s", r.Disposition)
	}
}

func TestLookupPolicy_ObserveOnly(t *testing.T) {
	r := LookupPolicy("workflow.step_failures")
	if r.Disposition != HealObserve {
		t.Fatalf("expected HealObserve, got %s", r.Disposition)
	}
}

func TestLookupPolicy_UnknownDefaultsToObserve(t *testing.T) {
	r := LookupPolicy("some.unknown.invariant.id")
	if r.Disposition != HealObserve {
		t.Fatalf("unknown invariant should default to HealObserve, got %s", r.Disposition)
	}
}

func TestLookupPolicy_WildcardMatch(t *testing.T) {
	r := LookupPolicy("pending.repo.reachable")
	if r.Disposition != HealObserve {
		t.Fatalf("pending.* should match HealObserve, got %s", r.Disposition)
	}
}

func TestPolicyV1_NoDuplicateInvariants(t *testing.T) {
	seen := make(map[string]bool)
	for _, r := range PolicyV1() {
		if seen[r.InvariantID] {
			t.Fatalf("duplicate invariant ID in policy: %s", r.InvariantID)
		}
		seen[r.InvariantID] = true
	}
}

func TestHealer_DryRun_NoMutations(t *testing.T) {
	healer := &Healer{DryRun: true}
	findings := []Finding{
		{
			InvariantID: "artifact.cache_digest_mismatch",
			EntityRef:   "node1/event",
		},
		{
			InvariantID: "artifact.installed_digest_mismatch",
			EntityRef:   "node1/prometheus",
		},
		{
			InvariantID: "workflow.step_failures",
			EntityRef:   "cluster.reconcile/dispatch",
		},
	}
	report := healer.Evaluate(context.Background(), findings)
	if report.AutoFixed != 1 {
		t.Fatalf("expected 1 auto-fixed (dry-run), got %d", report.AutoFixed)
	}
	if report.Proposed != 1 {
		t.Fatalf("expected 1 proposed, got %d", report.Proposed)
	}
	if report.Observed != 1 {
		t.Fatalf("expected 1 observed, got %d", report.Observed)
	}
	// In dry-run, nothing should actually execute.
	for _, r := range report.Results {
		if r.Executed {
			t.Fatalf("dry-run should not execute actions, but %s was executed", r.InvariantID)
		}
	}
}

func TestHealer_CacheMissing_NoOp(t *testing.T) {
	healer := &Healer{DryRun: false}
	findings := []Finding{
		{
			InvariantID: "artifact.cache_missing",
			EntityRef:   "node1/search",
		},
	}
	report := healer.Evaluate(context.Background(), findings)
	// cache_missing has AutoAction="" → no-op, classified as observed
	if report.Observed != 1 {
		t.Fatalf("expected 1 observed for cache_missing no-op, got %d", report.Observed)
	}
	if report.Results[0].Verified != true {
		t.Fatalf("cache_missing should be auto-verified (no-op)")
	}
}

// TestPolicy_ReleaseStuckResolved_IsPropose locks Patch B: the
// release.stuck_resolved invariant must NOT be auto-executed. Its concrete
// repair (patch_release_available) is a direct etcd write against the
// ServiceRelease object, which the action executor hard-blocks for Path A
// (executor.go hardBlocked()) — until Path B routes through the same gate,
// the policy disposition is propose-only and the AutoAction field is empty.
//
// Regression guard: a future change that flips this rule back to HealAuto
// without unifying the remediation path must fail this test.
func TestPolicy_ReleaseStuckResolved_IsPropose(t *testing.T) {
	r := LookupPolicy("release.stuck_resolved")
	if r.Disposition != HealPropose {
		t.Fatalf("release.stuck_resolved must be HealPropose (audit Patch B), got %s", r.Disposition)
	}
	if r.AutoAction != "" {
		t.Fatalf("release.stuck_resolved must have an empty AutoAction (no direct etcd writes from the background healer), got %q", r.AutoAction)
	}
}

// recordingRemoteOps captures every RemoteOps invocation so a test can
// assert which actions the healer attempted to dispatch. All methods
// return nil so the healer treats them as successful when DryRun=false.
type recordingRemoteOps struct {
	deleteCacheCalls   []string
	patchReleaseCalls  []string
	clearDriftCalls    []string
	seedOpsKnowledgeOK int
	convergedReply     bool
}

func (r *recordingRemoteOps) DeleteCacheArtifact(_ context.Context, nodeID, packageName, publisherID string) error {
	r.deleteCacheCalls = append(r.deleteCacheCalls, nodeID+"/"+packageName+"@"+publisherID)
	return nil
}

func (r *recordingRemoteOps) PatchReleasePhase(_ context.Context, releaseName, newPhase, reason string) error {
	r.patchReleaseCalls = append(r.patchReleaseCalls, releaseName+"->"+newPhase+":"+reason)
	return nil
}

func (r *recordingRemoteOps) ClearDriftObservation(_ context.Context, clusterID, driftType, entityRef string) error {
	r.clearDriftCalls = append(r.clearDriftCalls, clusterID+":"+driftType+":"+entityRef)
	return nil
}

func (r *recordingRemoteOps) IsServiceConverged(_ context.Context, _ string) (bool, error) {
	return r.convergedReply, nil
}

func (r *recordingRemoteOps) SeedOpsKnowledge(_ context.Context, _ string) error {
	r.seedOpsKnowledgeOK++
	return nil
}

// TestHealer_PatchReleaseAvailable_IsNotInvoked enforces the behavioural
// half of Patch B: even in enforce mode (DryRun=false), even when the
// guard condition (IsServiceConverged=true) would have allowed it, the
// healer must never dispatch patch_release_available for a
// release.stuck_resolved finding — because the rule no longer carries an
// AutoAction. The control case (artifact.cache_digest_mismatch) keeps the
// HealAuto path firing so the test fails on an over-zealous patch that
// disables auto-healing globally.
func TestHealer_PatchReleaseAvailable_IsNotInvoked(t *testing.T) {
	remote := &recordingRemoteOps{convergedReply: true}
	healer := &Healer{
		DryRun: false,
		Remote: remote,
	}
	findings := []Finding{
		{
			InvariantID: "release.stuck_resolved",
			EntityRef:   "core@globular.io/event",
		},
		// Control: cache_digest_mismatch is still HealAuto so the healer
		// machinery is exercised. If this fires, the test infrastructure
		// works; the patch_release_available absence is therefore
		// meaningful (not a false negative from a broken Evaluate).
		// NodeID must be ≥8 chars — actionDeleteStaleCache logs nodeID[:8]
		// for trace context, see healer.go:210.
		{
			InvariantID: "artifact.cache_digest_mismatch",
			EntityRef:   "eb9a2dac-05b0-52ac-9002-99d8ffd35902/event",
		},
	}
	report := healer.Evaluate(context.Background(), findings)

	if got := len(remote.patchReleaseCalls); got != 0 {
		t.Fatalf("PatchReleasePhase must NOT be invoked for release.stuck_resolved (Patch B); got %d call(s): %v",
			got, remote.patchReleaseCalls)
	}
	if got := len(remote.deleteCacheCalls); got != 1 {
		t.Fatalf("control: expected exactly 1 DeleteCacheArtifact call for cache_digest_mismatch, got %d (%v)",
			got, remote.deleteCacheCalls)
	}
	// release.stuck_resolved is now HealPropose → counted under Proposed.
	if report.Proposed != 1 {
		t.Fatalf("release.stuck_resolved must classify as Proposed (HealPropose), got Proposed=%d", report.Proposed)
	}
}
