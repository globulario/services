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
