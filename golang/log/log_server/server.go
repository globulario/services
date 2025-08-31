package main

import (
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/event/event_client"
	"github.com/globulario/services/golang/globular_client"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/interceptors"
	"github.com/globulario/services/golang/log/log_client"
	"github.com/globulario/services/golang/log/logpb"
	"github.com/globulario/services/golang/storage/storage_store"
	Utility "github.com/globulario/utility"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// Default service settings.
var (
	defaultPort  = 10029
	defaultProxy = 10030

	allow_all_origins = true
	allowed_origins   = ""
)

// server implements the LogService and manages persistence, events, and metrics.
type server struct {
	// Generic service metadata (managed by Globular)
	Id                 string
	Name               string
	Mac                string
	Domain             string
	Address            string
	Path               string
	Proto              string
	Port               int
	Proxy              int
	Monitoring_Port    int
	AllowAllOrigins    bool
	AllowedOrigins     string // comma-separated list
	Protocol           string
	Version            string
	PublisherID        string
	KeepUpToDate       bool
	Plaform            string
	Checksum           string
	KeepAlive          bool
	Description        string
	Keywords           []string
	Repositories       []string
	Discoveries        []string
	Process            int
	ProxyProcess       int
	ConfigPort         int
	ConfigPath         string
	LastError          string
	ModTime            int64
	TLS                bool
	State              string
	CertFile           string
	KeyFile            string
	CertAuthorityTrust string

	Permissions  []interface{}
	Dependencies []string

	grpcServer *grpc.Server
	Root       string

	// Persistence for logs (Badger)
	logs *storage_store.Badger_store

	// Prometheus metrics
	logCount *prometheus.CounterVec

	// Retention
	RetentionHours    int // how long to keep logs (default 7d)
	SweepEverySeconds int // how often to run the janitor (default 300s / 5m)
}

// Configuration path.
func (srv *server) GetConfigurationPath() string     { return srv.ConfigPath }
func (srv *server) SetConfigurationPath(path string) { srv.ConfigPath = path }

// Service address (host:port for grpc/proxy discovery).
func (srv *server) GetAddress() string        { return srv.Address }
func (srv *server) SetAddress(address string) { srv.Address = address }

func (srv *server) GetProcess() int { return srv.Process }
func (srv *server) SetProcess(pid int) {
	if pid == -1 && srv.logs != nil {
		_ = srv.logs.Close()
	}
	srv.Process = pid
}

func (srv *server) GetProxyProcess() int                  { return srv.ProxyProcess }
func (srv *server) SetProxyProcess(pid int)               { srv.ProxyProcess = pid }
func (srv *server) GetChecksum() string                   { return srv.Checksum }
func (srv *server) SetChecksum(checksum string)           { srv.Checksum = checksum }
func (srv *server) GetPlatform() string                   { return srv.Plaform }
func (srv *server) SetPlatform(platform string)           { srv.Plaform = platform }
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
func (srv *server) GetDescription() string                { return srv.Description }
func (srv *server) SetDescription(description string)     { srv.Description = description }
func (srv *server) GetMac() string                        { return srv.Mac }
func (srv *server) SetMac(mac string)                     { srv.Mac = mac }
func (srv *server) GetKeywords() []string                 { return srv.Keywords }
func (srv *server) SetKeywords(keywords []string)         { srv.Keywords = keywords }
func (srv *server) GetRepositories() []string             { return srv.Repositories }
func (srv *server) SetRepositories(repositories []string) { srv.Repositories = repositories }
func (srv *server) GetDiscoveries() []string              { return srv.Discoveries }
func (srv *server) SetDiscoveries(discoveries []string)   { srv.Discoveries = discoveries }
func (srv *server) GetConfigPort() int                    { return srv.ConfigPort }
func (srv *server) SetConfigPort(port int)                { srv.ConfigPort = port }
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

func (srv *server) GetPath() string             { return srv.Path }
func (srv *server) SetPath(path string)         { srv.Path = path }
func (srv *server) GetProto() string            { return srv.Proto }
func (srv *server) SetProto(proto string)       { srv.Proto = proto }
func (srv *server) GetPort() int                { return srv.Port }
func (srv *server) SetPort(port int)            { srv.Port = port }
func (srv *server) GetProxy() int               { return srv.Proxy }
func (srv *server) SetProxy(proxy int)          { srv.Proxy = proxy }
func (srv *server) GetProtocol() string         { return srv.Protocol }
func (srv *server) SetProtocol(protocol string) { srv.Protocol = protocol }
func (srv *server) GetAllowAllOrigins() bool    { return srv.AllowAllOrigins }
func (srv *server) SetAllowAllOrigins(v bool)   { srv.AllowAllOrigins = v }
func (srv *server) GetAllowedOrigins() string   { return srv.AllowedOrigins }
func (srv *server) SetAllowedOrigins(v string)  { srv.AllowedOrigins = v }
func (srv *server) GetDomain() string           { return srv.Domain }
func (srv *server) SetDomain(domain string)     { srv.Domain = domain }

// TLS metadata
func (srv *server) GetTls() bool                    { return srv.TLS }
func (srv *server) SetTls(hasTls bool)              { srv.TLS = hasTls }
func (srv *server) GetCertAuthorityTrust() string   { return srv.CertAuthorityTrust }
func (srv *server) SetCertAuthorityTrust(ca string) { srv.CertAuthorityTrust = ca }
func (srv *server) GetCertFile() string             { return srv.CertFile }
func (srv *server) SetCertFile(certFile string)     { srv.CertFile = certFile }
func (srv *server) GetKeyFile() string              { return srv.KeyFile }
func (srv *server) SetKeyFile(keyFile string)       { srv.KeyFile = keyFile }

func (srv *server) GetVersion() string             { return srv.Version }
func (srv *server) SetVersion(version string)      { srv.Version = version }
func (srv *server) GetPublisherID() string         { return srv.PublisherID }
func (srv *server) SetPublisherID(v string)        { srv.PublisherID = v }
func (srv *server) GetKeepUpToDate() bool          { return srv.KeepUpToDate }
func (srv *server) SetKeepUptoDate(v bool)         { srv.KeepUpToDate = v }
func (srv *server) GetKeepAlive() bool             { return srv.KeepAlive }
func (srv *server) SetKeepAlive(v bool)            { srv.KeepAlive = v }
func (srv *server) GetPermissions() []interface{}  { return srv.Permissions }
func (srv *server) SetPermissions(p []interface{}) { srv.Permissions = p }

// Init wires the GRPC server and loads config (called by Globular runtime).
func (srv *server) Init() error {
	if err := globular.InitService(srv); err != nil {
		return err
	}
	gs, err := globular.InitGrpcServer(srv, interceptors.ServerUnaryInterceptor, interceptors.ServerStreamInterceptor)
	if err != nil {
		return err
	}
	srv.grpcServer = gs
	return nil
}

// Save persists configuration.
func (srv *server) Save() error { return globular.SaveService(srv) }

// StartService launches the gRPC server.
func (srv *server) StartService() error { return globular.StartService(srv, srv.grpcServer) }

// StopService gracefully stops the gRPC server.
func (srv *server) StopService() error { return globular.StopService(srv, srv.grpcServer) }

// Event client helpers ////////////////////////////////////////////////////////

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

// Entry point //////////////////////////////////////////////////////////////////

func main() {
	// Initialize service with defaults
	s_impl := new(server)
	s_impl.Name = string(logpb.File_log_proto.Services().Get(0).FullName())
	s_impl.Proto = logpb.File_log_proto.Path()
	s_impl.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	s_impl.Port = defaultPort
	s_impl.Proxy = defaultProxy
	s_impl.Protocol = "grpc"
	s_impl.Domain, _ = config.GetDomain()
	s_impl.Address, _ = config.GetAddress()
	s_impl.Version = "0.0.1"
	s_impl.PublisherID = "localhost"
	s_impl.Description = "Cluster log collection and distribution service."
	s_impl.Keywords = []string{"log", "observability", "events", "monitoring"}
	s_impl.Repositories = make([]string, 0)
	s_impl.Discoveries = make([]string, 0)
	s_impl.Dependencies = []string{"event.EventService"}
	s_impl.Permissions = make([]interface{}, 0)
	s_impl.Process = -1
	s_impl.ProxyProcess = -1
	s_impl.AllowAllOrigins = allow_all_origins
	s_impl.AllowedOrigins = allowed_origins
	s_impl.KeepAlive = true
	s_impl.KeepUpToDate = true
	s_impl.Monitoring_Port = 9092
	s_impl.Root = config.GetDataDir()
	s_impl.RetentionHours = 24 * 7 // 7 days by default
	s_impl.SweepEverySeconds = 300 // every 5 minutes

	Utility.RegisterFunction("NewLogService_Client", log_client.NewLogService_Client)

	// CLI: service id and optional config path
	if len(os.Args) == 2 {
		s_impl.Id = os.Args[1]
	} else if len(os.Args) == 3 {
		s_impl.Id = os.Args[1]
		s_impl.ConfigPath = os.Args[2]
	}

	// Bootstrap / gRPC
	if err := s_impl.Init(); err != nil {
		fmt.Printf("fail to initialyse service %s: %s\n", s_impl.Name, s_impl.Id)
		return
	}
	if s_impl.Address == "" {
		s_impl.Address, _ = config.GetAddress()
	}

	// Open Badger store for logs
	s_impl.logs = storage_store.NewBadger_store()
	if err := s_impl.logs.Open(`{"path":"` + s_impl.Root + `", "name":"logs"}`); err != nil {
		fmt.Println("failed to open log store:", err)
	}

	// Kick off retention janitor (non-fatal if it errors internally)
	go s_impl.startRetentionJanitor()

	// Register the service and reflection
	logpb.RegisterLogServiceServer(s_impl.grpcServer, s_impl)
	reflection.Register(s_impl.grpcServer)

	// Prometheus metric: total log entries by (level, app, method)
	s_impl.logCount = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "log_entries_total",
			Help: "Total log entries by log level/application/method.",
		},
		[]string{"level", "application", "method"},
	)
	prometheus.MustRegister(s_impl.logCount)

	// Expose /metrics
	http.Handle("/metrics", promhttp.Handler())
	go func() {
		if err := http.ListenAndServe("0.0.0.0:"+Utility.ToString(s_impl.Monitoring_Port), nil); err != nil {
			// Non-fatal: metrics endpoint failed; service can still run.
			fmt.Println("prometheus metrics server error:", err)
		}
	}()

	// Go!
	_ = s_impl.StartService()
}
