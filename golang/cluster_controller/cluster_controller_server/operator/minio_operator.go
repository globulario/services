// @awareness namespace=globular.platform
// @awareness component=platform_controller.operator
// @awareness file_role=minio_objectstore_operator_implementation
// @awareness implements=globular.platform:intent.objectstore.topology_requires_contract
// @awareness risk=high
package operator

import "context"

type MinioOperator struct {
	nodes func() []string
}

func NewMinioOperator(nodes func() []string) Operator {
	return &MinioOperator{nodes: nodes}
}

func (o *MinioOperator) Name() string        { return "minio" }
func (o *MinioOperator) DependsOn() []string { return nil }

func (o *MinioOperator) AdmitPlan(ctx context.Context, req AdmitRequest) (AdmitDecision, error) {
	return AdmitDecision{Allowed: true}, nil
}

func (o *MinioOperator) Status(ctx context.Context, clusterID string) (*ServiceHealth, error) {
	return nil, nil
}
