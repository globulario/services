package main

import (
	"strings"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// classifyWorkflowError inspects a workflow dispatch error and returns
// (isTransient bool, reason string).
//
//   - isTransient == true  → keep release RESOLVED, apply retry patch, wait NextRetryUnixMs.
//   - isTransient == false → transition to FAILED (real execution error, needs operator attention).
//
// reason is a structured slug used as BlockedReason in the retry patch:
//
//	workflow_unavailable       — gRPC Unavailable or connection refused
//	workflow_circuit_open      — controller-side circuit breaker
//	workflow_deadline          — DeadlineExceeded
//	workflow_handler_missing   — preflight / no registered handler (bootstrap transient)
//	workflow_posture_gate      — posture gate: RECOVERY_ONLY
func classifyWorkflowError(err error) (transient bool, reason string) {
	if err == nil {
		return false, ""
	}

	// Check gRPC status codes first — most reliable signal.
	if st, ok := status.FromError(err); ok {
		switch st.Code() {
		case codes.Unavailable:
			return true, "workflow_unavailable"
		case codes.DeadlineExceeded:
			return true, "workflow_deadline"
		case codes.Unimplemented:
			// Workflow service has not implemented the requested action yet.
			// Treat as transient (service may be mid-upgrade).
			return true, "workflow_handler_missing"
		}
	}

	msg := err.Error()
	switch {
	case strings.Contains(msg, "circuit breaker"):
		return true, "workflow_circuit_open"
	case strings.Contains(msg, "connection refused"):
		return true, "workflow_unavailable"
	case strings.Contains(msg, "Unavailable"):
		return true, "workflow_unavailable"
	case strings.Contains(msg, "DeadlineExceeded"):
		return true, "workflow_deadline"
	case strings.Contains(msg, "preflight"),
		strings.Contains(msg, "no registered handler"),
		strings.Contains(msg, "handler not found"),
		strings.Contains(msg, "Unimplemented"):
		return true, "workflow_handler_missing"
	case strings.Contains(msg, "posture gate"):
		return true, "workflow_posture_gate"
	}

	return false, ""
}
