package resource_client

import (
	"context"
	"fmt"
	"io"
	"time"

	Utility "github.com/davecourtois/!utility"
	globular "github.com/globulario/services/golang/globular_client"
	"github.com/globulario/services/golang/resource/resourcepb"
	"github.com/globulario/services/golang/security"
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

	// The mac address of the server
	mac string

	// The name of the service
	name string

	// The client domain
	domain string

	// The address where connection with client can be done. ex: globule0.globular.cloud:10101
	address string

	//  keep the last connection state of the client.
	state string

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
func NewResourceService_Client(address string, id string) (*Resource_Client, error) {

	client := new(Resource_Client)
	err := globular.InitClient(client, address, id)
	if err != nil {
		fmt.Println("fail to create resource client with error: ", err)
		return nil, err
	}

	err = client.Reconnect()
	if err != nil {
		fmt.Println("fail to connect to the remote server with error: ", err)
		return nil, err
	}

	return client, nil
}

func (client *Resource_Client) Reconnect() error {

	var err error
	nb_try_connect := 10

	for i := 0; i < nb_try_connect; i++ {
		client.cc, err = globular.GetClientConnection(client)
		if err == nil {
			client.c = resourcepb.NewResourceServiceClient(client.cc)
			break
		}

		// wait 500 millisecond before next try
		time.Sleep(500 * time.Millisecond)
	}

	return err
}

// The address where the client can connect.
func (client *Resource_Client) SetAddress(address string) {
	client.address = address
}

func (client *Resource_Client) Invoke(method string, rqst interface{}, ctx context.Context) (interface{}, error) {
	if ctx == nil {
		ctx = client.GetCtx()
	}
	return globular.InvokeClientRequest(client.c, ctx, method, rqst)
}

func (client *Resource_Client) GetCtx() context.Context {
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

// Return the ipv4 address
// Return the address
func (client *Resource_Client) GetAddress() string {
	return client.address
}

// Return the last know connection state
func (client *Resource_Client) GetState() string {
	return client.state
}

// Return the domain
func (client *Resource_Client) GetDomain() string {
	return client.domain
}

// Return the id of the service instance
func (client *Resource_Client) GetId() string {
	return client.id
}

// Return the name of the service
func (client *Resource_Client) GetName() string {
	return client.name
}

func (client *Resource_Client) GetMac() string {
	return client.mac
}

// must be close when no more needed.
func (client *Resource_Client) Close() {
	client.cc.Close()
}

// Set grpc_service port.
func (client *Resource_Client) SetPort(port int) {
	client.port = port
}

// Return the grpc port number
func (client *Resource_Client) GetPort() int {
	return client.port
}

// Set the client name.
func (client *Resource_Client) SetId(id string) {
	client.id = id
}

// Set the client name.
func (client *Resource_Client) SetName(name string) {
	client.name = name
}

func (client *Resource_Client) SetMac(mac string) {
	client.mac = mac
}

// Set the domain.
func (client *Resource_Client) SetDomain(domain string) {
	client.domain = domain
}

func (client *Resource_Client) SetState(state string) {
	client.state = state
}

////////////////// TLS ///////////////////

// Get if the client is secure.
func (client *Resource_Client) HasTLS() bool {
	return client.hasTLS
}

// Get the TLS certificate file path
func (client *Resource_Client) GetCertFile() string {
	return client.certFile
}

// Get the TLS key file path
func (client *Resource_Client) GetKeyFile() string {
	return client.keyFile
}

// Get the TLS key file path
func (client *Resource_Client) GetCaFile() string {
	return client.caFile
}

// Set the client is a secure client.
func (client *Resource_Client) SetTLS(hasTls bool) {
	client.hasTLS = hasTls
}

// Set TLS certificate file path
func (client *Resource_Client) SetCertFile(certFile string) {
	client.certFile = certFile
}

// Set TLS key file path
func (client *Resource_Client) SetKeyFile(keyFile string) {
	client.keyFile = keyFile
}

// Set TLS authority trust certificate file path
func (client *Resource_Client) SetCaFile(caFile string) {
	client.caFile = caFile
}

////////////// API ////////////////

// //////////////////////////////////////////////////////////////////////////////
// Object Reference
// //////////////////////////////////////////////////////////////////////////////
func (client *Resource_Client) CreateReference(id, sourceCollection, field, targetId, targetCollection string) error {
	rqst := &resourcepb.CreateReferenceRqst{
		SourceId:         id,
		SourceCollection: sourceCollection,
		FieldName:        field,
		TargetId:         targetId,
		TargetCollection: targetCollection,
	}

	_, err := client.c.CreateReference(client.GetCtx(), rqst)
	return err
}

func (client *Resource_Client) DeleteReference(refId, targetId, targetField, targetCollection string) error {
	rqst := &resourcepb.DeleteReferenceRqst{
		RefId:            refId,
		TargetId:         targetId,
		TargetField:      targetField,
		TargetCollection: targetCollection,
	}

	_, err := client.c.DeleteReference(client.GetCtx(), rqst)
	return err
}

// //////////////////////////////////////////////////////////////////////////////
// Package Descriptor
// //////////////////////////////////////////////////////////////////////////////
func (client *Resource_Client) SetPackageDescriptor(descriptor *resourcepb.PackageDescriptor) error {

	// Create a new Organization.
	rqst := &resourcepb.SetPackageDescriptorRequest{
		PackageDescriptor: descriptor,
	}

	_, err := client.c.SetPackageDescriptor(client.GetCtx(), rqst)
	return err

}

////////////////////////////////////////////////////////////////////////////////
// organization
////////////////////////////////////////////////////////////////////////////////

// Create a new Organization
func (client *Resource_Client) CreateOrganization(token, id, name, email, description, icon string) error {

	// Create a new Organization.
	rqst := &resourcepb.CreateOrganizationRqst{
		Organization: &resourcepb.Organization{
			Id:          id,
			Name:        name,
			Description: description,
			Icon:        icon,
			Email:       email,
			Domain:      client.domain,
		},
	}

	// set the token in the context...
	md := metadata.New(map[string]string{"token": string(token), "domain": client.domain})
	ctx := metadata.NewOutgoingContext(client.GetCtx(), md)

	_, err := client.c.CreateOrganization(ctx, rqst)
	return err

}

// Create a new Organization
func (client *Resource_Client) DeleteOrganization(token, id string) error {

	// Create a new Organization.
	rqst := &resourcepb.DeleteOrganizationRqst{
		Organization: id,
	}
	// set the token in the context...
	md := metadata.New(map[string]string{"token": string(token), "domain": client.domain})
	ctx := metadata.NewOutgoingContext(client.GetCtx(), md)
	_, err := client.c.DeleteOrganization(ctx, rqst)
	return err

}

// Add to organization...
func (client *Resource_Client) AddOrganizationAccount(token, organizationId, accountId string) error {

	// Create a new Organization.
	rqst := &resourcepb.AddOrganizationAccountRqst{
		OrganizationId: organizationId,
		AccountId:      accountId,
	}

	// set the token in the context...
	md := metadata.New(map[string]string{"token": string(token), "domain": client.domain})
	ctx := metadata.NewOutgoingContext(client.GetCtx(), md)

	_, err := client.c.AddOrganizationAccount(ctx, rqst)
	return err
}

func (client *Resource_Client) AddOrganizationRole(token, organizationId, roleId string) error {

	// Create a new Organization.
	rqst := &resourcepb.AddOrganizationRoleRqst{
		OrganizationId: organizationId,
		RoleId:         roleId,
	}

	// set the token in the context...
	md := metadata.New(map[string]string{"token": string(token), "domain": client.domain})
	ctx := metadata.NewOutgoingContext(client.GetCtx(), md)

	_, err := client.c.AddOrganizationRole(ctx, rqst)
	return err
}

func (client *Resource_Client) AddOrganizationGroup(token, organizationId, groupId string) error {

	// Create a new Organization.
	rqst := &resourcepb.AddOrganizationGroupRqst{
		OrganizationId: organizationId,
		GroupId:        groupId,
	}

	// set the token in the context...
	md := metadata.New(map[string]string{"token": string(token), "domain": client.domain})
	ctx := metadata.NewOutgoingContext(client.GetCtx(), md)

	_, err := client.c.AddOrganizationGroup(ctx, rqst)
	return err
}

func (client *Resource_Client) AddOrganizationApplication(token, organizationId, applicationId string) error {

	// Create a new Organization.
	rqst := &resourcepb.AddOrganizationApplicationRqst{
		OrganizationId: organizationId,
		ApplicationId:  applicationId,
	}

	// set the token in the context...
	md := metadata.New(map[string]string{"token": string(token), "domain": client.domain})
	ctx := metadata.NewOutgoingContext(client.GetCtx(), md)

	_, err := client.c.AddOrganizationApplication(ctx, rqst)
	return err
}

// Remove from organization

func (client *Resource_Client) RemoveOrganizationAccount(token, organizationId, accountId string) error {

	// Create a new Organization.
	rqst := &resourcepb.RemoveOrganizationAccountRqst{
		OrganizationId: organizationId,
		AccountId:      accountId,
	}

	// set the token in the context...
	md := metadata.New(map[string]string{"token": string(token), "domain": client.domain})
	ctx := metadata.NewOutgoingContext(client.GetCtx(), md)

	_, err := client.c.RemoveOrganizationAccount(ctx, rqst)
	return err
}

func (client *Resource_Client) RemoveOrganizationRole(token, organizationId, roleId string) error {

	// Create a new Organization.
	rqst := &resourcepb.RemoveOrganizationRoleRqst{
		OrganizationId: organizationId,
		RoleId:         roleId,
	}

	// set the token in the context...
	md := metadata.New(map[string]string{"token": string(token), "domain": client.domain})
	ctx := metadata.NewOutgoingContext(client.GetCtx(), md)

	_, err := client.c.RemoveOrganizationRole(ctx, rqst)
	return err
}

func (client *Resource_Client) RemoveOrganizationGroup(token, organizationId, groupId string) error {

	// Create a new Organization.
	rqst := &resourcepb.RemoveOrganizationGroupRqst{
		OrganizationId: organizationId,
		GroupId:        groupId,
	}

	// set the token in the context...
	md := metadata.New(map[string]string{"token": string(token), "domain": client.domain})
	ctx := metadata.NewOutgoingContext(client.GetCtx(), md)

	_, err := client.c.RemoveOrganizationGroup(ctx, rqst)
	return err
}

func (client *Resource_Client) RemoveOrganizationApplication(token, organizationId, applicationId string) error {

	// Create a new Organization.
	rqst := &resourcepb.RemoveOrganizationApplicationRqst{
		OrganizationId: organizationId,
		ApplicationId:  applicationId,
	}

	// set the token in the context...
	md := metadata.New(map[string]string{"token": string(token), "domain": client.domain})
	ctx := metadata.NewOutgoingContext(client.GetCtx(), md)

	_, err := client.c.RemoveOrganizationApplication(ctx, rqst)
	return err
}

func (client *Resource_Client) IsOrganizationMemeber(user, organization string) (bool, error) {
	rqst := &resourcepb.IsOrgnanizationMemberRqst{
		AccountId:      user,
		OrganizationId: organization,
	}

	rsp, err := client.c.IsOrgnanizationMember(client.GetCtx(), rqst)

	if err != nil {
		return false, err
	}

	return rsp.Result, nil
}

func (client *Resource_Client) GetOrganizations(query string) ([]*resourcepb.Organization, error) {

	// Open the stream...
	organizations := make([]*resourcepb.Organization, 0)

	// I will execute a simple ldap search here...
	rqst := new(resourcepb.GetOrganizationsRqst)
	rqst.Query = query

	stream, err := client.c.GetOrganizations(client.GetCtx(), rqst)
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

		organizations = append(organizations, msg.Organizations...)

		if err != nil {
			return nil, err
		}
	}

	return organizations, err
}

func (client *Resource_Client) UpdateOrganization(token string, o *resourcepb.Organization) error {
	rqst := new(resourcepb.UpdateOrganizationRqst)
	rqst.OrganizationId = o.Id
	rqst.Values = `{"$set":{"name":"` + o.Name + `","description":"` + o.Description + `", "domain":"` + o.Domain + `"}}`

	// set the token in the context...
	md := metadata.New(map[string]string{"token": string(token), "domain": client.domain})
	ctx := metadata.NewOutgoingContext(client.GetCtx(), md)

	_, err := client.c.UpdateOrganization(ctx, rqst)

	return err
}

////////////////////////////////////////////////////////////////////////////////
// Account
////////////////////////////////////////////////////////////////////////////////

// Register a new Account.
func (client *Resource_Client) RegisterAccount(domain, id, name, email, password, confirmation_password string) error {
	rqst := &resourcepb.RegisterAccountRqst{
		Account: &resourcepb.Account{
			Id:       id,
			Name:     name,
			Email:    email,
			Password: password,
			Domain:   domain,
		},
		ConfirmPassword: confirmation_password,
	}

	_, err := client.c.RegisterAccount(client.GetCtx(), rqst)
	return err
}

// Get account with a given id/name
func (client *Resource_Client) GetAccount(id string) (*resourcepb.Account, error) {
	rqst := &resourcepb.GetAccountRqst{
		AccountId: id,
	}
	rsp, err := client.c.GetAccount(client.GetCtx(), rqst)

	if err != nil {
		return nil, err
	}

	return rsp.Account, nil
}

// Update account values
func (client *Resource_Client) SetAccount(token string, account *resourcepb.Account) error {
	rqst := &resourcepb.SetAccountRqst{
		Account: account,
	}

	// Save account values.
	md := metadata.New(map[string]string{"token": string(token), "domain": client.domain})
	ctx := metadata.NewOutgoingContext(context.Background(), md)

	_, err := client.c.SetAccount(ctx, rqst)

	if err != nil {
		return err
	}
	return nil
}

func (client *Resource_Client) GetAccounts(query string) ([]*resourcepb.Account, error) {
	rqst := &resourcepb.GetAccountsRqst{Query: query}
	stream, err := client.c.GetAccounts(context.Background(), rqst)
	if err != nil {
		return nil, err
	}

	accounts := make([]*resourcepb.Account, 0)

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

		accounts = append(accounts, msg.Accounts...)

		if err != nil {
			return nil, err
		}
	}

	return accounts, nil
}

// Set the new password.
func (client *Resource_Client) SetAccountPassword(accountId, token, oldPassword, newPassword string) error {
	rqst := &resourcepb.SetAccountPasswordRqst{
		AccountId:   accountId,
		OldPassword: oldPassword,
		NewPassword: newPassword,
	}

	// append the token.
	md := metadata.New(map[string]string{"token": string(token), "domain": client.domain})
	ctx := metadata.NewOutgoingContext(context.Background(), md)

	_, err := client.c.SetAccountPassword(ctx, rqst)

	if err != nil {
		return err
	}
	return nil
}

// Delete an account.
func (client *Resource_Client) DeleteAccount(id, token string) error {
	rqst := &resourcepb.DeleteAccountRqst{
		Id: id,
	}

	// Save account values.
	md := metadata.New(map[string]string{"token": string(token), "domain": client.domain})
	ctx := metadata.NewOutgoingContext(context.Background(), md)

	_, err := client.c.DeleteAccount(ctx, rqst)
	return err
}

/**
 * Set role to a account
 */
func (client *Resource_Client) AddAccountRole(accountId string, roleId string) error {
	rqst := &resourcepb.AddAccountRoleRqst{
		AccountId: accountId,
		RoleId:    roleId,
	}

	_, err := client.c.AddAccountRole(client.GetCtx(), rqst)

	return err
}

/**
 * Remove role from an account
 */
func (client *Resource_Client) RemoveAccountRole(accountId string, roleId string) error {
	rqst := &resourcepb.RemoveAccountRoleRqst{
		AccountId: accountId,
		RoleId:    roleId,
	}
	_, err := client.c.RemoveAccountRole(client.GetCtx(), rqst)

	return err
}

////////////////////////////////////////////////////////////////////////////////
// Sessions
////////////////////////////////////////////////////////////////////////////////

/**
 * Return a given session
 */
func (client *Resource_Client) GetSession(accountId string) (*resourcepb.Session, error) {
	rqst := &resourcepb.GetSessionRequest{
		AccountId: accountId,
	}
	rsp, err := client.c.GetSession(client.GetCtx(), rqst)
	if err != nil {
		return nil, err
	}

	return rsp.Session, nil
}

/**
 * Return the list of all active sessions on the server.
 */
func (client *Resource_Client) GetSessions(query string) ([]*resourcepb.Session, error) {
	rqst := &resourcepb.GetSessionsRequest{}
	rqst.Query = query
	rsp, err := client.c.GetSessions(client.GetCtx(), rqst)
	if err != nil {
		return nil, err
	}

	return rsp.Sessions, nil
}

/**
 * Remove a session
 */
func (client *Resource_Client) RemoveSession(accountId string) error {
	rqst := &resourcepb.RemoveSessionRequest{
		AccountId: accountId,
	}
	_, err := client.c.RemoveSession(client.GetCtx(), rqst)

	return err
}

/**
 * Update/Create a session.
 */
func (client *Resource_Client) UpdateSession(session *resourcepb.Session) error {
	rqst := &resourcepb.UpdateSessionRequest{
		Session: session,
	}

	_, err := client.c.UpdateSession(client.GetCtx(), rqst)

	return err
}

////////////////////////////////////////////////////////////////////////////////
// Group
////////////////////////////////////////////////////////////////////////////////

/**
 * Create a new group.
 */
func (client *Resource_Client) CreateGroup(token, id, name, description string) error {
	rqst := new(resourcepb.CreateGroupRqst)
	g := new(resourcepb.Group)
	g.Name = name
	g.Id = id
	g.Description = description
	rqst.Group = g

	// set the token in the context...
	md := metadata.New(map[string]string{"token": string(token)})
	ctx := metadata.NewOutgoingContext(client.GetCtx(), md)

	_, err := client.c.CreateGroup(ctx, rqst)

	return err
}

func (client *Resource_Client) AddGroupMemberAccount(token, groupId, accountId string) error {
	rqst := new(resourcepb.AddGroupMemberAccountRqst)
	rqst.AccountId = accountId
	rqst.GroupId = groupId

	// set the token in the context...
	md := metadata.New(map[string]string{"token": string(token), "domain": client.domain})
	ctx := metadata.NewOutgoingContext(client.GetCtx(), md)

	_, err := client.c.AddGroupMemberAccount(ctx, rqst)
	return err
}

func (client *Resource_Client) DeleteGroup(token, groupId string) error {
	rqst := new(resourcepb.DeleteGroupRqst)
	rqst.Group = groupId

	// set the token in the context...
	md := metadata.New(map[string]string{"token": string(token), "domain": client.domain})
	ctx := metadata.NewOutgoingContext(client.GetCtx(), md)

	_, err := client.c.DeleteGroup(ctx, rqst)
	return err
}

func (client *Resource_Client) RemoveGroupMemberAccount(token, groupId, accountId string) error {
	rqst := new(resourcepb.RemoveGroupMemberAccountRqst)
	rqst.AccountId = accountId
	rqst.GroupId = groupId

	// set the token in the context...
	md := metadata.New(map[string]string{"token": string(token), "domain": client.domain})
	ctx := metadata.NewOutgoingContext(client.GetCtx(), md)

	_, err := client.c.RemoveGroupMemberAccount(ctx, rqst)
	return err
}

func (client *Resource_Client) GetGroups(query string) ([]*resourcepb.Group, error) {

	// Open the stream...
	groups := make([]*resourcepb.Group, 0)
	// I will execute a simple ldap search here...
	rqst := new(resourcepb.GetGroupsRqst)
	rqst.Query = query

	stream, err := client.c.GetGroups(client.GetCtx(), rqst)
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

func (client *Resource_Client) UpdateGroup(token string, g *resourcepb.Group) error {
	rqst := new(resourcepb.UpdateGroupRqst)
	rqst.GroupId = g.Id

	rqst.Values = `{"$set":{"name":"` + g.Name + `","description":"` + g.Description + `", "domain":"` + g.Domain + `"}}`
	fmt.Println(rqst.Values)

	// set the token in the context...
	md := metadata.New(map[string]string{"token": string(token), "domain": client.domain})
	ctx := metadata.NewOutgoingContext(client.GetCtx(), md)

	_, err := client.c.UpdateGroup(ctx, rqst)

	return err
}

////////////////////////////////////////////////////////////////////////////////
// Role
////////////////////////////////////////////////////////////////////////////////

/**
 * Create a new role with given action list.
 */
func (client *Resource_Client) CreateRole(token, id, name string, actions []string) error {
	rqst := new(resourcepb.CreateRoleRqst)
	role := new(resourcepb.Role)
	role.Id = id
	role.Name = name
	role.Actions = actions
	rqst.Role = role

	// set the token in the context...
	md := metadata.New(map[string]string{"token": string(token), "domain": client.domain})
	ctx := metadata.NewOutgoingContext(client.GetCtx(), md)
	_, err := client.c.CreateRole(ctx, rqst)

	return err
}

func (client *Resource_Client) DeleteRole(name string) error {
	rqst := new(resourcepb.DeleteRoleRqst)
	rqst.RoleId = name

	_, err := client.c.DeleteRole(client.GetCtx(), rqst)

	return err
}

func (client *Resource_Client) UpdateRole(token string, r *resourcepb.Role) error {
	rqst := new(resourcepb.UpdateRoleRqst)
	rqst.RoleId = r.Id
	rqst.Values = `{"$set":{"name":"` + r.Name + `","description":"` + r.Description + `", "domain":"` + r.Domain + `"}}`

	// set the token in the context...
	md := metadata.New(map[string]string{"token": string(token), "domain": client.domain})
	ctx := metadata.NewOutgoingContext(client.GetCtx(), md)

	_, err := client.c.UpdateRole(ctx, rqst)

	return err
}

/**
 * Add a action to a given role.
 */
func (client *Resource_Client) AddRoleActions(roleId string, actions []string) error {
	rqst := &resourcepb.AddRoleActionsRqst{
		RoleId:  roleId,
		Actions: actions,
	}
	_, err := client.c.AddRoleActions(client.GetCtx(), rqst)

	return err
}

/**
 * Remove action from a given role.
 */
func (client *Resource_Client) RemoveRoleAction(roleId string, action string) error {
	rqst := &resourcepb.RemoveRoleActionRqst{
		RoleId: roleId,
		Action: action,
	}
	_, err := client.c.RemoveRoleAction(client.GetCtx(), rqst)

	return err
}

/**
 * Remove an action from all roles.
 */
func (client *Resource_Client) RemoveRolesAction(action string) error {
	rqst := &resourcepb.RemoveRolesActionRqst{
		Action: action,
	}
	_, err := client.c.RemoveRolesAction(client.GetCtx(), rqst)

	return err
}

/**
 * Remove action from a given application.
 */
func (client *Resource_Client) GetRoles(query string) ([]*resourcepb.Role, error) {
	rqst := &resourcepb.GetRolesRqst{
		Query: query,
	}

	stream, err := client.c.GetRoles(client.GetCtx(), rqst)
	if err != nil {
		return nil, err
	}

	roles := make([]*resourcepb.Role, 0)

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

		roles = append(roles, msg.Roles...)
	}

	return roles, nil
}

////////////////////////////////////////////////////////////////////////////////
// Peer
////////////////////////////////////////////////////////////////////////////////

// Register a peer with a given name and mac address.
func (client *Resource_Client) RegisterPeer(key string, peer *resourcepb.Peer) (*resourcepb.Peer, string, error) {
	rqst := &resourcepb.RegisterPeerRqst{
		Peer:      peer,
		PublicKey: string(key),
	}

	rsp, err := client.c.RegisterPeer(client.GetCtx(), rqst)
	if err != nil {
		return nil, "", err
	}

	return rsp.Peer, rsp.PublicKey, err

}

// Update peer
func (client *Resource_Client) UpdatePeer(token string, peer *resourcepb.Peer) error {
	rqst := &resourcepb.UpdatePeerRqst{
		Peer: peer,
	}

	ctx := client.GetCtx()
	if len(token) > 0 {

		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	_, err := client.c.UpdatePeer(ctx, rqst)
	return err
}

// Delete a peer
func (client *Resource_Client) DeletePeer(token, mac string) error {
	rqst := &resourcepb.DeletePeerRqst{
		Peer: &resourcepb.Peer{
			Mac: mac,
		},
	}

	ctx := client.GetCtx()
	if len(token) > 0 {

		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	_, err := client.c.DeletePeer(ctx, rqst)
	return err
}

/**
 * Add a action to a given peer.
 */
func (client *Resource_Client) AddPeerActions(token, mac string, actions []string) error {
	rqst := &resourcepb.AddPeerActionsRqst{
		Mac:     mac,
		Actions: actions,
	}

	ctx := client.GetCtx()
	if len(token) > 0 {

		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}
	_, err := client.c.AddPeerActions(ctx, rqst)

	return err
}

/**
 * Remove action from a given peer.
 */
func (client *Resource_Client) RemovePeerAction(token, mac, action string) error {
	rqst := &resourcepb.RemovePeerActionRqst{
		Mac:    mac,
		Action: action,
	}

	ctx := client.GetCtx()
	if len(token) > 0 {

		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	_, err := client.c.RemovePeerAction(ctx, rqst)

	return err
}

/**
 * Remove action from all peer's.
 */
func (client *Resource_Client) RemovePeersAction(token, action string) error {
	rqst := &resourcepb.RemovePeersActionRqst{
		Action: action,
	}

	ctx := client.GetCtx()
	if len(token) > 0 {

		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	_, err := client.c.RemovePeersAction(ctx, rqst)

	return err
}

/**
 * Retreive the peer public key
 */
func (client *Resource_Client) GetPeerPublicKey(mac string) (string, error) {
	rqst := &resourcepb.GetPeerPublicKeyRqst{
		Mac: mac,
	}

	rsp, err := client.c.GetPeerPublicKey(context.Background(), rqst)

	if err != nil {
		return "", err
	}
	return rsp.PublicKey, nil
}

/**
 * Get the list of peers.
 */
func (client *Resource_Client) GetPeers(query string) ([]*resourcepb.Peer, error) {
	rqst := &resourcepb.GetPeersRqst{
		Query: query,
	}

	stream, err := client.c.GetPeers(client.GetCtx(), rqst)
	if err != nil {

		return nil, err
	}

	peers := make([]*resourcepb.Peer, 0)

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

		peers = append(peers, msg.Peers...)
	}

	return peers, nil
}

////////////////////////////////////////////////////////////////////////////////
// Application
////////////////////////////////////////////////////////////////////////////////
/**
 * Add a action to a given application.
 */
func (client *Resource_Client) AddApplicationActions(token, applicationId string, actions []string) error {
	rqst := &resourcepb.AddApplicationActionsRqst{
		ApplicationId: applicationId,
		Actions:       actions,
	}

	// set the token in the context...
	md := metadata.New(map[string]string{"token": string(token), "domain": client.domain})
	ctx := metadata.NewOutgoingContext(client.GetCtx(), md)

	_, err := client.c.AddApplicationActions(ctx, rqst)

	return err
}

/**
 * Remove action from a given application.
 */
func (client *Resource_Client) RemoveApplicationAction(token, applicationId, action string) error {
	rqst := &resourcepb.RemoveApplicationActionRqst{
		ApplicationId: applicationId,
		Action:        action,
	}

	// set the token in the context...
	md := metadata.New(map[string]string{"token": string(token), "domain": client.domain})
	ctx := metadata.NewOutgoingContext(client.GetCtx(), md)

	_, err := client.c.RemoveApplicationAction(ctx, rqst)

	return err
}

/**
 * Remove action from a given application.
 */
func (client *Resource_Client) RemoveApplicationsAction(token, action string) error {
	rqst := &resourcepb.RemoveApplicationsActionRqst{
		Action: action,
	}

	// set the token in the context...
	md := metadata.New(map[string]string{"token": string(token), "domain": client.domain})
	ctx := metadata.NewOutgoingContext(client.GetCtx(), md)

	_, err := client.c.RemoveApplicationsAction(ctx, rqst)

	return err
}

/**
 * Retreive applications...
 */
func (client *Resource_Client) GetApplications(query string) ([]*resourcepb.Application, error) {
	rqst := &resourcepb.GetApplicationsRqst{
		Query: query,
	}

	stream, err := client.c.GetApplications(client.GetCtx(), rqst)
	if err != nil {
		return nil, err
	}

	applications := make([]*resourcepb.Application, 0)

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

		applications = append(applications, msg.Applications...)
	}

	return applications, nil
}

/**
 * Delete a given application
 */
func (client *Resource_Client) DeleteApplication(token, id string) error {
	rqst := &resourcepb.DeleteApplicationRqst{
		ApplicationId: id,
	}

	// set the token in the context...
	md := metadata.New(map[string]string{"token": string(token), "domain": client.domain})
	ctx := metadata.NewOutgoingContext(client.GetCtx(), md)

	_, err := client.c.DeleteApplication(ctx, rqst)
	if err != nil {
		return err
	}

	return nil
}

/**
 * Create an application descriptor...
 */
func (client *Resource_Client) CreateApplication(token, id, name, domain, password, path, publisherId, version, description, alias, icon string, actions, keywords []string) error {
	rqst := &resourcepb.CreateApplicationRqst{
		Application: &resourcepb.Application{
			Id:          id,
			Name:        name,
			Path:        path,
			Publisherid: publisherId,
			Version:     version,
			Description: description,
			Alias:       alias,
			Icon:        icon,
			Actions:     actions,
			Keywords:    keywords,
			Domain:      domain,
		},
	}

	// set the token in the context...
	md := metadata.New(map[string]string{"token": string(token), "application": id, "domain": client.domain, "organization": publisherId})
	ctx := metadata.NewOutgoingContext(client.GetCtx(), md)

	_, err := client.c.CreateApplication(ctx, rqst)
	if err != nil {
		return err
	}

	return nil
}

func (client *Resource_Client) UpdateApplication(token string, a *resourcepb.Application) error {
	rqst := new(resourcepb.UpdateApplicationRqst)
	rqst.ApplicationId = a.Id
	rqst.Values = `{"$set":{"version":"` + a.Version + `","path":"` + a.Path + `","icon":"` + a.Icon + `","publisherid":"` + a.Publisherid + `","alias":"` + a.Alias + `","name":"` + a.Name + `","description":"` + a.Description + `", "domain":"` + a.Domain + `"}}`

	// set the token in the context...
	md := metadata.New(map[string]string{"token": string(token), "domain": client.domain})
	ctx := metadata.NewOutgoingContext(client.GetCtx(), md)

	_, err := client.c.UpdateApplication(ctx, rqst)

	return err
}

/**
 * Return the applicaiton version.
 */
func (client *Resource_Client) GetApplicationVersion(id string) (string, error) {

	rqst := &resourcepb.GetApplicationVersionRqst{
		Id: id,
	}

	rsp, err := client.c.GetApplicationVersion(client.GetCtx(), rqst)
	if err != nil {
		return "", err
	}

	return rsp.Version, nil
}

/**
 * Return the applicaiton icon.
 */
func (client *Resource_Client) GetApplicationIcon(id string) (string, error) {

	rqst := &resourcepb.GetApplicationIconRqst{
		Id: id,
	}

	rsp, err := client.c.GetApplicationIcon(client.GetCtx(), rqst)
	if err != nil {
		return "", err
	}

	return rsp.Icon, nil
}

/**
 * Return the applicaiton icon.
 */
func (client *Resource_Client) GetApplicationAlias(id string) (string, error) {

	rqst := &resourcepb.GetApplicationAliasRqst{
		Id: id,
	}

	rsp, err := client.c.GetApplicationAlias(client.GetCtx(), rqst)
	if err != nil {
		return "", err
	}

	return rsp.Alias, nil
}

////////////////////////////////////////////////////////////////////////////////
// Package
////////////////////////////////////////////////////////////////////////////////

func (client *Resource_Client) GetPackageDescriptor(pacakageId, publisherId, version string) (*resourcepb.PackageDescriptor, error) {
	rqst := &resourcepb.GetPackageDescriptorRequest{
		ServiceId:   pacakageId,
		PublisherId: publisherId,
	}

	rsp, err := client.c.GetPackageDescriptor(client.GetCtx(), rqst)
	if err != nil {
		return nil, err
	}

	descriptors := rsp.Results
	descriptor := descriptors[0]
	var lastVersion *Utility.Version

	for i := 0; i < len(descriptors); i++ {
		if len(version) > 0 {
			if descriptors[i].Version == version {
				descriptor = descriptors[i]
				break
			}
		} else {
			if lastVersion == nil {
				lastVersion = Utility.NewVersion(descriptors[i].Version)
				descriptor = descriptors[i]
			} else {
				version_ := Utility.NewVersion(descriptors[i].Version)
				if version_.Compare(lastVersion) == 1 {
					lastVersion = version_
					descriptor = descriptors[i]
				}
			}
		}
	}

	return descriptor, err
}

/**
 * Return the package checksum.
 */
func (client *Resource_Client) GetPackageBundleChecksum(id string) (string, error) {
	rqst := &resourcepb.GetPackageBundleChecksumRequest{
		Id: id,
	}

	rsp, err := client.c.GetPackageBundleChecksum(client.GetCtx(), rqst)
	if err != nil {
		return "", err
	}

	return rsp.Checksum, nil
}

/**
 * Set package bundle information.
 */
func (client *Resource_Client) SetPackageBundle(checksum, platform string, size int32, modified int64, descriptor *resourcepb.PackageDescriptor) error {
	rqst := &resourcepb.SetPackageBundleRequest{
		Bundle: &resourcepb.PackageBundle{
			PackageDescriptor: descriptor,
			Checksum:          checksum,
			Plaform:           platform,
			Size:              size,
			Modified:          modified,
		},
	}

	_, err := client.c.SetPackageBundle(client.GetCtx(), rqst)
	if err != nil {
		return err
	}

	return nil
}

func (client *Resource_Client) CreateNotification(notification *resourcepb.Notification) error {
	rqst := &resourcepb.CreateNotificationRqst{}
	rqst.Notification = notification

	_, err := client.c.CreateNotification(client.GetCtx(), rqst)
	if err != nil {
		return err
	}

	return nil
}
