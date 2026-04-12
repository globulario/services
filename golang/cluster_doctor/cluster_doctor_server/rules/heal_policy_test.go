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
