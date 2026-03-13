package actions

import (
	"context"
	"net"
	"testing"

	"google.golang.org/grpc"
	healthpb "google.golang.org/grpc/health/grpc_health_v1"
	"google.golang.org/protobuf/types/known/structpb"
)

// mockHealthServer implements the gRPC Health service for testing.
type mockHealthServer struct {
	healthpb.UnimplementedHealthServer
	status healthpb.HealthCheckResponse_ServingStatus
}

func (s *mockHealthServer) Check(_ context.Context, _ *healthpb.HealthCheckRequest) (*healthpb.HealthCheckResponse, error) {
	return &healthpb.HealthCheckResponse{Status: s.status}, nil
}

func startMockHealthServer(t *testing.T, status healthpb.HealthCheckResponse_ServingStatus) (addr string, cleanup func()) {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	srv := grpc.NewServer()
	healthpb.RegisterHealthServer(srv, &mockHealthServer{status: status})
	go srv.Serve(lis)
	return lis.Addr().String(), func() { srv.Stop() }
}

func TestGRPCHealthProbe_Serving(t *testing.T) {
	addr, cleanup := startMockHealthServer(t, healthpb.HealthCheckResponse_SERVING)
	defer cleanup()

	probe := grpcHealthProbeAction{}
	args, _ := structpb.NewStruct(map[string]interface{}{
		"address":    addr,
		"timeout_ms": float64(3000),
	})

	result, err := probe.Apply(context.Background(), args)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if result == "" {
		t.Error("expected non-empty result")
	}
}

func TestGRPCHealthProbe_NotServing(t *testing.T) {
	addr, cleanup := startMockHealthServer(t, healthpb.HealthCheckResponse_NOT_SERVING)
	defer cleanup()

	probe := grpcHealthProbeAction{}
	args, _ := structpb.NewStruct(map[string]interface{}{
		"address":    addr,
		"timeout_ms": float64(3000),
	})

	_, err := probe.Apply(context.Background(), args)
	if err == nil {
		t.Fatal("expected error for NOT_SERVING status")
	}
}

func TestGRPCHealthProbe_Unreachable(t *testing.T) {
	probe := grpcHealthProbeAction{}
	args, _ := structpb.NewStruct(map[string]interface{}{
		"address":    "127.0.0.1:1", // unlikely to be listening
		"timeout_ms": float64(500),
	})

	_, err := probe.Apply(context.Background(), args)
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}

func TestGRPCHealthProbe_Validate(t *testing.T) {
	probe := grpcHealthProbeAction{}

	if err := probe.Validate(nil); err == nil {
		t.Error("expected error for nil args")
	}

	empty, _ := structpb.NewStruct(map[string]interface{}{})
	if err := probe.Validate(empty); err == nil {
		t.Error("expected error for missing address")
	}

	valid, _ := structpb.NewStruct(map[string]interface{}{"address": "localhost:50051"})
	if err := probe.Validate(valid); err != nil {
		t.Errorf("expected no error, got: %v", err)
	}
}

func TestGRPCHealthProbe_Registered(t *testing.T) {
	h := Get("probe.grpc_health")
	if h == nil {
		t.Fatal("probe.grpc_health not registered in action registry")
	}
	if h.Name() != "probe.grpc_health" {
		t.Errorf("expected name probe.grpc_health, got %s", h.Name())
	}
}
