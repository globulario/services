// Package main refactors the original server.go to match the clean structure
// you liked (Echo-style):
//   - slog for structured logs
//   - --describe and --health CLI paths that do not touch etcd before printing
//   - lean main() that wires Globular lifecycle (Init/Save/Start/Stop)
//   - clear getters/setters to satisfy Globular's service contract
//   - concise, readable errors
package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	// Gate calls into config/etcd until after we handle --describe/--health
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/event/event_client"
	"github.com/globulario/services/golang/globular_client"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/persistence/persistence_client"
	"github.com/globulario/services/golang/persistence/persistence_store"
	"github.com/globulario/services/golang/rbac/rbac_client"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/storage/storage_store"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	// Keep the original service proto import so we can register the server type
	// (handlers are defined elsewhere in this package).
	"github.com/globulario/services/golang/resource/resource_client"
	"github.com/globulario/services/golang/resource/resourcepb"
)

// -----------------------------------------------------------------------------
// Defaults & CORS
// -----------------------------------------------------------------------------

var (
	defaultPort  = 10000
	defaultProxy = defaultPort + 1

	allowAllOrigins   = true
	allowedOriginsStr = ""
)

// -----------------------------------------------------------------------------
// Server type (unchanged fields preserved, ordering cleaned)
// -----------------------------------------------------------------------------

type server struct {
	// Core metadata
	Id           string
	Mac          string
	Name         string
	Domain       string
	Address      string
	Path         string
	Proto        string
	Port         int
	Proxy        int
	Protocol     string
	Version      string
	PublisherID  string
	Description  string
	Keywords     []string
	Repositories []string
	Discoveries  []string

	// Policy / ops
	AllowAllOrigins bool
	AllowedOrigins  string
	KeepUpToDate    bool
	Plaform         string
	Checksum        string
	KeepAlive       bool
	Permissions     []interface{}
	Dependencies    []string
	Process         int
	ProxyProcess    int
	ConfigPath      string
	LastError       string
	State           string
	ModTime         int64

	// TLS
	TLS                bool
	CertFile           string
	KeyFile            string
	CertAuthorityTrust string

	// The session time out.
	SessionTimeout int

	// Data store where account, role ect are keep...
	store persistence_store.Store

	// In-memory cache to speed up things.
	cache    *storage_store.BigCache_store
	cacheTTL time.Duration

	isReady bool

	// The grpc server.
	grpcServer *grpc.Server

	// The backend infos.
	Backend_type               string
	Backend_address            string
	Backend_port               int64
	Backend_user               string
	Backend_password           string
	Backend_replication_factor int64
	DataPath                   string
}

// -----------------------------------------------------------------------------
// Globular service contract — getters/setters kept concise
// -----------------------------------------------------------------------------

func (s *server) GetConfigurationPath() string          { return s.ConfigPath }
func (s *server) SetConfigurationPath(path string)      { s.ConfigPath = path }
func (s *server) GetAddress() string                    { return s.Address }
func (s *server) SetAddress(address string)             { s.Address = address }
func (s *server) GetProcess() int                       { return s.Process }
func (s *server) SetProcess(pid int)                    { s.Process = pid }
func (s *server) GetProxyProcess() int                  { return s.ProxyProcess }
func (s *server) SetProxyProcess(pid int)               { s.ProxyProcess = pid }
func (s *server) GetLastError() string                  { return s.LastError }
func (s *server) SetLastError(err string)               { s.LastError = err }
func (s *server) SetModTime(modtime int64)              { s.ModTime = modtime }
func (s *server) GetModTime() int64                     { return s.ModTime }
func (s *server) GetId() string                         { return s.Id }
func (s *server) SetId(id string)                       { s.Id = id }
func (s *server) GetName() string                       { return s.Name }
func (s *server) SetName(name string)                   { s.Name = name }
func (s *server) GetMac() string                        { return s.Mac }
func (s *server) SetMac(mac string)                     { s.Mac = mac }
func (s *server) GetDescription() string                { return s.Description }
func (s *server) SetDescription(description string)     { s.Description = description }
func (s *server) GetKeywords() []string                 { return s.Keywords }
func (s *server) SetKeywords(keywords []string)         { s.Keywords = keywords }
func (s *server) GetRepositories() []string             { return s.Repositories }
func (s *server) SetRepositories(repositories []string) { s.Repositories = repositories }
func (s *server) GetDiscoveries() []string              { return s.Discoveries }
func (s *server) SetDiscoveries(discoveries []string)   { s.Discoveries = discoveries }
func (s *server) Dist(path string) (string, error)      { return globular.Dist(path, s) }
func (s *server) GetDependencies() []string {
	if s.Dependencies == nil {
		s.Dependencies = make([]string, 0)
	}
	return s.Dependencies
}
func (s *server) SetDependency(dep string) {
	if s.Dependencies == nil {
		s.Dependencies = make([]string, 0)
	}
	if !Utility.Contains(s.Dependencies, dep) {
		s.Dependencies = append(s.Dependencies, dep)
	}
}
func (s *server) GetChecksum() string               { return s.Checksum }
func (s *server) SetChecksum(checksum string)       { s.Checksum = checksum }
func (s *server) GetPlatform() string               { return s.Plaform }
func (s *server) SetPlatform(platform string)       { s.Plaform = platform }
func (s *server) GetPath() string                   { return s.Path }
func (s *server) SetPath(path string)               { s.Path = path }
func (s *server) GetProto() string                  { return s.Proto }
func (s *server) SetProto(proto string)             { s.Proto = proto }
func (s *server) GetPort() int                      { return s.Port }
func (s *server) SetPort(port int)                  { s.Port = port }
func (s *server) GetProxy() int                     { return s.Proxy }
func (s *server) SetProxy(proxy int)                { s.Proxy = proxy }
func (s *server) GetProtocol() string               { return s.Protocol }
func (s *server) SetProtocol(protocol string)       { s.Protocol = protocol }
func (s *server) GetAllowAllOrigins() bool          { return s.AllowAllOrigins }
func (s *server) SetAllowAllOrigins(v bool)         { s.AllowAllOrigins = v }
func (s *server) GetAllowedOrigins() string         { return s.AllowedOrigins }
func (s *server) SetAllowedOrigins(v string)        { s.AllowedOrigins = v }
func (s *server) GetState() string                  { return s.State }
func (s *server) SetState(state string)             { s.State = state }
func (s *server) GetDomain() string                 { return s.Domain }
func (s *server) SetDomain(domain string)           { s.Domain = domain }
func (s *server) GetTls() bool                      { return s.TLS }
func (s *server) SetTls(hasTls bool)                { s.TLS = hasTls }
func (s *server) GetCertAuthorityTrust() string     { return s.CertAuthorityTrust }
func (s *server) SetCertAuthorityTrust(ca string)   { s.CertAuthorityTrust = ca }
func (s *server) GetCertFile() string               { return s.CertFile }
func (s *server) SetCertFile(certFile string)       { s.CertFile = certFile }
func (s *server) GetKeyFile() string                { return s.KeyFile }
func (s *server) SetKeyFile(keyFile string)         { s.KeyFile = keyFile }
func (s *server) GetVersion() string                { return s.Version }
func (s *server) SetVersion(version string)         { s.Version = version }
func (s *server) GetPublisherID() string            { return s.PublisherID }
func (s *server) SetPublisherID(id string)          { s.PublisherID = id }
func (s *server) GetKeepUpToDate() bool             { return s.KeepUpToDate }
func (s *server) SetKeepUptoDate(val bool)          { s.KeepUpToDate = val }
func (s *server) GetKeepAlive() bool                { return s.KeepAlive }
func (s *server) SetKeepAlive(val bool)             { s.KeepAlive = val }
func (s *server) GetPermissions() []interface{}     { return s.Permissions }
func (s *server) SetPermissions(perm []interface{}) { s.Permissions = perm }

// Lifecycle
func (s *server) Init() error {
	if err := globular.InitService(s); err != nil {
		return err
	}
	gs, err := globular.InitGrpcServer(s)
	if err != nil {
		return err
	}
	s.grpcServer = gs
	return nil
}
func (s *server) Save() error         { return globular.SaveService(s) }
func (s *server) StartService() error { return globular.StartService(s, s.grpcServer) }
func (s *server) StopService() error  { return globular.StopService(s, s.grpcServer) }

func (srv *server) RolesDefault() []resourcepb.Role {
	domain, _ := config.GetDomain()

	return []resourcepb.Role{
		{
			Id:          "role:resource.viewer",
			Name:        "Resource Viewer",
			Domain:      domain,
			Description: "Read-only access to accounts, roles, groups, orgs, apps, peers, packages, sessions, calls, and notifications.",
			Actions: []string{
				// Orgs / Groups / Roles / Apps (read)
				"/resource.ResourceService/GetOrganizations",
				"/resource.ResourceService/GetGroups",
				"/resource.ResourceService/GetRoles",
				"/resource.ResourceService/GetApplications",
				// Accounts (read)
				"/resource.ResourceService/GetAccount",
				"/resource.ResourceService/GetAccounts",
				// Peers (read)
				"/resource.ResourceService/GetPeers",
				"/resource.ResourceService/GetPeerApprovalState",
				"/resource.ResourceService/GetPeerPublicKey",
				// Packages (read)
				"/resource.ResourceService/FindPackages",
				"/resource.ResourceService/GetPackageDescriptor",
				"/resource.ResourceService/GetPackagesDescriptor",
				"/resource.ResourceService/GetPackageBundleChecksum",
				"/resource.ResourceService/GetApplicationVersion",
				"/resource.ResourceService/GetApplicationAlias",
				"/resource.ResourceService/GetApplicationIcon",
				// Sessions (read)
				"/resource.ResourceService/GetSessions",
				"/resource.ResourceService/GetSession",
				// Calls (read)
				"/resource.ResourceService/GetCallHistory",
				// Notifications (read)
				"/resource.ResourceService/GetNotifications",
				// Membership check
				"/resource.ResourceService/IsOrgnanizationMember",
			},
			TypeName: "resource.Role",
		},
		{
			Id:          "role:resource.editor",
			Name:        "Resource Editor",
			Domain:      domain,
			Description: "Create/update resources and manage memberships/relations. No destructive admin ops.",
			Actions: []string{
				// References
				"/resource.ResourceService/CreateReference",
				"/resource.ResourceService/DeleteReference",
				// Orgs
				"/resource.ResourceService/CreateOrganization",
				"/resource.ResourceService/UpdateOrganization",
				"/resource.ResourceService/AddOrganizationAccount",
				"/resource.ResourceService/AddOrganizationGroup",
				"/resource.ResourceService/AddOrganizationRole",
				"/resource.ResourceService/AddOrganizationApplication",
				"/resource.ResourceService/RemoveOrganizationAccount",
				"/resource.ResourceService/RemoveOrganizationGroup",
				"/resource.ResourceService/RemoveOrganizationRole",
				"/resource.ResourceService/RemoveOrganizationApplication",
				// Groups
				"/resource.ResourceService/CreateGroup",
				"/resource.ResourceService/UpdateGroup",
				"/resource.ResourceService/AddGroupMemberAccount",
				"/resource.ResourceService/RemoveGroupMemberAccount",
				// Accounts (non-destructive)
				"/resource.ResourceService/RegisterAccount",
				"/resource.ResourceService/SetAccount",
				"/resource.ResourceService/SetEmail",
				"/resource.ResourceService/SetAccountContact",
				"/resource.ResourceService/AddAccountRole",
				"/resource.ResourceService/RemoveAccountRole",
				// Roles (non-destructive)
				"/resource.ResourceService/CreateRole",
				"/resource.ResourceService/UpdateRole",
				"/resource.ResourceService/AddRoleActions",
				"/resource.ResourceService/RemoveRoleAction",
				// Applications (non-destructive)
				"/resource.ResourceService/CreateApplication",
				"/resource.ResourceService/UpdateApplication",
				"/resource.ResourceService/AddApplicationActions",
				"/resource.ResourceService/RemoveApplicationAction",
				// Packages (non-destructive write)
				"/resource.ResourceService/SetPackageDescriptor",
				"/resource.ResourceService/SetPackageBundle",
				// Sessions (write)
				"/resource.ResourceService/UpdateSession",
				// Calls (write)
				"/resource.ResourceService/SetCall",
				"/resource.ResourceService/DeleteCall",
				// Notifications (write)
				"/resource.ResourceService/CreateNotification",
				"/resource.ResourceService/DeleteNotification",
				"/resource.ResourceService/ClearAllNotifications",
				"/resource.ResourceService/ClearNotificationsByType",
			},
			TypeName: "resource.Role",
		},
		{
			Id:          "role:resource.admin",
			Name:        "Resource Admin",
			Domain:      domain,
			Description: "Full administrative control over ResourceService.",
			Actions: []string{
				// Everything viewer can do
				"/resource.ResourceService/GetOrganizations",
				"/resource.ResourceService/GetGroups",
				"/resource.ResourceService/GetRoles",
				"/resource.ResourceService/GetApplications",
				"/resource.ResourceService/GetAccount",
				"/resource.ResourceService/GetAccounts",
				"/resource.ResourceService/GetPeers",
				"/resource.ResourceService/GetPeerApprovalState",
				"/resource.ResourceService/GetPeerPublicKey",
				"/resource.ResourceService/FindPackages",
				"/resource.ResourceService/GetPackageDescriptor",
				"/resource.ResourceService/GetPackagesDescriptor",
				"/resource.ResourceService/GetPackageBundleChecksum",
				"/resource.ResourceService/GetApplicationVersion",
				"/resource.ResourceService/GetApplicationAlias",
				"/resource.ResourceService/GetApplicationIcon",
				"/resource.ResourceService/GetSessions",
				"/resource.ResourceService/GetSession",
				"/resource.ResourceService/GetCallHistory",
				"/resource.ResourceService/GetNotifications",
				"/resource.ResourceService/IsOrgnanizationMember",

				// Everything editor can do
				"/resource.ResourceService/CreateReference",
				"/resource.ResourceService/DeleteReference",
				"/resource.ResourceService/CreateOrganization",
				"/resource.ResourceService/UpdateOrganization",
				"/resource.ResourceService/AddOrganizationAccount",
				"/resource.ResourceService/AddOrganizationGroup",
				"/resource.ResourceService/AddOrganizationRole",
				"/resource.ResourceService/AddOrganizationApplication",
				"/resource.ResourceService/RemoveOrganizationAccount",
				"/resource.ResourceService/RemoveOrganizationGroup",
				"/resource.ResourceService/RemoveOrganizationRole",
				"/resource.ResourceService/RemoveOrganizationApplication",
				"/resource.ResourceService/CreateGroup",
				"/resource.ResourceService/UpdateGroup",
				"/resource.ResourceService/AddGroupMemberAccount",
				"/resource.ResourceService/RemoveGroupMemberAccount",
				"/resource.ResourceService/RegisterAccount",
				"/resource.ResourceService/SetAccount",
				"/resource.ResourceService/SetEmail",
				"/resource.ResourceService/SetAccountContact",
				"/resource.ResourceService/AddAccountRole",
				"/resource.ResourceService/RemoveAccountRole",
				"/resource.ResourceService/CreateRole",
				"/resource.ResourceService/UpdateRole",
				"/resource.ResourceService/AddRoleActions",
				"/resource.ResourceService/RemoveRoleAction",
				"/resource.ResourceService/CreateApplication",
				"/resource.ResourceService/UpdateApplication",
				"/resource.ResourceService/AddApplicationActions",
				"/resource.ResourceService/RemoveApplicationAction",
				"/resource.ResourceService/SetPackageDescriptor",
				"/resource.ResourceService/SetPackageBundle",
				"/resource.ResourceService/UpdateSession",
				"/resource.ResourceService/SetCall",
				"/resource.ResourceService/DeleteCall",
				"/resource.ResourceService/CreateNotification",
				"/resource.ResourceService/DeleteNotification",
				"/resource.ResourceService/ClearAllNotifications",
				"/resource.ResourceService/ClearNotificationsByType",

				// Admin-only destructive / global ops
				"/resource.ResourceService/DeleteOrganization",
				"/resource.ResourceService/DeleteGroup",
				"/resource.ResourceService/DeleteRole",
				"/resource.ResourceService/DeleteApplication",
				"/resource.ResourceService/DeleteAccount",
				"/resource.ResourceService/SetAccountPassword",
				"/resource.ResourceService/RemoveSession",
				"/resource.ResourceService/ClearCalls",
				"/resource.ResourceService/RemoveRolesAction",
				"/resource.ResourceService/RemoveApplicationsAction",
				"/resource.ResourceService/RegisterPeer",
				"/resource.ResourceService/UpdatePeer",
				"/resource.ResourceService/DeletePeer",
				"/resource.ResourceService/AddPeerActions",
				"/resource.ResourceService/RemovePeerAction",
				"/resource.ResourceService/RemovePeersAction",
				"/resource.ResourceService/AcceptPeer",
				"/resource.ResourceService/RejectPeer",
			},
			TypeName: "resource.Role",
		},
	}
}

// ////////////////////// Resource Client ////////////////////////////////////////////
func getResourceClient(address string) (*resource_client.Resource_Client, error) {
	Utility.RegisterFunction("NewResourceService_Client", resource_client.NewResourceService_Client)
	client, err := globular_client.GetClient(address, "resource.ResourceService", "NewResourceService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*resource_client.Resource_Client), nil
}

// ////////////////////// persistence Client ////////////////////////////////////////////
func getPersistenceClient(address string) (*persistence_client.Persistence_Client, error) {
	Utility.RegisterFunction("NewPersistenceService_Client", persistence_client.NewPersistenceService_Client)
	client, err := globular_client.GetClient(address, "persistence.PersistenceService", "NewPersistenceService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*persistence_client.Persistence_Client), nil
}

// /////////////////// event service functions ////////////////////////////////////
func getEventClient(address string) (*event_client.Event_Client, error) {
	Utility.RegisterFunction("NewEventService_Client", event_client.NewEventService_Client)
	client, err := globular_client.GetClient(address, "event.EventService", "NewEventService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*event_client.Event_Client), nil
}

// when services state change that publish
func (srv *server) publishEvent(evt string, data []byte, address string) error {

	client, err := getEventClient(address)
	if err != nil {
		return err
	}

	err = client.Publish(evt, data)
	return err
}

// Public event to a peer other than the default one...
func (srv *server) publishRemoteEvent(address, evt string, data []byte) error {

	client, err := getEventClient(address)
	if err != nil {
		return err
	}

	return client.Publish(evt, data)
}

//////////////////////////////////////// RBAC Functions ///////////////////////////////////////////////

/**
 * Get the rbac client.
 */
func getRbacClient(address string) (*rbac_client.Rbac_Client, error) {
	Utility.RegisterFunction("NewRbacService_Client", rbac_client.NewRbacService_Client)
	client, err := globular_client.GetClient(address, "rbac.RbacService", "NewRbacService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*rbac_client.Rbac_Client), nil
}

func (srv *server) addResourceOwner(token, path, subject, resourceType string, subjectType rbacpb.SubjectType) error {
	rbac_client_, err := getRbacClient(srv.Address)
	if err != nil {
		return err
	}

	err = rbac_client_.AddResourceOwner(token, path, subject, resourceType, subjectType)
	return err
}

func (srv *server) deleteResourcePermissions(token, path string) error {
	rbac_client_, err := getRbacClient(srv.Address)
	if err != nil {
		return err
	}

	err = rbac_client_.DeleteResourcePermissions(token, path)
	return err
}

func (srv *server) deleteAllAccess(token, subject string, subjectType rbacpb.SubjectType) error {
	rbac_client_, err := getRbacClient(srv.Address)
	if err != nil {
		return err
	}

	return rbac_client_.DeleteAllAccess(token, subject, subjectType)
}

func (srv *server) SetAccountAllocatedSpace(token, accountId string, space uint64) error {
	rbac_client_, err := getRbacClient(srv.Address)
	if err != nil {
		return err
	}

	return rbac_client_.SetAccountAllocatedSpace(token, accountId, space)
}

////////////////////////////////// Resource functions ///////////////////////////////////////////////

// ** MONGO backend, it must reside on the same server as the resource server (at this time...)

/**
 * Connection to mongo db local store.
 */
func (srv *server) getPersistenceStore() (persistence_store.Store, error) {
	// That service made user of persistence service.

	if srv.store == nil {

		var options_str = ""

		switch srv.Backend_type {
		case "SCYLLA":
			srv.store = new(persistence_store.ScyllaStore)
			options := map[string]interface{}{"keyspace": "local_resource", "replication_factor": srv.Backend_replication_factor, "hosts": []string{srv.Backend_address}, "port": srv.Backend_port}
			options_, _ := Utility.ToJson(options)
			options_str = string(options_)
		case "MONGO":

			process, err := Utility.GetProcessIdsByName("mongod")
			if err != nil {
				logger.Error("fail to get process id for mongod", "error", err)
				return nil, err
			}

			if len(process) == 0 {
				logger.Error("mongod is not running on this server, please start it before starting the resource server")
				return nil, errors.New("mongod is not running on this server, please start it before starting the resource server")
			}

			srv.store = new(persistence_store.MongoStore)

		case "SQL":

			srv.store = new(persistence_store.SqlStore)
			options := map[string]interface{}{"driver": "sqlite3", "charset": "utf8", "path": srv.DataPath + "/sql-data"}
			options_, _ := Utility.ToJson(options)
			options_str = string(options_)

		default:
			return nil, errors.New("unknown backend type " + srv.Backend_type)
		}

		// Connect to the store.
		err := srv.store.Connect("local_resource", srv.Backend_address, int32(srv.Backend_port), srv.Backend_user, srv.Backend_password, "local_resource", 5000, options_str)
		if err != nil {

			logger.Error("fail to connect to store", "error", err)
			os.Exit(1)
		}

		err = srv.store.Ping(context.Background(), "local_resource")
		if err != nil {
			logger.Error("fail to reach store", "error", err)
			return nil, err
		}

		srv.isReady = true

		logger.Info("store is running and ready to be used", "address", srv.Backend_address+":"+Utility.ToString(srv.Backend_port))
		switch srv.Backend_type {
		case "SQL":
			// Create tables if not already exist.
			err := srv.store.(*persistence_store.SqlStore).CreateTable(
				context.Background(),
				"local_resource",
				"local_resource",
				"Accounts",
				[]string{
					"name TEXT",
					"email TEXT",
					"domain TEXT",
					"password TEXT",
					"refresh_token TEXT",
					"first_name TEXT",
					"last_name TEXT",
					"middle_name TEXT",
					"profile_picture TEXT", // data URL or path
				},
			)
			if err != nil {
				logger.Error("fail to create table Accounts", "error", err)
			}

			err = srv.store.(*persistence_store.SqlStore).CreateTable(
				context.Background(), "local_resource", "local_resource", "Applications",
				[]string{"name TEXT", "domain TEXT", "description TEXT", "icon TEXT", "alias TEXT", "password TEXT", "store TEXT", "last_deployed INTEGER", "path TEXT", "version TEXT", "PublisherID TEXT", "creation_date INTEGER"})
			if err != nil {
				logger.Error("fail to create table Applications", "error", err)
			}

			// Create organizations table.
			err = srv.store.(*persistence_store.SqlStore).CreateTable(
				context.Background(), "local_resource", "local_resource", "Organizations",
				[]string{"name TEXT", "domain TEXT", "description TEXT", "icon TEXT", "email TEXT"})
			if err != nil {
				logger.Error("fail to create table Organizations", "error", err)
			}

			// Create roles table.
			err = srv.store.(*persistence_store.SqlStore).CreateTable(
				context.Background(), "local_resource", "local_resource", "Roles",
				[]string{"name TEXT", "domain TEXT", "description TEXT"})
			if err != nil {
				logger.Error("fail to create table Roles", "error", err)
			}

			// Create groups table.
			err = srv.store.(*persistence_store.SqlStore).CreateTable(
				context.Background(), "local_resource", "local_resource", "Groups",
				[]string{"name TEXT", "domain TEXT", "description TEXT"})
			if err != nil {
				logger.Error("fail to create table Groups", "error", err)
			}

			// Create peers table.
			err = srv.store.(*persistence_store.SqlStore).CreateTable(
				context.Background(), "local_resource", "local_resource", "Peers",
				[]string{"domain TEXT", "hostname TEXT", "external_ip_address TEXT", "local_ip_address TEXT", "mac TEXT", "protocol TEXT", "state INTEGER", "PortHTTP INTEGER", "PortHTTPS INTEGER"})
			if err != nil {
				logger.Error("fail to create table Peers", "error", err)
			}

			// (Duplicate Peers creation kept as in your original)
			err = srv.store.(*persistence_store.SqlStore).CreateTable(
				context.Background(), "local_resource", "local_resource", "Peers",
				[]string{"domain TEXT", "hostname TEXT", "external_ip_address TEXT", "local_ip_address TEXT", "mac TEXT", "protocol TEXT", "state INTEGER", "PortHTTP INTEGER", "PortHTTPS INTEGER"})
			if err != nil {
				logger.Error("fail to create table Peers", "error", err)
			}

			// Create the sessions table.
			err = srv.store.(*persistence_store.SqlStore).CreateTable(
				context.Background(), "local_resource", "local_resource", "Sessions",
				[]string{"account_id TEXT", "domain TEXT", "state INTEGER", "last_state_time INTEGER", "expire_at INTEGER"})
			if err != nil {
				logger.Error("fail to create table Sessions", "error", err)
			}

			// Create the notifications table.
			err = srv.store.(*persistence_store.SqlStore).CreateTable(
				context.Background(), "local_resource", "local_resource", "Notifications",
				[]string{"date REAL", "domain TEXT", "message TEXT", "recipient TEXT", "sender TEXT", "mac TEXT", "notification_type INTEGER"})
			if err != nil {
				logger.Error("fail to create table Notifications", "error", err)
			}

		case "SCYLLA":
			// Create tables if not already exist.
			err := srv.store.(*persistence_store.ScyllaStore).CreateTable(
				context.Background(),
				"local_resource",
				"local_resource",
				"Accounts",
				[]string{
					"name TEXT",
					"email TEXT",
					"domain TEXT",
					"password TEXT",
					"first_name TEXT",
					"last_name TEXT",
					"middle_name TEXT",
					"profile_picture TEXT", // data URL or path
				},
			)
			if err != nil {
				logger.Error("fail to create table Accounts", "error", err)
			}

			err = srv.store.(*persistence_store.ScyllaStore).CreateTable(
				context.Background(), "local_resource", "local_resource", "Applications",
				[]string{"name TEXT", "domain TEXT", "description TEXT", "icon TEXT", "alias TEXT", "password TEXT", "store TEXT", "last_deployed INT", "path TEXT", "version TEXT", "PublisherID TEXT", "creation_date INT"})
			if err != nil {
				logger.Error("fail to create table Applications", "error", err)
			}

			// Create organizations table.
			err = srv.store.(*persistence_store.ScyllaStore).CreateTable(
				context.Background(), "local_resource", "local_resource", "Organizations",
				[]string{"name TEXT", "domain TEXT", "description TEXT", "icon TEXT", "email TEXT"})
			if err != nil {
				logger.Error("fail to create table Organizations", "error", err)
			}

			// Create roles table.
			err = srv.store.(*persistence_store.ScyllaStore).CreateTable(
				context.Background(), "local_resource", "local_resource", "Roles",
				[]string{"name TEXT", "domain TEXT", "description TEXT"})
			if err != nil {
				logger.Error("fail to create table Roles", "error", err)
			}

			// Create groups table.
			err = srv.store.(*persistence_store.ScyllaStore).CreateTable(
				context.Background(), "local_resource", "local_resource", "Groups",
				[]string{"name TEXT", "domain TEXT", "description TEXT"})
			if err != nil {
				logger.Error("fail to create table Groups", "error", err)
			}

			// Create peers table.
			err = srv.store.(*persistence_store.ScyllaStore).CreateTable(
				context.Background(), "local_resource", "local_resource", "Peers",
				[]string{"domain TEXT", "hostname TEXT", "external_ip_address TEXT", "local_ip_address TEXT", "mac TEXT", "protocol TEXT", "state INT", "PortHTTP INT", "PortHTTPS INT"})
			if err != nil {
				logger.Error("fail to create table Peers", "error", err)
			}

			// Create the sessions table.
			err = srv.store.(*persistence_store.ScyllaStore).CreateTable(
				context.Background(), "local_resource", "local_resource", "Sessions",
				[]string{"account_id TEXT", "domain TEXT", "state INT", "last_state_time BIGINT", "expire_at BIGINT"})
			if err != nil {
				logger.Error("fail to create table Sessions", "error", err)
			}

			// Create the notifications table.
			err = srv.store.(*persistence_store.ScyllaStore).CreateTable(
				context.Background(), "local_resource", "local_resource", "Notifications",
				[]string{"date DOUBLE", "domain TEXT", "message TEXT", "recipient TEXT", "sender TEXT", "mac TEXT", "notification_type INT"})
			if err != nil {
				logger.Error("fail to create table Notifications", "error", err)
			}

			// I will get the number of peers...
			peers, err := srv.getPeers(`{"state":1}`)
			if err == nil {
				if len(peers) > 0 {
					// Now the replication factor should be nb of peers - 1 and at max 3.
					rf := int64(len(peers))
					if rf > 3 {
						rf = 3
					}
					if rf != srv.Backend_replication_factor {
						srv.Backend_replication_factor = rf
						logger.Info("adjusting scylla replication factor to " + Utility.ToString(srv.Backend_replication_factor) + " (based on number of peers)")
					}
				}
			}

			// run admin script.
			script := `ALTER KEYSPACE local_resource WITH REPLICATION = {'class':'SimpleStrategy','replication_factor':` + Utility.ToString(srv.Backend_replication_factor) + `}`
			err = srv.store.(*persistence_store.ScyllaStore).RunAdminCmd(
				context.Background(), "local_resource", srv.Backend_user, srv.Backend_password, script)
			if err != nil {
				logger.Error("fail to run admin script '%s'", script, "error", err)
			}
		}

	} else if !srv.isReady {
		nbTry := 100
		for range nbTry {
			time.Sleep(100 * time.Millisecond)
			if srv.isReady {
				break
			}
		}
	}

	return srv.store, nil
}

// -----------------------------------------------------------------------------
// main()
// -----------------------------------------------------------------------------

func main() {
	s := new(server)

	// Populate ONLY safe defaults before config/etcd is touched.
	s.Name = string(resourcepb.File_resource_proto.Services().Get(0).FullName())
	s.Proto = resourcepb.File_resource_proto.Path()
	s.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	s.Port = defaultPort
	s.Proxy = defaultProxy
	s.Protocol = "grpc"
	s.Version = "0.0.1"
	s.PublisherID = "localhost"
	s.Description = "Resource service"
	s.Keywords = []string{"resource", "rbac", "accounts", "globular"}
	s.Repositories = make([]string, 0)
	s.Discoveries = make([]string, 0)
	s.Dependencies = make([]string, 0)
	s.Permissions = make([]interface{}, 0)
	s.Process = -1
	s.ProxyProcess = -1
	s.AllowAllOrigins = allowAllOrigins
	s.AllowedOrigins = allowedOriginsStr
	s.KeepAlive = true
	s.KeepUpToDate = true
	s.cacheTTL = 5 * time.Minute // adjust

	// init BigCache
    s.cache = storage_store.NewBigCache_store()

    // pass lifeWindow via JSON (your Open() already supports options)
    // e.g. shards/lifeWindow are bigcache options; tune to your traffic
    _ = s.cache.Open(`{"lifeWindow":"5m","shards":64}`)


	s.Permissions = []interface{}{
		// --- References ----------------------------------------------------------
		map[string]interface{}{
			"action":     "/resource.ResourceService/CreateReference",
			"permission": "write",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "SourceId", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "TargetId", "permission": "write"},
			},
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/DeleteReference",
			"permission": "write",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "RefId", "permission": "write"},
				// Target side is impacted as well
				map[string]interface{}{"index": 0, "field": "TargetId", "permission": "write"},
			},
		},

		// --- Organizations -------------------------------------------------------
		map[string]interface{}{
			"action":     "/resource.ResourceService/CreateOrganization",
			"permission": "admin",
			"resources":  []interface{}{}, // action-level gate only
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/UpdateOrganization",
			"permission": "write",
			"resources":  []interface{}{map[string]interface{}{"index": 0, "field": "OrganizationId", "permission": "write"}},
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/GetOrganizations",
			"permission": "read",
			"resources":  []interface{}{},
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/DeleteOrganization",
			"permission": "admin",
			"resources":  []interface{}{map[string]interface{}{"index": 0, "field": "Organization", "permission": "admin"}},
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/AddOrganizationAccount",
			"permission": "write",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "OrganizationId", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "AccountId", "permission": "write"},
			},
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/AddOrganizationGroup",
			"permission": "write",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "OrganizationId", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "GroupId", "permission": "write"},
			},
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/AddOrganizationRole",
			"permission": "write",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "OrganizationId", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "RoleId", "permission": "write"},
			},
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/AddOrganizationApplication",
			"permission": "write",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "OrganizationId", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "ApplicationId", "permission": "write"},
			},
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/RemoveOrganizationAccount",
			"permission": "write",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "OrganizationId", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "AccountId", "permission": "write"},
			},
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/RemoveOrganizationGroup",
			"permission": "write",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "OrganizationId", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "GroupId", "permission": "write"},
			},
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/RemoveOrganizationRole",
			"permission": "write",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "OrganizationId", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "RoleId", "permission": "write"},
			},
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/RemoveOrganizationApplication",
			"permission": "write",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "OrganizationId", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "ApplicationId", "permission": "write"},
			},
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/IsOrgnanizationMember",
			"permission": "read",
			"resources":  []interface{}{map[string]interface{}{"index": 0, "field": "OrganizationId", "permission": "read"}},
		},

		// --- Groups --------------------------------------------------------------
		map[string]interface{}{
			"action":     "/resource.ResourceService/CreateGroup",
			"permission": "write",
			"resources":  []interface{}{}, // create gate only
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/UpdateGroup",
			"permission": "write",
			"resources":  []interface{}{map[string]interface{}{"index": 0, "field": "GroupId", "permission": "write"}},
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/GetGroups",
			"permission": "read",
			"resources":  []interface{}{},
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/DeleteGroup",
			"permission": "admin",
			"resources":  []interface{}{map[string]interface{}{"index": 0, "field": "Group", "permission": "admin"}},
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/AddGroupMemberAccount",
			"permission": "write",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "GroupId", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "AccountId", "permission": "write"},
			},
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/RemoveGroupMemberAccount",
			"permission": "write",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "GroupId", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "AccountId", "permission": "write"},
			},
		},

		// --- Accounts ------------------------------------------------------------
		map[string]interface{}{
			"action":     "/resource.ResourceService/RegisterAccount",
			"permission": "write",
			"resources":  []interface{}{}, // creation; policy decides who can self-register
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/DeleteAccount",
			"permission": "admin",
			"resources":  []interface{}{map[string]interface{}{"index": 0, "field": "Id", "permission": "admin"}},
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/GetAccount",
			"permission": "read",
			"resources":  []interface{}{map[string]interface{}{"index": 0, "field": "AccountId", "permission": "read"}},
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/SetAccount",
			"permission": "write",
			"resources":  []interface{}{map[string]interface{}{"index": 0, "field": "Account.Id", "permission": "write"}},
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/SetAccountPassword",
			"permission": "admin", // privileged operation
			"resources":  []interface{}{map[string]interface{}{"index": 0, "field": "AccountId", "permission": "admin"}},
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/AddAccountRole",
			"permission": "write",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "AccountId", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "RoleId", "permission": "write"},
			},
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/RemoveAccountRole",
			"permission": "write",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "AccountId", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "RoleId", "permission": "write"},
			},
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/SetAccountContact",
			"permission": "write",
			"resources":  []interface{}{map[string]interface{}{"index": 0, "field": "AccountId", "permission": "write"}},
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/SetEmail",
			"permission": "write",
			"resources":  []interface{}{map[string]interface{}{"index": 0, "field": "AccountId", "permission": "write"}},
		},

		// --- Roles ---------------------------------------------------------------
		map[string]interface{}{
			"action":     "/resource.ResourceService/CreateRole",
			"permission": "write",
			"resources":  []interface{}{}, // create gate only
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/UpdateRole",
			"permission": "write",
			"resources":  []interface{}{map[string]interface{}{"index": 0, "field": "RoleId", "permission": "write"}},
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/DeleteRole",
			"permission": "admin",
			"resources":  []interface{}{map[string]interface{}{"index": 0, "field": "RoleId", "permission": "admin"}},
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/AddRoleActions",
			"permission": "write",
			"resources":  []interface{}{map[string]interface{}{"index": 0, "field": "RoleId", "permission": "write"}},
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/RemoveRoleAction",
			"permission": "write",
			"resources":  []interface{}{map[string]interface{}{"index": 0, "field": "RoleId", "permission": "write"}},
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/RemoveRolesAction",
			"permission": "admin",
			"resources":  []interface{}{}, // global action cleanup
		},

		// --- Applications --------------------------------------------------------
		map[string]interface{}{
			"action":     "/resource.ResourceService/CreateApplication",
			"permission": "write",
			"resources":  []interface{}{}, // create gate only
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/UpdateApplication",
			"permission": "write",
			"resources":  []interface{}{map[string]interface{}{"index": 0, "field": "ApplicationId", "permission": "write"}},
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/DeleteApplication",
			"permission": "admin",
			"resources":  []interface{}{map[string]interface{}{"index": 0, "field": "ApplicationId", "permission": "admin"}},
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/AddApplicationActions",
			"permission": "write",
			"resources":  []interface{}{map[string]interface{}{"index": 0, "field": "ApplicationId", "permission": "write"}},
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/RemoveApplicationAction",
			"permission": "write",
			"resources":  []interface{}{map[string]interface{}{"index": 0, "field": "ApplicationId", "permission": "write"}},
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/RemoveApplicationsAction",
			"permission": "admin",
			"resources":  []interface{}{}, // global action cleanup
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/GetApplicationVersion",
			"permission": "read",
			"resources":  []interface{}{map[string]interface{}{"index": 0, "field": "Id", "permission": "read"}},
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/GetApplicationIcon",
			"permission": "read",
			"resources":  []interface{}{map[string]interface{}{"index": 0, "field": "Id", "permission": "read"}},
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/GetApplicationAlias",
			"permission": "read",
			"resources":  []interface{}{map[string]interface{}{"index": 0, "field": "Id", "permission": "read"}},
		},

		// --- Peers ---------------------------------------------------------------
		map[string]interface{}{
			"action":     "/resource.ResourceService/RegisterPeer",
			"permission": "admin",
			"resources":  []interface{}{}, // new peer; gate action only
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/UpdatePeer",
			"permission": "admin",
			"resources":  []interface{}{map[string]interface{}{"index": 0, "field": "Peer.Mac", "permission": "admin"}},
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/DeletePeer",
			"permission": "admin",
			"resources":  []interface{}{map[string]interface{}{"index": 0, "field": "Peer.Mac", "permission": "admin"}},
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/AddPeerActions",
			"permission": "admin",
			"resources":  []interface{}{map[string]interface{}{"index": 0, "field": "Mac", "permission": "admin"}},
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/RemovePeerAction",
			"permission": "admin",
			"resources":  []interface{}{map[string]interface{}{"index": 0, "field": "Mac", "permission": "admin"}},
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/RemovePeersAction",
			"permission": "admin",
			"resources":  []interface{}{}, // global action cleanup
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/AcceptPeer",
			"permission": "admin",
			"resources":  []interface{}{map[string]interface{}{"index": 0, "field": "Peer.Mac", "permission": "admin"}},
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/RejectPeer",
			"permission": "admin",
			"resources":  []interface{}{map[string]interface{}{"index": 0, "field": "Peer.Mac", "permission": "admin"}},
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/GetPeerApprovalState",
			"permission": "read",
			"resources":  []interface{}{map[string]interface{}{"index": 0, "field": "Mac", "permission": "read"}},
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/GetPeerPublicKey",
			"permission": "read",
			"resources":  []interface{}{map[string]interface{}{"index": 0, "field": "Mac", "permission": "read"}},
		},

		// --- Packages (descriptors & bundles) -----------------------------------
		map[string]interface{}{
			"action":     "/resource.ResourceService/FindPackages",
			"permission": "read",
			"resources":  []interface{}{},
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/GetPackageDescriptor",
			"permission": "read",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "ServiceId", "permission": "read"},
				map[string]interface{}{"index": 0, "field": "PublisherID", "permission": "read"},
			},
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/GetPackagesDescriptor",
			"permission": "read",
			"resources":  []interface{}{},
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/SetPackageDescriptor",
			"permission": "admin",
			"resources":  []interface{}{map[string]interface{}{"index": 0, "field": "PackageDescriptor.Id", "permission": "admin"}},
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/SetPackageBundle",
			"permission": "admin",
			"resources":  []interface{}{map[string]interface{}{"index": 0, "field": "Bundle.PackageDescriptor.Id", "permission": "admin"}},
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/GetPackageBundleChecksum",
			"permission": "read",
			"resources":  []interface{}{map[string]interface{}{"index": 0, "field": "Id", "permission": "read"}},
		},

		// --- Sessions ------------------------------------------------------------
		map[string]interface{}{
			"action":     "/resource.ResourceService/UpdateSession",
			"permission": "write",
			"resources":  []interface{}{map[string]interface{}{"index": 0, "field": "Session.AccountId", "permission": "write"}},
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/GetSessions",
			"permission": "read",
			"resources":  []interface{}{},
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/RemoveSession",
			"permission": "admin",
			"resources":  []interface{}{map[string]interface{}{"index": 0, "field": "AccountId", "permission": "admin"}},
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/GetSession",
			"permission": "read",
			"resources":  []interface{}{map[string]interface{}{"index": 0, "field": "AccountId", "permission": "read"}},
		},

		// --- Calls ---------------------------------------------------------------
		map[string]interface{}{
			"action":     "/resource.ResourceService/GetCallHistory",
			"permission": "read",
			"resources":  []interface{}{map[string]interface{}{"index": 0, "field": "AccountId", "permission": "read"}},
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/SetCall",
			"permission": "write",
			"resources":  []interface{}{}, // call spans two parties; gate action
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/DeleteCall",
			"permission": "write",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "AccountId", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "Uuid", "permission": "write"},
			},
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/ClearCalls",
			"permission": "admin",
			"resources":  []interface{}{map[string]interface{}{"index": 0, "field": "AccountId", "permission": "admin"}},
		},

		// --- Notifications -------------------------------------------------------
		map[string]interface{}{
			"action":     "/resource.ResourceService/CreateNotification",
			"permission": "write",
			"resources":  []interface{}{map[string]interface{}{"index": 0, "field": "Notification.Recipient", "permission": "write"}},
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/GetNotifications",
			"permission": "read",
			"resources":  []interface{}{map[string]interface{}{"index": 0, "field": "Recipient", "permission": "read"}},
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/DeleteNotification",
			"permission": "write",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "Recipient", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "Id", "permission": "write"},
			},
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/ClearAllNotifications",
			"permission": "write",
			"resources":  []interface{}{map[string]interface{}{"index": 0, "field": "Recipient", "permission": "write"}},
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/ClearNotificationsByType",
			"permission": "write",
			"resources":  []interface{}{map[string]interface{}{"index": 0, "field": "Recipient", "permission": "write"}},
		},
	}

	// ---- CLI flags handled BEFORE any call that might touch etcd ----
	args := os.Args[1:]
	if len(args) == 0 {
		s.Id = Utility.GenerateUUID(s.Name + ":" + s.Address)
		allocator, err := config.NewDefaultPortAllocator()
		if err != nil {
			fmt.Println("fail to create port allocator", "error", err)
			os.Exit(1)
		}
		p, err := allocator.Next(s.Id)
		if err != nil {
			fmt.Println("fail to allocate port", "error", err)
			os.Exit(1)
		}
		s.Port = p
	}

	for _, a := range args {
		switch strings.ToLower(a) {
		case "--describe":
			// Only compute ephemeral data here; avoid etcd
			s.Process = os.Getpid()
			s.State = "starting"
			if v, ok := os.LookupEnv("GLOBULAR_DOMAIN"); ok && v != "" {
				s.Domain = strings.ToLower(v)
			} else {
				s.Domain = "localhost"
			}
			if v, ok := os.LookupEnv("GLOBULAR_ADDRESS"); ok && v != "" {
				s.Address = strings.ToLower(v)
			} else {
				s.Address = "localhost:" + Utility.ToString(s.Port)
			}
			b, err := globular.DescribeJSON(s)
			if err != nil {
				logger.Error("describe error", "service", s.Name, "id", s.Id, "err", err)
				os.Exit(2)
			}
			_, _ = os.Stdout.Write(b)
			_, _ = os.Stdout.Write([]byte("\n"))
			return
		case "--health":
			if s.Port == 0 || s.Name == "" {
				logger.Error("health error: uninitialized", "service", s.Name, "port", s.Port)
				os.Exit(2)
			}
			b, err := globular.HealthJSON(s, &globular.HealthOptions{Timeout: 1500 * time.Millisecond})
			if err != nil {
				logger.Error("health error", "service", s.Name, "id", s.Id, "err", err)
				os.Exit(2)
			}
			_, _ = os.Stdout.Write(b)
			_, _ = os.Stdout.Write([]byte("\n"))
			return
		case "--help", "-h", "/?":
			printUsage()
			return
		case "--version", "-v":
			fmt.Println(s.Version)
			return

		}
	}

	// Optional positional args (unchanged behavior)
	if len(args) == 1 && !strings.HasPrefix(args[0], "-") {
		s.Id = args[0]
	} else if len(args) == 2 && !strings.HasPrefix(args[0], "-") && !strings.HasPrefix(args[1], "-") {
		s.Id = args[0]
		s.ConfigPath = args[1]
	}

	// Now it’s safe to read local config (may try etcd or file fallback)
	if d, err := config.GetDomain(); err == nil {
		s.Domain = d
	} else {
		s.Domain = "localhost"
	}
	if a, err := config.GetAddress(); err == nil && strings.TrimSpace(a) != "" {
		s.Address = a
	}

	// Backend informations.
	s.Backend_type = "SQL" // use SQL as default backend.
	s.Backend_port = 27018 // Here I will use the port beside the default one in case MONGO is already exist

	// I will try to get process named "scylla" if not found I will use MongoDB as backend.
	if process, err := Utility.GetProcessIdsByName("scylla"); err == nil && process != nil {
		s.Backend_type = "SCYLLA"
		if s.TLS {
			s.Backend_port = 9142
		} else {
			s.Backend_port = 9042
		}

		logger.Info("Scylla process detected, using Scylla as backend store", "process", process)
	}

	s.Backend_address = s.Address
	s.Backend_replication_factor = 1

	s.Backend_user = "sa"
	s.Backend_password = "adminadmin"
	s.DataPath = config.GetDataDir()

	start := time.Now()
	if err := s.Init(); err != nil {
		logger.Error("service init failed", "service", s.Name, "id", s.Id, "err", err)
		os.Exit(1)
	}

	// Register service implementation handlers (implemented elsewhere in this package)
	resourcepb.RegisterResourceServiceServer(s.grpcServer, s)
	reflection.Register(s.grpcServer)

	logger.Info("service ready",
		"service", s.Name,
		"port", s.Port,
		"proxy", s.Proxy,
		"protocol", s.Protocol,
		"domain", s.Domain,
		"listen_ms", time.Since(start).Milliseconds(),
		"pid", s.Process,
		"id", s.Id,
		"version", s.Version,
		"store", s.Backend_type,
	)

	if err := s.StartService(); err != nil {
		logger.Error("service start failed", "service", s.Name, "id", s.Id, "err", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  resource_server [service_id] [config_path]")
	fmt.Println("Options:")
	fmt.Println("  --describe    Print service metadata as JSON and exit")
	fmt.Println("  --health      Print service health as JSON and exit")
	fmt.Println("Examples:")
	fmt.Println("  resource_server my-id /etc/globular/resource/config.json")
	fmt.Println("  resource_server --describe")
	fmt.Println("  resource_server --health")
}
