package operator

import "context"

type ScyllaOperator struct {
	nodes func() []string
}

func NewScyllaOperator(nodes func() []string) Operator {
	return &ScyllaOperator{nodes: nodes}
}

func (o *ScyllaOperator) Name() string        { return "scylla" }
func (o *ScyllaOperator) DependsOn() []string { return nil }

func (o *ScyllaOperator) AdmitPlan(ctx context.Context, req AdmitRequest) (AdmitDecision, error) {
	return AdmitDecision{Allowed: true}, nil
}

func (o *ScyllaOperator) Status(ctx context.Context, clusterID string) (*ServiceHealth, error) {
	return nil, nil
}
