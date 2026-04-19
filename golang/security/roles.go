package security

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/globulario/services/golang/policy"
)

// Role name constants used across Globular services.
// These are the canonical role identifiers stored in the RBAC service.
// All roles grant semantic action keys (e.g. "workflow.read") resolved from
// proto annotations via authzgen → permissions.generated.json at runtime.
const (
	// ── Human-facing roles ────────────────────────────────────────────────

	// RoleViewer has read-only operational visibility (workflow, cluster,
	// repository, monitoring, AI observability).
	RoleViewer = "globular-viewer"

	// RoleAdmin has full cluster administrative authority expressed as
	// semantic action-key wildcards (workflow.*, cluster_controller.*, ...).
	// Uses no raw /* grant — prefer explicit families.
	RoleAdmin = "globular-admin"

	// RoleOperator is the day-2 human operator: extends viewer with workflow
	// controls (retry/cancel/resume), join approvals, and limited AI ops.
	RoleOperator = "globular-operator"

	// RoleSecurityAdmin manages RBAC, identities, and authentication.
	RoleSecurityAdmin = "globular-security-admin"

	// RoleRepositoryEditor can publish and manage artifacts but not delete them.
	RoleRepositoryEditor = "globular-repository-editor"

	// RoleRepositoryAdmin has full repository lifecycle authority.
	RoleRepositoryAdmin = "globular-repository-admin"

	// RoleAIOperator can read and write AI services but not admin or execute.
	RoleAIOperator = "globular-ai-operator"

	// RoleAIAdmin has full AI authority including the executor.
	RoleAIAdmin = "globular-ai-admin"

	// RoleMonitoringViewer has read-only access to Prometheus metrics/alerts.
	RoleMonitoringViewer = "globular-monitoring-viewer"

	// RoleBackupAdmin can run, inspect, and restore backups.
	RoleBackupAdmin = "globular-backup-admin"

	// ── Legacy human role (prefer RoleRepositoryEditor for new deployments) ─

	// RolePublisher can upload artifacts and publish services/apps to the registry.
	RolePublisher = "globular-publisher"

	// ── Service-account roles ─────────────────────────────────────────────

	// RoleControllerSA is the service account for the cluster controller.
	// Full control-plane authority plus workflow and repository reads.
	RoleControllerSA = "globular-controller-sa"

	// RoleNodeAgentSA is the service account for node agents.
	// Node execution and reporting only; cannot mutate desired state.
	RoleNodeAgentSA = "globular-node-agent-sa"

	// RoleNodeExecutor is the per-node scoped role for node_<uuid> principals.
	// Narrower than RoleNodeAgentSA — only plan execution and status reporting.
	RoleNodeExecutor = "globular-node-executor"

	// RoleWorkflowWriterSA is the internal service account for workflow producers.
	// Grants only the workflow internal write path (admin actions).
	RoleWorkflowWriterSA = "globular-workflow-writer-sa"

	// RoleAIWatcherSA is the service account for the AI watcher daemon.
	RoleAIWatcherSA = "globular-ai-watcher-sa"

	// RoleAIMemorySA is the service account for the AI memory service.
	RoleAIMemorySA = "globular-ai-memory-sa"

	// RoleAIRouterSA is the service account for the AI router.
	RoleAIRouterSA = "globular-ai-router-sa"

	// RoleAIExecutorSA is the service account for the AI executor.
	// Dangerous: grants execute authority. No broad cluster control.
	RoleAIExecutorSA = "globular-ai-executor-sa"

	// RoleRepositoryPublisherSA is the service account for automated publish pipelines.
	RoleRepositoryPublisherSA = "globular-repository-publisher-sa"

	// ── Exceptional roles (tightly guarded) ──────────────────────────────

	// RoleBootstrapSA is accepted only in the Day-0 bootstrap window before
	// the RBAC store exists. Must not be used in steady-state operation.
	RoleBootstrapSA = "globular-bootstrap-sa"

	// RoleBreakglassAdmin grants full access (/*) for disaster recovery only.
	// Every use must be logged. Must be disabled by default.
	RoleBreakglassAdmin = "globular-breakglass-admin"
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

// ReloadClusterRoles re-reads cluster-roles.json from disk and rebuilds the
// method index. Falls back to the embedded default when no policy file exists
// on disk (e.g. in tests or fresh installs before EnsureClusterRolesDeployed).
func ReloadClusterRoles() {
	if extRoles, ok, _ := policy.LoadClusterRoles(); ok {
		RolePermissions = extRoles
		slog.Info("security: reloaded cluster roles from policy file", "roles", len(extRoles))
	} else if embedded, err := policy.LoadEmbeddedClusterRoles(); err == nil {
		RolePermissions = embedded
		slog.Info("security: loaded embedded cluster roles (no file on disk)", "roles", len(embedded))
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
	// RBAC service methods are always excluded to prevent circular gRPC calls
	// when the interceptor calls checkRoleBinding.
	if strings.HasPrefix(action, "/rbac.RbacService/") {
		return false
	}
	if methodSet[action] {
		return true
	}
	for _, p := range methodPrefix {
		if strings.HasPrefix(action, p) {
			return true
		}
		// Also match gRPC paths against action-key wildcard prefixes:
		// "/cluster_controller.Service/Method" matches "cluster_controller." prefix
		if strings.HasPrefix(action, "/") && strings.HasPrefix(action[1:], p) {
			return true
		}
	}
	// If action is a raw gRPC method path, resolve it to a stable action key
	// and check the key against the index. This handles the case where
	// cluster-roles.json uses semantic keys (dns.*) but the caller passes
	// a gRPC path (/dns.DnsService/SetA).
	if strings.HasPrefix(action, "/") {
		if actionKey := policy.GlobalResolver().Resolve(action); actionKey != action {
			if methodSet[actionKey] {
				return true
			}
			for _, p := range methodPrefix {
				if strings.HasPrefix(actionKey, p) {
					return true
				}
			}
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

	// Forward shim: if the action is a stable action key,
	// check whether any legacy method-path aliases match the role grants.
	// Covers: roles still containing "/file.FileService/ReadFile" but interceptor sends "file.read".
	// TODO: Remove once all role grants use stable action keys.
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

	// Reverse shim: if the action is a raw gRPC method path, resolve it to
	// its stable action key and check that key against the role grants.
	// Covers: roles using semantic "dns.*" wildcards but caller passes
	// "/dns.DnsService/SetA" (e.g. tests or older interceptors without resolver data).
	if strings.HasPrefix(action, "/") {
		if actionKey := policy.GlobalResolver().Resolve(action); actionKey != action {
			for _, role := range roles {
				for _, perm := range RolePermissions[role] {
					if matchesPermission(perm, actionKey) {
						return true
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
	// Action-key wildcard: "file.*" matches "file.read" or "/file.FileService/ReadFile"
	if strings.HasSuffix(perm, ".*") {
		prefix := strings.TrimSuffix(perm, "*")
		if strings.HasPrefix(action, prefix) {
			return true
		}
		// Also match gRPC paths: "/cluster_controller.Service/Method" matches "cluster_controller.*"
		if strings.HasPrefix(action, "/") && strings.HasPrefix(action[1:], prefix) {
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
	required := []string{
		RoleAdmin, RoleOperator, RoleViewer,
		RoleControllerSA, RoleNodeAgentSA, RoleNodeExecutor,
		RoleBootstrapSA, RoleBreakglassAdmin,
	}
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
