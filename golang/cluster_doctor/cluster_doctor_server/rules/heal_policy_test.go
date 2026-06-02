package rules

import (
	"context"
	"testing"
)

// TestLookupPolicy_KnownInvariant — post Patch C Milestone 2, every former
// HealAuto rule with a non-empty AutoAction has been demoted to HealPropose.
// artifact.cache_digest_mismatch is now propose-only; Milestone 3 will
// re-enable it via the gated ExecuteRemediation path.
func TestLookupPolicy_KnownInvariant(t *testing.T) {
	r := LookupPolicy("artifact.cache_digest_mismatch")
	if r.Disposition != HealPropose {
		t.Fatalf("expected HealPropose (M2 demotion), got %s", r.Disposition)
	}
	if r.AutoAction != "" {
		t.Fatalf("expected empty AutoAction (no direct dispatch from healer), got %q", r.AutoAction)
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

// TestHealer_DryRun_NoMutations verifies the fundamental safety contract:
// regardless of DryRun, regardless of disposition, the Healer never
// mutates cluster state directly. After Milestones 1+2, every HealAuto
// rule with a non-empty AutoAction has been demoted to HealPropose, so
// cache_digest_mismatch (formerly HealAuto) now counts as Proposed and
// the Dispatcher receives zero calls.
//
// The dispatcher-recording fake confirms the Path B mutation surface is
// closed: no Dispatch invocations even in non-dry-run mode (when there
// are no HealAuto-with-AutoAction findings).
func TestHealer_DryRun_NoMutations(t *testing.T) {
	dispatcher := &recordingDispatcher{}
	healer := &Healer{DryRun: true, Dispatcher: dispatcher}
	findings := []Finding{
		{
			InvariantID: "artifact.cache_digest_mismatch", // HealPropose post-M2
			EntityRef:   "node1/event",
		},
		{
			InvariantID: "artifact.installed_digest_mismatch", // HealPropose
			EntityRef:   "node1/prometheus",
		},
		{
			InvariantID: "workflow.step_failures", // HealObserve
			EntityRef:   "cluster.reconcile/dispatch",
		},
	}
	report := healer.Evaluate(context.Background(), findings)

	if got := len(dispatcher.calls); got != 0 {
		t.Fatalf("dry-run with all-demoted policy must produce zero Dispatch calls, got %d: %+v", got, dispatcher.calls)
	}
	if report.AutoFixed != 0 {
		t.Fatalf("expected 0 auto-fixed (no HealAuto-with-AutoAction in policy), got %d", report.AutoFixed)
	}
	if report.Proposed != 2 {
		t.Fatalf("expected 2 proposed (cache_digest_mismatch + installed_digest_mismatch), got %d", report.Proposed)
	}
	if report.Observed != 1 {
		t.Fatalf("expected 1 observed (workflow.step_failures), got %d", report.Observed)
	}
	for _, r := range report.Results {
		if r.Executed {
			t.Fatalf("dry-run must produce no Executed=true results, but %s was executed", r.InvariantID)
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

// recordingDispatcher captures every Dispatch invocation so a test can
// assert which auto-actions the healer attempted to route through the
// gated path. Returning (false, "", nil) mirrors the production
// gatedDispatcher's "no RemediationAction representation" branch.
type recordingDispatcher struct {
	calls []dispatchCall
}

type dispatchCall struct {
	InvariantID string
	EntityRef   string
	AutoAction  string
	DryRun      bool
}

func (r *recordingDispatcher) Dispatch(_ context.Context, f Finding, autoAction string, dryRun bool) (bool, string, error) {
	r.calls = append(r.calls, dispatchCall{
		InvariantID: f.InvariantID,
		EntityRef:   f.EntityRef,
		AutoAction:  autoAction,
		DryRun:      dryRun,
	})
	return false, "", nil
}

// TestHealer_PatchReleaseAvailable_IsNotInvoked enforces Patch B's
// invariant: release.stuck_resolved is HealPropose with no AutoAction, so
// the healer never asks the Dispatcher to handle it. Combined with Patch
// C Milestone 2 (delete_stale_cache, seed_ops_knowledge, and
// workflow.drift_stuck also demoted to HealPropose), the Dispatcher
// receives zero calls — the legacy direct-mutation path is closed and
// no replacement auto-route is wired yet.
func TestHealer_PatchReleaseAvailable_IsNotInvoked(t *testing.T) {
	dispatcher := &recordingDispatcher{}
	healer := &Healer{
		DryRun:     false,
		Dispatcher: dispatcher,
	}
	findings := []Finding{
		{
			FindingID:   "f-release-stuck",
			InvariantID: "release.stuck_resolved",
			EntityRef:   "core@globular.io/event",
		},
		{
			FindingID:   "f-cache-digest",
			InvariantID: "artifact.cache_digest_mismatch",
			EntityRef:   "eb9a2dac-05b0-52ac-9002-99d8ffd35902/event",
		},
	}
	report := healer.Evaluate(context.Background(), findings)

	// Filter for any patch_release_available dispatch (defence in depth —
	// even if the auto-action surface grew, this specific action class
	// must never reach the Dispatcher through release.stuck_resolved).
	for _, c := range dispatcher.calls {
		if c.AutoAction == "patch_release_available" {
			t.Fatalf("patch_release_available must NOT be dispatched (Patch B); got %+v", c)
		}
		if c.InvariantID == "release.stuck_resolved" {
			t.Fatalf("release.stuck_resolved must NOT reach the Dispatcher (HealPropose only); got %+v", c)
		}
	}
	// Both findings are demoted to HealPropose in Milestones 1+2, so
	// Proposed=2 and AutoFixed=0.
	if report.Proposed != 2 {
		t.Fatalf("expected 2 proposed (release.stuck_resolved + cache_digest_mismatch), got Proposed=%d", report.Proposed)
	}
	if report.AutoFixed != 0 {
		t.Fatalf("expected 0 auto-fixed (no HealAuto with AutoAction remains), got AutoFixed=%d", report.AutoFixed)
	}
}
