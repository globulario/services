package main

import (
	"context"
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"google.golang.org/grpc"
)

type fakeObjectStorePoolClient struct {
	req  *cluster_controllerpb.SanitizeObjectStorePoolRequest
	resp *cluster_controllerpb.SanitizeObjectStorePoolResponse
}

func (f *fakeObjectStorePoolClient) SanitizeObjectStorePool(_ context.Context, req *cluster_controllerpb.SanitizeObjectStorePoolRequest, _ ...grpc.CallOption) (*cluster_controllerpb.SanitizeObjectStorePoolResponse, error) {
	f.req = req
	return f.resp, nil
}

// TestRunObjectstoreTopologySanitizePool_RoutesThroughOwnerRPC proves the RT-2
// migration of `objectstore topology sanitize-pool`: the sanitize is driven
// through the controller's typed SanitizeObjectStorePool RPC (the owner of
// /globular/clustercontroller/state), carrying the dry_run flag — not a raw etcd
// read-modify-write that would clobber the controller state blob.
func TestRunObjectstoreTopologySanitizePool_RoutesThroughOwnerRPC(t *testing.T) {
	oldConn := controllerConnFactory
	oldFactory := objectStorePoolClientFactory
	oldDryRun := topoSanitizeDryRun
	t.Cleanup(func() {
		controllerConnFactory = oldConn
		objectStorePoolClientFactory = oldFactory
		topoSanitizeDryRun = oldDryRun
	})
	controllerConnFactory = func() (grpc.ClientConnInterface, error) { return nil, nil }
	fc := &fakeObjectStorePoolClient{resp: &cluster_controllerpb.SanitizeObjectStorePoolResponse{
		Before:     []string{"10.0.0.1", "10.0.0.2"},
		After:      []string{"10.0.0.1"},
		Removed:    []string{"10.0.0.2"},
		Generation: 7,
		Applied:    true,
	}}
	objectStorePoolClientFactory = func(grpc.ClientConnInterface) objectStorePoolClient { return fc }

	topoSanitizeDryRun = false
	if err := runObjectstoreTopologySanitizePool(objectstoreTopologySanitizePoolCmd, nil); err != nil {
		t.Fatalf("runObjectstoreTopologySanitizePool: %v", err)
	}
	if fc.req == nil {
		t.Fatal("expected SanitizeObjectStorePool (owner RPC) to be called — no raw etcd write")
	}
	if fc.req.GetDryRun() {
		t.Error("expected dry_run=false to propagate to the RPC")
	}
}
