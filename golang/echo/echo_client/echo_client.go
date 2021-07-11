package echo_client

import (
	"strconv"

	"context"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/echo/echopb"
	globular "github.com/globulario/services/golang/globular_client"
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

func (echo_client *Echo_Client) Invoke(method string, rqst interface{}, ctx context.Context) (interface{}, error) {
	if ctx == nil {
		ctx = globular.GetClientContext(echo_client)
	}
	return globular.InvokeClientRequest(echo_client.c, ctx, method, rqst)
}

// Return the domain
func (echo_client *Echo_Client) GetDomain() string {
	return echo_client.domain
}

// Return the address
func (echo_client *Echo_Client) GetAddress() string {
	return echo_client.domain + ":" + strconv.Itoa(echo_client.port)
}

// Return the id of the service instance
func (echo_client *Echo_Client) GetId() string {
	return echo_client.id
}

// Return the name of the service
func (echo_client *Echo_Client) GetName() string {
	return echo_client.name
}

func (echo_client *Echo_Client) GetMac() string {
	return echo_client.mac
}

// must be close when no more needed.
func (echo_client *Echo_Client) Close() {
	echo_client.cc.Close()
}

// Set grpc_service port.
func (echo_client *Echo_Client) SetPort(port int) {
	echo_client.port = port
}

// Set the client instance id.
func (echo_client *Echo_Client) SetId(id string) {
	echo_client.id = id
}

// Set the client name.
func (echo_client *Echo_Client) SetName(name string) {
	echo_client.name = name
}

func (echo_client *Echo_Client) SetMac(mac string) {
	echo_client.mac = mac
}

// Set the domain.
func (echo_client *Echo_Client) SetDomain(domain string) {
	echo_client.domain = domain
}

////////////////// TLS ///////////////////

// Get if the client is secure.
func (echo_client *Echo_Client) HasTLS() bool {
	return echo_client.hasTLS
}

// Get the TLS certificate file path
func (echo_client *Echo_Client) GetCertFile() string {
	return echo_client.certFile
}

// Get the TLS key file path
func (echo_client *Echo_Client) GetKeyFile() string {
	return echo_client.keyFile
}

// Get the TLS key file path
func (echo_client *Echo_Client) GetCaFile() string {
	return echo_client.caFile
}

// Set the client is a secure client.
func (echo_client *Echo_Client) SetTLS(hasTls bool) {
	echo_client.hasTLS = hasTls
}

// Set TLS certificate file path
func (echo_client *Echo_Client) SetCertFile(certFile string) {
	echo_client.certFile = certFile
}

// Set TLS key file path
func (echo_client *Echo_Client) SetKeyFile(keyFile string) {
	echo_client.keyFile = keyFile
}

// Set TLS authority trust certificate file path
func (echo_client *Echo_Client) SetCaFile(caFile string) {
	echo_client.caFile = caFile
}

////////////////// Api //////////////////////
// Stop the service.
func (echo_client *Echo_Client) StopService() {
	echo_client.c.Stop(globular.GetClientContext(echo_client), &echopb.StopRequest{})
}

func (echo_client *Echo_Client) Echo(token string, msg interface{}) (string, error) {

	rqst := &echopb.EchoRequest{
		Message: Utility.ToString(msg),
	}

	ctx := globular.GetClientContext(echo_client)
	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)
		md.Append("token", string(token))
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	rsp, err := echo_client.c.Echo(ctx, rqst)
	if err != nil {
		return "", err
	}
	return rsp.Message, nil
}
