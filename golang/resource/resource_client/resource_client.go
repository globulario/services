package resource_client

import (
	"context"
	"io"
	"log"
	"strconv"
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

	// The mac address of the server
	mac string

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

func (resource_client *Resource_Client) Invoke(method string, rqst interface{}, ctx context.Context) (interface{}, error) {
	if ctx == nil {
		ctx = globular.GetClientContext(resource_client)
	}
	return globular.InvokeClientRequest(resource_client.c, ctx, method, rqst)
}

// Return the ipv4 address
// Return the address
func (resource_client *Resource_Client) GetAddress() string {
	return resource_client.domain + ":" + strconv.Itoa(resource_client.port)
}

// Return the domain
func (resource_client *Resource_Client) GetDomain() string {
	return resource_client.domain
}

// Return the id of the service instance
func (resource_client *Resource_Client) GetId() string {
	return resource_client.id
}

// Return the name of the service
func (resource_client *Resource_Client) GetName() string {
	return resource_client.name
}

func (resource_client *Resource_Client) GetMac() string {
	return resource_client.mac
}


// must be close when no more needed.
func (resource_client *Resource_Client) Close() {
	resource_client.cc.Close()
}

// Set grpc_service port.
func (resource_client *Resource_Client) SetPort(port int) {
	resource_client.port = port
}

// Set the client name.
func (resource_client *Resource_Client) SetId(id string) {
	resource_client.id = id
}

// Set the client name.
func (resource_client *Resource_Client) SetName(name string) {
	resource_client.name = name
}

func (resource_client *Resource_Client) SetMac(mac string) {
	resource_client.mac = mac
}

// Set the domain.
func (resource_client *Resource_Client) SetDomain(domain string) {
	resource_client.domain = domain
}

////////////////// TLS ///////////////////

// Get if the client is secure.
func (resource_client *Resource_Client) HasTLS() bool {
	return resource_client.hasTLS
}

// Get the TLS certificate file path
func (resource_client *Resource_Client) GetCertFile() string {
	return resource_client.certFile
}

// Get the TLS key file path
func (resource_client *Resource_Client) GetKeyFile() string {
	return resource_client.keyFile
}

// Get the TLS key file path
func (resource_client *Resource_Client) GetCaFile() string {
	return resource_client.caFile
}

// Set the client is a secure client.
func (resource_client *Resource_Client) SetTLS(hasTls bool) {
	resource_client.hasTLS = hasTls
}

// Set TLS certificate file path
func (resource_client *Resource_Client) SetCertFile(certFile string) {
	resource_client.certFile = certFile
}

// Set TLS key file path
func (resource_client *Resource_Client) SetKeyFile(keyFile string) {
	resource_client.keyFile = keyFile
}

// Set TLS authority trust certificate file path
func (resource_client *Resource_Client) SetCaFile(caFile string) {
	resource_client.caFile = caFile
}

////////////// API ////////////////

////////////////////////////////////////////////////////////////////////////////
// Package Descriptor
////////////////////////////////////////////////////////////////////////////////
func (resource_client *Resource_Client) SetPackageDescriptor(descriptor *resourcepb.PackageDescriptor) error {

	// Create a new Organization.
	rqst := &resourcepb.SetPackageDescriptorRequest{
		Descriptor_: descriptor,
	}

	_, err := resource_client.c.SetPackageDescriptor(globular.GetClientContext(resource_client), rqst)
	return err

}

////////////////////////////////////////////////////////////////////////////////
// Organisation
////////////////////////////////////////////////////////////////////////////////

// Create a new Organization
func (resource_client *Resource_Client) CreateOrganization(id string, name string) error {

	// Create a new Organization.
	rqst := &resourcepb.CreateOrganizationRqst{
		Organization: &resourcepb.Organization{
			Id:   id,
			Name: name,
		},
	}

	_, err := resource_client.c.CreateOrganization(globular.GetClientContext(resource_client), rqst)
	return err

}

// Create a new Organization
func (resource_client *Resource_Client) DeleteOrganization(id string) error {

	// Create a new Organization.
	rqst := &resourcepb.DeleteOrganizationRqst{
		Organization: id,
	}

	_, err := resource_client.c.DeleteOrganization(globular.GetClientContext(resource_client), rqst)
	return err

}

// Add to Organisation...
func (resource_client *Resource_Client) AddOrganizationAccount(organisationId string, accountId string) error {

	// Create a new Organization.
	rqst := &resourcepb.AddOrganizationAccountRqst{
		OrganizationId: organisationId,
		AccountId:      accountId,
	}

	_, err := resource_client.c.AddOrganizationAccount(globular.GetClientContext(resource_client), rqst)
	return err
}

func (resource_client *Resource_Client) AddOrganizationRole(organisationId string, roleId string) error {

	// Create a new Organization.
	rqst := &resourcepb.AddOrganizationRoleRqst{
		OrganizationId: organisationId,
		RoleId:         roleId,
	}

	_, err := resource_client.c.AddOrganizationRole(globular.GetClientContext(resource_client), rqst)
	return err
}

func (resource_client *Resource_Client) AddOrganizationGroup(organisationId string, groupId string) error {

	// Create a new Organization.
	rqst := &resourcepb.AddOrganizationGroupRqst{
		OrganizationId: organisationId,
		GroupId:        groupId,
	}

	_, err := resource_client.c.AddOrganizationGroup(globular.GetClientContext(resource_client), rqst)
	return err
}

func (resource_client *Resource_Client) AddOrganizationApplication(organisationId string, applicationId string) error {

	// Create a new Organization.
	rqst := &resourcepb.AddOrganizationApplicationRqst{
		OrganizationId: organisationId,
		ApplicationId:  applicationId,
	}

	_, err := resource_client.c.AddOrganizationApplication(globular.GetClientContext(resource_client), rqst)
	return err
}

// Remove from organization

func (resource_client *Resource_Client) RemoveOrganizationAccount(organisationId string, accountId string) error {

	// Create a new Organization.
	rqst := &resourcepb.RemoveOrganizationAccountRqst{
		OrganizationId: organisationId,
		AccountId:      accountId,
	}

	_, err := resource_client.c.RemoveOrganizationAccount(globular.GetClientContext(resource_client), rqst)
	return err
}

func (resource_client *Resource_Client) RemoveOrganizationRole(organisationId string, roleId string) error {

	// Create a new Organization.
	rqst := &resourcepb.RemoveOrganizationRoleRqst{
		OrganizationId: organisationId,
		RoleId:         roleId,
	}

	_, err := resource_client.c.RemoveOrganizationRole(globular.GetClientContext(resource_client), rqst)
	return err
}

func (resource_client *Resource_Client) RemoveOrganizationGroup(organisationId string, groupId string) error {

	// Create a new Organization.
	rqst := &resourcepb.RemoveOrganizationGroupRqst{
		OrganizationId: organisationId,
		GroupId:        groupId,
	}

	_, err := resource_client.c.RemoveOrganizationGroup(globular.GetClientContext(resource_client), rqst)
	return err
}

func (resource_client *Resource_Client) RemoveOrganizationApplication(organisationId string, applicationId string) error {

	// Create a new Organization.
	rqst := &resourcepb.RemoveOrganizationApplicationRqst{
		OrganizationId: organisationId,
		ApplicationId:  applicationId,
	}

	_, err := resource_client.c.RemoveOrganizationApplication(globular.GetClientContext(resource_client), rqst)
	return err
}

func (resource_client *Resource_Client) IsOrganizationMemeber(user, organization string) (bool, error) {
	rqst := &resourcepb.IsOrgnanizationMemberRqst{
		AccountId:      user,
		OrganizationId: organization,
	}

	rsp, err := resource_client.c.IsOrgnanizationMember(globular.GetClientContext(resource_client), rqst)

	if err != nil {
		return false, err
	}

	return rsp.Result, nil
}

////////////////////////////////////////////////////////////////////////////////
// Account
////////////////////////////////////////////////////////////////////////////////

// Register a new Account.
func (resource_client *Resource_Client) RegisterAccount(name string, email string, password string, confirmation_password string) error {
	rqst := &resourcepb.RegisterAccountRqst{
		Account: &resourcepb.Account{
			Name:     name,
			Email:    email,
			Password: password,
		},
		ConfirmPassword: confirmation_password,
	}

	_, err := resource_client.c.RegisterAccount(globular.GetClientContext(resource_client), rqst)
	return err
}

// Get account with a given id/name
func (resource_client *Resource_Client) GetAccount(id string) (*resourcepb.Account, error) {
	rqst := &resourcepb.GetAccountRqst{
		AccountId: id,
	}
	rsp, err := resource_client.c.GetAccount(globular.GetClientContext(resource_client), rqst)

	if err != nil {
		return nil, err
	}

	return rsp.Account, nil
}

// Set the new password.
func (resource_client *Resource_Client) SetAccountPassword(accountId, oldPassword, newPassword string) error {
	rqst := &resourcepb.SetAccountPasswordRqst{
		AccountId:   accountId,
		OldPassword: oldPassword,
		NewPassword: newPassword,
	}

	_, err := resource_client.c.SetAccountPassword(globular.GetClientContext(resource_client), rqst)

	if err != nil {
		return err
	}
	return nil
}

// Delete an account.
func (resource_client *Resource_Client) DeleteAccount(id string) error {
	rqst := &resourcepb.DeleteAccountRqst{
		Id: id,
	}

	_, err := resource_client.c.DeleteAccount(globular.GetClientContext(resource_client), rqst)
	return err
}

/**
 * Set role to a account
 */
func (resource_client *Resource_Client) AddAccountRole(accountId string, roleId string) error {
	rqst := &resourcepb.AddAccountRoleRqst{
		AccountId: accountId,
		RoleId:    roleId,
	}
	_, err := resource_client.c.AddAccountRole(globular.GetClientContext(resource_client), rqst)

	return err
}

/**
 * Remove role from an account
 */
func (resource_client *Resource_Client) RemoveAccountRole(accountId string, roleId string) error {
	rqst := &resourcepb.RemoveAccountRoleRqst{
		AccountId: accountId,
		RoleId:    roleId,
	}
	_, err := resource_client.c.RemoveAccountRole(globular.GetClientContext(resource_client), rqst)

	return err
}

////////////////////////////////////////////////////////////////////////////////
// Sessions
////////////////////////////////////////////////////////////////////////////////

/**
 * Return a given session
 */
func (resource_client *Resource_Client) GetSession(accountId string) (*resourcepb.Session, error) {
	rqst := &resourcepb.GetSessionRequest{
		AccountId: accountId,
	}
	rsp, err := resource_client.c.GetSession(globular.GetClientContext(resource_client), rqst)
	if err != nil {
		return nil, err
	}

	return rsp.Session, nil
}

/**
 * Return the list of all active sessions on the server.
 */
func (resource_client *Resource_Client) GetSessions(query string) ([]*resourcepb.Session, error) {
	rqst := &resourcepb.GetSessionsRequest{}
	rqst.Query = query
	rsp, err := resource_client.c.GetSessions(globular.GetClientContext(resource_client), rqst)
	if err != nil {
		return nil, err
	}

	return rsp.Sessions, nil
}

/**
 * Remove a session
 */
func (resource_client *Resource_Client) RemoveSession(accountId string) error {
	rqst := &resourcepb.RemoveSessionRequest{
		AccountId: accountId,
	}
	_, err := resource_client.c.RemoveSession(globular.GetClientContext(resource_client), rqst)

	return err
}

/**
 * Update/Create a session.
 */
func (resource_client *Resource_Client) UpdateSession(session *resourcepb.Session) error {
	rqst := &resourcepb.UpdateSessionRequest{
		Session: session,
	}

	_, err := resource_client.c.UpdateSession(globular.GetClientContext(resource_client), rqst)

	return err
}

////////////////////////////////////////////////////////////////////////////////
// Group
////////////////////////////////////////////////////////////////////////////////

/**
 * Create a new group.
 */
func (resource_client *Resource_Client) CreateGroup(id string, name string) error {
	rqst := new(resourcepb.CreateGroupRqst)
	g := new(resourcepb.Group)
	g.Name = name
	g.Id = id
	rqst.Group = g
	ctx := globular.GetClientContext(resource_client)
	_, err := resource_client.c.CreateGroup(ctx, rqst)

	return err
}

func (resource_client *Resource_Client) AddGroupMemberAccount(groupId string, accountId string) error {
	rqst := new(resourcepb.AddGroupMemberAccountRqst)
	rqst.AccountId = accountId
	rqst.GroupId = groupId

	ctx := globular.GetClientContext(resource_client)
	_, err := resource_client.c.AddGroupMemberAccount(ctx, rqst)
	return err
}

func (resource_client *Resource_Client) DeleteGroup(groupId string) error {
	rqst := new(resourcepb.DeleteGroupRqst)
	rqst.Group = groupId

	ctx := globular.GetClientContext(resource_client)
	_, err := resource_client.c.DeleteGroup(ctx, rqst)
	return err
}

func (resource_client *Resource_Client) RemoveGroupMemberAccount(groupId string, accountId string) error {
	rqst := new(resourcepb.RemoveGroupMemberAccountRqst)
	rqst.AccountId = accountId
	rqst.GroupId = groupId

	ctx := globular.GetClientContext(resource_client)
	_, err := resource_client.c.RemoveGroupMemberAccount(ctx, rqst)
	return err
}

func (resource_client *Resource_Client) GetGroups(query string) ([]*resourcepb.Group, error) {

	// Open the stream...
	groups := make([]*resourcepb.Group, 0)
	// I will execute a simple ldap search here...
	rqst := new(resourcepb.GetGroupsRqst)
	rqst.Query = query

	stream, err := resource_client.c.GetGroups(globular.GetClientContext(resource_client), rqst)
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
func (resource_client *Resource_Client) CreateRole(id string, name string, actions []string) error {
	rqst := new(resourcepb.CreateRoleRqst)
	role := new(resourcepb.Role)
	role.Id = id
	role.Name = name
	role.Actions = actions
	rqst.Role = role
	_, err := resource_client.c.CreateRole(globular.GetClientContext(resource_client), rqst)

	return err
}

func (resource_client *Resource_Client) DeleteRole(name string) error {
	rqst := new(resourcepb.DeleteRoleRqst)
	rqst.RoleId = name

	_, err := resource_client.c.DeleteRole(globular.GetClientContext(resource_client), rqst)

	return err
}

/**
 * Add a action to a given role.
 */
func (resource_client *Resource_Client) AddRoleActions(roleId string, actions []string) error {
	rqst := &resourcepb.AddRoleActionsRqst{
		RoleId:  roleId,
		Actions: actions,
	}
	_, err := resource_client.c.AddRoleActions(globular.GetClientContext(resource_client), rqst)

	return err
}

/**
 * Remove action from a given role.
 */
func (resource_client *Resource_Client) RemoveRoleAction(roleId string, action string) error {
	rqst := &resourcepb.RemoveRoleActionRqst{
		RoleId: roleId,
		Action: action,
	}
	_, err := resource_client.c.RemoveRoleAction(globular.GetClientContext(resource_client), rqst)

	return err
}

/**
 * Remove an action from all roles.
 */
func (resource_client *Resource_Client) RemoveRolesAction(action string) error {
	rqst := &resourcepb.RemoveRolesActionRqst{
		Action: action,
	}
	_, err := resource_client.c.RemoveRolesAction(globular.GetClientContext(resource_client), rqst)

	return err
}

/**
 * Remove action from a given application.
 */
func (resource_client *Resource_Client) GetRoles(query string) ([]*resourcepb.Role, error) {
	rqst := &resourcepb.GetRolesRqst{
		Query: query,
	}

	stream, err := resource_client.c.GetRoles(globular.GetClientContext(resource_client), rqst)
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
func (resource_client *Resource_Client) RegisterPeer(token, mac, domain, address, key, secret string) (*resourcepb.Peer, string, error) {
	log.Println("Register peer ", domain)
	rqst := &resourcepb.RegisterPeerRqst{
		Peer: &resourcepb.Peer{
			Domain:  domain,
			Mac:     mac,
			Address: address,
		},
		PublicKey: string(key),
		Secret: secret,
	}

	ctx := globular.GetClientContext(resource_client)
	if len(token) > 0 {
		md, _ := metadata.FromOutgoingContext(ctx)
		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	rsp, err := resource_client.c.RegisterPeer(ctx, rqst)
	if err != nil {
		log.Println(err)
		return nil, "", err
	}

	return rsp.Peer, rsp.PublicKey, err

}

// Delete a peer
func (resource_client *Resource_Client) DeletePeer(token, domain string) error {
	rqst := &resourcepb.DeletePeerRqst{
		Peer: &resourcepb.Peer{
			Domain: domain,
		},
	}

	ctx := globular.GetClientContext(resource_client)
	if len(token) > 0 {

		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	_, err := resource_client.c.DeletePeer(ctx, rqst)
	return err
}

/**
 * Add a action to a given peer.
 */
func (resource_client *Resource_Client) AddPeerActions(token, domain string, actions []string) error {
	rqst := &resourcepb.AddPeerActionsRqst{
		Domain:  domain,
		Actions: actions,
	}

	ctx := globular.GetClientContext(resource_client)
	if len(token) > 0 {

		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}
	_, err := resource_client.c.AddPeerActions(ctx, rqst)

	return err
}

/**
 * Remove action from a given peer.
 */
func (resource_client *Resource_Client) RemovePeerAction(token, domain, action string) error {
	rqst := &resourcepb.RemovePeerActionRqst{
		Domain: domain,
		Action: action,
	}

	ctx := globular.GetClientContext(resource_client)
	if len(token) > 0 {

		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	_, err := resource_client.c.RemovePeerAction(ctx, rqst)

	return err
}

/**
 * Remove action from all peer's.
 */
func (resource_client *Resource_Client) RemovePeersAction(token, action string) error {
	rqst := &resourcepb.RemovePeersActionRqst{
		Action: action,
	}

	ctx := globular.GetClientContext(resource_client)
	if len(token) > 0 {

		md, _ := metadata.FromOutgoingContext(ctx)

		if len(md.Get("token")) != 0 {
			md.Set("token", token)
		}
		ctx = metadata.NewOutgoingContext(context.Background(), md)
	}

	_, err := resource_client.c.RemovePeersAction(ctx, rqst)

	return err
}

/**
 * Remove action from a given application.
 */
func (resource_client *Resource_Client) GetPeers(query string) ([]*resourcepb.Peer, error) {
	rqst := &resourcepb.GetPeersRqst{
		Query: query,
	}

	stream, err := resource_client.c.GetPeers(globular.GetClientContext(resource_client), rqst)
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
func (resource_client *Resource_Client) AddApplicationActions(applicationId string, actions []string) error {
	rqst := &resourcepb.AddApplicationActionsRqst{
		ApplicationId: applicationId,
		Actions:       actions,
	}
	_, err := resource_client.c.AddApplicationActions(globular.GetClientContext(resource_client), rqst)

	return err
}

/**
 * Remove action from a given application.
 */
func (resource_client *Resource_Client) RemoveApplicationAction(applicationId string, action string) error {
	rqst := &resourcepb.RemoveApplicationActionRqst{
		ApplicationId: applicationId,
		Action:        action,
	}
	_, err := resource_client.c.RemoveApplicationAction(globular.GetClientContext(resource_client), rqst)

	return err
}

/**
 * Remove action from a given application.
 */
func (resource_client *Resource_Client) RemoveApplicationsAction(action string) error {
	rqst := &resourcepb.RemoveApplicationsActionRqst{
		Action: action,
	}
	_, err := resource_client.c.RemoveApplicationsAction(globular.GetClientContext(resource_client), rqst)

	return err
}

/**
 * Retreive applications...
 */
func (resource_client *Resource_Client) GetApplications(query string) ([]*resourcepb.Application, error) {
	rqst := &resourcepb.GetApplicationsRqst{
		Query: query,
	}

	stream, err := resource_client.c.GetApplications(globular.GetClientContext(resource_client), rqst)
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
func (resource_client *Resource_Client) DeleteApplication(id string) error {
	rqst := &resourcepb.DeleteApplicationRqst{
		ApplicationId: id,
	}

	_, err := resource_client.c.DeleteApplication(globular.GetClientContext(resource_client), rqst)
	if err != nil {
		return err
	}

	return nil
}

/**
 * Create an application descriptor...
 */
func (resource_client *Resource_Client) CreateApplication(id, password, path, publisherId, version, description, alias, icon string, actions, keywords []string) error {
	rqst := &resourcepb.CreateApplicationRqst{
		Application: &resourcepb.Application{
			Id:          id,
			Path:        path,
			Publisherid: publisherId,
			Version:     version,
			Description: description,
			Alias:       alias,
			Icon:        icon,
			Actions:     actions,
			Keywords:    keywords,
		},
	}

	_, err := resource_client.c.CreateApplication(globular.GetClientContext(resource_client), rqst)
	if err != nil {
		return err
	}

	return nil
}

/**
 * Return the applicaiton version.
 */
func (resource_client *Resource_Client) GetApplicationVersion(id string) (string, error) {

	rqst := &resourcepb.GetApplicationVersionRqst{
		Id: id,
	}

	rsp, err := resource_client.c.GetApplicationVersion(globular.GetClientContext(resource_client), rqst)
	if err != nil {
		return "", err
	}

	return rsp.Version, nil
}

/**
 * Return the applicaiton icon.
 */
func (resource_client *Resource_Client) GetApplicationIcon(id string) (string, error) {

	rqst := &resourcepb.GetApplicationIconRqst{
		Id: id,
	}

	rsp, err := resource_client.c.GetApplicationIcon(globular.GetClientContext(resource_client), rqst)
	if err != nil {
		return "", err
	}

	return rsp.Icon, nil
}

/**
 * Return the applicaiton icon.
 */
func (resource_client *Resource_Client) GetApplicationAlias(id string) (string, error) {

	rqst := &resourcepb.GetApplicationAliasRqst{
		Id: id,
	}

	rsp, err := resource_client.c.GetApplicationAlias(globular.GetClientContext(resource_client), rqst)
	if err != nil {
		return "", err
	}

	return rsp.Alias, nil
}

////////////////////////////////////////////////////////////////////////////////
// Package
////////////////////////////////////////////////////////////////////////////////

func (resource_client *Resource_Client) GetPackageDescriptor(pacakageId, publisherId, version string) (*resourcepb.PackageDescriptor, error) {
	rqst := &resourcepb.GetPackageDescriptorRequest{
		ServiceId:   pacakageId,
		PublisherId: publisherId,
	}

	rsp, err := resource_client.c.GetPackageDescriptor(globular.GetClientContext(resource_client), rqst)
	if err != nil {
		return nil, err
	}

	descriptors := rsp.Results
	descriptor := descriptors[0]
	for i := 0; i < len(descriptors); i++ {
		if descriptors[i].Version == version {
			descriptor = descriptors[i]
			break
		}
	}

	return descriptor, err
}

/**
 * Return the package checksum.
 */
func (resource_client *Resource_Client) GetPackageBundleChecksum(id string) (string, error) {
	rqst := &resourcepb.GetPackageBundleChecksumRequest{
		Id: id,
	}

	rsp, err := resource_client.c.GetPackageBundleChecksum(globular.GetClientContext(resource_client), rqst)
	if err != nil {
		return "", err
	}

	return rsp.Checksum, nil
}

/**
 * Set package bundle information.
 */
func (resource_client *Resource_Client) SetPackageBundle(checksum, platform string, size int32, modified int64, descriptor *resourcepb.PackageDescriptor) error {
	rqst := &resourcepb.SetPackageBundleRequest{
		Bundle: &resourcepb.PackageBundle{
			Descriptor_: descriptor,
			Checksum:    checksum,
			Plaform:     platform,
			Size:        size,
			Modified:    modified,
		},
	}

	_, err := resource_client.c.SetPackageBundle(globular.GetClientContext(resource_client), rqst)
	if err != nil {
		return err
	}

	return nil
}

func (resource_client *Resource_Client) CreateNotification(notification *resourcepb.Notification) error {
	rqst := &resourcepb.CreateNotificationRqst{}
	rqst.Notification = notification

	_, err := resource_client.c.CreateNotification(globular.GetClientContext(resource_client), rqst)
	if err != nil {
		return err
	}

	return nil
}
