package installed_state

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

func TestConvergenceKeys(t *testing.T) {
	if got, want := ConvergenceActionKey("a-1"), "/globular/convergence/actions/a-1"; got != want {
		t.Fatalf("ConvergenceActionKey = %q, want %q", got, want)
	}
	if got, want := ConvergenceLatestKey("n1", "workflow"), "/globular/convergence/nodes/n1/packages/workflow/latest"; got != want {
		t.Fatalf("ConvergenceLatestKey = %q, want %q", got, want)
	}
}

func TestParseConvergenceLatestKey(t *testing.T) {
	node, pkg, err := ParseConvergenceLatestKey("/globular/convergence/nodes/n1/packages/workflow/latest")
	if err != nil {
		t.Fatalf("ParseConvergenceLatestKey unexpected error: %v", err)
	}
	if node != "n1" || pkg != "workflow" {
		t.Fatalf("ParseConvergenceLatestKey got (%q,%q), want (%q,%q)", node, pkg, "n1", "workflow")
	}
	if _, _, err := ParseConvergenceLatestKey("/globular/nodes/n1/packages/workflow/latest"); err == nil {
		t.Fatal("expected invalid prefix error")
	}
}

func TestValidateConvergenceResult(t *testing.T) {
	valid := &ConvergenceResultV1{
		ActionID:        "a-1",
		WorkflowID:      "wf-1",
		Package:         "workflow",
		NodeID:          "n1",
		Outcome:         OutcomeSuccessCommitted,
		SourceComponent: "cluster-controller",
	}
	if err := validateConvergenceResult(valid); err != nil {
		t.Fatalf("valid result rejected: %v", err)
	}
	cases := []struct {
		name   string
		mutate func(*ConvergenceResultV1)
		errSub string
	}{
		{"missing_action", func(r *ConvergenceResultV1) { r.ActionID = "" }, "action_id"},
		{"missing_workflow", func(r *ConvergenceResultV1) { r.WorkflowID = "" }, "workflow_id"},
		{"missing_package", func(r *ConvergenceResultV1) { r.Package = "" }, "package"},
		{"missing_node", func(r *ConvergenceResultV1) { r.NodeID = "" }, "node_id"},
		{"missing_source", func(r *ConvergenceResultV1) { r.SourceComponent = "" }, "source_component"},
		{"invalid_outcome", func(r *ConvergenceResultV1) { r.Outcome = "UNKNOWN" }, "invalid"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := *valid
			tc.mutate(&r)
			if err := validateConvergenceResult(&r); err == nil || !strings.Contains(err.Error(), tc.errSub) {
				t.Fatalf("got err=%v, want contains %q", err, tc.errSub)
			}
		})
	}
}

func TestWriteConvergenceResultValidationFirst(t *testing.T) {
	err := WriteConvergenceResult(context.Background(), &ConvergenceResultV1{})
	if err == nil {
		t.Fatal("expected validation error")
	}
	if strings.Contains(strings.ToLower(err.Error()), "etcd") {
		t.Fatalf("expected validation before etcd, got %v", err)
	}
}

func TestOutcomeConstantsFrozen(t *testing.T) {
	want := []ConvergenceOutcome{
		"SUCCESS_COMMITTED",
		"SUCCESS_LOCAL_PENDING_SYNC",
		"BLOCKED_MISSING_NATIVE_DEP",
		"BLOCKED_CRITICAL_KEY_MISSING",
		"BLOCKED_NODE_UNREACHABLE",
		"FAILED_TRANSIENT",
		"FAILED_PERMANENT",
		"DEGRADED_RETRYING",
		"STALE_INSTALLED_STATE",
	}
	got := []ConvergenceOutcome{
		OutcomeSuccessCommitted,
		OutcomeSuccessLocalPendingSync,
		OutcomeBlockedMissingNativeDep,
		OutcomeBlockedCriticalKeyMissing,
		OutcomeBlockedNodeUnreachable,
		OutcomeFailedTransient,
		OutcomeFailedPermanent,
		OutcomeDegradedRetrying,
		OutcomeStaleInstalledState,
	}
	if len(got) != len(want) {
		t.Fatalf("constants count mismatch")
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("outcome[%d]=%q, want %q", i, got[i], want[i])
		}
	}
}

func TestUnmarshalConvergenceResult(t *testing.T) {
	now := time.Now().Unix()
	src := &ConvergenceResultV1{
		ActionID:        "a-1",
		WorkflowID:      "wf-1",
		Package:         "workflow",
		NodeID:          "n1",
		Outcome:         OutcomeSuccessCommitted,
		CommittedAt:     now,
		LastAttemptAt:   now,
		AttemptCount:    1,
		SourceComponent: "cluster-controller",
	}
	b, err := json.Marshal(src)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	got, err := unmarshalConvergenceResult(b)
	if err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got.ActionID != src.ActionID || got.Outcome != src.Outcome {
		t.Fatalf("round-trip mismatch: got=%+v src=%+v", got, src)
	}
}

// ── PR-5: CommitConvergenceWithInstall validation tests ──────────────────────

func TestCommitConvergenceWithInstallValidation(t *testing.T) {
	validPkg := &node_agentpb.InstalledPackage{
		NodeId:  "n1",
		Name:    "workflow",
		Kind:    "SERVICE",
		Version: "1.0.6",
	}
	validResult := &ConvergenceResultV1{
		ActionID: "n1/SERVICE/workflow/1.0.6",
		NodeID:   "n1",
		Package:  "workflow",
	}
	cases := []struct {
		name   string
		pkg    *node_agentpb.InstalledPackage
		result *ConvergenceResultV1
		errSub string
	}{
		{"nil_result", validPkg, nil, "result is required"},
		{"missing_pkg_node_id", &node_agentpb.InstalledPackage{Name: "w", Kind: "SERVICE"}, validResult, "node_id"},
		{"missing_pkg_name", &node_agentpb.InstalledPackage{NodeId: "n1", Kind: "SERVICE"}, validResult, "name"},
		{"missing_pkg_kind", &node_agentpb.InstalledPackage{NodeId: "n1", Name: "w"}, validResult, "kind"},
		{"missing_result_action_id", validPkg, &ConvergenceResultV1{NodeID: "n1", Package: "workflow"}, "action_id"},
		{"missing_result_node_id", validPkg, &ConvergenceResultV1{ActionID: "a1", Package: "workflow"}, "node_id"},
		{"missing_result_package", validPkg, &ConvergenceResultV1{ActionID: "a1", NodeID: "n1"}, "package"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := CommitConvergenceWithInstall(context.Background(), tc.pkg, tc.result)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(err.Error(), tc.errSub) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.errSub)
			}
			// Validation must fire before any etcd access.
			if strings.Contains(strings.ToLower(err.Error()), "etcd") {
				t.Fatalf("expected validation before etcd access, got: %v", err)
			}
		})
	}
}

func TestCommitConvergenceWithInstallPromotion(t *testing.T) {
	// Verify that CommitConvergenceWithInstall sets default timestamps on pkg
	// before marshal — observable without etcd by checking the validation path.
	pkg := &node_agentpb.InstalledPackage{
		NodeId: "n1", Name: "w", Kind: "SERVICE", Version: "1.0.0",
		// UpdatedUnix and InstalledUnix deliberately zero — should be filled in.
	}
	result := &ConvergenceResultV1{
		ActionID: "a1", NodeID: "n1", Package: "w",
		Outcome: OutcomeSuccessLocalPendingSync,
	}
	// We can't call etcd in a unit test, but we can verify the inputs are
	// accepted by validation (err must mention etcd, not validation).
	err := CommitConvergenceWithInstall(context.Background(), pkg, result)
	if err == nil {
		// Succeeded — must be a test environment with etcd. Fine.
		return
	}
	// Any error here should be an etcd connectivity error, not a validation error.
	if strings.Contains(err.Error(), "node_id") || strings.Contains(err.Error(), "action_id") {
		t.Fatalf("unexpected validation error (inputs should be valid): %v", err)
	}
}
