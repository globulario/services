package applications_manager_client

import (
	"context"
	"strings"
	"time"

	"github.com/globulario/services/golang/applications_manager/applications_managerpb"
	globular "github.com/globulario/services/golang/globular_client"
	"github.com/globulario/services/golang/security"

	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

////////////////////////////////////////////////////////////////////////////////
// echo Client Service
////////////////////////////////////////////////////////////////////////////////

type Applications_Manager_Client struct {
	cc *grpc.ClientConn
	c  applications_managerpb.ApplicationManagerServiceClient

	// The id of the service
	id string

	// The mac address of the server
	mac string

	// The name of the service
	name string

	// The client domain
	domain string

	//  keep the last connection state of the client.
	state string

	// The address where connection with client can be done. ex: globule0.globular.cloud:10101
	address string

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
func NewApplicationsManager_Client(address string, id string) (*Applications_Manager_Client, error) {
	client := new(Applications_Manager_Client)
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

func (client *Applications_Manager_Client) Reconnect() error {
	var err error
	nb_try_connect := 10

	for i := 0; i < nb_try_connect; i++ {
		client.cc, err = globular.GetClientConnection(client)
		if err == nil {
			client.c = applications_managerpb.NewApplicationManagerServiceClient(client.cc)
			break
		}

		// wait 500 millisecond before next try
		time.Sleep(500 * time.Millisecond)
	}

	return err

}

// The address where the client can connect.
func (client *Applications_Manager_Client) SetAddress(address string) {
	client.address = address
}

func (Applications_Manager_Client *Applications_Manager_Client) Invoke(method string, rqst interface{}, ctx context.Context) (interface{}, error) {
	if ctx == nil {
		ctx = Applications_Manager_Client.GetCtx()
	}
	return globular.InvokeClientRequest(Applications_Manager_Client.c, ctx, method, rqst)
}

func (client *Applications_Manager_Client) GetCtx() context.Context {
	if client.ctx == nil {
		client.ctx = globular.GetClientContext(client)
	}

	// refresh the client as needed...
	token, err := security.GetLocalToken(client.GetMac())
	if err == nil {
		md := metadata.New(map[string]string{"token": string(token), "domain": client.domain, "mac": client.GetMac(), "address": client.GetAddress()})
		client.ctx = metadata.NewOutgoingContext(client.ctx, md)
	}

	return client.ctx
}

// Return the domain
func (Applications_Manager_Client *Applications_Manager_Client) GetDomain() string {
	return Applications_Manager_Client.domain
}

// Return the address
func (Applications_Manager_Client *Applications_Manager_Client) GetAddress() string {
	return Applications_Manager_Client.address
}

// Return the id of the service instance
func (Applications_Manager_Client *Applications_Manager_Client) GetId() string {
	return Applications_Manager_Client.id
}

// Return the name of the service
func (Applications_Manager_Client *Applications_Manager_Client) GetName() string {
	return Applications_Manager_Client.name
}

func (Applications_Manager_Client *Applications_Manager_Client) GetMac() string {
	return Applications_Manager_Client.mac
}

// Return the last know connection state
func (client *Applications_Manager_Client) GetState() string {
	return client.state
}

// must be close when no more needed.
func (Applications_Manager_Client *Applications_Manager_Client) Close() {
	Applications_Manager_Client.cc.Close()
}

// Set grpc_service port.
func (Applications_Manager_Client *Applications_Manager_Client) SetPort(port int) {
	Applications_Manager_Client.port = port
}

// Return the grpc port number
func (client *Applications_Manager_Client) GetPort() int {
	return client.port
}

// Set the client instance id.
func (Applications_Manager_Client *Applications_Manager_Client) SetId(id string) {
	Applications_Manager_Client.id = id
}

// Set the client name.
func (Applications_Manager_Client *Applications_Manager_Client) SetName(name string) {
	Applications_Manager_Client.name = name
}

func (Applications_Manager_Client *Applications_Manager_Client) SetMac(mac string) {
	Applications_Manager_Client.mac = mac
}

// Set the domain.
func (Applications_Manager_Client *Applications_Manager_Client) SetDomain(domain string) {
	Applications_Manager_Client.domain = domain
}

func (client *Applications_Manager_Client) SetState(state string) {
	client.state = state
}

////////////////// TLS ///////////////////

// Get if the client is secure.
func (Applications_Manager_Client *Applications_Manager_Client) HasTLS() bool {
	return Applications_Manager_Client.hasTLS
}

// Get the TLS certificate file path
func (Applications_Manager_Client *Applications_Manager_Client) GetCertFile() string {
	return Applications_Manager_Client.certFile
}

// Get the TLS key file path
func (Applications_Manager_Client *Applications_Manager_Client) GetKeyFile() string {
	return Applications_Manager_Client.keyFile
}

// Get the TLS key file path
func (Applications_Manager_Client *Applications_Manager_Client) GetCaFile() string {
	return Applications_Manager_Client.caFile
}

// Set the client is a secure client.
func (Applications_Manager_Client *Applications_Manager_Client) SetTLS(hasTls bool) {
	Applications_Manager_Client.hasTLS = hasTls
}

// Set TLS certificate file path
func (Applications_Manager_Client *Applications_Manager_Client) SetCertFile(certFile string) {
	Applications_Manager_Client.certFile = certFile
}

// Set TLS key file path
func (Applications_Manager_Client *Applications_Manager_Client) SetKeyFile(keyFile string) {
	Applications_Manager_Client.keyFile = keyFile
}

// Set TLS authority trust certificate file path
func (Applications_Manager_Client *Applications_Manager_Client) SetCaFile(caFile string) {
	Applications_Manager_Client.caFile = caFile
}

////////////////// Api //////////////////////

/**
 * Intall a new application or update an existing one.
 */
func (client *Applications_Manager_Client) InstallApplication(token, domain, user, discoveryId, PublisherID, applicationId string, set_as_default bool) error {

	rqst := new(applications_managerpb.InstallApplicationRequest)
	rqst.DiscoveryId = discoveryId
	rqst.PublisherID = PublisherID
	rqst.ApplicationId = applicationId
	rqst.Domain = domain
	rqst.SetAsDefault = set_as_default
	ctx := client.GetCtx()

	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)
		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(ctx, md)
	}

	_, err := client.c.InstallApplication(ctx, rqst)
	return err
}

/**
 * Uninstall application, if no version is given the most recent version will
 * be install.
 */
func (client *Applications_Manager_Client) UninstallApplication(token string, domain string, user string, PublisherID string, applicationId string, version string) error {

	rqst := new(applications_managerpb.UninstallApplicationRequest)
	rqst.PublisherID = PublisherID
	rqst.ApplicationId = applicationId
	rqst.Version = version
	rqst.Domain = strings.Split(domain, ":")[0] // remove the port if one is given...
	ctx := client.GetCtx()

	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)
		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(ctx, md)
	}

	_, err := client.c.UninstallApplication(ctx, rqst)

	return err
}
