package main

import (
	"context"
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/config"
	"google.golang.org/grpc"
)

type fakeObjectStoreTopologyClient struct {
	req  *cluster_controllerpb.ApplyObjectStoreTopologyRequest
	resp *cluster_controllerpb.ApplyObjectStoreTopologyResponse
	err  error
}

func (f *fakeObjectStoreTopologyClient) ApplyObjectStoreTopology(_ context.Context, req *cluster_controllerpb.ApplyObjectStoreTopologyRequest, _ ...grpc.CallOption) (*cluster_controllerpb.ApplyObjectStoreTopologyResponse, error) {
	f.req = req
	if f.resp == nil {
		f.resp = &cluster_controllerpb.ApplyObjectStoreTopologyResponse{Status: "accepted", Generation: 1}
	}
	return f.resp, f.err
}

// TestRunObjectstoreTopologyApply_RoutesThroughOwnerRPC proves the RT-2 migration
// of `objectstore topology apply`: the apply is driven through the controller's
// typed ApplyObjectStoreTopology RPC (the owner of objectstore desired topology),
// carrying the proposal id and the destructive flag — not a raw etcd
// apply_request/apply_result handshake.
func TestRunObjectstoreTopologyApply_RoutesThroughOwnerRPC(t *testing.T) {
	oldConn := controllerConnFactory
	oldFactory := objectStoreTopologyClientFactory
	oldLoad := applyLoadProposal
	t.Cleanup(func() {
		controllerConnFactory = oldConn
		objectStoreTopologyClientFactory = oldFactory
		applyLoadProposal = oldLoad
	})
	controllerConnFactory = func() (grpc.ClientConnInterface, error) { return nil, nil }
	fc := &fakeObjectStoreTopologyClient{}
	objectStoreTopologyClientFactory = func(grpc.ClientConnInterface) objectStoreTopologyClient { return fc }

	// Valid (no validation errors), non-destructive proposal so pre-flight passes.
	applyLoadProposal = func(context.Context, string) (*config.TopologyProposal, error) {
		return &config.TopologyProposal{ProposalID: "prop-1", Status: "proposed"}, nil
	}

	applyProposalID = "prop-1"
	applyForceDestructive = false
	t.Cleanup(func() { applyProposalID = ""; applyForceDestructive = false })

	if err := runObjectstoreTopologyApply(objectstoreTopologyApplyCmd, nil); err != nil {
		t.Fatalf("runObjectstoreTopologyApply: %v", err)
	}
	if fc.req == nil {
		t.Fatal("expected ApplyObjectStoreTopology (owner RPC) to be called — no raw etcd handshake")
	}
	if fc.req.GetProposalId() != "prop-1" {
		t.Errorf("expected proposal_id=prop-1, got %q", fc.req.GetProposalId())
	}
}

// TestRunObjectstoreTopologyApply_DestructiveNeedsFlag proves the local pre-flight
// still refuses a destructive proposal without --i-understand-data-reset, before
// any RPC is issued.
func TestRunObjectstoreTopologyApply_DestructiveNeedsFlag(t *testing.T) {
	oldFactory := objectStoreTopologyClientFactory
	oldLoad := applyLoadProposal
	t.Cleanup(func() {
		objectStoreTopologyClientFactory = oldFactory
		applyLoadProposal = oldLoad
	})
	fc := &fakeObjectStoreTopologyClient{}
	objectStoreTopologyClientFactory = func(grpc.ClientConnInterface) objectStoreTopologyClient { return fc }
	applyLoadProposal = func(context.Context, string) (*config.TopologyProposal, error) {
		return &config.TopologyProposal{ProposalID: "prop-d", IsDestructive: true, DestructiveReasons: []string{"wipes data"}}, nil
	}

	applyProposalID = "prop-d"
	applyForceDestructive = false
	t.Cleanup(func() { applyProposalID = ""; applyForceDestructive = false })

	if err := runObjectstoreTopologyApply(objectstoreTopologyApplyCmd, nil); err == nil {
		t.Fatal("expected destructive apply to be refused without --i-understand-data-reset")
	}
	if fc.req != nil {
		t.Error("RPC must not be called when pre-flight rejects the proposal")
	}
}
