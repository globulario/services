package security

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
