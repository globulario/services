package resource_client

import (
	"context"
	"io/ioutil"
	"os"
	"strconv"
	"time"

	//"log"

	"github.com/davecourtois/Utility"
	globular "github.com/globulario/services/golang/globular_client"
	"github.com/globulario/services/golang/resource/resourcepb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

////////////////////////////////////////////////////////////////////////////////
// admin Client Service
////////////////////////////////////////////////////////////////////////////////

type Resource_Client struct {
	cc *grpc.ClientConn
	c  resourcepb.ResourceServiceClient

	// The id of the service
	id string

	// The name of the service
	name string

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
func NewResourceService_Client(address string, id string) (*Resource_Client, error) {

	client := new(Resource_Client)
	err := globular.InitClient(client, address, id)
	if err != nil {

		return nil, err
	}

	client.cc, err = globular.GetClientConnection(client)
	if err != nil {

		return nil, err
	}

	client.c = resourcepb.NewResourceServiceClient(client.cc)

	return client, nil
}

func (self *Resource_Client) Invoke(method string, rqst interface{}, ctx context.Context) (interface{}, error) {
	if ctx == nil {
		ctx = globular.GetClientContext(self)
	}
	return globular.InvokeClientRequest(self.c, ctx, method, rqst)
}

// Return the ipv4 address
// Return the address
func (self *Resource_Client) GetAddress() string {
	return self.domain + ":" + strconv.Itoa(self.port)
}

// Return the domain
func (self *Resource_Client) GetDomain() string {
	return self.domain
}

// Return the id of the service instance
func (self *Resource_Client) GetId() string {
	return self.id
}

// Return the name of the service
func (self *Resource_Client) GetName() string {
	return self.name
}

// must be close when no more needed.
func (self *Resource_Client) Close() {
	self.cc.Close()
}

// Set grpc_service port.
func (self *Resource_Client) SetPort(port int) {
	self.port = port
}

// Set the client name.
func (self *Resource_Client) SetId(id string) {
	self.id = id
}

// Set the client name.
func (self *Resource_Client) SetName(name string) {
	self.name = name
}

// Set the domain.
func (self *Resource_Client) SetDomain(domain string) {
	self.domain = domain
}

////////////////// TLS ///////////////////

// Get if the client is secure.
func (self *Resource_Client) HasTLS() bool {
	return self.hasTLS
}

// Get the TLS certificate file path
func (self *Resource_Client) GetCertFile() string {
	return self.certFile
}

// Get the TLS key file path
func (self *Resource_Client) GetKeyFile() string {
	return self.keyFile
}

// Get the TLS key file path
func (self *Resource_Client) GetCaFile() string {
	return self.caFile
}

// Set the client is a secure client.
func (self *Resource_Client) SetTLS(hasTls bool) {
	self.hasTLS = hasTls
}

// Set TLS certificate file path
func (self *Resource_Client) SetCertFile(certFile string) {
	self.certFile = certFile
}

// Set TLS key file path
func (self *Resource_Client) SetKeyFile(keyFile string) {
	self.keyFile = keyFile
}

// Set TLS authority trust certificate file path
func (self *Resource_Client) SetCaFile(caFile string) {
	self.caFile = caFile
}

////////////// API ////////////////
// Register a new Account.
func (self *Resource_Client) RegisterAccount(name string, email string, password string, confirmation_password string) error {
	rqst := &resourcepb.RegisterAccountRqst{
		Account: &resourcepb.Account{
			Name:     name,
			Email:    email,
			Password: "",
		},
		Password:        password,
		ConfirmPassword: confirmation_password,
	}

	_, err := self.c.RegisterAccount(globular.GetClientContext(self), rqst)
	if err != nil {
		return err
	}

	return nil
}

// Delete an account.
func (self *Resource_Client) DeleteAccount(id string) error {
	rqst := &resourcepb.DeleteAccountRqst{
		Id: id,
	}

	_, err := self.c.DeleteAccount(globular.GetClientContext(self), rqst)
	return err
}

// Authenticate a user.
func (self *Resource_Client) Authenticate(name string, password string) (string, error) {
	// In case of other domain than localhost I will rip off the token file
	// before each authentication.
	path := os.TempDir() + string(os.PathSeparator) + self.GetDomain() + "_token"
	if !Utility.IsLocal(self.GetDomain()) {
		// remove the file if it already exist.
		os.Remove(path)
	}

	rqst := &resourcepb.AuthenticateRqst{
		Name:     name,
		Password: password,
	}

	rsp, err := self.c.Authenticate(globular.GetClientContext(self), rqst)
	if err != nil {
		return "", err
	}

	// Here I will save the token into the temporary directory the token will be valid for a given time (default is 15 minutes)
	// it's the responsability of the client to keep it refresh... see Refresh token from the server...
	if !Utility.IsLocal(self.GetDomain()) {
		err = ioutil.WriteFile(path, []byte(rsp.Token), 0644)
		if err != nil {
			return "", err
		}
	}

	return rsp.Token, nil
}

/**
 *  Generate a new token from expired one.
 */
func (self *Resource_Client) RefreshToken(token string) (string, error) {
	rqst := new(resourcepb.RefreshTokenRqst)
	rqst.Token = token

	rsp, err := self.c.RefreshToken(globular.GetClientContext(self), rqst)
	if err != nil {
		return "", err
	}

	return rsp.Token, nil
}

/**
 * Create a new role with given action list.
 */
func (self *Resource_Client) CreateRole(name string, actions []string) error {
	rqst := new(resourcepb.CreateRoleRqst)
	role := new(resourcepb.Role)
	role.Name = name
	role.Actions = actions
	rqst.Role = role
	_, err := self.c.CreateRole(globular.GetClientContext(self), rqst)

	return err
}

func (self *Resource_Client) DeleteRole(name string) error {
	rqst := new(resourcepb.DeleteRoleRqst)
	rqst.RoleId = name

	_, err := self.c.DeleteRole(globular.GetClientContext(self), rqst)

	return err
}

/**
 * Add a action to a given role.
 */
func (self *Resource_Client) AddRoleAction(roleId string, action string) error {
	rqst := &resourcepb.AddRoleActionRqst{
		RoleId: roleId,
		Action: action,
	}
	_, err := self.c.AddRoleAction(globular.GetClientContext(self), rqst)

	return err
}

/**
 * Remove action from a given role.
 */
func (self *Resource_Client) RemoveRoleAction(roleId string, action string) error {
	rqst := &resourcepb.RemoveRoleActionRqst{
		RoleId: roleId,
		Action: action,
	}
	_, err := self.c.RemoveRoleAction(globular.GetClientContext(self), rqst)

	return err
}

/**
 * Add a action to a given application.
 */
func (self *Resource_Client) AddApplicationAction(applicationId string, action string) error {
	rqst := &resourcepb.AddApplicationActionRqst{
		ApplicationId: applicationId,
		Action:        action,
	}
	_, err := self.c.AddApplicationAction(globular.GetClientContext(self), rqst)

	return err
}

/**
 * Remove action from a given application.
 */
func (self *Resource_Client) RemoveApplicationAction(applicationId string, action string) error {
	rqst := &resourcepb.RemoveApplicationActionRqst{
		ApplicationId: applicationId,
		Action:        action,
	}
	_, err := self.c.RemoveApplicationAction(globular.GetClientContext(self), rqst)

	return err
}

/**
 * Set role to a account
 */
func (self *Resource_Client) AddAccountRole(accountId string, roleId string) error {
	rqst := &resourcepb.AddAccountRoleRqst{
		AccountId: accountId,
		RoleId:    roleId,
	}
	_, err := self.c.AddAccountRole(globular.GetClientContext(self), rqst)

	return err
}

/**
 * Remove role from an account
 */
func (self *Resource_Client) RemoveAccountRole(accountId string, roleId string) error {
	rqst := &resourcepb.RemoveAccountRoleRqst{
		AccountId: accountId,
		RoleId:    roleId,
	}
	_, err := self.c.RemoveAccountRole(globular.GetClientContext(self), rqst)

	return err
}

/**
 * Return the list of all available actions on the server.
 */
func (self *Resource_Client) GetAllActions() ([]string, error) {
	rqst := &resourcepb.GetAllActionsRqst{}
	rsp, err := self.c.GetAllActions(globular.GetClientContext(self), rqst)
	if err != nil {
		return nil, err
	}
	return rsp.Actions, err
}

/////////////////////////////// Ressouce permissions ///////////////////////////////

/**
 * Set file permission for a given user.
 */
func (self *Resource_Client) SetResourcePermissionByUser(userId string, path string, permission int32) error {
	rqst := &resourcepb.SetPermissionRqst{
		Permission: &resourcepb.ResourcePermission{
			Owner: &resourcepb.ResourcePermission_User{
				User: userId,
			},
			Path:   path,
			Number: permission,
		},
	}

	_, err := self.c.SetPermission(globular.GetClientContext(self), rqst)
	return err
}

/**
 * Set file permission for a given role.
 */
func (self *Resource_Client) SetResourcePermissionByRole(roleId string, path string, permission int32) error {
	rqst := &resourcepb.SetPermissionRqst{
		Permission: &resourcepb.ResourcePermission{
			Owner: &resourcepb.ResourcePermission_Role{
				Role: roleId,
			},
			Path:   path,
			Number: permission,
		},
	}

	_, err := self.c.SetPermission(globular.GetClientContext(self), rqst)
	return err
}

func (self *Resource_Client) GetResourcePermissions(path string) (string, error) {
	rqst := &resourcepb.GetPermissionsRqst{
		Path: path,
	}

	rsp, err := self.c.GetPermissions(globular.GetClientContext(self), rqst)
	if err != nil {
		return "", err
	}
	return rsp.GetPermissions(), nil
}

func (self *Resource_Client) DeleteResourcePermissions(path string, owner string) error {
	rqst := &resourcepb.DeletePermissionsRqst{
		Path:  path,
		Owner: owner,
	}

	_, err := self.c.DeletePermissions(globular.GetClientContext(self), rqst)

	return err
}

func (self *Resource_Client) GetAllFilesInfo() (string, error) {
	rqst := &resourcepb.GetAllFilesInfoRqst{}

	rsp, err := self.c.GetAllFilesInfo(globular.GetClientContext(self), rqst)
	if err != nil {
		return "", err
	}

	return rsp.GetResult(), nil
}

func (self *Resource_Client) ValidateUserResourceAccess(token string, path string, method string, permission int32) (bool, error) {
	rqst := &resourcepb.ValidateUserResourceAccessRqst{}
	rqst.Token = token
	rqst.Path = path
	rqst.Method = method
	rqst.Permission = permission

	rsp, err := self.c.ValidateUserResourceAccess(globular.GetClientContext(self), rqst)
	if err != nil {
		return false, err
	}

	return rsp.GetResult(), nil
}

func (self *Resource_Client) ValidateApplicationResourceAccess(application string, path string, method string, permission int32) (bool, error) {
	rqst := &resourcepb.ValidateApplicationResourceAccessRqst{}
	rqst.Name = application
	rqst.Path = path
	rqst.Method = method
	rqst.Permission = permission

	rsp, err := self.c.ValidateApplicationResourceAccess(globular.GetClientContext(self), rqst)
	if err != nil {
		return false, err
	}

	return rsp.GetResult(), nil
}

func (self *Resource_Client) ValidateUserAccess(token string, method string) (bool, error) {
	rqst := &resourcepb.ValidateUserAccessRqst{}
	rqst.Token = token
	rqst.Method = method

	rsp, err := self.c.ValidateUserAccess(globular.GetClientContext(self), rqst)
	if err != nil {
		return false, err
	}

	return rsp.GetResult(), nil
}

func (self *Resource_Client) ValidateApplicationAccess(application string, method string) (bool, error) {
	rqst := &resourcepb.ValidateApplicationAccessRqst{}
	rqst.Name = application
	rqst.Method = method
	rsp, err := self.c.ValidateApplicationAccess(globular.GetClientContext(self), rqst)
	if err != nil {
		return false, err
	}

	return rsp.GetResult(), nil
}

func (self *Resource_Client) DeleteRolePermissions(id string) error {
	rqst := &resourcepb.DeleteRolePermissionsRqst{
		Id: id,
	}
	_, err := self.c.DeleteRolePermissions(globular.GetClientContext(self), rqst)
	return err
}

func (self *Resource_Client) DeleteAccountPermissions(id string) error {
	rqst := &resourcepb.DeleteAccountPermissionsRqst{
		Id: id,
	}
	_, err := self.c.DeleteAccountPermissions(globular.GetClientContext(self), rqst)
	return err
}

func (self *Resource_Client) GetActionPermission(action string) ([]*resourcepb.ActionParameterResourcePermission, error) {
	rqst := &resourcepb.GetActionPermissionRqst{
		Action: action,
	}

	rsp, err := self.c.GetActionPermission(globular.GetClientContext(self), rqst)
	if err != nil {
		return nil, err
	}

	return rsp.ActionParameterResourcePermissions, nil
}

func (self *Resource_Client) SetResource(name string, path string, modified int64, size int64, token string) error {
	resource := &resourcepb.Resource{
		Name:     name,
		Path:     path,
		Modified: modified,
		Size:     size,
	}

	rqst := &resourcepb.SetResourceRqst{
		Resource: resource,
	}
	var err error
	if len(token) > 0 {
		md := metadata.New(map[string]string{"token": string(token), "domain": self.GetDomain(), "mac": Utility.MyMacAddr(), "ip": Utility.MyIP()})
		ctx := metadata.NewOutgoingContext(context.Background(), md)

		_, err = self.c.SetResource(ctx, rqst)
	} else {
		_, err = self.c.SetResource(globular.GetClientContext(self), rqst)
	}

	return err
}

func (self *Resource_Client) SetResourceOwner(owner string, path string, token string) error {
	rqst := &resourcepb.SetResourceOwnerRqst{
		Owner: owner,
		Path:  path,
	}
	var err error
	if len(token) > 0 {
		md := metadata.New(map[string]string{"token": string(token), "domain": self.GetDomain(), "mac": Utility.MyMacAddr(), "ip": Utility.MyIP()})
		ctx := metadata.NewOutgoingContext(context.Background(), md)

		_, err = self.c.SetResourceOwner(ctx, rqst)
	} else {
		_, err = self.c.SetResourceOwner(globular.GetClientContext(self), rqst)
	}

	return err
}

// Set action permission
func (self *Resource_Client) SetActionPermission(action string, actionParameterResourcePermissions []*resourcepb.ActionParameterResourcePermission, token string) error {
	var err error

	// Set action permission.
	rqst := &resourcepb.SetActionPermissionRqst{
		Action:                              action,
		ActionParameterResourcePermissions: actionParameterResourcePermissions,
	}

	if len(token) > 0 {
		md := metadata.New(map[string]string{"token": string(token), "domain": self.GetDomain(), "mac": Utility.MyMacAddr(), "ip": Utility.MyIP()})
		ctx := metadata.NewOutgoingContext(context.Background(), md)

		// Set action permission.
		_, err = self.c.SetActionPermission(ctx, rqst)
	} else {
		_, err = self.c.SetActionPermission(globular.GetClientContext(self), rqst)
	}

	return err
}

/////////////////////// Log ////////////////////////

// Append a new log information.
func (self *Resource_Client) Log(application string, user string, method string, err_ error) error {

	// Here I set a log information.
	rqst := new(resourcepb.LogRqst)
	info := new(resourcepb.LogInfo)
	info.Application = application
	info.UserName = user
	info.Method = method
	info.Date = time.Now().Unix()
	if err_ != nil {
		info.Message = err_.Error()
		info.Type = resourcepb.LogType_ERROR_MESSAGE
	} else {
		info.Type = resourcepb.LogType_INFO_MESSAGE
	}
	rqst.Info = info

	_, err := self.c.Log(globular.GetClientContext(self), rqst)

	return err
}

func (self *Resource_Client) CreateDirPermissions(token string, path string, name string) error {
	rqst := &resourcepb.CreateDirPermissionsRqst{
		Token: token,
		Path:  path,
		Name:  name,
	}
	_, err := self.c.CreateDirPermissions(globular.GetClientContext(self), rqst)
	return err
}

func (self *Resource_Client) RenameFilePermission(path string, oldName string, newName string) error {
	rqst := &resourcepb.RenameFilePermissionRqst{
		Path:    path,
		OldName: oldName,
		NewName: newName,
	}

	_, err := self.c.RenameFilePermission(globular.GetClientContext(self), rqst)
	return err
}

func (self *Resource_Client) DeleteDirPermissions(path string) error {
	rqst := &resourcepb.DeleteDirPermissionsRqst{
		Path: path,
	}
	_, err := self.c.DeleteDirPermissions(globular.GetClientContext(self), rqst)
	return err
}

func (self *Resource_Client) DeleteFilePermissions(path string) error {
	rqst := &resourcepb.DeleteFilePermissionsRqst{
		Path: path,
	}
	_, err := self.c.DeleteFilePermissions(globular.GetClientContext(self), rqst)
	return err
}

func (self *Resource_Client) ValidatePeerResourceAccess(domain string, path string, method string, permission int32) (bool, error) {
	rqst := &resourcepb.ValidatePeerResourceAccessRqst{}
	rqst.Domain = domain
	rqst.Path = path
	rqst.Method = method
	rqst.Permission = permission

	rsp, err := self.c.ValidatePeerResourceAccess(globular.GetClientContext(self), rqst)
	if err != nil {
		return false, err
	}

	return rsp.GetResult(), nil
}

func (self *Resource_Client) ValidatePeerAccess(domain string, method string) (bool, error) {
	rqst := &resourcepb.ValidatePeerAccessRqst{}
	rqst.Domain = domain
	rqst.Method = method
	rsp, err := self.c.ValidatePeerAccess(globular.GetClientContext(self), rqst)
	if err != nil {
		return false, err
	}

	return rsp.GetResult(), nil
}

// Register a peer with a given name and mac address.
func (self *Resource_Client) RegisterPeer(domain string) error {
	rqst := &resourcepb.RegisterPeerRqst{
		Peer: &resourcepb.Peer{
			Domain: domain,
		},
	}

	_, err := self.c.RegisterPeer(globular.GetClientContext(self), rqst)
	return err

}
