package main

// DNS gRPC service with storage-backed records and a UDP responder.

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/globulario/services/golang/backup_hook"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/dephealth"
	"github.com/globulario/services/golang/dns/dns_client"
	"github.com/globulario/services/golang/dns/dnspb"
	"github.com/globulario/services/golang/globular_client"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/netutil"
	"github.com/globulario/services/golang/policy"
	"github.com/globulario/services/golang/rbac/rbac_client"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/resource/resourcepb"
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
	defaultPort       = 10006
	defaultProxy      = 10007
	allowAllOrigins   = true
	allowedOriginsStr = ""

	// Global service pointer used by UDP DNS handler.
	srv *server
)

// Version information (set via ldflags during build)
var (
	Version   = "0.0.1"
	BuildTime = "unknown"
	GitCommit = "unknown"
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

	// ScyllaDB storage (shared across all DNS instances in the cluster)
	ScyllaHosts              []string `json:"ScyllaHosts"`
	ScyllaPort               int      `json:"ScyllaPort"`
	ScyllaReplicationFactor  int      `json:"ScyllaReplicationFactor"`

	// storage
	store              storage_store.Store
	connection_is_open bool

	// Dependency health watchdog — gates RPCs when ScyllaDB is unreachable.
	depHealth *dephealth.Watchdog
	depCancel context.CancelFunc

	mu sync.RWMutex
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
		if srv.depCancel != nil {
			srv.depCancel()
		}
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

// RolesDefault returns an empty set — roles are defined externally in
// cluster-roles.json and per-service policy files.
func (srv *server) RolesDefault() []resourcepb.Role {
	return []resourcepb.Role{}
}

// RBAC helper bound to this service address
func (srv *server) getRbacClient() (*rbac_client.Rbac_Client, error) {
	Utility.RegisterFunction("NewRbacService_Client", rbac_client.NewRbacService_Client)
	client, err := globular_client.GetClient(srv.Address, "rbac.RbacService", "NewRbacService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*rbac_client.Rbac_Client), nil
}

// createPermission: set caller as owner of resource path
func (srv *server) createPermission(ctx context.Context, path string) error {

	clientId, token, err := security.GetClientId(ctx)
	if err != nil {
		return err
	}
	rbacClient, err := srv.getRbacClient()
	if err != nil {
		return err
	}
	return rbacClient.AddResourceOwner(token, path, clientId, "domain", rbacpb.SubjectType_ACCOUNT)
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

	// Dependency health watchdog — gates RPCs when ScyllaDB is unreachable.
	srv.depHealth = dephealth.NewWatchdog(srv.Logger,
		dephealth.Dep("scylladb", func(ctx context.Context) error {
			// Use a lightweight read to verify ScyllaDB connectivity.
			_, err := srv.store.GetItem("__healthcheck__")
			// GetItem returns an error for missing keys on some stores;
			// any error that is NOT "not found" indicates a real problem.
			// A nil-data / not-found response means ScyllaDB is reachable.
			if err != nil && !strings.Contains(err.Error(), "not found") {
				return err
			}
			return nil
		}),
	)
	depCtx, depCancel := context.WithCancel(context.Background())
	srv.depCancel = depCancel
	go srv.depHealth.Start(depCtx)

	// I will get the existing domains from storage
	domains, err := srv.loadDomainsFromStore()
	if err == nil && domains != nil {
		srv.mu.Lock()
		srv.Domains = domains
		srv.mu.Unlock()
	}

	// Keep the hot-path domain cache fresh with changes from other nodes.
	srv.startDomainCacheRefresh(depCtx)

	// Bootstrap default internal zone if no domains configured
	if err := srv.ensureDefaultInternalZone(); err != nil {
		logger.Warn("Failed to bootstrap default internal zone", "err", err)
		// Non-fatal - continue startup
	}

	return nil
}

// ensureDefaultInternalZone creates a default internal DNS zone if none exist.
// This is called during Init() to bootstrap Day-0 installations.
func (srv *server) ensureDefaultInternalZone() error {
	// Read from ScyllaDB to get the current state across the mesh.
	domains, _ := srv.loadDomainsFromStore()
	if len(domains) > 0 {
		// Already have zones configured
		return nil
	}

	// Determine internal domain name
	internalDomain := netutil.DefaultClusterDomain()
	if !strings.HasSuffix(internalDomain, ".") {
		internalDomain += "."
	}

	logger.Info("Bootstrapping default internal DNS zone", "domain", internalDomain)

	// Create the zone by calling SetDomains
	_, err := srv.SetDomains(context.Background(), &dnspb.SetDomainsRequest{
		Domains: []string{internalDomain},
	})
	if err != nil {
		return fmt.Errorf("bootstrap zone: %w", err)
	}

	// Add baseline A records for core services.
	// Note: dns.<domain> is created automatically by ensureZoneAuthority
	// (called from SetDomains above) with the real node IP.
	baseRecords := []struct {
		name string
		ip   string
	}{
		{"globular-gateway." + internalDomain, "127.0.0.1"},
		{"controller." + internalDomain, "127.0.0.1"},
		{"etcd." + internalDomain, "127.0.0.1"},
	}

	for _, rec := range baseRecords {
		_, err := srv.SetA(context.Background(), &dnspb.SetARequest{
			Domain: rec.name,
			A:      rec.ip,
			Ttl:    300,
		})
		if err != nil {
			logger.Warn("Failed to add bootstrap DNS record", "record", rec.name, "err", err)
			// Continue adding other records even if one fails
		} else {
			logger.Info("Added bootstrap DNS record", "record", rec.name, "ip", rec.ip)
		}
	}

	logger.Info("Default internal zone created", "domain", internalDomain)
	return nil
}

func (srv *server) Save() error { return globular.SaveService(srv) }
func (srv *server) StartService() error {
	if srv.Logger == nil {
		srv.Logger = logger
	}
	if srv.DnsPort == 0 {
		srv.DnsPort = 53
	}

	go func(port int) {
		if err := ServeDns(port); err != nil && logger != nil {
			logger.Error("ServeDns failed", "port", port, "err", err)
		}
	}(srv.DnsPort)

	return globular.StartService(srv, srv.grpcServer)
}
func (srv *server) StopService() error          { return globular.StopService(srv, srv.grpcServer) }
func (srv *server) GetGrpcServer() *grpc.Server { return srv.grpcServer }

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

	// ScyllaDB hosts from the Tier-0 cluster key in etcd — never hardcode.
	hosts := srv.ScyllaHosts
	if len(hosts) == 0 {
		if h, err := config.GetScyllaHosts(); err == nil && len(h) > 0 {
			hosts = h
		} else if ip := detectLocalIP(); ip != "" {
			hosts = []string{ip}
		} else {
			return fmt.Errorf("scylla hosts: no hosts available (etcd key missing and IP detection failed)")
		}
	}
	port := srv.ScyllaPort
	if port == 0 {
		port = 9042
	}
	rf := srv.ScyllaReplicationFactor
	if rf <= 0 {
		rf = 1
	}

	opts := fmt.Sprintf(`{"hosts":%s,"port":%d,"keyspace":"dns","table":"records","replication_factor":%d,"consistency":"one"}`,
		mustJSON(hosts), port, rf)

	srv.store = storage_store.NewScylla_store("", "", rf)
	if err := srv.store.Open(opts); err != nil {
		return fmt.Errorf("scylla dns store: %w", err)
	}
	srv.connection_is_open = true
	return nil
}

// detectLocalIP returns the node's outbound IP (same technique used by ScyllaDB
// post-install and the node-agent). Returns "" if detection fails.
func detectLocalIP() string {
	conn, err := net.DialTimeout("udp", "8.8.8.8:80", time.Second)
	if err != nil {
		return ""
	}
	defer conn.Close()
	return conn.LocalAddr().(*net.UDPAddr).IP.String()
}

// mustJSON marshals v to a JSON string, panics on error (only used for static data).
func mustJSON(v interface{}) string {
	b, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func (srv *server) isManaged(domain string) bool {
	// Normalize: ensure trailing dot for comparison (domains are stored with trailing dot).
	domain = strings.ToLower(domain)
	if !strings.HasSuffix(domain, ".") {
		domain += "."
	}
	srv.mu.RLock()
	defer srv.mu.RUnlock()
	for _, d := range srv.Domains {
		if strings.HasSuffix(domain, d) {
			return true
		}
	}
	return false
}

// requireHealthy gates RPCs when distributed dependencies are down.
func (srv *server) requireHealthy() error {
	if srv.depHealth == nil {
		return nil
	}
	return srv.depHealth.RequireHealthy()
}

// loadDomainsFromStore reads the domain list from ScyllaDB on demand.
// Returns nil (empty list) if no domains are stored yet.
func (srv *server) loadDomainsFromStore() ([]string, error) {
	data, err := srv.store.GetItem("domains")
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, nil
	}
	var domains []string
	if err := json.Unmarshal(data, &domains); err != nil {
		return nil, fmt.Errorf("unmarshal domains: %w", err)
	}
	return domains, nil
}

// refreshDomainCache reloads srv.Domains from ScyllaDB so that the in-memory
// cache used by the UDP/TCP hot path stays current with changes made on other
// nodes in the mesh.
func (srv *server) refreshDomainCache() {
	domains, err := srv.loadDomainsFromStore()
	if err != nil {
		logger.Warn("domain cache refresh failed", "err", err)
		return
	}
	if domains == nil {
		return
	}
	srv.mu.Lock()
	srv.Domains = domains
	srv.mu.Unlock()
}

// startDomainCacheRefresh launches a background goroutine that re-reads the
// domain list from ScyllaDB every 30 seconds. This keeps the hot-path cache
// consistent with changes made by other DNS instances in the cluster.
func (srv *server) startDomainCacheRefresh(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				srv.refreshDomainCache()
			}
		}
	}()
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
	fmt.Fprintf(os.Stdout, `
%s - DNS service with storage-backed records and UDP/TCP resolution

USAGE:
  %s [OPTIONS] [<id>] [<configPath>]

OPTIONS:
  --debug         Enable debug logging
  --version       Print version information as JSON and exit
  --help          Show this usage information and exit
  --describe      Print service description as JSON (no etcd/config access)
  --health        Print service health as JSON (no etcd/config access)

ARGUMENTS:
  <id>            Service instance ID (optional)
  <configPath>    Optional path to configuration file

FEATURES:
  • Storage-backed DNS records and zones
  • UDP and TCP DNS resolution on port 53
  • Full CRUD operations for records (A, AAAA, CNAME, MX, TXT, NS, SOA)
  • Zone management with RBAC permissions
  • CAP_NET_BIND_SERVICE support for privileged port binding
  • Integration with Globular's distributed storage backend

EXAMPLES:
  %s --version
  %s --describe
  %s --debug dns-1
  %s dns-1 /etc/globular/dns/config.json

`, exe, exe, exe, exe, exe, exe)
}

func printVersion() {
	data := map[string]string{
		"service":    "dns",
		"version":    Version,
		"build_time": BuildTime,
		"git_commit": GitCommit,
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(data)
}

func initializeServerDefaults() *server {
	cfg := DefaultConfig()

	s := &server{
		Logger:            logger,
		Name:              string(dnspb.File_dns_proto.Services().Get(0).FullName()),
		Proto:             dnspb.File_dns_proto.Path(),
		Path:              func() string { p, _ := filepath.Abs(filepath.Dir(os.Args[0])); return p }(),
		Port:              cfg.Port,
		Proxy:             cfg.Proxy,
		AllowAllOrigins:   cfg.AllowAllOrigins,
		AllowedOrigins:    cfg.AllowedOrigins,
		Protocol:          cfg.Protocol,
		Domain:            cfg.Domain,
		Address:           cfg.Address,
		Description:       "DNS service with storage-backed records, zones, and UDP/TCP resolution",
		Keywords:          []string{"dns", "resolver", "records", "zones", "nameserver", "udp", "tcp"},
		Repositories:      globular.CloneStringSlice(cfg.Repositories),
		Discoveries:       globular.CloneStringSlice(cfg.Discoveries),
		Version:           Version,
		PublisherID:       cfg.PublisherID,
		KeepUpToDate:      cfg.KeepUpToDate,
		KeepAlive:         cfg.KeepAlive,
		Process:           cfg.Process,
		ProxyProcess:      cfg.ProxyProcess,
		DnsPort:                 cfg.DnsPort,
		Domains:                 globular.CloneStringSlice(cfg.Domains),
		ReplicationFactor:       cfg.ReplicationFactor,
		Root:                    cfg.Root,
		ScyllaHosts:             cfg.ScyllaHosts,
		ScyllaPort:              cfg.ScyllaPort,
		ScyllaReplicationFactor: cfg.ScyllaReplicationFactor,
		Dependencies:            globular.CloneStringSlice(cfg.Dependencies),
		Permissions: make([]any, 0),
	}

	if s.Root == "" {
		s.Root = config.GetDataDir()
	}
	_ = Utility.CreateDirIfNotExist(s.Root)

	// Ensure default address uses current port with routable IP
	if s.Address == "" {
		if ip := detectLocalIP(); ip != "" {
			s.Address = fmt.Sprintf("%s:%d", ip, s.Port)
		}
	}

	// set package-global for UDP handler
	srv = s
	return s
}

func setupGrpcService(s *server) {
	dnspb.RegisterDnsServiceServer(s.grpcServer, s)
	backup_hook.Register(s.grpcServer, s.newBackupHookHandler())
	reflection.Register(s.grpcServer)
}

// -----------------------------------------------------------------------------
// Main
// -----------------------------------------------------------------------------

func main() {
	srv = initializeServerDefaults()
	Utility.RegisterType(srv)

	var (
		enableDebug  = flag.Bool("debug", false, "enable debug logging")
		showVersion  = flag.Bool("version", false, "print version information as JSON and exit")
		showHelp     = flag.Bool("help", false, "show usage information and exit")
		showDescribe = flag.Bool("describe", false, "print service description as JSON and exit")
		showHealth   = flag.Bool("health", false, "print service health status as JSON and exit")
	)

	flag.Usage = printUsage
	flag.Parse()

	if *enableDebug {
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
		srv.Logger = logger
	}

	if *showVersion {
		printVersion()
		return
	}

	if *showHelp {
		printUsage()
		return
	}

	// Register method→action mappings for interceptor resolution.
	policy.GlobalResolver().Register([]policy.Permission{
		{Method: "/dns.DnsService/GetA", Action: "dns.record.read"},
		{Method: "/dns.DnsService/GetAAAA", Action: "dns.record.read"},
		{Method: "/dns.DnsService/GetText", Action: "dns.record.read"},
		{Method: "/dns.DnsService/GetNs", Action: "dns.record.read"},
		{Method: "/dns.DnsService/GetCName", Action: "dns.record.read"},
		{Method: "/dns.DnsService/GetMx", Action: "dns.record.read"},
		{Method: "/dns.DnsService/GetSoa", Action: "dns.record.read"},
		{Method: "/dns.DnsService/GetUri", Action: "dns.record.read"},
		{Method: "/dns.DnsService/GetCaa", Action: "dns.record.read"},
		{Method: "/dns.DnsService/GetAfsdb", Action: "dns.record.read"},
		{Method: "/dns.DnsService/SetDomains", Action: "dns.zone.write"},
		{Method: "/dns.DnsService/RemoveDomains", Action: "dns.zone.delete"},
		{Method: "/dns.DnsService/SetA", Action: "dns.record.write"},
		{Method: "/dns.DnsService/RemoveA", Action: "dns.record.delete"},
		{Method: "/dns.DnsService/SetAAAA", Action: "dns.record.write"},
		{Method: "/dns.DnsService/RemoveAAAA", Action: "dns.record.delete"},
		{Method: "/dns.DnsService/SetText", Action: "dns.record.write"},
		{Method: "/dns.DnsService/RemoveText", Action: "dns.record.delete"},
		{Method: "/dns.DnsService/SetNs", Action: "dns.record.write"},
		{Method: "/dns.DnsService/RemoveNs", Action: "dns.record.delete"},
		{Method: "/dns.DnsService/SetCName", Action: "dns.record.write"},
		{Method: "/dns.DnsService/RemoveCName", Action: "dns.record.delete"},
		{Method: "/dns.DnsService/SetMx", Action: "dns.record.write"},
		{Method: "/dns.DnsService/RemoveMx", Action: "dns.record.delete"},
		{Method: "/dns.DnsService/SetSoa", Action: "dns.record.write"},
		{Method: "/dns.DnsService/RemoveSoa", Action: "dns.record.delete"},
		{Method: "/dns.DnsService/SetUri", Action: "dns.record.write"},
		{Method: "/dns.DnsService/RemoveUri", Action: "dns.record.delete"},
		{Method: "/dns.DnsService/SetCaa", Action: "dns.record.write"},
		{Method: "/dns.DnsService/RemoveCaa", Action: "dns.record.delete"},
		{Method: "/dns.DnsService/SetAfsdb", Action: "dns.record.write"},
		{Method: "/dns.DnsService/RemoveAfsdb", Action: "dns.record.delete"},
		{Method: "/dns.DnsService/Stop", Action: "dns.stop"},
	})

	if *showDescribe {
		globular.HandleDescribeFlag(srv, logger)
		return
	}

	if *showHealth {
		health := map[string]interface{}{
			"service": srv.Name,
			"status":  "healthy",
			"version": srv.Version,
		}
		data, _ := json.MarshalIndent(health, "", "  ")
		fmt.Println(string(data))
		return
	}

	args := flag.Args()

	// DNS is a well-known service with fixed port (10006) - skip dynamic allocation
	// to ensure CLI can connect during bootstrap before service discovery is available
	// if err := globular.AllocatePortIfNeeded(srv, args); err != nil {
	// 	logger.Error("port allocation failed", "error", err)
	// 	os.Exit(1)
	// }

	globular.ParsePositionalArgs(srv, args)
	globular.LoadRuntimeConfig(srv)

	// preserve historical defaults when config is absent
	if srv.Domain == "" || srv.Domain == "localhost" {
		srv.Domain = netutil.DefaultClusterDomain()
	}
	if srv.Address == "" || strings.HasPrefix(srv.Address, "127.0.0.1:") || strings.HasPrefix(srv.Address, "localhost:") {
		if ip := detectLocalIP(); ip != "" {
			srv.Address = fmt.Sprintf("%s:%d", ip, srv.Port)
		}
	}

	Utility.RegisterFunction("NewDnsService_Client", dns_client.NewDnsService_Client)

	start := time.Now()
	if err := srv.Init(); err != nil {
		logger.Error("init failed", "service", srv.Name, "id", srv.Id, "err", err)
		os.Exit(1)
	}

	setupGrpcService(srv)

	logger.Info("service ready",
		"service", srv.Name,
		"port", srv.Port,
		"proxy", srv.Proxy,
		"protocol", srv.Protocol,
		"domain", srv.Domain,
		"listen_ms", time.Since(start).Milliseconds())

	lifecycle := globular.NewLifecycleManager(srv, logger)
	if err := lifecycle.Start(); err != nil {
		logger.Error("service start failed", "service", srv.Name, "err", err)
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

func (srv *server) setActionResourcesPermissions(token string, permissions map[string]interface{}) error {
	rbacClient, err := GetRbacClient(srv.Address)
	if err != nil {
		return err
	}
	return rbacClient.SetActionResourcesPermissions(token, permissions)
}
