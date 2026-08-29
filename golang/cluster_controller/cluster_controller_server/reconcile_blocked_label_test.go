package main

import (
	"errors"
	"testing"
)

// A dependency-blocked refusal must classify as transient, so the reconcile log
// can call it BLOCKED rather than FAILED. The workflow never ran; reporting it
// as a failure states an outcome that did not occur, and any reader grepping for
// failures — operator or probe — counts a correct refusal as a fault.
//
// This mirrors the distinction etcd.must_have_free_backend_space_for_reconciliation
// already requires between persistence-blocked and verification failure.
func TestDependencyBlockedClassifiesAsTransient(t *testing.T) {
	err := errors.New("WORKFLOW_DEPENDENCY_BLOCKED: dependency=scylla reason=workflow circuit breaker open: 287 failures in 5m0s, retry after 20s")
	transient, reason := classifyWorkflowError(err)
	if !transient {
		t.Fatal("dependency-blocked must classify as transient so it is logged BLOCKED, not FAILED")
	}
	if reason == "" {
		t.Error("expected a classification reason for a dependency-blocked refusal")
	}
}

// A genuine failure must NOT be softened into BLOCKED. The distinction only has
// value if real faults still read as failures.
func TestGenuineFailureStaysNonTransient(t *testing.T) {
	for _, msg := range []string{
		"step apply_per_node: verification failed: binary hash mismatch",
		"invalid desired state: kind and name disagree",
	} {
		if transient, _ := classifyWorkflowError(errors.New(msg)); transient {
			t.Errorf("non-transient error was classified transient and would be logged BLOCKED: %q", msg)
		}
	}
}

// A nil error is not a failure of any kind.
func TestNilErrorIsNotTransient(t *testing.T) {
	if transient, _ := classifyWorkflowError(nil); transient {
		t.Error("nil error classified as transient")
	}
}

// Service discovery finding no healthy workflow instance is transient: during
// bootstrap the workflow service has simply not registered yet. Before this was
// classified, ordinary startup logged "cluster.reconcile FAILED" and a reader
// grepping for failures counted a startup race as a fault.
func TestWorkflowUnreachableClassifiesAsTransient(t *testing.T) {
	err := errors.New("no workflow service instance is reachable (none registered in etcd, or every candidate is unhealthy)")
	transient, reason := classifyWorkflowError(err)
	if !transient {
		t.Fatal("workflow-unreachable must classify as transient — it resolves on its own once the service registers")
	}
	if reason != "workflow_unavailable" {
		t.Errorf("reason = %q, want workflow_unavailable — same condition as other unreachable spellings", reason)
	}
}
