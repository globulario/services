// Package main implements the Monitoring gRPC service wired for Globular.
// It provides structured logging with slog, safe --describe/--health
// handling before touching config/etcd, and preserves all public
// getters/setters and service lifecycle methods.
package main

import (
	"context"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/monitoring/monitoring_client"
	"github.com/globulario/services/golang/monitoring/monitoring_store"
	"github.com/globulario/services/golang/monitoring/monitoringpb"
	Utility "github.com/globulario/utility"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// -----------------------------------------------------------------------------
// Defaults & Logger
// -----------------------------------------------------------------------------

var (
	defaultPort       = 10019
	defaultProxy      = 10020
	allowAllOrigins   = true
	allowedOriginsStr = ""

	logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
)

// -----------------------------------------------------------------------------
// Types
// -----------------------------------------------------------------------------

// connection tracks a monitoring store endpoint.
type connection struct {
	Id   string                 // Connection id
	Host string                 // Host or IPv4
	Port int32                  // Port
	Type monitoringpb.StoreType // Currently Prometheus only
}

// server implements the Globular service + Monitoring fields.
type server struct {
	// Core metadata
	Id              string
	Mac             string
	Name            string
	Path            string
	Proto           string
	Port            int
	Proxy           int
	AllowAllOrigins bool
	AllowedOrigins  string
	Protocol        string
	Domain          string
	Address         string
	Description     string
	Keywords        []string
	Repositories    []string
	Discoveries     []string

	// TLS
	CertFile           string
	KeyFile            string
	CertAuthorityTrust string
	TLS                bool

	// Versioning / ops
	Version      string
	PublisherID  string
	KeepUpToDate bool
	Plaform      string
	Checksum     string
	KeepAlive    bool
	Permissions  []interface{}
	Dependencies []string
	Process      int
	ProxyProcess int
	ConfigPath   string
	LastError    string
	ModTime      int64
	State        string

	// Runtime
	grpcServer *grpc.Server

	// Monitoring connections / stores
	Connections map[string]connection
	stores      map[string]monitoring_store.Store
}

// -----------------------------------------------------------------------------
// Globular service contract (public prototypes preserved)
// -----------------------------------------------------------------------------

func (srv *server) GetConfigurationPath() string { return srv.ConfigPath }
func (srv *server) SetConfigurationPath(path string) { srv.ConfigPath = path }

func (srv *server) GetAddress() string { return srv.Address }
func (srv *server) SetAddress(address string) { srv.Address = address }

func (srv *server) GetProcess() int { return srv.Process }
func (srv *server) SetProcess(pid int) { srv.Process = pid }

func (srv *server) GetProxyProcess() int { return srv.ProxyProcess }
func (srv *server) SetProxyProcess(pid int) { srv.ProxyProcess = pid }

func (srv *server) GetState() string { return srv.State }
func (srv *server) SetState(state string) { srv.State = state }

func (srv *server) GetLastError() string { return srv.LastError }
func (srv *server) SetLastError(err string) { srv.LastError = err }

func (srv *server) SetModTime(modtime int64) { srv.ModTime = modtime }
func (srv *server) GetModTime() int64        { return srv.ModTime }

func (srv *server) GetId() string      { return srv.Id }
func (srv *server) SetId(id string)    { srv.Id = id }
func (srv *server) GetName() string    { return srv.Name }
func (srv *server) SetName(name string){ srv.Name = name }

func (srv *server) GetMac() string     { return srv.Mac }
func (srv *server) SetMac(mac string)  { srv.Mac = mac }

func (srv *server) GetDescription() string { return srv.Description }
func (srv *server) SetDescription(description string) { srv.Description = description }

func (srv *server) GetKeywords() []string { return srv.Keywords }
func (srv *server) SetKeywords(keywords []string) { srv.Keywords = keywords }

func (srv *server) Dist(path string) (string, error) { return globular.Dist(path, srv) }

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
func (srv *server) GetRepositories() []string { return srv.Repositories }
func (srv *server) SetRepositories(repositories []string) { srv.Repositories = repositories }
func (srv *server) GetDiscoveries() []string { return srv.Discoveries }
func (srv *server) SetDiscoveries(discoveries []string) { srv.Discoveries = discoveries }

func (srv *server) GetChecksum() string { return srv.Checksum }
func (srv *server) SetChecksum(checksum string) { srv.Checksum = checksum }

func (srv *server) GetPlatform() string { return srv.Plaform }
func (srv *server) SetPlatform(platform string) { srv.Plaform = platform }

func (srv *server) GetPath() string { return srv.Path }
func (srv *server) SetPath(path string) { srv.Path = path }

func (srv *server) GetProto() string { return srv.Proto }
func (srv *server) SetProto(proto string) { srv.Proto = proto }

func (srv *server) GetPort() int { return srv.Port }
func (srv *server) SetPort(port int) { srv.Port = port }

func (srv *server) GetProxy() int { return srv.Proxy }
func (srv *server) SetProxy(proxy int) { srv.Proxy = proxy }

func (srv *server) GetProtocol() string { return srv.Protocol }
func (srv *server) SetProtocol(protocol string) { srv.Protocol = protocol }

func (srv *server) GetAllowAllOrigins() bool { return srv.AllowAllOrigins }
func (srv *server) SetAllowAllOrigins(allowAllOrigins bool) { srv.AllowAllOrigins = allowAllOrigins }

func (srv *server) GetAllowedOrigins() string { return srv.AllowedOrigins }
func (srv *server) SetAllowedOrigins(allowedOrigins string) { srv.AllowedOrigins = allowedOrigins }

func (srv *server) GetDomain() string { return srv.Domain }
func (srv *server) SetDomain(domain string) { srv.Domain = domain }

func (srv *server) GetTls() bool { return srv.TLS }
func (srv *server) SetTls(hasTls bool) { srv.TLS = hasTls }

func (srv *server) GetCertAuthorityTrust() string { return srv.CertAuthorityTrust }
func (srv *server) SetCertAuthorityTrust(ca string) { srv.CertAuthorityTrust = ca }

func (srv *server) GetCertFile() string { return srv.CertFile }
func (srv *server) SetCertFile(certFile string) { srv.CertFile = certFile }

func (srv *server) GetKeyFile() string { return srv.KeyFile }
func (srv *server) SetKeyFile(keyFile string) { srv.KeyFile = keyFile }

func (srv *server) GetVersion() string { return srv.Version }
func (srv *server) SetVersion(version string) { srv.Version = version }

func (srv *server) GetPublisherID() string { return srv.PublisherID }
func (srv *server) SetPublisherID(PublisherID string) { srv.PublisherID = PublisherID }

func (srv *server) GetKeepUpToDate() bool { return srv.KeepUpToDate }
func (srv *server) SetKeepUptoDate(val bool) { srv.KeepUpToDate = val }

func (srv *server) GetKeepAlive() bool { return srv.KeepAlive }
func (srv *server) SetKeepAlive(val bool) { srv.KeepAlive = val }

func (srv *server) GetPermissions() []interface{} { return srv.Permissions }
func (srv *server) SetPermissions(permissions []interface{}) { srv.Permissions = permissions }

// Init creates/loads configuration and initializes gRPC.
func (srv *server) Init() error {
	srv.stores = make(map[string]monitoring_store.Store)
	srv.Connections = make(map[string]connection)

	if err := globular.InitService(srv); err != nil {
		return err
	}
	gs, err := globular.InitGrpcServer(srv)
	if err != nil {
		return err
	}
	srv.grpcServer = gs

	// Initialize stores for existing connections.
	for _, c := range srv.Connections {
		var store monitoring_store.Store
		var err error
		address := "http://" + c.Host + ":" + Utility.ToString(c.Port)

		if c.Type == monitoringpb.StoreType_PROMETHEUS {
			store, err = monitoring_store.NewPrometheusStore(address)
		}
		if err != nil {
			return err
		}
		if store == nil {
			return errors.New("fail to connect to store")
		}
		srv.stores[c.Id] = store
	}
	return nil
}

// Save persists configuration.
func (srv *server) Save() error { return globular.SaveService(srv) }

// StartService launches gRPC (and proxy if configured).
func (srv *server) StartService() error { return globular.StartService(srv, srv.grpcServer) }

// StopService gracefully stops gRPC.
func (srv *server) StopService() error { return globular.StopService(srv, srv.grpcServer) }

// Stop RPC.
func (srv *server) Stop(context.Context, *monitoringpb.StopRequest) (*monitoringpb.StopResponse, error) {
	return &monitoringpb.StopResponse{}, srv.StopService()
}

// -----------------------------------------------------------------------------
// main with --describe / --health
// -----------------------------------------------------------------------------

func main() {
	srv := new(server)

	// Fill ONLY fields that do NOT require config/etcd yet.
	srv.Name = string(monitoringpb.File_monitoring_proto.Services().Get(0).FullName())
	srv.Proto = monitoringpb.File_monitoring_proto.Path()
	srv.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	srv.Port = defaultPort
	srv.Proxy = defaultProxy
	srv.Protocol = "grpc"
	srv.PublisherID = "localhost"
	srv.Version = "0.0.1"
	srv.Permissions = make([]interface{}, 0)
	srv.Keywords = make([]string, 0)
	srv.Repositories = make([]string, 0)
	srv.Discoveries = make([]string, 0)
	srv.Dependencies = make([]string, 0)
	srv.AllowAllOrigins = allowAllOrigins
	srv.AllowedOrigins = allowedOriginsStr
	srv.Process = -1
	srv.ProxyProcess = -1
	srv.KeepAlive = true
	srv.KeepUpToDate = true

	// Register client ctor.
	Utility.RegisterFunction("NewMonitoringService_Client", monitoring_client.NewMonitoringService_Client)

	// ---- CLI handling BEFORE config access ----
	args := os.Args[1:]

	// Optional positional overrides (id, config path) if they don't start with '-'
	if len(args) == 1 && !strings.HasPrefix(args[0], "-") {
		srv.Id = args[0]
	} else if len(args) == 2 && !strings.HasPrefix(args[0], "-") && !strings.HasPrefix(args[1], "-") {
		srv.Id = args[0]
		srv.ConfigPath = args[1]
	}

	// Flags first (no etcd/config access here)
	for _, a := range args {
		switch strings.ToLower(a) {
		case "--describe":
			srv.Process = os.Getpid()
			srv.State = "starting"

			// Safe defaults for domain/address without etcd
			if v, ok := os.LookupEnv("GLOBULAR_DOMAIN"); ok && v != "" {
				srv.Domain = strings.ToLower(v)
			} else {
				srv.Domain = "localhost"
			}
			if v, ok := os.LookupEnv("GLOBULAR_ADDRESS"); ok && v != "" {
				srv.Address = strings.ToLower(v)
			} else {
				srv.Address = "localhost:" + Utility.ToString(srv.Port)
			}

			b, err := globular.DescribeJSON(srv)
			if err != nil {
				logger.Error("describe error", "service", srv.Name, "id", srv.Id, "err", err)
				os.Exit(2)
			}
			_, _ = os.Stdout.Write(b)
			_, _ = os.Stdout.Write([]byte("\n"))
			return

		case "--health":
			if srv.Port == 0 || srv.Name == "" {
				logger.Error("health error: uninitialized", "service", srv.Name, "port", srv.Port)
				os.Exit(2)
			}
			b, err := globular.HealthJSON(srv, &globular.HealthOptions{
				Timeout:     1500 * time.Millisecond,
				ServiceName: "",
			})
			if err != nil {
				logger.Error("health error", "service", srv.Name, "id", srv.Id, "err", err)
				os.Exit(2)
			}
			_, _ = os.Stdout.Write(b)
			_, _ = os.Stdout.Write([]byte("\n"))
			return
		}
	}

	// Now safe to access config (may read etcd / file fallback)
	if d, err := config.GetDomain(); err == nil && d != "" {
		srv.Domain = d
	}
	if a, err := config.GetAddress(); err == nil && strings.TrimSpace(a) != "" {
		srv.Address = a
	}

	// Init service
	start := time.Now()
	if err := srv.Init(); err != nil {
		logger.Error("service init failed", "service", srv.Name, "id", srv.Id, "err", err)
		os.Exit(1)
	}

	// Register RPC server + reflection
	monitoringpb.RegisterMonitoringServiceServer(srv.grpcServer, srv)
	reflection.Register(srv.grpcServer)

	logger.Info("service ready",
		"service", srv.Name,
		"port", srv.Port,
		"proxy", srv.Proxy,
		"protocol", srv.Protocol,
		"domain", srv.Domain,
		"listen_ms", time.Since(start).Milliseconds(),
	)

	if err := srv.StartService(); err != nil {
		logger.Error("service start failed", "service", srv.Name, "id", srv.Id, "err", err)
		os.Exit(1)
	}
}
