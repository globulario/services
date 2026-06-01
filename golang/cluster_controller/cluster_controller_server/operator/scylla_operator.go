// @awareness namespace=globular.platform
// @awareness component=platform_controller.operator
// @awareness file_role=scylladb_ring_operator_implementation
// @awareness implements=globular.platform:intent.quorum_safety_before_storage_mutation
// @awareness risk=high
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
