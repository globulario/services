package security

import (
	"context"
	"fmt"
	"github.com/dgrijalva/jwt-go"
	"github.com/globulario/services/golang/config"
	"io/ioutil"
	"time"
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
func GenerateToken(jwtKey []byte, timeout time.Duration, issuer, userId, userName, email string) (string, error) {

	// Declare the expiration time of the token
	now := time.Now()
	expirationTime := now.Add(timeout * time.Millisecond)

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
			IssuedAt:  now.Unix() - 1000,
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

	if err != nil {
		return claims.ID, claims.Username, claims.Email, claims.Issuer, claims.ExpiresAt, err
	}

	if !tkn.Valid {
		return claims.ID, claims.Username, claims.Email, claims.Issuer, claims.ExpiresAt, fmt.Errorf("invalid token!")
	}

	return claims.ID, claims.Username, claims.Email, claims.Issuer, claims.ExpiresAt, nil
}

/**
 * refresh the local token.
 */
func refreshLocalToken(token string) (string, error) {

	claims := &Claims{}
	_, err := jwt.ParseWithClaims(token, claims, func(token *jwt.Token) (interface{}, error) {

		// Get the jwt key from file.
		jwtKey, err := GetPeerKey(claims.Issuer)

		return jwtKey, err
	})

	if err != nil {
		return "", err
	}

	jwtKey, err := GetPeerKey(claims.Issuer)
	if err != nil {
		return "", err
	}

	token, err = GenerateToken(jwtKey, time.Duration(claims.ExpiresAt-claims.IssuedAt), claims.Issuer, "", "", claims.Email)
	if err != nil {
		return "", err
	}

	return token, err
}

/**
 * Return the local token string.
 */
func GetLocalToken(domain string) (string, error) {
	tokensPath := config.GetConfigDir() + "/tokens"
	path := tokensPath + "/" + domain + "_token"
	fmt.Println("---------------------------------> get local token: ", path)
	
	token, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}

	// Here I will validate the token...
	_, _, _, _, expiresAt, err := ValidateToken(string(token))
	if err != nil {
		return "", err
	}

	// Here I will test if I need to refresh the token...
	if time.Now().Before(time.Unix(expiresAt, 0)) {
		return string(token), nil
	}

	fmt.Println("---------------------------------> refresh local token: ", path)
	
	newToken, err := refreshLocalToken(string(token))
	if err == nil {
		err = ioutil.WriteFile(path, []byte(newToken), 0644)
	}

	return newToken, nil
}
