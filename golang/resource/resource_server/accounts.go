package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/resource/resourcepb"
	"github.com/globulario/services/golang/security"
	Utility "github.com/globulario/utility"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/encoding/protojson"
)

func (srv *server) createGroup(token, id, name, owner, description string, members []string) error {

	localDomain, err := config.GetDomain()
	if err != nil {
		return err
	}

	// test if the given domain is the local domain.
	if strings.Contains(id, "@") {
		domain := strings.Split(id, "@")[1]
		id = strings.Split(id, "@")[0]
		if domain != localDomain {
			return errors.New("you can't register group " + id + " with domain " + domain + " on domain " + localDomain)
		}
	}

	// Get the persistence connection
	p, err := srv.getPersistenceStore()
	if err != nil {
		return err
	}

	q := `{"_id":"` + id + `"}`

	// Here I will first look if a peer with a same name already exist on the
	// resources...
	count, _ := p.Count(context.Background(), "local_resource", "local_resource", "Groups", q, "")
	if count > 0 {
		return errors.New("Group with name '" + id + "' already exist!")
	}

	// No authorization exist for that peer I will insert it.
	// Here will create the new peer.
	g := make(map[string]interface{}, 0)
	g["_id"] = id
	g["name"] = name
	g["description"] = description
	g["domain"] = localDomain
	g["typeName"] = "Group"

	_, err = p.InsertOne(context.Background(), "local_resource", "local_resource", "Groups", g, "")
	if err != nil {
		return err
	}

	// Create references.
	for i := range members {

		if !strings.Contains(members[i], "@") {
			members[i] = members[i] + "@" + localDomain
		}

		err := srv.createCrossReferences(id, "Groups", "members", members[i], "Accounts", "groups")
		if err != nil {
			return err
		}
	}

	// Now create the resource permission.
	srv.addResourceOwner(token, id+"@"+srv.Domain, owner, "group", rbacpb.SubjectType_ACCOUNT)
	logger.Info("group created", "group_id", id, "owner", owner)
	return nil
}

/**
 * Create account dir for all account in the database if not already exist.
 */
func (srv *server) CreateAccountDir(ctx context.Context) error {
	p, err := srv.getPersistenceStore()
	if err != nil {
		return err
	}

	q := `{}`

	_, token, err := security.GetClientId(ctx)
	if err != nil {
		return err
	}

	// Make sure some account exist on the server.
	count, _ := p.Count(context.Background(), "local_resource", "local_resource", "Accounts", q, "")
	if count == 0 {
		return errors.New("no account exist in the database")
	}

	accounts, err := p.Find(context.Background(), "local_resource", "local_resource", "Accounts", q, "")
	if err != nil {
		return err
	}
	for i := 0; i < len(accounts); i++ {

		a := accounts[i].(map[string]interface{})
		id := a["_id"].(string)
		domain := a["domain"].(string)
		path := "/users/" + id + "@" + domain
		if !Utility.Exists(config.GetDataDir() + "/files" + path) {
			Utility.CreateDirIfNotExist(config.GetDataDir() + "/files" + path)
			srv.addResourceOwner(token, path, id+"@"+domain, "file", rbacpb.SubjectType_ACCOUNT)
		}
	}

	return nil
}

func (srv *server) createRole(ctx context.Context, id, name, owner string, description string, actions []string) error {

	localDomain, err := config.GetDomain()
	if err != nil {
		return err
	}

	_, token, err := security.GetClientId(ctx)
	if err != nil {
		return err
	}

	// test if the given domain is the local domain.
	if strings.Contains(id, "@") {
		domain := strings.Split(id, "@")[1]
		id = strings.Split(id, "@")[0]
		if domain != localDomain {
			return errors.New("you can't create role " + id + " with domain " + domain + " on domain " + localDomain)
		}
	}

	// That service made user of persistence service.
	p, err := srv.getPersistenceStore()
	if err != nil {
		return err
	}

	q := `{"_id":"` + id + `"}`

	_, err = p.FindOne(context.Background(), "local_resource", "local_resource", "Roles", q, ``)
	if err == nil {
		return errors.New("role named " + name + " already exist!")
	}

	// Here will create the new role.
	role := make(map[string]interface{})
	role["_id"] = id
	role["name"] = name
	role["actions"] = actions
	role["domain"] = localDomain
	role["description"] = description
	role["typeName"] = "Role"

	_, err = p.InsertOne(context.Background(), "local_resource", "local_resource", "Roles", role, "")
	if err != nil {
		return err
	}

	if name != "admin" {
		srv.addResourceOwner(token, id+"@"+srv.Domain, owner, "role", rbacpb.SubjectType_ACCOUNT)
	}

	return nil
}

/**
 *  hashPassword return the bcrypt hash of the password.
 */
func (srv *server) hashPassword(password string) (string, error) {
	haspassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(haspassword), nil
}

/**
 * Return the hash password.
 */
func (srv *server) validatePassword(password string, hash string) error {

	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if err != nil {
		hashPassword, err := srv.hashPassword(password)
		if err != nil {
			return fmt.Errorf("failed to hash password: %w", err)
		}
		return bcrypt.CompareHashAndPassword([]byte(hash), []byte(hashPassword))
	}
	return nil
}

// AccountExist checks whether an account exists based on the provided account ID.
// It supports both local and remote domain accounts. If the account ID contains a domain
// (e.g., "user@domain.com"), it verifies if the domain matches the local domain. If not,
// it attempts to find the account on the remote domain. For local accounts, it queries
// the persistence store to check for existence. Returns a response indicating whether
// the account exists, or an error if any issues occur during the process.
func (srv *server) AccountExist(ctx context.Context, rqst *resourcepb.AccountExistRqst) (*resourcepb.AccountExistRsp, error) {

	// Get the persistence connection
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	// Test with the _id
	accountId := rqst.Id

	if strings.Contains(accountId, "@") {
		domain := strings.Split(accountId, "@")[1]
		accountId = strings.Split(accountId, "@")[0]

		localDomain, err := config.GetDomain()
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		// find account on other domain.
		if localDomain != domain {

			_, err := srv.getRemoteAccount(accountId, domain)
			if err != nil {
				return nil, status.Errorf(
					codes.Internal,
					"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}

			// return true.
			return &resourcepb.AccountExistRsp{
				Result: true,
			}, nil

		}
	}

	q := `{"_id":"` + accountId + `"}`
	count, _ := p.Count(context.Background(), "local_resource", "local_resource", "Accounts", q, "")

	if count > 0 {
		return &resourcepb.AccountExistRsp{
			Result: true,
		}, nil
	}

	return nil, status.Errorf(
		codes.Internal,
		"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("account '"+rqst.Id+"' doesn't exist!")))

}

func (srv *server) getRemoteAccount(id string, domain string) (*resourcepb.Account, error) {
	fmt.Println("get account ", id, "from", domain)
	client, err := getResourceClient(domain)
	if err != nil {
		return nil, err
	}

	return client.GetAccount(id)
}

// AddGroupMemberAccount adds an account as a member to a specified group.
// It ensures that both AccountId and GroupId contain the domain suffix, appending it if necessary.
// The function creates cross-references between the group and the account, and publishes update events for both.
// Returns a response indicating success or an error if the operation fails.
func (srv *server) AddGroupMemberAccount(ctx context.Context, rqst *resourcepb.AddGroupMemberAccountRqst) (*resourcepb.AddGroupMemberAccountRsp, error) {

	if !strings.Contains(rqst.AccountId, "@") {
		rqst.AccountId += "@" + srv.Domain
	}

	if !strings.Contains(rqst.GroupId, "@") {
		rqst.GroupId += "@" + srv.Domain
	}

	err := srv.createCrossReferences(rqst.GroupId, "Groups", "members", rqst.AccountId, "Accounts", "groups")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	srv.publishEvent("update_group_"+rqst.GroupId+"_evt", []byte{}, srv.Address)
	srv.publishEvent("update_account_"+rqst.AccountId+"_evt", []byte{}, srv.Address)

	return &resourcepb.AddGroupMemberAccountRsp{Result: true}, nil
}

// AddOrganizationAccount associates an account with an organization by creating cross-references between them.
// It ensures that both AccountId and OrganizationId contain the domain suffix, appending it if necessary.
// After successfully creating the cross-references, it publishes update events for the organization.
// Returns a response indicating the result of the operation or an error if the process fails.
func (srv *server) AddOrganizationAccount(ctx context.Context, rqst *resourcepb.AddOrganizationAccountRqst) (*resourcepb.AddOrganizationAccountRsp, error) {

	if !strings.Contains(rqst.AccountId, "@") {
		rqst.AccountId += "@" + srv.Domain
	}

	if !strings.Contains(rqst.OrganizationId, "@") {
		rqst.OrganizationId += "@" + srv.Domain
	}

	err := srv.createCrossReferences(rqst.OrganizationId, "Organizations", "accounts", rqst.AccountId, "Accounts", "organizations")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	srv.publishEvent("update_organization_"+rqst.OrganizationId+"_evt", []byte{}, srv.Address)
	srv.publishEvent("update_organization_"+rqst.OrganizationId+"_evt", []byte{}, srv.Address)

	return &resourcepb.AddOrganizationAccountRsp{Result: true}, nil
}

// AddOrganizationApplication associates an application with an organization by creating cross-references between them.
// It ensures that both ApplicationId and OrganizationId are fully qualified with the domain if not already present.
// After successfully creating the references, it publishes update events for the organization.
// Returns a response indicating the result of the operation or an error if the process fails.
func (srv *server) AddOrganizationApplication(ctx context.Context, rqst *resourcepb.AddOrganizationApplicationRqst) (*resourcepb.AddOrganizationApplicationRsp, error) {

	if !strings.Contains(rqst.ApplicationId, "@") {
		rqst.ApplicationId += "@" + srv.Domain
	}

	if !strings.Contains(rqst.OrganizationId, "@") {
		rqst.OrganizationId += "@" + srv.Domain
	}

	err := srv.createCrossReferences(rqst.OrganizationId, "Organizations", "applications", rqst.ApplicationId, "Applications", "organizations")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	srv.publishEvent("update_organization_"+rqst.OrganizationId+"_evt", []byte{}, srv.Address)
	srv.publishEvent("update_organization_"+rqst.OrganizationId+"_evt", []byte{}, srv.Address)

	return &resourcepb.AddOrganizationApplicationRsp{Result: true}, nil
}

// AddOrganizationGroup adds a group to an organization by creating cross-references between them.
// It ensures that both the group ID and organization ID contain the domain suffix.
// After successfully creating the references, it publishes update events for the organization.
// Returns a response indicating the result or an error if the operation fails.
func (srv *server) AddOrganizationGroup(ctx context.Context, rqst *resourcepb.AddOrganizationGroupRqst) (*resourcepb.AddOrganizationGroupRsp, error) {

	if !strings.Contains(rqst.GroupId, "@") {
		rqst.GroupId += "@" + srv.Domain
	}

	if !strings.Contains(rqst.OrganizationId, "@") {
		rqst.OrganizationId += "@" + srv.Domain
	}

	err := srv.createCrossReferences(rqst.OrganizationId, "Organizations", "groups", rqst.GroupId, "Groups", "organizations")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	srv.publishEvent("update_organization_"+rqst.OrganizationId+"_evt", []byte{}, srv.Address)
	srv.publishEvent("update_organization_"+rqst.OrganizationId+"_evt", []byte{}, srv.Address)

	return &resourcepb.AddOrganizationGroupRsp{Result: true}, nil
}

// CreateGroup handles the creation of a new group resource.
// It retrieves the client ID from the context, invokes the internal group creation logic,
// publishes a "create_group_evt" event upon success, and returns the result.
// Returns an error if the client ID cannot be retrieved or if group creation fails.
func (srv *server) CreateGroup(ctx context.Context, rqst *resourcepb.CreateGroupRqst) (*resourcepb.CreateGroupRsp, error) {

	clientId, token, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}

	// Get the persistence connection
	err = srv.createGroup(token, rqst.Group.Id, rqst.Group.Name, clientId, rqst.Group.Description, rqst.Group.Members)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	jsonStr, err := json.Marshal(rqst.Group)
	if err == nil {
		srv.publishEvent("create_group_evt", jsonStr, srv.GetAddress())
	}

	return &resourcepb.CreateGroupRsp{
		Result: true,
	}, nil
}

// CreateOrganization creates a new organization in the persistence store.
// It first checks if an organization with the same ID already exists, and returns an error if so.
// The function ensures the organization's domain matches the local domain.
// It then inserts the organization with its properties and initializes empty lists for accounts, groups, roles, and applications.
// For each account, group, role, and application associated with the organization, cross-references are created.
// An event is published upon successful creation, and the resource owner is registered.
// Returns a CreateOrganizationRsp with the result or an error if any step fails.
func (srv *server) CreateOrganization(ctx context.Context, rqst *resourcepb.CreateOrganizationRqst) (*resourcepb.CreateOrganizationRsp, error) {

	clientId, token, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}

	// Get the persistence connection
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	q := `{"_id":"` + rqst.Organization.Id + `"}`

	// Here I will first look if a peer with a same name already exist on the
	// resources...
	count, _ := p.Count(context.Background(), "local_resource", "local_resource", "Organizations", q, "")
	if count > 0 {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("Organization with name '"+rqst.Organization.Id+"' already exist!")))
	}

	localDomain, err := config.GetDomain()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// test if the given domain is the local domain.
	if rqst.Organization.Domain != localDomain {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("you can't register organization "+rqst.Organization.Id+" with domain "+rqst.Organization.Domain+" on domain "+localDomain)))
	}

	// No authorization exist for that peer I will insert it.
	// Here will create the new peer.
	o := make(map[string]interface{}, 0)
	o["_id"] = rqst.Organization.Id
	o["name"] = rqst.Organization.Name
	o["icon"] = rqst.Organization.Icon
	o["email"] = rqst.Organization.Email
	o["description"] = rqst.Organization.Description
	o["domain"] = srv.Domain

	// Those are the list of entity linked to the organization
	o["accounts"] = make([]interface{}, 0)
	o["groups"] = make([]interface{}, 0)
	o["roles"] = make([]interface{}, 0)
	o["applications"] = make([]interface{}, 0)

	_, err = p.InsertOne(context.Background(), "local_resource", "local_resource", "Organizations", o, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// accounts...
	for i := 0; i < len(rqst.Organization.Accounts); i++ {
		if !strings.Contains(rqst.Organization.Accounts[i], "@") {
			rqst.Organization.Accounts[i] += "@" + rqst.Organization.Domain
		}
		srv.createCrossReferences(rqst.Organization.Accounts[i], "Accounts", "organizations", rqst.Organization.GetId()+"@"+rqst.Organization.Domain, "Organizations", "accounts")
	}

	// groups...
	for i := 0; i < len(rqst.Organization.Groups); i++ {
		if !strings.Contains(rqst.Organization.Groups[i], "@") {
			rqst.Organization.Groups[i] += "@" + rqst.Organization.Domain
		}
		srv.createCrossReferences(rqst.Organization.Groups[i], "Groups", "organizations", rqst.Organization.GetId()+"@"+rqst.Organization.Domain, "Organizations", "groups")
	}

	// roles...
	for i := 0; i < len(rqst.Organization.Roles); i++ {
		if !strings.Contains(rqst.Organization.Roles[i], "@") {
			rqst.Organization.Roles[i] += "@" + rqst.Organization.Domain
		}
		srv.createCrossReferences(rqst.Organization.Roles[i], "Roles", "organizations", rqst.Organization.GetId()+"@"+rqst.Organization.Domain, "Organizations", "roles")
	}

	// applications...
	for i := 0; i < len(rqst.Organization.Applications); i++ {
		if !strings.Contains(rqst.Organization.Applications[i], "@") {
			rqst.Organization.Applications[i] += "@" + rqst.Organization.Domain
		}
		srv.createCrossReferences(rqst.Organization.Roles[i], "Applications", "organizations", rqst.Organization.GetId()+"@"+rqst.Organization.Domain, "Organizations", "applications")
	}

	jsonStr, err := json.Marshal(rqst.Organization)
	if err == nil {
		srv.publishEvent("create_organization_evt", jsonStr, srv.Address)
	}

	// create the resource owner.
	srv.addResourceOwner(token, rqst.Organization.GetId()+"@"+rqst.Organization.Domain, clientId, "organization", rbacpb.SubjectType_ACCOUNT)

	return &resourcepb.CreateOrganizationRsp{
		Result: true,
	}, nil
}

// DeleteAccount deletes an account identified by the given request ID.
// It performs the following steps:
//   - Validates the domain of the account to ensure it matches the local domain.
//   - Retrieves the account from the persistence store.
//   - Removes references to the account from organizations, groups, and roles.
//   - Deletes all RBAC access associated with the account.
//   - Deletes the account from the persistence store.
//   - Removes the account from contacts in its database.
//   - Drops the user from the underlying database (MongoDB, Scylla, or SQL).
//   - Deletes the user's database and associated files.
//   - Publishes events related to account deletion.
//
// Returns a DeleteAccountRsp containing the result or an error if any operation fails.
func (srv *server) DeleteAccount(ctx context.Context, rqst *resourcepb.DeleteAccountRqst) (*resourcepb.DeleteAccountRsp, error) {
	accountId := rqst.Id
	localDomain, _ := config.GetDomain()
	domain, _ := config.GetDomain()

	if strings.Contains(accountId, "@") {
		domain = strings.Split(accountId, "@")[1]
		accountId = strings.Split(accountId, "@")[0]

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

	q := `{"_id":"` + accountId + `"}`

	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Accounts", q, ``)
	if err != nil {
		if err.Error() == "not found" {
			return &resourcepb.DeleteAccountRsp{Result: ""}, nil
		}

		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	account := values.(map[string]interface{})

	// Remove references.
	if account["organizations"] != nil {
		var organizations []interface{}
		switch account["organizations"].(type) {
		case primitive.A:
			organizations = []interface{}(account["organizations"].(primitive.A))
		case []interface{}:
			organizations = []interface{}(account["organizations"].([]interface{}))
		default:
			logger.Warn("unknown type", "value", account["organizations"])
		}
		for i := 0; i < len(organizations); i++ {
			organizationId := organizations[i].(map[string]interface{})["$id"].(string)
			srv.deleteReference(p, rqst.Id, organizationId, "accounts", "Organizations")
			srv.publishEvent("update_organization_"+organizationId+"_evt", []byte{}, srv.Address)
		}
	}

	if account["groups"] != nil {
		var groups []interface{}
		switch account["groups"].(type) {
		case primitive.A:
			groups = []interface{}(account["groups"].(primitive.A))
		case []interface{}:
			groups = []interface{}(account["groups"].([]interface{}))
		default:
			logger.Warn("unknown type", "value", account["groups"])
		}

		for i := 0; i < len(groups); i++ {
			groupId := groups[i].(map[string]interface{})["$id"].(string)
			srv.deleteReference(p, rqst.Id, groupId, "members", "Groups")
			srv.publishEvent("update_group_"+groupId+"_evt", []byte{}, srv.Address)
		}
	}

	if account["roles"] != nil {
		var roles []interface{}
		switch account["roles"].(type) {
		case primitive.A:
			roles = []interface{}(account["roles"].(primitive.A))
		case []interface{}:
			roles = []interface{}(account["roles"].([]interface{}))
		default:
			logger.Warn("unknown type", "value", account["roles"])
		}

		for i := 0; i < len(roles); i++ {
			roleId := roles[i].(map[string]interface{})["$id"].(string)
			srv.deleteReference(p, rqst.Id, roleId, "members", "Roles")
			srv.publishEvent("update_role_"+roleId+"_evt", []byte{}, srv.Address)
		}

	}

	_, token, err := security.GetClientId(ctx)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	srv.deleteAllAccess(token, accountId+"@"+domain, rbacpb.SubjectType_ACCOUNT)

	// Try to delete the account...
	err = p.DeleteOne(context.Background(), "local_resource", "local_resource", "Accounts", q, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	name := account["name"].(string)
	name = strings.ReplaceAll(strings.ReplaceAll(name, ".", "_"), "@", "_")
	name = strings.ReplaceAll(name, "-", "_")
	name = strings.ReplaceAll(name, ".", "_")
	name = strings.ReplaceAll(name, " ", "_")

	get_contacts := `{}`

	// so before remove database I need to remove the account from it contacts...
	contacts, err := p.Find(context.Background(), "local_resource", "local_resource", "Contacts", get_contacts, "")
	if err == nil {
		for i := 0; i < len(contacts); i++ {

			// Get the contact.
			contact := contacts[i].(map[string]interface{})
			name := contact["name"].(string)
			name = strings.ReplaceAll(strings.ReplaceAll(name, ".", "_"), "@", "_")
			name = strings.ReplaceAll(name, "-", "_")
			name = strings.ReplaceAll(name, ".", "_")
			name = strings.ReplaceAll(name, " ", "_")

			// So here I will call delete on the db...
			err = p.DeleteOne(context.Background(), "local_resource", "local_resource", "Contacts", q, "")

			if err == nil {
				// Here I will send delete contact event.
				srv.publishEvent("update_account_"+contact["_id"].(string)+"@"+contact["domain"].(string)+"_evt", []byte{}, srv.Address)
			}

		}
	}

	var dropUserScript string
	if p.GetStoreType() == "MONGO" {
		dropUserScript = fmt.Sprintf(
			`db=db.getSiblingDB('admin');db.dropUser('%s', {w: 'majority', wtimeout: 4000})`,
			name)
	} else if p.GetStoreType() == "SCYLLA" {
		dropUserScript = `` // TODO scylla db query.
	} else if p.GetStoreType() == "SQL" {
		q = `` // TODO sql query string here...
	} else {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("unknown database type "+p.GetStoreType())))
	}

	// I will execute the script with the admin function.
	err = p.RunAdminCmd(context.Background(), "local_resource", srv.Backend_user, srv.Backend_password, dropUserScript)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Remove the file...
	srv.deleteResourcePermissions(token, "/users/"+name+"@"+domain)
	srv.deleteAllAccess(token, name+"@"+domain, rbacpb.SubjectType_ACCOUNT)

	os.RemoveAll(config.GetDataDir() + "/files/users/" + name + "@" + domain)

	// Publish delete account event.
	srv.publishEvent("delete_account_"+name+"@"+domain+"_evt", []byte{}, srv.Address)
	srv.publishEvent("delete_account_evt", []byte(name+"@"+domain), srv.Address)

	return &resourcepb.DeleteAccountRsp{
		Result: rqst.Id,
	}, nil
}

// DeleteGroup deletes a group specified by the request from the persistence store.
// It performs the following operations:
//   - Validates the group domain and ensures it matches the local domain.
//   - Removes references to the group from associated accounts and organizations.
//   - Publishes update events for affected accounts and organizations.
//   - Deletes the group from the persistence store.
//   - Removes resource permissions and access controls related to the group.
//   - Publishes group deletion events.
//
// Returns a DeleteGroupRsp indicating the result or an error if the operation fails.
func (srv *server) DeleteGroup(ctx context.Context, rqst *resourcepb.DeleteGroupRqst) (*resourcepb.DeleteGroupRsp, error) {

	groupId := rqst.Group
	localDomain, err := config.GetDomain()

	if strings.Contains(groupId, "@") {
		domain := strings.Split(groupId, "@")[1]
		groupId = strings.Split(groupId, "@")[0]

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

	// Get the persistence connection
	p, err := srv.getPersistenceStore()
	if err != nil {
		logger.Info("log", "args", []interface{}{"fail to get persistence connection ", err})
		return nil, err
	}

	q := `{"_id":"` + groupId + `"}`

	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Groups", q, ``)
	if err != nil {
		if err.Error() == "not found" {
			return &resourcepb.DeleteGroupRsp{Result: true}, nil
		}

		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	group := values.(map[string]interface{})

	// I will remove it from accounts...

	if group["members"] != nil {

		var members []interface{}
		switch group["members"].(type) {
		case primitive.A:
			members = []interface{}(group["members"].(primitive.A))
		case []interface{}:
			members = group["members"].([]interface{})
		}

		for j := 0; j < len(members); j++ {
			accountId := members[j].(map[string]interface{})["$id"].(string)
			srv.deleteReference(p, rqst.Group, accountId, "groups", "Accounts")
			srv.publishEvent("update_account_"+accountId+"_evt", []byte{}, srv.Address)
		}
	}

	// I will remove it from organizations...
	if group["organizations"] != nil {

		var organizations []interface{}
		switch group["organizations"].(type) {
		case primitive.A:
			organizations = []interface{}(group["organizations"].(primitive.A))
		case []interface{}:
			organizations = group["organizations"].([]interface{})
		}

		if organizations != nil {
			for i := 0; i < len(organizations); i++ {
				organizationId := organizations[i].(map[string]interface{})["$id"].(string)
				srv.deleteReference(p, rqst.Group, organizationId, "groups", "Organizations")
				srv.publishEvent("update_organization_"+organizationId+"_evt", []byte{}, srv.Address)
			}
		}
	}

	err = p.DeleteOne(context.Background(), "local_resource", "local_resource", "Groups", q, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	groupId = group["_id"].(string) + "@" + group["domain"].(string)

	_, token, err := security.GetClientId(ctx)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	srv.deleteResourcePermissions(token, rqst.Group)
	srv.deleteAllAccess(token, groupId, rbacpb.SubjectType_GROUP)

	srv.publishEvent("delete_group_"+groupId+"_evt", []byte{}, srv.Address)

	srv.publishEvent("delete_group_evt", []byte(groupId), srv.Address)

	return &resourcepb.DeleteGroupRsp{
		Result: true,
	}, nil

}

// DeleteOrganization deletes an organization and all its associated references, including groups, roles, applications, and accounts.
// It first validates the organization domain, then removes references to related entities, publishes update events for each,
// deletes all access permissions for the organization, and finally deletes the organization record itself.
// Events are published to notify other services of the deletion and updates.
// Returns a DeleteOrganizationRsp with the result or an error if the operation fails.
func (srv *server) DeleteOrganization(ctx context.Context, rqst *resourcepb.DeleteOrganizationRqst) (*resourcepb.DeleteOrganizationRsp, error) {

	_, token, err := security.GetClientId(ctx)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	localDomain, err := config.GetDomain()
	organizationId := rqst.Organization
	if strings.Contains(organizationId, "@") {
		domain := strings.Split(organizationId, "@")[1]
		organizationId = strings.Split(organizationId, "@")[0]

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

	q := `{"_id":"` + organizationId + `"}`

	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Organizations", q, ``)
	if err != nil {
		if err.Error() == "not found" {
			return &resourcepb.DeleteOrganizationRsp{Result: true}, nil
		}
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	organization := values.(map[string]interface{})
	if organization["groups"] != nil {

		var groups []interface{}
		switch organization["groups"].(type) {
		case primitive.A:
			groups = []interface{}(organization["groups"].(primitive.A))
		case []interface{}:
			groups = organization["groups"].([]interface{})
		}

		if groups != nil {
			for i := 0; i < len(groups); i++ {
				groupId := groups[i].(map[string]interface{})["$id"].(string)
				err := srv.deleteReference(p, rqst.Organization, groupId, "organizations", "Groups")
				if err != nil {
					logger.Info("log", "args", []interface{}{err})
				}

				srv.publishEvent("update_group_"+groupId+"_evt", []byte{}, srv.Address)
			}
		}
	}

	if organization["roles"] != nil {

		var roles []interface{}
		switch organization["roles"].(type) {
		case primitive.A:
			roles = []interface{}(organization["roles"].(primitive.A))
		case []interface{}:
			roles = organization["roles"].([]interface{})
		}

		if roles != nil {
			for i := 0; i < len(roles); i++ {
				roleId := roles[i].(map[string]interface{})["$id"].(string)
				err := srv.deleteReference(p, rqst.Organization, roleId, "organizations", "Roles")
				if err != nil {
					logger.Info("log", "args", []interface{}{err})
				}

				srv.publishEvent("update_role_"+roleId+"_evt", []byte{}, srv.Address)
			}
		}
	}

	if organization["applications"] != nil {

		var applications []interface{}
		switch organization["applications"].(type) {
		case primitive.A:
			applications = []interface{}(organization["applications"].(primitive.A))
		case []interface{}:
			applications = organization["applications"].([]interface{})
		}

		if applications != nil {
			for i := 0; i < len(applications); i++ {
				applicationId := applications[i].(map[string]interface{})["$id"].(string)
				err := srv.deleteReference(p, rqst.Organization, applicationId, "organizations", "Applications")
				if err != nil {
					logger.Info("log", "args", []interface{}{err})
				}

				srv.publishEvent("update_application_"+applicationId+"_evt", []byte{}, srv.Address)
			}
		}
	}

	if organization["accounts"] != nil {

		var accounts []interface{}
		switch organization["accounts"].(type) {
		case primitive.A:
			accounts = []interface{}(organization["accounts"].(primitive.A))
		case []interface{}:
			accounts = organization["accounts"].([]interface{})
		}

		if accounts != nil {
			for i := 0; i < len(accounts); i++ {
				accountId := accounts[i].(map[string]interface{})["$id"].(string)
				err := srv.deleteReference(p, rqst.Organization, accountId, "organizations", "Accounts")
				if err != nil {
					logger.Info("log", "args", []interface{}{err})
				}

				srv.publishEvent("update_account_"+accountId+"_evt", []byte{}, srv.Address)
			}
		}
	}

	// Delete organization
	organizationId = organization["_id"].(string) + "@" + organization["domain"].(string)
	srv.deleteAllAccess(token, organizationId, rbacpb.SubjectType_ORGANIZATION)

	// Try to delete the account...
	err = p.DeleteOne(context.Background(), "local_resource", "local_resource", "Organizations", q, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	srv.deleteResourcePermissions(token, organizationId)
	srv.deleteAllAccess(token, organizationId, rbacpb.SubjectType_ORGANIZATION)

	srv.publishEvent("delete_organization_"+organizationId+"_evt", []byte{}, srv.Address)
	srv.publishEvent("delete_organization_evt", []byte(organizationId), srv.Address)

	return &resourcepb.DeleteOrganizationRsp{Result: true}, nil
}

// GetAccount retrieves an account by its ID, which may include a domain (e.g., "user@domain").
// If the account belongs to a remote domain, it fetches the account from the remote service.
// Otherwise, it queries the local persistence store for the account information.
// The function populates the Account response with basic fields, groups, roles, organizations,
// and user profile data (such as profile picture and names) if available.
// Returns a GetAccountRsp containing the account details or an error if retrieval fails.
func (srv *server) GetAccount(ctx context.Context, rqst *resourcepb.GetAccountRqst) (*resourcepb.GetAccountRsp, error) {

	// That service made user of persistence service.
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	accountId := rqst.AccountId

	if strings.Contains(accountId, "@") {
		domain := strings.Split(accountId, "@")[1]
		accountId = strings.Split(accountId, "@")[0]
		_domain, err := config.GetDomain()
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		if _domain != domain {
			a, err := srv.getRemoteAccount(accountId, domain)
			if err != nil {
				return nil, status.Errorf(
					codes.Internal,
					"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}
			return &resourcepb.GetAccountRsp{
				Account: a, // Return the token string.
			}, nil

		}
	}

	q := `{"_id":"` + accountId + `"}`

	if strings.Contains(q, "@") {
		// I will keep the first part of the string...
		q = strings.Split(q, "@")[0]
	}

	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Accounts", q, ``)
	if err != nil {
		logger.Info("log", "args", []interface{}{"fail to retrieve account:", accountId, " from database with error:", err})
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	account := values.(map[string]interface{})
	a := &resourcepb.Account{Id: account["_id"].(string), Name: account["name"].(string), Email: account["email"].(string), Password: account["password"].(string), Domain: account["domain"].(string)}
	if account["groups"] != nil {
		var groups []interface{}
		switch account["groups"].(type) {
		case primitive.A:
			groups = []interface{}(account["groups"].(primitive.A))
		case []interface{}:
			groups = []interface{}(account["groups"].([]interface{}))
		default:
			logger.Warn("unknown type", "value", account["groups"])
		}

		if groups != nil {
			for i := 0; i < len(groups); i++ {
				groupId := groups[i].(map[string]interface{})["$id"].(string)
				a.Groups = append(a.Groups, groupId)
			}
		}
	}

	if account["roles"] != nil {

		var roles []interface{}
		switch account["roles"].(type) {
		case primitive.A:
			roles = []interface{}(account["roles"].(primitive.A))
		case []interface{}:
			roles = []interface{}(account["roles"].([]interface{}))
		default:
			logger.Warn("unknown type", "value", account["roles"])
		}

		if roles != nil {
			for i := 0; i < len(roles); i++ {
				roleId := roles[i].(map[string]interface{})["$id"].(string)
				a.Roles = append(a.Roles, roleId)
			}
		}
	}

	if account["organizations"] != nil {
		var organizations []interface{}
		switch account["organizations"].(type) {
		case primitive.A:
			organizations = []interface{}(account["organizations"].(primitive.A))
		case []interface{}:
			organizations = []interface{}(account["organizations"].([]interface{}))
		default:
			logger.Warn("unknown type", "value", account["organizations"])
		}

		if organizations != nil {
			for i := 0; i < len(organizations); i++ {
				organizationId := organizations[i].(map[string]interface{})["$id"].(string)
				a.Organizations = append(a.Organizations, organizationId)
			}
		}
	}

	// Now the profile picture.

	// set the caller id.
	user_data, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Accounts", q, ``)
	if err == nil {
		// set the user infos....
		if user_data != nil {

			// Now I will get the user data from the user database.
			user_data_ := user_data.(map[string]interface{})
			if user_data_["profile_picture"] != nil {
				a.ProfilePicture = user_data_["profile_picture"].(string)
			}
			if user_data_["first_name"] != nil {
				a.FirstName = user_data_["first_name"].(string)
			}
			if user_data_["last_name"] != nil {
				a.LastName = user_data_["last_name"].(string)
			}
			if user_data_["middle_name"] != nil {
				a.Middle = user_data_["middle_name"].(string)
			}

			// try camel case.
			if user_data_["profilePicture"] != nil {
				a.ProfilePicture = user_data_["profilePicture"].(string)
			}
			if user_data_["firstName"] != nil {
				a.FirstName = user_data_["firstName"].(string)
			}
			if user_data_["lastName"] != nil {
				a.LastName = user_data_["lastName"].(string)
			}
			if user_data_["middleName"] != nil {
				a.Middle = user_data_["middleName"].(string)
			}
		}
	} else {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))

	}

	return &resourcepb.GetAccountRsp{
		Account: a, // Return the token string.
	}, nil

}

func (srv *server) getAccount(query string, options string) ([]*resourcepb.Account, error) {
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if query == "" {
		query = "{}"
	}

	accounts, err := p.Find(context.Background(), "local_resource", "local_resource", "Accounts", query, options)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	var results []*resourcepb.Account

	for _, acc := range accounts {
		account := acc.(map[string]interface{})
		lastName := ""
		firstName := ""
		middleName := ""
		profilePicture := ""

		if account["lastName"] != nil {
			lastName = account["lastName"].(string)
		}
		if account["firstName"] != nil {
			firstName = account["firstName"].(string)
		}
		if account["middleName"] != nil {
			middleName = account["middleName"].(string)
		}
		if account["profilePicture"] != nil {
			profilePicture = account["profilePicture"].(string)
		}

		a := &resourcepb.Account{
			Id:             account["_id"].(string),
			Name:           account["name"].(string),
			Email:          account["email"].(string),
			FirstName:      firstName,
			LastName:       lastName,
			Middle:         middleName,
			ProfilePicture: profilePicture,
			Domain: func() string {
				if account["domain"] != nil {
					return account["domain"].(string)
				}
				return srv.Domain
			}(),
		}

		// Process groups, roles, organizations
		processField := func(fieldName string, target *[]string) {
			if account[fieldName] != nil {
				var items []interface{}
				switch v := account[fieldName].(type) {
				case primitive.A:
					items = []interface{}(v)
				case []interface{}:
					items = v
				}

				for _, item := range items {
					if id, ok := item.(map[string]interface{})["$id"].(string); ok {
						*target = append(*target, id)
					}
				}
			}
		}

		processField("groups", &a.Groups)
		processField("roles", &a.Roles)
		processField("organizations", &a.Organizations)

		results = append(results, a)
	}

	return results, nil
}

// GetAccounts streams account information based on the provided query and options.
// It retrieves accounts using srv.getAccount, then sends them in batches of up to 100
// via the provided gRPC stream. If an error occurs during retrieval or streaming, it returns
// an appropriate gRPC status error. Any remaining accounts after batching are sent in a final message.
//
// Parameters:
//   - rqst: The request containing query and options for account retrieval.
//   - stream: The gRPC server stream to send account batches.
//
// Returns:
//   - error: An error if account retrieval or streaming fails, otherwise nil.
func (srv *server) GetAccounts(rqst *resourcepb.GetAccountsRqst, stream resourcepb.ResourceService_GetAccountsServer) error {
	accounts, err := srv.getAccount(rqst.Query, rqst.Options)
	if err != nil {
		return err
	}

	maxSize := 100
	values := make([]*resourcepb.Account, 0, maxSize)

	for _, a := range accounts {
		values = append(values, a)
		if len(values) >= maxSize {
			if err := stream.Send(&resourcepb.GetAccountsRsp{Accounts: values}); err != nil {
				return status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}
			values = values[:0]
		}
	}

	if len(values) > 0 {
		if err := stream.Send(&resourcepb.GetAccountsRsp{Accounts: values}); err != nil {
			return status.Errorf(codes.Internal, "%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	return nil
}

func (srv *server) getGroup(id string) (*resourcepb.Group, error) {

	p, err := srv.getPersistenceStore()

	if err != nil {
		return nil, err
	}

	q := `{"_id":"` + id + `"}`

	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Groups", q, ``)
	if err != nil {
		return nil, err
	}

	group := new(resourcepb.Group)

	if values != nil {
		group.Name = values.(map[string]interface{})["name"].(string)
		group.Id = values.(map[string]interface{})["_id"].(string)
		group.Description = values.(map[string]interface{})["description"].(string)
		group.Members = make([]string, 0)
		if values.(map[string]interface{})["domain"] != nil {
			group.Domain = values.(map[string]interface{})["domain"].(string)
		} else {
			group.Domain = srv.Domain
		}

		if values.(map[string]interface{})["members"] != nil {

			var members []interface{}
			switch values.(map[string]interface{})["members"].(type) {
			case primitive.A:
				members = []interface{}(values.(map[string]interface{})["members"].(primitive.A))
			case []interface{}:
				members = values.(map[string]interface{})["members"].([]interface{})
			}

			group.Members = make([]string, 0)
			for j := 0; j < len(members); j++ {
				group.Members = append(group.Members, members[j].(map[string]interface{})["$id"].(string))
			}
		}

		if values.(map[string]interface{})["organizations"] != nil {

			var organizations []interface{}
			switch values.(map[string]interface{})["organizations"].(type) {
			case primitive.A:
				organizations = []interface{}(values.(map[string]interface{})["organizations"].(primitive.A))
			case []interface{}:
				organizations = values.(map[string]interface{})["organizations"].([]interface{})
			}

			group.Organizations = make([]string, 0)
			for j := 0; j < len(organizations); j++ {
				group.Organizations = append(group.Organizations, organizations[j].(map[string]interface{})["$id"].(string))
			}
		}
		return group, nil
	} else {
		return nil, errors.New("group not found")
	}
}

// GetGroups streams groups from the persistence store based on the provided query and options.
// It retrieves group data, constructs resourcepb.Group objects, and sends them in batches over the gRPC stream.
// The method supports streaming members and organizations associated with each group.
// Returns an error if the persistence store cannot be accessed or if streaming fails.
func (srv *server) GetGroups(rqst *resourcepb.GetGroupsRqst, stream resourcepb.ResourceService_GetGroupsServer) error {
	// Get the persistence connection
	p, err := srv.getPersistenceStore()
	if err != nil {
		return err
	}

	query := rqst.Query
	if len(query) == 0 {
		query = "{}"
	}

	groups, err := p.Find(context.Background(), "local_resource", "local_resource", "Groups", query, rqst.Options)
	if err != nil {
		return status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// No I will stream the result over the networks.
	maxSize := 50
	values := make([]*resourcepb.Group, 0)

	for i := 0; i < len(groups); i++ {

		g := &resourcepb.Group{Name: groups[i].(map[string]interface{})["name"].(string), Id: groups[i].(map[string]interface{})["_id"].(string), Description: groups[i].(map[string]interface{})["description"].(string), Members: make([]string, 0)}
		if groups[i].(map[string]interface{})["domain"] != nil {
			g.Domain = groups[i].(map[string]interface{})["domain"].(string)
		} else {
			g.Domain = srv.Domain
		}

		if groups[i].(map[string]interface{})["members"] != nil {

			var members []interface{}
			switch groups[i].(map[string]interface{})["members"].(type) {
			case primitive.A:
				members = []interface{}(groups[i].(map[string]interface{})["members"].(primitive.A))
			case []interface{}:
				members = groups[i].(map[string]interface{})["members"].([]interface{})
			}

			g.Members = make([]string, 0)
			for j := 0; j < len(members); j++ {
				g.Members = append(g.Members, members[j].(map[string]interface{})["$id"].(string))
			}
		} else if groups[i].(map[string]interface{})["organizations"] != nil {

			var organizations []interface{}
			switch groups[i].(map[string]interface{})["organizations"].(type) {
			case primitive.A:
				organizations = []interface{}(groups[i].(map[string]interface{})["organizations"].(primitive.A))
			case []interface{}:
				organizations = groups[i].(map[string]interface{})["organizations"].([]interface{})
			}

			g.Organizations = make([]string, 0)
			for j := 0; j < len(organizations); j++ {
				g.Organizations = append(g.Organizations, organizations[j].(map[string]interface{})["$id"].(string))
			}

		}

		values = append(values, g)
		if len(values) >= maxSize {
			err := stream.Send(
				&resourcepb.GetGroupsRsp{
					Groups: values,
				},
			)
			if err != nil {
				return status.Errorf(
					codes.Internal,
					"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}
			values = make([]*resourcepb.Group, 0)
		}
	}

	// Send reminding values.
	err = stream.Send(
		&resourcepb.GetGroupsRsp{
			Groups: values,
		},
	)

	if err != nil {
		return status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return nil
}

// GetOrganizations streams a list of organizations matching the provided query and options.
// It retrieves organization data from the persistence store, constructs Organization protobuf objects,
// and sends them in batches over the provided gRPC stream. Each organization includes its groups, roles,
// accounts, and applications. If an error occurs during retrieval or streaming, an appropriate gRPC status
// error is returned.
//
// Parameters:
//   - rqst: The request containing the query and options for filtering organizations.
//   - stream: The gRPC stream to send batches of organizations.
//
// Returns:
//   - error: Returns a gRPC status error if any issue occurs during processing or streaming.
func (srv *server) GetOrganizations(rqst *resourcepb.GetOrganizationsRqst, stream resourcepb.ResourceService_GetOrganizationsServer) error {

	// Get the persistence connection
	p, err := srv.getPersistenceStore()
	if err != nil {
		return err
	}

	query := rqst.Query
	if len(query) == 0 {
		query = "{}"
	}

	organizations, err := p.Find(context.Background(), "local_resource", "local_resource", "Organizations", query, rqst.Options)
	if err != nil {
		return status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// No I will stream the result over the networks.
	maxSize := 50
	values := make([]*resourcepb.Organization, 0)
	for i := 0; i < len(organizations); i++ {
		o := organizations[i].(map[string]interface{})

		organization := new(resourcepb.Organization)
		organization.TypeName = "Organization"
		organization.Id = o["_id"].(string)
		organization.Name = o["name"].(string)
		organization.Icon = o["icon"].(string)
		organization.Description = o["description"].(string)
		organization.Email = o["email"].(string)
		if o["domain"] != nil {
			organization.Domain = o["domain"].(string)
		} else {
			organization.Domain = srv.Domain
		}

		// Here I will set the aggregation.

		// Groups
		if o["groups"] != nil {

			var groups []interface{}
			switch o["groups"].(type) {
			case primitive.A:
				groups = []interface{}(o["groups"].(primitive.A))
			case []interface{}:
				groups = o["groups"].([]interface{})
			}

			if groups != nil {
				for i := 0; i < len(groups); i++ {
					groupId := groups[i].(map[string]interface{})["$id"].(string)
					organization.Groups = append(organization.Groups, groupId)
				}
			}
		}

		// Roles
		if o["roles"] != nil {

			var roles []interface{}
			switch o["roles"].(type) {
			case primitive.A:
				roles = []interface{}(o["roles"].(primitive.A))
			case []interface{}:
				roles = o["roles"].([]interface{})
			}

			if roles != nil {
				for i := 0; i < len(roles); i++ {
					roleId := roles[i].(map[string]interface{})["$id"].(string)
					organization.Roles = append(organization.Roles, roleId)
				}
			}
		}

		// Accounts
		if o["accounts"] != nil {

			var accounts []interface{}
			switch o["accounts"].(type) {
			case primitive.A:
				accounts = []interface{}(o["accounts"].(primitive.A))
			case []interface{}:
				accounts = o["accounts"].([]interface{})
			}

			if accounts != nil {
				for i := 0; i < len(accounts); i++ {
					accountId := accounts[i].(map[string]interface{})["$id"].(string)
					organization.Accounts = append(organization.Accounts, accountId)
				}
			}
		}

		// Applications
		if o["applications"] != nil {

			var applications []interface{}
			switch o["applications"].(type) {
			case primitive.A:
				applications = []interface{}(o["applications"].(primitive.A))
			case []interface{}:
				applications = o["applications"].([]interface{})
			}

			if applications != nil {
				for i := 0; i < len(applications); i++ {
					applicationId := applications[i].(map[string]interface{})["$id"].(string)
					organization.Applications = append(organization.Applications, applicationId)
				}
			}
		}

		values = append(values, organization)
		if len(values) >= maxSize {
			err := stream.Send(
				&resourcepb.GetOrganizationsRsp{
					Organizations: values,
				},
			)
			if err != nil {
				return status.Errorf(
					codes.Internal,
					"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}
			values = make([]*resourcepb.Organization, 0)
		}
	}

	// Send reminding values.
	err = stream.Send(
		&resourcepb.GetOrganizationsRsp{
			Organizations: values,
		},
	)

	if err != nil {
		return status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return nil
}

func (srv *server) isOrganizationMemeber(account string, organization string) bool {
	// That service made user of persistence service.
	p, err := srv.getPersistenceStore()
	if err != nil {
		return false
	}

	q := `{"_id":"` + account + `"}`
	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Accounts", q, ``)
	if err != nil {
		return false
	}

	account_ := values.(map[string]interface{})
	if account_["organizations"] != nil {
		var organizations []interface{}
		switch account_["organizations"].(type) {
		case primitive.A:
			organizations = []interface{}(account_["organizations"].(primitive.A))
		case []interface{}:
			organizations = []interface{}(account_["organizations"].([]interface{}))
		default:
			logger.Warn("unknown type", "value", account_["organizations"])
		}

		for i := 0; i < len(organizations); i++ {
			organizationId := organizations[i].(map[string]interface{})["$id"].(string)
			if organization == organizationId {
				return true
			}
		}
	}

	return false

}

// IsOrgnanizationMember checks if the specified account is a member of the given organization.
// It takes a context and a request containing the account and organization IDs, and returns
// a response indicating membership status or an error if the operation fails.
func (srv *server) IsOrgnanizationMember(ctx context.Context, rqst *resourcepb.IsOrgnanizationMemberRqst) (*resourcepb.IsOrgnanizationMemberRsp, error) {
	result := srv.isOrganizationMemeber(rqst.AccountId, rqst.OrganizationId)

	return &resourcepb.IsOrgnanizationMemberRsp{
		Result: result,
	}, nil
}

/**
 * Register an Account.
 */
func (srv *server) registerAccount(ctx context.Context, domain, id, name, email, password, refresh_token, first_name, last_name, middle_name, profile_picture string, organizations []string, roles []string, groups []string) error {

	_, token, err := security.GetClientId(ctx)
	if err != nil {
		return err
	}

	localDomain, err := config.GetDomain()
	if err != nil {
		return err
	}

	if domain != localDomain {
		return errors.New("you cant register account with domain " + domain + " on domain " + localDomain)
	}

	// That service made user of persistence service.
	p, err := srv.getPersistenceStore()
	if err != nil {
		return err
	}

	// Check if the account already exist.
	q := `{"_id":"` + id + `"}`

	// first of all the Persistence service must be active.
	count, _ := p.Count(context.Background(), "local_resource", "local_resource", "Accounts", q, "")

	// one account already exist for the name we look for.
	if count == 1 && len(refresh_token) == 0 {
		return errors.New("account with name " + name + " already exist!")
	} else if count == 1 && len(refresh_token) != 0 {
		// so here I will update the account with the refresh token.
		err = p.UpdateOne(context.Background(), "local_resource", "local_resource", "Accounts", q, `{"$set":{"refresh_token":"`+refresh_token+`"}}`, "")
		if err != nil {
			fmt.Println("fail to update account with error ", err)
			return err
		}
		fmt.Println("account with name ", name, " has been updated with refresh token")
		return nil
	}

	// set the account object and set it basic roles.
	account := make(map[string]interface{})
	account["_id"] = id
	account["name"] = name
	account["email"] = email
	account["domain"] = domain

	if len(refresh_token) == 0 {
		account["password"], err = srv.hashPassword(password) // hide the password...
		if err != nil {
			fmt.Println("fail to hash password with error ", err)
			return err
		}

	} else {
		account["refresh_token"] = refresh_token
		account["password"] = ""
	}

	// List of aggregation.
	account["roles"] = make([]interface{}, 0)
	account["groups"] = make([]interface{}, 0)
	account["organizations"] = make([]interface{}, 0)
	account["typeName"] = "Account"
	account["first_name"] = first_name
	account["last_name"] = last_name
	account["middle_name"] = ""
	account["email"] = email
	account["profile_picture"] = profile_picture

	// Here I will insert the account in the database.
	_, err = p.InsertOne(context.Background(), "local_resource", "local_resource", "Accounts", account, "")
	if err != nil {
		fmt.Printf("fail to create account %s with error %s", name, err.Error())
		return err
	}

	// replace @ and . by _  * active directory
	name = strings.ReplaceAll(strings.ReplaceAll(name, "@", "_"), ".", "_")

	// Each account will have their own database and a use that can read and write

	// Organizations
	for i := range organizations {
		if !strings.Contains(organizations[i], "@") {
			organizations[i] = organizations[i] + "@" + localDomain
		}

		srv.createCrossReferences(organizations[i], "Organizations", "accounts", id, "Accounts", "organizations")
	}

	// Roles
	for i := range roles {
		if !strings.Contains(roles[i], "@") {
			roles[i] = roles[i] + "@" + localDomain
		}
		srv.createCrossReferences(roles[i], "Roles", "members", id, "Accounts", "roles")
	}

	// Groups
	for i := range groups {
		if !strings.Contains(groups[i], "@") {
			groups[i] = groups[i] + "@" + localDomain
		}
		srv.createCrossReferences(groups[i], "Groups", "members", id, "Accounts", "groups")
	}

	// Create the user file directory.
	path := "/users/" + id + "@" + localDomain
	Utility.CreateDirIfNotExist(config.GetDataDir() + "/files" + path)
	err = srv.addResourceOwner(token, path, id+"@"+localDomain, "file", rbacpb.SubjectType_ACCOUNT)
	if err != nil {
		fmt.Println("fail to add resource owner with error ", err)
	}

	// Now I will allocate the new account disk space.
	srv.SetAccountAllocatedSpace(token, id, 0)

	return err
}

// RegisterAccount handles the registration of a new account.
// It supports both regular and OAuth-based account registration.
// For regular accounts, it verifies password confirmation before proceeding.
// The method registers the account, generates an authentication token, validates the token,
// updates the user session state, and publishes an account creation event.
// Returns a RegisterAccountRsp containing the authentication token or an error if registration fails.
//
// Parameters:
//
//	ctx  - context for the request.
//	rqst - RegisterAccountRqst containing account details.
//
// Returns:
//
//	*resourcepb.RegisterAccountRsp - response containing the authentication token.
//	error                          - error if registration fails.
func (srv *server) RegisterAccount(ctx context.Context, rqst *resourcepb.RegisterAccountRqst) (*resourcepb.RegisterAccountRsp, error) {
	var account *resourcepb.Account
	// Regular account registration
	if rqst.Account != nil {
		account = rqst.Account

		// Confirm password
		if len(rqst.Account.RefreshToken) == 0 {
			if rqst.ConfirmPassword != account.Password {
				return nil, status.Errorf(
					codes.Internal,
					"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("fail to confirm your password")),
				)
			}
		}

	} else {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no account or OAuth account was provided")),
		)
	}

	// Register the account in the system
	err := srv.registerAccount(ctx,
		account.Domain, account.Id, account.Name, account.Email, account.Password, account.RefreshToken,
		account.FirstName, account.LastName, account.Middle, account.ProfilePicture,
		account.Organizations, account.Roles, account.Groups,
	)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err),
		)
	}

	// Generate a token for the new account
	tokenString, err := security.GenerateToken(srv.SessionTimeout, srv.Mac, account.Id, account.Name, account.Email, account.Domain)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err),
		)
	}

	// Validate the token...
	claims, err := security.ValidateToken(tokenString)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err),
		)
	}

	// Update user session.
	err = srv.updateSession(claims.ID+"@"+claims.UserDomain, resourcepb.SessionState_ONLINE, time.Now().Unix(), claims.RegisteredClaims.ExpiresAt.Unix())
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err),
		)
	}

	// Publish account creation event
	jsonStr, err := protojson.Marshal(account)
	if err == nil {
		srv.publishEvent("create_account_evt", jsonStr, srv.Address)
	}

	// Return the generated authentication token
	return &resourcepb.RegisterAccountRsp{
		Result: tokenString,
	}, nil
}

// RemoveGroupMemberAccount removes an account from a group by deleting the references
// between the account and the group in the persistence store. It also publishes events
// to notify that the group and account have been updated.
//
// Parameters:
//   - ctx: The context for the request.
//   - rqst: The request containing the AccountId and GroupId.
//
// Returns:
//   - *resourcepb.RemoveGroupMemberAccountRsp: The response indicating the result of the operation.
//   - error: An error if the removal fails.
//
// The function performs the following steps:
//  1. Retrieves the persistence store.
//  2. Deletes the reference from the account to the group ("members" in "Groups").
//  3. Deletes the reference from the group to the account ("groups" in "Accounts").
//  4. Publishes update events for both the group and the account.
func (srv *server) RemoveGroupMemberAccount(ctx context.Context, rqst *resourcepb.RemoveGroupMemberAccountRqst) (*resourcepb.RemoveGroupMemberAccountRsp, error) {
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	// That service made user of persistence service.
	err = srv.deleteReference(p, rqst.AccountId, rqst.GroupId, "members", "Groups")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = srv.deleteReference(p, rqst.GroupId, rqst.AccountId, "groups", "Accounts")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	srv.publishEvent("update_group_"+rqst.GroupId+"_evt", []byte{}, srv.Address)
	srv.publishEvent("update_account_"+rqst.AccountId+"_evt", []byte{}, srv.Address)

	return &resourcepb.RemoveGroupMemberAccountRsp{Result: true}, nil
}

// RemoveOrganizationAccount removes the association between an account and an organization.
// It deletes the references in both the account's and organization's records, publishes update events,
// and returns a response indicating the result.
// Returns an error if the persistence store cannot be accessed or if reference deletion fails.
func (srv *server) RemoveOrganizationAccount(ctx context.Context, rqst *resourcepb.RemoveOrganizationAccountRqst) (*resourcepb.RemoveOrganizationAccountRsp, error) {
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	err = srv.deleteReference(p, rqst.AccountId, rqst.OrganizationId, "accounts", "Organizations")
	if err != nil {
		return nil, err
	}

	err = srv.deleteReference(p, rqst.OrganizationId, rqst.AccountId, "organizations", "Accounts")
	if err != nil {
		return nil, err
	}

	srv.publishEvent("update_organization_"+rqst.OrganizationId+"_evt", []byte{}, srv.Address)
	srv.publishEvent("update_account_"+rqst.AccountId+"_evt", []byte{}, srv.Address)

	return &resourcepb.RemoveOrganizationAccountRsp{Result: true}, nil
}

// RemoveOrganizationApplication removes the association between an organization and an application.
// It deletes the references from both the organization's and application's records in the persistence store.
// After successful removal, it publishes update events for both the organization and the application.
// Returns a response indicating the result of the operation or an error if the removal fails.
func (srv *server) RemoveOrganizationApplication(ctx context.Context, rqst *resourcepb.RemoveOrganizationApplicationRqst) (*resourcepb.RemoveOrganizationApplicationRsp, error) {
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	err = srv.deleteReference(p, rqst.ApplicationId, rqst.OrganizationId, "applications", "Organizations")
	if err != nil {
		return nil, err
	}

	err = srv.deleteReference(p, rqst.OrganizationId, rqst.ApplicationId, "organizations", "Applications")
	if err != nil {
		return nil, err
	}

	srv.publishEvent("update_organization_"+rqst.OrganizationId+"_evt", []byte{}, srv.Address)
	srv.publishEvent("update_application_"+rqst.ApplicationId+"_evt", []byte{}, srv.Address)

	return &resourcepb.RemoveOrganizationApplicationRsp{Result: true}, nil
}

// RemoveOrganizationGroup removes the association between a group and an organization.
// It deletes the references in both directions: from the group to the organization and from the organization to the group.
// After successful removal, it publishes update events for both the organization and the group.
// Returns a response indicating the result of the operation or an error if any step fails.
func (srv *server) RemoveOrganizationGroup(ctx context.Context, rqst *resourcepb.RemoveOrganizationGroupRqst) (*resourcepb.RemoveOrganizationGroupRsp, error) {
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	err = srv.deleteReference(p, rqst.GroupId, rqst.OrganizationId, "groups", "Organizations")
	if err != nil {
		return nil, err
	}

	err = srv.deleteReference(p, rqst.OrganizationId, rqst.GroupId, "organizations", "Groups")
	if err != nil {
		return nil, err
	}

	srv.publishEvent("update_organization_"+rqst.OrganizationId+"_evt", []byte{}, srv.Address)
	srv.publishEvent("update_group_"+rqst.GroupId+"_evt", []byte{}, srv.Address)

	return &resourcepb.RemoveOrganizationGroupRsp{Result: true}, nil
}

// SetAccount updates an account in the resource server.
//
// It takes a context and a SetAccountRqst containing the account information to be updated.
// Returns a SetAccountRsp on success, or an error if the update fails.
func (srv *server) SetAccount(ctx context.Context, rqst *resourcepb.SetAccountRqst) (*resourcepb.SetAccountRsp, error) {
	if err := srv.updateAccount(ctx, rqst.Account); err != nil {
		return nil, err
	}
	return &resourcepb.SetAccountRsp{}, nil
}

// SetAccountContact sets or updates the contact information for a specific account.
// It validates the input, ensures the account belongs to the local domain, and persists
// the contact data in the appropriate database. If successful, it publishes update events
// for both the contact and the account.
//
// Parameters:
//
//	ctx - The context for the request.
//	rqst - The request containing the account ID and contact information.
//
// Returns:
//
//	*resourcepb.SetAccountContactRsp - The response indicating success.
//	error - An error if the operation fails.
//
// Errors:
//
//	Returns an error if the contact is missing, the account domain is invalid,
//	persistence store cannot be accessed, or the database operation fails.
func (srv *server) SetAccountContact(ctx context.Context, rqst *resourcepb.SetAccountContactRqst) (*resourcepb.SetAccountContactRsp, error) {

	if rqst.Contact == nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no contact was given")))
	}

	// Get the persistence connection
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	accountId := rqst.AccountId
	if strings.Contains(accountId, "@") {
		domain := strings.Split(accountId, "@")[1]
		localDomain, _ := config.GetDomain()
		if domain != localDomain {
			return nil, status.Errorf(
				codes.Internal,
				"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("fail to get account "+accountId+" with domain "+domain+" from globule at domain "+localDomain)))
		}
		accountId = strings.Split(accountId, "@")[0]
	}

	// set the account id.
	q := `{"_id":"` + rqst.Contact.Id + `"}`

	sentInvitation := `{"_id":"` + rqst.Contact.Id + `", "invitationTime":` + Utility.ToString(rqst.Contact.InvitationTime) + `, "status":"` + rqst.Contact.Status + `", "ringtone":"` + rqst.Contact.Ringtone + `", "profilePicture":"` + rqst.Contact.ProfilePicture + `"}`

	err = p.ReplaceOne(context.Background(), "local_resource", "local_resource", "Contacts", q, sentInvitation, `[{"upsert":true}]`)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	srv.publishEvent("update_account_"+rqst.Contact.Id+"_evt", []byte{}, srv.Address)
	srv.publishEvent("update_account_"+rqst.AccountId+"_evt", []byte{}, srv.Address)

	return &resourcepb.SetAccountContactRsp{Result: true}, nil
}

// SetAccountPassword updates the password for a specified account.
// It validates the old password (unless the request is from the 'sa' client),
// changes the password in the underlying persistence store (MongoDB, ScyllaDB, or SQL),
// hashes the new password, and updates it in the account record.
// If the 'sa' account password is changed, it updates the backend password and reconnects to the persistence service.
// Returns an error if any step fails.
//
// Parameters:
//
//	ctx - The context for the request.
//	rqst - The request containing the account ID, old password, and new password.
//
// Returns:
//
//	*resourcepb.SetAccountPasswordRsp - The response object.
//	error - An error if the operation fails.
func (srv *server) SetAccountPassword(ctx context.Context, rqst *resourcepb.SetAccountPasswordRqst) (*resourcepb.SetAccountPasswordRsp, error) {

	clientId, _, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}

	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	q := `{"_id":"` + rqst.AccountId + `"}`

	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Accounts", q, ``)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	account := values.(map[string]interface{})

	// Now update the sa password in mongo db.
	name := account["name"].(string)
	name = strings.ReplaceAll(strings.ReplaceAll(name, ".", "_"), "@", "_")

	fmt.Println("change password for account ", rqst.AccountId, " with name ", name, " requested by ", clientId)
	// In case the request doesn't came from the sa or sa@
	if clientId != "sa" && !strings.HasPrefix(clientId, "sa@") {
		err = srv.validatePassword(rqst.OldPassword, account["password"].(string))
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	if clientId == "sa" && (rqst.AccountId == "sa" || strings.HasPrefix(rqst.AccountId, "sa@")) {
		var changePasswordScript string
		if p.GetStoreType() == "MONGO" {
			changePasswordScript = fmt.Sprintf("db=db.getSiblingDB('admin');db.changeUserPassword('%s','%s');", name, rqst.NewPassword)
		} else if p.GetStoreType() == "SCYLLA" {
			changePasswordScript = fmt.Sprintf("ALTER USER '%s' WITH PASSWORD '%s';", name, rqst.NewPassword)
		} else if p.GetStoreType() == "SQL" {
			changePasswordScript = fmt.Sprintf("ALTER USER '%s' WITH PASSWORD '%s';", name, rqst.NewPassword)
		} else {
			return nil, status.Errorf(
				codes.Internal,
				"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("unknown database type "+p.GetStoreType())))
		}

		// Change the password...
		err = p.RunAdminCmd(context.Background(), "local_resource", srv.Backend_user, srv.Backend_password, changePasswordScript)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	// Create bcrypt...
	pwd, err := srv.hashPassword(rqst.NewPassword)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// so here the sa password has change so I need to update the backend password and reconnect to the persistence service.
	if clientId == "sa" && (rqst.AccountId == "sa" || strings.HasPrefix(rqst.AccountId, "sa@")) {
		srv.Backend_password = rqst.NewPassword
		srv.Save()

		// reconnect...
		srv.store = nil
		p, err = srv.getPersistenceStore()
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	setPassword := map[string]interface{}{"$set": map[string]interface{}{"password": string(pwd)}}
	setPassword_, _ := Utility.ToJson(setPassword)

	// Hash the password...
	err = p.UpdateOne(context.Background(), "local_resource", "local_resource", "Accounts", q, setPassword_, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &resourcepb.SetAccountPasswordRsp{}, nil
}

// SetEmail updates the email address of an account after verifying the old email.
// It retrieves the account information from the persistence store, checks if the provided old email matches,
// updates the email to the new value, and saves the changes back to the database.
// An event is published to notify about the account update.
// Returns a SetEmailResponse on success or an error if the operation fails.
//
// Parameters:
//
//	ctx - the context for the request.
//	rqst - the request containing AccountId, OldEmail, and NewEmail.
//
// Returns:
//
//	*resourcepb.SetEmailResponse - the response object.
//	error - error if the operation fails.
func (srv *server) SetEmail(ctx context.Context, rqst *resourcepb.SetEmailRequest) (*resourcepb.SetEmailResponse, error) {

	// Here I will set the root password.
	// First of all I will get the user information from the database.
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	accountId := rqst.AccountId

	q := `{"_id":"` + accountId + `"}`

	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Accounts", q, ``)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	account := values.(map[string]interface{})

	if account["email"].(string) != rqst.OldEmail {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("wrong email given")))
	}

	account["email"] = rqst.NewEmail

	// Here I will save the role.
	jsonStr := "{"
	jsonStr += `"name":"` + account["name"].(string) + `",`
	jsonStr += `"domain":"` + account["domain"].(string) + `",`
	jsonStr += `"email":"` + account["email"].(string) + `",`
	jsonStr += `"password":"` + account["password"].(string) + `",`
	jsonStr += `"roles":[`

	var roles []interface{}
	switch account["roles"].(type) {
	case primitive.A:
		roles = []interface{}(account["roles"].(primitive.A))
	case []interface{}:
		roles = []interface{}(account["roles"].([]interface{}))
	default:
		logger.Warn("unknown type", "value", account["roles"])
	}

	for j := 0; j < len(roles); j++ {
		db := roles[j].(map[string]interface{})["$db"].(string)
		db = strings.ReplaceAll(db, "@", "_")
		db = strings.ReplaceAll(db, ".", "_")
		jsonStr += `{`
		jsonStr += `"$ref":"` + roles[j].(map[string]interface{})["$ref"].(string) + `",`
		jsonStr += `"$id":"` + roles[j].(map[string]interface{})["$id"].(string) + `",`
		jsonStr += `"$db":"` + db + `"`
		jsonStr += `}`
		if j < len(roles)-1 {
			jsonStr += `,`
		}
	}
	jsonStr += `]`
	jsonStr += "}"

	err = p.ReplaceOne(context.Background(), "local_resource", "local_resource", "Accounts", q, jsonStr, ``)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	srv.publishEvent("update_account_"+rqst.AccountId+"_evt", []byte{}, srv.Address)

	// Return the token.
	return &resourcepb.SetEmailResponse{}, nil
}

func (srv *server) updateAccount(ctx context.Context, account *resourcepb.Account) error {
	p, err := srv.getPersistenceStore()
	if err != nil {
		return status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err),
		)
	}

	// Set the query.
	q := `{"_id":"` + account.Id + `"}`

	// Update main account information.
	setAccount := map[string]interface{}{
		"$set": map[string]interface{}{
			"name":  account.Name,
			"email": account.Email,
		},
	}
	setAccount_, _ := Utility.ToJson(setAccount)

	err = p.UpdateOne(ctx, "local_resource", "local_resource", "Accounts", q, setAccount_, "")
	if err != nil {
		return status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err),
		)
	}

	// Update user-specific data.
	setUserData := map[string]interface{}{
		"$set": map[string]interface{}{
			"profile_picture": account.ProfilePicture,
			"first_name":      account.FirstName,
			"last_name":       account.LastName,
			"middle_name":     account.Middle,
		},
	}
	setUserData_, _ := Utility.ToJson(setUserData)

	err = p.UpdateOne(ctx, "local_resource", "local_resource", "Accounts", q, setUserData_, "")
	if err != nil {
		return status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err),
		)
	}

	return nil
}

// UpdateGroup updates the details of a group identified by GroupId in the persistence store.
// It first checks if the group exists, and if so, updates its values with the provided data.
// Publishes an event after a successful update.
// Returns a response indicating the result of the operation or an error if the update fails.
//
// Parameters:
//
//	ctx - The context for the request.
//	rqst - The request containing the GroupId and new values.
//
// Returns:
//
//	*resourcepb.UpdateGroupRsp - The response indicating success.
//	error - An error if the operation fails.
func (srv *server) UpdateGroup(ctx context.Context, rqst *resourcepb.UpdateGroupRqst) (*resourcepb.UpdateGroupRsp, error) {
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	q := `{"_id":"` + rqst.GroupId + `"}`

	// Get the persistence connection
	count, err := p.Count(context.Background(), "local_resource", "local_resource", "Groups", q, "")
	if err != nil || count == 0 {
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	} else {
		err = p.UpdateOne(context.Background(), "local_resource", "local_resource", "Groups", q, rqst.Values, "")
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	srv.publishEvent("update_group_"+rqst.GroupId+"_evt", []byte{}, srv.Address)

	return &resourcepb.UpdateGroupRsp{
		Result: true,
	}, nil
}

// UpdateOrganization updates the details of an organization identified by OrganizationId.
// It first checks if the organization exists in the persistence store. If it does, it updates
// the organization's values with the provided data. After a successful update, an event is published.
// Returns a response indicating the result of the operation or an error if the update fails.
//
// Parameters:
//
//	ctx - The context for the request.
//	rqst - The request containing the OrganizationId and the new values.
//
// Returns:
//
//	*resourcepb.UpdateOrganizationRsp - The response indicating success.
//	error - An error if the operation fails.
func (srv *server) UpdateOrganization(ctx context.Context, rqst *resourcepb.UpdateOrganizationRqst) (*resourcepb.UpdateOrganizationRsp, error) {
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	q := `{"_id":"` + rqst.OrganizationId + `"}`

	// Get the persistence connection
	count, err := p.Count(context.Background(), "local_resource", "local_resource", "Organizations", q, "")
	if err != nil || count == 0 {
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	} else {

		err = p.UpdateOne(context.Background(), "local_resource", "local_resource", "Organizations", q, rqst.Values, "")
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	srv.publishEvent("update_organization_"+rqst.OrganizationId+"_evt", []byte{}, srv.Address)

	return &resourcepb.UpdateOrganizationRsp{
		Result: true,
	}, nil
}
