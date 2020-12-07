package rbac_client

import (
	"strconv"

	"context"

	// "github.com/davecourtois/Utility"
	globular "github.com/globulario/services/golang/globular_client"
	"github.com/globulario/services/golang/resource/resourcepb"
	"google.golang.org/grpc"
)

////////////////////////////////////////////////////////////////////////////////
// echo Client Service
////////////////////////////////////////////////////////////////////////////////

type Rbac_Client struct {
	cc *grpc.ClientConn
	c  resourcepb.RbacServiceClient

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
func NewRbacService_Client(address string, id string) (*Rbac_Client, error) {
	client := new(Rbac_Client)
	err := globular.InitClient(client, address, id)
	if err != nil {
		return nil, err
	}
	client.cc, err = globular.GetClientConnection(client)
	if err != nil {
		return nil, err
	}
	client.c = resourcepb.NewRbacServiceClient(client.cc)

	return client, nil
}

func (self *Rbac_Client) Invoke(method string, rqst interface{}, ctx context.Context) (interface{}, error) {
	if ctx == nil {
		ctx = globular.GetClientContext(self)
	}
	return globular.InvokeClientRequest(self.c, ctx, method, rqst)
}

// Return the domain
func (self *Rbac_Client) GetDomain() string {
	return self.domain
}

// Return the address
func (self *Rbac_Client) GetAddress() string {
	return self.domain + ":" + strconv.Itoa(self.port)
}

// Return the id of the service instance
func (self *Rbac_Client) GetId() string {
	return self.id
}

// Return the name of the service
func (self *Rbac_Client) GetName() string {
	return self.name
}

// must be close when no more needed.
func (self *Rbac_Client) Close() {
	self.cc.Close()
}

// Set grpc_service port.
func (self *Rbac_Client) SetPort(port int) {
	self.port = port
}

// Set the client instance id.
func (self *Rbac_Client) SetId(id string) {
	self.id = id
}

// Set the client name.
func (self *Rbac_Client) SetName(name string) {
	self.name = name
}

// Set the domain.
func (self *Rbac_Client) SetDomain(domain string) {
	self.domain = domain
}

////////////////// TLS ///////////////////

// Get if the client is secure.
func (self *Rbac_Client) HasTLS() bool {
	return self.hasTLS
}

// Get the TLS certificate file path
func (self *Rbac_Client) GetCertFile() string {
	return self.certFile
}

// Get the TLS key file path
func (self *Rbac_Client) GetKeyFile() string {
	return self.keyFile
}

// Get the TLS key file path
func (self *Rbac_Client) GetCaFile() string {
	return self.caFile
}

// Set the client is a secure client.
func (self *Rbac_Client) SetTLS(hasTls bool) {
	self.hasTLS = hasTls
}

// Set TLS certificate file path
func (self *Rbac_Client) SetCertFile(certFile string) {
	self.certFile = certFile
}

// Set TLS key file path
func (self *Rbac_Client) SetKeyFile(keyFile string) {
	self.keyFile = keyFile
}

// Set TLS authority trust certificate file path
func (self *Rbac_Client) SetCaFile(caFile string) {
	self.caFile = caFile
}

////////////////////////////////////  Api  /////////////////////////////////////
/** Set the action resources permissions **/
func (self *Rbac_Client) SetActionResourcesPermission(action string, resources []*resourcepb.ActionResourceParameterPermission) error {
	rqst := &resourcepb.SetActionResourcesPermissionRqst{
		Action:    action,
		Resources: resources,
	}

	_, err := self.c.SetActionResourcesPermission(globular.GetClientContext(self), rqst)
	return err
}

/** Get the action ressouces permission **/
func (self *Rbac_Client) GetActionResourcesPermission() {
	// Implement it
}

/** Set resource permissions this method will replace existing permission at once **/
func (self *Rbac_Client) SetResourcePermissions() {
	// Implement it
}

/** Delete a resource permissions (when a resource is deleted) **/
func (self *Rbac_Client) DeleteResourcePermissions() {
	// Implement it
}

/** Delete a specific resource permission **/
func (self *Rbac_Client) DeleteResourcePermission() {
	// Implement it
}

/** Set specific resource permission  ex. read permission... **/
func (self *Rbac_Client) SetResourcePermission() {
	// Implement it
}

/** Get a specific resource access **/
func (self *Rbac_Client) GetResourcePermission() {
	// Implement it
}

/** Get resource permissions **/
func (self *Rbac_Client) GetResourcePermissions() {
	// Implement it
}

/** Add resource owner do nothing if it already exist */
func (self *Rbac_Client) AddResourceOwner() {
	// Implement it
}

/** Remove resource owner */
func (self *Rbac_Client) RemoveResourceOwner() {
	// Implement it
}

/** That function must be call when a subject is removed to clean up permissions. */
func (self *Rbac_Client) DeleteAllAccess() {
	// Implement it
}

/** Validate if a user can get access to a given ressource for a given operation (read, write...) **/
func (self *Rbac_Client) ValidateAccess() {
	// Implement it
}

/** Return the list of access for a given subject */
func (self *Rbac_Client) GetAccesses() {
	// Implement it
}
