package operator

import (
	"context"
	"strings"
	"sync"

	"github.com/globulario/services/golang/plan/planpb"
	"google.golang.org/protobuf/types/known/structpb"
)

type AdmitRequest struct {
	Service        string
	NodeID         string
	DesiredVersion string
	DesiredHash    string
}

type AdmitDecision struct {
	Allowed             bool
	Reason              string
	RequeueAfterSeconds int64
}

type MutateRequest struct {
	Service         string
	NodeID          string
	Plan            *planpb.NodePlan
	DesiredDomain   string
	DesiredProtocol string
	ClusterID       string
}

type ServiceHealth struct {
	Service        string
	DesiredVersion string
	HealthyNodes   int32
	TotalNodes     int32
	Message        string
}

type Operator interface {
	Name() string
	DependsOn() []string
	AdmitPlan(ctx context.Context, req AdmitRequest) (AdmitDecision, error)
	MutatePlan(ctx context.Context, req MutateRequest) (*planpb.NodePlan, error)
	Status(ctx context.Context, clusterID string) (*ServiceHealth, error)
}

// Common helpers shared by operators.
func addLock(plan *planpb.NodePlan, lock string) {
	if plan == nil {
		return
	}
	for _, l := range plan.Locks {
		if l == lock {
			return
		}
	}
	plan.Locks = append(plan.Locks, lock)
}

func addProbe(plan *planpb.NodePlan, probe *planpb.Probe) {
	if plan == nil || plan.Spec == nil || probe == nil {
		return
	}
	plan.Spec.SuccessProbes = append(plan.Spec.SuccessProbes, probe)
}

func structpbFromMap(fields map[string]interface{}) *structpb.Struct {
	if len(fields) == 0 {
		return nil
	}
	s, _ := structpb.NewStruct(fields)
	return s
}

func isServicePlan(plan *planpb.NodePlan, svc string) bool {
	if plan == nil || plan.Spec == nil || plan.Spec.Desired == nil {
		return false
	}
	for _, s := range plan.Spec.Desired.Services {
		if strings.Contains(strings.ToLower(s.GetName()), strings.ToLower(svc)) {
			return true
		}
	}
	return false
}

type registry struct {
	mu        sync.RWMutex
	ops       map[string]Operator
	defaultOp Operator
}

var reg = &registry{
	ops:       make(map[string]Operator),
	defaultOp: noopOperator{},
}

func Register(service string, op Operator) {
	reg.mu.Lock()
	defer reg.mu.Unlock()
	reg.ops[service] = op
}

func Get(service string) Operator {
	reg.mu.RLock()
	defer reg.mu.RUnlock()
	if op, ok := reg.ops[service]; ok {
		return op
	}
	return reg.defaultOp
}

type noopOperator struct{}

func (noopOperator) Name() string        { return "noop" }
func (noopOperator) DependsOn() []string { return nil }
func (noopOperator) AdmitPlan(ctx context.Context, req AdmitRequest) (AdmitDecision, error) {
	return AdmitDecision{Allowed: true}, nil
}
func (noopOperator) MutatePlan(ctx context.Context, req MutateRequest) (*planpb.NodePlan, error) {
	return req.Plan, nil
}
func (noopOperator) Status(ctx context.Context, clusterID string) (*ServiceHealth, error) {
	return nil, nil
}
