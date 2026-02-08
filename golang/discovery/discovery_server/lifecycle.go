package main

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	globular "github.com/globulario/services/golang/globular_service"
)

// ServiceLifecycle defines the lifecycle management interface for a Globular service.
// Phase 1 Step 3: Extracted to separate concerns - lifecycle vs business logic.
type ServiceLifecycle interface {
	Start() error
	Stop() error
	Ready() bool
	Health() error
}

// lifecycleManager wraps a server and provides clean lifecycle management.
type lifecycleManager struct {
	srv    *server
	logger *slog.Logger
}

// newLifecycleManager creates a new lifecycle manager for the given server.
func newLifecycleManager(srv *server, logger *slog.Logger) *lifecycleManager {
	return &lifecycleManager{
		srv:    srv,
		logger: logger,
	}
}

// Start initializes and starts the service.
// This wraps the existing StartService() method with additional lifecycle logic.
func (lm *lifecycleManager) Start() error {
	lm.logger.Info("starting service",
		"name", lm.srv.Name,
		"id", lm.srv.Id,
		"port", lm.srv.Port,
	)

	// Ensure gRPC server is initialized
	if lm.srv.grpcServer == nil {
		return fmt.Errorf("gRPC server not initialized (call Init first)")
	}

	// Start the service using Globular's lifecycle
	if err := lm.srv.StartService(); err != nil {
		lm.logger.Error("failed to start service",
			"name", lm.srv.Name,
			"id", lm.srv.Id,
			"err", err,
		)
		return fmt.Errorf("start service failed: %w", err)
	}

	// Mark as running
	lm.srv.State = "running"

	lm.logger.Info("service started successfully",
		"name", lm.srv.Name,
		"id", lm.srv.Id,
		"port", lm.srv.Port,
	)

	return nil
}

// Stop gracefully shuts down the service.
func (lm *lifecycleManager) Stop() error {
	lm.logger.Info("stopping service",
		"name", lm.srv.Name,
		"id", lm.srv.Id,
	)

	// Mark as stopping
	lm.srv.State = "stopping"

	// Stop the service using Globular's lifecycle
	if err := lm.srv.StopService(); err != nil {
		lm.logger.Error("failed to stop service",
			"name", lm.srv.Name,
			"id", lm.srv.Id,
			"err", err,
		)
		return fmt.Errorf("stop service failed: %w", err)
	}

	// Mark as stopped
	lm.srv.State = "stopped"

	lm.logger.Info("service stopped successfully",
		"name", lm.srv.Name,
		"id", lm.srv.Id,
	)

	return nil
}

// Ready checks if the service is ready to serve requests.
// Returns true if the service is in a healthy running state.
func (lm *lifecycleManager) Ready() bool {
	// Check basic readiness criteria
	if lm.srv.grpcServer == nil {
		return false
	}

	if lm.srv.State != "running" {
		return false
	}

	if lm.srv.Port == 0 {
		return false
	}

	return true
}

// Health performs a health check on the service.
// Returns nil if healthy, error describing the problem otherwise.
func (lm *lifecycleManager) Health() error {
	if lm.srv.grpcServer == nil {
		return fmt.Errorf("gRPC server not initialized")
	}

	if lm.srv.Port == 0 {
		return fmt.Errorf("service port not configured")
	}

	if lm.srv.Name == "" {
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
func (lm *lifecycleManager) GracefulShutdown(timeout time.Duration) error {
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
func (lm *lifecycleManager) AwaitReady(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if lm.Ready() {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}

	return fmt.Errorf("service did not become ready within %v", timeout)
}

// --- Helper functions for backward compatibility ---

// StartService is the original lifecycle method, now delegated.
// Kept for Globular interface compatibility.
func (srv *server) StartService() error {
	return globular.StartService(srv, srv.grpcServer)
}

// StopService is the original lifecycle method, now delegated.
// Kept for Globular interface compatibility.
func (srv *server) StopService() error {
	return globular.StopService(srv, srv.grpcServer)
}
