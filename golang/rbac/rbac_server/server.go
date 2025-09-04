// Package main provides the RBAC gRPC service wired for Globular.
// It mirrors the clean structure and CLI ergonomics of the Echo example,
// adds --describe and --health, uses slog for logging, and clarifies errors.
package main

import (

	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/event/event_client"
	"github.com/globulario/services/golang/globular_client"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/log/log_client"
	"github.com/globulario/services/golang/log/logpb"
	"github.com/globulario/services/golang/rbac/rbac_client"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/resource/resource_client"
	"github.com/globulario/services/golang/resource/resourcepb"
	"github.com/globulario/services/golang/storage/storage_store"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/encoding/protojson"
)

// -----------------------------------------------------------------------------
// Defaults & CORS
// -----------------------------------------------------------------------------

var (
	defaultPort  = 10029
	defaultProxy = 10030

	allowAllOrigins   = true
	allowedOriginsStr = ""
)

// -----------------------------------------------------------------------------
// Server type (Globular contract + RBAC runtime)
// -----------------------------------------------------------------------------

type server struct {
	// Core metadata
	Id          string
	Mac         string
	Name        string
	Domain      string
	Address     string
	Path        string
	Proto       string
	Port        int
	Proxy       int
	Protocol    string
	Version     string
	PublisherID string
	Description string
	Keywords    []string
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
	CacheAddress    string

	// TLS
	TLS                bool
	CertFile           string
	KeyFile            string
	CertAuthorityTrust string

	// Runtime
	grpcServer *grpc.Server

	// Local KV/cache backends used by RBAC
	cache       *storage_store.BigCache_store
	permissions storage_store.Store

	// Data root (for permissions store)
	Root string
}

// -----------------------------------------------------------------------------
// Key/Value helpers for RBAC state
// -----------------------------------------------------------------------------

func (srv *server) setItem(key string, val []byte) error {
	if err := srv.cache.SetItem(key, val); err != nil {
		return err
	}
	return srv.permissions.SetItem(key, val)
}

func (srv *server) getItem(key string) ([]byte, error) {
	if val, err := srv.cache.GetItem(key); err == nil {
		return val, nil
	}
	return srv.permissions.GetItem(key)
}

func (srv *server) removeItem(key string) error {
	if err := srv.cache.RemoveItem(key); err != nil {
		return err
	}
	return srv.permissions.RemoveItem(key)
}

// -----------------------------------------------------------------------------
// Globular service contract (getters / setters)
// -----------------------------------------------------------------------------

func (srv *server) GetConfigurationPath() string           { return srv.ConfigPath }
func (srv *server) SetConfigurationPath(path string)       { srv.ConfigPath = path }
func (srv *server) GetAddress() string                     { return srv.Address }
func (srv *server) SetAddress(address string)              { srv.Address = address }
func (srv *server) GetProcess() int                        { return srv.Process }
func (srv *server) SetProcess(pid int)                     { srv.Process = pid }
func (srv *server) GetProxyProcess() int                   { return srv.ProxyProcess }
func (srv *server) SetProxyProcess(pid int)                { srv.ProxyProcess = pid }
func (srv *server) GetState() string                       { return srv.State }
func (srv *server) SetState(state string)                  { srv.State = state }
func (srv *server) GetLastError() string                   { return srv.LastError }
func (srv *server) SetLastError(err string)                { srv.LastError = err }
func (srv *server) SetModTime(modtime int64)               { srv.ModTime = modtime }
func (srv *server) GetModTime() int64                      { return srv.ModTime }
func (srv *server) GetId() string                          { return srv.Id }
func (srv *server) SetId(id string)                        { srv.Id = id }
func (srv *server) GetName() string                        { return srv.Name }
func (srv *server) SetName(name string)                    { srv.Name = name }
func (srv *server) GetMac() string                         { return srv.Mac }
func (srv *server) SetMac(mac string)                      { srv.Mac = mac }
func (srv *server) GetDescription() string                 { return srv.Description }
func (srv *server) SetDescription(description string)      { srv.Description = description }
func (srv *server) GetKeywords() []string                  { return srv.Keywords }
func (srv *server) SetKeywords(keywords []string)          { srv.Keywords = keywords }
func (srv *server) GetRepositories() []string              { return srv.Repositories }
func (srv *server) SetRepositories(repositories []string)  { srv.Repositories = repositories }
func (srv *server) GetDiscoveries() []string               { return srv.Discoveries }
func (srv *server) SetDiscoveries(discoveries []string)    { srv.Discoveries = discoveries }
func (srv *server) Dist(path string) (string, error)       { return globular.Dist(path, srv) }
func (srv *server) GetDependencies() []string {
	if srv.Dependencies == nil {
		srv.Dependencies = make([]string, 0)
	}
	return srv.Dependencies
}
func (srv *server) SetDependency(dependency string) {
	if srv.Dependencies == nil {
		srv.Dependencies = make([]string, 0)
	}
	if !Utility.Contains(srv.Dependencies, dependency) {
		srv.Dependencies = append(srv.Dependencies, dependency)
	}
}
func (srv *server) GetChecksum() string                    { return srv.Checksum }
func (srv *server) SetChecksum(checksum string)            { srv.Checksum = checksum }
func (srv *server) GetPlatform() string                    { return srv.Plaform }
func (srv *server) SetPlatform(platform string)            { srv.Plaform = platform }
func (srv *server) GetPath() string                        { return srv.Path }
func (srv *server) SetPath(path string)                    { srv.Path = path }
func (srv *server) GetProto() string                       { return srv.Proto }
func (srv *server) SetProto(proto string)                  { srv.Proto = proto }
func (srv *server) GetPort() int                           { return srv.Port }
func (srv *server) SetPort(port int)                       { srv.Port = port }
func (srv *server) GetProxy() int                          { return srv.Proxy }
func (srv *server) SetProxy(proxy int)                     { srv.Proxy = proxy }
func (srv *server) GetProtocol() string                    { return srv.Protocol }
func (srv *server) SetProtocol(protocol string)            { srv.Protocol = protocol }
func (srv *server) GetAllowAllOrigins() bool               { return srv.AllowAllOrigins }
func (srv *server) SetAllowAllOrigins(v bool)              { srv.AllowAllOrigins = v }
func (srv *server) GetAllowedOrigins() string              { return srv.AllowedOrigins }
func (srv *server) SetAllowedOrigins(v string)             { srv.AllowedOrigins = v }
func (srv *server) GetDomain() string                      { return srv.Domain }
func (srv *server) SetDomain(domain string)                { srv.Domain = domain }
func (srv *server) GetTls() bool                           { return srv.TLS }
func (srv *server) SetTls(hasTls bool)                     { srv.TLS = hasTls }
func (srv *server) GetCertAuthorityTrust() string          { return srv.CertAuthorityTrust }
func (srv *server) SetCertAuthorityTrust(ca string)        { srv.CertAuthorityTrust = ca }
func (srv *server) GetCertFile() string                    { return srv.CertFile }
func (srv *server) SetCertFile(certFile string)            { srv.CertFile = certFile }
func (srv *server) GetKeyFile() string                     { return srv.KeyFile }
func (srv *server) SetKeyFile(keyFile string)              { srv.KeyFile = keyFile }
func (srv *server) GetVersion() string                     { return srv.Version }
func (srv *server) SetVersion(version string)              { srv.Version = version }
func (srv *server) GetPublisherID() string                 { return srv.PublisherID }
func (srv *server) SetPublisherID(id string)               { srv.PublisherID = id }
func (srv *server) GetKeepUpToDate() bool                  { return srv.KeepUpToDate }
func (srv *server) SetKeepUptoDate(val bool)               { srv.KeepUpToDate = val }
func (srv *server) GetKeepAlive() bool                     { return srv.KeepAlive }
func (srv *server) SetKeepAlive(val bool)                  { srv.KeepAlive = val }
func (srv *server) GetPermissions() []interface{}          { return srv.Permissions }
func (srv *server) SetPermissions(p []interface{})         { srv.Permissions = p }

// -----------------------------------------------------------------------------
// Event / Log / Resource helpers
// -----------------------------------------------------------------------------

func (srv *server) getEventClient() (*event_client.Event_Client, error) {
	Utility.RegisterFunction("NewEventService_Client", event_client.NewEventService_Client)
	c, err := globular_client.GetClient(srv.Address, "event.EventService", "NewEventService_Client")
	if err != nil { return nil, err }
	return c.(*event_client.Event_Client), nil
}

func (srv *server) publish(event string, data []byte) error {
	ec, err := srv.getEventClient()
	if err != nil { return err }
	return ec.Publish(event, data)
}

func (srv *server) GetLogClient() (*log_client.Log_Client, error) {
	Utility.RegisterFunction("NewLogService_Client", log_client.NewLogService_Client)
	c, err := globular_client.GetClient(srv.Address, "log.LogService", "NewLogService_Client")
	if err != nil { return nil, err }
	return c.(*log_client.Log_Client), nil
}

func (srv *server) logServiceInfo(method, fileLine, functionName, msg string) error {
	lc, err := srv.GetLogClient()
	if err != nil { return err }
	return lc.Log(srv.Name, srv.Domain, method, logpb.LogLevel_INFO_MESSAGE, msg, fileLine, functionName)
}

func (srv *server) logServiceError(method, fileLine, functionName, msg string) error {
	lc, err := srv.GetLogClient()
	if err != nil { return err }
	return lc.Log(srv.Name, srv.Address, method, logpb.LogLevel_ERROR_MESSAGE, msg, fileLine, functionName)
}

func (srv *server) getResourceClient(address string) (*resource_client.Resource_Client, error) {
	Utility.RegisterFunction("NewResourceService_Client", resource_client.NewResourceService_Client)
	c, err := globular_client.GetClient(address, "resource.ResourceService", "NewResourceService_Client")
	if err != nil { return nil, err }
	return c.(*resource_client.Resource_Client), nil
}

// account/group/app/peer/org lookup helpers (cache first)

func (srv *server) getAccount(accountId string) (*resourcepb.Account, error) {
	if data, err := srv.cache.GetItem(accountId); err == nil {
		acc := new(resourcepb.Account)
		if err := protojson.Unmarshal(data, acc); err == nil { return acc, nil }
	}
	domain := srv.Domain
	if strings.Contains(accountId, "@") {
		parts := strings.Split(accountId, "@")
		if len(parts) == 2 && parts[1] != "" { domain = parts[1] }
		accountId = parts[0]
	}
	rc, err := srv.getResourceClient(domain)
	if err != nil { return nil, err }
	acc, err := rc.GetAccount(accountId)
	if err != nil { return nil, err }
	if b, err := protojson.Marshal(acc); err == nil { _ = srv.cache.SetItem(accountId, b) }
	return acc, nil
}

func (srv *server) accountExist(id string) (bool, string) {
	acc, err := srv.getAccount(id)
	if err != nil || acc == nil { return false, "" }
	return true, acc.Id + "@" + acc.Domain
}

func (srv *server) getGroup(groupId string) (*resourcepb.Group, error) {
	if data, err := srv.cache.GetItem(groupId); err == nil {
		g := new(resourcepb.Group)
		if err := protojson.Unmarshal(data, g); err == nil { return g, nil }
	}
	domain := srv.Domain
	if strings.Contains(groupId, "@") {
		parts := strings.Split(groupId, "@")
		if len(parts) == 2 && parts[1] != "" { domain = parts[1] }
		groupId = parts[0]
	}
	rc, err := srv.getResourceClient(domain)
	if err != nil { return nil, err }
	gs, err := rc.GetGroups(`{"_id":"` + groupId + `"}`)
	if err != nil { return nil, err }
	if len(gs) == 0 { return nil, errors.New("group not found: " + groupId) }
	if b, err := protojson.Marshal(gs[0]); err == nil { _ = srv.cache.SetItem(groupId, b) }
	return gs[0], nil
}

func (srv *server) groupExist(id string) (bool, string) {
	g, err := srv.getGroup(id)
	if err != nil || g == nil { return false, "" }
	return true, g.Id + "@" + g.Domain
}

func (srv *server) getApplication(applicationId string) (*resourcepb.Application, error) {
	domain := srv.Domain
	if strings.Contains(applicationId, "@") {
		parts := strings.Split(applicationId, "@")
		if len(parts) == 2 && parts[1] != "" { domain = parts[1] }
		applicationId = parts[0]
	}
	q0 := `{"_id":"` + applicationId + `"}`
	q1 := `{"name":"` + applicationId + `"}`
	rc, err := srv.getResourceClient(domain)
	if err != nil { return nil, err }
	apps, err := rc.GetApplications(q0)
	if err != nil || len(apps) == 0 { apps, err = rc.GetApplications(q1) }
	if err != nil { return nil, err }
	if len(apps) == 0 { return nil, errors.New("application not found: " + applicationId) }
	return apps[0], nil
}

func (srv *server) applicationExist(id string) (bool, string) {
	app, err := srv.getApplication(id)
	if err != nil || app == nil { return false, "" }
	return true, app.Id + "@" + app.Domain
}

func (srv *server) getPeer(peerId string) (*resourcepb.Peer, error) {
	addr, _ := config.GetAddress()
	rc, err := srv.getResourceClient(addr)
	if err != nil { return nil, err }
	ps, err := rc.GetPeers(`{"mac":"` + peerId + `"}`)
	if err != nil { return nil, err }
	if len(ps) == 0 { return nil, errors.New("peer not found: " + peerId) }
	return ps[0], nil
}

func (srv *server) peerExist(id string) bool {
	p, err := srv.getPeer(id)
	return err == nil && p != nil
}

func (srv *server) getOrganization(organizationId string) (*resourcepb.Organization, error) {
	domain := srv.Domain
	if strings.Contains(organizationId, "@") {
		parts := strings.Split(organizationId, "@")
		if len(parts) == 2 && parts[1] != "" { domain = parts[1] }
		organizationId = parts[0]
	}
	rc, err := srv.getResourceClient(domain)
	if err != nil { return nil, err }
	orgs, err := rc.GetOrganizations(`{"_id":"` + organizationId + `"}`)
	if err != nil { return nil, err }
	if len(orgs) == 0 { return nil, errors.New("organization not found: " + organizationId) }
	return orgs[0], nil
}

func (srv *server) organizationExist(id string) (bool, string) {
	o, err := srv.getOrganization(id)
	if err != nil || o == nil { return false, "" }
	return true, o.Id + "@" + o.Domain
}

func (srv *server) getRoles() ([]*resourcepb.Role, error) {
	rc, err := srv.getResourceClient(srv.Address)
	if err != nil { return nil, err }
	rs, err := rc.GetRoles("")
	if err != nil { return nil, err }
	return rs, nil
}

func (srv *server) getGroups() ([]*resourcepb.Group, error) {
	rc, err := srv.getResourceClient(srv.Address)
	if err != nil { return nil, err }
	gs, err := rc.GetGroups(`{}`)
	if err != nil { return nil, err }
	return gs, nil
}

func (srv *server) getOrganizations() ([]*resourcepb.Organization, error) {
	rc, err := srv.getResourceClient(srv.Address)
	if err != nil { return nil, err }
	os_, err := rc.GetOrganizations("")
	if err != nil { return nil, err }
	return os_, nil
}

func (srv *server) getRole(roleId string) (*resourcepb.Role, error) {
	domain := srv.Domain
	if strings.Contains(roleId, "@") {
		parts := strings.Split(roleId, "@")
		if len(parts) == 2 && parts[1] != "" { domain = parts[1] }
		roleId = parts[0]
	}
	rc, err := srv.getResourceClient(domain)
	if err != nil { return nil, err }
	rs, err := rc.GetRoles(`{"_id":"` + roleId + `"}`)
	if err != nil { return nil, err }
	if len(rs) == 0 { return nil, errors.New("role not found: " + roleId) }
	return rs[0], nil
}

func (srv *server) roleExist(id string) (bool, string) {
	r, err := srv.getRole(id)
	if err != nil || r == nil { return false, "" }
	return true, r.Id + "@" + r.Domain
}

// -----------------------------------------------------------------------------
// Lifecycle (Init/Save/Start/Stop) and gRPC plumbing
// -----------------------------------------------------------------------------

func (srv *server) Init() error {
	if err := globular.InitService(srv); err != nil { return err }
	gs, err := globular.InitGrpcServer(srv)
	if err != nil { return err }
	srv.grpcServer = gs
	return nil
}

func (srv *server) Save() error                { return globular.SaveService(srv) }
func (srv *server) StartService() error        { return globular.StartService(srv, srv.grpcServer) }
func (srv *server) StopService() error         { return globular.StopService(srv, srv.grpcServer) }
/*
// Optional administrative RPCs (example): Stop
func (srv *server) Stop(ctx context.Context, _ *rbacpb.StopRqst) (*rbacpb.StopResponse, error) {
	return &rbacpb.StopResponse{}, srv.StopService()
}*/

// -----------------------------------------------------------------------------
// Main
// -----------------------------------------------------------------------------

var logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{ Level: slog.LevelInfo }))

func main() {
	srv := new(server)

	// Fill metadata that doesn't require etcd/config yet.
	srv.Name = string(rbacpb.File_rbac_proto.Services().Get(0).FullName())
	srv.Proto = rbacpb.File_rbac_proto.Path()
	srv.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	srv.Port = defaultPort
	srv.Proxy = defaultProxy
	srv.Protocol = "grpc"
	srv.Version = "0.0.1"
	srv.PublisherID = "localhost"
	srv.Description = "RBAC service managing permissions and access control."
	srv.Keywords = []string{"rbac", "permissions", "security"}
	srv.Repositories, srv.Discoveries = make([]string, 0), make([]string, 0)
	srv.Dependencies = []string{"resource.ResourceService"}
	srv.Permissions = make([]interface{}, 0)
	srv.Process, srv.ProxyProcess = -1, -1
	srv.AllowAllOrigins, srv.AllowedOrigins = allowAllOrigins, allowedOriginsStr
	srv.KeepAlive, srv.KeepUpToDate = true, true
	srv.CacheAddress = srv.Address

	// Register RBAC client ctor for other components if needed.
	Utility.RegisterFunction("NewRbacService_Client", rbac_client.NewRbacService_Client)

	// ---- CLI flags handled BEFORE any call that might touch etcd ----
	args := os.Args[1:]
	if len(args) == 0 {
		printUsage()
		return
	}

	for _, a := range args {
		switch strings.ToLower(a) {
		case "--describe":
			// Best-effort runtime fields without hitting etcd
			srv.Process = os.Getpid()
			srv.State = "starting"

			// Prefer env if present; otherwise harmless defaults
			if v, ok := os.LookupEnv("GLOBULAR_DOMAIN"); ok && v != "" { srv.Domain = strings.ToLower(v) } else { srv.Domain = "localhost" }
			if v, ok := os.LookupEnv("GLOBULAR_ADDRESS"); ok && v != "" { srv.Address = strings.ToLower(v) } else { srv.Address = "localhost:" + Utility.ToString(srv.Port) }

			b, err := globular.DescribeJSON(srv)
			if err != nil {
				logger.Error("describe failed", "service", srv.Name, "id", srv.Id, "err", err)
				os.Exit(2)
			}
			_, _ = os.Stdout.Write(b)
			_, _ = os.Stdout.Write([]byte("\n"))
			return

		case "--health":
			if srv.Port == 0 || srv.Name == "" {
				logger.Error("health: missing required fields", "service", srv.Name, "port", srv.Port)
				os.Exit(2)
			}
			b, err := globular.HealthJSON(srv, &globular.HealthOptions{ Timeout: 1500 * time.Millisecond })
			if err != nil {
				logger.Error("health probe failed", "service", srv.Name, "id", srv.Id, "err", err)
				os.Exit(2)
			}
			_, _ = os.Stdout.Write(b)
			_, _ = os.Stdout.Write([]byte("\n"))
			return
		}
	}

	// Optional positional args (unchanged from legacy): service_id [config_path]
	if len(args) == 1 && !strings.HasPrefix(args[0], "-") {
		srv.Id = args[0]
	} else if len(args) == 2 && !strings.HasPrefix(args[0], "-") && !strings.HasPrefix(args[1], "-") {
		srv.Id = args[0]
		srv.ConfigPath = args[1]
	}

	// Safe to fetch config (may consult etcd or local file fallback)
	if d, err := config.GetDomain(); err == nil { srv.Domain = d } else { srv.Domain = "localhost" }
	if a, err := config.GetAddress(); err == nil && strings.TrimSpace(a) != "" { srv.Address = a }
	if srv.CacheAddress == "localhost" || srv.CacheAddress == "" { srv.CacheAddress = srv.Address }
	if srv.Root == "" { srv.Root = config.GetDataDir() }

	// Open cache store
	srv.cache = storage_store.NewBigCache_store()
	if err := srv.cache.Open(""); err != nil {
		logger.Error("cache open failed", "path", srv.Root+"/cache", "err", err)
	}

	// Open permissions store (badger)
	srv.permissions = storage_store.NewBadger_store()
	if err := srv.permissions.Open(`{"path":"` + srv.Root + `", "name":"permissions"}`); err != nil {
		logger.Error("permissions store open failed", "path", srv.Root+"/permissions", "err", err)
	}

	start := time.Now()
	if err := srv.Init(); err != nil {
		logger.Error("service init failed", "service", srv.Name, "id", srv.Id, "err", err)
		os.Exit(1)
	}

	// Clear precomputed USED_SPACE keys on startup (ensures fresh computation)
	if idsRaw, err := srv.getItem("USED_SPACE"); err == nil {
		var ids []string
		if jsonErr := json.Unmarshal(idsRaw, &ids); jsonErr == nil {
			for _, k := range ids { _ = srv.removeItem(k) }
		}
	}

	// Register RPCs
	rbacpb.RegisterRbacServiceServer(srv.grpcServer, srv)
	reflection.Register(srv.grpcServer)

	logger.Info("service ready",
		"service", srv.Name,
		"port", srv.Port,
		"proxy", srv.Proxy,
		"protocol", srv.Protocol,
		"domain", srv.Domain,
		"listen_ms", time.Since(start).Milliseconds())

	if err := srv.StartService(); err != nil {
		logger.Error("service start failed", "service", srv.Name, "id", srv.Id, "err", err)
		os.Exit(1)
	}
}

func printUsage() {
	logger.Info("Usage:\n  rbac_server [service_id] [config_path]\nOptions:\n  --describe    Print service metadata as JSON and exit\n  --health      Print service health as JSON and exit\nExamples:\n  rbac_server my-rbac-id /etc/globular/rbac/config.json\n  rbac_server --describe\n  rbac_server --health")
}
