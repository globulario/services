package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"sort"

	//"reflect"
	"strings"
	"time"

	"encoding/json"
	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/interceptors"
	"github.com/globulario/services/golang/persistence/persistence_store"
	"github.com/globulario/services/golang/resource/resourcepb"

	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/golang/protobuf/jsonpb"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/structpb"
)

func (resource_server *server) deleteReference(p persistence_store.Store, refId, targetId, targetField, targetCollection string) error {

	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", targetCollection, `{"_id":"`+targetId+`"}`, ``)
	if err != nil {
		return err
	}

	target := values.(map[string]interface{})

	if target[targetField] == nil {
		return errors.New("No field named " + targetField + " was found in object with id " + targetId + "!")
	}

	references := []interface{}(target[targetField].(primitive.A))
	references_ := make([]interface{}, 0)
	for j := 0; j < len(references); j++ {
		if references[j].(map[string]interface{})["$id"] != refId {
			references_ = append(references_, references[j])
		}
	}

	target[targetField] = references_
	jsonStr := serialyseObject(target)

	err = p.ReplaceOne(context.Background(), "local_resource", "local_resource", targetCollection, `{"_id":"`+targetId+`"}`, jsonStr, ``)
	if err != nil {
		return err
	}

	return nil
}

func (resource_server *server) createReference(p persistence_store.Store, id, sourceCollection, field, targetId, targetCollection string) error {
	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", sourceCollection, `{"_id":"`+id+`"}`, ``)
	if err != nil {
		return err
	}

	log.Println("create reference ", id, " source ", sourceCollection, " field ", field, " field ", targetId, " target ", targetCollection)
	source := values.(map[string]interface{})
	references := make([]interface{}, 0)
	if source[field] != nil {
		references = []interface{}(source[field].(primitive.A))
	}

	for j := 0; j < len(references); j++ {
		if references[j].(map[string]interface{})["$id"] == targetId {
			return errors.New(" named " + targetId + " aleready exist in  " + field + "!")
		}
	}

	// append the account.
	source[field] = append(references, map[string]interface{}{"$ref": targetCollection, "$id": targetId, "$db": "local_resource"})
	jsonStr := serialyseObject(source)

	err = p.ReplaceOne(context.Background(), "local_resource", "local_resource", sourceCollection, `{"_id":"`+id+`"}`, jsonStr, ``)
	if err != nil {
		return err
	}

	return nil
}

func (resource_server *server) createCrossReferences(sourceId, sourceCollection, sourceField, targetId, targetCollection, targetField string) error {
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return err
	}

	err = resource_server.createReference(p, targetId, targetCollection, targetField, sourceId, sourceCollection)
	if err != nil {
		//return err
		log.Println(err)
	}

	err = resource_server.createReference(p, sourceId, sourceCollection, sourceField, targetId, targetCollection)

	return err

}

func (resource_server *server) registerAccount(id string, name string, email string, password string, organizations []string, contacts []string, roles []string, groups []string) error {

	// That service made user of persistence service.
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return err
	}

	// first of all the Persistence service must be active.
	count, err := p.Count(context.Background(), "local_resource", "local_resource", "Accounts", `{"$or":[{"_id":"`+id+`"},{"name":"`+id+`"} ]}`, "")
	if err != nil {
		return err
	}

	// one account already exist for the name.
	if count == 1 {
		return errors.New("account with name " + name + " already exist!")
	}

	// set the account object and set it basic roles.
	account := make(map[string]interface{}, 0)
	account["_id"] = id
	account["name"] = name
	account["email"] = email
	account["password"] = Utility.GenerateUUID(password) // hide the password...

	// List of aggregation.
	account["roles"] = make([]interface{}, 0)
	account["groups"] = make([]interface{}, 0)
	account["organizations"] = make([]interface{}, 0)

	// append guest role if not already exist.
	if !Utility.Contains(roles, "guest") {
		roles = append(roles, "guest")
	}

	// Here I will insert the account in the database.
	_, err = p.InsertOne(context.Background(), "local_resource", "local_resource", "Accounts", account, "")
	if err != nil {
		return err
	}

	// replace @ and . by _
	name = strings.ReplaceAll(strings.ReplaceAll(name, "@", "_"), ".", "_")

	// Each account will have their own database and a use that can read and write
	// into it.
	// Here I will wrote the script for mongoDB...
	createUserScript := fmt.Sprintf("db=db.getSiblingDB('%s_db');db.createCollection('user_data');db=db.getSiblingDB('admin');db.createUser({user: '%s', pwd: '%s',roles: [{ role: 'dbOwner', db: '%s_db' }]});", name, name, password, name)

	// I will execute the sript with the admin function.
	err = p.RunAdminCmd(context.Background(), "local_resource", resource_server.Backend_user, resource_server.Backend_password, createUserScript)
	if err != nil {
		return err
	}

	/* TODO create connection...
	err = p.CreateConnection(name+"_db", name+"_db", address, float64(port), 0, name, password, 5000, "", false)
	if err != nil {
		return errors.New("no persistence service are available to store resource information")
	}
	*/

	// Now I will set the reference for
	// Contact...
	for i := 0; i < len(contacts); i++ {
		resource_server.addAccountContact(id, contacts[i])
	}

	// Organizations
	for i := 0; i < len(organizations); i++ {
		resource_server.createCrossReferences(organizations[i], "Organizations", "accounts", id, "Accounts", "organizations")
	}

	// Roles
	for i := 0; i < len(roles); i++ {
		resource_server.createCrossReferences(roles[i], "Roles", "members", id, "Accounts", "roles")
	}

	// Groups
	for i := 0; i < len(groups); i++ {
		resource_server.createCrossReferences(groups[i], "Groups", "members", id, "Accounts", "groups")
	}

	return nil

}

// Set the root password
func (resource_server *server) SetEmail(ctx context.Context, rqst *resourcepb.SetEmailRequest) (*resourcepb.SetEmailResponse, error) {

	// Here I will set the root password.
	// First of all I will get the user information from the database.
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	accountId := rqst.AccountId
	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Accounts", `{"$or":[{"_id":"`+accountId+`"},{"name":"`+accountId+`"} ]}`, ``)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	account := values.(map[string]interface{})

	if account["email"].(string) != rqst.OldEmail {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("wrong email given")))
	}

	account["email"] = rqst.NewEmail

	// Here I will save the role.
	jsonStr := "{"
	jsonStr += `"name":"` + account["name"].(string) + `",`
	jsonStr += `"email":"` + account["email"].(string) + `",`
	jsonStr += `"password":"` + account["password"].(string) + `",`
	jsonStr += `"roles":[`
	account["roles"] = []interface{}(account["roles"].(primitive.A))
	for j := 0; j < len(account["roles"].([]interface{})); j++ {
		db := account["roles"].([]interface{})[j].(map[string]interface{})["$db"].(string)
		db = strings.ReplaceAll(db, "@", "_")
		db = strings.ReplaceAll(db, ".", "_")
		jsonStr += `{`
		jsonStr += `"$ref":"` + account["roles"].([]interface{})[j].(map[string]interface{})["$ref"].(string) + `",`
		jsonStr += `"$id":"` + account["roles"].([]interface{})[j].(map[string]interface{})["$id"].(string) + `",`
		jsonStr += `"$db":"` + db + `"`
		jsonStr += `}`
		if j < len(account["roles"].([]interface{}))-1 {
			jsonStr += `,`
		}
	}
	jsonStr += `]`
	jsonStr += "}"

	// set the new email.
	account["email"] = rqst.NewEmail

	err = p.ReplaceOne(context.Background(), "local_resource", "local_resource", "Accounts", `{"name":"`+account["name"].(string)+`"}`, jsonStr, ``)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Return the token.
	return &resourcepb.SetEmailResponse{}, nil
}

/* Register a new Account */
func (resource_server *server) RegisterAccount(ctx context.Context, rqst *resourcepb.RegisterAccountRqst) (*resourcepb.RegisterAccountRsp, error) {
	if rqst.ConfirmPassword != rqst.Account.Password {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("fail to confirm your password")))

	}

	if rqst.Account == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no account information was given")))

	}

	err := resource_server.registerAccount(rqst.Account.Name, rqst.Account.Name, rqst.Account.Email, rqst.Account.Password, rqst.Account.Organizations, rqst.Account.Contacts, rqst.Account.Roles, rqst.Account.Groups)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Generate a token to identify the user.
	tokenString, err := interceptors.GenerateToken(resource_server.jwtKey, resource_server.SessionTimeout, rqst.Account.Id, rqst.Account.Name, rqst.Account.Email)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	id, _, _, expireAt, _ := interceptors.ValidateToken(tokenString)
	_, err = p.InsertOne(context.Background(), "local_resource", "local_resource", "Tokens", map[string]interface{}{"_id": id, "expireAt": Utility.ToString(expireAt)}, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Now I will
	return &resourcepb.RegisterAccountRsp{
		Result: tokenString, // Return the token string.
	}, nil
}

//* Return the list accounts *
func (resource_server *server) GetAccounts(rqst *resourcepb.GetAccountsRqst, stream resourcepb.ResourceService_GetAccountsServer) error {
	// Get the persistence connection
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return err
	}

	query := rqst.Query
	if len(query) == 0 {
		query = "{}"
	}

	accounts, err := p.Find(context.Background(), "local_resource", "local_resource", "Accounts", query, ``)
	if err != nil {
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// No I will stream the result over the networks.
	maxSize := 100
	values := make([]*resourcepb.Account, 0)

	for i := 0; i < len(accounts); i++ {
		account := accounts[i].(map[string]interface{})
		a := &resourcepb.Account{Id: account["_id"].(string), Name: account["name"].(string), Email: account["email"].(string)}

		if account["groups"] != nil {
			groups := []interface{}(account["groups"].(primitive.A))
			if groups != nil {
				for i := 0; i < len(groups); i++ {
					groupId := groups[i].(map[string]interface{})["$id"].(string)
					a.Groups = append(a.Groups, groupId)
				}
			}
		}

		if account["roles"] != nil {
			roles := []interface{}(account["roles"].(primitive.A))
			if roles != nil {
				for i := 0; i < len(roles); i++ {
					roleId := roles[i].(map[string]interface{})["$id"].(string)
					a.Roles = append(a.Roles, roleId)
				}
			}
		}

		if account["organizations"] != nil {
			organizations := []interface{}(account["organizations"].(primitive.A))
			if organizations != nil {
				for i := 0; i < len(organizations); i++ {
					organizationId := organizations[i].(map[string]interface{})["$id"].(string)
					a.Organizations = append(a.Organizations, organizationId)
				}
			}
		}

		if account["contacts"] != nil {
			contacts := []interface{}(account["contacts"].(primitive.A))
			if contacts != nil {
				for i := 0; i < len(contacts); i++ {
					contactId := contacts[i].(map[string]interface{})["$id"].(string)
					a.Contacts = append(a.Contacts, contactId)
				}
			}
		}

		values = append(values, a)

		if len(values) >= maxSize {
			err := stream.Send(
				&resourcepb.GetAccountsRsp{
					Accounts: values,
				},
			)
			if err != nil {
				return status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}
			values = make([]*resourcepb.Account, 0)
		}
	}

	// Send reminding values.
	err = stream.Send(
		&resourcepb.GetAccountsRsp{
			Accounts: values,
		},
	)

	if err != nil {
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return nil
}

func (resource_server *server) addAccountContact(accountId string, contactId string) error {

	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return err
	}

	err = resource_server.createReference(p, accountId, "Accounts", "contacts", contactId, "Accounts")
	if err != nil {
		return err
	}

	return nil
}

//* Add contact to a given account *
func (resource_server *server) AddAccountContact(ctx context.Context, rqst *resourcepb.AddAccountContactRqst) (*resourcepb.AddAccountContactRsp, error) {

	err := resource_server.addAccountContact(rqst.AccountId, rqst.ContactId)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &resourcepb.AddAccountContactRsp{Result: true}, nil
}

//* Remove a contact from a given account *
func (resource_server *server) RemoveAccountContact(ctx context.Context, rqst *resourcepb.RemoveAccountContactRqst) (*resourcepb.RemoveAccountContactRsp, error) {
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	// That service made user of persistence service.
	err = resource_server.deleteReference(p, rqst.AccountId, rqst.ContactId, "contacts", "Accounts")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &resourcepb.RemoveAccountContactRsp{Result: true}, nil
}

func (resource_server *server) AccountExist(ctx context.Context, rqst *resourcepb.AccountExistRqst) (*resourcepb.AccountExistRsp, error) {
	var exist bool
	// Get the persistence connection
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return nil, err
	}
	// Test with the _id
	accountId := rqst.Id
	count, _ := p.Count(context.Background(), "local_resource", "local_resource", "Accounts", `{"$or":[{"_id":"`+accountId+`"},{"name":"`+accountId+`"} ]}`, "")
	if count > 0 {
		exist = true
	}

	// Test with the name
	if !exist {
		count, _ := p.Count(context.Background(), "local_resource", "local_resource", "Accounts", `{"$or":[{"_id":"`+accountId+`"},{"name":"`+accountId+`"} ]}`, "")
		if count > 0 {
			exist = true
		}
	}

	// Test with the email.
	if !exist {
		count, _ := p.Count(context.Background(), "local_resource", "local_resource", "Accounts", `{"email":"`+rqst.Id+`"}`, "")
		if count > 0 {
			exist = true
		}
	}
	if exist {
		return &resourcepb.AccountExistRsp{
			Result: true,
		}, nil
	}

	return nil, status.Errorf(
		codes.Internal,
		Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("Account with id name or email '"+rqst.Id+"' dosent exist!")))

}

// Test if account is a member of organisation.
func (resource_server *server) isOrganizationMemeber(account string, organization string) bool {
	// That service made user of persistence service.
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return false
	}

	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Accounts", `{"$or":[{"_id":"`+account+`"},{"name":"`+account+`"} ]}`, ``)
	if err != nil {
		return false
	}

	account_ := values.(map[string]interface{})
	if account_["organizations"] != nil {
		organizations := []interface{}(account_["organizations"].(primitive.A))
		for i := 0; i < len(organizations); i++ {
			organizationId := organizations[i].(map[string]interface{})["$id"].(string)
			if organization == organizationId {
				return true
			}
		}
	}

	return false

}

//* Test if an account is part of an organization *
func (resource_server *server) IsOrgnanizationMember(ctx context.Context, rqst *resourcepb.IsOrgnanizationMemberRqst) (*resourcepb.IsOrgnanizationMemberRsp, error) {
	result := resource_server.isOrganizationMemeber(rqst.AccountId, rqst.OrganizationId)

	return &resourcepb.IsOrgnanizationMemberRsp{
		Result: result,
	}, nil
}

//* Delete an account *
func (resource_server *server) DeleteAccount(ctx context.Context, rqst *resourcepb.DeleteAccountRqst) (*resourcepb.DeleteAccountRsp, error) {
	// That service made user of persistence service.
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return nil, err
	}
	accountId := rqst.Id
	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Accounts", `{"$or":[{"_id":"`+accountId+`"},{"name":"`+accountId+`"} ]}`, ``)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	account := values.(map[string]interface{})

	// Remove references.
	if account["organizations"] != nil {
		organizations := []interface{}(account["organizations"].(primitive.A))
		for i := 0; i < len(organizations); i++ {
			organizationId := organizations[i].(map[string]interface{})["$id"].(string)
			resource_server.deleteReference(p, rqst.Id, organizationId, "accounts", "Accounts")
		}
	}

	if account["groups"] != nil {
		groups := []interface{}(account["groups"].(primitive.A))
		for i := 0; i < len(groups); i++ {
			groupId := groups[i].(map[string]interface{})["$id"].(string)
			resource_server.deleteReference(p, rqst.Id, groupId, "members", "Accounts")
		}
	}

	if account["roles"] != nil {
		roles := []interface{}(account["roles"].(primitive.A))
		for i := 0; i < len(roles); i++ {
			roleId := roles[i].(map[string]interface{})["$id"].(string)
			resource_server.deleteReference(p, rqst.Id, roleId, "members", "Accounts")
		}
	}

	// Try to delete the account...
	err = p.DeleteOne(context.Background(), "local_resource", "local_resource", "Accounts", `{"$or":[{"_id":"`+accountId+`"},{"name":"`+accountId+`"} ]}`, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Delete permissions
	// TODO delete account permissions

	// Delete the token.
	err = p.DeleteOne(context.Background(), "local_resource", "local_resource", "Tokens", `{"_id":"`+rqst.Id+`"}`, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	name := account["name"].(string)
	name = strings.ReplaceAll(strings.ReplaceAll(name, ".", "_"), "@", "_")

	// Here I will drop the db user.
	dropUserScript := fmt.Sprintf(
		`db=db.getSiblingDB('admin');db.dropUser('%s', {w: 'majority', wtimeout: 4000})`,
		name)

	// I will execute the sript with the admin function.
	err = p.RunAdminCmd(context.Background(), "local_resource", resource_server.Backend_user, resource_server.Backend_password, dropUserScript)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Remove the user database.
	err = p.DeleteDatabase(context.Background(), "local_resource", name+"_db")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	/* TODO fix it...
	p_, _ := resource_server.getPersistenceSaConnection()
	err = p_.DeleteConnection(name + "_db")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	*/

	return &resourcepb.DeleteAccountRsp{
		Result: rqst.Id,
	}, nil
}

/**
 * Crete a new role or Update existing one if it already exist.
 */

/** TODO set the Updating part..

role := roles[i]
count, err := store.Count(context.Background(), "local_resource", "local_resource", "Roles", `{"_id":"`+role.Id+`"}`, "")
if err != nil || count == 0 {
	r := make(map[string]interface{}, 0)
	r["_id"] = role.Id
	r["name"] = role.Name
	r["actions"] = role.Actions
	r["members"] = []string{}
	_, err := store.InsertOne(context.Background(), "local_resource", "local_resource", "Roles", r, "")
	if err != nil {
		return err
	}
} else {
	actions_, err := Utility.ToJson(role.Actions)
	if err != nil {
		return err
	}
	err = store.UpdateOne(context.Background(), "local_resource", "local_resource", "Roles", `{"_id":"`+role.Id+`"}`, `{ "$set":{"name":"`+role.Name+`"}}, { "$set":{"actions":`+actions_+`}}`, "")
	if err != nil {
		return err
	}
}
*/

func (resource_server *server) createRole(id string, name string, actions []string) error {
	// That service made user of persistence service.
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return err
	}

	// Here I will test if a newer token exist for that user if it's the case
	// I will not refresh that token.
	_, err = p.FindOne(context.Background(), "local_resource", "local_resource", "Roles", `{"_id":"`+id+`"}`, ``)
	if err == nil {
		return errors.New("Role named " + name + "already exist!")
	}

	// Here will create the new role.
	role := make(map[string]interface{}, 0)
	role["_id"] = id
	role["name"] = name
	role["actions"] = actions

	_, err = p.InsertOne(context.Background(), "local_resource", "local_resource", "Roles", role, "")
	if err != nil {
		return err
	}

	return nil
}

//* Create a role with given action list *
func (resource_server *server) CreateRole(ctx context.Context, rqst *resourcepb.CreateRoleRqst) (*resourcepb.CreateRoleRsp, error) {
	// That service made user of persistence service.
	err := resource_server.createRole(rqst.Role.Id, rqst.Role.Name, rqst.Role.Actions)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Now I will set the reference for

	// members...
	for i := 0; i < len(rqst.Role.Members); i++ {
		resource_server.createCrossReferences(rqst.Role.Members[i], "Accounts", "roles", rqst.Role.GetId(), "Roles", "members")
	}

	// Organizations
	for i := 0; i < len(rqst.Role.Organizations); i++ {
		resource_server.createCrossReferences(rqst.Role.Organizations[i], "Organizations", "roles", rqst.Role.GetId(), "Roles", "organizations")
	}

	return &resourcepb.CreateRoleRsp{Result: true}, nil
}

func (resource_server *server) GetRoles(rqst *resourcepb.GetRolesRqst, stream resourcepb.ResourceService_GetRolesServer) error {
	// Get the persistence connection
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return err
	}

	query := rqst.Query
	if len(query) == 0 {
		query = "{}"
	}

	roles, err := p.Find(context.Background(), "local_resource", "local_resource", "Roles", query, ``)
	if err != nil {
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// No I will stream the result over the networks.
	maxSize := 100
	values := make([]*resourcepb.Role, 0)

	for i := 0; i < len(roles); i++ {
		role := roles[i].(map[string]interface{})
		r := &resourcepb.Role{Id: role["_id"].(string), Name: role["name"].(string), Actions: make([]string, 0)}

		if role["actions"] != nil {
			actions := []interface{}(role["actions"].(primitive.A))
			if actions != nil {
				for i := 0; i < len(actions); i++ {
					r.Actions = append(r.Actions, actions[i].(string))
				}
			}
		}

		if role["organizations"] != nil {
			organizations := []interface{}(role["organizations"].(primitive.A))
			if organizations != nil {
				for i := 0; i < len(organizations); i++ {
					organizationId := organizations[i].(map[string]interface{})["$id"].(string)
					r.Organizations = append(r.Organizations, organizationId)
				}
			}
		}

		if role["members"] != nil {
			members := []interface{}(role["members"].(primitive.A))
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
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
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
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return nil
}

// That function is necessary to serialyse reference and kept field orders
func serialyseObject(obj map[string]interface{}) string {
	// Here I will save the role.
	jsonStr, _ := Utility.ToJson(obj)
	jsonStr = strings.ReplaceAll(jsonStr, `"$ref"`, `"__a__"`)
	jsonStr = strings.ReplaceAll(jsonStr, `"$id"`, `"__b__"`)
	jsonStr = strings.ReplaceAll(jsonStr, `"$db"`, `"__c__"`)

	obj_ := make(map[string]interface{}, 0)

	json.Unmarshal([]byte(jsonStr), &obj_)
	jsonStr, _ = Utility.ToJson(obj_)
	jsonStr = strings.ReplaceAll(jsonStr, `"__a__"`, `"$ref"`)
	jsonStr = strings.ReplaceAll(jsonStr, `"__b__"`, `"$id"`)
	jsonStr = strings.ReplaceAll(jsonStr, `"__c__"`, `"$db"`)

	return jsonStr
}

//* Delete a role with a given id *
func (resource_server *server) DeleteRole(ctx context.Context, rqst *resourcepb.DeleteRoleRqst) (*resourcepb.DeleteRoleRsp, error) {

	// That service made user of persistence service.
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	// Remove references
	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Roles", `{"_id":"`+rqst.RoleId+`"}`, ``)
	if err != nil {
		return nil, err
	}

	role := values.(map[string]interface{})
	roleId := role["_id"].(string)

	// Remove it from the accounts
	if role["members"] != nil {
		accounts := []interface{}(role["members"].(primitive.A))
		for i := 0; i < len(accounts); i++ {
			accountId := accounts[i].(map[string]interface{})["$id"].(string)
			resource_server.deleteReference(p, accountId, roleId, "roles", "Accounts")
		}
	}

	// I will remove it from organizations...
	if role["organizations"] != nil {
		organizations := []interface{}(role["organizations"].(primitive.A))
		for i := 0; i < len(organizations); i++ {
			organizationId := organizations[i].(map[string]interface{})["$id"].(string)
			resource_server.deleteReference(p, rqst.RoleId, organizationId, "roles", "Roles")
		}
	}

	err = p.DeleteOne(context.Background(), "local_resource", "local_resource", "Roles", `{"_id":"`+rqst.RoleId+`"}`, ``)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// TODO delete role permissions

	return &resourcepb.DeleteRoleRsp{Result: true}, nil
}

//* Append an action to existing role. *
func (resource_server *server) AddRoleActions(ctx context.Context, rqst *resourcepb.AddRoleActionsRqst) (*resourcepb.AddRoleActionsRsp, error) {
	// That service made user of persistence service.
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	// Here I will test if a newer token exist for that user if it's the case
	// I will not refresh that token.
	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Roles", `{"_id":"`+rqst.RoleId+`"}`, ``)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	role := values.(map[string]interface{})

	needSave := false
	if role["actions"] == nil {
		role["actions"] = rqst.Actions
		needSave = true
	} else {
		actions := []interface{}(role["actions"].(primitive.A))
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

		jsonStr, _ := json.Marshal(role)
		err := p.ReplaceOne(context.Background(), "local_resource", "local_resource", "Roles", `{"_id":"`+rqst.RoleId+`"}`, string(jsonStr), ``)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	return &resourcepb.AddRoleActionsRsp{Result: true}, nil
}

//* Remove an action to existing role. *
func (resource_server *server) RemoveRoleAction(ctx context.Context, rqst *resourcepb.RemoveRoleActionRqst) (*resourcepb.RemoveRoleActionRsp, error) {

	// That service made user of persistence service.
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	// Here I will test if a newer token exist for that user if it's the case
	// I will not refresh that token.
	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Roles", `{"_id":"`+rqst.RoleId+`"}`, ``)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	role := values.(map[string]interface{})

	needSave := false
	if role["actions"] == nil {
		role["actions"] = []string{rqst.Action}
		needSave = true
	} else {
		exist := false
		actions := make([]interface{}, 0)
		actions_ := []interface{}(role["actions"].(primitive.A))
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
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("Role named "+rqst.RoleId+"not contain actions named "+rqst.Action+"!")))
		}
	}

	if needSave {
		// jsonStr, _ := json.Marshal(role)
		jsonStr := serialyseObject(role)

		err := p.ReplaceOne(context.Background(), "local_resource", "local_resource", "Roles", `{"_id":"`+rqst.RoleId+`"}`, string(jsonStr), ``)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	return &resourcepb.RemoveRoleActionRsp{Result: true}, nil
}

//* Add role to a given account *
func (resource_server *server) AddAccountRole(ctx context.Context, rqst *resourcepb.AddAccountRoleRqst) (*resourcepb.AddAccountRoleRsp, error) {
	// That service made user of persistence service.
	err := resource_server.createCrossReferences(rqst.RoleId, "Roles", "members", rqst.AccountId, "Accounts", "roles")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &resourcepb.AddAccountRoleRsp{Result: true}, nil
}

//* Remove a role from a given account *
func (resource_server *server) RemoveAccountRole(ctx context.Context, rqst *resourcepb.RemoveAccountRoleRqst) (*resourcepb.RemoveAccountRoleRsp, error) {
	// That service made user of persistence service.
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	// Here I will test if a newer token exist for that user if it's the case
	// I will not refresh that token.
	accountId := rqst.AccountId
	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Accounts", `{"$or":[{"_id":"`+accountId+`"},{"name":"`+accountId+`"} ]}`, ``)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("No account named "+accountId+" exist!")))
	}

	account := values.(map[string]interface{})

	// Now I will test if the account already contain the role.
	if account["roles"] != nil {
		roles := make([]interface{}, 0)
		roles_ := []interface{}(account["roles"].(primitive.A))
		needSave := false
		for j := 0; j < len(roles_); j++ {
			if roles_[j].(map[string]interface{})["$id"] == rqst.RoleId {
				needSave = true
			} else {
				roles = append(roles, roles_[j])
			}
		}

		if !needSave {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("Account named "+rqst.AccountId+" does not contain role "+rqst.RoleId+"!")))
		}

		// append the newly created role.
		account["roles"] = roles
		jsonStr := serialyseObject(account)

		err = p.ReplaceOne(context.Background(), "local_resource", "local_resource", "Accounts", `{"$or":[{"_id":"`+rqst.AccountId+`"},{"name":"`+rqst.AccountId+`"} ]}`, jsonStr, ``)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

	}
	return &resourcepb.RemoveAccountRoleRsp{Result: true}, nil
}

func (resource_server *server) CreateApplication(ctx context.Context, rqst *resourcepb.CreateApplicationRqst) (*resourcepb.CreateApplicationRsp, error) {

	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if rqst.Application == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no application object was given in the request")))
	}

	count, err := p.Count(context.Background(), "local_resource", "local_resource", "Applications", `{"_id":"`+rqst.Application.Id+`"}`, "")

	application := make(map[string]interface{}, 0)
	application["_id"] = rqst.Application.Id
	application["password"] = Utility.GenerateUUID(rqst.Application.Id)
	application["path"] = "/" + rqst.Application.Id // The path must be the same as the application name.
	application["publisherid"] = rqst.Application.Publisherid
	application["version"] = rqst.Application.Version
	application["description"] = rqst.Application.Description
	application["actions"] = rqst.Application.Actions
	application["keywords"] = rqst.Application.Keywords
	application["icon"] = rqst.Application.Icon
	application["alias"] = rqst.Application.Alias

	// Save the actual time.
	application["last_deployed"] = time.Now().Unix() // save it as unix time.

	// Here I will set the resource to manage the applicaiton access permission.
	if err != nil || count == 0 {

		// create the application database.
		createApplicationUserDbScript := fmt.Sprintf(
			"db=db.getSiblingDB('%s_db');db.createCollection('application_data');db=db.getSiblingDB('admin');db.createUser({user: '%s', pwd: '%s',roles: [{ role: 'dbOwner', db: '%s_db' }]});",
			rqst.Application.Id, rqst.Application.Id, application["password"].(string), rqst.Application.Id)

		err = p.RunAdminCmd(context.Background(), "local_resource", resource_server.Backend_user, resource_server.Backend_password, createApplicationUserDbScript)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		application["creation_date"] = time.Now().Unix() // save it as unix time.
		_, err := p.InsertOne(context.Background(), "local_resource", "local_resource", "Applications", application, "")
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		/** TODO does I need to create connection here...
		err = p.CreateConnection(name+"_db", name+"_db", address, float64(port), 0, name, application["password"].(string), 5000, "", false)
		if err != nil {
			return err
		}
		*/

	} else {
		actions_, _ := Utility.ToJson(rqst.Application.Actions)
		keywords_, _ := Utility.ToJson(rqst.Application.Keywords)

		err := p.UpdateOne(context.Background(), "local_resource", "local_resource", "Applications", `{"_id":"`+rqst.Application.Id+`"}`, `{ "$set":{ "last_deployed":`+Utility.ToString(time.Now().Unix())+` }, "$set":{"keywords":`+keywords_+`}, "$set":{"actions":`+actions_+`},"$set":{"publisherid":"`+rqst.Application.Publisherid+`"},"$set":{"description":"`+rqst.Application.Description+`"},"$set":{"alias":"`+rqst.Application.Alias+`"},"$set":{"icon":"`+rqst.Application.Icon+`"}, "$set":{"version":"`+rqst.Application.Version+`"}}`, "")

		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}
	return &resourcepb.CreateApplicationRsp{}, nil
}

/**
 * Delete application Data from the backend.
 */
func (resource_server *server) deleteApplication(applicationId string) error {

	// That service made user of persistence service.
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return err
	}

	// Here I will test if a newer token exist for that user if it's the case
	// I will not refresh that token.
	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Applications", `{"_id":"`+applicationId+`"}`, ``)
	if err != nil {
		return err
	}

	application := values.(map[string]interface{})

	// I will remove it from organization...
	if application["organizations"] != nil {
		organizations := []interface{}(application["organizations"].(primitive.A))

		for i := 0; i < len(organizations); i++ {
			organizationId := organizations[i].(map[string]interface{})["$id"].(string)
			resource_server.deleteReference(p, applicationId, organizationId, "applications", "Applications")
		}
	}

	// I will remove the directory.
	err = os.RemoveAll(application["path"].(string))
	if err != nil {
		return err
	}

	// Now I will remove the database create for the application.
	err = p.DeleteDatabase(context.Background(), "local_resource", applicationId+"_db")
	if err != nil {
		return err
	}

	// Finaly I will remove the entry in  the table.
	err = p.DeleteOne(context.Background(), "local_resource", "local_resource", "Applications", `{"_id":"`+applicationId+`"}`, "")
	if err != nil {
		return err
	}

	// Delete permissions
	err = p.Delete(context.Background(), "local_resource", "local_resource", "Permissions", `{"owner":"`+applicationId+`"}`, "")
	if err != nil {
		return err
	}

	// Drop the application user.
	// Here I will drop the db user.
	dropUserScript := fmt.Sprintf(
		`db=db.getSiblingDB('admin');db.dropUser('%s', {w: 'majority', wtimeout: 4000})`,
		applicationId)

	// I will execute the sript with the admin function.
	err = p.RunAdminCmd(context.Background(), "local_resource", resource_server.Backend_user, resource_server.Backend_password, dropUserScript)
	if err != nil {
		return err
	}

	return nil
}

//* Delete an application from the server. *
func (resource_server *server) DeleteApplication(ctx context.Context, rqst *resourcepb.DeleteApplicationRqst) (*resourcepb.DeleteApplicationRsp, error) {

	// That service made user of persistence service.
	err := resource_server.deleteApplication(rqst.ApplicationId)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	// TODO delete dir permission associate with the application.

	return &resourcepb.DeleteApplicationRsp{
		Result: true,
	}, nil
}

func (resource_server *server) GetApplicationVersion(ctx context.Context, rqst *resourcepb.GetApplicationVersionRqst) (*resourcepb.GetApplicationVersionRsp, error) {
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	var previousVersion string
	previous, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Applications", `{"_id":"`+rqst.Id+`"}`, `[{"Projection":{"version":1}}]`)
	if err == nil {
		if previous != nil {
			if previous.(map[string]interface{})["version"] != nil {
				previousVersion = previous.(map[string]interface{})["version"].(string)
			}
		}
	} else {

		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))

	}

	return &resourcepb.GetApplicationVersionRsp{
		Version: previousVersion,
	}, nil

}

func (resource_server *server) GetApplicationAlias(ctx context.Context, rqst *resourcepb.GetApplicationAliasRqst) (*resourcepb.GetApplicationAliasRsp, error) {
	// That service made user of persistence service.
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Now I will retreive the application icon...
	data, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Applications", `{"_id":"`+rqst.Id+`"}`, `[{"Projection":{"alias":1}}]`)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &resourcepb.GetApplicationAliasRsp{
		Alias: data.(string),
	}, nil
}

func (resource_server *server) GetApplicationIcon(ctx context.Context, rqst *resourcepb.GetApplicationIconRqst) (*resourcepb.GetApplicationIconRsp, error) {
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Now I will retreive the application icon...
	data, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Applications", `{"_id":"`+rqst.Id+`"}`, `[{"Projection":{"icon":1}}]`)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &resourcepb.GetApplicationIconRsp{
		Icon: data.(string),
	}, nil
}

//* Append an action to existing application. *
func (resource_server *server) AddApplicationActions(ctx context.Context, rqst *resourcepb.AddApplicationActionsRqst) (*resourcepb.AddApplicationActionsRsp, error) {
	// That service made user of persistence service.
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	// Here I will test if a newer token exist for that user if it's the case
	// I will not refresh that token.
	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Applications", `{"_id":"`+rqst.ApplicationId+`"}`, ``)
	if err != nil {

		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	application := values.(map[string]interface{})
	needSave := false
	if application["actions"] == nil {
		application["actions"] = rqst.Actions
		needSave = true
	} else {
		application["actions"] = []interface{}(application["actions"].(primitive.A))
		for j := 0; j < len(rqst.Actions); j++ {
			exist := false
			for i := 0; i < len(application["actions"].([]interface{})); i++ {
				if application["actions"].([]interface{})[i].(string) == rqst.Actions[j] {
					exist = true
					break
				}
				if !exist {
					application["actions"] = append(application["actions"].([]interface{}), rqst.Actions[j])
					needSave = true
				}
			}
		}

	}

	if needSave {
		jsonStr := serialyseObject(application)
		err := p.ReplaceOne(context.Background(), "local_resource", "local_resource", "Applications", `{"_id":"`+rqst.ApplicationId+`"}`, string(jsonStr), ``)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	return &resourcepb.AddApplicationActionsRsp{Result: true}, nil
}

//* Remove an action to existing application. *
func (resource_server *server) RemoveApplicationAction(ctx context.Context, rqst *resourcepb.RemoveApplicationActionRqst) (*resourcepb.RemoveApplicationActionRsp, error) {

	// That service made user of persistence service.
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Applications", `{"_id":"`+rqst.ApplicationId+`"}`, ``)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	application := values.(map[string]interface{})

	needSave := false
	if application["actions"] == nil {
		application["actions"] = []string{rqst.Action}
		needSave = true
	} else {
		exist := false
		actions := make([]interface{}, 0)
		application["actions"] = []interface{}(application["actions"].(primitive.A))
		for i := 0; i < len(application["actions"].([]interface{})); i++ {
			if application["actions"].([]interface{})[i].(string) == rqst.Action {
				exist = true
			} else {
				actions = append(actions, application["actions"].([]interface{})[i])
			}
		}
		if exist {
			application["actions"] = actions
			needSave = true
		} else {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("Application named "+rqst.ApplicationId+" not contain actions named "+rqst.Action+"!")))
		}
	}

	if needSave {
		jsonStr := serialyseObject(application)
		err := p.ReplaceOne(context.Background(), "local_resource", "local_resource", "Applications", `{"_id":"`+rqst.ApplicationId+`"}`, string(jsonStr), ``)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	return &resourcepb.RemoveApplicationActionRsp{Result: true}, nil
}

//////////////////////////// Loggin info ///////////////////////////////////////
func (resource_server *server) validateActionRequest(rqst interface{}, method string, subject string, subjectType rbacpb.SubjectType) (bool, error) {

	infos, err := resource_server.getActionResourcesPermissions(method)
	if err != nil {
		infos = make([]*rbacpb.ResourceInfos, 0)
	} else {
		// Here I will get the params...
		val, _ := Utility.CallMethod(rqst, "ProtoReflect", []interface{}{})
		rqst_ := val.(protoreflect.Message)
		if rqst_.Descriptor().Fields().Len() > 0 {
			for i := 0; i < len(infos); i++ {
				// Get the path value from retreive infos.
				param := rqst_.Descriptor().Fields().Get(Utility.ToInt(infos[i].Index))
				path, _ := Utility.CallMethod(rqst, "Get"+strings.ToUpper(string(param.Name())[0:1])+string(param.Name())[1:], []interface{}{})
				infos[i].Path = path.(string)
			}
		}
	}

	return resource_server.validateAction(method, subject, rbacpb.SubjectType_ACCOUNT, infos)
}

// unaryInterceptor calls authenticateClient with current context
func (resource_server *server) unaryResourceInterceptor(ctx context.Context, rqst interface{}, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (interface{}, error) {
	method := info.FullMethod

	// The token and the application id.
	var token string
	var application string

	if md, ok := metadata.FromIncomingContext(ctx); ok {
		application = strings.Join(md["application"], "")
		token = strings.Join(md["token"], "")
	}

	hasAccess := false

	// If the address is local I will give the permission.
	/* Test only
	peer_, _ := peer.FromContext(ctx)
	address := peer_.Addr.String()
	address = address[0:strings.Index(address, ":")]
	if Utility.IsLocal(address) {
		hasAccess = true
	}*/

	var err error
	// Here some method are accessible by default.
	if method == "/admin.adminService/GetConfig" ||
		method == "/admin.adminService/DownloadGlobular" ||
		method == "/resource.ResourceService/GetAllActions" ||
		method == "/resource.ResourceService/RegisterAccount" ||
		method == "/resource.ResourceService/GetAccounts" ||
		method == "/resource.ResourceService/RegisterPeer" ||
		method == "/resource.ResourceService/GetPeers" ||
		method == "/resource.ResourceService/AccountExist" ||
		method == "/resource.ResourceService/Authenticate" ||
		method == "/resource.ResourceService/RefreshToken" ||
		method == "/resource.ResourceService/GetAllFilesInfo" ||
		method == "/resource.ResourceService/GetAllApplicationsInfo" ||
		method == "/resource.ResourceService/ValidateToken" ||
		method == "/rbac.RbacService/ValidateAction" ||
		method == "/rbac.RbacService/ValidateAccess" ||
		method == "/rbac.RbacService/GetResourcePermissions" ||
		method == "/rbac.RbacService/GetResourcePermission" ||
		method == "/log.LogService/Log" ||
		method == "/log.LogService/DeleteLog" ||
		method == "/log.LogService/GetLog" ||
		method == "/log.LogService/ClearAllLog" {
		hasAccess = true
	}

	var clientId string

	// Test if the user has access to execute the method
	if len(token) > 0 && !hasAccess {
		var expiredAt int64
		var err error

		/*clientId*/
		clientId, _, _, expiredAt, err = interceptors.ValidateToken(token)
		if err != nil {
			return nil, err
		}

		if expiredAt < time.Now().Unix() {
			return nil, errors.New("the token is expired")
		}

		hasAccess = clientId == "sa"
		if !hasAccess {
			hasAccess, _ = resource_server.validateActionRequest(rqst, method, clientId, rbacpb.SubjectType_ACCOUNT)
		}
	}

	// Test if the application has access to execute the method.
	if len(application) > 0 && !hasAccess {
		// TODO validate rpc method access
		hasAccess, _ = resource_server.validateActionRequest(rqst, method, application, rbacpb.SubjectType_APPLICATION)
	}

	if !hasAccess {
		err := errors.New("Permission denied to execute method " + method + " user:" + clientId + " application:" + application)
		return nil, err
	}

	// Execute the action.
	result, err := handler(ctx, rqst)
	return result, err
}

// Stream interceptor.
func (resource_server *server) streamResourceInterceptor(srv interface{}, stream grpc.ServerStream, info *grpc.StreamServerInfo, handler grpc.StreamHandler) error {

	err := handler(srv, stream)
	if err != nil {
		return err
	}

	return nil
}

///////////////////////  resource management. /////////////////
func (resource_server *server) GetAllApplicationsInfo(ctx context.Context, rqst *resourcepb.GetAllApplicationsInfoRqst) (*resourcepb.GetAllApplicationsInfoRsp, error) {

	// That service made user of persistence service.
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	// So here I will get the list of retreived permission.
	values, err := p.Find(context.Background(), "local_resource", "local_resource", "Applications", `{}`, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Convert to struct.
	infos := make([]*structpb.Struct, 0)
	for i := 0; i < len(values); i++ {
		values_ := values[i].(map[string]interface{})

		if values_["icon"] == nil {
			values_["icon"] = ""
		}

		if values_["alias"] == nil {
			values_["alias"] = ""
		}

		info, err := structpb.NewStruct(map[string]interface{}{"_id": values_["_id"], "name": values_["_id"], "path": values_["path"], "creation_date": values_["creation_date"], "last_deployed": values_["last_deployed"], "alias": values_["alias"], "icon": values_["icon"], "description": values_["description"]})
		if err == nil {
			infos = append(infos, info)
		} else {
			log.Println(err)
		}
	}

	return &resourcepb.GetAllApplicationsInfoRsp{
		Applications: infos,
	}, nil

}

////////////////////////////////////////////////////////////////////////////////
// Peer's Authorization and Authentication code.
////////////////////////////////////////////////////////////////////////////////

//* Register a new Peer on the network *
func (resource_server *server) RegisterPeer(ctx context.Context, rqst *resourcepb.RegisterPeerRqst) (*resourcepb.RegisterPeerRsp, error) {
	// A peer want to be part of the network.

	// Get the persistence connection
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	// Here I will first look if a peer with a same name already exist on the
	// resources...
	_id := Utility.GenerateUUID(rqst.Peer.Domain)

	count, _ := p.Count(context.Background(), "local_resource", "local_resource", "Peers", `{"_id":"`+_id+`"}`, "")
	if count > 0 {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("Peer with name '"+rqst.Peer.Domain+"' already exist!")))
	}

	// No authorization exist for that peer I will insert it.
	// Here will create the new peer.
	peer := make(map[string]interface{}, 0)
	peer["_id"] = _id
	peer["domain"] = rqst.Peer.Domain
	peer["actions"] = make([]interface{}, 0)

	_, err = p.InsertOne(context.Background(), "local_resource", "local_resource", "Peers", peer, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &resourcepb.RegisterPeerRsp{
		Result: true,
	}, nil
}

//* Return the list of authorized peers *
func (resource_server *server) GetPeers(rqst *resourcepb.GetPeersRqst, stream resourcepb.ResourceService_GetPeersServer) error {
	// Get the persistence connection
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return err
	}

	query := rqst.Query
	if len(query) == 0 {
		query = "{}"
	}

	peers, err := p.Find(context.Background(), "local_resource", "local_resource", "Peers", query, ``)
	if err != nil {
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// No I will stream the result over the networks.
	maxSize := 100
	values := make([]*resourcepb.Peer, 0)

	for i := 0; i < len(peers); i++ {
		p := &resourcepb.Peer{Domain: peers[i].(map[string]interface{})["domain"].(string), Actions: make([]string, 0)}
		peers[i].(map[string]interface{})["actions"] = []interface{}(peers[i].(map[string]interface{})["actions"].(primitive.A))
		for j := 0; j < len(peers[i].(map[string]interface{})["actions"].([]interface{})); j++ {
			p.Actions = append(p.Actions, peers[i].(map[string]interface{})["actions"].([]interface{})[j].(string))
		}

		values = append(values, p)

		if len(values) >= maxSize {
			err := stream.Send(
				&resourcepb.GetPeersRsp{
					Peers: values,
				},
			)
			if err != nil {
				return status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}
			values = make([]*resourcepb.Peer, 0)
		}
	}

	// Send reminding values.
	err = stream.Send(
		&resourcepb.GetPeersRsp{
			Peers: values,
		},
	)

	if err != nil {
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return nil
}

//* Remove a peer from the network *
func (resource_server *server) DeletePeer(ctx context.Context, rqst *resourcepb.DeletePeerRqst) (*resourcepb.DeletePeerRsp, error) {
	// Get the persistence connection
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	// No authorization exist for that peer I will insert it.
	// Here will create the new peer.
	_id := Utility.GenerateUUID(rqst.Peer.Domain)

	err = p.DeleteOne(context.Background(), "local_resource", "local_resource", "Peers", `{"_id":"`+_id+`"}`, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Delete permissions
	err = p.Delete(context.Background(), "local_resource", "local_resource", "Permissions", `{"owner":"`+_id+`"}`, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &resourcepb.DeletePeerRsp{
		Result: true,
	}, nil
}

//* Add peer action permission *
func (resource_server *server) AddPeerActions(ctx context.Context, rqst *resourcepb.AddPeerActionsRqst) (*resourcepb.AddPeerActionsRsp, error) {
	// That service made user of persistence service.
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return nil, err
	}
	_id := Utility.GenerateUUID(rqst.Domain)

	// Here I will test if a newer token exist for that user if it's the case
	// I will not refresh that token.
	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Peers", `{"_id":"`+_id+`"}`, ``)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	peer := values.(map[string]interface{})

	needSave := false
	if peer["actions"] == nil {
		peer["actions"] = rqst.Actions
		needSave = true
	} else {
		actions := []interface{}(peer["actions"].(primitive.A))
		for j := 0; j < len(rqst.Actions); j++ {
			exist := false
			for i := 0; i < len(peer["actions"].(primitive.A)); i++ {
				if peer["actions"].(primitive.A)[i].(string) == rqst.Actions[j] {
					exist = true
					break
				}
			}
			if !exist {
				actions = append(actions, rqst.Actions[j])
				needSave = true
			}
		}
		peer["actions"] = actions
	}

	if needSave {
		jsonStr := serialyseObject(peer)
		err := p.ReplaceOne(context.Background(), "local_resource", "local_resource", "Peers", `{"_id":"`+_id+`"}`, string(jsonStr), ``)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	return &resourcepb.AddPeerActionsRsp{Result: true}, nil

}

//* Remove peer action permission *
func (resource_server *server) RemovePeerAction(ctx context.Context, rqst *resourcepb.RemovePeerActionRqst) (*resourcepb.RemovePeerActionRsp, error) {
	// That service made user of persistence service.
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return nil, err
	}
	_id := Utility.GenerateUUID(rqst.Domain)
	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Peers", `{"_id":"`+_id+`"}`, ``)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	peer := values.(map[string]interface{})

	needSave := false
	if peer["actions"] == nil {
		peer["actions"] = []string{rqst.Action}
		needSave = true
	} else {
		exist := false
		actions := make([]interface{}, 0)
		for i := 0; i < len(peer["actions"].(primitive.A)); i++ {
			if peer["actions"].(primitive.A)[i].(string) == rqst.Action {
				exist = true
			} else {
				actions = append(actions, peer["actions"].(primitive.A)[i])
			}
		}
		if exist {
			peer["actions"] = actions
			needSave = true
		} else {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("Peer named "+rqst.Domain+" not contain actions named "+rqst.Action+"!")))
		}
	}

	if needSave {
		jsonStr := serialyseObject(peer)
		err := p.ReplaceOne(context.Background(), "local_resource", "local_resource", "Peers", `{"_id":"`+_id+`"}`, string(jsonStr), ``)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	return &resourcepb.RemovePeerActionRsp{Result: true}, nil
}

//* Register a new organization
func (resource_server *server) CreateOrganization(ctx context.Context, rqst *resourcepb.CreateOrganizationRqst) (*resourcepb.CreateOrganizationRsp, error) {

	// Get the persistence connection
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	// Here I will first look if a peer with a same name already exist on the
	// resources...
	count, _ := p.Count(context.Background(), "local_resource", "local_resource", "Organizations", `{"_id":"`+rqst.Organization.Id+`"}`, "")
	if count > 0 {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("Organization with name '"+rqst.Organization.Id+"' already exist!")))
	}

	// No authorization exist for that peer I will insert it.
	// Here will create the new peer.
	g := make(map[string]interface{}, 0)
	g["_id"] = rqst.Organization.Id
	g["name"] = rqst.Organization.Name

	// Those are the list of entity linked to the organisation
	g["accounts"] = make([]interface{}, 0)
	g["groups"] = make([]interface{}, 0)
	g["roles"] = make([]interface{}, 0)
	g["applications"] = make([]interface{}, 0)

	_, err = p.InsertOne(context.Background(), "local_resource", "local_resource", "Organizations", g, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// accounts...
	for i := 0; i < len(rqst.Organization.Accounts); i++ {
		resource_server.createCrossReferences(rqst.Organization.Accounts[i], "Accounts", "organizations", rqst.Organization.GetId(), "Organizations", "accounts")
	}

	// groups...
	for i := 0; i < len(rqst.Organization.Groups); i++ {
		resource_server.createCrossReferences(rqst.Organization.Groups[i], "Groups", "organizations", rqst.Organization.GetId(), "Organizations", "groups")
	}

	// roles...
	for i := 0; i < len(rqst.Organization.Roles); i++ {
		resource_server.createCrossReferences(rqst.Organization.Roles[i], "Roles", "organizations", rqst.Organization.GetId(), "Organizations", "roles")
	}

	// applications...
	for i := 0; i < len(rqst.Organization.Applications); i++ {
		resource_server.createCrossReferences(rqst.Organization.Roles[i], "Applications", "organizations", rqst.Organization.GetId(), "Organizations", "applications")
	}

	return &resourcepb.CreateOrganizationRsp{
		Result: true,
	}, nil
}

//* Return the list of organizations
func (resource_server *server) GetOrganizations(rqst *resourcepb.GetOrganizationsRqst, stream resourcepb.ResourceService_GetOrganizationsServer) error {
	// Get the persistence connection
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return err
	}

	query := rqst.Query
	if len(query) == 0 {
		query = "{}"
	}

	organizations, err := p.Find(context.Background(), "local_resource", "local_resource", "Organizations", query, ``)
	if err != nil {
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// No I will stream the result over the networks.
	maxSize := 50
	values := make([]*resourcepb.Organization, 0)
	for i := 0; i < len(organizations); i++ {
		o := organizations[i].(map[string]interface{})

		organization := new(resourcepb.Organization)
		organization.Id = o["_id"].(string)
		organization.Name = o["name"].(string)

		// Here I will set the aggregation.

		// Groups
		if o["groups"] != nil {
			groups := []interface{}(o["groups"].(primitive.A))
			if groups != nil {
				for i := 0; i < len(groups); i++ {
					groupId := groups[i].(map[string]interface{})["$id"].(string)
					organization.Groups = append(organization.Groups, groupId)
				}
			}
		}

		// Roles
		if o["roles"] != nil {
			roles := []interface{}(o["roles"].(primitive.A))
			if roles != nil {
				for i := 0; i < len(roles); i++ {
					roleId := roles[i].(map[string]interface{})["$id"].(string)
					organization.Roles = append(organization.Roles, roleId)
				}
			}
		}

		// Accounts
		if o["accounts"] != nil {
			accounts := []interface{}(o["accounts"].(primitive.A))
			if accounts != nil {
				for i := 0; i < len(accounts); i++ {
					accountId := accounts[i].(map[string]interface{})["$id"].(string)
					organization.Accounts = append(organization.Accounts, accountId)
				}
			}
		}

		// Applications
		if o["applications"] != nil {
			applications := []interface{}(o["applications"].(primitive.A))
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
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
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
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return nil
}

//* Add Account *
func (resource_server *server) AddOrganizationAccount(ctx context.Context, rqst *resourcepb.AddOrganizationAccountRqst) (*resourcepb.AddOrganizationAccountRsp, error) {
	err := resource_server.createCrossReferences(rqst.OrganizationId, "Organizations", "accounts", rqst.AccountId, "Accounts", "organizations")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &resourcepb.AddOrganizationAccountRsp{Result: true}, nil
}

//* Add Group *
func (resource_server *server) AddOrganizationGroup(ctx context.Context, rqst *resourcepb.AddOrganizationGroupRqst) (*resourcepb.AddOrganizationGroupRsp, error) {
	err := resource_server.createCrossReferences(rqst.OrganizationId, "Organizations", "groups", rqst.GroupId, "Groups", "organizations")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &resourcepb.AddOrganizationGroupRsp{Result: true}, nil
}

//* Add Role *
func (resource_server *server) AddOrganizationRole(ctx context.Context, rqst *resourcepb.AddOrganizationRoleRqst) (*resourcepb.AddOrganizationRoleRsp, error) {
	err := resource_server.createCrossReferences(rqst.OrganizationId, "Organizations", "roles", rqst.RoleId, "Roles", "organizations")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &resourcepb.AddOrganizationRoleRsp{Result: true}, nil
}

//* Add Application *
func (resource_server *server) AddOrganizationApplication(ctx context.Context, rqst *resourcepb.AddOrganizationApplicationRqst) (*resourcepb.AddOrganizationApplicationRsp, error) {
	err := resource_server.createCrossReferences(rqst.OrganizationId, "Organizations", "applications", rqst.ApplicationId, "Applications", "organizations")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &resourcepb.AddOrganizationApplicationRsp{Result: true}, nil
}

//* Remove Account *
func (resource_server *server) RemoveOrganizationAccount(ctx context.Context, rqst *resourcepb.RemoveOrganizationAccountRqst) (*resourcepb.RemoveOrganizationAccountRsp, error) {
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	err = resource_server.deleteReference(p, rqst.AccountId, rqst.OrganizationId, "accounts", "Organizations")
	if err != nil {
		return nil, err
	}

	err = resource_server.deleteReference(p, rqst.OrganizationId, rqst.AccountId, "organizations", "Accounts")
	if err != nil {
		return nil, err
	}

	return &resourcepb.RemoveOrganizationAccountRsp{Result: true}, nil
}

//* Remove Group *
func (resource_server *server) RemoveOrganizationGroup(ctx context.Context, rqst *resourcepb.RemoveOrganizationGroupRqst) (*resourcepb.RemoveOrganizationGroupRsp, error) {
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	err = resource_server.deleteReference(p, rqst.GroupId, rqst.OrganizationId, "groups", "Organizations")
	if err != nil {
		return nil, err
	}

	err = resource_server.deleteReference(p, rqst.OrganizationId, rqst.GroupId, "organizations", "Groups")
	if err != nil {
		return nil, err
	}

	return &resourcepb.RemoveOrganizationGroupRsp{Result: true}, nil
}

//* Remove Role *
func (resource_server *server) RemoveOrganizationRole(ctx context.Context, rqst *resourcepb.RemoveOrganizationRoleRqst) (*resourcepb.RemoveOrganizationRoleRsp, error) {
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	err = resource_server.deleteReference(p, rqst.RoleId, rqst.OrganizationId, "roles", "Organizations")
	if err != nil {
		return nil, err
	}

	err = resource_server.deleteReference(p, rqst.OrganizationId, rqst.RoleId, "organizations", "Roles")
	if err != nil {
		return nil, err
	}

	return &resourcepb.RemoveOrganizationRoleRsp{Result: true}, nil
}

//* Remove Application *
func (resource_server *server) RemoveOrganizationApplication(ctx context.Context, rqst *resourcepb.RemoveOrganizationApplicationRqst) (*resourcepb.RemoveOrganizationApplicationRsp, error) {
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	err = resource_server.deleteReference(p, rqst.ApplicationId, rqst.OrganizationId, "applications", "Organizations")
	if err != nil {
		return nil, err
	}

	err = resource_server.deleteReference(p, rqst.OrganizationId, rqst.ApplicationId, "organizations", "Applications")
	if err != nil {
		return nil, err
	}

	return &resourcepb.RemoveOrganizationApplicationRsp{Result: true}, nil
}

//* Delete organization
func (resource_server *server) DeleteOrganization(ctx context.Context, rqst *resourcepb.DeleteOrganizationRqst) (*resourcepb.DeleteOrganizationRsp, error) {

	// That service made user of persistence service.
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Organizations", `{"_id":"`+rqst.Organization+`"}`, ``)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	organization := values.(map[string]interface{})
	if organization["groups"] != nil {
		groups := []interface{}(organization["groups"].(primitive.A))
		if groups != nil {
			for i := 0; i < len(groups); i++ {
				groupId := groups[i].(map[string]interface{})["$id"].(string)
				resource_server.deleteReference(p, rqst.Organization, groupId, "organizations", "Organizations")
			}
		}
	}

	if organization["roles"].(primitive.A) != nil {
		roles := []interface{}(organization["roles"].(primitive.A))
		if roles != nil {
			for i := 0; i < len(roles); i++ {
				roleId := roles[i].(map[string]interface{})["$id"].(string)
				resource_server.deleteReference(p, rqst.Organization, roleId, "organizations", "Organizations")
			}
		}
	}

	if organization["applications"].(primitive.A) != nil {
		applications := []interface{}(organization["applications"].(primitive.A))
		if applications != nil {
			for i := 0; i < len(applications); i++ {
				applicationId := applications[i].(map[string]interface{})["$id"].(string)
				resource_server.deleteReference(p, rqst.Organization, applicationId, "organizations", "Organizations")
			}
		}
	}

	if organization["accounts"].(primitive.A) != nil {
		accounts := []interface{}(organization["accounts"].(primitive.A))
		if accounts != nil {
			for i := 0; i < len(accounts); i++ {
				accountsId := accounts[i].(map[string]interface{})["$id"].(string)
				resource_server.deleteReference(p, rqst.Organization, accountsId, "organizations", "Organizations")
			}
		}
	}

	// Try to delete the account...
	err = p.DeleteOne(context.Background(), "local_resource", "local_resource", "Organizations", `{"_id":"`+rqst.Organization+`"}`, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &resourcepb.DeleteOrganizationRsp{Result: true}, nil
}

/**
 * Create a group with a given name of update existing one.
 */
/* TODO set the update part of the function.
 		count, err := store.Count(context.Background(), "local_resource", "local_resource", "Groups", `{"_id":"`+group.Id+`"}`, "")
		if err != nil || count == 0 {
			g := make(map[string]interface{}, 0)
			g["_id"] = group.Id
			g["name"] = group.Name
			g["members"] = []string{}
			_, err := store.InsertOne(context.Background(), "local_resource", "local_resource", "Groups", g, "")
			if err != nil {
				return err
			}
		} else {

			err = store.UpdateOne(context.Background(), "local_resource", "local_resource", "Groups", `{"_id":"`+group.Id+`"}`, `{ "$set":{"name":"`+group.Name+`"}}`, "")
			if err != nil {
				return err
			}
		}
*/
func (resource_server *server) createGroup(id, name string, members []string) error {
	// Get the persistence connection
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return err
	}

	// Here I will first look if a peer with a same name already exist on the
	// resources...
	count, _ := p.Count(context.Background(), "local_resource", "local_resource", "Groups", `{"_id":"`+id+`"}`, "")
	if count > 0 {
		return errors.New("Group with name '" + id + "' already exist!")
	}

	// No authorization exist for that peer I will insert it.
	// Here will create the new peer.
	g := make(map[string]interface{}, 0)
	g["_id"] = id
	g["name"] = name

	_, err = p.InsertOne(context.Background(), "local_resource", "local_resource", "Groups", g, "")
	if err != nil {
		return err
	}

	// Create references.
	for i := 0; i < len(members); i++ {
		err := resource_server.createCrossReferences(id, "Groups", "members", members[i], "Accounts", "groups")
		if err != nil {
			log.Println(err)
		}
	}

	return nil
}

//* Register a new group
func (resource_server *server) CreateGroup(ctx context.Context, rqst *resourcepb.CreateGroupRqst) (*resourcepb.CreateGroupRsp, error) {
	// Get the persistence connection
	err := resource_server.createGroup(rqst.Group.Id, rqst.Group.Name, rqst.Group.Members)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &resourcepb.CreateGroupRsp{
		Result: true,
	}, nil
}

//* Return the list of organizations
func (resource_server *server) GetGroups(rqst *resourcepb.GetGroupsRqst, stream resourcepb.ResourceService_GetGroupsServer) error {
	// Get the persistence connection
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return err
	}

	query := rqst.Query
	if len(query) == 0 {
		query = "{}"
	}

	groups, err := p.Find(context.Background(), "local_resource", "local_resource", "Groups", query, ``)
	if err != nil {
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// No I will stream the result over the networks.
	maxSize := 50
	values := make([]*resourcepb.Group, 0)
	for i := 0; i < len(groups); i++ {

		g := &resourcepb.Group{Name: groups[i].(map[string]interface{})["name"].(string), Id: groups[i].(map[string]interface{})["_id"].(string), Members: make([]string, 0)}

		if groups[i].(map[string]interface{})["members"] != nil {
			members := []interface{}(groups[i].(map[string]interface{})["members"].(primitive.A))
			g.Members = make([]string, 0)
			for j := 0; j < len(members); j++ {
				g.Members = append(g.Members, members[j].(map[string]interface{})["$id"].(string))
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
						Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
				}
				values = make([]*resourcepb.Group, 0)
			}
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
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return nil
}

//* Delete organization
func (resource_server *server) DeleteGroup(ctx context.Context, rqst *resourcepb.DeleteGroupRqst) (*resourcepb.DeleteGroupRsp, error) {
	// Get the persistence connection
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Groups", `{"_id":"`+rqst.Group+`"}`, ``)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	group := values.(map[string]interface{})

	// I will remove it from accounts...

	if group["members"] != nil {
		members := []interface{}(group["members"].(primitive.A))
		for j := 0; j < len(members); j++ {
			resource_server.deleteReference(p, rqst.Group, members[j].(map[string]interface{})["$id"].(string), "groups", members[j].(map[string]interface{})["$ref"].(string))
		}
	}

	// I will remove it from organizations...
	if group["organizations"] != nil {
		organizations := []interface{}(group["organizations"].(primitive.A))
		if organizations != nil {
			for i := 0; i < len(organizations); i++ {
				organizationId := organizations[i].(map[string]interface{})["$id"].(string)
				resource_server.deleteReference(p, rqst.Group, organizationId, "groups", "Groups")
			}
		}
	}

	err = p.DeleteOne(context.Background(), "local_resource", "local_resource", "Groups", `{"_id":"`+rqst.Group+`"}`, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &resourcepb.DeleteGroupRsp{
		Result: true,
	}, nil

}

//* Add a member account to the group *
func (resource_server *server) AddGroupMemberAccount(ctx context.Context, rqst *resourcepb.AddGroupMemberAccountRqst) (*resourcepb.AddGroupMemberAccountRsp, error) {

	err := resource_server.createCrossReferences(rqst.GroupId, "Groups", "members", rqst.AccountId, "Accounts", "groups")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &resourcepb.AddGroupMemberAccountRsp{Result: true}, nil
}

//* Remove member account from the group *
func (resource_server *server) RemoveGroupMemberAccount(ctx context.Context, rqst *resourcepb.RemoveGroupMemberAccountRqst) (*resourcepb.RemoveGroupMemberAccountRsp, error) {
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	// That service made user of persistence service.
	err = resource_server.deleteReference(p, rqst.AccountId, rqst.GroupId, "members", "Groups")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = resource_server.deleteReference(p, rqst.GroupId, rqst.AccountId, "groups", "Accounts")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &resourcepb.RemoveGroupMemberAccountRsp{Result: true}, nil
}

////////////////////////////////////////////////////////////////////////////////////
// Notification implementation
////////////////////////////////////////////////////////////////////////////////////
//* Create a notification
func (resource_server *server) CreateNotification(ctx context.Context, rqst *resourcepb.CreateNotificationRqst) (*resourcepb.CreateNotificationRsp, error) {
	return nil, errors.New("not implemented")
}

//* Retreive notifications
func (resource_server *server) GetNotifications(rqst *resourcepb.GetNotificationsRqst, stream resourcepb.ResourceService_GetNotificationsServer) error {
	return errors.New("not implemented")
}

//* Remove a notification
func (resource_server *server) DeleteNotification(ctx context.Context, rqst *resourcepb.DeleteNotificationRqst) (*resourcepb.DeleteNotificationRsp, error) {
	return nil, errors.New("not implemented")
}

//* Remove all Notification
func (resource_server *server) ClearAllNotifications(ctx context.Context, rqst *resourcepb.ClearAllNotificationsRqst) (*resourcepb.ClearAllNotificationsRsp, error) {
	return nil, errors.New("not implemented")
}

//* Remove all notification of a given type
func (resource_server *server) ClearNotificationsByType(ctx context.Context, rqst *resourcepb.ClearNotificationsByTypeRqst) (*resourcepb.ClearNotificationsByTypeRsp, error) {
	return nil, errors.New("not implemented")
}


/////////////////////////////////////////////////////////////////////////////////////////
// Pakage informations...
/////////////////////////////////////////////////////////////////////////////////////////

// Find packages by keywords...
func (server *server) FindPackages(ctx context.Context, rqst *resourcepb.FindPackagesDescriptorRequest) (*resourcepb.FindPackagesDescriptorResponse, error) {
	// That service made user of persistence service.
	p, err := server.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	kewordsStr, err := Utility.ToJson(rqst.Keywords)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Test...
	query := `{"keywords": { "$all" : ` + kewordsStr + `}}`

	data, err := p.Find(context.Background(), "local_resource", "local_resource", "Packages", query, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	descriptors := make([]*resourcepb.PackageDescriptor, len(data))
	for i := 0; i < len(data); i++ {
		descriptor := data[i].(map[string]interface{})
		descriptors[i] = new(resourcepb.PackageDescriptor)
		descriptors[i].Id = descriptor["id"].(string)
		descriptors[i].Name = descriptor["name"].(string)
		descriptors[i].Description = descriptor["description"].(string)
		descriptors[i].PublisherId = descriptor["publisherid"].(string)
		descriptors[i].Version = descriptor["version"].(string)
		descriptors[i].Icon = descriptor["icon"].(string)
		descriptors[i].Alias = descriptor["alias"].(string)
		if descriptor["keywords"] != nil {
			descriptor["keywords"] = []interface{}(descriptor["keywords"].(primitive.A))
			descriptors[i].Keywords = make([]string, len(descriptor["keywords"].([]interface{})))
			for j := 0; j < len(descriptor["keywords"].([]interface{})); j++ {
				descriptors[i].Keywords[j] = descriptor["keywords"].([]interface{})[j].(string)
			}
		}
		if descriptor["actions"] != nil {
			descriptor["actions"] = []interface{}(descriptor["actions"].(primitive.A))
			descriptors[i].Actions = make([]string, len(descriptor["actions"].([]interface{})))
			for j := 0; j < len(descriptor["actions"].([]interface{})); j++ {
				descriptors[i].Actions[j] = descriptor["actions"].([]interface{})[j].(string)
			}
		}
		if descriptor["discoveries"] != nil {
			descriptor["discoveries"] = []interface{}(descriptor["discoveries"].(primitive.A))
			descriptors[i].Discoveries = make([]string, len(descriptor["discoveries"].([]interface{})))
			for j := 0; j < len(descriptor["discoveries"].([]interface{})); j++ {
				descriptors[i].Discoveries[j] = descriptor["discoveries"].([]interface{})[j].(string)
			}
		}

		if descriptor["repositories"] != nil {
			descriptor["repositories"] = []interface{}(descriptor["repositories"].(primitive.A))
			descriptors[i].Repositories = make([]string, len(descriptor["repositories"].([]interface{})))
			for j := 0; j < len(descriptor["repositories"].([]interface{})); j++ {
				descriptors[i].Repositories[j] = descriptor["repositories"].([]interface{})[j].(string)
			}
		}
	}

	// Return the list of Service Descriptor.
	return &resourcepb.FindPackagesDescriptorResponse{
		Results: descriptors,
	}, nil
}

//* Retrun all version of a given packages. *
func (server *server) GetPackageDescriptor(ctx context.Context, rqst *resourcepb.GetPackageDescriptorRequest) (*resourcepb.GetPackageDescriptorResponse, error) {
	p, err := server.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	query := `{"id":"` + rqst.ServiceId + `", "publisherid":"` + rqst.PublisherId + `"}`

	values, err := p.Find(context.Background(), "local_resource", "local_resource", "Packages", query, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	if len(values) == 0 {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("No service descriptor with id "+rqst.ServiceId+" was found for publisher id "+rqst.PublisherId)))
	}

	descriptors := make([]*resourcepb.PackageDescriptor, len(values))
	for i := 0; i < len(values); i++ {

		descriptor := values[i].(map[string]interface{})
		descriptors[i] = new(resourcepb.PackageDescriptor)
		descriptors[i].Id = descriptor["id"].(string)
		descriptors[i].Name = descriptor["name"].(string)
		if descriptor["alias"] != nil {
			descriptors[i].Alias = descriptor["alias"].(string)
		} else {
			descriptors[i].Alias = descriptors[i].Name
		}
		if descriptor["icon"] != nil {
			descriptors[i].Icon = descriptor["icon"].(string)
		}
		if descriptor["description"] != nil {
			descriptors[i].Description = descriptor["description"].(string)
		}
		if descriptor["publisherid"] != nil {
			descriptors[i].PublisherId = descriptor["publisherid"].(string)
		}
		if descriptor["version"] != nil {
			descriptors[i].Version = descriptor["version"].(string)
		}
		descriptors[i].Type = resourcepb.PackageType(Utility.ToInt(descriptor["type"]))

		if descriptor["keywords"] != nil {
			descriptor["keywords"] = []interface{}(descriptor["keywords"].(primitive.A))
			descriptors[i].Keywords = make([]string, len(descriptor["keywords"].([]interface{})))
			for j := 0; j < len(descriptor["keywords"].([]interface{})); j++ {
				descriptors[i].Keywords[j] = descriptor["keywords"].([]interface{})[j].(string)
			}
		}

		if descriptor["actions"] != nil {
			descriptor["actions"] = []interface{}(descriptor["actions"].(primitive.A))
			descriptors[i].Actions = make([]string, len(descriptor["actions"].([]interface{})))
			for j := 0; j < len(descriptor["actions"].([]interface{})); j++ {
				descriptors[i].Actions[j] = descriptor["actions"].([]interface{})[j].(string)
			}
		}

		if descriptor["discoveries"] != nil {
			descriptor["discoveries"] = []interface{}(descriptor["discoveries"].(primitive.A))
			descriptors[i].Discoveries = make([]string, len(descriptor["discoveries"].([]interface{})))
			for j := 0; j < len(descriptor["discoveries"].([]interface{})); j++ {
				descriptors[i].Discoveries[j] = descriptor["discoveries"].([]interface{})[j].(string)
			}
		}

		if descriptor["repositories"] != nil {
			descriptor["repositories"] = []interface{}(descriptor["repositories"].(primitive.A))
			descriptors[i].Repositories = make([]string, len(descriptor["repositories"].([]interface{})))
			for j := 0; j < len(descriptor["repositories"].([]interface{})); j++ {
				descriptors[i].Repositories[j] = descriptor["repositories"].([]interface{})[j].(string)
			}
		}
	}

	sort.Slice(descriptors[:], func(i, j int) bool {
		return descriptors[i].Version > descriptors[j].Version
	})

	// Return the list of Service Descriptor.
	return &resourcepb.GetPackageDescriptorResponse{
		Results: descriptors,
	}, nil
}

//* Return the list of all services *
func (server *server) GetPackagesDescriptor(rqst *resourcepb.GetPackagesDescriptorRequest, stream resourcepb.ResourceService_GetPackagesDescriptorServer) error {
	p, err := server.getPersistenceStore()
	if err != nil {
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	data, err := p.Find(context.Background(), "local_resource", "local_resource", "Services", `{}`, "")
	if err != nil {
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	descriptors := make([]*resourcepb.PackageDescriptor, 0)
	for i := 0; i < len(data); i++ {
		descriptor := new(resourcepb.PackageDescriptor)

		descriptor.Id = data[i].(map[string]interface{})["id"].(string)
		descriptor.Name = data[i].(map[string]interface{})["name"].(string)
		descriptor.Description = data[i].(map[string]interface{})["description"].(string)
		descriptor.PublisherId = data[i].(map[string]interface{})["publisherid"].(string)
		descriptor.Version = data[i].(map[string]interface{})["version"].(string)
		descriptor.Icon = data[i].(map[string]interface{})["icon"].(string)
		descriptor.Alias = data[i].(map[string]interface{})["alias"].(string)
		descriptor.Type = resourcepb.PackageType(Utility.ToInt(data[i].(map[string]interface{})["type"]))

		if data[i].(map[string]interface{})["keywords"] != nil {
			data[i].(map[string]interface{})["keywords"] = []interface{}(data[i].(map[string]interface{})["keywords"].(primitive.A))
			descriptor.Keywords = make([]string, len(data[i].(map[string]interface{})["keywords"].([]interface{})))
			for j := 0; j < len(data[i].(map[string]interface{})["keywords"].([]interface{})); j++ {
				descriptor.Keywords[j] = data[i].(map[string]interface{})["keywords"].([]interface{})[j].(string)
			}
		}

		if data[i].(map[string]interface{})["actions"] != nil {
			data[i].(map[string]interface{})["actions"] = []interface{}(data[i].(map[string]interface{})["actions"].(primitive.A))
			descriptor.Actions = make([]string, len(data[i].(map[string]interface{})["actions"].([]interface{})))
			for j := 0; j < len(data[i].(map[string]interface{})["actions"].([]interface{})); j++ {
				descriptor.Actions[j] = data[i].(map[string]interface{})["actions"].([]interface{})[j].(string)
			}
		}

		if data[i].(map[string]interface{})["discoveries"] != nil {
			data[i].(map[string]interface{})["discoveries"] = []interface{}(data[i].(map[string]interface{})["discoveries"].(primitive.A))
			descriptor.Discoveries = make([]string, len(data[i].(map[string]interface{})["discoveries"].([]interface{})))
			for j := 0; j < len(data[i].(map[string]interface{})["discoveries"].([]interface{})); j++ {
				descriptor.Discoveries[j] = data[i].(map[string]interface{})["discoveries"].([]interface{})[j].(string)
			}
		}

		if data[i].(map[string]interface{})["repositories"] != nil {
			data[i].(map[string]interface{})["repositories"] = []interface{}(data[i].(map[string]interface{})["repositories"].(primitive.A))
			descriptor.Repositories = make([]string, len(data[i].(map[string]interface{})["repositories"].([]interface{})))
			for j := 0; j < len(data[i].(map[string]interface{})["repositories"].([]interface{})); j++ {
				descriptor.Repositories[j] = data[i].(map[string]interface{})["repositories"].([]interface{})[j].(string)
			}
		}

		descriptors = append(descriptors, descriptor)
		// send at each 20
		if i%20 == 0 {
			stream.Send(&resourcepb.GetPackagesDescriptorResponse{
				Results: descriptors,
			})
			descriptors = make([]*resourcepb.PackageDescriptor, 0)
		}
	}

	if len(descriptors) > 0 {
		stream.Send(&resourcepb.GetPackagesDescriptorResponse{
			Results: descriptors,
		})
	}

	// Return the list of Service Descriptor.
	return nil
}

/**
 * Create / Update a pacakge descriptor
 */
func (server *server) SetPackageDescriptor(ctx context.Context, rqst *resourcepb.SetPackageDescriptorRequest) (*resourcepb.SetPackageDescriptorResponse, error) {
	p, err := server.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	var marshaler jsonpb.Marshaler

	jsonStr, err := marshaler.MarshalToString(rqst.Descriptor_)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// little fix...
	jsonStr = strings.ReplaceAll(jsonStr, "publisherId", "publisherid")

	// Always create a new if not already exist.
	err = p.ReplaceOne(context.Background(), "local_resource", "local_resource", "Services", `{"id":"`+rqst.Descriptor_.Id+`", "publisherid":"`+rqst.Descriptor_.PublisherId+`", "version":"`+rqst.Descriptor_.Version+`"}`, jsonStr, `[{"upsert": true}]`)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &resourcepb.SetPackageDescriptorResponse{
		Result: true,
	}, nil
}

//* Get the package bundle checksum use for validation *
func (server *server) GetPackageBundleChecksum(ctx context.Context, rqst *resourcepb.GetPackageBundleChecksumRequest) (*resourcepb.GetPackageBundleChecksumResponse, error) {
	p, err := server.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", "PackageBundle", `{"_id":"`+rqst.Id+`"}`, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Here I will retreive the values from the db and
	return &resourcepb.GetPackageBundleChecksumResponse{
		Checksum: values.(map[string]interface{})["checksum"].(string),
	}, nil

}

//* Set the package bundle (without data)
func (server *server) SetPackageBundle(ctx context.Context, rqst *resourcepb.SetPackageBundleRequest) (*resourcepb.SetPackageBundleResponse, error) {
	bundle := rqst.Bundle

	p, err := server.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Generate the bundle id....
	id := Utility.GenerateUUID(bundle.Descriptor_.PublisherId + "%" + bundle.Descriptor_.Name + "%" + bundle.Descriptor_.Version + "%" + bundle.Descriptor_.Id + "%" + bundle.Plaform)

	log.Println(id)
	jsonStr, err := Utility.ToJson(map[string]interface{}{"_id": id, "checksum": bundle.Checksum, "platform": bundle.Plaform, "publisherid": bundle.Descriptor_.PublisherId, "servicename": bundle.Descriptor_.Name, "serviceid": bundle.Descriptor_.Id, "modified": time.Now().Unix(), "size": len(bundle.Binairies)})
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = p.ReplaceOne(context.Background(), "local_resource", "local_resource", "PackageBundle", `{"_id":"`+id+`"}`, jsonStr, `[{"upsert": true}]`)

	return nil, err
}
