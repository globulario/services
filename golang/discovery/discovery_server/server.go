package main

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/discovery/discovery_client"
	"github.com/globulario/services/golang/discovery/discoverypb"
	"github.com/globulario/services/golang/event/event_client"
	"github.com/globulario/services/golang/globular_client"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/rbac/rbac_client"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/resource/resource_client"
	"github.com/globulario/services/golang/resource/resourcepb"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/encoding/protojson"
)

// -----------------------------------------------------------------------------
// Defaults & globals
// -----------------------------------------------------------------------------

var (
	defaultPort       = 10029
	defaultProxy      = 10030
	allowAllOrigins   = true
	allowedOriginsStr = ""
)

// STDERR logger so --describe/--health JSON stays clean on STDOUT
var logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

// -----------------------------------------------------------------------------
// Service definition
// -----------------------------------------------------------------------------

// server implements the Globular service contract and Discovery RPC handlers.
//
// Phase 1 Refactoring Complete:
// - Business logic extracted to handlers.go (PublishService, PublishApplication)
// - Lifecycle management extracted to lifecycle.go (Start/Stop/Ready/Health)
// - Config operations extracted to config.go (load/save/validate)
// - Main initialization simplified in server.go (8 helper functions)
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
	Plaform         string // Note: typo preserved for compatibility
	Checksum        string
	Permissions     []any    // RBAC action permissions
	Dependencies    []string // Required services

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

// -----------------------------------------------------------------------------
// Globular runtime getters/setters (signatures unchanged)
// -----------------------------------------------------------------------------

func (srv *server) GetConfigurationPath() string      { return srv.ConfigPath }
func (srv *server) SetConfigurationPath(path string)  { srv.ConfigPath = path }
func (srv *server) GetAddress() string                { return srv.Address }
func (srv *server) SetAddress(address string)         { srv.Address = address }
func (srv *server) GetProcess() int                   { return srv.Process }
func (srv *server) SetProcess(pid int)                { srv.Process = pid }
func (srv *server) GetProxyProcess() int              { return srv.ProxyProcess }
func (srv *server) SetProxyProcess(pid int)           { srv.ProxyProcess = pid }
func (srv *server) GetState() string                  { return srv.State }
func (srv *server) SetState(state string)             { srv.State = state }
func (srv *server) GetLastError() string              { return srv.LastError }
func (srv *server) SetLastError(err string)           { srv.LastError = err }
func (srv *server) SetModTime(modtime int64)          { srv.ModTime = modtime }
func (srv *server) GetModTime() int64                 { return srv.ModTime }
func (srv *server) GetId() string                     { return srv.Id }
func (srv *server) SetId(id string)                   { srv.Id = id }
func (srv *server) GetName() string                   { return srv.Name }
func (srv *server) SetName(name string)               { srv.Name = name }
func (srv *server) GetMac() string                    { return srv.Mac }
func (srv *server) SetMac(mac string)                 { srv.Mac = mac }
func (srv *server) GetDescription() string            { return srv.Description }
func (srv *server) SetDescription(description string) { srv.Description = description }
func (srv *server) GetKeywords() []string             { return srv.Keywords }
func (srv *server) SetKeywords(keywords []string)     { srv.Keywords = keywords }
func (srv *server) GetRepositories() []string         { return srv.Repositories }
func (srv *server) SetRepositories(v []string)        { srv.Repositories = v }
func (srv *server) GetDiscoveries() []string          { return srv.Discoveries }
func (srv *server) SetDiscoveries(v []string)         { srv.Discoveries = v }
func (srv *server) Dist(path string) (string, error)  { return globular.Dist(path, srv) }
func (srv *server) GetDependencies() []string {
	if srv.Dependencies == nil {
		srv.Dependencies = []string{}
	}
	return srv.Dependencies
}
func (srv *server) SetDependency(dep string) {
	if srv.Dependencies == nil {
		srv.Dependencies = []string{}
	}
	if !Utility.Contains(srv.Dependencies, dep) {
		srv.Dependencies = append(srv.Dependencies, dep)
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
func (srv *server) GetGrpcServer() *grpc.Server              { return srv.grpcServer }
func (srv *server) GetProtocol() string                      { return srv.Protocol }
func (srv *server) SetProtocol(protocol string)              { srv.Protocol = protocol }
func (srv *server) GetAllowAllOrigins() bool                 { return srv.AllowAllOrigins }
func (srv *server) SetAllowAllOrigins(b bool)                { srv.AllowAllOrigins = b }
func (srv *server) GetAllowedOrigins() string                { return srv.AllowedOrigins }
func (srv *server) SetAllowedOrigins(s string)               { srv.AllowedOrigins = s }
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
func (srv *server) GetKeepAlive() bool                 { return srv.KeepAlive }
func (srv *server) SetKeepAlive(val bool)              { srv.KeepAlive = val }
func (srv *server) GetPermissions() []any              { return srv.Permissions }
func (srv *server) SetPermissions(permissions []any)   { srv.Permissions = permissions }

// RolesDefault returns curated roles for PackageDiscovery.
func (srv *server) RolesDefault() []resourcepb.Role {
	domain, _ := config.GetDomain()

	servicePublisher := resourcepb.Role{
		Id:          "role:discovery.service_publisher",
		Name:        "Service Publisher",
		Domain:      domain,
		Description: "Can publish service packages to allowed repositories/discoveries.",
		Actions: []string{
			"/discovery.PackageDiscovery/PublishService",
		},
		TypeName: "resource.Role",
	}

	appPublisher := resourcepb.Role{
		Id:          "role:discovery.app_publisher",
		Name:        "Application Publisher",
		Domain:      domain,
		Description: "Can publish application packages to allowed repositories/discoveries.",
		Actions: []string{
			"/discovery.PackageDiscovery/PublishApplication",
		},
		TypeName: "resource.Role",
	}

	publisher := resourcepb.Role{
		Id:          "role:discovery.publisher",
		Name:        "Publisher",
		Domain:      domain,
		Description: "Can publish services and applications.",
		Actions: []string{
			"/discovery.PackageDiscovery/PublishService",
			"/discovery.PackageDiscovery/PublishApplication",
		},
		TypeName: "resource.Role",
	}

	admin := resourcepb.Role{
		Id:          "role:discovery.admin",
		Name:        "Discovery Admin",
		Domain:      domain,
		Description: "Full publish rights across repositories/discoveries.",
		Actions: []string{
			"/discovery.PackageDiscovery/PublishService",
			"/discovery.PackageDiscovery/PublishApplication",
		},
		TypeName: "resource.Role",
	}

	return []resourcepb.Role{servicePublisher, appPublisher, publisher, admin}
}

// Lifecycle
func (srv *server) Init() error {
	if err := globular.InitService(srv); err != nil {
		return err
	}
	gs, err := globular.InitGrpcServer(srv) // interceptors wired internally (auth-template style)
	if err != nil {
		return err
	}
	srv.grpcServer = gs
	return nil
}
func (srv *server) Save() error { return globular.SaveService(srv) }

// NOTE: StartService() and StopService() moved to lifecycle.go in Phase 1 Step 3

// -----------------------------------------------------------------------------
// RBAC helpers
// -----------------------------------------------------------------------------

func GetRbacClient(address string) (*rbac_client.Rbac_Client, error) {
	Utility.RegisterFunction("NewRbacService_Client", rbac_client.NewRbacService_Client)
	client, err := globular_client.GetClient(address, "rbac.RbacService", "NewRbacService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*rbac_client.Rbac_Client), nil
}
func (srv *server) getResourcePermissions(path string) (*rbacpb.Permissions, error) {
	rbacClient, err := GetRbacClient(srv.Address)
	if err != nil {
		return nil, err
	}
	return rbacClient.GetResourcePermissions(path)
}
func (srv *server) setResourcePermissions(token, path, resourceType string, permissions *rbacpb.Permissions) error {
	rbacClient, err := GetRbacClient(srv.Address)
	if err != nil {
		return err
	}
	return rbacClient.SetResourcePermissions(token, path, resourceType, permissions)
}
func (srv *server) validateAccess(subject string, subjectType rbacpb.SubjectType, name string, path string) (bool, bool, error) {
	rbacClient, err := GetRbacClient(srv.Address)
	if err != nil {
		return false, false, err
	}
	return rbacClient.ValidateAccess(subject, subjectType, name, path)
}
func (srv *server) addResourceOwner(token, path, resourceType, subject string, subjectType rbacpb.SubjectType) error {
	rbacClient, err := GetRbacClient(srv.Address)
	if err != nil {
		return err
	}
	return rbacClient.AddResourceOwner(token, path, subject, resourceType, subjectType)
}

// -----------------------------------------------------------------------------
// Event & Resource helpers
// -----------------------------------------------------------------------------

func (srv *server) getEventClient(address string) (*event_client.Event_Client, error) {
	Utility.RegisterFunction("NewEventService_Client", event_client.NewEventService_Client)
	client, err := globular_client.GetClient(address, "event.EventService", "NewEventService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*event_client.Event_Client), nil
}
func (srv *server) publish(domain, event string, data []byte) error {
	evtClient, err := srv.getEventClient(domain)
	if err != nil {
		return err
	}
	if err = evtClient.Publish(event, data); err != nil {
		logger.Error("failed to publish event", "event", event, "domain", domain, "err", err)
	}
	return err
}
func (srv *server) getResourceClient(address string) (*resource_client.Resource_Client, error) {
	Utility.RegisterFunction("NewResourceService_Client", resource_client.NewResourceService_Client)
	client, err := globular_client.GetClient(address, "resource.ResourceService", "NewResourceService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*resource_client.Resource_Client), nil
}
func (srv *server) isOrganizationMember(user, organization string) (bool, error) {
	address, _ := config.GetAddress()
	if strings.Contains(user, "@") {
		parts := strings.SplitN(user, "@", 2)
		user = parts[0]
		address = parts[1]
	}
	resourceClient, err := srv.getResourceClient(address)
	if err != nil {
		return false, err
	}
	return resourceClient.IsOrganizationMemeber(user, organization)
}
func (srv *server) publishPackageDescriptor(descriptor *resourcepb.PackageDescriptor) error {
	address, _ := config.GetAddress()
	resourceClient, err := srv.getResourceClient(address)
	if err != nil {
		return err
	}
	if err = resourceClient.SetPackageDescriptor(descriptor); err != nil {
		return err
	}
	payload, err := protojson.Marshal(descriptor)
	if err != nil {
		return err
	}
	return srv.publish(address, descriptor.PublisherID+":"+descriptor.Id, payload)
}

// -----------------------------------------------------------------------------
// Usage
// -----------------------------------------------------------------------------

func printUsage() {
	exe := filepath.Base(os.Args[0])
	os.Stdout.WriteString(`
Usage: ` + exe + ` [options] <id> [configPath]

Options:
  --describe      Print service description as JSON (no etcd/config access)
  --health        Print service health as JSON (no etcd/config access)

Arguments:
  <id>            Service instance ID
  [configPath]    Optional path to configuration file

Example:
  ` + exe + ` discovery-1 /etc/globular/discovery/config.json

`)
}

// -----------------------------------------------------------------------------
// Main
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

// initializeServerDefaults sets up the server with default values before config loading.
// This MUST NOT touch etcd or any external config - only local defaults.
func initializeServerDefaults() *server {
	srv := new(server)

	// Basic identity
	srv.Name = string(discoverypb.File_discovery_proto.Services().Get(0).FullName())
	srv.Proto = discoverypb.File_discovery_proto.Path()
	srv.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	srv.Version = "0.0.1"
	srv.PublisherID = "localhost"
	srv.Description = "Service discovery client"
	srv.Keywords = []string{"Discovery", "Package", "Service", "Application"}

	// Network defaults
	srv.Port = defaultPort
	srv.Proxy = defaultProxy
	srv.Protocol = "grpc"

	// Service discovery
	srv.Repositories = []string{}
	srv.Discoveries = []string{}

	// Dependencies
	srv.Dependencies = []string{"rbac.RbacService", "resource.ResourceService"}

	// CORS defaults
	srv.AllowAllOrigins = allowAllOrigins
	srv.AllowedOrigins = allowedOriginsStr

	// Lifecycle defaults
	srv.KeepAlive = true
	srv.KeepUpToDate = true
	srv.Process = -1
	srv.ProxyProcess = -1

	// RBAC permissions for PackageDiscovery
	srv.Permissions = []any{
		// PublishService permission
		map[string]any{
			"action":     "/discovery.PackageDiscovery/PublishService",
			"permission": "write",
			"resources": []any{
				map[string]any{"index": 0, "field": "RepositoryId", "permission": "write"},
				map[string]any{"index": 0, "field": "DiscoveryId", "permission": "write"},
			},
		},
		// PublishApplication permission
		map[string]any{
			"action":     "/discovery.PackageDiscovery/PublishApplication",
			"permission": "write",
			"resources": []any{
				map[string]any{"index": 0, "field": "Repository", "permission": "write"},
				map[string]any{"index": 0, "field": "Discovery", "permission": "write"},
			},
		},
	}

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
// - initializeServerDefaults (service-specific defaults and RBAC permissions)
// - setupGrpcService (service-specific gRPC registration)


// setupGrpcService registers the Discovery service with the gRPC server.
func setupGrpcService(srv *server) {
	discoverypb.RegisterPackageDiscoveryServer(srv.grpcServer, srv)
	reflection.Register(srv.grpcServer)
}

// main configures and starts the Discovery service.
// Phase 1 Step 4: Simplified to use extracted components and helper functions.
func main() {
	// Initialize server with defaults (no etcd/config access yet)
	srv := initializeServerDefaults()

	// Handle CLI flags
	args := os.Args[1:]

	// Handle --debug flag (modifies global logger, must be before other flag handling)
	for _, a := range args {
		if strings.ToLower(a) == "--debug" {
			logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
			break
		}
	}

	// Handle informational flags (may exit early)
	if globular.HandleInformationalFlags(srv, args, logger, printUsage) {
		return
	}

	// Allocate port if needed (before etcd access)
	if err := globular.AllocatePortIfNeeded(srv, args); err != nil {
		logger.Error("port allocation failed", "error", err)
		os.Exit(1)
	}

	// Parse positional arguments
	globular.ParsePositionalArgs(srv, args)

	// Load runtime config from backend (etcd or file fallback)
	globular.LoadRuntimeConfig(srv)

	// Register client constructor
	Utility.RegisterFunction("NewDiscoveryService_Client", discovery_client.NewDiscoveryService_Client)

	// Initialize service (creates gRPC server, loads config)
	start := time.Now()
	if err := srv.Init(); err != nil {
		logger.Error("service init failed", "service", srv.Name, "id", srv.Id, "err", err)
		os.Exit(1)
	}

	// Register gRPC service handlers
	setupGrpcService(srv)

	logger.Info("service ready",
		"service", srv.Name,
		"port", srv.Port,
		"proxy", srv.Proxy,
		"protocol", srv.Protocol,
		"domain", srv.Domain,
		"init_ms", time.Since(start).Milliseconds())

	// Start service using shared lifecycle manager
	lm := globular.NewLifecycleManager(srv, logger)
	if err := lm.Start(); err != nil {
		logger.Error("service start failed", "service", srv.Name, "id", srv.Id, "err", err)
		os.Exit(1)
	}
}
