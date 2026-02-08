package main

import (
	"log/slog"
	"os"
	"testing"
	"time"

	"google.golang.org/grpc"
)

// TestLifecycleManagerCreation verifies lifecycle manager can be created
func TestLifecycleManagerCreation(t *testing.T) {
	srv := &server{
		Name:  "discovery.PackageDiscovery",
		Id:    "test-id",
		Port:  10029,
		State: "initialized",
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	lm := newLifecycleManager(srv, logger)

	if lm == nil {
		t.Fatal("newLifecycleManager returned nil")
	}

	if lm.srv != srv {
		t.Error("lifecycle manager does not reference correct server")
	}

	if lm.logger != logger {
		t.Error("lifecycle manager does not reference correct logger")
	}
}

// TestReadyCheck verifies the Ready() method logic
func TestReadyCheck(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	tests := []struct {
		name  string
		srv   *server
		ready bool
	}{
		{
			name: "ready - all criteria met",
			srv: &server{
				Name:       "discovery.PackageDiscovery",
				Port:       10029,
				State:      "running",
				grpcServer: grpc.NewServer(), // Type doesn't matter for Ready() check
			},
			ready: true,
		},
		{
			name: "not ready - no grpc server",
			srv: &server{
				Name:       "discovery.PackageDiscovery",
				Port:       10029,
				State:      "running",
				grpcServer: nil,
			},
			ready: false,
		},
		{
			name: "not ready - wrong state",
			srv: &server{
				Name:       "discovery.PackageDiscovery",
				Port:       10029,
				State:      "starting",
				grpcServer: grpc.NewServer(),
			},
			ready: false,
		},
		{
			name: "not ready - no port",
			srv: &server{
				Name:       "discovery.PackageDiscovery",
				Port:       0,
				State:      "running",
				grpcServer: grpc.NewServer(),
			},
			ready: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lm := newLifecycleManager(tt.srv, logger)
			ready := lm.Ready()

			if ready != tt.ready {
				t.Errorf("Ready() = %v, want %v", ready, tt.ready)
			}
		})
	}
}

// TestHealthCheck verifies the Health() method logic
func TestHealthCheck(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))

	tests := []struct {
		name      string
		srv       *server
		wantError bool
	}{
		{
			name: "healthy",
			srv: &server{
				Name:       "discovery.PackageDiscovery",
				Port:       10029,
				grpcServer: grpc.NewServer(),
			},
			wantError: false,
		},
		{
			name: "unhealthy - no grpc server",
			srv: &server{
				Name:       "discovery.PackageDiscovery",
				Port:       10029,
				grpcServer: nil,
			},
			wantError: true,
		},
		{
			name: "unhealthy - no port",
			srv: &server{
				Name:       "discovery.PackageDiscovery",
				Port:       0,
				grpcServer: grpc.NewServer(),
			},
			wantError: true,
		},
		{
			name: "unhealthy - no name",
			srv: &server{
				Name:       "",
				Port:       10029,
				grpcServer: grpc.NewServer(),
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			lm := newLifecycleManager(tt.srv, logger)
			err := lm.Health()

			if (err != nil) != tt.wantError {
				t.Errorf("Health() error = %v, wantError %v", err, tt.wantError)
			}
		})
	}
}

// TestAwaitReadyTimeout verifies timeout behavior
func TestAwaitReadyTimeout(t *testing.T) {
	srv := &server{
		Name:  "discovery.PackageDiscovery",
		Port:  10029,
		State: "starting", // Not ready
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	lm := newLifecycleManager(srv, logger)

	// Should timeout since service never becomes ready
	err := lm.AwaitReady(100 * time.Millisecond)
	if err == nil {
		t.Error("AwaitReady should have timed out")
	}
}

// TestAwaitReadySuccess verifies success case
func TestAwaitReadySuccess(t *testing.T) {
	srv := &server{
		Name:       "discovery.PackageDiscovery",
		Port:       10029,
		State:      "running",
		grpcServer: grpc.NewServer(),
	}

	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	lm := newLifecycleManager(srv, logger)

	// Should succeed immediately since service is ready
	err := lm.AwaitReady(1 * time.Second)
	if err != nil {
		t.Errorf("AwaitReady failed: %v", err)
	}
}

// TestGracefulShutdownTimeout verifies timeout behavior
func TestGracefulShutdownTimeout(t *testing.T) {
	// This test verifies the timeout mechanism works
	// We can't actually test a real shutdown without a running server
	t.Skip("Skipping - requires running server to test shutdown timeout")
}

// TestLifecycleInvariant documents lifecycle behavior
func TestLifecycleInvariant(t *testing.T) {
	t.Log("Lifecycle Component Contract:")
	t.Log("1. Start() initializes and starts the service")
	t.Log("2. Stop() gracefully shuts down the service")
	t.Log("3. Ready() returns true only when fully operational")
	t.Log("4. Health() returns nil only when healthy")
	t.Log("5. GracefulShutdown() enforces timeout")
	t.Log("6. AwaitReady() blocks until ready or timeout")
	t.Log("")
	t.Log("Phase 1 Step 3: Lifecycle extracted for clean separation of concerns")
}

// Note: Using grpc.NewServer() for tests instead of mocks
// The actual gRPC server instance is not started in unit tests
