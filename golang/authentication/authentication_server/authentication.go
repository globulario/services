package main

import (
	"context"
	"errors"
	"github.com/globulario/services/golang/authentication/authenticationpb"
)

//* Validate a token *
func (sever *server) ValidateToken(ctx context.Context, rqst *authenticationpb.ValidateTokenRqst) (*authenticationpb.ValidateTokenRsp, error) {
	return nil, errors.New("not implemented")
}

//* Refresh token get a new token *
func (sever *server) RefreshToken(ctx context.Context, rqst *authenticationpb.RefreshTokenRqst) (*authenticationpb.RefreshTokenRsp, error) {
	return nil, errors.New("not implemented")
}

//* Set the account password *
func (sever *server) SetPassword(ctx context.Context, rqst *authenticationpb.SetPasswordRequest) (*authenticationpb.SetPasswordResponse, error) {
	return nil, errors.New("not implemented")
}

//Set the root password
func (sever *server) SetRootPassword(ctx context.Context, rqst *authenticationpb.SetRootPasswordRequest) (*authenticationpb.SetRootPasswordResponse, error) {
	return nil, errors.New("not implemented")
}

//Set the root email
func (sever *server) SetRootEmail(ctx context.Context, rqst *authenticationpb.SetRootEmailRequest) (*authenticationpb.SetRootEmailResponse, error) {
	return nil, errors.New("not implemented")
}

//* Authenticate a user *
func (sever *server) Authenticate(ctx context.Context, rqst *authenticationpb.AuthenticateRqst) (*authenticationpb.AuthenticateRsp, error) {
	return nil, errors.New("not implemented")
}
