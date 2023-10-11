package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"io/ioutil"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/authentication/authentication_client"
	"github.com/globulario/services/golang/authentication/authenticationpb"
	"github.com/globulario/services/golang/config"
	config_ "github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/globular_client"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/resource/resource_client"
	"github.com/globulario/services/golang/resource/resourcepb"
	"github.com/globulario/services/golang/security"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

var (
	dataPath   = config.GetDataDir()
	configPath = config.GetConfigDir() + "/config.json"
)

// * Validate a token *
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

// * Refresh token get a new token *
func (server *server) RefreshToken(ctx context.Context, rqst *authenticationpb.RefreshTokenRqst) (*authenticationpb.RefreshTokenRsp, error) {

	// first of all I will validate the current token.
	claims, err := security.ValidateToken(rqst.Token)

	if err != nil {
		if !strings.Contains(err.Error(), "token is expired") {
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

	tokenString, err := security.GenerateToken(server.SessionTimeout, claims.Issuer, claims.Id, claims.Username, claims.Email, claims.UserDomain)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// get the active session.
	session, err := server.getSession(claims.Id)
	if err != nil {
		session = new(resourcepb.Session)
		session.AccountId = claims.Id + "@" + claims.UserDomain
	}

	// set the last session time.
	session.LastStateTime = time.Now().Unix()
	session.State = resourcepb.SessionState_ONLINE

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

// * Set the account password *
func (server *server) SetPassword(ctx context.Context, rqst *authenticationpb.SetPasswordRequest) (*authenticationpb.SetPasswordResponse, error) {
	var token string
	var clientId string

	if md, ok := metadata.FromIncomingContext(ctx); ok {
		token = strings.Join(md["token"], "")

		if len(token) > 0 {
			claims, err := security.ValidateToken(token)
			if err != nil {
				return nil, status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}
			clientId = claims.Id + "@" + claims.UserDomain
		} else {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("application manager SetPassword no token was given")))
		}
	}

	// Here I will get the account info.
	account, err := server.getAccount(rqst.AccountId)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// test if the user is the sa.
	domain, _ := config.GetDomain()

	// The user must be the one who he pretend to be.
	if account.Id+"@"+account.Domain != clientId {
		if clientId != "sa@"+domain {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("you can't change other account password")))
		}
	} else {
		// Now I will validate the password received with the one in the account
		err = server.validatePassword(rqst.OldPassword, account.Password)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	// Now I will update the account...
	err = server.changeAccountPassword(rqst.AccountId, token, rqst.OldPassword, rqst.NewPassword)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	issuer, err := Utility.MyMacAddr(Utility.MyLocalIP())
	if err != nil {
		return nil, err
	}

	// Issue the token for the actual sever.
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		// The application...
		issuer = strings.Join(md["issuer"], "")
	}

	// finaly I will call authenticate to generate the token string and set it at return...
	tokenString, err := server.authenticate(account.Id, rqst.NewPassword, issuer)
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
	var token string
	var clientId string

	if md, ok := metadata.FromIncomingContext(ctx); ok {
		token = strings.Join(md["token"], "")

		if len(token) > 0 {
			claims, err := security.ValidateToken(token)
			if err != nil {
				return nil, status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}
			clientId = claims.Id + "@" + claims.UserDomain
		} else {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("application manager SetPassword no token was given")))
		}
	}

	if clientId != "sa" {
		if !Utility.Exists(configPath) {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("only sa can change root password")))
		}
	}

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
	} else {

		// Here I will get the account info.
		account, err := server.getAccount("sa")
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
	}

	// Now I will update the account...
	err = server.changeAccountPassword("sa", token, rqst.OldPassword, rqst.NewPassword)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	config["RootPassword"] = rqst.NewPassword
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

	macAddress, err := Utility.MyMacAddr(Utility.MyLocalIP())
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	localDomain, _ := config_.GetDomain()

	// The token string
	tokenString, err := security.GenerateToken(server.SessionTimeout, macAddress, "sa", "sa", config["AdminEmail"].(string), localDomain)
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

// Set the root email
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

	// Get the mac address
	macAddress, err := Utility.MyMacAddr(Utility.MyLocalIP())
	if err != nil {
		return err
	}

	// Now I will generate keys if not already exist.
	if macAddress == mac {
		return security.GeneratePeerKeys(mac)
	}

	return nil
}

/* Authenticate a user */
func (server *server) authenticate(accountId, pwd, issuer string) (string, error) {
	fmt.Println("authenticate ", accountId, "issuer", issuer)

	// If the user is the root...
	if accountId == "sa" || strings.HasPrefix(accountId, "sa@") {

		// The root password will be
		if !Utility.Exists(configPath) {
			return "", status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no configuration found at "+`"`+configPath+`"`)))
		}

		data, err := os.ReadFile(configPath)
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
				return "", status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}
		}

		localDomain, _ := config_.GetDomain()
		tokenString, err := security.GenerateToken(server.SessionTimeout, issuer, "sa", "sa", config["AdminEmail"].(string), localDomain)
		if err != nil {
			return "", status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		// Create the user file directory.
		if strings.Contains(accountId, "@") {
			path := "/users/" + accountId
			Utility.CreateDirIfNotExist(dataPath + "/files" + path)
			server.addResourceOwner(path, "file", "sa", rbacpb.SubjectType_ACCOUNT)
		}

		// Be sure the password is correct.
		/*
			err = server.changeAccountPassword(accountId, tokenString, pwd, pwd)
			if err != nil {
				return "", status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}*/

		// set back the password in the config file.
		config["RootPassword"] = pwd
		jsonStr, err := Utility.ToJson(config)
		if err != nil {
			return "", status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		err = ioutil.WriteFile(configPath, []byte(jsonStr), 0644)
		if err != nil {
			return "", status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		// save the password...
		return tokenString, nil
	}

	// Here I will get the account info.
	account, err := server.getAccount(accountId)
	if err != nil {
		return "", err
	}

	err = server.validatePassword(pwd, account.Password)
	if err != nil {
		server.logServiceInfo("Authenticate", Utility.FileLine(), Utility.FunctionName(), err.Error())
		// Now if the LDAP service is configure I will try to authenticate with it...
		if len(server.LdapConnectionId) != 0 {
			err := server.authenticateLdap(account.Name, pwd)
			if err != nil {
				fmt.Println("fail to authenticate with error ", err)
				return "", err
			}
			// set back the password.
			// the old password can be left blank if the token was generated for sa.
			token, err := security.GetLocalToken(server.Mac)
			if err != nil {
				return "", err
			}

			err = server.changeAccountPassword(account.Id, token, "", pwd)
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
	session.AccountId = account.Id + "@" + account.Domain

	// The token string
	tokenString, err := security.GenerateToken(server.SessionTimeout, issuer, account.Id, account.Name, account.Email, account.Domain)
	if err != nil {
		server.logServiceInfo("Authenticate", Utility.FileLine(), Utility.FunctionName(), err.Error())
		return "", err
	}

	// get the expire time.
	claims, _ := security.ValidateToken(tokenString)

	// Create the user file directory.
	Utility.CreateDirIfNotExist(dataPath + "/files/users/" + claims.Id + "@" + account.Domain)

	// be sure the user is the owner of that directory...
	server.addResourceOwner("/users/"+claims.Id+"@"+account.Domain, "file", claims.Id+"@"+account.Domain, rbacpb.SubjectType_ACCOUNT)

	session.ExpireAt = claims.StandardClaims.ExpiresAt
	session.State = resourcepb.SessionState_ONLINE
	session.LastStateTime = time.Now().Unix()

	err = server.updateSession(session)
	if err != nil {
		server.logServiceInfo("Authenticate", Utility.FileLine(), Utility.FunctionName(), err.Error())
		return "", err
	}

	return tokenString, err
}

// Process existing file...
func (server *server) processFiles() {

}

func GetResourceClient(domain string) (*resource_client.Resource_Client, error) {
	Utility.RegisterFunction("NewResourceService_Client", resource_client.NewResourceService_Client)
	client, err := globular_client.GetClient(domain, "resource.ResourceService", "NewResourceService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*resource_client.Resource_Client), nil
}

func GetAuthenticationClient(domain string) (*authentication_client.Authentication_Client, error) {
	Utility.RegisterFunction("NewAuthenticationService_Client", authentication_client.NewAuthenticationService_Client)
	client, err := globular_client.GetClient(domain, "authentication.AuthenticationService", "NewAuthenticationService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*authentication_client.Authentication_Client), nil
}

// * Authenticate a user *
func (server *server) Authenticate(ctx context.Context, rqst *authenticationpb.AuthenticateRqst) (*authenticationpb.AuthenticateRsp, error) {

	mac, err := Utility.MyMacAddr(Utility.MyLocalIP())
	if err != nil {
		return nil, err
	}

	var tokenString string

	// The issuer is the mac address from where the request come from.
	// If the issuer is empty then I will use the mac address of the server.
	// The rqst.Name is the account id, if the account is part of the domain I will try to authenticate it locally.
	if strings.Contains(rqst.Name, "@") {
		domain:= strings.Split(rqst.Name, "@")[1]
		if domain == server.Domain {
			rqst.Issuer = mac
		}
	}

	// Set the mac addresse
	if len(rqst.Issuer) == 0 {
		rqst.Issuer = mac
	} else if rqst.Issuer == mac {
		// Try to authenticate on the server directly...
		tokenString, err = server.authenticate(rqst.Name, rqst.Password, rqst.Issuer)
		if err == nil {
			return &authenticationpb.AuthenticateRsp{
				Token: tokenString,
			}, nil
		}
	}

	// Now I will try each peer...
	if err != nil {
		peers, _ := server.getPeers()
		if len(peers) == 0 {
			uuid := Utility.GenerateUUID(rqst.Name + rqst.Password + rqst.Issuer)

			// no matter what happen the token must be remove...
			defer Utility.RemoveString(server.authentications_, uuid)
			if Utility.Contains(server.authentications_, uuid) {
				return nil, errors.New("fail to authenticate " + rqst.Name + " on " + rqst.Issuer)
			}

			// append the string in the list to cut infinite recursion
			server.authentications_ = append(server.authentications_, uuid)

			// I will try to authenticate the peer on other resource service...
			for i := 0; i < len(peers); i++ {
				peer := peers[i]
				address := peer.Domain
				if peer.Protocol == "https" {
					address += ":" + Utility.ToString(peer.PortHttps)
				} else {
					address += ":" + Utility.ToString(peer.PortHttp)
				}

				resource_client_, err := GetResourceClient(address)
				if err == nil {
					defer resource_client_.Close()
					account, err := resource_client_.GetAccount(rqst.Name)
					if err == nil {
						// an account was found with that name...
						authentication_client_, err := GetAuthenticationClient(address)
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
		}

		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("fail to authenticate user "+rqst.Name+" from "+rqst.Issuer)))
	}

	return &authenticationpb.AuthenticateRsp{
		Token: tokenString,
	}, nil
}

// * Generate a token for a peer with a given mac address *
func (server *server) GeneratePeerToken(ctx context.Context, rqst *authenticationpb.GeneratePeerTokenRequest) (*authenticationpb.GeneratePeerTokenResponse, error) {

	var userId, userName, email, userDomain string

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

			// So here I will
			userId = claims.Id
			userDomain = claims.UserDomain

		} else {
			return nil, errors.New("no token was given")
		}
	}

	// The generated token.
	token, err := security.GenerateToken(server.SessionTimeout, rqst.Mac, userId, userName, email, userDomain)

	return &authenticationpb.GeneratePeerTokenResponse{
		Token: token,
	}, err
}
