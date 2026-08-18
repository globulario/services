package security

import "testing"

// The controller dispatches every cluster mutation through the workflow service
// (intent:workflow.source_of_operational_truth). ExecuteWorkflow is gated on the
// action "workflow.dispatch".
//
// The founding/controller node authenticates to the workflow service with its
// own mTLS cert (CN == node name) and self-binds that identity at startup
// (ensureLocalNodeExecutorBinding). Until 2026-08-10 it bound ONLY
// globular-node-executor, which grants "workflow.admin" — a sibling action key,
// not a wildcard covering "workflow.dispatch". The result on a clean 5-node
// bring-up was 2112 `PermissionDenied: workflow.dispatch` errors: no
// release.apply.package ran for any infrastructure package, and node.bootstrap
// failed for every joining node.
//
// It was nearly invisible because the bootstrap phase machine and the
// node-agent's local repair path both compensate — every node still reached
// workload_ready, so the cluster looked healthy while its workflow-driven
// convergence path was entirely dead.

// TestControllerSAGrantsWorkflowDispatch pins the grant the controller needs.
func TestControllerSAGrantsWorkflowDispatch(t *testing.T) {
	ReloadClusterRoles()
	if !HasRolePermission([]string{RoleControllerSA}, "workflow.dispatch") {
		t.Fatalf("role %q must grant %q — the controller cannot dispatch any workflow without it",
			RoleControllerSA, "workflow.dispatch")
	}
}

// TestNodeExecutorDoesNotGrantWorkflowDispatch pins the least-privilege half.
//
// Joining nodes are bound to globular-node-executor by ensureNodeExecutorBinding.
// Granting dispatch there would have "fixed" the controller by handing every
// worker node the right to dispatch arbitrary cluster workflows — a privilege
// escalation, and the forbidden repair for this failure.
func TestNodeExecutorDoesNotGrantWorkflowDispatch(t *testing.T) {
	ReloadClusterRoles()
	if HasRolePermission([]string{RoleNodeExecutor}, "workflow.dispatch") {
		t.Fatalf("role %q must NOT grant %q: joining worker nodes carry this role, and dispatch rights belong to %q alone",
			RoleNodeExecutor, "workflow.dispatch", RoleControllerSA)
	}
}

// TestLocalNodeRoleSetCoversBothSides pins that the controller node's combined
// self-binding satisfies both the recorder (workflow.admin, node-executor) and
// the dispatcher (workflow.dispatch, controller-sa). Either role alone is
// insufficient — that gap is exactly what shipped.
func TestLocalNodeRoleSetCoversBothSides(t *testing.T) {
	ReloadClusterRoles()
	combined := []string{RoleNodeExecutor, RoleControllerSA}
	for _, action := range []string{"workflow.admin", "workflow.dispatch"} {
		if !HasRolePermission(combined, action) {
			t.Errorf("the controller node's role set %v must grant %q", combined, action)
		}
	}
}
