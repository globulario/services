package main

import (
	"context"
	"testing"

	ai_executorpb "github.com/globulario/services/golang/ai_executor/ai_executorpb"
)

// TestRemediator_UnknownTierFailsSafeNoAutoExecute is the ratchet for
// meta.least_privilege_is_not_a_default_it_is_an_explicit_grant. The only path
// that takes a real action (auto-remediate) must be an EXPLICIT grant (tier==1),
// never the fall-through. tier is a raw int32 (proto3, no enum), so any value
// other than the three defined tiers — buggy caller, peer proposal, future proto
// tier, garbled field — must default to a NON-executing outcome (approval /
// skip), never EXECUTING. Before 2026-06-09 the execute path was the else branch,
// so any unrecognized tier auto-remediated without approval.
func TestRemediator_UnknownTierFailsSafeNoAutoExecute(t *testing.T) {
	r := &remediator{} // non-execute tiers never touch the dispatcher

	diag := &ai_executorpb.Diagnosis{
		IncidentId:     "inc-test",
		ProposedAction: "restart_service",
		ActionReason:   "test",
	}

	// Only tier 1 may reach EXECUTING. Every other value — including ones that
	// don't exist yet — must NOT auto-execute.
	for _, tier := range []int32{-1, 3, 4, 7, 99, 2147483647} {
		act := r.execute(context.Background(), diag, tier)
		if act.GetStatus() == ai_executorpb.ActionStatus_ACTION_EXECUTING {
			t.Errorf("tier %d auto-executed — an unrecognized tier must fail safe to "+
				"approval-required, never auto-remediate (least-privilege: the dangerous "+
				"action is an explicit grant, not the default)", tier)
		}
		// Fail-safe outcome is approval-required (PENDING).
		if act.GetStatus() != ai_executorpb.ActionStatus_ACTION_PENDING {
			t.Errorf("tier %d: expected fail-safe ACTION_PENDING (approval-required), got %v",
				tier, act.GetStatus())
		}
	}

	// Tier 0 (observe) and tier 2 (approval) must also never execute.
	if r.execute(context.Background(), diag, 0).GetStatus() == ai_executorpb.ActionStatus_ACTION_EXECUTING {
		t.Error("tier 0 (observe) must not execute")
	}
	if r.execute(context.Background(), diag, 2).GetStatus() == ai_executorpb.ActionStatus_ACTION_EXECUTING {
		t.Error("tier 2 (approval) must not execute")
	}
}
