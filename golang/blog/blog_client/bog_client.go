package blog_client

import (
	"errors"
	"strconv"

	"context"

	//"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/blog/blogpb"
	globular "github.com/globulario/services/golang/globular_client"
	"github.com/globulario/services/golang/security"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

////////////////////////////////////////////////////////////////////////////////
// echo Client Service
////////////////////////////////////////////////////////////////////////////////

type Blog_Client struct {
	cc *grpc.ClientConn
	c  blogpb.BlogServiceClient

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
func NewBlogService_Client(address string, id string) (*Blog_Client, error) {
	client := new(Blog_Client)
	err := globular.InitClient(client, address, id)
	if err != nil {
		return nil, err
	}
	client.cc, err = globular.GetClientConnection(client)
	if err != nil {
		return nil, err
	}
	client.c = blogpb.NewBlogServiceClient(client.cc)

	return client, nil
}

// Return the configuration from the configuration server.
func (client *Blog_Client) GetConfiguration(address string) (map[string]interface{}, error) {
	return nil, errors.New("no implemented...")
}

func (client *Blog_Client) Invoke(method string, rqst interface{}, ctx context.Context) (interface{}, error) {
	if ctx == nil {
		ctx = client.GetCtx()
	}
	return globular.InvokeClientRequest(client.c, ctx, method, rqst)
}

func (client *Blog_Client) GetCtx() context.Context {
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
func (client *Blog_Client) GetDomain() string {
	return client.domain
}

// Return the address
func (client *Blog_Client) GetAddress() string {
	return client.domain + ":" + strconv.Itoa(client.port)
}

// Return the id of the service instance
func (client *Blog_Client) GetId() string {
	return client.id
}

// Return the name of the service
func (client *Blog_Client) GetName() string {
	return client.name
}

func (client *Blog_Client) GetMac() string {
	return client.mac
}

// must be close when no more needed.
func (client *Blog_Client) Close() {
	client.cc.Close()
}

// Set grpc_service port.
func (client *Blog_Client) SetPort(port int) {
	client.port = port
}

// Set the client instance id.
func (client *Blog_Client) SetId(id string) {
	client.id = id
}

// Set the client name.
func (client *Blog_Client) SetName(name string) {
	client.name = name
}

func (client *Blog_Client) SetMac(mac string) {
	client.mac = mac
}

// Set the domain.
func (client *Blog_Client) SetDomain(domain string) {
	client.domain = domain
}

////////////////// TLS ///////////////////

// Get if the client is secure.
func (client *Blog_Client) HasTLS() bool {
	return client.hasTLS
}

// Get the TLS certificate file path
func (client *Blog_Client) GetCertFile() string {
	return client.certFile
}

// Get the TLS key file path
func (client *Blog_Client) GetKeyFile() string {
	return client.keyFile
}

// Get the TLS key file path
func (client *Blog_Client) GetCaFile() string {
	return client.caFile
}

// Set the client is a secure client.
func (client *Blog_Client) SetTLS(hasTls bool) {
	client.hasTLS = hasTls
}

// Set TLS certificate file path
func (client *Blog_Client) SetCertFile(certFile string) {
	client.certFile = certFile
}

// Set TLS key file path
func (client *Blog_Client) SetKeyFile(keyFile string) {
	client.keyFile = keyFile
}

// Set TLS authority trust certificate file path
func (client *Blog_Client) SetCaFile(caFile string) {
	client.caFile = caFile
}

////////////////// Api //////////////////////
