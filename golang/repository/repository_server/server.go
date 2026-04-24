// Package main implements the Repository gRPC service wired for Globular.
// It provides structured logging via slog, clean getters/setters that satisfy
// Globular's service contract, and CLI utilities: --describe and --health.
package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/globular_client"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/interceptors"
	"github.com/globulario/services/golang/policy"
	"github.com/globulario/services/golang/repository/repository_client"
	"github.com/globulario/services/golang/repository/repositorypb"
	"github.com/globulario/services/golang/resource/resource_client"
	"github.com/globulario/services/golang/resource/resourcepb"
	"github.com/globulario/services/golang/storage_backend"
	"github.com/globulario/services/golang/workflow"
	"github.com/minio/minio-go/v7"
	"github.com/minio/minio-go/v7/pkg/credentials"
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

// server implements the Repository gRPC service with Globular lifecycle management.
//
// Phase 1 Refactoring Complete:
// - Config component: Clean configuration management (config.go)
// - Handlers component: Pure business logic for package management (handlers.go)
// - Lifecycle component: Service lifecycle with Start/Stop/Ready/Health (lifecycle.go)
// - Main cleanup: Simplified from 175 to 47 lines via helper functions
//
// The server struct contains all fields required by the Globular service contract,
// plus Repository-specific fields (Root). Business logic is in handlers.go.
type server struct {
	// --- Core Identity ---
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

	// --- Service Metadata ---
	Description  string
	Keywords     []string
	Repositories []string
	Discoveries  []string
	Dependencies []string

	// --- Policy & Operations ---
	AllowAllOrigins bool
	AllowedOrigins  string
	KeepUpToDate    bool
	KeepAlive       bool
	Permissions     []any

	// --- Runtime State ---
	Process      int
	ProxyProcess int
	ConfigPath   string
	LastError    string
	State        string
	ModTime      int64

	// --- Platform Metadata ---
	Plaform  string // NOTE: field spelling preserved for proto compatibility
	Checksum string

	// --- gRPC Implementation ---
	repositorypb.UnimplementedPackageRepositoryServer
	grpcServer *grpc.Server

	// --- TLS Configuration ---
	TLS                bool
	CertFile           string
	KeyFile            string
	CertAuthorityTrust string

	// --- Repository-Specific Fields ---
	Root              string                   // Base data directory (artifacts/ lives under this)
	GCRetentionWindow int                      // Number of PUBLISHED builds per series kept from GC (default 3)
	MinioConfig *config.MinioProxyConfig  // MinIO config from etcd (required in multi-node)
	minioClient *minio.Client
	storage     storage_backend.Storage
	cache       *manifestCache            // in-memory TTL cache for manifest reads
	scylla      manifestLedger            // ScyllaDB manifest metadata store (nil until connected)
	depHealth   *depHealthWatchdog        // dependency health monitor

	// --- Workflow tracing ---
	workflowRec *workflow.Recorder
}

// SetPermissions implements globular_service.Service.
func (srv *server) SetPermissions(permissions []any) {
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
func (srv *server) GetPermissions() []any {
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

// RolesDefault returns an empty set — roles are defined externally in
// cluster-roles.json and per-service policy files.
func (srv *server) RolesDefault() []resourcepb.Role {
	return []resourcepb.Role{}
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

	// DownloadArtifact is read-only and protected by cluster_id validation.
	// Node-agents call it during autonomous plan execution without user tokens.
	interceptors.AllowUnauthenticated(
		"/repository.PackageRepository/DownloadArtifact",
	)

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

// NOTE: StartService() and StopService() moved to lifecycle.go in Phase 1 Step 3

// -----------------------------------------------------------------------------
// Resource manager helpers (Repository needs these)
// -----------------------------------------------------------------------------

// getResourceClient returns a connected Resource service client.
// Uses direct service address (not gateway) so the call carries mTLS auth
// and passes the cluster_id interceptor check.
func (srv *server) getResourceClient() (*resource_client.Resource_Client, error) {
	// Prefer direct address from etcd registry — avoids Envoy TLS termination
	// which strips mTLS identity and triggers cluster_id enforcement.
	directAddr := config.ResolveLocalServiceAddr("resource.ResourceService")
	if directAddr == "" {
		directAddr, _ = config.GetAddress()
	}
	Utility.RegisterFunction("NewResourceService_Client", resource_client.NewResourceService_Client)
	client, err := globular_client.GetClient(directAddr, "resource.ResourceService", "NewResourceService_Client")
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
// Storage backend (MinIO or local filesystem)
// -----------------------------------------------------------------------------

const minioContractPath = "/var/lib/globular/objectstore/minio.json"
const minioCredentialsPath = "/var/lib/globular/minio/credentials"

func (srv *server) minioEnabled() bool {
	return srv.MinioConfig != nil && srv.MinioConfig.Endpoint != "" && srv.MinioConfig.Bucket != ""
}

func (srv *server) ensureMinioClient() error {
	if srv.minioClient != nil {
		return nil
	}
	cfg := srv.MinioConfig
	auth := cfg.Auth
	if auth == nil {
		auth = &config.MinioProxyAuth{Mode: config.MinioProxyAuthModeNone}
	}
	var creds *credentials.Credentials
	switch auth.Mode {
	case config.MinioProxyAuthModeAccessKey:
		creds = credentials.NewStaticV4(auth.AccessKey, auth.SecretKey, "")
	case config.MinioProxyAuthModeFile:
		data, err := os.ReadFile(auth.CredFile)
		if err != nil {
			return fmt.Errorf("read minio credentials file: %w", err)
		}
		parts := strings.Split(strings.TrimSpace(string(data)), ":")
		if len(parts) != 2 {
			return fmt.Errorf("invalid minio credentials file format")
		}
		creds = credentials.NewStaticV4(parts[0], parts[1], "")
	default:
		creds = credentials.NewStaticV4("", "", "")
	}
	opts := &minio.Options{Creds: creds, Secure: cfg.Secure}
	// Always install the cluster DNS dialer so *.globular.internal names
	// resolve via Globular DNS (system resolver has no knowledge of them).
	transport := &http.Transport{DialContext: config.ClusterDialContext}
	if cfg.Secure {
		tlsCfg, err := buildMinioTLSConfig(cfg)
		if err != nil {
			return fmt.Errorf("build minio TLS config: %w", err)
		}
		if tlsCfg != nil {
			transport.TLSClientConfig = tlsCfg
		}
	}
	opts.Transport = transport
	client, err := minio.New(cfg.Endpoint, opts)
	if err != nil {
		return err
	}
	srv.minioClient = client
	return nil
}

// buildMinioTLSConfig returns a tls.Config for the MinIO endpoint.
// If CABundlePath is set, it is loaded for server-cert verification.
// For loopback endpoints with no CA bundle, InsecureSkipVerify is used
// (acceptable because traffic is local-only).
func buildMinioTLSConfig(cfg *config.MinioProxyConfig) (*tls.Config, error) {
	// Loopback or same-host endpoints skip verification — MinIO is always
	// co-located on the same node, traffic never leaves the machine.
	host, _, _ := net.SplitHostPort(cfg.Endpoint)
	if host == "127.0.0.1" || host == "::1" || host == "localhost" || isLocalIP(host) {
		return &tls.Config{InsecureSkipVerify: true}, nil //nolint:gosec // local only
	}
	caPath := cfg.CABundlePath
	if caPath == "" {
		// Fallback: try the well-known cluster CA path. Without this,
		// secure connections to MinIO fail with "certificate signed by
		// unknown authority" when the etcd config doesn't include caBundlePath.
		const defaultCAPath = "/var/lib/globular/pki/ca.pem"
		if _, err := os.Stat(defaultCAPath); err == nil {
			caPath = defaultCAPath
		}
	}
	if caPath != "" {
		caCert, err := os.ReadFile(caPath)
		if err != nil {
			return nil, fmt.Errorf("read CA bundle %q: %w", caPath, err)
		}
		pool := x509.NewCertPool()
		pool.AppendCertsFromPEM(caCert)
		return &tls.Config{RootCAs: pool}, nil
	}
	return nil, nil
}

// isLocalIP checks if the given IP belongs to this machine's network interfaces.
func isLocalIP(ip string) bool {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return false
	}
	for _, a := range addrs {
		if ipNet, ok := a.(*net.IPNet); ok && ipNet.IP.String() == ip {
			return true
		}
	}
	return false
}

func (srv *server) loadMinioConfig() *config.MinioProxyConfig {
	// etcd is the only source of truth. No env vars, no disk contracts, no
	// localhost fallbacks. Endpoint is a DNS name resolved via cluster DNS.
	//
	// Authority routing: if /globular/repository/authority is set, use its
	// minio_endpoint instead of the cluster-wide minio/config endpoint.
	// This prevents split-brain when minio.globular.internal round-robins to a
	// node whose MinIO buckets are empty (non-authority node).
	cfg, err := config.BuildMinioProxyConfig()
	if err != nil {
		return nil
	}

	// Override endpoint with authority's if available.
	if auth, authErr := config.LoadRepositoryAuthority(); authErr == nil && auth.MinioEndpoint != "" {
		if auth.MinioEndpoint != cfg.Endpoint {
			logger.Info("repository authority: overriding minio endpoint",
				"cluster_endpoint", cfg.Endpoint,
				"authority_endpoint", auth.MinioEndpoint,
				"authority_node", auth.NodeID)
		}
		cfg.Endpoint = auth.MinioEndpoint
	}

	return cfg
}

func parseMinioConfigFromMap(m map[string]interface{}) *config.MinioProxyConfig {
	cfg := &config.MinioProxyConfig{}
	if v, ok := m["endpoint"].(string); ok {
		cfg.Endpoint = v
	}
	if v, ok := m["bucket"].(string); ok {
		cfg.Bucket = v
	}
	if v, ok := m["prefix"].(string); ok {
		cfg.Prefix = v
	}
	if v, ok := m["secure"].(bool); ok {
		cfg.Secure = v
	}
	if v, ok := m["caBundlePath"].(string); ok {
		cfg.CABundlePath = v
	}
	if authRaw, ok := m["auth"].(map[string]interface{}); ok {
		cfg.Auth = &config.MinioProxyAuth{}
		if mode, ok := authRaw["mode"].(string); ok {
			cfg.Auth.Mode = mode
		}
		if ak, ok := authRaw["accessKey"].(string); ok {
			cfg.Auth.AccessKey = ak
		}
		if sk, ok := authRaw["secretKey"].(string); ok {
			cfg.Auth.SecretKey = sk
		}
		if cf, ok := authRaw["credFile"].(string); ok {
			cfg.Auth.CredFile = cf
		}
	}
	return cfg
}

func (srv *server) initStorage() error {
	if !srv.minioEnabled() {
		return fmt.Errorf("MinIO configuration missing — the repository requires distributed " +
			"object storage (etcd key %s must be set by the cluster controller)",
			config.EtcdKeyMinioConfig)
	}
	if err := srv.ensureMinioClient(); err != nil {
		return fmt.Errorf("MinIO client init: %w", err)
	}
	m, err := storage_backend.NewMinioStorage(srv.minioClient, srv.MinioConfig.Bucket, srv.MinioConfig.Prefix)
	if err != nil {
		return fmt.Errorf("MinIO storage init: %w", err)
	}
	srv.storage = m
	logger.Info("minio storage initialized",
		"endpoint", srv.MinioConfig.Endpoint,
		"bucket", srv.MinioConfig.Bucket)
	return nil
}

// requireHealthy gates RPCs behind the dependency health check.
// Returns a gRPC UNAVAILABLE error if MinIO or ScyllaDB is down.
func (srv *server) requireHealthy() error {
	if srv.depHealth == nil {
		return nil // watchdog not yet started (startup)
	}
	return srv.depHealth.RequireHealthy()
}

// Storage returns the configured backend. MinIO is required — there is no
// local filesystem fallback. If storage was not initialized, the service is
// broken and RPCs should not reach this point (dep_health gates them).
func (srv *server) Storage() storage_backend.Storage {
	if srv.storage == nil {
		logger.Error("BUG: Storage() called but storage backend is nil — MinIO not initialized")
	}
	return srv.storage
}

// reachabilityConfig returns the ReachabilityConfig that should be used for
// all reachability computations (deletion guards, revoke guards, GC).
// It reads the configured retention window from the server config, falling back
// to the compiled-in default when the field is zero or negative.
func (srv *server) reachabilityConfig() ReachabilityConfig {
	w := srv.GCRetentionWindow
	if w <= 0 {
		w = defaultRetentionWindow
	}
	return ReachabilityConfig{RetentionWindow: w}
}

// -----------------------------------------------------------------------------
// CLI / entrypoint
// -----------------------------------------------------------------------------

var logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
	Level: slog.LevelInfo,
}))

// -----------------------------------------------------------------------------
// Helper functions for main() (Phase 1 Step 4)
// -----------------------------------------------------------------------------

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

// initializeServerDefaults creates a server with all default field values.
// This is the "god object" initialization extracted for clarity.
func initializeServerDefaults() *server {
	s := new(server)

	// Core metadata
	s.Name = string(repositorypb.File_repository_proto.Services().Get(0).FullName())
	s.Proto = repositorypb.File_repository_proto.Path()
	s.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	s.Port = defaultPort
	s.Proxy = defaultProxy
	s.Protocol = "grpc"
	s.Version = ""
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

	// RBAC permissions for package upload/download
	s.Permissions = make([]any, 0)

	// Runtime defaults
	s.Process = -1
	s.ProxyProcess = -1
	s.AllowAllOrigins = allowAllOrigins
	s.AllowedOrigins = allowedOriginsStr
	s.KeepAlive = true
	s.KeepUpToDate = true

	// Manifest cache for reducing storage backend reads from reconcile loops.
	s.cache = newManifestCache()

	// Workflow recorder for publish tracing (fire-and-forget, never blocks uploads).
	// Route through the Envoy gateway so it works on any node.
	clusterID := "globular.internal"
	if d, err := config.GetDomain(); err == nil && d != "" {
		clusterID = d
	}
	s.workflowRec = workflow.NewRecorderWithResolver(func() string {
		if addr := config.ResolveServiceAddr("workflow.WorkflowService", ""); addr != "" {
			return addr
		}
		if addr, err := config.GetMeshAddress(); err == nil {
			return addr // routes through Envoy service mesh (:443)
		}
		return ""
	}, clusterID)

	return s
}

// handleInformationalFlags processes --describe, --health, --help, --version, --debug.
// Returns true if the program should exit (flag was handled).

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

func setupGrpcService(srv *server) {
	Utility.RegisterFunction("NewRepositoryService_Client", repository_client.NewRepositoryService_Client)
	repositorypb.RegisterPackageRepositoryServer(srv.grpcServer, srv)
	reflection.Register(srv.grpcServer)
}

func main() {
	// 1. Initialize server with defaults
	s := initializeServerDefaults()

	// 2. Parse CLI arguments
	args := os.Args[1:]

	// Handle --debug flag (modifies global logger, must be before other flag handling)
	for _, a := range args {
		if strings.ToLower(a) == "--debug" {
			logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
			break
		}
	}

	// Register method→action mappings with the global resolver for interceptor use.
	policy.GlobalResolver().Register([]policy.Permission{
		{Method: "/repository.PackageRepository/DownloadBundle", Action: "repository.read"},
		{Method: "/repository.PackageRepository/ListBundles", Action: "repository.read"},
		{Method: "/repository.PackageRepository/ListArtifacts", Action: "repository.read"},
		{Method: "/repository.PackageRepository/GetArtifactManifest", Action: "repository.read"},
		{Method: "/repository.PackageRepository/DownloadArtifact", Action: "repository.read"},
		{Method: "/repository.PackageRepository/SearchArtifacts", Action: "repository.read"},
		{Method: "/repository.PackageRepository/GetArtifactVersions", Action: "repository.read"},
		{Method: "/repository.PackageRepository/GetNamespace", Action: "repository.read"},
		{Method: "/repository.PackageRepository/UploadBundle", Action: "repository.write"},
		{Method: "/repository.PackageRepository/UploadArtifact", Action: "repository.write"},
		{Method: "/repository.PackageRepository/SetArtifactState", Action: "repository.write"},
		{Method: "/repository.PackageRepository/DeleteArtifact", Action: "repository.delete"},
	})

	// 3. Handle informational flags (--describe, --health, --help, --version)
	if globular.HandleInformationalFlags(s, args, logger, printUsage) {
		return // Flag was handled, exit
	}

	// 4. Parse positional arguments (service_id, config_path)
	globular.ParsePositionalArgs(s, args)

	// 5. Allocate port if no arguments provided
	if err := globular.AllocatePortIfNeeded(s, args); err != nil {
		logger.Error("port allocation failed", "err", err)
		os.Exit(1)
	}

	// 6. Load runtime config (domain, address from file or etcd)
	globular.LoadRuntimeConfig(s)

	// 7. Initialize service (creates gRPC server, loads persisted config)
	start := time.Now()
	if err := s.Init(); err != nil {
		logger.Error("service init failed", "service", s.Name, "id", s.Id, "err", err)
		os.Exit(1)
	}
	// Always override Root after Init() so a stale etcd value can't redirect packages
	// to the wrong local path.
	s.Root = config.GetDataDir()

	// 7a. Connect to ScyllaDB (distributed manifest metadata store).
	// ScyllaDB is a hard dependency — if unreachable, the service starts degraded
	// and the health watchdog will mark it NOT_SERVING.
	scylla, scyllaErr := connectScylla()
	if scyllaErr != nil {
		logger.Error("scylladb connection failed — service will start degraded", "err", scyllaErr)
	}
	s.scylla = scylla

	// 7b. Load MinIO config (etcd only — no env vars, no disk fallbacks).
	s.MinioConfig = s.loadMinioConfig()
	if s.MinioConfig != nil {
		logger.Info("minio storage configured",
			"endpoint", s.MinioConfig.Endpoint,
			"bucket", s.MinioConfig.Bucket,
			"prefix", s.MinioConfig.Prefix)
	}
	if err := s.initStorage(); err != nil {
		logger.Error("minio storage init failed — service will start degraded", "err", err)
		// Do NOT fall back to local filesystem. The repository requires distributed
		// storage. The health watchdog will mark us NOT_SERVING.
	}

	// 7c. Start dependency health watchdog.
	// Continuously monitors MinIO + ScyllaDB. Gates RPCs with UNAVAILABLE when
	// either dependency is down. Recovery is automatic.
	//
	// The watchdog holds a *scyllaStore directly (for Ping/Reconnect); the server
	// holds a manifestLedger interface (for business-logic operations + testability).
	// They point to the same underlying *scyllaStore when both are set.
	var concreteScylla *scyllaStore
	if s.scylla != nil {
		concreteScylla = s.scylla.(*scyllaStore)
	}
	s.depHealth = newDepHealthWatchdog(s.storage, concreteScylla, logger)
	s.depHealth.onScyllaReady = func(scylla *scyllaStore) {
		s.scylla = scylla
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go s.depHealth.Start(ctx)

	// Phase 2: Validate storage topology — reject unsafe round-robin endpoints
	// in standalone_authority mode before serving any RPCs.
	if topErr := s.validateStorageTopology(ctx); topErr != nil {
		logger.Error("storage topology validation failed — service will start degraded",
			"err", topErr)
		// Non-fatal: log and continue. The dep_health watchdog gates RPCs.
		// Operators must fix the topology config to restore full service.
	}

	// 7d. Run trust model migration (idempotent — only on first run).
	if s.storage != nil {
		s.MigrateToTrustModel(ctx)
	}

	// 7d2. Phase 2: backfill build_id for existing artifacts (idempotent).
	if s.storage != nil {
		s.MigrateBuildIDs(ctx)
	}

	// 7d3. Phase 3: build release ledger from existing PUBLISHED artifacts (idempotent).
	if s.storage != nil {
		s.MigrateReleaseLedger(ctx)
	}

	// 7e. Start publish reconciler (retries stuck VERIFIED artifacts).
	pr := newPublishReconciler(s)
	pr.Start(ctx)

	// 7f. Phase 4: start reservation cleanup goroutine.
	startReservationCleanup(ctx)

	// 8. Register gRPC service and reflection
	setupGrpcService(s)

	logger.Info("service ready",
		"service", s.Name,
		"port", s.Port,
		"proxy", s.Proxy,
		"protocol", s.Protocol,
		"domain", s.Domain,
		"scylladb", scyllaErr == nil,
		"minio", s.storage != nil,
		"init_ms", time.Since(start).Milliseconds())

	// 9. Start service using shared lifecycle manager
	lm := globular.NewLifecycleManager(s, logger)
	if err := lm.Start(); err != nil {
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
