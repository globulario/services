package main

import (
	"context"
	"errors"
	"strings"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/resource/resourcepb"
	"github.com/globulario/services/golang/security"
	Utility "github.com/globulario/utility"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

// AddAccountRole associates an account with a role within the current domain.
// It ensures that both the RoleId and AccountId contain the domain suffix, appending it if necessary.
// The method creates cross-references between the role and account using the persistence service.
// Upon successful association, it publishes an update event for the role.
// Returns a response indicating the result of the operation or an error if the association fails.
func (srv *server) AddAccountRole(ctx context.Context, rqst *resourcepb.AddAccountRoleRqst) (*resourcepb.AddAccountRoleRsp, error) {

	if !strings.Contains(rqst.RoleId, "@") {
		rqst.RoleId = rqst.RoleId + "@" + srv.Domain
	}

	if !strings.Contains(rqst.AccountId, "@") {
		rqst.AccountId = rqst.AccountId + "@" + srv.Domain
	}

	// That service made user of persistence service.
	err := srv.createCrossReferences(rqst.RoleId, "Roles", "members", rqst.AccountId, "Accounts", "roles")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	srv.publishEvent("update_role_"+rqst.RoleId+"_evt", []byte{}, srv.Address)

	return &resourcepb.AddAccountRoleRsp{Result: true}, nil
}

func (srv *server) AddOrganizationRole(ctx context.Context, rqst *resourcepb.AddOrganizationRoleRqst) (*resourcepb.AddOrganizationRoleRsp, error) {

	if !strings.Contains(rqst.RoleId, "@") {
		rqst.RoleId += "@" + srv.Domain
	}

	if !strings.Contains(rqst.OrganizationId, "@") {
		rqst.OrganizationId += "@" + srv.Domain
	}

	err := srv.createCrossReferences(rqst.OrganizationId, "Organizations", "roles", rqst.RoleId, "Roles", "organizations")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	srv.publishEvent("update_organization_"+rqst.OrganizationId+"_evt", []byte{}, srv.Address)
	srv.publishEvent("update_organization_"+rqst.OrganizationId+"_evt", []byte{}, srv.Address)

	return &resourcepb.AddOrganizationRoleRsp{Result: true}, nil
}

// AddRoleActions adds one or more actions to a role identified by RoleId.
// It first checks if the RoleId contains a domain and validates that the domain matches the local domain.
// The method retrieves the role from the persistence store, and if the role does not have any actions,
// it initializes the actions with the provided list. Otherwise, it appends new actions that do not already exist.
// If any changes are made, the role is updated in the persistence store.
// An event is published to notify about the role update.
// Returns a response indicating success or an error if any operation fails.
func (srv *server) AddRoleActions(ctx context.Context, rqst *resourcepb.AddRoleActionsRqst) (*resourcepb.AddRoleActionsRsp, error) {
	roleId := rqst.RoleId
	localDomain, err := config.GetDomain()

	if strings.Contains(roleId, "@") {
		domain := strings.Split(roleId, "@")[1]
		roleId = strings.Split(roleId, "@")[0]

		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		if localDomain != domain {

			return nil, status.Errorf(
				codes.Internal,
				"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("cannot delete object from domain "+domain+" from domain "+localDomain)))
		}
	}

	// That service made user of persistence service.
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	q := `{"_id":"` + roleId + `"}`

	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Roles", q, ``)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	role := values.(map[string]interface{})

	needSave := false
	if role["actions"] == nil {
		role["actions"] = rqst.Actions
		needSave = true
	} else {
		var actions []interface{}
		switch role["actions"].(type) {
		case primitive.A:
			actions = []interface{}(role["actions"].(primitive.A))
		case []interface{}:
			actions = []interface{}(role["actions"].([]interface{}))
		default:
			logger.Warn("unknown type", "value", role["actions"])
		}

		for j := 0; j < len(rqst.Actions); j++ {
			exist := false
			for i := 0; i < len(actions); i++ {
				if actions[i].(string) == rqst.Actions[j] {
					exist = true
					break
				}
			}
			if !exist {
				// append only if not already there.
				actions = append(actions, rqst.Actions[j])
				needSave = true
			}
		}
		role["actions"] = actions
	}

	if needSave {

		// jsonStr, _ := Utility.ToJson(role)
		jsonStr := serialyseObject(role)

		err := p.ReplaceOne(context.Background(), "local_resource", "local_resource", "Roles", q, string(jsonStr), ``)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	srv.publishEvent("update_role_"+rqst.RoleId+"_evt", []byte{}, srv.Address)

	return &resourcepb.AddRoleActionsRsp{Result: true}, nil
}

// CreateRole handles the creation of a new role in the system.
// It retrieves the client ID from the context, creates the role using the persistence service,
// sets cross-references for members and organizations, publishes a creation event, and returns the result.
// Returns an error if any step fails.
//
// Parameters:
//   ctx - the context for the request, containing authentication and tracing information.
//   rqst - the request containing the role details to be created.
//
// Returns:
//   *resourcepb.CreateRoleRsp - the response indicating the result of the operation.
//   error - an error if the operation fails.
func (srv *server) CreateRole(ctx context.Context, rqst *resourcepb.CreateRoleRqst) (*resourcepb.CreateRoleRsp, error) {

	clientId, _, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}

	// That service made user of persistence service.
	err = srv.createRole(ctx, rqst.Role.Id, rqst.Role.Name, clientId, rqst.Role.Description, rqst.Role.Actions)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Now I will set the reference for

	// members...
	for i := 0; i < len(rqst.Role.Members); i++ {
		srv.createCrossReferences(rqst.Role.Members[i], "Accounts", "roles", rqst.Role.GetId()+"@"+rqst.Role.GetDomain(), "Roles", "members")
	}

	// Organizations
	for i := 0; i < len(rqst.Role.Organizations); i++ {
		srv.createCrossReferences(rqst.Role.Organizations[i], "Organizations", "roles", rqst.Role.GetId()+"@"+rqst.Role.GetDomain(), "Roles", "organizations")
	}

	jsonStr, err := protojson.Marshal(rqst.Role)

	if err == nil {
		srv.publishEvent("create_role_evt", jsonStr, srv.GetAddress())
	}

	return &resourcepb.CreateRoleRsp{Result: true}, nil
}

func (srv *server) DeleteRole(ctx context.Context, rqst *resourcepb.DeleteRoleRqst) (*resourcepb.DeleteRoleRsp, error) {

	_, token, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}

	// set the role id.
	roleId := rqst.RoleId
	localDomain, err := config.GetDomain()

	if strings.Contains(roleId, "@") {
		domain := strings.Split(roleId, "@")[1]
		roleId = strings.Split(roleId, "@")[0]

		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		if localDomain != domain {

			return nil, status.Errorf(
				codes.Internal,
				"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("cannot delete object from domain "+domain+" from domain "+localDomain)))
		}
	}

	// That service made user of persistence service.
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	q := `{"_id":"` + roleId + `"}`

	// Remove references
	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Roles", q, ``)
	if err != nil {
		if err.Error() == "not found" {
			return &resourcepb.DeleteRoleRsp{Result: true}, nil
		}

		return nil, err
	}

	role := values.(map[string]interface{})

	// Remove it from the accounts
	if role["members"] != nil {
		
		var accounts []interface{}
		switch role["members"].(type) {
		case primitive.A:
			accounts = []interface{}(role["members"].(primitive.A))
		case []interface{}:
			accounts = []interface{}(role["members"].([]interface{}))
		default:
			logger.Warn("unknown type", "value", role["members"])
		}
		for i := 0; i < len(accounts); i++ {
			accountId := accounts[i].(map[string]interface{})["$id"].(string)
			srv.deleteReference(p, accountId, roleId, "roles", "Accounts")
			srv.publishEvent("update_account_"+accountId+"_evt", []byte{}, srv.Address)
		}
	}

	// I will remove it from organizations...
	if role["organizations"] != nil {
		var organizations []interface{}
		switch role["organizations"].(type) {
		case primitive.A:
			organizations = []interface{}(role["organizations"].(primitive.A))
		case []interface{}:
			organizations = []interface{}(role["organizations"].([]interface{}))
		default:
			logger.Warn("unknown type", "value", role["organizations"])
		}

		for i := 0; i < len(organizations); i++ {
			organizationId := organizations[i].(map[string]interface{})["$id"].(string)
			srv.deleteReference(p, rqst.RoleId, organizationId, "roles", "Roles")
			srv.publishEvent("update_organization_"+organizationId+"_evt", []byte{}, srv.Address)
		}
	}

	err = p.DeleteOne(context.Background(), "local_resource", "local_resource", "Roles", q, ``)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// TODO delete role permissions
	srv.deleteResourcePermissions(token, rqst.RoleId)
	srv.deleteAllAccess(token,rqst.RoleId, rbacpb.SubjectType_ROLE)

	srv.publishEvent("delete_role_"+rqst.RoleId+"_evt", []byte{}, srv.Address)
	srv.publishEvent("delete_role_evt", []byte(rqst.RoleId), srv.Address)

	return &resourcepb.DeleteRoleRsp{Result: true}, nil
}

func (srv *server) getRole(id string) (*resourcepb.Role, error) {
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	q := `{"_id":"` + id + `"}`

	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Roles", q, ``)
	if err != nil {
		return nil, err
	}

	role := values.(map[string]interface{})
	r := &resourcepb.Role{Id: role["_id"].(string), Name: role["name"].(string), Actions: make([]string, 0)}

	if role["domain"] != nil {
		r.Domain = role["domain"].(string)
	} else {
		r.Domain = srv.Domain
	}

	if role["actions"] != nil {
		var actions []interface{}
		switch role["actions"].(type) {
		case primitive.A:
			actions = []interface{}(role["actions"].(primitive.A))
		case []interface{}:
			actions = []interface{}(role["actions"].([]interface{}))
		default:
			logger.Warn("unknown type", "value", role["actions"])
		}
		if actions != nil {
			for i := 0; i < len(actions); i++ {
				r.Actions = append(r.Actions, actions[i].(string))
			}
		}
	}

	if role["organizations"] != nil {
		var organizations []interface{}
		switch role["organizations"].(type) {
		case primitive.A:
			organizations = []interface{}(role["organizations"].(primitive.A))
		case []interface{}:
			organizations = []interface{}(role["organizations"].([]interface{}))
		default:
			logger.Warn("unknown type", "value", role["organizations"])
		}

		if organizations != nil {
			for i := 0; i < len(organizations); i++ {
				organizationId := organizations[i].(map[string]interface{})["$id"].(string)
				r.Organizations = append(r.Organizations, organizationId)
			}
		}
	}

	if role["members"] != nil {
		var members []interface{}
		switch role["members"].(type) {
		case primitive.A:
			members = []interface{}(role["members"].(primitive.A))
		case []interface{}:
			members = []interface{}(role["members"].([]interface{}))
		default:
			logger.Warn("unknown type", "value", role["members"])
		}

		if members != nil {
			for i := 0; i < len(members); i++ {
				memberId := members[i].(map[string]interface{})["$id"].(string)
				r.Members = append(r.Members, memberId)
			}
		}
	}

	return r, nil
}

// GetRoles streams roles from the persistence store based on the provided query and options.
// It retrieves roles from the "local_resource" collection, processes their fields, and sends them in batches
// over the gRPC stream. Each role includes its ID, name, description, domain, actions, organizations, and members.
// If an error occurs during retrieval or streaming, it returns a gRPC status error.
func (srv *server) GetRoles(rqst *resourcepb.GetRolesRqst, stream resourcepb.ResourceService_GetRolesServer) error {

	// Get the persistence connection
	p, err := srv.getPersistenceStore()
	if err != nil {
		return err
	}

	query := rqst.Query
	if query == "" {
		query = `{}`
	}

	roles, err := p.Find(context.Background(), "local_resource", "local_resource", "Roles", query, rqst.Options)
	if err != nil {
		return status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// No I will stream the result over the networks.
	maxSize := 100
	values := make([]*resourcepb.Role, 0)

	for i := range roles {
		role := roles[i].(map[string]interface{})
		r := &resourcepb.Role{Id: role["_id"].(string), Name: role["name"].(string), Description: role["description"].(string), Actions: make([]string, 0)}

		if role["domain"] != nil {
			r.Domain = role["domain"].(string)
		} else {
			r.Domain = srv.Domain
		}

		if role["actions"] != nil {
			var actions []interface{}
			switch role["actions"].(type) {
			case primitive.A:
				actions = []interface{}(role["actions"].(primitive.A))
			case []interface{}:
				actions = []interface{}(role["actions"].([]interface{}))
			default:
				logger.Warn("unknown type", "value", role["actions"])
			}
			if actions != nil {
				for i := 0; i < len(actions); i++ {
					r.Actions = append(r.Actions, actions[i].(string))
				}
			}
		}

		if role["organizations"] != nil {
			var organizations []interface{}
			switch role["organizations"].(type) {
			case primitive.A:
				organizations = []interface{}(role["organizations"].(primitive.A))
			case []interface{}:
				organizations = []interface{}(role["organizations"].([]interface{}))
			default:
				logger.Warn("unknown type", "value", role["organizations"])
			}

			if organizations != nil {
				for i := 0; i < len(organizations); i++ {
					organizationId := organizations[i].(map[string]interface{})["$id"].(string)
					r.Organizations = append(r.Organizations, organizationId)
				}
			}
		}

		if role["members"] != nil {
			var members []interface{}
			switch role["members"].(type) {
			case primitive.A:
				members = []interface{}(role["members"].(primitive.A))
			case []interface{}:
				members = []interface{}(role["members"].([]interface{}))
			default:
				logger.Warn("unknown type", "value", role["members"])
			}

			if members != nil {
				for i := 0; i < len(members); i++ {
					memberId := members[i].(map[string]interface{})["$id"].(string)
					r.Members = append(r.Members, memberId)
				}
			}
		}

		values = append(values, r)

		if len(values) >= maxSize {
			err := stream.Send(
				&resourcepb.GetRolesRsp{
					Roles: values,
				},
			)
			if err != nil {
				return status.Errorf(
					codes.Internal,
					"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}
			values = make([]*resourcepb.Role, 0)
		}
	}

	// Send reminding values.
	err = stream.Send(
		&resourcepb.GetRolesRsp{
			Roles: values,
		},
	)

	if err != nil {
		return status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return nil
}

// RemoveAccountRole removes the association between an account and a role.
// It deletes the references in both directions: from the account to the role and from the role to the account.
// After successful removal, it publishes update events for both the role and the account.
// Returns a response indicating the result or an error if the operation fails.
func (srv *server) RemoveAccountRole(ctx context.Context, rqst *resourcepb.RemoveAccountRoleRqst) (*resourcepb.RemoveAccountRoleRsp, error) {

	// That service made user of persistence service.
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	// That service made user of persistence service.
	err = srv.deleteReference(p, rqst.AccountId, rqst.RoleId, "members", "Roles")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = srv.deleteReference(p, rqst.RoleId, rqst.AccountId, "roles", "Accounts")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	srv.publishEvent("update_role_"+rqst.RoleId+"_evt", []byte{}, srv.Address)
	srv.publishEvent("update_account_"+rqst.AccountId+"_evt", []byte{}, srv.Address)

	return &resourcepb.RemoveAccountRoleRsp{Result: true}, nil
}

// RemoveOrganizationRole removes the association between a role and an organization.
// It deletes the references in both directions: from the role to the organization and from the organization to the role.
// After successful removal, it publishes update events for both the organization and the role.
// Returns a response indicating the result of the operation or an error if any step fails.
func (srv *server) RemoveOrganizationRole(ctx context.Context, rqst *resourcepb.RemoveOrganizationRoleRqst) (*resourcepb.RemoveOrganizationRoleRsp, error) {
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	err = srv.deleteReference(p, rqst.RoleId, rqst.OrganizationId, "roles", "Organizations")
	if err != nil {
		return nil, err
	}

	err = srv.deleteReference(p, rqst.OrganizationId, rqst.RoleId, "organizations", "Roles")
	if err != nil {
		return nil, err
	}

	srv.publishEvent("update_organization_"+rqst.OrganizationId+"_evt", []byte{}, srv.Address)
	srv.publishEvent("update_role_"+rqst.RoleId+"_evt", []byte{}, srv.Address)

	return &resourcepb.RemoveOrganizationRoleRsp{Result: true}, nil
}

// RemoveRoleAction removes a specified action from a role identified by RoleId.
// It validates the domain of the role, retrieves the role from the persistence store,
// and removes the action if it exists in the role's actions list. If the action is
// successfully removed, the updated role is saved back to the persistence store and
// an update event is published. Returns an error if the role or action is not found,
// or if there are issues with domain validation or persistence operations.
//
// Parameters:
//   ctx - The context for the request.
//   rqst - The request containing the RoleId and Action to be removed.
//
// Returns:
//   *resourcepb.RemoveRoleActionRsp - The response indicating success.
//   error - An error if the operation fails.
func (srv *server) RemoveRoleAction(ctx context.Context, rqst *resourcepb.RemoveRoleActionRqst) (*resourcepb.RemoveRoleActionRsp, error) {
	roleId := rqst.RoleId
	localDomain, err := config.GetDomain()
	if strings.Contains(roleId, "@") {
		domain := strings.Split(roleId, "@")[1]
		roleId = strings.Split(roleId, "@")[0]

		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		if localDomain != domain {

			return nil, status.Errorf(
				codes.Internal,
				"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("cannot delete object from domain "+domain+" from domain "+localDomain)))
		}
	}

	// That service made user of persistence service.
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	q := `{"_id":"` + roleId + `"}`

	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Roles", q, ``)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	role := values.(map[string]interface{})

	needSave := false
	if role["actions"] == nil {
		role["actions"] = []string{rqst.Action}
		needSave = true
	} else {
		exist := false
		actions := make([]interface{}, 0)
		var actions_ []interface{}
		switch role["actions"].(type) {
		case primitive.A:
			actions_ = []interface{}(role["actions"].(primitive.A))
		case []interface{}:
			actions_ = []interface{}(role["actions"].([]interface{}))
		default:
			logger.Warn("unknown type", "value", role["actions"])
		}

		for i := 0; i < len(actions_); i++ {
			if actions_[i].(string) == rqst.Action {
				exist = true
			} else {
				actions = append(actions, actions_[i])
			}
		}

		if exist {
			role["actions"] = actions
			needSave = true
		} else {
			return nil, status.Errorf(
				codes.Internal,
				"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("Role named "+roleId+" not contain actions named "+rqst.Action+"!")))
		}
	}

	if needSave {
		// jsonStr, _ := Utility.ToJson(role)
		jsonStr := serialyseObject(role)

		err := p.ReplaceOne(context.Background(), "local_resource", "local_resource", "Roles", q, string(jsonStr), ``)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	srv.publishEvent("update_role_"+rqst.RoleId+"_evt", []byte{}, srv.Address)

	return &resourcepb.RemoveRoleActionRsp{Result: true}, nil
}

// RemoveRolesAction removes a specified action from all roles in the "Roles" collection.
// It retrieves all roles, checks if the action exists in each role's actions list, and removes it if present.
// If a role is modified, it is updated in the persistence store and an update event is published.
// Returns a response indicating success or an error if any operation fails.
//
// Parameters:
//   ctx  - The context for the request.
//   rqst - The request containing the action to be removed.
//
// Returns:
//   *resourcepb.RemoveRolesActionRsp - The response indicating the result of the operation.
//   error                            - An error if the operation fails.
func (srv *server) RemoveRolesAction(ctx context.Context, rqst *resourcepb.RemoveRolesActionRqst) (*resourcepb.RemoveRolesActionRsp, error) {
	// That service made user of persistence service.
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	q := `{}`

	values, err := p.Find(context.Background(), "local_resource", "local_resource", "Roles", q, ``)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	for i := 0; i < len(values); i++ {
		role := values[i].(map[string]interface{})

		needSave := false
		if role["actions"] == nil {
			role["actions"] = []string{rqst.Action}
			needSave = true
		} else {
			exist := false
			var actions []interface{}
			switch role["actions"].(type) {
			case primitive.A:
				actions = []interface{}(role["actions"].(primitive.A))
			case []interface{}:
				actions = []interface{}(role["actions"].([]interface{}))
			default:
				logger.Warn("unknown type", "value", role["actions"])
			}

			var actions_ []interface{}
			for i := 0; i < len(actions); i++ {
				if actions[i].(string) == rqst.Action {
					exist = true
				} else {
					actions_ = append(actions_, actions[i])
				}
			}

			if exist {
				role["actions"] = actions_
				needSave = true
			}
		}

		if needSave {
			// jsonStr, _ := Utility.ToJson(role)
			jsonStr := serialyseObject(role)

			q = `{"_id":"` + role["_id"].(string) + `"}`

			err := p.ReplaceOne(context.Background(), "local_resource", "local_resource", "Roles", q, string(jsonStr), ``)
			if err != nil {
				return nil, status.Errorf(
					codes.Internal,
					"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}

			srv.publishEvent("update_role_"+role["_id"].(string)+"@"+role["domain"].(string)+"_evt", []byte{}, srv.Address)

		}
	}

	return &resourcepb.RemoveRolesActionRsp{Result: true}, nil
}

// UpdateRole updates the role identified by RoleId with the provided values.
// It first checks if the role exists in the persistence store. If the role exists,
// it updates the role with the new values. If the role does not exist or an error occurs,
// an appropriate error is returned. Upon successful update, an event is published.
// Returns a response indicating the result of the update operation.
func (srv *server) UpdateRole(ctx context.Context, rqst *resourcepb.UpdateRoleRqst) (*resourcepb.UpdateRoleRsp, error) {
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	q := `{"_id":"` + rqst.RoleId + `"}`

	// Get the persistence connection
	count, err := p.Count(context.Background(), "local_resource", "local_resource", "Roles", q, "")
	if err != nil || count == 0 {
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	} else {

		err = p.UpdateOne(context.Background(), "local_resource", "local_resource", "Roles", q, rqst.Values, "")
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	srv.publishEvent("update_role_"+rqst.RoleId+"_evt", []byte{}, srv.Address)

	return &resourcepb.UpdateRoleRsp{
		Result: true,
	}, nil
}
