package main

// DNS gRPC service with storage-backed records and a UDP responder.

import (
	"context"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/dns/dns_client"
	"github.com/globulario/services/golang/dns/dnspb"
	"github.com/globulario/services/golang/globular_client"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/rbac/rbac_client"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/security"
	"github.com/globulario/services/golang/storage/storage_store"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// -----------------------------------------------------------------------------
// Defaults & globals
// -----------------------------------------------------------------------------

var (
	defaultPort       = 10033
	defaultProxy      = 10034
	allowAllOrigins   = true
	allowedOriginsStr = ""

	// Global service pointer used by UDP DNS handler.
	s *server
)

// STDERR logger so --describe/--health JSON stays clean on STDOUT
var logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

// -----------------------------------------------------------------------------
// Service definition
// -----------------------------------------------------------------------------

type server struct {
	// Logger
	Logger *slog.Logger

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
	Plaform            string // kept for API compatibility
	KeepAlive          bool
	Permissions        []interface{}
	Dependencies       []string
	Process            int
	ProxyProcess       int
	ConfigPath         string
	LastError          string
	ModTime            int64
	Root               string // storage root

	// gRPC
	grpcServer *grpc.Server

	// DNS-specific
	DnsPort           int
	Domains           []string
	ReplicationFactor int

	// storage
	store              storage_store.Store
	connection_is_open bool
}

// -----------------------------------------------------------------------------
// Globular runtime getters/setters (signatures unchanged)
// -----------------------------------------------------------------------------

func (srv *server) GetConfigurationPath() string     { return srv.ConfigPath }
func (srv *server) SetConfigurationPath(path string) { srv.ConfigPath = path }
func (srv *server) GetAddress() string               { return srv.Address }
func (srv *server) SetAddress(address string)        { srv.Address = address }
func (srv *server) GetProcess() int                  { return srv.Process }
func (srv *server) SetProcess(pid int) {
	if pid == -1 {
		if srv.store != nil {
			_ = srv.store.Close()
		}
		srv.connection_is_open = false
	}
	srv.Process = pid
}
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

// RBAC helper bound to this service address
func (srv *server) GetRbacClient() (*rbac_client.Rbac_Client, error) {
	Utility.RegisterFunction("NewRbacService_Client", rbac_client.NewRbacService_Client)
	client, err := globular_client.GetClient(srv.Address, "rbac.RbacService", "NewRbacService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*rbac_client.Rbac_Client), nil
}

// createPermission: set caller as owner of resource path
func (srv *server) createPermission(ctx context.Context, path string) error {
	clientId, _, err := security.GetClientId(ctx)
	if err != nil {
		return err
	}
	rbacClient, err := srv.GetRbacClient()
	if err != nil {
		return err
	}
	return rbacClient.AddResourceOwner(path, "domain", clientId, rbacpb.SubjectType_ACCOUNT)
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

	if srv.Root == "" {
		logger.Warn("StorageDataPath not set; using temp dir will be non-persistent")
	}

	// single storage open here
	if err := srv.openConnection(); err != nil {
		return err
	}

	// expose to UDP DNS handler
	s = srv
	return nil
}
func (srv *server) Save() error         { return globular.SaveService(srv) }
func (srv *server) StartService() error { return globular.StartService(srv, srv.grpcServer) }
func (srv *server) StopService() error  { return globular.StopService(srv, srv.grpcServer) }

// gRPC Stop endpoint
func (srv *server) Stop(context.Context, *dnspb.StopRequest) (*dnspb.StopResponse, error) {
	return &dnspb.StopResponse{}, srv.StopService()
}

// -----------------------------------------------------------------------------
// Storage / utils
// -----------------------------------------------------------------------------

func (srv *server) openConnection() error {
	if srv.connection_is_open {
		return nil
	}
	srv.store = storage_store.NewBadger_store()
	if err := srv.store.Open(`{"path":"` + srv.Root + `","name":"dns","syncWrites":true}`); err != nil {
		return err
	}
	srv.connection_is_open = true
	return nil
}

func (srv *server) isManaged(domain string) bool {
	for _, d := range srv.Domains {
		if strings.HasSuffix(domain, d) {
			return true
		}
	}
	return false
}

// orderIPsByPrivacy: private first, stable-ish order
func orderIPsByPrivacy(ips []string) []string {
	cloned := make([]string, len(ips))
	copy(cloned, ips)
	priv, pub := make([]string, 0), make([]string, 0)
	for _, s := range cloned {
		ip := net.ParseIP(s)
		if ip != nil && ip.IsPrivate() {
			priv = append(priv, s)
		} else {
			pub = append(pub, s)
		}
	}
	return append(priv, pub...)
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
  ` + exe + ` dns-1 /etc/globular/dns/config.json

`)
}

// -----------------------------------------------------------------------------
// Main
// -----------------------------------------------------------------------------

func main() {
	// Build skeleton (no etcd/config yet)
	srv := new(server)
	srv.Logger = logger
	Utility.RegisterType(srv)

	srv.Name = string(dnspb.File_dns_proto.Services().Get(0).FullName())
	srv.Proto = dnspb.File_dns_proto.Path()
	srv.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	srv.Port = defaultPort
	srv.Proxy = defaultProxy
	srv.Protocol = "grpc"
	srv.Version = "0.0.1"
	srv.PublisherID = "localhost"
	srv.Description = "DNS service"
	srv.Keywords = []string{"DNS", "Records", "Resolver"}
	srv.Repositories = []string{}
	srv.Discoveries = []string{"log.LogService", "rbac.RbacService"}
	srv.Dependencies = []string{}
	srv.Process = -1
	srv.ProxyProcess = -1
	srv.KeepAlive = true
	srv.KeepUpToDate = true
	srv.AllowAllOrigins = allowAllOrigins
	srv.AllowedOrigins = allowedOriginsStr
	srv.DnsPort = 53 // standard DNS port
	srv.Root = config.GetDataDir()
	_ = Utility.CreateDirIfNotExist(srv.Root)

	// Dynamic client registration
	Utility.RegisterFunction("NewDnsService_Client", dns_client.NewDnsService_Client)

	// Default permissions for DNS ops on a given domain
	srv.Permissions = []interface{}{
		map[string]interface{}{"action": "/dns.DnsService/SetA", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "write"}}},
		map[string]interface{}{"action": "/dns.DnsService/SetAAAA", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "write"}}},
		map[string]interface{}{"action": "/dns.DnsService/RemoveA", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "delete"}}},
		map[string]interface{}{"action": "/dns.DnsService/RemoveAAAA", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "delete"}}},
		map[string]interface{}{"action": "/dns.DnsService/RemoveText", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "delete"}}},
		map[string]interface{}{"action": "/dns.DnsService/SetText", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "write"}}},
	}

	// CLI flags BEFORE touching config
	args := os.Args[1:]
	if len(args) == 0 {
		printUsage()
		return
	}
	for _, a := range args {
		switch strings.ToLower(a) {
		case "--describe":
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
			b, err := globular.HealthJSON(srv, &globular.HealthOptions{Timeout: 1500 * time.Millisecond})
			if err != nil {
				logger.Error("health error", "service", srv.Name, "id", srv.Id, "err", err)
				os.Exit(2)
			}
			os.Stdout.Write(b)
			os.Stdout.Write([]byte("\n"))
			return
		}
	}

	// Optional positional args: <id> [configPath]
	if len(args) == 1 && !strings.HasPrefix(args[0], "-") {
		srv.Id = args[0]
	} else if len(args) == 2 && !strings.HasPrefix(args[0], "-") && !strings.HasPrefix(args[1], "-") {
		srv.Id = args[0]
		srv.ConfigPath = args[1]
	}

	// Safe to touch config now
	if d, err := config.GetDomain(); err == nil {
		srv.Domain = d
	} else {
		srv.Domain = "localhost"
	}
	if a, err := config.GetAddress(); err == nil && strings.TrimSpace(a) != "" {
		srv.Address = a
	}

	start := time.Now()
	if err := srv.Init(); err != nil {
		logger.Error("init failed", "service", srv.Name, "id", srv.Id, "err", err)
		os.Exit(1)
	}

	// Start UDP DNS responder
	go func(port int) {
		if err := ServeDns(port); err != nil {
			logger.Error("ServeDns failed", "port", port, "err", err)
		} else {
			logger.Info("ServeDns stopped", "port", port)
		}
	}(srv.DnsPort)

	// Register gRPC & reflection
	dnspb.RegisterDnsServiceServer(srv.grpcServer, srv)
	reflection.Register(srv.grpcServer)

	logger.Info("service ready",
		"service", srv.Name,
		"port", srv.Port,
		"proxy", srv.Proxy,
		"protocol", srv.Protocol,
		"domain", srv.Domain,
		"listen_ms", time.Since(start).Milliseconds())

	// Start gRPC serving
	if err := srv.StartService(); err != nil {
		logger.Error("StartService failed", "err", err)
		os.Exit(1)
	}
}

// -----------------------------------------------------------------------------
// Extra RBAC helper (standalone, unchanged behavior)
// -----------------------------------------------------------------------------

func GetRbacClient(address string) (*rbac_client.Rbac_Client, error) {
	Utility.RegisterFunction("NewRbacService_Client", rbac_client.NewRbacService_Client)
	client, err := globular_client.GetClient(address, "rbac.RbacService", "NewRbacService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*rbac_client.Rbac_Client), nil
}

func (srv *server) setActionResourcesPermissions(permissions map[string]interface{}) error {
	rbacClient, err := GetRbacClient(srv.Address)
	if err != nil {
		return err
	}
	return rbacClient.SetActionResourcesPermissions(permissions)
}
