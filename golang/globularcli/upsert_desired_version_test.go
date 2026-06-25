package main

import (
	"context"
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"google.golang.org/grpc"
)

type fakeDesiredServiceClient struct {
	lastReq *cluster_controllerpb.UpsertDesiredServiceRequest
	err     error
}

func (f *fakeDesiredServiceClient) UpsertDesiredService(_ context.Context, req *cluster_controllerpb.UpsertDesiredServiceRequest, _ ...grpc.CallOption) (*cluster_controllerpb.DesiredState, error) {
	f.lastReq = req
	return &cluster_controllerpb.DesiredState{}, f.err
}

// TestUpsertServiceDesiredVersion_RoutesThroughOwnerRPC proves the RT-2 migration of
// `pkg override apply/remove`: the ServiceDesiredVersion write goes through the
// controller's typed UpsertDesiredService RPC (the owner of /globular/resources) —
// not a raw etcd write — carrying the exact (service, version, build) identity.
func TestUpsertServiceDesiredVersion_RoutesThroughOwnerRPC(t *testing.T) {
	oldConn := controllerConnFactory
	oldFactory := desiredServiceClientFactory
	defer func() { controllerConnFactory = oldConn; desiredServiceClientFactory = oldFactory }()
	controllerConnFactory = func() (grpc.ClientConnInterface, error) { return nil, nil }

	fc := &fakeDesiredServiceClient{}
	desiredServiceClientFactory = func(grpc.ClientConnInterface) desiredServiceClient { return fc }

	if err := upsertServiceDesiredVersion("echo", "1.2.3", 7, "bid-xyz"); err != nil {
		t.Fatalf("upsertServiceDesiredVersion: %v", err)
	}
	if fc.lastReq == nil {
		t.Fatal("expected UpsertDesiredService (owner RPC) to be called — no raw etcd write")
	}
	svc := fc.lastReq.GetService()
	if svc.GetServiceId() != "echo" || svc.GetVersion() != "1.2.3" ||
		svc.GetBuildNumber() != 7 || svc.GetBuildId() != "bid-xyz" {
		t.Errorf("DesiredService wrong: id=%q ver=%q build=%d bid=%q",
			svc.GetServiceId(), svc.GetVersion(), svc.GetBuildNumber(), svc.GetBuildId())
	}
}
