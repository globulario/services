package engine

import (
	"context"
	"fmt"
	"sync"
	"testing"

	"github.com/globulario/services/golang/workflow/v1alpha1"
)

// These are the engine-level halves of the runtime ratchet for
// meta.state_mutations_must_be_durably_committed_before_side_effects:
//   T1 — terminal-state guarantee (every run ends terminal/parked, never stuck)
//   T2 — no-alternative-path (a failed step retries its OWN action or fails the
//        run; the engine never substitutes a different action, which would make
//        the audit trail lie).
// They complement the server-level commit-before-execute gate
// (TestExecuteWorkflow_StartRunCommitFailure_NoSideEffects).

// isTerminalOrParked reports whether a status is a legitimate end-of-Execute
// state. RUNNING/PENDING/"" are NOT — a run left in one of those is the
// "frozen partial" bug class: a multi-step mutation that started but owns no
// obligation to finish.
func isTerminalOrParked(s RunStatus) bool {
	switch s {
	case RunSucceeded, RunFailed, RunDeferred:
		return true
	default:
		return false
	}
}

// TestEngine_AlwaysReachesTerminalState is the T1 ratchet. Across every handled
// step outcome (success, business failure, retry-exhaustion-with-defer), the Run
// returned by Execute MUST be terminal or parked — never RUNNING/PENDING/nil.
func TestEngine_AlwaysReachesTerminalState(t *testing.T) {
	mkDef := func(name, action string, retry *v1alpha1.RetryPolicy, deferP *v1alpha1.DeferPolicy) *v1alpha1.WorkflowDefinition {
		return &v1alpha1.WorkflowDefinition{
			APIVersion: v1alpha1.APIVersion,
			Kind:       v1alpha1.Kind,
			Metadata:   v1alpha1.WorkflowMetadata{Name: name},
			Spec: v1alpha1.WorkflowDefinitionSpec{
				Strategy: v1alpha1.ExecutionStrategy{Mode: v1alpha1.StrategySingle},
				Steps: []v1alpha1.WorkflowStepSpec{
					{ID: "only", Actor: v1alpha1.ActorInstaller, Action: action, Retry: retry, Defer: deferP},
				},
			},
		}
	}

	cases := []struct {
		name    string
		def     *v1alpha1.WorkflowDefinition
		handler ActionHandler
		want    RunStatus
	}{
		{
			name:    "success_is_SUCCEEDED",
			def:     mkDef("t1-success", "installer.ok", nil, nil),
			handler: func(context.Context, ActionRequest) (*ActionResult, error) { return &ActionResult{OK: true}, nil },
			want:    RunSucceeded,
		},
		{
			name:    "business_failure_is_FAILED",
			def:     mkDef("t1-fail", "installer.fail", nil, nil),
			handler: func(context.Context, ActionRequest) (*ActionResult, error) { return nil, fmt.Errorf("boom") },
			want:    RunFailed,
		},
		{
			name: "retry_exhaustion_with_defer_is_DEFERRED",
			def: mkDef("t1-defer", "installer.transient",
				&v1alpha1.RetryPolicy{MaxAttempts: 2, Backoff: &v1alpha1.ScalarString{Raw: "1ms"}},
				&v1alpha1.DeferPolicy{Cooldown: &v1alpha1.ScalarString{Raw: "30s"}, MaxDefers: 4, BlockerTags: []string{"runtime.active:x@n"}}),
			handler: func(context.Context, ActionRequest) (*ActionResult, error) { return nil, fmt.Errorf("transient") },
			want:    RunDeferred,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			router := NewRouter()
			router.Register(v1alpha1.ActorInstaller, tc.def.Spec.Steps[0].Action, tc.handler)
			run, _ := (&Engine{Router: router}).Execute(context.Background(), tc.def, nil)
			if run == nil {
				t.Fatal("Execute returned a nil run — no terminal state at all (frozen partial)")
			}
			if !isTerminalOrParked(run.Status) {
				t.Fatalf("run left non-terminal: status=%q (want terminal/parked)", run.Status)
			}
			if run.Status != tc.want {
				t.Errorf("run status = %q, want %q", run.Status, tc.want)
			}
		})
	}
}

// TestEngine_NoAlternativePath_OnlyPlanActionsDispatched is the T2 ratchet. When
// a step fails, the engine retries that step's OWN action or fails the run — it
// never substitutes a different action. Every (actor, action) dispatched must be
// one declared in the compiled plan; a downstream action whose dependency failed
// must not be dispatched at all.
func TestEngine_NoAlternativePath_OnlyPlanActionsDispatched(t *testing.T) {
	def := &v1alpha1.WorkflowDefinition{
		APIVersion: v1alpha1.APIVersion,
		Kind:       v1alpha1.Kind,
		Metadata:   v1alpha1.WorkflowMetadata{Name: "t2-no-alt"},
		Spec: v1alpha1.WorkflowDefinitionSpec{
			Strategy: v1alpha1.ExecutionStrategy{Mode: v1alpha1.StrategyDAG},
			Steps: []v1alpha1.WorkflowStepSpec{
				{ID: "a", Actor: v1alpha1.ActorInstaller, Action: "installer.a",
					Retry: &v1alpha1.RetryPolicy{MaxAttempts: 3, Backoff: &v1alpha1.ScalarString{Raw: "1ms"}}},
				{ID: "b", Actor: v1alpha1.ActorInstaller, Action: "installer.b", DependsOn: []string{"a"}},
			},
		},
	}
	planActions := map[string]bool{"installer.a": true, "installer.b": true}

	var mu sync.Mutex
	var dispatched []string
	router := NewRouter()
	// A fallback records EVERY dispatch to the actor — declared or invented —
	// so an alternative action would be caught here, not silently resolved by a
	// per-action handler.
	router.RegisterFallback(v1alpha1.ActorInstaller, func(_ context.Context, req ActionRequest) (*ActionResult, error) {
		mu.Lock()
		dispatched = append(dispatched, req.Action)
		mu.Unlock()
		if req.Action == "installer.a" {
			return nil, fmt.Errorf("step a always fails")
		}
		return &ActionResult{OK: true}, nil
	})

	run, _ := (&Engine{Router: router}).Execute(context.Background(), def, nil)
	gotStatus := RunStatus("")
	if run != nil {
		gotStatus = run.Status
	}
	if gotStatus != RunFailed {
		t.Fatalf("want RunFailed (step a fails, no defer), got %q", gotStatus)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(dispatched) == 0 {
		t.Fatal("no actions dispatched — expected step a to be attempted")
	}
	seen := map[string]bool{}
	for _, a := range dispatched {
		seen[a] = true
		if !planActions[a] {
			t.Errorf("engine dispatched action %q which is NOT in the compiled plan — alternative-path violation", a)
		}
	}
	// b depends on the failed a, so b must never have been dispatched: the
	// engine does not fall forward to a downstream action when the upstream
	// intent failed.
	if seen["installer.b"] {
		t.Error("downstream action installer.b was dispatched despite its dependency failing")
	}
	// The only action ever dispatched is the failed step's own action (its
	// retries) — never an alternative.
	if !seen["installer.a"] || len(seen) != 1 {
		t.Errorf("expected only installer.a dispatched (its own retries), got distinct actions %v", keysOfBoolMap(seen))
	}
}

func keysOfBoolMap(m map[string]bool) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
