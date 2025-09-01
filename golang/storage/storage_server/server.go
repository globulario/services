package main

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/globulario/services/golang/config"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/interceptors"
	Utility "github.com/globulario/utility"

	"github.com/globulario/services/golang/storage/storage_client"
	"github.com/globulario/services/golang/storage/storage_store"
	"github.com/globulario/services/golang/storage/storagepb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// global logger (initialized in main)
var logger *slog.Logger

// Keep connection information here.
type connection struct {
	Id   string            // The connection id
	Name string            // The kv store name
	Type storagepb.StoreType
}

// server implements the Storage gRPC service and Globular service hooks.
type server struct {
	// Globular metadata / config
	Id              string
	Name            string
	Mac             string
	Path            string
	Proto           string
	Port            int
	Proxy           int
	AllowAllOrigins bool
	AllowedOrigins  string // comma-separated origins
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
	LastError       string
	State           string
	ModTime         int64

	// TLS / versioning
	CertFile            string
	KeyFile             string
	CertAuthorityTrust  string
	TLS                 bool
	Version             string
	PublisherID         string
	Plaform             string
	KeepUpToDate        bool
	Checksum            string
	KeepAlive           bool
	Permissions         []interface{} // action permissions for the service
	Dependencies        []string      // required services

	// Runtime
	grpcServer *grpc.Server

	// Storage connections and stores
	Connections map[string]connection
	stores      map[string]storage_store.Store
}

func (srv *server) GetConfigurationPath() string { return srv.ConfigPath }
func (srv *server) SetConfigurationPath(path string) { srv.ConfigPath = path }

// GetAddress returns the HTTP address where /config is served.
func (srv *server) GetAddress() string { return srv.Address }
func (srv *server) SetAddress(address string) { srv.Address = address }

func (srv *server) GetProcess() int { return srv.Process }

// SetProcess sets the process ID and closes all open stores to ensure a clean handoff.
func (srv *server) SetProcess(pid int) {
	for _, store := range srv.stores {
		if store != nil {
			_ = store.Close()
		}
	}
	srv.Process = pid
}

func (srv *server) GetProxyProcess() int         { return srv.ProxyProcess }
func (srv *server) SetProxyProcess(pid int)      { srv.ProxyProcess = pid }
func (srv *server) GetState() string             { return srv.State }
func (srv *server) SetState(state string)        { srv.State = state }
func (srv *server) GetLastError() string         { return srv.LastError }
func (srv *server) SetLastError(err string)      { srv.LastError = err }
func (srv *server) SetModTime(modtime int64)     { srv.ModTime = modtime }
func (srv *server) GetModTime() int64            { return srv.ModTime }
func (srv *server) GetId() string                { return srv.Id }
func (srv *server) SetId(id string)              { srv.Id = id }
func (srv *server) GetName() string              { return srv.Name }
func (srv *server) SetName(name string)          { srv.Name = name }
func (srv *server) GetMac() string               { return srv.Mac }
func (srv *server) SetMac(mac string)            { srv.Mac = mac }
func (srv *server) GetDescription() string       { return srv.Description }
func (srv *server) SetDescription(d string)      { srv.Description = d }
func (srv *server) GetKeywords() []string        { return srv.Keywords }
func (srv *server) SetKeywords(k []string)       { srv.Keywords = k }
func (srv *server) GetRepositories() []string    { return srv.Repositories }
func (srv *server) SetRepositories(r []string)   { srv.Repositories = r }
func (srv *server) GetDiscoveries() []string     { return srv.Discoveries }
func (srv *server) SetDiscoveries(d []string)    { srv.Discoveries = d }
func (srv *server) GetChecksum() string          { return srv.Checksum }
func (srv *server) SetChecksum(cs string)        { srv.Checksum = cs }
func (srv *server) GetPlatform() string          { return srv.Plaform }
func (srv *server) SetPlatform(p string)         { srv.Plaform = p }
func (srv *server) GetPath() string              { return srv.Path }
func (srv *server) SetPath(path string)          { srv.Path = path }
func (srv *server) GetProto() string             { return srv.Proto }
func (srv *server) SetProto(proto string)        { srv.Proto = proto }
func (srv *server) GetPort() int                 { return srv.Port }
func (srv *server) SetPort(port int)             { srv.Port = port }
func (srv *server) GetProxy() int                { return srv.Proxy }
func (srv *server) SetProxy(proxy int)           { srv.Proxy = proxy }
func (srv *server) GetProtocol() string          { return srv.Protocol }
func (srv *server) SetProtocol(p string)         { srv.Protocol = p }
func (srv *server) GetAllowAllOrigins() bool     { return srv.AllowAllOrigins }
func (srv *server) SetAllowAllOrigins(b bool)    { srv.AllowAllOrigins = b }
func (srv *server) GetAllowedOrigins() string    { return srv.AllowedOrigins }
func (srv *server) SetAllowedOrigins(s string)   { srv.AllowedOrigins = s }
func (srv *server) GetDomain() string            { return srv.Domain }
func (srv *server) SetDomain(d string)           { srv.Domain = d }
func (srv *server) GetTls() bool                 { return srv.TLS }
func (srv *server) SetTls(b bool)                { srv.TLS = b }
func (srv *server) GetCertAuthorityTrust() string { return srv.CertAuthorityTrust }
func (srv *server) SetCertAuthorityTrust(ca string) { srv.CertAuthorityTrust = ca }
func (srv *server) GetCertFile() string          { return srv.CertFile }
func (srv *server) SetCertFile(cf string)        { srv.CertFile = cf }
func (srv *server) GetKeyFile() string           { return srv.KeyFile }
func (srv *server) SetKeyFile(kf string)         { srv.KeyFile = kf }
func (srv *server) GetVersion() string           { return srv.Version }
func (srv *server) SetVersion(v string)          { srv.Version = v }
func (srv *server) GetPublisherID() string       { return srv.PublisherID }
func (srv *server) SetPublisherID(pid string)    { srv.PublisherID = pid }
func (srv *server) GetKeepUpToDate() bool        { return srv.KeepUpToDate }
func (srv *server) SetKeepUptoDate(val bool)     { srv.KeepUpToDate = val }
func (srv *server) GetKeepAlive() bool           { return srv.KeepAlive }
func (srv *server) SetKeepAlive(val bool)        { srv.KeepAlive = val }
func (srv *server) GetPermissions() []interface{} { return srv.Permissions }
func (srv *server) SetPermissions(p []interface{}) { srv.Permissions = p }

// Dist packages the service for distribution via Globular.
func (srv *server) Dist(path string) (string, error) { return globular.Dist(path, srv) }

// GetDependencies returns required service IDs.
func (srv *server) GetDependencies() []string {
	if srv.Dependencies == nil {
		srv.Dependencies = make([]string, 0)
	}
	return srv.Dependencies
}

// SetDependency appends a dependency if not already present.
func (srv *server) SetDependency(dep string) {
	if srv.Dependencies == nil {
		srv.Dependencies = make([]string, 0)
	}
	if !Utility.Contains(srv.Dependencies, dep) {
		srv.Dependencies = append(srv.Dependencies, dep)
	}
}

// Init prepares config/runtime, initializes gRPC server with interceptors.
func (srv *server) Init() error {
	srv.stores = make(map[string]storage_store.Store)
	srv.Connections = make(map[string]connection)

	if err := globular.InitService(srv); err != nil {
		return err
	}
	var err error
	srv.grpcServer, err = globular.InitGrpcServer(
		srv,
		interceptors.ServerUnaryInterceptor,
		interceptors.ServerStreamInterceptor,
	)
	return err
}

// Save persists the current configuration to disk.
func (srv *server) Save() error { return globular.SaveService(srv) }

// StartService starts the bound gRPC server.
func (srv *server) StartService() error { return globular.StartService(srv, srv.grpcServer) }

// StopService gracefully stops the gRPC server.
func (srv *server) StopService() error { return globular.StopService(srv, srv.grpcServer) }

// Stop is the RPC endpoint to stop the service process.
func (srv *server) Stop(context.Context, *storagepb.StopRequest) (*storagepb.StopResponse, error) {
	return &storagepb.StopResponse{}, srv.StopService()
}

const (
	defaultPort        = 10013
	defaultProxy       = 10014
	allowAllOrigins    = true
	allowedOriginsList = ""
)

// main boots the Storage service.
func main() {
	// Structured logger to stdout
	logger = slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	// Actual server implementation.
	srv := new(server)
	srv.Connections = make(map[string]connection)
	srv.Name = string(storagepb.File_storage_proto.Services().Get(0).FullName())
	srv.Proto = storagepb.File_storage_proto.Path()
	srv.Port = defaultPort
	srv.Proxy = defaultProxy
	srv.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	srv.Protocol = "grpc"
	srv.Domain, _ = config.GetDomain()
	srv.Address, _ = config.GetAddress()
	srv.Version = "0.0.1"
	srv.AllowAllOrigins = allowAllOrigins
	srv.AllowedOrigins = allowedOriginsList
	srv.PublisherID = "localhost"
	srv.Keywords = make([]string, 0)
	srv.Repositories = make([]string, 0)
	srv.Discoveries = make([]string, 0)
	srv.Dependencies = make([]string, 0)
	srv.Process = -1
	srv.ProxyProcess = -1
	srv.KeepAlive = true
	srv.KeepUpToDate = true

	// Allow dynamic wiring of the client factory.
	Utility.RegisterFunction("NewStorageService_Client", storage_client.NewStorageService_Client)

	// Accept optional args: id [configPath]
	if len(os.Args) == 2 {
		srv.Id = os.Args[1]
	} else if len(os.Args) == 3 {
		srv.Id = os.Args[1]
		srv.ConfigPath = os.Args[2]
	}

	// Initialize and hydrate config
	if err := srv.Init(); err != nil {
		logger.Error("service init failed", "service", srv.Name, "id", srv.Id, "err", err)
		os.Exit(1)
	}
	if srv.Address == "" {
		srv.Address, _ = config.GetAddress()
	}

	// Register service and reflection
	storagepb.RegisterStorageServiceServer(srv.grpcServer, srv)
	reflection.Register(srv.grpcServer)

	// Start
	logger.Info("starting storage service",
		"name", srv.Name, "id", srv.Id, "addr", srv.Address, "port", srv.Port, "proxy", srv.Proxy)
	if err := srv.StartService(); err != nil {
		logger.Error("service start failed", "err", err)
		os.Exit(1)
	}
}
