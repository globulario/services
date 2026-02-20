package main

import (
	"context"
	"strings"
	"testing"

	"github.com/globulario/services/golang/authentication/authenticationpb"
	"google.golang.org/grpc"
)

type fakeAuthClient struct {
	authCalled bool
	rootCalled bool
	lastRoot   *authenticationpb.SetRootPasswordRequest
	authErr    error
	rootErr    error
}

func (f *fakeAuthClient) Authenticate(ctx context.Context, in *authenticationpb.AuthenticateRqst, opts ...grpc.CallOption) (*authenticationpb.AuthenticateRsp, error) {
	f.authCalled = true
	if f.authErr != nil {
		return nil, f.authErr
	}
	return &authenticationpb.AuthenticateRsp{Token: "tok"}, nil
}

func (f *fakeAuthClient) SetRootPassword(ctx context.Context, in *authenticationpb.SetRootPasswordRequest, opts ...grpc.CallOption) (*authenticationpb.SetRootPasswordResponse, error) {
	f.rootCalled = true
	f.lastRoot = in
	if f.rootErr != nil {
		return nil, f.rootErr
	}
	return &authenticationpb.SetRootPasswordResponse{Token: "newtok"}, nil
}

type authFakeConn struct{}

func (authFakeConn) Invoke(ctx context.Context, method string, args any, reply any, opts ...grpc.CallOption) error {
	return nil
}

func (authFakeConn) NewStream(ctx context.Context, desc *grpc.StreamDesc, method string, opts ...grpc.CallOption) (grpc.ClientStream, error) {
	return nil, nil
}

func TestValidatePasswordPolicyCLI(t *testing.T) {
	if err := validatePasswordPolicyCLI("short"); err == nil {
		t.Fatalf("expected policy failure")
	}
	if err := validatePasswordPolicyCLI("ValidPassword123!"); err != nil {
		t.Fatalf("expected policy pass, got %v", err)
	}
}

func TestRootPassConfirmMismatch(t *testing.T) {
	fc := &fakeAuthClient{}
	oldClient := authClientFactory
	oldConn := authConnFactory
	defer func() {
		authClientFactory = oldClient
		authConnFactory = oldConn
		rootOld, rootNew, rootConfirm = "", "", ""
	}()
	authClientFactory = func(conn grpc.ClientConnInterface) authServiceClient { return fc }
	authConnFactory = func() (grpc.ClientConnInterface, func(), error) { return authFakeConn{}, func() {}, nil }

	rootOld, rootNew, rootConfirm = "old", "newPassword123!", "different"
	err := authRootPassCmd.RunE(nil, nil)
	if err == nil || !strings.Contains(err.Error(), "confirm") {
		t.Fatalf("expected confirm error, got %v", err)
	}
	if fc.rootCalled {
		t.Fatalf("SetRootPassword should not be called on confirm mismatch")
	}
}

func TestRootPassSuccess(t *testing.T) {
	fc := &fakeAuthClient{}
	oldClient := authClientFactory
	oldConn := authConnFactory
	defer func() {
		authClientFactory = oldClient
		authConnFactory = oldConn
		rootOld, rootNew, rootConfirm = "", "", ""
	}()
	authClientFactory = func(conn grpc.ClientConnInterface) authServiceClient { return fc }
	authConnFactory = func() (grpc.ClientConnInterface, func(), error) { return authFakeConn{}, func() {}, nil }

	rootOld, rootNew, rootConfirm = "oldPlain123!", "NewPassword123!", "NewPassword123!"
	if err := authRootPassCmd.RunE(nil, nil); err != nil {
		t.Fatalf("RunE: %v", err)
	}
	if !fc.rootCalled {
		t.Fatalf("expected SetRootPassword to be called")
	}
	if fc.lastRoot.OldPassword != rootOld || fc.lastRoot.NewPassword != rootNew {
		t.Fatalf("unexpected request: %#v", fc.lastRoot)
	}
}

func TestLoginWarnsDefaultRoot(t *testing.T) {
	if !isDefaultRoot("sa@domain", "adminadmin") {
		t.Fatalf("expected default root detection")
	}
	if isDefaultRoot("user", "adminadmin") {
		t.Fatalf("non-root should not be flagged")
	}
}

func TestRootPassPolicyFailure(t *testing.T) {
	fc := &fakeAuthClient{}
	oldClient := authClientFactory
	oldConn := authConnFactory
	defer func() {
		authClientFactory = oldClient
		authConnFactory = oldConn
		rootOld, rootNew, rootConfirm = "", "", ""
	}()
	authClientFactory = func(conn grpc.ClientConnInterface) authServiceClient { return fc }
	authConnFactory = func() (grpc.ClientConnInterface, func(), error) { return authFakeConn{}, func() {}, nil }

	rootOld, rootNew, rootConfirm = "old", "short", "short"
	err := authRootPassCmd.RunE(nil, nil)
	if err == nil || !strings.Contains(err.Error(), "12") {
		t.Fatalf("expected policy error, got %v", err)
	}
	if fc.rootCalled {
		t.Fatalf("SetRootPassword should not be called on policy failure")
	}
}
