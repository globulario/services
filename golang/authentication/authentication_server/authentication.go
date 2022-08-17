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
	config_ "github.com/globulario/services/golang/config"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
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

	var token string
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		token = strings.Join(md["token"], "")
		if len(token) == 0 {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("application manager SetPassword no token was given")))
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
	if accountId == "sa" {
		fmt.Println("autenticate sa")
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

		// In that particular case I will set the issuer to the current mac...
		issuer, err := Utility.MyMacAddr(Utility.MyLocalIP())
		if err != nil {
			return "", err
		}

		localDomain, _ := config_.GetDomain()
		tokenString, err := security.GenerateToken(server.SessionTimeout, issuer, "sa", "sa", config["AdminEmail"].(string), localDomain)
		if err != nil {
			return "", status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		// Create the user file directory.
		path := "/users/sa"
		Utility.CreateDirIfNotExist(dataPath + "/files" + path)
		server.addResourceOwner(path, "file", "sa", rbacpb.SubjectType_ACCOUNT)

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
			token, err := config.GetToken(server.Domain)
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
	session.AccountId = account.Id

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
	server.addResourceOwner("/users/" + claims.Id + "@" + account.Domain, "file", claims.Id+"@"+account.Domain, rbacpb.SubjectType_ACCOUNT)

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

//* Authenticate a user *
func (server *server) Authenticate(ctx context.Context, rqst *authenticationpb.AuthenticateRqst) (*authenticationpb.AuthenticateRsp, error) {

	mac, err := Utility.MyMacAddr(Utility.MyLocalIP())
	if err != nil {
		return nil, err
	}

	// Set the mac addresse
	if len(rqst.Issuer) == 0 {
		rqst.Issuer = mac
	} /*else {
		// The request came from external source...
		if rqst.Issuer != mac {
			localDomain, err := config.GetDomain()
			if err != nil {
				return nil, err
			}

			// Try autenticate...
			if strings.HasSuffix(rqst.Name, "@"+localDomain) {
				rqst.Issuer = mac
				rqst.Name = rqst.Name[0:strings.Index(rqst.Name, "@")]
			}
		}
	}*/

	// Try to authenticate on the server directy...
	tokenString, err := server.authenticate(rqst.Name, rqst.Password, rqst.Issuer)

	// Now I will try each peer...
	if err != nil {
		fmt.Println("fail to authenticate on " + rqst.Issuer + " i will try to authenticate on other peers...")
		uuid := Utility.GenerateUUID(rqst.Name + rqst.Password + rqst.Issuer)
		// no matter what happen the token must be remove...
		defer Utility.RemoveString(server.authentications_, uuid)
		if Utility.Contains(server.authentications_, uuid) {
			return nil, errors.New("fail to authenticate " + rqst.Name + " on " + rqst.Issuer)
		}

		// append the string in the list to cut infinite recursion
		server.authentications_ = append(server.authentications_, uuid)

		// I will try to authenticate the peer on other resource service...
		peers, err := server.getPeers()
		if err == nil {
			for i := 0; i < len(peers); i++ {
				peer := peers[i]
				address := peer.Domain
				if peer.Protocol == "https" {
					address += ":" + Utility.ToString(peer.PortHttps)
				} else {
					address += ":" + Utility.ToString(peer.PortHttp)
				}
				resource_client_, err := resource_client.NewResourceService_Client(address, "resource.ResourceService")
				if err == nil {
					defer resource_client_.Close()
					account, err := resource_client_.GetAccount(rqst.Name)
					if err == nil {
						// an account was found with that name...
						authentication_client_, err := authentication_client.NewAuthenticationService_Client(address, "authentication.AuthenticationService")
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

//* Generate a token for a peer with a given mac address *
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
