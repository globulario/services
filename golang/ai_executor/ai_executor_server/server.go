// Package main implements the AI Executor service — the action layer
// that diagnoses incidents and executes remediation. The watcher sees,
// the executor acts.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/globulario/services/golang/ai_executor/ai_executorpb"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/resource/resourcepb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

var (
	Version   = "0.1.0"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

var (
	defaultPort  = 10230
	defaultProxy = 10231
)

var logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

type server struct {
	// Globular service metadata
	Id                 string
	Mac                string
	Name               string
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
	Process            int
	ProxyProcess       int
	ConfigPath         string
	LastError          string
	ModTime            int64

	ai_executorpb.UnimplementedAiExecutorServiceServer
	grpcServer *grpc.Server

	// Executor state
	diagnoser  *diagnoser
	remediator *remediator
	jobStore   *jobStore
	notifier   *multiNotifier
	stats      executorStats
	statsMu    sync.Mutex
	startedAt  time.Time

	// Recent actions log
	recentActions   []*ai_executorpb.RemediationAction
	recentActionsMu sync.RWMutex

	// Anthropic API config — when set, calls API directly instead of CLI.
	Anthropic AnthropicConfig `json:"Anthropic"`

	// Peer collaboration — multi-node AI consensus.
	peers *peerManager

	// Conversation store — persistent chat history in ScyllaDB.
	convStore *conversationStore
}

type executorStats struct {
	IncidentsProcessed int64
	DiagnosesCompleted int64
	ActionsExecuted    int64
	ActionsFailed      int64
}

// Globular service contract
func (srv *server) GetConfigurationPath() string        { return srv.ConfigPath }
func (srv *server) SetConfigurationPath(path string)    { srv.ConfigPath = path }
func (srv *server) GetAddress() string                  { return srv.Address }
func (srv *server) SetAddress(address string)           { srv.Address = address }
func (srv *server) GetProcess() int                     { return srv.Process }
func (srv *server) SetProcess(pid int)                  { srv.Process = pid }
func (srv *server) GetProxyProcess() int                { return srv.ProxyProcess }
func (srv *server) SetProxyProcess(pid int)             { srv.ProxyProcess = pid }
func (srv *server) GetState() string                    { return srv.State }
func (srv *server) SetState(state string)               { srv.State = state }
func (srv *server) GetLastError() string                { return srv.LastError }
func (srv *server) SetLastError(err string)             { srv.LastError = err }
func (srv *server) SetModTime(modtime int64)            { srv.ModTime = modtime }
func (srv *server) GetModTime() int64                   { return srv.ModTime }
func (srv *server) GetId() string                       { return srv.Id }
func (srv *server) SetId(id string)                     { srv.Id = id }
func (srv *server) GetName() string                     { return srv.Name }
func (srv *server) SetName(name string)                 { srv.Name = name }
func (srv *server) GetMac() string                      { return srv.Mac }
func (srv *server) SetMac(mac string)                   { srv.Mac = mac }
func (srv *server) GetDescription() string              { return srv.Description }
func (srv *server) SetDescription(description string)   { srv.Description = description }
func (srv *server) GetKeywords() []string               { return srv.Keywords }
func (srv *server) SetKeywords(keywords []string)       { srv.Keywords = keywords }
func (srv *server) Dist(path string) (string, error)    { return globular.Dist(path, srv) }
func (srv *server) GetDependencies() []string {
	if srv.Dependencies == nil { srv.Dependencies = []string{} }
	return srv.Dependencies
}
func (srv *server) SetDependency(dep string) {
	if srv.Dependencies == nil { srv.Dependencies = []string{} }
	for _, d := range srv.Dependencies { if d == dep { return } }
	srv.Dependencies = append(srv.Dependencies, dep)
}
func (srv *server) GetChecksum() string                      { return srv.Checksum }
func (srv *server) SetChecksum(checksum string)              { srv.Checksum = checksum }
func (srv *server) GetPlatform() string                      { return srv.Plaform }
func (srv *server) SetPlatform(platform string)              { srv.Plaform = platform }
func (srv *server) GetRepositories() []string                { return srv.Repositories }
func (srv *server) SetRepositories(v []string)               { srv.Repositories = v }
func (srv *server) GetDiscoveries() []string                 { return srv.Discoveries }
func (srv *server) SetDiscoveries(v []string)                { srv.Discoveries = v }
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
func (srv *server) SetPublisherID(p string)                  { srv.PublisherID = p }
func (srv *server) GetKeepUpToDate() bool                    { return srv.KeepUpToDate }
func (srv *server) SetKeepUptoDate(val bool)                 { srv.KeepUpToDate = val }
func (srv *server) GetKeepAlive() bool                       { return srv.KeepAlive }
func (srv *server) SetKeepAlive(val bool)                    { srv.KeepAlive = val }
func (srv *server) GetPermissions() []interface{}            { return srv.Permissions }
func (srv *server) SetPermissions(permissions []interface{}) { srv.Permissions = permissions }
func (srv *server) GetGrpcServer() *grpc.Server              { return srv.grpcServer }
func (srv *server) RolesDefault() []resourcepb.Role          { return []resourcepb.Role{} }

func (srv *server) Init() error {
	if err := globular.InitService(srv); err != nil {
		return err
	}
	gs, err := globular.InitGrpcServer(srv)
	if err != nil {
		return err
	}
	srv.grpcServer = gs
	srv.diagnoser = newDiagnoser(srv.Anthropic)
	srv.remediator = newRemediator()
	srv.jobStore = newJobStore()
	srv.notifier = newMultiNotifier()
	srv.startedAt = time.Now()

	// Start expiry checker in background.
	go srv.expiryLoop()

	// Hot-reload: periodically scan for AI credentials.
	go srv.credentialWatchLoop()

	// Conversation store: connect to ScyllaDB for chat history.
	srv.convStore = newConversationStore()
	go func() {
		// Give ScyllaDB time to be ready on Day-0 boot.
		for i := 0; i < 10; i++ {
			if err := srv.convStore.connect(); err == nil {
				return
			}
			time.Sleep(5 * time.Second)
		}
		logger.Warn("conversation_store: could not connect to ScyllaDB, conversations will be unavailable")
	}()

	// Peer collaboration: discover and connect to ai-executors on other nodes.
	// Use Mac (per-instance identifier) as the local node ID — srv.GetId()
	// returns the service type ID, which is the same across all ai_executor
	// instances in the cluster and would make the peer loop skip everyone.
	hostname, _ := os.Hostname()
	srv.peers = newPeerManager(srv.GetMac(), hostname, nil)
	go srv.peers.startDiscoveryLoop(context.Background())
	return nil
}

// credentialWatchLoop periodically checks for AI credentials.
// When a token becomes available (user logs into Claude Code, or another
// node syncs to etcd), it hot-swaps the diagnoser's anthropic client
// so AI reasoning activates without a service restart.
func (srv *server) credentialWatchLoop() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		// Already have a working client? Nothing to do.
		if srv.diagnoser.anthropic != nil && srv.diagnoser.anthropic.isAvailable() {
			// Check if token needs refresh.
			if srv.diagnoser.anthropic.refreshToken != "" {
				_ = srv.diagnoser.anthropic.ensureValidToken()
			}
			continue
		}

		// Try to create a new client from available sources.
		client := newAnthropicClient(srv.Anthropic)
		if client != nil && client.isAvailable() {
			srv.diagnoser.anthropic = client
			logger.Info("credential-watch: AI backend activated (hot-reload)")
		}
	}
}

func (srv *server) Save() error { return globular.SaveService(srv) }

func (srv *server) StartService() error {
	return globular.StartService(srv, srv.grpcServer)
}

// expiryLoop checks for stale approvals every minute.
func (srv *server) expiryLoop() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		expired := srv.jobStore.expireStaleApprovals()
		if expired > 0 {
			logger.Info("expired stale approvals", "count", expired)
		}
	}
}

func (srv *server) StopService() error {
	if srv.diagnoser != nil && srv.diagnoser.claude != nil {
		srv.diagnoser.claude.shutdown()
	}
	if srv.convStore != nil {
		srv.convStore.close()
	}
	return globular.StopService(srv, srv.grpcServer)
}

func initializeServerDefaults() *server {
	return &server{
		Name:            "ai_executor.AiExecutorService",
		Proto:           "ai_executor.proto",
		Path:            func() string { p, _ := filepath.Abs(filepath.Dir(os.Args[0])); return p }(),
		Port:            defaultPort,
		Proxy:           defaultProxy,
		Protocol:        "grpc",
		Version:         Version,
		PublisherID:     "localhost",
		Description:     "AI Executor — incident diagnosis and remediation engine",
		Keywords:        []string{"ai", "executor", "diagnosis", "remediation", "incidents"},
		AllowAllOrigins: true,
		KeepAlive:       true,
		KeepUpToDate:    true,
		Process:         -1,
		ProxyProcess:    -1,
		Repositories:    make([]string, 0),
		Discoveries:     make([]string, 0),
		Dependencies:    []string{"ai_memory.AiMemoryService"},
		Permissions:     make([]interface{}, 0),
	}
}

func setupGrpcService(s *server) {
	ai_executorpb.RegisterAiExecutorServiceServer(s.grpcServer, s)
	reflection.Register(s.grpcServer)
}

func main() {
	srv := initializeServerDefaults()
	var (
		enableDebug  = flag.Bool("debug", false, "enable debug logging")
		showVersion  = flag.Bool("version", false, "print version info and exit")
		showHelp     = flag.Bool("help", false, "show usage and exit")
		showDescribe = flag.Bool("describe", false, "print service description and exit")
		showHealth   = flag.Bool("health", false, "print health status and exit")
	)
	flag.Usage = func() {
		exe := filepath.Base(os.Args[0])
		fmt.Fprintf(os.Stdout, "%s - AI Executor Service\n\nDiagnoses incidents and executes remediation actions.\n\nUSAGE:\n  %s [OPTIONS] [<id>] [<configPath>]\n\n", exe, exe)
		flag.PrintDefaults()
	}
	flag.Parse()

	if *enableDebug {
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
	}
	if *showHelp { flag.Usage(); return }
	if *showVersion {
		data, _ := json.MarshalIndent(map[string]string{"version": Version, "build_time": BuildTime, "git_commit": GitCommit}, "", "  ")
		fmt.Println(string(data)); return
	}
	if *showDescribe { globular.HandleDescribeFlag(srv, logger); return }
	if *showHealth {
		data, _ := json.MarshalIndent(map[string]interface{}{"service": srv.Name, "status": "healthy", "version": srv.Version}, "", "  ")
		fmt.Println(string(data)); return
	}

	args := flag.Args()
	if err := globular.AllocatePortIfNeeded(srv, args); err != nil {
		logger.Error("port allocation failed", "error", err); os.Exit(1)
	}
	globular.ParsePositionalArgs(srv, args)
	globular.LoadRuntimeConfig(srv)

	logger.Info("starting ai_executor service", "service", srv.Name, "version", srv.Version)
	start := time.Now()
	if err := srv.Init(); err != nil {
		logger.Error("init failed", "err", err); os.Exit(1)
	}
	setupGrpcService(srv)
	logger.Info("service ready", "service", srv.Name, "port", srv.Port, "startup_ms", time.Since(start).Milliseconds())

	lm := globular.NewLifecycleManager(srv, logger)
	if err := lm.Start(); err != nil {
		logger.Error("start failed", "err", err); os.Exit(1)
	}
}
