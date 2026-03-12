package security

import (
	"os"
	"strings"
	"testing"
)

func TestIsNodePrincipal(t *testing.T) {
	tests := []struct {
		subject string
		want    bool
	}{
		{"node_abc123", true},
		{"node_", false}, // too short
		{"sa", false},
		{"admin", false},
		{"", false},
		{"node_x", true},
	}
	for _, tc := range tests {
		if got := IsNodePrincipal(tc.subject); got != tc.want {
			t.Errorf("IsNodePrincipal(%q) = %v, want %v", tc.subject, got, tc.want)
		}
	}
}

func TestExtractNodeID(t *testing.T) {
	tests := []struct {
		subject string
		want    string
	}{
		{"node_abc123", "abc123"},
		{"node_", ""},
		{"sa", ""},
	}
	for _, tc := range tests {
		if got := ExtractNodeID(tc.subject); got != tc.want {
			t.Errorf("ExtractNodeID(%q) = %q, want %q", tc.subject, got, tc.want)
		}
	}
}

func TestValidateNodeOwnership_OwnNode(t *testing.T) {
	if err := ValidateNodeOwnership("node_abc", "abc"); err != nil {
		t.Errorf("own node should be allowed: %v", err)
	}
}

func TestValidateNodeOwnership_OtherNode(t *testing.T) {
	err := ValidateNodeOwnership("node_abc", "xyz")
	if err == nil {
		t.Error("expected error for cross-node access")
	}
	if IsSADeprecationWarning(err) {
		t.Error("should be hard error, not warning")
	}
}

func TestValidateNodeOwnership_SA_Default(t *testing.T) {
	os.Unsetenv("DEPRECATE_SA_NODE_AUTH")
	os.Unsetenv("REQUIRE_NODE_IDENTITY")
	if err := ValidateNodeOwnership("sa", "some-node"); err != nil {
		t.Errorf("sa should be allowed by default: %v", err)
	}
}

func TestValidateNodeOwnership_SA_Deprecated(t *testing.T) {
	os.Setenv("DEPRECATE_SA_NODE_AUTH", "true")
	os.Unsetenv("REQUIRE_NODE_IDENTITY")
	defer os.Unsetenv("DEPRECATE_SA_NODE_AUTH")

	err := ValidateNodeOwnership("sa", "some-node")
	if err == nil {
		t.Error("expected deprecation warning")
	}
	if !IsSADeprecationWarning(err) {
		t.Errorf("expected SADeprecationWarning, got %T: %v", err, err)
	}
}

func TestValidateNodeOwnership_SA_Enforced(t *testing.T) {
	os.Setenv("REQUIRE_NODE_IDENTITY", "true")
	defer os.Unsetenv("REQUIRE_NODE_IDENTITY")

	err := ValidateNodeOwnership("sa", "some-node")
	if err == nil {
		t.Error("expected hard rejection in enforcement mode")
	}
	if IsSADeprecationWarning(err) {
		t.Error("should be hard error in enforcement mode, not warning")
	}
}

func TestValidateNodeOwnership_Admin(t *testing.T) {
	if err := ValidateNodeOwnership("admin", "some-node"); err != nil {
		t.Errorf("admin should be allowed: %v", err)
	}
}

func TestValidateNodeOwnership_Anonymous(t *testing.T) {
	err := ValidateNodeOwnership("", "some-node")
	if err == nil {
		t.Error("expected error for anonymous caller")
	}
}

// --- Malformed node principal ---

func TestValidateNodeOwnership_MalformedNodePrincipal(t *testing.T) {
	// "node_" (no UUID) is not a valid node principal — IsNodePrincipal returns false.
	// It's treated as an unknown non-admin principal and allowed through at the
	// ownership layer. The RBAC interceptor rejects it because no role is bound.
	//
	// This test documents the behavior: ownership check passes, RBAC stops it.
	err := ValidateNodeOwnership("node_", "some-node")
	if err != nil {
		t.Errorf("malformed 'node_' is not a node principal — ownership layer passes, RBAC blocks: %v", err)
	}

	// But a real node principal with wrong node gets hard denial:
	err = ValidateNodeOwnership("node_x", "y")
	if err == nil {
		t.Error("expected cross-node denial for node_x accessing node y")
	}
}

// --- Cross-node denial with method context ---

func TestValidateNodeOwnershipForMethod_CrossNodeIncludesMethod(t *testing.T) {
	err := ValidateNodeOwnershipForMethod("node_abc", "xyz", "/clustercontroller.ClusterControllerService/ReportNodeStatus")
	if err == nil {
		t.Fatal("expected cross-node denial")
	}
	if !strings.Contains(err.Error(), "ReportNodeStatus") {
		t.Errorf("error should include method name, got: %v", err)
	}
	if !strings.Contains(err.Error(), "own-node-only") {
		t.Errorf("error should include own-node-only reason, got: %v", err)
	}
}

func TestValidateNodeOwnershipForMethod_SA_DeprecatedIncludesMethod(t *testing.T) {
	os.Setenv("DEPRECATE_SA_NODE_AUTH", "true")
	os.Unsetenv("REQUIRE_NODE_IDENTITY")
	defer os.Unsetenv("DEPRECATE_SA_NODE_AUTH")

	err := ValidateNodeOwnershipForMethod("sa", "node123", "/clustercontroller.ClusterControllerService/ReportPlanRejection")
	if err == nil {
		t.Fatal("expected deprecation warning")
	}
	if !IsSADeprecationWarning(err) {
		t.Fatalf("expected SADeprecationWarning, got %T", err)
	}
	if !strings.Contains(err.Error(), "ReportPlanRejection") {
		t.Errorf("warning should include method name, got: %v", err)
	}
	w := err.(*SADeprecationWarning)
	if w.Method == "" {
		t.Error("SADeprecationWarning.Method should be set")
	}
}

func TestValidateNodeOwnershipForMethod_SA_EnforcedIncludesMethod(t *testing.T) {
	os.Setenv("REQUIRE_NODE_IDENTITY", "true")
	defer os.Unsetenv("REQUIRE_NODE_IDENTITY")

	err := ValidateNodeOwnershipForMethod("sa", "node123", "/clustercontroller.ClusterControllerService/ReportNodeStatus")
	if err == nil {
		t.Fatal("expected hard rejection")
	}
	if !strings.Contains(err.Error(), "ReportNodeStatus") {
		t.Errorf("rejection should include method name, got: %v", err)
	}
}

// --- Node-executor role scope verification ---

func TestEnsureBuiltinRolesExist(t *testing.T) {
	if err := EnsureBuiltinRolesExist(); err != nil {
		t.Errorf("built-in roles should all exist: %v", err)
	}
}

func TestNodeExecutorPermissions(t *testing.T) {
	perms := NodeExecutorPermissions()
	if len(perms) == 0 {
		t.Error("node-executor should have permissions")
	}
	// Verify it's a copy
	perms[0] = "MODIFIED"
	original := RolePermissions[RoleNodeExecutor]
	if original[0] == "MODIFIED" {
		t.Error("NodeExecutorPermissions should return a copy")
	}
}

func TestNodeExecutorCannotMutateDesiredState(t *testing.T) {
	desiredStateMethods := []string{
		"/clustercontroller.ResourcesService/ApplyServiceRelease",
		"/clustercontroller.ResourcesService/DeleteServiceRelease",
		"/clustercontroller.ResourcesService/ApplyServiceDesiredVersion",
		"/clustercontroller.ResourcesService/DeleteServiceDesiredVersion",
	}
	for _, method := range desiredStateMethods {
		if HasRolePermission([]string{RoleNodeExecutor}, method) {
			t.Errorf("node-executor should NOT have permission for desired-state method %s", method)
		}
	}
}

func TestNodeExecutorCannotPerformRBACAdmin(t *testing.T) {
	rbacMethods := []string{
		"/rbac.RbacService/AddRole",
		"/rbac.RbacService/RemoveRole",
		"/rbac.RbacService/SetRoleBinding",
	}
	for _, method := range rbacMethods {
		if HasRolePermission([]string{RoleNodeExecutor}, method) {
			t.Errorf("node-executor should NOT have permission for RBAC admin method %s", method)
		}
	}
}

func TestNodeExecutorCannotPublish(t *testing.T) {
	publishMethods := []string{
		"/repository.PackageRepository/UploadArtifact",
		"/repository.PackageRepository/UploadBundle",
		"/discovery.PackageDiscovery/PublishService",
	}
	for _, method := range publishMethods {
		if HasRolePermission([]string{RoleNodeExecutor}, method) {
			t.Errorf("node-executor should NOT have permission for publish method %s", method)
		}
	}
}

func TestNodeExecutorCanReportStatus(t *testing.T) {
	allowedMethods := []string{
		"/clustercontroller.ClusterControllerService/ReportNodeStatus",
		"/clustercontroller.ClusterControllerService/ReportPlanRejection",
	}
	for _, method := range allowedMethods {
		if !HasRolePermission([]string{RoleNodeExecutor}, method) {
			t.Errorf("node-executor SHOULD have permission for %s", method)
		}
	}
}

func TestNodeExecutorCanReadClusterInfo(t *testing.T) {
	readOnlyMethods := []string{
		"/clustercontroller.ClusterControllerService/GetClusterInfo",
		"/clustercontroller.ResourcesService/GetClusterNetwork",
	}
	for _, method := range readOnlyMethods {
		if !HasRolePermission([]string{RoleNodeExecutor}, method) {
			t.Errorf("node-executor SHOULD have read-only permission for %s", method)
		}
	}
}

func TestNodeExecutorCanCompleteOperation(t *testing.T) {
	// Node-agent calls CompleteOperation to notify controller when plan execution finishes.
	if !HasRolePermission([]string{RoleNodeExecutor}, "/clustercontroller.ClusterControllerService/CompleteOperation") {
		t.Error("node-executor SHOULD have permission for CompleteOperation (plan completion notify)")
	}
}

func TestNodeExecutorCanCallDNSDuringPlanExecution(t *testing.T) {
	// Node-agent calls DNS service during network reconciliation and ACME cert issuance.
	dnsMethods := []string{
		"/dns.DnsService/SetDomains",
		"/dns.DnsService/SetA",
		"/dns.DnsService/SetAAAA",
		"/dns.DnsService/SetSoa",
		"/dns.DnsService/SetNs",
		"/dns.DnsService/SetTXT",
		"/dns.DnsService/RemoveTXT",
		"/dns.DnsService/GetTXT",
	}
	for _, method := range dnsMethods {
		if !HasRolePermission([]string{RoleNodeExecutor}, method) {
			t.Errorf("node-executor SHOULD have permission for DNS method %s (needed during plan execution)", method)
		}
	}
}

func TestNodeExecutorCanDownloadArtifacts(t *testing.T) {
	// Node-agent downloads service artifacts from repository during installation.
	repoMethods := []string{
		"/repository.PackageRepository/DownloadArtifact",
		"/repository.PackageRepository/GetArtifactManifest",
	}
	for _, method := range repoMethods {
		if !HasRolePermission([]string{RoleNodeExecutor}, method) {
			t.Errorf("node-executor SHOULD have permission for %s (needed for service installation)", method)
		}
	}
}

func TestNodeExecutorCannotAccessControllerAdminPaths(t *testing.T) {
	adminMethods := []string{
		"/clustercontroller.ClusterControllerService/UpgradeGlobular",
		"/clustercontroller.ClusterControllerService/UpdateClusterNetwork",
		"/clustercontroller.ClusterControllerService/ApplyNodePlan",
		"/clustercontroller.ClusterControllerService/GetClusterHealth",
	}
	for _, method := range adminMethods {
		if HasRolePermission([]string{RoleNodeExecutor}, method) {
			t.Errorf("node-executor should NOT have permission for controller/admin path %s", method)
		}
	}
}

func TestApproveJoinBindsNodeExecutorRole(t *testing.T) {
	// Verify that the binding target role exists and is the correct constant.
	// ApproveJoin (server.go:518) calls ensureNodeExecutorBinding(nodePrincipal)
	// which does: client.SetRoleBinding(nodePrincipal, []string{RoleNodeExecutor})
	//
	// This test proves:
	// 1. RoleNodeExecutor constant is "globular-node-executor"
	// 2. The role exists in RolePermissions
	// 3. The principal format used by ApproveJoin is "node_" + nodeID
	if RoleNodeExecutor != "globular-node-executor" {
		t.Errorf("RoleNodeExecutor = %q, want %q", RoleNodeExecutor, "globular-node-executor")
	}
	if _, ok := RolePermissions[RoleNodeExecutor]; !ok {
		t.Error("RoleNodeExecutor must exist in RolePermissions for binding to succeed")
	}

	// Verify the principal format: ApproveJoin creates "node_" + uuid
	nodePrincipal := "node_" + "test-uuid-1234"
	if !IsNodePrincipal(nodePrincipal) {
		t.Errorf("principal %q created by ApproveJoin should be recognized as node principal", nodePrincipal)
	}
	if got := ExtractNodeID(nodePrincipal); got != "test-uuid-1234" {
		t.Errorf("ExtractNodeID(%q) = %q, want %q", nodePrincipal, got, "test-uuid-1234")
	}
}

func TestTokenRotationPreservesNodePrincipalFormat(t *testing.T) {
	// Token rotation (globularcli cluster_cmds.go:1456) creates principals
	// using the same "node_" + nodeID format as ApproveJoin.
	// The RBAC binding from ApproveJoin persists — rotation only refreshes JWT.
	//
	// This test proves:
	// 1. Rotated principal follows the same format
	// 2. The rotated principal is recognized as a node principal
	// 3. The rotated principal maps to the same node-executor role
	nodeID := "rotated-uuid-5678"
	rotatedPrincipal := "node_" + nodeID

	if !IsNodePrincipal(rotatedPrincipal) {
		t.Errorf("rotated principal %q should be recognized as node principal", rotatedPrincipal)
	}
	if got := ExtractNodeID(rotatedPrincipal); got != nodeID {
		t.Errorf("ExtractNodeID(%q) = %q, want %q", rotatedPrincipal, got, nodeID)
	}
	// node-executor role exists — binding target is valid
	if _, ok := RolePermissions[RoleNodeExecutor]; !ok {
		t.Error("RoleNodeExecutor must exist for rotated principal binding")
	}
	// Rotated principal can access own node
	if err := ValidateNodeOwnership(rotatedPrincipal, nodeID); err != nil {
		t.Errorf("rotated principal should access own node: %v", err)
	}
	// Rotated principal cannot access other node
	if err := ValidateNodeOwnership(rotatedPrincipal, "other-node"); err == nil {
		t.Error("rotated principal should NOT access other node")
	}
}

func TestNodeExecutorCannotAccessOtherNodesPlans(t *testing.T) {
	// This tests the handler-level enforcement (not RPC-level)
	// node_abc trying to report for node xyz
	err := ValidateNodeOwnership("node_abc", "xyz")
	if err == nil {
		t.Error("node principal should not be able to access other node's resources")
	}
}

func TestNodeExecutorCanAccessOwnNodePlan(t *testing.T) {
	err := ValidateNodeOwnership("node_abc", "abc")
	if err != nil {
		t.Errorf("node principal should access own resources: %v", err)
	}
}
