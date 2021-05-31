package service_manager_client

import (
	"strconv"
	"log"
	"context"
	//"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/services_manager/services_managerpb"
	globular "github.com/globulario/services/golang/globular_client"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

////////////////////////////////////////////////////////////////////////////////
// echo Client Service
////////////////////////////////////////////////////////////////////////////////

type Services_Manager_Client struct {
	cc *grpc.ClientConn
	c  services_managerpb.ServicesManagerServiceClient

	// The id of the service
	id string

	// The name of the service
	name string

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
func NewServicesManagerService_Client(address string, id string) (*Services_Manager_Client, error) {
	client := new(Services_Manager_Client)
	err := globular.InitClient(client, address, id)
	if err != nil {
		return nil, err
	}
	client.cc, err = globular.GetClientConnection(client)
	if err != nil {
		return nil, err
	}
	client.c = services_managerpb.NewServicesManagerServiceClient(client.cc)

	return client, nil
}

func (client *Services_Manager_Client) Invoke(method string, rqst interface{}, ctx context.Context) (interface{}, error) {
	if ctx == nil {
		ctx = globular.GetClientContext(client)
	}
	return globular.InvokeClientRequest(client.c, ctx, method, rqst)
}

// Return the domain
func (client *Services_Manager_Client) GetDomain() string {
	return client.domain
}

// Return the address
func (client *Services_Manager_Client) GetAddress() string {
	return client.domain + ":" + strconv.Itoa(client.port)
}

// Return the id of the service instance
func (client *Services_Manager_Client) GetId() string {
	return client.id
}

// Return the name of the service
func (client *Services_Manager_Client) GetName() string {
	return client.name
}

// must be close when no more needed.
func (client *Services_Manager_Client) Close() {
	client.cc.Close()
}

// Set grpc_service port.
func (client *Services_Manager_Client) SetPort(port int) {
	client.port = port
}

// Set the client instance id.
func (client *Services_Manager_Client) SetId(id string) {
	client.id = id
}

// Set the client name.
func (client *Services_Manager_Client) SetName(name string) {
	client.name = name
}

// Set the domain.
func (client *Services_Manager_Client) SetDomain(domain string) {
	client.domain = domain
}

////////////////// TLS ///////////////////

// Get if the client is secure.
func (client *Services_Manager_Client) HasTLS() bool {
	return client.hasTLS
}

// Get the TLS certificate file path
func (client *Services_Manager_Client) GetCertFile() string {
	return client.certFile
}

// Get the TLS key file path
func (client *Services_Manager_Client) GetKeyFile() string {
	return client.keyFile
}

// Get the TLS key file path
func (client *Services_Manager_Client) GetCaFile() string {
	return client.caFile
}

// Set the client is a secure client.
func (client *Services_Manager_Client) SetTLS(hasTls bool) {
	client.hasTLS = hasTls
}

// Set TLS certificate file path
func (client *Services_Manager_Client) SetCertFile(certFile string) {
	client.certFile = certFile
}

// Set TLS key file path
func (client *Services_Manager_Client) SetKeyFile(keyFile string) {
	client.keyFile = keyFile
}

// Set TLS authority trust certificate file path
func (client *Services_Manager_Client) SetCaFile(caFile string) {
	client.caFile = caFile
}

////////////////// Api //////////////////////

/**
 * Intall a new service or update an existing one.
 */
 func (client *Services_Manager_Client) InstallService(token string, domain string, user string, discoveryId string, publisherId string, serviceId string) error {

	log.Println("Install service", serviceId, "publisherId", publisherId, "discovery", discoveryId, "on", domain)
	log.Println("token: ", token)

	rqst := new(services_managerpb.InstallServiceRequest)
	rqst.DicorveryId = discoveryId
	rqst.PublisherId = publisherId
	rqst.ServiceId = serviceId
	ctx := globular.GetClientContext(client)
	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)
		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	_, err := client.c.InstallService(ctx, rqst)
	
	// Fail to install service.
	if err != nil {
		log.Println("fail to install service with error ", err.Error())
	}

	return err
}

/**
 * Intall a new service or update an existing one.
 */
func (client *Services_Manager_Client) UninstallService(token string, domain string, user string, publisherId string, serviceId string, version string) error {

	rqst := new(services_managerpb.UninstallServiceRequest)
	rqst.PublisherId = publisherId
	rqst.ServiceId = serviceId
	rqst.Version = version
	ctx := globular.GetClientContext(client)
	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	_, err := client.c.UninstallService(ctx, rqst)

	return err
}

func (client *Services_Manager_Client) StartServiceInstance(id string) (int, int, error) {
	rqst := new(services_managerpb.StartServiceInstanceRequest)
	rqst.ServiceId = id
	rsp, err := client.c.StartServiceInstance(globular.GetClientContext(client), rqst)
	if err != nil {
		return -1, -1, err
	}

	return int(rsp.ServicePid), int(rsp.ProxyPid), nil
}

func (client *Services_Manager_Client) StopServiceInstance(id string) error {
	rqst := new(services_managerpb.StopServiceInstanceRequest)
	rqst.ServiceId = id
	_, err := client.c.StopServiceInstance(globular.GetClientContext(client), rqst)
	if err != nil {
		return err
	}

	return nil
}

func (client *Services_Manager_Client) RestartAllServices() error {
	rqst := new(services_managerpb.RestartAllServicesRequest)

	_, err := client.c.RestartAllServices(globular.GetClientContext(client), rqst)
	if err != nil {
		return err
	}

	return nil
}