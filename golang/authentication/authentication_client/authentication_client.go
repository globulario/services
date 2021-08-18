package authentication_client

import (
	"context"
	"io/ioutil"
	"log"
	"os"
	"strconv"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/authentication/authenticationpb"
	globular "github.com/globulario/services/golang/globular_client"
	"google.golang.org/grpc"
)

////////////////////////////////////////////////////////////////////////////////
// Authentication  Client Service
////////////////////////////////////////////////////////////////////////////////
var (
	tokensPath = "/etc/globular/config/tokens"
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
}

// Create a connection to the service.
func NewAuthenticationService_Client(address string, id string) (*Authentication_Client, error) {
	client := new(Authentication_Client)
	err := globular.InitClient(client, address, id)
	if err != nil {
		return nil, err
	}
	client.cc, err = globular.GetClientConnection(client)
	if err != nil {
		return nil, err
	}
	client.c = authenticationpb.NewAuthenticationServiceClient(client.cc)

	return client, nil
}

func (client *Authentication_Client) Invoke(method string, rqst interface{}, ctx context.Context) (interface{}, error) {
	if ctx == nil {
		ctx = globular.GetClientContext(client)
	}
	return globular.InvokeClientRequest(client.c, ctx, method, rqst)
}

// Return the domain
func (client *Authentication_Client) GetDomain() string {
	return client.domain
}

// Return the address
func (client *Authentication_Client) GetAddress() string {
	return client.domain + ":" + strconv.Itoa(client.port)
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
	// In case of other domain than localhost I will rip off the token file
	// before each authentication.
	folderPath := "/Program Files/Globular"
	if Utility.Exists(folderPath) {
		tokensPath = folderPath + tokensPath
	}

	err := Utility.CreateDirIfNotExist(tokensPath)
	if err != nil {
		log.Println("fail to create dir ", tokensPath, " with error ", err)
		return "",  err
	}
	
	path := tokensPath + "/" + client.GetDomain() + "_token"

	rqst := &authenticationpb.AuthenticateRqst{
		Name:     name,
		Password: password,
	}

	log.Println("Authenticate", name," on domain ", client.GetDomain() )

	rsp, err := client.c.Authenticate(globular.GetClientContext(client), rqst)
	if err != nil {
		log.Println("fail to authenticate!")
		return "", err
	}

	// Here I will save the token into the temporary directory the token will be valid for a given time (default is 15 minutes)
	// it's the responsability of the client to keep it refresh... see Refresh token from the server...
	if !Utility.IsLocal(client.GetDomain()) {
		// remove the file if it already exist.
		if Utility.Exists(path) {
			err := os.Remove(path)
			if err != nil {
				return "", err
			}
		}
		// create the new token for the domain.
		err = ioutil.WriteFile(path, []byte(rsp.Token), 0644)
		if err != nil {
			return "", err
		}
	}

	return rsp.Token, nil
}

/**
 *  Generate a new token from expired one.
 */
func (client *Authentication_Client) RefreshToken(token string) (string, error) {
	rqst := new(authenticationpb.RefreshTokenRqst)
	rqst.Token = token

	rsp, err := client.c.RefreshToken(globular.GetClientContext(client), rqst)
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

	rsp, err := client.c.SetPassword(globular.GetClientContext(client), rqst)
	if err != nil {
		return "", err
	}

	return rsp.Token, nil
}

func (client *Authentication_Client) SetRootPassword(old_password, new_password string) (string, error) {

	rqst := new(authenticationpb.SetRootPasswordRequest)
	rqst.OldPassword = old_password
	rqst.NewPassword = new_password

	rsp, err := client.c.SetRootPassword(globular.GetClientContext(client), rqst)
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

	_, err := client.c.SetRootEmail(globular.GetClientContext(client), rqst)
	if err != nil {
		return err
	}

	return nil
}

/**
 * Return an error if the token is not valid.
 */
func (client *Authentication_Client) ValidateToken(token string) error {
	rqst := new(authenticationpb.ValidateTokenRqst)

	rqst.Token = token

	_, err := client.c.ValidateToken(globular.GetClientContext(client), rqst)
	if err != nil {
		return err
	}

	return nil
}
