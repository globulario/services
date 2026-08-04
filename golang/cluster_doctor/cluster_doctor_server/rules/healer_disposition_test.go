package rules

import (
	"context"
	"testing"

	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

// HealReport counter proof.
//
// EXECUTION IS AN EVENT. CONVERGENCE IS THE SUCCESSFUL OUTCOME.
//
// AutoFixed used to count anything the dispatcher reported as executed, so a
// restart that completed while the invariant stayed violated was reported as an
// auto-fix. These tests pin the honest meaning of every counter, and pin that a
// governed refusal is never charged to the executor failure budget.

// scriptedDispatcher returns a fixed DispatchResult and counts invocations, so a
// test can assert both the classification and how many dispatches the circuit
// breaker allowed.
type scriptedDispatcher struct {
	result DispatchResult
	calls  int
}

func (d *scriptedDispatcher) Dispatch(context.Context, Finding, string, bool) DispatchResult {
	d.calls++
	return d.result
}

func healAutoFinding(id string) Finding {
	return Finding{
		FindingID:       id,
		InvariantID:     "synthetic.heal_auto",
		EntityRef:       "test/" + id,
		InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
	}
}

func healAutoPolicy(string) HealRule {
	return HealRule{
		InvariantID: "synthetic.heal_auto",
		Disposition: HealAuto,
		AutoAction:  "synthetic_action",
	}
}

func TestHealReport_CounterMatrix(t *testing.T) {
	for _, tc := range []struct {
		name   string
		result DispatchResult
		want   HealReport
		why    string
	}{
		{
			name: "converged is the only auto-fix",
			result: DispatchResult{
				Disposition: DispatchConverged, Executed: true, Verified: true, Converged: true,
				WorkflowRunID: "run-1", ActionCheckID: "chk-1",
			},
			want: HealReport{AutoFixed: 1},
			why:  "ran AND post-action evidence proved the finding cleared",
		},
		{
			name: "executed but not converged is a failed repair",
			result: DispatchResult{
				Disposition: DispatchExecutedNotConverged, Executed: true, Verified: true,
			},
			want: HealReport{ExecutedNotConverged: 1, Errors: 1},
			why:  "the action ran and the cluster is still broken — not an auto-fix",
		},
		{
			name: "executed but unverified is neither success nor failure",
			result: DispatchResult{
				Disposition: DispatchExecutedUnverified, Executed: true,
			},
			want: HealReport{ExecutedUnverified: 1},
			why:  "a real mutation with unknown convergence must not be claimed either way",
		},
		{
			name:   "execution failure counts as an error",
			result: DispatchResult{Disposition: DispatchExecutionFailed},
			want:   HealReport{ExecutionFailed: 1, Errors: 1},
			why:    "the run could not start or the executor failed",
		},
		{
			name:   "governed refusal counts only as refused",
			result: DispatchResult{Disposition: DispatchRefused, ActionCheckID: "chk-refused"},
			want:   HealReport{Refused: 1},
			why:    "a refusal is governance working, not a malfunction",
		},
		{
			name:   "proposed counts only as proposed",
			result: DispatchResult{Disposition: DispatchProposed},
			want:   HealReport{Proposed: 1},
			why:    "nothing was attempted",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := &scriptedDispatcher{result: tc.result}
			h := &Healer{Dispatcher: d, PolicyLookup: healAutoPolicy}
			got := h.Evaluate(context.Background(), []Finding{healAutoFinding("f-1")})

			if got.AutoFixed != tc.want.AutoFixed {
				t.Errorf("AutoFixed = %d, want %d — %s", got.AutoFixed, tc.want.AutoFixed, tc.why)
			}
			if got.Errors != tc.want.Errors {
				t.Errorf("Errors = %d, want %d — %s", got.Errors, tc.want.Errors, tc.why)
			}
			if got.ExecutionFailed != tc.want.ExecutionFailed {
				t.Errorf("ExecutionFailed = %d, want %d", got.ExecutionFailed, tc.want.ExecutionFailed)
			}
			if got.ExecutedNotConverged != tc.want.ExecutedNotConverged {
				t.Errorf("ExecutedNotConverged = %d, want %d", got.ExecutedNotConverged, tc.want.ExecutedNotConverged)
			}
			if got.ExecutedUnverified != tc.want.ExecutedUnverified {
				t.Errorf("ExecutedUnverified = %d, want %d", got.ExecutedUnverified, tc.want.ExecutedUnverified)
			}
			if got.Refused != tc.want.Refused {
				t.Errorf("Refused = %d, want %d — %s", got.Refused, tc.want.Refused, tc.why)
			}
			if got.Proposed != tc.want.Proposed {
				t.Errorf("Proposed = %d, want %d", got.Proposed, tc.want.Proposed)
			}
		})
	}
}

// TestHealReport_ActionCheckIDReachesTheReport proves the governance decision
// survives to the serialized report for refused and successful attempts alike.
// A decision an operator cannot trace back to the attempt it authorized is not
// accountable.
func TestHealReport_ActionCheckIDReachesTheReport(t *testing.T) {
	for _, tc := range []struct {
		name   string
		result DispatchResult
	}{
		{"converged", DispatchResult{Disposition: DispatchConverged, Executed: true, Verified: true,
			Converged: true, ActionCheckID: "chk-success", WorkflowRunID: "run-1"}},
		{"refused", DispatchResult{Disposition: DispatchRefused, ActionCheckID: "chk-refused",
			GovernanceStatus: "needs_evidence"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := &scriptedDispatcher{result: tc.result}
			h := &Healer{Dispatcher: d, PolicyLookup: healAutoPolicy}
			report := h.Evaluate(context.Background(), []Finding{healAutoFinding("f-1")})

			if len(report.Results) != 1 {
				t.Fatalf("results = %d, want 1", len(report.Results))
			}
			r := report.Results[0]
			if r.ActionCheckID != tc.result.ActionCheckID {
				t.Errorf("ActionCheckID = %q, want %q — the decision must reach the report",
					r.ActionCheckID, tc.result.ActionCheckID)
			}
			if r.DispatchDisposition != tc.result.Disposition {
				t.Errorf("DispatchDisposition = %q, want %q", r.DispatchDisposition, tc.result.Disposition)
			}
			if tc.result.GovernanceStatus != "" && r.GovernanceStatus != tc.result.GovernanceStatus {
				t.Errorf("GovernanceStatus = %q, want %q", r.GovernanceStatus, tc.result.GovernanceStatus)
			}
			if r.WorkflowRunID != tc.result.WorkflowRunID {
				t.Errorf("WorkflowRunID = %q, want %q", r.WorkflowRunID, tc.result.WorkflowRunID)
			}
		})
	}
}

// TestHealer_CircuitBreaker_RefusalDoesNotCharge proves a governed refusal never
// consumes the executor failure budget.
//
// If it did, governance could disable the healer by working correctly: a run of
// refusals would trip the breaker and stop the doctor from attempting repairs
// that were never malfunctioning in the first place.
func TestHealer_CircuitBreaker_RefusalDoesNotCharge(t *testing.T) {
	findings := []Finding{
		healAutoFinding("f-1"), healAutoFinding("f-2"),
		healAutoFinding("f-3"), healAutoFinding("f-4"),
	}

	t.Run("refusals never trip the breaker", func(t *testing.T) {
		d := &scriptedDispatcher{result: DispatchResult{Disposition: DispatchRefused}}
		h := &Healer{Dispatcher: d, PolicyLookup: healAutoPolicy, MaxFailures: 2}
		report := h.Evaluate(context.Background(), findings)

		if d.calls != len(findings) {
			t.Errorf("dispatched %d time(s), want %d — refusals must not trip the breaker, "+
				"or correct governance would disable the healer", d.calls, len(findings))
		}
		if report.Refused != len(findings) {
			t.Errorf("Refused = %d, want %d", report.Refused, len(findings))
		}
		if report.Errors != 0 {
			t.Errorf("Errors = %d, want 0 — a refusal is not a failed mutation", report.Errors)
		}
	})

	t.Run("execution failures do trip the breaker", func(t *testing.T) {
		d := &scriptedDispatcher{result: DispatchResult{Disposition: DispatchExecutionFailed}}
		h := &Healer{Dispatcher: d, PolicyLookup: healAutoPolicy, MaxFailures: 2}
		h.Evaluate(context.Background(), findings)

		if d.calls != 2 {
			t.Errorf("dispatched %d time(s), want 2 — the breaker must stop execution after "+
				"MaxFailures real failures", d.calls)
		}
	})

	t.Run("verified non-convergence also trips the breaker", func(t *testing.T) {
		d := &scriptedDispatcher{result: DispatchResult{
			Disposition: DispatchExecutedNotConverged, Executed: true, Verified: true,
		}}
		h := &Healer{Dispatcher: d, PolicyLookup: healAutoPolicy, MaxFailures: 2}
		h.Evaluate(context.Background(), findings)

		if d.calls != 2 {
			t.Errorf("dispatched %d time(s), want 2 — repeating a repair that verifiably does not "+
				"converge is what the breaker exists to stop", d.calls)
		}
	})
}
