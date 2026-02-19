// rbac_role_bindings.go: SetRoleBinding / GetRoleBinding / ListRoleBindings handlers.
//
// Storage layout:
//   ROLE_BINDINGS/<subject>  →  JSON array of role-name strings
//
// These methods implement the v1 role-binding control plane.
// Management-method protection (requiring a role to call these) is a v1.1 item.

package main

import (
	"context"
	"encoding/json"
	"strings"

	"github.com/globulario/services/golang/rbac/rbacpb"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const roleBindingPrefix = "ROLE_BINDINGS/"

// SetRoleBinding creates or replaces the role binding for a subject.
func (srv *server) SetRoleBinding(ctx context.Context, rqst *rbacpb.SetRoleBindingRqst) (*rbacpb.SetRoleBindingRsp, error) {
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
func (srv *server) GetRoleBinding(ctx context.Context, rqst *rbacpb.GetRoleBindingRqst) (*rbacpb.GetRoleBindingRsp, error) {
	if rqst.GetSubject() == "" {
		return nil, status.Error(codes.InvalidArgument, "subject must not be empty")
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
func (srv *server) ListRoleBindings(rqst *rbacpb.ListRoleBindingsRqst, stream rbacpb.RbacService_ListRoleBindingsServer) error {
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
