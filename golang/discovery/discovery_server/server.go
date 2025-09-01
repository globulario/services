package main

import (
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/discovery/discovery_client"
	"github.com/globulario/services/golang/discovery/discoverypb"
	"github.com/globulario/services/golang/event/event_client"
	"github.com/globulario/services/golang/globular_client"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/interceptors"
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
// Defaults and globals
// -----------------------------------------------------------------------------

var (
	// Default ports for the Discovery microservice and its reverse proxy.
	defaultPort  = 10029
	defaultProxy = 10030

	// CORS controls
	allowAllOrigins = true        // if true, allow all origins
	allowedOrigins  string = ""   // comma-separated list when allowAllOrigins is false
)

// -----------------------------------------------------------------------------
// Service definition
// -----------------------------------------------------------------------------

// server implements the Discovery service process/runtime configuration and
// embeds a running gRPC server instance.
type server struct {
	// Globular service metadata
	Id              string   // unique ID for this service instance
	Mac             string
	Name            string    // gRPC service name (must match .proto)
	Domain          string    // domain where the service is reachable
	Address         string    // http(s) address exposing /config
	Path            string    // executable directory
	Proto           string    // path to the .proto file
	Port            int       // gRPC port
	Proxy           int       // reverse-proxy port (gRPC-Web)
	AllowAllOrigins bool      // if true, any origin is allowed
	AllowedOrigins  string    // CSV of allowed origins when AllowAllOrigins=false
	Protocol        string    // "grpc", "http", "https", etc.
	Version         string    // semantic version of the service
	PublisherID     string    // publisher identifier
	KeepUpToDate    bool      // auto-update when true
	Plaform         string    // note: kept as-is to preserve existing public API
	Checksum        string    // build checksum
	KeepAlive       bool      // restart automatically if process exits
	Description     string
	Keywords        []string
	Repositories    []string
	Discoveries     []string
	Process         int       // PID of service process (managed by Globular)
	ProxyProcess    int       // PID of proxy process (managed by Globular)
	ConfigPath      string    // path to config file/dir
	LastError       string    // last fatal/non-fatal error string
	State           string    // current service state
	ModTime         int64     // config modification time (unix)

	// TLS
	TLS                 bool   // if true, service runs with TLS
	CertFile            string // server-signed X.509 public key
	KeyFile             string // private RSA key
	CertAuthorityTrust  string // CA trust file

	// Permissions and dependencies
	Permissions  []interface{} // action permissions for the service
	Dependencies []string      // required services (rbac, resource, etc.)

	// gRPC server instance
	grpcServer *grpc.Server
}

// -----------------------------------------------------------------------------
// Globular runtime getters/setters (exported: keep prototypes unchanged)
// -----------------------------------------------------------------------------

// GetConfigurationPath returns the configuration path for this service.
func (srv *server) GetConfigurationPath() string { return srv.ConfigPath }

// SetConfigurationPath sets the configuration path for this service.
func (srv *server) SetConfigurationPath(path string) { srv.ConfigPath = path }

// GetAddress returns the HTTP(S) address where /config is exposed.
func (srv *server) GetAddress() string { return srv.Address }

// SetAddress sets the HTTP(S) address where /config is exposed.
func (srv *server) SetAddress(address string) { srv.Address = address }

// GetProcess returns the PID of the service process (if managed).
func (srv *server) GetProcess() int { return srv.Process }

// SetProcess sets the PID of the service process (if managed).
func (srv *server) SetProcess(pid int) { srv.Process = pid }

// GetProxyProcess returns the PID of the reverse-proxy process (if managed).
func (srv *server) GetProxyProcess() int { return srv.ProxyProcess }

// SetProxyProcess sets the PID of the reverse-proxy process (if managed).
func (srv *server) SetProxyProcess(pid int) { srv.ProxyProcess = pid }

// GetState returns the current service state string.
func (srv *server) GetState() string { return srv.State }

// SetState sets the current service state string.
func (srv *server) SetState(state string) { srv.State = state }

// GetLastError returns the last error message recorded by the service.
func (srv *server) GetLastError() string { return srv.LastError }

// SetLastError sets the last error message recorded by the service.
func (srv *server) SetLastError(err string) { srv.LastError = err }

// SetModTime sets the configuration modification time (unix).
func (srv *server) SetModTime(modtime int64) { srv.ModTime = modtime }

// GetModTime returns the configuration modification time (unix).
func (srv *server) GetModTime() int64 { return srv.ModTime }

// GetId returns the unique identifier of this service instance.
func (srv *server) GetId() string { return srv.Id }

// SetId sets the unique identifier of this service instance.
func (srv *server) SetId(id string) { srv.Id = id }

// GetName returns the gRPC service name.
func (srv *server) GetName() string { return srv.Name }

// SetName sets the gRPC service name.
func (srv *server) SetName(name string) { srv.Name = name }

// GetMac returns the MAC address associated with this service (if any).
func (srv *server) GetMac() string { return srv.Mac }

// SetMac sets the MAC address associated with this service.
func (srv *server) SetMac(mac string) { srv.Mac = mac }

// GetDescription returns the human-friendly description of the service.
func (srv *server) GetDescription() string { return srv.Description }

// SetDescription sets the human-friendly description of the service.
func (srv *server) SetDescription(description string) { srv.Description = description }

// GetKeywords returns the list of keywords for the service.
func (srv *server) GetKeywords() []string { return srv.Keywords }

// SetKeywords sets the list of keywords for the service.
func (srv *server) SetKeywords(keywords []string) { srv.Keywords = keywords }

// GetRepositories returns the list of repositories associated with the service.
func (srv *server) GetRepositories() []string { return srv.Repositories }

// SetRepositories sets the list of repositories associated with the service.
func (srv *server) SetRepositories(repositories []string) { srv.Repositories = repositories }

// GetDiscoveries returns the list of discovery endpoints.
func (srv *server) GetDiscoveries() []string { return srv.Discoveries }

// SetDiscoveries sets the list of discovery endpoints.
func (srv *server) SetDiscoveries(discoveries []string) { srv.Discoveries = discoveries }

// Dist packages the service at the given path using Globular’s distribution flow.
func (srv *server) Dist(path string) (string, error) { return globular.Dist(path, srv) }

// GetDependencies returns the declared service dependencies.
func (srv *server) GetDependencies() []string {
	if srv.Dependencies == nil {
		srv.Dependencies = make([]string, 0)
	}
	return srv.Dependencies
}

// SetDependency appends a dependency to the service if it is not already present.
func (srv *server) SetDependency(dependency string) {
	if srv.Dependencies == nil {
		srv.Dependencies = make([]string, 0)
	}
	if !Utility.Contains(srv.Dependencies, dependency) {
		srv.Dependencies = append(srv.Dependencies, dependency)
	}
}

// GetChecksum returns the build checksum for this service.
func (srv *server) GetChecksum() string { return srv.Checksum }

// SetChecksum sets the build checksum for this service.
func (srv *server) SetChecksum(checksum string) { srv.Checksum = checksum }

// GetPlatform returns the target platform string.
// (Note: field name is kept as Plaform to preserve public API.)
func (srv *server) GetPlatform() string { return srv.Plaform }

// SetPlatform sets the target platform string.
func (srv *server) SetPlatform(platform string) { srv.Plaform = platform }

// GetPath returns the path of the executable.
func (srv *server) GetPath() string { return srv.Path }

// SetPath sets the path of the executable.
func (srv *server) SetPath(path string) { srv.Path = path }

// GetProto returns the path of the .proto file.
func (srv *server) GetProto() string { return srv.Proto }

// SetProto sets the path of the .proto file.
func (srv *server) SetProto(proto string) { srv.Proto = proto }

// GetPort returns the gRPC port.
func (srv *server) GetPort() int { return srv.Port }

// SetPort sets the gRPC port.
func (srv *server) SetPort(port int) { srv.Port = port }

// GetProxy returns the reverse-proxy (gRPC-Web) port.
func (srv *server) GetProxy() int { return srv.Proxy }

// SetProxy sets the reverse-proxy (gRPC-Web) port.
func (srv *server) SetProxy(proxy int) { srv.Proxy = proxy }

// GetProtocol returns the transport protocol ("grpc", "http", "https", etc.).
func (srv *server) GetProtocol() string { return srv.Protocol }

// SetProtocol sets the transport protocol ("grpc", "http", "https", etc.).
func (srv *server) SetProtocol(protocol string) { srv.Protocol = protocol }

// GetAllowAllOrigins reports whether all origins are allowed for CORS.
func (srv *server) GetAllowAllOrigins() bool { return srv.AllowAllOrigins }

// SetAllowAllOrigins sets whether all origins are allowed for CORS.
func (srv *server) SetAllowAllOrigins(allowAllOrigins bool) { srv.AllowAllOrigins = allowAllOrigins }

// GetAllowedOrigins returns the CSV list of allowed origins (when not allowing all).
func (srv *server) GetAllowedOrigins() string { return srv.AllowedOrigins }

// SetAllowedOrigins sets the CSV list of allowed origins.
func (srv *server) SetAllowedOrigins(allowedOrigins string) { srv.AllowedOrigins = allowedOrigins }

// GetDomain returns the domain part of the service address.
func (srv *server) GetDomain() string { return srv.Domain }

// SetDomain sets the domain part of the service address.
func (srv *server) SetDomain(domain string) { srv.Domain = domain }

// TLS section

// GetTls returns true if the service runs with TLS.
// NOTE: method name kept as-is to preserve public API.
func (srv *server) GetTls() bool { return srv.TLS }

// SetTls enables or disables TLS for this service.
// NOTE: method name kept as-is to preserve public API.
func (srv *server) SetTls(hasTls bool) { srv.TLS = hasTls }

// GetCertAuthorityTrust returns the CA trust file path.
func (srv *server) GetCertAuthorityTrust() string { return srv.CertAuthorityTrust }

// SetCertAuthorityTrust sets the CA trust file path.
func (srv *server) SetCertAuthorityTrust(ca string) { srv.CertAuthorityTrust = ca }

// GetCertFile returns the certificate file path.
func (srv *server) GetCertFile() string { return srv.CertFile }

// SetCertFile sets the certificate file path.
func (srv *server) SetCertFile(certFile string) { srv.CertFile = certFile }

// GetKeyFile returns the private key file path.
func (srv *server) GetKeyFile() string { return srv.KeyFile }

// SetKeyFile sets the private key file path.
func (srv *server) SetKeyFile(keyFile string) { srv.KeyFile = keyFile }

// GetVersion returns the service version string.
func (srv *server) GetVersion() string { return srv.Version }

// SetVersion sets the service version string.
func (srv *server) SetVersion(version string) { srv.Version = version }

// GetPublisherID returns the publisher identifier.
func (srv *server) GetPublisherID() string { return srv.PublisherID }

// SetPublisherID sets the publisher identifier.
func (srv *server) SetPublisherID(PublisherID string) { srv.PublisherID = PublisherID }

// GetKeepUpToDate returns whether the service should keep itself up to date.
func (srv *server) GetKeepUpToDate() bool { return srv.KeepUpToDate }

// SetKeepUptoDate sets whether the service should keep itself up to date.
// NOTE: method name kept as-is to preserve public API.
func (srv *server) SetKeepUptoDate(val bool) { srv.KeepUpToDate = val }

// GetKeepAlive returns whether the service should be kept alive (auto-restart).
func (srv *server) GetKeepAlive() bool { return srv.KeepAlive }

// SetKeepAlive sets whether the service should be kept alive (auto-restart).
func (srv *server) SetKeepAlive(val bool) { srv.KeepAlive = val }

// GetPermissions returns the permissions array associated with this service.
func (srv *server) GetPermissions() []interface{} { return srv.Permissions }

// SetPermissions sets the permissions array associated with this service.
func (srv *server) SetPermissions(permissions []interface{}) { srv.Permissions = permissions }

// Init initializes the service configuration and gRPC server instance.
func (srv *server) Init() error {
	if err := globular.InitService(srv); err != nil {
		return err
	}
	grpcSrv, err := globular.InitGrpcServer(srv, interceptors.ServerUnaryInterceptor, interceptors.ServerStreamInterceptor)
	if err != nil {
		return err
	}
	srv.grpcServer = grpcSrv
	return nil
}

// Save persists the current configuration to disk.
func (srv *server) Save() error { return globular.SaveService(srv) }

// StartService starts the gRPC server and supporting processes.
func (srv *server) StartService() error { return globular.StartService(srv, srv.grpcServer) }

// StopService stops the gRPC server and supporting processes.
func (srv *server) StopService() error { return globular.StopService(srv, srv.grpcServer) }

// -----------------------------------------------------------------------------
// RBAC helpers
// -----------------------------------------------------------------------------

// GetRbacClient returns a connected RBAC client for the given address.
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

func (srv *server) addResourceOwner(path, resourceType, subject string, subjectType rbacpb.SubjectType) error {
	rbacClient, err := GetRbacClient(srv.Address)
	if err != nil {
		return err
	}
	return rbacClient.AddResourceOwner(path, resourceType, subject, subjectType)
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
		slog.Error("failed to publish event",
			"event", event,
			"domain", domain,
			"err", err,
		)
	}
	return err
}

// Resource manager client
func (srv *server) getResourceClient(address string) (*resource_client.Resource_Client, error) {
	Utility.RegisterFunction("NewResourceService_Client", resource_client.NewResourceService_Client)
	client, err := globular_client.GetClient(address, "resource.ResourceService", "NewResourceService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*resource_client.Resource_Client), nil
}

// isOrganizationMember returns true if user belongs to organization at address.
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

// publishPackageDescriptor creates/updates the PackageDescriptor and publishes an event.
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
// Main: bootstrap and run
// -----------------------------------------------------------------------------

// main boots the Discovery service, registers it to gRPC, and starts serving.
func main() {
	// Structured logger setup
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	// Service bootstrap with defaults
	s := new(server)
	s.Name = string(discoverypb.File_discovery_proto.Services().Get(0).FullName())
	s.Proto = discoverypb.File_discovery_proto.Path()
	s.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	s.Port = defaultPort
	s.Proxy = defaultProxy
	s.Protocol = "grpc"
	s.Domain, _ = config.GetDomain()
	s.Address, _ = config.GetAddress()
	s.Version = "0.0.1"
	s.PublisherID = "localhost"
	s.Description = "Service discovery client"
	s.Keywords = []string{"Discovery", "Package", "Service", "Application"}
	s.Repositories = make([]string, 0)
	s.Discoveries = make([]string, 0)
	s.Dependencies = []string{"rbac.RbacService", "resource.ResourceService"}
	s.Permissions = make([]interface{}, 2)
	s.Process = -1
	s.ProxyProcess = -1
	s.KeepAlive = true
	s.KeepUpToDate = true
	s.AllowAllOrigins = allowAllOrigins
	s.AllowedOrigins = allowedOrigins

	// Register discovery client constructor (for dynamic routing)
	Utility.RegisterFunction("NewDiscoveryService_Client", discovery_client.NewDiscoveryService_Client)

	// Optional CLI args: id [configPath]
	if len(os.Args) == 2 {
		s.Id = os.Args[1]
	} else if len(os.Args) == 3 {
		s.Id = os.Args[1]
		s.ConfigPath = os.Args[2]
	}

	// Default permissions for publishing actions
	s.Permissions[0] = map[string]interface{}{
		"action": "/discovery.DiscoveryService/PublishService",
		"resources": []interface{}{
			map[string]interface{}{"index": 0, "permission": "owner"},
		},
	}
	s.Permissions[1] = map[string]interface{}{
		"action": "/discovery.DiscoveryService/PublishApplication",
		"resources": []interface{}{
			map[string]interface{}{"index": 0, "permission": "owner"},
		},
	}

	slog.Info("initializing discovery service",
		"name", s.Name,
		"id", s.Id,
		"version", s.Version,
		"domain", s.Domain,
		"address", s.Address,
		"port", s.Port,
		"proxy", s.Proxy,
	)

	// Initialize config and gRPC server
	if err := s.Init(); err != nil {
		slog.Error("failed to initialize service",
			"name", s.Name,
			"id", s.Id,
			"err", err,
		)
		os.Exit(1)
	}

	if s.Address == "" {
		if addr, _ := config.GetAddress(); addr != "" {
			s.Address = addr
		}
	}

	// Register the service server and reflection
	discoverypb.RegisterPackageDiscoveryServer(s.grpcServer, s)
	reflection.Register(s.grpcServer)

	slog.Info("starting discovery service",
		"name", s.Name,
		"id", s.Id,
		"protocol", s.Protocol,
		"port", s.Port,
	)

	// Start serving
	if err := s.StartService(); err != nil {
		slog.Error("service start failed",
			"name", s.Name,
			"id", s.Id,
			"err", err,
		)
		os.Exit(1)
	}
}
