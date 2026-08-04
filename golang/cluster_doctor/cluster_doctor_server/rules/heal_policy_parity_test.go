package rules

import (
	"context"
	"strings"
	"testing"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

// This file guards the defect found on 2026-08-01.
//
// PolicyV1 carried a rule keyed "node.units.not_running". No code anywhere
// emitted that id — nodeUnitsRunning emits "node.systemd.units_running" — so
// LookupPolicy missed and returned the HealObserve default. The rule's stated
// intent ("Restart the failed systemd unit") applied to nothing, for its entire
// existence, without a single error or log line. A finding that carried a
// structured SYSTEMCTL_RESTART action was silently classified "Unknown
// invariant — no policy defined".
//
// WHY THERE IS NO EXHAUSTIVE PARITY TEST HERE
//
// The obvious guard — assert every PolicyV1 id matches a registered invariant —
// produces false positives, because an Invariant's ID() is NOT the only id its
// findings carry. artifactIntegrity{} alone emits artifact.cache_digest_mismatch,
// artifact.desired_version_mismatch, artifact.desired_build_mismatch and more as
// finding-level sub-ids. The authoritative set is "ids that some finding may
// carry at runtime", which no static enumeration exposes today.
//
// Closing that gap properly means having invariants declare their emitted ids
// (e.g. an EmittedIDs() method) so policy coverage becomes checkable. Until
// then this file locks the specific rule that was broken and the specific id
// that was wrong, and the coverage gap is stated rather than papered over with
// a test that cries wolf.

// TestHealPolicy_DeadInvariantIDNotReintroduced pins the exact mistake.
//
// "node.units.not_running" is not a typo of a live id — it is an id nothing has
// ever emitted. If it reappears in the policy, the drifted-unit rule silently
// stops applying again.
func TestHealPolicy_DeadInvariantIDNotReintroduced(t *testing.T) {
	for _, rule := range PolicyV1() {
		if rule.InvariantID == "node.units.not_running" {
			t.Errorf("heal policy reintroduced %q — no invariant emits that id.\n"+
				"nodeUnitsRunning emits %q; LookupPolicy is an exact-match lookup, so this\n"+
				"rule would fall through to HealObserve and apply to nothing.",
				rule.InvariantID, nodeUnitsRunning{}.ID())
		}
	}
}

// TestHealPolicy_DriftedUnitRuleResolvesFromTheEmittedID verifies the policy is
// reachable from the id the rule actually emits — the property that was broken.
//
// Asserting on PolicyV1's contents would not have caught the original bug: the
// rule was present and well-formed, just unreachable. So this goes through
// LookupPolicy with the emitted id, exactly as the healer does.
func TestHealPolicy_DriftedUnitRuleResolvesFromTheEmittedID(t *testing.T) {
	emitted := nodeUnitsRunning{}.ID()

	r := LookupPolicy(emitted)
	if r.Disposition == HealObserve && strings.Contains(r.Action, "Unknown invariant") {
		t.Fatalf("LookupPolicy(%q) fell through to the unknown-invariant default —\n"+
			"the drifted-unit heal rule is unreachable from the id the invariant emits", emitted)
	}
	if r.InvariantID != emitted {
		t.Errorf("LookupPolicy(%q) resolved to rule %q", emitted, r.InvariantID)
	}
}

// TestHealPolicy_DriftedUnitRestartIsAuto locks the Milestone-3 promotion.
//
// This is the first rule allowed to execute without human approval, and it is
// safe only because the dispatch traverses ExecuteRemediation, where behavioral
// governance requires a promoted principle and the observed finding as evidence.
// Demoting it back to HealPropose should be deliberate and reviewed; removing
// the AutoAction is worse than demoting, because the healer then treats the rule
// as an informational no-op and reports it as observed — which looks identical
// to healing while nothing is repaired.
func TestHealPolicy_DriftedUnitRestartIsAuto(t *testing.T) {
	r := LookupPolicy(nodeUnitsRunning{}.ID())

	if r.Disposition != HealAuto {
		t.Errorf("%s disposition = %q, want %q.\n"+
			"This is the Milestone-3 auto-heal rule; demoting it should be a deliberate,\n"+
			"reviewed change, not an accident.", nodeUnitsRunning{}.ID(), r.Disposition, HealAuto)
	}
	if r.AutoAction == "" {
		t.Error("the drifted-unit rule is HealAuto with an empty AutoAction.\n" +
			"The healer treats that as an informational no-op and reports it as observed —\n" +
			"indistinguishable from a rule that healed, while nothing was repaired.")
	}
}

// TestHealPolicy_DriftedUnitRoutesToDispatchUnderRealPolicy proves the
// Milestone-3 behavior change at the policy/healer seam, using the REAL PolicyV1.
//
// The existing routing test injects a synthetic HealAuto rule, so it proves the
// wiring works but says nothing about whether any real rule reaches it. That is
// precisely the gap the dead invariant id hid in: routing was fine, and no real
// finding ever arrived. This runs a genuine drifted-unit finding through the
// real policy and asserts a dispatch actually happens.
//
// In production that dispatch lands on gatedDispatcher → ExecuteRemediation,
// where behavioral governance requires the promoted principle and the observed
// finding as evidence before anything runs. This covers the leg from policy to
// dispatch; the gate beyond it is covered in behavioral_governance_test.go.
func TestHealPolicy_DriftedUnitRoutesToDispatchUnderRealPolicy(t *testing.T) {
	emitted := nodeUnitsRunning{}.ID()
	unit := "globular-torrent.service"

	dispatcher := &recordingDispatcher{}
	healer := &Healer{
		DryRun:     false,
		Dispatcher: dispatcher,
		// No PolicyLookup override: the REAL PolicyV1 is under test.
	}

	findings := []Finding{{
		FindingID:   "f-unit-1",
		InvariantID: emitted,
		EntityRef:   "node-1/" + unit,
		// A drifted unit is a conclusive failure, and remediation now requires
		// one: the healer refuses any auto-action whose finding is not a FAIL
		// with closed evidence. Leaving this at the zero value made the fixture
		// INVARIANT_UNKNOWN, which the policy seam under test never sees in
		// production for a genuinely drifted unit.
		InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		Remediation: []*cluster_doctorpb.RemediationStep{
			actionStep(1, "Restart the drifted unit", "",
				systemctlRestartAction(unit, "node-1")),
		},
	}}

	report := healer.Evaluate(context.Background(), findings)

	if len(dispatcher.calls) != 1 {
		t.Fatalf("a drifted globular-* unit must reach the Dispatcher under the real policy; "+
			"got %d dispatch call(s). report=%+v\n"+
			"Before 2026-08-01 this was 0: the policy rule was keyed to an id nothing emits, "+
			"so the finding was classified 'Unknown invariant — no policy defined'.",
			len(dispatcher.calls), report)
	}
	if got := dispatcher.calls[0].InvariantID; got != emitted {
		t.Errorf("dispatched invariant_id = %q, want %q", got, emitted)
	}
	if dispatcher.calls[0].AutoAction == "" {
		t.Error("dispatch carried an empty auto_action — the healer would treat the rule " +
			"as an informational no-op and report success while repairing nothing")
	}
}

// TestHealPolicy_UnknownInvariantStaysObserve verifies the safe default holds.
// Whatever else changes, an invariant the policy does not cover must never
// acquire a mutating disposition by accident.
func TestHealPolicy_UnknownInvariantStaysObserve(t *testing.T) {
	r := LookupPolicy("totally.unknown.invariant.that.no.rule.emits")
	if r.Disposition != HealObserve {
		t.Errorf("unknown invariant disposition = %q, want %q — the policy must never "+
			"mutate what it does not explicitly cover", r.Disposition, HealObserve)
	}
}
