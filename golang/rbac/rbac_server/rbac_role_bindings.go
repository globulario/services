// rbac_role_bindings.go: SetRoleBinding / GetRoleBinding / ListRoleBindings handlers.
//
// Storage layout:
//   ROLE_BINDINGS/<subject>  →  JSON array of role-name strings
//
// Access control (v1 — handler-level, bypasses interceptor which skips /rbac.RbacService/*):
//   SetRoleBinding   — globular-admin role required (bootstrap exempt)
//   GetRoleBinding   — globular-admin OR self-read (bootstrap exempt)
//   ListRoleBindings — globular-admin role required (bootstrap exempt)

package main

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/security"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const roleBindingPrefix = "ROLE_BINDINGS/"

// callerIsAdmin reports whether subject holds the globular-admin role.
// Reads directly from storage to avoid a circular call through GetRoleBinding.
func (srv *server) callerIsAdmin(subject string) (bool, error) {
	data, err := srv.getItem(roleBindingPrefix + subject)
	if err != nil {
		if strings.Contains(err.Error(), "item not found") || strings.Contains(err.Error(), "Key not found") {
			return false, nil
		}
		return false, err
	}
	var roles []string
	if err := json.Unmarshal(data, &roles); err != nil {
		return false, err
	}
	// globular-admin has "/*" which HasRolePermission matches for any method.
	return security.HasRolePermission(roles, "/*"), nil
}

// requireAdmin returns nil if the caller is allowed to manage role bindings:
//   - during bootstrap (gate already enforces loopback + 30-min window + allowlist)
//   - OR the caller holds globular-admin role
func (srv *server) requireAdmin(ctx context.Context) error {
	authCtx := security.FromContext(ctx)

	// Bootstrap: gate has already validated loopback + time window + allowlist.
	if authCtx != nil && authCtx.IsBootstrap {
		return nil
	}

	if authCtx == nil || authCtx.Subject == "" {
		return status.Error(codes.Unauthenticated,
			"authentication required to manage role bindings")
	}

	ok, err := srv.callerIsAdmin(authCtx.Subject)
	if err != nil {
		return status.Errorf(codes.Internal, "role lookup failed: %v", err)
	}
	if !ok {
		return status.Errorf(codes.PermissionDenied,
			"permission denied: globular-admin role required to manage role bindings (caller: %s)",
			authCtx.Subject)
	}
	return nil
}

// SetRoleBinding creates or replaces the role binding for a subject.
// Requires globular-admin role (or bootstrap mode).
func (srv *server) SetRoleBinding(ctx context.Context, rqst *rbacpb.SetRoleBindingRqst) (*rbacpb.SetRoleBindingRsp, error) {
	if err := srv.requireAdmin(ctx); err != nil {
		return nil, err
	}

	if rqst.GetBinding() == nil || rqst.GetBinding().GetSubject() == "" {
		return nil, status.Error(codes.InvalidArgument, "binding.subject must not be empty")
	}

	subject := rqst.GetBinding().GetSubject()
	roles := rqst.GetBinding().GetRoles()
	if roles == nil {
		roles = []string{}
	}

	data, err := json.Marshal(roles)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s",
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if err := srv.setItem(roleBindingPrefix+subject, data); err != nil {
		return nil, status.Errorf(codes.Internal, "%s",
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &rbacpb.SetRoleBindingRsp{}, nil
}

// GetRoleBinding fetches the role binding for a subject.
// Returns an empty binding (not an error) if no binding exists.
// Requires globular-admin role OR the caller is reading their own binding.
func (srv *server) GetRoleBinding(ctx context.Context, rqst *rbacpb.GetRoleBindingRqst) (*rbacpb.GetRoleBindingRsp, error) {
	if rqst.GetSubject() == "" {
		return nil, status.Error(codes.InvalidArgument, "subject must not be empty")
	}

	authCtx := security.FromContext(ctx)
	if authCtx == nil || !authCtx.IsBootstrap {
		if authCtx == nil || authCtx.Subject == "" {
			return nil, status.Error(codes.Unauthenticated,
				"authentication required to read role bindings")
		}
		// Allow self-read; otherwise require admin.
		if authCtx.Subject != rqst.GetSubject() {
			ok, err := srv.callerIsAdmin(authCtx.Subject)
			if err != nil {
				return nil, status.Errorf(codes.Internal, "role lookup failed: %v", err)
			}
			if !ok {
				return nil, status.Errorf(codes.PermissionDenied,
					"permission denied: globular-admin role required to read other subjects' bindings (caller: %s)",
					authCtx.Subject)
			}
		}
	}

	data, err := srv.getItem(roleBindingPrefix + rqst.GetSubject())
	if err != nil {
		if strings.Contains(err.Error(), "item not found") || strings.Contains(err.Error(), "Key not found") {
			// No binding stored — return empty (not an error)
			return &rbacpb.GetRoleBindingRsp{
				Binding: &rbacpb.RoleBinding{Subject: rqst.GetSubject(), Roles: []string{}},
			}, nil
		}
		return nil, status.Errorf(codes.Internal, "%s",
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	var roles []string
	if err := json.Unmarshal(data, &roles); err != nil {
		return nil, status.Errorf(codes.Internal, "%s",
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &rbacpb.GetRoleBindingRsp{
		Binding: &rbacpb.RoleBinding{
			Subject: rqst.GetSubject(),
			Roles:   roles,
		},
	}, nil
}

// ListRoleBindings streams all stored role bindings.
// Requires globular-admin role (or bootstrap mode).
func (srv *server) ListRoleBindings(rqst *rbacpb.ListRoleBindingsRqst, stream rbacpb.RbacService_ListRoleBindingsServer) error {
	if err := srv.requireAdmin(stream.Context()); err != nil {
		return err
	}

	keys, err := srv.permissions.GetAllKeys()
	if err != nil {
		return status.Errorf(codes.Internal, "%s",
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	for _, key := range keys {
		if !strings.HasPrefix(key, roleBindingPrefix) {
			continue
		}
		subject := strings.TrimPrefix(key, roleBindingPrefix)

		data, err := srv.getItem(key)
		if err != nil {
			continue // skip corrupted entries
		}

		var roles []string
		if err := json.Unmarshal(data, &roles); err != nil {
			continue // skip corrupted entries
		}

		if err := stream.Send(&rbacpb.ListRoleBindingsRsp{
			Binding: &rbacpb.RoleBinding{Subject: subject, Roles: roles},
		}); err != nil {
			return err
		}
	}

	return nil
}
