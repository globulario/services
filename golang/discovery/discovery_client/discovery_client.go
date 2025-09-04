package discovery_client

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/discovery/discoverypb"
	globular "github.com/globulario/services/golang/globular_client"
	"github.com/globulario/services/golang/resource/resourcepb"
	"github.com/globulario/services/golang/security"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

////////////////////////////////////////////////////////////////////////////////
// Discovery Client
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
	if err := globular.InitClient(client, address, id); err != nil {
		return nil, err
	}
	if err := client.Reconnect(); err != nil {
		return nil, err
	}
	return client, nil
}

func (client *Dicovery_Client) Reconnect() error {
	var err error
	const tries = 10
	for i := 0; i < tries; i++ {
		client.cc, err = globular.GetClientConnection(client)
		if err == nil {
			client.c = discoverypb.NewPackageDiscoveryClient(client.cc)
			break
		}
		time.Sleep(500 * time.Millisecond)
	}
	return err
}

// The address where the client can connect.
func (client *Dicovery_Client) SetAddress(address string) { client.address = address }

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
	if token, err := security.GetLocalToken(client.GetMac()); err == nil {
		md := metadata.New(map[string]string{
			"token":   string(token),
			"domain":  client.domain,
			"mac":     client.GetMac(),
			"address": client.GetAddress(),
		})
		client.ctx = metadata.NewOutgoingContext(context.Background(), md)
	}
	return client.ctx
}

// Return the domain
func (client *Dicovery_Client) GetDomain() string { return client.domain }

// Return the address
func (client *Dicovery_Client) GetAddress() string { return client.address }

// Return the last know connection state
func (client *Dicovery_Client) GetState() string { return client.state }

// Return the id of the service instance
func (client *Dicovery_Client) GetId() string { return client.id }

// Return the name of the service
func (client *Dicovery_Client) GetName() string { return client.name }

func (client *Dicovery_Client) GetMac() string { return client.mac }

// must be close when no more needed.
func (client *Dicovery_Client) Close() { client.cc.Close() }

// Set grpc_service port.
func (client *Dicovery_Client) SetPort(port int) { client.port = port }

// Return the grpc port number
func (client *Dicovery_Client) GetPort() int { return client.port }

// Set the client instance id.
func (client *Dicovery_Client) SetId(id string) { client.id = id }

// Set the client name.
func (client *Dicovery_Client) SetName(name string) { client.name = name }

func (client *Dicovery_Client) SetMac(mac string) { client.mac = mac }

// Set the domain.
func (client *Dicovery_Client) SetDomain(domain string) { client.domain = domain }

func (client *Dicovery_Client) SetState(state string) { client.state = state }

////////////////// TLS ///////////////////

func (client *Dicovery_Client) HasTLS() bool                 { return client.hasTLS }
func (client *Dicovery_Client) GetCertFile() string          { return client.certFile }
func (client *Dicovery_Client) GetKeyFile() string           { return client.keyFile }
func (client *Dicovery_Client) GetCaFile() string            { return client.caFile }
func (client *Dicovery_Client) SetTLS(hasTls bool)           { client.hasTLS = hasTls }
func (client *Dicovery_Client) SetCertFile(certFile string)  { client.certFile = certFile }
func (client *Dicovery_Client) SetKeyFile(keyFile string)    { client.keyFile = keyFile }
func (client *Dicovery_Client) SetCaFile(caFile string)      { client.caFile = caFile }

////////////////// API //////////////////////

// PublishService publishes a service stored in etcd.
// NOTE: the `configPath` parameter now strictly means service **Id or Name**.
func (client *Dicovery_Client) PublishService(user, organization, token, domain, configPath, platform string) error {
	idOrName := strings.TrimSpace(configPath)
	if idOrName == "" {
		return errors.New("no service id or name provided")
	}

	// etcd lookup only (config.json is gone)
	s, err := config.GetServiceConfigurationById(idOrName)
	if err != nil || s == nil {
		return errors.New("service not found in etcd")
	}

	// Keywords → []string
	var keywords []string
	switch kv := s["Keywords"].(type) {
	case []interface{}:
		for _, it := range kv {
			if str, ok := it.(string); ok {
				keywords = append(keywords, str)
			}
		}
	case []string:
		keywords = kv
	}

	// Defaults
	discoveries := []string{domain}
	repositories := []string{domain}

	if !strings.Contains(user, "@") {
		user += "@" + client.GetDomain()
	}

	for _, disc := range discoveries {
		rqst := &discoverypb.PublishServiceRequest{
			User:         user,
			Organization: organization,
			Description:  Utility.ToString(s["Description"]),
			DiscoveryId:  disc,
			RepositoryId: repositories[0],
			Keywords:     keywords,
			Version:      Utility.ToString(s["Version"]),
			ServiceId:    Utility.ToString(s["Id"]),
			ServiceName:  Utility.ToString(s["Name"]),
			Platform:     platform,
		}

		// attach/override token in context if provided
		ctx := client.GetCtx()
		if token != "" {
			if md, ok := metadata.FromOutgoingContext(ctx); ok {
				if len(md.Get("token")) != 0 {
					md.Set("token", token)
					ctx = metadata.NewOutgoingContext(context.Background(), md)
				}
			}
		}

		if _, err := client.c.PublishService(ctx, rqst); err != nil {
			return err
		}
	}
	return nil
}

// PublishApplication stays the same (explicit fields).
func (client *Dicovery_Client) PublishApplication(
	token, user, organization, path, name, address, version, description, icon, alias,
	repositoryId, discoveryId string,
	actions, keywords []string,
	roles []*resourcepb.Role,
	groups []*resourcepb.Group,
) error {
	if len(token) == 0 {
		return errors.New("no token was provided")
	}

	if !strings.Contains(user, "@") {
		user += "@" + client.GetDomain()
	}

	rqst := &discoverypb.PublishApplicationRequest{
		User:         user,
		Organization: organization,
		Name:         name,
		Domain:       client.GetDomain(),
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
	if token != "" {
		if md, ok := metadata.FromOutgoingContext(ctx); ok {
			if len(md.Get("token")) != 0 {
				md.Set("token", token)
				ctx = metadata.NewOutgoingContext(context.Background(), md)
			}
		}
	}

	_, err := client.c.PublishApplication(ctx, rqst)
	return err
}
