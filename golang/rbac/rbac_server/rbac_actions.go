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
	// Exception
	if len(resources) == 0 {
		if strings.HasPrefix(action, "/echo.EchoService") ||
			strings.HasPrefix(action, "/resource.ResourceService") ||
			strings.HasPrefix(action, "/event.EventService") ||
			action == "/file.FileService/GetFileInfo" {
			return true, false, nil
		}
	}


	// test if the subject exist.
	subject, err := srv.validateSubject(subject, subjectType)
	if err != nil {
		return false, false, err
	}

	var actions []string

	// Validate the access for a given suject...
	hasAccess := false

	// So first of all I will validate the actions itself...
	if subjectType == rbacpb.SubjectType_APPLICATION {
		//srv.logServiceInfo("", Utility.FileLine(), Utility.FunctionName(), "validate action "+action+" for application "+subject)

		application, err := srv.getApplication(subject)
		if err != nil {

			return false, false, err
		}

		actions = application.Actions

	} else if subjectType == rbacpb.SubjectType_PEER {
		//srv.logServiceInfo("", Utility.FileLine(), Utility.FunctionName(), "validate action "+action+" for peer "+subject)
		peer, err := srv.getPeer(subject)
		if err != nil {
			return false, false, err
		}
		actions = peer.Actions

	} else if subjectType == rbacpb.SubjectType_ROLE {
		//srv.logServiceInfo("", Utility.FileLine(), Utility.FunctionName(), "validate action "+action+" for role "+subject)
		role, err := srv.getRole(subject)
		if err != nil {
			return false, false, err
		}

		// If the role is sa then I will it has all permission...
		domain, _ := config.GetDomain()
		if role.Domain == domain && role.Name == "admin" {
			return true, false, nil
		}

		actions = role.Actions

	} else if subjectType == rbacpb.SubjectType_ACCOUNT {
		//srv.logServiceInfo("", Utility.FileLine(), Utility.FunctionName(), "validate action "+action+" for account "+subject)
		// If the user is the super admin i will return true.
		if subject == "sa@"+srv.Domain {
			return true, false, nil
		}

		account, err := srv.getAccount(subject)
		if err != nil {
			return false, false, err
		}

		// call the rpc method.
		if account.Roles != nil {

			for i := range account.Roles {
				roleId := account.Roles[i]

				// Here I will add the domain to the role id if it's not already set.
				if !strings.Contains(roleId, "@") {
					roleId = roleId + "@" + srv.Domain
				}

				// if the role id is local admin
				if roleId == "admin@"+srv.Domain {
					return true, false, nil
				} else if strings.HasSuffix(roleId, "@"+srv.Domain) {
					hasAccess, _, _ = srv.validateAction(action, roleId, rbacpb.SubjectType_ROLE, resources)
					if hasAccess {
						break
					}
				}

			}
		}

		// Validate external account with local roles....
		if !hasAccess {
			roles, err := srv.getRoles()
			if err == nil {
				for i := range roles {
					roleId := roles[i].Id + "@" + roles[i].Domain

					if Utility.Contains(roles[i].Members, subject) {

						// if the role id is local admin
						hasAccess, _, _ = srv.validateAction(action, roleId, rbacpb.SubjectType_ROLE, resources)
						if hasAccess {
							break
						}
					}
				}
			}
		}
	}

	if !hasAccess {
		if actions != nil {
			for i := 0; i < len(actions) && !hasAccess; i++ {
				if actions[i] == action {
					hasAccess = true
					break
				}
			}
		}
	}

	if !hasAccess {
		return false, true, nil
	} else if subjectType == rbacpb.SubjectType_ROLE {
		// I will not validate the resource access for the role only the method.
		return true, false, nil
	}

	// Now I will validate the resource access infos
	permissions_, _ := srv.getActionResourcesPermissions(action)
	if len(resources) > 0 {
		if permissions_ == nil {
			err := errors.New("no resources path are given for validations")
			return false, false, err
		}
		for i := range resources {
			if len(resources[i].Path) > 0 { // Here if the path is empty i will simply not validate it.
				hasAccess, accessDenied, err := srv.validateAccess(subject, subjectType, resources[i].Permission, resources[i].Path)
				if err != nil {
					return false, false, err
				}

				return hasAccess, accessDenied, nil
			}
		}
	}

	//srv.logServiceInfo("", Utility.FileLine(), Utility.FunctionName(), "subject "+subject+" can call the method '"+action)
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
