package operator

import (
	"context"
	"testing"

	"github.com/globulario/services/golang/plan/planpb"
)

type testOp struct {
	allowed bool
}

func (o testOp) Name() string        { return "test" }
func (o testOp) DependsOn() []string { return nil }
func (o testOp) AdmitPlan(ctx context.Context, req AdmitRequest) (AdmitDecision, error) {
	return AdmitDecision{Allowed: o.allowed, Reason: "test"}, nil
}
func (o testOp) MutatePlan(ctx context.Context, req MutateRequest) (*planpb.NodePlan, error) {
	return req.Plan, nil
}
func (o testOp) Status(ctx context.Context, clusterID string) (*ServiceHealth, error) {
	return &ServiceHealth{Service: "svc", HealthyNodes: 1, TotalNodes: 1}, nil
}

func TestRegistryDefault(t *testing.T) {
	op := Get("unknown")
	dec, err := op.AdmitPlan(context.Background(), AdmitRequest{Service: "x"})
	if err != nil {
		t.Fatalf("admit: %v", err)
	}
	if !dec.Allowed {
		t.Fatalf("default operator should allow")
	}
}

func TestRegistryCustom(t *testing.T) {
	Register("svc", testOp{allowed: false})
	op := Get("svc")
	dec, err := op.AdmitPlan(context.Background(), AdmitRequest{Service: "svc"})
	if err != nil {
		t.Fatalf("admit: %v", err)
	}
	if dec.Allowed {
		t.Fatalf("expected deny from custom operator")
	}
}
