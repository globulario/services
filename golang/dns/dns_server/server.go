package main

// Package main implements the DNS gRPC service used by Globular. It provides a
// storage-backed API to manage DNS records (A/AAAA/TXT/NS/MX/SOA/CNAME/URI/AFSDB/CAA)
// and a UDP DNS responder (via miekg/dns). All operational events are logged with slog.

import (
	"context"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/dns/dns_client"
	"github.com/globulario/services/golang/dns/dnspb"
	"github.com/globulario/services/golang/globular_client"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/interceptors"
	"github.com/globulario/services/golang/rbac/rbac_client"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/security"
	"github.com/globulario/services/golang/storage/storage_store"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// Default network configuration. (gRPC and reverse proxy)
var (
	defaultPort       = 10033
	defaultProxy      = 10034
	allow_all_origins = true
	allowed_origins   string

	// Global pointer used by the UDP DNS handler to reach the service.
	s *server
)

// server is the concrete implementation of the DNS service. It also implements
// Globular's service interface to be managed alongside other services.
type server struct {
	// Logger used for all local logging.
	Logger *slog.Logger

	// Generic service metadata required by Globular.
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
	Domain             string // Service domain (host) used by other services to reach us.
	Address            string // Full address (host[:port]) where this service can be reached.
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
	Plaform            string // Note: kept as-is for backward compatibility.
	KeepAlive          bool
	Permissions        []interface{}
	Dependencies       []string
	Process            int
	ProxyProcess       int
	ConfigPath         string
	LastError          string
	ModTime            int64
	Root               string // Root path for the local storage (badger).

	// The gRPC server instance.
	grpcServer *grpc.Server

	// DNS-specific configuration/state.
	DnsPort           int      // UDP port for the DNS server.
	Domains           []string // List of managed (authoritative) domains.
	ReplicationFactor int      // Reserved for storage backends that need it.

	// Persistent storage for records.
	store storage_store.Store

	connection_is_open bool
}

/* =========================
   Globular service methods
   ========================= */

// GetConfigurationPath returns the path where the service configuration is stored.
func (srv *server) GetConfigurationPath() string { return srv.ConfigPath }

// SetConfigurationPath sets the path where the service configuration is stored.
func (srv *server) SetConfigurationPath(path string) { srv.ConfigPath = path }

// GetAddress returns the HTTP address where the configuration can be fetched (/config).
func (srv *server) GetAddress() string { return srv.Address }

// SetAddress sets the HTTP address where the configuration can be fetched (/config).
func (srv *server) SetAddress(address string) { srv.Address = address }

// GetProcess returns the PID of the running process (or -1 if not running).
func (srv *server) GetProcess() int { return srv.Process }

// SetProcess sets the PID of the running process. If pid == -1, it closes the storage connection.
func (srv *server) SetProcess(pid int) {
	if pid == -1 {
		if srv.store != nil {
			_ = srv.store.Close()
		}
		srv.connection_is_open = false
	}
	srv.Process = pid
}

// GetProxyProcess returns the PID of the proxy process.
func (srv *server) GetProxyProcess() int { return srv.ProxyProcess }

// SetProxyProcess sets the PID of the proxy process.
func (srv *server) SetProxyProcess(pid int) { srv.ProxyProcess = pid }

// GetState returns the current service state.
func (srv *server) GetState() string { return srv.State }

// SetState sets the current service state.
func (srv *server) SetState(state string) { srv.State = state }

// GetLastError returns the last error message.
func (srv *server) GetLastError() string { return srv.LastError }

// SetLastError sets the last error message.
func (srv *server) SetLastError(err string) { srv.LastError = err }

// SetModTime sets the last modification time.
func (srv *server) SetModTime(modtime int64) { srv.ModTime = modtime }

// GetModTime gets the last modification time.
func (srv *server) GetModTime() int64 { return srv.ModTime }

// GetId returns the service instance id.
func (srv *server) GetId() string { return srv.Id }

// SetId sets the service instance id.
func (srv *server) SetId(id string) { srv.Id = id }

// GetName returns the gRPC service name.
func (srv *server) GetName() string { return srv.Name }

// SetName sets the gRPC service name.
func (srv *server) SetName(name string) { srv.Name = name }

// GetMac returns the service host MAC address.
func (srv *server) GetMac() string { return srv.Mac }

// SetMac sets the service host MAC address.
func (srv *server) SetMac(mac string) { srv.Mac = mac }

// GetDescription returns the service description.
func (srv *server) GetDescription() string { return srv.Description }

// SetDescription sets the service description.
func (srv *server) SetDescription(description string) { srv.Description = description }

// GetKeywords returns the service keywords.
func (srv *server) GetKeywords() []string { return srv.Keywords }

// SetKeywords sets the service keywords.
func (srv *server) SetKeywords(keywords []string) { srv.Keywords = keywords }

// Dist packages the service for distribution at the given path.
func (srv *server) Dist(path string) (string, error) { return globular.Dist(path, srv) }

// GetDependencies returns the list of required services.
func (srv *server) GetDependencies() []string {
	if srv.Dependencies == nil {
		srv.Dependencies = make([]string, 0)
	}
	return srv.Dependencies
}

// SetDependency adds a service dependency if it is not already present.
func (srv *server) SetDependency(dependency string) {
	if srv.Dependencies == nil {
		srv.Dependencies = make([]string, 0)
	}
	if !Utility.Contains(srv.Dependencies, dependency) {
		srv.Dependencies = append(srv.Dependencies, dependency)
	}
}

// GetChecksum returns the service checksum.
func (srv *server) GetChecksum() string { return srv.Checksum }

// SetChecksum sets the service checksum.
func (srv *server) SetChecksum(checksum string) { srv.Checksum = checksum }

// GetPlatform returns the platform (OS/Arch).
func (srv *server) GetPlatform() string { return srv.Plaform }

// SetPlatform sets the platform (OS/Arch).
func (srv *server) SetPlatform(platform string) { srv.Plaform = platform }

// GetRepositories returns the list of repositories.
func (srv *server) GetRepositories() []string { return srv.Repositories }

// SetRepositories sets the list of repositories.
func (srv *server) SetRepositories(repositories []string) { srv.Repositories = repositories }

// GetDiscoveries returns the list of discovery endpoints.
func (srv *server) GetDiscoveries() []string { return srv.Discoveries }

// SetDiscoveries sets the list of discovery endpoints.
func (srv *server) SetDiscoveries(discoveries []string) { srv.Discoveries = discoveries }

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

// GetProxy returns the reverse-proxy port (gRPC-Web).
func (srv *server) GetProxy() int { return srv.Proxy }

// SetProxy sets the reverse-proxy port (gRPC-Web).
func (srv *server) SetProxy(proxy int) { srv.Proxy = proxy }

// GetProtocol returns the transport protocol (http/https/tls/grpc).
func (srv *server) GetProtocol() string { return srv.Protocol }

// SetProtocol sets the transport protocol (http/https/tls/grpc).
func (srv *server) SetProtocol(protocol string) { srv.Protocol = protocol }

// GetAllowAllOrigins indicates whether all origins are allowed.
func (srv *server) GetAllowAllOrigins() bool { return srv.AllowAllOrigins }

// SetAllowAllOrigins sets whether all origins are allowed.
func (srv *server) SetAllowAllOrigins(allowAllOrigins bool) { srv.AllowAllOrigins = allowAllOrigins }

// GetAllowedOrigins returns the comma-separated list of allowed origins.
func (srv *server) GetAllowedOrigins() string { return srv.AllowedOrigins }

// SetAllowedOrigins sets the comma-separated list of allowed origins.
func (srv *server) SetAllowedOrigins(allowedOrigins string) { srv.AllowedOrigins = allowedOrigins }

// GetDomain returns the configured service domain (host).
func (srv *server) GetDomain() string { return srv.Domain }

// SetDomain sets the configured service domain (host).
func (srv *server) SetDomain(domain string) { srv.Domain = domain }

// GetTls returns true if TLS is enabled.
func (srv *server) GetTls() bool { return srv.TLS }

// SetTls enables or disables TLS.
func (srv *server) SetTls(hasTls bool) { srv.TLS = hasTls }

// GetCertAuthorityTrust returns the CA trust file path.
func (srv *server) GetCertAuthorityTrust() string { return srv.CertAuthorityTrust }

// SetCertAuthorityTrust sets the CA trust file path.
func (srv *server) SetCertAuthorityTrust(ca string) { srv.CertAuthorityTrust = ca }

// GetCertFile returns the TLS certificate file path.
func (srv *server) GetCertFile() string { return srv.CertFile }

// SetCertFile sets the TLS certificate file path.
func (srv *server) SetCertFile(certFile string) { srv.CertFile = certFile }

// GetKeyFile returns the TLS private key file path.
func (srv *server) GetKeyFile() string { return srv.KeyFile }

// SetKeyFile sets the TLS private key file path.
func (srv *server) SetKeyFile(keyFile string) { srv.KeyFile = keyFile }

// GetVersion returns the service version.
func (srv *server) GetVersion() string { return srv.Version }

// SetVersion sets the service version.
func (srv *server) SetVersion(version string) { srv.Version = version }

// GetPublisherID returns the publisher id.
func (srv *server) GetPublisherID() string { return srv.PublisherID }

// SetPublisherID sets the publisher id.
func (srv *server) SetPublisherID(PublisherID string) { srv.PublisherID = PublisherID }

// GetKeepUpToDate returns true if auto-update is enabled.
func (srv *server) GetKeepUpToDate() bool { return srv.KeepUpToDate }

// SetKeepUptoDate sets auto-update behavior.
func (srv *server) SetKeepUptoDate(val bool) { srv.KeepUpToDate = val }

// GetKeepAlive returns true if keep-alive is enabled.
func (srv *server) GetKeepAlive() bool { return srv.KeepAlive }

// SetKeepAlive sets keep-alive behavior.
func (srv *server) SetKeepAlive(val bool) { srv.KeepAlive = val }

// GetPermissions returns the action/resource permissions for the service.
func (srv *server) GetPermissions() []interface{} { return srv.Permissions }

// SetPermissions sets the action/resource permissions for the service.
func (srv *server) SetPermissions(permissions []interface{}) { srv.Permissions = permissions }

// GetRbacClient returns an RBAC client bound to this service's address.
func (srv *server) GetRbacClient() (*rbac_client.Rbac_Client, error) {
	Utility.RegisterFunction("NewRbacService_Client", rbac_client.NewRbacService_Client)
	client, err := globular_client.GetClient(srv.Address, "rbac.RbacService", "NewRbacService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*rbac_client.Rbac_Client), nil
}

// createPermission sets the caller (from ctx) as the owner of the given resource path.
func (srv *server) createPermission(ctx context.Context, path string) error {
	clientId, _, err := security.GetClientId(ctx)
	if err != nil {
		return err
	}
	rbac_client_, err := srv.GetRbacClient()
	if err != nil {
		return err
	}
	return rbac_client_.AddResourceOwner(path, "domain", clientId, rbacpb.SubjectType_ACCOUNT)
}

// Init initializes the service configuration and gRPC server. Must be called before StartService.
func (srv *server) Init() error {
	if err := globular.InitService(srv); err != nil {
		return err
	}
	var err error
	srv.grpcServer, err = globular.InitGrpcServer(srv, interceptors.ServerUnaryInterceptor, interceptors.ServerStreamInterceptor)
	if err != nil {
		return err
	}
	if len(srv.Root) == 0 {
		srv.Logger.Warn("StorageDataPath not set; using temporary path is acceptable if persistence is not required")
	}
	s = srv
	return nil
}

// Save persists the current configuration to disk.
func (srv *server) Save() error { return globular.SaveService(srv) }

// StartService starts the gRPC server for this DNS service.
func (srv *server) StartService() error { return globular.StartService(srv, srv.grpcServer) }

// StopService gracefully stops the gRPC server for this DNS service.
func (srv *server) StopService() error { return globular.StopService(srv, srv.grpcServer) }

// Stop is the gRPC endpoint that stops the service. It returns when the stop request is scheduled.
func (srv *server) Stop(context.Context, *dnspb.StopRequest) (*dnspb.StopResponse, error) {
	return &dnspb.StopResponse{}, srv.StopService()
}

/* =========================
   Storage / connection util
   ========================= */

// openConnection opens the persistent store if not already open.
func (srv *server) openConnection() error {
	if srv.connection_is_open {
		return nil
	}
	srv.store = storage_store.NewBadger_store()
	if err := srv.store.Open(`{
  "path":"` + srv.Root + `",
  "name":"dns",
  "syncWrites": true
}`); err != nil {
		return err
	}
	srv.connection_is_open = true
	return nil
}

// isManaged returns true if the given domain (or subdomain) is within a managed zone.
func (srv *server) isManaged(domain string) bool {
	for i := range srv.Domains {
		if strings.HasSuffix(domain, srv.Domains[i]) {
			return true
		}
	}
	return false
}

/* =====
   main
   ===== */

// main wires up the DNS service, starts the UDP DNS responder, and serves the gRPC API.
// Public API signatures (protobuf-generated service) are preserved exactly.
func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))

	// Concrete server implementation.
	s_impl := new(server)
	s_impl.Logger = logger
	Utility.RegisterType(s_impl)
	s_impl.Name = string(dnspb.File_dns_proto.Services().Get(0).FullName())
	s_impl.Proto = dnspb.File_dns_proto.Path()
	s_impl.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	s_impl.Port = defaultPort
	s_impl.Proxy = defaultProxy
	s_impl.Protocol = "grpc"
	s_impl.Domain, _ = config.GetDomain()
	s_impl.Address, _ = config.GetAddress()
	s_impl.Version = "0.0.1"
	s_impl.DnsPort = 5353
	s_impl.PublisherID = "localhost"
	s_impl.Permissions = make([]interface{}, 6)
	s_impl.AllowAllOrigins = allow_all_origins
	s_impl.AllowedOrigins = allowed_origins
	s_impl.Keywords = make([]string, 0)
	s_impl.Repositories = make([]string, 0)
	s_impl.Discoveries = make([]string, 0)
	s_impl.Dependencies = make([]string, 0)
	s_impl.Process = -1
	s_impl.ProxyProcess = -1
	s_impl.KeepAlive = true
	s_impl.KeepUpToDate = true
	Utility.RegisterFunction("NewDnsService_Client", dns_client.NewDnsService_Client)

	// Local storage root.
	s_impl.Root = config.GetDataDir()
	Utility.CreateDirIfNotExist(s_impl.Root)

	// Default permissions for DNS operations on a given domain.
	s_impl.Permissions[0] = map[string]interface{}{"action": "/dns.DnsService/SetA", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "write"}}}
	s_impl.Permissions[1] = map[string]interface{}{"action": "/dns.DnsService/SetAAAA", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "write"}}}
	s_impl.Permissions[5] = map[string]interface{}{"action": "/dns.DnsService/SetText", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "write"}}}
	s_impl.Permissions[2] = map[string]interface{}{"action": "/dns.DnsService/RemoveA", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "delete"}}}
	s_impl.Permissions[3] = map[string]interface{}{"action": "/dns.DnsService/RemoveAAAA", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "delete"}}}
	s_impl.Permissions[4] = map[string]interface{}{"action": "/dns.DnsService/RemoveText", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "delete"}}}

	// Parse optional args (id, config path).
	if len(os.Args) == 2 {
		s_impl.Id = os.Args[1]
	} else if len(os.Args) == 3 {
		s_impl.Id = os.Args[1]
		s_impl.ConfigPath = os.Args[2]
	}

	// Initialize service and gRPC server.
	if err := s_impl.Init(); err != nil {
		logger.Error("init failed", "service", s_impl.Name, "id", s_impl.Id, "err", err)
	}

	if s_impl.Address == "" {
		s_impl.Address, _ = config.GetAddress()
	}

	// Start the UDP DNS responder.
	go func(port int) {
		if err := ServeDns(port); err != nil {
			s_impl.Logger.Error("ServeDns failed", "port", port, "err", err)
		} else {
			s_impl.Logger.Info("ServeDns stopped", "port", port)
		}
	}(s_impl.DnsPort)

	// Register gRPC endpoints and reflection.
	dnspb.RegisterDnsServiceServer(s_impl.grpcServer, s_impl)
	reflection.Register(s_impl.grpcServer)

	// Open persistent store.
	if err := s_impl.openConnection(); err != nil {
		s_impl.Logger.Error("openConnection failed", "err", err)
		return
	}

	// Start the gRPC service.
	if err := s_impl.StartService(); err != nil {
		s_impl.Logger.Error("StartService failed", "err", err)
	}
}

/* ======================
   RBAC helper operations
   ====================== */

// GetRbacClient returns an RBAC client at a given address.
func GetRbacClient(address string) (*rbac_client.Rbac_Client, error) {
	Utility.RegisterFunction("NewRbacService_Client", rbac_client.NewRbacService_Client)
	client, err := globular_client.GetClient(address, "rbac.RbacService", "NewRbacService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*rbac_client.Rbac_Client), nil
}

// setActionResourcesPermissions sets permissions for action/resources on the RBAC service.
func (srv *server) setActionResourcesPermissions(permissions map[string]interface{}) error {
	rbac_client_, err := GetRbacClient(srv.Address)
	if err != nil {
		return err
	}
	return rbac_client_.SetActionResourcesPermissions(permissions)
}

// orderIPsByPrivacy places private IPs first while keeping relative order within the privacy group.
func orderIPsByPrivacy(ips []string) []string {
	cloned := make([]string, len(ips))
	copy(cloned, ips)
	// stable-ish: prefer private over public
	out := make([]string, 0, len(ips))
	priv, pub := make([]string, 0), make([]string, 0)
	for _, s := range cloned {
		ip := net.ParseIP(s)
		if ip != nil && ip.IsPrivate() {
			priv = append(priv, s)
		} else {
			pub = append(pub, s)
		}
	}
	out = append(out, priv...)
	out = append(out, pub...)
	return out
}
