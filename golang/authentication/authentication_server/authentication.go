package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"time"

	"io/ioutil"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/authentication/authentication_client"
	"github.com/globulario/services/golang/authentication/authenticationpb"
	"github.com/globulario/services/golang/config"
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
func (srv *server) ValidateToken(ctx context.Context, rqst *authenticationpb.ValidateTokenRqst) (*authenticationpb.ValidateTokenRsp, error) {

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
func (srv *server) RefreshToken(ctx context.Context, rqst *authenticationpb.RefreshTokenRqst) (*authenticationpb.RefreshTokenRsp, error) {

	// first of all I will validate the current token.
	claims, err := security.ValidateToken(rqst.Token)
	if err != nil {
		if !strings.Contains(err.Error(), "token is expired") {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	if len(claims.UserDomain) == 0 {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("no user domain was found in the token")))
	}

	// If the token is older than seven day without being refresh then I retrun an error.
	if time.Unix(claims.StandardClaims.ExpiresAt, 0).Before(time.Now().AddDate(0, 0, -7)) {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("the token cannot be refresh after 7 day")))
	}

	tokenString, err := security.GenerateToken(srv.SessionTimeout, claims.Issuer, claims.Id, claims.Username, claims.Email, claims.UserDomain)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// get the active session.
	session, err := srv.getSession(claims.Id)
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

	// srv.logServiceInfo("RefreshToken", Utility.FileLine(), Utility.FunctionName(), "token expireAt: "+time.Unix(expireAt, 0).Local().String()+" actual time is "+time.Now().Local().String())
	// save the session in the backend.
	err = srv.updateSession(session)
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
func (srv *server) SetPassword(ctx context.Context, rqst *authenticationpb.SetPasswordRequest) (*authenticationpb.SetPasswordResponse, error) {

	// Get validated user id and token.
	clientId, token, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}

	// Here I will get the account info.
	account, err := srv.getAccount(rqst.AccountId)
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
		err = srv.validatePassword(rqst.OldPassword, account.Password)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	// Now I will update the account...
	err = srv.changeAccountPassword(rqst.AccountId, token, rqst.OldPassword, rqst.NewPassword)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	issuer, err := config.GetMacAddress()
	if err != nil {
		return nil, err
	}

	// Issue the token for the actual sever.
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		// The application...
		issuer = strings.Join(md["issuer"], "")
	}

	// finaly I will call authenticate to generate the token string and set it at return...
	tokenString, err := srv.authenticate(account.Id, rqst.NewPassword, issuer)
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
func (srv *server) SetRootPassword(ctx context.Context, rqst *authenticationpb.SetRootPasswordRequest) (*authenticationpb.SetRootPasswordResponse, error) {
	// Get validated user id and token.
	clientId, token, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}

	domain, _ := config.GetDomain()
	if clientId != "sa@"+domain {
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

	srvConfig := make(map[string]interface{})
	err = json.Unmarshal(data, &srvConfig)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// Now I go Globular config file I will get the password.
	password := srvConfig["RootPassword"].(string)

	// adminadmin is the default password...
	if password == "adminadmin" {
		if rqst.OldPassword != password {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("the given password dosent match existing one")))
		}
	} else {

		// Here I will get the account info.
		account, err := srv.getAccount("sa")
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		// Now I will validate the password received with the one in the account
		err = srv.validatePassword(rqst.OldPassword, account.Password)
		if err != nil {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	// Now I will update the account...
	err = srv.changeAccountPassword("sa", token, rqst.OldPassword, rqst.NewPassword)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	srvConfig["RootPassword"] = rqst.NewPassword
	jsonStr, err := Utility.ToJson(srvConfig)
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

	macAddress, err := config.GetMacAddress()
	if err != nil {
		return nil, err
	}

	// The token string
	tokenString, err := security.GenerateToken(srv.SessionTimeout, macAddress, "sa", "sa", srvConfig["AdminEmail"].(string), srv.Domain)
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
func (srv *server) SetRootEmail(ctx context.Context, rqst *authenticationpb.SetRootEmailRequest) (*authenticationpb.SetRootEmailResponse, error) {

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
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("the g1736628884iven email dosent match existing one")))

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
func (srv *server) setKey(mac string) error {

	// Get the mac address
	macAddress, err := config.GetMacAddress()
	if err != nil {
		return err
	}

	// Now I will generate keys if not already exist.
	if macAddress == mac {
		return security.GeneratePeerKeys(mac)
	}

	return nil
}

// validateGoogleToken checks if the provided access token is valid
func (srv *server) validateGoogleToken(accessToken string) (bool, error) {
	validationURL := "https://www.googleapis.com/oauth2/v3/tokeninfo?access_token=" + accessToken

	// Make the HTTP request
	resp, err := http.Get(validationURL)
	if err != nil {
		return false, fmt.Errorf("failed to validate token: %v", err)
	}
	defer resp.Body.Close()

	// Read response body
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("failed to read response body: %v", err)
	}

	// Check HTTP status code
	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("invalid token, status code: %d, response: %s", resp.StatusCode, string(body))
	}

	// Parse JSON response
	var tokenInfo map[string]interface{}
	err = json.Unmarshal(body, &tokenInfo)
	if err != nil {
		return false, fmt.Errorf("failed to parse JSON response: %v", err)
	}

	// Ensure the token has an `expires_in` field (indicates it's still valid)
	if _, exists := tokenInfo["expires_in"]; !exists {
		return false, errors.New("invalid access token: missing expiration info")
	}

	return true, nil
}

/* Authenticate a user */
func (srv *server) authenticate(accountId, pwd, issuer string) (string, error) {

	fmt.Println("authenticate: ", accountId, issuer)

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
			err = srv.validatePassword(pwd, password)
			if err != nil {
				return "", status.Errorf(
					codes.Internal,
					Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
			}
		}

		tokenString, err := security.GenerateToken(srv.SessionTimeout, issuer, "sa", "sa", config["AdminEmail"].(string), srv.Domain)
		if err != nil {
			return "", status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}

		// Create the user file directory.
		if strings.Contains(accountId, "@") {
			path := "/users/" + accountId
			Utility.CreateDirIfNotExist(dataPath + "/files" + path)
			srv.addResourceOwner(path, "file", "sa@"+srv.Domain, rbacpb.SubjectType_ACCOUNT)
		}

		// Be sure the password is correct.
		/*
			err = srv.changeAccountPassword(accountId, tokenString, pwd, pwd)
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
	account, err := srv.getAccount(accountId)
	if err != nil {
		return "", err
	}

	// Check if `pwd` is empty, indicating OAuth authentication
	if pwd == "" {
		if account.RefreshToken == "" {
			return "", errors.New("no password or refresh token provided")
		}



		// Call Google API to refresh the token
		refreshURL := fmt.Sprintf("https://%s/refresh_google_token?refresh_token=%s", srv.Domain, account.RefreshToken)
		resp, err := http.Get(refreshURL)
		if err != nil {
			return "", fmt.Errorf("failed to call refresh token API: %v", err)
		}
		defer resp.Body.Close()

		// Read response body
		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("failed to read response: %v", err)
		}

		// Parse the response (assuming it returns a JSON with "access_token")
		var result map[string]string
		err = json.Unmarshal(body, &result)
		if err != nil {
			return "", fmt.Errorf("failed to parse JSON response: %v", err)
		}

		accessToken, exists := result["access_token"]
		if !exists {
			return "", errors.New("no access token found in response")
		}

		// Validate the Google token (optional, if needed)
		valid, err := srv.validateGoogleToken(accessToken)
		if err != nil || !valid {
			return "", fmt.Errorf("invalid Google token: %v", err)
		}

	} else {
		err = srv.validatePassword(pwd, account.Password)
		if err != nil {
			srv.logServiceInfo("Authenticate", Utility.FileLine(), Utility.FunctionName(), err.Error())
			// Now if the LDAP service is configure I will try to authenticate with it...
			if len(srv.LdapConnectionId) != 0 {
				err := srv.authenticateLdap(account.Name, pwd)
				if err != nil {
					fmt.Println("fail to authenticate with error ", err)
					return "", err
				}
				// set back the password.
				// the old password can be left blank if the token was generated for sa.
				token, err := security.GetLocalToken(srv.Mac)
				if err != nil {
					return "", err
				}

				err = srv.changeAccountPassword(account.Id, token, "", pwd)
				if err != nil {
					fmt.Println("fail to change password: ", account.Id, err)
					return "", err
				}
			} else {
				return "", err
			}
		}
	}

	// Now I will create the session and generate it token.
	session := new(resourcepb.Session)
	session.AccountId = account.Id + "@" + account.Domain

	// The token string
	tokenString, err := security.GenerateToken(srv.SessionTimeout, issuer, account.Id, account.Name, account.Email, account.Domain)
	if err != nil {
		srv.logServiceInfo("Authenticate", Utility.FileLine(), Utility.FunctionName(), err.Error())
		return "", err
	}

	// get the expire time.
	claims, _ := security.ValidateToken(tokenString)

	owner := claims.Id
	if !strings.Contains(owner, "@") {
		owner += "@" + claims.UserDomain
	}

	// Create the user file directory.
	Utility.CreateDirIfNotExist(dataPath + "/files/users/" + account.Id + "@" + account.Domain)

	// be sure the user is the owner of that directory...
	srv.addResourceOwner("/users/"+account.Id+"@"+account.Domain, "file", owner, rbacpb.SubjectType_ACCOUNT)

	session.ExpireAt = claims.StandardClaims.ExpiresAt
	session.State = resourcepb.SessionState_ONLINE
	session.LastStateTime = time.Now().Unix()
	err = srv.updateSession(session)
	if err != nil {
		srv.logServiceInfo("Authenticate", Utility.FileLine(), Utility.FunctionName(), err.Error())
		return "", err
	}

	return tokenString, err
}

// Process existing file...
func (srv *server) processFiles() {

}

func GetResourceClient(address string) (*resource_client.Resource_Client, error) {
	Utility.RegisterFunction("NewResourceService_Client", resource_client.NewResourceService_Client)
	client, err := globular_client.GetClient(address, "resource.ResourceService", "NewResourceService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*resource_client.Resource_Client), nil
}

func GetAuthenticationClient(address string) (*authentication_client.Authentication_Client, error) {
	Utility.RegisterFunction("NewAuthenticationService_Client", authentication_client.NewAuthenticationService_Client)
	client, err := globular_client.GetClient(address, "authentication.AuthenticationService", "NewAuthenticationService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*authentication_client.Authentication_Client), nil
}

// * Authenticate a user *
func (srv *server) Authenticate(ctx context.Context, rqst *authenticationpb.AuthenticateRqst) (*authenticationpb.AuthenticateRsp, error) {

	var tokenString string
	var err error

	// The issuer is the mac address from where the request come from.
	// If the issuer is empty then I will use the mac address of the srv.
	// The rqst.Name is the account id, if the account is part of the domain I will try to authenticate it locally.
	if rqst.Name == "sa" {
		tokenString, err = srv.authenticate(rqst.Name, rqst.Password, srv.Mac)
		if err != nil {
			return nil, err
		}

		return &authenticationpb.AuthenticateRsp{
			Token: tokenString,
		}, nil

	}

	if strings.Contains(rqst.Name, "@") {
		domain := strings.Split(rqst.Name, "@")[1]
		if domain == srv.Domain {
			rqst.Issuer = srv.Mac
		}
	}

	// Set the mac addresse
	if len(rqst.Issuer) == 0 {
		rqst.Issuer = srv.Mac
	} else if rqst.Issuer == srv.Mac {
		// Try to authenticate on the server directly...
		tokenString, err = srv.authenticate(rqst.Name, rqst.Password, rqst.Issuer)
		if err == nil {
			return &authenticationpb.AuthenticateRsp{
				Token: tokenString,
			}, nil
		}
	}

	// Now I will try each peer...
	if err != nil {
		peers, _ := srv.getPeers()
		if len(peers) == 0 {
			uuid := Utility.GenerateUUID(rqst.Name + rqst.Password + rqst.Issuer)

			// no matter what happen the token must be remove...
			defer Utility.RemoveString(srv.authentications_, uuid)
			if Utility.Contains(srv.authentications_, uuid) {
				return nil, errors.New("fail to authenticate " + rqst.Name + " on " + rqst.Issuer)
			}

			// append the string in the list to cut infinite recursion
			srv.authentications_ = append(srv.authentications_, uuid)

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
func (srv *server) GeneratePeerToken(ctx context.Context, rqst *authenticationpb.GeneratePeerTokenRequest) (*authenticationpb.GeneratePeerTokenResponse, error) {

	clientId, _, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}

	userId := strings.Split(clientId, "@")[0]
	userDomain := strings.Split(clientId, "@")[1]

	// The generated token.
	token, err := security.GenerateToken(srv.SessionTimeout, rqst.Mac, userId, "", "", userDomain)

	return &authenticationpb.GeneratePeerTokenResponse{
		Token: token,
	}, err
}
