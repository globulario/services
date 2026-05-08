package main

// globular:tested_by desired.bootstrap_state_requires_promotion

import (
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

// TestBootstrapRecordIgnoredAsFinalByConsumers verifies that bootstrap-labeled
// InfrastructureRelease records are not treated as authoritative desired state
// by convergence consumers. The bootstrap-source label signals that the record
// was inferred from observed installed state, not set by an operator.
//
// Invariant: desired.bootstrap_state_requires_promotion
func TestBootstrapRecordIgnoredAsFinalByConsumers(t *testing.T) {
	// Bootstrap record — inferred from observed installed state (Day-0 materialization).
	bootstrapMeta := &cluster_controllerpb.ObjectMeta{Name: "globular/etcd"}
	stampBootstrapLabels(bootstrapMeta)

	if !isBootstrapRecord(bootstrapMeta) {
		t.Error("isBootstrapRecord: expected true for bootstrap-labeled meta")
	}
	if isAuthoritative(bootstrapMeta) {
		t.Error("isAuthoritative: expected false — bootstrap record must not be authoritative")
	}
	// Convergence consumers must not treat this as final desired state.
	if bootstrapConvergenceAllowed(bootstrapMeta) {
		t.Error("bootstrapConvergenceAllowed: expected false — consumers must block convergence on bootstrap records")
	}

	// Operator-set record — no labels, never passed through bootstrap path.
	operatorMeta := &cluster_controllerpb.ObjectMeta{Name: "globular/etcd"}
	if isBootstrapRecord(operatorMeta) {
		t.Error("isBootstrapRecord: expected false for unlabeled meta (operator-set)")
	}
	if !isAuthoritative(operatorMeta) {
		t.Error("isAuthoritative: expected true — unlabeled meta is always authoritative")
	}
	if !bootstrapConvergenceAllowed(operatorMeta) {
		t.Error("bootstrapConvergenceAllowed: expected true for operator-set record")
	}

	// Nil meta — treated as authoritative (no labels means operator-set).
	if isBootstrapRecord(nil) {
		t.Error("isBootstrapRecord: expected false for nil meta")
	}
	if !bootstrapConvergenceAllowed(nil) {
		t.Error("bootstrapConvergenceAllowed: expected true for nil meta")
	}
}

// TestFirstBootClaimsConvergenceOnlyAfterPromotion verifies that a Day-0
// bootstrap record blocks convergence claims until the promotion reconciler
// calls promoteToAuthoritative. This prevents observer-inferred desired state
// from being treated as permanent cluster intent before operator confirmation.
//
// Invariant: desired.bootstrap_state_requires_promotion
func TestFirstBootClaimsConvergenceOnlyAfterPromotion(t *testing.T) {
	meta := &cluster_controllerpb.ObjectMeta{Name: "globular/scylladb"}
	stampBootstrapLabels(meta)

	// Before promotion: convergence must not be claimed.
	if bootstrapConvergenceAllowed(meta) {
		t.Error("before promotion: bootstrapConvergenceAllowed must return false")
	}
	if !isBootstrapRecord(meta) {
		t.Error("before promotion: isBootstrapRecord must return true")
	}

	// Simulate promotion reconciler confirming Phase == AVAILABLE.
	promoteToAuthoritative(meta)

	// After promotion: convergence is now allowed.
	if !bootstrapConvergenceAllowed(meta) {
		t.Error("after promotion: bootstrapConvergenceAllowed must return true")
	}
	if isBootstrapRecord(meta) {
		t.Error("after promotion: isBootstrapRecord must return false (bootstrap label cleared)")
	}
	if !isAuthoritative(meta) {
		t.Error("after promotion: isAuthoritative must return true")
	}
}
