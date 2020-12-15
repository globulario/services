package resource_client

import (
	//"bytes"
	"context"
	"io"
	"io/ioutil"
	"os"
	"strconv"

	//	"time"

	//"log"

	"github.com/davecourtois/Utility"
	globular "github.com/globulario/services/golang/globular_client"
	"github.com/globulario/services/golang/resource/resourcepb"
	"google.golang.org/grpc"
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

////////////////////////////////////////////////////////////////////////////////
// Organisation
////////////////////////////////////////////////////////////////////////////////

// Create a new Organization
func (self *Resource_Client) CreateOrganization(id string, name string) error {

	// Create a new Organization.
	rqst := &resourcepb.CreateOrganizationRqst{
		Organization: &resourcepb.Organization{
			Id:   id,
			Name: name,
		},
	}

	_, err := self.c.CreateOrganization(globular.GetClientContext(self), rqst)
	return err

}

// Create a new Organization
func (self *Resource_Client) DeleteOrganization(id string) error {

	// Create a new Organization.
	rqst := &resourcepb.DeleteOrganizationRqst{
		Organization: id,
	}

	_, err := self.c.DeleteOrganization(globular.GetClientContext(self), rqst)
	return err

}

// Add to Organisation...
func (self *Resource_Client) AddOrganizationAccount(organisationId string, accountId string) error {

	// Create a new Organization.
	rqst := &resourcepb.AddOrganizationAccountRqst{
		OrganizationId: organisationId,
		AccountId:      accountId,
	}

	_, err := self.c.AddOrganizationAccount(globular.GetClientContext(self), rqst)
	return err
}

func (self *Resource_Client) AddOrganizationRole(organisationId string, roleId string) error {

	// Create a new Organization.
	rqst := &resourcepb.AddOrganizationRoleRqst{
		OrganizationId: organisationId,
		RoleId:         roleId,
	}

	_, err := self.c.AddOrganizationRole(globular.GetClientContext(self), rqst)
	return err
}

func (self *Resource_Client) AddOrganizationGroup(organisationId string, groupId string) error {

	// Create a new Organization.
	rqst := &resourcepb.AddOrganizationGroupRqst{
		OrganizationId: organisationId,
		GroupId:        groupId,
	}

	_, err := self.c.AddOrganizationGroup(globular.GetClientContext(self), rqst)
	return err
}

func (self *Resource_Client) AddOrganizationApplication(organisationId string, applicationId string) error {

	// Create a new Organization.
	rqst := &resourcepb.AddOrganizationApplicationRqst{
		OrganizationId: organisationId,
		ApplicationId:  applicationId,
	}

	_, err := self.c.AddOrganizationApplication(globular.GetClientContext(self), rqst)
	return err
}

// Remove from organization

func (self *Resource_Client) RemoveOrganizationAccount(organisationId string, accountId string) error {

	// Create a new Organization.
	rqst := &resourcepb.RemoveOrganizationAccountRqst{
		OrganizationId: organisationId,
		AccountId:      accountId,
	}

	_, err := self.c.RemoveOrganizationAccount(globular.GetClientContext(self), rqst)
	return err
}

func (self *Resource_Client) RemoveOrganizationRole(organisationId string, roleId string) error {

	// Create a new Organization.
	rqst := &resourcepb.RemoveOrganizationRoleRqst{
		OrganizationId: organisationId,
		RoleId:         roleId,
	}

	_, err := self.c.RemoveOrganizationRole(globular.GetClientContext(self), rqst)
	return err
}

func (self *Resource_Client) RemoveOrganizationGroup(organisationId string, groupId string) error {

	// Create a new Organization.
	rqst := &resourcepb.RemoveOrganizationGroupRqst{
		OrganizationId: organisationId,
		GroupId:        groupId,
	}

	_, err := self.c.RemoveOrganizationGroup(globular.GetClientContext(self), rqst)
	return err
}

func (self *Resource_Client) RemoveOrganizationApplication(organisationId string, applicationId string) error {

	// Create a new Organization.
	rqst := &resourcepb.RemoveOrganizationApplicationRqst{
		OrganizationId: organisationId,
		ApplicationId:  applicationId,
	}

	_, err := self.c.RemoveOrganizationApplication(globular.GetClientContext(self), rqst)
	return err
}

////////////////////////////////////////////////////////////////////////////////
// Account
////////////////////////////////////////////////////////////////////////////////

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
	return err
}

// Delete an account.
func (self *Resource_Client) DeleteAccount(id string) error {
	rqst := &resourcepb.DeleteAccountRqst{
		Id: id,
	}

	_, err := self.c.DeleteAccount(globular.GetClientContext(self), rqst)
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

////////////////////////////////////////////////////////////////////////////////
// Group
////////////////////////////////////////////////////////////////////////////////

/**
 * Create a new group.
 */
func (self *Resource_Client) CreateGroup(id string, name string) error {
	rqst := new(resourcepb.CreateGroupRqst)
	g := new(resourcepb.Group)
	g.Name = name
	g.Id = id
	rqst.Group = g
	ctx := globular.GetClientContext(self)
	_, err := self.c.CreateGroup(ctx, rqst)

	return err
}

func (self *Resource_Client) AddGroupMemberAccount(groupId string, accountId string) error {
	rqst := new(resourcepb.AddGroupMemberAccountRqst)
	rqst.AccountId = accountId
	rqst.GroupId = groupId

	ctx := globular.GetClientContext(self)
	_, err := self.c.AddGroupMemberAccount(ctx, rqst)
	return err
}

func (self *Resource_Client) DeleteGroup(groupId string) error {
	rqst := new(resourcepb.DeleteGroupRqst)
	rqst.Group = groupId

	ctx := globular.GetClientContext(self)
	_, err := self.c.DeleteGroup(ctx, rqst)
	return err
}

func (self *Resource_Client) RemoveGroupMemberAccount(groupId string, accountId string) error {
	rqst := new(resourcepb.RemoveGroupMemberAccountRqst)
	rqst.AccountId = accountId
	rqst.GroupId = groupId

	ctx := globular.GetClientContext(self)
	_, err := self.c.RemoveGroupMemberAccount(ctx, rqst)
	return err
}

func (self *Resource_Client) GetGroups(query string) ([]*resourcepb.Group, error) {

	// Open the stream...
	groups := make([]*resourcepb.Group, 0)
	// I will execute a simple ldap search here...
	rqst := new(resourcepb.GetGroupsRqst)
	rqst.Query = query

	stream, err := self.c.GetGroups(globular.GetClientContext(self), rqst)
	if err != nil {
		return nil, err
	}

	// Here I will create the final array
	for {
		msg, err := stream.Recv()
		if err == io.EOF {
			// end of stream...
			break
		}

		if err != nil {
			return nil, err
		}

		groups = append(groups, msg.Groups...)

		if err != nil {
			return nil, err
		}
	}

	return groups, err
}

////////////////////////////////////////////////////////////////////////////////
// Role
////////////////////////////////////////////////////////////////////////////////

/**
 * Create a new role with given action list.
 */
func (self *Resource_Client) CreateRole(id string, name string, actions []string) error {
	rqst := new(resourcepb.CreateRoleRqst)
	role := new(resourcepb.Role)
	role.Id = id
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

////////////////////////////////////////////////////////////////////////////////
// Peer
////////////////////////////////////////////////////////////////////////////////

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

// Delete a peer
func (self *Resource_Client) DeletePeer(domain string) error {
	rqst := &resourcepb.DeletePeerRqst{
		Peer: &resourcepb.Peer{
			Domain: domain,
		},
	}
	_, err := self.c.DeletePeer(globular.GetClientContext(self), rqst)
	return err
}

////////////////////////////////////////////////////////////////////////////////
// Application
////////////////////////////////////////////////////////////////////////////////
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
 * Validata action...
 */
func (self *Resource_Client) ValidateAction(action string, subject string, subjectType resourcepb.SubjectType, resources []*resourcepb.ResourceInfos) (bool, error) {
	rqst := &resourcepb.ValidateActionRqst{
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
func (self *Resource_Client) GetActionResourceInfos(action string) ([]*resourcepb.ResourceInfos, error) {
	rqst := &resourcepb.GetActionResourceInfosRqst{
		Action: action,
	}

	rsp, err := self.c.GetActionResourceInfos(globular.GetClientContext(self), rqst)
	if err != nil {
		return nil, err
	}

	return rsp.Infos, err
}
