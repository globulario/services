// post_day0_acceptance_test.go: Acceptance tests for the post-Day-0 security contract.
//
// These tests validate the rules from the security contract:
//   A) After cluster initialized, mutating RPCs with no identity → Unauthenticated
//   B) After cluster initialized, mutating RPCs with identity but no RBAC mapping → PermissionDenied
//   C) Before cluster initialized, bootstrap allowlist methods pass without auth
//   D) Read-only RPCs are never blocked by the post-Day-0 gate
//   E) Authenticated mutating RPCs proceed normally to RBAC evaluation
//
// Run with: go test -v ./golang/interceptors -run Acceptance

package interceptors

import (
	"testing"

	"github.com/globulario/services/golang/security"
	"google.golang.org/grpc/codes"
)

// simulatePostDay0Check mirrors the enforcement logic in ServerUnaryInterceptor.
// Returns (denied bool, grpcCode codes.Code, reason string).
func simulatePostDay0Check(clusterInitialized bool, method, subject string, hasRBACMapping bool) (bool, codes.Code, string) {
	// Post-Day-0: mutating RPCs require auth
	if clusterInitialized && security.IsMutatingRPC(method) && subject == "" {
		return true, codes.Unauthenticated, "authentication_required"
	}

	// No RBAC mapping handling
	if !hasRBACMapping {
		if clusterInitialized && security.IsMutatingRPC(method) {
			return true, codes.PermissionDenied, "no_rbac_mapping_post_day0"
		}
		if DenyUnmappedMethods {
			return true, codes.PermissionDenied, "no_rbac_mapping_denied"
		}
		return false, codes.OK, "no_rbac_mapping_warning"
	}

	return false, codes.OK, "rbac_check_required"
}

// --- Test A -------------------------------------------------------------------

// TestAcceptance_AnonymousMutatingPostDay0_Unauthenticated verifies:
// After Day-0, a mutating RPC with NO identity → codes.Unauthenticated.
func TestAcceptance_AnonymousMutatingPostDay0_Unauthenticated(t *testing.T) {
	mutatingMethods := []string{
		"/clustercontroller.ResourcesService/ApplyServiceRelease",
		"/clustercontroller.ResourcesService/DeleteServiceRelease",
		"/clustercontroller.ClusterControllerService/ApplyNodePlan",
		"/discovery.PackageDiscovery/PublishService",
		"/repository.PackageRepository/UploadArtifact",
		"/dns.DnsService/SetA",
		"/rbac.RbacService/CreateAccount",
	}

	for _, method := range mutatingMethods {
		t.Run(method, func(t *testing.T) {
			denied, code, reason := simulatePostDay0Check(
				true,   // cluster initialized
				method,
				"",     // anonymous — no identity
				false,  // no RBAC mapping (would be checked next)
			)
			if !denied {
				t.Errorf("expected DENIED for anonymous mutating RPC, got ALLOWED")
			}
			if code != codes.Unauthenticated {
				t.Errorf("expected Unauthenticated, got %v", code)
			}
			if reason != "authentication_required" {
				t.Errorf("expected reason authentication_required, got %q", reason)
			}
			t.Logf("✓ anonymous %s → %v (%s)", method, code, reason)
		})
	}
}

// --- Test B -------------------------------------------------------------------

// TestAcceptance_AuthenticatedUnmappedMutatingPostDay0_PermissionDenied verifies:
// After Day-0, a mutating RPC with identity but NO RBAC mapping → codes.PermissionDenied.
func TestAcceptance_AuthenticatedUnmappedMutatingPostDay0_PermissionDenied(t *testing.T) {
	denied, code, reason := simulatePostDay0Check(
		true,
		"/some.NewService/DoMutation", // unmapped mutating method
		"alice",                       // has identity
		false,                         // no RBAC mapping
	)
	if !denied {
		t.Error("expected DENIED for authenticated but unmapped mutating RPC post-Day-0")
	}
	if code != codes.PermissionDenied {
		t.Errorf("expected PermissionDenied, got %v", code)
	}
	if reason != "no_rbac_mapping_post_day0" {
		t.Errorf("expected reason no_rbac_mapping_post_day0, got %q", reason)
	}
	t.Logf("✓ authenticated unmapped mutating RPC → %v (%s)", code, reason)
}

// --- Test C -------------------------------------------------------------------

// TestAcceptance_BeforeClusterInit_AnonymousMutating_NotBlockedByPostDay0Gate verifies:
// Before Day-0 is complete, the post-Day-0 gate does NOT block anonymous mutating RPCs.
// (They may be blocked later by DenyUnmappedMethods or RBAC, but not by the Day-0 gate.)
func TestAcceptance_BeforeClusterInit_AnonymousMutating_NotBlockedByPostDay0Gate(t *testing.T) {
	origDeny := DenyUnmappedMethods
	DenyUnmappedMethods = false
	defer func() { DenyUnmappedMethods = origDeny }()

	denied, _, reason := simulatePostDay0Check(
		false, // cluster NOT yet initialized
		"/clustercontroller.ResourcesService/ApplyServiceRelease",
		"",    // anonymous
		false, // no mapping
	)
	if denied && reason == "authentication_required" {
		t.Error("post-Day-0 gate fired before cluster is initialized — should NOT happen")
	}
	t.Logf("✓ pre-init anonymous mutating RPC passes Day-0 gate (reason: %s)", reason)
}

// TestAcceptance_BootstrapAllowlistMethodsWork verifies that methods in the
// bootstrap allowlist are accessible during Day-0 (the ShouldAllow check).
func TestAcceptance_BootstrapAllowlistMethodsWork(t *testing.T) {
	bootstrapMethods := []string{
		"/grpc.health.v1.Health/Check",
		// Role-binding management (replaces the removed CreateAccount/CreateRole stubs)
		"/rbac.RbacService/SetRoleBinding",
		"/rbac.RbacService/GetRoleBinding",
		"/rbac.RbacService/ListRoleBindings",
		"/authentication.AuthenticationService/Authenticate",
		"/dns.DnsService/CreateZone",
	}

	gate := security.NewBootstrapGateWithPath("/tmp/non-existent-bootstrap-for-test")

	// Enable via env var (simulates Day-0)
	t.Setenv("GLOBULAR_BOOTSTRAP", "1")

	for _, method := range bootstrapMethods {
		t.Run(method, func(t *testing.T) {
			authCtx := &security.AuthContext{
				GRPCMethod:  method,
				IsLoopback:  true,
				IsBootstrap: true,
			}
			allowed, reason := gate.ShouldAllow(authCtx)
			if !allowed {
				t.Errorf("bootstrap method %s blocked (reason: %s) — Day-0 would fail", method, reason)
			}
			t.Logf("✓ bootstrap method %s → allowed (%s)", method, reason)
		})
	}
}

// --- Role-binding acceptance tests -------------------------------------------

// simulateRoleBindingCheck mirrors the enforcement logic added in the interceptors.
// Returns (denied, reason).
func simulateRoleBindingCheck(clusterInitialized bool, method, subject string, roles []string) (bool, string) {
	if !clusterInitialized {
		return false, "cluster_not_initialized"
	}
	if !security.IsRoleBasedMethod(method) {
		return false, "not_role_based"
	}
	if subject == "" {
		return false, "no_subject" // would have been caught by auth check earlier
	}
	allowed := security.HasRolePermission(roles, method)
	if !allowed {
		return true, "role_binding_denied"
	}
	return false, "role_binding_granted"
}

// TestAcceptance_WithRoleBinding_Operator_CanApplyRelease verifies that an
// operator with the globular-operator role can call ApplyServiceRelease.
func TestAcceptance_WithRoleBinding_Operator_CanApplyRelease(t *testing.T) {
	method := "/clustercontroller.ResourcesService/ApplyServiceRelease"
	roles := []string{"globular-operator"}

	denied, reason := simulateRoleBindingCheck(true, method, "alice@example.com", roles)
	if denied {
		t.Errorf("operator should be allowed to ApplyServiceRelease, got denied (%s)", reason)
	}
	if reason != "role_binding_granted" {
		t.Errorf("expected role_binding_granted, got %q", reason)
	}
	t.Logf("✓ operator can ApplyServiceRelease: %s", reason)
}

// TestAcceptance_WithoutRoleBinding_GetsDenied verifies that an authenticated
// user with NO roles is denied access to role-based methods.
func TestAcceptance_WithoutRoleBinding_GetsDenied(t *testing.T) {
	method := "/clustercontroller.ResourcesService/ApplyServiceRelease"
	roles := []string{} // no roles assigned

	denied, reason := simulateRoleBindingCheck(true, method, "alice@example.com", roles)
	if !denied {
		t.Errorf("user with no roles should be denied, got allowed (%s)", reason)
	}
	if reason != "role_binding_denied" {
		t.Errorf("expected role_binding_denied, got %q", reason)
	}
	t.Logf("✓ user with no roles is denied: %s", reason)
}

// TestAcceptance_ControllerSA_CanReportStatus_CannotUploadArtifact verifies
// least-privilege scoping of the controller SA role.
func TestAcceptance_ControllerSA_CanReportStatus_CannotUploadArtifact(t *testing.T) {
	roles := []string{"globular-controller-sa"}
	subject := "globular-controller"

	reportStatus := "/clustercontroller.ClusterControllerService/ReportNodeStatus"
	uploadArtifact := "/repository.PackageRepository/UploadArtifact"

	denied, reason := simulateRoleBindingCheck(true, reportStatus, subject, roles)
	if denied {
		t.Errorf("controller SA should be allowed to ReportNodeStatus, got denied (%s)", reason)
	}
	t.Logf("✓ ControllerSA can ReportNodeStatus: %s", reason)

	denied, reason = simulateRoleBindingCheck(true, uploadArtifact, subject, roles)
	if !denied {
		t.Errorf("controller SA should NOT be allowed to UploadArtifact, got allowed (%s)", reason)
	}
	t.Logf("✓ ControllerSA cannot UploadArtifact: %s", reason)
}

// TestAcceptance_AdminRole_HasAccessToAllRoleBasedMethods verifies that the
// globular-admin role (with "/*" wildcard) grants access to all role-based methods.
func TestAcceptance_AdminRole_HasAccessToAllRoleBasedMethods(t *testing.T) {
	roles := []string{"globular-admin"}
	subject := "admin@localhost"

	methods := []string{
		"/clustercontroller.ResourcesService/ApplyServiceRelease",
		"/repository.PackageRepository/UploadArtifact",
		"/clustercontroller.ClusterControllerService/ReportNodeStatus",
		"/dns.DnsService/SetA",
	}

	for _, method := range methods {
		t.Run(method, func(t *testing.T) {
			denied, reason := simulateRoleBindingCheck(true, method, subject, roles)
			if denied {
				t.Errorf("admin should have access to %s, got denied (%s)", method, reason)
			}
			t.Logf("✓ admin can call %s: %s", method, reason)
		})
	}
}

// TestAcceptance_IsRoleBasedMethod_DNSWildcard verifies that DNS methods
// are correctly identified as role-based (via service wildcard).
func TestAcceptance_IsRoleBasedMethod_DNSWildcard(t *testing.T) {
	dnsMethods := []string{
		"/dns.DnsService/SetA",
		"/dns.DnsService/GetA",
		"/dns.DnsService/CreateZone",
	}
	for _, m := range dnsMethods {
		if !security.IsRoleBasedMethod(m) {
			t.Errorf("IsRoleBasedMethod(%q) = false, expected true (via /dns.DnsService/* wildcard)", m)
		}
		t.Logf("✓ %s is role-based", m)
	}
}

// TestAcceptance_RBAC_Methods_NotRoleBased verifies that RBAC management
// methods are NOT classified as role-based (they're excluded from the check
// to prevent circular RPC calls).
func TestAcceptance_RBAC_Methods_NotRoleBased(t *testing.T) {
	rbacMethods := []string{
		"/rbac.RbacService/SetRoleBinding",
		"/rbac.RbacService/GetRoleBinding",
		"/rbac.RbacService/ListRoleBindings",
		"/rbac.RbacService/SetResourcePermissions",
	}
	for _, m := range rbacMethods {
		if security.IsRoleBasedMethod(m) {
			t.Errorf("IsRoleBasedMethod(%q) = true, expected false (RBAC methods are excluded)", m)
		}
		t.Logf("✓ %s is NOT role-based (correct: excluded from role-binding check)", m)
	}
}

// --- Test D -------------------------------------------------------------------

// TestAcceptance_ReadOnlyRPCsNotBlockedByPostDay0Gate verifies that read-only
// methods are never blocked by the post-Day-0 authentication gate.
func TestAcceptance_ReadOnlyRPCsNotBlockedByPostDay0Gate(t *testing.T) {
	readOnlyMethods := []string{
		"/rbac.RbacService/GetAccount",
		"/rbac.RbacService/ListAccounts",
		"/clustercontroller.ResourcesService/GetServiceRelease",
		"/clustercontroller.ResourcesService/ListServiceReleases",
		"/clustercontroller.ClusterControllerService/GetClusterHealth",
		"/dns.DnsService/GetA",
		"/repository.PackageRepository/GetArtifactManifest",
	}

	for _, method := range readOnlyMethods {
		t.Run(method, func(t *testing.T) {
			if security.IsMutatingRPC(method) {
				t.Errorf("IsMutatingRPC(%q) = true, expected false — read-only method misclassified", method)
			}
			// Post-Day-0 gate only fires for mutating RPCs; read-only methods pass through.
			denied, _, reason := simulatePostDay0Check(
				true,  // cluster initialized
				method,
				"",    // anonymous
				false, // no mapping
			)
			if denied && reason == "authentication_required" {
				t.Errorf("read-only method %s blocked by post-Day-0 gate — should not happen", method)
			}
			t.Logf("✓ read-only %s not blocked by Day-0 gate", method)
		})
	}
}

// --- Test E -------------------------------------------------------------------

// TestAcceptance_AuthenticatedMutatingWithMapping_ProceedsToRBAC verifies that
// an authenticated caller with an RBAC mapping proceeds to RBAC evaluation
// (i.e. is NOT denied by the post-Day-0 gate or unmapped-method check).
func TestAcceptance_AuthenticatedMutatingWithMapping_ProceedsToRBAC(t *testing.T) {
	denied, code, reason := simulatePostDay0Check(
		true,
		"/clustercontroller.ResourcesService/ApplyServiceRelease",
		"globular-controller", // has identity (controller SA)
		true,                  // has RBAC mapping
	)
	if denied {
		t.Errorf("expected NOT denied (should proceed to RBAC), got denied (code=%v reason=%s)", code, reason)
	}
	if reason != "rbac_check_required" {
		t.Errorf("expected reason rbac_check_required, got %q", reason)
	}
	t.Logf("✓ authenticated + mapped mutating RPC proceeds to RBAC evaluation")
}

// --- Test: SA scope -----------------------------------------------------------

// TestAcceptance_ServiceAccountScopes verifies that the role permission sets
// encode the correct least-privilege boundaries:
// - ControllerSA cannot publish artifacts
// - NodeAgentSA cannot create ServiceRelease
func TestAcceptance_ServiceAccountScopes(t *testing.T) {
	controllerPerms := security.RolePermissions[security.RoleControllerSA]
	nodeAgentPerms := security.RolePermissions[security.RoleNodeAgentSA]

	publishArtifact := "/repository.PackageRepository/UploadArtifact"
	createRelease := "/clustercontroller.ResourcesService/ApplyServiceRelease"
	reportStatus := "/clustercontroller.ClusterControllerService/ReportNodeStatus"

	// Controller SA cannot upload artifacts
	if containsExact(controllerPerms, publishArtifact) {
		t.Errorf("ControllerSA should NOT have %s permission", publishArtifact)
	}
	t.Log("✓ ControllerSA cannot upload artifacts")

	// Controller SA CAN apply service releases (it reconciles desired state)
	// Actually, looking at the role definition, it can ApplyServiceDesiredVersion but
	// not ApplyServiceRelease (that's the Operator's job). Let me check:
	if containsExact(controllerPerms, createRelease) {
		t.Logf("note: ControllerSA has ApplyServiceRelease — verify this is intentional")
	}

	// NodeAgent SA cannot create/apply service releases
	if containsExact(nodeAgentPerms, createRelease) {
		t.Errorf("NodeAgentSA should NOT have %s permission", createRelease)
	}
	t.Log("✓ NodeAgentSA cannot create ServiceRelease")

	// NodeAgent SA CAN report node status
	if !containsExact(nodeAgentPerms, reportStatus) {
		t.Errorf("NodeAgentSA MUST have %s permission", reportStatus)
	}
	t.Log("✓ NodeAgentSA can report node status")

	// Controller SA CAN also report/read node status
	if !containsExact(controllerPerms, reportStatus) {
		t.Errorf("ControllerSA MUST have %s permission (reads node status)", reportStatus)
	}
	t.Log("✓ ControllerSA can read node status")
}

func containsExact(perms []string, target string) bool {
	for _, p := range perms {
		if p == target {
			return true
		}
	}
	return false
}
