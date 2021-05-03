package ca_client

import (
	"strconv"

	"context"

	"github.com/globulario/services/golang/ca/capb"
	globular "github.com/globulario/services/golang/globular_client"
	"google.golang.org/grpc"
)

////////////////////////////////////////////////////////////////////////////////
// Ca Client Service
////////////////////////////////////////////////////////////////////////////////

type Ca_Client struct {
	cc *grpc.ClientConn
	c  capb.CertificateAuthorityClient

	// The id of the service
	id string

	// The name of the service
	name string

	// The port
	port int

	// The client domain
	domain string

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
func NewCaService_Client(address string, id string) (*Ca_Client, error) {
	client := new(Ca_Client)

	err := globular.InitClient(client, address, id)
	if err != nil {
		return nil, err
	}
	client.cc, err = globular.GetClientConnection(client)
	if err != nil {
		return nil, err
	}
	client.c = capb.NewCertificateAuthorityClient(client.cc)

	return client, nil
}

func (ca_client *Ca_Client) Invoke(method string, rqst interface{}, ctx context.Context) (interface{}, error) {
	if ctx == nil {
		ctx = globular.GetClientContext(ca_client)
	}
	return globular.InvokeClientRequest(ca_client.c, ctx, method, rqst)
}

// Return the address
func (ca_client *Ca_Client) GetAddress() string {
	return ca_client.domain + ":" + strconv.Itoa(ca_client.port)
}

// Return the domain
func (ca_client *Ca_Client) GetDomain() string {
	return ca_client.domain
}

// Return the id of the service instance
func (ca_client *Ca_Client) GetId() string {
	return ca_client.id
}

// Return the name of the service
func (ca_client *Ca_Client) GetName() string {
	return ca_client.name
}

// must be close when no more needed.
func (ca_client *Ca_Client) Close() {
	ca_client.cc.Close()
}

// Set grpc_service port.
func (ca_client *Ca_Client) SetPort(port int) {
	ca_client.port = port
}

// Set the client instance id.
func (ca_client *Ca_Client) SetId(id string) {
	ca_client.id = id
}

// Set the client name.
func (ca_client *Ca_Client) SetName(name string) {
	ca_client.name = name
}

// Set the domain.
func (ca_client *Ca_Client) SetDomain(domain string) {
	ca_client.domain = domain
}

////////////////// TLS ///////////////////

// Get if the client is secure.
func (ca_client *Ca_Client) HasTLS() bool {
	return ca_client.hasTLS
}

// Get the TLS certificate file path
func (ca_client *Ca_Client) GetCertFile() string {
	return ca_client.certFile
}

// Get the TLS key file path
func (ca_client *Ca_Client) GetKeyFile() string {
	return ca_client.keyFile
}

// Get the TLS key file path
func (ca_client *Ca_Client) GetCaFile() string {
	return ca_client.caFile
}

// Set the client is a secure client.
func (ca_client *Ca_Client) SetTLS(hasTls bool) {
	ca_client.hasTLS = hasTls
}

// Set TLS certificate file path
func (ca_client *Ca_Client) SetCertFile(certFile string) {
	ca_client.certFile = certFile
}

// Set TLS key file path
func (ca_client *Ca_Client) SetKeyFile(keyFile string) {
	ca_client.keyFile = keyFile
}

// Set TLS authority trust certificate file path
func (ca_client *Ca_Client) SetCaFile(caFile string) {
	ca_client.caFile = caFile
}

////////////////////////////////////////////////////////////////////////////////
// CA functions.
////////////////////////////////////////////////////////////////////////////////

/**
 * Take signing request and made it sign by the server. If succed a signed
 * certificate is return.
 */
func (ca_client *Ca_Client) SignCertificate(csr string) (string, error) {
	// The certificate request.
	rqst := new(capb.SignCertificateRequest)
	rqst.Csr = csr

	rsp, err := ca_client.c.SignCertificate(globular.GetClientContext(ca_client), rqst)
	if err == nil {
		return rsp.Crt, nil
	}

	return "", err
}

/**
 * Get the ca.crt file content.
 */
func (ca_client *Ca_Client) GetCaCertificate() (string, error) {
	rqst := new(capb.GetCaCertificateRequest)

	rsp, err := ca_client.c.GetCaCertificate(globular.GetClientContext(ca_client), rqst)
	if err == nil {
		return rsp.Ca, nil
	}
	return "", err
}
