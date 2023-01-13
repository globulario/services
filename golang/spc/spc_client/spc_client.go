package spc_client

import (
	"context"
	"strconv"

	"github.com/globulario/services/golang/config/config_client"
	"github.com/globulario/services/golang/globular_client"
	globular "github.com/globulario/services/golang/globular_client"
	"github.com/globulario/services/golang/security"
	"github.com/globulario/services/golang/spc/spcpb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

////////////////////////////////////////////////////////////////////////////////
// SPC Client Service
////////////////////////////////////////////////////////////////////////////////
type SPC_Client struct {
	cc *grpc.ClientConn
	c  spcpb.SpcServiceClient

	// The id of the service
	id string

	// The mac address of the server
	mac string

	// The name of the service
	name string

	// The domain
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
func NewSpcService_Client(address string, id string) (*SPC_Client, error) {
	client := new(SPC_Client)
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

func (client *SPC_Client) Reconnect () error{
	var err error
	
	client.cc, err = globular.GetClientConnection(client)
	if err != nil {
		return  err
	}

	client.c = spcpb.NewSpcServiceClient(client.cc)

	return nil
}

// The address where the client can connect.
func (client *SPC_Client) SetAddress(address string) {
	client.address = address
}

func (client *SPC_Client) GetConfiguration(address, id string) (map[string]interface{}, error) {
	client_, err := globular_client.GetClient(address, "config.ConfigService", "config_client.NewConfigService_Client")
	if err != nil {
		return nil, err
	}
	return client_.(*config_client.Config_Client).GetServiceConfiguration(id)
}

func (spc_client *SPC_Client) Invoke(method string, rqst interface{}, ctx context.Context) (interface{}, error) {
	if ctx == nil {
		ctx = spc_client.GetCtx()
	}
	return globular.InvokeClientRequest(spc_client.c, ctx, method, rqst)
}

func (client *SPC_Client) GetCtx() context.Context {
	if client.ctx == nil {
		client.ctx = globular.GetClientContext(client)
	}
	token, err := security.GetLocalToken(client.GetMac())
	if err == nil {
		md := metadata.New(map[string]string{"token": string(token), "domain": client.domain, "mac": client.GetMac()})
		client.ctx = metadata.NewOutgoingContext(context.Background(), md)
	}
	return client.ctx
}

// Return the last know connection state
func (client *SPC_Client) GetState() string {
	return client.state
}

// Return the domain
func (spc_client *SPC_Client) GetDomain() string {
	return spc_client.domain
}

// Return the address
func (spc_client *SPC_Client) GetAddress() string {
	return spc_client.domain + ":" + strconv.Itoa(spc_client.port)
}

// Return the id of the service instance
func (spc_client *SPC_Client) GetId() string {
	return spc_client.id
}

// Return the name of the service
func (spc_client *SPC_Client) GetName() string {
	return spc_client.name
}

func (spc_client *SPC_Client) GetMac() string {
	return spc_client.mac
}

// must be close when no more needed.
func (spc_client *SPC_Client) Close() {
	spc_client.cc.Close()
}

// Set grpc_service port.
func (spc_client *SPC_Client) SetPort(port int) {
	spc_client.port = port
}

// Return the grpc port number
func (client *SPC_Client) GetPort() int {
	return client.port
}

// Set the service instance id
func (spc_client *SPC_Client) SetId(id string) {
	spc_client.id = id
}

// Set the client name.
func (spc_client *SPC_Client) SetName(name string) {
	spc_client.name = name
}

func (spc_client *SPC_Client) SetMac(mac string) {
	spc_client.mac = mac
}

// Set the domain.
func (spc_client *SPC_Client) SetDomain(domain string) {
	spc_client.domain = domain
}

func (client *SPC_Client) SetState(state string) {
	client.state = state
}

////////////////// TLS ///////////////////

// Get if the client is secure.
func (spc_client *SPC_Client) HasTLS() bool {
	return spc_client.hasTLS
}

// Get the TLS certificate file path
func (spc_client *SPC_Client) GetCertFile() string {
	return spc_client.certFile
}

// Get the TLS key file path
func (spc_client *SPC_Client) GetKeyFile() string {
	return spc_client.keyFile
}

// Get the TLS key file path
func (spc_client *SPC_Client) GetCaFile() string {
	return spc_client.caFile
}

// Set the client is a secure client.
func (spc_client *SPC_Client) SetTLS(hasTls bool) {
	spc_client.hasTLS = hasTls
}

// Set TLS certificate file path
func (spc_client *SPC_Client) SetCertFile(certFile string) {
	spc_client.certFile = certFile
}

// Set TLS key file path
func (spc_client *SPC_Client) SetKeyFile(keyFile string) {
	spc_client.keyFile = keyFile
}

// Set TLS authority trust certificate file path
func (spc_client *SPC_Client) SetCaFile(caFile string) {
	spc_client.caFile = caFile
}

// Stop the service.
func (spc_client *SPC_Client) StopService() {
	spc_client.c.Stop(globular.GetClientContext(spc_client), &spcpb.StopRequest{})
}
