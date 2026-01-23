package main

import (
	"context"
	"fmt"
	"net"
	"testing"

	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/grpc/test/bufconn"
)

type testControllerServer struct {
	clustercontrollerpb.UnimplementedClusterControllerServiceServer
	handler func(ctx context.Context, req *clustercontrollerpb.ReportNodeStatusRequest) (*clustercontrollerpb.ReportNodeStatusResponse, error)
}

func (s *testControllerServer) ReportNodeStatus(ctx context.Context, req *clustercontrollerpb.ReportNodeStatusRequest) (*clustercontrollerpb.ReportNodeStatusResponse, error) {
	if s.handler != nil {
		return s.handler(ctx, req)
	}
	return &clustercontrollerpb.ReportNodeStatusResponse{}, nil
}

func startBufconnController(t *testing.T, handler func(ctx context.Context, req *clustercontrollerpb.ReportNodeStatusRequest) (*clustercontrollerpb.ReportNodeStatusResponse, error)) *bufconn.Listener {
	t.Helper()
	lis := bufconn.Listen(1024 * 1024)
	s := grpc.NewServer()
	clustercontrollerpb.RegisterClusterControllerServiceServer(s, &testControllerServer{handler: handler})
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
	follower := startBufconnController(t, func(ctx context.Context, req *clustercontrollerpb.ReportNodeStatusRequest) (*clustercontrollerpb.ReportNodeStatusResponse, error) {
		followerCalls++
		return nil, status.Error(codes.FailedPrecondition, "not leader (leader_addr=leader:9999)")
	})
	leader := startBufconnController(t, func(ctx context.Context, req *clustercontrollerpb.ReportNodeStatusRequest) (*clustercontrollerpb.ReportNodeStatusResponse, error) {
		leaderCalls++
		return &clustercontrollerpb.ReportNodeStatusResponse{}, nil
	})

	srv := NewNodeAgentServer("", nil)
	srv.useInsecure = true
	srv.nodeID = "node-1"
	srv.controllerEndpoint = "follower"
	srv.controllerDialer = func(ctx context.Context, target string, opts ...grpc.DialOption) (*grpc.ClientConn, error) {
		var l *bufconn.Listener
		switch target {
		case "follower":
			l = follower
		case "leader:9999":
			l = leader
		default:
			return nil, fmt.Errorf("unknown target %s", target)
		}
		return grpc.DialContext(ctx, target, append(opts, grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
			return l.Dial()
		}))...)
	}

	statusReq := &clustercontrollerpb.NodeStatus{
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
