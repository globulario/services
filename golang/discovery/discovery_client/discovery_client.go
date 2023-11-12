package discovery_client

import (
	"context"
	"strings"
	"time"

	"encoding/json"
	"errors"
	"io/ioutil"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/discovery/discoverypb"
	globular "github.com/globulario/services/golang/globular_client"
	"github.com/globulario/services/golang/resource/resourcepb"
	"github.com/globulario/services/golang/security"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

////////////////////////////////////////////////////////////////////////////////
// echo Client Service
////////////////////////////////////////////////////////////////////////////////

type Dicovery_Client struct {
	cc *grpc.ClientConn
	c  discoverypb.PackageDiscoveryClient

	// The id of the service
	id string

	// The name of the service
	name string

	// The mac address of the server
	mac string

	// The client domain
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
func NewDiscoveryService_Client(address string, id string) (*Dicovery_Client, error) {
	client := new(Dicovery_Client)
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

func (client *Dicovery_Client) Reconnect() error {
	var err error
	nb_try_connect := 10

	for i := 0; i < nb_try_connect; i++ {
		client.cc, err = globular.GetClientConnection(client)
		if err == nil {
			client.c = discoverypb.NewPackageDiscoveryClient(client.cc)
			break
		}

		// wait 500 millisecond before next try
		time.Sleep(500 * time.Millisecond)
	}

	return err
}

// The address where the client can connect.
func (client *Dicovery_Client) SetAddress(address string) {
	client.address = address
}

func (client *Dicovery_Client) Invoke(method string, rqst interface{}, ctx context.Context) (interface{}, error) {
	if ctx == nil {
		ctx = client.GetCtx()
	}
	return globular.InvokeClientRequest(client.c, ctx, method, rqst)
}

func (client *Dicovery_Client) GetCtx() context.Context {
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
func (client *Dicovery_Client) GetDomain() string {
	return client.domain
}

// Return the address
func (client *Dicovery_Client) GetAddress() string {
	return client.address
}

// Return the last know connection state
func (client *Dicovery_Client) GetState() string {
	return client.state
}

// Return the id of the service instance
func (client *Dicovery_Client) GetId() string {
	return client.id
}

// Return the name of the service
func (client *Dicovery_Client) GetName() string {
	return client.name
}

func (client *Dicovery_Client) GetMac() string {
	return client.mac
}

// must be close when no more needed.
func (client *Dicovery_Client) Close() {
	client.cc.Close()
}

// Set grpc_service port.
func (client *Dicovery_Client) SetPort(port int) {
	client.port = port
}

// Return the grpc port number
func (client *Dicovery_Client) GetPort() int {
	return client.port
}

// Set the client instance id.
func (client *Dicovery_Client) SetId(id string) {
	client.id = id
}

// Set the client name.
func (client *Dicovery_Client) SetName(name string) {
	client.name = name
}

func (client *Dicovery_Client) SetMac(mac string) {
	client.mac = mac
}

// Set the domain.
func (client *Dicovery_Client) SetDomain(domain string) {
	client.domain = domain
}

func (client *Dicovery_Client) SetState(state string) {
	client.state = state
}

////////////////// TLS ///////////////////

// Get if the client is secure.
func (client *Dicovery_Client) HasTLS() bool {
	return client.hasTLS
}

// Get the TLS certificate file path
func (client *Dicovery_Client) GetCertFile() string {
	return client.certFile
}

// Get the TLS key file path
func (client *Dicovery_Client) GetKeyFile() string {
	return client.keyFile
}

// Get the TLS key file path
func (client *Dicovery_Client) GetCaFile() string {
	return client.caFile
}

// Set the client is a secure client.
func (client *Dicovery_Client) SetTLS(hasTls bool) {
	client.hasTLS = hasTls
}

// Set TLS certificate file path
func (client *Dicovery_Client) SetCertFile(certFile string) {
	client.certFile = certFile
}

// Set TLS key file path
func (client *Dicovery_Client) SetKeyFile(keyFile string) {
	client.keyFile = keyFile
}

// Set TLS authority trust certificate file path
func (client *Dicovery_Client) SetCaFile(caFile string) {
	client.caFile = caFile
}

////////////////// Api //////////////////////

/**
 * Publish a service from a runing globular server.
 */
func (Services_Manager_Client *Dicovery_Client) PublishService(user, organization, token, domain, configPath, platform string) error {

	// Here I will try to read the service configuation from the path.
	configs, _ := Utility.FindFileByName(configPath, "config.json")
	if len(configs) == 0 {
		return errors.New("no configuration file was found")
	}

	s := make(map[string]interface{})
	data, err := ioutil.ReadFile(configs[0])
	if err != nil {
		return err
	}

	err = json.Unmarshal(data, &s)
	if err != nil {
		return err
	}

	keywords := make([]string, 0)
	if s["Keywords"] != nil {
		for i := 0; i < len(s["Keywords"].([]interface{})); i++ {
			keywords = append(keywords, s["Keywords"].([]interface{})[i].(string))
		}
	}
	/*
		if s["Repositories"] == nil {
			s["Repositories"] = []interface{}{domain}
		}

		repositories := s["Repositories"].([]interface{})
		if len(repositories) == 0 {
			repositories = []interface{}{"localhost"}
		}

		if s["Discoveries"] == nil {
			return errors.New("no discovery was set on that server")
		}
	*/

	discoveries := []interface{}{domain}
	repositories := []interface{}{domain}

	if len(token) > 0 {
		claims, _ := security.ValidateToken(token)
		if !strings.Contains(user, "@") {
			if len(claims.UserDomain) == 0 {
				return errors.New("no user domain was found in the token")
			}
			
			user += "@" + claims.UserDomain
		}
	}

	for i := 0; i < len(discoveries); i++ {
		rqst := new(discoverypb.PublishServiceRequest)
		rqst.User = user
		rqst.Organization = organization
		rqst.Description = s["Description"].(string)
		rqst.DicorveryId = discoveries[i].(string)
		rqst.RepositoryId = repositories[0].(string)
		rqst.Keywords = keywords
		rqst.Version = s["Version"].(string)
		rqst.ServiceId = s["Id"].(string)
		rqst.ServiceName = s["Name"].(string)
		rqst.Platform = platform

		// Set the token into the context and send the request.
		ctx := Services_Manager_Client.GetCtx()
		if len(token) > 0 {
			md, _ := metadata.FromOutgoingContext(ctx)
			if len(md.Get("token")) != 0 {
				md.Set("token", token)
			}

			ctx = metadata.NewOutgoingContext(context.Background(), md)
		}

		Services_Manager_Client.c.PublishService(ctx, rqst)
	}

	return nil
}

/**
 * Publish an application on the server.
 */
func (client *Dicovery_Client) PublishApplication(token, user, organization, path, name, domain, version, description, icon, alias, repositoryId, discoveryId string, actions, keywords []string, roles []*resourcepb.Role, groups []*resourcepb.Group) error {
	// TODO upload the package and publish the application after see old admin client code bundle from the path...
	if len(token) == 0 {
		return errors.New("no token was provided")
	}

	claims, _ := security.ValidateToken(token)

	if !strings.Contains(user, "@") {
		if len(claims.UserDomain) == 0 {
			return errors.New("no user domain was found in the token")
		}
		
		if len(claims.UserDomain) == 0 {
			return errors.New("no user domain was found in the token")
		}

		user += "@" + claims.UserDomain
	}

	rqst := &discoverypb.PublishApplicationRequest{
		User:         user,
		Organization: organization,
		Name:         name,
		Domain:       domain,
		Version:      version,
		Description:  description,
		Icon:         icon,
		Alias:        alias,
		Repository:   repositoryId,
		Discovery:    discoveryId,
		Actions:      actions,
		Keywords:     keywords,
		Roles:        roles,
		Path:         path,
		Groups:       groups,
	}

	ctx := client.GetCtx()
	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	_, err := client.c.PublishApplication(ctx, rqst)

	if err != nil {
		return err
	}

	return err
}
