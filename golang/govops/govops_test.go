package govops

import (
	"testing"

	pb "github.com/globulario/services/golang/govops/governed_operationpb"
)

// validDesiredApply returns a fully-formed, correctly-routed APPLY that mutates
// owner-owned desired state. Each test mutates one field to assert one rule.
func validDesiredApply() *pb.OperationRequest {
	return &pb.OperationRequest{
		Id:     "op-1",
		Actor:  pb.ActorKind_ACTOR_AGENT,
		Action: "set_desired_version",
		Target: &pb.OperationTarget{
			Subsystem:    "desired_state",
			ResourceType: "ServiceDesiredVersion",
			ResourceId:   "echo",
			Owner:        "cluster_controller",
		},
		Authority: &pb.OperationAuthority{
			RequiredOwnerPath: "cluster_controller.UpsertDesiredService",
			CallerIdentity:    "sa:mcp",
			Permission:        "CapDesiredWrite",
		},
		ExpectedEffect: &pb.ExpectedEffect{
			MutatesDesired:      true,
			BumpsGeneration:     true,
			TriggersReconcile:   true,
			RefreshesProjection: true,
		},
		Evidence:  &pb.OperationEvidence{BeforeSnapshot: "sha256:abc"},
		Execution: &pb.OperationExecution{Mode: pb.ExecutionMode_APPLY, ApprovedPath: pb.ApprovedPath_OWNER_RPC},
		Postconditions: &pb.OperationPostconditions{
			RequiredChecks: []string{"generation_advanced", "reconcile_observed"},
			RollbackPlan:   "re-pin previous desired version",
			LedgerRequired: true,
		},
	}
}

func hasCode(d Decision, code string) bool {
	for _, v := range d.Violations {
		if v.Code == code {
			return true
		}
	}
	return false
}

func TestValidate_CleanOwnerRoutedApply_Allowed(t *testing.T) {
	d := Validate(validDesiredApply())
	if d.Kind != DecisionAllowed {
		t.Fatalf("clean owner-routed apply: got %s, violations=%v", d.Kind, d.Violations)
	}
}

func TestValidate_RawWriteToOwnerOwned_NeedsOwnerPath(t *testing.T) {
	r := validDesiredApply()
	r.Execution.ApprovedPath = pb.ApprovedPath_DIAGNOSTIC_READONLY // raw write
	d := Validate(r)
	if d.Kind != DecisionNeedsOwnerPath {
		t.Fatalf("raw write to owner-owned: got %s, want needs_owner_path; violations=%v", d.Kind, d.Violations)
	}
	if !hasCode(d, "owner_path_required") {
		t.Errorf("missing owner_path_required violation: %v", d.Violations)
	}
}

func TestValidate_ForbiddenPath_Refused(t *testing.T) {
	r := validDesiredApply()
	r.Execution.ApprovedPath = pb.ApprovedPath_FORBIDDEN
	d := Validate(r)
	if d.Kind != DecisionRefused || !hasCode(d, "forbidden_path") {
		t.Fatalf("forbidden path: got %s, violations=%v", d.Kind, d.Violations)
	}
}

func TestValidate_MissingGenerationBump_Refused(t *testing.T) {
	r := validDesiredApply()
	r.ExpectedEffect.BumpsGeneration = false
	d := Validate(r)
	if d.Kind != DecisionRefused || !hasCode(d, "generation_bump_required") {
		t.Fatalf("missing generation bump: got %s, violations=%v", d.Kind, d.Violations)
	}
}

func TestValidate_MissingReconcileTrigger_Refused(t *testing.T) {
	r := validDesiredApply()
	r.ExpectedEffect.TriggersReconcile = false
	d := Validate(r)
	if !hasCode(d, "reconcile_required") {
		t.Fatalf("missing reconcile: got %s, violations=%v", d.Kind, d.Violations)
	}
}

func TestValidate_MissingProjectionRefresh_Refused(t *testing.T) {
	r := validDesiredApply()
	r.ExpectedEffect.RefreshesProjection = false
	d := Validate(r)
	if !hasCode(d, "projection_refresh_required") {
		t.Fatalf("missing projection refresh: %v", d.Violations)
	}
}

func TestValidate_MissingBeforeSnapshot_Refused(t *testing.T) {
	r := validDesiredApply()
	r.Evidence.BeforeSnapshot = ""
	d := Validate(r)
	if !hasCode(d, "before_snapshot_required") {
		t.Fatalf("missing before_snapshot: %v", d.Violations)
	}
}

func TestValidate_UnexplainedAuthority_Refused(t *testing.T) {
	r := validDesiredApply()
	r.Authority.CallerIdentity = ""
	d := Validate(r)
	if !hasCode(d, "authority_unexplained") {
		t.Fatalf("unexplained authority: %v", d.Violations)
	}
}

func TestValidate_MissingPostconditions_Refused(t *testing.T) {
	r := validDesiredApply()
	r.Postconditions = &pb.OperationPostconditions{} // no checks, no rollback, no ledger
	d := Validate(r)
	for _, code := range []string{"postcondition_checks_missing", "rollback_plan_missing", "ledger_required"} {
		if !hasCode(d, code) {
			t.Errorf("missing expected violation %q: %v", code, d.Violations)
		}
	}
	if d.Kind != DecisionRefused {
		t.Fatalf("got %s, want refused", d.Kind)
	}
}

func TestValidate_DryRun_AllowedButAdvisory(t *testing.T) {
	r := validDesiredApply()
	r.Execution.Mode = pb.ExecutionMode_DRY_RUN
	r.Execution.ApprovedPath = pb.ApprovedPath_DIAGNOSTIC_READONLY
	r.ExpectedEffect.BumpsGeneration = false
	d := Validate(r)
	if d.Kind != DecisionAllowed {
		t.Fatalf("dry-run must be allowed (plan only), got %s", d.Kind)
	}
	// ...but it must still surface what an APPLY would trip.
	if !hasCode(d, "owner_path_required") || !hasCode(d, "generation_bump_required") {
		t.Fatalf("dry-run must surface advisory violations: %v", d.Violations)
	}
}

func TestValidate_NonMutatingRead_Allowed(t *testing.T) {
	r := validDesiredApply()
	r.ExpectedEffect = &pb.ExpectedEffect{} // read-only effect
	r.Execution.ApprovedPath = pb.ApprovedPath_DIAGNOSTIC_READONLY
	d := Validate(r)
	if d.Kind != DecisionAllowed || len(d.Violations) != 0 {
		t.Fatalf("non-mutating read must be allowed cleanly, got %s %v", d.Kind, d.Violations)
	}
}

func TestValidate_BreakGlass_SatisfiedVsRefused(t *testing.T) {
	r := validDesiredApply()
	r.Execution = &pb.OperationExecution{Mode: pb.ExecutionMode_BREAK_GLASS, ApprovedPath: pb.ApprovedPath_DIAGNOSTIC_READONLY}
	// Satisfied: snapshot + rollback + ledger + a post-reconcile check.
	if d := Validate(r); d.Kind != DecisionBreakGlassOnly {
		t.Fatalf("well-formed break-glass: got %s, violations=%v", d.Kind, d.Violations)
	}
	// Drop the reconcile check -> refused.
	r.Postconditions.RequiredChecks = []string{"snapshot_taken"}
	if d := Validate(r); d.Kind != DecisionRefused || !hasCode(d, "post_reconcile_required") {
		t.Fatalf("break-glass without reconcile check must be refused: %s %v", d.Kind, d.Violations)
	}
}

// Subsumption: the governor's approval scope for its five policy patterns is
// reproducible from an OperationRequest, so the gateway is one gate, not two.
func TestApprovalScope_ReproducesGovernorPolicies(t *testing.T) {
	cases := []struct {
		name   string
		mutate func(*pb.OperationRequest)
		want   string
	}{
		{"services repair -> destructive", func(r *pb.OperationRequest) { r.Action = "services_repair" }, ScopeDestructive},
		{"cluster bootstrap -> destructive", func(r *pb.OperationRequest) { r.Action = "cluster_bootstrap" }, ScopeDestructive},
		{"pkg publish -> publish", func(r *pb.OperationRequest) {
			r.Action = "pkg_publish"
			r.Target.Subsystem = "repository"
		}, ScopePublish},
		{"services desired set -> production", func(r *pb.OperationRequest) { r.Action = "services_desired_set" }, ScopeProduction},
		{"dns a set -> production", func(r *pb.OperationRequest) {
			r.Action = "dns_a_set"
			r.ExpectedEffect = &pb.ExpectedEffect{MutatesStatus: true}
		}, ScopeProduction},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := validDesiredApply()
			tc.mutate(r)
			if got := ApprovalScope(r); got != tc.want {
				t.Errorf("ApprovalScope = %q, want %q", got, tc.want)
			}
		})
	}
}

// Subsumption: a completed ledger entry projects onto a valid audittrail
// desired-write record (all required fields non-empty), so the two need not be
// separate writers.
func TestProjectDesiredWrite_ProducesValidRecord(t *testing.T) {
	e := &pb.OperationLedgerEntry{
		OperationId:    "op-1",
		Timestamp:      "2026-06-24T00:00:00Z",
		Actor:          "sa:mcp",
		Tool:           "globular_cli",
		Command:        "services desired set echo 1.2.3",
		TargetResource: "echo",
		TargetOwner:    "cluster_controller",
		AwgInvariants:  []string{"desired.keyed_by_kind_and_name"},
		Result:         pb.OperationResult_COMPLETED,
	}
	rec := ProjectDesiredWrite(e)
	if rec.Service == "" || rec.Actor == "" || rec.Source == "" || rec.Action == "" || rec.Reason == "" {
		t.Fatalf("projected record missing required field(s): %+v", rec)
	}
	if rec.Service != "echo" || rec.Reason != "desired.keyed_by_kind_and_name" {
		t.Errorf("unexpected projection: %+v", rec)
	}
}

// Reason falls back to the command when the ledger names no invariant.
func TestProjectDesiredWrite_ReasonFallback(t *testing.T) {
	e := &pb.OperationLedgerEntry{
		Actor: "sa:mcp", Tool: "cli", Command: "set x", TargetResource: "x",
	}
	if got := ProjectDesiredWrite(e).Reason; got != "set x" {
		t.Errorf("reason fallback = %q, want command", got)
	}
}
