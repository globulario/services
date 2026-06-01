// @awareness namespace=globular.platform
// @awareness component=platform_controller.operator
// @awareness file_role=etcd_cluster_operator_implementation
// @awareness implements=globular.platform:intent.etcd.is_source_of_truth
// @awareness risk=critical
package operator

import "context"

type EtcdOperator struct {
	nodes func() []string
}

func NewEtcdOperator(nodes func() []string) Operator {
	return &EtcdOperator{nodes: nodes}
}

func (o *EtcdOperator) Name() string        { return "etcd" }
func (o *EtcdOperator) DependsOn() []string { return nil }

func (o *EtcdOperator) AdmitPlan(ctx context.Context, req AdmitRequest) (AdmitDecision, error) {
	return AdmitDecision{Allowed: true}, nil
}

func (o *EtcdOperator) Status(ctx context.Context, clusterID string) (*ServiceHealth, error) {
	return nil, nil
}
