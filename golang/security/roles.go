package security

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/globulario/services/golang/policy"
)

// Role name constants used across Globular services.
// These are the canonical role identifiers stored in the RBAC service.
const (
	// RoleAdmin has full access to all cluster operations ("/*" wildcard).
	RoleAdmin = "globular-admin"

	// RolePublisher can upload artifacts and publish services/apps to the registry.
	RolePublisher = "globular-publisher"

	// RoleOperator can manage service releases, install/uninstall services,
	// and manage domains/ingress.  Intended for human operators.
	RoleOperator = "globular-operator"

	// RoleControllerSA is the least-privilege service account for the
	// cluster-controller.  It can read/apply release state and node plans,
	// but CANNOT publish artifacts or create new releases.
	RoleControllerSA = "globular-controller-sa"

	// RoleNodeAgentSA is the least-privilege service account for node agents.
	// It can report node status and execute plans addressed to the local node,
	// but CANNOT create or modify ServiceRelease objects.
	RoleNodeAgentSA = "globular-node-agent-sa"

	// RoleNodeExecutor is the per-node scoped role for node_<uuid> principals.
	// It can only operate on its own node's plans, status, and packages.
	// It CANNOT modify desired state, RBAC, publish packages, or access other nodes.
	RoleNodeExecutor = "globular-node-executor"
)

// RolePermissions maps each role to the set of gRPC method paths or stable
// action keys it is allowed to call.
//
// Loaded exclusively from an external cluster-roles.json policy file.
// Search order:
//  1. /etc/globular/policy/rbac/cluster-roles.json          (admin override)
//  2. /var/lib/globular/policy/rbac/cluster-roles.generated.json  (package-shipped)
//  3. /var/lib/globular/policy/rbac/cluster-roles.json       (legacy)
//
// "/*" is the global wildcard: grants access to every gRPC method.
// "/pkg.Service/*" is a service wildcard: grants access to all methods in
// the named service.
var RolePermissions map[string][]string

// methodSet is the set of exact gRPC methods listed in RolePermissions
// (excluding global "/*" wildcard, but including service-wildcard prefixes).
var (
	methodSet    map[string]bool
	methodPrefix []string
)

func init() {
	// Load cluster roles from external policy file.
	// On service nodes, cluster-roles.json lives in /var/lib/globular/policy/rbac/.
	// On developer machines (CLI usage), the file is typically absent — that's fine
	// because the CLI doesn't make role-binding decisions locally.
	if extRoles, ok, _ := policy.LoadClusterRoles(); ok {
		RolePermissions = extRoles
		slog.Info("security: loaded cluster roles from external policy file", "roles", len(extRoles))
	} else {
		slog.Debug("security: no cluster-roles.json found — RolePermissions empty (normal for CLI usage)")
		RolePermissions = make(map[string][]string)
	}

	rebuildMethodIndex()
}

// rebuildMethodIndex recomputes methodSet and methodPrefix from RolePermissions.
// Handles both gRPC method paths (/pkg.Service/Method) and stable action keys (file.read).
func rebuildMethodIndex() {
	methodSet = make(map[string]bool)
	methodPrefix = nil
	for _, entries := range RolePermissions {
		for _, m := range entries {
			if m == "/*" || m == "*" {
				continue // global wildcard
			} else if strings.HasSuffix(m, "/*") || strings.HasSuffix(m, ".*") {
				// wildcard — record the prefix (both /pkg.Service/* and file.*)
				prefix := m[:len(m)-1] // strip the trailing *
				methodPrefix = append(methodPrefix, prefix)
			} else {
				methodSet[m] = true
			}
		}
	}
}

// IsRoleBasedMethod returns true if the given action (stable action key or
// gRPC method path) is explicitly managed by the role-binding system (i.e.
// appears in at least one non-global entry in RolePermissions, either by
// exact match or wildcard prefix).
func IsRoleBasedMethod(action string) bool {
	if methodSet[action] {
		return true
	}
	for _, p := range methodPrefix {
		if strings.HasPrefix(action, p) {
			return true
		}
	}
	// Migration compatibility: if the action is a stable key, also check
	// whether any of its legacy method-path aliases are role-based.
	// TODO: Remove once all RolePermissions entries use stable action keys.
	if policy.IsActionKey(action) {
		if legacyMethods := policy.GlobalResolver().LegacyMethods(action); len(legacyMethods) > 0 {
			for _, method := range legacyMethods {
				if methodSet[method] {
					return true
				}
				for _, p := range methodPrefix {
					if strings.HasPrefix(method, p) {
						return true
					}
				}
			}
		}
	}
	return false
}

// HasRolePermission returns true if any of the given roles grants access to
// the specified action. Supports:
//   - Exact match: "file.read" == "file.read", or "/pkg.Service/Method" == "/pkg.Service/Method"
//   - Global wildcard: "*" or "/*" grants all
//   - Action-key wildcard: "file.*" matches "file.read", "file.write", etc.
//   - Method-path wildcard: "/pkg.Service/*" matches "/pkg.Service/Method"
//
// Migration compatibility: if the action is a stable action key (e.g., "file.read")
// and no role grants match it directly, the function also checks legacy method-path
// aliases via the ActionResolver. This allows old grants like "/file.FileService/ReadFile"
// to still authorize "file.read" during the migration period.
// TODO: Remove legacy alias matching once all roles use stable action keys.
func HasRolePermission(roles []string, action string) bool {
	for _, role := range roles {
		for _, perm := range RolePermissions[role] {
			if matchesPermission(perm, action) {
				return true
			}
		}
	}

	// Migration compatibility shim: if the action is a stable action key,
	// check whether any legacy method-path aliases match the role grants.
	// This covers the case where roles still contain grants like
	// "/file.FileService/ReadFile" but the interceptor sends "file.read".
	// TODO: Remove this shim once all role grants use stable action keys.
	if policy.IsActionKey(action) {
		if legacyMethods := policy.GlobalResolver().LegacyMethods(action); len(legacyMethods) > 0 {
			for _, method := range legacyMethods {
				for _, role := range roles {
					for _, perm := range RolePermissions[role] {
						if matchesPermission(perm, method) {
							return true
						}
					}
				}
			}
		}
	}

	return false
}

// matchesPermission checks if a single permission grant matches an action.
func matchesPermission(perm, action string) bool {
	// Global wildcards
	if perm == "*" || perm == "/*" {
		return true
	}
	// Exact match
	if perm == action {
		return true
	}
	// Action-key wildcard: "file.*" matches "file.read"
	if strings.HasSuffix(perm, ".*") {
		prefix := strings.TrimSuffix(perm, "*")
		if strings.HasPrefix(action, prefix) {
			return true
		}
	}
	// Legacy method-path wildcard: "/pkg.Service/*" matches "/pkg.Service/Method"
	if strings.HasSuffix(perm, "/*") {
		prefix := strings.TrimSuffix(perm, "*")
		if strings.HasPrefix(action, prefix) {
			return true
		}
	}
	return false
}

// EnsureBuiltinRolesExist verifies that the expected roles exist in the
// loaded RolePermissions map. Called at cluster bootstrap to guarantee
// that role bindings never target missing roles.
// Returns an error listing any missing roles.
func EnsureBuiltinRolesExist() error {
	required := []string{RoleAdmin, RolePublisher, RoleOperator, RoleControllerSA, RoleNodeAgentSA, RoleNodeExecutor}
	var missing []string
	for _, role := range required {
		if _, ok := RolePermissions[role]; !ok {
			missing = append(missing, role)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing roles in cluster-roles.json: %v — ensure the policy file is deployed", missing)
	}
	return nil
}

// NodeExecutorPermissions returns a copy of the node-executor permission list.
// Useful for audit and bootstrap verification.
func NodeExecutorPermissions() []string {
	perms := RolePermissions[RoleNodeExecutor]
	out := make([]string, len(perms))
	copy(out, perms)
	return out
}

// DefaultServiceAccountNames returns the service account identities for
// built-in automation principals.  These principals are typically represented
// as JWT applications (email == "") with these exact Subject values.
var DefaultServiceAccountNames = map[string]string{
	// The cluster-controller service account
	"controller": "globular-controller",
	// The node-agent service account
	"node-agent": "globular-node-agent",
	// The gateway service account
	"gateway": "globular-gateway",
}
