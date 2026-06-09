package interceptors

import (
	"testing"

	"github.com/globulario/services/golang/security"
)

// TestRBACFallback_FailsClosedForUserMutationOutsideBootstrap is the ratchet for
// meta.fail_safe_defaults_when_authority_is_uncertain. On the RBAC-uncertain
// (fallback) path, a real user invoking a MUTATING method while NOT in the
// bootstrap window must be DENIED — even when a local role would grant it. That
// "any local role grants → ALLOW" path was the silent admin-escalation window
// the principle named (a viewer clearing an admin check during an RBAC outage).
// The read-only relaxation and the "sa" service-identity exemption are kept,
// because neither can widen into a state-changing escalation.
func TestRBACFallback_FailsClosedForUserMutationOutsideBootstrap(t *testing.T) {
	// Pin the bootstrap gate inactive deterministically (no flag file), so the
	// test does not depend on the host's /var/lib/globular/bootstrap.enabled.
	savedGate := security.DefaultBootstrapGate
	security.DefaultBootstrapGate = security.NewBootstrapGateWithPath("/nonexistent/bootstrap.enabled")
	t.Cleanup(func() { security.DefaultBootstrapGate = savedGate })

	// A local role that GRANTS the test action, so a permissive fallback WOULD
	// allow it. The fail-safe must deny the mutating case anyway.
	savedRoles := security.RolePermissions
	security.RolePermissions = map[string][]string{"local-admin": {"test.action"}}
	t.Cleanup(func() { security.RolePermissions = savedRoles })

	const actionKey = "test.action"
	const mutating = "/test.Service/DeleteThing" // IsMutatingRPC → true
	const readOnly = "/test.Service/GetThing"    // IsMutatingRPC → false

	// Fixtures must classify as intended (IsMutatingRPC fails closed on unknowns).
	if !security.IsMutatingRPC(mutating) {
		t.Fatalf("fixture %q should classify as mutating", mutating)
	}
	if security.IsMutatingRPC(readOnly) {
		t.Fatalf("fixture %q should classify as read-only", readOnly)
	}
	if security.DefaultBootstrapGate.IsActive() {
		t.Fatal("bootstrap gate should be inactive with a nonexistent flag path")
	}

	// 1. User + mutating + not bootstrap → DENY, despite local-admin granting it.
	if rbacUncertainAllow("alice", actionKey, mutating) {
		t.Error("FAIL-SAFE VIOLATED: a user's mutating method was allowed during an RBAC outage " +
			"(silent privilege escalation) — it must fail closed")
	}
	// 2. User + read-only → permissive (local grant honored): cannot escalate to a mutation.
	if !rbacUncertainAllow("alice", actionKey, readOnly) {
		t.Error("read-only method should retain the permissive local-roles fallback")
	}
	// 3. "sa" service identity + mutating → permissive (service plumbing, not a user principal;
	//    must not deadlock on an RBAC blip).
	if !rbacUncertainAllow("sa", actionKey, mutating) {
		t.Error("sa service identity should retain the permissive fallback")
	}
}
