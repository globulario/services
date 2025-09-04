package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/globulario/services/golang/authentication/authentication_client"
	"github.com/globulario/services/golang/authentication/authenticationpb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/globular_client"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/ldap/ldap_client"
	"github.com/globulario/services/golang/rbac/rbac_client"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/resource/resource_client"
	"github.com/globulario/services/golang/resource/resourcepb"
	Utility "github.com/globulario/utility"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// Defaults
var (
	defaultPort  = 10029
	defaultProxy = 10030

	allowAllOrigins   = true
	allowedOriginsStr = ""
)

// Service impl consumed by Globular
// (Keep public method signatures unchanged.)
type server struct {
	Id              string
	Name            string
	Mac             string
	Domain          string
	Address         string
	Path            string
	Proto           string
	Port            int
	Proxy           int
	AllowAllOrigins bool
	AllowedOrigins  string
	Protocol        string
	Version         string
	PublisherID     string
	KeepUpToDate    bool
	KeepAlive       bool
	Checksum        string
	Plaform         string
	Description     string
	Keywords        []string
	Repositories    []string
	Discoveries     []string
	Process         int
	ProxyProcess    int
	ConfigPath      string
	ConfigPort      int
	LastError       string
	ModTime         int64
	State           string
	TLS             bool

	CertFile           string
	KeyFile            string
	CertAuthorityTrust string

	Permissions  []interface{}
	Dependencies []string

	WatchSessionsDelay int
	SessionTimeout     int
	LdapConnectionId   string

	exit_ chan bool

	grpcServer *grpc.Server

	// used to cut infinite recursion.
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
func (srv *server) GetPermissions() []interface{}   { return srv.Permissions }
func (srv *server) SetPermissions(v []interface{})  { srv.Permissions = v }

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
func (srv *server) Save() error         { return globular.SaveService(srv) }
func (srv *server) StartService() error { return globular.StartService(srv, srv.grpcServer) }
func (srv *server) StopService() error  { return globular.StopService(srv, srv.grpcServer) }

// --- logger to STDERR so stdout stays clean for JSON outputs ---
var logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

// ////////////////////// LDAP helpers //////////////////////
func GetLdapClient(address string) (*ldap_client.LDAP_Client, error) {
	client, err := globular_client.GetClient(address, "ldap.LdapService", "ldap_client.NewLdapService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*ldap_client.LDAP_Client), nil
}
func (srv *server) authenticateLdap(userId, password string) error {
	ldapClient, err := GetLdapClient(srv.Address)
	if err != nil {
		logger.Error("ldap connect failed", "address", srv.Address, "err", err)
		return err
	}
	return ldapClient.Authenticate(srv.LdapConnectionId, userId, password)
}

// ////////////////////// RBAC helpers //////////////////////
func GetRbacClient(address string) (*rbac_client.Rbac_Client, error) {
	Utility.RegisterFunction("NewRbacService_Client", rbac_client.NewRbacService_Client)
	client, err := globular_client.GetClient(address, "rbac.RbacService", "NewRbacService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*rbac_client.Rbac_Client), nil
}
func (srv *server) addResourceOwner(path, resourceType, subject string, subjectType rbacpb.SubjectType) error {
	rbacClient, err := GetRbacClient(srv.Address)
	if err != nil {
		return err
	}
	return rbacClient.AddResourceOwner(path, resourceType, subject, subjectType)
}

// ////////////////////// Resource helpers //////////////////////
func (srv *server) getResourceClient(address string) (*resource_client.Resource_Client, error) {
	Utility.RegisterFunction("NewResourceService_Client", resource_client.NewResourceService_Client)
	client, err := globular_client.GetClient(address, "resource.ResourceService", "NewResourceService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*resource_client.Resource_Client), nil
}
func (srv *server) getSessions() ([]*resourcepb.Session, error) {
	resourceClient, err := srv.getResourceClient(srv.GetAddress())
	if err != nil {
		return nil, err
	}
	return resourceClient.GetSessions(`{"state":0}`)
}
func (srv *server) removeSession(accountId string) error {
	resourceClient, err := srv.getResourceClient(srv.GetAddress())
	if err != nil {
		return err
	}
	return resourceClient.RemoveSession(accountId)
}
func (srv *server) updateSession(session *resourcepb.Session) error {
	resourceClient, err := srv.getResourceClient(srv.GetAddress())
	if err != nil {
		return err
	}
	return resourceClient.UpdateSession(session)
}
func (srv *server) getSession(accountId string) (*resourcepb.Session, error) {
	domain := srv.GetDomain()
	if strings.Contains(accountId, "@") {
		domain = strings.Split(accountId, "@")[1]
	}

	resourceClient, err := srv.getResourceClient(domain)
	if err != nil {
		return nil, err
	}
	return resourceClient.GetSession(accountId)
}
func (srv *server) getAccount(accountId string) (*resourcepb.Account, error) {
	domain := srv.GetDomain()
	if strings.Contains(accountId, "@") {
		domain = strings.Split(accountId, "@")[1]
	}

	resourceClient, err := srv.getResourceClient(domain)
	if err != nil {
		return nil, err
	}
	return resourceClient.GetAccount(accountId)
}
func (srv *server) changeAccountPassword(accountId, token, oldPassword, newPassword string) error {
	domain := srv.GetDomain()
	if strings.Contains(accountId, "@") {
		domain = strings.Split(accountId, "@")[1]
	}

	if strings.Contains(accountId, "@") {
		accountId = strings.Split(accountId, "@")[0]
	}
	resourceClient, err := srv.getResourceClient(domain)
	if err != nil {
		return err
	}
	return resourceClient.SetAccountPassword(accountId, token, oldPassword, newPassword)
}
func (srv *server) getPeers() ([]*resourcepb.Peer, error) {
	resourceClient, err := srv.getResourceClient(srv.GetAddress())
	if err != nil {
		return nil, err
	}
	peers, err := resourceClient.GetPeers(`{}`)
	if err != nil {
		return nil, err
	}
	if len(peers) == 0 {
		return nil, errors.New("no peers found")
	}
	return peers, nil
}

// ////////////////////// Auth helpers //////////////////////
func (srv *server) validatePassword(password, hashed string) error {
	return bcrypt.CompareHashAndPassword([]byte(hashed), []byte(password))
}

func (srv *server) removeExpiredSessions() {
	ticker := time.NewTicker(time.Duration(srv.WatchSessionsDelay) * time.Second)
	go func() {
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
			case <-srv.exit_:
				logger.Info("session watcher stopped")
				return
			}
		}
	}()
}

// --- Usage text ---
func printUsage() {
	fmt.Fprintf(os.Stdout, `
Usage: %s [options] <id> [configPath]

Options:
  --describe      Print service description as JSON (no etcd/config access)
  --health        Print service health as JSON (no etcd/config access)

Arguments:
  <id>            Service instance ID
  [configPath]    Optional path to configuration file

Example:
  %s auth-1 /etc/globular/auth/config.json

`, filepath.Base(os.Args[0]), filepath.Base(os.Args[0]))
}

// main wires the Authentication service with --describe/--health shortcuts
func main() {
	// Skeleton only (no etcd access yet)
	s := new(server)
	s.Name = string(authenticationpb.File_authentication_proto.Services().Get(0).FullName())
	s.Proto = authenticationpb.File_authentication_proto.Path()
	s.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	s.Port = defaultPort
	s.Proxy = defaultProxy
	s.Protocol = "grpc"
	s.Version = "0.0.1"
	s.PublisherID = "localhost"
	s.Description = "Authentication service"
	s.Keywords = []string{"Authentication"}
	s.Repositories = []string{}
	s.Discoveries = []string{}
	s.Dependencies = []string{"event.EventService"}
	s.Permissions = []interface{}{}
	s.WatchSessionsDelay = 60
	s.SessionTimeout = 15
	s.Process = -1
	s.ProxyProcess = -1
	s.KeepAlive = true
	s.KeepUpToDate = true
	s.AllowAllOrigins = allowAllOrigins
	s.AllowedOrigins = allowedOriginsStr
	s.exit_ = make(chan bool)
	s.LdapConnectionId = ""
	s.authentications_ = []string{}

	// CLI flags BEFORE touching config
	args := os.Args[1:]
	if len(args) == 0 {
		printUsage()
		return
	}
	for _, a := range args {
		switch strings.ToLower(a) {
		case "--describe":
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
			os.Stdout.Write(b)
			os.Stdout.Write([]byte("\n"))
			return
		case "--health":
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
	}

	// Optional positional args: <id> [configPath]
	if len(args) == 1 && !strings.HasPrefix(args[0], "-") {
		s.Id = args[0]
	} else if len(args) == 2 && !strings.HasPrefix(args[0], "-") && !strings.HasPrefix(args[1], "-") {
		s.Id = args[0]
		s.ConfigPath = args[1]
	}

	// Safe to touch config now
	if d, err := config.GetDomain(); err == nil {
		s.Domain = d
	} else {
		s.Domain = "localhost"
	}
	if a, err := config.GetAddress(); err == nil && strings.TrimSpace(a) != "" {
		s.Address = a
	}

	// Register client ctor for dynamic routing
	Utility.RegisterFunction("NewAuthenticationService_Client", authentication_client.NewAuthenticationService_Client)

	start := time.Now()
	if err := s.Init(); err != nil {
		logger.Error("service init failed", "service", s.Name, "id", s.Id, "err", err)
		os.Exit(1)
	}

	// gRPC registration
	authenticationpb.RegisterAuthenticationServiceServer(s.grpcServer, s)
	reflection.Register(s.grpcServer)

	// background session janitor
	s.removeExpiredSessions()

	// Make peers able to mint JWTs (symm. keys)
	macAddress, err := config.GetMacAddress()
	if err != nil {
		logger.Error("mac get failed", "err", err)
		os.Exit(1)
	}
	if err := s.setKey(macAddress); err != nil { // setKey defined elsewhere in this package
		logger.Error("peer keys generate failed", "mac", macAddress, "err", err)
		os.Exit(1)
	}

	logger.Info("service ready",
		"service", s.Name,
		"port", s.Port,
		"proxy", s.Proxy,
		"protocol", s.Protocol,
		"domain", s.Domain,
		"listen_ms", time.Since(start).Milliseconds())

	if err := s.StartService(); err != nil {
		logger.Error("service start failed", "err", err)
		os.Exit(1)
	}

	// graceful exit
	s.exit_ <- true
}
