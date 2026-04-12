// Package main provides the Globular compute service — a workflow-driven
// distributed execution subsystem. It manages compute definitions, jobs,
// units, and results following the 4-layer state model.
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

	"github.com/globulario/services/golang/compute/computepb"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/resource/resourcepb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// Default ports.
var (
	defaultPort  = 10300
	defaultProxy = defaultPort + 1

	allowAllOrigins   = true
	allowedOriginsStr = ""
)

// server implements the Globular service contract and ComputeService RPCs.
type server struct {
	// --- Service Identity ---
	Id          string
	Mac         string
	Name        string
	Domain      string
	Address     string
	Path        string
	Proto       string
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
	Plaform         string
	Checksum        string
	Permissions     []any
	Dependencies    []string

	// --- Runtime State ---
	Process      int
	ProxyProcess int
	ConfigPath   string
	LastError    string
	State        string
	ModTime      int64

	// --- TLS Configuration ---
	TLS                bool
	CertFile           string
	KeyFile            string
	CertAuthorityTrust string

	// --- gRPC Runtime ---
	grpcServer *grpc.Server
}

// ─── Globular service contract (getters/setters) ─────────────────────────────

func (srv *server) GetConfigurationPath() string              { return srv.ConfigPath }
func (srv *server) SetConfigurationPath(path string)          { srv.ConfigPath = path }
func (srv *server) GetAddress() string                        { return srv.Address }
func (srv *server) SetAddress(address string)                 { srv.Address = address }
func (srv *server) GetProcess() int                           { return srv.Process }
func (srv *server) SetProcess(pid int)                        { srv.Process = pid }
func (srv *server) GetProxyProcess() int                      { return srv.ProxyProcess }
func (srv *server) SetProxyProcess(pid int)                   { srv.ProxyProcess = pid }
func (srv *server) GetState() string                          { return srv.State }
func (srv *server) SetState(state string)                     { srv.State = state }
func (srv *server) GetLastError() string                      { return srv.LastError }
func (srv *server) SetLastError(err string)                   { srv.LastError = err }
func (srv *server) SetModTime(modtime int64)                  { srv.ModTime = modtime }
func (srv *server) GetModTime() int64                         { return srv.ModTime }
func (srv *server) GetId() string                             { return srv.Id }
func (srv *server) SetId(id string)                           { srv.Id = id }
func (srv *server) GetName() string                           { return srv.Name }
func (srv *server) SetName(name string)                       { srv.Name = name }
func (srv *server) GetDescription() string                    { return srv.Description }
func (srv *server) SetDescription(description string)         { srv.Description = description }
func (srv *server) GetMac() string                            { return srv.Mac }
func (srv *server) SetMac(mac string)                         { srv.Mac = mac }
func (srv *server) GetKeywords() []string                     { return srv.Keywords }
func (srv *server) SetKeywords(keywords []string)             { srv.Keywords = keywords }
func (srv *server) GetRepositories() []string                 { return srv.Repositories }
func (srv *server) SetRepositories(repositories []string)     { srv.Repositories = repositories }
func (srv *server) GetDiscoveries() []string                  { return srv.Discoveries }
func (srv *server) SetDiscoveries(discoveries []string)       { srv.Discoveries = discoveries }
func (srv *server) Dist(path string) (string, error)          { return globular.Dist(path, srv) }
func (srv *server) GetChecksum() string                       { return srv.Checksum }
func (srv *server) SetChecksum(checksum string)               { srv.Checksum = checksum }
func (srv *server) GetPlatform() string                       { return srv.Plaform }
func (srv *server) SetPlatform(platform string)               { srv.Plaform = platform }
func (srv *server) GetPath() string                           { return srv.Path }
func (srv *server) SetPath(path string)                       { srv.Path = path }
func (srv *server) GetProto() string                          { return srv.Proto }
func (srv *server) SetProto(proto string)                     { srv.Proto = proto }
func (srv *server) GetPort() int                              { return srv.Port }
func (srv *server) SetPort(port int)                          { srv.Port = port }
func (srv *server) GetProxy() int                             { return srv.Proxy }
func (srv *server) SetProxy(proxy int)                        { srv.Proxy = proxy }
func (srv *server) GetGrpcServer() *grpc.Server               { return srv.grpcServer }
func (srv *server) GetProtocol() string                       { return srv.Protocol }
func (srv *server) SetProtocol(protocol string)               { srv.Protocol = protocol }
func (srv *server) GetAllowAllOrigins() bool                  { return srv.AllowAllOrigins }
func (srv *server) SetAllowAllOrigins(v bool)                 { srv.AllowAllOrigins = v }
func (srv *server) GetAllowedOrigins() string                 { return srv.AllowedOrigins }
func (srv *server) SetAllowedOrigins(v string)                { srv.AllowedOrigins = v }
func (srv *server) GetDomain() string                         { return srv.Domain }
func (srv *server) SetDomain(domain string)                   { srv.Domain = domain }
func (srv *server) GetTls() bool                              { return srv.TLS }
func (srv *server) SetTls(hasTls bool)                        { srv.TLS = hasTls }
func (srv *server) GetCertAuthorityTrust() string             { return srv.CertAuthorityTrust }
func (srv *server) SetCertAuthorityTrust(ca string)           { srv.CertAuthorityTrust = ca }
func (srv *server) GetCertFile() string                       { return srv.CertFile }
func (srv *server) SetCertFile(certFile string)               { srv.CertFile = certFile }
func (srv *server) GetKeyFile() string                        { return srv.KeyFile }
func (srv *server) SetKeyFile(keyFile string)                 { srv.KeyFile = keyFile }
func (srv *server) GetVersion() string                        { return srv.Version }
func (srv *server) SetVersion(version string)                 { srv.Version = version }
func (srv *server) GetPublisherID() string                    { return srv.PublisherID }
func (srv *server) SetPublisherID(id string)                  { srv.PublisherID = id }
func (srv *server) GetKeepUpToDate() bool                     { return srv.KeepUpToDate }
func (srv *server) SetKeepUptoDate(val bool)                  { srv.KeepUpToDate = val }
func (srv *server) GetKeepAlive() bool                        { return srv.KeepAlive }
func (srv *server) SetKeepAlive(val bool)                     { srv.KeepAlive = val }
func (srv *server) GetPermissions() []any                     { return srv.Permissions }
func (srv *server) SetPermissions(permissions []any)          { srv.Permissions = permissions }
func (srv *server) RolesDefault() []resourcepb.Role           { return []resourcepb.Role{} }

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
	for _, d := range srv.Dependencies {
		if d == dependency {
			return
		}
	}
	srv.Dependencies = append(srv.Dependencies, dependency)
}

// ─── Lifecycle ───────────────────────────────────────────────────────────────

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

func (srv *server) Save() error { return globular.SaveService(srv) }

func (srv *server) StartService() error {
	return globular.StartService(srv, srv.grpcServer)
}

func (srv *server) StopService() error {
	return globular.StopService(srv, srv.grpcServer)
}

// ─── ComputeService RPC stubs ────────────────────────────────────────────────

func (srv *server) RegisterComputeDefinition(ctx context.Context, req *computepb.RegisterComputeDefinitionRequest) (*computepb.RegisterComputeDefinitionResponse, error) {
	return nil, fmt.Errorf("RegisterComputeDefinition not implemented")
}

func (srv *server) GetComputeDefinition(ctx context.Context, req *computepb.GetComputeDefinitionRequest) (*computepb.GetComputeDefinitionResponse, error) {
	return nil, fmt.Errorf("GetComputeDefinition not implemented")
}

func (srv *server) ListComputeDefinitions(ctx context.Context, req *computepb.ListComputeDefinitionsRequest) (*computepb.ListComputeDefinitionsResponse, error) {
	return nil, fmt.Errorf("ListComputeDefinitions not implemented")
}

func (srv *server) ValidateComputeDefinition(ctx context.Context, req *computepb.ValidateComputeDefinitionRequest) (*computepb.ValidateComputeDefinitionResponse, error) {
	return nil, fmt.Errorf("ValidateComputeDefinition not implemented")
}

func (srv *server) SubmitComputeJob(ctx context.Context, req *computepb.SubmitComputeJobRequest) (*computepb.SubmitComputeJobResponse, error) {
	return nil, fmt.Errorf("SubmitComputeJob not implemented")
}

func (srv *server) GetComputeJob(ctx context.Context, req *computepb.GetComputeJobRequest) (*computepb.GetComputeJobResponse, error) {
	return nil, fmt.Errorf("GetComputeJob not implemented")
}

func (srv *server) ListComputeJobs(ctx context.Context, req *computepb.ListComputeJobsRequest) (*computepb.ListComputeJobsResponse, error) {
	return nil, fmt.Errorf("ListComputeJobs not implemented")
}

func (srv *server) CancelComputeJob(ctx context.Context, req *computepb.CancelComputeJobRequest) (*computepb.CancelComputeJobResponse, error) {
	return nil, fmt.Errorf("CancelComputeJob not implemented")
}

func (srv *server) GetComputeResult(ctx context.Context, req *computepb.GetComputeResultRequest) (*computepb.GetComputeResultResponse, error) {
	return nil, fmt.Errorf("GetComputeResult not implemented")
}

func (srv *server) ListComputeUnits(ctx context.Context, req *computepb.ListComputeUnitsRequest) (*computepb.ListComputeUnitsResponse, error) {
	return nil, fmt.Errorf("ListComputeUnits not implemented")
}

func (srv *server) GetComputeUnit(ctx context.Context, req *computepb.GetComputeUnitRequest) (*computepb.GetComputeUnitResponse, error) {
	return nil, fmt.Errorf("GetComputeUnit not implemented")
}

// ─── Initialization ──────────────────────────────────────────────────────────

func initializeServerDefaults() *server {
	srv := new(server)

	srv.Name = string(computepb.File_compute_proto.Services().Get(0).FullName())
	srv.Proto = computepb.File_compute_proto.Path()
	srv.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	srv.Version = Version
	srv.PublisherID = "localhost"
	srv.Description = "Workflow-driven distributed compute execution subsystem"
	srv.Keywords = []string{"compute", "distributed", "workflow", "execution", "batch"}

	srv.Port = defaultPort
	srv.Proxy = defaultProxy
	srv.Protocol = "grpc"

	srv.AllowAllOrigins = allowAllOrigins
	srv.AllowedOrigins = allowedOriginsStr

	srv.KeepAlive = true
	srv.KeepUpToDate = true
	srv.Process = -1
	srv.ProxyProcess = -1

	srv.Repositories = make([]string, 0)
	srv.Discoveries = make([]string, 0)
	srv.Dependencies = make([]string, 0)
	srv.Permissions = make([]any, 0)

	return srv
}

func setupGrpcService(srv *server) {
	computepb.RegisterComputeServiceServer(srv.grpcServer, srv)
	reflection.Register(srv.grpcServer)
}

// ─── main ────────────────────────────────────────────────────────────────────

var (
	Version   = "0.0.1"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

var logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
	Level: slog.LevelInfo,
}))

func main() {
	srv := initializeServerDefaults()

	var (
		enableDebug  = flag.Bool("debug", false, "enable debug logging")
		showVersion  = flag.Bool("version", false, "print version information as JSON and exit")
		showHelp     = flag.Bool("help", false, "show usage information and exit")
		showDescribe = flag.Bool("describe", false, "print service description as JSON and exit")
	)

	flag.Usage = printUsage
	flag.Parse()

	if *enableDebug {
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
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
		globular.HandleDescribeFlag(srv, logger)
		return
	}

	args := flag.Args()
	if err := globular.AllocatePortIfNeeded(srv, args); err != nil {
		logger.Error("port allocation failed", "error", err)
		os.Exit(1)
	}

	globular.ParsePositionalArgs(srv, args)
	globular.LoadRuntimeConfig(srv)

	logger.Info("starting compute service", "service", srv.Name, "version", srv.Version, "domain", srv.Domain)

	start := time.Now()
	if err := srv.Init(); err != nil {
		logger.Error("service init failed", "service", srv.Name, "id", srv.Id, "err", err)
		os.Exit(1)
	}
	logger.Info("service initialized", "duration_ms", time.Since(start).Milliseconds())

	setupGrpcService(srv)

	logger.Info("service ready", "service", srv.Name, "version", srv.Version, "port", srv.Port, "domain", srv.Domain, "startup_ms", time.Since(start).Milliseconds())

	lm := globular.NewLifecycleManager(srv, logger)
	if err := lm.Start(); err != nil {
		logger.Error("service start failed", "service", srv.Name, "id", srv.Id, "err", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Compute Service - Workflow-driven distributed execution")
	fmt.Println()
	fmt.Println("USAGE:")
	fmt.Println("  compute_server [OPTIONS] [id] [config_path]")
	fmt.Println()
	fmt.Println("OPTIONS:")
	fmt.Println("  --debug       Enable debug logging")
	fmt.Println("  --describe    Print service description as JSON and exit")
	fmt.Println("  --version     Print version information as JSON and exit")
	fmt.Println("  --help        Show this help message and exit")
}

func printVersion() {
	info := map[string]string{
		"service":    "compute",
		"version":    Version,
		"build_time": BuildTime,
		"git_commit": GitCommit,
	}
	data, _ := json.MarshalIndent(info, "", "  ")
	fmt.Println(string(data))
}
