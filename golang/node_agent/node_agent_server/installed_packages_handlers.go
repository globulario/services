package main

import (
	"context"
	"strings"

	"github.com/globulario/services/golang/installed_state"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ListInstalledPackages returns all installed packages on this node, optionally filtered by kind.
func (srv *NodeAgentServer) ListInstalledPackages(ctx context.Context, req *node_agentpb.ListInstalledPackagesRequest) (*node_agentpb.ListInstalledPackagesResponse, error) {
	nodeID := strings.TrimSpace(req.GetNodeId())
	if nodeID == "" {
		nodeID = srv.nodeID
	}

	kind := strings.TrimSpace(req.GetKind())
	pkgs, err := installed_state.ListInstalledPackages(ctx, nodeID, kind)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "list installed packages: %v", err)
	}
	return &node_agentpb.ListInstalledPackagesResponse{Packages: pkgs}, nil
}

// SetInstalledPackage writes or updates an installed package record in etcd.
func (srv *NodeAgentServer) SetInstalledPackage(ctx context.Context, req *node_agentpb.SetInstalledPackageRequest) (*node_agentpb.SetInstalledPackageResponse, error) {
	pkg := req.GetPackage()
	if pkg == nil {
		return nil, status.Error(codes.InvalidArgument, "package is required")
	}
	if strings.TrimSpace(pkg.GetName()) == "" {
		return nil, status.Error(codes.InvalidArgument, "package name is required")
	}
	if strings.TrimSpace(pkg.GetKind()) == "" {
		return nil, status.Error(codes.InvalidArgument, "package kind is required")
	}

	// Default node_id to this node if not specified.
	if strings.TrimSpace(pkg.GetNodeId()) == "" {
		pkg.NodeId = srv.nodeID
	}

	if err := installed_state.WriteInstalledPackage(ctx, pkg); err != nil {
		return nil, status.Errorf(codes.Internal, "set installed package: %v", err)
	}
	return &node_agentpb.SetInstalledPackageResponse{
		Ok:      true,
		Message: "package " + pkg.GetName() + " set to " + pkg.GetStatus(),
	}, nil
}

// GetInstalledPackage returns a single installed package record.
func (srv *NodeAgentServer) GetInstalledPackage(ctx context.Context, req *node_agentpb.GetInstalledPackageRequest) (*node_agentpb.GetInstalledPackageResponse, error) {
	nodeID := strings.TrimSpace(req.GetNodeId())
	if nodeID == "" {
		nodeID = srv.nodeID
	}
	kind := strings.TrimSpace(req.GetKind())
	name := strings.TrimSpace(req.GetName())
	if name == "" {
		return nil, status.Error(codes.InvalidArgument, "name is required")
	}
	if kind == "" {
		return nil, status.Error(codes.InvalidArgument, "kind is required")
	}

	pkg, err := installed_state.GetInstalledPackage(ctx, nodeID, kind, name)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "get installed package: %v", err)
	}
	if pkg == nil {
		return nil, status.Errorf(codes.NotFound, "package %s/%s not found on node %s", kind, name, nodeID)
	}
	return &node_agentpb.GetInstalledPackageResponse{Package: pkg}, nil
}
