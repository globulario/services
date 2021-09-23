package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"io/ioutil"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/authentication/authenticationpb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/resource/resourcepb"
	"github.com/globulario/services/golang/security"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var (
	dataPath   = config.GetDataDir()
	configPath = config.GetConfigDir() + "/config.json"
	tokensPath = config.GetConfigDir() + "/tokens"
)

//* Validate a token *
func (server *server) ValidateToken(ctx context.Context, rqst *authenticationpb.ValidateTokenRqst) (*authenticationpb.ValidateTokenRsp, error) {

	id, _, _, _, expireAt, err := security.ValidateToken(rqst.Token)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &authenticationpb.ValidateTokenRsp{
		ClientId: id,
		Expired:  expireAt,
	}, nil
}

//* Refresh token get a new token *
func (server *server) RefreshToken(ctx context.Context, rqst *authenticationpb.RefreshTokenRqst) (*authenticationpb.RefreshTokenRsp, error) {

	// first of all I will validate the current token.
	id, name, email, issuer, expireAt, err := security.ValidateToken(rqst.Token)

	if err != nil {
		if !strings.HasPrefix(err.Error(), "token is expired") {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	// If the token is older than seven day without being refresh then I retrun an error.
	if time.Unix(expireAt, 0).Before(time.Now().AddDate(0, 0, -7)) {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("the token cannot be refresh after 7 day")))
	}

	key, err := security.GetPeerKey(issuer)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	tokenString, err := security.GenerateToken(key, time.Duration(server.SessionTimeout), Utility.MyMacAddr(), id, name, email)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Here I will refresh the existing token file.
	if id == "sa" {
		// So here I will keep the token...
		ioutil.WriteFile(tokensPath+"/"+server.Domain+"_token", []byte(tokenString), 0644)
	} else {
		// get the active session.
		session, err := server.getSession(id)
		if err != nil {
			session = new(resourcepb.Session)
			session.AccountId = id
			session.State = resourcepb.SessionState_ONLINE
		}

		// get back the new expireAt
		_, _, _, _, expireAt, _ = security.ValidateToken(tokenString)
		session.ExpireAt = expireAt

		server.logServiceInfo("RefreshToken", Utility.FileLine(), Utility.FunctionName(), "token expireAt: "+time.Unix(expireAt, 0).Local().String()+" actual time is "+time.Now().Local().String())
		// save the session in the backend.
		err = server.updateSession(session)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	// return the token string.
	return &authenticationpb.RefreshTokenRsp{
		Token: tokenString,
	}, nil
}

//* Set the account password *
func (server *server) SetPassword(ctx context.Context, rqst *authenticationpb.SetPasswordRequest) (*authenticationpb.SetPasswordResponse, error) {

	// Here I will get the account info.
	account, err := server.getAccount(rqst.AccountId)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Now I will validate the password received with the one in the account
	err = server.validatePassword(rqst.OldPassword, account.Password)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Now I will update the account...
	err = server.changeAccountPassword(rqst.AccountId, rqst.OldPassword, rqst.NewPassword)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// finaly I will call authenticate to generate the token string and set it at return...
	tokenString, err := server.authenticate(account.Id, rqst.NewPassword)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Set the password.
	return &authenticationpb.SetPasswordResponse{
		Token: tokenString,
	}, nil
}

// Set the root password, the root password will be save in the configuration file.
func (server *server) SetRootPassword(ctx context.Context, rqst *authenticationpb.SetRootPasswordRequest) (*authenticationpb.SetRootPasswordResponse, error) {

	// The root password will be
	if !Utility.Exists(configPath) {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no configuration found at "+`"`+configPath+`"`)))
	}

	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	config := make(map[string]interface{})
	err = json.Unmarshal(data, &config)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Now I go Globular config file I will get the password.
	password := config["RootPassword"].(string)

	// adminadmin is the default password...
	if password == "adminadmin" {
		if rqst.OldPassword != password {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("the given password dosent match existing one")))
		}

		// In that case I will simply hash the new given password.
		password, err = server.hashPassword(rqst.NewPassword)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	config["RootPassword"] = password
	jsonStr, err := Utility.ToJson(config)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = ioutil.WriteFile(configPath, []byte(jsonStr), 0644)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	key, err := security.GetLocalKey()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// The token string
	tokenString, err := security.GenerateToken(key, time.Duration(server.SessionTimeout), Utility.MyMacAddr(), "sa", "sa", config["AdminEmail"].(string))
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Generate a token...
	return &authenticationpb.SetRootPasswordResponse{
		Token: tokenString,
	}, nil
}

//Set the root email
func (server *server) SetRootEmail(ctx context.Context, rqst *authenticationpb.SetRootEmailRequest) (*authenticationpb.SetRootEmailResponse, error) {

	// The root password will be
	if !Utility.Exists(configPath) {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no configuration found at "+`"`+configPath+`"`)))
	}

	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	config := make(map[string]interface{})
	err = json.Unmarshal(data, &config)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Now I go Globular config file I will get the password.
	email := config["AdminEmail"].(string)
	if email != rqst.OldEmail {

		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("the given email dosent match existing one")))

	}

	config["AdminEmail"] = rqst.NewEmail
	jsonStr, err := Utility.ToJson(config)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	err = ioutil.WriteFile(configPath, []byte(jsonStr), 0644)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &authenticationpb.SetRootEmailResponse{}, nil
}

/**
 * This will set peer private and public key. The keys will by save
 * in the keypath.
 */
func (server *server) setKey(mac string) error {

	// Now I will generate keys if not already exist.
	if Utility.MyMacAddr() == mac {

		return security.GeneratePeerKeys(mac)

	}

	return nil
}

/* Authenticate a user */
func (server *server) authenticate(accountId, pwd string) (string, error) {
	fmt.Println("server authenticate rqst ", time.Now().Unix())
	key, err := security.GetLocalKey()
	if err != nil {
		return "", status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// If the user is the root...
	if accountId == "sa" {
		// The root password will be
		if !Utility.Exists(configPath) {
			return "", status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no configuration found at "+`"`+configPath+`"`)))
		}

		data, err := ioutil.ReadFile(configPath)
		if err != nil {
			return "", status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		config := make(map[string]interface{})
		err = json.Unmarshal(data, &config)
		if err != nil {
			return "", status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		// Now I go Globular config file I will get the password.
		password := config["RootPassword"].(string)

		// adminadmin is the default password...
		if password == "adminadmin" {
			if pwd != password {
				return "", status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("the given password dosent match existing one")))
			}
		} else {
			// Now I will validate the password received with the one in the account
			err = server.validatePassword(pwd, password)
			if err != nil {
				return "", err
			}
		}

		tokenString, err := security.GenerateToken(key, time.Duration(server.SessionTimeout), Utility.MyMacAddr(), "sa", "sa", config["AdminEmail"].(string))
		if err != nil {
			return "", status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		// So here I will keep the token...
		ioutil.WriteFile(tokensPath+"/"+server.Domain+"_token", []byte(tokenString), 0644)
		fmt.Println("server authenticate response ", time.Now().Unix())
		
		return tokenString, nil
	}

	// Here I will get the account info.
	account, err := server.getAccount(accountId)
	if err != nil {
		return "", err
	}

	// Now I will validate the password received with the one in the account
	err = server.validatePassword(pwd, account.Password)
	if err != nil {
		return "", err
	}

	// Now I will create the session and generate it token.
	session := new(resourcepb.Session)
	session.AccountId = account.Id

	// The token string
	tokenString, err := security.GenerateToken(key, time.Duration(server.SessionTimeout), Utility.MyMacAddr(), account.Id, account.Name, account.Email)
	if err != nil {
		return "", err
	}

	// get the expire time.
	_, user, email, _, expireAt, _ := security.ValidateToken(tokenString)
	defer server.logServiceInfo("Authenticate", Utility.FileLine(), Utility.FunctionName(), "user "+user+":"+email+" successfuly authenticaded token is valid for "+Utility.ToString(server.SessionTimeout/1000/60)+" minutes from now.")

	// Create the user file directory.
	path := "/users/" + user
	Utility.CreateDirIfNotExist(dataPath + "/files" + path)
	server.addResourceOwner(path, user, rbacpb.SubjectType_ACCOUNT)

	session.ExpireAt = expireAt
	session.State = resourcepb.SessionState_ONLINE
	session.LastStateTime = time.Now().Unix()
	fmt.Println("server authenticate response ", time.Now().Unix())
	err = server.updateSession(session)
	if err != nil {
		return "", err
	}

	fmt.Println("server authenticate response ", time.Now().Unix())
	return tokenString, nil
}

//* Authenticate a user *
func (server *server) Authenticate(ctx context.Context, rqst *authenticationpb.AuthenticateRqst) (*authenticationpb.AuthenticateRsp, error) {
	server.logServiceInfo("Authenticate", Utility.FileLine(), Utility.FunctionName(), "user "+rqst.Name+" try to connect")

	tokenString, err := server.authenticate(rqst.Name, rqst.Password)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &authenticationpb.AuthenticateRsp{
		Token: tokenString,
	}, nil
}
