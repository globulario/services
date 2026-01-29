package main

import (
	"context"
	"testing"

	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
	"github.com/globulario/services/golang/clustercontroller/resourcestore"
	"github.com/globulario/services/golang/plan/planpb"
	"go.etcd.io/etcd/api/v3/mvccpb"
	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/protobuf/proto"
)

type mapKV struct {
	data map[string]string
}

func newMapKV() *mapKV {
	return &mapKV{data: make(map[string]string)}
}

func (m *mapKV) Get(_ context.Context, key string, _ ...clientv3.OpOption) (*clientv3.GetResponse, error) {
	val, ok := m.data[key]
	if !ok {
		return &clientv3.GetResponse{}, nil
	}
	return &clientv3.GetResponse{
		Kvs: []*mvccpb.KeyValue{
			{Key: []byte(key), Value: []byte(val)},
		},
	}, nil
}

func (m *mapKV) Put(_ context.Context, key, val string, _ ...clientv3.OpOption) (*clientv3.PutResponse, error) {
	m.data[key] = val
	return &clientv3.PutResponse{}, nil
}

type fakePlanStore struct {
	lastPlan *planpb.NodePlan
	count    int
	status   *planpb.NodePlanStatus
}

func (f *fakePlanStore) PutCurrentPlan(ctx context.Context, nodeID string, plan *planpb.NodePlan) error {
	f.count++
	f.lastPlan = proto.Clone(plan).(*planpb.NodePlan)
	return nil
}
func (f *fakePlanStore) GetCurrentPlan(ctx context.Context, nodeID string) (*planpb.NodePlan, error) {
	if f.lastPlan == nil {
		return nil, nil
	}
	return proto.Clone(f.lastPlan).(*planpb.NodePlan), nil
}
func (f *fakePlanStore) PutStatus(ctx context.Context, nodeID string, status *planpb.NodePlanStatus) error {
	if status == nil {
		f.status = nil
		return nil
	}
	f.status = proto.Clone(status).(*planpb.NodePlanStatus)
	return nil
}
func (f *fakePlanStore) GetStatus(ctx context.Context, nodeID string) (*planpb.NodePlanStatus, error) {
	if f.status == nil {
		return nil, nil
	}
	return proto.Clone(f.status).(*planpb.NodePlanStatus), nil
}
func (f *fakePlanStore) AppendHistory(ctx context.Context, nodeID string, plan *planpb.NodePlan) error {
	return nil
}

func TestReconcileSkipsWhenHashUnchanged(t *testing.T) {
	kv := newMapKV()
	ps := &fakePlanStore{}
	srv := &server{
		cfg:       &clusterControllerConfig{},
		state:     &controllerState{Nodes: map[string]*nodeState{"n1": {NodeID: "n1"}}},
		kv:        kv,
		planStore: ps,
		resources: resourcestore.NewMemStore(),
	}
	net := &clustercontrollerpb.DesiredNetwork{Domain: "example.com", Protocol: "http", PortHttp: 80}
	_, _ = srv.resources.Apply(context.Background(), "ClusterNetwork", &clustercontrollerpb.ClusterNetwork{
		Meta: &clustercontrollerpb.ObjectMeta{Name: "default", Generation: 1},
		Spec: &clustercontrollerpb.ClusterNetworkSpec{
			ClusterDomain: net.GetDomain(),
			Protocol:      net.GetProtocol(),
			PortHttp:      net.GetPortHttp(),
			PortHttps:     net.GetPortHttps(),
		},
	})
	srv.reconcileNodes(context.Background())
	if ps.count != 1 {
		t.Fatalf("expected 1 plan write, got %d", ps.count)
	}
	// Mark applied to simulate convergence so second reconcile should skip.
	hash, err := hashDesiredNetwork(net)
	if err != nil {
		t.Fatalf("hashDesiredNetwork: %v", err)
	}
	if err := srv.putNodeAppliedHash(context.Background(), "n1", hash); err != nil {
		t.Fatalf("putNodeAppliedHash: %v", err)
	}
	srv.reconcileNodes(context.Background())
	if ps.count != 1 {
		t.Fatalf("expected no additional plan writes, got %d", ps.count)
	}
}
