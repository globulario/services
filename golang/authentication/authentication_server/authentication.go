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
	"github.com/globulario/services/golang/authentication/authentication_client"
	"github.com/globulario/services/golang/authentication/authenticationpb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/resource/resource_client"
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

	claims, err := security.ValidateToken(rqst.Token)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &authenticationpb.ValidateTokenRsp{
		ClientId: claims.Id,
		Expired:  claims.StandardClaims.ExpiresAt,
	}, nil
}

//* Refresh token get a new token *
func (server *server) RefreshToken(ctx context.Context, rqst *authenticationpb.RefreshTokenRqst) (*authenticationpb.RefreshTokenRsp, error) {

	// first of all I will validate the current token.
	claims, err := security.ValidateToken(rqst.Token)

	if err != nil {
		if !strings.HasPrefix(err.Error(), "token is expired") {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	// If the token is older than seven day without being refresh then I retrun an error.
	if time.Unix(claims.StandardClaims.ExpiresAt, 0).Before(time.Now().AddDate(0, 0, -7)) {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("the token cannot be refresh after 7 day")))
	}

	key, err := security.GetPeerKey(claims.Issuer)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	tokenString, err := security.GenerateToken(key, server.SessionTimeout, Utility.MyMacAddr(), claims.Id, claims.Username, claims.Email)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// get the active session.
	session, err := server.getSession(claims.Id)
	if err != nil {
		session = new(resourcepb.Session)
		session.AccountId = claims.Id
		session.State = resourcepb.SessionState_ONLINE
	}

	// get back the new expireAt
	claims, _ = security.ValidateToken(tokenString)
	session.ExpireAt = claims.StandardClaims.ExpiresAt

	// server.logServiceInfo("RefreshToken", Utility.FileLine(), Utility.FunctionName(), "token expireAt: "+time.Unix(expireAt, 0).Local().String()+" actual time is "+time.Now().Local().String())
	// save the session in the backend.
	err = server.updateSession(session)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
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
	tokenString, err := security.GenerateToken(key, server.SessionTimeout, Utility.MyMacAddr(), "sa", "sa", config["AdminEmail"].(string))
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

		tokenString, err := security.GenerateToken(key, server.SessionTimeout, Utility.MyMacAddr(), "sa", "sa", config["AdminEmail"].(string))
		if err != nil {
			return "", status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		// Create the user file directory.
		path := "/users/sa"
		Utility.CreateDirIfNotExist(dataPath + "/files" + path)
		server.addResourceOwner(path, "sa", rbacpb.SubjectType_ACCOUNT)

		return tokenString, nil
	}

	// Here I will get the account info.
	account, err := server.getAccount(accountId)
	if err != nil {
		return "", err
	}

	err = server.validatePassword(pwd, account.Password)
	if err != nil {
		// Now if the LDAP service is configure I will try to authenticate with it...
		if len(server.LdapConnectionId) != 0 {
			err := server.authenticateLdap(account.Name, pwd)
			if err != nil {
				return "", err
			}
			// set back the password.
			// the old password can be left blank if the token was generated for sa.
			err = server.changeAccountPassword(account.Id, ``, pwd)
			if err != nil {
				fmt.Println("fail to change password: ", account.Id, err)
				return "", err
			}
		} else {
			return "", err
		}
	}

	// Now I will create the session and generate it token.
	session := new(resourcepb.Session)
	session.AccountId = account.Id

	// The token string
	tokenString, err := security.GenerateToken(key, server.SessionTimeout, Utility.MyMacAddr(), account.Id, account.Name, account.Email)
	if err != nil {
		return "", err
	}

	// get the expire time.
	claims, _ := security.ValidateToken(tokenString)

	// Create the user file directory.
	path := "/users/" + claims.Id
	Utility.CreateDirIfNotExist(dataPath + "/files" + path)
	server.addResourceOwner(path, claims.Id, rbacpb.SubjectType_ACCOUNT)

	session.ExpireAt = claims.StandardClaims.ExpiresAt
	session.State = resourcepb.SessionState_ONLINE
	session.LastStateTime = time.Now().Unix()

	err = server.updateSession(session)
	if err != nil {
		return "", err
	}

	return tokenString, err
}

//* Authenticate a user *
func (server *server) Authenticate(ctx context.Context, rqst *authenticationpb.AuthenticateRqst) (*authenticationpb.AuthenticateRsp, error) {
	server.logServiceInfo("Authenticate", Utility.FileLine(), Utility.FunctionName(), "user "+rqst.Name+" try to connect")

	// Try to authenticate on the server directy...
	tokenString, err := server.authenticate(rqst.Name, rqst.Password)

	// Now I will try each peer...
	if err != nil {
		// I will try to authenticate the peer on other resource service...
		peers, err := server.getPeers()
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		for i := 0; i < len(peers); i++ {
			peer := peers[i]
			resource_client_, err := resource_client.NewResourceService_Client(peer.Address, "resource.ResourceService")
			if err == nil {
				defer resource_client_.Close()
				account, err := resource_client_.GetAccount(rqst.Name)
				if err == nil {
					// an account was found with that name...
					authentication_client_, err := authentication_client.NewAuthenticationService_Client(peer.Address, "authentication.AuthenticationService")
					if err == nil {
						defer authentication_client_.Close()
						tokenString, err := authentication_client_.Authenticate(account.Id, rqst.Password)
						if err == nil {
							return &authenticationpb.AuthenticateRsp{
								Token: tokenString,
							}, nil
						}
					}

				}
			}
		}

		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("fail to authenticate user " + rqst.Name)))
	}

	return &authenticationpb.AuthenticateRsp{
		Token: tokenString,
	}, nil
}
