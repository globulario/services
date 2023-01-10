package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"os"
	"sort"

	//"reflect"
	"strings"
	"time"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/resource/resource_client"
	"github.com/globulario/services/golang/resource/resourcepb"
	"github.com/globulario/services/golang/security"
	"github.com/golang/protobuf/jsonpb"
	"go.mongodb.org/mongo-driver/bson/primitive"

	// "go.mongodb.org/mongo-driver/x/mongo/driver/session"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

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
	jsonStr += `"domain":"` + account["domain"].(string) + `",`
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

	domain, _ := config.GetDomain()

	resource_server.publishEvent("update_account_"+rqst.AccountId+"_evt", []byte{}, domain)

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

	err := resource_server.registerAccount(rqst.Account.Domain, rqst.Account.Id, rqst.Account.Name, rqst.Account.Email, rqst.Account.Password, rqst.Account.Organizations, rqst.Account.Roles, rqst.Account.Groups)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Generate a token to identify the user.
	macAddress, err := Utility.MyMacAddr(Utility.MyLocalIP())
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	tokenString, _ := security.GenerateToken(resource_server.SessionTimeout, macAddress, rqst.Account.Id, rqst.Account.Name, rqst.Account.Email, rqst.Account.Domain)
	claims, err := security.ValidateToken(tokenString)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = resource_server.updateSession(claims.Id+"@"+claims.UserDomain, 0, time.Now().Unix(), claims.StandardClaims.ExpiresAt)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	domain, _ := config.GetDomain()
	var marshaler jsonpb.Marshaler
	jsonStr, err := marshaler.MarshalToString(rqst.Account)
	if err == nil {
		resource_server.publishEvent("create_account_evt", []byte(jsonStr), domain)
	}

	// Now I will
	return &resourcepb.RegisterAccountRsp{
		Result: tokenString, // Return the token string.
	}, nil
}

// * Return a given account
func (resource_server *server) GetAccount(ctx context.Context, rqst *resourcepb.GetAccountRqst) (*resourcepb.GetAccountRsp, error) {
	// That service made user of persistence service.
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	accountId := rqst.AccountId
	if strings.Contains(accountId, "@") {
		domain = strings.Split(accountId, "@")[1]
		accountId = strings.Split(accountId, "@")[0]

		_domain, err := config.GetDomain()
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		if _domain != domain {
			a, err := resource_server.getRemoteAccount(accountId, domain)
			if err != nil {
				return nil, status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}
			return &resourcepb.GetAccountRsp{
				Account: a, // Return the token string.
			}, nil

		}
	}

	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Accounts", `{"$or":[{"_id":"`+accountId+`"},{"name":"`+accountId+`"} ]}`, ``)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	account := values.(map[string]interface{})
	a := &resourcepb.Account{Id: account["_id"].(string), Name: account["name"].(string), Email: account["email"].(string), Password: account["password"].(string), Domain: account["domain"].(string)}
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

	// Now the profile picture.

	// set the caller id.
	db := accountId
	db = strings.ReplaceAll(strings.ReplaceAll(db, ".", "_"), "@", "_")
	db += "_db"

	user_data, err := p.FindOne(context.Background(), "local_resource", db, "user_data", `{"$or":[{"_id":"`+accountId+`"},{"name":"`+accountId+`"} ]}`, ``)
	if err == nil {
		// set the user infos....
		if user_data != nil {

			user_data_ := user_data.(map[string]interface{})
			if user_data_["profilePicture_"] != nil {
				a.ProfilePicture = user_data_["profilePicture_"].(string)
			}
			if user_data_["firstName_"] != nil {
				a.FirstName = user_data_["firstName_"].(string)
			}
			if user_data_["lastName_"] != nil {
				a.LastName = user_data_["lastName_"].(string)
			}
			if user_data_["middleName_"] != nil {
				a.Middle = user_data_["middleName_"].(string)
			}

		}
	} else {
		fmt.Println("fail to retreive user data ", db, accountId, err)
	}

	return &resourcepb.GetAccountRsp{
		Account: a, // Return the token string.
	}, nil

}

// * Update account password.
// TODO make sure only user can
func (resource_server *server) SetAccountPassword(ctx context.Context, rqst *resourcepb.SetAccountPasswordRqst) (*resourcepb.SetAccountPasswordRsp, error) {

	var clientId string
	var err error

	// Now I will index the conversation to be retreivable for it creator...
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		token := strings.Join(md["token"], "")
		if len(token) > 0 {
			claims, err := security.ValidateToken(token)

			if err != nil {
				return nil, status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}

			clientId = claims.Id + "@" + claims.UserDomain
		} else {
			return nil, errors.New("SetAccountPassword no token was given")
		}
	}

	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Accounts", `{"$or":[{"_id":"`+rqst.AccountId+`"},{"name":"`+rqst.AccountId+`"} ]}`, ``)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	account := values.(map[string]interface{})

	// Now update the sa password in mongo db.
	name := account["name"].(string)
	name = strings.ReplaceAll(strings.ReplaceAll(name, ".", "_"), "@", "_")

	// In case the request dosent came from the sa...
	if clientId != "sa" {
		err = resource_server.validatePassword(rqst.OldPassword, account["password"].(string))
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	// Change the password...
	changePasswordScript := fmt.Sprintf(
		"db=db.getSiblingDB('admin');db.changeUserPassword('%s','%s');", name, rqst.NewPassword)
	err = p.RunAdminCmd(context.Background(), "local_resource", resource_server.Backend_user, resource_server.Backend_password, changePasswordScript)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Create bcrypt...
	pwd, err := resource_server.hashPassword(rqst.NewPassword)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Hash the password...
	err = p.UpdateOne(context.Background(), "local_resource", "local_resource", "Accounts", `{"_id":"`+rqst.AccountId+`"}`, `{ "$set":{"password":"`+string(pwd)+`"}}`, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &resourcepb.SetAccountPasswordRsp{}, nil
}

// * Save an account
func (resource_server *server) SetAccount(ctx context.Context, rqst *resourcepb.SetAccountRqst) (*resourcepb.SetAccountRsp, error) {
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = p.UpdateOne(context.Background(), "local_resource", "local_resource", "Accounts", `{"_id":"`+rqst.Account.Id+`"}`, `{ "$set":{"name":"`+rqst.Account.Name+`"}, "$set":{"email":"`+rqst.Account.Email+`"}, "$set":{"domain":"`+rqst.Account.Domain+`"} }`, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Set values from the accound db itself.
	db := rqst.Account.Id
	db = strings.ReplaceAll(strings.ReplaceAll(db, ".", "_"), "@", "_")
	db += "_db"

	user_data, err := p.FindOne(context.Background(), "local_resource", db, "user_data", `{"$or":[{"_id":"`+rqst.Account.Id+`"},{"name":"`+rqst.Account.Id+`"} ]}`, ``)
	if err == nil {
		// set the user infos....
		if user_data != nil {
			user_data_ := user_data.(map[string]interface{})
			if user_data_["profilePicture_"] != nil {
				rqst.Account.ProfilePicture = user_data_["profilePicture_"].(string)
			}
			if user_data_["firstName_"] != nil {
				rqst.Account.FirstName = user_data_["firstName_"].(string)
			}
			if user_data_["lastName_"] != nil {
				rqst.Account.LastName = user_data_["lastName_"].(string)
			}
			if user_data_["middleName_"] != nil {
				rqst.Account.Middle = user_data_["middleName_"].(string)
			}

		}
	} else {
		fmt.Println("---> fail to retreive user data ", db, rqst.Account.Id, err)
	}

	return &resourcepb.SetAccountRsp{}, nil

}

// * Return the list accounts *
func (resource_server *server) GetAccounts(rqst *resourcepb.GetAccountsRqst, stream resourcepb.ResourceService_GetAccountsServer) error {
	// Get the persistence connection
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	query := rqst.Query
	if len(query) == 0 {
		query = "{}"
	}

	accounts, err := p.Find(context.Background(), "local_resource", "local_resource", "Accounts", query, rqst.Options)
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

		if account["domain"] != nil {
			a.Domain = account["domain"].(string)
		} else {
			a.Domain = resource_server.Domain
		}

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

		// set the caller id.
		db := a.Id
		db = strings.ReplaceAll(strings.ReplaceAll(db, ".", "_"), "@", "_")
		db += "_db"

		user_data, err := p.FindOne(context.Background(), "local_resource", db, "user_data", `{"$or":[{"_id":"`+a.Id+`"},{"name":"`+a.Id+`"} ]}`, ``)
		if err == nil {
			// set the user infos....
			if user_data != nil {
				user_data_ := user_data.(map[string]interface{})
				if user_data_["profilePicture_"] != nil {
					a.ProfilePicture = user_data_["profilePicture_"].(string)
				}
				if user_data_["firstName_"] != nil {
					a.FirstName = user_data_["firstName_"].(string)
				}
				if user_data_["lastName_"] != nil {
					a.LastName = user_data_["lastName_"].(string)
				}
				if user_data_["middleName_"] != nil {
					a.Middle = user_data_["middleName_"].(string)
				}

			}
		} else {
			fmt.Println("fail to retreive user data ", db, a.Id, err)
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

// * Add contact to a given account *
func (resource_server *server) SetAccountContact(ctx context.Context, rqst *resourcepb.SetAccountContactRqst) (*resourcepb.SetAccountContactRsp, error) {

	if rqst.Contact == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no contact was given")))
	}

	// Get the persistence connection
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	accountId := rqst.AccountId
	if strings.Contains(accountId, "@") {
		domain := strings.Split(accountId, "@")[1]
		localDomain, _ := config.GetDomain()
		if domain != localDomain {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("fail to get account "+accountId+" with domain "+domain+" from globule at domain "+localDomain)))
		}
		accountId = strings.Split(accountId, "@")[0]
	}

	// set the account id.
	db := accountId
	db = strings.ReplaceAll(strings.ReplaceAll(db, ".", "_"), "@", "_")
	db += "_db"

	sentInvitation := `{"_id":"` + rqst.Contact.Id + `", "invitationTime":` + Utility.ToString(rqst.Contact.InvitationTime) + `, "status":"` + rqst.Contact.Status + `", "ringtone":"` + rqst.Contact.Ringtone + `", "profilePicture":"` + rqst.Contact.ProfilePicture + `"}`

	err = p.ReplaceOne(context.Background(), "local_resource", db, "Contacts", `{"_id":"`+rqst.Contact.Id+`"}`, sentInvitation, `[{"upsert":true}]`)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// send event.
	resource_server.publishEvent("update_account_"+rqst.Contact.Id+"_evt", []byte{}, strings.Split(rqst.Contact.Id, "@")[1])
	resource_server.publishEvent("update_account_"+rqst.AccountId+"_evt", []byte{}, strings.Split(rqst.AccountId, "@")[1])

	return &resourcepb.SetAccountContactRsp{Result: true}, nil
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

	if strings.Contains(accountId, "@") {
		domain := strings.Split(accountId, "@")[1]
		accountId = strings.Split(accountId, "@")[0]

		localDomain, err := config.GetDomain()
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		// find account on other domain.
		if localDomain != domain {

			_, err := resource_server.getRemoteAccount(accountId, domain)
			if err != nil {
				return nil, status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}

			// return true.
			return &resourcepb.AccountExistRsp{
				Result: true,
			}, nil

		}

	}

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

// Test if account is a member of organization.
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

// * Test if an account is part of an organization *
func (resource_server *server) IsOrgnanizationMember(ctx context.Context, rqst *resourcepb.IsOrgnanizationMemberRqst) (*resourcepb.IsOrgnanizationMemberRsp, error) {
	result := resource_server.isOrganizationMemeber(rqst.AccountId, rqst.OrganizationId)

	return &resourcepb.IsOrgnanizationMemberRsp{
		Result: result,
	}, nil
}

// * Delete an account *
func (resource_server *server) DeleteAccount(ctx context.Context, rqst *resourcepb.DeleteAccountRqst) (*resourcepb.DeleteAccountRsp, error) {
	accountId := rqst.Id
	localDomain, _ := config.GetDomain()

	if strings.Contains(accountId, "@") {
		domain := strings.Split(accountId, "@")[1]
		accountId = strings.Split(accountId, "@")[0]

		if localDomain != domain {

			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("i cant's delete object from domain "+domain+" from domain "+localDomain)))
		}

	}

	// That service made user of persistence service.
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return nil, err
	}

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
			resource_server.deleteReference(p, rqst.Id, organizationId, "accounts", "Organizations")
			resource_server.publishEvent("update_organization_"+organizationId+"_evt", []byte{}, localDomain)
			if strings.Contains(organizationId, "@") {
				resource_server.publishEvent("update_organization_"+organizationId+"_evt", []byte{}, strings.Split(organizationId, "@")[1])
			}
		}
	}

	if account["groups"] != nil {
		groups := []interface{}(account["groups"].(primitive.A))
		for i := 0; i < len(groups); i++ {
			groupId := groups[i].(map[string]interface{})["$id"].(string)
			resource_server.deleteReference(p, rqst.Id, groupId, "members", "Groups")
			resource_server.publishEvent("update_group_"+groupId+"_evt", []byte{}, localDomain)
			if strings.Contains(groupId, "@") {
				resource_server.publishEvent("update_group_"+groupId+"_evt", []byte{}, strings.Split(groupId, "@")[1])
			}
		}
	}

	if account["roles"] != nil {
		roles := []interface{}(account["roles"].(primitive.A))
		for i := 0; i < len(roles); i++ {
			roleId := roles[i].(map[string]interface{})["$id"].(string)
			resource_server.deleteReference(p, rqst.Id, roleId, "members", "Roles")
			resource_server.publishEvent("update_role_"+roleId+"_evt", []byte{}, localDomain)
			if strings.Contains(roleId, "@") {
				resource_server.publishEvent("update_role_"+roleId+"_evt", []byte{}, strings.Split(roleId, "@")[1])
			}
		}

	}

	resource_server.deleteAllAccess(accountId+"@"+domain, rbacpb.SubjectType_ACCOUNT)

	// Try to delete the account...
	err = p.DeleteOne(context.Background(), "local_resource", "local_resource", "Accounts", `{"$or":[{"_id":"`+accountId+`"},{"name":"`+accountId+`"} ]}`, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	name := account["name"].(string)
	domain := account["domain"].(string)
	name = strings.ReplaceAll(strings.ReplaceAll(name, ".", "_"), "@", "_")

	// so before remove database I need to remove the accout from it contacts...
	contacts, err := p.Find(context.Background(), "local_resource", name+"_db", "Contacts", "{}", "")
	if err == nil {
		for i := 0; i < len(contacts); i++ {
			contact := contacts[i].(map[string]interface{})

			// So here I will call delete on the db...
			err = p.DeleteOne(context.Background(), "local_resource", contact["_id"].(string)+"_db", "Contacts", `{"_id":"`+name+`"}`, "")

			if err == nil {
				// Here I will send delete contact event.
				resource_server.publishEvent("update_account_"+contact["_id"].(string)+"@"+contact["domain"].(string)+"_evt", []byte{}, domain)
				resource_server.publishEvent("update_account_"+contact["_id"].(string)+"@"+contact["domain"].(string)+"_evt", []byte{}, contact["domain"].(string))
			}

		}
	}

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

	// Remove the file...
	resource_server.deleteResourcePermissions("/users/" + name + "@" + domain)

	os.RemoveAll(config.GetDataDir() + "/files/users/" + name + "@" + domain)

	// Publish delete account event.
	resource_server.publishEvent("delete_account_"+name+"@"+domain+"_evt", []byte{}, domain)
	resource_server.publishEvent("delete_account_evt", []byte(name+"@"+domain), domain)

	return &resourcepb.DeleteAccountRsp{
		Result: rqst.Id,
	}, nil
}

// Create an object reference inside another object, ex. add a reference to an account (field 'members') into a group.
func (resource_server *server) CreateReference(ctx context.Context, rqst *resourcepb.CreateReferenceRqst) (*resourcepb.CreateReferenceRsp, error) {
	// That service made user of persistence service.
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = resource_server.createReference(p, rqst.SourceId, rqst.SourceCollection, rqst.FieldName, rqst.TargetId, rqst.TargetCollection)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// reference was created...
	return &resourcepb.CreateReferenceRsp{}, nil
}

// Delete a reference from an object.
func (resource_server *server) DeleteReference(ctx context.Context, rqst *resourcepb.DeleteReferenceRqst) (*resourcepb.DeleteReferenceRsp, error) {
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = resource_server.deleteReference(p, rqst.RefId, rqst.TargetId, rqst.TargetField, rqst.TargetCollection)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &resourcepb.DeleteReferenceRsp{}, nil
}

/**
 * Crete a new role or Update existing one if it already exist.
 */

// * Create a role with given action list *
func (resource_server *server) CreateRole(ctx context.Context, rqst *resourcepb.CreateRoleRqst) (*resourcepb.CreateRoleRsp, error) {
	var clientId string
	var domain string
	var err error

	// Now I will index the conversation to be retreivable for it creator...
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		token := strings.Join(md["token"], "")
		if len(token) > 0 {
			claims, err := security.ValidateToken(token)

			if err != nil {
				return nil, status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}

			clientId = claims.Id + "@" + claims.UserDomain
			domain = claims.Domain
		} else {
			return nil, errors.New("SetAccountPassword no token was given")
		}
	}

	// That service made user of persistence service.
	err = resource_server.createRole(rqst.Role.Id, rqst.Role.Name, clientId+"@"+domain, rqst.Role.Actions)
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

	var marshaler jsonpb.Marshaler
	jsonStr, err := marshaler.MarshalToString(rqst.Role)
	if err == nil {
		resource_server.publishEvent("create_role_evt", []byte(jsonStr), domain)
	}

	return &resourcepb.CreateRoleRsp{Result: true}, nil
}

/**
 * Create a group with a given name of update existing one.
 */
func (resource_server *server) UpdateRole(ctx context.Context, rqst *resourcepb.UpdateRoleRqst) (*resourcepb.UpdateRoleRsp, error) {
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Get the persistence connection
	count, err := p.Count(context.Background(), "local_resource", "local_resource", "Roles", `{"_id":"`+rqst.RoleId+`"}`, "")
	if err != nil || count == 0 {
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	} else {

		err = p.UpdateOne(context.Background(), "local_resource", "local_resource", "Roles", `{"_id":"`+rqst.RoleId+`"}`, rqst.Values, "")
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	localDomain, err := config.GetDomain()
	resource_server.publishEvent("update_role_"+rqst.RoleId+"_evt", []byte{}, localDomain)

	return &resourcepb.UpdateRoleRsp{
		Result: true,
	}, nil
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

	roles, err := p.Find(context.Background(), "local_resource", "local_resource", "Roles", query, rqst.Options)
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
		r := &resourcepb.Role{Id: role["_id"].(string), Name: role["name"].(string), Domain: role["domain"].(string), Actions: make([]string, 0)}
		if role["domain"] != nil {
			r.Domain = role["domain"].(string)
		} else {
			r.Domain = resource_server.Domain
		}

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

// * Delete a role with a given id *
func (resource_server *server) DeleteRole(ctx context.Context, rqst *resourcepb.DeleteRoleRqst) (*resourcepb.DeleteRoleRsp, error) {

	// set the role id.
	roleId := rqst.RoleId
	localDomain, err := config.GetDomain()

	if strings.Contains(roleId, "@") {
		domain := strings.Split(roleId, "@")[1]
		roleId = strings.Split(roleId, "@")[0]

		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		if localDomain != domain {

			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("i cant's delete object from domain "+domain+" from domain "+localDomain)))
		}
	}

	// That service made user of persistence service.
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	// Remove references
	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Roles", `{"_id":"`+roleId+`"}`, ``)
	if err != nil {
		return nil, err
	}

	role := values.(map[string]interface{})

	// Remove it from the accounts
	if role["members"] != nil {
		accounts := []interface{}(role["members"].(primitive.A))
		for i := 0; i < len(accounts); i++ {
			accountId := accounts[i].(map[string]interface{})["$id"].(string)
			resource_server.deleteReference(p, accountId, roleId, "roles", "Accounts")
			resource_server.publishEvent("update_account_"+accountId+"_evt", []byte{}, strings.Split(accountId, "@")[1])
		}
	}

	// I will remove it from organizations...
	if role["organizations"] != nil {
		organizations := []interface{}(role["organizations"].(primitive.A))
		for i := 0; i < len(organizations); i++ {
			organizationId := organizations[i].(map[string]interface{})["$id"].(string)
			resource_server.deleteReference(p, rqst.RoleId, organizationId, "roles", "Roles")
			resource_server.publishEvent("update_organization_"+organizationId+"_evt", []byte{}, strings.Split(organizationId, "@")[1])
		}
	}

	err = p.DeleteOne(context.Background(), "local_resource", "local_resource", "Roles", `{"_id":"`+roleId+`"}`, ``)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// TODO delete role permissions
	resource_server.deleteResourcePermissions(rqst.RoleId)
	resource_server.publishEvent("delete_role_"+rqst.RoleId+"_evt", []byte{}, localDomain)
	resource_server.publishEvent("delete_role_evt", []byte(rqst.RoleId), localDomain)

	return &resourcepb.DeleteRoleRsp{Result: true}, nil
}

// * Append an action to existing role. *
func (resource_server *server) AddRoleActions(ctx context.Context, rqst *resourcepb.AddRoleActionsRqst) (*resourcepb.AddRoleActionsRsp, error) {
	roleId := rqst.RoleId
	localDomain, err := config.GetDomain()

	if strings.Contains(roleId, "@") {
		domain := strings.Split(roleId, "@")[1]
		roleId = strings.Split(roleId, "@")[0]

		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		if localDomain != domain {

			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("i cant's delete object from domain "+domain+" from domain "+localDomain)))
		}
	}

	// That service made user of persistence service.
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Roles", `{"_id":"`+roleId+`"}`, ``)
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

		// jsonStr, _ := json.Marshal(role)
		jsonStr := serialyseObject(role)

		err := p.ReplaceOne(context.Background(), "local_resource", "local_resource", "Roles", `{"_id":"`+roleId+`"}`, string(jsonStr), ``)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	resource_server.publishEvent("update_role_"+rqst.RoleId+"_evt", []byte{}, localDomain)

	return &resourcepb.AddRoleActionsRsp{Result: true}, nil
}

// * Remove an action to existing role. *
func (resource_server *server) RemoveRolesAction(ctx context.Context, rqst *resourcepb.RemoveRolesActionRqst) (*resourcepb.RemoveRolesActionRsp, error) {

	localDomain, _ := config.GetDomain()

	// That service made user of persistence service.
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	values, err := p.Find(context.Background(), "local_resource", "local_resource", "Roles", `{}`, ``)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	for i := 0; i < len(values); i++ {
		role := values[i].(map[string]interface{})

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
			}
		}

		if needSave {
			// jsonStr, _ := json.Marshal(role)
			jsonStr := serialyseObject(role)

			err := p.ReplaceOne(context.Background(), "local_resource", "local_resource", "Roles", `{"_id":"`+role["_id"].(string)+`"}`, string(jsonStr), ``)
			if err != nil {
				return nil, status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}

			resource_server.publishEvent("update_role_"+role["_id"].(string)+"@"+role["domain"].(string)+"_evt", []byte{}, localDomain)

		}
	}

	return &resourcepb.RemoveRolesActionRsp{Result: true}, nil
}

// * Remove an action to existing role. *
func (resource_server *server) RemoveRoleAction(ctx context.Context, rqst *resourcepb.RemoveRoleActionRqst) (*resourcepb.RemoveRoleActionRsp, error) {
	roleId := rqst.RoleId
	localDomain, err := config.GetDomain()
	if strings.Contains(roleId, "@") {
		domain := strings.Split(roleId, "@")[1]
		roleId = strings.Split(roleId, "@")[0]

		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		if localDomain != domain {

			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("i cant's delete object from domain "+domain+" from domain "+localDomain)))
		}
	}

	// That service made user of persistence service.
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Roles", `{"_id":"`+roleId+`"}`, ``)
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
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("Role named "+roleId+"not contain actions named "+rqst.Action+"!")))
		}
	}

	if needSave {
		// jsonStr, _ := json.Marshal(role)
		jsonStr := serialyseObject(role)

		err := p.ReplaceOne(context.Background(), "local_resource", "local_resource", "Roles", `{"_id":"`+roleId+`"}`, string(jsonStr), ``)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	resource_server.publishEvent("update_role_"+rqst.RoleId+"_evt", []byte{}, localDomain)

	return &resourcepb.RemoveRoleActionRsp{Result: true}, nil
}

// * Add role to a given account *
func (resource_server *server) AddAccountRole(ctx context.Context, rqst *resourcepb.AddAccountRoleRqst) (*resourcepb.AddAccountRoleRsp, error) {
	// That service made user of persistence service.
	err := resource_server.createCrossReferences(rqst.RoleId, "Roles", "members", rqst.AccountId, "Accounts", "roles")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	resource_server.publishEvent("update_role_"+rqst.RoleId+"_evt", []byte{}, strings.Split(rqst.RoleId, "@")[1])
	return &resourcepb.AddAccountRoleRsp{Result: true}, nil
}

// * Remove a role from a given account *
func (resource_server *server) RemoveAccountRole(ctx context.Context, rqst *resourcepb.RemoveAccountRoleRqst) (*resourcepb.RemoveAccountRoleRsp, error) {

	// That service made user of persistence service.
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	// That service made user of persistence service.
	err = resource_server.deleteReference(p, rqst.AccountId, rqst.RoleId, "members", "Roles")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = resource_server.deleteReference(p, rqst.RoleId, rqst.AccountId, "roles", "Accounts")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	resource_server.publishEvent("update_role_"+rqst.RoleId+"_evt", []byte{}, strings.Split(rqst.RoleId, "@")[1])
	resource_server.publishEvent("update_account_"+rqst.AccountId+"_evt", []byte{}, strings.Split(rqst.AccountId, "@")[1])

	return &resourcepb.RemoveAccountRoleRsp{Result: true}, nil
}

func (resource_server *server) save_application(app *resourcepb.Application, owner string) error {

	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return err
	}

	if app == nil {
		return errors.New("no application object was given in the request")
	}

	count, err := p.Count(context.Background(), "local_resource", "local_resource", "Applications", `{"_id":"`+app.Id+`"}`, "")

	application := make(map[string]interface{}, 0)
	application["_id"] = app.Id
	application["path"] = "/" + app.Id // The path must be the same as the application name.
	application["publisherid"] = app.Publisherid
	application["version"] = app.Version
	application["domain"] = resource_server.Domain // the domain where the application is save...
	application["description"] = app.Description
	application["actions"] = app.Actions
	application["keywords"] = app.Keywords
	application["icon"] = app.Icon
	application["alias"] = app.Alias

	// be sure the domain is set correctly
	if len(app.Domain) == 0 {
		app.Domain, _ = config.GetDomain()
	}

	application["domain"] = app.Domain

	application["password"] = app.Password
	if len(application["password"].(string)) == 0 {
		application["password"] = app.Id
	}
	application["store"] = p.GetStoreType()

	// Save the actual time.
	application["last_deployed"] = time.Now().Unix() // save it as unix time.

	// Here I will set the resource to manage the applicaiton access permission.
	if err != nil || count == 0 {

		// create the application database.
		createApplicationUserDbScript := fmt.Sprintf(
			"db=db.getSiblingDB('%s_db');db.createCollection('application_data');db=db.getSiblingDB('admin');db.createUser({user: '%s', pwd: '%s',roles: [{ role: 'dbOwner', db: '%s_db' }]});",
			app.Id, app.Id, app.Id, app.Id)

		err = p.RunAdminCmd(context.Background(), "local_resource", resource_server.Backend_user, resource_server.Backend_password, createApplicationUserDbScript)
		if err != nil {
			return err
		}

		application["creation_date"] = time.Now().Unix() // save it as unix time.
		_, err := p.InsertOne(context.Background(), "local_resource", "local_resource", "Applications", application, "")
		if err != nil {
			return err
		}

		// give time to mongodb...
		// create ressour ce application...
		defer resource_server.createApplicationConnection(app)

	} else {
		actions_, _ := Utility.ToJson(app.Actions)
		keywords_, _ := Utility.ToJson(app.Keywords)
		err := p.UpdateOne(context.Background(), "local_resource", "local_resource", "Applications", `{"_id":"`+app.Id+`"}`, `{ "$set":{ "last_deployed":`+Utility.ToString(time.Now().Unix())+` }, "$set":{"keywords":`+keywords_+`}, "$set":{"actions":`+actions_+`},"$set":{"publisherid":"`+app.Publisherid+`"},"$set":{"description":"`+app.Description+`"},"$set":{"alias":"`+app.Alias+`"},"$set":{"icon":"`+app.Icon+`"}, "$set":{"version":"`+app.Version+`"}}`, "")

		if err != nil {
			return err
		}
	}

	// Create the application file directory.
	path := "/applications/" + app.Id
	Utility.CreateDirIfNotExist(config.GetDataDir() + "/files" + path)

	// Add resource owner
	resource_server.addResourceOwner(path, "file", app.Id, rbacpb.SubjectType_APPLICATION)

	// Add application owner
	resource_server.addResourceOwner(app.Id+"@"+app.Domain, "application", owner, rbacpb.SubjectType_ACCOUNT)

	// Publish application.
	resource_server.publishEvent("update_application_"+app.Id+"@"+app.Domain+"_evt", []byte{}, app.Domain)

	return nil
}

// /////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// Application
// /////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
func (resource_server *server) CreateApplication(ctx context.Context, rqst *resourcepb.CreateApplicationRqst) (*resourcepb.CreateApplicationRsp, error) {

	var clientId string
	var domain string

	// Now I will index the conversation to be retreivable for it creator...
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		token := strings.Join(md["token"], "")
		if len(token) > 0 {
			claims, err := security.ValidateToken(token)
			if err != nil {
				return nil, status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}
			clientId = claims.Id + "@" + claims.UserDomain
			domain = claims.Domain
		} else {
			return nil, errors.New("resource server CreateApplication no token was given")
		}
	}

	err := resource_server.save_application(rqst.Application, clientId+"@"+domain)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	var marshaler jsonpb.Marshaler
	jsonStr, err := marshaler.MarshalToString(rqst.Application)
	if err == nil {
		resource_server.publishEvent("create_application_evt", []byte(jsonStr), domain)
	}

	return &resourcepb.CreateApplicationRsp{}, nil
}

// * Update application informations.
func (resource_server *server) UpdateApplication(ctx context.Context, rqst *resourcepb.UpdateApplicationRqst) (*resourcepb.UpdateApplicationRsp, error) {
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = p.UpdateOne(context.Background(), "local_resource", "local_resource", "Applications", `{"_id":"`+rqst.ApplicationId+`"}`, rqst.Values, "")

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	localDomain, _ := config.GetDomain()
	resource_server.publishEvent("update_application_"+rqst.ApplicationId+"_evt", []byte{}, localDomain)

	return &resourcepb.UpdateApplicationRsp{}, nil
}

// * Delete an application from the server. *
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

// * Append an action to existing application. *
func (resource_server *server) AddApplicationActions(ctx context.Context, rqst *resourcepb.AddApplicationActionsRqst) (*resourcepb.AddApplicationActionsRsp, error) {
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

	resource_server.publishEvent("update_application_"+rqst.ApplicationId+"_evt", []byte{}, application["domain"].(string))

	return &resourcepb.AddApplicationActionsRsp{Result: true}, nil
}

// * Remove an action to existing application. *
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

	resource_server.publishEvent("update_application_"+rqst.ApplicationId+"_evt", []byte{}, application["domain"].(string))

	return &resourcepb.RemoveApplicationActionRsp{Result: true}, nil
}

// * Remove an action to existing application. *
func (resource_server *server) RemoveApplicationsAction(ctx context.Context, rqst *resourcepb.RemoveApplicationsActionRqst) (*resourcepb.RemoveApplicationsActionRsp, error) {

	// That service made user of persistence service.
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	values, err := p.Find(context.Background(), "local_resource", "local_resource", "Applications", `{}`, ``)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	for i := 0; i < len(values); i++ {
		application := values[i].(map[string]interface{})

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
			}
		}

		if needSave {
			jsonStr := serialyseObject(application)
			err := p.ReplaceOne(context.Background(), "local_resource", "local_resource", "Applications", `{"_id":"`+application["_id"].(string)+`"}`, string(jsonStr), ``)
			resource_server.publishEvent("update_application_"+application["_id"].(string)+"_evt", []byte{}, application["domain"].(string))
			if err != nil {
				return nil, status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}
		}
	}

	return &resourcepb.RemoveApplicationsActionRsp{Result: true}, nil
}

// /////////////////////  resource management. /////////////////
func (resource_server *server) GetApplications(rqst *resourcepb.GetApplicationsRqst, stream resourcepb.ResourceService_GetApplicationsServer) error {
	// That service made user of persistence service.
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return err
	}
	query := rqst.Query
	if len(query) == 0 {
		query = "{}" // all
	}

	// So here I will get the list of retreived permission.

	values, err := p.Find(context.Background(), "local_resource", "local_resource", "Applications", query, rqst.Options)
	if err != nil {
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Convert to Application.
	for i := 0; i < len(values); i++ {
		values_ := values[i].(map[string]interface{})

		if values_["icon"] == nil {
			values_["icon"] = ""
		}

		if values_["alias"] == nil {
			values_["alias"] = ""
		}

		// Set the date
		creationDate := int64(Utility.ToInt(values_["creation_date"]))
		lastDeployed := int64(Utility.ToInt(values_["last_deployed"]))

		// Here I will also append the list of actions.
		actions := make([]string, 0)
		if values_["actions"] != nil {
			actions_ := []interface{}(values_["actions"].(primitive.A))

			for i := 0; i < len(actions_); i++ {
				actions = append(actions, actions_[i].(string))
			}
		}
		application := &resourcepb.Application{Id: values_["_id"].(string), Name: values_["_id"].(string), Domain: values_["domain"].(string), Path: values_["path"].(string), CreationDate: creationDate, LastDeployed: lastDeployed, Alias: values_["alias"].(string), Icon: values_["icon"].(string), Description: values_["description"].(string), Publisherid: values_["publisherid"].(string), Version: values_["version"].(string), Actions: actions}

		// TODO validate token...
		application.Password = values_["password"].(string)

		err := stream.Send(&resourcepb.GetApplicationsRsp{
			Applications: []*resourcepb.Application{application},
		})
		if err != nil {
			return err
		}
	}

	return nil

}

// //////////////////////////////////////////////////////////////////////////////
// Peer's Authorization and Authentication code.
// //////////////////////////////////////////////////////////////////////////////
func getLocalPeer() *resourcepb.Peer {
	// Now I will return peers actual informations.
	hostname, _ := os.Hostname()
	domain, _ := config.GetDomain()
	localConfig, _ := config.GetLocalConfig(true)

	local_peer_ := new(resourcepb.Peer)
	local_peer_.Protocol = localConfig["Protocol"].(string)
	local_peer_.PortHttp = int32(Utility.ToInt(localConfig["PortHttp"]))
	local_peer_.PortHttps = int32(Utility.ToInt(localConfig["PortHttps"]))
	local_peer_.Hostname = hostname
	local_peer_.Domain = domain
	local_peer_.ExternalIpAddress = Utility.MyIP()
	local_peer_.LocalIpAddress = Utility.MyLocalIP()
	local_peer_.Mac, _ = Utility.MyMacAddr(local_peer_.LocalIpAddress)
	local_peer_.State = resourcepb.PeerApprovalState_PEER_PENDING

	return local_peer_
}

// Register the actual peer (the one that running the resource server) to the one
// running at domain.
func (resource_server *server) registerPeer(token, address string) (*resourcepb.Peer, string, error) {
	// Connect to remote server and call Register peer on it...
	fmt.Println("connect to ressource client at address: ", address)
	client, err := resource_client.NewResourceService_Client(address, "resource.ResourceService")
	if err != nil {
		fmt.Println("1896 fail to connect with client with error ", err)
		return nil, "", err
	}

	// get the local public key.
	key, err := security.GetLocalKey()
	if err != nil {
		fmt.Println("fail to get local key with error ", err)
		return nil, "", err
	}

	// Get the configuration address with it http port...
	domain, _ := config.GetDomain()
	hostname, err := os.Hostname()
	if err != nil {
		return nil, "", err
	}
	macAddress, err := Utility.MyMacAddr(Utility.MyLocalIP())

	localConfig, err := config.GetLocalConfig(true)
	httpPort := Utility.ToInt(localConfig["PortHttp"])
	httpsPort := Utility.ToInt(localConfig["PortHttps"])
	protocol := localConfig["Protocol"].(string)

	if err != nil {
		fmt.Println("fail to get local config ", err)
		return nil, "", err
	}

	return client.RegisterPeer(token, string(key), &resourcepb.Peer{Protocol: protocol, PortHttp: int32(httpPort), PortHttps: int32(httpsPort), Hostname: hostname, Mac: macAddress, Domain: domain, ExternalIpAddress: Utility.MyIP(), LocalIpAddress: Utility.MyLocalIP()})

}

// * Connect tow peer toggether on the network.
func (resource_server *server) RegisterPeer(ctx context.Context, rqst *resourcepb.RegisterPeerRqst) (*resourcepb.RegisterPeerRsp, error) {


	// Get the persistence connection
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// set the remote peer in /etc/hosts
	resource_server.setLocalHosts(rqst.Peer)

	// Here I will first look if a peer with a same name already exist on the
	// resources...
	if len(rqst.Peer.Mac) > 0 {
		values, _ := p.FindOne(context.Background(), "local_resource", "local_resource", "Peers", `{"_id":"`+Utility.GenerateUUID(rqst.Peer.Mac)+`"}`, "")
		if values != nil {
			p := initPeer(values)
			pubKey, err := security.GetPeerKey(p.Mac)
			if err != nil {
				return nil, status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}

			return &resourcepb.RegisterPeerRsp{
				Peer:      p,
				PublicKey: string(pubKey),
			}, nil
		}
	}

	// No authorization exist for that peer I will insert it.
	// Here will create the new peer.
	peer := make(map[string]interface{})
	peer["hostname"] = rqst.Peer.Hostname
	peer["domain"] = rqst.Peer.Domain

	var marshaler jsonpb.Marshaler

	// If no mac address was given it mean the request came from a web application
	// so the intention is to register the server itself on another server...
	// This can also be done with the command line tool but in that case all values will be
	// set on the peers...
	if len(rqst.Peer.Mac) == 0 {
		if md, ok := metadata.FromIncomingContext(ctx); ok {
			token := strings.Join(md["token"], "")

			address_ := rqst.Peer.Domain
			if rqst.Peer.Protocol == "https" {
				address_ += ":" + Utility.ToString(rqst.Peer.PortHttps)
			} else {
				address_ += ":" + Utility.ToString(rqst.Peer.PortHttp)
			}

			// In that case I want to register the server to another server.
			peer_, public_key, err := resource_server.registerPeer(token, address_)
			if err != nil {
				return nil, status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}

			// Save the received values on the db
			peer := make(map[string]interface{})
			peer["_id"] = Utility.GenerateUUID(peer_.Mac) // The peer mac address will be use as peers id
			peer["domain"] = peer_.Domain

			// keep the address where the configuration can be found...
			// in case of docker instance that will be usefull to get peer addres config...
			peer["protocol"] = rqst.Peer.Protocol
			peer["portHttps"] = rqst.Peer.PortHttps
			peer["portHttp"] = rqst.Peer.PortHttp
			peer["hostname"] = peer_.Hostname
			peer["mac"] = peer_.Mac
			peer["local_ip_address"] = peer_.LocalIpAddress
			peer["external_ip_address"] = peer_.ExternalIpAddress
			peer["state"] = resourcepb.PeerApprovalState_PEER_ACCETEP
			peer["actions"] = []interface{}{}

			if err != nil {
				return nil, status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}

			_, err = p.InsertOne(context.Background(), "local_resource", "local_resource", "Peers", peer, "")
			if err != nil {
				return nil, status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}

			// Here I wiil save the public key in the keys directory.
			err = security.SetPeerPublicKey(peer_.Mac, public_key)
			if err != nil {
				return nil, status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}

			// set the remote peer in /etc/hosts
			resource_server.setLocalHosts(peer_)

			// in case local dns is use that peers will be able to change values releated to it domain.
			// but no other peer will be able to do it...
			resource_server.addResourceOwner(peer_.Domain, "domain", peer_.Mac, rbacpb.SubjectType_PEER)

			jsonStr, err := marshaler.MarshalToString(peer_)
			if err != nil {
				return nil, err
			}

			// Update peer event.
			localDomain, _ := config.GetDomain()
			resource_server.publishEvent("update_peers_evt", []byte(jsonStr), localDomain)

			address := rqst.Peer.Domain
			if rqst.Peer.Protocol == "https" {
				address += ":" + Utility.ToString(rqst.Peer.PortHttps)
			} else {
				address += ":" + Utility.ToString(rqst.Peer.PortHttp)
			}

			// So here I need to publish my information as a pee

			// Publish local peer information...
			jsonStr, err = marshaler.MarshalToString(getLocalPeer())
			if err != nil {
				return nil, err
			}

			resource_server.publishRemoteEvent(address, "update_peers_evt", []byte(jsonStr))

			// Set peer action
			resource_server.addPeerActions(peer_.Mac, []string{"/dns.DnsService/SetA"})
			resource_server.addPeerActions(peer_.Mac, []string{"/dns.DnsService/SetAAAA"})
			resource_server.addPeerActions(peer_.Mac, []string{"/dns.DnsService/SetCAA"})
			resource_server.addPeerActions(peer_.Mac, []string{"/dns.DnsService/SetText"})
			resource_server.addPeerActions(peer_.Mac, []string{"/dns.DnsService/RemoveText"})

			// Send back the peers informations.
			return &resourcepb.RegisterPeerRsp{Peer: peer_, PublicKey: public_key}, nil

		} else {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("resource server RegisterPeer no token was given")))

		}
	}

	// Here I will keep the peer info until it will be accepted by the admin of the other peer.
	peer["_id"] = Utility.GenerateUUID(rqst.Peer.Mac)
	peer["mac"] = rqst.Peer.Mac
	peer["protocol"] = rqst.Peer.Protocol
	peer["portHttps"] = rqst.Peer.PortHttps
	peer["portHttp"] = rqst.Peer.PortHttp
	peer["local_ip_address"] = rqst.Peer.LocalIpAddress
	peer["external_ip_address"] = rqst.Peer.ExternalIpAddress
	peer["state"] = resourcepb.PeerApprovalState_PEER_PENDING
	peer["actions"] = []interface{}{}

	// if the token is generate by the sa and it has permission i will accept the peer directly
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		token := strings.Join(md["token"], "")
		if len(token) > 0 {
			claims, err := security.ValidateToken(token)
			if err == nil {
				if claims.Id == "sa" {
					peer["state"] = resourcepb.PeerApprovalState_PEER_ACCETEP
					peer["actions"] = []interface{}{"/dns.DnsService/SetA"}
					peer["actions"] = []interface{}{"/dns.DnsService/SetAAAA"}
					peer["actions"] = []interface{}{"/dns.DnsService/SetCAA"}
					peer["actions"] = []interface{}{"/dns.DnsService/SetText"}
					peer["actions"] = []interface{}{"/dns.DnsService/RemoveText"}
					domain := rqst.Peer.Hostname
					if len(rqst.Peer.Domain) > 0 {
						domain += "." + rqst.Peer.Domain
					}
					resource_server.addResourceOwner(domain, "domain", rqst.Peer.Mac, rbacpb.SubjectType_PEER)
				}
			}
		}
	}

	// Insert the peer into the local resource database.
	_, err = p.InsertOne(context.Background(), "local_resource", "local_resource", "Peers", peer, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Here I wiil save the public key in the keys directory.
	err = security.SetPeerPublicKey(rqst.Peer.Mac, rqst.PublicKey)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// actions will need to be set by admin latter...
	pubKey, err := security.GetPeerKey(getLocalPeer().Mac)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	jsonStr, err := marshaler.MarshalToString(rqst.Peer)
	if err != nil {
		return nil, err
	}

	// signal peers changes...
	localDomain, _ := config.GetDomain()
	resource_server.publishEvent("update_peers_evt", []byte(jsonStr), localDomain)

	address_ := rqst.Peer.Domain
	if rqst.Peer.Protocol == "https" {
		address_ += ":" + Utility.ToString(rqst.Peer.PortHttps)
	} else {
		address_ += ":" + Utility.ToString(rqst.Peer.PortHttp)
	}

	jsonStr, err = marshaler.MarshalToString(getLocalPeer())
	if err != nil {
		return nil, err
	}
	resource_server.publishRemoteEvent(address_, "update_peers_evt", []byte(jsonStr))

	// set the remote peer in /etc/hosts
	resource_server.setLocalHosts(getLocalPeer())

	return &resourcepb.RegisterPeerRsp{
		Peer:      getLocalPeer(),
		PublicKey: string(pubKey),
	}, nil
}

// * Return the peer public key */
func (resource_server *server) GetPeerPublicKey(ctx context.Context, rqst *resourcepb.GetPeerPublicKeyRqst) (*resourcepb.GetPeerPublicKeyRsp, error) {
	public_key, err := resource_server.getPeerPublicKey(rqst.RemotePeerAddress, rqst.Mac)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &resourcepb.GetPeerPublicKeyRsp{PublicKey: public_key}, nil
}

// * Accept a given peer *
func (resource_server *server) AcceptPeer(ctx context.Context, rqst *resourcepb.AcceptPeerRqst) (*resourcepb.AcceptPeerRsp, error) {
	// Get the persistence connection
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = p.UpdateOne(context.Background(), "local_resource", "local_resource", "Peers", `{"mac":"`+rqst.Peer.Mac+`"}`, `{ "$set":{"state":1}}`, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Add actions require by peer...
	resource_server.addPeerActions(rqst.Peer.Mac, []string{"/dns.DnsService/SetA"})
	resource_server.addPeerActions(rqst.Peer.Mac, []string{"/dns.DnsService/SetAAAA"})
	resource_server.addPeerActions(rqst.Peer.Mac, []string{"/dns.DnsService/SetCAA"})
	resource_server.addPeerActions(rqst.Peer.Mac, []string{"/dns.DnsService/SetText"})
	resource_server.addPeerActions(rqst.Peer.Mac, []string{"/dns.DnsService/RemoveText"})

	// set the remote peer in /etc/hosts
	resource_server.setLocalHosts(rqst.Peer)

	// Here I will append the resource owner...
	domain := rqst.Peer.Hostname
	if len(rqst.Peer.Domain) > 0 {
		domain += "." + rqst.Peer.Domain
	}

	// in case local dns is use that peers will be able to change values releated to it domain.
	// but no other peer will be able to do it...
	resource_server.addResourceOwner(domain, "domain", rqst.Peer.Mac, rbacpb.SubjectType_PEER)
	var marshaler jsonpb.Marshaler
	jsonStr, err := marshaler.MarshalToString(rqst.Peer)
	if err != nil {
		return nil, err
	}

	// signal peers changes...
	localDomain, _ := config.GetDomain()
	resource_server.publishEvent("update_peers_evt", []byte(jsonStr), localDomain)

	address_ := rqst.Peer.Domain
	if rqst.Peer.Protocol == "https" {
		address_ += ":" + Utility.ToString(rqst.Peer.PortHttps)
	} else {
		address_ += ":" + Utility.ToString(rqst.Peer.PortHttp)
	}

	jsonStr, err = marshaler.MarshalToString(getLocalPeer())
	if err != nil {
		return nil, err
	}
	resource_server.publishRemoteEvent(address_, "update_peers_evt", []byte(jsonStr))

	return &resourcepb.AcceptPeerRsp{Result: true}, nil
}

// * Reject a given peer, note that the peer will stay reject, so
// I will be imposible to request again and again, util it will be
// explicitly removed from the peer's list
func (resource_server *server) RejectPeer(ctx context.Context, rqst *resourcepb.RejectPeerRqst) (*resourcepb.RejectPeerRsp, error) {

	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = p.UpdateOne(context.Background(), "local_resource", "local_resource", "Peers", `{"mac":"`+rqst.Peer.Mac+`"}`, `{ "$set":{"state":2}}`, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	var marshaler jsonpb.Marshaler
	jsonStr, err := marshaler.MarshalToString(rqst.Peer)
	if err != nil {
		return nil, err
	}

	// signal peers changes...
	localDomain, _ := config.GetDomain()
	resource_server.publishEvent("update_peers_evt", []byte(jsonStr), localDomain)

	address_ := rqst.Peer.Domain
	if rqst.Peer.Protocol == "https" {
		address_ += ":" + Utility.ToString(rqst.Peer.PortHttps)
	} else {
		address_ += ":" + Utility.ToString(rqst.Peer.PortHttp)
	}

	jsonStr, err = marshaler.MarshalToString(getLocalPeer())
	if err != nil {
		return nil, err
	}
	resource_server.publishRemoteEvent(address_, "update_peers_evt", []byte(jsonStr))

	return &resourcepb.RejectPeerRsp{Result: true}, nil
}

/**
 * Return the state of approval of a peer by anther one.
 */
func (resource_server *server) GetPeerApprovalState(ctx context.Context, rqst *resourcepb.GetPeerApprovalStateRqst) (*resourcepb.GetPeerApprovalStateRsp, error) {
	mac := rqst.Mac
	if len(mac) == 0 {
		var err error
		mac, err = Utility.MyMacAddr(Utility.MyLocalIP())
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	fmt.Println("try to get peer info from ", rqst.RemotePeerAddress)
	peer, err := resource_server.getPeerInfos(rqst.RemotePeerAddress, mac)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &resourcepb.GetPeerApprovalStateRsp{State: peer.GetState()}, nil
}

func initPeer(values interface{}) *resourcepb.Peer {
	state := resourcepb.PeerApprovalState(values.(map[string]interface{})["state"].(int32))
	p := &resourcepb.Peer{Protocol: values.(map[string]interface{})["protocol"].(string), PortHttp: int32(Utility.ToInt(values.(map[string]interface{})["portHttp"])), PortHttps: int32(Utility.ToInt(values.(map[string]interface{})["portHttps"])), Hostname: values.(map[string]interface{})["hostname"].(string), Domain: values.(map[string]interface{})["domain"].(string), ExternalIpAddress: values.(map[string]interface{})["external_ip_address"].(string), LocalIpAddress: values.(map[string]interface{})["local_ip_address"].(string), Mac: values.(map[string]interface{})["mac"].(string), Actions: make([]string, 0), State: state}
	values.(map[string]interface{})["actions"] = []interface{}(values.(map[string]interface{})["actions"].(primitive.A))
	for j := 0; j < len(values.(map[string]interface{})["actions"].([]interface{})); j++ {
		p.Actions = append(p.Actions, values.(map[string]interface{})["actions"].([]interface{})[j].(string))
	}

	return p
}

// * Return the list of authorized peers *
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

	peers, err := p.Find(context.Background(), "local_resource", "local_resource", "Peers", query, rqst.Options)
	if err != nil {
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// No I will stream the result over the networks.
	maxSize := 100
	values := make([]*resourcepb.Peer, 0)

	for i := 0; i < len(peers); i++ {
		p := initPeer(peers[i])
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

func (resource_server *server) deletePeer(token, address string) error {
	// Connect to remote server and call Register peer on it...
	client, err := resource_client.NewResourceService_Client(address, "resource.ResourceService")
	if err != nil {
		return err
	}

	macAddress, err := Utility.MyMacAddr(Utility.MyLocalIP())
	if err != nil {
		return err
	}

	return client.DeletePeer(token, macAddress)

}

// * Update a peer
func (resource_server *server) UpdatePeer(ctx context.Context, rqst *resourcepb.UpdatePeerRqst) (*resourcepb.UpdatePeerRsp, error) {
	// Get the persistence connection
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	values, err := p.FindOne(ctx, "local_resource", "local_resource", "Peers", `{"mac":"`+rqst.Peer.Mac+`"}`, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	peer := values.(map[string]interface{})

	// Here I will save only value that can change over time.
	peer["protocol"] = rqst.Peer.Protocol
	peer["portHttps"] = rqst.Peer.PortHttps
	peer["portHttp"] = rqst.Peer.PortHttp
	peer["local_ip_address"] = rqst.Peer.LocalIpAddress
	peer["external_ip_address"] = rqst.Peer.ExternalIpAddress

	jsonStr, _ := Utility.ToJson(peer)

	// Save the peer.
	err = p.ReplaceOne(context.Background(), "local_resource", "local_resource", "Peers", `{"mac":"`+rqst.Peer.Mac+`"}`, jsonStr, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	var marshaler jsonpb.Marshaler
	jsonStr, err = marshaler.MarshalToString(rqst.Peer)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	localDomain, _ := config.GetDomain()

	// signal peers changes...
	resource_server.publishEvent("update_peer_"+rqst.Peer.Mac+"_evt", []byte{}, localDomain)
	resource_server.publishEvent("update_peer_"+rqst.Peer.Mac+"_evt", []byte{}, rqst.Peer.Domain)

	// give the peer information...
	resource_server.publishEvent("update_peers_evt", []byte(jsonStr), localDomain)

	return &resourcepb.UpdatePeerRsp{Result: true}, nil
}

// * Remove a peer from the network *
func (resource_server *server) DeletePeer(ctx context.Context, rqst *resourcepb.DeletePeerRqst) (*resourcepb.DeletePeerRsp, error) {
	// Get the persistence connection
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	count, err := p.Count(context.Background(), "local_resource", "local_resource", "Peers", `{"mac":"`+rqst.Peer.Mac+`"}`, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// not an error...
	if count == 0 {
		return &resourcepb.DeletePeerRsp{
			Result: true,
		}, nil
	}

	resource_server.deleteAllAccess(rqst.Peer.Mac, rbacpb.SubjectType_PEER)

	err = p.DeleteOne(context.Background(), "local_resource", "local_resource", "Peers", `{"mac":"`+rqst.Peer.Mac+`"}`, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Delete permissions
	err = p.Delete(context.Background(), "local_resource", "local_resource", "Permissions", `{"owner":"`+rqst.Peer.Mac+`"}`, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Delete peer public key...
	security.DeletePublicKey(rqst.Peer.Mac)

	// remove from /etc/hosts
	resource_server.removeFromLocalHosts(rqst.Peer)

	// Here I will append the resource owner...
	domain := rqst.Peer.Hostname
	if len(rqst.Peer.Domain) > 0 {
		domain += "." + rqst.Peer.Domain
	}

	// remove permission associated with that peer...
	resource_server.deleteResourcePermissions(domain)

	// signal peers changes...
	localDomain, _ := config.GetDomain()
	resource_server.publishEvent("delete_peer"+rqst.Peer.Mac+"_evt", []byte{}, localDomain)
	resource_server.publishEvent("delete_peer"+rqst.Peer.Mac+"_evt", []byte{}, rqst.Peer.Domain)
	resource_server.publishEvent("delete_peer_evt", []byte(rqst.Peer.Mac), localDomain)
	resource_server.publishEvent("delete_peer_evt", []byte(rqst.Peer.Mac), rqst.Peer.Domain)

	address_ := rqst.Peer.Domain
	if rqst.Peer.Protocol == "https" {
		address_ += ":" + Utility.ToString(rqst.Peer.PortHttps)
	} else {
		address_ += ":" + Utility.ToString(rqst.Peer.PortHttp)
	}

	// Also remove the peer at the other end...
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		token := strings.Join(md["token"], "")
		resource_server.deletePeer(token, address_)
	}

	return &resourcepb.DeletePeerRsp{
		Result: true,
	}, nil
}

func (resource_server *server) addPeerActions(mac string, actions_ []string) error {
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return err
	}

	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Peers", `{"mac":"`+mac+`"}`, ``)
	if err != nil {
		return err
	}

	peer := values.(map[string]interface{})

	needSave := false
	if peer["actions"] == nil {
		peer["actions"] = actions_
		needSave = true
	} else {
		actions := []interface{}(peer["actions"].(primitive.A))
		for j := 0; j < len(actions_); j++ {
			exist := false
			for i := 0; i < len(peer["actions"].(primitive.A)); i++ {
				if peer["actions"].(primitive.A)[i].(string) == actions_[j] {
					exist = true
					break
				}
			}
			if !exist {
				actions = append(actions, actions_[j])
				needSave = true
			}
		}
		peer["actions"] = actions
	}

	if needSave {
		jsonStr := serialyseObject(peer)
		err := p.ReplaceOne(context.Background(), "local_resource", "local_resource", "Peers", `{"mac":"`+mac+`"}`, string(jsonStr), ``)
		if err != nil {
			return err
		}
	}

	// signal peers changes...
	localDomain, _ := config.GetDomain()
	resource_server.publishEvent("update_peer_"+mac+"_evt", []byte{}, localDomain)

	return nil
}

// * Add peer action permission *
func (resource_server *server) AddPeerActions(ctx context.Context, rqst *resourcepb.AddPeerActionsRqst) (*resourcepb.AddPeerActionsRsp, error) {

	err := resource_server.addPeerActions(rqst.Mac, rqst.Actions)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	localDomain, _ := config.GetDomain()
	resource_server.publishEvent("update_peer_"+rqst.Mac+"_evt", []byte{}, localDomain)

	return &resourcepb.AddPeerActionsRsp{Result: true}, nil

}

// * Remove peer action permission *
func (resource_server *server) RemovePeerAction(ctx context.Context, rqst *resourcepb.RemovePeerActionRqst) (*resourcepb.RemovePeerActionRsp, error) {
	// That service made user of persistence service.
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Peers", `{"mac":"`+rqst.Mac+`"}`, ``)
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
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("Peer "+rqst.Mac+" not contain actions named "+rqst.Action+"!")))
		}
	}

	if needSave {
		jsonStr := serialyseObject(peer)
		err := p.ReplaceOne(context.Background(), "local_resource", "local_resource", "Peers", `{"mac":"`+rqst.Mac+`"}`, string(jsonStr), ``)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	localDomain, _ := config.GetDomain()

	// signal peers changes...
	resource_server.publishEvent("update_peer_"+rqst.Mac+"_evt", []byte{}, localDomain)

	return &resourcepb.RemovePeerActionRsp{Result: true}, nil
}

func (resource_server *server) RemovePeersAction(ctx context.Context, rqst *resourcepb.RemovePeersActionRqst) (*resourcepb.RemovePeersActionRsp, error) {
	// That service made user of persistence service.
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	values, err := p.Find(context.Background(), "local_resource", "local_resource", "Peers", `{}`, ``)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	for i := 0; i < len(values); i++ {
		peer := values[i].(map[string]interface{})

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
			}
		}

		if needSave {
			localDomain, _ := config.GetDomain()
			jsonStr := serialyseObject(peer)
			err := p.ReplaceOne(context.Background(), "local_resource", "local_resource", "Peers", `{"_id":"`+peer["_id"].(string)+`"}`, string(jsonStr), ``)
			resource_server.publishEvent("update_peer_"+peer["_id"].(string)+"_evt", []byte{}, localDomain)
			if err != nil {
				return nil, status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}
		}
	}

	return &resourcepb.RemovePeersActionRsp{Result: true}, nil
}

/////////////////////////////////////////////////////////////////////////////////////////
// Organization
/////////////////////////////////////////////////////////////////////////////////////////

// * Register a new organization
func (resource_server *server) CreateOrganization(ctx context.Context, rqst *resourcepb.CreateOrganizationRqst) (*resourcepb.CreateOrganizationRsp, error) {

	var clientId string
	var domain string

	// Now I will index the conversation to be retreivable for it creator...
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		token := strings.Join(md["token"], "")
		if len(token) > 0 {
			claims, err := security.ValidateToken(token)
			if err != nil {
				return nil, status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}
			clientId = claims.Id + "@" + claims.UserDomain
			domain = claims.Domain
		} else {
			return nil, errors.New("resource server CreateOrganization no token was given")
		}
	}

	// Get the persistence connection
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	// Here I will first look if a peer with a same name already exist on the
	// resources...
	count, _ := p.Count(context.Background(), "local_resource", "local_resource", "Organizations", `{"$or":[{"_id":"`+rqst.Organization.Id+`"},{"name":"`+rqst.Organization.Id+`"},{"name":"`+rqst.Organization.Name+`"} ]}`, "")
	if count > 0 {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("Organization with name '"+rqst.Organization.Id+"' already exist!")))
	}

	localDomain, err := config.GetDomain()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// test if the given domain is the local domain.
	if rqst.Organization.Domain != localDomain {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("you can't register group "+rqst.Organization.Id+" with domain "+rqst.Organization.Domain+" on domain "+localDomain)))
	}

	// No authorization exist for that peer I will insert it.
	// Here will create the new peer.
	o := make(map[string]interface{}, 0)
	o["_id"] = rqst.Organization.Id
	o["name"] = rqst.Organization.Name
	o["icon"] = rqst.Organization.Icon
	o["email"] = rqst.Organization.Email
	o["description"] = rqst.Organization.Email
	o["domain"] = resource_server.Domain

	// Those are the list of entity linked to the organization
	o["accounts"] = make([]interface{}, 0)
	o["groups"] = make([]interface{}, 0)
	o["roles"] = make([]interface{}, 0)
	o["applications"] = make([]interface{}, 0)

	_, err = p.InsertOne(context.Background(), "local_resource", "local_resource", "Organizations", o, "")
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

	var marshaler jsonpb.Marshaler
	jsonStr, err := marshaler.MarshalToString(rqst.Organization)
	if err == nil {
		localDomain, _ := config.GetDomain()
		resource_server.publishEvent("create_organization_evt", []byte(jsonStr), localDomain)
	}

	// create the resource owner.
	resource_server.addResourceOwner(rqst.Organization.GetId()+"@"+rqst.Organization.Domain, "organization", clientId+"@"+domain, rbacpb.SubjectType_ACCOUNT)

	return &resourcepb.CreateOrganizationRsp{
		Result: true,
	}, nil
}

// Update an organization informations.
func (resource_server *server) UpdateOrganization(ctx context.Context, rqst *resourcepb.UpdateOrganizationRqst) (*resourcepb.UpdateOrganizationRsp, error) {
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Get the persistence connection
	count, err := p.Count(context.Background(), "local_resource", "local_resource", "Organizations", `{"_id":"`+rqst.OrganizationId+`"}`, "")
	if err != nil || count == 0 {
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	} else {

		err = p.UpdateOne(context.Background(), "local_resource", "local_resource", "Organizations", `{"_id":"`+rqst.OrganizationId+`"}`, rqst.Values, "")
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	localDomain, _ := config.GetDomain()
	resource_server.publishEvent("update_organization_"+rqst.OrganizationId+"_evt", []byte{}, localDomain)

	return &resourcepb.UpdateOrganizationRsp{
		Result: true,
	}, nil
}

// * Return the list of organizations
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

	organizations, err := p.Find(context.Background(), "local_resource", "local_resource", "Organizations", query, rqst.Options)
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
		organization.Icon = o["icon"].(string)
		organization.Description = o["description"].(string)
		organization.Email = o["email"].(string)
		if o["domain"] != nil {
			organization.Domain = o["domain"].(string)
		} else {
			organization.Domain = resource_server.Domain
		}

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

// * Add Account *
func (resource_server *server) AddOrganizationAccount(ctx context.Context, rqst *resourcepb.AddOrganizationAccountRqst) (*resourcepb.AddOrganizationAccountRsp, error) {
	err := resource_server.createCrossReferences(rqst.OrganizationId, "Organizations", "accounts", rqst.AccountId, "Accounts", "organizations")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	resource_server.publishEvent("update_organization_"+rqst.OrganizationId+"_evt", []byte{}, strings.Split(rqst.OrganizationId, "@")[1])
	resource_server.publishEvent("update_organization_"+rqst.OrganizationId+"_evt", []byte{}, strings.Split(rqst.AccountId, "@")[1])

	return &resourcepb.AddOrganizationAccountRsp{Result: true}, nil
}

// * Add Group *
func (resource_server *server) AddOrganizationGroup(ctx context.Context, rqst *resourcepb.AddOrganizationGroupRqst) (*resourcepb.AddOrganizationGroupRsp, error) {
	err := resource_server.createCrossReferences(rqst.OrganizationId, "Organizations", "groups", rqst.GroupId, "Groups", "organizations")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	resource_server.publishEvent("update_organization_"+rqst.OrganizationId+"_evt", []byte{}, strings.Split(rqst.OrganizationId, "@")[1])
	resource_server.publishEvent("update_organization_"+rqst.OrganizationId+"_evt", []byte{}, strings.Split(rqst.GroupId, "@")[1])

	return &resourcepb.AddOrganizationGroupRsp{Result: true}, nil
}

// * Add Role *
func (resource_server *server) AddOrganizationRole(ctx context.Context, rqst *resourcepb.AddOrganizationRoleRqst) (*resourcepb.AddOrganizationRoleRsp, error) {
	err := resource_server.createCrossReferences(rqst.OrganizationId, "Organizations", "roles", rqst.RoleId, "Roles", "organizations")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	resource_server.publishEvent("update_organization_"+rqst.OrganizationId+"_evt", []byte{}, strings.Split(rqst.OrganizationId, "@")[1])
	resource_server.publishEvent("update_organization_"+rqst.OrganizationId+"_evt", []byte{}, strings.Split(rqst.RoleId, "@")[1])

	return &resourcepb.AddOrganizationRoleRsp{Result: true}, nil
}

// * Add Application *
func (resource_server *server) AddOrganizationApplication(ctx context.Context, rqst *resourcepb.AddOrganizationApplicationRqst) (*resourcepb.AddOrganizationApplicationRsp, error) {
	err := resource_server.createCrossReferences(rqst.OrganizationId, "Organizations", "applications", rqst.ApplicationId, "Applications", "organizations")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	resource_server.publishEvent("update_organization_"+rqst.OrganizationId+"_evt", []byte{}, strings.Split(rqst.OrganizationId, "@")[1])
	resource_server.publishEvent("update_organization_"+rqst.OrganizationId+"_evt", []byte{}, strings.Split(rqst.ApplicationId, "@")[1])

	return &resourcepb.AddOrganizationApplicationRsp{Result: true}, nil
}

// * Remove Account *
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

	resource_server.publishEvent("update_organization_"+rqst.OrganizationId+"_evt", []byte{}, strings.Split(rqst.OrganizationId, "@")[1])
	resource_server.publishEvent("update_organization_"+rqst.OrganizationId+"_evt", []byte{}, strings.Split(rqst.AccountId, "@")[1])

	return &resourcepb.RemoveOrganizationAccountRsp{Result: true}, nil
}

// * Remove Group *
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

	resource_server.publishEvent("update_organization_"+rqst.OrganizationId+"_evt", []byte{}, strings.Split(rqst.OrganizationId, "@")[1])
	resource_server.publishEvent("update_organization_"+rqst.OrganizationId+"_evt", []byte{}, strings.Split(rqst.GroupId, "@")[1])

	return &resourcepb.RemoveOrganizationGroupRsp{Result: true}, nil
}

// * Remove Role *
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

	resource_server.publishEvent("update_organization_"+rqst.OrganizationId+"_evt", []byte{}, strings.Split(rqst.OrganizationId, "@")[1])
	resource_server.publishEvent("update_organization_"+rqst.OrganizationId+"_evt", []byte{}, strings.Split(rqst.RoleId, "@")[1])

	return &resourcepb.RemoveOrganizationRoleRsp{Result: true}, nil
}

// * Remove Application *
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

	resource_server.publishEvent("update_organization_"+rqst.OrganizationId+"_evt", []byte{}, strings.Split(rqst.OrganizationId, "@")[1])
	resource_server.publishEvent("update_organization_"+rqst.OrganizationId+"_evt", []byte{}, strings.Split(rqst.ApplicationId, "@")[1])

	return &resourcepb.RemoveOrganizationApplicationRsp{Result: true}, nil
}

// * Delete organization
func (resource_server *server) DeleteOrganization(ctx context.Context, rqst *resourcepb.DeleteOrganizationRqst) (*resourcepb.DeleteOrganizationRsp, error) {

	localDomain, err := config.GetDomain()
	organizationId := rqst.Organization
	if strings.Contains(organizationId, "@") {
		domain := strings.Split(organizationId, "@")[1]
		organizationId = strings.Split(organizationId, "@")[0]

		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		if localDomain != domain {

			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("i cant's delete object from domain "+domain+" from domain "+localDomain)))
		}
	}

	// That service made user of persistence service.
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Organizations", `{"_id":"`+organizationId+`"}`, ``)
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
				err := resource_server.deleteReference(p, rqst.Organization, groupId, "organizations", "Groups")
				if err != nil {
					fmt.Println(err)
				}
				resource_server.publishEvent("update_group_"+groupId+"_evt", []byte{}, strings.Split(groupId, "@")[1])
			}
		}
	}

	if organization["roles"].(primitive.A) != nil {
		roles := []interface{}(organization["roles"].(primitive.A))
		if roles != nil {
			for i := 0; i < len(roles); i++ {
				roleId := roles[i].(map[string]interface{})["$id"].(string)
				err := resource_server.deleteReference(p, rqst.Organization, roleId, "organizations", "Roles")
				if err != nil {
					fmt.Println(err)
				}
				resource_server.publishEvent("update_role_"+roleId+"_evt", []byte{}, strings.Split(roleId, "@")[1])
			}
		}
	}

	if organization["applications"].(primitive.A) != nil {
		applications := []interface{}(organization["applications"].(primitive.A))
		if applications != nil {
			for i := 0; i < len(applications); i++ {
				applicationId := applications[i].(map[string]interface{})["$id"].(string)
				err := resource_server.deleteReference(p, rqst.Organization, applicationId, "organizations", "Applications")
				if err != nil {
					fmt.Println(err)
				}
				resource_server.publishEvent("update_application_"+applicationId+"_evt", []byte{}, strings.Split(applicationId, "@")[1])
			}
		}
	}

	if organization["accounts"].(primitive.A) != nil {
		accounts := []interface{}(organization["accounts"].(primitive.A))
		if accounts != nil {
			for i := 0; i < len(accounts); i++ {
				accountId := accounts[i].(map[string]interface{})["$id"].(string)
				err := resource_server.deleteReference(p, rqst.Organization, accountId, "organizations", "Accounts")
				if err != nil {
					fmt.Println(err)
				}
				resource_server.publishEvent("update_account_"+accountId+"_evt", []byte{}, strings.Split(accountId, "@")[1])
			}
		}
	}

	// Delete organization
	organizationId = organization["_id"].(string) + "@" + organization["domain"].(string)
	resource_server.deleteAllAccess(organizationId, rbacpb.SubjectType_ORGANIZATION)

	// Try to delete the account...
	err = p.DeleteOne(context.Background(), "local_resource", "local_resource", "Organizations", `{"_id":"`+organization["_id"].(string)+`"}`, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	resource_server.deleteResourcePermissions(organizationId)
	resource_server.publishEvent("delete_organization_"+organizationId+"_evt", []byte{}, localDomain)
	resource_server.publishEvent("delete_organization_evt", []byte(organizationId), localDomain)

	return &resourcepb.DeleteOrganizationRsp{Result: true}, nil
}

/**
 * Create a group with a given name of update existing one.
 */
func (resource_server *server) UpdateGroup(ctx context.Context, rqst *resourcepb.UpdateGroupRqst) (*resourcepb.UpdateGroupRsp, error) {
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Get the persistence connection
	count, err := p.Count(context.Background(), "local_resource", "local_resource", "Groups", `{"_id":"`+rqst.GroupId+`"}`, "")
	if err != nil || count == 0 {
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	} else {

		err = p.UpdateOne(context.Background(), "local_resource", "local_resource", "Groups", `{"_id":"`+rqst.GroupId+`"}`, rqst.Values, "")
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	localDomain, err := config.GetDomain()
	resource_server.publishEvent("update_group_"+rqst.GroupId+"_evt", []byte{}, localDomain)

	return &resourcepb.UpdateGroupRsp{
		Result: true,
	}, nil
}

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

// * Register a new group
func (resource_server *server) CreateGroup(ctx context.Context, rqst *resourcepb.CreateGroupRqst) (*resourcepb.CreateGroupRsp, error) {

	var clientId string
	var domain string

	// Now I will index the conversation to be retreivable for it creator...
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		token := strings.Join(md["token"], "")
		if len(token) > 0 {
			claims, err := security.ValidateToken(token)
			if err != nil {
				return nil, status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}
			clientId = claims.Id + "@" + claims.UserDomain
			domain = claims.Domain
		} else {
			return nil, errors.New("CreateGroup no token was given")
		}
	}

	// Get the persistence connection
	err := resource_server.createGroup(rqst.Group.Id, rqst.Group.Name, clientId+"@"+domain, rqst.Group.Description, rqst.Group.Members)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	var marshaler jsonpb.Marshaler
	jsonStr, err := marshaler.MarshalToString(rqst.Group)
	if err == nil {
		resource_server.publishEvent("create_group_evt", []byte(jsonStr), domain)
	}

	return &resourcepb.CreateGroupRsp{
		Result: true,
	}, nil
}

// * Return the list of organizations
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

	groups, err := p.Find(context.Background(), "local_resource", "local_resource", "Groups", query, rqst.Options)
	if err != nil {
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// No I will stream the result over the networks.
	maxSize := 50
	values := make([]*resourcepb.Group, 0)

	for i := 0; i < len(groups); i++ {

		g := &resourcepb.Group{Name: groups[i].(map[string]interface{})["name"].(string), Id: groups[i].(map[string]interface{})["_id"].(string), Description: groups[i].(map[string]interface{})["description"].(string), Members: make([]string, 0)}
		if groups[i].(map[string]interface{})["domain"] != nil {
			g.Domain = groups[i].(map[string]interface{})["domain"].(string)
		} else {
			g.Domain = resource_server.Domain
		}

		if groups[i].(map[string]interface{})["members"] != nil {
			members := []interface{}(groups[i].(map[string]interface{})["members"].(primitive.A))
			g.Members = make([]string, 0)
			for j := 0; j < len(members); j++ {
				g.Members = append(g.Members, members[j].(map[string]interface{})["$id"].(string))
			}
		} else if groups[i].(map[string]interface{})["organizations"] != nil {
			organizations := []interface{}(groups[i].(map[string]interface{})["organizations"].(primitive.A))
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
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
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
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return nil
}

// * Delete organization
func (resource_server *server) DeleteGroup(ctx context.Context, rqst *resourcepb.DeleteGroupRqst) (*resourcepb.DeleteGroupRsp, error) {

	groupId := rqst.Group
	localDomain, err := config.GetDomain()

	if strings.Contains(groupId, "@") {
		domain := strings.Split(groupId, "@")[1]
		groupId = strings.Split(groupId, "@")[0]

		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		if localDomain != domain {

			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("i cant's delete object from domain "+domain+" from domain "+localDomain)))
		}
	}

	// Get the persistence connection
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		fmt.Println("fail to get persistence connection ", err)
		return nil, err
	}

	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Groups", `{"_id":"`+groupId+`"}`, ``)
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
			accountId := members[j].(map[string]interface{})["$id"].(string)
			resource_server.deleteReference(p, rqst.Group, accountId, "groups", "Accounts")
			resource_server.publishEvent("update_account_"+accountId+"_evt", []byte{}, strings.Split(accountId, "@")[1])
		}
	}

	// I will remove it from organizations...
	if group["organizations"] != nil {
		organizations := []interface{}(group["organizations"].(primitive.A))
		if organizations != nil {
			for i := 0; i < len(organizations); i++ {
				organizationId := organizations[i].(map[string]interface{})["$id"].(string)
				resource_server.deleteReference(p, rqst.Group, organizationId, "groups", "Organizations")
				resource_server.publishEvent("update_organization_"+organizationId+"_evt", []byte{}, strings.Split(organizationId, "@")[1])
			}
		}
	}

	groupId = group["_id"].(string) + "@" + group["domain"].(string)
	resource_server.deleteAllAccess(groupId, rbacpb.SubjectType_GROUP)

	err = p.DeleteOne(context.Background(), "local_resource", "local_resource", "Groups", `{"_id":"`+group["_id"].(string)+`"}`, "")
	if err != nil {
		fmt.Println("3043", err)
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	resource_server.deleteResourcePermissions(rqst.Group)

	resource_server.publishEvent("delete_group_"+groupId+"_evt", []byte{}, localDomain)

	resource_server.publishEvent("delete_group_evt", []byte(groupId), localDomain)

	return &resourcepb.DeleteGroupRsp{
		Result: true,
	}, nil

}

// * Add a member account to the group *
func (resource_server *server) AddGroupMemberAccount(ctx context.Context, rqst *resourcepb.AddGroupMemberAccountRqst) (*resourcepb.AddGroupMemberAccountRsp, error) {

	err := resource_server.createCrossReferences(rqst.GroupId, "Groups", "members", rqst.AccountId, "Accounts", "groups")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	resource_server.publishEvent("update_group_"+rqst.GroupId+"_evt", []byte{}, strings.Split(rqst.GroupId, "@")[1])
	resource_server.publishEvent("update_account_"+rqst.AccountId+"_evt", []byte{}, strings.Split(rqst.AccountId, "@")[1])

	return &resourcepb.AddGroupMemberAccountRsp{Result: true}, nil
}

// * Remove member account from the group *
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

	resource_server.publishEvent("update_group_"+rqst.GroupId+"_evt", []byte{}, strings.Split(rqst.GroupId, "@")[1])
	resource_server.publishEvent("update_account_"+rqst.AccountId+"_evt", []byte{}, strings.Split(rqst.AccountId, "@")[1])

	return &resourcepb.RemoveGroupMemberAccountRsp{Result: true}, nil
}

// //////////////////////////////////////////////////////////////////////////////////
// Notification implementation
// //////////////////////////////////////////////////////////////////////////////////
// * Create a notification
func (resource_server *server) CreateNotification(ctx context.Context, rqst *resourcepb.CreateNotificationRqst) (*resourcepb.CreateNotificationRsp, error) {

	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	// so the recipient here is the id of the user...
	recipient := strings.Split(rqst.Notification.Recipient, "@")[0]

	count, _ := p.Count(context.Background(), "local_resource", recipient+"_db", "Notifications", `{"_id":"`+rqst.Notification.Id+`"}`, "")
	if count > 0 {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("Notification with id " + rqst.Notification.Id + " already exist")))
	}


	// if the account is not on the domain will redirect the request...
	if rqst.Notification.NotificationType == resourcepb.NotificationType_USER_NOTIFICATION {
		recipient := rqst.Notification.Recipient
		if strings.Contains(recipient, "@") {
			domain := strings.Split(recipient, "@")[1]
			localDomain, _ := config.GetDomain()
			if localDomain != domain {
				client, err := resource_client.NewResourceService_Client(domain, "resource.ResourceService")
				if err != nil {
					return nil, status.Errorf(
						codes.Internal,
						Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
				}

				err = client.CreateNotification(rqst.Notification)
				if err != nil {
					return nil, status.Errorf(
						codes.Internal,
						Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
				}

				return &resourcepb.CreateNotificationRsp{}, nil
			}
		}
	}



	_, err = p.InsertOne(context.Background(), "local_resource", recipient+"_db", "Notifications", rqst.Notification, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	var marshaler jsonpb.Marshaler
	jsonStr, err := marshaler.MarshalToString(rqst.Notification)
	localDomain, _ := config.GetDomain()
	if err == nil {
		resource_server.publishEvent("create_notification_evt", []byte(jsonStr), localDomain)
	}

	return &resourcepb.CreateNotificationRsp{}, nil
}

// * Retreive notifications
func (resource_server *server) GetNotifications(rqst *resourcepb.GetNotificationsRqst, stream resourcepb.ResourceService_GetNotificationsServer) error {

	recipient := strings.Split(rqst.Recipient, "@")[0]

	// Get the persistence connection
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return err
	}

	query := `{}`

	notifications, err := p.Find(context.Background(), "local_resource", recipient+"_db", "Notifications", query, "")
	if err != nil {
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// No I will stream the result over the networks.
	maxSize := 50
	values := make([]*resourcepb.Notification, 0)
	for i := 0; i < len(notifications); i++ {
		n_ := notifications[i].(map[string]interface{})
		notificationType := resourcepb.NotificationType(int32(Utility.ToInt(n_["notificationtype"])))
		values = append(values, &resourcepb.Notification{Id: n_["id"].(string), Sender: n_["sender"].(string), Date: n_["date"].(int64), Recipient: n_["recipient"].(string), Message: n_["message"].(string), NotificationType: notificationType})
		if len(values) >= maxSize {
			err := stream.Send(
				&resourcepb.GetNotificationsRsp{
					Notifications: values,
				},
			)
			if err != nil {
				return status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}
			values = make([]*resourcepb.Notification, 0)
		}
	}

	// Send reminding values.
	err = stream.Send(
		&resourcepb.GetNotificationsRsp{
			Notifications: values,
		},
	)

	if err != nil {
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return nil
}

// * Remove a notification
func (resource_server *server) DeleteNotification(ctx context.Context, rqst *resourcepb.DeleteNotificationRqst) (*resourcepb.DeleteNotificationRsp, error) {

	recipient := strings.Split(rqst.Recipient, "@")[0]

	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = p.DeleteOne(context.Background(), "local_resource", recipient+"_db", "Notifications", `{"id":"`+rqst.Id+`"}`, ``)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	localDomain, _ := config.GetDomain()
	resource_server.publishEvent("delete_notification_"+rqst.Id+"_evt", []byte{}, localDomain)
	resource_server.publishEvent("delete_notification_evt", []byte(rqst.Id), localDomain)

	return &resourcepb.DeleteNotificationRsp{}, nil
}

// * Remove all Notification
func (resource_server *server) ClearAllNotifications(ctx context.Context, rqst *resourcepb.ClearAllNotificationsRqst) (*resourcepb.ClearAllNotificationsRsp, error) {

	recipient := strings.Split(rqst.Recipient, "@")[0]

	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = p.Delete(context.Background(), "local_resource", recipient+"_db", "Notifications", `{}`, ``)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	localDomain, _ := config.GetDomain()
	resource_server.publishEvent("clear_notification_evt", []byte{}, localDomain)

	return &resourcepb.ClearAllNotificationsRsp{}, nil
}

// * Remove all notification of a given type
func (resource_server *server) ClearNotificationsByType(ctx context.Context, rqst *resourcepb.ClearNotificationsByTypeRqst) (*resourcepb.ClearNotificationsByTypeRsp, error) {
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = p.Delete(context.Background(), "local_resource", rqst.Recipient+"_db", "Notifications", `{ "notificationtype":`+Utility.ToString(rqst.NotificationType)+`}`, ``)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &resourcepb.ClearNotificationsByTypeRsp{}, nil
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

// * Retrun all version of a given packages. *
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
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("No package descriptor with id "+rqst.ServiceId+" was found for publisher id "+rqst.PublisherId)))
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
	if len(descriptors) > 1 {
		sort.Slice(descriptors[:], func(i, j int) bool {
			return descriptors[i].Version > descriptors[j].Version
		})
	}

	// Return the list of Service Descriptor.
	return &resourcepb.GetPackageDescriptorResponse{
		Results: descriptors,
	}, nil
}

// * Return the list of all packages *
func (server *server) GetPackagesDescriptor(rqst *resourcepb.GetPackagesDescriptorRequest, stream resourcepb.ResourceService_GetPackagesDescriptorServer) error {
	p, err := server.getPersistenceStore()
	if err != nil {
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	query := rqst.Query
	if len(query) == 0 {
		query = "{}"
	}

	data, err := p.Find(context.Background(), "local_resource", "local_resource", "Packages", query, rqst.Options)
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
		if data[i].(map[string]interface{})["icon"] != nil {
			descriptor.Icon = data[i].(map[string]interface{})["icon"].(string)
		}

		if data[i].(map[string]interface{})["alias"] != nil {
			descriptor.Alias = data[i].(map[string]interface{})["alias"].(string)
		}

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

	jsonStr, err := marshaler.MarshalToString(rqst.PackageDescriptor)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// little fix...
	jsonStr = strings.ReplaceAll(jsonStr, "publisherId", "publisherid")

	// Always create a new if not already exist.
	err = p.ReplaceOne(context.Background(), "local_resource", "local_resource", "Packages", `{"id":"`+rqst.PackageDescriptor.Id+`", "publisherid":"`+rqst.PackageDescriptor.PublisherId+`", "version":"`+rqst.PackageDescriptor.Version+`"}`, jsonStr, `[{"upsert": true}]`)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &resourcepb.SetPackageDescriptorResponse{
		Result: true,
	}, nil
}

// * Get the package bundle checksum use for validation *
func (server *server) GetPackageBundleChecksum(ctx context.Context, rqst *resourcepb.GetPackageBundleChecksumRequest) (*resourcepb.GetPackageBundleChecksumResponse, error) {
	p, err := server.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Bundles", `{"_id":"`+rqst.Id+`"}`, "")
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

// * Set the package bundle (without data)
func (server *server) SetPackageBundle(ctx context.Context, rqst *resourcepb.SetPackageBundleRequest) (*resourcepb.SetPackageBundleResponse, error) {
	bundle := rqst.Bundle

	p, err := server.getPersistenceStore()
	if err != nil {
		server.logServiceError("SetPackageBundle", Utility.FileLine(), Utility.FunctionName(), err.Error())
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Generate the bundle id....
	id := Utility.GenerateUUID(bundle.PackageDescriptor.PublisherId + "%" + bundle.PackageDescriptor.Name + "%" + bundle.PackageDescriptor.Version + "%" + bundle.PackageDescriptor.Id + "%" + bundle.Plaform)

	jsonStr, err := Utility.ToJson(map[string]interface{}{"_id": id, "checksum": bundle.Checksum, "platform": bundle.Plaform, "publisherid": bundle.PackageDescriptor.PublisherId, "servicename": bundle.PackageDescriptor.Name, "serviceid": bundle.PackageDescriptor.Id, "modified": bundle.Modified, "size": bundle.Size})
	if err != nil {
		server.logServiceError("SetPackageBundle", Utility.FileLine(), Utility.FunctionName(), err.Error())
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = p.ReplaceOne(context.Background(), "local_resource", "local_resource", "Bundles", `{"_id":"`+id+`"}`, jsonStr, `[{"upsert": true}]`)
	if err != nil {
		server.logServiceError("SetPackageBundle", Utility.FileLine(), Utility.FunctionName(), err.Error())
		return nil, err
	}
	return &resourcepb.SetPackageBundleResponse{Result: true}, nil
}

/////////////////////////////////////////////////////////////////////////////////////////
// Session
/////////////////////////////////////////////////////////////////////////////////////////

func (server *server) updateSession(accountId string, state resourcepb.SessionState, last_session_time, expire_at int64) error {
	expiration := time.Unix(expire_at, 0)
	delay := time.Until(expiration)
	if state != resourcepb.SessionState_OFFLINE {
		if expiration.Before(time.Now()) {
			return errors.New("session is already expired " + expiration.Local().String() + " " + Utility.ToString(math.Floor(delay.Minutes())) + ` minutes ago`)
		}
	}

	p, err := server.getPersistenceStore()
	if err != nil {
		return err
	}

	// Log a message to display update session...
	//server.logServiceInfo("updateSession", Utility.FileLine(), Utility.FunctionName(), "update session for user "+accountId+" last_session_time: "+time.Unix(last_session_time, 0).Local().String()+" expire_at: "+time.Unix(expire_at, 0).Local().String())
	session := map[string]interface{}{"accountId": accountId, "expire_at": expire_at, "last_state_time": last_session_time, "state": state}
	jsonStr, err := Utility.ToJson(session)
	if err != nil {
		return err
	}

	// send update_session event
	//server.publishEvent("session_state_" + accountId+ "_change_event",  []byte(jsonStr))

	return p.ReplaceOne(context.Background(), "local_resource", "local_resource", "Sessions", `{"_id":"`+accountId+`"}`, jsonStr, `[{"upsert":true}]`)

}

// * Update user session informations
func (server *server) UpdateSession(ctx context.Context, rqst *resourcepb.UpdateSessionRequest) (*resourcepb.UpdateSessionResponse, error) {

	err := server.updateSession(rqst.Session.AccountId, rqst.Session.State, rqst.Session.LastStateTime, rqst.Session.ExpireAt)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &resourcepb.UpdateSessionResponse{}, nil
}

// * Remove session
func (server *server) RemoveSession(ctx context.Context, rqst *resourcepb.RemoveSessionRequest) (*resourcepb.RemoveSessionResponse, error) {
	p, err := server.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Now I will remove the token...
	err = p.DeleteOne(context.Background(), "local_resource", "local_resource", "Sessions", `{"_id":"`+rqst.AccountId+`"}`, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &resourcepb.RemoveSessionResponse{}, nil
}

func (server *server) GetSessions(ctx context.Context, rqst *resourcepb.GetSessionsRequest) (*resourcepb.GetSessionsResponse, error) {
	p, err := server.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	sessions, err := p.Find(context.Background(), "local_resource", "local_resource", "Sessions", rqst.Query, rqst.Options)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	sessions_ := make([]*resourcepb.Session, 0)
	for i := 0; i < len(sessions); i++ {
		session := sessions[i].(map[string]interface{})
		expireAt := Utility.ToInt(session["expire_at"])
		lastStateTime := Utility.ToInt(session["last_state_time"])
		sessions_ = append(sessions_, &resourcepb.Session{AccountId: session["_id"].(string), ExpireAt: int64(expireAt), LastStateTime: int64(lastStateTime), State: resourcepb.SessionState(session["state"].(int32))})
	}

	return &resourcepb.GetSessionsResponse{
		Sessions: sessions_,
	}, nil
}

func (server *server) getSession(accountId string) (*resourcepb.Session, error) {
	p, err := server.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	// Now I will remove the token...
	session_, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Sessions", `{"_id":"`+accountId+`"}`, "")
	if err != nil {
		return nil, err
	}

	session := session_.(map[string]interface{})

	expireAt := Utility.ToInt(session["expire_at"])
	lastStateTime := Utility.ToInt(session["last_state_time"])

	if expireAt == 0 || lastStateTime == 0 {
		return nil, errors.New("invalid session with id " + accountId + " expire_at has value " + time.Unix(int64(expireAt), 0).Local().String() + " last_state_time " + time.Unix(int64(lastStateTime), 0).Local().String())
	}

	var state resourcepb.SessionState

	if session["state"] != nil {
		state = resourcepb.SessionState(session["state"].(int32))
	}

	return &resourcepb.Session{AccountId: session["_id"].(string), ExpireAt: int64(expireAt), LastStateTime: int64(lastStateTime), State: state}, nil
}

// * Return a session for a given user
func (server *server) GetSession(ctx context.Context, rqst *resourcepb.GetSessionRequest) (*resourcepb.GetSessionResponse, error) {

	// Now I will remove the token...
	session, err := server.getSession(rqst.AccountId)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &resourcepb.GetSessionResponse{
		Session: session,
	}, nil
}

/////////////////////////////////////////////////////////////////////////////////////////////////////
// Call's
/////////////////////////////////////////////////////////////////////////////////////////////////////

// * Return the list of calls for a given account *
func (resource_server *server) GetCallHistory(ctx context.Context, rqst *resourcepb.GetCallHistoryRqst) (*resourcepb.GetCallHistoryRsp, error) {
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Keep the id portion only...
	accountId := rqst.AccountId
	if strings.Contains(accountId, "@") {
		domain := strings.Split(accountId, "@")[1]
		localDomain, _ := config.GetDomain()
		if domain != localDomain {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no account found with id "+accountId)))

		}
		accountId = strings.Split(accountId, "@")[0]
	}

	// set the caller id.
	db := accountId
	db = strings.ReplaceAll(strings.ReplaceAll(db, ".", "_"), "@", "_")
	db += "_db"

	query := `{"$or":[{"caller":"` + rqst.AccountId + `"},{"callee":"` + rqst.AccountId + `"} ]}`
	results, err := p.Find(context.Background(), "local_resource", db, "calls", query, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	calls := make([]*resourcepb.Call, len(results))
	for i := 0; i < len(results); i++ {
		call := results[i].(map[string]interface{})
		calls[i] = &resourcepb.Call{Caller: call["caller"].(string), Callee: call["callee"].(string), Uuid: call["_id"].(string), StartTime: int64(call["start_time"].(int32)), EndTime: int64(call["end_time"].(int32))}
	}

	return &resourcepb.GetCallHistoryRsp{Calls: calls}, nil
}

func (resource_server *server) setCall(accountId string, call *resourcepb.Call) error {

	// Get the persistence connection
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return err
	}

	// set the caller id.
	db := accountId
	db = strings.ReplaceAll(strings.ReplaceAll(db, ".", "_"), "@", "_")
	db += "_db"

	// rename the uuid to _id (for mongo identifier)
	call_ := map[string]interface{}{"caller": call.Caller, "callee": call.Callee, "_id": call.Uuid, "start_time": call.StartTime, "end_time": call.EndTime}
	jsonStr, _ := Utility.ToJson(call_)

	err = p.ReplaceOne(context.Background(), "local_resource", db, "calls", `{"_id":"`+call.Uuid+`"}`, jsonStr, `[{"upsert":true}]`)
	if err != nil {
		return err
	}

	return nil
}

// * Set calling information *
func (resource_server *server) SetCall(ctx context.Context, rqst *resourcepb.SetCallRqst) (*resourcepb.SetCallRsp, error) {

	// Get the persistence connection
	if strings.Contains(rqst.Call.Caller, "@") {
		domain := strings.Split(rqst.Call.Caller, "@")[1]
		localDomain, _ := config.GetDomain()
		if domain == localDomain {
			err := resource_server.setCall(strings.Split(rqst.Call.Caller, "@")[0], rqst.Call)
			if err != nil {
				return nil, status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}
		}
	} else {
		err := resource_server.setCall(rqst.Call.Caller, rqst.Call)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	if strings.Contains(rqst.Call.Callee, "@") {
		domain := strings.Split(rqst.Call.Callee, "@")[1]
		localDomain, _ := config.GetDomain()
		if domain == localDomain {
			err := resource_server.setCall(strings.Split(rqst.Call.Callee, "@")[0], rqst.Call)
			if err != nil {
				return nil, status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}
		}
	} else {
		err := resource_server.setCall(rqst.Call.Callee, rqst.Call)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	return &resourcepb.SetCallRsp{}, nil
}

func(resource_server *server) deleteCall(account_id, uuid string) error{
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return err
	}

	// Keep the id portion only...
	accountId := account_id
	if strings.Contains(accountId, "@") {
		domain := strings.Split(accountId, "@")[1]
		localDomain, _ := config.GetDomain()
		if domain != localDomain {
			return err
		}
		accountId = strings.Split(accountId, "@")[0]
	}

	// set the caller id.
	db := accountId
	db = strings.ReplaceAll(strings.ReplaceAll(db, ".", "_"), "@", "_")
	db += "_db"
	
	err = p.DeleteOne(context.Background(), "local_resource", db, "calls", `{"_id":"`+uuid+`"}`, "")
	if err != nil {
		return err
	}

	return nil
}

// * Delete a calling infos *
func (resource_server *server) DeleteCall(ctx context.Context, rqst *resourcepb.DeleteCallRqst) (*resourcepb.DeleteCallRsp, error) {

	err := resource_server.deleteCall(rqst.AccountId, rqst.Uuid)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &resourcepb.DeleteCallRsp{}, nil
}

// * Clear Call's *
func (resource_server *server) ClearCalls(ctx context.Context, rqst *resourcepb.ClearCallsRqst) (*resourcepb.ClearCallsRsp, error) {

	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Keep the id portion only...
	accountId := rqst.AccountId
	if strings.Contains(accountId, "@") {
		domain := strings.Split(accountId, "@")[1]
		localDomain, _ := config.GetDomain()
		if domain != localDomain {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no account found with id "+accountId)))

		}
		accountId = strings.Split(accountId, "@")[0]
	}

	// set the caller id.
	db := accountId
	db = strings.ReplaceAll(strings.ReplaceAll(db, ".", "_"), "@", "_")
	db += "_db"

	query := rqst.Filter
	results, err := p.Find(context.Background(), "local_resource", db, "calls", query, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Delete the call.
	for i := 0; i < len(results); i++ {
		call := results[i].(map[string]interface{})
		resource_server.deleteCall(rqst.AccountId, call["_id"].(string))
	}

	return &resourcepb.ClearCallsRsp{}, nil
}
