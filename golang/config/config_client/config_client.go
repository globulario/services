package config_client

import (
	"strconv"

	"context"

	// "github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/config/configpb"
	globular "github.com/globulario/services/golang/globular_client"
	"github.com/globulario/services/golang/security"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

////////////////////////////////////////////////////////////////////////////////
// config Client Service
////////////////////////////////////////////////////////////////////////////////

type Config_Client struct {
	cc *grpc.ClientConn
	c  configpb.ConfigServiceClient

	// The id of the service
	id string

	// The name of the service
	name string

	// The client domain
	domain string

	// The mac address of the server
	mac string

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
func NewConfigService_Client(address string, id string) (*Config_Client, error) {
	client := new(Config_Client)
	err := globular.InitClient(client, address, id)
	if err != nil {
		return nil, err
	}
	client.cc, err = globular.GetClientConnection(client)
	if err != nil {
		return nil, err
	}
	client.c = configpb.NewConfigServiceClient(client.cc)

	return client, nil
}

func (client *Config_Client) Invoke(method string, rqst interface{}, ctx context.Context) (interface{}, error) {
	if ctx == nil {
		ctx = client.GetCtx()
	}
	return globular.InvokeClientRequest(client.c, ctx, method, rqst)
}

func (client *Config_Client) GetCtx() context.Context {
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

// Return the domain
func (client *Config_Client) GetDomain() string {
	return client.domain
}

// Return the address
func (client *Config_Client) GetAddress() string {
	return client.domain + ":" + strconv.Itoa(client.port)
}

// Return the id of the service instance
func (client *Config_Client) GetId() string {
	return client.id
}

// Return the name of the service
func (client *Config_Client) GetName() string {
	return client.name
}

func (client *Config_Client) GetMac() string {
	return client.mac
}

// must be close when no more needed.
func (client *Config_Client) Close() {
	client.cc.Close()
}

// Set grpc_service port.
func (client *Config_Client) SetPort(port int) {
	client.port = port
}

// Set the client instance id.
func (client *Config_Client) SetId(id string) {
	client.id = id
}

// Set the client name.
func (client *Config_Client) SetName(name string) {
	client.name = name
}

func (client *Config_Client) SetMac(mac string) {
	client.mac = mac
}

// Set the domain.
func (client *Config_Client) SetDomain(domain string) {
	client.domain = domain
}

////////////////// TLS ///////////////////

// Get if the client is secure.
func (client *Config_Client) HasTLS() bool {
	return client.hasTLS
}

// Get the TLS certificate file path
func (client *Config_Client) GetCertFile() string {
	return client.certFile
}

// Get the TLS key file path
func (client *Config_Client) GetKeyFile() string {
	return client.keyFile
}

// Get the TLS key file path
func (client *Config_Client) GetCaFile() string {
	return client.caFile
}

// Set the client is a secure client.
func (client *Config_Client) SetTLS(hasTls bool) {
	client.hasTLS = hasTls
}

// Set TLS certificate file path
func (client *Config_Client) SetCertFile(certFile string) {
	client.certFile = certFile
}

// Set TLS key file path
func (client *Config_Client) SetKeyFile(keyFile string) {
	client.keyFile = keyFile
}

// Set TLS authority trust certificate file path
func (client *Config_Client) SetCaFile(caFile string) {
	client.caFile = caFile
}

////////////////// Api //////////////////////
// Specific config client function here.
func (client *Config_Client) GetServiceConfiguration(path string) (map[string]interface{}, error){
	// TODO implement it.
	return nil, nil
}