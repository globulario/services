package ldap_client

import (
	// "context"
	// "log"
	"strconv"

	"encoding/json"

	"context"

	globular "github.com/globulario/services/golang/globular_client"
	"github.com/globulario/services/golang/ldap/ldappb"
	"google.golang.org/grpc"
)

////////////////////////////////////////////////////////////////////////////////
// LDAP Client Service
////////////////////////////////////////////////////////////////////////////////

type LDAP_Client struct {
	cc *grpc.ClientConn
	c  ldappb.LdapServiceClient

	// The id of the service on the server.
	id string

	// The mac address of the server
	mac string

	// The name of the service
	name string

	// The ipv4 address
	addresse string

	// The client domain
	domain string

	// The port number
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
func NewLdapService_Client(address string, id string) (*LDAP_Client, error) {
	client := new(LDAP_Client)
	err := globular.InitClient(client, address, id)
	if err != nil {
		return nil, err
	}
	client.cc, err = globular.GetClientConnection(client)
	if err != nil {
		return nil, err
	}
	client.c = ldappb.NewLdapServiceClient(client.cc)

	return client, nil
}

func (ldap_client *LDAP_Client) Invoke(method string, rqst interface{}, ctx context.Context) (interface{}, error) {
	if ctx == nil {
		ctx = globular.GetClientContext(ldap_client)
	}
	return globular.InvokeClientRequest(ldap_client.c, ctx, method, rqst)
}

// Return the domain
func (ldap_client *LDAP_Client) GetDomain() string {
	return ldap_client.domain
}

func (ldap_client *LDAP_Client) GetAddress() string {
	return ldap_client.domain + ":" + strconv.Itoa(ldap_client.port)
}

// Return the id of the service
func (ldap_client *LDAP_Client) GetId() string {
	return ldap_client.id
}

// Return the name of the service
func (ldap_client *LDAP_Client) GetName() string {
	return ldap_client.name
}

func (ldap_client *LDAP_Client) GetMac() string {
	return ldap_client.mac
}

// must be close when no more needed.
func (ldap_client *LDAP_Client) Close() {
	ldap_client.cc.Close()
}

// Set grpc_service port.
func (ldap_client *LDAP_Client) SetPort(port int) {
	ldap_client.port = port
}

// Set the client id.
func (ldap_client *LDAP_Client) SetId(id string) {
	ldap_client.id = id
}

func (ldap_client *LDAP_Client) SetMac(mac string) {
	ldap_client.mac = mac
}

// Set the client name.
func (ldap_client *LDAP_Client) SetName(name string) {
	ldap_client.name = name
}

// Set the domain.
func (ldap_client *LDAP_Client) SetDomain(domain string) {
	ldap_client.domain = domain
}

////////////////// TLS ///////////////////

// Get if the client is secure.
func (ldap_client *LDAP_Client) HasTLS() bool {
	return ldap_client.hasTLS
}

// Get the TLS certificate file path
func (ldap_client *LDAP_Client) GetCertFile() string {
	return ldap_client.certFile
}

// Get the TLS key file path
func (ldap_client *LDAP_Client) GetKeyFile() string {
	return ldap_client.keyFile
}

// Get the TLS key file path
func (ldap_client *LDAP_Client) GetCaFile() string {
	return ldap_client.caFile
}

// Set the client is a secure client.
func (ldap_client *LDAP_Client) SetTLS(hasTls bool) {
	ldap_client.hasTLS = hasTls
}

// Set TLS certificate file path
func (ldap_client *LDAP_Client) SetCertFile(certFile string) {
	ldap_client.certFile = certFile
}

// Set TLS key file path
func (ldap_client *LDAP_Client) SetKeyFile(keyFile string) {
	ldap_client.keyFile = keyFile
}

// Set TLS authority trust certificate file path
func (ldap_client *LDAP_Client) SetCaFile(caFile string) {
	ldap_client.caFile = caFile
}

////////////////////////// LDAP ////////////////////////////////////////////////
// Stop the service.
func (ldap_client *LDAP_Client) StopService() {
	ldap_client.c.Stop(globular.GetClientContext(ldap_client), &ldappb.StopRequest{})
}

func (ldap_client *LDAP_Client) CreateConnection(connectionId string, user string, password string, host string, port int32) error {
	// Create a new connection
	rqst := &ldappb.CreateConnectionRqst{
		Connection: &ldappb.Connection{
			Id:       connectionId,
			User:     user,
			Password: password,
			Port:     port,
			Host:     host, //"mon-dc-p01.UD6.UF6",
		},
	}

	_, err := ldap_client.c.CreateConnection(globular.GetClientContext(ldap_client), rqst)

	return err
}

func (ldap_client *LDAP_Client) DeleteConnection(connectionId string) error {

	rqst := &ldappb.DeleteConnectionRqst{
		Id: connectionId,
	}

	_, err := ldap_client.c.DeleteConnection(globular.GetClientContext(ldap_client), rqst)

	return err
}

func (ldap_client *LDAP_Client) Authenticate(connectionId string, userId string, password string) error {

	rqst := &ldappb.AuthenticateRqst{
		Id:    connectionId,
		Login: userId,
		Pwd:   password,
	}

	_, err := ldap_client.c.Authenticate(globular.GetClientContext(ldap_client), rqst)
	return err
}

func (ldap_client *LDAP_Client) Search(connectionId string, BaseDN string, Filter string, Attributes []string) ([][]interface{}, error) {

	// I will execute a simple ldap search here...
	rqst := &ldappb.SearchRqst{
		Search: &ldappb.Search{
			Id:         connectionId,
			BaseDN:     BaseDN,
			Filter:     Filter,
			Attributes: Attributes,
		},
	}

	rsp, err := ldap_client.c.Search(globular.GetClientContext(ldap_client), rqst)
	if err != nil {
		return nil, err
	}

	values := make([][]interface{}, 0)
	err = json.Unmarshal([]byte(rsp.Result), &values)

	return values, err

}
