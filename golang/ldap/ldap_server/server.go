// Package main implements the LDAP gRPC service used by Globular.
// This file contains the service struct, configuration getters/setters,
// Globular lifecycle (Init/Save/Start/Stop), resource-service helpers,
// and the main() entrypoint.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/globular_client"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/ldap/ldap_client"
	"github.com/globulario/services/golang/ldap/ldappb"
	"github.com/globulario/services/golang/resource/resource_client"
	"github.com/globulario/services/golang/resource/resourcepb"
	Utility "github.com/globulario/utility"
	"github.com/go-ldap/ldap/v3"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/reflection"
	"google.golang.org/grpc/status"
)

// -------------------- Defaults & CORS --------------------
var (
	defaultPort       = 10031
	defaultProxy      = 10032
	allowAllOrigins   = true
	allowedOriginsStr = ""
)

// STDERR logger so --describe/--health JSON stays clean on STDOUT.
var logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

// ldapConn is a tiny wrapper so server.go doesn't need to import go-ldap.
type ldapConn struct{ *ldap.Conn }

// Keep connection information here.
type connection struct {
	Id       string // connection id
	Host     string // hostname or IPv4
	User     string
	Password string
	Port     int32
	conn     *ldapConn // defined in ldap.go
}

// server implements the Globular service plus LDAP wiring.
type server struct {
	// Global attributes of the service.
	Id                   string
	Mac                  string
	Name                 string
	Path                 string
	Proto                string
	Port                 int
	Proxy                int
	Protocol             string
	AllowAllOrigins      bool
	AllowedOrigins       string // comma-separated
	Domain               string
	Address              string
	Description          string
	Keywords             []string
	Repositories         []string
	Discoveries          []string
	CertAuthorityTrust   string
	CertFile             string
	KeyFile              string
	Version              string
	TLS                  bool
	PublisherID          string
	KeepUpToDate         bool
	Plaform              string
	Checksum             string
	KeepAlive            bool
	Permissions          []interface{} // RBAC action permissions
	Dependencies         []string      // required services
	Process              int
	ProxyProcess         int
	ConfigPath           string
	LastError            string
	ModTime              int64
	State                string
	DynamicMethodRouting []interface{}

	// gRPC server.
	grpcServer *grpc.Server

	// LDAP runtime
	Connections   map[string]connection
	LdapSyncInfos map[string]interface{}
}

func (srv *server) startLdapServer() {
	panic("unimplemented")
}

// -------------------- Getters/Setters (public API kept intact) --------------------

func (srv *server) GetAddress() string                    { return srv.Address }
func (srv *server) SetAddress(address string)             { srv.Address = address }
func (srv *server) GetProcess() int                       { return srv.Process }
func (srv *server) SetProcess(pid int)                    { srv.Process = pid }
func (srv *server) GetProxyProcess() int                  { return srv.ProxyProcess }
func (srv *server) SetProxyProcess(pid int)               { srv.ProxyProcess = pid }
func (srv *server) GetState() string                      { return srv.State }
func (srv *server) SetState(state string)                 { srv.State = state }
func (srv *server) GetLastError() string                  { return srv.LastError }
func (srv *server) SetLastError(err string)               { srv.LastError = err }
func (srv *server) SetModTime(modtime int64)              { srv.ModTime = modtime }
func (srv *server) GetModTime() int64                     { return srv.ModTime }
func (srv *server) GetId() string                         { return srv.Id }
func (srv *server) SetId(id string)                       { srv.Id = id }
func (srv *server) GetName() string                       { return srv.Name }
func (srv *server) SetName(name string)                   { srv.Name = name }
func (srv *server) GetDescription() string                { return srv.Description }
func (srv *server) SetDescription(description string)     { srv.Description = description }
func (srv *server) GetMac() string                        { return srv.Mac }
func (srv *server) SetMac(mac string)                     { srv.Mac = mac }
func (srv *server) GetKeywords() []string                 { return srv.Keywords }
func (srv *server) SetKeywords(keywords []string)         { srv.Keywords = keywords }
func (srv *server) GetRepositories() []string             { return srv.Repositories }
func (srv *server) SetRepositories(repositories []string) { srv.Repositories = repositories }
func (srv *server) GetDiscoveries() []string              { return srv.Discoveries }
func (srv *server) SetDiscoveries(discoveries []string)   { srv.Discoveries = discoveries }
func (srv *server) Dist(path string) (string, error)      { return globular.Dist(path, srv) }
func (srv *server) GetDependencies() []string {
	if srv.Dependencies == nil {
		srv.Dependencies = []string{}
	}
	return srv.Dependencies
}
func (srv *server) SetDependency(dependency string) {
	if !Utility.Contains(srv.GetDependencies(), dependency) {
		srv.Dependencies = append(srv.Dependencies, dependency)
	}
}
func (srv *server) GetChecksum() string                      { return srv.Checksum }
func (srv *server) SetChecksum(checksum string)              { srv.Checksum = checksum }
func (srv *server) GetPlatform() string                      { return srv.Plaform }
func (srv *server) SetPlatform(platform string)              { srv.Plaform = platform }
func (srv *server) GetPath() string                          { return srv.Path }
func (srv *server) SetPath(path string)                      { srv.Path = path }
func (srv *server) GetProto() string                         { return srv.Proto }
func (srv *server) SetProto(proto string)                    { srv.Proto = proto }
func (srv *server) GetPort() int                             { return srv.Port }
func (srv *server) SetPort(port int)                         { srv.Port = port }
func (srv *server) GetProxy() int                            { return srv.Proxy }
func (srv *server) SetProxy(proxy int)                       { srv.Proxy = proxy }
func (srv *server) GetProtocol() string                      { return srv.Protocol }
func (srv *server) SetProtocol(protocol string)              { srv.Protocol = protocol }
func (srv *server) GetAllowAllOrigins() bool                 { return srv.AllowAllOrigins }
func (srv *server) SetAllowAllOrigins(allowAllOrigins bool)  { srv.AllowAllOrigins = allowAllOrigins }
func (srv *server) GetAllowedOrigins() string                { return srv.AllowedOrigins }
func (srv *server) SetAllowedOrigins(allowedOrigins string)  { srv.AllowedOrigins = allowedOrigins }
func (srv *server) GetDomain() string                        { return srv.Domain }
func (srv *server) SetDomain(domain string)                  { srv.Domain = domain }
func (srv *server) GetTls() bool                             { return srv.TLS }
func (srv *server) SetTls(hasTls bool)                       { srv.TLS = hasTls }
func (srv *server) GetCertAuthorityTrust() string            { return srv.CertAuthorityTrust }
func (srv *server) SetCertAuthorityTrust(ca string)          { srv.CertAuthorityTrust = ca }
func (srv *server) GetCertFile() string                      { return srv.CertFile }
func (srv *server) SetCertFile(certFile string)              { srv.CertFile = certFile }
func (srv *server) GetKeyFile() string                       { return srv.KeyFile }
func (srv *server) SetKeyFile(keyFile string)                { srv.KeyFile = keyFile }
func (srv *server) GetVersion() string                       { return srv.Version }
func (srv *server) SetVersion(version string)                { srv.Version = version }
func (srv *server) GetPublisherID() string                   { return srv.PublisherID }
func (srv *server) SetPublisherID(PublisherID string)        { srv.PublisherID = PublisherID }
func (srv *server) GetKeepUpToDate() bool                    { return srv.KeepUpToDate }
func (srv *server) SetKeepUptoDate(val bool)                 { srv.KeepUpToDate = val }
func (srv *server) GetKeepAlive() bool                       { return srv.KeepAlive }
func (srv *server) SetKeepAlive(val bool)                    { srv.KeepAlive = val }
func (srv *server) GetPermissions() []interface{}            { return srv.Permissions }
func (srv *server) SetPermissions(permissions []interface{}) { srv.Permissions = permissions }

func (srv *server) RolesDefault() []resourcepb.Role {
	domain, _ := config.GetDomain()

	return []resourcepb.Role{
		{
			Id:          "role:ldap.viewer",
			Name:        "LDAP Viewer",
			Domain:      domain,
			Description: "Read-only access to LDAP search and sync configuration.",
			Actions: []string{
				"/ldap.LdapService/Search",
				"/ldap.LdapService/getLdapSyncInfo",
			},
			TypeName: "resource.Role",
		},
		{
			Id:          "role:ldap.authenticator",
			Name:        "LDAP Authenticator",
			Domain:      domain,
			Description: "Can perform LDAP Authenticate (e.g., login flows).",
			Actions: []string{
				"/ldap.LdapService/Authenticate",
			},
			TypeName: "resource.Role",
		},
		{
			Id:          "role:ldap.connector.admin",
			Name:        "LDAP Connector Admin",
			Domain:      domain,
			Description: "Manage LDAP connections (create/delete/close).",
			Actions: []string{
				"/ldap.LdapService/CreateConnection",
				"/ldap.LdapService/DeleteConnection",
				"/ldap.LdapService/Close",
			},
			TypeName: "resource.Role",
		},
		{
			Id:          "role:ldap.sync.admin",
			Name:        "LDAP Sync Admin",
			Domain:      domain,
			Description: "Manage sync configuration and run synchronization.",
			Actions: []string{
				"/ldap.LdapService/setLdapSyncInfo",
				"/ldap.LdapService/deleteLdapSyncInfo",
				"/ldap.LdapService/getLdapSyncInfo",
				"/ldap.LdapService/Synchronize",
			},
			TypeName: "resource.Role",
		},
		{
			Id:          "role:ldap.admin",
			Name:        "LDAP Service Admin",
			Domain:      domain,
			Description: "Full control over LDAP service, including stop.",
			Actions: []string{
				"/ldap.LdapService/Stop",
				// connection admin
				"/ldap.LdapService/CreateConnection",
				"/ldap.LdapService/DeleteConnection",
				"/ldap.LdapService/Close",
				// sync admin
				"/ldap.LdapService/setLdapSyncInfo",
				"/ldap.LdapService/deleteLdapSyncInfo",
				"/ldap.LdapService/getLdapSyncInfo",
				"/ldap.LdapService/Synchronize",
				// read & exec
				"/ldap.LdapService/Search",
				"/ldap.LdapService/Authenticate",
			},
			TypeName: "resource.Role",
		},
	}
}

// Init creates/loads configuration and initializes the gRPC server.
func (srv *server) Init() error {
	if err := globular.InitService(srv); err != nil {
		return err
	}
	grpcSrv, err := globular.InitGrpcServer(srv)
	if err != nil {
		return err
	}
	srv.grpcServer = grpcSrv
	return nil
}

// Save persists service configuration to disk.
func (srv *server) Save() error { return globular.SaveService(srv) }

// StartService begins serving gRPC.
func (srv *server) StartService() error { return globular.StartService(srv, srv.grpcServer) }

// StopService gracefully stops gRPC serving.
func (srv *server) StopService() error { return globular.StopService(srv, srv.grpcServer) }

// Stop gRPC via API call.
func (srv *server) Stop(ctx context.Context, _ *ldappb.StopRequest) (*ldappb.StopResponse, error) {
	return &ldappb.StopResponse{}, srv.StopService()
}

/** Connect to an LDAP server. */
func (srv *server) connect(id string, userId string, pwd string) (*ldapConn, error) {
	info := srv.Connections[id]
	conn, err := ldap.Dial("tcp", fmt.Sprintf("%s:%d", info.Host, info.Port))
	if err != nil {
		return nil, err
	}
	conn.SetTimeout(3 * time.Second)
	if len(userId) > 0 {
		if len(pwd) > 0 {
			err = conn.Bind(userId, pwd)
		} else {
			err = conn.UnauthenticatedBind(userId)
		}
		if err != nil {
			return nil, err
		}
	} else {
		if len(info.Password) > 0 {
			err = conn.Bind(info.User, info.Password)
		} else {
			err = conn.UnauthenticatedBind(info.User)
		}
		if err != nil {
			return nil, err
		}
	}
	return &ldapConn{conn}, nil
}

// --- Resource service helpers (used by sync) ---

func (srv *server) getResourceClient() (*resource_client.Resource_Client, error) {
	Utility.RegisterFunction("NewResourceService_Client", resource_client.NewResourceService_Client)
	client, err := globular_client.GetClient(srv.Address, "resource.ResourceService", "NewResourceService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*resource_client.Resource_Client), nil
}

func (srv *server) createGroup(token, id, name, description string) error {
	rc, err := srv.getResourceClient()
	if err != nil {
		return err
	}
	return rc.CreateGroup(token, id, name, description)
}

func (srv *server) registerAccount(domain, id, name, email, password string) error {
	rc, err := srv.getResourceClient()
	if err != nil {
		return err
	}
	return rc.RegisterAccount(domain, id, name, email, password, password)
}

func (srv *server) addGroupMemberAccount(token, groupId, accountId string) error {
	rc, err := srv.getResourceClient()
	if err != nil {
		return err
	}
	return rc.AddGroupMemberAccount(token, groupId, accountId)
}

func (srv *server) removeGroupMemberAccount(token, groupId, accountId string) error {
	rc, err := srv.getResourceClient()
	if err != nil {
		return err
	}
	return rc.RemoveGroupMemberAccount(token, groupId, accountId)
}

func (srv *server) getAccount(id string) (*resourcepb.Account, error) {
	rc, err := srv.getResourceClient()
	if err != nil {
		return nil, err
	}
	return rc.GetAccount(id)
}

func (srv *server) getGroup(id string) (*resourcepb.Group, error) {
	rc, err := srv.getResourceClient()
	if err != nil {
		return nil, err
	}
	groups, err := rc.GetGroups(`{"_id":"` + id + `"}`)
	if len(groups) > 0 {
		return groups[0], nil
	}
	if err != nil {
		return nil, err
	}
	return nil, status.Errorf(codes.NotFound, "no group found with id %s", id)
}

// -------------------- CLI helpers --------------------

func printUsage() {
	fmt.Fprintf(os.Stdout, `
Usage: %s [options] <id> [configPath]

Options:
  --describe   Print service description as JSON (no etcd/config access)
  --health     Print service health as JSON (no etcd/config access)

`, filepath.Base(os.Args[0]))
}

// --- main entrypoint ---

func main() {
	// Base skeleton (no etcd/config yet)
	s := &server{
		Connections:     make(map[string]connection),
		Name:            string(ldappb.File_ldap_proto.Services().Get(0).FullName()),
		Proto:           ldappb.File_ldap_proto.Path(),
		Path:            "",
		Port:            defaultPort,
		Proxy:           defaultProxy,
		Protocol:        "grpc",
		Version:         "0.0.1",
		AllowAllOrigins: allowAllOrigins,
		AllowedOrigins:  allowedOriginsStr,
		PublisherID:     "localhost",
		Permissions:     []interface{}{}, // keep empty; no nil entries
		Keywords:        []string{"LDAP", "Directory"},
		Repositories:    []string{},
		Discoveries:     []string{},
		Dependencies:    []string{},
		Process:         -1,
		ProxyProcess:    -1,
		KeepAlive:       true,
		KeepUpToDate:    true,
	}

	// s.Permissions for ldap.LdapService
	s.Permissions = []interface{}{
		// ---- Control plane
		map[string]interface{}{
			"action":     "/ldap.LdapService/Stop",
			"permission": "admin",
			"resources":  []interface{}{},
		},

		// ---- Connection lifecycle (sensitive; changes runtime + persisted config)
		map[string]interface{}{
			"action":     "/ldap.LdapService/CreateConnection",
			"permission": "admin",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "Connection.Id", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "Connection.Host", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "Connection.Port", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "Connection.User", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "Connection.Password", "permission": "write"}, // secret; admin gate
			},
		},
		map[string]interface{}{
			"action":     "/ldap.LdapService/DeleteConnection",
			"permission": "admin",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "Id", "permission": "delete"},
			},
		},
		map[string]interface{}{
			"action":     "/ldap.LdapService/Close",
			"permission": "admin", // closing shared connection affects others
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "Id", "permission": "write"},
			},
		},

		// ---- Directory search (read-only)
		map[string]interface{}{
			"action":     "/ldap.LdapService/Search",
			"permission": "read",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "Search.Id", "permission": "read"},
				map[string]interface{}{"index": 0, "field": "Search.BaseDN", "permission": "read"},
				map[string]interface{}{"index": 0, "field": "Search.Filter", "permission": "read"},
				map[string]interface{}{"index": 0, "field": "Search.Attributes", "permission": "read"},
			},
		},

		// ---- Authentication (exec against directory; does not mutate service state)
		map[string]interface{}{
			"action":     "/ldap.LdapService/Authenticate",
			"permission": "exec",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "Id", "permission": "read"},
				map[string]interface{}{"index": 0, "field": "Login", "permission": "read"},
				map[string]interface{}{"index": 0, "field": "Pwd", "permission": "read"}, // sensitive; check transport/TLS upstream
			},
		},

		// ---- Sync configuration CRUD
		map[string]interface{}{
			"action":     "/ldap.LdapService/setLdapSyncInfo",
			"permission": "admin",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "Info.Id", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "Info.ConnectionId", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "Info.Refresh", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "Info.GroupSyncInfo.Id", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "Info.GroupSyncInfo.Base", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "Info.GroupSyncInfo.Query", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "Info.UserSyncInfo.Id", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "Info.UserSyncInfo.Email", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "Info.UserSyncInfo.Base", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "Info.UserSyncInfo.Query", "permission": "write"},
			},
		},
		map[string]interface{}{
			"action":     "/ldap.LdapService/deleteLdapSyncInfo",
			"permission": "admin",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "Id", "permission": "delete"},
			},
		},
		map[string]interface{}{
			"action":     "/ldap.LdapService/getLdapSyncInfo",
			"permission": "read",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "Id", "permission": "read"},
			},
		},

		// ---- Synchronize (executes external effects: creates/updates accounts & groups via Resource/RBAC)
		map[string]interface{}{
			"action":     "/ldap.LdapService/Synchronize",
			"permission": "admin",
			"resources":  []interface{}{}, // uses stored config; no fields on request
		},
	}

	// Register LDAP client factory for other services.
	Utility.RegisterFunction("NewLdapService_Client", ldap_client.NewLdapService_Client)

	// Resolve binary path.
	if p, err := filepath.Abs(filepath.Dir(os.Args[0])); err == nil {
		s.Path = p
	}

	// ---------- CLI BEFORE config ----------
	args := os.Args[1:]
	if len(args) == 0 {
		printUsage()
		return
	}
	for _, a := range args {
		switch strings.ToLower(a) {
		case "--describe":
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
			os.Stdout.Write(b)
			os.Stdout.Write([]byte("\n"))
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
			os.Stdout.Write(b)
			os.Stdout.Write([]byte("\n"))
			return
		}
	}

	// Optional positional args: <id> [configPath]
	if len(args) == 1 && !strings.HasPrefix(args[0], "-") {
		s.Id = args[0]
	} else if len(args) == 2 && !strings.HasPrefix(args[0], "-") && !strings.HasPrefix(args[1], "-") {
		s.Id = args[0]
		s.ConfigPath = args[1]
	}

	// ---------- Safe to touch config ----------
	if d, err := config.GetDomain(); err == nil {
		s.Domain = d
	} else {
		s.Domain = "localhost"
	}
	if a, err := config.GetAddress(); err == nil && strings.TrimSpace(a) != "" {
		s.Address = a
	}

	// Initialize service & gRPC.
	start := time.Now()
	if err := s.Init(); err != nil {
		logger.Error("failed to initialize service", "name", s.Name, "id", s.Id, "err", err)
		os.Exit(1)
	}
	if s.Address == "" {
		if addr, _ := config.GetAddress(); addr != "" {
			s.Address = addr
		}
	}

	// Register gRPC server implementation.
	ldappb.RegisterLdapServiceServer(s.grpcServer, s)
	reflection.Register(s.grpcServer)

	// Start embedded LDAP listener (389/636) in background.
	//go s.startLdapServer()

	logger.Info("service ready",
		"service", s.Name,
		"port", s.Port,
		"proxy", s.Proxy,
		"protocol", s.Protocol,
		"domain", s.Domain,
		"listen_ms", time.Since(start).Milliseconds())

	// Start gRPC service (blocking).
	if err := s.StartService(); err != nil {
		logger.Error("gRPC service stopped with error", "err", err)
	}
}
