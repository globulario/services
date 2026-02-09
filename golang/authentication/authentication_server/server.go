package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/globulario/services/golang/authentication/authentication_client"
	"github.com/globulario/services/golang/authentication/authenticationpb"
	"github.com/globulario/services/golang/config"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/resource/resourcepb"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// Defaults
var (
	defaultPort  = 10000
	defaultProxy = defaultPort + 1

	allowAllOrigins   = true
	allowedOriginsStr = ""
)

// Version information (set via ldflags during build)
var (
	Version   = "0.0.1"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

// --- logger to STDERR so stdout stays clean for JSON outputs ---
var logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

// server implements the Globular service contract and Authentication RPC handlers.
type server struct {
	// Service identity
	Id          string
	Name        string
	Mac         string
	Domain      string
	Address     string
	Path        string
	Proto       string
	Port        int
	Proxy       int
	Protocol    string
	Version     string
	PublisherID string
	Description string
	Keywords    []string

	// Metadata & discovery
	Repositories []string
	Discoveries  []string
	Dependencies []string

	// Policy & operations
	AllowAllOrigins bool
	AllowedOrigins  string
	KeepUpToDate    bool
	KeepAlive       bool
	Checksum        string
	Plaform         string
	Permissions     []any

	// Runtime state
	Process      int
	ProxyProcess int
	ConfigPath   string
	ConfigPort   int
	LastError    string
	ModTime      int64
	State        string

	// TLS configuration
	TLS                bool
	CertFile           string
	KeyFile            string
	CertAuthorityTrust string

	// Auth-specific
	WatchSessionsDelay int
	SessionTimeout     int
	LdapConnectionId   string

	// gRPC runtime
	grpcServer *grpc.Server

	// Background controls
	exitCh           chan struct{}
	authentications_ []string
}

// --- Getters/Setters required by Globular (unchanged signatures) ---
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
func (srv *server) GetChecksum() string               { return srv.Checksum }
func (srv *server) SetChecksum(checksum string)       { srv.Checksum = checksum }
func (srv *server) GetPlatform() string               { return srv.Plaform }
func (srv *server) SetPlatform(platform string)       { srv.Plaform = platform }
func (srv *server) GetDescription() string            { return srv.Description }
func (srv *server) SetDescription(description string) { srv.Description = description }
func (srv *server) GetKeywords() []string             { return srv.Keywords }
func (srv *server) SetKeywords(keywords []string)     { srv.Keywords = keywords }
func (srv *server) GetConfigPort() int                { return srv.ConfigPort }
func (srv *server) SetConfigPort(port int)            { srv.ConfigPort = port }
func (srv *server) GetConfigAddress() string {
	domain := srv.GetAddress()
	if strings.Contains(domain, ":") {
		domain = strings.Split(domain, ":")[0]
	}
	return domain + ":" + Utility.ToString(srv.ConfigPort)
}
func (srv *server) GetRepositories() []string        { return srv.Repositories }
func (srv *server) SetRepositories(v []string)       { srv.Repositories = v }
func (srv *server) GetDiscoveries() []string         { return srv.Discoveries }
func (srv *server) SetDiscoveries(v []string)        { srv.Discoveries = v }
func (srv *server) Dist(path string) (string, error) { return globular.Dist(path, srv) }
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
func (srv *server) GetPath() string                 { return srv.Path }
func (srv *server) SetPath(path string)             { srv.Path = path }
func (srv *server) GetProto() string                { return srv.Proto }
func (srv *server) SetProto(proto string)           { srv.Proto = proto }
func (srv *server) GetPort() int                    { return srv.Port }
func (srv *server) SetPort(port int)                { srv.Port = port }
func (srv *server) GetProxy() int                   { return srv.Proxy }
func (srv *server) SetProxy(proxy int)              { srv.Proxy = proxy }
func (srv *server) GetProtocol() string             { return srv.Protocol }
func (srv *server) SetProtocol(protocol string)     { srv.Protocol = protocol }
func (srv *server) GetAllowAllOrigins() bool        { return srv.AllowAllOrigins }
func (srv *server) SetAllowAllOrigins(b bool)       { srv.AllowAllOrigins = b }
func (srv *server) GetAllowedOrigins() string       { return srv.AllowedOrigins }
func (srv *server) SetAllowedOrigins(s string)      { srv.AllowedOrigins = s }
func (srv *server) GetDomain() string               { return srv.Domain }
func (srv *server) SetDomain(domain string)         { srv.Domain = domain }
func (srv *server) GetTls() bool                    { return srv.TLS }
func (srv *server) SetTls(hasTls bool)              { srv.TLS = hasTls }
func (srv *server) GetCertAuthorityTrust() string   { return srv.CertAuthorityTrust }
func (srv *server) SetCertAuthorityTrust(ca string) { srv.CertAuthorityTrust = ca }
func (srv *server) GetCertFile() string             { return srv.CertFile }
func (srv *server) SetCertFile(certFile string)     { srv.CertFile = certFile }
func (srv *server) GetKeyFile() string              { return srv.KeyFile }
func (srv *server) SetKeyFile(keyFile string)       { srv.KeyFile = keyFile }
func (srv *server) GetVersion() string              { return srv.Version }
func (srv *server) SetVersion(version string)       { srv.Version = version }
func (srv *server) GetPublisherID() string          { return srv.PublisherID }
func (srv *server) SetPublisherID(p string)         { srv.PublisherID = p }
func (srv *server) GetKeepUpToDate() bool           { return srv.KeepUpToDate }
func (srv *server) SetKeepUptoDate(val bool)        { srv.KeepUpToDate = val }
func (srv *server) GetKeepAlive() bool              { return srv.KeepAlive }
func (srv *server) SetKeepAlive(val bool)           { srv.KeepAlive = val }
func (srv *server) GetPermissions() []any           { return srv.Permissions }
func (srv *server) SetPermissions(v []any)          { srv.Permissions = v }
func (srv *server) GetGrpcServer() *grpc.Server     { return srv.grpcServer }

// RolesDefault returns curated roles for AuthenticationService.
func (srv *server) RolesDefault() []resourcepb.Role {
	domain, _ := config.GetDomain()

	return []resourcepb.Role{
		{
			Id:          "role:auth.password",
			Name:        "Password Self-Service",
			Domain:      domain,
			Description: "Change account passwords (subject to server-side ownership checks).",
			Actions: []string{
				"/authentication.AuthenticationService/SetPassword",
			},
			TypeName: "resource.Role",
		},
		{
			Id:          "role:auth.peer-token",
			Name:        "Peer Token Issuer",
			Domain:      domain,
			Description: "Generate tokens for peers identified by MAC.",
			Actions: []string{
				"/authentication.AuthenticationService/GeneratePeerToken",
			},
			TypeName: "resource.Role",
		},
		{
			Id:          "role:auth.root",
			Name:        "Root Credential Manager",
			Domain:      domain,
			Description: "Manage root credentials and administrator email.",
			Actions: []string{
				"/authentication.AuthenticationService/SetRootPassword",
				"/authentication.AuthenticationService/SetRootEmail",
			},
			TypeName: "resource.Role",
		},
		{
			Id:          "role:auth.admin",
			Name:        "Authentication Admin",
			Domain:      domain,
			Description: "Full control over authentication management operations.",
			Actions: []string{
				"/authentication.AuthenticationService/SetPassword",
				"/authentication.AuthenticationService/GeneratePeerToken",
				"/authentication.AuthenticationService/SetRootPassword",
				"/authentication.AuthenticationService/SetRootEmail",
			},
			TypeName: "resource.Role",
		},
	}
}

// Lifecycle
func (srv *server) Init() error {
	if err := globular.InitService(srv); err != nil {
		return err
	}
	gs, err := globular.InitGrpcServer(srv) // interceptors wired internally
	if err != nil {
		return err
	}
	srv.grpcServer = gs
	return nil
}

func (srv *server) Save() error { return globular.SaveService(srv) }

func (srv *server) StartService() error {
	if srv.exitCh == nil {
		srv.exitCh = make(chan struct{})
	}
	srv.removeExpiredSessions()

	macAddress, err := config.GetMacAddress()
	if err != nil {
		close(srv.exitCh)
		srv.exitCh = nil
		return err
	}

	if err := srv.setKey(macAddress); err != nil {
		close(srv.exitCh)
		srv.exitCh = nil
		return err
	}

	return globular.StartService(srv, srv.grpcServer)
}

func (srv *server) StopService() error {
	if srv.exitCh != nil {
		select {
		case <-srv.exitCh:
		default:
			close(srv.exitCh)
		}
		srv.exitCh = nil
	}
	return globular.StopService(srv, srv.grpcServer)
}

// Session janitor
func (srv *server) removeExpiredSessions() {
	if srv.exitCh == nil {
		srv.exitCh = make(chan struct{})
	}
	ticker := time.NewTicker(time.Duration(srv.WatchSessionsDelay) * time.Second)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				sessions, err := srv.getSessions()
				if err != nil {
					logger.Warn("sessions list failed", "err", err)
					continue
				}
				now := time.Now().Unix()
				for _, session := range sessions {
					if session.ExpireAt < now {
						session.State = 1
						if err := srv.updateSession(session); err != nil {
							logger.Warn("session expire update failed", "accountId", session.AccountId, "err", err)
						} else {
							logger.Info("session expired", "accountId", session.AccountId)
						}
					}
				}
			case <-srv.exitCh:
				logger.Info("session watcher stopped")
				return
			}
		}
	}()
}

// initializeServerDefaults sets up the server with default values before config loading.
func initializeServerDefaults() *server {
	s := new(server)
	s.Name = string(authenticationpb.File_authentication_proto.Services().Get(0).FullName())
	s.Proto = authenticationpb.File_authentication_proto.Path()
	s.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	s.Port = defaultPort
	s.Proxy = defaultProxy
	s.Protocol = "grpc"
	s.Version = Version // Use build-time version
	s.PublisherID = "localhost"
	s.Description = "Authentication service with password management, session handling, and peer token generation"
	s.Keywords = []string{"authentication", "auth", "login", "password", "session", "token", "ldap", "security"}
	s.Repositories = []string{}
	s.Discoveries = []string{}
	s.Dependencies = []string{"event.EventService"}
	s.Permissions = []any{
		map[string]any{
			"action": "/authentication.AuthenticationService/SetPassword",
			"resources": []any{
				map[string]any{"index": 0, "permission": "write"},
			},
		},
		map[string]any{
			"action":     "/authentication.AuthenticationService/SetRootPassword",
			"permission": "owner",
		},
		map[string]any{
			"action":     "/authentication.AuthenticationService/SetRootEmail",
			"permission": "owner",
		},
		map[string]any{
			"action": "/authentication.AuthenticationService/GeneratePeerToken",
			"resources": []any{
				map[string]any{"index": 0, "permission": "write"},
			},
		},
	}

	s.WatchSessionsDelay = 60
	s.SessionTimeout = 15
	s.Process = -1
	s.ProxyProcess = -1
	s.KeepAlive = true
	s.KeepUpToDate = true
	s.AllowAllOrigins = allowAllOrigins
	s.AllowedOrigins = allowedOriginsStr
	s.exitCh = make(chan struct{})
	s.LdapConnectionId = ""
	s.authentications_ = []string{}

	// Leave Domain/Address empty; HandleDescribeFlag/LoadRuntimeConfig will populate.
	s.Id = Utility.GenerateUUID(s.Name + ":" + fmt.Sprintf("localhost:%d", s.Port))

	return s
}

// setupGrpcService registers the Authentication service with the gRPC server.
func setupGrpcService(srv *server) {
	authenticationpb.RegisterAuthenticationServiceServer(srv.grpcServer, srv)
	reflection.Register(srv.grpcServer)
	Utility.RegisterFunction("NewAuthenticationService_Client", authentication_client.NewAuthenticationService_Client)
}

// --- Usage text ---
func printUsage() {
	fmt.Println("Globular Authentication Service")
	fmt.Println()
	fmt.Println("USAGE:")
	fmt.Println("  authentication_server [OPTIONS] [<id> [configPath]]")
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
	fmt.Println("  • Password management (set, validate, reset)")
	fmt.Println("  • Session handling with configurable timeouts")
	fmt.Println("  • Peer token generation for inter-service authentication")
	fmt.Println("  • LDAP integration support")
	fmt.Println("  • Root account management")
	fmt.Println()
	fmt.Println("EXAMPLES:")
	fmt.Println("  # Start with auto-generated ID and default config")
	fmt.Println("  authentication_server")
	fmt.Println()
	fmt.Println("  # Start with specific service ID")
	fmt.Println("  authentication_server auth-1")
	fmt.Println()
	fmt.Println("  # Enable debug logging")
	fmt.Println("  authentication_server --debug")
	fmt.Println()
	fmt.Println("  # Print service metadata")
	fmt.Println("  authentication_server --describe")
	fmt.Println()
	fmt.Println("  # Check service health")
	fmt.Println("  authentication_server --health")
}

func printVersion() {
	info := map[string]string{
		"service":    "authentication",
		"version":    Version,
		"build_time": BuildTime,
		"git_commit": GitCommit,
	}
	data, _ := json.MarshalIndent(info, "", "  ")
	fmt.Println(string(data))
}

// main wires the Authentication service with shared CLI + lifecycle primitives.
func main() {
	// Skeleton only (no etcd access yet)
	s := initializeServerDefaults()

	// Define CLI flags (BEFORE any arg parsing)
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
		logger.Debug("debug logging enabled")
	}
	slog.SetDefault(logger)

	// Handle early-exit flags
	if *showHelp {
		printUsage()
		return
	}
	if *showVersion {
		printVersion()
		return
	}

	// Handle --describe and --health flags
	if *showDescribe {
		// Set ephemeral data for describe
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
		if s.Id == "" {
			s.Id = Utility.GenerateUUID(s.Name + ":" + s.Address)
		}
		b, err := globular.DescribeJSON(s)
		if err != nil {
			logger.Error("describe error", "service", s.Name, "id", s.Id, "err", err)
			os.Exit(2)
		}
		os.Stdout.Write(b)
		os.Stdout.Write([]byte("\n"))
		return
	}

	if *showHealth {
		if s.Port == 0 || s.Name == "" {
			logger.Error("health error: uninitialized", "service", s.Name, "port", s.Port)
			os.Exit(2)
		}
		b, err := globular.HealthJSON(s, &globular.HealthOptions{Timeout: 1500 * time.Millisecond})
		if err != nil {
			logger.Error("health error", "service", s.Name, "id", s.Id, "err", err)
			os.Exit(2)
		}
		os.Stdout.Write(b)
		os.Stdout.Write([]byte("\n"))
		return
	}

	args := flag.Args() // Get remaining positional args

	// Positional args
	globular.ParsePositionalArgs(s, args)

	// Port allocation when needed
	if err := globular.AllocatePortIfNeeded(s, args); err != nil {
		logger.Error("fail to allocate port", "error", err)
		os.Exit(1)
	}

	// Runtime config (domain/address)
	globular.LoadRuntimeConfig(s)

	logger.Info("starting authentication service",
		"service", s.Name,
		"version", s.Version,
		"domain", s.Domain,
		"address", s.Address,
		"port", s.Port,
		"session_timeout", s.SessionTimeout,
	)

	start := time.Now()
	logger.Debug("initializing service", "service", s.Name, "id", s.Id)
	if err := s.Init(); err != nil {
		logger.Error("service init failed", "service", s.Name, "id", s.Id, "err", err)
		os.Exit(1)
	}
	logger.Debug("service init completed", "duration_ms", time.Since(start).Milliseconds())

	logger.Debug("registering gRPC handlers", "service", s.Name)
	setupGrpcService(s)
	logger.Debug("gRPC handlers registered")

	logger.Info("service ready",
		"service", s.Name,
		"port", s.Port,
		"proxy", s.Proxy,
		"protocol", s.Protocol,
		"domain", s.Domain,
		"startup_ms", time.Since(start).Milliseconds(),
		"version", s.Version,
	)

	lifecycle := globular.NewLifecycleManager(s, logger)
	if err := lifecycle.Start(); err != nil {
		logger.Error("service start failed", "err", err)
		os.Exit(1)
	}

	// wait for termination
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	<-sigChan

	logger.Info("shutdown signal received, initiating graceful shutdown")
	if err := lifecycle.GracefulShutdown(30 * time.Second); err != nil {
		logger.Error("graceful shutdown failed", "err", err)
		os.Exit(1)
	}
	logger.Info("service stopped gracefully")
}
