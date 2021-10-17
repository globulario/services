package security

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"strings"
	"sync"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/davecourtois/Utility"
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

	jwt.StandardClaims
}

// Generate a token for a ginven user.
func GenerateToken(jwtKey []byte, timeout int, issuer, userId, userName, email string) (string, error) {

	// Declare the expiration time of the token
	now := time.Now()

	expirationTime := now.Add(time.Duration(timeout) * time.Minute)

	// Create the JWT claims, which includes the username and expiry time
	claims := &Claims{
		ID:       userId,
		Username: userName,
		Email:    email,
		StandardClaims: jwt.StandardClaims{
			// In JWT, the expiry time is expressed as unix milliseconds
			Id:        userId,
			ExpiresAt: expirationTime.Unix(),
			Subject:   userName,
			Issuer:    issuer,
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
func ValidateToken(token string) (string, string, string, string, int64, error) {

	// Initialize a new instance of `Claims`
	claims := &Claims{}

	// Parse the JWT string and store the result in `claims`.
	// Note that we are passing the key in this method as well. This method will return an error
	// if the token is invalid (if it has expired according to the expiry time we set on sign in),
	// or if the signature does not match
	tkn, err := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (interface{}, error) {

		// Get the jwt key from file.
		jwtKey, err := GetPeerKey(claims.Issuer)

		return jwtKey, err
	})

	if time.Now().After(time.Unix(claims.ExpiresAt, 0)) {
		return  claims.ID, claims.Username, claims.Email, claims.Issuer, claims.ExpiresAt, errors.New("the token is expired")
	}

	if err != nil {
		return claims.ID, claims.Username, claims.Email, claims.Issuer, claims.ExpiresAt, err
	}

	if !tkn.Valid {
		return claims.ID, claims.Username, claims.Email, claims.Issuer, claims.ExpiresAt, errors.New("invalid token")
	}

	return claims.ID, claims.Username, claims.Email, claims.Issuer, claims.ExpiresAt, nil
}

/**
 * refresh the local token.
 */
func refreshLocalToken(token string) (string, error) {
	fmt.Println("Refresh token...")
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (interface{}, error) {

		// Get the jwt key from file.
		jwtKey, err := GetPeerKey(claims.Issuer)

		return jwtKey, err
	})

	if err != nil && !strings.HasPrefix(err.Error(), "token is expired"){
		return "", err
	}

	jwtKey, err := GetPeerKey(claims.Issuer)
	if err != nil {
		return "", err
	}

	// Now I will get the duration from the configuration.
	globular := make(map[string] interface{})
	data, err := ioutil.ReadFile(config.GetConfigDir() + "/config.json")
	if err != nil {
		return "", err
	}

	err = json.Unmarshal(data, &globular)
	if err != nil {
		return "", err
	}

	token, err = GenerateToken(jwtKey, Utility.ToInt(globular["SessionTimeout"]), claims.Issuer, claims.Id, claims.Username, claims.Email)
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

	tokensPath := config.GetConfigDir() + "/tokens"
	path := tokensPath + "/" + domain + "_token"
	token_, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}

	// keep the token in the map...
	tokens.Store(domain, string(token_))

	return string(token_), nil
}

/**
 * Return the local token string.
 */
func GetLocalToken(domain string) (string, error) {

	token, err := getLocalToken(domain)
	if err != nil {
		return "", err
	}

	// Here I will validate the token...
	_, _, _, _, expireAt, err := ValidateToken(string(token))

	if err == nil {
		return string(token), nil
	}

	// If the token is older than seven day without being refresh then I retrun an error.
	if time.Unix(expireAt, 0).Before(time.Now().AddDate(0, 0, -7)) {
		return "", errors.New("the token cannot be refresh after 7 day")
	}
	
	newToken, err := refreshLocalToken(string(token))
	if err != nil{
		return "", err
	}

	// keep the token in the map...
	tokens.Store(domain, newToken)

	if err == nil {
		tokensPath := config.GetConfigDir() + "/tokens"
		path := tokensPath + "/" + domain + "_token"
		err = ioutil.WriteFile(path, []byte(newToken), 0644)
		if err != nil {
			return "", err
		}
	}

	return newToken, nil
}
