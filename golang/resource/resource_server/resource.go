package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/globular_client"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/globulario/services/golang/persistence/persistence_client"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/resource/resource_client"
	"github.com/globulario/services/golang/resource/resourcepb"
	"github.com/globulario/services/golang/security"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// Set the root password
func (srv *server) SetEmail(ctx context.Context, rqst *resourcepb.SetEmailRequest) (*resourcepb.SetEmailResponse, error) {

	// Here I will set the root password.
	// First of all I will get the user information from the database.
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	accountId := rqst.AccountId

	q := `{"_id":"` + accountId + `"}`

	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Accounts", q, ``)
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

	//account["roles"] = []interface{}(account["roles"].(primitive.A))
	var roles []interface{}
	switch account["roles"].(type) {
	case primitive.A:
		roles = []interface{}(account["roles"].(primitive.A))
	case []interface{}:
		roles = []interface{}(account["roles"].([]interface{}))
	default:
		fmt.Println("unknown type ", account["roles"])
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
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	srv.publishEvent("update_account_"+rqst.AccountId+"_evt", []byte{}, srv.Address)

	// Return the token.
	return &resourcepb.SetEmailResponse{}, nil
}

/* Register a new Account */
func (srv *server) RegisterAccount(ctx context.Context, rqst *resourcepb.RegisterAccountRqst) (*resourcepb.RegisterAccountRsp, error) {
	rqst.Account.TypeName = "Account"
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

	err := srv.registerAccount(rqst.Account.Domain, rqst.Account.Id, rqst.Account.Name, rqst.Account.Email, rqst.Account.Password, rqst.Account.Organizations, rqst.Account.Roles, rqst.Account.Groups)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Generate a token to identify the user.
	tokenString, _ := security.GenerateToken(srv.SessionTimeout, srv.Mac, rqst.Account.Id, rqst.Account.Name, rqst.Account.Email, rqst.Account.Domain)
	claims, err := security.ValidateToken(tokenString)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = srv.updateSession(claims.Id+"@"+claims.UserDomain, 0, time.Now().Unix(), claims.StandardClaims.ExpiresAt)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	jsonStr, err := protojson.Marshal(rqst.Account)
	if err == nil {
		srv.publishEvent("create_account_evt", jsonStr, srv.Address)
	}
	// Now I will
	return &resourcepb.RegisterAccountRsp{
		Result: tokenString, // Return the token string.
	}, nil
}

// * Return a given account
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
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		if _domain != domain {
			a, err := srv.getRemoteAccount(accountId, domain)
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

	q := `{"_id":"` + accountId + `"}`
	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Accounts", q, ``)
	if err != nil {
		fmt.Println("fail to retreive account:", accountId, " from database with error:", err.Error())
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
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
			fmt.Println("unknown type ", account["groups"])
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
			fmt.Println("unknown type ", account["roles"])
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
			fmt.Println("unknown type ", account["organizations"])
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
	db := accountId
	db = strings.ReplaceAll(strings.ReplaceAll(db, ".", "_"), "@", "_")
	db = strings.ReplaceAll(db, "-", "_")
	db = strings.ReplaceAll(db, ".", "_")
	db = strings.ReplaceAll(db, " ", "_")

	db += "_db"

	q = `{"$and":[{"_id":"` + accountId + `", "domain":"` + a.Domain + `"}]}`

	user_data, err := p.FindOne(context.Background(), "local_resource", db, "user_data", q, ``)
	if err == nil {
		// set the user infos....
		if user_data != nil {

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
		}
	}

	return &resourcepb.GetAccountRsp{
		Account: a, // Return the token string.
	}, nil

}

// * Update account password.
// TODO make sure only user can
func (srv *server) SetAccountPassword(ctx context.Context, rqst *resourcepb.SetAccountPasswordRqst) (*resourcepb.SetAccountPasswordRsp, error) {

	clientId, _, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}

	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	q := `{"_id":"` + rqst.AccountId + `"}`

	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Accounts", q, ``)
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
		err = srv.validatePassword(rqst.OldPassword, account["password"].(string))
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

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
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("unknown database type "+p.GetStoreType())))
	}

	// Change the password...
	err = p.RunAdminCmd(context.Background(), "local_resource", srv.Backend_user, srv.Backend_password, changePasswordScript)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Create bcrypt...
	pwd, err := srv.hashPassword(rqst.NewPassword)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
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
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	setPassword := map[string]interface{}{"$set": map[string]interface{}{"password": string(pwd)}}
	setPassword_, _ := Utility.ToJson(setPassword)

	// Hash the password...
	err = p.UpdateOne(context.Background(), "local_resource", "local_resource", "Accounts", q, setPassword_, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &resourcepb.SetAccountPasswordRsp{}, nil
}

// * Save an account
func (srv *server) SetAccount(ctx context.Context, rqst *resourcepb.SetAccountRqst) (*resourcepb.SetAccountRsp, error) {
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Set the query.
	q := `{"_id":"` + rqst.Account.Id + `"}`

	// Set the field and the values to update.
	setAccount := map[string]interface{}{"$set": map[string]interface{}{"name": rqst.Account.Name, "email": rqst.Account.Email, "domain": srv.Domain}}
	setAccount_, _ := Utility.ToJson(setAccount)

	err = p.UpdateOne(context.Background(), "local_resource", "local_resource", "Accounts", q, setAccount_, "")
	if err != nil {

		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Set values from the accound db itself.
	db := rqst.Account.Id
	db = strings.ReplaceAll(strings.ReplaceAll(db, ".", "_"), "@", "_")
	db = strings.ReplaceAll(db, "-", "_")
	db = strings.ReplaceAll(db, ".", "_")
	db = strings.ReplaceAll(db, " ", "_")

	db += "_db"

	q = `{"_id":"` + rqst.Account.Id + `"}`

	user_data, err := p.FindOne(context.Background(), "local_resource", db, "user_data", q, ``)
	if err == nil {
		// set the user infos....
		if user_data != nil {
			user_data_ := user_data.(map[string]interface{})
			if user_data_["profile_picture"] != nil {
				rqst.Account.ProfilePicture = user_data_["profile_picture"].(string)
			}
			if user_data_["first_name"] != nil {
				rqst.Account.FirstName = user_data_["first_name"].(string)
			}
			if user_data_["last_name"] != nil {
				rqst.Account.LastName = user_data_["last_name"].(string)
			}
			if user_data_["middle_name"] != nil {
				rqst.Account.Middle = user_data_["middle_name"].(string)
			}

		}
	}

	return &resourcepb.SetAccountRsp{}, nil

}

// * Return the list accounts *
func (srv *server) GetAccounts(rqst *resourcepb.GetAccountsRqst, stream resourcepb.ResourceService_GetAccountsServer) error {

	// Get the persistence connection
	p, err := srv.getPersistenceStore()
	if err != nil {
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	query := rqst.Query
	if query == "" {
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
		a := &resourcepb.Account{Id: account["_id"].(string), Name: account["name"].(string), Email: account["email"].(string), Domain: account["domain"].(string)}

		if account["domain"] == nil {
			a.Domain = srv.Domain
		}

		if account["groups"] != nil {
			var groups []interface{}
			switch account["groups"].(type) {
			case primitive.A:
				groups = []interface{}(account["groups"].(primitive.A))
			case []interface{}:
				groups = []interface{}(account["groups"].([]interface{}))
			default:
				fmt.Println("unknown type ", account["groups"])
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
				fmt.Println("unknown type ", account["roles"])
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
				fmt.Println("unknown type ", account["organizations"])
			}

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
		db = strings.ReplaceAll(db, "-", "_")
		db = strings.ReplaceAll(db, ".", "_")
		db = strings.ReplaceAll(db, " ", "_")

		db += "_db"

		q := `{"_id":"` + a.Id + `"}`

		user_data, err := p.FindOne(context.Background(), "local_resource", db, "user_data", q, ``)
		if err == nil {
			// set the user infos....
			if user_data != nil {
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

// * Add contact to a given account *
func (srv *server) SetAccountContact(ctx context.Context, rqst *resourcepb.SetAccountContactRqst) (*resourcepb.SetAccountContactRsp, error) {

	if rqst.Contact == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no contact was given")))
	}

	// Get the persistence connection
	p, err := srv.getPersistenceStore()
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
	db = strings.ReplaceAll(db, "-", "_")
	db = strings.ReplaceAll(db, ".", "_")
	db = strings.ReplaceAll(db, " ", "_")

	db += "_db"

	q := `{"_id":"` + rqst.Contact.Id + `"}`

	sentInvitation := `{"_id":"` + rqst.Contact.Id + `", "invitationTime":` + Utility.ToString(rqst.Contact.InvitationTime) + `, "status":"` + rqst.Contact.Status + `", "ringtone":"` + rqst.Contact.Ringtone + `", "profilePicture":"` + rqst.Contact.ProfilePicture + `"}`

	err = p.ReplaceOne(context.Background(), "local_resource", db, "Contacts", q, sentInvitation, `[{"upsert":true}]`)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	srv.publishEvent("update_account_"+rqst.Contact.Id+"_evt", []byte{}, srv.Address)
	srv.publishEvent("update_account_"+rqst.AccountId+"_evt", []byte{}, srv.Address)

	return &resourcepb.SetAccountContactRsp{Result: true}, nil
}

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
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		// find account on other domain.
		if localDomain != domain {

			_, err := srv.getRemoteAccount(accountId, domain)
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

	q := `{"_id":"` + accountId + `"}`
	count, _ := p.Count(context.Background(), "local_resource", "local_resource", "Accounts", q, "")

	if count > 0 {
		return &resourcepb.AccountExistRsp{
			Result: true,
		}, nil
	}

	return nil, status.Errorf(
		codes.Internal,
		Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("account '"+rqst.Id+"' dosent exist!")))

}

// Test if account is a member of organization.
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
			fmt.Println("unknown type ", account_["organizations"])
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

// * Test if an account is part of an organization *
func (srv *server) IsOrgnanizationMember(ctx context.Context, rqst *resourcepb.IsOrgnanizationMemberRqst) (*resourcepb.IsOrgnanizationMemberRsp, error) {
	result := srv.isOrganizationMemeber(rqst.AccountId, rqst.OrganizationId)

	return &resourcepb.IsOrgnanizationMemberRsp{
		Result: result,
	}, nil
}

// * Delete an account *
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
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("i cant's delete object from domain "+domain+" from domain "+localDomain)))
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
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
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
			fmt.Println("unknown type ", account["organizations"])
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
			fmt.Println("unknown type ", account["groups"])
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
			fmt.Println("unknown type ", account["roles"])
		}

		for i := 0; i < len(roles); i++ {
			roleId := roles[i].(map[string]interface{})["$id"].(string)
			srv.deleteReference(p, rqst.Id, roleId, "members", "Roles")
			srv.publishEvent("update_role_"+roleId+"_evt", []byte{}, srv.Address)
		}

	}

	srv.deleteAllAccess(accountId+"@"+domain, rbacpb.SubjectType_ACCOUNT)

	// Try to delete the account...
	err = p.DeleteOne(context.Background(), "local_resource", "local_resource", "Accounts", q, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	name := account["name"].(string)
	name = strings.ReplaceAll(strings.ReplaceAll(name, ".", "_"), "@", "_")
	name = strings.ReplaceAll(name, "-", "_")
	name = strings.ReplaceAll(name, ".", "_")
	name = strings.ReplaceAll(name, " ", "_")

	get_contacts := `{}`

	// so before remove database I need to remove the accout from it contacts...
	contacts, err := p.Find(context.Background(), "local_resource", name+"_db", "Contacts", get_contacts, "")
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
			err = p.DeleteOne(context.Background(), "local_resource", name+"_db", "Contacts", q, "")

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
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("unknown database type "+p.GetStoreType())))
	}

	// I will execute the sript with the admin function.
	err = p.RunAdminCmd(context.Background(), "local_resource", srv.Backend_user, srv.Backend_password, dropUserScript)
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
	srv.deleteResourcePermissions("/users/" + name + "@" + domain)
	srv.deleteAllAccess(name+"@"+domain, rbacpb.SubjectType_ACCOUNT)

	os.RemoveAll(config.GetDataDir() + "/files/users/" + name + "@" + domain)

	// Publish delete account event.
	srv.publishEvent("delete_account_"+name+"@"+domain+"_evt", []byte{}, srv.Address)
	srv.publishEvent("delete_account_evt", []byte(name+"@"+domain), srv.Address)

	return &resourcepb.DeleteAccountRsp{
		Result: rqst.Id,
	}, nil
}

// Create an object reference inside another object, ex. add a reference to an account (field 'members') into a group.
func (srv *server) CreateReference(ctx context.Context, rqst *resourcepb.CreateReferenceRqst) (*resourcepb.CreateReferenceRsp, error) {
	// That service made user of persistence service.
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = srv.createReference(p, rqst.SourceId, rqst.SourceCollection, rqst.FieldName, rqst.TargetId, rqst.TargetCollection)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// reference was created...
	return &resourcepb.CreateReferenceRsp{}, nil
}

// Delete a reference from an object.
func (srv *server) DeleteReference(ctx context.Context, rqst *resourcepb.DeleteReferenceRqst) (*resourcepb.DeleteReferenceRsp, error) {
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = srv.deleteReference(p, rqst.RefId, rqst.TargetId, rqst.TargetField, rqst.TargetCollection)
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
func (srv *server) CreateRole(ctx context.Context, rqst *resourcepb.CreateRoleRqst) (*resourcepb.CreateRoleRsp, error) {

	clientId, _, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}

	// That service made user of persistence service.
	err = srv.createRole(rqst.Role.Id, rqst.Role.Name, clientId, rqst.Role.Description, rqst.Role.Actions)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
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

/**
 * Create a group with a given name of update existing one.
 */
func (srv *server) UpdateRole(ctx context.Context, rqst *resourcepb.UpdateRoleRqst) (*resourcepb.UpdateRoleRsp, error) {
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	q := `{"_id":"` + rqst.RoleId + `"}`

	// Get the persistence connection
	count, err := p.Count(context.Background(), "local_resource", "local_resource", "Roles", q, "")
	if err != nil || count == 0 {
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	} else {

		err = p.UpdateOne(context.Background(), "local_resource", "local_resource", "Roles", q, rqst.Values, "")
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	srv.publishEvent("update_role_"+rqst.RoleId+"_evt", []byte{}, srv.Address)

	return &resourcepb.UpdateRoleRsp{
		Result: true,
	}, nil
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
			fmt.Println("unknown type ", role["actions"])
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
			fmt.Println("unknown type ", role["organizations"])
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
			fmt.Println("unknown type ", role["members"])
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
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// No I will stream the result over the networks.
	maxSize := 100
	values := make([]*resourcepb.Role, 0)

	for i := 0; i < len(roles); i++ {
		role := roles[i].(map[string]interface{})
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
				fmt.Println("unknown type ", role["actions"])
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
				fmt.Println("unknown type ", role["organizations"])
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
				fmt.Println("unknown type ", role["members"])
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
func (srv *server) DeleteRole(ctx context.Context, rqst *resourcepb.DeleteRoleRqst) (*resourcepb.DeleteRoleRsp, error) {

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
			fmt.Println("unknown type ", role["members"])
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
			fmt.Println("unknown type ", role["organizations"])
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
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// TODO delete role permissions
	srv.deleteResourcePermissions(rqst.RoleId)
	srv.deleteAllAccess(rqst.RoleId, rbacpb.SubjectType_ROLE)

	srv.publishEvent("delete_role_"+rqst.RoleId+"_evt", []byte{}, srv.Address)
	srv.publishEvent("delete_role_evt", []byte(rqst.RoleId), srv.Address)

	return &resourcepb.DeleteRoleRsp{Result: true}, nil
}

// * Append an action to existing role. *
func (srv *server) AddRoleActions(ctx context.Context, rqst *resourcepb.AddRoleActionsRqst) (*resourcepb.AddRoleActionsRsp, error) {
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
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	q := `{"_id":"` + roleId + `"}`

	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Roles", q, ``)
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
		var actions []interface{}
		switch role["actions"].(type) {
		case primitive.A:
			actions = []interface{}(role["actions"].(primitive.A))
		case []interface{}:
			actions = []interface{}(role["actions"].([]interface{}))
		default:
			fmt.Println("unknown type ", role["actions"])
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
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	srv.publishEvent("update_role_"+rqst.RoleId+"_evt", []byte{}, srv.Address)

	return &resourcepb.AddRoleActionsRsp{Result: true}, nil
}

// * Remove an action to existing role. *
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
			var actions []interface{}
			switch role["actions"].(type) {
			case primitive.A:
				actions = []interface{}(role["actions"].(primitive.A))
			case []interface{}:
				actions = []interface{}(role["actions"].([]interface{}))
			default:
				fmt.Println("unknown type ", role["actions"])
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
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}

			srv.publishEvent("update_role_"+role["_id"].(string)+"@"+role["domain"].(string)+"_evt", []byte{}, srv.Address)

		}
	}

	return &resourcepb.RemoveRolesActionRsp{Result: true}, nil
}

// * Remove an action to existing role. *
func (srv *server) RemoveRoleAction(ctx context.Context, rqst *resourcepb.RemoveRoleActionRqst) (*resourcepb.RemoveRoleActionRsp, error) {
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
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	q := `{"_id":"` + roleId + `"}`

	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Roles", q, ``)
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
		var actions_ []interface{}
		switch role["actions"].(type) {
		case primitive.A:
			actions_ = []interface{}(role["actions"].(primitive.A))
		case []interface{}:
			actions_ = []interface{}(role["actions"].([]interface{}))
		default:
			fmt.Println("unknown type ", role["actions"])
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
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("Role named "+roleId+" not contain actions named "+rqst.Action+"!")))
		}
	}

	if needSave {
		// jsonStr, _ := Utility.ToJson(role)
		jsonStr := serialyseObject(role)

		err := p.ReplaceOne(context.Background(), "local_resource", "local_resource", "Roles", q, string(jsonStr), ``)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	srv.publishEvent("update_role_"+rqst.RoleId+"_evt", []byte{}, srv.Address)

	return &resourcepb.RemoveRoleActionRsp{Result: true}, nil
}

// * Add role to a given account *
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
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	srv.publishEvent("update_role_"+rqst.RoleId+"_evt", []byte{}, srv.Address)

	return &resourcepb.AddAccountRoleRsp{Result: true}, nil
}

// * Remove a role from a given account *
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
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = srv.deleteReference(p, rqst.RoleId, rqst.AccountId, "roles", "Accounts")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	srv.publishEvent("update_role_"+rqst.RoleId+"_evt", []byte{}, srv.Address)
	srv.publishEvent("update_account_"+rqst.AccountId+"_evt", []byte{}, srv.Address)

	return &resourcepb.RemoveAccountRoleRsp{Result: true}, nil
}

// * save a new application *
func (srv *server) save_application(app *resourcepb.Application, owner string) error {

	p, err := srv.getPersistenceStore()
	if err != nil {
		return err
	}

	if app == nil {
		return errors.New("no application object was given in the request")
	}

	q := `{"_id":"` + app.Id + `"}`

	count, err := p.Count(context.Background(), "local_resource", "local_resource", "Applications", q, "")

	application := make(map[string]interface{}, 0)
	application["_id"] = app.Id
	application["name"] = app.Name
	application["path"] = "/" + app.Name // The path must be the same as the application name.
	application["publisherid"] = app.Publisherid
	application["version"] = app.Version
	application["domain"] = srv.Domain // the domain where the application is save...
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

	db := app.Name + "_db"
	db = strings.ReplaceAll(db, "-", "_")
	db = strings.ReplaceAll(db, ".", "_")
	db = strings.ReplaceAll(db, " ", "_")

	// Here I will set the resource to manage the applicaiton access permission.
	if err != nil || count == 0 {

		var createApplicationDbScript string
		if p.GetStoreType() == "MONGO" {
			createApplicationDbScript = fmt.Sprintf(
				"db=db.getSiblingDB('%s');db.createCollection('application_data');db=db.getSiblingDB('admin');db.createUser({user: '%s', pwd: '%s',roles: [{ role: 'dbOwner', db: '%s' }]});", db, app.Name, app.Name, db)
		} else if p.GetStoreType() == "SCYLLA" {
			createApplicationDbScript = fmt.Sprintf(
				"CREATE KEYSPACE %s WITH REPLICATION = { 'class' : 'SimpleStrategy', 'replication_factor' : %d}; CREATE TABLE %s.application_data (id text PRIMARY KEY, data text);", db, srv.Backend_replication_factor, db)
		} else if p.GetStoreType() == "SQL" {
			q = `` // TODO sql query string here...
		} else {
			return errors.New("unknown database type " + p.GetStoreType())
		}

		// create the application database if not exist.
		if p.GetStoreType() == "MONGO" {
			err = p.RunAdminCmd(context.Background(), "local_resource", srv.Backend_user, srv.Backend_password, createApplicationDbScript)
			if err != nil {
				return err
			}
		} else if p.GetStoreType() == "SCYLLA" {
			err = p.RunAdminCmd(context.Background(), "local_resource", srv.Backend_user, srv.Backend_password, createApplicationDbScript)
			if err != nil {
				if !strings.Contains(err.Error(), "existing keyspace") {
					return err
				}
			}
		}

		application["creation_date"] = time.Now().Unix() // save it as unix time.
		_, err := p.InsertOne(context.Background(), "local_resource", "local_resource", "Applications", application, "")
		if err != nil {
			fmt.Println("error while inserting application ", err)
			return err
		}

	} else {
		if app.CreationDate == 0 {
			application["creation_date"] = time.Now().Unix() // save it as unix time.
		} else {
			application["creation_date"] = app.CreationDate
		}

		jsonStr, _ := Utility.ToJson(application)

		err := p.ReplaceOne(context.Background(), "local_resource", "local_resource", "Applications", q, jsonStr, "")

		if err != nil {
			return err
		}
	}

	// Create the application file directory.
	path := "/applications/" + app.Name
	Utility.CreateDirIfNotExist(config.GetDataDir() + "/files" + path)

	// Add resource owner
	srv.addResourceOwner(path, "file", app.Id+"@"+app.Domain, rbacpb.SubjectType_APPLICATION)

	// Add application owner
	srv.addResourceOwner(app.Id+"@"+app.Domain, "application", owner, rbacpb.SubjectType_ACCOUNT)

	// Publish application.
	srv.publishEvent("update_application_"+app.Id+"@"+app.Domain+"_evt", []byte{}, srv.Address)

	// Now I will create the application connection.
	address, _ := config.GetAddress()
	persistenceClient, err := GetPersistenceClient(address)

	if err != nil {
		return err
	}

	var storeType float64
	if srv.Backend_type == "SQL" {
		storeType = 1.0
	} else if srv.Backend_type == "MONGO" {
		storeType = 0.0
	} else if srv.Backend_type == "SCYLLA" {
		storeType = 2.0
	}

	// I will replace all special characters by underscore.

	// Now I will create the application connection.
	err = persistenceClient.CreateConnection(app.Name, db, address, float64(srv.Backend_port), storeType, srv.Backend_user, srv.Backend_password, 500, "", false)
	if err != nil {
		return err
	}

	return nil
}

// /////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
// Application
// /////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////
func (srv *server) CreateApplication(ctx context.Context, rqst *resourcepb.CreateApplicationRqst) (*resourcepb.CreateApplicationRsp, error) {

	clientId, _, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}

	err = srv.save_application(rqst.Application, clientId)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	jsonStr, err := protojson.Marshal(rqst.Application)
	if err == nil {
		srv.publishEvent("create_application_evt", jsonStr, srv.GetAddress())
	}

	return &resourcepb.CreateApplicationRsp{}, nil
}

// * Update application informations.
func (srv *server) UpdateApplication(ctx context.Context, rqst *resourcepb.UpdateApplicationRqst) (*resourcepb.UpdateApplicationRsp, error) {
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	q := `{"_id":"` + rqst.ApplicationId + `"}`

	err = p.UpdateOne(context.Background(), "local_resource", "local_resource", "Applications", q, rqst.Values, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	srv.publishEvent("update_application_"+rqst.ApplicationId+"_evt", []byte{}, srv.Address)

	return &resourcepb.UpdateApplicationRsp{}, nil
}

// * Delete an application from the srv. *
func (srv *server) DeleteApplication(ctx context.Context, rqst *resourcepb.DeleteApplicationRqst) (*resourcepb.DeleteApplicationRsp, error) {

	// That service made user of persistence service.
	err := srv.deleteApplication(rqst.ApplicationId)
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

func (srv *server) GetApplicationVersion(ctx context.Context, rqst *resourcepb.GetApplicationVersionRqst) (*resourcepb.GetApplicationVersionRsp, error) {

	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	var q string
	if p.GetStoreType() == "MONGO" {
		q = `{"name":"` + rqst.Id + `"}`
	} else if p.GetStoreType() == "SCYLLA" || p.GetStoreType() == "SQL" {
		q = `SELECT * FROM Applications WHERE name='` + rqst.Id + `'`
	} else {
		return nil, errors.New("unknown database type " + p.GetStoreType())
	}

	var previousVersion string
	previous, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Applications", q, `[{"Projection":{"version":1}}]`)
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

func (srv *server) GetApplicationAlias(ctx context.Context, rqst *resourcepb.GetApplicationAliasRqst) (*resourcepb.GetApplicationAliasRsp, error) {
	// That service made user of persistence service.
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	q := `{"_id":"` + rqst.Id + `"}`

	// Now I will retreive the application icon...
	data, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Applications", q, `[{"Projection":{"alias":1}}]`)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &resourcepb.GetApplicationAliasRsp{
		Alias: data.(string),
	}, nil
}

func (srv *server) GetApplicationIcon(ctx context.Context, rqst *resourcepb.GetApplicationIconRqst) (*resourcepb.GetApplicationIconRsp, error) {
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	q := `{"_id":"` + rqst.Id + `"}`

	// Now I will retreive the application icon...
	data, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Applications", q, `[{"Projection":{"icon":1}}]`)
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
func (srv *server) AddApplicationActions(ctx context.Context, rqst *resourcepb.AddApplicationActionsRqst) (*resourcepb.AddApplicationActionsRsp, error) {
	// That service made user of persistence service.
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	q := `{"_id":"` + rqst.ApplicationId + `"}`

	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Applications", q, ``)
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
		var actions_ []interface{}
		switch application["actions"].(type) {
		case primitive.A:
			actions_ = []interface{}(application["actions"].(primitive.A))
		case []interface{}:
			actions_ = []interface{}(application["actions"].([]interface{}))
		default:
			fmt.Println("unknown type ", application["actions"])
		}

		for j := 0; j < len(rqst.Actions); j++ {
			exist := false
			for i := 0; i < len(actions_); i++ {
				if actions_[i].(string) == rqst.Actions[j] {
					exist = true
					break
				}
				if !exist {
					actions_ = append(actions_, rqst.Actions[j])
					needSave = true
				}
			}
		}
	}

	if needSave {
		jsonStr := serialyseObject(application)
		err := p.ReplaceOne(context.Background(), "local_resource", "local_resource", "Applications", q, string(jsonStr), ``)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	srv.publishEvent("update_application_"+rqst.ApplicationId+"_evt", []byte{}, srv.Address)

	return &resourcepb.AddApplicationActionsRsp{Result: true}, nil
}

// * Remove an action to existing application. *
func (srv *server) RemoveApplicationAction(ctx context.Context, rqst *resourcepb.RemoveApplicationActionRqst) (*resourcepb.RemoveApplicationActionRsp, error) {

	// That service made user of persistence service.
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	q := `{"_id":"` + rqst.ApplicationId + `"}`

	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Applications", q, ``)
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

		var actions_ []interface{}
		switch application["actions"].(type) {
		case primitive.A:
			actions_ = []interface{}(application["actions"].(primitive.A))
		case []interface{}:
			actions_ = []interface{}(application["actions"].([]interface{}))
		default:
			fmt.Println("unknown type ", application["actions"])
		}

		for i := 0; i < len(actions_); i++ {
			if actions_[i].(string) == rqst.Action {
				exist = true
			} else {
				actions = append(actions, actions_[i])
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
		err := p.ReplaceOne(context.Background(), "local_resource", "local_resource", "Applications", q, string(jsonStr), ``)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	srv.publishEvent("update_application_"+rqst.ApplicationId+"_evt", []byte{}, srv.Address)

	return &resourcepb.RemoveApplicationActionRsp{Result: true}, nil
}

// * Remove an action to existing application. *
func (srv *server) RemoveApplicationsAction(ctx context.Context, rqst *resourcepb.RemoveApplicationsActionRqst) (*resourcepb.RemoveApplicationsActionRsp, error) {

	// That service made user of persistence service.
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	var q string
	if p.GetStoreType() == "MONGO" {
		q = `{}`
	} else if p.GetStoreType() == "SCYLLA" {
		q = `` // TODO scylla db query.
	} else if p.GetStoreType() == "SQL" {
		q = `SELECT * FROM Applications` // TODO sql query string here...
	} else {
		return nil, errors.New("unknown database type " + p.GetStoreType())
	}

	values, err := p.Find(context.Background(), "local_resource", "local_resource", "Applications", q, ``)
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

			var actions_ []interface{}
			switch application["actions"].(type) {
			case primitive.A:
				actions_ = []interface{}(application["actions"].(primitive.A))
			case []interface{}:
				actions_ = []interface{}(application["actions"].([]interface{}))
			default:
				fmt.Println("unknown type ", application["actions"])
			}

			for i := 0; i < len(actions_); i++ {
				if actions_[i].(string) == rqst.Action {
					exist = true
				} else {
					actions = append(actions, actions_[i])
				}
			}
			if exist {
				application["actions"] = actions
				needSave = true
			}
		}

		if needSave {
			jsonStr := serialyseObject(application)
			q = `{"_id":"` + application["_id"].(string) + `"}`
			err := p.ReplaceOne(context.Background(), "local_resource", "local_resource", "Applications", q, string(jsonStr), ``)
			srv.publishEvent("update_application_"+application["_id"].(string)+"_evt", []byte{}, srv.Address)
			if err != nil {
				return nil, status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}
		}
	}

	return &resourcepb.RemoveApplicationsActionRsp{Result: true}, nil
}

/**
 * Get application informations.
 */
func (srv *server) getApplications(query string, options string) ([]*resourcepb.Application, error) {

	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	if len(query) == 0 {
		query = "{}"
	}

	// So here I will get the list of retreived permission.
	values, err := p.Find(context.Background(), "local_resource", "local_resource", "Applications", query, options)
	if err != nil {
		return nil, err
	}

	applications := make([]*resourcepb.Application, 0)

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

			var actions_ []interface{}
			switch values_["actions"].(type) {
			case primitive.A:
				actions_ = []interface{}(values_["actions"].(primitive.A))
			case []interface{}:
				actions_ = []interface{}(values_["actions"].([]interface{}))
			default:
				fmt.Println("unknown type ", values_["actions"])
			}

			for i := 0; i < len(actions_); i++ {
				actions = append(actions, actions_[i].(string))
			}
		}
		application := &resourcepb.Application{Id: values_["_id"].(string), Name: values_["name"].(string), Domain: values_["domain"].(string), Path: values_["path"].(string), CreationDate: creationDate, LastDeployed: lastDeployed, Alias: values_["alias"].(string), Icon: values_["icon"].(string), Description: values_["description"].(string), Publisherid: values_["publisherid"].(string), Version: values_["version"].(string), Actions: actions}

		// TODO validate token...
		application.Password = values_["password"].(string)

		if err != nil {
			return nil, err
		}

		applications = append(applications, application)
	}

	return applications, nil
}

// /////////////////////  resource management. /////////////////
func (srv *server) GetApplications(rqst *resourcepb.GetApplicationsRqst, stream resourcepb.ResourceService_GetApplicationsServer) error {

	applications, err := srv.getApplications(rqst.Query, rqst.Options)

	if err != nil {
		return err
	}

	for i := 0; i < len(applications); i++ {
		err := stream.Send(&resourcepb.GetApplicationsRsp{
			Applications: []*resourcepb.Application{applications[i]},
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
	local_peer_.TypeName = "Peer"
	local_peer_.Protocol = localConfig["Protocol"].(string)
	local_peer_.PortHttp = int32(Utility.ToInt(localConfig["PortHttp"]))
	local_peer_.PortHttps = int32(Utility.ToInt(localConfig["PortHttps"]))
	local_peer_.Hostname = hostname
	local_peer_.Domain = domain
	local_peer_.ExternalIpAddress = Utility.MyIP()
	local_peer_.LocalIpAddress = config.GetLocalIP()
	local_peer_.Mac, _ = config.GetMacAddress()
	local_peer_.State = resourcepb.PeerApprovalState_PEER_PENDING

	return local_peer_
}

// ////////////////////// Resource Client ////////////////////////////////////////////
func GetPersistenceClient(address string) (*persistence_client.Persistence_Client, error) {
	Utility.RegisterFunction("NewPersistenceService_Client", persistence_client.NewPersistenceService_Client)
	client, err := globular_client.GetClient(address, "persistence.PersistenceService", "NewPersistenceService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*persistence_client.Persistence_Client), nil
}

// ////////////////////// Resource Client ////////////////////////////////////////////
func GetResourceClient(address string) (*resource_client.Resource_Client, error) {
	Utility.RegisterFunction("NewResourceService_Client", resource_client.NewResourceService_Client)
	client, err := globular_client.GetClient(address, "resource.ResourceService", "NewResourceService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*resource_client.Resource_Client), nil
}

// Register the actual peer (the one that running the resource server) to the one
// running at domain.
func (srv *server) registerPeer(address string) (*resourcepb.Peer, string, error) {
	// Connect to remote server and call Register peer on it...
	client, err := GetResourceClient(address)
	if err != nil {
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

	macAddress, err := config.GetMacAddress()
	if err != nil {
		return nil, "", err
	}

	localConfig, err := config.GetLocalConfig(true)
	httpPort := Utility.ToInt(localConfig["PortHttp"])
	httpsPort := Utility.ToInt(localConfig["PortHttps"])
	protocol := localConfig["Protocol"].(string)

	if err != nil {
		fmt.Println("fail to get local config ", err)
		return nil, "", err
	}

	return client.RegisterPeer(string(key), &resourcepb.Peer{Protocol: protocol, PortHttp: int32(httpPort), PortHttps: int32(httpsPort), Hostname: hostname, Mac: macAddress, Domain: domain, ExternalIpAddress: Utility.MyIP(), LocalIpAddress: config.GetLocalIP()})
}

// * Connect tow peer toggether on the network.
func (srv *server) RegisterPeer(ctx context.Context, rqst *resourcepb.RegisterPeerRqst) (*resourcepb.RegisterPeerRsp, error) {

	if rqst.Peer == nil {
		return nil, errors.New("no peer object was given in the request")
	}

	if rqst.Peer.LocalIpAddress == "" {
		return nil, errors.New("no local ip address was given in the request")
	}

	if rqst.Peer.ExternalIpAddress == "" {
		return nil, errors.New("no external ip address was given in the request")
	}

	// set the remote peer in /etc/hosts
	srv.setLocalHosts(rqst.Peer)

	// Here I will first look if a peer with a same name already exist on the
	if srv.Mac == rqst.Peer.Mac {
		return nil, errors.New("can not register peer to itself")
	}

	// Get the persistence connection
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	q := `{"_id":"` + Utility.GenerateUUID(rqst.Peer.Mac) +  `"}`

	// Here I will first look if a peer with a same name already exist on the
	// resources...
	if len(rqst.Peer.Mac) > 0 {
		values, _ := p.FindOne(context.Background(), "local_resource", "local_resource", "Peers", q, "")
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
	peer["protocol"] = rqst.Peer.Protocol

	// If no mac address was given it mean the request came from a web application
	// so the intention is to register the server itself on another srv...
	// This can also be done with the command line tool but in that case all values will be
	// set on the peers...

	if len(rqst.Peer.Mac) == 0 {


		// In that case I will use the hostname and the domain to set the address
		address_ := rqst.Peer.Hostname 
		if rqst.Peer.Domain != "localhost" {
			address_ += "." + rqst.Peer.Domain
		}

		if rqst.Peer.Protocol == "https" {
			address_ += ":" + Utility.ToString(rqst.Peer.PortHttps)
		} else {
			address_ += ":" + Utility.ToString(rqst.Peer.PortHttp)
		}

		// In that case I want to register the server to another srv.
		peer_, public_key, err := srv.registerPeer(address_)
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
		srv.setLocalHosts(peer_)

		// in case local dns is use that peers will be able to change values releated to it domain.
		// but no other peer will be able to do it...
		srv.addResourceOwner(peer_.Domain, "domain", peer_.Mac, rbacpb.SubjectType_PEER)

		jsonStr, err := protojson.Marshal(peer_)
		if err != nil {
			return nil, err
		}

		// Update peer event.
		srv.publishEvent("update_peers_evt", jsonStr, srv.Address)

		address := rqst.Peer.Hostname
		if rqst.Peer.Domain != "localhost" {
			address += "." + rqst.Peer.Domain
		}

		if rqst.Peer.Protocol == "https" {
			address += ":" + Utility.ToString(rqst.Peer.PortHttps)
		} else {
			address += ":" + Utility.ToString(rqst.Peer.PortHttp)
		}

		// So here I need to publish my information as a pee

		// Publish local peer information...
		jsonStr, err = protojson.Marshal(getLocalPeer())
		if err != nil {
			return nil, err
		}

		srv.publishRemoteEvent(address, "update_peers_evt", jsonStr)

		// Set peer action
		srv.addPeerActions(peer_.Mac, []string{"/dns.DnsService/SetA"})
		srv.addPeerActions(peer_.Mac, []string{"/dns.DnsService/SetAAAA"})
		srv.addPeerActions(peer_.Mac, []string{"/dns.DnsService/SetCaa"})
		srv.addPeerActions(peer_.Mac, []string{"/dns.DnsService/SetNs"})
		srv.addPeerActions(peer_.Mac, []string{"/dns.DnsService/SetMx"})
		srv.addPeerActions(peer_.Mac, []string{"/dns.DnsService/SetSoa"})
		srv.addPeerActions(peer_.Mac, []string{"/dns.DnsService/SetText"})
		srv.addPeerActions(peer_.Mac, []string{"/dns.DnsService/RemoveText"})

		// Send back the peers informations.
		return &resourcepb.RegisterPeerRsp{Peer: peer_, PublicKey: public_key}, nil

	}

	// Here I will keep the peer info until it will be accepted by the admin of the other peer.
	peer["_id"] = Utility.GenerateUUID(rqst.Peer.Mac)
	peer["mac"] = rqst.Peer.Mac
	peer["hostname"] = rqst.Peer.Hostname
	peer["domain"] = rqst.Peer.Domain
	peer["protocol"] = rqst.Peer.Protocol
	peer["portHttps"] = rqst.Peer.PortHttps
	peer["portHttp"] = rqst.Peer.PortHttp
	peer["local_ip_address"] = rqst.Peer.LocalIpAddress
	peer["external_ip_address"] = rqst.Peer.ExternalIpAddress
	peer["state"] = resourcepb.PeerApprovalState_PEER_PENDING
	peer["actions"] = []interface{}{}

	// if the token is generate by the sa and it has permission i will accept the peer directly
	/*
		peer["state"] = resourcepb.PeerApprovalState_PEER_ACCETEP
		peer["actions"] = []interface{}{"/dns.DnsService/SetA"}
		peer["actions"] = []interface{}{"/dns.DnsService/SetAAAA"}
		peer["actions"] = []interface{}{"/dns.DnsService/SetCAA"}
		peer["actions"] = []interface{}{"/dns.DnsService/SetText"}
		peer["actions"] = []interface{}{"/dns.DnsService/RemoveText"}
	*/

	domain := rqst.Peer.Hostname
	if len(rqst.Peer.Domain) > 0 {
		domain += "." + rqst.Peer.Domain
	}

	srv.addResourceOwner(domain, "domain", rqst.Peer.Mac, rbacpb.SubjectType_PEER)

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


	jsonStr, err := protojson.Marshal(rqst.Peer)
	if err != nil {
		return nil, err
	}

	// signal peers changes...
	srv.publishEvent("update_peers_evt", jsonStr, srv.GetAddress())

	address_ := rqst.Peer.Hostname 
	if rqst.Peer.Domain != "localhost" {
		address_ += "." + rqst.Peer.Domain
	}

	if rqst.Peer.Protocol == "https" {
		address_ += ":" + Utility.ToString(rqst.Peer.PortHttps)
	} else {
		address_ += ":" + Utility.ToString(rqst.Peer.PortHttp)
	}

	jsonStr, err = protojson.Marshal(getLocalPeer())
	if err != nil {
		return nil, err
	}

	srv.publishRemoteEvent(address_, "update_peers_evt", jsonStr)

	srv.setLocalHosts(getLocalPeer())

	return &resourcepb.RegisterPeerRsp{
		Peer:      getLocalPeer(),
		PublicKey: string(pubKey),
	}, nil
}

// * Return the peer public key */
func (srv *server) GetPeerPublicKey(ctx context.Context, rqst *resourcepb.GetPeerPublicKeyRqst) (*resourcepb.GetPeerPublicKeyRsp, error) {
	public_key, err := srv.getPeerPublicKey(rqst.RemotePeerAddress, rqst.Mac)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &resourcepb.GetPeerPublicKeyRsp{PublicKey: public_key}, nil
}

// * Accept a given peer *
func (srv *server) AcceptPeer(ctx context.Context, rqst *resourcepb.AcceptPeerRqst) (*resourcepb.AcceptPeerRsp, error) {
	// Get the persistence connection
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	q := `{"_id":"` + Utility.GenerateUUID(rqst.Peer.Mac) + `"}`

	// Now I will retreive the peer informations.
	setState := map[string]interface{}{"$set": map[string]interface{}{"state": resourcepb.PeerApprovalState_PEER_ACCETEP}}
	setStateStr, err := Utility.ToJson(setState)
	if err != nil {
		return nil, err
	}

	err = p.UpdateOne(context.Background(), "local_resource", "local_resource", "Peers", q, setStateStr, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Add actions require by peer...
	srv.addPeerActions(rqst.Peer.Mac, []string{"/dns.DnsService/SetA"})
	srv.addPeerActions(rqst.Peer.Mac, []string{"/dns.DnsService/SetAAAA"})
	srv.addPeerActions(rqst.Peer.Mac, []string{"/dns.DnsService/SetCAA"})
	srv.addPeerActions(rqst.Peer.Mac, []string{"/dns.DnsService/SetText"})
	srv.addPeerActions(rqst.Peer.Mac, []string{"/dns.DnsService/RemoveText"})

	// set the remote peer in /etc/hosts
	srv.setLocalHosts(rqst.Peer)

	// Here I will append the resource owner...
	domain := rqst.Peer.Hostname
	if rqst.Peer.Domain != "localhost" {
		domain += "." + rqst.Peer.Domain
	}

	// in case local dns is use that peers will be able to change values releated to it domain.
	// but no other peer will be able to do it...
	srv.addResourceOwner(domain, "domain", rqst.Peer.Mac, rbacpb.SubjectType_PEER)
	jsonStr, err := protojson.Marshal(rqst.Peer)
	if err != nil {
		return nil, err
	}

	// signal peers changes...
	srv.publishEvent("update_peers_evt", jsonStr, srv.Address)

	address_ := rqst.Peer.Hostname
	if rqst.Peer.Domain != "localhost" {
		address_ += "." + rqst.Peer.Domain
	}

	if rqst.Peer.Protocol == "https" {
		address_ += ":" + Utility.ToString(rqst.Peer.PortHttps)
	} else {
		address_ += ":" + Utility.ToString(rqst.Peer.PortHttp)
	}

	jsonStr, err = protojson.Marshal(getLocalPeer())
	if err != nil {
		return nil, err
	}
	srv.publishRemoteEvent(address_, "update_peers_evt", jsonStr)

	return &resourcepb.AcceptPeerRsp{Result: true}, nil
}

// * Reject a given peer, note that the peer will stay reject, so
// I will be imposible to request again and again, util it will be
// explicitly removed from the peer's list
func (srv *server) RejectPeer(ctx context.Context, rqst *resourcepb.RejectPeerRqst) (*resourcepb.RejectPeerRsp, error) {

	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	q := `{"_id":"` + Utility.GenerateUUID(rqst.Peer.Mac) +`"}`

	setState := `{ "$set":{"state":2}}`

	err = p.UpdateOne(context.Background(), "local_resource", "local_resource", "Peers", q, setState, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	jsonStr, err := protojson.Marshal(rqst.Peer)
	if err != nil {
		return nil, err
	}

	// signal peers changes...
	srv.publishEvent("update_peers_evt", jsonStr, srv.Address)

	address_ := rqst.Peer.Hostname
	if rqst.Peer.Domain != "localhost" {
		address_ += "." + rqst.Peer.Domain
	}

	if rqst.Peer.Protocol == "https" {
		address_ += ":" + Utility.ToString(rqst.Peer.PortHttps)
	} else {
		address_ += ":" + Utility.ToString(rqst.Peer.PortHttp)
	}

	jsonStr, err = protojson.Marshal(getLocalPeer())
	if err != nil {
		return nil, err
	}
	srv.publishRemoteEvent(address_, "update_peers_evt", jsonStr)

	return &resourcepb.RejectPeerRsp{Result: true}, nil
}

/**
 * Return the state of approval of a peer by anther one.
 */
func (srv *server) GetPeerApprovalState(ctx context.Context, rqst *resourcepb.GetPeerApprovalStateRqst) (*resourcepb.GetPeerApprovalStateRsp, error) {
	mac := rqst.Mac
	if len(mac) == 0 {
		var err error
		mac, err = config.GetMacAddress()
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	peer, err := srv.getPeerInfos(rqst.RemotePeerAddress, mac)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &resourcepb.GetPeerApprovalStateRsp{State: peer.GetState()}, nil
}

func initPeer(values interface{}) *resourcepb.Peer {
	values_ := values.(map[string]interface{})
	state := resourcepb.PeerApprovalState(int32(Utility.ToInt(values_["state"])))

	portHttp := int32(80)
	if values_["portHttp"] != nil {
		portHttp = int32(Utility.ToInt(values_["portHttp"]))
	} else if values_["port_http"] != nil {
		portHttp = int32(Utility.ToInt(values_["port_http"]))
	}

	portHttps := int32(443)
	if values_["portHttps"] != nil {
		portHttps = int32(Utility.ToInt(values_["portHttps"]))
	} else if values_["port_https"] != nil {
		portHttps = int32(Utility.ToInt(values_["port_https"]))
	}

	hostname := values_["hostname"].(string)
	domain := values_["domain"].(string)

	externalIpAddress := ""
	if values_["external_ip_address"] != nil {
		externalIpAddress = values_["external_ip_address"].(string)
	} else if values_["externalIpAddress"] != nil {
		externalIpAddress = values_["externalIpAddress"].(string)
	}

	localIpAddress := ""
	if values_["local_ip_address"] != nil {
		localIpAddress = values_["local_ip_address"].(string)
	} else if values_["localIpAddress"] != nil {
		localIpAddress = values_["localIpAddress"].(string)
	}

	mac := values_["mac"].(string)
	p := &resourcepb.Peer{Protocol: values_["protocol"].(string), PortHttp: portHttp, PortHttps: portHttps, Hostname: hostname, Domain: domain, ExternalIpAddress: externalIpAddress, LocalIpAddress: localIpAddress, Mac: mac, Actions: make([]string, 0), State: state}

	var actions_ []interface{}
	switch values_["actions"].(type) {
	case primitive.A:
		actions_ = []interface{}(values_["actions"].(primitive.A))
	case []interface{}:
		actions_ = values_["actions"].([]interface{})
	}

	for j := 0; j < len(actions_); j++ {
		p.Actions = append(p.Actions, actions_[j].(string))
	}

	return p
}

// * Return the list of authorized peers *
func (srv *server) GetPeers(rqst *resourcepb.GetPeersRqst, stream resourcepb.ResourceService_GetPeersServer) error {

	// Get the persistence connection
	p, err := srv.getPersistenceStore()
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
		if p.Mac != srv.Mac {
			values = append(values, p)
		}
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

func (srv *server) deletePeer(token, address string) error {
	// Connect to remote server and call Register peer on it...
	client, err := GetResourceClient(address)
	if err != nil {
		return err
	}

	return client.DeletePeer(token, srv.Mac)

}

// * Update a peer
func (srv *server) UpdatePeer(ctx context.Context, rqst *resourcepb.UpdatePeerRqst) (*resourcepb.UpdatePeerRsp, error) {
	// Get the persistence connection
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	q := `{"_id":"` + Utility.GenerateUUID(rqst.Peer.Mac) + `"}`

	values, err := p.FindOne(ctx, "local_resource", "local_resource", "Peers", q, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// init the peer object.
	peer := initPeer(values)

	// Here I will update the peer information.
	peer.Protocol = rqst.Peer.Protocol
	peer.PortHttps = rqst.Peer.PortHttps
	peer.PortHttp = rqst.Peer.PortHttp
	peer.LocalIpAddress = rqst.Peer.LocalIpAddress
	peer.ExternalIpAddress = rqst.Peer.ExternalIpAddress

	jsonStr, err := json.Marshal(rqst.Peer)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// update peer values.
	setValues := map[string]interface{}{"$set": map[string]interface{}{"hostname": rqst.Peer.Hostname, "domain": rqst.Peer.Domain, "protocol": rqst.Peer.Protocol, "local_ip_address": rqst.Peer.LocalIpAddress, "external_ip_address": rqst.Peer.ExternalIpAddress}}

	if p.GetStoreType() == "SCYLLA" {
		// Scylla does not support camel case...
		setValues["$set"].(map[string]interface{})["port_https"] = rqst.Peer.PortHttps
		setValues["$set"].(map[string]interface{})["port_http"] = rqst.Peer.PortHttp
	} else {
		// MONGO and SQL
		setValues["$set"].(map[string]interface{})["portHttps"] = rqst.Peer.PortHttps
		setValues["$set"].(map[string]interface{})["portHttp"] = rqst.Peer.PortHttp
	}

	setValues_, err := Utility.ToJson(setValues)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = p.UpdateOne(context.Background(), "local_resource", "local_resource", "Peers", q, setValues_, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// signal peers changes...
	srv.publishEvent("update_peer_"+rqst.Peer.Mac+"_evt", []byte{}, srv.Address)

	address := rqst.Peer.Hostname
	if len(rqst.Peer.Domain) > 0 {
		address += "." + rqst.Peer.Domain
	}

	if rqst.Peer.Protocol == "https" {
		address += ":" + Utility.ToString(rqst.Peer.PortHttps)
	} else {
		address += ":" + Utility.ToString(rqst.Peer.PortHttp)
	}
	srv.publishEvent("update_peer_"+rqst.Peer.Mac+"_evt", []byte{}, address)

	// give the peer information...
	srv.publishEvent("update_peers_evt", jsonStr, srv.Address)

	return &resourcepb.UpdatePeerRsp{Result: true}, nil
}

// * Remove a peer from the network *
func (srv *server) DeletePeer(ctx context.Context, rqst *resourcepb.DeletePeerRqst) (*resourcepb.DeletePeerRsp, error) {
	// Get the persistence connection
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	q := `{"_id":"` + Utility.GenerateUUID(rqst.Peer.Mac) +`"}`

	// try to get the peer from the database.
	data, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Peers", q, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// the peer was not found.
	if data == nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no peer with mac "+rqst.Peer.Mac+" was found")))
	}

	// init the peer object.
	peer := initPeer(data)

	// Delete all peer access.
	srv.deleteAllAccess(peer.Mac, rbacpb.SubjectType_PEER)

	err = p.DeleteOne(context.Background(), "local_resource", "local_resource", "Peers", q, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Delete permissions
	srv.deleteResourcePermissions(peer.Mac)
	srv.deleteAllAccess(rqst.Peer.Mac, rbacpb.SubjectType_PEER)

	// Delete peer public key...
	security.DeletePublicKey(peer.Mac)

	// remove from /etc/hosts
	srv.removeFromLocalHosts(peer)

	// Here I will append the resource owner...
	domain := peer.Hostname
	if len(peer.Domain) > 0 {
		domain += "." + peer.Domain
	}

	// signal peers changes...
	address := peer.Hostname
	if peer.Domain != "localhost" {
		address += "." + peer.Domain
	}

	if peer.Protocol == "https" {
		address += ":" + Utility.ToString(peer.PortHttps)
	} else {
		address += ":" + Utility.ToString(peer.PortHttp)
	}

	srv.publishEvent("delete_peer"+peer.Mac+"_evt", []byte{}, srv.Address)
	srv.publishEvent("delete_peer"+peer.Mac+"_evt", []byte{}, address)

	srv.publishEvent("delete_peer_evt", []byte(peer.Mac), srv.Address)
	srv.publishEvent("delete_peer_evt", []byte(peer.Mac), address)

	address_ := peer.Domain
	if peer.Protocol == "https" {
		address_ += ":" + Utility.ToString(peer.PortHttps)
	} else {
		address_ += ":" + Utility.ToString(peer.PortHttp)
	}

	// Also remove the peer at the other end...
	_, token, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}

	srv.deletePeer(token, address_)

	return &resourcepb.DeletePeerRsp{
		Result: true,
	}, nil
}

func (srv *server) addPeerActions(mac string, actions_ []string) error {
	p, err := srv.getPersistenceStore()
	if err != nil {
		return err
	}

	var q string
	if p.GetStoreType() == "MONGO" {
		q = `{"mac":"` + mac + `"}`
	} else if p.GetStoreType() == "SCYLLA" || p.GetStoreType() == "SQL" {
		q = `SELECT * FROM Peers WHERE mac='` + mac + `'`
	} else {
		return errors.New("unknown database type " + p.GetStoreType())
	}

	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Peers", q, ``)
	if err != nil {
		return err
	}

	peer := values.(map[string]interface{})

	needSave := false
	if peer["actions"] == nil {
		peer["actions"] = actions_
		needSave = true
	} else {

		var actions []interface{}
		switch peer["actions"].(type) {
		case primitive.A:
			actions = []interface{}(peer["actions"].(primitive.A))
		case []interface{}:
			actions = peer["actions"].([]interface{})
		}

		for j := 0; j < len(actions_); j++ {
			exist := false
			for i := 0; i < len(actions); i++ {
				if actions[i].(string) == actions_[j] {
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
		err := p.ReplaceOne(context.Background(), "local_resource", "local_resource", "Peers", q, string(jsonStr), ``)
		if err != nil {
			return err
		}
	}

	// signal peers changes...
	srv.publishEvent("update_peer_"+mac+"_evt", []byte{}, srv.Address)

	return nil
}

// * Add peer action permission *
func (srv *server) AddPeerActions(ctx context.Context, rqst *resourcepb.AddPeerActionsRqst) (*resourcepb.AddPeerActionsRsp, error) {

	err := srv.addPeerActions(rqst.Mac, rqst.Actions)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	srv.publishEvent("update_peer_"+rqst.Mac+"_evt", []byte{}, srv.Address)

	return &resourcepb.AddPeerActionsRsp{Result: true}, nil

}

// * Remove peer action permission *
func (srv *server) RemovePeerAction(ctx context.Context, rqst *resourcepb.RemovePeerActionRqst) (*resourcepb.RemovePeerActionRsp, error) {
	// That service made user of persistence service.
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	var q string
	if p.GetStoreType() == "MONGO" {
		q = `{"mac":"` + rqst.Mac + `"}`
	} else if p.GetStoreType() == "SCYLLA" || p.GetStoreType() == "SQL" {
		q = `SELECT * FROM Peers WHERE mac='` + rqst.Mac + `'`
	} else {
		return nil, errors.New("unknown database type " + p.GetStoreType())
	}

	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Peers", q, ``)
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
		err := p.ReplaceOne(context.Background(), "local_resource", "local_resource", "Peers", q, string(jsonStr), ``)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	// signal peers changes...
	srv.publishEvent("update_peer_"+rqst.Mac+"_evt", []byte{}, srv.Address)

	return &resourcepb.RemovePeerActionRsp{Result: true}, nil
}

func (srv *server) RemovePeersAction(ctx context.Context, rqst *resourcepb.RemovePeersActionRqst) (*resourcepb.RemovePeersActionRsp, error) {
	// That service made user of persistence service.
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	q := `{}`

	values, err := p.Find(context.Background(), "local_resource", "local_resource", "Peers", q, ``)
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
			q = `{"_id":"` + peer["_id"].(string) + `"}`
			jsonStr := serialyseObject(peer)
			err := p.ReplaceOne(context.Background(), "local_resource", "local_resource", "Peers", q, string(jsonStr), ``)
			srv.publishEvent("update_peer_"+peer["_id"].(string)+"_evt", []byte{}, srv.Address)
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
func (srv *server) CreateOrganization(ctx context.Context, rqst *resourcepb.CreateOrganizationRqst) (*resourcepb.CreateOrganizationRsp, error) {

	clientId, _, err := security.GetClientId(ctx)
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
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
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
	srv.addResourceOwner(rqst.Organization.GetId()+"@"+rqst.Organization.Domain, "organization", clientId, rbacpb.SubjectType_ACCOUNT)

	return &resourcepb.CreateOrganizationRsp{
		Result: true,
	}, nil
}

// Update an organization informations.
func (srv *server) UpdateOrganization(ctx context.Context, rqst *resourcepb.UpdateOrganizationRqst) (*resourcepb.UpdateOrganizationRsp, error) {
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	q := `{"_id":"` + rqst.OrganizationId + `"}`

	// Get the persistence connection
	count, err := p.Count(context.Background(), "local_resource", "local_resource", "Organizations", q, "")
	if err != nil || count == 0 {
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	} else {

		err = p.UpdateOne(context.Background(), "local_resource", "local_resource", "Organizations", q, rqst.Values, "")
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	srv.publishEvent("update_organization_"+rqst.OrganizationId+"_evt", []byte{}, srv.Address)

	return &resourcepb.UpdateOrganizationRsp{
		Result: true,
	}, nil
}

// * Return the list of organizations
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
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
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
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	srv.publishEvent("update_organization_"+rqst.OrganizationId+"_evt", []byte{}, srv.Address)
	srv.publishEvent("update_organization_"+rqst.OrganizationId+"_evt", []byte{}, srv.Address)

	return &resourcepb.AddOrganizationAccountRsp{Result: true}, nil
}

// * Add Group *
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
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	srv.publishEvent("update_organization_"+rqst.OrganizationId+"_evt", []byte{}, srv.Address)
	srv.publishEvent("update_organization_"+rqst.OrganizationId+"_evt", []byte{}, srv.Address)

	return &resourcepb.AddOrganizationGroupRsp{Result: true}, nil
}

// * Add Role *
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
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	srv.publishEvent("update_organization_"+rqst.OrganizationId+"_evt", []byte{}, srv.Address)
	srv.publishEvent("update_organization_"+rqst.OrganizationId+"_evt", []byte{}, srv.Address)

	return &resourcepb.AddOrganizationRoleRsp{Result: true}, nil
}

// * Add Application *
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
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	srv.publishEvent("update_organization_"+rqst.OrganizationId+"_evt", []byte{}, srv.Address)
	srv.publishEvent("update_organization_"+rqst.OrganizationId+"_evt", []byte{}, srv.Address)

	return &resourcepb.AddOrganizationApplicationRsp{Result: true}, nil
}

// * Remove Account *
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
	srv.publishEvent("update_organization_"+rqst.OrganizationId+"_evt", []byte{}, srv.Address)

	return &resourcepb.RemoveOrganizationAccountRsp{Result: true}, nil
}

// * Remove Group *
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

// * Remove Role *
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

// * Remove Application *
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

// * Delete organization
func (srv *server) DeleteOrganization(ctx context.Context, rqst *resourcepb.DeleteOrganizationRqst) (*resourcepb.DeleteOrganizationRsp, error) {

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
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
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
					fmt.Println(err)
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
					fmt.Println(err)
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
					fmt.Println(err)
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
					fmt.Println(err)
				}

				srv.publishEvent("update_account_"+accountId+"_evt", []byte{}, srv.Address)
			}
		}
	}

	// Delete organization
	organizationId = organization["_id"].(string) + "@" + organization["domain"].(string)
	srv.deleteAllAccess(organizationId, rbacpb.SubjectType_ORGANIZATION)

	// Try to delete the account...
	err = p.DeleteOne(context.Background(), "local_resource", "local_resource", "Organizations", q, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	srv.deleteResourcePermissions(organizationId)
	srv.deleteAllAccess(organizationId, rbacpb.SubjectType_ORGANIZATION)

	srv.publishEvent("delete_organization_"+organizationId+"_evt", []byte{}, srv.Address)
	srv.publishEvent("delete_organization_evt", []byte(organizationId), srv.Address)

	return &resourcepb.DeleteOrganizationRsp{Result: true}, nil
}

/**
 * Create a group with a given name of update existing one.
 */
func (srv *server) UpdateGroup(ctx context.Context, rqst *resourcepb.UpdateGroupRqst) (*resourcepb.UpdateGroupRsp, error) {
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	q := `{"_id":"` + rqst.GroupId + `"}`

	// Get the persistence connection
	count, err := p.Count(context.Background(), "local_resource", "local_resource", "Groups", q, "")
	if err != nil || count == 0 {
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	} else {
		err = p.UpdateOne(context.Background(), "local_resource", "local_resource", "Groups", q, rqst.Values, "")
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	srv.publishEvent("update_group_"+rqst.GroupId+"_evt", []byte{}, srv.Address)

	return &resourcepb.UpdateGroupRsp{
		Result: true,
	}, nil
}

// * Register a new group
func (srv *server) CreateGroup(ctx context.Context, rqst *resourcepb.CreateGroupRqst) (*resourcepb.CreateGroupRsp, error) {

	clientId, _, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}

	// Get the persistence connection
	err = srv.createGroup(rqst.Group.Id, rqst.Group.Name, clientId, rqst.Group.Description, rqst.Group.Members)

	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	jsonStr, err := json.Marshal(rqst.Group)
	if err == nil {
		srv.publishEvent("create_group_evt", jsonStr, srv.GetAddress())
	}

	return &resourcepb.CreateGroupRsp{
		Result: true,
	}, nil
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

// * Return the list of organizations
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
func (srv *server) DeleteGroup(ctx context.Context, rqst *resourcepb.DeleteGroupRqst) (*resourcepb.DeleteGroupRsp, error) {

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
	p, err := srv.getPersistenceStore()
	if err != nil {
		fmt.Println("fail to get persistence connection ", err)
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
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
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
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	groupId = group["_id"].(string) + "@" + group["domain"].(string)

	srv.deleteResourcePermissions(rqst.Group)
	srv.deleteAllAccess(groupId, rbacpb.SubjectType_GROUP)

	srv.publishEvent("delete_group_"+groupId+"_evt", []byte{}, srv.Address)

	srv.publishEvent("delete_group_evt", []byte(groupId), srv.Address)

	return &resourcepb.DeleteGroupRsp{
		Result: true,
	}, nil

}

// * Add a member account to the group *
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
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	srv.publishEvent("update_group_"+rqst.GroupId+"_evt", []byte{}, srv.Address)
	srv.publishEvent("update_account_"+rqst.AccountId+"_evt", []byte{}, srv.Address)

	return &resourcepb.AddGroupMemberAccountRsp{Result: true}, nil
}

// * Remove member account from the group *
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
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = srv.deleteReference(p, rqst.GroupId, rqst.AccountId, "groups", "Accounts")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	srv.publishEvent("update_group_"+rqst.GroupId+"_evt", []byte{}, srv.Address)
	srv.publishEvent("update_account_"+rqst.AccountId+"_evt", []byte{}, srv.Address)

	return &resourcepb.RemoveGroupMemberAccountRsp{Result: true}, nil
}

// //////////////////////////////////////////////////////////////////////////////////
// Notification implementation
// //////////////////////////////////////////////////////////////////////////////////
// * Create a notification
func (srv *server) CreateNotification(ctx context.Context, rqst *resourcepb.CreateNotificationRqst) (*resourcepb.CreateNotificationRsp, error) {

	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	q := `{"_id":"` + rqst.Notification.Id + `"}`

	// so the recipient here is the id of the user...
	recipient := strings.Split(rqst.Notification.Recipient, "@")[0]

	name := strings.ReplaceAll(recipient, "-", "_")
	name = strings.ReplaceAll(name, ".", "_")
	name = strings.ReplaceAll(name, " ", "_")

	count, _ := p.Count(context.Background(), "local_resource", name+"_db", "Notifications", q, "")
	if count > 0 {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("Notification with id "+rqst.Notification.Id+" already exist")))
	}

	// if the account is not on the domain will redirect the request...
	if rqst.Notification.NotificationType == resourcepb.NotificationType_USER_NOTIFICATION {
		recipient := rqst.Notification.Recipient
		localDomain, _ := config.GetDomain()
		if strings.Contains(recipient, "@") {
			domain := strings.Split(recipient, "@")[1]

			if localDomain != domain {
				client, err := GetResourceClient(domain)
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
		} else {
			recipient += "@" + localDomain
		}
	}

	// insert notification into recipient database
	_, err = p.InsertOne(context.Background(), "local_resource", name+"_db", "Notifications", rqst.Notification, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	jsonStr, err := json.Marshal(rqst.Notification)
	if err == nil {
		srv.publishEvent("create_notification_evt", jsonStr, srv.GetAddress())
	}

	return &resourcepb.CreateNotificationRsp{}, nil
}

// * Retreive notifications
func (srv *server) GetNotifications(rqst *resourcepb.GetNotificationsRqst, stream resourcepb.ResourceService_GetNotificationsServer) error {

	if len(rqst.Recipient) == 0 {
		return status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("recipient is empty")))
	}

	db := rqst.Recipient
	if strings.Contains(db, "@") {
		db = strings.Split(db, "@")[0]
	}

	db = strings.ReplaceAll(db, "-", "_")
	db = strings.ReplaceAll(db, ".", "_")
	db = strings.ReplaceAll(db, " ", "_")

	db += "_db"

	if !strings.Contains(rqst.Recipient, "@") {
		rqst.Recipient += "@" + srv.Domain
	}

	// Get the persistence connection
	p, err := srv.getPersistenceStore()
	if err != nil {
		return err
	}

	query := `{"recipient":"` + rqst.Recipient + `"}`
	if len(query) == 0 {
		query = "{}"
	}

	notifications, err := p.Find(context.Background(), "local_resource", db, "Notifications", query, "")
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
		noticationDate := Utility.ToInt(n_["date"])

		values = append(values, &resourcepb.Notification{Id: n_["_id"].(string), Mac: n_["mac"].(string), Sender: n_["sender"].(string), Date: int64(noticationDate), Recipient: n_["recipient"].(string), Message: n_["message"].(string), NotificationType: notificationType})
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
func (srv *server) DeleteNotification(ctx context.Context, rqst *resourcepb.DeleteNotificationRqst) (*resourcepb.DeleteNotificationRsp, error) {

	if len(rqst.Recipient) == 0 {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("recipient is empty")))
	}

	db := rqst.Recipient
	if strings.Contains(db, "@") {
		db = strings.Split(db, "@")[0]
	}

	db = strings.ReplaceAll(db, "-", "_")
	db = strings.ReplaceAll(db, ".", "_")
	db = strings.ReplaceAll(db, " ", "_")

	db += "_db"

	if !strings.Contains(rqst.Recipient, "@") {
		rqst.Recipient += "@" + srv.Domain
	}

	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	q := `{"_id":"` + rqst.Id + `"}`

	err = p.DeleteOne(context.Background(), "local_resource", db, "Notifications", q, ``)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	srv.publishEvent("delete_notification_"+rqst.Id+"_evt", []byte{}, srv.Address)
	srv.publishEvent("delete_notification_evt", []byte(rqst.Id), srv.Address)

	return &resourcepb.DeleteNotificationRsp{}, nil
}

// * Remove all Notification
func (srv *server) ClearAllNotifications(ctx context.Context, rqst *resourcepb.ClearAllNotificationsRqst) (*resourcepb.ClearAllNotificationsRsp, error) {

	if len(rqst.Recipient) == 0 {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("recipient is empty")))
	}

	db := rqst.Recipient
	if strings.Contains(db, "@") {
		db = strings.Split(db, "@")[0]
	}

	db = strings.ReplaceAll(db, "-", "_")
	db = strings.ReplaceAll(db, ".", "_")
	db = strings.ReplaceAll(db, " ", "_")

	db += "_db"

	if !strings.Contains(rqst.Recipient, "@") {
		rqst.Recipient += "@" + srv.Domain
	}

	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	var query string
	if p.GetStoreType() == "MONGO" {
		query = `{}`
	} else if p.GetStoreType() == "SCYLLA" {
		query = `SELECT * FROM Notifications WHERE recipient='` + rqst.Recipient + `' ALLOW FILTERING`
	} else if p.GetStoreType() == "SQL" {
		query = `SELECT * FROM Notifications WHERE recipient='` + rqst.Recipient + `'`
	} else {
		return nil, errors.New("unknown database type " + p.GetStoreType())
	}

	err = p.Delete(context.Background(), "local_resource", db, "Notifications", query, ``)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	srv.publishEvent("clear_notification_evt", []byte{}, srv.Address)

	return &resourcepb.ClearAllNotificationsRsp{}, nil
}

// * Remove all notification of a given type
func (srv *server) ClearNotificationsByType(ctx context.Context, rqst *resourcepb.ClearNotificationsByTypeRqst) (*resourcepb.ClearNotificationsByTypeRsp, error) {
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	notificationType := int32(rqst.NotificationType)

	db := rqst.Recipient
	if strings.Contains(db, "@") {
		db = strings.Split(db, "@")[0]
	}

	db = strings.ReplaceAll(db, "-", "_")
	db = strings.ReplaceAll(db, ".", "_")
	db = strings.ReplaceAll(db, " ", "_")

	db += "_db"

	var query string
	if p.GetStoreType() == "MONGO" {
		query = `{ "notificationtype":` + Utility.ToString(notificationType) + `}`
	} else if p.GetStoreType() == "SCYLLA" || p.GetStoreType() == "SQL" {
		query = `SELECT * FROM Notifications`
	} else {
		return nil, errors.New("unknown database type " + p.GetStoreType())
	}

	err = p.Delete(context.Background(), "local_resource", db, "Notifications", query, ``)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Send event to all concern client.
	domain, _ := config.GetDomain()
	evt_client, err := GetEventClient(domain)
	if err == nil {
		evt := rqst.Recipient + "_clear_user_notifications_evt"
		evt_client.Publish(evt, []byte{})
	}

	return &resourcepb.ClearNotificationsByTypeRsp{}, nil
}

/////////////////////////////////////////////////////////////////////////////////////////
// Pakage informations...
/////////////////////////////////////////////////////////////////////////////////////////

// Find packages by keywords...
func (srv *server) FindPackages(ctx context.Context, rqst *resourcepb.FindPackagesDescriptorRequest) (*resourcepb.FindPackagesDescriptorResponse, error) {
	// That service made user of persistence service.
	p, err := srv.getPersistenceStore()
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

	var query string
	if p.GetStoreType() == "MONGO" {
		query = `{"keywords": { "$all" : ` + kewordsStr + `}}`
	} else if p.GetStoreType() == "SCYLLA" {
		query = `SELECT * FROM Packages WHERE keywords='` + kewordsStr + `' ALLOW FILTERING`
	} else if p.GetStoreType() == "SQL" {
		query = `SELECT * FROM Packages WHERE keywords='` + kewordsStr + `'`
	} else {
		return nil, errors.New("unknown database type " + p.GetStoreType())
	}

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
		descriptors[i].TypeName = "PackageDescriptor"
		descriptors[i].Id = descriptor["_id"].(string)
		descriptors[i].Name = descriptor["name"].(string)
		descriptors[i].Description = descriptor["description"].(string)
		descriptors[i].PublisherId = descriptor["publisherid"].(string)
		descriptors[i].Version = descriptor["version"].(string)
		descriptors[i].Icon = descriptor["icon"].(string)
		descriptors[i].Alias = descriptor["alias"].(string)
		if descriptor["keywords"] != nil {

			var keywords []interface{}
			switch descriptor["keywords"].(type) {
			case primitive.A:
				keywords = []interface{}(descriptor["keywords"].(primitive.A))
			case []interface{}:
				keywords = descriptor["keywords"].([]interface{})
			}

			descriptors[i].Keywords = make([]string, len(keywords))
			for j := 0; j < len(keywords); j++ {
				descriptors[i].Keywords[j] = keywords[j].(string)
			}
		}
		if descriptor["actions"] != nil {

			var actions []interface{}
			switch descriptor["actions"].(type) {
			case primitive.A:
				actions = []interface{}(descriptor["actions"].(primitive.A))
			case []interface{}:
				actions = descriptor["actions"].([]interface{})
			}

			descriptors[i].Actions = make([]string, len(actions))
			for j := 0; j < len(actions); j++ {
				descriptors[i].Actions[j] = actions[j].(string)
			}
		}
		if descriptor["discoveries"] != nil {

			var discoveries []interface{}
			switch descriptor["discoveries"].(type) {
			case primitive.A:
				discoveries = []interface{}(descriptor["discoveries"].(primitive.A))
			case []interface{}:
				discoveries = descriptor["discoveries"].([]interface{})
			}

			descriptors[i].Discoveries = make([]string, len(discoveries))
			for j := 0; j < len(discoveries); j++ {
				descriptors[i].Discoveries[j] = discoveries[j].(string)
			}
		}

		if descriptor["repositories"] != nil {

			var repositories []interface{}
			switch descriptor["repositories"].(type) {
			case primitive.A:
				repositories = []interface{}(descriptor["repositories"].(primitive.A))
			case []interface{}:
				repositories = descriptor["repositories"].([]interface{})
			}

			descriptors[i].Repositories = make([]string, len(repositories))
			for j := 0; j < len(repositories); j++ {
				descriptors[i].Repositories[j] = repositories[j].(string)
			}
		}
	}

	// Return the list of Service Descriptor.
	return &resourcepb.FindPackagesDescriptorResponse{
		Results: descriptors,
	}, nil
}

// * Retrun all version of a given packages. *
func (srv *server) GetPackageDescriptor(ctx context.Context, rqst *resourcepb.GetPackageDescriptorRequest) (*resourcepb.GetPackageDescriptorResponse, error) {

	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	var query string
	if p.GetStoreType() == "MONGO" {
		query = `{"name":"` + rqst.ServiceId + `", "publisherid":"` + rqst.PublisherId + `"}`
	} else if p.GetStoreType() == "SCYLLA" {
		query = `SELECT * FROM Packages WHERE name='` + rqst.ServiceId + `' AND publisherid='` + rqst.PublisherId + `' ALLOW FILTERING`
	} else if p.GetStoreType() == "SQL" {
		query = `SELECT * FROM Packages WHERE name='` + rqst.ServiceId + `' AND publisherid='` + rqst.PublisherId + `'`
	} else {
		return nil, errors.New("unknown database type " + p.GetStoreType())
	}

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
		descriptors[i].TypeName = "PackageDescriptor"
		descriptors[i].Id = descriptor["_id"].(string)
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

			var keywords []interface{}
			switch descriptor["keywords"].(type) {
			case primitive.A:
				keywords = []interface{}(descriptor["keywords"].(primitive.A))
			case []interface{}:
				keywords = descriptor["keywords"].([]interface{})
			}

			descriptors[i].Keywords = make([]string, len(keywords))
			for j := 0; j < len(keywords); j++ {
				descriptors[i].Keywords[j] = keywords[j].(string)
			}
		}

		if descriptor["actions"] != nil {
			var actions []interface{}
			switch descriptor["actions"].(type) {
			case primitive.A:
				actions = []interface{}(descriptor["actions"].(primitive.A))
			case []interface{}:
				actions = descriptor["actions"].([]interface{})
			}

			descriptors[i].Actions = make([]string, len(actions))
			for j := 0; j < len(actions); j++ {
				descriptors[i].Actions[j] = actions[j].(string)
			}
		}

		if descriptor["discoveries"] != nil {

			var discoveries []interface{}
			switch descriptor["discoveries"].(type) {
			case primitive.A:
				discoveries = []interface{}(descriptor["discoveries"].(primitive.A))
			case []interface{}:
				discoveries = descriptor["discoveries"].([]interface{})
			}

			descriptors[i].Discoveries = make([]string, len(discoveries))
			for j := 0; j < len(discoveries); j++ {
				descriptors[i].Discoveries[j] = discoveries[j].(string)
			}
		}

		if descriptor["repositories"] != nil {

			var repositories []interface{}
			switch descriptor["repositories"].(type) {
			case primitive.A:
				repositories = []interface{}(descriptor["repositories"].(primitive.A))
			case []interface{}:
				repositories = descriptor["repositories"].([]interface{})
			}
			descriptors[i].Repositories = make([]string, len(repositories))
			for j := 0; j < len(repositories); j++ {
				descriptors[i].Repositories[j] = repositories[j].(string)
			}
		}

		if descriptor["groups"] != nil {
			var groups []interface{}
			switch descriptor["groups"].(type) {
			case primitive.A:
				groups = []interface{}(descriptor["groups"].(primitive.A))
			case []interface{}:
				groups = descriptor["groups"].([]interface{})
			}

			descriptors[i].Groups = make([]*resourcepb.Group, len(groups))

			for j := 0; j < len(groups); j++ {
				groupId := groups[j].(map[string]interface{})["$id"].(string)
				g, err := srv.getGroup(groupId)
				if err == nil {
					descriptors[i].Groups[j] = g
				}
			}
		}

		if descriptor["roles"] != nil {

			var roles []interface{}
			switch descriptor["roles"].(type) {
			case primitive.A:
				roles = []interface{}(descriptor["roles"].(primitive.A))
			case []interface{}:
				roles = descriptor["roles"].([]interface{})
			}

			descriptors[i].Roles = make([]*resourcepb.Role, len(roles))

			for j := 0; j < len(roles); j++ {

				// Get the role id.
				roleId := roles[j].(map[string]interface{})["$id"].(string)

				// Get the role.
				role_, err := srv.getRole(roleId)
				if err == nil {
					// set it back in the package descriptor.
					descriptors[i].Roles[j] = role_
				}
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
func (srv *server) GetPackagesDescriptor(rqst *resourcepb.GetPackagesDescriptorRequest, stream resourcepb.ResourceService_GetPackagesDescriptorServer) error {
	p, err := srv.getPersistenceStore()
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
		descriptor.TypeName = "PackageDescriptor"
		descriptor.Id = data[i].(map[string]interface{})["_id"].(string)
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

			var keywords []interface{}
			switch data[i].(map[string]interface{})["keywords"].(type) {
			case primitive.A:
				keywords = []interface{}(data[i].(map[string]interface{})["keywords"].(primitive.A))
			case []interface{}:
				keywords = data[i].(map[string]interface{})["keywords"].([]interface{})
			}

			descriptor.Keywords = make([]string, len(keywords))
			for j := 0; j < len(keywords); j++ {
				descriptor.Keywords[j] = keywords[j].(string)
			}
		}

		if data[i].(map[string]interface{})["actions"] != nil {

			var actions []interface{}
			switch data[i].(map[string]interface{})["actions"].(type) {
			case primitive.A:
				actions = []interface{}(data[i].(map[string]interface{})["actions"].(primitive.A))
			case []interface{}:
				actions = data[i].(map[string]interface{})["actions"].([]interface{})
			}

			descriptor.Actions = make([]string, len(actions))
			for j := 0; j < len(actions); j++ {
				descriptor.Actions[j] = actions[j].(string)
			}
		}

		if data[i].(map[string]interface{})["discoveries"] != nil {

			var discoveries []interface{}
			switch data[i].(map[string]interface{})["discoveries"].(type) {
			case primitive.A:
				discoveries = []interface{}(data[i].(map[string]interface{})["discoveries"].(primitive.A))
			case []interface{}:
				discoveries = data[i].(map[string]interface{})["discoveries"].([]interface{})
			}

			descriptor.Discoveries = make([]string, len(discoveries))
			for j := 0; j < len(discoveries); j++ {
				descriptor.Discoveries[j] = discoveries[j].(string)
			}
		}

		if data[i].(map[string]interface{})["repositories"] != nil {

			var repositories []interface{}
			switch data[i].(map[string]interface{})["repositories"].(type) {
			case primitive.A:
				repositories = []interface{}(data[i].(map[string]interface{})["repositories"].(primitive.A))
			case []interface{}:
				repositories = data[i].(map[string]interface{})["repositories"].([]interface{})
			}

			descriptor.Repositories = make([]string, len(repositories))
			for j := 0; j < len(repositories); j++ {
				descriptor.Repositories[j] = repositories[j].(string)
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
func (srv *server) SetPackageDescriptor(ctx context.Context, rqst *resourcepb.SetPackageDescriptorRequest) (*resourcepb.SetPackageDescriptorResponse, error) {

	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	var q string
	if p.GetStoreType() == "MONGO" {
		q = `{"name":"` + rqst.PackageDescriptor.Name + `", "publisherid":"` + rqst.PackageDescriptor.PublisherId + `", "version":"` + rqst.PackageDescriptor.Version + `"}`
	} else if p.GetStoreType() == "SCYLLA" {
		q = `SELECT * FROM Packages WHERE name='` + rqst.PackageDescriptor.Name + `' AND publisherid='` + rqst.PackageDescriptor.PublisherId + `' AND version='` + rqst.PackageDescriptor.Version + `' ALLOW FILTERING`
	} else if p.GetStoreType() == "SQL" {
		q = `SELECT * FROM Packages WHERE name='` + rqst.PackageDescriptor.Name + `' AND publisherid='` + rqst.PackageDescriptor.PublisherId + `' AND version='` + rqst.PackageDescriptor.Version + `'`
	} else {
		return nil, errors.New("unknown database type " + p.GetStoreType())
	}

	rqst.PackageDescriptor.TypeName = "PackageDescriptor"
	rqst.PackageDescriptor.Id = Utility.GenerateUUID(rqst.PackageDescriptor.PublisherId + "%" + rqst.PackageDescriptor.Name + "%" + rqst.PackageDescriptor.Version)

	for i := 0; i < len(rqst.PackageDescriptor.Groups); i++ {
		rqst.PackageDescriptor.Groups[i].TypeName = "Group"
	}

	for i := 0; i < len(rqst.PackageDescriptor.Roles); i++ {
		rqst.PackageDescriptor.Roles[i].TypeName = "Role"
	}

	jsonStr, err := json.Marshal(rqst.PackageDescriptor)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// little fix...
	jsonStr_ := strings.ReplaceAll(string(jsonStr), "publisherId", "publisherid")

	// Always create a new if not already exist.
	err = p.ReplaceOne(context.Background(), "local_resource", "local_resource", "Packages", q, jsonStr_, `[{"upsert": true}]`)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	count, err := p.Count(context.Background(), "local_resource", "local_resource", "Packages", q, "")
	if count == 0 || err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("unable to create the package descriptor")))

	}

	return &resourcepb.SetPackageDescriptorResponse{
		Result: true,
	}, nil
}

// * Get the package bundle checksum use for validation *
func (srv *server) GetPackageBundleChecksum(ctx context.Context, rqst *resourcepb.GetPackageBundleChecksumRequest) (*resourcepb.GetPackageBundleChecksumResponse, error) {
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	q := `{"_id":"` + rqst.Id + `"}`

	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Bundles", q, "")
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
func (srv *server) SetPackageBundle(ctx context.Context, rqst *resourcepb.SetPackageBundleRequest) (*resourcepb.SetPackageBundleResponse, error) {
	bundle := rqst.Bundle

	p, err := srv.getPersistenceStore()
	if err != nil {
		srv.logServiceError("SetPackageBundle", Utility.FileLine(), Utility.FunctionName(), err.Error())
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Generate the bundle id....
	id := Utility.GenerateUUID(bundle.PackageDescriptor.PublisherId + "%" + bundle.PackageDescriptor.Name + "%" + bundle.PackageDescriptor.Version + "%" + bundle.PackageDescriptor.Id + "%" + bundle.Plaform)

	jsonStr, err := Utility.ToJson(map[string]interface{}{"_id": id, "checksum": bundle.Checksum, "platform": bundle.Plaform, "publisherid": bundle.PackageDescriptor.PublisherId, "servicename": bundle.PackageDescriptor.Name, "serviceid": bundle.PackageDescriptor.Id, "modified": bundle.Modified, "size": bundle.Size})
	if err != nil {
		srv.logServiceError("SetPackageBundle", Utility.FileLine(), Utility.FunctionName(), err.Error())
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	q := `{"_id":"` + id + `"}`

	err = p.ReplaceOne(context.Background(), "local_resource", "local_resource", "Bundles", q, jsonStr, `[{"upsert": true}]`)
	if err != nil {
		srv.logServiceError("SetPackageBundle", Utility.FileLine(), Utility.FunctionName(), err.Error())
		return nil, err
	}
	return &resourcepb.SetPackageBundleResponse{Result: true}, nil
}

/////////////////////////////////////////////////////////////////////////////////////////
// Session
/////////////////////////////////////////////////////////////////////////////////////////

func (srv *server) updateSession(accountId string, state resourcepb.SessionState, last_session_time, expire_at int64) error {

	expiration := time.Unix(expire_at, 0)
	delay := time.Until(expiration)
	if state != resourcepb.SessionState_OFFLINE {
		if expiration.Before(time.Now()) {
			return errors.New("session is already expired " + expiration.Local().String() + " " + Utility.ToString(math.Floor(delay.Minutes())) + ` minutes ago`)
		}
	}

	p, err := srv.getPersistenceStore()
	if err != nil {
		return err
	}

	// Log a message to display update session...
	//srv.logServiceInfo("updateSession", Utility.FileLine(), Utility.FunctionName(), "update session for user "+accountId+" last_session_time: "+time.Unix(last_session_time, 0).Local().String()+" expire_at: "+time.Unix(expire_at, 0).Local().String())
	session := map[string]interface{}{"_id": Utility.ToString(last_session_time), "domain": srv.Domain, "accountId": accountId, "expire_at": expire_at, "last_state_time": last_session_time, "state": state}
	jsonStr, err := Utility.ToJson(session)
	if err != nil {
		return err
	}

	// send update_session event
	var q string
	if p.GetStoreType() == "MONGO" {
		q = `{"accountId":"` + accountId + `"}`
	} else if p.GetStoreType() == "SCYLLA" {
		q = `SELECT * FROM Sessions WHERE accountId='` + accountId + `' ALLOW FILTERING`
	} else if p.GetStoreType() == "SQL" {
		session["_id"] = Utility.RandomUUID() // set a random id for sql db.
		q = `SELECT * FROM Sessions WHERE accountId='` + accountId + `'`
	} else {
		return errors.New("unknown database type " + p.GetStoreType())
	}

	return p.ReplaceOne(context.Background(), "local_resource", "local_resource", "Sessions", q, jsonStr, `[{"upsert":true}]`)

}

// * Update user session informations
func (srv *server) UpdateSession(ctx context.Context, rqst *resourcepb.UpdateSessionRequest) (*resourcepb.UpdateSessionResponse, error) {

	err := srv.updateSession(rqst.Session.AccountId, rqst.Session.State, rqst.Session.LastStateTime, rqst.Session.ExpireAt)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &resourcepb.UpdateSessionResponse{}, nil
}

// * Remove session
func (srv *server) RemoveSession(ctx context.Context, rqst *resourcepb.RemoveSessionRequest) (*resourcepb.RemoveSessionResponse, error) {
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	var q string
	if p.GetStoreType() == "MONGO" {
		q = `{"accountId":"` + rqst.AccountId + `"}`
	} else if p.GetStoreType() == "SCYLLA" {
		q = `SELECT * FROM Sessions WHERE accountId='` + rqst.AccountId + `' ALLOW FILTERING`
	} else if p.GetStoreType() == "SQL" {
		q = `SELECT * FROM Sessions WHERE accountId='` + rqst.AccountId + `'`
	} else {
		return nil, errors.New("unknown database type " + p.GetStoreType())
	}

	// Now I will remove the token...
	err = p.Delete(context.Background(), "local_resource", "local_resource", "Sessions", q, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &resourcepb.RemoveSessionResponse{}, nil
}

func (srv *server) GetSessions(ctx context.Context, rqst *resourcepb.GetSessionsRequest) (*resourcepb.GetSessionsResponse, error) {
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	query := rqst.Query
	if len(query) == 0 {
		query = "{}"
	} else {
		if p.GetStoreType() == "SQL" {
			paremeters := make(map[string]interface{})
			err := json.Unmarshal([]byte(query), &paremeters)
			if err != nil {
				return nil, status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}

			query = `SELECT * FROM Sessions WHERE `
			if paremeters["state"] != nil {
				query += ` state=` + Utility.ToString(paremeters["state"])
			}

		}
	}

	sessions, err := p.Find(context.Background(), "local_resource", "local_resource", "Sessions", query, rqst.Options)
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
		state := int32(Utility.ToInt(session["state"]))
		sessions_ = append(sessions_, &resourcepb.Session{AccountId: session["accountId"].(string), ExpireAt: int64(expireAt), LastStateTime: int64(lastStateTime), State: resourcepb.SessionState(state)})
	}

	return &resourcepb.GetSessionsResponse{
		Sessions: sessions_,
	}, nil
}

func (srv *server) getSession(accountId string) (*resourcepb.Session, error) {
	p, err := srv.getPersistenceStore()
	if err != nil {
		return nil, err
	}

	var q string
	if p.GetStoreType() == "MONGO" {
		q = `{"accountId":"` + accountId + `"}`
	} else if p.GetStoreType() == "SCYLLA" {
		q = `SELECT * FROM Sessions WHERE accountId='` + accountId + `' ALLOW FILTERING`
	} else if p.GetStoreType() == "SQL" {
		q = `SELECT * FROM Sessions WHERE accountId='` + accountId + `'`
	} else {
		return nil, errors.New("unknown database type " + p.GetStoreType())
	}

	// Now I will remove the token...
	session_, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Sessions", q, "")
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
		state = resourcepb.SessionState(int32(Utility.ToInt(session["state"])))
	}

	return &resourcepb.Session{AccountId: session["accountId"].(string), ExpireAt: int64(expireAt), LastStateTime: int64(lastStateTime), State: state}, nil
}

// * Return a session for a given user
func (srv *server) GetSession(ctx context.Context, rqst *resourcepb.GetSessionRequest) (*resourcepb.GetSessionResponse, error) {

	fmt.Println("get session for ", rqst.AccountId)

	// Now I will remove the token...
	session, err := srv.getSession(rqst.AccountId)
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
func (srv *server) GetCallHistory(ctx context.Context, rqst *resourcepb.GetCallHistoryRqst) (*resourcepb.GetCallHistoryRsp, error) {
	p, err := srv.getPersistenceStore()
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
	db = strings.ReplaceAll(db, "-", "_")
	db = strings.ReplaceAll(db, ".", "_")
	db = strings.ReplaceAll(db, " ", "_")

	db += "_db"

	var query string
	if p.GetStoreType() == "MONGO" {
		query = `{"$or":[{"caller":"` + rqst.AccountId + `"},{"callee":"` + rqst.AccountId + `"} ]}`
	} else if p.GetStoreType() == "SCYLLA" {
		query = `SELECT * FROM Calls WHERE caller='` + rqst.AccountId + `' OR callee='` + rqst.AccountId + `' ALLOW FILTERING`
	} else if p.GetStoreType() == "SQL" {
		query = `SELECT * FROM Calls WHERE caller='` + rqst.AccountId + `' OR callee='` + rqst.AccountId + `'`
	} else {
		return nil, errors.New("unknown database type " + p.GetStoreType())
	}

	results, err := p.Find(context.Background(), "local_resource", db, "calls", query, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	calls := make([]*resourcepb.Call, len(results))
	for i := 0; i < len(results); i++ {
		call := results[i].(map[string]interface{})
		startTime := Utility.ToInt(call["start_time"])
		endTime := Utility.ToInt(call["end_time"])

		calls[i] = &resourcepb.Call{Caller: call["caller"].(string), Callee: call["callee"].(string), Uuid: call["_id"].(string), StartTime: int64(startTime), EndTime: int64(endTime)}
	}

	return &resourcepb.GetCallHistoryRsp{Calls: calls}, nil
}

func (srv *server) setCall(accountId string, call *resourcepb.Call) error {

	// Get the persistence connection
	p, err := srv.getPersistenceStore()
	if err != nil {
		return err
	}

	// set the caller id.
	db := accountId
	db = strings.ReplaceAll(strings.ReplaceAll(db, ".", "_"), "@", "_")
	db = strings.ReplaceAll(db, "-", "_")
	db = strings.ReplaceAll(db, ".", "_")
	db = strings.ReplaceAll(db, " ", "_")

	db += "_db"

	// rename the uuid to _id (for mongo identifier)
	call_ := map[string]interface{}{"caller": call.Caller, "callee": call.Callee, "_id": call.Uuid, "start_time": call.StartTime, "end_time": call.EndTime}
	jsonStr, _ := Utility.ToJson(call_)

	q := `{"_id":"` + call.Uuid + `"}`

	err = p.ReplaceOne(context.Background(), "local_resource", db, "calls", q, jsonStr, `[{"upsert":true}]`)
	if err != nil {
		return err
	}

	return nil
}

// * Set calling information *
func (srv *server) SetCall(ctx context.Context, rqst *resourcepb.SetCallRqst) (*resourcepb.SetCallRsp, error) {

	// Get the persistence connection
	if strings.Contains(rqst.Call.Caller, "@") {
		domain := strings.Split(rqst.Call.Caller, "@")[1]
		localDomain, _ := config.GetDomain()
		if domain == localDomain {
			err := srv.setCall(strings.Split(rqst.Call.Caller, "@")[0], rqst.Call)
			if err != nil {
				return nil, status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}
		}
	} else {
		err := srv.setCall(rqst.Call.Caller, rqst.Call)
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
			err := srv.setCall(strings.Split(rqst.Call.Callee, "@")[0], rqst.Call)
			if err != nil {
				return nil, status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}
		}
	} else {
		err := srv.setCall(rqst.Call.Callee, rqst.Call)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	return &resourcepb.SetCallRsp{}, nil
}

func (srv *server) deleteCall(account_id, uuid string) error {
	p, err := srv.getPersistenceStore()
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
	db = strings.ReplaceAll(db, "-", "_")
	db = strings.ReplaceAll(db, ".", "_")
	db = strings.ReplaceAll(db, " ", "_")

	db += "_db"

	q := `{"_id":"` + uuid + `"}`

	err = p.DeleteOne(context.Background(), "local_resource", db, "calls", q, "")
	if err != nil {
		return err
	}

	return nil
}

// * Delete a calling infos *
func (srv *server) DeleteCall(ctx context.Context, rqst *resourcepb.DeleteCallRqst) (*resourcepb.DeleteCallRsp, error) {

	err := srv.deleteCall(rqst.AccountId, rqst.Uuid)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &resourcepb.DeleteCallRsp{}, nil
}

// * Clear Call's *
func (srv *server) ClearCalls(ctx context.Context, rqst *resourcepb.ClearCallsRqst) (*resourcepb.ClearCallsRsp, error) {

	p, err := srv.getPersistenceStore()
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
	db = strings.ReplaceAll(db, "-", "_")
	db = strings.ReplaceAll(db, ".", "_")
	db = strings.ReplaceAll(db, " ", "_")

	db += "_db"

	query := rqst.Filter
	if len(query) == 0 {
		query = "{}"
	}

	results, err := p.Find(context.Background(), "local_resource", db, "calls", query, "")
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Delete the call.
	for i := 0; i < len(results); i++ {
		call := results[i].(map[string]interface{})
		srv.deleteCall(rqst.AccountId, call["_id"].(string))
	}

	return &resourcepb.ClearCallsRsp{}, nil
}
