package rules

import (
	"context"
	"testing"

	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

// TestPolicy_ArtifactCacheDigestMismatch_IsHealAuto locks Patch C
// Milestone 3: artifact.cache_digest_mismatch is the single re-enabled
// guarded auto-heal action. Disposition is HealAuto with AutoAction
// "delete_stale_cache"; any change that removes or weakens this requires
// updating the test and explaining the migration plan in the PR.
func TestPolicy_ArtifactCacheDigestMismatch_IsHealAuto(t *testing.T) {
	r := LookupPolicy("artifact.cache_digest_mismatch")
	if r.Disposition != HealAuto {
		t.Fatalf("artifact.cache_digest_mismatch must be HealAuto (M3 re-enable); got %s", r.Disposition)
	}
	if r.AutoAction != "delete_stale_cache" {
		t.Fatalf("AutoAction must be delete_stale_cache; got %q", r.AutoAction)
	}
	// Reverse check: every OTHER mutation-capable invariant the M2 audit
	// flagged remains HealPropose. Locks the "only one guarded auto-heal
	// in M3" rule.
	for _, invariant := range []string{
		"release.stuck_resolved",
		"workflow.drift_stuck",
		"ops_knowledge.seed_deferred",
	} {
		got := LookupPolicy(invariant)
		if got.Disposition != HealPropose {
			t.Fatalf("%s must remain HealPropose (M3 promotes only cache_digest_mismatch); got %s",
				invariant, got.Disposition)
		}
		if got.AutoAction != "" {
			t.Fatalf("%s must have empty AutoAction; got %q", invariant, got.AutoAction)
		}
	}
}

// TestHealer_CacheDigestMismatch_DispatchesThroughExecuteRemediation
// confirms the M3 wiring: a cache_digest_mismatch finding produces
// exactly one Dispatch call (which in production routes through
// ExecuteRemediation), with dryRun forwarded and the canonical AutoAction
// name. The Dispatcher fake records the call; the gatedDispatcher →
// ExecuteRemediation hop is covered by tests in the server package.
func TestHealer_CacheDigestMismatch_DispatchesThroughExecuteRemediation(t *testing.T) {
	dispatcher := &recordingDispatcher{}
	healer := &Healer{
		DryRun:     false,
		Dispatcher: dispatcher,
	}
	findings := []Finding{
		{
			FindingID:   "f-cache-1",
			InvariantID: "artifact.cache_digest_mismatch",
			EntityRef:   "node-uuid/event",
		},
	}
	healer.Evaluate(context.Background(), findings)

	if len(dispatcher.calls) != 1 {
		t.Fatalf("HealAuto cache_digest_mismatch must produce 1 Dispatch call; got %d: %+v",
			len(dispatcher.calls), dispatcher.calls)
	}
	c := dispatcher.calls[0]
	if c.InvariantID != "artifact.cache_digest_mismatch" {
		t.Fatalf("Dispatch invariant_id = %q, want artifact.cache_digest_mismatch", c.InvariantID)
	}
	if c.AutoAction != "delete_stale_cache" {
		t.Fatalf("Dispatch auto_action = %q, want delete_stale_cache", c.AutoAction)
	}
	if c.DryRun {
		t.Fatalf("Dispatch DryRun = true, want false (Healer{DryRun:false})")
	}
}

// TestDeleteCacheArtifact_UsesNodeAgentTypedRPC verifies the rule emits a
// structured DELETE_CACHE_ARTIFACT action — not FILE_DELETE — when
// reporting a cache_digest_mismatch finding. This is the load-bearing
// shape that lets the gated executor route through the typed
// node_agent.DeleteCacheArtifact RPC instead of a generic file-delete
// path (which doesn't exist on the node-agent and was explicitly NOT
// added in M3 to keep the mutation surface narrow).
func TestDeleteCacheArtifact_UsesNodeAgentTypedRPC(t *testing.T) {
	steps := remediationFor("artifact.cache_digest_mismatch",
		"node-uuid", "event", "SERVICE")
	if len(steps) == 0 {
		t.Fatalf("expected remediation steps for cache_digest_mismatch; got 0")
	}
	// Find the first step with a structured action.
	var action *cluster_doctorpb.RemediationAction
	for _, st := range steps {
		if a := st.GetAction(); a != nil {
			action = a
			break
		}
	}
	if action == nil {
		t.Fatalf("expected a structured action on at least one step; got none")
	}
	if action.GetActionType() != cluster_doctorpb.ActionType_DELETE_CACHE_ARTIFACT {
		t.Fatalf("action_type = %s, want DELETE_CACHE_ARTIFACT (FILE_DELETE explicitly rejected in M3)",
			action.GetActionType())
	}
	if action.GetRisk() != cluster_doctorpb.ActionRisk_RISK_LOW {
		t.Fatalf("risk = %s, want RISK_LOW (auto-executable cache cleanup)", action.GetRisk())
	}
	if !action.GetIdempotent() {
		t.Fatalf("action must be marked idempotent (cache delete is reversible via re-fetch)")
	}
	params := action.GetParams()
	if params["node_id"] != "node-uuid" {
		t.Fatalf("params[node_id] = %q, want node-uuid", params["node_id"])
	}
	if params["package_name"] != "event" {
		t.Fatalf("params[package_name] = %q, want event", params["package_name"])
	}
	if params["publisher_id"] == "" {
		t.Fatalf("params[publisher_id] must be set (cache root is publisher-scoped)")
	}
	// Defensive: must not include a `path` param — path is owned by node-agent.
	if _, has := params["path"]; has {
		t.Fatalf("params must NOT include `path` — node-agent owns path construction; got params=%+v", params)
	}
}
