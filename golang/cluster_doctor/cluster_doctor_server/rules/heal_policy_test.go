package rules

import (
	"context"
	"testing"

	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

// TestLookupPolicy_KnownInvariant — post Patch C Milestone 3,
// artifact.cache_digest_mismatch is re-enabled as the single guarded
// auto-heal action and dispatches DELETE_CACHE_ARTIFACT through the
// ExecuteRemediation gate.
func TestLookupPolicy_KnownInvariant(t *testing.T) {
	r := LookupPolicy("artifact.cache_digest_mismatch")
	if r.Disposition != HealAuto {
		t.Fatalf("expected HealAuto (M3 re-enable), got %s", r.Disposition)
	}
	if r.AutoAction != "delete_stale_cache" {
		t.Fatalf("expected AutoAction=delete_stale_cache, got %q", r.AutoAction)
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
// in dry-run mode, the Healer forwards DryRun=true to every Dispatch call
// AND no Executed=true result lands in the report.
func TestHealer_DryRun_NoMutations(t *testing.T) {
	dispatcher := &recordingDispatcher{}
	healer := &Healer{DryRun: true, Dispatcher: dispatcher}
	findings := []Finding{
		{
			InvariantID:     "artifact.cache_digest_mismatch", // HealAuto (M3 re-enable)
			EntityRef:       "node1/event",
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
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

	if got := len(dispatcher.calls); got != 1 {
		t.Fatalf("expected exactly 1 Dispatch call (cache_digest_mismatch in M3), got %d: %+v",
			got, dispatcher.calls)
	}
	if !dispatcher.calls[0].DryRun {
		t.Fatalf("Dispatcher must receive DryRun=true when Healer.DryRun=true; got call=%+v", dispatcher.calls[0])
	}
	if report.AutoFixed != 0 {
		t.Fatalf("expected 0 auto-fixed (dispatcher returned executed=false), got %d", report.AutoFixed)
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
// release.stuck_resolved invariant must NOT be auto-executed.
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
// gated path.
type recordingDispatcher struct {
	calls []dispatchCall
}

type dispatchCall struct {
	InvariantID string
	EntityRef   string
	AutoAction  string
	DryRun      bool
}

func (r *recordingDispatcher) Dispatch(_ context.Context, f Finding, autoAction string, dryRun bool) DispatchResult {
	r.calls = append(r.calls, dispatchCall{
		InvariantID: f.InvariantID,
		EntityRef:   f.EntityRef,
		AutoAction:  autoAction,
		DryRun:      dryRun,
	})
	return DispatchResult{Disposition: DispatchProposed}
}

// TestHealer_PatchReleaseAvailable_IsNotInvoked enforces Patch B's
// invariant: release.stuck_resolved is HealPropose with no AutoAction, so
// the healer never asks the Dispatcher to handle it.
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
			FindingID:       "f-cache-digest",
			InvariantID:     "artifact.cache_digest_mismatch",
			EntityRef:       "eb9a2dac-05b0-52ac-9002-99d8ffd35902/event",
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		},
	}
	report := healer.Evaluate(context.Background(), findings)

	for _, c := range dispatcher.calls {
		if c.AutoAction == "patch_release_available" {
			t.Fatalf("patch_release_available must NOT be dispatched (Patch B); got %+v", c)
		}
		if c.InvariantID == "release.stuck_resolved" {
			t.Fatalf("release.stuck_resolved must NOT reach the Dispatcher (HealPropose only); got %+v", c)
		}
	}
	// release.stuck_resolved is proposed by policy. The conclusive cache failure
	// reaches the dispatcher, whose recording fake returns executed=false, so it
	// is also represented as a proposal.
	if report.Proposed != 2 {
		t.Fatalf("expected 2 proposed (release.stuck_resolved + gated cache action), got Proposed=%d", report.Proposed)
	}
	if report.AutoFixed != 0 {
		t.Fatalf("expected 0 auto-fixed (dispatcher returned executed=false), got AutoFixed=%d", report.AutoFixed)
	}
}