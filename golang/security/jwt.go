package security

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"strings"
	"sync"
	"time"

	"github.com/davecourtois/Utility"
	"github.com/dgrijalva/jwt-go"
	"github.com/globulario/services/golang/config"
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
	ID       string `json:"id"`
	Username string `json:"username"`
	Email    string `json:"email"`
	Domain   string `json:"domain"`
	Address  string `json:"address"`
	jwt.StandardClaims
}

// Generate a token for a ginven user.
func GenerateToken(timeout int, mac, userId, userName, email string) (string, error) {

	issuer, err := Utility.MyMacAddr(Utility.MyLocalIP())
	if err != nil {
		return "", err
	}

	audience := ""

	if mac != issuer {
		audience = mac
	}

	var jwtKey []byte

	// Here I will get the key...
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

	// Declare the expiration time of the token
	now := time.Now()

	expirationTime := now.Add(time.Duration(timeout) * time.Minute)

	domain, err := config.GetDomain()
	if err != nil {
		return "", err
	}

	address, err := config.GetAddress()
	if err != nil {
		return "", err
	}

	// Create the JWT claims, which includes the username and expiry time
	claims := &Claims{
		ID:       userId,
		Username: userName,
		Email:    email,
		Domain:   domain,
		Address:  address,
		StandardClaims: jwt.StandardClaims{
			// In JWT, the expiry time is expressed as unix milliseconds
			Id:        userId,
			ExpiresAt: expirationTime.Unix(),
			Subject:   userId,
			Issuer:    issuer,
			Audience:  audience,
			IssuedAt:  now.Unix() - 1000, // make sure the IssuedAt is not in the futur...
		},
	}

	// Declare the token with the algorithm used for signing, and the claims
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Create the JWT string
	tokenString, err := token.SignedString(jwtKey)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

/** Validate a Token **/
func ValidateToken(token string) (*Claims, error) {

	// Initialize a new instance of `Claims`
	claims := &Claims{}

	// Parse the JWT string and store the result in `claims`.
	// Note that we are passing the key in this method as well. This method will return an error
	// if the token is invalid (if it has expired according to the expiry time we set on sign in),
	// or if the signature does not match
	tkn, err := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (interface{}, error) {
		macAddress, err := Utility.MyMacAddr(Utility.MyLocalIP())
		if err != nil {
			return "", err
		}

		// Get the jwt key from file.
		if len(claims.StandardClaims.Audience) > 0 {
			if claims.StandardClaims.Audience != macAddress {
				return GetPeerKey(claims.StandardClaims.Audience)
			}
		}

		key, err := GetPeerKey(claims.StandardClaims.Issuer)
		if err != nil {
			return "", err
		}
		return key, nil
	})

	if time.Now().After(time.Unix(claims.ExpiresAt, 0)) {
		return claims, errors.New("the token is expired")
	}

	if err != nil {
		return claims, err
	}

	if !tkn.Valid {
		return claims, errors.New("invalid token")
	}

	return claims, nil
}

/**
 * refresh the local token.
 */
func refreshLocalToken(token string) (string, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (interface{}, error) {
		macAddress, err := Utility.MyMacAddr(Utility.MyLocalIP())
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

	if err != nil && !strings.HasPrefix(err.Error(), "token is expired") {
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

	token, err = GenerateToken(Utility.ToInt(globular["SessionTimeout"]), claims.StandardClaims.Issuer, claims.Id, claims.Username, claims.Email)
	if err != nil {
		return "", err
	}
	return token, err
}

// Here I will keep the token in a map so it will be less file reading...
var tokens = new(sync.Map)

func getLocalToken(domain string) (string, error) {
	token, ok := tokens.Load(domain)
	if ok {
		if token != nil {
			return token.(string), nil
		}
	}

	return "", errors.New("no token found")
}

/**
 * Return the local token from the memory map. All token will be lost each time the server reboot.
 */
func GetLocalToken(mac string) (string, error) {

	token, err := getLocalToken(mac)
	if err != nil {
		return "", err
	}

	// Here I will validate the token...
	claims, err := ValidateToken(string(token))

	if err == nil {
		return string(token), nil
	}

	// If the token is older than seven day without being refresh then I retrun an error.
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
