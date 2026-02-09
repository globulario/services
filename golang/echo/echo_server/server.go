// Package main provides a minimal Echo gRPC service wired for Globular.
// It exposes a service with structured logging via slog and a clean,
// well-documented server type that satisfies Globular's service contract.
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

	"github.com/globulario/services/golang/echo/echo_client"
	"github.com/globulario/services/golang/echo/echopb"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/resource/resourcepb"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// Default ports.
var (
	defaultPort  = 10000
	defaultProxy = defaultPort + 1

	// Allow all origins by default.
	allowAllOrigins   = true
	allowedOriginsStr = ""
)

// server implements the Globular service contract and Echo RPC handlers.
//
// Phase 1 Refactoring Complete:
// - Business logic extracted to handlers.go (pure functions)
// - Lifecycle management extracted to lifecycle.go (Start/Stop/Ready/Health)
// - Config operations extracted to config.go (load/save/validate)
// - Main initialization simplified in server.go
//
// This struct retains all fields and getter/setters required by the Globular
// service framework. These fields implement the service contract and MUST remain
// for compatibility with globular.InitService(), globular.SaveService(), etc.
type server struct {
	// --- Service Identity ---
	Id          string
	Mac         string
	Name        string
	Domain      string
	Address     string
	Path        string   // Executable path
	Proto       string   // .proto file path
	Version     string
	PublisherID string
	Description string
	Keywords    []string

	// --- Network Configuration ---
	Port     int
	Proxy    int
	Protocol string

	// --- Service Discovery ---
	Repositories []string
	Discoveries  []string

	// --- Policy & Operations ---
	AllowAllOrigins bool
	AllowedOrigins  string
	KeepUpToDate    bool
	KeepAlive       bool
	Plaform         string   // Note: typo preserved for compatibility
	Checksum        string
	Permissions     []any    // Action permissions
	Dependencies    []string

	// --- Runtime State ---
	Process      int    // PID or -1
	ProxyProcess int    // Proxy PID or -1
	ConfigPath   string // Path to config file
	LastError    string
	State        string // e.g., "running", "stopped"
	ModTime      int64  // Unix timestamp

	// --- TLS Configuration ---
	TLS                bool
	CertFile           string
	KeyFile            string
	CertAuthorityTrust string

	// --- gRPC Runtime ---
	grpcServer *grpc.Server
}

// --- Globular service contract (getters/setters) ---

// GetConfigurationPath returns the path to the service configuration file.
func (srv *server) GetConfigurationPath() string { return srv.ConfigPath }

// SetConfigurationPath sets the path to the service configuration file.
func (srv *server) SetConfigurationPath(path string) { srv.ConfigPath = path }

// GetAddress returns the HTTP address where /config can be reached.
func (srv *server) GetAddress() string { return srv.Address }

// SetAddress sets the HTTP address where /config can be reached.
func (srv *server) SetAddress(address string) { srv.Address = address }

// GetProcess returns the process id of the service, or -1 if not started.
func (srv *server) GetProcess() int { return srv.Process }

// SetProcess records the process id of the service.
func (srv *server) SetProcess(pid int) { srv.Process = pid }

// GetProxyProcess returns the reverse-proxy process id, or -1 if not started.
func (srv *server) GetProxyProcess() int { return srv.ProxyProcess }

// SetProxyProcess records the reverse-proxy process id.
func (srv *server) SetProxyProcess(pid int) { srv.ProxyProcess = pid }

// GetState returns the current service state (e.g., "running").
func (srv *server) GetState() string { return srv.State }

// SetState updates the current service state.
func (srv *server) SetState(state string) { srv.State = state }

// GetLastError returns the last error message recorded by the service.
func (srv *server) GetLastError() string { return srv.LastError }

// SetLastError records the last error message.
func (srv *server) SetLastError(err string) { srv.LastError = err }

// SetModTime sets the last modification time (unix seconds).
func (srv *server) SetModTime(modtime int64) { srv.ModTime = modtime }

// GetModTime returns the last modification time (unix seconds).
func (srv *server) GetModTime() int64 { return srv.ModTime }

// GetId returns the unique id of this service instance.
func (srv *server) GetId() string { return srv.Id }

// SetId sets the unique id of this service instance.
func (srv *server) SetId(id string) { srv.Id = id }

// GetName returns the gRPC service name.
func (srv *server) GetName() string { return srv.Name }

// SetName sets the gRPC service name.
func (srv *server) SetName(name string) { srv.Name = name }

// GetDescription returns the service description.
func (srv *server) GetDescription() string { return srv.Description }

// SetDescription sets the service description.
func (srv *server) SetDescription(description string) { srv.Description = description }

// GetMac returns the MAC address of the host (if set by the platform).
func (srv *server) GetMac() string { return srv.Mac }

// SetMac sets the MAC address of the host.
func (srv *server) SetMac(mac string) { srv.Mac = mac }

// GetKeywords returns the service keywords.
func (srv *server) GetKeywords() []string { return srv.Keywords }

// SetKeywords sets the service keywords.
func (srv *server) SetKeywords(keywords []string) { srv.Keywords = keywords }

// GetRepositories returns repositories associated with the service.
func (srv *server) GetRepositories() []string { return srv.Repositories }

// SetRepositories sets repositories associated with the service.
func (srv *server) SetRepositories(repositories []string) { srv.Repositories = repositories }

// GetDiscoveries returns discovery endpoints for the service.
func (srv *server) GetDiscoveries() []string { return srv.Discoveries }

// SetDiscoveries sets discovery endpoints for the service.
func (srv *server) SetDiscoveries(discoveries []string) { srv.Discoveries = discoveries }

// Dist packages (distributes) the service into the given path using Globular.
func (srv *server) Dist(path string) (string, error) { return globular.Dist(path, srv) }

// GetDependencies returns the list of dependent services.
func (srv *server) GetDependencies() []string {
	if srv.Dependencies == nil {
		srv.Dependencies = make([]string, 0)
	}
	return srv.Dependencies
}

// SetDependency appends a dependency if it is not already present.
func (srv *server) SetDependency(dependency string) {
	if srv.Dependencies == nil {
		srv.Dependencies = make([]string, 0)
	}
	if !Utility.Contains(srv.Dependencies, dependency) {
		srv.Dependencies = append(srv.Dependencies, dependency)
	}
}

// GetChecksum returns the binary checksum.
func (srv *server) GetChecksum() string { return srv.Checksum }

// SetChecksum sets the binary checksum.
func (srv *server) SetChecksum(checksum string) { srv.Checksum = checksum }

// GetPlatform returns the service platform (e.g., "linux/amd64").
func (srv *server) GetPlatform() string { return srv.Plaform }

// SetPlatform sets the service platform (e.g., "linux/amd64").
func (srv *server) SetPlatform(platform string) { srv.Plaform = platform }

// GetPath returns the executable path.
func (srv *server) GetPath() string { return srv.Path }

// SetPath sets the executable path.
func (srv *server) SetPath(path string) { srv.Path = path }

// GetProto returns the path to the .proto file.
func (srv *server) GetProto() string { return srv.Proto }

// SetProto sets the path to the .proto file.
func (srv *server) SetProto(proto string) { srv.Proto = proto }

// GetPort returns the gRPC port.
func (srv *server) GetPort() int { return srv.Port }

// SetPort sets the gRPC port.
func (srv *server) SetPort(port int) { srv.Port = port }

// GetProxy returns the reverse proxy port (for gRPC-Web).
func (srv *server) GetProxy() int { return srv.Proxy }

// SetProxy sets the reverse proxy port (for gRPC-Web).
func (srv *server) SetProxy(proxy int) { srv.Proxy = proxy }

// GetGrpcServer returns the gRPC server instance (for lifecycle management).
func (srv *server) GetGrpcServer() *grpc.Server { return srv.grpcServer }

// GetProtocol returns the network protocol (e.g., "grpc", "tls", "https").
func (srv *server) GetProtocol() string { return srv.Protocol }

// SetProtocol sets the network protocol (e.g., "grpc", "tls", "https").
func (srv *server) SetProtocol(protocol string) { srv.Protocol = protocol }

// GetAllowAllOrigins returns whether all origins are allowed.
func (srv *server) GetAllowAllOrigins() bool { return srv.AllowAllOrigins }

// SetAllowAllOrigins toggles whether all origins are allowed.
func (srv *server) SetAllowAllOrigins(allowAllOrigins bool) { srv.AllowAllOrigins = allowAllOrigins }

// GetAllowedOrigins returns the comma-separated list of allowed origins.
func (srv *server) GetAllowedOrigins() string { return srv.AllowedOrigins }

// SetAllowedOrigins sets the comma-separated list of allowed origins.
func (srv *server) SetAllowedOrigins(allowedOrigins string) { srv.AllowedOrigins = allowedOrigins }

// GetDomain returns the configured domain (ip or DNS name).
func (srv *server) GetDomain() string { return srv.Domain }

// SetDomain sets the configured domain (ip or DNS name).
func (srv *server) SetDomain(domain string) { srv.Domain = domain }

// GetTls returns true when TLS is enabled.
func (srv *server) GetTls() bool { return srv.TLS }

// SetTls enables or disables TLS.
func (srv *server) SetTls(hasTls bool) { srv.TLS = hasTls }

// GetCertAuthorityTrust returns the CA bundle path.
func (srv *server) GetCertAuthorityTrust() string { return srv.CertAuthorityTrust }

// SetCertAuthorityTrust sets the CA bundle path.
func (srv *server) SetCertAuthorityTrust(ca string) { srv.CertAuthorityTrust = ca }

// GetCertFile returns the TLS certificate path.
func (srv *server) GetCertFile() string { return srv.CertFile }

// SetCertFile sets the TLS certificate path.
func (srv *server) SetCertFile(certFile string) { srv.CertFile = certFile }

// GetKeyFile returns the TLS private key path.
func (srv *server) GetKeyFile() string { return srv.KeyFile }

// SetKeyFile sets the TLS private key path.
func (srv *server) SetKeyFile(keyFile string) { srv.KeyFile = keyFile }

// GetVersion returns the service version.
func (srv *server) GetVersion() string { return srv.Version }

// SetVersion sets the service version.
func (srv *server) SetVersion(version string) { srv.Version = version }

// GetPublisherID returns the publisher ID.
func (srv *server) GetPublisherID() string { return srv.PublisherID }

// SetPublisherID sets the publisher ID.
func (srv *server) SetPublisherID(PublisherID string) { srv.PublisherID = PublisherID }

// GetKeepUpToDate returns whether auto-updates are enabled.
func (srv *server) GetKeepUpToDate() bool { return srv.KeepUpToDate }

// SetKeepUptoDate toggles auto-updates.
func (srv *server) SetKeepUptoDate(val bool) { srv.KeepUpToDate = val }

// GetKeepAlive returns whether the service should be kept alive by the supervisor.
func (srv *server) GetKeepAlive() bool { return srv.KeepAlive }

// SetKeepAlive toggles keep-alive behavior.
func (srv *server) SetKeepAlive(val bool) { srv.KeepAlive = val }

// GetPermissions returns the action permissions configured for this service.
func (srv *server) GetPermissions() []any { return srv.Permissions }

// SetPermissions sets the action permissions for this service.
func (srv *server) SetPermissions(permissions []any) { srv.Permissions = permissions }

// Init initializes the service configuration and gRPC server.
func (srv *server) Init() error {
	// Create or load the service configuration via Globular.
	if err := globular.InitService(srv); err != nil {
		return err
	}

	gs, err := globular.InitGrpcServer(srv)
	if err != nil {
		return err
	}
	srv.grpcServer = gs
	return nil
}

// Save persists the current configuration to disk.
func (srv *server) Save() error { return globular.SaveService(srv) }

// NOTE: StartService() and StopService() moved to lifecycle.go in Phase 1 Step 3

// Stop stops the service via gRPC.
func (srv *server) Stop(ctx context.Context, _ *echopb.StopRequest) (*echopb.StopResponse, error) {
	return &echopb.StopResponse{}, srv.StopService()
}

// --- main entrypoint ---

// Version information (set via ldflags during build)
var (
	Version   = "0.0.1"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

var logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
	Level: slog.LevelInfo,
}))

// RolesDefault returns default roles for the service (required by Globular).
func (srv *server) RolesDefault() []resourcepb.Role {
	return []resourcepb.Role{}
}

// -----------------------------------------------------------------------------
// Lifecycle methods (Phase 2 Step 2)
// -----------------------------------------------------------------------------

// StartService starts the gRPC server (required by LifecycleService interface).
func (srv *server) StartService() error {
	return globular.StartService(srv, srv.grpcServer)
}

// StopService stops the gRPC server (required by LifecycleService interface).
func (srv *server) StopService() error {
	return globular.StopService(srv, srv.grpcServer)
}

// initializeServerDefaults sets up the server with default values before config loading.
// This MUST NOT touch etcd or any external config - only local defaults.
func initializeServerDefaults() *server {
	srv := new(server)

	// Basic identity
	srv.Name = string(echopb.File_echo_proto.Services().Get(0).FullName())
	srv.Proto = echopb.File_echo_proto.Path()
	srv.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	srv.Version = Version
	srv.PublisherID = "localhost"
	srv.Description = "Echo service for testing and service health verification"
	srv.Keywords = []string{"echo", "test", "health", "ping", "diagnostic"}

	// Network defaults
	srv.Port = defaultPort
	srv.Proxy = defaultProxy
	srv.Protocol = "grpc"

	// CORS defaults
	srv.AllowAllOrigins = allowAllOrigins
	srv.AllowedOrigins = allowedOriginsStr

	// Lifecycle defaults
	srv.KeepAlive = true
	srv.KeepUpToDate = true
	srv.Process = -1
	srv.ProxyProcess = -1

	// Initialize empty slices
	srv.Repositories = make([]string, 0)
	srv.Discoveries = make([]string, 0)
	srv.Dependencies = make([]string, 0)
	srv.Permissions = make([]any, 0)

	return srv
}

// -----------------------------------------------------------------------------
// Helper functions for main() (Phase 2 Step 1)
// -----------------------------------------------------------------------------
//
// NOTE: Common CLI helper functions moved to globular_service/cli_helpers.go:
// - HandleInformationalFlags, HandleDescribeFlag, HandleHealthFlag
// - ParsePositionalArgs, AllocatePortIfNeeded, LoadRuntimeConfig
//
// Service-specific functions kept here:
// - initializeServerDefaults (service-specific defaults and permissions)
// - setupGrpcService (service-specific gRPC registration)

// setupGrpcService registers the Echo service with the gRPC server.
func setupGrpcService(srv *server) {
	echopb.RegisterEchoServiceServer(srv.grpcServer, srv)
	reflection.Register(srv.grpcServer)
}

// main configures and starts the Echo service.
// Phase 2: Modern CLI with flag package.
func main() {
	srv := initializeServerDefaults()

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
		data, _ := json.MarshalIndent(srv, "", "  ")
		fmt.Println(string(data))
		return
	}

	if *showHealth {
		health := map[string]interface{}{
			"service": srv.Name,
			"status":  "healthy",
			"version": srv.Version,
		}
		data, _ := json.MarshalIndent(health, "", "  ")
		fmt.Println(string(data))
		return
	}

	args := flag.Args()
	if err := globular.AllocatePortIfNeeded(srv, args); err != nil {
		logger.Error("port allocation failed", "error", err)
		os.Exit(1)
	}

	globular.ParsePositionalArgs(srv, args)
	globular.LoadRuntimeConfig(srv)

	Utility.RegisterFunction("NewEchoService_Client", echo_client.NewEchoService_Client)

	logger.Info("starting echo service", "service", srv.Name, "version", srv.Version, "domain", srv.Domain)

	start := time.Now()
	if err := srv.Init(); err != nil {
		logger.Error("service init failed", "service", srv.Name, "id", srv.Id, "err", err)
		os.Exit(1)
	}
	logger.Info("service initialized", "duration_ms", time.Since(start).Milliseconds())

	setupGrpcService(srv)
	logger.Debug("gRPC handlers registered")

	logger.Info("service ready", "service", srv.Name, "version", srv.Version, "port", srv.Port, "domain", srv.Domain, "startup_ms", time.Since(start).Milliseconds())

	lm := globular.NewLifecycleManager(srv, logger)
	if err := lm.Start(); err != nil {
		logger.Error("service start failed", "service", srv.Name, "id", srv.Id, "err", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Echo Service - Testing and health verification")
	fmt.Println()
	fmt.Println("USAGE:")
	fmt.Println("  echo-service [OPTIONS] [id] [config_path]")
	fmt.Println()
	fmt.Println("OPTIONS:")
	fmt.Println("  --debug       Enable debug logging")
	fmt.Println("  --describe    Print service description as JSON and exit")
	fmt.Println("  --health      Print service health status as JSON and exit")
	fmt.Println("  --version     Print version information as JSON and exit")
	fmt.Println("  --help        Show this help message and exit")
	fmt.Println()
	fmt.Println("FEATURES:")
	fmt.Println("  • Simple echo/ping functionality")
	fmt.Println("  • Service health verification")
	fmt.Println("  • Testing and diagnostics")
	fmt.Println()
	fmt.Println("EXAMPLES:")
	fmt.Println("  echo-service")
	fmt.Println("  echo-service --version")
	fmt.Println("  echo-service --debug")
}

func printVersion() {
	info := map[string]string{
		"service":    "echo",
		"version":    Version,
		"build_time": BuildTime,
		"git_commit": GitCommit,
	}
	data, _ := json.MarshalIndent(info, "", "  ")
	fmt.Println(string(data))
}
