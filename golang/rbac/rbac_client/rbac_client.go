package rbac_client

import (
	"context"
	"strconv"

	// "github.com/davecourtois/Utility"
	globular "github.com/globulario/services/golang/globular_client"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"google.golang.org/grpc"
)

////////////////////////////////////////////////////////////////////////////////
// echo Client Service
////////////////////////////////////////////////////////////////////////////////

type Rbac_Client struct {
	cc *grpc.ClientConn
	c  rbacpb.RbacServiceClient

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
	client.c = rbacpb.NewRbacServiceClient(client.cc)

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

/** Set resource permissions this method will replace existing permission at once **/
func (self *Rbac_Client) SetResourcePermissions(path string, permissions *rbacpb.Permissions) error {
	rqst := &rbacpb.SetResourcePermissionsRqst{
		Path:        path,
		Permissions: permissions,
	}

	_, err := self.c.SetResourcePermissions(globular.GetClientContext(self), rqst)
	return err

}

/** Get resource permissions **/
func (self *Rbac_Client) GetResourcePermission(path string, permissionName string, permissionType rbacpb.PermissionType) (*rbacpb.Permission, error) {
	rqst := &rbacpb.GetResourcePermissionRqst{
		Name: permissionName,
		Type: permissionType,
		Path: path,
	}

	rsp, err := self.c.GetResourcePermission(globular.GetClientContext(self), rqst)
	if err != nil {
		return nil, err
	}
	return rsp.Permission, err
}

/** Get resource permissions for a given path**/
func (self *Rbac_Client) GetResourcePermissions(path string) (*rbacpb.Permissions, error) {
	rqst := &rbacpb.GetResourcePermissionsRqst{
		Path: path,
	}

	rsp, err := self.c.GetResourcePermissions(globular.GetClientContext(self), rqst)
	if err != nil {
		return nil, err
	}
	return rsp.Permissions, err
}

/** Delete a resource permissions (when a resource is deleted) **/
func (self *Rbac_Client) DeleteResourcePermissions(path string) error {
	rqst := &rbacpb.DeleteResourcePermissionsRqst{
		Path: path,
	}

	_, err := self.c.DeleteResourcePermissions(globular.GetClientContext(self), rqst)
	return err
}

/** Delete a specific resource permission **/
func (self *Rbac_Client) DeleteResourcePermission(path string, permissionName string, permissionType rbacpb.PermissionType) error {
	rqst := &rbacpb.DeleteResourcePermissionRqst{
		Name: permissionName,
		Type: permissionType,
		Path: path,
	}

	_, err := self.c.DeleteResourcePermission(globular.GetClientContext(self), rqst)
	return err
}

/** Set specific resource permission  ex. read permission... **/
func (self *Rbac_Client) SetResourcePermission(path string, permission *rbacpb.Permission, permissionType rbacpb.PermissionType) error {
	rqst := &rbacpb.SetResourcePermissionRqst{
		Permission: permission,
		Type:       permissionType,
		Path:       path,
	}

	_, err := self.c.SetResourcePermission(globular.GetClientContext(self), rqst)
	return err
}

/** Add resource owner do nothing if it already exist */
func (self *Rbac_Client) AddResourceOwner(path string, owner string, subjectType rbacpb.SubjectType) error {
	rqst := &rbacpb.AddResourceOwnerRqst{
		Type:    subjectType,
		Subject: owner,
		Path:    path,
	}

	_, err := self.c.AddResourceOwner(globular.GetClientContext(self), rqst)
	return err

}

/** Remove resource owner */
func (self *Rbac_Client) RemoveResourceOwner(path string, owner string, subjectType rbacpb.SubjectType) error {
	rqst := &rbacpb.RemoveResourceOwnerRqst{
		Subject: owner,
		Path:    path,
		Type:    subjectType,
	}

	_, err := self.c.RemoveResourceOwner(globular.GetClientContext(self), rqst)
	return err
}

/** That function must be call when a subject is removed to clean up permissions. */
func (self *Rbac_Client) DeleteAllAccess(subject string, subjectType rbacpb.SubjectType) error {
	rqst := &rbacpb.DeleteAllAccessRqst{
		Subject: subject,
		Type:    subjectType,
	}

	_, err := self.c.DeleteAllAccess(globular.GetClientContext(self), rqst)
	return err

}

/** Validate if a user can get access to a given Resource for a given operation (read, write...) **/
func (self *Rbac_Client) ValidateAccess(subject string, subjectType rbacpb.SubjectType, permission string, path string) (bool, error) {

	rqst := &rbacpb.ValidateAccessRqst{
		Subject:    subject,
		Type:       subjectType,
		Path:       path,
		Permission: permission,
	}

	rsp, err := self.c.ValidateAccess(globular.GetClientContext(self), rqst)
	if err != nil {
		return false, err
	}

	return rsp.GetResult(), nil
}

/**
 * Validata action...
 */
func (self *Rbac_Client) ValidateAction(action string, subject string, subjectType rbacpb.SubjectType, resources []*rbacpb.ResourceInfos) (bool, error) {
	rqst := &rbacpb.ValidateActionRqst{
		Action:  action,
		Subject: subject,
		Type:    subjectType,
		Infos:   resources,
	}

	rsp, err := self.c.ValidateAction(globular.GetClientContext(self), rqst)
	if err != nil {
		return false, err
	}

	return rsp.Result, nil

}

/**
 * Get action ressource paramater infos...
 */
func (self *Rbac_Client) GetActionResourceInfos(action string) ([]*rbacpb.ResourceInfos, error) {
	rqst := &rbacpb.GetActionResourceInfosRqst{
		Action: action,
	}

	rsp, err := self.c.GetActionResourceInfos(globular.GetClientContext(self), rqst)
	if err != nil {
		return nil, err
	}

	return rsp.Infos, err
}
