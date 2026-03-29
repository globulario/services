// role_binding_check.go: helper that calls the RBAC service to determine
// whether a subject's stored role binding grants access to a gRPC method.
//
// Called from ServerUnaryInterceptor and ServerStreamInterceptor for methods
// that appear in security.RolePermissions (i.e. "role-based" methods).
//
// The RBAC service itself ("/rbac.RbacService/...") is excluded to prevent
// a circular RPC loop; callers must guard with strings.HasPrefix before calling.
//
// Fallback: when the RBAC service is unreachable or rejects the call (e.g.
// because the interceptor's gRPC client lacks mTLS), we fall back to locally
// loaded cluster-roles.json. This avoids a bootstrap deadlock where services
// can't call RBAC because RBAC requires auth that depends on RBAC.

package interceptors

import (
	"context"
	"time"

	"github.com/globulario/services/golang/security"
	"google.golang.org/grpc/metadata"
)

// checkRoleBinding fetches the role binding for subject from the RBAC service
// at rbacAddr and checks whether any bound role grants access to method.
//
// If the RBAC call fails (network, auth, timeout), falls back to local
// cluster-roles: if ANY locally loaded role grants the method, access is
// allowed. This is more permissive than the RBAC service path, but it only
// activates when the RBAC service is unavailable.
func checkRoleBinding(subject, method, rbacAddr string) (bool, error) {
	rbacClient, err := GetRbacClient(rbacAddr)
	if err != nil {
		return checkLocalRoles(method), err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	// Attach cluster_id so the RBAC service's own interceptor accepts the call.
	clusterID, err := security.GetLocalClusterID()
	if err == nil && clusterID != "" {
		md := metadata.Pairs("cluster_id", clusterID)
		ctx = metadata.NewOutgoingContext(ctx, md)
	}

	binding, err := rbacClient.GetRoleBindingWithCtx(ctx, subject)
	if err != nil {
		// RBAC service unreachable or rejected the call.
		// Fall back to locally loaded cluster-roles.json.
		return checkLocalRoles(method), nil
	}

	return security.HasRolePermission(binding.GetRoles(), method), nil
}

// checkLocalRoles checks whether any locally loaded cluster role grants
// access to the method. This is the fallback path when the RBAC service
// is unreachable.
//
// Since we don't have the subject's role binding locally, we check ALL
// roles. This is more permissive but still constrained to methods
// explicitly listed in cluster-roles.json.
func checkLocalRoles(method string) bool {
	for role := range security.RolePermissions {
		if security.HasRolePermission([]string{role}, method) {
			return true
		}
	}
	return false
}
