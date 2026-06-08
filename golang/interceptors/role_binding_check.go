// @awareness namespace=globular.platform
// @awareness component=platform.interceptors.role_binding
// @awareness file_role=cached_role_binding_check_with_local_cluster_roles_fallback
// @awareness implements=globular.platform:intent.interceptors.role_binding_fallback_uses_local_cluster_roles
// @awareness implements=globular.platform:intent.rbac.service_excludes_self_from_interceptor
// @awareness relates_to=globular.platform:invariant.meta.fail_safe_defaults_when_authority_is_uncertain
// (relationship: KNOWN PARTIAL VIOLATION — annotated as relates_to because the
//  scanner vocabulary has no typed "partially_violates" relation yet; the
//  prose comment below carries the operative signal.)
// @awareness risk=high
//
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
//
// KNOWN GAP — partial violation of
// meta.fail_safe_defaults_when_authority_is_uncertain. The fallback uses
// checkLocalRoles, which returns ALLOW if ANY locally-loaded role grants
// the method — so a viewer-only user can clear an admin-only check during
// an RBAC outage. The bootstrap-deadlock argument justifies the relaxation
// during the bootstrap window; it does NOT justify it during normal
// operation. Tracked as the closest violation the principle has — the
// structural fix is either (a) a fail-closed mode gated by a real
// bootstrap-window check, or (b) a per-method "fallback_safe" allowlist
// in cluster-roles.json restricting the fallback to read-only operations.
// Until the structural fix lands, every fallback fires a metric and a
// WARN log so operators can detect and audit the relaxation window.

package interceptors

import (
	"context"
	"expvar"
	"log/slog"
	"sync"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/security"
	"google.golang.org/grpc/metadata"
)

// rbacFallbackCount counts how often the local-roles fallback fired because
// the RBAC service was unreachable. Surfaces the relaxation window for
// operators and the security team. Every increment means a request was
// authorized by checkLocalRoles instead of the user's actual role binding —
// a structural permission relaxation that should be near-zero in steady state.
var rbacFallbackCount = expvar.NewInt("rbac.fallback_local_roles_used")

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
//
func checkRoleBinding(subject, method, rbacAddr string) (bool, error) {
	// Check cache first.
	if roles, ok := getCachedRoleBinding(subject); ok {
		return security.HasRolePermission(roles, method), nil
	}

	rbacClient, err := GetRbacClient(rbacAddr)
	if err != nil {
		// RBAC client construction failed (mTLS not ready, DNS not
		// resolved, etc.). Activate local-roles fallback — see
		// KNOWN GAP at top of file. Error is preserved AND surfaced
		// via metric + WARN log so the relaxation is auditable.
		rbacFallbackCount.Add(1)
		slog.Warn("rbac fallback: RBAC client unavailable, using local-roles fallback (permissive)",
			"subject", subject, "method", method, "reason", "client_construct_failed", "error", err)
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
		// Fall back to locally loaded cluster-roles.json — see
		// KNOWN GAP at top of file. The error is now PROPAGATED to
		// the caller (was previously swallowed as nil) so the
		// caller can detect the relaxation and adjust its own
		// decision logic. The metric + WARN log surfaces the same
		// signal to ops/security.
		rbacFallbackCount.Add(1)
		slog.Warn("rbac fallback: RBAC lookup failed, using local-roles fallback (permissive)",
			"subject", subject, "method", method, "reason", "get_role_binding_failed", "error", err)
		return checkLocalRoles(method), err
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
