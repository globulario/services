// Package main implements the Repository gRPC service wired for Globular.
// It provides structured logging via slog, clean getters/setters that satisfy
// Globular's service contract, and CLI utilities: --describe and --health.
package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/globular_client"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/repository/repository_client"
	"github.com/globulario/services/golang/repository/repositorypb"
	"github.com/globulario/services/golang/resource/resource_client"
	"github.com/globulario/services/golang/resource/resourcepb"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// -----------------------------------------------------------------------------
// Defaults & CORS
// -----------------------------------------------------------------------------

var (
	defaultPort  = 10000
	defaultProxy = defaultPort + 1

	allowAllOrigins   = true // allow all by default
	allowedOriginsStr = ""   // comma-separated list (if not allowAllOrigins)
)

// -----------------------------------------------------------------------------
// Server
// -----------------------------------------------------------------------------

// server implements Globular service plumbing + Repository RPCs (see repository.go).
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

	// Ops / policy
	AllowAllOrigins bool
	AllowedOrigins  string
	KeepUpToDate    bool
	Plaform         string // NOTE: field spelling preserved for compatibility
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

	// Runtime
	grpcServer *grpc.Server

	// Service-specific
	Root string // base data dir (packages-repository lives under this)
}

// SetPermissions implements globular_service.Service.
func (srv *server) SetPermissions(permissions []interface{}) {
	srv.Permissions = permissions
}

// -----------------------------------------------------------------------------
// Globular service contract (documented getters/setters)
// -----------------------------------------------------------------------------

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

// GetMac returns the MAC address of the host (if set by the platform).
func (srv *server) GetMac() string { return srv.Mac }

// SetMac sets the MAC address of the host.
func (srv *server) SetMac(mac string) { srv.Mac = mac }

// GetDescription returns the service description.
func (srv *server) GetDescription() string { return srv.Description }

// SetDescription sets the service description.
func (srv *server) SetDescription(description string) { srv.Description = description }

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

// GetPermissions returns the permissions associated with the service.
func (srv *server) GetPermissions() []interface{} {
	return srv.Permissions
}

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

func (srv *server) RolesDefault() []resourcepb.Role {
	domain, _ := config.GetDomain()

	return []resourcepb.Role{
		{
			Id:          "role:repo.viewer",
			Name:        "Repository Viewer",
			Domain:      domain,
			Description: "Read/download access to published bundles.",
			Actions: []string{
				"/repository.PackageRepository/DownloadBundle",
			},
			TypeName: "resource.Role",
		},
		{
			Id:          "role:repo.publisher",
			Name:        "Repository Publisher",
			Domain:      domain,
			Description: "Can upload (publish) bundles to an organization namespace and download.",
			Actions: []string{
				"/repository.PackageRepository/UploadBundle",
				"/repository.PackageRepository/DownloadBundle",
			},
			TypeName: "resource.Role",
		},
		{
			Id:          "role:repo.admin",
			Name:        "Repository Admin",
			Domain:      domain,
			Description: "Full control over repository operations.",
			Actions: []string{
				"/repository.PackageRepository/UploadBundle",
				"/repository.PackageRepository/DownloadBundle",
			},
			TypeName: "resource.Role",
		},
	}
}

// -----------------------------------------------------------------------------
// Lifecycle
// -----------------------------------------------------------------------------

// Init initializes the service configuration and gRPC server.
func (srv *server) Init() error {
	// Create or load the service configuration via Globular.
	if err := globular.InitService(srv); err != nil {
		return err
	}

	// If your Globular requires interceptors:
	gs, err := globular.InitGrpcServer(srv)
	if err != nil {
		return err
	}
	srv.grpcServer = gs
	return nil
}

// Save persists the current configuration to disk.
func (srv *server) Save() error { return globular.SaveService(srv) }

// StartService begins serving gRPC (and proxy if configured).
func (srv *server) StartService() error { return globular.StartService(srv, srv.grpcServer) }

// StopService gracefully stops the running gRPC server.
func (srv *server) StopService() error { return globular.StopService(srv, srv.grpcServer) }

// -----------------------------------------------------------------------------
// Resource manager helpers (Repository needs these)
// -----------------------------------------------------------------------------

// getResourceClient returns a connected Resource service client.
func (srv *server) getResourceClient() (*resource_client.Resource_Client, error) {
	address, _ := config.GetAddress()
	Utility.RegisterFunction("NewResourceService_Client", resource_client.NewResourceService_Client)
	client, err := globular_client.GetClient(address, "resource.ResourceService", "NewResourceService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*resource_client.Resource_Client), nil
}

// getPackageBundleChecksum returns the checksum stored for a bundle id.
func (srv *server) getPackageBundleChecksum(id string) (string, error) {
	resourceClient, err := srv.getResourceClient()
	if err != nil {
		return "", err
	}
	return resourceClient.GetPackageBundleChecksum(id)
}

// setPackageBundle persists bundle metadata in the Resource service.
func (srv *server) setPackageBundle(checksum, platform string, size int32, modified int64, descriptor *resourcepb.PackageDescriptor) error {
	resourceClient, err := srv.getResourceClient()
	if err != nil {
		return err
	}
	if err := resourceClient.SetPackageBundle(checksum, platform, size, modified, descriptor); err != nil {
		return err
	}
	return nil
}

// -----------------------------------------------------------------------------
// Log service helpers (kept for compatibility with your existing logging)
// -----------------------------------------------------------------------------

// -----------------------------------------------------------------------------
// CLI / entrypoint
// -----------------------------------------------------------------------------

var logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
	Level: slog.LevelInfo,
}))

func main() {
	s := new(server)

	// Fill ONLY fields that do NOT call into config/etcd yet.
	s.Name = string(repositorypb.File_repository_proto.Services().Get(0).FullName())
	s.Proto = repositorypb.File_repository_proto.Path()
	s.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	s.Port = defaultPort
	s.Proxy = defaultProxy
	s.Protocol = "grpc"
	s.Version = "0.0.1"
	s.PublisherID = "localhost"
	s.Description = "Repository service where packages are stored."
	s.Keywords = []string{"Repo", "Repository", "Package", "Service"}
	s.Repositories = make([]string, 0)
	s.Discoveries = make([]string, 0)
	s.Dependencies = []string{
		"resource.ResourceService",
		"log.LogService",
		"applications_manager.ApplicationManagerService",
	}
	// In your repository service init/constructor:
	s.Permissions = []interface{}{
		// ---- Upload a package bundle (publishing to an organization namespace)
		map[string]interface{}{
			"action":     "/repository.PackageRepository/UploadBundle",
			"permission": "write",
			"resources": []interface{}{
				// UploadBundleRequest.organization — who owns the repo namespace
				map[string]interface{}{"index": 0, "field": "Organization", "permission": "write"},
			},
		},

		// ---- Download a package bundle (read access to publisher namespace / platform)
		map[string]interface{}{
			"action":     "/repository.PackageRepository/DownloadBundle",
			"permission": "read",
			"resources": []interface{}{
				// DownloadBundleRequest.descriptor.publisher_id — publisher namespace
				map[string]interface{}{"index": 0, "field": "Descriptor.PublisherID", "permission": "read"},
				// DownloadBundleRequest.platform — optional platform gate
				map[string]interface{}{"index": 0, "field": "Platform", "permission": "read"},
			},
		},
	}

	s.Process = -1
	s.ProxyProcess = -1
	s.AllowAllOrigins = allowAllOrigins
	s.AllowedOrigins = allowedOriginsStr
	s.KeepAlive = true
	s.KeepUpToDate = true

	// Default data dir for repo storage
	s.Root = config.GetDataDir()

	// ---- CLI flags handled BEFORE any call that might touch etcd ----
	args := os.Args[1:]
	if len(args) == 0 {
		s.Id = Utility.GenerateUUID(s.Name + ":" + s.Address)
		allocator, err := config.NewDefaultPortAllocator()
		if err != nil {
			logger.Error("fail to create port allocator", "error", err)
			os.Exit(1)
		}
		p, err := allocator.Next(s.Id)
		if err != nil {
			logger.Error("fail to allocate port", "error", err)
			os.Exit(1)
		}
		s.Port = p
	}

	for _, a := range args {
		switch strings.ToLower(a) {
		case "--describe":
			// best-effort runtime fields without hitting etcd
			s.Process = os.Getpid()
			s.State = "starting"

			// Provide harmless defaults for Domain/Address that don’t need etcd
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
			b, err := globular.HealthJSON(s, &globular.HealthOptions{
				Timeout:     1500 * time.Millisecond,
				ServiceName: "", // overall
			})
			if err != nil {
				logger.Error("health error", "service", s.Name, "id", s.Id, "err", err)
				os.Exit(2)
			}
			_, _ = os.Stdout.Write(b)
			_, _ = os.Stdout.Write([]byte("\n"))
			return
		case "--debug":
			logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
		case "--help", "-h", "/?":
			printUsage()
			return
		case "--version", "-v":
			fmt.Println(s.Version)
			return
		default:
			// skip unknown flags for now (e.g. positional args)
		}
	}

	// Optional positional args (unchanged)
	if len(args) == 1 && !strings.HasPrefix(args[0], "-") {
		s.Id = args[0]
	} else if len(args) == 2 && !strings.HasPrefix(args[0], "-") && !strings.HasPrefix(args[1], "-") {
		s.Id = args[0]
		s.ConfigPath = args[1]
	}

	// Safe to read local config now (file or etcd fallback handled by config pkg)
	if d, err := config.GetDomain(); err == nil {
		s.Domain = d
	} else {
		s.Domain = "localhost"
	}
	if a, err := config.GetAddress(); err == nil && strings.TrimSpace(a) != "" {
		s.Address = a
	}

	// Register client ctor for Repository (kept for compatibility)
	Utility.RegisterFunction("NewRepositoryService_Client", repository_client.NewRepositoryService_Client)

	start := time.Now()
	if err := s.Init(); err != nil {
		logger.Error("service init failed", "service", s.Name, "id", s.Id, "err", err)
		os.Exit(1)
	}

	repositorypb.RegisterPackageRepositoryServer(s.grpcServer, s)
	reflection.Register(s.grpcServer)

	logger.Info("service ready",
		"service", s.Name,
		"port", s.Port,
		"proxy", s.Proxy,
		"protocol", s.Protocol,
		"domain", s.Domain,
		"listen_ms", time.Since(start).Milliseconds())

	if err := s.StartService(); err != nil {
		logger.Error("service start failed", "service", s.Name, "id", s.Id, "err", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  repository_server [service_id] [config_path]")
	fmt.Println("Options:")
	fmt.Println("  --describe    Print service metadata as JSON and exit")
	fmt.Println("  --health      Print service health as JSON and exit")
	fmt.Println("Examples:")
	fmt.Println("  repository_server my-repo-id /etc/globular/repository/config.json")
	fmt.Println("  repository_server --describe")
	fmt.Println("  repository_server --health")
}
