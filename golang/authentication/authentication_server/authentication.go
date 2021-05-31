package main

import (
	"context"
	"errors"
	//"log"
	"strings"
	"time"

	"io/ioutil"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/authentication/authenticationpb"
	"github.com/globulario/services/golang/interceptors"
	"github.com/globulario/services/golang/resource/resourcepb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

//* Validate a token *
func (server *server) ValidateToken(ctx context.Context, rqst *authenticationpb.ValidateTokenRqst) (*authenticationpb.ValidateTokenRsp, error) {
	id, _, _, expireAt, err := interceptors.ValidateToken(rqst.Token)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}
	return &authenticationpb.ValidateTokenRsp{
		ClientId: id,
		Expired:  expireAt,
	}, nil
}

//* Refresh token get a new token *
func (server *server) RefreshToken(ctx context.Context, rqst *authenticationpb.RefreshTokenRqst) (*authenticationpb.RefreshTokenRsp, error) {

	// first of all I will validate the current token.
	id, name, email, expireAt, err := interceptors.ValidateToken(rqst.Token)

	if err != nil {
		if !strings.HasPrefix(err.Error(), "token is expired") {
			return nil, status.Errorf(
				codes.Internal,
				Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
		}
	}

	// If the token is older than seven day without being refresh then I retrun an error.
	if time.Unix(expireAt, 0).Before(time.Now().AddDate(0, 0, -7)) {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), errors.New("the token cannot be refresh after 7 day")))
	}

	// Here I will test if a newer token exist for that user if it's the case
	// I will not refresh that token.
	session, err := server.getSession(id)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// That mean a newer token was already refresh.
	if time.Unix(expireAt, 0).Before(time.Unix(session.ExpireAt, 0)) {
		err := errors.New("that token cannot not be refresh because a newer one already exist. You need to re-authenticate in order to get a new token")
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	key, err := server.getKey()
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	tokenString, err := interceptors.GenerateToken([]byte(key), time.Duration(server.SessionTimeout), id, name, email)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// get back the new expireAt
	_, _, _, expireAt, _ = interceptors.ValidateToken(tokenString)
	session.Token = tokenString
	session.ExpireAt = expireAt

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

//Set the root password
func (server *server) SetRootPassword(ctx context.Context, rqst *authenticationpb.SetRootPasswordRequest) (*authenticationpb.SetRootPasswordResponse, error) {
	// Reset the passowrd.
	return nil, errors.New("not implemented")
}

//Set the root email
func (server *server) SetRootEmail(ctx context.Context, rqst *authenticationpb.SetRootEmailRequest) (*authenticationpb.SetRootEmailResponse, error) {
	return nil, errors.New("not implemented")
}

/**
 * Set the secret key that will be use to validate token. That key will be generate each time the server will be
 * restarted and all token generated with previous key will be automatically invalidated...
 */
func (server *server) setKey() error {
	return ioutil.WriteFile(keyPath+"/globular_key", []byte(Utility.RandomUUID()), 0644)
}

/**
 * Get the key from the file.
 */
func (server *server) getKey() (string, error) {
	data, err := ioutil.ReadFile(keyPath + "/globular_key")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

/* Authenticate a user */
func (server *server) authenticate(accountId string, pwd string) (string, error) {
	// Here I will get the account info.
	account, err := server.getAccount(accountId)
	if err != nil {
		return "", err
	}

	// Now I will validate the password received with the one in the account
	err = server.validatePassword(pwd, account.Password)
	if err != nil {
		return "", err
	}

	// Now I will create the session and generate it token.
	session := new(resourcepb.Session)
	session.AccountId = account.Id

	key, err := server.getKey()
	if err != nil {
		return "", status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	// The token string
	tokenString, err := interceptors.GenerateToken([]byte(key), time.Duration(server.SessionTimeout), account.Id, account.Name, account.Email)
	if err != nil {
		return "", err
	}

	// get the expire time.
	_, _, _, expireAt, _ := interceptors.ValidateToken(tokenString)

	session.Token = tokenString
	session.ExpireAt = expireAt
	session.State = resourcepb.SessionState_ONLINE
	session.LastStateTime = time.Now().Unix()

	err = server.updateSession(session)
	if err != nil {
		return "", err
	}

	return tokenString, nil
}

//* Authenticate a user *
func (server *server) Authenticate(ctx context.Context, rqst *authenticationpb.AuthenticateRqst) (*authenticationpb.AuthenticateRsp, error) {

	tokenString, err := server.authenticate(rqst.Name, rqst.Password)
	if err != nil {
		return nil, status.Errorf(
			codes.Internal,
			Utility.JsonErrorStr(Utility.FunctionName(), Utility.FileLine(), err))
	}

	return &authenticationpb.AuthenticateRsp{
		Token: tokenString,
	}, nil
}
