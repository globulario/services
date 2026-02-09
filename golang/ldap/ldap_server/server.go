// Package main implements the LDAP gRPC service used by Globular.
// This file contains the service struct, configuration getters/setters,
// Globular lifecycle (Init/Save/Start/Stop), resource-service helpers,
// and the main() entrypoint.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/globular_client"
	globular "github.com/globulario/services/golang/globular_service"
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

// Version information (set via ldflags during build)
var (
	Version   = "0.0.1"
	BuildTime = "unknown"
	GitCommit = "unknown"
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
	Logger               *slog.Logger

	// gRPC server.
	grpcServer *grpc.Server

	// LDAP runtime
	Connections   map[string]connection
	LdapSyncInfos map[string]interface{}

	LdapListenAddr  string // e.g. "127.0.0.1:10389" (empty -> "0.0.0.0:389")
	LdapsListenAddr string // e.g. "127.0.0.1:10636" (empty -> "0.0.0.0:636")
	DisableLDAPS    bool   // when true, skip starting LDAPS listener

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

func loadDefaultPermissions() []interface{} {
	return []interface{}{
		// ---- Control plane
		map[string]interface{}{
			"action":     "/ldap.LdapService/Stop",
			"permission": "admin",
			"resources":  []interface{}{},
		},

		// ---- Connection lifecycle
		map[string]interface{}{
			"action":     "/ldap.LdapService/CreateConnection",
			"permission": "admin",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "Connection.Id", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "Connection.Host", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "Connection.Port", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "Connection.User", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "Connection.Password", "permission": "write"},
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
			"permission": "admin",
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

		// ---- Authentication
		map[string]interface{}{
			"action":     "/ldap.LdapService/Authenticate",
			"permission": "exec",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "Id", "permission": "read"},
				map[string]interface{}{"index": 0, "field": "Login", "permission": "read"},
				map[string]interface{}{"index": 0, "field": "Pwd", "permission": "read"},
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

		// ---- Synchronize
		map[string]interface{}{
			"action":     "/ldap.LdapService/Synchronize",
			"permission": "admin",
			"resources":  []interface{}{},
		},
	}
}

// Init creates/loads configuration and initializes the gRPC server.
func (srv *server) Init() error {
	srv.ensureRuntimeState()
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
func (srv *server) StartService() error {
	srv.ensureRuntimeState()
	if err := srv.StartLDAPFacade(); err != nil {
		logger.Warn("failed to start LDAP facade", "err", err)
	}
	return globular.StartService(srv, srv.grpcServer)
}

// StopService gracefully stops gRPC serving.
func (srv *server) StopService() error          { return globular.StopService(srv, srv.grpcServer) }
func (srv *server) GetGrpcServer() *grpc.Server { return srv.grpcServer }

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
	fmt.Println("LDAP Service - Directory integration and user synchronization")
	fmt.Println()
	fmt.Println("USAGE:")
	fmt.Println("  ldap-service [OPTIONS] [id] [config_path]")
	fmt.Println()
	fmt.Println("OPTIONS:")
	fmt.Println("  --debug       Enable debug logging")
	fmt.Println("  --describe    Print service description as JSON and exit")
	fmt.Println("  --health      Print service health status as JSON and exit")
	fmt.Println("  --version     Print version information as JSON and exit")
	fmt.Println("  --help        Show this help message and exit")
	fmt.Println()
	fmt.Println("FEATURES:")
	fmt.Println("  • LDAP directory integration")
	fmt.Println("  • User synchronization from LDAP/AD")
	fmt.Println("  • Embedded LDAP server (port 389)")
	fmt.Println("  • LDAPS support (port 636)")
	fmt.Println()
	fmt.Println("EXAMPLES:")
	fmt.Println("  ldap-service")
	fmt.Println("  ldap-service --version")
	fmt.Println("  ldap-service --debug")
}

func printVersion() {
	info := map[string]string{
		"service":    "ldap",
		"version":    Version,
		"build_time": BuildTime,
		"git_commit": GitCommit,
	}
	data, _ := json.MarshalIndent(info, "", "  ")
	fmt.Println(string(data))
}

// --- main entrypoint ---

func main() {
	s := initializeServerDefaults()

	var (
		enableDebug  = flag.Bool("debug", false, "enable debug logging")
		showVersion  = flag.Bool("version", false, "print version information as JSON and exit")
		showHelp     = flag.Bool("help", false, "show usage information and exit")
		showDescribe = flag.Bool("describe", false, "print service description as JSON and exit")
		showHealth   = flag.Bool("health", false, "print service health status as JSON and exit")
	)

	flag.Usage = printUsage
	flag.Parse()

	if *enableDebug {
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
		s.Logger = logger
		logger.Debug("debug logging enabled")
	}

	if *showHelp {
		printUsage()
		return
	}

	if *showVersion {
		printVersion()
		return
	}

	if *showDescribe {
		data, _ := json.MarshalIndent(s, "", "  ")
		fmt.Println(string(data))
		return
	}

	if *showHealth {
		health := map[string]interface{}{
			"service": s.Name,
			"status":  "healthy",
			"version": s.Version,
		}
		data, _ := json.MarshalIndent(health, "", "  ")
		fmt.Println(string(data))
		return
	}

	args := flag.Args()
	if err := globular.AllocatePortIfNeeded(s, args); err != nil {
		logger.Error("fail to allocate port", "error", err)
		os.Exit(1)
	}

	globular.ParsePositionalArgs(s, args)
	globular.LoadRuntimeConfig(s)

	if s.Domain == "" {
		s.Domain = "localhost"
	}
	if s.Address == "" {
		s.Address = fmt.Sprintf("localhost:%d", s.Port)
	}

	logger.Info("starting ldap service", "service", s.Name, "version", s.Version, "domain", s.Domain)

	start := time.Now()
	if err := s.Init(); err != nil {
		logger.Error("failed to initialize service", "name", s.Name, "id", s.Id, "err", err)
		os.Exit(1)
	}
	logger.Info("service initialized", "duration_ms", time.Since(start).Milliseconds())

	ldappb.RegisterLdapServiceServer(s.grpcServer, s)
	reflection.Register(s.grpcServer)
	logger.Debug("gRPC handlers registered")

	logger.Info("service ready", "service", s.Name, "version", s.Version, "port", s.Port, "domain", s.Domain, "startup_ms", time.Since(start).Milliseconds())

	lifecycle := globular.NewLifecycleManager(s, logger)
	if err := lifecycle.Start(); err != nil {
		logger.Error("service start failed", "service", s.Name, "err", err)
		os.Exit(1)
	}
}

func initializeServerDefaults() *server {
	cfg := DefaultConfig()
	s := &server{
		Connections:     map[string]connection{},
		LdapSyncInfos:   map[string]interface{}{},
		Name:            string(ldappb.File_ldap_proto.Services().Get(0).FullName()),
		Proto:           ldappb.File_ldap_proto.Path(),
		Path:            "",
		Port:            cfg.Port,
		Proxy:           cfg.Proxy,
		Protocol:        cfg.Protocol,
		Version:         Version,
		Description:     "LDAP service with directory integration and user synchronization",
		AllowAllOrigins: cfg.AllowAllOrigins,
		AllowedOrigins:  cfg.AllowedOrigins,
		PublisherID:     cfg.PublisherID,
		Permissions:     loadDefaultPermissions(),
		Keywords:        []string{"ldap", "directory", "authentication", "user", "sync", "active-directory"},
		Repositories:    globular.CloneStringSlice(cfg.Repositories),
		Discoveries:     globular.CloneStringSlice(cfg.Discoveries),
		Dependencies:    globular.CloneStringSlice(cfg.Dependencies),
		Process:         -1,
		ProxyProcess:    -1,
		KeepAlive:       cfg.KeepAlive,
		KeepUpToDate:    cfg.KeepUpToDate,
		LdapListenAddr:  cfg.LdapListenAddr,
		LdapsListenAddr: cfg.LdapsListenAddr,
		DisableLDAPS:    cfg.DisableLDAPS,
		Logger:          logger,
	}

	if p, err := filepath.Abs(filepath.Dir(os.Args[0])); err == nil {
		s.Path = p
	}

	s.Domain, s.Address = globular.GetDefaultDomainAddress(s.Port)
	return s
}

func (srv *server) ensureRuntimeState() {
	if srv.Connections == nil {
		srv.Connections = map[string]connection{}
	}
	if srv.LdapSyncInfos == nil {
		srv.LdapSyncInfos = map[string]interface{}{}
	}
	if srv.Logger == nil {
		srv.Logger = logger
	}
}
