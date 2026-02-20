package main

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/big"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/globulario/services/golang/authentication/authentication_client"
	"github.com/globulario/services/golang/authentication/authenticationpb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/globular_client"
	"github.com/globulario/services/golang/ldap/ldap_client"
	"github.com/globulario/services/golang/rbac/rbac_client"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/resource/resource_client"
	"github.com/globulario/services/golang/resource/resourcepb"
	"github.com/globulario/services/golang/security"
	Utility "github.com/globulario/utility"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/emptypb"
)

var (
	dataPath   = config.GetDataDir()
	configPath = filepath.Join(config.GetConfigDir(), "config.json")
)

func normalizeAccountId(id string) string {
	if i := strings.IndexByte(id, '@'); i >= 0 {
		return id[:i]
	}
	return id
}

func isBcryptHash(s string) bool {
	return strings.HasPrefix(s, "$2a$") || strings.HasPrefix(s, "$2b$") || strings.HasPrefix(s, "$2y$") || strings.HasPrefix(s, "$2$")
}

// validatePasswordPolicy enforces a minimal server-side password policy.
// Requirements:
//   - at least 12 characters
//   - contains at least 3 of 4 classes: lower, upper, digit, special
//   - no spaces or control characters
func validatePasswordPolicy(pw string) error {
	if len(pw) < 12 {
		return errors.New("password must be at least 12 characters")
	}
	hasLower := strings.IndexFunc(pw, func(r rune) bool { return r >= 'a' && r <= 'z' }) >= 0
	hasUpper := strings.IndexFunc(pw, func(r rune) bool { return r >= 'A' && r <= 'Z' }) >= 0
	hasDigit := strings.IndexFunc(pw, func(r rune) bool { return r >= '0' && r <= '9' }) >= 0
	hasSpecial := strings.IndexFunc(pw, func(r rune) bool {
		return r >= 33 && r <= 126 && !(r >= '0' && r <= '9') && !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z')
	}) >= 0
	classes := 0
	for _, ok := range []bool{hasLower, hasUpper, hasDigit, hasSpecial} {
		if ok {
			classes++
		}
	}
	if classes < 3 {
		return errors.New("password must include at least 3 of: lowercase, uppercase, digit, special")
	}
	if strings.IndexFunc(pw, func(r rune) bool { return r <= 32 }) >= 0 {
		return errors.New("password may not contain spaces or control characters")
	}
	return nil
}

// ---- logging helpers (no signature changes to public API) ----

func logInternal(op string, err error, kv ...any) error {
	args := append(kv, "err", err)
	slog.Error(op, args...)
	return status.Errorf(
		codes.Internal,
		"%s", Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err),
	)
}

// ValidateToken validates the provided JWT token and returns the associated client ID and expiration time.
func (srv *server) ValidateToken(ctx context.Context, rqst *authenticationpb.ValidateTokenRqst) (*authenticationpb.ValidateTokenRsp, error) {
	claims, err := security.ValidateToken(rqst.Token)
	if err != nil {
		return nil, logInternal("ValidateToken:validate", err)
	}
	slog.Info("ValidateToken:ok", "clientId", claims.ID, "exp", claims.RegisteredClaims.ExpiresAt)
	return &authenticationpb.ValidateTokenRsp{
		ClientId: claims.ID,
		Expired:  claims.RegisteredClaims.ExpiresAt.Unix(),
	}, nil
}

// RefreshToken handles the refresh of an expired or soon-to-expire authentication token.
func (srv *server) RefreshToken(ctx context.Context, rqst *authenticationpb.RefreshTokenRqst) (*authenticationpb.RefreshTokenRsp, error) {
	claims, err := security.ValidateToken(rqst.Token)
	if err != nil && !strings.Contains(err.Error(), "token is expired") {
		return nil, logInternal("RefreshToken:validate", err)
	}

	// refuse refresh if token expired > 7 days ago
	if time.Unix(claims.RegisteredClaims.ExpiresAt.Unix(), 0).Before(time.Now().AddDate(0, 0, -7)) {
		return nil, logInternal("RefreshToken:tooOld", errors.New("the token cannot be refreshed after 7 days"))
	}

	tokenString, err := security.GenerateToken(
		srv.SessionTimeout, claims.Issuer, claims.ID, claims.Username, claims.Email,
	)
	if err != nil {
		return nil, logInternal("RefreshToken:generate", err)
	}

	// session maintenance
	session, err := srv.getSession(claims.ID)
	if err != nil {
		session = new(resourcepb.Session)
		session.AccountId = claims.ID
	}
	session.LastStateTime = time.Now().Unix()
	session.State = resourcepb.SessionState_ONLINE

	newClaims, _ := security.ValidateToken(tokenString)
	session.ExpireAt = newClaims.RegisteredClaims.ExpiresAt.Unix()

	if err = srv.updateSession(session); err != nil {
		return nil, logInternal("RefreshToken:updateSession", err)
	}

	slog.Info("RefreshToken:ok", "accountId", session.AccountId, "exp", session.ExpireAt)
	return &authenticationpb.RefreshTokenRsp{Token: tokenString}, nil
}

// SetPassword changes the password for a specified account.
func (srv *server) SetPassword(ctx context.Context, rqst *authenticationpb.SetPasswordRequest) (*authenticationpb.SetPasswordResponse, error) {

	clientId, token, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}

	accountId := normalizeAccountId(rqst.AccountId)
	account, err := srv.getAccount(accountId)
	if err != nil {
		return nil, logInternal("SetPassword:getAccount", err, "accountId", rqst.AccountId)
	}

	if normalizeAccountId(clientId) != account.Id {
		domain, _ := config.GetDomain()
		if clientId != "sa@"+domain && normalizeAccountId(clientId) != "sa" {
			return nil, logInternal("SetPassword:permission", errors.New("you can't change another account's password"))
		}
	} else {
		if err = srv.validatePassword(rqst.OldPassword, account.Password); err != nil {
			return nil, logInternal("SetPassword:validateOld", err, "accountId", rqst.AccountId)
		}
	}

	if err = validatePasswordPolicy(rqst.NewPassword); err != nil {
		return nil, logInternal("SetPassword:policy", err, "accountId", rqst.AccountId)
	}
	if err = srv.changeAccountPassword(accountId, token, rqst.OldPassword, rqst.NewPassword); err != nil {
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

	slog.Info("SetPassword:ok", "accountId", rqst.AccountId)
	return &authenticationpb.SetPasswordResponse{Token: tokenString}, nil
}

// SetRootPassword changes the root ("sa") account password.
func (srv *server) SetRootPassword(ctx context.Context, rqst *authenticationpb.SetRootPasswordRequest) (*authenticationpb.SetRootPasswordResponse, error) {
	clientId, token, err := security.GetClientId(ctx)
	if err != nil {
		return nil, err
	}

	if normalizeAccountId(clientId) != "sa" {
		return nil, logInternal("SetRootPassword:permission", errors.New("only 'sa' can change root password"))
	}

	if !Utility.Exists(configPath) {
		return nil, logInternal("SetRootPassword:missingConfig", errors.New("no configuration found at "+`"`+configPath+`"`))
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, logInternal("SetRootPassword:readConfig", err)
	}

	srvConfig := make(map[string]any)
	if err = json.Unmarshal(data, &srvConfig); err != nil {
		return nil, logInternal("SetRootPassword:parseConfig", err)
	}

	password, _ := srvConfig["RootPassword"].(string)

	effective := password
	if effective == "" {
		effective = "adminadmin"
	}

	if isBcryptHash(password) {
		if err = bcrypt.CompareHashAndPassword([]byte(password), []byte(rqst.OldPassword)); err != nil {
			return nil, logInternal("SetRootPassword:validateOld", errors.New("the given password doesn't match the existing one"))
		}
	} else {
		if rqst.OldPassword != effective {
			return nil, logInternal("SetRootPassword:defaultMismatch", errors.New("the given password doesn't match the existing one"))
		}
	}

	if err = validatePasswordPolicy(rqst.NewPassword); err != nil {
		return nil, logInternal("SetRootPassword:policy", err)
	}

	if err = srv.changeAccountPassword("sa", token, rqst.OldPassword, rqst.NewPassword); err != nil {
		if srv.Address != "" {
			return nil, logInternal("SetRootPassword:change", err)
		}
		slog.Warn("SetRootPassword:changeAccountPassword skipped (no address)", "err", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(rqst.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return nil, logInternal("SetRootPassword:hash", err)
	}
	srvConfig["RootPassword"] = string(hash)
	jsonStr, err := Utility.ToJson(srvConfig)
	if err != nil {
		return nil, logInternal("SetRootPassword:marshalConfig", err)
	}
	if err = os.WriteFile(configPath, []byte(jsonStr), 0600); err != nil {
		return nil, logInternal("SetRootPassword:writeConfig", err)
	}

	macAddress, err := config.GetMacAddress()
	if err != nil {
		return nil, err
	}

	adminEmail, _ := srvConfig["AdminEmail"].(string)
	tokenString, err := security.GenerateToken(srv.SessionTimeout, macAddress, "sa", "sa", adminEmail)
	if err != nil {
		return nil, logInternal("SetRootPassword:generateToken", err)
	}

	slog.Info("SetRootPassword:ok")
	return &authenticationpb.SetRootPasswordResponse{Token: tokenString}, nil
}

// SetRootEmail updates the root administrator email in the server configuration.
func (srv *server) SetRootEmail(ctx context.Context, rqst *authenticationpb.SetRootEmailRequest) (*authenticationpb.SetRootEmailResponse, error) {
	if !Utility.Exists(configPath) {
		return nil, logInternal("SetRootEmail:missingConfig", errors.New("no configuration found at "+`"`+configPath+`"`))
	}

	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, logInternal("SetRootEmail:readConfig", err)
	}

	cfg := make(map[string]any)
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

	slog.Info("SetRootEmail:ok", "newEmail", rqst.NewEmail)
	return &authenticationpb.SetRootEmailResponse{}, nil
}

// setKey generates peer private/public keys for this host if the mac matches.
func (srv *server) setKey(mac string) error {
	macAddress, err := config.GetMacAddress()
	if err != nil {
		return err
	}

	if macAddress == mac {
		slog.Info("setKey:generate", "mac", mac)
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

	var tokenInfo map[string]any
	if err = json.Unmarshal(body, &tokenInfo); err != nil {
		return false, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	if _, exists := tokenInfo["expires_in"]; !exists {
		return false, errors.New("invalid access token: missing expiration info")
	}

	return true, nil
}

// authenticate authenticates either root (sa) via config or a regular account (password / LDAP / OAuth).
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

		cfg := make(map[string]any)
		if err = json.Unmarshal(data, &cfg); err != nil {
			return "", logInternal("authenticate:root:parseConfig", err)
		}

		password, _ := cfg["RootPassword"].(string)

		effective := password
		if effective == "" {
			effective = "adminadmin"
		}

		if isBcryptHash(password) {
			if err = srv.validatePassword(pwd, password); err != nil {
				return "", logInternal("authenticate:root:validate", err)
			}
		} else {
			if pwd != effective {
				return "", logInternal("authenticate:root:defaultMismatch", errors.New("the given password doesn't match the existing one"))
			}
			// Upgrade legacy plaintext to bcrypt once authentication succeeds.
			if hash, herr := bcrypt.GenerateFromPassword([]byte(pwd), bcrypt.DefaultCost); herr == nil {
				cfg["RootPassword"] = string(hash)
				if jsonStr, merr := Utility.ToJson(cfg); merr == nil {
					_ = os.WriteFile(configPath, []byte(jsonStr), 0600)
				}
			}
		}

		adminEmail, _ := cfg["AdminEmail"].(string)
		tokenString, err := security.GenerateToken(srv.SessionTimeout, issuer, "sa", "sa", adminEmail)
		if err != nil {
			return "", logInternal("authenticate:root:generate", err)
		}

		// prepare home folder and resource owner mapping for sa (domain-free)
		path := "/users/sa"
		Utility.CreateDirIfNotExist(dataPath + "/files" + path)
		_ = srv.addResourceOwner(tokenString, path, "file", "sa", rbacpb.SubjectType_ACCOUNT)

		slog.Info("authenticate:root:ok")
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
			slog.Info("authenticate:passwordMismatch", "accountId", account.Id)
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
	sid := normalizeAccountId(account.Id)
	session.AccountId = sid

	tokenString, err := security.GenerateToken(srv.SessionTimeout, issuer, account.Id, account.Name, account.Email)
	if err != nil {
		return "", logInternal("authenticate:generate", err)
	}

	claims, _ := security.ValidateToken(tokenString)
	Utility.CreateDirIfNotExist(dataPath + "/files/users/" + sid)
	owner := claims.ID
	_ = srv.addResourceOwner(tokenString, "/users/"+sid, "file", owner, rbacpb.SubjectType_ACCOUNT)

	session.ExpireAt = claims.RegisteredClaims.ExpiresAt.Unix()
	session.State = resourcepb.SessionState_ONLINE
	session.LastStateTime = time.Now().Unix()

	if err = srv.updateSession(session); err != nil {
		return "", logInternal("authenticate:updateSession", err)
	}

	slog.Info("authenticate:ok", "accountId", session.AccountId, "exp", session.ExpireAt)
	return tokenString, nil
}

// GetResourceClient returns a Resource service client.
func getResourceClient(address string) (*resource_client.Resource_Client, error) {
	Utility.RegisterFunction("NewResourceService_Client", resource_client.NewResourceService_Client)
	client, err := globular_client.GetClient(address, "resource.ResourceService", "NewResourceService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*resource_client.Resource_Client), nil
}

// GetAuthenticationClient returns an Authentication service client.
func getAuthenticationClient(address string) (*authentication_client.Authentication_Client, error) {
	Utility.RegisterFunction("NewAuthenticationService_Client", authentication_client.NewAuthenticationService_Client)
	client, err := globular_client.GetClient(address, "authentication.AuthenticationService", "NewAuthenticationService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*authentication_client.Authentication_Client), nil
}

// Authenticate authenticates an account and returns a signed token.
func (srv *server) Authenticate(ctx context.Context, rqst *authenticationpb.AuthenticateRqst) (*authenticationpb.AuthenticateRsp, error) {
	var (
		tokenString string
		err         error
	)

	// Normalize first so that "sa@domain" inputs are treated identically to "sa".
	rqst.Name = normalizeAccountId(rqst.Name)

	if rqst.Name == "sa" {
		tokenString, err = srv.authenticate(rqst.Name, rqst.Password, srv.Mac)
		if err != nil {
			return nil, err
		}
		return &authenticationpb.AuthenticateRsp{Token: tokenString}, nil
	}

	if len(rqst.Issuer) == 0 {
		rqst.Issuer = srv.Mac
	}
	if rqst.Issuer == srv.Mac {
		tokenString, err = srv.authenticate(rqst.Name, rqst.Password, rqst.Issuer)
		if err == nil {
			return &authenticationpb.AuthenticateRsp{Token: tokenString}, nil
		}
	}

	if err != nil {
		nodes, _ := srv.getNodeIdentities()
		if len(nodes) == 0 {
			uuid := Utility.GenerateUUID(rqst.Name + rqst.Password + rqst.Issuer)
			defer Utility.RemoveString(srv.authentications_, uuid)
			if Utility.Contains(srv.authentications_, uuid) {
				return nil, errors.New("failed to authenticate " + rqst.Name + " on " + rqst.Issuer)
			}
			srv.authentications_ = append(srv.authentications_, uuid)

			for i := range nodes {
				node := nodes[i]
				address := node.Domain
				if node.Protocol == "https" {
					address += ":" + Utility.ToString(node.PortHttps)
				} else {
					address += ":" + Utility.ToString(node.PortHttp)
				}

				resourceClient, err := getResourceClient(address)
				if err == nil {
					defer resourceClient.Close()
					account, err := resourceClient.GetAccount(rqst.Name)
					if err == nil {
						authClient, err := getAuthenticationClient(address)
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

	token, err := security.GenerateToken(srv.SessionTimeout, rqst.Mac, userId, "", "")
	if err != nil {
		return nil, logInternal("GeneratePeerToken:generate", err)
	}

	slog.Info("GeneratePeerToken:ok", "issuerMac", rqst.Mac, "userId", userId, "userDomain", userDomain)
	return &authenticationpb.GeneratePeerTokenResponse{Token: token}, nil
}

// --- LDAP helpers ---
func GetLdapClient(address string) (*ldap_client.LDAP_Client, error) {
	client, err := globular_client.GetClient(address, "ldap.LdapService", "ldap_client.NewLdapService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*ldap_client.LDAP_Client), nil
}

func (srv *server) authenticateLdap(userId, password string) error {
	ldapClient, err := GetLdapClient(srv.Address)
	if err != nil {
		logger.Error("ldap connect failed", "address", srv.Address, "err", err)
		return err
	}
	return ldapClient.Authenticate(srv.LdapConnectionId, userId, password)
}

// --- RBAC helpers ---
func GetRbacClient(address string) (*rbac_client.Rbac_Client, error) {
	Utility.RegisterFunction("NewRbacService_Client", rbac_client.NewRbacService_Client)
	client, err := globular_client.GetClient(address, "rbac.RbacService", "NewRbacService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*rbac_client.Rbac_Client), nil
}

func (srv *server) addResourceOwner(token, path, resourceType, subject string, subjectType rbacpb.SubjectType) error {
	if srv.Address == "" {
		return nil
	}
	rbacClient, err := GetRbacClient(srv.Address)
	if err != nil {
		return err
	}
	return rbacClient.AddResourceOwner(token, path, subject, resourceType, subjectType)
}

// --- Resource helpers ---
func (srv *server) getResourceClient(address string) (*resource_client.Resource_Client, error) {
	Utility.RegisterFunction("NewResourceService_Client", resource_client.NewResourceService_Client)
	client, err := globular_client.GetClient(address, "resource.ResourceService", "NewResourceService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*resource_client.Resource_Client), nil
}

func (srv *server) getSessions() ([]*resourcepb.Session, error) {
	resourceClient, err := srv.getResourceClient(srv.GetAddress())
	if err != nil {
		return nil, err
	}
	return resourceClient.GetSessions(`{"state":0}`)
}

func (srv *server) removeSession(accountId string) error {
	resourceClient, err := srv.getResourceClient(srv.GetAddress())
	if err != nil {
		return err
	}
	return resourceClient.RemoveSession(accountId)
}

func (srv *server) updateSession(session *resourcepb.Session) error {
	resourceClient, err := srv.getResourceClient(srv.GetAddress())
	if err != nil {
		return err
	}
	return resourceClient.UpdateSession(session)
}

func (srv *server) getSession(accountId string) (*resourcepb.Session, error) {
	resourceClient, err := srv.getResourceClient(srv.GetAddress())
	if err != nil {
		return nil, err
	}
	return resourceClient.GetSession(normalizeAccountId(accountId))
}

func (srv *server) getAccount(accountId string) (*resourcepb.Account, error) {
	resourceClient, err := srv.getResourceClient(srv.GetAddress())
	if err != nil {
		return nil, err
	}
	return resourceClient.GetAccount(normalizeAccountId(accountId))
}

func (srv *server) changeAccountPassword(accountId, token, oldPassword, newPassword string) error {
	accountId = normalizeAccountId(accountId)
	resourceClient, err := srv.getResourceClient(srv.GetAddress())
	if err != nil {
		return err
	}
	return resourceClient.SetAccountPassword(accountId, token, oldPassword, newPassword)
}

func (srv *server) getNodeIdentities() ([]*resourcepb.NodeIdentity, error) {
	resourceClient, err := srv.getResourceClient(srv.GetAddress())
	if err != nil {
		return nil, err
	}
	nodes, err := resourceClient.ListNodeIdentities(`{}`, "")
	if err != nil {
		return nil, err
	}
	if len(nodes) == 0 {
		return nil, errors.New("no node identities found")
	}
	return nodes, nil
}

// --- Auth helpers ---
func (srv *server) validatePassword(password, hashed string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashed), []byte(password))
}

// IssueClientCertificate generates a fresh client certificate signed by the cluster CA
// for the authenticated caller.
//
// Security requirements:
//   - Caller must be authenticated (non-anonymous AuthContext.Subject).
//   - Subject CN = normalized caller identity (strip @domain suffix).
//   - EKU includes ClientAuth only.
//   - Validity: 30 days.
//   - Private key is generated server-side and returned in the response; it is NOT persisted.
func (srv *server) IssueClientCertificate(ctx context.Context, _ *emptypb.Empty) (*authenticationpb.IssueClientCertificateResponse, error) {
	// Verify caller is authenticated
	authCtx := security.FromContext(ctx)
	if authCtx == nil || authCtx.Subject == "" {
		return nil, status.Error(codes.Unauthenticated, "IssueClientCertificate: authentication required")
	}
	subject := authCtx.Subject // already normalized by interceptor (no @domain suffix)

	// Load cluster CA certificate and key
	caCertPath := config.GetCACertificatePath()
	caKeyPath := config.GetCAKeyPath()

	caCertPEM, err := os.ReadFile(caCertPath)
	if err != nil {
		slog.Error("IssueClientCertificate: read CA cert", "path", caCertPath, "err", err)
		return nil, status.Errorf(codes.Internal, "IssueClientCertificate: read CA: %v", err)
	}
	caKeyPEM, err := os.ReadFile(caKeyPath)
	if err != nil {
		slog.Error("IssueClientCertificate: read CA key", "path", caKeyPath, "err", err)
		return nil, status.Errorf(codes.Internal, "IssueClientCertificate: read CA key: %v", err)
	}

	// Parse CA cert
	caBlock, _ := pem.Decode(caCertPEM)
	if caBlock == nil {
		return nil, status.Error(codes.Internal, "IssueClientCertificate: invalid CA certificate PEM")
	}
	caCert, err := x509.ParseCertificate(caBlock.Bytes)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "IssueClientCertificate: parse CA cert: %v", err)
	}

	// Parse CA key (supports RSA and EC)
	caKeyBlock, _ := pem.Decode(caKeyPEM)
	if caKeyBlock == nil {
		return nil, status.Error(codes.Internal, "IssueClientCertificate: invalid CA key PEM")
	}
	var caPrivKey interface{}
	switch caKeyBlock.Type {
	case "RSA PRIVATE KEY":
		caPrivKey, err = x509.ParsePKCS1PrivateKey(caKeyBlock.Bytes)
	case "EC PRIVATE KEY":
		caPrivKey, err = x509.ParseECPrivateKey(caKeyBlock.Bytes)
	default:
		caPrivKey, err = x509.ParsePKCS8PrivateKey(caKeyBlock.Bytes)
	}
	if err != nil {
		return nil, status.Errorf(codes.Internal, "IssueClientCertificate: parse CA key: %v", err)
	}

	// Generate client RSA private key (2048-bit)
	clientKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "IssueClientCertificate: generate key: %v", err)
	}

	// Build certificate serial number
	serial, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		return nil, status.Errorf(codes.Internal, "IssueClientCertificate: serial: %v", err)
	}

	now := time.Now().UTC()
	template := &x509.Certificate{
		SerialNumber: serial,
		Subject: pkix.Name{
			CommonName: subject,
		},
		NotBefore:             now,
		NotAfter:              now.Add(30 * 24 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		BasicConstraintsValid: true,
		IsCA:                  false,
	}

	certDER, err := x509.CreateCertificate(rand.Reader, template, caCert, &clientKey.PublicKey, caPrivKey)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "IssueClientCertificate: sign cert: %v", err)
	}

	// PEM-encode outputs
	clientCertPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	clientKeyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(clientKey)})

	slog.Info("IssueClientCertificate: issued", "subject", subject, "expires", template.NotAfter)
	return &authenticationpb.IssueClientCertificateResponse{
		CaCrtPem:     caCertPEM,
		ClientCrtPem: clientCertPEM,
		ClientKeyPem: clientKeyPEM,
	}, nil
}
