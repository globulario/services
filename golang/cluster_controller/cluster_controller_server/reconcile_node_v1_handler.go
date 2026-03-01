package main

import (
	"context"
	"strings"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ReconcileNodeV1 triggers an immediate reconcile cycle for the cluster.
// It validates that the requested node exists and that this instance is the
// leader, then enqueues a reconcile signal and returns. The actual reconcile
// runs asynchronously in the reconcile-loop goroutine.
func (srv *server) ReconcileNodeV1(ctx context.Context, req *cluster_controllerpb.ReconcileNodeV1Request) (*cluster_controllerpb.ReconcileNodeV1Response, error) {
	nodeId := strings.TrimSpace(req.GetNodeId())
	if nodeId == "" {
		return nil, status.Error(codes.InvalidArgument, "node_id is required")
	}

	if !srv.isLeader() {
		return nil, status.Error(codes.FailedPrecondition, "not the cluster controller leader")
	}

	srv.lock("ReconcileNodeV1")
	_, exists := srv.state.Nodes[nodeId]
	srv.unlock()

	if !exists {
		return nil, status.Errorf(codes.NotFound, "node %q not found in cluster", nodeId)
	}

	if srv.enqueueReconcile != nil {
		srv.enqueueReconcile()
	}

	return &cluster_controllerpb.ReconcileNodeV1Response{}, nil
}
