package main

import (
	"context"
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/config"
	"google.golang.org/grpc"
)

type fakeObjectStorePlacementClient struct {
	approveReq *cluster_controllerpb.ApproveObjectStoreDiskRequest
	rejectReq  *cluster_controllerpb.RejectObjectStoreDiskRequest
	planReq    *cluster_controllerpb.PlanObjectStoreTopologyRequest
}

func (f *fakeObjectStorePlacementClient) ApproveObjectStoreDisk(_ context.Context, req *cluster_controllerpb.ApproveObjectStoreDiskRequest, _ ...grpc.CallOption) (*cluster_controllerpb.ApproveObjectStoreDiskResponse, error) {
	f.approveReq = req
	return &cluster_controllerpb.ApproveObjectStoreDiskResponse{PathHash: "ph"}, nil
}
func (f *fakeObjectStorePlacementClient) RejectObjectStoreDisk(_ context.Context, req *cluster_controllerpb.RejectObjectStoreDiskRequest, _ ...grpc.CallOption) (*cluster_controllerpb.RejectObjectStoreDiskResponse, error) {
	f.rejectReq = req
	return &cluster_controllerpb.RejectObjectStoreDiskResponse{Ok: true}, nil
}
func (f *fakeObjectStorePlacementClient) PlanObjectStoreTopology(_ context.Context, req *cluster_controllerpb.PlanObjectStoreTopologyRequest, _ ...grpc.CallOption) (*cluster_controllerpb.PlanObjectStoreTopologyResponse, error) {
	f.planReq = req
	return &cluster_controllerpb.PlanObjectStoreTopologyResponse{ProposalId: "prop-1"}, nil
}

func withFakePlacement(t *testing.T) *fakeObjectStorePlacementClient {
	t.Helper()
	oldConn := controllerConnFactory
	oldFactory := objectStorePlacementClientFactory
	t.Cleanup(func() {
		controllerConnFactory = oldConn
		objectStorePlacementClientFactory = oldFactory
	})
	controllerConnFactory = func() (grpc.ClientConnInterface, error) { return nil, nil }
	fc := &fakeObjectStorePlacementClient{}
	objectStorePlacementClientFactory = func(grpc.ClientConnInterface) objectStorePlacementClient { return fc }
	return fc
}

// TestRunObjectstoreDiskApprove_RoutesThroughOwnerRPC proves `disk approve` admits
// through the controller's ApproveObjectStoreDisk RPC (the owner of placement),
// not a direct config.SaveAdmittedDisk write.
func TestRunObjectstoreDiskApprove_RoutesThroughOwnerRPC(t *testing.T) {
	fc := withFakePlacement(t)
	approveNodeID, approveNodeIP, approvePath, approveDrives = "n1", "10.0.0.1", "/mnt/data", 2
	approveForceRoot, approveForceData = false, false
	t.Cleanup(func() {
		approveNodeID, approveNodeIP, approvePath, approveDrives = "", "", "", 0
		approveForceRoot, approveForceData = false, false
	})

	if err := runObjectstoreDiskApprove(objectstoreDiskApproveCmd, nil); err != nil {
		t.Fatalf("runObjectstoreDiskApprove: %v", err)
	}
	if fc.approveReq == nil {
		t.Fatal("expected ApproveObjectStoreDisk (owner RPC) — no direct config write")
	}
	if fc.approveReq.GetNodeId() != "n1" || fc.approveReq.GetPath() != "/mnt/data" || fc.approveReq.GetDrivesPerNode() != 2 {
		t.Errorf("wrong approve request: %+v", fc.approveReq)
	}
}

// TestRunObjectstoreDiskReject_RoutesThroughOwnerRPC proves `disk reject` routes
// through RejectObjectStoreDisk, not a direct config.DeleteAdmittedDisk write.
func TestRunObjectstoreDiskReject_RoutesThroughOwnerRPC(t *testing.T) {
	fc := withFakePlacement(t)
	rejectNodeID, rejectPath = "n1", "/mnt/data"
	t.Cleanup(func() { rejectNodeID, rejectPath = "", "" })

	if err := runObjectstoreDiskReject(objectstoreDiskRejectCmd, nil); err != nil {
		t.Fatalf("runObjectstoreDiskReject: %v", err)
	}
	if fc.rejectReq == nil {
		t.Fatal("expected RejectObjectStoreDisk (owner RPC) — no direct config write")
	}
	if fc.rejectReq.GetNodeId() != "n1" || fc.rejectReq.GetPath() != "/mnt/data" {
		t.Errorf("wrong reject request: %+v", fc.rejectReq)
	}
}

// TestRunObjectstoreTopologyPlan_RoutesThroughOwnerRPC proves `topology plan`
// builds the proposal CLI-side (reads) but persists it through
// PlanObjectStoreTopology, not a direct config.SaveTopologyProposal write.
func TestRunObjectstoreTopologyPlan_RoutesThroughOwnerRPC(t *testing.T) {
	fc := withFakePlacement(t)

	oldLoad := planLoadAdmittedDisks
	t.Cleanup(func() { planLoadAdmittedDisks = oldLoad })
	planLoadAdmittedDisks = func(context.Context) ([]*config.AdmittedDisk, error) {
		return []*config.AdmittedDisk{{NodeID: "n1", NodeIP: "10.0.0.1", Path: "/mnt/data", DrivesPerNode: 1}}, nil
	}

	oldJSON := planJSON
	planJSON = true // skip the human pretty-printer
	t.Cleanup(func() { planJSON = oldJSON })

	if err := runObjectstoreTopologyPlan(objectstoreTopologyPlanCmd, nil); err != nil {
		t.Fatalf("runObjectstoreTopologyPlan: %v", err)
	}
	if fc.planReq == nil {
		t.Fatal("expected PlanObjectStoreTopology (owner RPC) — no direct config write")
	}
	if len(fc.planReq.GetProposalJson()) == 0 {
		t.Error("expected the CLI-built proposal JSON to be sent to the controller")
	}
}
