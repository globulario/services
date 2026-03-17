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

// RolePermissions maps each role to the set of gRPC method paths it is
// allowed to call.  These are the MINIMUM permission sets for each role;
// admins can always expand them via the RBAC service.
//
// "/*" is the global wildcard: grants access to every gRPC method.
// "/pkg.Service/*" is a service wildcard: grants access to all methods in
// the named service.
var RolePermissions = map[string][]string{
	// -------------------------------------------------------------------------
	// Admin: unrestricted access
	// -------------------------------------------------------------------------
	RoleAdmin: {"/*"},

	// -------------------------------------------------------------------------
	// Publisher: artifact/service publishing pipeline
	// -------------------------------------------------------------------------
	RolePublisher: {
		"/discovery.PackageDiscovery/PublishService",
		"/discovery.PackageDiscovery/PublishApplication",
		"/repository.PackageRepository/UploadArtifact",
		"/repository.PackageRepository/UploadBundle",
		// Read-back for verification after publish
		"/repository.PackageRepository/GetArtifactManifest",
		"/repository.PackageRepository/ListArtifacts",
		"/discovery.PackageDiscovery/GetPackageDescriptor",
	},

	// -------------------------------------------------------------------------
	// Operator: service lifecycle + domain management
	// -------------------------------------------------------------------------
	RoleOperator: {
		// ServiceRelease CRUD
		"/clustercontroller.ResourcesService/ApplyServiceRelease",
		"/clustercontroller.ResourcesService/GetServiceRelease",
		"/clustercontroller.ResourcesService/ListServiceReleases",
		"/clustercontroller.ResourcesService/DeleteServiceRelease",
		// Desired-version management
		"/clustercontroller.ResourcesService/ApplyServiceDesiredVersion",
		"/clustercontroller.ResourcesService/DeleteServiceDesiredVersion",
		"/clustercontroller.ResourcesService/ListServiceDesiredVersions",
		// Node plans (apply and inspect)
		"/clustercontroller.ClusterControllerService/ApplyNodePlan",
		"/clustercontroller.ClusterControllerService/GetNodePlan",
		"/clustercontroller.ClusterControllerService/ListNodes",
		// Cluster lifecycle
		"/clustercontroller.ClusterControllerService/UpgradeGlobular",
		"/clustercontroller.ClusterControllerService/UpdateClusterNetwork",
		// Domain / DNS management
		"/dns.DnsService/*",
		// Health / status (read)
		"/clustercontroller.ClusterControllerService/GetClusterHealth",
		"/clustercontroller.ClusterControllerService/GetClusterInfo",
	},

	// -------------------------------------------------------------------------
	// Controller SA: least-privilege for cluster-controller automation
	// -------------------------------------------------------------------------
	RoleControllerSA: {
		// Read ServiceRelease (needed to compute reconciliation diff)
		"/clustercontroller.ResourcesService/GetServiceRelease",
		"/clustercontroller.ResourcesService/ListServiceReleases",
		// Apply desired-version state (output of reconcile)
		"/clustercontroller.ResourcesService/ApplyServiceDesiredVersion",
		"/clustercontroller.ResourcesService/ListServiceDesiredVersions",
		// Apply node plans (schedule work on agents)
		"/clustercontroller.ClusterControllerService/ApplyNodePlan",
		"/clustercontroller.ClusterControllerService/GetNodePlan",
		// Read node status
		"/clustercontroller.ClusterControllerService/ListNodes",
		"/clustercontroller.ClusterControllerService/ReportNodeStatus",
		// Watch for streaming updates
		"/clustercontroller.ResourcesService/Watch",
		"/clustercontroller.ClusterControllerService/WatchOperations",
		// Complete operations
		"/clustercontroller.ClusterControllerService/CompleteOperation",
		// Cluster network (read)
		"/clustercontroller.ResourcesService/GetClusterNetwork",
		// Health (read)
		"/clustercontroller.ClusterControllerService/GetClusterHealth",
		"/clustercontroller.ClusterControllerService/GetClusterInfo",
	},

	// -------------------------------------------------------------------------
	// Node executor: per-node scoped role for node_<uuid> principals
	// -------------------------------------------------------------------------
	RoleNodeExecutor: {
		// Report own status to controller
		"/clustercontroller.ClusterControllerService/ReportNodeStatus",
		// Report plan rejections
		"/clustercontroller.ClusterControllerService/ReportPlanRejection",
		// Join workflow (needed during bootstrap)
		"/clustercontroller.ClusterControllerService/RequestJoin",
		"/clustercontroller.ClusterControllerService/GetJoinRequestStatus",
		// Execute plans addressed to this node
		"/nodeagent.NodeAgentService/ApplyPlan",
		"/nodeagent.NodeAgentService/ApplyPlanV1",
		"/nodeagent.NodeAgentService/GetPlanStatusV1",
		"/nodeagent.NodeAgentService/WatchPlanStatusV1",
		"/nodeagent.NodeAgentService/WatchOperation",
		"/nodeagent.NodeAgentService/GetInventory",
		// Installed-state reporting (own node only)
		"/nodeagent.NodeAgentService/ListInstalledPackages",
		"/nodeagent.NodeAgentService/GetInstalledPackage",
		// Download artifacts from repository (v1: unauthenticated; listed for audit)
		"/repository.PackageRepository/DownloadArtifact",
		"/repository.PackageRepository/GetArtifactManifest",
		// Notify controller when plan execution completes
		"/clustercontroller.ClusterControllerService/CompleteOperation",
		// Cluster info needed for plan execution
		"/clustercontroller.ClusterControllerService/GetClusterInfo",
		"/clustercontroller.ResourcesService/GetClusterNetwork",
		// DNS operations needed during network reconciliation and ACME cert issuance
		// (node-agent calls local DNS service during plan execution)
		"/dns.DnsService/SetDomains",
		"/dns.DnsService/SetA",
		"/dns.DnsService/SetAAAA",
		"/dns.DnsService/SetSoa",
		"/dns.DnsService/SetNs",
		"/dns.DnsService/SetTXT",
		"/dns.DnsService/RemoveTXT",
		"/dns.DnsService/GetTXT",
		// Backup/restore operations (own node)
		"/nodeagent.NodeAgentService/RunBackupProvider",
		"/nodeagent.NodeAgentService/GetBackupTaskResult",
		"/nodeagent.NodeAgentService/RunRestoreProvider",
		"/nodeagent.NodeAgentService/GetRestoreTaskResult",
	},

	// -------------------------------------------------------------------------
	// Node-agent SA: least-privilege for per-node agent processes
	// -------------------------------------------------------------------------
	RoleNodeAgentSA: {
		// Report own status
		"/clustercontroller.ClusterControllerService/ReportNodeStatus",
		// Request and complete join workflow
		"/clustercontroller.ClusterControllerService/RequestJoin",
		"/clustercontroller.ClusterControllerService/GetJoinRequestStatus",
		// Execute plans addressed to this node
		"/nodeagent.NodeAgentService/ApplyPlan",
		"/nodeagent.NodeAgentService/WatchOperation",
		"/nodeagent.NodeAgentService/GetInventory",
		// Bootstrap (restricted to loopback by BootstrapGate during Day-0)
		"/nodeagent.NodeAgentService/BootstrapFirstNode",
		// Cluster info needed for plan execution
		"/clustercontroller.ClusterControllerService/GetClusterInfo",
		"/clustercontroller.ResourcesService/GetClusterNetwork",
	},
}

// methodSet is the set of exact gRPC methods listed in RolePermissions
// (excluding global "/*" wildcard, but including service-wildcard prefixes).
var (
	methodSet    map[string]bool
	methodPrefix []string
)

func init() {
	// Try loading cluster roles from external policy file; merge with compiled defaults.
	if extRoles, ok, _ := policy.LoadClusterRoles(); ok {
		// External file replaces compiled defaults entirely.
		RolePermissions = extRoles
		slog.Info("security: using cluster roles from external policy file")
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
	return false
}

// HasRolePermission returns true if any of the given roles grants access to
// the specified action. Supports:
//   - Exact match: "file.read" == "file.read", or "/pkg.Service/Method" == "/pkg.Service/Method"
//   - Global wildcard: "*" or "/*" grants all
//   - Action-key wildcard: "file.*" matches "file.read", "file.write", etc.
//   - Method-path wildcard: "/pkg.Service/*" matches "/pkg.Service/Method"
//
// The action parameter should be a stable action key (e.g., "file.read") when
// available, or a raw gRPC method path for backward compatibility.
func HasRolePermission(roles []string, action string) bool {
	for _, role := range roles {
		for _, perm := range RolePermissions[role] {
			if matchesPermission(perm, action) {
				return true
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

// EnsureBuiltinRolesExist verifies that all built-in roles (including
// node-executor) exist in the role permission map. This is called at
// cluster bootstrap to guarantee that role bindings never target missing roles.
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
		return fmt.Errorf("missing built-in roles in RolePermissions: %v", missing)
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
