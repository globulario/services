package globular_service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"google.golang.org/grpc"
)

// LifecycleService defines the interface required for lifecycle management.
// Services must implement these methods to use the shared lifecycle manager.
//
// Phase 2 Step 2: Extracted from Echo, Discovery, and Repository services.
type LifecycleService interface {
	// Identity
	GetId() string
	GetName() string
	GetPort() int

	// State management
	GetState() string
	SetState(string)

	// Lifecycle operations
	StartService() error
	StopService() error

	// gRPC server access (for readiness checks)
	GetGrpcServer() *grpc.Server
}

// LifecycleManager wraps a service and provides clean lifecycle management.
//
// Phase 2 Step 2: Generic lifecycle manager extracted from service-specific implementations.
type LifecycleManager struct {
	srv    LifecycleService
	logger *slog.Logger
}

// NewLifecycleManager creates a new lifecycle manager for the given service.
func NewLifecycleManager(srv LifecycleService, logger *slog.Logger) *LifecycleManager {
	return &LifecycleManager{
		srv:    srv,
		logger: logger,
	}
}

// Start initializes and starts the service.
// This wraps the existing StartService() method with additional lifecycle logic.
func (lm *LifecycleManager) Start() error {
	lm.logger.Info("starting service",
		"name", lm.srv.GetName(),
		"id", lm.srv.GetId(),
		"port", lm.srv.GetPort(),
	)

	// Ensure gRPC server is initialized
	if lm.srv.GetGrpcServer() == nil {
		return fmt.Errorf("gRPC server not initialized (call Init first)")
	}

	// Start the service using Globular's lifecycle
	if err := lm.srv.StartService(); err != nil {
		lm.logger.Error("failed to start service",
			"name", lm.srv.GetName(),
			"id", lm.srv.GetId(),
			"err", err,
		)
		return fmt.Errorf("start service failed: %w", err)
	}

	// Mark as running
	lm.srv.SetState("running")

	lm.logger.Info("service started successfully",
		"name", lm.srv.GetName(),
		"id", lm.srv.GetId(),
		"port", lm.srv.GetPort(),
	)

	return nil
}

// Stop gracefully shuts down the service.
func (lm *LifecycleManager) Stop() error {
	lm.logger.Info("stopping service",
		"name", lm.srv.GetName(),
		"id", lm.srv.GetId(),
	)

	// Mark as stopping
	lm.srv.SetState("stopping")

	// Stop the service using Globular's lifecycle
	if err := lm.srv.StopService(); err != nil {
		lm.logger.Error("failed to stop service",
			"name", lm.srv.GetName(),
			"id", lm.srv.GetId(),
			"err", err,
		)
		return fmt.Errorf("stop service failed: %w", err)
	}

	// Mark as stopped
	lm.srv.SetState("stopped")

	lm.logger.Info("service stopped successfully",
		"name", lm.srv.GetName(),
		"id", lm.srv.GetId(),
	)

	return nil
}

// Ready checks if the service is ready to serve requests.
// Returns true if the service is in a healthy running state.
func (lm *LifecycleManager) Ready() bool {
	// Check basic readiness criteria
	if lm.srv.GetGrpcServer() == nil {
		return false
	}

	if lm.srv.GetState() != "running" {
		return false
	}

	if lm.srv.GetPort() == 0 {
		return false
	}

	return true
}

// Health performs a health check on the service.
// Returns nil if healthy, error describing the problem otherwise.
func (lm *LifecycleManager) Health() error {
	if lm.srv.GetGrpcServer() == nil {
		return fmt.Errorf("gRPC server not initialized")
	}

	if lm.srv.GetPort() == 0 {
		return fmt.Errorf("service port not configured")
	}

	if lm.srv.GetName() == "" {
		return fmt.Errorf("service name not configured")
	}

	// Check if we can create a basic context (smoke test)
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	if ctx.Err() != nil {
		return fmt.Errorf("context creation failed: %w", ctx.Err())
	}

	return nil
}

// GracefulShutdown performs a graceful shutdown with timeout.
// This is a helper for main() to ensure clean shutdown.
func (lm *LifecycleManager) GracefulShutdown(timeout time.Duration) error {
	lm.logger.Info("initiating graceful shutdown",
		"timeout", timeout,
	)

	// Create a channel to signal completion
	done := make(chan error, 1)

	// Stop in a goroutine
	go func() {
		done <- lm.Stop()
	}()

	// Wait for completion or timeout
	select {
	case err := <-done:
		return err
	case <-time.After(timeout):
		return fmt.Errorf("graceful shutdown timed out after %v", timeout)
	}
}

// AwaitReady blocks until the service is ready or timeout is reached.
// Useful for startup synchronization.
func (lm *LifecycleManager) AwaitReady(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if lm.Ready() {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}

	return fmt.Errorf("service did not become ready within %v", timeout)
}
