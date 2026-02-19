package main

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/globulario/services/golang/plan/planpb"
)

type stubPlanStore struct {
	status *planpb.NodePlanStatus
}

func (s *stubPlanStore) PutCurrentPlan(ctx context.Context, nodeID string, plan *planpb.NodePlan) error {
	return nil
}
func (s *stubPlanStore) GetCurrentPlan(ctx context.Context, nodeID string) (*planpb.NodePlan, error) {
	return nil, nil
}
func (s *stubPlanStore) PutStatus(ctx context.Context, nodeID string, status *planpb.NodePlanStatus) error {
	s.status = status
	return nil
}
func (s *stubPlanStore) GetStatus(ctx context.Context, nodeID string) (*planpb.NodePlanStatus, error) {
	return s.status, nil
}
func (s *stubPlanStore) AppendHistory(ctx context.Context, nodeID string, plan *planpb.NodePlan) error {
	return nil
}

func TestLockConflictMarksPlanFailed(t *testing.T) {
	ps := &stubPlanStore{}
	srv := &NodeAgentServer{
		nodeID:     "node-1",
		planStore:  ps,
		state:      newNodeAgentState(),
		operations: map[string]*operation{},
	}
	srv.lockAcquirer = func(ctx context.Context, plan *planpb.NodePlan) (*planLockGuard, error) {
		return nil, errors.New("lock service:busy service:gateway")
	}

	plan := &planpb.NodePlan{
		NodeId: "node-1",
		PlanId: "plan-1",
		Locks:  []string{"service:gateway"},
		Spec:   &planpb.PlanSpec{},
	}

	srv.runStoredPlan(context.Background(), plan, nil)

	if ps.status == nil {
		t.Fatalf("expected status to be written")
	}
	if ps.status.GetState() != planpb.PlanState_PLAN_FAILED {
		t.Fatalf("expected state PLAN_FAILED, got %v", ps.status.GetState())
	}
	if ps.status.GetFinishedUnixMs() == 0 {
		t.Fatalf("expected FinishedUnixMs to be set")
	}
	if ps.status.GetErrorMessage() == "" || ps.status.GetErrorMessage()[:13] != "LOCK_CONFLICT" {
		t.Fatalf("expected LOCK_CONFLICT in error message, got %q", ps.status.GetErrorMessage())
	}
	if ps.status.GetCurrentStepId() != "" || len(ps.status.GetSteps()) != 0 {
		t.Fatalf("expected no steps to run on lock conflict")
	}
	if ps.status.GetStartedUnixMs() == 0 {
		t.Fatalf("expected StartedUnixMs set by newPlanStatus")
	}
	// Ensure timestamp sanity
	if time.Since(time.UnixMilli(int64(ps.status.GetFinishedUnixMs()))) > time.Minute {
		t.Fatalf("finished timestamp too old")
	}
}
