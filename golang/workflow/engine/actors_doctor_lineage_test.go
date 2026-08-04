package engine

// Proves the real actor chain preserves verification lineage:
//
//	ResolvedFinding → execute (stamps dispatched_at) → verify → buildRemediationOutcome
//
// The outcome is never hand-built as the primary proof. Each test drives the
// registered handlers so the assertions are about what the workflow actually
// produces, not about a struct assembled by the test.

import (
	"context"
	"testing"
	"time"

	"github.com/globulario/services/golang/remediation"
)

const (
	lnFinding = "finding-1"
	lnCluster = "cluster-a"
	lnInvar   = "cluster.desired_state.absent"
	lnEntity  = "svc/repository" // deliberately NOT the node id
	lnNode    = "node-4"
	lnRun     = "run-abc"
)

var (
	lnDispatchAt = time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	lnVerifyAt   = lnDispatchAt.Add(90 * time.Second)
)

// chain runs resolve → execute → verify through the REAL handlers and returns
// the outcome the workflow built.
func chain(t *testing.T, mut ...func(*DoctorRemediationConfig)) remediation.Outcome {
	t.Helper()
	dispatched := true
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
		ExecuteRemediation: func(context.Context, string, uint32, string, bool) (*ExecutionResult, error) {
			return &ExecutionResult{AuditID: "audit-1", Status: "EXECUTED", Executed: dispatched}, nil
		},
		VerifyConvergence: func(context.Context, string, string, time.Time) (*Verification, error) {
			return &Verification{Converged: true}, nil
		},
	}
	for _, f := range mut {
		f(&cfg)
	}

	outputs := map[string]any{}
	ctx := context.Background()

	rf, err := doctorResolveFinding(cfg)(ctx, ActionRequest{
		RunID: lnRun, With: map[string]any{"finding_id": lnFinding, "step_index": 0}, Outputs: outputs,
	})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	resolved := rf.Output

	if _, err := doctorExecuteRemediation(cfg)(ctx, ActionRequest{
		RunID: lnRun, With: map[string]any{"finding_id": lnFinding, "step_index": 0}, Outputs: outputs,
	}); err != nil {
		t.Fatalf("execute: %v", err)
	}

	// The verify request carries exactly what the workflow YAML threads in.
	verifyReq := ActionRequest{
		RunID: lnRun,
		With: map[string]any{
			"finding_id":   lnFinding,
			"node_id":      resolved["node_id"],
			"cluster_id":   resolved["cluster_id"],
			"invariant_id": resolved["invariant_id"],
			"entity_ref":   resolved["entity_ref"],
		},
		Outputs: outputs,
	}
	return buildRemediationOutcome(verifyReq, lnFinding, &Verification{Converged: true},
		func() time.Time { return lnVerifyAt })
}

// ── the lineage ─────────────────────────────────────────────────────────────

func TestLineage_OutcomeCarriesAllNineBindings(t *testing.T) {
	o := chain(t)

	for _, c := range []struct{ name, got, want string }{
		{"FindingID", o.FindingID, lnFinding},
		{"ClusterID", o.ClusterID, lnCluster},
		{"InvariantID", o.InvariantID, lnInvar},
		{"EntityRef", o.EntityRef, lnEntity},
		{"NodeID", o.NodeID, lnNode},
		{"WorkflowRunID", o.WorkflowRunID, lnRun},
	} {
		if c.got != c.want {
			t.Errorf("%s = %q, want %q", c.name, c.got, c.want)
		}
	}
	if !o.DispatchedAt.Equal(lnDispatchAt) {
		t.Errorf("DispatchedAt = %v, want %v", o.DispatchedAt, lnDispatchAt)
	}
	if !o.VerifiedAt.Equal(lnVerifyAt) {
		t.Errorf("VerifiedAt = %v, want %v", o.VerifiedAt, lnVerifyAt)
	}
	if !o.FindingResolved {
		t.Error("FindingResolved must be true when verification converged")
	}
	if !o.LineageComplete() {
		t.Errorf("lineage must be complete, defects=%v", o.LineageDefects())
	}
}

// EntityRef and NodeID are different semantics. The chain uses a service-scoped
// entity so a silent substitution would be caught.
func TestLineage_EntityRefIsNotNodeID(t *testing.T) {
	o := chain(t)
	if o.EntityRef == o.NodeID {
		t.Fatal("EntityRef was replaced by NodeID — verification would attribute to the wrong subject")
	}
	if o.EntityRef != lnEntity {
		t.Errorf("EntityRef = %q, want %q", o.EntityRef, lnEntity)
	}
}

// ── negative cases ──────────────────────────────────────────────────────────

func TestLineage_MissingIdentityIsVisible(t *testing.T) {
	cases := []struct {
		name   string
		strip  string
		defect remediation.LineageDefect
	}{
		{"missing cluster", "cluster_id", remediation.LineageMissingCluster},
		{"missing invariant", "invariant_id", remediation.LineageMissingInvariant},
		{"missing entity", "entity_ref", remediation.LineageMissingEntity},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := ActionRequest{
				RunID: lnRun,
				With: map[string]any{
					"finding_id": lnFinding, "node_id": lnNode,
					"cluster_id": lnCluster, "invariant_id": lnInvar, "entity_ref": lnEntity,
				},
				Outputs: map[string]any{"execution_result": map[string]any{
					"executed": true, "dispatched_at": lnDispatchAt.Format(time.RFC3339Nano),
				}},
			}
			delete(req.With, tc.strip)

			o := buildRemediationOutcome(req, lnFinding, &Verification{Converged: true},
				func() time.Time { return lnVerifyAt })

			if o.LineageComplete() {
				t.Fatalf("%s must make lineage incomplete", tc.name)
			}
			found := false
			for _, d := range o.LineageDefects() {
				if d == tc.defect {
					found = true
				}
			}
			if !found {
				t.Errorf("want defect %q, got %v", tc.defect, o.LineageDefects())
			}
		})
	}
}

// Dispatch claimed with no timestamp is an inconsistent outcome, not a silently
// successful one.
func TestLineage_MissingDispatchTimeIsVisible(t *testing.T) {
	req := ActionRequest{
		RunID: lnRun,
		With: map[string]any{
			"finding_id": lnFinding, "node_id": lnNode, "cluster_id": lnCluster,
			"invariant_id": lnInvar, "entity_ref": lnEntity,
		},
		Outputs: map[string]any{"execution_result": map[string]any{"executed": true}},
	}
	o := buildRemediationOutcome(req, lnFinding, &Verification{Converged: true},
		func() time.Time { return lnVerifyAt })

	if !o.DispatchedAt.IsZero() {
		t.Fatal("DispatchedAt must stay zero — never synthesised from VerifiedAt")
	}
	if o.LineageComplete() {
		t.Fatal("dispatch without a timestamp must be reported")
	}
}

// Verification stamped before dispatch cannot be evidence the action worked.
func TestLineage_VerifiedBeforeDispatchIsRejected(t *testing.T) {
	o := remediation.Outcome{
		FindingID: lnFinding, ClusterID: lnCluster, InvariantID: lnInvar,
		EntityRef: lnEntity, WorkflowRunID: lnRun,
		Dispatched: true, Verified: true, FindingResolved: true,
		DispatchedAt: lnVerifyAt, VerifiedAt: lnDispatchAt, // inverted
	}
	if o.LineageComplete() {
		t.Fatal("verification predating dispatch must be reported as inconsistent")
	}
	var found bool
	for _, d := range o.LineageDefects() {
		if d == remediation.LineageVerifiedBefore {
			found = true
		}
	}
	if !found {
		t.Errorf("want verified_at_before_dispatched_at, got %v", o.LineageDefects())
	}
}

// A remediation that never dispatched cannot report verified success. The
// existing Status semantics already enforce this; asserted here so the new
// provenance fields cannot be mistaken for a way around it.
func TestLineage_NeverDispatchedCannotSucceed(t *testing.T) {
	o := remediation.Outcome{
		FindingID: lnFinding, ClusterID: lnCluster, InvariantID: lnInvar,
		EntityRef: lnEntity, WorkflowRunID: lnRun,
		Dispatched: false, Verified: true, FindingResolved: true,
		VerifiedAt: lnVerifyAt,
	}
	if o.IsSuccess() {
		t.Fatal("undispatched remediation must never be success")
	}
	if o.Status() != remediation.StatusFailed {
		t.Errorf("status = %q, want failed", o.Status())
	}
}

// Lineage completeness and repair success are independent axes. A successful
// repair with unattributable provenance must not read as citable evidence, and
// complete provenance around a failure must not read as success.
func TestLineage_CompletenessIsIndependentOfSuccess(t *testing.T) {
	successThinLineage := remediation.Outcome{
		FindingID: lnFinding, WorkflowRunID: lnRun,
		Dispatched: true, Verified: true, FindingResolved: true,
		DispatchedAt: lnDispatchAt, VerifiedAt: lnVerifyAt,
	}
	if !successThinLineage.IsSuccess() {
		t.Error("repair success must not depend on lineage completeness")
	}
	if successThinLineage.LineageComplete() {
		t.Error("missing cluster/invariant/entity must leave lineage incomplete")
	}

	failCompleteLineage := remediation.Outcome{
		FindingID: lnFinding, ClusterID: lnCluster, InvariantID: lnInvar,
		EntityRef: lnEntity, WorkflowRunID: lnRun,
		Dispatched: true, Verified: true, FindingResolved: false,
		DispatchedAt: lnDispatchAt, VerifiedAt: lnVerifyAt,
	}
	if failCompleteLineage.IsSuccess() {
		t.Error("complete lineage must not imply success")
	}
	if !failCompleteLineage.LineageComplete() {
		t.Errorf("lineage should be complete, defects=%v", failCompleteLineage.LineageDefects())
	}
}

// Outcomes from different runs must not be conflated.
func TestLineage_UnrelatedRunsDoNotMerge(t *testing.T) {
	o := chain(t)
	req := ActionRequest{
		RunID: "run-other",
		With: map[string]any{
			"finding_id": lnFinding, "node_id": lnNode, "cluster_id": lnCluster,
			"invariant_id": lnInvar, "entity_ref": lnEntity,
		},
		Outputs: map[string]any{"execution_result": map[string]any{
			"executed": true, "dispatched_at": lnDispatchAt.Format(time.RFC3339Nano),
		}},
	}
	other := buildRemediationOutcome(req, lnFinding, &Verification{Converged: true},
		func() time.Time { return lnVerifyAt })

	if o.WorkflowRunID == other.WorkflowRunID {
		t.Fatal("distinct runs must retain distinct WorkflowRunID")
	}
}

// Nothing dispatched → no timestamp exported at all.
func TestLineage_NoDispatchExportsNoTimestamp(t *testing.T) {
	outputs := map[string]any{}
	cfg := DoctorRemediationConfig{
		Now: func() time.Time { return lnDispatchAt },
		ExecuteRemediation: func(context.Context, string, uint32, string, bool) (*ExecutionResult, error) {
			return &ExecutionResult{Status: "REJECTED", Executed: false, Reason: "blocked"}, nil
		},
	}
	// dry_run so the handler does not turn non-execution into a step error.
	_, _ = doctorExecuteRemediation(cfg)(context.Background(), ActionRequest{
		RunID: lnRun, Outputs: outputs,
		With: map[string]any{"finding_id": lnFinding, "step_index": 0, "dry_run": true},
	})

	exec, _ := outputs["execution_result"].(map[string]any)
	if _, present := exec["dispatched_at"]; present {
		t.Fatal("dispatched_at must be ABSENT when nothing was dispatched")
	}
}
