package main

import (
	"context"

	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// server implements ClusterControllerService with placeholder logic until the control plane is wired.
type server struct {
	clustercontrollerpb.UnimplementedClusterControllerServiceServer

	cfg     *clusterControllerConfig
	cfgPath string
}

func newServer(cfg *clusterControllerConfig, cfgPath string) *server {
	return &server{
		cfg:     cfg,
		cfgPath: cfgPath,
	}
}

func (srv *server) Enroll(ctx context.Context, req *clustercontrollerpb.EnrollRequest) (*clustercontrollerpb.EnrollResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "ClusterController.Enroll is not implemented yet")
}

func (srv *server) ListJoinRequests(req *clustercontrollerpb.ListJoinRequestsRequest, stream clustercontrollerpb.ClusterControllerService_ListJoinRequestsServer) error {
	return status.Errorf(codes.Unimplemented, "ListJoinRequests is not implemented yet")
}

func (srv *server) ApproveNode(ctx context.Context, req *clustercontrollerpb.ApproveNodeRequest) (*clustercontrollerpb.ApproveNodeResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "ApproveNode is not implemented yet")
}

func (srv *server) RejectNode(ctx context.Context, req *clustercontrollerpb.RejectNodeRequest) (*clustercontrollerpb.RejectNodeResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "RejectNode is not implemented yet")
}

func (srv *server) ListNodes(req *clustercontrollerpb.ListNodesRequest, stream clustercontrollerpb.ClusterControllerService_ListNodesServer) error {
	return status.Errorf(codes.Unimplemented, "ListNodes is not implemented yet")
}

func (srv *server) SetNodeProfiles(ctx context.Context, req *clustercontrollerpb.SetNodeProfilesRequest) (*clustercontrollerpb.SetNodeProfilesResponse, error) {
	return nil, status.Errorf(codes.Unimplemented, "SetNodeProfiles is not implemented yet")
}

func (srv *server) WatchNodeOperations(req *clustercontrollerpb.WatchNodeOperationsRequest, stream clustercontrollerpb.ClusterControllerService_WatchNodeOperationsServer) error {
	return status.Errorf(codes.Unimplemented, "WatchNodeOperations is not implemented yet")
}
