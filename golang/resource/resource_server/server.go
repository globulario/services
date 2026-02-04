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
	"encoding/json"
	"errors"
	"fmt"
	"net"
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
	"github.com/globulario/services/golang/resource/resource_server/internal/security"
	"github.com/globulario/services/golang/resource/resourcepb"
)

// -----------------------------------------------------------------------------
// Defaults & CORS
// -----------------------------------------------------------------------------

var (
	defaultPort  = 10010
	defaultProxy = defaultPort + 1

	allowAllOrigins   = true
	allowedOriginsStr = ""
)

const (
	// nodeIdentityCollection stores node identities; the table name is kept for compatibility with the legacy Peers collection.
	nodeIdentityCollection = "Peers"
)

// -----------------------------------------------------------------------------
// Server type (unchanged fields preserved, ordering cleaned)
// -----------------------------------------------------------------------------

type server struct {
	resourcepb.UnimplementedResourceServiceServer
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

	// Bootstrap mode: when true, RBAC unavailability is non-fatal
	bootstrapMode bool

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

// -----------------------------------------------------------------------------
// Scylla Host Resolution
// -----------------------------------------------------------------------------

// tcpProbe attempts to dial a TCP address and returns true if successful.
func tcpProbe(host string, port int, timeout time.Duration) bool {
	addr := fmt.Sprintf("%s:%d", host, port)
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}

// detectPrimaryIP returns the primary non-loopback IP of this node.
func detectPrimaryIP() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	for _, iface := range ifaces {
		// Skip loopback and down interfaces
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}

			// Return first non-loopback IPv4
			if ip != nil && ip.To4() != nil && !ip.IsLoopback() {
				return ip.String(), nil
			}
		}
	}

	return "", fmt.Errorf("no primary IP found")
}

// resolveScyllaHost determines the Scylla host to connect to.
// Priority: env var > probe 127.0.0.1 > probe node primary IP
func resolveScyllaHost(port int) (string, error) {
	// 1. Explicit env override
	if v := strings.TrimSpace(os.Getenv("GLOBULAR_SCYLLA_HOST")); v != "" {
		logger.Info("scylla host from env", "host", v)
		return v, nil
	}

	// 2. Try loopback
	if tcpProbe("127.0.0.1", port, 500*time.Millisecond) {
		logger.Info("scylla host detected", "host", "127.0.0.1", "method", "tcp-probe")
		return "127.0.0.1", nil
	}

	// 3. Fallback to node primary IP
	ip, err := detectPrimaryIP()
	if err == nil && tcpProbe(ip, port, 500*time.Millisecond) {
		logger.Info("scylla host detected", "host", ip, "method", "tcp-probe")
		return ip, nil
	}

	return "", fmt.Errorf("no reachable scylla host found (tried: 127.0.0.1, %s)", ip)
}

// retryWithBackoff retries fn with exponential backoff until timeout.
// Backoff: 1s, 2s, 4s, 8s, 16s, 32s, ... (capped at 32s between attempts)
func retryWithBackoff(timeout time.Duration, fn func() error) error {
	deadline := time.Now().Add(timeout)
	backoff := 1 * time.Second
	attempt := 0

	for {
		attempt++
		err := fn()
		if err == nil {
			if attempt > 1 {
				logger.Info("operation succeeded after retries", "attempts", attempt)
			}
			return nil
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("operation failed after %d attempts over %s: %w", attempt, timeout, err)
		}

		logger.Warn("operation failed, retrying",
			"attempt", attempt,
			"next_retry_in", backoff.String(),
			"error", err)

		time.Sleep(backoff)

		// Exponential backoff with 32s cap
		backoff *= 2
		if backoff > 32*time.Second {
			backoff = 32 * time.Second
		}
	}
}

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
			Description: "Read-only access to accounts, roles, groups, orgs, apps, node identities, packages, sessions, calls, and notifications.",
			Actions: []string{
				// Orgs / Groups / Roles / Apps (read)
				"/resource.ResourceService/GetOrganizations",
				"/resource.ResourceService/GetGroups",
				"/resource.ResourceService/GetRoles",
				"/resource.ResourceService/GetApplications",
				// Accounts (read)
				"/resource.ResourceService/GetAccount",
				"/resource.ResourceService/GetAccounts",
				// Node identities (read)
				"/resource.ResourceService/GetNodeIdentity",
				"/resource.ResourceService/ListNodeIdentities",
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
				"/resource.ResourceService/GetNodeIdentity",
				"/resource.ResourceService/ListNodeIdentities",
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
				"/resource.ResourceService/UpsertNodeIdentity",
				"/resource.ResourceService/SetNodeIdentityEnabled",
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
func (srv *server) getRbacClient() (*rbac_client.Rbac_Client, error) {
	Utility.RegisterFunction("NewRbacService_Client", rbac_client.NewRbacService_Client)
	client, err := srv.dialRbacWithRetry()
	if err != nil {
		if srv.bootstrapMode {
			logger.Warn("rbac client unavailable in bootstrap mode", "error", err)
			return nil, nil
		}
		return nil, err
	}
	if client == nil {
		return nil, nil
	}
	return client, nil
}

func (srv *server) addResourceOwner(token, path, subject, resourceType string, subjectType rbacpb.SubjectType) error {
	rbac_client_, err := srv.getRbacClient()
	if err != nil {
		return err
	}

	// If RBAC client is nil (bootstrap mode), skip RBAC operations gracefully
	if rbac_client_ == nil {
		logger.Warn("rbac disabled; skipping owner assignment",
			"path", path,
			"subject", subject,
			"resourceType", resourceType,
		)
		return nil
	}

	err = rbac_client_.AddResourceOwner(token, path, subject, resourceType, subjectType)
	if err != nil {
		// In bootstrap mode with bootstrap token, don't fail on RBAC errors
		if srv.bootstrapMode && token == "internal-bootstrap" {
			logger.Warn("rbac owner assignment failed in bootstrap mode",
				"error", err,
				"path", path,
			)
			return nil
		}
		return err
	}
	return nil
}

func (srv *server) deleteResourcePermissions(token, path string) error {
	rbac_client_, err := srv.getRbacClient()
	if err != nil {
		return err
	}

	// If RBAC client is nil (bootstrap mode), skip RBAC operations gracefully
	if rbac_client_ == nil {
		logger.Warn("rbac disabled; skipping delete resource permissions", "path", path)
		return nil
	}

	err = rbac_client_.DeleteResourcePermissions(token, path)
	return err
}

func (srv *server) deleteAllAccess(token, subject string, subjectType rbacpb.SubjectType) error {
	rbac_client_, err := srv.getRbacClient()
	if err != nil {
		return err
	}

	// If RBAC client is nil (bootstrap mode), skip RBAC operations gracefully
	if rbac_client_ == nil {
		logger.Warn("rbac disabled; skipping delete all access", "subject", subject)
		return nil
	}

	return rbac_client_.DeleteAllAccess(token, subject, subjectType)
}

func (srv *server) SetAccountAllocatedSpace(token, accountId string, space uint64) error {
	rbac_client_, err := srv.getRbacClient()
	if err != nil {
		return err
	}

	// If RBAC client is nil (bootstrap mode), skip RBAC operations gracefully
	if rbac_client_ == nil {
		logger.Warn("rbac disabled; skipping set account allocated space", "accountId", accountId)
		return nil
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

		// Connect to the store with retry logic (90 seconds total)
		err := retryWithBackoff(90*time.Second, func() error {
			if connErr := srv.store.Connect("local_resource", srv.Backend_address, int32(srv.Backend_port), srv.Backend_user, srv.Backend_password, "local_resource", 5000, options_str); connErr != nil {
				return fmt.Errorf("connect to %s:%d: %w", srv.Backend_address, srv.Backend_port, connErr)
			}
			if pingErr := srv.store.Ping(context.Background(), "local_resource"); pingErr != nil {
				return fmt.Errorf("ping %s:%d: %w", srv.Backend_address, srv.Backend_port, pingErr)
			}
			return nil
		})

		if err != nil {
			logger.Error("scylla unavailable after retries", "error", err)
			return nil, err // Let systemd handle restart
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
				[]string{"name TEXT", "domain TEXT", "icon TEXT", "description TEXT"})
			if err != nil {
				logger.Error("fail to create table Groups", "error", err)
			}

			// Create peers table.
			err = srv.store.(*persistence_store.SqlStore).CreateTable(
				context.Background(), "local_resource", "local_resource", nodeIdentityCollection,
				[]string{"domain TEXT", "hostname TEXT", "external_ip_address TEXT", "local_ip_address TEXT", "mac TEXT", "protocol TEXT", "state INTEGER", "PortHTTP INTEGER", "PortHTTPS INTEGER"})
			if err != nil {
				logger.Error("fail to create table Peers", "error", err)
			}

			// (Duplicate Peers creation kept as in your original)
			err = srv.store.(*persistence_store.SqlStore).CreateTable(
				context.Background(), "local_resource", "local_resource", nodeIdentityCollection,
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
				[]string{"name TEXT", "domain TEXT", "icon TEXT", "description TEXT"})
			if err != nil {
				logger.Error("fail to create table Groups", "error", err)
			}

			// Create peers table.
			err = srv.store.(*persistence_store.ScyllaStore).CreateTable(
				context.Background(), "local_resource", "local_resource", nodeIdentityCollection,
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

			// run admin script.
			script := `ALTER KEYSPACE local_resource WITH REPLICATION = {'class':'SimpleStrategy','replication_factor':` + Utility.ToString(srv.Backend_replication_factor) + `}`
			err = srv.store.(*persistence_store.ScyllaStore).RunAdminCmd(
				context.Background(), "local_resource", srv.Backend_user, srv.Backend_password, script)
			if err != nil {
				logger.Error("fail to run admin script", "script", script, "error", err)
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

	// Check for bootstrap mode via environment variable
	// In bootstrap mode, RBAC unavailability is non-fatal
	s.bootstrapMode = os.Getenv("GLOBULAR_BOOTSTRAP") == "1"
	if s.bootstrapMode {
		logger.Info("running in bootstrap mode - RBAC failures will be non-fatal")
	}

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

		// --- Node Identities ---------------------------------------------------
		map[string]interface{}{
			"action":     "/resource.ResourceService/ListNodeIdentities",
			"permission": "read",
			"resources":  []interface{}{},
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/GetNodeIdentity",
			"permission": "read",
			"resources":  []interface{}{map[string]interface{}{"index": 0, "field": "NodeId", "permission": "read"}},
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/UpsertNodeIdentity",
			"permission": "admin",
			"resources":  []interface{}{map[string]interface{}{"index": 0, "field": "Node.NodeId", "permission": "admin"}},
		},
		map[string]interface{}{
			"action":     "/resource.ResourceService/SetNodeIdentityEnabled",
			"permission": "admin",
			"resources":  []interface{}{map[string]interface{}{"index": 0, "field": "NodeId", "permission": "admin"}},
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
			if s.Id == "" {
				s.Id = Utility.GenerateUUID(s.Name + ":" + s.Address)
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
		s.Address = strings.TrimSpace(a)
	}
	if strings.TrimSpace(s.Address) == "" {
		s.Address = "localhost:" + Utility.ToString(s.Port)
	}

	// Backend informations.
	scyllaDetected := false
	if process, err := Utility.GetProcessIdsByName("scylla"); err == nil && process != nil {
		scyllaDetected = true
		logger.Info("Scylla process detected, using Scylla as backend store", "process", process)
	}
	computeBackendConfig(s, scyllaDetected)
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

	// Use internal bootstrap context for service startup operations
	// This provides a valid identity without requiring external authentication
	err := s.CreateAccountDir(security.BootstrapContext())
	if err != nil {
		logger.Error("fail to create account dir", "error", err)
	}

	// Resolve RBAC endpoint before starting service (for error classification)
	rbacEndpoint, _ := resolveRbacEndpoint(s.bootstrapMode)

	if err := s.StartService(); err != nil {
		if s.bootstrapMode && isLikelyRbacError(err, rbacEndpoint) {
			logger.Warn("StartService failed due to RBAC in bootstrap mode; continuing",
				"service", s.Name, "id", s.Id, "err", err, "rbac_endpoint", rbacEndpoint)
		} else {
			logger.Error("service start failed", "service", s.Name, "id", s.Id, "err", err)
			os.Exit(1)
		}
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

// computeBackendConfig sets backend type, ports, and address safely.
func computeBackendConfig(s *server, scyllaDetected bool) {
	// Ensure address is populated.
	if strings.TrimSpace(s.Address) == "" {
		s.Address = "localhost:" + Utility.ToString(s.Port)
	}

	s.Backend_type = "SQL" // default backend
	s.Backend_port = 27018 // alternative to default Mongo port

	if scyllaDetected {
		s.Backend_type = "SCYLLA"
		if s.TLS {
			s.Backend_port = 9142
		} else {
			s.Backend_port = 9042
		}
	}

	// Backend host: resolve using env > tcp probe > node IP (NOT service listen address)
	resolvedHost, err := resolveScyllaHost(int(s.Backend_port))
	if err != nil {
		logger.Warn("scylla host resolution failed, will retry during connect", "error", err)
		// Fallback to localhost as last resort - connection will be retried
		s.Backend_address = "localhost"
	} else {
		s.Backend_address = resolvedHost
	}
}

// resolveRbacEndpoint finds the rbac service address from service configs.
func resolveRbacEndpoint(bootstrap bool) (string, error) {
	servicesDir := strings.TrimSpace(os.Getenv("GLOBULAR_SERVICES_DIR"))
	if servicesDir == "" {
		servicesDir = "/var/lib/globular/services"
	}
	entries, err := os.ReadDir(servicesDir)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(filepath.Join(servicesDir, e.Name()))
		if err != nil {
			continue
		}
		var cfg map[string]interface{}
		if err := json.Unmarshal(data, &cfg); err != nil {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(Utility.ToString(cfg["Name"])))
		if name != strings.ToLower("rbac.RbacService") && name != "rbac" && name != "globular-rbac" {
			continue
		}
		addr := strings.TrimSpace(Utility.ToString(cfg["Address"]))
		port := Utility.ToInt(cfg["Port"])
		if addr == "" && port > 0 {
			addr = fmt.Sprintf("127.0.0.1:%d", port)
		} else if port > 0 && !strings.Contains(addr, ":") {
			addr = fmt.Sprintf("%s:%d", addr, port)
		}
		if addr == "" {
			continue
		}
		return addr, nil
	}
	if bootstrap {
		return "", nil
	}
	return "", fmt.Errorf("rbac service config not found in %s", servicesDir)
}

// dialRbacWithRetry tries to create an RBAC client with retry budget.
func (srv *server) dialRbacWithRetry() (*rbac_client.Rbac_Client, error) {
	var lastErr error
	deadline := time.Now().Add(90 * time.Second)
	backoff := 2 * time.Second

	for {
		seedAddr, err := resolveRbacEndpoint(srv.bootstrapMode)
		if err != nil {
			lastErr = fmt.Errorf("rbac: endpoint resolution failed: %w", err)
		} else if seedAddr != "" {
			c, err := globular_client.GetClient(seedAddr, "rbac.RbacService", "NewRbacService_Client")
			if err == nil {
				return c.(*rbac_client.Rbac_Client), nil
			}
			lastErr = fmt.Errorf("rbac: %w", err)
		} else {
			lastErr = fmt.Errorf("rbac: endpoint unresolved")
		}

		if time.Now().After(deadline) {
			if srv.bootstrapMode {
				return nil, lastErr
			}
			return nil, lastErr
		}
		time.Sleep(backoff)
		if backoff < 10*time.Second {
			backoff *= 2
		}
	}
}

func isLikelyRbacError(err error, rbacEndpoint string) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())

	// Strong signals - directly RBAC-related
	for _, kw := range []string{
		"rbac",
		"rbac:",
		"newrbacservice_client",
		"permission",
		"access denied",
		"constructor returned error", // globular_client.GetClient path
	} {
		if strings.Contains(msg, kw) {
			return true
		}
	}

	// Common gRPC transport failures (often do NOT mention "rbac")
	for _, kw := range []string{
		"rpc error",
		"unavailable",
		"connection refused",
		"transport is closing",
		"deadline exceeded",
		"context deadline exceeded",
	} {
		if strings.Contains(msg, kw) {
			// If we can tie it to the rbac endpoint, treat as RBAC
			if rbacEndpoint == "" {
				// In bootstrap, safe to assume transport failure is RBAC-related
				return true
			}
			if strings.Contains(msg, strings.ToLower(rbacEndpoint)) {
				return true
			}
			// Also check if message contains "rbac" anywhere
			if strings.Contains(msg, "rbac") {
				return true
			}
		}
	}

	return false
}
