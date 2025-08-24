package authentication_client

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/globulario/services/golang/authentication/authenticationpb"
	"github.com/globulario/services/golang/config"
	globular "github.com/globulario/services/golang/globular_client"
	"github.com/globulario/services/golang/security"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

// //////////////////////////////////////////////////////////////////////////////
// Authentication  Client Service
// //////////////////////////////////////////////////////////////////////////////
var (
	tokensPath = config.GetConfigDir() + "/tokens"
)

type Authentication_Client struct {
	cc *grpc.ClientConn
	c  authenticationpb.AuthenticationServiceClient

	// The id of the service
	id string

	// The mac address of the server
	mac string

	// The name of the service
	name string

	// The client domain
	domain string

	// The address where connection with client can be done. ex: globule0.globular.cloud:10101
	address string

	//  keep the last connection state of the client.
	state string

	// The port
	port int

	// is the connection is secure?
	hasTLS bool

	// Link to client key file
	keyFile string

	// Link to client certificate file.
	certFile string

	// certificate authority file
	caFile string

	// The client context
	ctx context.Context
}

// Create a connection to the service.
func NewAuthenticationService_Client(address string, id string) (*Authentication_Client, error) {
	client := new(Authentication_Client)
	err := globular.InitClient(client, address, id)
	if err != nil {
		return nil, err
	}

	err = client.Reconnect()
	if err != nil {
		return nil, err
	}
	return client, nil
}

func (client *Authentication_Client) Reconnect() error {

	var err error
	nb_try_connect := 10

	for i := 0; i < nb_try_connect; i++ {
		client.cc, err = globular.GetClientConnection(client)
		if err == nil {
			client.c = authenticationpb.NewAuthenticationServiceClient(client.cc)
			break
		}

		// wait 500 millisecond before next try
		time.Sleep(500 * time.Millisecond)
	}

	return err

}

// The address where the client can connect.
func (client *Authentication_Client) SetAddress(address string) {
	client.address = address
}

func (client *Authentication_Client) Invoke(method string, rqst interface{}, ctx context.Context) (interface{}, error) {
	if ctx == nil {
		ctx = client.GetCtx()
	}
	return globular.InvokeClientRequest(client.c, ctx, method, rqst)
}

func (client *Authentication_Client) GetCtx() context.Context {
	if client.ctx == nil {
		client.ctx = globular.GetClientContext(client)
	}

	// refresh the client as needed...
	token, err := security.GetLocalToken(client.GetMac())
	if err == nil {
		md := metadata.New(map[string]string{"token": string(token), "domain": client.domain, "mac": client.GetMac(), "address": client.GetAddress()})
		client.ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	return client.ctx
}

// Return the domain
func (client *Authentication_Client) GetDomain() string {
	return client.domain
}

// Return the last know connection state
func (client *Authentication_Client) GetState() string {
	return client.state
}

// Return the address
func (client *Authentication_Client) GetAddress() string {
	return client.address
}

// Return the id of the service instance
func (client *Authentication_Client) GetId() string {
	return client.id
}

// Return the name of the service
func (client *Authentication_Client) GetName() string {
	return client.name
}

func (client *Authentication_Client) GetMac() string {
	return client.mac
}

// must be close when no more needed.
func (client *Authentication_Client) Close() {
	client.cc.Close()
}

// Set grpc_service port.
func (client *Authentication_Client) SetPort(port int) {
	client.port = port
}

// Return the grpc port number
func (client *Authentication_Client) GetPort() int {
	return client.port
}

// Set the client instance id.
func (client *Authentication_Client) SetId(id string) {
	client.id = id
}

// Set the client name.
func (client *Authentication_Client) SetName(name string) {
	client.name = name
}

func (client *Authentication_Client) SetMac(mac string) {
	client.mac = mac
}

// Set the domain.
func (client *Authentication_Client) SetDomain(domain string) {
	client.domain = domain
}

func (client *Authentication_Client) SetState(state string) {
	client.state = state
}

////////////////// TLS ///////////////////

// Get if the client is secure.
func (client *Authentication_Client) HasTLS() bool {
	return client.hasTLS
}

// Get the TLS certificate file path
func (client *Authentication_Client) GetCertFile() string {
	return client.certFile
}

// Get the TLS key file path
func (client *Authentication_Client) GetKeyFile() string {
	return client.keyFile
}

// Get the TLS key file path
func (client *Authentication_Client) GetCaFile() string {
	return client.caFile
}

// Set the client is a secure client.
func (client *Authentication_Client) SetTLS(hasTls bool) {
	client.hasTLS = hasTls
}

// Set TLS certificate file path
func (client *Authentication_Client) SetCertFile(certFile string) {
	client.certFile = certFile
}

// Set TLS key file path
func (client *Authentication_Client) SetKeyFile(keyFile string) {
	client.keyFile = keyFile
}

// Set TLS authority trust certificate file path
func (client *Authentication_Client) SetCaFile(caFile string) {
	client.caFile = caFile
}

////////////////// Api //////////////////////

// Authenticate a user.
func (client *Authentication_Client) Authenticate(name string, password string) (string, error) {

	// Get the mac address of the server.
	macAddress, err := config.GetMacAddress()
	if err != nil {
		log.Println("fail to get mac address with error ", err)
		return "", err
	}

	// In case of other domain than localhost I will rip off the token file
	// before each authentication.
	err = Utility.CreateDirIfNotExist(tokensPath)
	if err != nil {
		log.Println("fail to create dir ", tokensPath, " with error ", err)
		return "", err
	}

	rqst := &authenticationpb.AuthenticateRqst{
		Name:     name,
		Password: password,
		Issuer:   macAddress,
	}

	rsp, err := client.c.Authenticate(client.GetCtx(), rqst)
	if err != nil {
		log.Println("fail to authenticate ", name, " on domain ", client.GetAddress(), " with error ", err)
		return "", err
	}

	if len(rsp.Token) == 0 {
		return "", fmt.Errorf("fail to authenticate %s on domain %s", name, client.GetAddress())
	}

	return rsp.Token, nil
}

/**
 *  Generate a new token from expired one.
 */
func (client *Authentication_Client) RefreshToken(token string) (string, error) {
	rqst := new(authenticationpb.RefreshTokenRqst)
	rqst.Token = token

	rsp, err := client.c.RefreshToken(client.GetCtx(), rqst)
	if err != nil {
		return "", err
	}

	return rsp.Token, nil
}

/**
 * Set account password.
 */
func (client *Authentication_Client) SetPassword(user, old_password, new_password string) (string, error) {

	rqst := new(authenticationpb.SetPasswordRequest)
	rqst.OldPassword = old_password
	rqst.NewPassword = new_password
	rqst.AccountId = user

	rsp, err := client.c.SetPassword(client.GetCtx(), rqst)
	if err != nil {
		return "", err
	}

	return rsp.Token, nil
}

func (client *Authentication_Client) SetRootPassword(old_password, new_password string) (string, error) {

	rqst := new(authenticationpb.SetRootPasswordRequest)
	rqst.OldPassword = old_password
	rqst.NewPassword = new_password

	rsp, err := client.c.SetRootPassword(client.GetCtx(), rqst)
	if err != nil {
		return "", err
	}

	return rsp.Token, nil
}

/**
 * Set the Root email.
 */
func (client *Authentication_Client) SetRootEmail(oldEmail, newEmail string) error {

	rqst := new(authenticationpb.SetRootEmailRequest)
	rqst.NewEmail = newEmail
	rqst.OldEmail = oldEmail

	_, err := client.c.SetRootEmail(client.GetCtx(), rqst)
	if err != nil {
		return err
	}

	return nil
}

/**
 * Return an error if the token is not valid.
 */
func (client *Authentication_Client) ValidateToken(token string) (string, int64, error) {
	rqst := new(authenticationpb.ValidateTokenRqst)

	rqst.Token = token

	rsp, err := client.c.ValidateToken(client.GetCtx(), rqst)
	if err != nil {
		return "", -1, err
	}

	return rsp.ClientId, rsp.Expired, nil
}
