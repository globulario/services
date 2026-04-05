package engine

import (
	"context"
	"strings"
	"testing"
	"time"
)

// TestNoPlanVocabularyInWorkflowActions verifies that workflow-native action
// names do not reference plan concepts. This is a vocabulary guard from the
// full-workflow-only migration: no new code should introduce plan-oriented
// abstractions.
func TestNoPlanVocabularyInWorkflowActions(t *testing.T) {
	router := NewRouter()

	// Register all workflow-native actions.
	RegisterReleaseControllerActions(router, ReleaseControllerConfig{})
	RegisterNodeDirectApplyActions(router, NodeDirectApplyConfig{})
	RegisterReconcileControllerActions(router, ReconcileControllerConfig{})
	RegisterWorkflowServiceActions(router, WorkflowServiceConfig{})
	RegisterNodeRepairControllerActions(router, NodeRepairControllerConfig{})
	RegisterNodeRepairAgentActions(router, NodeRepairAgentConfig{})
	RegisterDoctorRemediationActions(router, DoctorRemediationConfig{})

	// Collect all registered action names.
	router.mu.RLock()
	defer router.mu.RUnlock()

	planWords := []string{"plan.", "compile_plan", "dispatch_plan", "wait_for_slot", "execute_plan", "plan_status"}
	for key := range router.handlers {
		for _, word := range planWords {
			if strings.Contains(key, word) {
				t.Errorf("action %q contains plan vocabulary %q — workflow-native actions must not reference plans", key, word)
			}
		}
	}
}

// TestWorkflowReleaseNoPlanArtifacts runs the release.apply.package workflow
// and verifies that no plan-related outputs or step IDs appear in the results.
func TestWorkflowReleaseNoPlanArtifacts(t *testing.T) {
	opts := defaultReleaseOpts()
	router := newReleaseTestRouter(t, opts)
	eng := &Engine{Router: router}

	def := loadReleaseInfraDef(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	run, err := eng.Execute(ctx, def, releaseInputs("node-1"))
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if run.Status != RunSucceeded {
		t.Fatalf("expected SUCCEEDED, got %s", run.Status)
	}

	// No step ID should contain "plan".
	for id := range run.Steps {
		if strings.Contains(id, "plan") {
			t.Errorf("step ID %q contains 'plan' — workflow path must not produce plan artifacts", id)
		}
	}

	// No output key should contain "plan".
	for key := range run.Outputs {
		if strings.Contains(key, "plan") {
			t.Errorf("output key %q contains 'plan' — workflow path must not produce plan artifacts", key)
		}
	}
}
