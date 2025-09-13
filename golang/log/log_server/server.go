// Package main implements the Log gRPC service wired for Globular.
// It uses structured logging (slog), supports --describe / --health,
// exposes Prometheus metrics, and manages log persistence with Badger.
package main

import (
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/event/event_client"
	"github.com/globulario/services/golang/globular_client"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/log/log_client"
	"github.com/globulario/services/golang/log/logpb"
	"github.com/globulario/services/golang/resource/resourcepb"
	"github.com/globulario/services/golang/storage/storage_store"
	Utility "github.com/globulario/utility"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// -----------------------------------------------------------------------------
// Defaults
// -----------------------------------------------------------------------------

var (
	defaultPort  = 10000
	defaultProxy = defaultPort + 1

	allowAllOrigins   = true
	allowedOriginsStr = ""
)

var logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
	Level: slog.LevelInfo,
}))

// -----------------------------------------------------------------------------
// Server
// -----------------------------------------------------------------------------

// server implements the LogService and Globular service contract.
type server struct {
	// Core metadata managed by Globular
	Id           string
	Name         string
	Mac          string
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
	Plaform         string
	Checksum        string
	KeepAlive       bool
	Permissions     []interface{}
	Dependencies    []string
	Process         int
	ProxyProcess    int
	ConfigPort      int
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
	Root       string

	// Persistence
	logs *storage_store.Badger_store

	// Metrics
	logCount        *prometheus.CounterVec
	Monitoring_Port int

	// Retention
	RetentionHours    int // default 7d (set in main)
	SweepEverySeconds int // default 300s (set in main)
}

// --- Globular service contract (getters/setters) ---

func (srv *server) GetConfigurationPath() string     { return srv.ConfigPath }
func (srv *server) SetConfigurationPath(path string) { srv.ConfigPath = path }

func (srv *server) GetAddress() string        { return srv.Address }
func (srv *server) SetAddress(address string) { srv.Address = address }

func (srv *server) GetProcess() int { return srv.Process }
func (srv *server) SetProcess(pid int) {
	// Close store if we are marked as stopped.
	if pid == -1 && srv.logs != nil {
		_ = srv.logs.Close()
	}
	srv.Process = pid
}

func (srv *server) GetProxyProcess() int       { return srv.ProxyProcess }
func (srv *server) SetProxyProcess(pid int)    { srv.ProxyProcess = pid }
func (srv *server) GetChecksum() string        { return srv.Checksum }
func (srv *server) SetChecksum(sum string)     { srv.Checksum = sum }
func (srv *server) GetPlatform() string        { return srv.Plaform }
func (srv *server) SetPlatform(p string)       { srv.Plaform = p }
func (srv *server) GetState() string           { return srv.State }
func (srv *server) SetState(state string)      { srv.State = state }
func (srv *server) GetLastError() string       { return srv.LastError }
func (srv *server) SetLastError(e string)      { srv.LastError = e }
func (srv *server) SetModTime(mt int64)        { srv.ModTime = mt }
func (srv *server) GetModTime() int64          { return srv.ModTime }
func (srv *server) GetId() string              { return srv.Id }
func (srv *server) SetId(id string)            { srv.Id = id }
func (srv *server) GetName() string            { return srv.Name }
func (srv *server) SetName(name string)        { srv.Name = name }
func (srv *server) GetDescription() string     { return srv.Description }
func (srv *server) SetDescription(d string)    { srv.Description = d }
func (srv *server) GetMac() string             { return srv.Mac }
func (srv *server) SetMac(mac string)          { srv.Mac = mac }
func (srv *server) GetKeywords() []string      { return srv.Keywords }
func (srv *server) SetKeywords(k []string)     { srv.Keywords = k }
func (srv *server) GetRepositories() []string  { return srv.Repositories }
func (srv *server) SetRepositories(r []string) { srv.Repositories = r }
func (srv *server) GetDiscoveries() []string   { return srv.Discoveries }
func (srv *server) SetDiscoveries(d []string)  { srv.Discoveries = d }
func (srv *server) GetConfigPort() int         { return srv.ConfigPort }
func (srv *server) SetConfigPort(p int)        { srv.ConfigPort = p }
func (srv *server) GetConfigAddress() string {
	return srv.GetDomain() + ":" + Utility.ToString(srv.ConfigPort)
}
func (srv *server) Dist(path string) (string, error) { return globular.Dist(path, srv) }

func (srv *server) GetDependencies() []string {
	if srv.Dependencies == nil {
		srv.Dependencies = make([]string, 0)
	}
	return srv.Dependencies
}
func (srv *server) SetDependency(dep string) {
	if srv.Dependencies == nil {
		srv.Dependencies = make([]string, 0)
	}
	if !Utility.Contains(srv.Dependencies, dep) {
		srv.Dependencies = append(srv.Dependencies, dep)
	}
}

func (srv *server) GetPath() string            { return srv.Path }
func (srv *server) SetPath(p string)           { srv.Path = p }
func (srv *server) GetProto() string           { return srv.Proto }
func (srv *server) SetProto(p string)          { srv.Proto = p }
func (srv *server) GetPort() int               { return srv.Port }
func (srv *server) SetPort(p int)              { srv.Port = p }
func (srv *server) GetProxy() int              { return srv.Proxy }
func (srv *server) SetProxy(px int)            { srv.Proxy = px }
func (srv *server) GetProtocol() string        { return srv.Protocol }
func (srv *server) SetProtocol(proto string)   { srv.Protocol = proto }
func (srv *server) GetAllowAllOrigins() bool   { return srv.AllowAllOrigins }
func (srv *server) SetAllowAllOrigins(v bool)  { srv.AllowAllOrigins = v }
func (srv *server) GetAllowedOrigins() string  { return srv.AllowedOrigins }
func (srv *server) SetAllowedOrigins(v string) { srv.AllowedOrigins = v }
func (srv *server) GetDomain() string          { return srv.Domain }
func (srv *server) SetDomain(d string)         { srv.Domain = d }

func (srv *server) GetTls() bool                    { return srv.TLS }
func (srv *server) SetTls(v bool)                   { srv.TLS = v }
func (srv *server) GetCertAuthorityTrust() string   { return srv.CertAuthorityTrust }
func (srv *server) SetCertAuthorityTrust(ca string) { srv.CertAuthorityTrust = ca }
func (srv *server) GetCertFile() string             { return srv.CertFile }
func (srv *server) SetCertFile(cf string)           { srv.CertFile = cf }
func (srv *server) GetKeyFile() string              { return srv.KeyFile }
func (srv *server) SetKeyFile(kf string)            { srv.KeyFile = kf }

func (srv *server) GetVersion() string             { return srv.Version }
func (srv *server) SetVersion(v string)            { srv.Version = v }
func (srv *server) GetPublisherID() string         { return srv.PublisherID }
func (srv *server) SetPublisherID(v string)        { srv.PublisherID = v }
func (srv *server) GetKeepUpToDate() bool          { return srv.KeepUpToDate }
func (srv *server) SetKeepUptoDate(v bool)         { srv.KeepUpToDate = v }
func (srv *server) GetKeepAlive() bool             { return srv.KeepAlive }
func (srv *server) SetKeepAlive(v bool)            { srv.KeepAlive = v }
func (srv *server) GetPermissions() []interface{}  { return srv.Permissions }
func (srv *server) SetPermissions(p []interface{}) { srv.Permissions = p }

func (srv *server) RolesDefault() []resourcepb.Role {
	domain, _ := config.GetDomain()

	return []resourcepb.Role{
		{
			Id:          "role:log.viewer",
			Name:        "Log Viewer",
			Domain:      domain,
			Description: "Read-only access to query logs.",
			Actions: []string{
				"/log.LogService/GetLog",
			},
			TypeName: "resource.Role",
		},
		{
			Id:          "role:log.writer",
			Name:        "Log Writer",
			Domain:      domain,
			Description: "Can append new log entries.",
			Actions: []string{
				"/log.LogService/Log",
			},
			TypeName: "resource.Role",
		},
		{
			Id:          "role:log.operator",
			Name:        "Log Operator",
			Domain:      domain,
			Description: "Operate on individual log entries (delete specific items).",
			Actions: []string{
				"/log.LogService/GetLog",
				"/log.LogService/DeleteLog",
			},
			TypeName: "resource.Role",
		},
		{
			Id:          "role:log.admin",
			Name:        "Log Admin",
			Domain:      domain,
			Description: "Full control over LogService, including bulk clears.",
			Actions: []string{
				"/log.LogService/Log",
				"/log.LogService/GetLog",
				"/log.LogService/DeleteLog",
				"/log.LogService/ClearAllLog",
			},
			TypeName: "resource.Role",
		},
	}
}

// --- Lifecycle ---

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

// --- Optional: event helpers ---

func (srv *server) getEventClient() (*event_client.Event_Client, error) {
	Utility.RegisterFunction("NewEventService_Client", event_client.NewEventService_Client)
	c, err := globular_client.GetClient(srv.Address, "event.EventService", "NewEventService_Client")
	if err != nil {
		return nil, err
	}
	return c.(*event_client.Event_Client), nil
}

func (srv *server) publish(event string, data []byte) error {
	ec, err := srv.getEventClient()
	if err != nil {
		return err
	}
	return ec.Publish(event, data)
}

// -----------------------------------------------------------------------------
// main
// -----------------------------------------------------------------------------

func main() {
	srv := new(server)

	// Fill ONLY fields that don’t touch etcd yet.
	srv.Name = string(logpb.File_log_proto.Services().Get(0).FullName())
	srv.Proto = logpb.File_log_proto.Path()
	srv.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	srv.Port = defaultPort
	srv.Proxy = defaultProxy
	srv.Protocol = "grpc"
	srv.Version = "0.0.1"
	srv.PublisherID = "localhost"
	srv.Description = "Cluster log collection and distribution service."
	srv.Keywords = []string{"log", "observability", "events", "monitoring"}
	srv.Repositories = make([]string, 0)
	srv.Discoveries = make([]string, 0)
	srv.Dependencies = []string{"event.EventService"}
	// s.Permissions for log.LogService
	srv.Permissions = []interface{}{
		// ---- Ingest (append-only)
		map[string]interface{}{
			"action":     "/log.LogService/Log",
			"permission": "write",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "Info.Application", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "Info.Method", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "Info.Level", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "Info.Message", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "Info.Line", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "Info.TimestampMs", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "Info.Component", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "Info.Fields", "permission": "write"},
			},
		},

		// ---- Read (query)
		map[string]interface{}{
			"action":     "/log.LogService/GetLog",
			"permission": "read",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "Query", "permission": "read"},
			},
		},

		// ---- Point delete (surgical removals)
		map[string]interface{}{
			"action":     "/log.LogService/DeleteLog",
			"permission": "delete",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "Log.Id", "permission": "delete"},
				map[string]interface{}{"index": 0, "field": "Log.Application", "permission": "delete"},
				map[string]interface{}{"index": 0, "field": "Log.Level", "permission": "delete"},
			},
		},

		// ---- Bulk clear (dangerous)
		map[string]interface{}{
			"action":     "/log.LogService/ClearAllLog",
			"permission": "admin", // bulk destructive; gate with admin
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "Query", "permission": "delete"},
			},
		},
	}

	srv.Process = -1
	srv.ProxyProcess = -1
	srv.AllowAllOrigins = allowAllOrigins
	srv.AllowedOrigins = allowedOriginsStr
	srv.KeepAlive = true
	srv.KeepUpToDate = true
	srv.Monitoring_Port = 9092
	srv.Root = config.GetDataDir()
	srv.RetentionHours = 24 * 7 // 7 days
	srv.SweepEverySeconds = 300 // 5 minutes

	// Register client ctor for this service.
	Utility.RegisterFunction("NewLogService_Client", log_client.NewLogService_Client)

	// ---- CLI flags BEFORE any config/etcd access ----
	args := os.Args[1:]
	if len(args) == 0 {
		srv.Id = Utility.GenerateUUID(srv.Name + ":" + srv.Address)
		allocator, err := config.NewDefaultPortAllocator()
		if err != nil {
			logger.Error("fail to create port allocator", "error", err)
			os.Exit(1)
		}
		p, err := allocator.Next(srv.Id)
		if err != nil {
			logger.Error("fail to allocate port", "error", err)
			os.Exit(1)
		}
		srv.Port = p
	}

	for _, a := range args {
		switch strings.ToLower(a) {
		case "--describe":
			srv.Process = os.Getpid()
			srv.State = "starting"

			// Safe defaults without etcd calls
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

			b, err := globular.DescribeJSON(srv)
			if err != nil {
				logger.Error("describe error", "service", srv.Name, "id", srv.Id, "err", err)
				os.Exit(2)
			}
			os.Stdout.Write(b)
			os.Stdout.Write([]byte("\n"))
			return

		case "--health":
			if srv.Port == 0 || srv.Name == "" {
				logger.Error("health error: uninitialized", "service", srv.Name, "port", srv.Port)
				os.Exit(2)
			}
			b, err := globular.HealthJSON(srv, &globular.HealthOptions{
				Timeout:     1500 * time.Millisecond,
				ServiceName: "",
			})
			if err != nil {
				logger.Error("health error", "service", srv.Name, "id", srv.Id, "err", err)
				os.Exit(2)
			}
			os.Stdout.Write(b)
			os.Stdout.Write([]byte("\n"))
			return
		case "--debug":
			logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
		case "--help", "-h", "/?":
			printUsage()
			return
		case "--version", "-v":
			fmt.Println(srv.Version)
			return
		default:
			// skip unknown flags for now (e.g. positional args)
		}
	}

	// Optional positional args
	if len(args) == 1 && !strings.HasPrefix(args[0], "-") {
		srv.Id = args[0]
	} else if len(args) == 2 && !strings.HasPrefix(args[0], "-") && !strings.HasPrefix(args[1], "-") {
		srv.Id = args[0]
		srv.ConfigPath = args[1]
	}

	// Safe to access config now (may hit etcd or local fallback)
	if d, err := config.GetDomain(); err == nil {
		srv.Domain = d
	} else {
		srv.Domain = "localhost"
	}
	if a, err := config.GetAddress(); err == nil && strings.TrimSpace(a) != "" {
		srv.Address = a
	}

	// Bootstrap gRPC
	start := time.Now()
	if err := srv.Init(); err != nil {
		logger.Error("service init failed", "service", srv.Name, "id", srv.Id, "err", err)
		os.Exit(1)
	}

	// Register RPCs + reflection
	logpb.RegisterLogServiceServer(srv.grpcServer, srv)
	reflection.Register(srv.grpcServer)

	// Open Badger store
	srv.logs = storage_store.NewBadger_store()
	if err := srv.logs.Open(`{"path":"` + srv.Root + `", "name":"logs"}`); err != nil {
		logger.Error("failed to open log store", "err", err)
		// non-fatal: service can still start; decide if you prefer os.Exit(1)
	}

	// Start retention janitor (best effort)
	go srv.startRetentionJanitor()

	// Prometheus metrics
	srv.logCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "log_entries_total",
			Help: "Total log entries by log level/application/method.",
		},
		[]string{"level", "application", "method"},
	)
	prometheus.MustRegister(srv.logCount)

	// /metrics endpoint
	http.Handle("/metrics", promhttp.Handler())
	go func() {
		addr := "0.0.0.0:" + Utility.ToString(srv.Monitoring_Port)
		if err := http.ListenAndServe(addr, nil); err != nil {
			logger.Warn("prometheus metrics server error", "addr", addr, "err", err)
		}
	}()

	logger.Info("service ready",
		"service", srv.Name,
		"port", srv.Port,
		"proxy", srv.Proxy,
		"protocol", srv.Protocol,
		"domain", srv.Domain,
		"listen_ms", time.Since(start).Milliseconds())

	if err := srv.StartService(); err != nil {
		logger.Error("service start failed", "service", srv.Name, "id", srv.Id, "err", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  server [id] [config_path] [--describe] [--health]")
	fmt.Println()
	fmt.Println("Options:")
	fmt.Println("  --describe   Print service metadata as JSON and exit.")
	fmt.Println("  --health     Print service health status as JSON and exit.")
	fmt.Println("  id           Optional service instance ID.")
	fmt.Println("  config_path  Optional configuration file path.")
	fmt.Println()
	fmt.Println("Environment variables:")
	fmt.Println("  GLOBULAR_DOMAIN   Override service domain.")
	fmt.Println("  GLOBULAR_ADDRESS  Override service address.")
}

// -----------------------------------------------------------------------------
// helpers
// -----------------------------------------------------------------------------
