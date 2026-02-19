// role_binding_check.go: helper that calls the RBAC service to determine
// whether a subject's stored role binding grants access to a gRPC method.
//
// Called from ServerUnaryInterceptor and ServerStreamInterceptor for methods
// that appear in security.RolePermissions (i.e. "role-based" methods).
//
// The RBAC service itself ("/rbac.RbacService/...") is excluded to prevent
// a circular RPC loop; callers must guard with strings.HasPrefix before calling.

package interceptors

import (
	"context"
	"time"

	"github.com/globulario/services/golang/security"
)

// checkRoleBinding fetches the role binding for subject from the RBAC service
// at rbacAddr and checks whether any bound role grants access to method.
// Returns (allowed, error). Errors are treated as "no access" (fail closed).
func checkRoleBinding(subject, method, rbacAddr string) (bool, error) {
	rbacClient, err := GetRbacClient(rbacAddr)
	if err != nil {
		return false, err
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	binding, err := rbacClient.GetRoleBindingWithCtx(ctx, subject)
	if err != nil {
		// No binding or RBAC unavailable → deny access (fail closed)
		return false, nil
	}

	return security.HasRolePermission(binding.GetRoles(), method), nil
}
