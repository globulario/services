package rules

import (
	"testing"
	"time"

	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	"github.com/globulario/services/golang/workflow/workflowpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// TestWorkflowCorrelationAbandoned_NoneEmits is the silence check: with
// no abandoned correlations in the snapshot the rule must produce zero
// findings. Doctor scans should not light up just because the workflow
// service is reachable.
func TestWorkflowCorrelationAbandoned_NoneEmits(t *testing.T) {
	snap := &collector.Snapshot{}
	findings := (workflowCorrelationAbandoned{}).Evaluate(snap, testConfig())
	if len(findings) != 0 {
		t.Errorf("expected 0 findings on empty snapshot, got %d", len(findings))
	}
}

// TestWorkflowCorrelationAbandoned_OneEmitsError verifies the basic
// path: one abandoned correlation produces one ERROR-severity finding
// with the right summary, evidence keys, and remediation.
func TestWorkflowCorrelationAbandoned_OneEmitsError(t *testing.T) {
	abandonedAt := time.Now().Add(-3 * time.Minute)
	snap := &collector.Snapshot{
		AbandonedDeferCorrelations: []*workflowpb.CorrelationDeferStateRecord{
			{
				CorrelationId:    "InfrastructureRelease/core@globular.io/keepalived",
				DeferCount:       5,
				MaxDefers:        5,
				LastStepId:       "verify_runtime",
				LastReason:       "verify runtime keepalived: status=inactive (want active)",
				LastBlockerTags:  []string{"runtime.active:keepalived@nuc"},
				LastDeferUntilMs: time.Now().UnixMilli(),
				Abandoned:        true,
				AbandonedAt:      timestamppb.New(abandonedAt),
			},
		},
	}
	findings := (workflowCorrelationAbandoned{}).Evaluate(snap, testConfig())
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d", len(findings))
	}
	f := findings[0]
	if f.InvariantID != "workflow.correlation.abandoned" {
		t.Errorf("invariant_id = %q", f.InvariantID)
	}
	if f.Severity != cluster_doctorpb.Severity_SEVERITY_ERROR {
		t.Errorf("severity = %v, want SEVERITY_ERROR", f.Severity)
	}
	if f.Category != "workflow" {
		t.Errorf("category = %q, want workflow", f.Category)
	}
	if f.EntityRef != "InfrastructureRelease/core@globular.io/keepalived" {
		t.Errorf("entity_ref = %q", f.EntityRef)
	}
	if f.InvariantStatus != cluster_doctorpb.InvariantStatus_INVARIANT_FAIL {
		t.Errorf("invariant_status = %v", f.InvariantStatus)
	}
	if len(f.Evidence) == 0 {
		t.Fatal("expected evidence")
	}
	kv := f.Evidence[0].KeyValues
	if kv["defer_count"] != "5" || kv["max_defers"] != "5" {
		t.Errorf("evidence count/max wrong: %v", kv)
	}
	if kv["last_step_id"] != "verify_runtime" {
		t.Errorf("evidence last_step_id = %q", kv["last_step_id"])
	}
	if kv["last_blocker_tags"] != "runtime.active:keepalived@nuc" {
		t.Errorf("evidence last_blocker_tags = %q", kv["last_blocker_tags"])
	}
	// Remediation must mention ClearCorrelationDeferState (operator hook).
	foundClear := false
	for _, r := range f.Remediation {
		if r.CliCommand != "" && contains(r.CliCommand, "ClearCorrelationDeferState") {
			foundClear = true
		}
	}
	if !foundClear {
		t.Error("remediation must include ClearCorrelationDeferState invocation")
	}
}

// TestWorkflowCorrelationAbandoned_PerCorrelationFinding verifies the
// per-correlation circuit breaker pattern: independent abandoned rows
// produce independent findings with distinct entity_refs (NOT one
// aggregate). The doctor view is "release X is stuck", "release Y is
// stuck" — never "all releases are stuck".
func TestWorkflowCorrelationAbandoned_PerCorrelationFinding(t *testing.T) {
	now := time.Now()
	snap := &collector.Snapshot{
		AbandonedDeferCorrelations: []*workflowpb.CorrelationDeferStateRecord{
			{CorrelationId: "rel/A", DeferCount: 5, MaxDefers: 5, LastStepId: "vr", Abandoned: true, AbandonedAt: timestamppb.New(now)},
			{CorrelationId: "rel/B", DeferCount: 5, MaxDefers: 5, LastStepId: "vr", Abandoned: true, AbandonedAt: timestamppb.New(now)},
			{CorrelationId: "rel/C", DeferCount: 5, MaxDefers: 5, LastStepId: "vr", Abandoned: true, AbandonedAt: timestamppb.New(now)},
		},
	}
	findings := (workflowCorrelationAbandoned{}).Evaluate(snap, testConfig())
	if len(findings) != 3 {
		t.Fatalf("expected 3 findings (one per correlation), got %d", len(findings))
	}
	seen := map[string]bool{}
	for _, f := range findings {
		seen[f.EntityRef] = true
	}
	for _, want := range []string{"rel/A", "rel/B", "rel/C"} {
		if !seen[want] {
			t.Errorf("missing finding for %s", want)
		}
	}
}

func contains(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
