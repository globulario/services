package engine

// Commit 4D: the workflow receipt must carry the identity the verdict is about.
//
// 4C-pre added subject identity and dispatch time to remediation.Outcome but
// left outcomeAsMap projecting only the verdict, so an out-of-process reader
// (CLI, MCP, dashboard) saw "SUCCEEDED" with no way to recover WHAT succeeded.
// Per docs/intent/workflow.step_receipts_are_evidence a receipt must let an
// operator reconstruct what happened; a status without a subject cannot.
//
// These tests compare the map against the canonical Outcome directly, so the
// two cannot drift into disagreeing about the same run.

import (
	"context"
	"testing"
	"time"

	"github.com/globulario/services/golang/remediation"
)

// omOutcome is the outcome the REAL chain produces, obtained from the real
// handlers rather than assembled here.
func omOutcome(t *testing.T) remediation.Outcome {
	t.Helper()
	return chain(t)
}

// omConfig is the converging config the real handlers run against.
func omConfig(mut ...func(*DoctorRemediationConfig)) DoctorRemediationConfig {
	cfg := DoctorRemediationConfig{
		Now: func() time.Time { return lnDispatchAt },
		ResolveFinding: func(_ context.Context, _ string, id string, idx uint32, _ string) (*ResolvedFinding, error) {
			return &ResolvedFinding{
				FindingID: id, StepIndex: idx, NodeID: lnNode,
				ActionType: "SYSTEMCTL_RESTART", Risk: "LOW", Idempotent: true,
				Description: "restart", HasAction: true,
				ClusterID: lnCluster, InvariantID: lnInvar, EntityRef: lnEntity,
			}, nil
		},
		ExecuteRemediation: func(context.Context, string, string, uint32, string, bool, string) (*ExecutionResult, error) {
			return &ExecutionResult{AuditID: "audit-1", Status: "EXECUTED", Executed: true}, nil
		},
		VerifyConvergence: func(context.Context, string, string, time.Time) (*Verification, error) {
			return &Verification{Converged: true}, nil
		},
	}
	for _, f := range mut {
		f(&cfg)
	}
	return cfg
}

// runChainOutputs drives resolve → execute → verify through the REAL handlers
// and returns the run outputs an out-of-process consumer would read.
func runChainOutputs(t *testing.T, mut ...func(*DoctorRemediationConfig)) map[string]any {
	t.Helper()
	cfg := omConfig(mut...)
	outputs := map[string]any{}
	ctx := context.Background()

	if _, err := doctorResolveFinding(cfg)(ctx, ActionRequest{
		RunID: lnRun, With: map[string]any{"finding_id": lnFinding, "step_index": 0}, Outputs: outputs,
	}); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if _, err := doctorExecuteRemediation(cfg)(ctx, ActionRequest{
		RunID: lnRun, With: map[string]any{"finding_id": lnFinding, "step_index": 0}, Outputs: outputs,
	}); err != nil {
		t.Fatalf("execute: %v", err)
	}
	resolved, _ := outputs["resolved_finding"].(map[string]any)
	if _, err := doctorVerifyConvergence(cfg)(ctx, ActionRequest{
		RunID: lnRun,
		With: map[string]any{
			"finding_id": lnFinding, "node_id": resolved["node_id"],
			"cluster_id": resolved["cluster_id"], "invariant_id": resolved["invariant_id"],
			"entity_ref": resolved["entity_ref"],
		},
		Outputs: outputs,
	}); err != nil {
		t.Fatalf("verify: %v", err)
	}
	return outputs
}

func TestOutcomeMap_CarriesEveryLineageField(t *testing.T) {
	o := omOutcome(t)
	m := outcomeAsMap(o)

	strs := map[string]string{
		"finding_id":      o.FindingID,
		"workflow_run_id": o.WorkflowRunID,
		"cluster_id":      o.ClusterID,
		"invariant_id":    o.InvariantID,
		"entity_ref":      o.EntityRef,
		"node_id":         o.NodeID,
		"status":          string(o.Status()),
		"reason":          o.Reason(),
	}
	for k, want := range strs {
		got, ok := m[k].(string)
		if !ok {
			t.Errorf("%s missing from the receipt or not a string (%T)", k, m[k])
			continue
		}
		if got != want {
			t.Errorf("%s = %q, want %q", k, got, want)
		}
	}

	bools := map[string]bool{
		"dispatched":       o.Dispatched,
		"verified":         o.Verified,
		"finding_resolved": o.FindingResolved,
		"is_success":       o.IsSuccess(),
		"lineage_complete": o.LineageComplete(),
	}
	for k, want := range bools {
		got, ok := m[k].(bool)
		if !ok {
			t.Errorf("%s missing from the receipt or not a bool (%T)", k, m[k])
			continue
		}
		if got != want {
			t.Errorf("%s = %t, want %t", k, got, want)
		}
	}

	// Canonical RFC3339Nano, matching what the execute step stamps into
	// execution_result.dispatched_at — one spelling of a timestamp, not two.
	if got := m["dispatched_at"]; got != o.DispatchedAt.UTC().Format(time.RFC3339Nano) {
		t.Errorf("dispatched_at = %v, want %s", got, o.DispatchedAt.UTC().Format(time.RFC3339Nano))
	}
	if got := m["verified_at"]; got != o.VerifiedAt.UTC().Format(time.RFC3339Nano) {
		t.Errorf("verified_at = %v, want %s", got, o.VerifiedAt.UTC().Format(time.RFC3339Nano))
	}
}

// entity_ref and node_id are separate keys. The chain uses a service-scoped
// entity so a silent substitution in the receipt is visible.
func TestOutcomeMap_EntityRefIsNotNodeID(t *testing.T) {
	m := outcomeAsMap(omOutcome(t))
	if m["entity_ref"] == m["node_id"] {
		t.Fatalf("entity_ref and node_id collapsed to %v — the receipt would name the wrong subject", m["entity_ref"])
	}
	if m["entity_ref"] != lnEntity {
		t.Errorf("entity_ref = %v, want %q", m["entity_ref"], lnEntity)
	}
}

// A remediation that never dispatched has no dispatch time. Emitting the Go zero
// date would read as "dispatched at year 1" to any consumer that parses it.
func TestOutcomeMap_ZeroTimestampsAreOmittedNotZeroDates(t *testing.T) {
	m := outcomeAsMap(remediation.Outcome{FindingID: "f", WorkflowRunID: "r"})

	if _, present := m["dispatched_at"]; present {
		t.Error("dispatched_at must be ABSENT when nothing was dispatched, never a zero date")
	}
	if _, present := m["verified_at"]; present {
		t.Error("verified_at must be ABSENT when nothing was verified")
	}
}

// An unattributable outcome must say so, and name which bindings are missing.
func TestOutcomeMap_ReportsLineageDefects(t *testing.T) {
	o := omOutcome(t)
	o.ClusterID = ""
	m := outcomeAsMap(o)

	if m["lineage_complete"] != false {
		t.Fatal("lineage_complete must be false when a binding is missing")
	}
	defects, ok := m["lineage_defects"].([]string)
	if !ok || len(defects) == 0 {
		t.Fatalf("lineage_defects must name the missing bindings, got %v", m["lineage_defects"])
	}
	var found bool
	for _, d := range defects {
		if d == string(remediation.LineageMissingCluster) {
			found = true
		}
	}
	if !found {
		t.Errorf("want %q among defects, got %v", remediation.LineageMissingCluster, defects)
	}

	// Success and lineage stay independent in the receipt too.
	if m["is_success"] != true {
		t.Error("a missing binding must not change whether the repair worked")
	}
}

// The receipt is written into run outputs by the real verify handler, not only
// by a direct call to the projection.
func TestOutcomeMap_ReachesRunOutputsThroughTheRealHandler(t *testing.T) {
	outputs := runChainOutputs(t)

	got, ok := outputs["remediation_outcome"].(map[string]any)
	if !ok {
		t.Fatalf("verify step must write remediation_outcome into run outputs, got %T", outputs["remediation_outcome"])
	}
	for _, k := range []string{
		"cluster_id", "invariant_id", "entity_ref", "node_id",
		"finding_id", "workflow_run_id", "dispatched_at", "verified_at",
		"finding_resolved", "lineage_complete",
	} {
		if _, present := got[k]; !present {
			t.Errorf("%s missing from the receipt an out-of-process consumer would read", k)
		}
	}
	if got["cluster_id"] != lnCluster {
		t.Errorf("cluster_id = %v, want %q", got["cluster_id"], lnCluster)
	}
	if got["entity_ref"] != lnEntity {
		t.Errorf("entity_ref = %v, want %q", got["entity_ref"], lnEntity)
	}
}

// ── the ObserveOutcome hook ─────────────────────────────────────────────────

// Every outcome reaches the learning sink, including one that did not converge.
// Filtering to successes would mean the behavioral record only ever contained
// repairs that worked.
func TestObserveOutcome_FiresForUnsuccessfulRemediationToo(t *testing.T) {
	var seen []remediation.Outcome
	cfg := omConfig(func(c *DoctorRemediationConfig) {
		// Did NOT converge.
		c.VerifyConvergence = func(context.Context, string, string, time.Time) (*Verification, error) {
			return &Verification{Converged: false, FindingStillPresent: true}, nil
		}
		c.ObserveOutcome = func(_ context.Context, o remediation.Outcome) { seen = append(seen, o) }
	})

	outputs := map[string]any{}
	ctx := context.Background()
	if _, err := doctorResolveFinding(cfg)(ctx, ActionRequest{
		RunID: lnRun, With: map[string]any{"finding_id": lnFinding, "step_index": 0}, Outputs: outputs,
	}); err != nil {
		t.Fatalf("resolve: %v", err)
	}
	if _, err := doctorExecuteRemediation(cfg)(ctx, ActionRequest{
		RunID: lnRun, With: map[string]any{"finding_id": lnFinding, "step_index": 0}, Outputs: outputs,
	}); err != nil {
		t.Fatalf("execute: %v", err)
	}
	resolved, _ := outputs["resolved_finding"].(map[string]any)
	// The verify step MUST fail the step — an unresolved finding is not success.
	_, err := doctorVerifyConvergence(cfg)(ctx, ActionRequest{
		RunID: lnRun,
		With: map[string]any{
			"finding_id": lnFinding, "node_id": resolved["node_id"],
			"cluster_id": resolved["cluster_id"], "invariant_id": resolved["invariant_id"],
			"entity_ref": resolved["entity_ref"],
		},
		Outputs: outputs,
	})
	if err == nil {
		t.Fatal("a finding still present after remediation must fail the verify step")
	}

	if len(seen) != 1 {
		t.Fatalf("the learning sink must receive the outcome even when the repair failed, got %d", len(seen))
	}
	if seen[0].FindingResolved {
		t.Error("the recorded outcome must reflect that the finding did NOT clear")
	}
	if seen[0].Status() != remediation.StatusDegraded {
		t.Errorf("status = %q, want DEGRADED", seen[0].Status())
	}
	// Identity still travels, so an unsuccessful repair is attributable too.
	if seen[0].ClusterID != lnCluster || seen[0].EntityRef != lnEntity {
		t.Error("a failed repair must still carry its subject identity")
	}
}

// A panicking or slow sink is the caller's problem, but the engine must not
// require one: a nil hook is the default and must be inert.
func TestObserveOutcome_NilHookIsInert(t *testing.T) {
	if o := chain(t); !o.IsSuccess() {
		t.Fatal("the default chain has no ObserveOutcome and must still succeed")
	}
}
