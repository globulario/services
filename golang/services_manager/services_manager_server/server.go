package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/event/event_client"
	"github.com/globulario/services/golang/event/eventpb"
	"github.com/globulario/services/golang/globular_client"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/log/log_client"
	"github.com/globulario/services/golang/log/logpb"
	"github.com/globulario/services/golang/process"
	"github.com/globulario/services/golang/rbac/rbac_client"
	"github.com/globulario/services/golang/resource/resource_client"
	"github.com/globulario/services/golang/resource/resourcepb"
	"github.com/globulario/services/golang/security"
	service_manager_client "github.com/globulario/services/golang/services_manager/services_manager_client"
	"github.com/globulario/services/golang/services_manager/services_managerpb"
	Utility "github.com/globulario/utility"
)

// -----------------------------------------------------------------------------
// Defaults & globals
// -----------------------------------------------------------------------------

var (
	defaultPort        = 10000
	defaultProxy       = defaultPort + 1
	allowAllOriginsDef = true
	allowedOriginsDef  = ""

	logger = slog.Default()
)

// -----------------------------------------------------------------------------
// Server
// -----------------------------------------------------------------------------

// server defines the Globular service-manager server instance.
type server struct {
	// Identity & addressing
	Id      string
	Name    string
	Mac     string
	Domain  string
	Address string

	// Executable & interface
	Path     string
	Proto    string
	Port     int
	Proxy    int
	Protocol string // grpc|http(s)|tls

	// Access control / CORS
	AllowAllOrigins bool
	AllowedOrigins  string // comma-separated

	// Packaging & ownership
	Version     string
	PublisherID string
	Plaform     string
	Checksum    string

	// Lifecycle / behavior
	KeepUpToDate bool
	KeepAlive    bool

	// Metadata
	Description  string
	Keywords     []string
	Repositories []string
	Discoveries  []string

	// Runtime state
	Process      int
	ProxyProcess int
	ConfigPath   string
	LastError    string
	State        string
	ModTime      int64
	TLS          bool

	// TLS files
	CertFile           string
	KeyFile            string
	CertAuthorityTrust string

	// RBAC / deps
	Permissions  []interface{}
	Dependencies []string

	// gRPC runtime
	grpcServer *grpc.Server
	methods    []string

	// Paths
	Root     string
	Creds    string
	DataPath string

	// Others
	PortsRange                 string
	Certificate                string
	CertificateAuthorityBundle string
	done                       chan bool
}

// -----------------------------------------------------------------------------
// Configuration getters/setters (public, signatures preserved)
// -----------------------------------------------------------------------------

// GetConfigurationPath returns the configuration path.
func (srv *server) GetConfigurationPath() string { return srv.ConfigPath }

// SetConfigurationPath sets the configuration path.
func (srv *server) SetConfigurationPath(path string) { srv.ConfigPath = path }

// GetAddress returns the HTTP address where configuration can be fetched (/config).
func (srv *server) GetAddress() string { return srv.Address }

// SetAddress sets the service address.
func (srv *server) SetAddress(address string) { srv.Address = address }

// GetProcess returns the current process PID.
func (srv *server) GetProcess() int { return srv.Process }

// SetProcess sets the current process PID.
func (srv *server) SetProcess(pid int) { srv.Process = pid }

// GetProxyProcess returns the current proxy process PID.
func (srv *server) GetProxyProcess() int { return srv.ProxyProcess }

// SetProxyProcess sets the current proxy process PID.
func (srv *server) SetProxyProcess(pid int) { srv.ProxyProcess = pid }

// GetState returns the current service state.
func (srv *server) GetState() string { return srv.State }

// SetState sets the current service state.
func (srv *server) SetState(state string) { srv.State = state }

// GetLastError returns the last error message.
func (srv *server) GetLastError() string { return srv.LastError }

// SetLastError sets the last error message.
func (srv *server) SetLastError(err string) { srv.LastError = err }

// SetModTime sets the modification time (unix).
func (srv *server) SetModTime(modtime int64) { srv.ModTime = modtime }

// GetModTime returns the modification time (unix).
func (srv *server) GetModTime() int64 { return srv.ModTime }

// GetId returns the unique service instance ID.
func (srv *server) GetId() string { return srv.Id }

// SetId sets the unique service instance ID.
func (srv *server) SetId(id string) { srv.Id = id }

// GetName returns the gRPC service name.
func (srv *server) GetName() string { return srv.Name }

// SetName sets the gRPC service name.
func (srv *server) SetName(name string) { srv.Name = name }

// GetMac returns the server MAC address.
func (srv *server) GetMac() string { return srv.Mac }

// SetMac sets the server MAC address.
func (srv *server) SetMac(mac string) { srv.Mac = mac }

// GetDescription returns the service description.
func (srv *server) GetDescription() string { return srv.Description }

// SetDescription sets the service description.
func (srv *server) SetDescription(description string) { srv.Description = description }

// GetKeywords returns the service keywords.
func (srv *server) GetKeywords() []string { return srv.Keywords }

// SetKeywords sets the service keywords.
func (srv *server) SetKeywords(keywords []string) { srv.Keywords = keywords }

// GetRepositories returns the repositories for the service.
func (srv *server) GetRepositories() []string { return srv.Repositories }

// SetRepositories sets the repositories for the service.
func (srv *server) SetRepositories(repositories []string) { srv.Repositories = repositories }

// GetDiscoveries returns the discovery endpoints.
func (srv *server) GetDiscoveries() []string { return srv.Discoveries }

// SetDiscoveries sets the discovery endpoints.
func (srv *server) SetDiscoveries(discoveries []string) { srv.Discoveries = discoveries }

// Dist delegates to globular.Dist to package the service.
func (srv *server) Dist(path string) (string, error) { return globular.Dist(path, srv) }

// GetDependencies returns the required service dependencies.
func (srv *server) GetDependencies() []string {
	if srv.Dependencies == nil {
		srv.Dependencies = make([]string, 0)
	}
	return srv.Dependencies
}

// SetDependency appends a dependency if not already present.
func (srv *server) SetDependency(dependency string) {
	if srv.Dependencies == nil {
		srv.Dependencies = make([]string, 0)
	}
	if !Utility.Contains(srv.Dependencies, dependency) {
		srv.Dependencies = append(srv.Dependencies, dependency)
	}
}

// GetChecksum returns the package checksum.
func (srv *server) GetChecksum() string { return srv.Checksum }

// SetChecksum sets the package checksum.
func (srv *server) SetChecksum(checksum string) { srv.Checksum = checksum }

// GetPlatform returns the target platform string.
func (srv *server) GetPlatform() string { return srv.Plaform }

// SetPlatform sets the target platform string.
func (srv *server) SetPlatform(platform string) { srv.Plaform = platform }

// GetPath returns the service executable path.
func (srv *server) GetPath() string { return srv.Path }

// SetPath sets the service executable path.
func (srv *server) SetPath(path string) { srv.Path = path }

// GetProto returns the .proto path.
func (srv *server) GetProto() string { return srv.Proto }

// SetProto sets the .proto path.
func (srv *server) SetProto(proto string) { srv.Proto = proto }

// GetPort returns the gRPC port.
func (srv *server) GetPort() int { return srv.Port }

// SetPort sets the gRPC port.
func (srv *server) SetPort(port int) { srv.Port = port }

// GetProxy returns the reverse proxy port (gRPC-Web).
func (srv *server) GetProxy() int { return srv.Proxy }

// SetProxy sets the reverse proxy port (gRPC-Web).
func (srv *server) SetProxy(proxy int) { srv.Proxy = proxy }

// GetProtocol returns the service protocol (grpc|http|https).
func (srv *server) GetProtocol() string { return srv.Protocol }

// SetProtocol sets the service protocol (grpc|http|https).
func (srv *server) SetProtocol(protocol string) { srv.Protocol = protocol }

// GetAllowAllOrigins returns true if all origins are allowed (CORS).
func (srv *server) GetAllowAllOrigins() bool { return srv.AllowAllOrigins }

// SetAllowAllOrigins sets whether all origins are allowed (CORS).
func (srv *server) SetAllowAllOrigins(allowAllOrigins bool) { srv.AllowAllOrigins = allowAllOrigins }

// GetAllowedOrigins returns the comma-separated allowed origins.
func (srv *server) GetAllowedOrigins() string { return srv.AllowedOrigins }

// SetAllowedOrigins sets the comma-separated allowed origins.
func (srv *server) SetAllowedOrigins(allowedOrigins string) { srv.AllowedOrigins = allowedOrigins }

// GetDomain returns the domain (host) name.
func (srv *server) GetDomain() string { return srv.Domain }

// SetDomain sets the domain (host) name.
func (srv *server) SetDomain(domain string) { srv.Domain = domain }

// GetTls returns true if TLS is enabled.
func (srv *server) GetTls() bool { return srv.TLS }

// SetTls enables/disables TLS.
func (srv *server) SetTls(hasTls bool) { srv.TLS = hasTls }

// GetCertAuthorityTrust returns the CA bundle path.
func (srv *server) GetCertAuthorityTrust() string { return srv.CertAuthorityTrust }

// SetCertAuthorityTrust sets the CA bundle path.
func (srv *server) SetCertAuthorityTrust(ca string) { srv.CertAuthorityTrust = ca }

// GetCertFile returns the TLS cert file path.
func (srv *server) GetCertFile() string { return srv.CertFile }

// SetCertFile sets the TLS cert file path.
func (srv *server) SetCertFile(certFile string) { srv.CertFile = certFile }

// GetKeyFile returns the TLS key file path.
func (srv *server) GetKeyFile() string { return srv.KeyFile }

// SetKeyFile sets the TLS key file path.
func (srv *server) SetKeyFile(keyFile string) { srv.KeyFile = keyFile }

// GetVersion returns the service version.
func (srv *server) GetVersion() string { return srv.Version }

// SetVersion sets the service version.
func (srv *server) SetVersion(version string) { srv.Version = version }

// GetPublisherID returns the publisher ID.
func (srv *server) GetPublisherID() string { return srv.PublisherID }

// SetPublisherID sets the publisher ID.
func (srv *server) SetPublisherID(PublisherID string) { srv.PublisherID = PublisherID }

// GetKeepUpToDate returns true if auto-update is enabled.
func (srv *server) GetKeepUpToDate() bool { return srv.KeepUpToDate }

// SetKeepUptoDate sets auto-update behavior.
func (srv *server) SetKeepUptoDate(val bool) { srv.KeepUpToDate = val }

// GetKeepAlive returns true if the service should be kept alive.
func (srv *server) GetKeepAlive() bool { return srv.KeepAlive }

// SetKeepAlive sets whether the service should be kept alive.
func (srv *server) SetKeepAlive(val bool) { srv.KeepAlive = val }

// GetPermissions returns the action permissions.
func (srv *server) GetPermissions() []interface{} { return srv.Permissions }

// SetPermissions sets the action permissions.
func (srv *server) SetPermissions(permissions []interface{}) { srv.Permissions = permissions }


func (srv *server) RolesDefault() []resourcepb.Role {
	domain, _ := config.GetDomain()

	return []resourcepb.Role{
		{
			Id:          "role:services.viewer",
			Name:        "Services Viewer",
			Domain:      domain,
			Description: "Read-only visibility into services and their actions.",
			Actions: []string{
				"/services_manager.ServicesManagerService/GetServicesConfiguration",
				"/services_manager.ServicesManagerService/GetAllActions",
			},
			TypeName: "resource.Role",
		},
		{
			Id:          "role:services.operator",
			Name:        "Services Operator",
			Domain:      domain,
			Description: "Can start/stop instances and view configuration.",
			Actions: []string{
				"/services_manager.ServicesManagerService/GetServicesConfiguration",
				"/services_manager.ServicesManagerService/GetAllActions",
				"/services_manager.ServicesManagerService/StartServiceInstance",
				"/services_manager.ServicesManagerService/StopServiceInstance",
			},
			TypeName: "resource.Role",
		},
		{
			Id:          "role:services.admin",
			Name:        "Services Admin",
			Domain:      domain,
			Description: "Full control over service lifecycle and configuration.",
			Actions: []string{
				"/services_manager.ServicesManagerService/InstallService",
				"/services_manager.ServicesManagerService/UninstallService",
				"/services_manager.ServicesManagerService/StartServiceInstance",
				"/services_manager.ServicesManagerService/StopServiceInstance",
				"/services_manager.ServicesManagerService/RestartAllServices",
				"/services_manager.ServicesManagerService/GetServicesConfiguration",
				"/services_manager.ServicesManagerService/GetAllActions",
				"/services_manager.ServicesManagerService/SaveServiceConfig",
			},
			TypeName: "resource.Role",
		},
	}
}

// -----------------------------------------------------------------------------
// Lifecycle
// -----------------------------------------------------------------------------

// Init creates/loads configuration and initializes the gRPC server.
func (srv *server) Init() error {
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

// Save persists configuration values.
func (srv *server) Save() error { return globular.SaveService(srv) }

// StartService starts the gRPC server.
func (srv *server) StartService() error { return globular.StartService(srv, srv.grpcServer) }

// StopService stops the gRPC server.
func (srv *server) StopService() error { return globular.StopService(srv, srv.grpcServer) }

// -----------------------------------------------------------------------------
// Clients & event helpers
// -----------------------------------------------------------------------------

func (srv *server) getEventClient(address string) (*event_client.Event_Client, error) {
	Utility.RegisterFunction("NewEventService_Client", event_client.NewEventService_Client)
	client, err := globular_client.GetClient(address, "event.EventService", "NewEventService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*event_client.Event_Client), nil
}

func (srv *server) publishUpdateServiceConfigEvent(cfg map[string]interface{}) error {
	data, err := json.Marshal(cfg)
	if err != nil {
		return err
	}
	client, err := srv.getEventClient(srv.Domain)
	if err != nil {
		return err
	}
	return client.Publish("update_globular_service_configuration_evt", data)
}

func (srv *server) publish(domain, event string, data []byte) error {
	eventClient, err := srv.getEventClient(domain)
	if err != nil {
		logger.Error("get event client failed", "domain", domain, "err", err)
		return err
	}
	if err := eventClient.Publish(event, data); err != nil {
		logger.Error("publish event failed", "event", event, "domain", domain, "err", err)
		return err
	}
	return nil
}

func (srv *server) subscribe(domain, evt string, listener func(evt *eventpb.Event)) error {
	eventClient, err := srv.getEventClient(domain)
	if err != nil {
		logger.Error("get event client failed", "domain", domain, "err", err)
		return err
	}
	if err := eventClient.Subscribe(evt, srv.Id, listener); err != nil {
		logger.Error("subscribe to event failed", "event", evt, "domain", domain, "err", err)
		return err
	}
	return nil
}

func (srv *server) getResourceClient() (*resource_client.Resource_Client, error) {
	Utility.RegisterFunction("NewResourceService_Client", resource_client.NewResourceService_Client)
	client, err := globular_client.GetClient(srv.Address, "resource.ResourceService", "NewResourceService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*resource_client.Resource_Client), nil
}

// RBAC

// GetRbacClient returns the RBAC client.
func (srv *server) GetRbacClient() (*rbac_client.Rbac_Client, error) {
	Utility.RegisterFunction("NewRbacService_Client", rbac_client.NewRbacService_Client)
	client, err := globular_client.GetClient(srv.Address, "rbac.RbacService", "NewRbacService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*rbac_client.Rbac_Client), nil
}

func (srv *server) setActionResourcesPermissions(permissions map[string]interface{}) error {
	r, err := srv.GetRbacClient()
	if err != nil {
		return err
	}
	return r.SetActionResourcesPermissions(permissions)
}

// Log service

// GetLogClient returns the Log client.
func (srv *server) GetLogClient() (*log_client.Log_Client, error) {
	Utility.RegisterFunction("NewLogService_Client", log_client.NewLogService_Client)
	client, err := globular_client.GetClient(srv.Address, "log.LogService", "NewLogService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*log_client.Log_Client), nil
}

func (srv *server) logServiceInfo(method, fileLine, functionName, infos string) error {
	lc, err := srv.GetLogClient()
	if err != nil {
		return err
	}
	return lc.Log(srv.Name, srv.Domain, method, logpb.LogLevel_INFO_MESSAGE, infos, fileLine, functionName)
}

func (srv *server) logServiceError(method, fileLine, functionName, infos string) error {
	lc, err := srv.GetLogClient()
	if err != nil {
		return err
	}
	return lc.Log(srv.Name, srv.Address, method, logpb.LogLevel_ERROR_MESSAGE, infos, fileLine, functionName)
}

// Resource helpers

func (srv *server) removeRolesAction(action string) error {
	rc, err := srv.getResourceClient()
	if err != nil {
		return err
	}
	return rc.RemoveRolesAction(action)
}

func (srv *server) removeApplicationsAction(token, action string) error {
	rc, err := srv.getResourceClient()
	if err != nil {
		return err
	}
	return rc.RemoveApplicationsAction(token, action)
}

func (srv *server) removePeersAction(action string) error {
	rc, err := srv.getResourceClient()
	if err != nil {
		return err
	}
	return rc.RemovePeersAction("", action)
}

func (srv *server) setRoleActions(roleId string, actions []string) error {
	rc, err := srv.getResourceClient()
	if err != nil {
		return err
	}
	return rc.AddRoleActions(roleId, actions)
}

// -----------------------------------------------------------------------------
// Service manager helpers
// -----------------------------------------------------------------------------

func (srv *server) stopService(s map[string]interface{}) error {
	if err := process.KillServiceProcess(s); err != nil {
		return err
	}

	// Update config
	s["State"] = "killed"
	s["Process"] = -1
	s["ProxyProcess"] = -1

	if err := config.SaveServiceConfiguration(s); err != nil {
		return err
	}
	return srv.publishUpdateServiceConfigEvent(s)
}

func (srv *server) uninstallService(token, PublisherID, serviceId, version string, deletePermissions bool) error {
	services, err := config.GetServicesConfigurations()
	if err != nil {
		return err
	}

	for _, s := range services {
		pub := s["PublisherID"].(string)
		id := s["Id"].(string)
		ver := s["Version"].(string)
		name := s["Name"].(string)

		if pub == PublisherID && id == serviceId && ver == version {
			// Stop service
			_ = srv.stopService(s)

			// Get actions to delete
			toDelete, err := config.GetServiceMethods(name, PublisherID, version)
			if err != nil {
				return err
			}

			if deletePermissions {
				for _, act := range toDelete {
					_ = srv.removeRolesAction(act)
					_ = srv.removeApplicationsAction(token, act)
					_ = srv.removePeersAction(act)
				}
			}

			// refresh local methods set
			methods := make([]string, 0, len(srv.methods))
			for _, m := range srv.methods {
				if !Utility.Contains(toDelete, m) {
					methods = append(methods, m)
				}
			}
			srv.methods = methods
			if err := srv.registerMethods(); err != nil {
				logger.Warn("register methods after uninstall failed", "err", err)
			}

			// Remove files
			path := filepath.ToSlash(filepath.Join(srv.Root, "services", PublisherID, name, version, serviceId))
			if Utility.Exists(path) {
				if err := os.RemoveAll(path); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func (srv *server) registerMethods() error {
	// Ensure 'sa' role has all actions
	return srv.setRoleActions("sa", srv.methods)
}

// updateService reacts to repo events and updates a service if KeepUpToDate is true.
func updateService(srv *server, service map[string]interface{}) func(evt *eventpb.Event) {
	return func(evt *eventpb.Event) {
		logger.Info("update service event received", "event", string(evt.Name))

		kup, _ := service["KeepUpToDate"].(bool)
		if !kup {
			return
		}

		descriptor := new(resourcepb.PackageDescriptor)
		if err := protojson.Unmarshal(evt.Data, descriptor); err != nil {
			logger.Error("parse package descriptor failed", "err", err)
			return
		}

		logger.Info("updating service",
			"name", descriptor.Name,
			"PublisherID", descriptor.PublisherID,
			"id", descriptor.Id,
			"version", descriptor.Version)

		token, err := security.GetLocalToken(srv.Mac)
		if err != nil {
			logger.Error("get local token failed", "err", err)
			return
		}

		if srv.stopService(service) == nil {
			if srv.uninstallService(token, descriptor.PublisherID, descriptor.Id, service["Version"].(string), true) == nil {
				if err := srv.installService(token, descriptor); err != nil {
					logger.Error("service update failed", "err", err)
				} else {
					logger.Info("service updated", "name", service["Name"])
				}
			}
		}
	}
}

// -----------------------------------------------------------------------------
// CLI helpers: --describe and --health
// -----------------------------------------------------------------------------

type describePayload struct {
	Id           string   `json:"Id"`
	Name         string   `json:"Name"`
	PublisherID  string   `json:"PublisherID"`
	Version      string   `json:"Version"`
	Description  string   `json:"Description"`
	Keywords     []string `json:"Keywords"`
	Dependencies []string `json:"Dependencies"`
	Protocol     string   `json:"Protocol"`
	DefaultPort  int      `json:"DefaultPort"`
	DefaultProxy int      `json:"DefaultProxy"`
	ServiceIdArg string   `json:"ServiceIdArgHint"`
}

func (srv *server) describeJSON() []byte {
	p := describePayload{
		Id:           srv.Id,
		Name:         srv.Name,
		PublisherID:  srv.PublisherID,
		Version:      srv.Version,
		Description:  srv.Description,
		Keywords:     srv.Keywords,
		Dependencies: srv.Dependencies,
		Protocol:     "grpc",
		DefaultPort:  defaultPort,
		DefaultProxy: defaultProxy,
		ServiceIdArg: "<service-id>",
	}
	b, _ := json.MarshalIndent(p, "", "  ")
	return b
}

// healthCheck performs a lightweight self-check.
func (srv *server) healthCheck() error {
	// Minimal checks that don’t require network:
	if srv.Name == "" || srv.Proto == "" {
		return fmt.Errorf("missing required fields: Name or Proto")
	}
	if srv.Root == "" {
		return fmt.Errorf("missing Root path")
	}
	return nil
}

// -----------------------------------------------------------------------------
// main
// -----------------------------------------------------------------------------

// main configures and starts the ServicesManager service.
// Supports:
//
//	--describe : print a JSON descriptor and exit 0
//	--health   : run a lightweight health-check and exit 0/1
func main() {
	// Basic flags for utility behavior (do not change service public API).
	describe := flag.Bool("describe", false, "Print a JSON service descriptor and exit.")
	health := flag.Bool("health", false, "Run a lightweight health-check and exit.")
	flag.Parse()

	// Service init with defaults
	s := &server{
		Name:            string(services_managerpb.File_services_manager_proto.Services().Get(0).FullName()),
		Proto:           services_managerpb.File_services_manager_proto.Path(),
		Port:            defaultPort,
		Proxy:           defaultProxy,
		Protocol:        "grpc",
		Version:         "0.0.1",
		PublisherID:     "localhost",
		Description:     "Microservice manager service",
		Keywords:        []string{"Manager", "Service"},
		Repositories:    []string{},
		Discoveries:     []string{},
		Dependencies:    []string{"resource.ResourceService", "rbac.RbacService", "event.EventService"},
		Permissions:     []interface{}{},
		Process:         -1,
		ProxyProcess:    -1,
		KeepAlive:       true,
		KeepUpToDate:    true,
		AllowAllOrigins: allowAllOriginsDef,
		AllowedOrigins:  allowedOriginsDef,
		methods:         []string{},
		PortsRange:      "10000-10100",
		done:            make(chan bool),
	}

	s.Permissions = []interface{}{
		// ---- Install a service (create/modify service resources)
		map[string]interface{}{
			"action":     "/services_manager.ServicesManagerService/InstallService",
			"permission": "admin",
			"resources": []interface{}{
				// InstallServiceRequest.serviceId
				map[string]interface{}{"index": 0, "field": "ServiceId", "permission": "admin"},
				// InstallServiceRequest.PublisherID
				map[string]interface{}{"index": 0, "field": "PublisherID", "permission": "admin"},
				// InstallServiceRequest.version
				map[string]interface{}{"index": 0, "field": "Version", "permission": "admin"},
				// Note: dicorveryId is not a protected resource path; action-level control is enough.
			},
		},

		// ---- Uninstall a service (destructive)
		map[string]interface{}{
			"action":     "/services_manager.ServicesManagerService/UninstallService",
			"permission": "admin",
			"resources": []interface{}{
				// UninstallServiceRequest.serviceId
				map[string]interface{}{"index": 0, "field": "ServiceId", "permission": "admin"},
				// UninstallServiceRequest.PublisherID
				map[string]interface{}{"index": 0, "field": "PublisherID", "permission": "admin"},
				// UninstallServiceRequest.version
				map[string]interface{}{"index": 0, "field": "Version", "permission": "admin"},
				// deletePermissions is a flag, not a resource.
			},
		},

		// ---- Stop a service instance
		map[string]interface{}{
			"action":     "/services_manager.ServicesManagerService/StopServiceInstance",
			"permission": "admin",
			"resources": []interface{}{
				// StopServiceInstanceRequest.service_id
				map[string]interface{}{"index": 0, "field": "ServiceId", "permission": "admin"},
			},
		},

		// ---- Start a service instance
		map[string]interface{}{
			"action":     "/services_manager.ServicesManagerService/StartServiceInstance",
			"permission": "admin",
			"resources": []interface{}{
				// StartServiceInstanceRequest.service_id
				map[string]interface{}{"index": 0, "field": "ServiceId", "permission": "admin"},
			},
		},

		// ---- Restart all services (global op, no per-resource param)
		map[string]interface{}{
			"action":     "/services_manager.ServicesManagerService/RestartAllServices",
			"permission": "admin",
			"resources":  []interface{}{},
		},

		// ---- Get services configuration (read-only)
		map[string]interface{}{
			"action":     "/services_manager.ServicesManagerService/GetServicesConfiguration",
			"permission": "read",
			"resources":  []interface{}{},
		},

		// ---- List all actions (read-only)
		map[string]interface{}{
			"action":     "/services_manager.ServicesManagerService/GetAllActions",
			"permission": "read",
			"resources":  []interface{}{},
		},

		// ---- Save a service configuration (writes config blob; no path field)
		map[string]interface{}{
			"action":     "/services_manager.ServicesManagerService/SaveServiceConfig",
			"permission": "admin",
			"resources":  []interface{}{},
		},
	}

	// Paths & environment
	s.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	s.Domain, _ = config.GetDomain()
	s.Address, _ = config.GetAddress()
	s.Root = filepath.ToSlash(config.GetGlobularExecPath())
	s.DataPath = config.GetDataDir()
	s.Creds = filepath.ToSlash(filepath.Join(config.GetConfigDir(), "tls"))

	// Parse positional args: <Id> [ConfigPath]
	if len(os.Args) >= 2 {
		// If first non-flag arg is set, treat it as ID.
		// Caveat: flag package consumes flags; we need to detect extras.
		// We’ll read from flag.Args() for non-flag leftovers.
		args := flag.Args()
		if len(args) >= 1 {
			s.Id = args[0]
		}
		if len(args) >= 2 {
			s.ConfigPath = args[1]
		}
	}

	// Utility registration for client factory
	Utility.RegisterFunction("NewServicesManagerService_Client", service_manager_client.NewServicesManagerService_Client)

	// Quick utility modes
	if *describe {
		// If Id not provided, emit a neutral one for tooling.
		if s.Id == "" {
			s.Id = "services_manager"
		}
		os.Stdout.Write(s.describeJSON())
		os.Stdout.Write([]byte("\n"))
		return
	}

	if *health {
		if err := s.healthCheck(); err != nil {
			logger.Error("health check failed", "err", err)
			os.Exit(1)
		}
		logger.Info("ok")
		return
	}

	// Initialize and start gRPC service
	if err := s.Init(); err != nil {
		logger.Error("initialize service failed", "name", s.Name, "id", s.Id, "err", err)
		os.Exit(1)
	}

	// Normalize Address if still empty
	if s.Address == "" {
		s.Address, _ = config.GetAddress()
	}

	// Register gRPC server
	services_managerpb.RegisterServicesManagerServiceServer(s.grpcServer, s)
	reflection.Register(s.grpcServer)

	// Subscribe to repo events to keep apps up to date
	go func() {
		services, err := config.GetServicesConfigurations()
		if err != nil {
			logger.Warn("get services configurations failed", "err", err)
			return
		}
		for _, svc := range services {
			pub := svc["PublisherID"].(string)
			evt := pub + ":" + svc["Id"].(string)
			values := strings.Split(pub, "@")
			if len(values) == 2 {
				if err := s.subscribe(values[1], evt, updateService(s, svc)); err != nil {
					logger.Warn("subscribe to update event failed", "publisher", values[1], "evt", evt, "err", err)
				}
			}
		}
	}()

	// Start serving
	if err := s.StartService(); err != nil {
		logger.Error("start service failed", "name", s.Name, "id", s.Id, "err", err)
		os.Exit(1)
	}
}
