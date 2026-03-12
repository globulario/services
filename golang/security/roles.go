package security

import (
	"fmt"
	"strings"
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
	methodSet = make(map[string]bool)
	for _, methods := range RolePermissions {
		for _, m := range methods {
			if m == "/*" {
				continue // global wildcard: don't add every method in existence
			} else if strings.HasSuffix(m, "/*") {
				// service wildcard — record the prefix
				prefix := strings.TrimSuffix(m, "*")
				methodPrefix = append(methodPrefix, prefix)
			} else {
				methodSet[m] = true
			}
		}
	}
}

// IsRoleBasedMethod returns true if the gRPC full method is explicitly managed
// by the role-binding system (i.e. appears in at least one non-global entry in
// RolePermissions, either by exact match or service-wildcard prefix).
func IsRoleBasedMethod(method string) bool {
	if methodSet[method] {
		return true
	}
	for _, p := range methodPrefix {
		if strings.HasPrefix(method, p) {
			return true
		}
	}
	return false
}

// HasRolePermission returns true if any of the given roles grants access to
// the specified gRPC method.  Supports exact, global "/*", and service
// wildcard "/pkg.Service/*" patterns.
func HasRolePermission(roles []string, method string) bool {
	for _, role := range roles {
		for _, perm := range RolePermissions[role] {
			if perm == "/*" || perm == method {
				return true
			}
			if strings.HasSuffix(perm, "/*") {
				prefix := strings.TrimSuffix(perm, "*")
				if strings.HasPrefix(method, prefix) {
					return true
				}
			}
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
