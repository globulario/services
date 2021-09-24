package echo_client

import (
	"strconv"

	"context"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/echo/echopb"
	globular "github.com/globulario/services/golang/globular_client"
	"github.com/globulario/services/golang/security"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

////////////////////////////////////////////////////////////////////////////////
// echo Client Service
////////////////////////////////////////////////////////////////////////////////

type Echo_Client struct {
	cc *grpc.ClientConn
	c  echopb.EchoServiceClient

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
func NewEchoService_Client(address string, id string) (*Echo_Client, error) {
	client := new(Echo_Client)
	err := globular.InitClient(client, address, id)
	if err != nil {
		return nil, err
	}
	client.cc, err = globular.GetClientConnection(client)
	if err != nil {
		return nil, err
	}
	client.c = echopb.NewEchoServiceClient(client.cc)

	return client, nil
}

func (client *Echo_Client) Invoke(method string, rqst interface{}, ctx context.Context) (interface{}, error) {
	if ctx == nil {
		ctx = client.GetCtx()
	}
	return globular.InvokeClientRequest(client.c, ctx, method, rqst)
}

func (client *Echo_Client) GetCtx() context.Context {
	if client.ctx == nil {
		client.ctx = globular.GetClientContext(client)
	}
	token, err := security.GetLocalToken(client.GetDomain())
	if err == nil {
		md := metadata.New(map[string]string{"token": string(token), "domain": client.domain, "mac": client.GetMac()})
		client.ctx = metadata.NewOutgoingContext(context.Background(), md)
	}
	return client.ctx
}

// Return the domain
func (client *Echo_Client) GetDomain() string {
	return client.domain
}

// Return the address
func (client *Echo_Client) GetAddress() string {
	return client.domain + ":" + strconv.Itoa(client.port)
}

// Return the id of the service instance
func (client *Echo_Client) GetId() string {
	return client.id
}

// Return the name of the service
func (client *Echo_Client) GetName() string {
	return client.name
}

func (client *Echo_Client) GetMac() string {
	return client.mac
}

// must be close when no more needed.
func (client *Echo_Client) Close() {
	client.cc.Close()
}

// Set grpc_service port.
func (client *Echo_Client) SetPort(port int) {
	client.port = port
}

// Set the client instance id.
func (client *Echo_Client) SetId(id string) {
	client.id = id
}

// Set the client name.
func (client *Echo_Client) SetName(name string) {
	client.name = name
}

func (client *Echo_Client) SetMac(mac string) {
	client.mac = mac
}

// Set the domain.
func (client *Echo_Client) SetDomain(domain string) {
	client.domain = domain
}

////////////////// TLS ///////////////////

// Get if the client is secure.
func (client *Echo_Client) HasTLS() bool {
	return client.hasTLS
}

// Get the TLS certificate file path
func (client *Echo_Client) GetCertFile() string {
	return client.certFile
}

// Get the TLS key file path
func (client *Echo_Client) GetKeyFile() string {
	return client.keyFile
}

// Get the TLS key file path
func (client *Echo_Client) GetCaFile() string {
	return client.caFile
}

// Set the client is a secure client.
func (client *Echo_Client) SetTLS(hasTls bool) {
	client.hasTLS = hasTls
}

// Set TLS certificate file path
func (client *Echo_Client) SetCertFile(certFile string) {
	client.certFile = certFile
}

// Set TLS key file path
func (client *Echo_Client) SetKeyFile(keyFile string) {
	client.keyFile = keyFile
}

// Set TLS authority trust certificate file path
func (client *Echo_Client) SetCaFile(caFile string) {
	client.caFile = caFile
}

////////////////// Api //////////////////////
// Stop the service.
func (client *Echo_Client) StopService() {
	client.c.Stop(client.GetCtx(), &echopb.StopRequest{})
}

func (client *Echo_Client) Echo(token string, msg interface{}) (string, error) {

	rqst := &echopb.EchoRequest{
		Message: Utility.ToString(msg),
	}

	ctx := client.GetCtx()
	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)
		md.Append("token", string(token))
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	rsp, err := client.c.Echo(ctx, rqst)
	if err != nil {
		return "", err
	}
	return rsp.Message, nil
}
