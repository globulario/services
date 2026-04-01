package operator

import (
	"context"
	"sync"

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
	RequeueAfterSeconds int
}

type MutateRequest struct {
	Service         string
	NodeID          string
	NodeIP          string
	DesiredDomain   string
	DesiredProtocol string
	ClusterID       string
}

type ServiceHealth struct {
	Healthy bool
	Message string
}

type Operator interface {
	Name() string
	DependsOn() []string
	AdmitPlan(ctx context.Context, req AdmitRequest) (AdmitDecision, error)
	Status(ctx context.Context, clusterID string) (*ServiceHealth, error)
}

func structpbFromMap(fields map[string]any) *structpb.Struct {
	if len(fields) == 0 {
		return nil
	}
	s, _ := structpb.NewStruct(fields)
	return s
}

type registry struct {
	mu  sync.RWMutex
	ops map[string]Operator
}

var global = &registry{ops: make(map[string]Operator)}

func Register(name string, op Operator) {
	global.mu.Lock()
	defer global.mu.Unlock()
	global.ops[name] = op
}

func Get(name string) Operator {
	global.mu.RLock()
	defer global.mu.RUnlock()
	if op, ok := global.ops[name]; ok {
		return op
	}
	return noopOperator{}
}

type noopOperator struct{}

func (noopOperator) Name() string                         { return "noop" }
func (noopOperator) DependsOn() []string                  { return nil }
func (noopOperator) AdmitPlan(ctx context.Context, req AdmitRequest) (AdmitDecision, error) {
	return AdmitDecision{Allowed: true}, nil
}
func (noopOperator) Status(ctx context.Context, clusterID string) (*ServiceHealth, error) {
	return nil, nil
}
