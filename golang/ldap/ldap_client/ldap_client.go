package ldap_client

import (
	// "context"
	// "log"

	"context"
	"encoding/json"

	"github.com/globulario/services/golang/config/config_client"
	"github.com/globulario/services/golang/globular_client"
	globular "github.com/globulario/services/golang/globular_client"
	"github.com/globulario/services/golang/ldap/ldappb"
	"github.com/globulario/services/golang/security"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
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

	//  keep the last connection state of the client.
	state string

	// The client domain
	domain string

	// The address where connection with client can be done. ex: globule0.globular.cloud:10101
	address string

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

	// The client context
	ctx context.Context
}

// Create a connection to the service.
func NewLdapService_Client(address string, id string) (*LDAP_Client, error) {
	client := new(LDAP_Client)
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

func (client *LDAP_Client) Reconnect () error{
	var err error
	
	client.cc, err = globular.GetClientConnection(client)
	if err != nil {
		return  err
	}

	client.c = ldappb.NewLdapServiceClient(client.cc)
	return nil
}

// The address where the client can connect.
func (client *LDAP_Client) SetAddress(address string) {
	client.address = address
}

// Return the configuration from the configuration server.
func (client *LDAP_Client) GetConfiguration(address, id string) (map[string]interface{}, error) {
	client_, err := globular_client.GetClient(address, "config.ConfigService", "config_client.NewConfigService_Client")
	if err != nil {
		return nil, err
	}
	return client_.(*config_client.Config_Client).GetServiceConfiguration(id)
}

func (client *LDAP_Client) Invoke(method string, rqst interface{}, ctx context.Context) (interface{}, error) {
	if ctx == nil {
		ctx = client.GetCtx()
	}
	return globular.InvokeClientRequest(client.c, ctx, method, rqst)
}

func (client *LDAP_Client) GetCtx() context.Context {
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
func (client *LDAP_Client) GetDomain() string {
	return client.domain
}

func (client *LDAP_Client) GetAddress() string {
	return client.address
}

// Return the last know connection state
func (client *LDAP_Client) GetState() string {
	return client.state
}

// Return the id of the service
func (client *LDAP_Client) GetId() string {
	return client.id
}

// Return the name of the service
func (client *LDAP_Client) GetName() string {
	return client.name
}

func (client *LDAP_Client) GetMac() string {
	return client.mac
}

// must be close when no more needed.
func (client *LDAP_Client) Close() {
	client.cc.Close()
}

// Set grpc_service port.
func (client *LDAP_Client) SetPort(port int) {
	client.port = port
}

// Return the grpc port number
func (client *LDAP_Client) GetPort() int {
	return client.port
}

// Set the client id.
func (client *LDAP_Client) SetId(id string) {
	client.id = id
}

func (client *LDAP_Client) SetMac(mac string) {
	client.mac = mac
}

// Set the client name.
func (client *LDAP_Client) SetName(name string) {
	client.name = name
}

func (client *LDAP_Client) SetState(state string) {
	client.state = state
}

// Set the domain.
func (client *LDAP_Client) SetDomain(domain string) {
	client.domain = domain
}

////////////////// TLS ///////////////////

// Get if the client is secure.
func (client *LDAP_Client) HasTLS() bool {
	return client.hasTLS
}

// Get the TLS certificate file path
func (client *LDAP_Client) GetCertFile() string {
	return client.certFile
}

// Get the TLS key file path
func (client *LDAP_Client) GetKeyFile() string {
	return client.keyFile
}

// Get the TLS key file path
func (client *LDAP_Client) GetCaFile() string {
	return client.caFile
}

// Set the client is a secure client.
func (client *LDAP_Client) SetTLS(hasTls bool) {
	client.hasTLS = hasTls
}

// Set TLS certificate file path
func (client *LDAP_Client) SetCertFile(certFile string) {
	client.certFile = certFile
}

// Set TLS key file path
func (client *LDAP_Client) SetKeyFile(keyFile string) {
	client.keyFile = keyFile
}

// Set TLS authority trust certificate file path
func (client *LDAP_Client) SetCaFile(caFile string) {
	client.caFile = caFile
}

////////////////////////// LDAP ////////////////////////////////////////////////
// Stop the service.
func (client *LDAP_Client) StopService() {
	client.c.Stop(client.GetCtx(), &ldappb.StopRequest{})
}

func (client *LDAP_Client) CreateConnection(connectionId string, user string, password string, host string, port int32) error {
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

	_, err := client.c.CreateConnection(client.GetCtx(), rqst)

	return err
}

func (client *LDAP_Client) DeleteConnection(connectionId string) error {

	rqst := &ldappb.DeleteConnectionRqst{
		Id: connectionId,
	}

	_, err := client.c.DeleteConnection(client.GetCtx(), rqst)

	return err
}

func (client *LDAP_Client) Authenticate(connectionId string, userId string, password string) error {

	rqst := &ldappb.AuthenticateRqst{
		Id:    connectionId,
		Login: userId,
		Pwd:   password,
	}

	
	_, err := client.c.Authenticate(client.GetCtx(), rqst)

	return err
}

func (client *LDAP_Client) Search(connectionId string, BaseDN string, Filter string, Attributes []string) ([][]interface{}, error) {

	// I will execute a simple ldap search here...
	rqst := &ldappb.SearchRqst{
		Search: &ldappb.Search{
			Id:         connectionId,
			BaseDN:     BaseDN,
			Filter:     Filter,
			Attributes: Attributes,
		},
	}

	rsp, err := client.c.Search(client.GetCtx(), rqst)
	if err != nil {
		return nil, err
	}

	values := make([][]interface{}, 0)
	err = json.Unmarshal([]byte(rsp.Result), &values)

	return values, err

}
