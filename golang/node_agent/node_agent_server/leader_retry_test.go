package main

import (
	"context"
	"net"
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

type testControllerServer struct {
	cluster_controllerpb.UnimplementedClusterControllerServiceServer
	handler func(ctx context.Context, req *cluster_controllerpb.ReportNodeStatusRequest) (*cluster_controllerpb.ReportNodeStatusResponse, error)
}

func (s *testControllerServer) ReportNodeStatus(ctx context.Context, req *cluster_controllerpb.ReportNodeStatusRequest) (*cluster_controllerpb.ReportNodeStatusResponse, error) {
	if s.handler != nil {
		return s.handler(ctx, req)
	}
	return &cluster_controllerpb.ReportNodeStatusResponse{}, nil
}

func startBufconnController(t *testing.T, handler func(ctx context.Context, req *cluster_controllerpb.ReportNodeStatusRequest) (*cluster_controllerpb.ReportNodeStatusResponse, error)) *bufconn.Listener {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	s := grpc.NewServer()
	cluster_controllerpb.RegisterClusterControllerServiceServer(s, &testControllerServer{handler: handler})
	go func() {
		if err := s.Serve(lis); err != nil {
			t.Logf("bufconn serve exited: %v", err)
		}
	}()
	t.Cleanup(s.Stop)
	return lis
}

func TestReportStatusRetriesToLeader(t *testing.T) {
	followerCalls := 0
	leaderCalls := 0
	follower := startBufconnController(t, func(ctx context.Context, req *cluster_controllerpb.ReportNodeStatusRequest) (*cluster_controllerpb.ReportNodeStatusResponse, error) {
		followerCalls++
		return nil, status.Error(codes.FailedPrecondition, "not leader (leader_addr=leader:9999)")
	})
	leader := startBufconnController(t, func(ctx context.Context, req *cluster_controllerpb.ReportNodeStatusRequest) (*cluster_controllerpb.ReportNodeStatusResponse, error) {
		leaderCalls++
		return &cluster_controllerpb.ReportNodeStatusResponse{}, nil
	})

	// Build plain gRPC connections via bufconn (no TLS needed in tests).
	makeConn := func(lis *bufconn.Listener) *grpc.ClientConn {
		conn, err := grpc.DialContext(context.Background(), "bufnet",
			grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
				return lis.Dial()
			}),
			grpc.WithInsecure(), //nolint:staticcheck // bufconn test only
		)
		if err != nil {
			t.Fatalf("bufconn dial: %v", err)
		}
		t.Cleanup(func() { conn.Close() })
		return conn
	}
	followerClient := cluster_controllerpb.NewClusterControllerServiceClient(makeConn(follower))
	leaderClient := cluster_controllerpb.NewClusterControllerServiceClient(makeConn(leader))

	srv := NewNodeAgentServer("", nil, NodeAgentConfig{})
	srv.useInsecure = true
	srv.nodeID = "node-1"
	srv.controllerEndpoint = "follower"
	// Use override so we can inject pre-built clients without a TLS dial.
	srv.controllerClientOverride = func(addr string) cluster_controllerpb.ClusterControllerServiceClient {
		switch addr {
		case "follower":
			return followerClient
		case "leader:9999":
			return leaderClient
		default:
			t.Fatalf("unexpected target %s", addr)
			return nil
		}
	}
	srv.controllerClient = followerClient

	statusReq := &cluster_controllerpb.NodeStatus{
		NodeId: "node-1",
	}
	if err := srv.sendStatusWithRetry(context.Background(), statusReq); err != nil {
		t.Fatalf("sendStatusWithRetry error: %v", err)
	}
	if followerCalls != 1 {
		t.Fatalf("expected 1 follower call, got %d", followerCalls)
	}
	if leaderCalls != 1 {
		t.Fatalf("expected 1 leader call, got %d", leaderCalls)
	}
}
