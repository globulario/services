package rbac_client

import (
	"context"
	"io"
	"time"

	globular "github.com/globulario/services/golang/globular_client"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/security"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/structpb"
)

////////////////////////////////////////////////////////////////////////////////
// echo Client Service
////////////////////////////////////////////////////////////////////////////////

type Rbac_Client struct {
	cc *grpc.ClientConn
	c  rbacpb.RbacServiceClient

	// The id of the service
	id string

	// The mac address of the server
	mac string

	// The name of the service
	name string

	// The client domain
	domain string

	//  keep the last connection state of the client.
	state string

	// The address where connection with client can be done. ex: globule0.globular.cloud:10101
	address string

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
func NewRbacService_Client(address string, id string) (*Rbac_Client, error) {
	client := new(Rbac_Client)
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

func (client *Rbac_Client) Reconnect() error {

	var err error
	nb_try_connect := 10

	for i := 0; i < nb_try_connect; i++ {
		client.cc, err = globular.GetClientConnection(client)
		if err == nil {
			client.c = rbacpb.NewRbacServiceClient(client.cc)
			break
		}

		// wait 500 millisecond before next try
		time.Sleep(500 * time.Millisecond)
	}

	return err
}

// The address where the client can connect.
func (client *Rbac_Client) SetAddress(address string) {
	client.address = address
}

func (client *Rbac_Client) Invoke(method string, rqst interface{}, ctx context.Context) (interface{}, error) {
	if ctx == nil {
		ctx = client.GetCtx()
	}
	return globular.InvokeClientRequest(client.c, ctx, method, rqst)
}

func (client *Rbac_Client) GetCtx() context.Context {
	if client.ctx == nil {
		client.ctx = globular.GetClientContext(client)
	}
	token, err := security.GetLocalToken(client.GetMac())
	if err == nil {
		md := metadata.New(map[string]string{"token": string(token), "domain": client.domain, "mac": client.GetMac(), "address": client.GetAddress()})
		client.ctx = metadata.NewOutgoingContext(context.Background(), md)
	}
	return client.ctx
}

// Return the last know connection state
func (client *Rbac_Client) GetState() string {
	return client.state
}

// Return the domain
func (client *Rbac_Client) GetDomain() string {
	return client.domain
}

// Return the address
func (client *Rbac_Client) GetAddress() string {
	return client.address
}

// Return the id of the service instance
func (client *Rbac_Client) GetId() string {
	return client.id
}

// Return the name of the service
func (client *Rbac_Client) GetName() string {
	return client.name
}

func (client *Rbac_Client) GetMac() string {
	return client.mac
}

// must be close when no more needed.
func (client *Rbac_Client) Close() {
	client.cc.Close()
}

// Set grpc_service port.
func (client *Rbac_Client) SetPort(port int) {
	client.port = port
}

// Return the grpc port number
func (client *Rbac_Client) GetPort() int {
	return client.port
}

// Set the client instance id.
func (client *Rbac_Client) SetId(id string) {
	client.id = id
}

// Set the client name.
func (client *Rbac_Client) SetName(name string) {
	client.name = name
}

func (client *Rbac_Client) SetMac(mac string) {
	client.mac = mac
}

// Set the domain.
func (client *Rbac_Client) SetDomain(domain string) {
	client.domain = domain
}

func (client *Rbac_Client) SetState(state string) {
	client.state = state
}

////////////////// TLS ///////////////////

// Get if the client is secure.
func (client *Rbac_Client) HasTLS() bool {
	return client.hasTLS
}

// Get the TLS certificate file path
func (client *Rbac_Client) GetCertFile() string {
	return client.certFile
}

// Get the TLS key file path
func (client *Rbac_Client) GetKeyFile() string {
	return client.keyFile
}

// Get the TLS key file path
func (client *Rbac_Client) GetCaFile() string {
	return client.caFile
}

// Set the client is a secure client.
func (client *Rbac_Client) SetTLS(hasTls bool) {
	client.hasTLS = hasTls
}

// Set TLS certificate file path
func (client *Rbac_Client) SetCertFile(certFile string) {
	client.certFile = certFile
}

// Set TLS key file path
func (client *Rbac_Client) SetKeyFile(keyFile string) {
	client.keyFile = keyFile
}

// Set TLS authority trust certificate file path
func (client *Rbac_Client) SetCaFile(caFile string) {
	client.caFile = caFile
}

////////////////////////////////////  Api  /////////////////////////////////////

/** Set resource permissions this method will replace existing permission at once **/
func (client *Rbac_Client) SetResourcePermissions(token, path, resource_type string, permissions *rbacpb.Permissions) error {
	// set value if not already set...
	permissions.Path = path
	permissions.ResourceType = resource_type

	rqst := &rbacpb.SetResourcePermissionsRqst{
		Path:         path,
		ResourceType: resource_type,
		Permissions:  permissions,
	}

	// set the token in the context...
	md := metadata.New(map[string]string{"token": string(token), "domain": client.domain})
	ctx := metadata.NewOutgoingContext(client.GetCtx(), md)

	_, err := client.c.SetResourcePermissions(ctx, rqst)

	return err

}

/** Get resource permissions **/
func (client *Rbac_Client) GetResourcePermission(path string, permissionName string, permissionType rbacpb.PermissionType) (*rbacpb.Permission, error) {
	rqst := &rbacpb.GetResourcePermissionRqst{
		Name: permissionName,
		Type: permissionType,
		Path: path,
	}

	rsp, err := client.c.GetResourcePermission(client.GetCtx(), rqst)
	if err != nil {
		return nil, err
	}
	return rsp.Permission, err
}

/** Get resource permissions for a given path**/
func (client *Rbac_Client) GetResourcePermissions(path string) (*rbacpb.Permissions, error) {
	rqst := &rbacpb.GetResourcePermissionsRqst{
		Path: path,
	}

	rsp, err := client.c.GetResourcePermissions(client.GetCtx(), rqst)
	if err != nil {
		return nil, err
	}
	return rsp.Permissions, err
}

/** Delete a resource permissions (when a resource is deleted) **/
func (client *Rbac_Client) DeleteResourcePermissions(path string) error {
	rqst := &rbacpb.DeleteResourcePermissionsRqst{
		Path: path,
	}

	_, err := client.c.DeleteResourcePermissions(client.GetCtx(), rqst)
	return err
}

/** Get the list of resource permission by type **/
func (client *Rbac_Client) GetResourcePermissionsByResourceType(resource_type string) ([]*rbacpb.Permissions, error) {
	rqst := &rbacpb.GetResourcePermissionsByResourceTypeRqst{
		ResourceType: resource_type,
	}

	stream, err := client.c.GetResourcePermissionsByResourceType(client.GetCtx(), rqst)
	if err != nil {
		return nil, err
	}

	permissions := make([]*rbacpb.Permissions, 0)

	for {
		msg, err := stream.Recv()
		if err != nil {
			if err == io.EOF {
				break
			} else {
				return nil, err
			}

		} else if msg != nil {
			if len(msg.Permissions) == 0 {
				break
			} else {
				permissions = append(permissions, msg.Permissions...)
			}
		}
	}

	return permissions, nil
}

/** Delete a specific resource permission **/
func (client *Rbac_Client) DeleteResourcePermission(path string, permissionName string, permissionType rbacpb.PermissionType) error {
	rqst := &rbacpb.DeleteResourcePermissionRqst{
		Name: permissionName,
		Type: permissionType,
		Path: path,
	}

	_, err := client.c.DeleteResourcePermission(client.GetCtx(), rqst)
	return err
}

/** Set specific resource permission  ex. read permission... **/
func (client *Rbac_Client) SetResourcePermission(path string, permission *rbacpb.Permission, permissionType rbacpb.PermissionType) error {
	rqst := &rbacpb.SetResourcePermissionRqst{
		Permission: permission,
		Type:       permissionType,
		Path:       path,
	}

	_, err := client.c.SetResourcePermission(client.GetCtx(), rqst)
	return err
}

/** Add resource owner do nothing if it already exist */
func (client *Rbac_Client) AddResourceOwner(path, resourceType, owner string, subjectType rbacpb.SubjectType) error {
	rqst := &rbacpb.AddResourceOwnerRqst{
		Type:         subjectType,
		Subject:      owner,
		Path:         path,
		ResourceType: resourceType,
	}

	_, err := client.c.AddResourceOwner(client.GetCtx(), rqst)
	return err

}

/** Remove resource owner */
func (client *Rbac_Client) RemoveResourceOwner(path string, owner string, subjectType rbacpb.SubjectType) error {
	rqst := &rbacpb.RemoveResourceOwnerRqst{
		Subject: owner,
		Path:    path,
		Type:    subjectType,
	}

	_, err := client.c.RemoveResourceOwner(client.GetCtx(), rqst)
	return err
}

/** That function must be call when a subject is removed to clean up permissions. */
func (client *Rbac_Client) DeleteAllAccess(subject string, subjectType rbacpb.SubjectType) error {
	rqst := &rbacpb.DeleteAllAccessRqst{
		Subject: subject,
		Type:    subjectType,
	}

	_, err := client.c.DeleteAllAccess(client.GetCtx(), rqst)
	return err

}

/** Validate if a user can get access to a given Resource for a given operation (read, write...) **/
func (client *Rbac_Client) ValidateAccess(subject string, subjectType rbacpb.SubjectType, permission string, path string) (bool, bool, error) {

	rqst := &rbacpb.ValidateAccessRqst{
		Subject:    subject,
		Type:       subjectType,
		Path:       path,
		Permission: permission,
	}

	rsp, err := client.c.ValidateAccess(client.GetCtx(), rqst)
	if err != nil {
		return false, false, err
	}

	return rsp.HasAccess, rsp.AccessDenied, nil
}

/** Validate if a user can get access to a given Resource for a given operation (read, write...) **/
func (client *Rbac_Client) ValidateSubjectSpace(subject string, subjectType rbacpb.SubjectType, required_space uint64) (bool, error) {

	rqst := &rbacpb.ValidateSubjectSpaceRqst{
		Subject:       subject,
		Type:          subjectType,
		RequiredSpace: required_space,
	}

	rsp, err := client.c.ValidateSubjectSpace(client.GetCtx(), rqst)
	if err != nil {
		return false, err
	}

	return rsp.HasSpace, nil
}

/**
 * Validata action...
 */
func (client *Rbac_Client) ValidateAction(action string, subject string, subjectType rbacpb.SubjectType, resources []*rbacpb.ResourceInfos) (bool, bool, error) {
	rqst := &rbacpb.ValidateActionRqst{
		Action:  action,
		Subject: subject,
		Type:    subjectType,
		Infos:   resources,
	}

	rsp, err := client.c.ValidateAction(client.GetCtx(), rqst)
	if err != nil {
		return false, false, err
	}

	return rsp.HasAccess, rsp.AccessDenied, nil

}

/**
 * Get action resource paramater infos...
 */
func (client *Rbac_Client) GetActionResourceInfos(action string) ([]*rbacpb.ResourceInfos, error) {
	rqst := &rbacpb.GetActionResourceInfosRqst{
		Action: action,
	}

	rsp, err := client.c.GetActionResourceInfos(client.GetCtx(), rqst)
	if err != nil {
		return nil, err
	}

	return rsp.Infos, err
}

func (client *Rbac_Client) SetActionResourcesPermissions(permissions map[string]interface{}) error {
	permissions_, err := structpb.NewStruct(permissions)
	if err != nil {
		return err
	}
	rqst := &rbacpb.SetActionResourcesPermissionsRqst{
		Permissions: permissions_,
	}

	_, err = client.c.SetActionResourcesPermissions(client.GetCtx(), rqst)
	if err != nil {
		return err
	}
	return nil
}

/** Set subject allocated space **/
func (client *Rbac_Client) SetAccountAllocatedSpace(accountId string, space uint64) error {

	rqst := &rbacpb.SetSubjectAllocatedSpaceRqst{
		Subject: accountId,
		Type:     rbacpb.SubjectType_ACCOUNT,
		AllocatedSpace:    space,
	}

	_, err := client.c.SetSubjectAllocatedSpace(client.GetCtx(), rqst)
	if err != nil {
		return err
	}

	return nil
}
