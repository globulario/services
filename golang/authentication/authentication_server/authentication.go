package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/globulario/services/golang/authentication/authentication_client"
	"github.com/globulario/services/golang/authentication/authenticationpb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/globular_client"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/resource/resource_client"
	"github.com/globulario/services/golang/resource/resourcepb"
	"github.com/globulario/services/golang/security"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
)

var (
	dataPath   = config.GetDataDir()
	configPath = config.GetConfigDir() + "/config.json"
)

// ---- logging helpers (no signature changes to public API) ----

func logInternal(op string, err error, kv ...any) error {
	args := append(kv, "err", err)
	slog.Error(op, args...)
	return status.Errorf(
		codes.Internal,
		Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err),
	)
}

// ValidateToken validates a JWT and returns its client id and expiration.
func (srv *server) ValidateToken(ctx context.Context, rqst *authenticationpb.ValidateTokenRqst) (*authenticationpb.ValidateTokenRsp, error) {
	claims, err := security.ValidateToken(rqst.Token)
	if err != nil {
		return nil, logInternal("ValidateToken:validate", err)
	}
	logInfo("ValidateToken:ok", "clientId", claims.Id, "exp", claims.StandardClaims.ExpiresAt)
	return &authenticationpb.ValidateTokenRsp{
		ClientId: claims.Id,
		Expired:  claims.StandardClaims.ExpiresAt,
	}, nil
}

// RefreshToken refreshes a token unless it’s been expired for more than 7 days.
func (srv *server) RefreshToken(ctx context.Context, rqst *authenticationpb.RefreshTokenRqst) (*authenticationpb.RefreshTokenRsp, error) {
	claims, err := security.ValidateToken(rqst.Token)
	if err != nil && !strings.Contains(err.Error(), "token is expired") {
		return nil, logInternal("RefreshToken:validate", err)
	}

	if len(claims.UserDomain) == 0 {
		return nil, logInternal("RefreshToken:userDomain", errors.New("no user domain found in token"))
	}

	// refuse refresh if token expired > 7 days ago
	if time.Unix(claims.StandardClaims.ExpiresAt, 0).Before(time.Now().AddDate(0, 0, -7)) {
		return nil, logInternal("RefreshToken:tooOld", errors.New("the token cannot be refreshed after 7 days"))
	}

	tokenString, err := security.GenerateToken(
		srv.SessionTimeout, claims.Issuer, claims.Id, claims.Username, claims.Email, claims.UserDomain,
	)
	if err != nil {
		return nil, logInternal("RefreshToken:generate", err)
	}

	// session maintenance
	session, err := srv.getSession(claims.Id)
	if err != nil {
		session = new(resourcepb.Session)
		session.AccountId = claims.Id + "@" + claims.UserDomain
	}
	session.LastStateTime = time.Now().Unix()
	session.State = resourcepb.SessionState_ONLINE

	newClaims, _ := security.ValidateToken(tokenString)
	session.ExpireAt = newClaims.StandardClaims.ExpiresAt

	if err = srv.updateSession(session); err != nil {
		return nil, logInternal("RefreshToken:updateSession", err)
	}

	logInfo("RefreshToken:ok", "accountId", session.AccountId, "exp", session.ExpireAt)
	return &authenticationpb.RefreshTokenRsp{Token: tokenString}, nil
}

// SetPassword changes an account password and returns a fresh token.
func (srv *server) SetPassword(ctx context.Context, rqst *authenticationpb.SetPasswordRequest) (*authenticationpb.SetPasswordResponse, error) {
	clientId, token, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}

	account, err := srv.getAccount(rqst.AccountId)
	if err != nil {
		return nil, logInternal("SetPassword:getAccount", err, "accountId", rqst.AccountId)
	}

	domain, _ := config.GetDomain()
	if account.Id+"@"+account.Domain != clientId {
		if clientId != "sa@"+domain {
			return nil, logInternal("SetPassword:permission", errors.New("you can't change another account's password"))
		}
	} else {
		if err = srv.validatePassword(rqst.OldPassword, account.Password); err != nil {
			return nil, logInternal("SetPassword:validateOld", err, "accountId", rqst.AccountId)
		}
	}

	if err = srv.changeAccountPassword(rqst.AccountId, token, rqst.OldPassword, rqst.NewPassword); err != nil {
		return nil, logInternal("SetPassword:change", err, "accountId", rqst.AccountId)
	}

	issuer, err := config.GetMacAddress()
	if err != nil {
		return nil, err
	}
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		if v := strings.Join(md["issuer"], ""); v != "" {
			issuer = v
		}
	}

	tokenString, err := srv.authenticate(account.Id, rqst.NewPassword, issuer)
	if err != nil {
		return nil, logInternal("SetPassword:authenticate", err, "accountId", rqst.AccountId)
	}

	logInfo("SetPassword:ok", "accountId", rqst.AccountId)
	return &authenticationpb.SetPasswordResponse{Token: tokenString}, nil
}

// SetRootPassword updates the root (sa) password in backend+config, returning a new root token.
func (srv *server) SetRootPassword(ctx context.Context, rqst *authenticationpb.SetRootPasswordRequest) (*authenticationpb.SetRootPasswordResponse, error) {
	clientId, token, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}

	domain, _ := config.GetDomain()
	if clientId != "sa@"+domain {
		if !Utility.Exists(configPath) {
			return nil, logInternal("SetRootPassword:permission", errors.New("only 'sa' can change root password"))
		}
	}

	if !Utility.Exists(configPath) {
		return nil, logInternal("SetRootPassword:missingConfig", errors.New("no configuration found at "+`"`+configPath+`"`))
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, logInternal("SetRootPassword:readConfig", err)
	}

	srvConfig := make(map[string]interface{})
	if err = json.Unmarshal(data, &srvConfig); err != nil {
		return nil, logInternal("SetRootPassword:parseConfig", err)
	}

	password, _ := srvConfig["RootPassword"].(string)

	if password == "adminadmin" {
		if rqst.OldPassword != password {
			return nil, logInternal("SetRootPassword:defaultMismatch", errors.New("the given password doesn't match the existing one"))
		}
	} else {
		account, err := srv.getAccount("sa")
		if err != nil {
			return nil, logInternal("SetRootPassword:getAccount", err)
		}
		if err = srv.validatePassword(rqst.OldPassword, account.Password); err != nil {
			return nil, logInternal("SetRootPassword:validateOld", err)
		}
	}

	if err = srv.changeAccountPassword("sa", token, rqst.OldPassword, rqst.NewPassword); err != nil {
		return nil, logInternal("SetRootPassword:change", err)
	}

	srvConfig["RootPassword"] = rqst.NewPassword
	jsonStr, err := Utility.ToJson(srvConfig)
	if err != nil {
		return nil, logInternal("SetRootPassword:marshalConfig", err)
	}
	if err = os.WriteFile(configPath, []byte(jsonStr), 0644); err != nil {
		return nil, logInternal("SetRootPassword:writeConfig", err)
	}

	macAddress, err := config.GetMacAddress()
	if err != nil {
		return nil, err
	}

	tokenString, err := security.GenerateToken(srv.SessionTimeout, macAddress, "sa", "sa", srvConfig["AdminEmail"].(string), srv.Domain)
	if err != nil {
		return nil, logInternal("SetRootPassword:generateToken", err)
	}

	logInfo("SetRootPassword:ok")
	return &authenticationpb.SetRootPasswordResponse{Token: tokenString}, nil
}

// SetRootEmail updates the admin email in the configuration.
func (srv *server) SetRootEmail(ctx context.Context, rqst *authenticationpb.SetRootEmailRequest) (*authenticationpb.SetRootEmailResponse, error) {
	if !Utility.Exists(configPath) {
		return nil, logInternal("SetRootEmail:missingConfig", errors.New("no configuration found at "+`"`+configPath+`"`))
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, logInternal("SetRootEmail:readConfig", err)
	}

	cfg := make(map[string]interface{})
	if err = json.Unmarshal(data, &cfg); err != nil {
		return nil, logInternal("SetRootEmail:parseConfig", err)
	}

	email, _ := cfg["AdminEmail"].(string)
	if email != rqst.OldEmail {
		return nil, logInternal("SetRootEmail:mismatch", errors.New("the given email doesn't match the existing one"))
	}

	cfg["AdminEmail"] = rqst.NewEmail
	jsonStr, err := Utility.ToJson(cfg)
	if err != nil {
		return nil, logInternal("SetRootEmail:marshalConfig", err)
	}
	if err = os.WriteFile(configPath, []byte(jsonStr), 0644); err != nil {
		return nil, logInternal("SetRootEmail:writeConfig", err)
	}

	logInfo("SetRootEmail:ok", "newEmail", rqst.NewEmail)
	return &authenticationpb.SetRootEmailResponse{}, nil
}

/*
setKey generates peer private/public keys for this host if the mac matches.
*/
func (srv *server) setKey(mac string) error {
	macAddress, err := config.GetMacAddress()
	if err != nil {
		return err
	}

	if macAddress == mac {
		logInfo("setKey:generate", "mac", mac)
		return security.GeneratePeerKeys(mac)
	}
	return nil
}

// validateGoogleToken checks if the provided access token is valid via Google's tokeninfo endpoint.
func (srv *server) validateGoogleToken(accessToken string) (bool, error) {
	validationURL := "https://www.googleapis.com/oauth2/v3/tokeninfo?access_token=" + accessToken

	resp, err := http.Get(validationURL)
	if err != nil {
		return false, fmt.Errorf("failed to validate token: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return false, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return false, fmt.Errorf("invalid token, status code: %d, response: %s", resp.StatusCode, string(body))
	}

	var tokenInfo map[string]interface{}
	if err = json.Unmarshal(body, &tokenInfo); err != nil {
		return false, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	if _, exists := tokenInfo["expires_in"]; !exists {
		return false, errors.New("invalid access token: missing expiration info")
	}

	return true, nil
}

/* authenticate authenticates either root (sa) via config or a regular account (password / LDAP / OAuth).
It returns a signed JWT on success. */
func (srv *server) authenticate(accountId, pwd, issuer string) (string, error) {
	// Root path
	if accountId == "sa" || strings.HasPrefix(accountId, "sa@") {
		if !Utility.Exists(configPath) {
			return "", logInternal("authenticate:root:missingConfig", errors.New("no configuration found at "+`"`+configPath+`"`))
		}

		data, err := os.ReadFile(configPath)
		if err != nil {
			return "", logInternal("authenticate:root:readConfig", err)
		}

		cfg := make(map[string]interface{})
		if err = json.Unmarshal(data, &cfg); err != nil {
			return "", logInternal("authenticate:root:parseConfig", err)
		}

		password, _ := cfg["RootPassword"].(string)

		if password == "adminadmin" {
			if pwd != password {
				return "", logInternal("authenticate:root:defaultMismatch", errors.New("the given password doesn't match the existing one"))
			}
		} else {
			if err = srv.validatePassword(pwd, password); err != nil {
				return "", logInternal("authenticate:root:validate", err)
			}
		}

		tokenString, err := security.GenerateToken(srv.SessionTimeout, issuer, "sa", "sa", cfg["AdminEmail"].(string), srv.Domain)
		if err != nil {
			return "", logInternal("authenticate:root:generate", err)
		}

		// prepare home folder if using "sa@domain"
		if strings.Contains(accountId, "@") {
			path := "/users/" + accountId
			Utility.CreateDirIfNotExist(dataPath + "/files" + path)
			_ = srv.addResourceOwner(path, "file", "sa@"+srv.Domain, rbacpb.SubjectType_ACCOUNT)
		}

		// persist updated root password (keep current)
		cfg["RootPassword"] = pwd
		jsonStr, err := Utility.ToJson(cfg)
		if err != nil {
			return "", logInternal("authenticate:root:marshalConfig", err)
		}
		if err = os.WriteFile(configPath, []byte(jsonStr), 0644); err != nil {
			return "", logInternal("authenticate:root:writeConfig", err)
		}

		logInfo("authenticate:root:ok")
		return tokenString, nil
	}

	// Regular account path
	account, err := srv.getAccount(accountId)
	if err != nil {
		return "", err
	}

	if pwd == "" {
		// OAuth path
		if account.RefreshToken == "" {
			return "", errors.New("no password or refresh token provided")
		}

		refreshURL := fmt.Sprintf("https://%s/refresh_google_token?refresh_token=%s", srv.Domain, account.RefreshToken)
		resp, err := http.Get(refreshURL)
		if err != nil {
			return "", fmt.Errorf("failed to call refresh token API: %w", err)
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", fmt.Errorf("failed to read response: %w", err)
		}

		var result map[string]string
		if err = json.Unmarshal(body, &result); err != nil {
			return "", fmt.Errorf("failed to parse JSON response: %w", err)
		}

		accessToken, exists := result["access_token"]
		if !exists {
			return "", errors.New("no access token found in response")
		}

		valid, err := srv.validateGoogleToken(accessToken)
		if err != nil || !valid {
			return "", fmt.Errorf("invalid Google token: %w", err)
		}
	} else {
		// Password path (+ optional LDAP fallback)
		if err = srv.validatePassword(pwd, account.Password); err != nil {
			logInfo("authenticate:passwordMismatch", "accountId", account.Id)
			if len(srv.LdapConnectionId) != 0 {
				if err := srv.authenticateLdap(account.Name, pwd); err != nil {
					slog.Warn("authenticate:ldapFailed", "accountId", account.Id, "err", err)
					return "", err
				}
				// sync password from LDAP
				token, err := security.GetLocalToken(srv.Mac)
				if err != nil {
					return "", err
				}
				if err = srv.changeAccountPassword(account.Id, token, "", pwd); err != nil {
					slog.Warn("authenticate:syncPassword", "accountId", account.Id, "err", err)
					return "", err
				}
			} else {
				return "", err
			}
		}
	}

	// create session + token
	session := new(resourcepb.Session)
	session.AccountId = account.Id + "@" + account.Domain

	tokenString, err := security.GenerateToken(srv.SessionTimeout, issuer, account.Id, account.Name, account.Email, account.Domain)
	if err != nil {
		return "", logInternal("authenticate:generate", err)
	}

	claims, _ := security.ValidateToken(tokenString)
	owner := claims.Id
	if !strings.Contains(owner, "@") {
		owner += "@" + claims.UserDomain
	}

	Utility.CreateDirIfNotExist(dataPath + "/files/users/" + account.Id + "@" + account.Domain)
	_ = srv.addResourceOwner("/users/"+account.Id+"@"+account.Domain, "file", owner, rbacpb.SubjectType_ACCOUNT)

	session.ExpireAt = claims.StandardClaims.ExpiresAt
	session.State = resourcepb.SessionState_ONLINE
	session.LastStateTime = time.Now().Unix()

	if err = srv.updateSession(session); err != nil {
		return "", logInternal("authenticate:updateSession", err)
	}

	logInfo("authenticate:ok", "accountId", session.AccountId, "exp", session.ExpireAt)
	return tokenString, nil
}

// processFiles is currently a no-op placeholder.
func (srv *server) processFiles() {}

// GetResourceClient returns a Resource service client.
func GetResourceClient(address string) (*resource_client.Resource_Client, error) {
	Utility.RegisterFunction("NewResourceService_Client", resource_client.NewResourceService_Client)
	client, err := globular_client.GetClient(address, "resource.ResourceService", "NewResourceService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*resource_client.Resource_Client), nil
}

// GetAuthenticationClient returns an Authentication service client.
func GetAuthenticationClient(address string) (*authentication_client.Authentication_Client, error) {
	Utility.RegisterFunction("NewAuthenticationService_Client", authentication_client.NewAuthenticationService_Client)
	client, err := globular_client.GetClient(address, "authentication.AuthenticationService", "NewAuthenticationService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*authentication_client.Authentication_Client), nil
}

// Authenticate authenticates an account and returns a signed token.
// If Name is "sa", it authenticates against local config.
// If issuer is empty and the account belongs to this domain, srv.Mac is used.
// If local auth fails, peers are tried to locate the account.
func (srv *server) Authenticate(ctx context.Context, rqst *authenticationpb.AuthenticateRqst) (*authenticationpb.AuthenticateRsp, error) {
	var (
		tokenString string
		err         error
	)

	if rqst.Name == "sa" {
		tokenString, err = srv.authenticate(rqst.Name, rqst.Password, srv.Mac)
		if err != nil {
			return nil, err
		}
		return &authenticationpb.AuthenticateRsp{Token: tokenString}, nil
	}

	if strings.Contains(rqst.Name, "@") {
		if domain := strings.Split(rqst.Name, "@")[1]; domain == srv.Domain {
			rqst.Issuer = srv.Mac
		}
	}

	if len(rqst.Issuer) == 0 {
		rqst.Issuer = srv.Mac
	} else if rqst.Issuer == srv.Mac {
		tokenString, err = srv.authenticate(rqst.Name, rqst.Password, rqst.Issuer)
		if err == nil {
			return &authenticationpb.AuthenticateRsp{Token: tokenString}, nil
		}
	}

	if err != nil {
		peers, _ := srv.getPeers()
		if len(peers) == 0 {
			uuid := Utility.GenerateUUID(rqst.Name + rqst.Password + rqst.Issuer)
			defer Utility.RemoveString(srv.authentications_, uuid)
			if Utility.Contains(srv.authentications_, uuid) {
				return nil, errors.New("failed to authenticate " + rqst.Name + " on " + rqst.Issuer)
			}
			srv.authentications_ = append(srv.authentications_, uuid)

			for i := range peers {
				peer := peers[i]
				address := peer.Domain
				if peer.Protocol == "https" {
					address += ":" + Utility.ToString(peer.PortHttps)
				} else {
					address += ":" + Utility.ToString(peer.PortHttp)
				}

				resourceClient, err := GetResourceClient(address)
				if err == nil {
					defer resourceClient.Close()
					account, err := resourceClient.GetAccount(rqst.Name)
					if err == nil {
						authClient, err := GetAuthenticationClient(address)
						if err == nil {
							defer authClient.Close()
							tokenString, err := authClient.Authenticate(account.Id, rqst.Password)
							if err == nil {
								return &authenticationpb.AuthenticateRsp{Token: tokenString}, nil
							}
						}
					}
				}
			}
		}

		return nil, logInternal("Authenticate:failed", errors.New("failed to authenticate user "+rqst.Name+" from "+rqst.Issuer))
	}

	return &authenticationpb.AuthenticateRsp{Token: tokenString}, nil
}

// GeneratePeerToken generates a token for a peer identified by MAC, issued for the caller.
func (srv *server) GeneratePeerToken(ctx context.Context, rqst *authenticationpb.GeneratePeerTokenRequest) (*authenticationpb.GeneratePeerTokenResponse, error) {
	clientId, _, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}

	userId := strings.Split(clientId, "@")[0]
	userDomain := strings.Split(clientId, "@")[1]

	token, err := security.GenerateToken(srv.SessionTimeout, rqst.Mac, userId, "", "", userDomain)
	if err != nil {
		return nil, logInternal("GeneratePeerToken:generate", err)
	}

	logInfo("GeneratePeerToken:ok", "issuerMac", rqst.Mac, "userId", userId, "userDomain", userDomain)
	return &authenticationpb.GeneratePeerTokenResponse{Token: token}, nil
}
