// rbac_actions.go: action-level permissioning and resource validation.

package main

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/rbac/rbacpb"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

/*
* Set action permissions.
When gRPC service methode are called they must validate the resource pass in parameters.
So each service is reponsible to give access permissions requirement.
*/
func (srv *server) setActionResourcesPermissions(permissions map[string]interface{}) error {

	// So here I will keep values in local storage.cap()
	data, err := json.Marshal(permissions["resources"])
	if err != nil {
		return err
	}

	return srv.setItem(permissions["action"].(string), data)
}

// SetActionResourcesPermissions sets the permissions for resources associated with a specific action.
// It receives a request containing a map of permissions and applies them using the internal
// setActionResourcesPermissions method. Returns an empty response on success or an error if the operation fails.
//
// Parameters:
//   ctx - The context for the request, used for cancellation and deadlines.
//   rqst - The request containing the permissions to set.
//
// Returns:
//   *rbacpb.SetActionResourcesPermissionsRsp - The response indicating success.
//   error - An error if the operation fails.
func (srv *server) SetActionResourcesPermissions(ctx context.Context, rqst *rbacpb.SetActionResourcesPermissionsRqst) (*rbacpb.SetActionResourcesPermissionsRsp, error) {

	err := srv.setActionResourcesPermissions(rqst.Permissions.AsMap())
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &rbacpb.SetActionResourcesPermissionsRsp{}, nil
}

func (srv *server) getActionResourcesPermissions(action string) ([]*rbacpb.ResourceInfos, error) {

	if len(action) == 0 {
		return nil, errors.New("no action given")
	}
	data, err := srv.getItem(action)
	infos_ := make([]*rbacpb.ResourceInfos, 0)
	if err != nil {
		if !strings.Contains(err.Error(), "item not found") || strings.Contains(err.Error(), "Key not found") {
			return nil, err
		} else {
			// no infos_ found...
			return infos_, nil
		}
	}
	infos := make([]interface{}, 0)
	err = json.Unmarshal(data, &infos)

	for i := range infos {
		info := infos[i].(map[string]interface{})
		field := ""
		if info["field"] != nil {
			field = info["field"].(string)
		}
		infos_ = append(infos_, &rbacpb.ResourceInfos{Index: int32(Utility.ToInt(info["index"])), Permission: info["permission"].(string), Field: field})
	}

	return infos_, err
}

// GetActionResourceInfos retrieves information about resources and their permissions associated with a specific action.
// It takes a context and a request containing the action name, and returns a response with resource info or an error.
// In case of failure, it returns a gRPC internal error with detailed information.
func (srv *server) GetActionResourceInfos(ctx context.Context, rqst *rbacpb.GetActionResourceInfosRqst) (*rbacpb.GetActionResourceInfosRsp, error) {
	infos, err := srv.getActionResourcesPermissions(rqst.Action)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &rbacpb.GetActionResourceInfosRsp{Infos: infos}, nil
}

func (srv *server) validateAction(action string, subject string, subjectType rbacpb.SubjectType, resources []*rbacpb.ResourceInfos) (bool, bool, error) {
	// ---------------------------------------------------------------
	// Exceptions: allow a few infra calls when no resources provided
	// ---------------------------------------------------------------
	if len(resources) == 0 {
		if strings.HasPrefix(action, "/echo.EchoService") ||
			strings.HasPrefix(action, "/resource.ResourceService") ||
			strings.HasPrefix(action, "/event.EventService") ||
			action == "/file.FileService/GetFileInfo" {
			return true, false, nil
		}
	}

	// Validate/normalize subject (e.g., ensure it exists; may return canonical id)
	subject, err := srv.validateSubject(subject, subjectType)
	if err != nil {
		return false, false, err
	}

	// Helper: ensure id is fully-qualified with domain if missing
	withDomain := func(id string, domain string) string {
		if id == "" || strings.Contains(id, "@") {
			return id
		}
		return id + "@" + domain
	}

	var actions []string
	hasAccess := false

	switch subjectType {
	case rbacpb.SubjectType_APPLICATION:
		app, err := srv.getApplication(subject)
		if err != nil {
			return false, false, err
		}
		actions = app.Actions

	case rbacpb.SubjectType_PEER:
		peer, err := srv.getPeer(subject)
		if err != nil {
			return false, false, err
		}
		actions = peer.Actions

	case rbacpb.SubjectType_ROLE:
		role, err := srv.getRole(subject)
		if err != nil {
			return false, false, err
		}
		// Local "admin" role → full access
		domain, _ := config.GetDomain()
		if role.Domain == domain && role.Name == "admin" {
			return true, false, nil
		}
		actions = role.Actions

	case rbacpb.SubjectType_ACCOUNT:
		// Super-admin account → full access
		if subject == "sa@"+srv.Domain {
			return true, false, nil
		}

		account, err := srv.getAccount(subject)
		if err != nil {
			return false, false, err
		}

		// -----------------------------------------------------------
		// (A) Direct Account → Role assignments (unchanged)
		// -----------------------------------------------------------
		if account.Roles != nil {
			for _, rid := range account.Roles {
				roleId := withDomain(rid, srv.Domain)

				// Local admin role → full access
				if roleId == "admin@"+srv.Domain {
					return true, false, nil
				}

				// Only recurse for local roles (keep previous semantics)
				if strings.HasSuffix(roleId, "@"+srv.Domain) {
					ok, _, _ := srv.validateAction(action, roleId, rbacpb.SubjectType_ROLE, resources)
					if ok {
						hasAccess = true
						break
					}
				}
			}
		}

		// -----------------------------------------------------------
		// (B) External account that local roles list in Role.Accounts
		//     (existing behavior retained)
		// -----------------------------------------------------------
		if !hasAccess {
			if roles, err := srv.getRoles(); err == nil {
				for i := range roles {
					roleFQN := roles[i].Id + "@" + roles[i].Domain
					if Utility.Contains(roles[i].Accounts, subject) {
						ok, _, _ := srv.validateAction(action, roleFQN, rbacpb.SubjectType_ROLE, resources)
						if ok {
							hasAccess = true
							break
						}
					}
				}
			}
		}

		// -----------------------------------------------------------
		// (C) NEW: Account → Groups → Roles
		//     Group now contains Roles []string; if the account is in
		//     a group, grant via any group role.
		// -----------------------------------------------------------
		if !hasAccess {
			// 1) Collect groups for this account
			var accountGroups []struct {
				Id     string
				Domain string
				Roles  []string
			}

			// Prefer a direct membership list if your Account model has it (account.Groups).
			// If not, derive via srv.getGroups().
			haveGroups := false
			if len(account.Groups) > 0 {
				haveGroups = true
			}

			if haveGroups {
				// We need group domains & roles; fetch groups and filter to those ids.
				if groups, err := srv.getGroups(); err == nil {
					for _, g := range groups {
						// accept both raw id and fqdn
						if Utility.Contains(account.Groups, g.Id) ||
							Utility.Contains(account.Groups, withDomain(g.Id, g.Domain)) {
							accountGroups = append(accountGroups, struct {
								Id     string
								Domain string
								Roles  []string
							}{Id: g.Id, Domain: g.Domain, Roles: g.Roles})
						}
					}
				}
			} else {
				// Derive by scanning groups to find membership (Members/Accounts fields)
				if groups, err := srv.getGroups(); err == nil {
					for _, g := range groups {
						if Utility.Contains(g.Accounts, subject) {
							accountGroups = append(accountGroups, struct {
								Id     string
								Domain string
								Roles  []string
							}{Id: g.Id, Domain: g.Domain, Roles: g.Roles})
						}
					}
				}
			}

			// 2) From groups, walk their roles and validate
			if len(accountGroups) > 0 {
				seen := make(map[string]struct{})
				for _, ag := range accountGroups {
					for _, rid := range ag.Roles {
						roleFQN := withDomain(rid, ag.Domain)
						if _, done := seen[roleFQN]; done {
							continue
						}
						seen[roleFQN] = struct{}{}

						ok, _, _ := srv.validateAction(action, roleFQN, rbacpb.SubjectType_ROLE, resources)
						if ok {
							hasAccess = true
							break
						}
					}
					if hasAccess {
						break
					}
				}
			}
		}
	}

	// If still not granted, check the subject's own action list
	if !hasAccess && actions != nil {
		for _, a := range actions {
			if a == action {
				hasAccess = true
				break
			}
		}
	}

	// If method not granted, return early (resource checks not needed)
	if !hasAccess {
		return false, true, nil
	}

	// Roles: method-level only (resource checks happen at the caller’s subject)
	if subjectType == rbacpb.SubjectType_ROLE {
		return true, false, nil
	}

	// -----------------------------------------------------------
	// Resource-level checks (if resources provided)
	// -----------------------------------------------------------
	permissions_, _ := srv.getActionResourcesPermissions(action)
	if len(resources) > 0 {
		if permissions_ == nil {
			return false, false, errors.New("no resources path are given for validations")
		}
		for i := range resources {
			if len(resources[i].Path) > 0 { // empty path => skip validation
				ok, accessDenied, err := srv.validateAccess(subject, subjectType, resources[i].Permission, resources[i].Path)
				if err != nil {
					return false, false, err
				}
				return ok, accessDenied, nil
			}
		}
	}

	// Granted: method ok, no resource constraints to enforce
	return true, false, nil
}

// ValidateAction validates whether a given subject is allowed to perform a specified action.
// It checks the request for a valid action, and then delegates the permission check to validateAction.
// Returns a response indicating if access is granted and any access denial reasons.
// In case of errors (e.g., missing action or internal validation failure), returns a gRPC error.
func (srv *server) ValidateAction(ctx context.Context, rqst *rbacpb.ValidateActionRqst) (*rbacpb.ValidateActionRsp, error) {

	// So here From the context I will validate if the application can execute the action...
	var err error
	if len(rqst.Action) == 0 {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no action was given to validate")))
	}

	// If the address is local I will give the permission.
	hasAccess, accessDenied, err := srv.validateAction(rqst.Action, rqst.Subject, rqst.Type, rqst.Infos)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &rbacpb.ValidateActionRsp{
		HasAccess:    hasAccess,
		AccessDenied: accessDenied,
	}, nil
}
