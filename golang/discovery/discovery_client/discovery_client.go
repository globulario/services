package discovery_client

import (
	"context"
	"strconv"

	"encoding/json"
	"errors"
	"io/ioutil"
	"log"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/discovery/discoverypb"
	globular "github.com/globulario/services/golang/globular_client"
	"github.com/globulario/services/golang/resource/resourcepb"
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
func NewDiscoveryService_Client(address string, id string) (*Dicovery_Client, error) {
	client := new(Dicovery_Client)
	err := globular.InitClient(client, address, id)
	if err != nil {
		return nil, err
	}

	client.cc, err = globular.GetClientConnection(client)
	if err != nil {
		return nil, err
	}

	client.c = discoverypb.NewPackageDiscoveryClient(client.cc)

	return client, nil
}

func (client *Dicovery_Client) Invoke(method string, rqst interface{}, ctx context.Context) (interface{}, error) {
	if ctx == nil {
		ctx = globular.GetClientContext(client)
	}
	return globular.InvokeClientRequest(client.c, ctx, method, rqst)
}

// Return the domain
func (client *Dicovery_Client) GetDomain() string {
	return client.domain
}

// Return the address
func (client *Dicovery_Client) GetAddress() string {
	return client.domain + ":" + strconv.Itoa(client.port)
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
	if s["Repositories"] == nil {
		s["Repositories"] = []interface{}{"localhost"}
	}
	repositories := s["Repositories"].([]interface{})
	if len(repositories) == 0 {
		repositories = []interface{}{"localhost"}

	}

	if s["Discoveries"] == nil {
		return errors.New("no discovery was set on that server")
	}

	discoveries := s["Discoveries"].([]interface{})

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
		ctx := globular.GetClientContext(Services_Manager_Client)
		if len(token) > 0 {
			md, _ := metadata.FromOutgoingContext(ctx)

			if len(md.Get("token")) != 0 {
				md.Set("token", token)
			}
			ctx = metadata.NewOutgoingContext(context.Background(), md)
		}

		_, err = Services_Manager_Client.c.PublishService(ctx, rqst)
		if err != nil {
			log.Println("fail to publish service at ", discoveries[i], err)
		}
	}

	return nil
}

/**
 * Publish an application on the server.
 */
func (client *Dicovery_Client) PublishApplication(user, organization, path, name, domain, version, description, icon, alias, repositoryId, discoveryId string, actions, keywords []string, roles []*resourcepb.Role, groups []*resourcepb.Group) error {
	// TODO upload the package and publish the application after see old admin client code bundle from the path...

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
		Groups: 	  groups,
	}

	_, err := client.c.PublishApplication(globular.GetClientContext(client), rqst)

	return err
}
