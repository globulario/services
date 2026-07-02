package main

import (
	"context"
	"net"
	"path/filepath"
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
)

type joinLifecycleController struct {
	cluster_controllerpb.UnimplementedClusterControllerServiceServer
	reportNodeStatus func(context.Context, *cluster_controllerpb.ReportNodeStatusRequest) (*cluster_controllerpb.ReportNodeStatusResponse, error)
	requestJoin      func(context.Context, *cluster_controllerpb.RequestJoinRequest) (*cluster_controllerpb.RequestJoinResponse, error)
	getJoinStatus    func(context.Context, *cluster_controllerpb.GetJoinRequestStatusRequest) (*cluster_controllerpb.GetJoinRequestStatusResponse, error)
}

func (s *joinLifecycleController) ReportNodeStatus(ctx context.Context, req *cluster_controllerpb.ReportNodeStatusRequest) (*cluster_controllerpb.ReportNodeStatusResponse, error) {
	if s.reportNodeStatus != nil {
		return s.reportNodeStatus(ctx, req)
	}
	return &cluster_controllerpb.ReportNodeStatusResponse{}, nil
}

func (s *joinLifecycleController) RequestJoin(ctx context.Context, req *cluster_controllerpb.RequestJoinRequest) (*cluster_controllerpb.RequestJoinResponse, error) {
	if s.requestJoin != nil {
		return s.requestJoin(ctx, req)
	}
	return &cluster_controllerpb.RequestJoinResponse{}, nil
}

func (s *joinLifecycleController) GetJoinRequestStatus(ctx context.Context, req *cluster_controllerpb.GetJoinRequestStatusRequest) (*cluster_controllerpb.GetJoinRequestStatusResponse, error) {
	if s.getJoinStatus != nil {
		return s.getJoinStatus(ctx, req)
	}
	return &cluster_controllerpb.GetJoinRequestStatusResponse{Status: "pending"}, nil
}

func startJoinLifecycleController(t *testing.T, server *joinLifecycleController) *grpc.ClientConn {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	grpcServer := grpc.NewServer()
	cluster_controllerpb.RegisterClusterControllerServiceServer(grpcServer, server)
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			t.Logf("bufconn serve exited: %v", err)
		}
	}()
	t.Cleanup(grpcServer.Stop)

	conn, err := grpc.DialContext(context.Background(), "bufnet",
		grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return lis.Dial()
		}),
		grpc.WithInsecure(), //nolint:staticcheck // bufconn test only
	)
	if err != nil {
		t.Fatalf("bufconn dial: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close() })
	return conn
}

func TestSendStatusWithRetry_SatisfiesLegacyJoinAfterAutoRegistration(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "node-agent", "state.json")
	srv := NewNodeAgentServer(statePath, newNodeAgentState(), NodeAgentConfig{
		NodeID:        "node-1",
		JoinToken:     "join-token",
		ClusterMode:   true,
		AdvertiseAddr: "10.0.0.10:11000",
	})
	srv.controllerEndpoint = "bufconn"
	srv.controllerClient = cluster_controllerpb.NewClusterControllerServiceClient(startJoinLifecycleController(t, &joinLifecycleController{}))

	if err := srv.sendStatusWithRetry(context.Background(), &cluster_controllerpb.NodeStatus{NodeId: "node-1"}); err != nil {
		t.Fatalf("sendStatusWithRetry: %v", err)
	}
	if srv.joinToken != "" {
		t.Fatalf("joinToken not cleared after successful auto-registration: %q", srv.joinToken)
	}
	if srv.state.JoinToken != "" {
		t.Fatalf("state.JoinToken not cleared after successful auto-registration: %q", srv.state.JoinToken)
	}
	persisted, err := loadNodeAgentState(statePath)
	if err != nil {
		t.Fatalf("loadNodeAgentState: %v", err)
	}
	if persisted.JoinToken != "" {
		t.Fatalf("persisted join token not cleared: %q", persisted.JoinToken)
	}
	if persisted.NodeID != "node-1" {
		t.Fatalf("persisted node ID = %q, want node-1", persisted.NodeID)
	}
}

func TestSendStatusWithRetry_DoesNotClearExplicitJoinLifecycle(t *testing.T) {
	statePath := filepath.Join(t.TempDir(), "node-agent", "state.json")
	state := &nodeAgentState{
		JoinID:        "join-123",
		JoinPlanJSON:  []byte(`{"join_id":"join-123"}`),
		JoinToken:     "join-token",
		NodeID:        "node-1",
		RequestID:     "",
		ClusterDomain: "globular.internal",
		AdvertiseIP:   "10.0.0.10",
		AdvertiseFQDN: "node-1.globular.internal",
		Protocol:      "https",
	}
	srv := NewNodeAgentServer(statePath, state, NodeAgentConfig{
		JoinToken:     "join-token",
		ClusterMode:   true,
		AdvertiseAddr: "10.0.0.10:11000",
	})
	srv.controllerEndpoint = "bufconn"
	srv.controllerClient = cluster_controllerpb.NewClusterControllerServiceClient(startJoinLifecycleController(t, &joinLifecycleController{}))

	if err := srv.sendStatusWithRetry(context.Background(), &cluster_controllerpb.NodeStatus{NodeId: "node-1"}); err != nil {
		t.Fatalf("sendStatusWithRetry: %v", err)
	}
	if srv.joinToken != "join-token" {
		t.Fatalf("legacy join token was cleared for explicit join lifecycle: %q", srv.joinToken)
	}
	if srv.state.JoinID != "join-123" {
		t.Fatalf("JoinID changed unexpectedly: %q", srv.state.JoinID)
	}
}

func TestAutoInitiateJoin_V1StillRequestsWhenLegacyJoinUnsatisfied(t *testing.T) {
	requestJoinCalls := 0
	conn := startJoinLifecycleController(t, &joinLifecycleController{
		requestJoin: func(ctx context.Context, req *cluster_controllerpb.RequestJoinRequest) (*cluster_controllerpb.RequestJoinResponse, error) {
			requestJoinCalls++
			return &cluster_controllerpb.RequestJoinResponse{
				RequestId: "req-123",
				Status:    "pending",
			}, nil
		},
		getJoinStatus: func(ctx context.Context, req *cluster_controllerpb.GetJoinRequestStatusRequest) (*cluster_controllerpb.GetJoinRequestStatusResponse, error) {
			return &cluster_controllerpb.GetJoinRequestStatusResponse{Status: "pending"}, nil
		},
	})

	statePath := filepath.Join(t.TempDir(), "node-agent", "state.json")
	srv := NewNodeAgentServer(statePath, newNodeAgentState(), NodeAgentConfig{
		NodeID:        "node-1",
		JoinToken:     "join-token",
		ClusterMode:   true,
		AdvertiseAddr: "10.0.0.10:11000",
	})
	srv.controllerEndpoint = "bufconn"
	srv.controllerClient = cluster_controllerpb.NewClusterControllerServiceClient(conn)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	srv.autoInitiateJoin(ctx)
	cancel()

	if requestJoinCalls != 1 {
		t.Fatalf("RequestJoin calls = %d, want 1", requestJoinCalls)
	}
	if srv.joinRequestID != "req-123" {
		t.Fatalf("joinRequestID = %q, want req-123", srv.joinRequestID)
	}
}
