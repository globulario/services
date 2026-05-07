package runtime

import (
	"context"
	"fmt"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// GrpcServiceStatusSource pulls operational service state from the cluster_controller.
// It synthesizes ServiceStatus records from GetClusterHealthV1 per-node data.
type GrpcServiceStatusSource struct {
	addr   string
	conn   *grpc.ClientConn
	client cluster_controllerpb.ClusterControllerServiceClient
}

// NewGrpcServiceStatusSource dials the cluster_controller service at addr.
func NewGrpcServiceStatusSource(addr string) (*GrpcServiceStatusSource, error) {
	if addr == "" {
		return nil, fmt.Errorf("service status source: addr is empty")
	}
	conn, err := grpc.NewClient(addr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("service status source: dial %s: %w", addr, err)
	}
	return &GrpcServiceStatusSource{
		addr:   addr,
		conn:   conn,
		client: cluster_controllerpb.NewClusterControllerServiceClient(conn),
	}, nil
}

// Close releases the gRPC connection.
func (s *GrpcServiceStatusSource) Close() { _ = s.conn.Close() }

// SourceInfo implements sourceIdentifier.
func (s *GrpcServiceStatusSource) SourceInfo() (string, bool) { return "cluster_controller.grpc", false }

// Services returns per-node service statuses derived from GetClusterHealthV1.
func (s *GrpcServiceStatusSource) Services(ctx context.Context) ([]ServiceStatus, error) {
	resp, err := s.client.GetClusterHealthV1(ctx, &cluster_controllerpb.GetClusterHealthV1Request{})
	if err != nil {
		return nil, fmt.Errorf("GetClusterHealthV1: %w", err)
	}

	// Build a map of desired versions for convergence state annotation.
	desiredVersion := make(map[string]string)
	for _, svc := range resp.GetServices() {
		desiredVersion[svc.GetServiceName()] = svc.GetDesiredVersion()
	}

	var out []ServiceStatus
	for _, node := range resp.GetNodes() {
		nodeID := node.GetNodeId()
		converged := node.GetDesiredServicesHash() == node.GetAppliedServicesHash()
		for svcName, version := range node.GetInstalledVersions() {
			state := "RUNNING"
			if !converged && version != desiredVersion[svcName] {
				state = "DEGRADED"
			}
			out = append(out, ServiceStatus{
				ServiceID: svcName,
				NodeID:    nodeID,
				Version:   version,
				State:     state,
			})
		}
	}
	return out, nil
}
