package security

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/davecourtois/Utility"
	"github.com/dgrijalva/jwt-go"
	"github.com/globulario/services/golang/config"
	"google.golang.org/grpc/metadata"
)

// Authentication holds the login/password
type Authentication struct {
	Token string
}

// GetRequestMetadata gets the current request metadata
func (a *Authentication) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{
		"token": a.Token,
	}, nil
}

// RequireTransportSecurity indicates whether the credentials requires transport security
func (a *Authentication) RequireTransportSecurity() bool {
	return true
}

// Create a struct that will be encoded to a JWT.
// We add jwt.StandardClaims as an embedded type, to provide fields like expiry time
type Claims struct {
	ID         string `json:"id"`
	Username   string `json:"username"`
	Email      string `json:"email"`
	Domain     string `json:"domain"` // Where the token was generated
	UserDomain string `json:"user_domain"`
	Address    string `json:"address"`
	jwt.StandardClaims
}

// Generate a token for a given user.
func GenerateToken(timeout int, mac, userId, userName, email, userDomain string) (string, error) {

	// Get current time
	now := time.Now()

	// Set the expiration time for the token
	expirationTime := now.Add(time.Duration(timeout) * time.Minute)

	// Get the issuer (MAC address)
	issuer, err := config.GetMacAddress()
	if err != nil {
		return "", err
	}

	// Set audience based on MAC address
	audience := ""
	if mac != issuer {
		audience = mac
	}

	// Get the JWT signing key
	var jwtKey []byte
	if len(audience) > 0 {
		jwtKey, err = GetPeerKey(audience)
		if err != nil {
			return "", err
		}
	} else {
		jwtKey, err = GetPeerKey(issuer)
		if err != nil {
			return "", err
		}
	}

	// Get domain and address info
	domain, err := config.GetDomain()
	if err != nil {
		return "", err
	}

	address, err := config.GetAddress()
	if err != nil {
		return "", err
	}

	// Create the JWT claims with user information and expiration time
	claims := &Claims{
		ID:         userId,
		Username:   userName,
		UserDomain: userDomain,
		Email:      email,
		Domain:     domain,
		Address:    address,
		StandardClaims: jwt.StandardClaims{
			// Set the expiration time of the token (in Unix seconds)
			ExpiresAt: expirationTime.Unix(),
			// Set issued at time
			IssuedAt: now.Unix(),
			// The ID, subject, issuer, and audience
			Id:        userId,
			Subject:   userId,
			Issuer:    issuer,
			Audience:  audience,
		},
	}

	// Create the JWT token using the specified signing method and claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Generate the signed token string
	tokenString, err := token.SignedString(jwtKey)
	if err != nil {
		return "", err
	}

	// Optionally validate the token here (if needed)
	_, err = ValidateToken(tokenString)
	if err != nil {
		fmt.Println("Failed to generate token:", err)
		return "", err
	}

	// Return the generated token string
	//fmt.Println("Token generated successfully")
	//fmt.Println("Token:", tokenString)

	return tokenString, nil
}

/** Validate a Token **/
func ValidateToken(token string) (*Claims, error) {

	// Initialize a new instance of `Claims`
	claims := &Claims{}

	// Parse the JWT string and store the result in `claims`.
	// This method will return an error if the token is invalid or if the signature doesn't match
	tkn, err := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (interface{}, error) {
		// Retrieve the MAC address (issuer) and determine the key to use
		macAddress, err := config.GetMacAddress()
		if err != nil {
			return nil, err
		}

		// Check if the audience is different from the MAC address (this might happen when the token is intended for a specific peer)
		if len(claims.StandardClaims.Audience) > 0 {
			// If the audience is different, we retrieve the peer's key
			if claims.StandardClaims.Audience != macAddress {
				return GetPeerKey(claims.StandardClaims.Audience)
			}
		}

		// Otherwise, return the key associated with the issuer
		key, err := GetPeerKey(claims.StandardClaims.Issuer)
		if err != nil {
			return nil, err
		}

		// Return the correct key to verify the signature
		return key, nil
	})

	// Check if there was an error during the token parsing process
	if err != nil {
		return claims, fmt.Errorf("failed to parse token: %v", err)
	}

	// Check if the token has expired
	if time.Now().After(time.Unix(claims.ExpiresAt, 0)) {
		return claims, errors.New("token has expired or is invalid " + time.Unix(claims.ExpiresAt, 0).String())
	}

	// Check if the token signature is valid
	if !tkn.Valid {
		return claims, errors.New("invalid token signature")
	}

	// If everything is fine, return the claims
	return claims, nil
}

func GetClientAddress(ctx context.Context) (string, error) {
	// Now I will index the conversation to be retreivable for it creator...
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		token := strings.Join(md["token"], "")
		if len(token) > 0 {
			claims, err := ValidateToken(token)
			if err != nil {
				return "", err
			}

			if len(claims.Address) == 0 {
				return "", errors.New("no address found in the token")
			}

			return claims.Address, nil

		} else {
			return "", errors.New("no token found in the request")
		}
	}

	return "", errors.New("fail to validate the token")
}

func GetClientId(ctx context.Context) (string, string, error) {
	var username string
	var token string

	// Now I will index the conversation to be retreivable for it creator...
	if md, ok := metadata.FromIncomingContext(ctx); ok {
		token = strings.Join(md["token"], "")
		if len(token) > 0 {
			claims, err := ValidateToken(token)
			if err != nil {
				return "", "", err
			}

			if len(claims.UserDomain) == 0 {
				return "", "", errors.New("no user domain found in the token")
			}

			username = claims.Id + "@" + claims.UserDomain

		} else {
			return "", "", errors.New("no token found in the request")
		}
	}

	return username, token, nil
}

/**
 * refresh the local token.
 */
func refreshLocalToken(token string) (string, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (interface{}, error) {
		macAddress, err := config.GetMacAddress()
		if err != nil {
			return "", err
		}

		if len(claims.StandardClaims.Audience) > 0 {
			if claims.StandardClaims.Audience != macAddress {
				return GetPeerKey(claims.StandardClaims.Audience)
			}
		}

		return GetPeerKey(claims.StandardClaims.Issuer)
	})

	if err != nil && !strings.Contains(err.Error(), "token is expired") {
		return "", err
	}

	// Now I will get the duration from the configuration.
	globular := make(map[string]interface{})
	data, err := ioutil.ReadFile(config.GetConfigDir() + "/config.json")
	if err != nil {
		return "", err
	}

	err = json.Unmarshal(data, &globular)
	if err != nil {
		return "", err
	}

	timeout := Utility.ToInt(globular["SessionTimeout"])
	token, err = GenerateToken(timeout, claims.StandardClaims.Issuer, claims.Id, claims.Username, claims.Email, claims.UserDomain)
	if err != nil {
		return "", err
	}
	return token, err
}

// Here I will keep the token in a map so it will be less file reading...
var tokens = new(sync.Map)

func getLocalToken(mac string) (string, error) {
	token, ok := tokens.Load(mac)
	if ok {
		if token != nil {
			return token.(string), nil
		}
	}

	// if the token is not in the local map I will try to read it from file...
	// ex /etc/globular/config/tokens/globule-ryzen.globular.cloud_token
	mac = strings.ReplaceAll(mac, ":", "_")
	path := config.GetConfigDir() + "/tokens/" + mac + "_token"

	if Utility.Exists(path) {
		data, err := os.ReadFile(path)
		if err == nil {
			token := string(data)
			return token, nil
		}
	}

	return "", errors.New("no token found")
}

func SetLocalToken(mac, domain, id, name, email string, timeout int) error {
	mac = strings.ReplaceAll(mac, ":", "_")
	os.Remove(config.GetConfigDir() + "/tokens/" + mac + "_token")

	tokenString, err := GenerateToken(timeout, mac, id, name, email, domain)
	if err != nil {
		fmt.Println("fail to generate token with error: ", err)
		return err
	}

	err = ioutil.WriteFile(config.GetConfigDir()+"/tokens/"+mac+"_token", []byte(tokenString), 0644)
	if err != nil {
		fmt.Println("fail to save local token with error: ", err)
		return err
	}

	// keep in the local map...
	tokens.Store(mac, tokenString)

	return nil
}

/**
 * Return the local token from the memory map. All token will be lost each time the server reboot.
 */
func GetLocalToken(mac string) (string, error) {

	token, _ := getLocalToken(mac)
	if len(token) == 0 {
		return "", errors.New("no token was found for mac address " + mac)
	}

	// Here I will validate the token...
	claims, err := ValidateToken(string(token))
	if err == nil {
		return string(token), nil
	}

	if time.Unix(claims.StandardClaims.ExpiresAt, 0).Before(time.Now().AddDate(0, 0, -7)) {
		return "", errors.New("the token cannot be refresh after 7 day")
	}

	newToken, err := refreshLocalToken(string(token))
	if err != nil {
		return "", err
	}

	// keep the token in the map...
	tokens.Store(mac, newToken)

	return newToken, nil
}
