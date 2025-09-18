// Package main provides the Services Manager gRPC service wired for Globular.
// It aligns with the Echo example style: early CLI gating (--describe/--health)
// before touching config/etcd, structured logging via slog, and a clean startup.
package main

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

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
	service_manager_client "github.com/globulario/services/golang/services_manager/services_manager_client"
	"github.com/globulario/services/golang/services_manager/services_managerpb"
	Utility "github.com/globulario/utility"
)

// -----------------------------------------------------------------------------
// Defaults & logging
// -----------------------------------------------------------------------------

var (
	defaultPort        = 10000
	defaultProxy       = defaultPort + 1
	allowAllOriginsDef = true
	allowedOriginsDef  = ""

	logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
)

// -----------------------------------------------------------------------------
// Server
// -----------------------------------------------------------------------------

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
// Globular contract (getters/setters)
// -----------------------------------------------------------------------------

func (srv *server) GetConfigurationPath() string          { return srv.ConfigPath }
func (srv *server) SetConfigurationPath(path string)      { srv.ConfigPath = path }
func (srv *server) GetAddress() string                    { return srv.Address }
func (srv *server) SetAddress(address string)             { srv.Address = address }
func (srv *server) GetProcess() int                       { return srv.Process }
func (srv *server) SetProcess(pid int)                    { srv.Process = pid }
func (srv *server) GetProxyProcess() int                  { return srv.ProxyProcess }
func (srv *server) SetProxyProcess(pid int)               { srv.ProxyProcess = pid }
func (srv *server) GetState() string                      { return srv.State }
func (srv *server) SetState(state string)                 { srv.State = state }
func (srv *server) GetLastError() string                  { return srv.LastError }
func (srv *server) SetLastError(err string)               { srv.LastError = err }
func (srv *server) SetModTime(modtime int64)              { srv.ModTime = modtime }
func (srv *server) GetModTime() int64                     { return srv.ModTime }
func (srv *server) GetId() string                         { return srv.Id }
func (srv *server) SetId(id string)                       { srv.Id = id }
func (srv *server) GetName() string                       { return srv.Name }
func (srv *server) SetName(name string)                   { srv.Name = name }
func (srv *server) GetMac() string                        { return srv.Mac }
func (srv *server) SetMac(mac string)                     { srv.Mac = mac }
func (srv *server) GetDescription() string                { return srv.Description }
func (srv *server) SetDescription(description string)     { srv.Description = description }
func (srv *server) GetKeywords() []string                 { return srv.Keywords }
func (srv *server) SetKeywords(keywords []string)         { srv.Keywords = keywords }
func (srv *server) GetRepositories() []string             { return srv.Repositories }
func (srv *server) SetRepositories(repositories []string) { srv.Repositories = repositories }
func (srv *server) GetDiscoveries() []string              { return srv.Discoveries }
func (srv *server) SetDiscoveries(discoveries []string)   { srv.Discoveries = discoveries }
func (srv *server) Dist(path string) (string, error)      { return globular.Dist(path, srv) }
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
func (srv *server) GetProtocol() string                      { return srv.Protocol }
func (srv *server) SetProtocol(protocol string)              { srv.Protocol = protocol }
func (srv *server) GetAllowAllOrigins() bool                 { return srv.AllowAllOrigins }
func (srv *server) SetAllowAllOrigins(v bool)                { srv.AllowAllOrigins = v }
func (srv *server) GetAllowedOrigins() string                { return srv.AllowedOrigins }
func (srv *server) SetAllowedOrigins(v string)               { srv.AllowedOrigins = v }
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
func (srv *server) GetKeepAlive() bool                       { return srv.KeepAlive }
func (srv *server) SetKeepAlive(val bool)                    { srv.KeepAlive = val }
func (srv *server) GetPermissions() []interface{}            { return srv.Permissions }
func (srv *server) SetPermissions(permissions []interface{}) { srv.Permissions = permissions }

// -----------------------------------------------------------------------------
// Roles defaults
// -----------------------------------------------------------------------------

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
func (srv *server) Save() error         { return globular.SaveService(srv) }
func (srv *server) StartService() error { return globular.StartService(srv, srv.grpcServer) }
func (srv *server) StopService() error  { return globular.StopService(srv, srv.grpcServer) }

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
	s["State"] = "killed"
	s["Process"] = -1
	s["ProxyProcess"] = -1
	if err := config.SaveServiceConfiguration(s); err != nil {
		return err
	}
	return srv.publishUpdateServiceConfigEvent(s)
}

func (srv *server) registerMethods() error {
	return srv.setRoleActions("sa", srv.methods)
}

// -----------------------------------------------------------------------------
// CLI helpers: --describe / --health (Echo-style gating)
// -----------------------------------------------------------------------------

// lightweight self check for --health that avoids network calls
func (srv *server) healthCheck() error {
	if srv.Name == "" || srv.Proto == "" {
		return fmt.Errorf("missing required fields: Name or Proto")
	}
	if srv.Path == "" {
		return fmt.Errorf("missing Path")
	}
	return nil
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  services_manager [service_id] [config_path]")
	fmt.Println("Options:")
	fmt.Println("  --describe    Print service metadata as JSON and exit")
	fmt.Println("  --health      Print service health as JSON and exit")
	fmt.Println("  --debug       Enable debug logging")
	fmt.Println("  --version     Print version")
}

// -----------------------------------------------------------------------------
// main
// -----------------------------------------------------------------------------

func main() {
	s := new(server)

	// Fill ONLY fields that do NOT require calling config/etcd yet.
	s.Name = string(services_managerpb.File_services_manager_proto.Services().Get(0).FullName())
	s.Proto = services_managerpb.File_services_manager_proto.Path()
	s.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	s.Port = defaultPort
	s.Proxy = defaultProxy
	s.Protocol = "grpc"
	s.Version = "0.0.1"
	s.PublisherID = "localhost"
	s.Description = "Microservice manager service"
	s.Keywords = []string{"Manager", "Service"}
	s.Repositories = make([]string, 0)
	s.Discoveries = make([]string, 0)
	s.Dependencies = []string{"resource.ResourceService", "rbac.RbacService", "event.EventService"}
	s.Permissions = make([]interface{}, 0)
	s.Process = -1
	s.ProxyProcess = -1
	s.KeepAlive = true
	s.KeepUpToDate = true
	s.AllowAllOrigins = allowAllOriginsDef
	s.AllowedOrigins = allowedOriginsDef
	s.methods = []string{}
	s.PortsRange = "10000-10100"
	s.done = make(chan bool)

	// Permissions (kept as in your original)
	s.Permissions = []interface{}{
		map[string]interface{}{
			"action":     "/services_manager.ServicesManagerService/InstallService",
			"permission": "admin",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "ServiceId", "permission": "admin"},
				map[string]interface{}{"index": 0, "field": "PublisherID", "permission": "admin"},
				map[string]interface{}{"index": 0, "field": "Version", "permission": "admin"},
			},
		},
		map[string]interface{}{
			"action":     "/services_manager.ServicesManagerService/UninstallService",
			"permission": "admin",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "ServiceId", "permission": "admin"},
				map[string]interface{}{"index": 0, "field": "PublisherID", "permission": "admin"},
				map[string]interface{}{"index": 0, "field": "Version", "permission": "admin"},
			},
		},
		map[string]interface{}{
			"action":     "/services_manager.ServicesManagerService/StopServiceInstance",
			"permission": "admin",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "ServiceId", "permission": "admin"},
			},
		},
		map[string]interface{}{
			"action":     "/services_manager.ServicesManagerService/StartServiceInstance",
			"permission": "admin",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "ServiceId", "permission": "admin"},
			},
		},
		map[string]interface{}{
			"action":     "/services_manager.ServicesManagerService/RestartAllServices",
			"permission": "admin",
			"resources":  []interface{}{},
		},
		map[string]interface{}{
			"action":     "/services_manager.ServicesManagerService/GetServicesConfiguration",
			"permission": "read",
			"resources":  []interface{}{},
		},
		map[string]interface{}{
			"action":     "/services_manager.ServicesManagerService/GetAllActions",
			"permission": "read",
			"resources":  []interface{}{},
		},
		map[string]interface{}{
			"action":     "/services_manager.ServicesManagerService/SaveServiceConfig",
			"permission": "admin",
			"resources":  []interface{}{},
		},
	}

	// ---- CLI gating BEFORE config/etcd usage ----
	args := os.Args[1:]

	// If no args (common supervisor case), allocate a unique port early.
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
			// Only compute ephemeral data here; avoid etcd
			s.Process = os.Getpid()
			s.State = "starting"
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
			b, err := globular.DescribeJSON(s)
			if err != nil {
				logger.Error("describe error", "service", s.Name, "id", s.Id, "err", err)
				os.Exit(2)
			}
			_, _ = os.Stdout.Write(b)
			_, _ = os.Stdout.Write([]byte("\n"))
			return


		case "--health":
			if err := s.healthCheck(); err != nil {
				logger.Error("health check failed", "err", err)
				os.Exit(1)
			}
			// If you want parity with Echo’s HealthJSON, keep the minimal OK.
			_, _ = os.Stdout.Write([]byte(`{"ok":true}` + "\n"))
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
			// positional args handled later
		}
	}

	// Positional args: <Id> [ConfigPath]
	if len(args) == 1 && !strings.HasPrefix(args[0], "-") {
		s.Id = args[0]
	} else if len(args) == 2 && !strings.HasPrefix(args[0], "-") && !strings.HasPrefix(args[1], "-") {
		s.Id = args[0]
		s.ConfigPath = args[1]
	}

	// Safe to read local config now.
	if d, err := config.GetDomain(); err == nil {
		s.Domain = d
	} else {
		s.Domain = "localhost"
	}
	if a, err := config.GetAddress(); err == nil && strings.TrimSpace(a) != "" {
		s.Address = a
	}
	s.Root = filepath.ToSlash(config.GetGlobularExecPath())
	s.DataPath = config.GetDataDir()
	s.Creds = filepath.ToSlash(filepath.Join(config.GetConfigDir(), "tls"))

	// Utility registration for client factory
	Utility.RegisterFunction("NewServicesManagerService_Client", service_manager_client.NewServicesManagerService_Client)

	start := time.Now()
	if err := s.Init(); err != nil {
		logger.Error("initialize service failed", "name", s.Name, "id", s.Id, "err", err)
		os.Exit(1)
	}

	// Register gRPC server + reflection
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

	logger.Info("service ready",
		"service", s.Name,
		"port", s.Port,
		"proxy", s.Proxy,
		"protocol", s.Protocol,
		"domain", s.Domain,
		"listen_ms", time.Since(start).Milliseconds())

	if err := s.StartService(); err != nil {
		logger.Error("start service failed", "name", s.Name, "id", s.Id, "err", err)
		os.Exit(1)
	}
}
