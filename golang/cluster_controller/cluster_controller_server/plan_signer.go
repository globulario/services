package main

import (
	"log"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/rbac/rbac_client"
	"github.com/globulario/services/golang/security"
)

// planSigner is retained as a minimal struct for backward compatibility.
// The plan signing system is deprecated — workflows don't use signed plans.
type planSigner struct{}

// initPlanSigner is a no-op — plan signing removed.
// Kept because main.go calls it during startup; will be removed in a future cleanup.
func (srv *server) initPlanSigner() error {
	srv.planSignerState = &planSigner{}
	return nil
}

// ensureNodeExecutorBinding creates an RBAC role binding for a node principal.
// Best-effort: logs warning on failure, does not block the caller. Returns the
// error so retrying callers (ensureLocalNodeExecutorBinding) can react; the
// fire-and-forget join caller ignores it.
//
// Joining nodes get exactly globular-node-executor. The controller node needs
// more (see ensureLocalNodeExecutorBinding), so the role set is a parameter
// rather than a constant — a joining worker must never receive the
// controller's dispatch rights.
func (srv *server) ensureNodeExecutorBinding(nodePrincipal string, roles ...string) error {
	if len(roles) == 0 {
		roles = []string{security.RoleNodeExecutor}
	}
	address, err := config.GetAddress()
	if err != nil {
		log.Printf("WARN ensureNodeExecutorBinding: cannot resolve local address: %v", err)
		return err
	}

	client, err := rbac_client.NewRbacService_Client(address, "rbac.RbacService")
	if err != nil {
		log.Printf("WARN ensureNodeExecutorBinding: cannot connect to RBAC service: %v", err)
		return err
	}
	defer client.Close()

	if err := client.SetRoleBinding(nodePrincipal, roles); err != nil {
		log.Printf("WARN ensureNodeExecutorBinding: failed to set role binding for %s: %v", nodePrincipal, err)
		return err
	}
	log.Printf("ensureNodeExecutorBinding: bound %s to roles %v", nodePrincipal, roles)
	return nil
}

// localNodeRoles is the role set for the founding/controller node's OWN mTLS
// identity.
//
// globular-node-executor covers the node-agent side (workflow.admin, so the
// trace recorder can persist RecordOutcome / RecordPhaseTransition).
//
// globular-controller-sa covers the controller side. It is required because
// ExecuteWorkflow is gated on action "workflow.dispatch", which
// globular-node-executor does NOT grant — "workflow.admin" is a sibling action
// key, not a wildcard over it. Without this role the controller cannot dispatch
// ANY workflow: on a clean 5-node bring-up (2026-08-10) the controller logged
// 2112 `PermissionDenied: workflow.dispatch` errors, killing release.apply.package
// for every infrastructure package and node.bootstrap for every joining node.
//
// The failure was near-silent because the bootstrap phase machine and the
// node-agent's local repair both cover for it: the cluster still reached
// workload_ready, so it looked converged while the entire workflow-driven
// convergence path — the one that owns cluster mutations
// (intent:workflow.source_of_operational_truth) — was dead.
//
// This is scoped to the LOCAL node deliberately. Adding workflow.dispatch to
// globular-node-executor instead would hand dispatch rights to every joining
// worker, which is a privilege escalation, not a fix.
var localNodeRoles = []string{security.RoleNodeExecutor, security.RoleControllerSA}

// ensureLocalNodeExecutorBinding binds the founding/controller node's OWN mTLS
// identity to globular-node-executor at startup.
//
// Joining nodes receive a node-executor binding via ensureNodeExecutorBinding
// during the join flow (handlers_join.go), but the founding node bootstraps and
// never joins — so without this it holds NO binding granting workflow.admin. Its
// workflow trace recorder authenticates with the node's mTLS cert CN (which
// equals config.GetName()), so RecordOutcome / RecordPhaseTransition were hard-
// denied by the workflow service's RBAC interceptor. The recorder is
// fire-and-forget, so that failure was silent (the audit trail simply never
// persisted). This closes the founding-node special-case.
//
// Note the identity scheme: joining nodes are bound by their node_<uuid> token
// principal, but the founding node's recorder presents its mTLS cert CN, so the
// binding subject here is the node NAME (config.GetName()), not node_<uuid>.
//
// Best-effort with bounded retry — the local RBAC service may not be reachable
// the instant the controller starts. SetRoleBinding is idempotent, and the
// binding persists once written.
func (srv *server) ensureLocalNodeExecutorBinding() {
	name, err := config.GetName()
	if err != nil || name == "" {
		log.Printf("WARN ensureLocalNodeExecutorBinding: cannot resolve local node name: %v", err)
		return
	}
	for attempt := 1; attempt <= 12; attempt++ {
		if err := srv.ensureNodeExecutorBinding(name, localNodeRoles...); err == nil {
			return // success logged by ensureNodeExecutorBinding
		}
		time.Sleep(10 * time.Second)
	}
	log.Printf("WARN ensureLocalNodeExecutorBinding: gave up binding local node %q to %v after retries (RBAC unreachable) — "+
		"the controller will be denied workflow.dispatch and cannot run release or bootstrap workflows",
		name, localNodeRoles)
}
