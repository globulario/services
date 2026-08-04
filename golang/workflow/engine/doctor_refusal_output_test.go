package engine

import (
	"context"
	"fmt"
	"testing"

	"github.com/globulario/services/golang/remediation"
)

// Real-execution proof that a governed refusal produces STRUCTURED output that
// survives the failing step.
//
// This test exists because of P1-A. The refusal contract was written, reviewed
// and believed, but the classifier looked for a key
// (execution_result.governance_refused) that no code path ever wrote, and the
// execute step returned its error before writing execution_result at all. The
// refusal was therefore unclassifiable in production while every unit test that
// constructed synthetic outputs passed.
//
// So this drives the REAL workflow through the REAL definition and asserts on
// the run's own accumulated outputs. Synthetic OutputsJson cannot prove that the
// producer and the consumer agree — only execution can.

// refusalDoctor scripts a governed refusal and counts every downstream call, so
// the test can prove nothing executed rather than merely that it reported so.
type refusalDoctor struct {
	fakeDoctor
	verdict     *GateVerdict
	gateErr     error
	gateCalls   int
	observeCall int
	observed    []remediation.Outcome
}

func (r *refusalDoctor) config() DoctorRemediationConfig {
	cfg := r.fakeDoctor.config()
	cfg.GateAction = func(ctx context.Context, req GateRequest) (GateVerdict, error) {
		r.gateCalls++
		if r.gateErr != nil {
			return GateVerdict{}, r.gateErr
		}
		return *r.verdict, nil
	}
	cfg.ObserveOutcome = func(ctx context.Context, o remediation.Outcome) {
		r.observeCall++
		r.observed = append(r.observed, o)
	}
	return cfg
}

func newRefusalDoctor() *refusalDoctor {
	return &refusalDoctor{
		fakeDoctor: fakeDoctor{
			resolved: &ResolvedFinding{
				NodeID: "node-1", ActionType: "SYSTEMCTL_RESTART", Risk: "RISK_LOW",
				Idempotent: true, HasAction: true,
				ClusterID: "c1", InvariantID: "node.systemd.units_running",
				EntityRef: "node-1/globular-log.service",
			},
			execResult: &ExecutionResult{AuditID: "rem-should-not-happen", Executed: true},
			verify:     &Verification{Converged: true},
		},
	}
}

// TestDoctorWorkflow_GovernedRefusalWritesStructuredOutput proves the refusal
// path end to end: written before the error, retained in run outputs, and
// carrying the real decision identity.
func TestDoctorWorkflow_GovernedRefusalWritesStructuredOutput(t *testing.T) {
	doc := newRefusalDoctor()
	doc.verdict = &GateVerdict{
		ActionCheckID: "chk-refused-1",
		Governed:      true,
		Allowed:       false,
		Status:        "needs_evidence",
		Reason:        "required evidence does not qualify",
	}

	run := runDoctorWorkflow(t, doc.config(), map[string]any{
		"finding_id":           "f-refused",
		"step_index":           0,
		"dry_run":              false,
		"finding_binding_mode": FindingBindingOperatorCurrent,
	})

	if run.Status == RunSucceeded {
		t.Fatal("a governed refusal must not produce a succeeded run")
	}

	// The refusal output must have SURVIVED the step's error into run outputs.
	dr, ok := run.Outputs["dispatch_result"].(map[string]any)
	if !ok {
		t.Fatalf("dispatch_result missing from run outputs (have %v) — this is exactly P1-A: "+
			"the refusal is unclassifiable because the step errored before recording it",
			outputKeys(run.Outputs))
	}
	if got := dr["disposition"]; got != DispositionRefused {
		t.Errorf("disposition = %v, want %q", got, DispositionRefused)
	}
	if got := dr["action_check_id"]; got != "chk-refused-1" {
		t.Errorf("action_check_id = %v, want chk-refused-1 — a refusal an operator cannot trace "+
			"back to its decision is not accountable", got)
	}
	if got := dr["status"]; got != "needs_evidence" {
		t.Errorf("governance status = %v, want needs_evidence", got)
	}
	if executed, _ := dr["executed"].(bool); executed {
		t.Error("a refusal must never claim dispatch")
	}

	// Nothing may have happened downstream.
	if doc.gateCalls != 1 {
		t.Errorf("gate consulted %d time(s), want exactly 1 — one action, one decision", doc.gateCalls)
	}
	if doc.execCalls != 0 {
		t.Errorf("executor called %d time(s), want 0 — a refusal must perform no side effect", doc.execCalls)
	}
	if doc.verifyCalls != 0 {
		t.Errorf("verifier called %d time(s), want 0 — nothing ran, so there is nothing to verify", doc.verifyCalls)
	}
	if doc.observeCall != 0 {
		t.Errorf("outcome observed %d time(s), want 0 — nothing executed, so no remediation "+
			"outcome may claim dispatch or verification", doc.observeCall)
	}
	if _, present := run.Outputs["execution_result"]; present {
		t.Error("execution_result must be absent — the executor was never reached")
	}
}

// TestDoctorWorkflow_GovernanceUnavailableIsDistinctFromRefusal proves the two
// stop the action for different reasons and must be classified differently: a
// refusal is a decision, an unreachable governor is infrastructure failure and
// must charge the circuit breaker.
func TestDoctorWorkflow_GovernanceUnavailableIsDistinctFromRefusal(t *testing.T) {
	doc := newRefusalDoctor()
	doc.gateErr = fmt.Errorf("behavioral memory unreachable")

	run := runDoctorWorkflow(t, doc.config(), map[string]any{
		"finding_id":           "f-gov-down",
		"step_index":           0,
		"dry_run":              false,
		"finding_binding_mode": FindingBindingOperatorCurrent,
	})

	if run.Status == RunSucceeded {
		t.Fatal("an unreachable governor must not produce a succeeded run")
	}

	dr, ok := run.Outputs["dispatch_result"].(map[string]any)
	if !ok {
		t.Fatalf("dispatch_result missing from run outputs (have %v)", outputKeys(run.Outputs))
	}
	if got := dr["disposition"]; got != DispositionExecutionFailed {
		t.Errorf("disposition = %v, want %q — an outage is not a decision", got, DispositionExecutionFailed)
	}
	unavailable, _ := dr["governance_unavailable"].(bool)
	if !unavailable {
		t.Error("governance_unavailable must be set, so a governance outage is distinguishable " +
			"from a stream of legitimate refusals")
	}
	if got := dr["disposition"]; got == DispositionRefused {
		t.Error("an unreachable governor must never be reported as a refusal")
	}

	if doc.execCalls != 0 {
		t.Errorf("executor called %d time(s), want 0", doc.execCalls)
	}
	if doc.observeCall != 0 {
		t.Errorf("outcome observed %d time(s), want 0", doc.observeCall)
	}
}

// TestDoctorWorkflow_OneOutcomePerExecutedAction proves the single-producer
// rule: one executed action yields exactly one remediation outcome.
func TestDoctorWorkflow_OneOutcomePerExecutedAction(t *testing.T) {
	doc := newRefusalDoctor()
	doc.verdict = &GateVerdict{ActionCheckID: "chk-allow", Governed: true, Allowed: true, Status: "allowed"}

	run := runDoctorWorkflow(t, doc.config(), map[string]any{
		"finding_id":           "f-ok",
		"step_index":           0,
		"dry_run":              false,
		"finding_binding_mode": FindingBindingOperatorCurrent,
	})

	if run.Status != RunSucceeded {
		t.Fatalf("expected SUCCEEDED, got %s", run.Status)
	}
	if doc.gateCalls != 1 {
		t.Errorf("gate consulted %d time(s), want exactly 1 — gating on both the healer and the "+
			"workflow would mint two action checks for one action", doc.gateCalls)
	}
	if doc.execCalls != 1 {
		t.Errorf("executor called %d time(s), want exactly 1", doc.execCalls)
	}
	if doc.observeCall != 1 {
		t.Errorf("outcome observed %d time(s), want exactly 1 — one executed action, one "+
			"behavioral outcome, one governed-outcome link", doc.observeCall)
	}
	if len(doc.observed) == 1 && doc.observed[0].ActionCheckID != "chk-allow" {
		t.Errorf("outcome ActionCheckID = %q, want chk-allow — the outcome must tie back to the "+
			"decision that authorized it", doc.observed[0].ActionCheckID)
	}
}

// TestDoctorWorkflow_DryRunRecordsNoOutcome proves a rehearsal never
// manufactures promotion support.
func TestDoctorWorkflow_DryRunRecordsNoOutcome(t *testing.T) {
	doc := newRefusalDoctor()
	doc.verdict = &GateVerdict{ActionCheckID: "chk-dry", Governed: true, Allowed: true, Status: "allowed"}
	doc.execResult = &ExecutionResult{AuditID: "rem-dry", Executed: false, Status: "dry_run"}
	doc.verify = &Verification{Converged: false, FindingStillPresent: true}

	runDoctorWorkflow(t, doc.config(), map[string]any{
		"finding_id":           "f-dry",
		"step_index":           0,
		"dry_run":              true,
		"finding_binding_mode": FindingBindingOperatorCurrent,
	})

	for _, o := range doc.observed {
		if o.Dispatched {
			t.Error("a dry run must never record an outcome claiming dispatch — counting a " +
				"rehearsal would manufacture support for an action nobody performed")
		}
	}
}

func outputKeys(m map[string]any) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
