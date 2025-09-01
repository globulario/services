package main

import (
	"log/slog"
	"os"
	"path/filepath"
	"github.com/globulario/services/golang/catalog/catalog_client"
	"github.com/globulario/services/golang/catalog/catalogpb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/event/event_client"
	"github.com/globulario/services/golang/globular_client"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/interceptors"
	"github.com/globulario/services/golang/persistence/persistence_client"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// -----------------------------------------------------------------------------
// Defaults & CORS
// -----------------------------------------------------------------------------

var (
	defaultPort       = 10017
	defaultProxy      = 10018
	allow_all_origins = true
	allowed_origins   = ""
)

// -----------------------------------------------------------------------------
// Service implementation
// -----------------------------------------------------------------------------

// server implements the Catalog gRPC microservice and the Globular runtime interface.
type server struct {
	// Generic service attributes required by Globular runtime.
	Id              string
	Name            string
	Mac             string
	Port            int
	Proxy           int
	Path            string
	Proto           string
	AllowAllOrigins bool
	AllowedOrigins  string
	Protocol        string
	Domain          string
	Address         string
	Description     string
	Keywords        []string
	Repositories    []string
	Discoveries     []string
	Process         int
	ProxyProcess    int
	ConfigPath      string
	State           string
	LastError       string
	CertFile           string
	KeyFile            string
	CertAuthorityTrust string
	TLS                bool
	Version            string
	PublisherID        string
	KeepUpToDate       bool
	KeepAlive          bool
	Checksum           string
	Plaform            string
	ModTime            int64

	// Service configuration and dependencies.
	Services     map[string]interface{}
	Permissions  []interface{}
	Dependencies []string

	// External clients.
	persistenceClient *persistence_client.Persistence_Client
	eventClient       *event_client.Event_Client

	// Runtime component.
	grpcServer *grpc.Server
}

// --- Globular getters/setters (unchanged signatures) ---

func (srv *server) GetConfigurationPath() string        { return srv.ConfigPath }
func (srv *server) SetConfigurationPath(path string)    { srv.ConfigPath = path }
func (srv *server) GetAddress() string                  { return srv.Address }
func (srv *server) SetAddress(address string)           { srv.Address = address }
func (srv *server) GetProcess() int                     { return srv.Process }
func (srv *server) SetProcess(pid int)                  { srv.Process = pid }
func (srv *server) GetProxyProcess() int                { return srv.ProxyProcess }
func (srv *server) SetProxyProcess(pid int)             { srv.ProxyProcess = pid }
func (srv *server) GetState() string                    { return srv.State }
func (srv *server) SetState(state string)               { srv.State = state }
func (srv *server) GetLastError() string                { return srv.LastError }
func (srv *server) SetLastError(err string)             { srv.LastError = err }
func (srv *server) SetModTime(modtime int64)            { srv.ModTime = modtime }
func (srv *server) GetModTime() int64                   { return srv.ModTime }
func (srv *server) GetId() string                       { return srv.Id }
func (srv *server) SetId(id string)                     { srv.Id = id }
func (srv *server) GetName() string                     { return srv.Name }
func (srv *server) SetName(name string)                 { srv.Name = name }
func (srv *server) GetMac() string                      { return srv.Mac }
func (srv *server) SetMac(mac string)                   { srv.Mac = mac }
func (srv *server) GetChecksum() string                 { return srv.Checksum }
func (srv *server) SetChecksum(checksum string)         { srv.Checksum = checksum }
func (srv *server) GetPlatform() string                 { return srv.Plaform }
func (srv *server) SetPlatform(platform string)         { srv.Plaform = platform }
func (srv *server) GetDescription() string              { return srv.Description }
func (srv *server) SetDescription(description string)   { srv.Description = description }
func (srv *server) GetKeywords() []string               { return srv.Keywords }
func (srv *server) SetKeywords(keywords []string)       { srv.Keywords = keywords }
func (srv *server) GetRepositories() []string           { return srv.Repositories }
func (srv *server) SetRepositories(repositories []string) { srv.Repositories = repositories }
func (srv *server) GetDiscoveries() []string            { return srv.Discoveries }
func (srv *server) SetDiscoveries(discoveries []string) { srv.Discoveries = discoveries }
func (srv *server) Dist(path string) (string, error)    { return globular.Dist(path, srv) }
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
func (srv *server) GetPath() string                 { return srv.Path }
func (srv *server) SetPath(path string)             { srv.Path = path }
func (srv *server) GetProto() string                { return srv.Proto }
func (srv *server) SetProto(proto string)           { srv.Proto = proto }
func (srv *server) GetPort() int                    { return srv.Port }
func (srv *server) SetPort(port int)                { srv.Port = port }
func (srv *server) GetProxy() int                   { return srv.Proxy }
func (srv *server) SetProxy(proxy int)              { srv.Proxy = proxy }
func (srv *server) GetProtocol() string             { return srv.Protocol }
func (srv *server) SetProtocol(protocol string)     { srv.Protocol = protocol }
func (srv *server) GetAllowAllOrigins() bool        { return srv.AllowAllOrigins }
func (srv *server) SetAllowAllOrigins(v bool)       { srv.AllowAllOrigins = v }
func (srv *server) GetAllowedOrigins() string       { return srv.AllowedOrigins }
func (srv *server) SetAllowedOrigins(v string)      { srv.AllowedOrigins = v }
func (srv *server) GetDomain() string               { return srv.Domain }
func (srv *server) SetDomain(domain string)         { srv.Domain = domain }
func (srv *server) GetTls() bool                    { return srv.TLS }
func (srv *server) SetTls(hasTls bool)              { srv.TLS = hasTls }
func (srv *server) GetCertAuthorityTrust() string   { return srv.CertAuthorityTrust }
func (srv *server) SetCertAuthorityTrust(ca string) { srv.CertAuthorityTrust = ca }
func (srv *server) GetCertFile() string             { return srv.CertFile }
func (srv *server) SetCertFile(certFile string)     { srv.CertFile = certFile }
func (srv *server) GetKeyFile() string              { return srv.KeyFile }
func (srv *server) SetKeyFile(keyFile string)       { srv.KeyFile = keyFile }
func (srv *server) GetVersion() string              { return srv.Version }
func (srv *server) SetVersion(version string)       { srv.Version = version }
func (srv *server) GetPublisherID() string          { return srv.PublisherID }
func (srv *server) SetPublisherID(PublisherID string) { srv.PublisherID = PublisherID }
func (srv *server) GetKeepUpToDate() bool           { return srv.KeepUpToDate }
func (srv *server) SetKeepUptoDate(val bool)        { srv.KeepUpToDate = val }
func (srv *server) GetKeepAlive() bool              { return srv.KeepAlive }
func (srv *server) SetKeepAlive(val bool)           { srv.KeepAlive = val }
func (srv *server) GetPermissions() []interface{}   { return srv.Permissions }
func (srv *server) SetPermissions(p []interface{})  { srv.Permissions = p }

// GetPersistenceClient returns a Persistence client bound to the given address.
func GetPersistenceClient(address string) (*persistence_client.Persistence_Client, error) {
	Utility.RegisterFunction("NewPersistenceService_Client", persistence_client.NewPersistenceService_Client)
	client, err := globular_client.GetClient(address, "persistence.PersistenceService", "NewPersistenceService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*persistence_client.Persistence_Client), nil
}

// GetEventClient returns an Event client bound to the given address.
func GetEventClient(address string) (*event_client.Event_Client, error) {
	Utility.RegisterFunction("NewEventService_Client", event_client.NewEventService_Client)
	client, err := globular_client.GetClient(address, "event.EventService", "NewEventService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*event_client.Event_Client), nil
}

// Init initializes the service configuration and gRPC server, and connects to dependencies.
func (srv *server) Init() error {
	// Initialize config.
	if err := globular.InitService(srv); err != nil {
		slog.Error("init service failed", "service", srv.Name, "id", srv.Id, "err", err)
		return err
	}

	// Initialize gRPC server.
	grpcSrv, err := globular.InitGrpcServer(srv, interceptors.ServerUnaryInterceptor, interceptors.ServerStreamInterceptor)
	if err != nil {
		slog.Error("init grpc server failed", "service", srv.Name, "err", err)
		return err
	}
	srv.grpcServer = grpcSrv

	// Connect to Persistence.
	if srv.Services["Persistence"] != nil {
		cfg := srv.Services["Persistence"].(map[string]interface{})
		addr := cfg["Address"].(string)
		srv.persistenceClient, err = GetPersistenceClient(addr)
		if err != nil {
			slog.Error("connect persistence failed", "address", addr, "err", err)
		}
	}

	// Connect to Event.
	if srv.Services["Event"] != nil {
		cfg := srv.Services["Event"].(map[string]interface{})
		addr := cfg["Address"].(string)
		srv.eventClient, err = GetEventClient(addr)
		if err != nil {
			slog.Error("connect event failed", "address", addr, "err", err)
		}
	}

	slog.Info("service initialized", "service", srv.Name, "id", srv.Id, "address", srv.Address)
	return nil
}

// Save persists the current service configuration to disk.
func (srv *server) Save() error { return globular.SaveService(srv) }

// StartService starts the gRPC server and proxy if configured.
func (srv *server) StartService() error {
	slog.Info("starting service", "service", srv.Name, "port", srv.Port, "proxy", srv.Proxy, "protocol", srv.Protocol)
	return globular.StartService(srv, srv.grpcServer)
}

// StopService gracefully stops the gRPC server.
func (srv *server) StopService() error {
	slog.Info("stopping service", "service", srv.Name)
	return globular.StopService(srv, srv.grpcServer)
}

// -----------------------------------------------------------------------------
// Entrypoint
// -----------------------------------------------------------------------------

// main boots the Catalog service and blocks until the gRPC server stops.
func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	s := new(server)
	s.Name = string(catalogpb.File_catalog_proto.Services().Get(0).FullName())
	s.Proto = catalogpb.File_catalog_proto.Path()
	s.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	s.Port = defaultPort
	s.Proxy = defaultProxy
	s.Protocol = "grpc"
	s.Domain, _ = config.GetDomain()
	s.Address, _ = config.GetAddress()
	s.Version = "0.0.1"
	s.Keywords = make([]string, 0)
	s.Repositories = make([]string, 0)
	s.Discoveries = make([]string, 0)
	s.Dependencies = make([]string, 0)
	s.Process = -1
	s.ProxyProcess = -1
	s.KeepAlive = true
	s.KeepUpToDate = true
	s.AllowAllOrigins = allow_all_origins
	s.AllowedOrigins = allowed_origins

	Utility.RegisterFunction("NewCatalogService_Client", catalog_client.NewCatalogService_Client)

	// Default dependency config from local address.
	s.Services = make(map[string]interface{})
	s.Services["Persistence"] = map[string]interface{}{"Address": s.Address}
	s.Services["Event"] = map[string]interface{}{"Address": s.Address}

	// ID / config path from args.
	if len(os.Args) == 2 {
		s.Id = os.Args[1]
	} else if len(os.Args) == 3 {
		s.Id = os.Args[1]
		s.ConfigPath = os.Args[2]
	}

	// Init service.
	if err := s.Init(); err != nil {
		slog.Error("service init failed", "service", s.Name, "id", s.Id, "err", err)
		os.Exit(1)
	}
	if s.Address == "" {
		s.Address, _ = config.GetAddress()
	}

	// Register gRPC service.
	catalogpb.RegisterCatalogServiceServer(s.grpcServer, s)
	reflection.Register(s.grpcServer)
	slog.Info("gRPC service registered", "service", s.Name, "port", s.Port)

	// Serve.
	if err := s.StartService(); err != nil {
		slog.Error("service start failed", "err", err)
		os.Exit(1)
	}
}
