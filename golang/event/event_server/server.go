package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/event/event_client"
	"github.com/globulario/services/golang/event/eventpb"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/resource/resourcepb"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var (
	defaultPort       = 10050
	defaultProxy      = 10051
	allow_all_origins = true
	allowed_origins   = ""
)

// Version information (set via ldflags during build)
var (
	Version   = "0.0.1"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

var logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

const permissionsJSON = `[
  {"action":"/event.EventService/OnEvent","permission":"read","resources":[{"index":0,"field":"Uuid","permission":"read"}]},
  {"action":"/event.EventService/Quit","permission":"read","resources":[{"index":0,"field":"Uuid","permission":"read"}]},
  {"action":"/event.EventService/Subscribe","permission":"read","resources":[{"index":0,"field":"Name","permission":"read"}]},
  {"action":"/event.EventService/UnSubscribe","permission":"read","resources":[{"index":0,"field":"Name","permission":"read"}]},
  {"action":"/event.EventService/Publish","permission":"write","resources":[{"index":0,"field":"Evt.Name","permission":"write"}]},
  {"action":"/event.EventService/Stop","permission":"write","resources":[]}
]`

type server struct {
	Id                 string
	Name               string
	Mac                string
	Path               string
	Proto              string
	Port               int
	Proxy              int
	AllowAllOrigins    bool
	AllowedOrigins     string
	Protocol           string
	Domain             string
	Address            string
	Description        string
	Keywords           []string
	Repositories       []string
	Discoveries        []string
	Process            int
	ProxyProcess       int
	ConfigPath         string
	LastError          string
	ModTime            int64
	State              string
	CertFile           string
	KeyFile            string
	CertAuthorityTrust string
	TLS                bool
	Version            string
	PublisherID        string
	KeepUpToDate       bool
	Checksum           string
	Plaform            string
	KeepAlive          bool
	Permissions        []interface{}
	Dependencies       []string

	grpcServer *grpc.Server
	actions    chan map[string]interface{}
	exit       chan bool

	logger *slog.Logger
}

// Globular getters/setters
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
func (srv *server) Dist(path string) (string, error)  { return globular.Dist(path, srv) }
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
func (srv *server) SetAllowAllOrigins(allowAllOrigins bool)  { srv.AllowAllOrigins = allowAllOrigins }
func (srv *server) GetAllowedOrigins() string                { return srv.AllowedOrigins }
func (srv *server) SetAllowedOrigins(allowedOrigins string)  { srv.AllowedOrigins = allowedOrigins }
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
func (srv *server) GetRepositories() []string                { return srv.Repositories }
func (srv *server) SetRepositories(repositories []string)    { srv.Repositories = repositories }
func (srv *server) GetDiscoveries() []string                 { return srv.Discoveries }
func (srv *server) SetDiscoveries(discoveries []string)      { srv.Discoveries = discoveries }
func (srv *server) GetKeepUpToDate() bool                    { return srv.KeepUpToDate }
func (srv *server) SetKeepUptoDate(val bool)                 { srv.KeepUpToDate = val }
func (srv *server) GetKeepAlive() bool                       { return srv.KeepAlive }
func (srv *server) SetKeepAlive(val bool)                    { srv.KeepAlive = val }
func (srv *server) GetPermissions() []interface{}            { return srv.Permissions }
func (srv *server) SetPermissions(permissions []interface{}) { srv.Permissions = permissions }

func (srv *server) RolesDefault() []resourcepb.Role {
	domain, _ := config.GetDomain()

	reader := resourcepb.Role{
		Id:          "role:event.reader",
		Name:        "Event Reader",
		Domain:      domain,
		Description: "Subscribe to channels and receive events.",
		Actions: []string{
			"/event.EventService/OnEvent",
			"/event.EventService/Quit",
			"/event.EventService/Subscribe",
			"/event.EventService/UnSubscribe",
		},
		TypeName: "resource.Role",
	}

	publisher := resourcepb.Role{
		Id:          "role:event.publisher",
		Name:        "Event Publisher",
		Domain:      domain,
		Description: "Publish to channels (and read if allowed).",
		Actions: []string{
			"/event.EventService/Publish",
			"/event.EventService/OnEvent",
			"/event.EventService/Quit",
			"/event.EventService/Subscribe",
			"/event.EventService/UnSubscribe",
		},
		TypeName: "resource.Role",
	}

	admin := resourcepb.Role{
		Id:          "role:event.admin",
		Name:        "Event Admin",
		Domain:      domain,
		Description: "Full control over event channels and publishing.",
		Actions:     append([]string{"/event.EventService/Stop"}, publisher.Actions...),
		TypeName:    "resource.Role",
	}

	return []resourcepb.Role{reader, publisher, admin}
}

func loadDefaultPermissions() []interface{} {
	var out []interface{}
	_ = json.Unmarshal([]byte(permissionsJSON), &out)
	return out
}

func (srv *server) ensureRuntimeChannels() {
	if srv.actions == nil {
		srv.actions = make(chan map[string]interface{}, 1024)
	}
	if srv.exit == nil {
		srv.exit = make(chan bool)
	}
}

func (srv *server) Init() error {
	srv.ensureRuntimeChannels()

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

func (srv *server) Save() error { return globular.SaveService(srv) }

func (srv *server) StartService() error {
	srv.ensureRuntimeChannels()
	go srv.run()
	return globular.StartService(srv, srv.grpcServer)
}

func (srv *server) StopService() error {
	srv.ensureRuntimeChannels()
	select {
	case srv.exit <- true:
	default:
	}
	return globular.StopService(srv, srv.grpcServer)
}

func (srv *server) GetGrpcServer() *grpc.Server { return srv.grpcServer }

func initializeServerDefaults() *server {
	srv := new(server)
	srv.Name = string(eventpb.File_event_proto.Services().Get(0).FullName())
	srv.Proto = eventpb.File_event_proto.Path()
	srv.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	srv.Port = defaultPort
	srv.Proxy = defaultProxy
	srv.Protocol = "grpc"
	srv.Version = Version // Use build-time version
	srv.PublisherID = "localhost"
	srv.Description = "Event service with pub/sub messaging, event subscriptions, and real-time notifications"
	srv.Keywords = []string{"event", "pubsub", "subscribe", "publish", "messaging", "notifications", "realtime"}
	srv.Repositories = []string{}
	srv.Discoveries = []string{}
	srv.Dependencies = []string{}
	srv.Permissions = loadDefaultPermissions()
	srv.Process = -1
	srv.ProxyProcess = -1
	srv.KeepAlive = true
	srv.KeepUpToDate = true
	srv.AllowAllOrigins = allow_all_origins
	srv.AllowedOrigins = allowed_origins
	srv.logger = logger
	srv.ensureRuntimeChannels()

	srv.Domain, srv.Address = globular.GetDefaultDomainAddress(srv.Port)

	return srv
}

func setupGrpcService(srv *server) {
	eventpb.RegisterEventServiceServer(srv.grpcServer, srv)
	reflection.Register(srv.grpcServer)
}

func printUsage() {
	fmt.Println("Globular Event Service")
	fmt.Println()
	fmt.Println("USAGE:")
	fmt.Println("  event_server [OPTIONS] [<id> [configPath]]")
	fmt.Println()
	fmt.Println("OPTIONS:")
	fmt.Println("  --debug       Enable debug logging")
	fmt.Println("  --describe    Print service description as JSON and exit")
	fmt.Println("  --health      Print service health status as JSON and exit")
	fmt.Println("  --version     Print version information as JSON and exit")
	fmt.Println("  --help        Show this help message and exit")
	fmt.Println()
	fmt.Println("POSITIONAL ARGUMENTS:")
	fmt.Println("  id          Service instance ID (optional, auto-generated if not provided)")
	fmt.Println("  configPath  Path to service configuration file (optional)")
	fmt.Println()
	fmt.Println("ENVIRONMENT VARIABLES:")
	fmt.Println("  GLOBULAR_DOMAIN      Override service domain")
	fmt.Println("  GLOBULAR_ADDRESS     Override service address")
	fmt.Println()
	fmt.Println("FEATURES:")
	fmt.Println("  • Pub/Sub messaging with event subscriptions")
	fmt.Println("  • Real-time event notifications")
	fmt.Println("  • Multiple subscribers per event channel")
	fmt.Println("  • Event publishing with filtering")
	fmt.Println("  • Subscription management (subscribe/unsubscribe)")
	fmt.Println()
	fmt.Println("EXAMPLES:")
	fmt.Println("  # Start with auto-generated ID and default config")
	fmt.Println("  event_server")
	fmt.Println()
	fmt.Println("  # Start with specific service ID")
	fmt.Println("  event_server event-1")
	fmt.Println()
	fmt.Println("  # Enable debug logging")
	fmt.Println("  event_server --debug")
	fmt.Println()
	fmt.Println("  # Print service metadata")
	fmt.Println("  event_server --describe")
	fmt.Println()
	fmt.Println("  # Check service health")
	fmt.Println("  event_server --health")
}

func printVersion() {
	info := map[string]string{
		"service":    "event",
		"version":    Version,
		"build_time": BuildTime,
		"git_commit": GitCommit,
	}
	data, _ := json.MarshalIndent(info, "", "  ")
	fmt.Println(string(data))
}

func main() {
	srv := initializeServerDefaults()

	// Define CLI flags
	var (
		enableDebug  = flag.Bool("debug", false, "enable debug logging")
		showVersion  = flag.Bool("version", false, "print version information as JSON and exit")
		showHelp     = flag.Bool("help", false, "show usage information and exit")
		showDescribe = flag.Bool("describe", false, "print service description as JSON and exit")
		showHealth   = flag.Bool("health", false, "print service health status as JSON and exit")
	)

	flag.Usage = printUsage
	flag.Parse()

	// Apply debug logging if requested
	if *enableDebug {
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
		srv.logger = logger
		logger.Debug("debug logging enabled")
	}

	// Handle early-exit flags
	if *showHelp {
		printUsage()
		return
	}
	if *showVersion {
		printVersion()
		return
	}

	// Handle --describe flag
	if *showDescribe {
		srv.Process = os.Getpid()
		srv.State = "starting"
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
		if srv.Id == "" {
			srv.Id = Utility.GenerateUUID(srv.Name + ":" + srv.Address)
		}
		b, err := globular.DescribeJSON(srv)
		if err != nil {
			logger.Error("describe error", "service", srv.Name, "id", srv.Id, "err", err)
			os.Exit(2)
		}
		os.Stdout.Write(b)
		os.Stdout.Write([]byte("\n"))
		return
	}

	// Handle --health flag
	if *showHealth {
		if srv.Port == 0 || srv.Name == "" {
			logger.Error("health error: uninitialized", "service", srv.Name, "port", srv.Port)
			os.Exit(2)
		}
		b, err := globular.HealthJSON(srv, &globular.HealthOptions{Timeout: 1500 * time.Millisecond})
		if err != nil {
			logger.Error("health error", "service", srv.Name, "id", srv.Id, "err", err)
			os.Exit(2)
		}
		os.Stdout.Write(b)
		os.Stdout.Write([]byte("\n"))
		return
	}

	args := flag.Args()

	if err := globular.AllocatePortIfNeeded(srv, args); err != nil {
		logger.Error("port allocation failed", "error", err)
		os.Exit(1)
	}

	globular.ParsePositionalArgs(srv, args)
	globular.LoadRuntimeConfig(srv)

	Utility.RegisterFunction("NewEventService_Client", event_client.NewEventService_Client)

	logger.Info("starting event service",
		"service", srv.Name,
		"version", srv.Version,
		"domain", srv.Domain,
		"address", srv.Address,
		"port", srv.Port,
	)

	start := time.Now()
	logger.Debug("initializing service", "service", srv.Name, "id", srv.Id)
	if err := srv.Init(); err != nil {
		logger.Error("service init failed", "service", srv.Name, "id", srv.Id, "err", err)
		os.Exit(1)
	}
	logger.Debug("service init completed", "duration_ms", time.Since(start).Milliseconds())

	logger.Debug("registering gRPC handlers", "service", srv.Name)
	setupGrpcService(srv)
	logger.Debug("gRPC handlers registered")

	logger.Info("service ready",
		"service", srv.Name,
		"port", srv.Port,
		"proxy", srv.Proxy,
		"protocol", srv.Protocol,
		"domain", srv.Domain,
		"startup_ms", time.Since(start).Milliseconds(),
		"version", srv.Version,
	)

	lifecycle := globular.NewLifecycleManager(srv, logger)
	if err := lifecycle.Start(); err != nil {
		logger.Error("service start failed", "service", srv.Name, "err", err)
		os.Exit(1)
	}
}
