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
	"log/slog"
	"sync"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/security"
	"google.golang.org/grpc/metadata"
)

// roleBindingTTL controls how long a cached role binding is considered fresh.
// Short enough to pick up RBAC changes quickly, long enough to avoid
// hammering the RBAC service on every inbound request.
const roleBindingTTL = 30 * time.Second

// roleBindingEntry is a cached role binding result for a subject.
type roleBindingEntry struct {
	roles     []string
	expiresAt int64 // unix seconds
}

// roleBindingCache maps subject → cached roles + expiry.
var roleBindingCache sync.Map

// checkRoleBinding fetches the role binding for subject from the RBAC service
// at rbacAddr and checks whether any bound role grants access to method.
//
// Results are cached per-subject for roleBindingTTL to avoid flooding RBAC
// with repeated lookups for the same identity.
//
// If the RBAC call fails (network, auth, timeout), falls back to local
// cluster-roles: if ANY locally loaded role grants the method, access is
// allowed. This is more permissive than the RBAC service path, but it only
// activates when the RBAC service is unavailable.
func checkRoleBinding(subject, method, rbacAddr string) (bool, error) {
	// Check cache first.
	if roles, ok := getCachedRoleBinding(subject); ok {
		return security.HasRolePermission(roles, method), nil
	}

	rbacClient, err := GetRbacClient(rbacAddr)
	if err != nil {
		return checkLocalRoles(method), err
	}

	// Build a properly authenticated context so the RBAC service's interceptor
	// sees subject "sa" and grants access via the superadmin bypass.
	// Previously this only sent cluster_id, causing the RBAC call to arrive
	// unauthenticated — triggering a recursive auth failure pattern.
	ctx, cancel := context.WithTimeout(serviceCallContext(), 3*time.Second)
	defer cancel()

	binding, err := rbacClient.GetRoleBindingWithCtx(ctx, subject)
	if err != nil {
		// RBAC service unreachable or rejected the call.
		// Fall back to locally loaded cluster-roles.json.
		return checkLocalRoles(method), nil
	}

	roles := binding.GetRoles()
	putCachedRoleBinding(subject, roles)
	return security.HasRolePermission(roles, method), nil
}

// getCachedRoleBinding returns the cached roles for subject if still fresh.
func getCachedRoleBinding(subject string) ([]string, bool) {
	val, ok := roleBindingCache.Load(subject)
	if !ok {
		return nil, false
	}
	entry := val.(roleBindingEntry)
	if time.Now().Unix() > entry.expiresAt {
		roleBindingCache.Delete(subject)
		return nil, false
	}
	return entry.roles, true
}

// putCachedRoleBinding stores a role binding result with TTL.
func putCachedRoleBinding(subject string, roles []string) {
	roleBindingCache.Store(subject, roleBindingEntry{
		roles:     roles,
		expiresAt: time.Now().Unix() + int64(roleBindingTTL.Seconds()),
	})
}

// serviceCallContext builds an outgoing gRPC context with a fresh "sa" service
// token and cluster_id metadata — the same credentials that GetClientContext
// provides for normal service-to-service calls.
func serviceCallContext() context.Context {
	md := metadata.MD{}

	localMac, err := config.GetMacAddress()
	if err != nil {
		slog.Warn("serviceCallContext: local MAC lookup failed", "error", err)
	} else {
		token, err := security.GenerateServiceToken(localMac)
		if err != nil {
			slog.Warn("serviceCallContext: service token generation failed", "error", err)
		} else {
			md.Set("token", token)
			md.Set("authorization", "Bearer "+token)
			md.Set("mac", localMac)
		}
	}

	clusterID, _ := security.GetLocalClusterID()
	if clusterID != "" {
		md.Set("cluster_id", clusterID)
	}

	return metadata.NewOutgoingContext(context.Background(), md)
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
