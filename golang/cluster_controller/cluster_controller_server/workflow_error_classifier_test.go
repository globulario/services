package main

import (
	"errors"
	"fmt"
	"testing"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// ── Phase 4 tests: classifyWorkflowError ─────────────────────────────────────

func TestClassifyWorkflowError_GRPCUnavailable(t *testing.T) {
	err := status.Error(codes.Unavailable, "connection refused")
	transient, reason := classifyWorkflowError(err)
	if !transient {
		t.Error("gRPC Unavailable must be classified as transient")
	}
	if reason != "workflow_unavailable" {
		t.Errorf("reason = %q, want workflow_unavailable", reason)
	}
}

func TestClassifyWorkflowError_GRPCDeadlineExceeded(t *testing.T) {
	err := status.Error(codes.DeadlineExceeded, "context deadline exceeded")
	transient, reason := classifyWorkflowError(err)
	if !transient {
		t.Error("gRPC DeadlineExceeded must be classified as transient")
	}
	if reason != "workflow_deadline" {
		t.Errorf("reason = %q, want workflow_deadline", reason)
	}
}

func TestClassifyWorkflowError_GRPCUnimplemented(t *testing.T) {
	err := status.Error(codes.Unimplemented, "method not implemented")
	transient, reason := classifyWorkflowError(err)
	if !transient {
		t.Error("gRPC Unimplemented must be classified as transient (service mid-upgrade)")
	}
	if reason != "workflow_handler_missing" {
		t.Errorf("reason = %q, want workflow_handler_missing", reason)
	}
}

func TestClassifyWorkflowError_CircuitBreaker(t *testing.T) {
	err := fmt.Errorf("workflow circuit breaker open: 5 failures in 5m0s, retry after 28s")
	transient, reason := classifyWorkflowError(err)
	if !transient {
		t.Error("circuit breaker error must be classified as transient")
	}
	if reason != "workflow_circuit_open" {
		t.Errorf("reason = %q, want workflow_circuit_open", reason)
	}
}

func TestClassifyWorkflowError_ConnectionRefused(t *testing.T) {
	err := fmt.Errorf("ExecuteWorkflow release.apply.package: connection refused")
	transient, reason := classifyWorkflowError(err)
	if !transient {
		t.Error("connection refused must be classified as transient")
	}
	if reason != "workflow_unavailable" {
		t.Errorf("reason = %q, want workflow_unavailable", reason)
	}
}

func TestClassifyWorkflowError_Preflight(t *testing.T) {
	err := fmt.Errorf("preflight release.apply.package: 16 action(s) have no registered handler")
	transient, reason := classifyWorkflowError(err)
	if !transient {
		t.Error("preflight error must be classified as transient")
	}
	if reason != "workflow_handler_missing" {
		t.Errorf("reason = %q, want workflow_handler_missing", reason)
	}
}

func TestClassifyWorkflowError_PostureGate(t *testing.T) {
	err := fmt.Errorf("posture gate: cluster in RECOVERY_ONLY — release.apply.package dispatch suppressed")
	transient, reason := classifyWorkflowError(err)
	if !transient {
		t.Error("posture gate error must be classified as transient")
	}
	if reason != "workflow_posture_gate" {
		t.Errorf("reason = %q, want workflow_posture_gate", reason)
	}
}

func TestClassifyWorkflowError_Permanent_ItemFailures(t *testing.T) {
	err := fmt.Errorf("step apply_per_node: 1/2 items failed")
	transient, reason := classifyWorkflowError(err)
	if transient {
		t.Errorf("item failures must be permanent (transient=%v reason=%q)", transient, reason)
	}
}

func TestClassifyWorkflowError_Permanent_ChecksumMismatch(t *testing.T) {
	err := fmt.Errorf("install_package: checksum mismatch")
	transient, reason := classifyWorkflowError(err)
	if transient {
		t.Errorf("checksum mismatch must be permanent (transient=%v reason=%q)", transient, reason)
	}
}

func TestClassifyWorkflowError_Nil(t *testing.T) {
	transient, reason := classifyWorkflowError(nil)
	if transient || reason != "" {
		t.Errorf("nil error: transient=%v reason=%q, want false/empty", transient, reason)
	}
}

func TestClassifyWorkflowError_WrappedGRPC(t *testing.T) {
	inner := status.Error(codes.Unavailable, "scylla down")
	wrapped := fmt.Errorf("workflow service call: %w", inner)
	transient, reason := classifyWorkflowError(wrapped)
	// Wrapped gRPC errors: status.FromError unwraps, so this should be transient.
	// If not, the string "Unavailable" fallback catches it.
	if !transient {
		t.Errorf("wrapped Unavailable error must be transient (reason=%q)", reason)
	}
}

func TestClassifyWorkflowError_PlainErrors(t *testing.T) {
	// Permanent errors that must not be misclassified.
	permanent := []string{
		"version mismatch: want 1.2.3 got 1.2.2",
		"node n1 rejected: disk full",
		"apply failed: permission denied",
	}
	for _, msg := range permanent {
		t.Run(msg, func(t *testing.T) {
			transient, reason := classifyWorkflowError(errors.New(msg))
			if transient {
				t.Errorf("error %q classified as transient (reason=%q) — should be permanent", msg, reason)
			}
		})
	}
}
