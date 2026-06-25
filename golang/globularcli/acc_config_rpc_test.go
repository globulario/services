package main

import (
	"context"
	"encoding/json"
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"google.golang.org/grpc"
)

type fakeAccConfigClient struct {
	setReq   *cluster_controllerpb.SetAccConfigRequest
	resetReq *cluster_controllerpb.ResetAccConfigRequest
	deleted  bool
}

func (f *fakeAccConfigClient) SetAccConfig(_ context.Context, req *cluster_controllerpb.SetAccConfigRequest, _ ...grpc.CallOption) (*cluster_controllerpb.SetAccConfigResponse, error) {
	f.setReq = req
	return &cluster_controllerpb.SetAccConfigResponse{Ok: true}, nil
}

func (f *fakeAccConfigClient) ResetAccConfig(_ context.Context, req *cluster_controllerpb.ResetAccConfigRequest, _ ...grpc.CallOption) (*cluster_controllerpb.ResetAccConfigResponse, error) {
	f.resetReq = req
	return &cluster_controllerpb.ResetAccConfigResponse{Deleted: f.deleted}, nil
}

func withFakeAccClient(t *testing.T, fc accConfigClient) {
	t.Helper()
	oldConn := controllerConnFactory
	oldFactory := accConfigClientFactory
	t.Cleanup(func() { controllerConnFactory = oldConn; accConfigClientFactory = oldFactory })
	controllerConnFactory = func() (grpc.ClientConnInterface, error) { return nil, nil }
	accConfigClientFactory = func(grpc.ClientConnInterface) accConfigClient { return fc }
}

// TestRunAccSet_RoutesThroughOwnerRPC proves the RT-2 migration of `cluster acc set`:
// the ACC config write goes through the controller's typed SetAccConfig RPC (the
// owner of /globular/system/acc/config) carrying well-formed JSON — not a raw etcd
// Put. The merge of operator flags happens CLI-side; the controller receives the
// final opaque blob.
func TestRunAccSet_RoutesThroughOwnerRPC(t *testing.T) {
	fc := &fakeAccConfigClient{}
	withFakeAccClient(t, fc)

	// Supply an empty current config so the merge-read does not touch real etcd.
	oldRead := accReadCurrentConfig
	t.Cleanup(func() { accReadCurrentConfig = oldRead })
	accReadCurrentConfig = func(context.Context) (accConfig, error) { return accConfig{}, nil }

	// A flag must be set, otherwise runAccSet refuses before any write.
	accSetP1AuthzSize = 321
	t.Cleanup(func() { accSetP1AuthzSize = 0 })

	if err := runAccSet(accSetCmd, nil); err != nil {
		t.Fatalf("runAccSet: %v", err)
	}
	if fc.setReq == nil {
		t.Fatal("expected SetAccConfig (owner RPC) to be called — no raw etcd write")
	}
	var got accConfig
	if err := json.Unmarshal(fc.setReq.GetConfigJson(), &got); err != nil {
		t.Fatalf("config_json is not valid JSON: %v", err)
	}
	if got.P1AuthzSize != 321 {
		t.Errorf("expected merged P1AuthzSize=321 in committed blob, got %d", got.P1AuthzSize)
	}
}

// TestRunAccReset_RoutesThroughOwnerRPC proves `cluster acc reset` retracts the key
// through the owner's typed ResetAccConfig RPC, not a raw etcd Delete.
func TestRunAccReset_RoutesThroughOwnerRPC(t *testing.T) {
	fc := &fakeAccConfigClient{deleted: true}
	withFakeAccClient(t, fc)

	if err := runAccReset(accResetCmd, nil); err != nil {
		t.Fatalf("runAccReset: %v", err)
	}
	if fc.resetReq == nil {
		t.Fatal("expected ResetAccConfig (owner RPC) to be called — no raw etcd delete")
	}
}
