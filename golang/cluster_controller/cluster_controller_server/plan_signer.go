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
func (srv *server) ensureNodeExecutorBinding(nodePrincipal string) error {
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

	if err := client.SetRoleBinding(nodePrincipal, []string{security.RoleNodeExecutor}); err != nil {
		log.Printf("WARN ensureNodeExecutorBinding: failed to set role binding for %s: %v", nodePrincipal, err)
		return err
	}
	log.Printf("ensureNodeExecutorBinding: bound %s to role %s", nodePrincipal, security.RoleNodeExecutor)
	return nil
}

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
		if err := srv.ensureNodeExecutorBinding(name); err == nil {
			return // success logged by ensureNodeExecutorBinding
		}
		time.Sleep(10 * time.Second)
	}
	log.Printf("WARN ensureLocalNodeExecutorBinding: gave up binding local node %q to %s after retries (RBAC unreachable)",
		name, security.RoleNodeExecutor)
}
