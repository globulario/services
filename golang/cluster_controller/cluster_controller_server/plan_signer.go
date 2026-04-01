package main

import (
	"log"

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
// Best-effort: logs warning on failure, does not block the caller.
func (srv *server) ensureNodeExecutorBinding(nodePrincipal string) {
	address, err := config.GetAddress()
	if err != nil {
		log.Printf("WARN ensureNodeExecutorBinding: cannot resolve local address: %v", err)
		return
	}

	client, err := rbac_client.NewRbacService_Client(address, "rbac.RbacService")
	if err != nil {
		log.Printf("WARN ensureNodeExecutorBinding: cannot connect to RBAC service: %v", err)
		return
	}
	defer client.Close()

	if err := client.SetRoleBinding(nodePrincipal, []string{security.RoleNodeExecutor}); err != nil {
		log.Printf("WARN ensureNodeExecutorBinding: failed to set role binding for %s: %v", nodePrincipal, err)
		return
	}
	log.Printf("ensureNodeExecutorBinding: bound %s to role %s", nodePrincipal, security.RoleNodeExecutor)
}
