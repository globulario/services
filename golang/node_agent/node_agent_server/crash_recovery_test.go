package main

import (
	"context"
	"errors"
	"testing"

	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/actions"
	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/planexec"
	"github.com/globulario/services/golang/plan/planpb"
	"google.golang.org/protobuf/types/known/structpb"
)

// ── Crash recovery conformance tests ─────────────────────────────────────────

// crashCounterAction counts invocations and fails on the first call, succeeds on retries.
// When used as a probe, it also tracks calls separately so probes can fail until
// the step has succeeded at least once.
type crashCounterAction struct {
	stepCalls  int
	probeCalls int
	stepDone   bool
}

func (a *crashCounterAction) Name() string { return "test.crash_counter" }
func (a *crashCounterAction) Validate(args *structpb.Struct) error {
	return nil
}
func (a *crashCounterAction) Apply(ctx context.Context, args *structpb.Struct) (string, error) {
	a.stepCalls++
	if a.stepCalls == 1 {
		return "", errors.New("simulated crash")
	}
	a.stepDone = true
	return "ok", nil
}

// counterProbeAction is a probe that fails until the crashCounterAction has succeeded.
type counterProbeAction struct {
	counter *crashCounterAction
}

func (p *counterProbeAction) Name() string                         { return "test.counter_probe" }
func (p *counterProbeAction) Validate(args *structpb.Struct) error { return nil }
func (p *counterProbeAction) Apply(ctx context.Context, args *structpb.Struct) (string, error) {
	if !p.counter.stepDone {
		return "", errors.New("step not yet completed")
	}
	return "ok", nil
}

// alwaysOKAction always succeeds.
type alwaysOKAction struct{}

func (alwaysOKAction) Name() string                                                          { return "test.always_ok" }
func (alwaysOKAction) Validate(args *structpb.Struct) error                                  { return nil }
func (alwaysOKAction) Apply(ctx context.Context, args *structpb.Struct) (string, error) {
	return "ok", nil
}

// alwaysFailAction always fails (simulates unreachable repository).
type alwaysFailAction struct{}

func (alwaysFailAction) Name() string                                                          { return "test.always_fail" }
func (alwaysFailAction) Validate(args *structpb.Struct) error                                  { return nil }
func (alwaysFailAction) Apply(ctx context.Context, args *structpb.Struct) (string, error) {
	return "", errors.New("repository unavailable")
}

func init() {
	actions.Register(alwaysOKAction{})
	actions.Register(alwaysFailAction{})
}

func TestConformance_InterruptedPlan_ResumableOnRetry(t *testing.T) {
	// Register stateful actions for this test.
	counter := &crashCounterAction{}
	probe := &counterProbeAction{counter: counter}
	actions.Register(counter)
	actions.Register(probe)

	plan := &planpb.NodePlan{
		NodeId: "node-1",
		Spec: &planpb.PlanSpec{
			Steps: []*planpb.PlanStep{
				{
					Id:     "step-1",
					Action: "test.crash_counter",
					Policy: &planpb.StepPolicy{MaxRetries: 2},
				},
			},
			// Probe fails until step succeeds, preventing quick-success path.
			SuccessProbes: []*planpb.Probe{
				{Type: "test.counter_probe"},
			},
		},
	}

	runner := planexec.NewRunner("node-1", nil)
	status, err := runner.ReconcilePlan(context.Background(), plan, nil)

	// The plan should eventually succeed because the step passes on retry.
	if err != nil {
		t.Fatalf("ReconcilePlan returned error: %v", err)
	}
	if status.GetState() != planpb.PlanState_PLAN_SUCCEEDED {
		t.Errorf("expected PLAN_SUCCEEDED, got %v (error: %s)", status.GetState(), status.GetErrorMessage())
	}
	if counter.stepCalls < 2 {
		t.Errorf("expected at least 2 calls to crash_counter step, got %d", counter.stepCalls)
	}
}

func TestConformance_AllStepsFail_PlanFails(t *testing.T) {
	plan := &planpb.NodePlan{
		NodeId: "node-1",
		Policy: &planpb.PlanPolicy{
			MaxRetries:  0,
			FailureMode: planpb.FailureMode_FAILURE_MODE_ABORT,
		},
		Spec: &planpb.PlanSpec{
			Steps: []*planpb.PlanStep{
				{Id: "step-fail", Action: "test.always_fail"},
			},
			// Need a failing probe so EvaluateInvariants doesn't short-circuit to success.
			SuccessProbes: []*planpb.Probe{
				{Type: "test.always_fail"},
			},
		},
	}

	runner := planexec.NewRunner("node-1", nil)
	status, _ := runner.ReconcilePlan(context.Background(), plan, nil)
	if status.GetState() != planpb.PlanState_PLAN_FAILED {
		t.Errorf("expected PLAN_FAILED, got %v", status.GetState())
	}
}

func TestConformance_RollbackTriggeredOnStepFailure(t *testing.T) {
	plan := &planpb.NodePlan{
		NodeId: "node-1",
		Policy: &planpb.PlanPolicy{
			MaxRetries:  0,
			FailureMode: planpb.FailureMode_FAILURE_MODE_ROLLBACK,
		},
		Spec: &planpb.PlanSpec{
			Steps: []*planpb.PlanStep{
				{Id: "step-fail", Action: "test.always_fail"},
			},
			Rollback: []*planpb.PlanStep{
				{Id: "rollback-1", Action: "test.always_ok"},
			},
			SuccessProbes: []*planpb.Probe{
				{Type: "test.always_fail"},
			},
		},
	}

	runner := planexec.NewRunner("node-1", nil)
	status, _ := runner.ReconcilePlan(context.Background(), plan, nil)
	if status.GetState() != planpb.PlanState_PLAN_ROLLED_BACK {
		t.Errorf("expected PLAN_ROLLED_BACK, got %v", status.GetState())
	}
}

func TestConformance_RollbackFailure_PlanFails(t *testing.T) {
	plan := &planpb.NodePlan{
		NodeId: "node-1",
		Policy: &planpb.PlanPolicy{
			MaxRetries:  0,
			FailureMode: planpb.FailureMode_FAILURE_MODE_ROLLBACK,
		},
		Spec: &planpb.PlanSpec{
			Steps: []*planpb.PlanStep{
				{Id: "step-fail", Action: "test.always_fail"},
			},
			Rollback: []*planpb.PlanStep{
				{Id: "rollback-fail", Action: "test.always_fail"},
			},
			SuccessProbes: []*planpb.Probe{
				{Type: "test.always_fail"},
			},
		},
	}

	runner := planexec.NewRunner("node-1", nil)
	status, _ := runner.ReconcilePlan(context.Background(), plan, nil)
	if status.GetState() != planpb.PlanState_PLAN_FAILED {
		t.Errorf("expected PLAN_FAILED when rollback also fails, got %v", status.GetState())
	}
}

func TestConformance_NilPlan_NoError(t *testing.T) {
	runner := planexec.NewRunner("node-1", nil)
	status, err := runner.ReconcilePlan(context.Background(), nil, nil)
	if err != nil {
		t.Fatalf("nil plan should not error: %v", err)
	}
	// status can be nil for nil plan
	_ = status
}

func TestConformance_EmptySteps_SucceedsWithProbes(t *testing.T) {
	plan := &planpb.NodePlan{
		NodeId: "node-1",
		Spec: &planpb.PlanSpec{
			Steps: []*planpb.PlanStep{},
			SuccessProbes: []*planpb.Probe{
				{Type: "test.always_ok"},
			},
		},
	}

	runner := planexec.NewRunner("node-1", nil)
	status, err := runner.ReconcilePlan(context.Background(), plan, nil)
	if err != nil {
		t.Fatalf("ReconcilePlan error: %v", err)
	}
	if status.GetState() != planpb.PlanState_PLAN_SUCCEEDED {
		t.Errorf("expected PLAN_SUCCEEDED for empty steps with passing probes, got %v", status.GetState())
	}
}
