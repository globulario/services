package main

import (
	"context"
	"errors"
	"testing"

	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"google.golang.org/grpc"
)

// fakeNodeAgentClient embeds the full client interface so only the two methods
// setInstalledBuildID uses need real implementations; any other call would panic
// (and none is made).
type fakeNodeAgentClient struct {
	node_agentpb.NodeAgentServiceClient
	getPkg  *node_agentpb.InstalledPackage
	getErr  error
	lastSet *node_agentpb.SetInstalledPackageRequest
}

func (f *fakeNodeAgentClient) GetInstalledPackage(_ context.Context, _ *node_agentpb.GetInstalledPackageRequest, _ ...grpc.CallOption) (*node_agentpb.GetInstalledPackageResponse, error) {
	if f.getErr != nil {
		return nil, f.getErr
	}
	return &node_agentpb.GetInstalledPackageResponse{Package: f.getPkg}, nil
}

func (f *fakeNodeAgentClient) SetInstalledPackage(_ context.Context, req *node_agentpb.SetInstalledPackageRequest, _ ...grpc.CallOption) (*node_agentpb.SetInstalledPackageResponse, error) {
	f.lastSet = req
	return &node_agentpb.SetInstalledPackageResponse{Ok: true}, nil
}

// TestSetInstalledBuildID_StampsViaOwnerRPC proves the RT-2 migration of
// `state canonicalize --fix-installed --metadata-only`: build_id is stamped through
// the node-agent's GetInstalledPackage + SetInstalledPackage (the owner of
// /globular/nodes) — not a raw etcd write — preserving every other field.
func TestSetInstalledBuildID_StampsViaOwnerRPC(t *testing.T) {
	fc := &fakeNodeAgentClient{
		getPkg: &node_agentpb.InstalledPackage{
			Name: "scylla", Kind: "INFRASTRUCTURE", NodeId: "node-1",
			Version: "5.4.0", Checksum: "sha256:abc", Status: "installed", BuildId: "",
		},
	}
	if err := setInstalledBuildID(context.Background(), fc, "node-1", "INFRASTRUCTURE", "scylla", "bid-123"); err != nil {
		t.Fatalf("setInstalledBuildID: %v", err)
	}
	if fc.lastSet == nil {
		t.Fatal("expected SetInstalledPackage (owner RPC) to be called — no raw etcd write")
	}
	pkg := fc.lastSet.GetPackage()
	if pkg.GetBuildId() != "bid-123" {
		t.Errorf("build_id = %q, want bid-123", pkg.GetBuildId())
	}
	if pkg.GetVersion() != "5.4.0" || pkg.GetChecksum() != "sha256:abc" || pkg.GetStatus() != "installed" {
		t.Errorf("sibling fields not preserved: version=%q checksum=%q status=%q",
			pkg.GetVersion(), pkg.GetChecksum(), pkg.GetStatus())
	}
}

// TestSetInstalledBuildID_NotFound: when the record is absent, fail without writing.
func TestSetInstalledBuildID_NotFound(t *testing.T) {
	fc := &fakeNodeAgentClient{getPkg: nil}
	if err := setInstalledBuildID(context.Background(), fc, "node-1", "SERVICE", "ghost", "bid"); err == nil {
		t.Error("expected error when installed package not found")
	}
	if fc.lastSet != nil {
		t.Error("must not SetInstalledPackage when the record was not found")
	}
}

// TestSetInstalledBuildID_GetError: a GET failure surfaces and skips the SET.
func TestSetInstalledBuildID_GetError(t *testing.T) {
	fc := &fakeNodeAgentClient{getErr: errors.New("agent down")}
	if err := setInstalledBuildID(context.Background(), fc, "node-1", "SERVICE", "echo", "bid"); err == nil {
		t.Error("expected error when GetInstalledPackage fails")
	}
	if fc.lastSet != nil {
		t.Error("must not SetInstalledPackage when GET failed")
	}
}
