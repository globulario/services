package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/globulario/services/golang/authentication/authentication_client"
	"github.com/globulario/services/golang/authentication/authenticationpb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/event/event_client"
	"github.com/globulario/services/golang/globular_client"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/interceptors"
	"github.com/globulario/services/golang/ldap/ldap_client"
	"github.com/globulario/services/golang/log/log_client"
	"github.com/globulario/services/golang/log/logpb"
	"github.com/globulario/services/golang/rbac/rbac_client"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/resource/resource_client"
	"github.com/globulario/services/golang/resource/resourcepb"
	Utility "github.com/globulario/utility"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"

	//"google.golang.org/grpc/grpclog"
	// "errors"
	"time"

	"google.golang.org/grpc/reflection"
)

// The default values.
var (
	defaultPort  = 10029
	defaultProxy = 10030

	// By default all origins are allowed.
	allow_all_origins = true

	// comma separeated values.
	allowed_origins string = ""
)

// Value need by Globular to start the services...
type server struct {
	// The global attribute of the services.
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
	AllowedOrigins  string // comma separated string.
	Protocol        string
	Version         string
	PublisherId     string
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

	// server-signed X.509 public keys for distribution
	CertFile string

	// a private RSA key to sign and authenticate the public key
	KeyFile string

	// a private RSA key to sign and authenticate the public key
	CertAuthorityTrust string

	Permissions []interface{} // contains the action permission for the services.

	Dependencies []string // The list of services needed by this services.

	WatchSessionsDelay int // The time in second to refresh sessions...

	SessionTimeout int // The time before session expire.

	LdapConnectionId string // If define the authentication will be validate by LDAP...

	exit_ chan bool

	// The grpc server.
	grpcServer *grpc.Server

	// use to cut infinite recursion.
	authentications_ []string
}

// The path of the configuration.
func (srv *server) GetConfigurationPath() string {
	return srv.ConfigPath
}

func (srv *server) SetConfigurationPath(path string) {
	srv.ConfigPath = path
}

// The http address where the configuration can be found /config
func (srv *server) GetAddress() string {
	return srv.Address
}

func (srv *server) SetAddress(address string) {
	srv.Address = address
}

func (srv *server) GetProcess() int {
	return srv.Process
}

func (srv *server) SetProcess(pid int) {
	srv.Process = pid
}

func (srv *server) GetProxyProcess() int {
	return srv.ProxyProcess
}

func (srv *server) SetProxyProcess(pid int) {
	srv.ProxyProcess = pid
}

// The current service state
func (srv *server) GetState() string {
	return srv.State
}

func (srv *server) SetState(state string) {
	srv.State = state
}

// The last error
func (srv *server) GetLastError() string {
	return srv.LastError
}

func (srv *server) SetLastError(err string) {
	srv.LastError = err
}

// The modeTime
func (srv *server) SetModTime(modtime int64) {
	srv.ModTime = modtime
}
func (srv *server) GetModTime() int64 {
	return srv.ModTime
}

// Globular services implementation...
// The id of a particular service instance.
func (srv *server) GetId() string {
	return srv.Id
}
func (srv *server) SetId(id string) {
	srv.Id = id
}

// The name of a service, must be the gRpc Service name.
func (srv *server) GetName() string {
	return srv.Name
}
func (srv *server) SetName(name string) {
	srv.Name = name
}

func (srv *server) GetMac() string {
	return srv.Mac
}

func (srv *server) SetMac(mac string) {
	srv.Mac = mac
}

func (srv *server) GetChecksum() string {

	return srv.Checksum
}

func (srv *server) SetChecksum(checksum string) {
	srv.Checksum = checksum
}

func (srv *server) GetPlatform() string {
	return srv.Plaform
}

func (srv *server) SetPlatform(platform string) {
	srv.Plaform = platform
}

// The description of the service
func (srv *server) GetDescription() string {
	return srv.Description
}
func (srv *server) SetDescription(description string) {
	srv.Description = description
}

// The list of keywords of the services.
func (srv *server) GetKeywords() []string {
	return srv.Keywords
}
func (srv *server) SetKeywords(keywords []string) {
	srv.Keywords = keywords
}

// The path of the .proto file.
func (srv *server) GetConfigPort() int {
	return srv.ConfigPort
}

func (srv *server) SetConfigPort(port int) {
	srv.ConfigPort = port
}

// Return the address where the configuration can be found...
func (srv *server) GetConfigAddress() string {
	domain := srv.GetAddress()
	if strings.Contains(domain, ":") {
		domain = strings.Split(domain, ":")[0]
	}
	return domain + ":" + Utility.ToString(srv.ConfigPort)
}

func (srv *server) GetRepositories() []string {
	return srv.Repositories
}
func (srv *server) SetRepositories(repositories []string) {
	srv.Repositories = repositories
}

func (srv *server) GetDiscoveries() []string {
	return srv.Discoveries
}
func (srv *server) SetDiscoveries(discoveries []string) {
	srv.Discoveries = discoveries
}

// Dist
func (srv *server) Dist(path string) (string, error) {

	return globular.Dist(path, srv)
}

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

	// Append the depency to the list.
	if !Utility.Contains(srv.Dependencies, dependency) {
		srv.Dependencies = append(srv.Dependencies, dependency)
	}
}

// The path of the executable.
func (srv *server) GetPath() string {
	return srv.Path
}
func (srv *server) SetPath(path string) {
	srv.Path = path
}

// The path of the .proto file.
func (srv *server) GetProto() string {
	return srv.Proto
}
func (srv *server) SetProto(proto string) {
	srv.Proto = proto
}

// The gRpc port.
func (srv *server) GetPort() int {
	return srv.Port
}
func (srv *server) SetPort(port int) {
	srv.Port = port
}

// The reverse proxy port (use by gRpc Web)
func (srv *server) GetProxy() int {
	return srv.Proxy
}
func (srv *server) SetProxy(proxy int) {
	srv.Proxy = proxy
}

// Can be one of http/https/tls
func (srv *server) GetProtocol() string {
	return srv.Protocol
}
func (srv *server) SetProtocol(protocol string) {
	srv.Protocol = protocol
}

// Return true if all Origins are allowed to access the mircoservice.
func (srv *server) GetAllowAllOrigins() bool {
	return srv.AllowAllOrigins
}
func (srv *server) SetAllowAllOrigins(allowAllOrigins bool) {
	srv.AllowAllOrigins = allowAllOrigins
}

// If AllowAllOrigins is false then AllowedOrigins will contain the
// list of address that can reach the services.
func (srv *server) GetAllowedOrigins() string {
	return srv.AllowedOrigins
}

func (srv *server) SetAllowedOrigins(allowedOrigins string) {
	srv.AllowedOrigins = allowedOrigins
}

// Can be a ip address or domain name.
func (srv *server) GetDomain() string {
	return srv.Domain
}
func (srv *server) SetDomain(domain string) {
	srv.Domain = domain
}

// TLS section

// If true the service run with TLS. The
func (srv *server) GetTls() bool {
	return srv.TLS
}
func (srv *server) SetTls(hasTls bool) {
	srv.TLS = hasTls
}

// The certificate authority file
func (srv *server) GetCertAuthorityTrust() string {
	return srv.CertAuthorityTrust
}
func (srv *server) SetCertAuthorityTrust(ca string) {
	srv.CertAuthorityTrust = ca
}

// The certificate file.
func (srv *server) GetCertFile() string {
	return srv.CertFile
}
func (srv *server) SetCertFile(certFile string) {
	srv.CertFile = certFile
}

// The key file.
func (srv *server) GetKeyFile() string {
	return srv.KeyFile
}
func (srv *server) SetKeyFile(keyFile string) {
	srv.KeyFile = keyFile
}

// The service version
func (srv *server) GetVersion() string {
	return srv.Version
}
func (srv *server) SetVersion(version string) {
	srv.Version = version
}

// The publisher id.
func (srv *server) GetPublisherId() string {
	return srv.PublisherId
}
func (srv *server) SetPublisherId(publisherId string) {
	srv.PublisherId = publisherId
}

func (srv *server) GetKeepUpToDate() bool {
	return srv.KeepUpToDate
}
func (srv *server) SetKeepUptoDate(val bool) {
	srv.KeepUpToDate = val
}

func (srv *server) GetKeepAlive() bool {
	return srv.KeepAlive
}
func (srv *server) SetKeepAlive(val bool) {
	srv.KeepAlive = val
}

func (srv *server) GetPermissions() []interface{} {
	return srv.Permissions
}
func (srv *server) SetPermissions(permissions []interface{}) {
	srv.Permissions = permissions
}

// Create the configuration file if is not already exist.
func (srv *server) Init() error {

	// Get the configuration path.
	err := globular.InitService(srv)
	if err != nil {
		return err
	}

	// Initialyse GRPC srv.
	srv.grpcServer, err = globular.InitGrpcServer(srv, interceptors.ServerUnaryInterceptor, interceptors.ServerStreamInterceptor)
	if err != nil {
		return err
	}

	return nil

}

// Save the configuration values.
func (srv *server) Save() error {
	// Create the file...
	return globular.SaveService(srv)
}

func (srv *server) StartService() error {
	return globular.StartService(srv, srv.grpcServer)
}

func (srv *server) StopService() error {
	return globular.StopService(srv, srv.grpcServer)
}

///////////////////////  LDAP Services functions ////////////////////////////////////////////////
/**
 * Get the rbac client.
 */
func GetLdapClient(address string) (*ldap_client.LDAP_Client, error) {

	client, err := globular_client.GetClient(address, "ldap.LdapService", "ldap_client.NewLdapService_Client")
	if err != nil {
		return nil, err
	}

	return client.(*ldap_client.LDAP_Client), nil
}

// Authenticate user with LDAP srv.
func (srv *server) authenticateLdap(userId string, password string) error {
	ldap_client_, err := GetLdapClient(srv.Address)
	if err != nil {
		fmt.Println("fail to connect to ldap service with error: ", err)
		return err
	}

	// Return autentication result.
	return ldap_client_.Authenticate(srv.LdapConnectionId, userId, password)
}

///////////////////////  RBAC Services functions ////////////////////////////////////////////////
/**
 * Get the rbac client.
 */
func GetRbacClient(address string) (*rbac_client.Rbac_Client, error) {
	Utility.RegisterFunction("NewRbacService_Client", rbac_client.NewRbacService_Client)
	client, err := globular_client.GetClient(address, "rbac.RbacService", "NewRbacService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*rbac_client.Rbac_Client), nil
}

func (srv *server) addResourceOwner(path, resourceType, subject string, subjectType rbacpb.SubjectType) error {
	rbac_client_, err := GetRbacClient(srv.Address)
	if err != nil {
		return err
	}

	err = rbac_client_.AddResourceOwner(path, resourceType, subject, subjectType)
	return err
}

///////////////////////  Log Services functions ////////////////////////////////////////////////

/**
 * Get the log client.
 */
func (srv *server) GetLogClient() (*log_client.Log_Client, error) {
	Utility.RegisterFunction("NewLogService_Client", log_client.NewLogService_Client)
	client, err := globular_client.GetClient(srv.Address, "log.LogService", "NewLogService_Client")
	if err != nil {
		return nil, err
	}

	return client.(*log_client.Log_Client), nil
}
func (srv *server) logServiceInfo(method, fileLine, functionName, infos string) error {
	log_client_, err := srv.GetLogClient()
	if err != nil {
		return err
	}
	return log_client_.Log(srv.Name, srv.Domain, method, logpb.LogLevel_INFO_MESSAGE, infos, fileLine, functionName)
}

func (srv *server) logServiceError(method, fileLine, functionName, infos string) error {
	log_client_, err := srv.GetLogClient()
	if err != nil {
		return err
	}
	return log_client_.Log(srv.Name, srv.Address, method, logpb.LogLevel_ERROR_MESSAGE, infos, fileLine, functionName)
}

// /////////////////// event service functions ////////////////////////////////////
func (srv *server) getEventClient() (*event_client.Event_Client, error) {
	Utility.RegisterFunction("NewEventService_Client", event_client.NewEventService_Client)
	client, err := globular_client.GetClient(srv.Address, "event.EventService", "NewEventService_Client")

	if err != nil {
		return nil, err
	}

	return client.(*event_client.Event_Client), nil
}

func (srv *server) publish(event string, data []byte) error {
	eventClient, err := srv.getEventClient()
	if err != nil {
		return err
	}
	return eventClient.Publish(event, data)
}

// /////////////////// resource service functions ////////////////////////////////////
func (srv *server) getResourceClient(address string) (*resource_client.Resource_Client, error) {
	Utility.RegisterFunction("NewResourceService_Client", resource_client.NewResourceService_Client)
	client, err := globular_client.GetClient(address, "resource.ResourceService", "NewResourceService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*resource_client.Resource_Client), nil
}

/**
 * Get actives sessions
 */
func (srv *server) getSessions() ([]*resourcepb.Session, error) {
	resourceClient, err := srv.getResourceClient(srv.GetAddress())
	if err != nil {
		return nil, err
	}

	return resourceClient.GetSessions(`{"state":0}`)
}

/** Now yet use **/
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

	// toto fix that...
	domain = srv.GetAddress()

	resourceClient, err := srv.getResourceClient(domain)
	if err != nil {
		return nil, err
	}

	return resourceClient.GetSession(accountId)
}

/**
 * Retreive an account with a given id.
 */
func (srv *server) getAccount(accountId string) (*resourcepb.Account, error) {
	domain := srv.GetDomain()
	if strings.Contains(accountId, "@") {
		domain = strings.Split(accountId, "@")[1]
	}

	// toto fix that...
	domain = srv.GetAddress()

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

	// toto fix that...
	domain = srv.GetAddress()

	// take the first part of the account.
	if strings.Contains(accountId, "@") {
		accountId = strings.Split(accountId, "@")[0]
	}

	resourceClient, err := srv.getResourceClient(domain)
	if err != nil {
		return err
	}
	return resourceClient.SetAccountPassword(accountId, token, oldPassword, newPassword)
}

/**
 * Return a peer with a given id
 */
func (srv *server) getPeers() ([]*resourcepb.Peer, error) {
	resourceClient, err := srv.getResourceClient(srv.GetAddress())
	if err != nil {
		return nil, err
	}
	// return all peers register on a globule...
	peers, err := resourceClient.GetPeers(`{}`)
	if err != nil {
		return nil, err
	}

	if len(peers) == 0 {
		return nil, errors.New("no peers found")
	}

	return peers, nil
}

///////////////////////////////////// Authentication specific services ///////////////////////////////////////

/**
 * validate the user password.
 */
func (srv *server) validatePassword(password string, hashedPassword string) error {

	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}

/**
 * Return the hash value of a given password.
 */
func (srv *server) hashPassword(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

	if err != nil {
		return "", err
	}

	return string(hash), nil
}

/**
 * Invalidate expired session token...
 * TODO remove sessions older than a week...
 */
func (srv *server) removeExpiredSessions() {
	ticker := time.NewTicker(time.Duration(srv.WatchSessionsDelay) * time.Second)
	go func() {
		for {
			select {
			case <-ticker.C:
				// Connect to service update events...
				// I will iterate over the list token and close expired session...
				sessions, err := srv.getSessions()
				if err == nil {
					for i := 0; i < len(sessions); i++ {
						session := sessions[i]
						if session.ExpireAt < time.Now().Unix() {
							session.State = 1
							srv.updateSession(session)
						}
					}
				}
			case <-srv.exit_:
				return // exit from the loop when the service exit.
			}
		}
	}()
}

// That service is use to give access to SQL.
// port number must be pass as argument.
func main() {

	// set the logger.
	//grpclog.SetLogger(log.New(os.Stdout, "echo_service: ", log.LstdFlags))

	// Set the log information in case of crash...
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Initialyse service with default values.
	s_impl := new(server)
	s_impl.Name = string(authenticationpb.File_authentication_proto.Services().Get(0).FullName())
	s_impl.Proto = authenticationpb.File_authentication_proto.Path()
	s_impl.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	s_impl.Port = defaultPort
	s_impl.Proxy = defaultProxy
	s_impl.Protocol = "grpc"
	s_impl.Domain, _ = config.GetDomain()
	s_impl.Address, _ = config.GetAddress()
	s_impl.Version = "0.0.1"
	s_impl.PublisherId = "localhost"
	s_impl.Description = "Authentication service"
	s_impl.Keywords = []string{"Authentication"}
	s_impl.Repositories = make([]string, 0)
	s_impl.Discoveries = make([]string, 0)
	s_impl.Dependencies = []string{"event.EventService"}
	s_impl.Permissions = make([]interface{}, 0)
	s_impl.WatchSessionsDelay = 60
	s_impl.SessionTimeout = 15
	s_impl.Process = -1
	s_impl.ProxyProcess = -1
	s_impl.KeepAlive = true
	s_impl.KeepUpToDate = true
	s_impl.AllowAllOrigins = allow_all_origins
	s_impl.AllowedOrigins = allowed_origins
	s_impl.exit_ = make(chan bool)
	s_impl.LdapConnectionId = ""
	s_impl.authentications_ = make([]string, 0)

	// Register the client function, so it can be use for dynamic routing, (ex: ["GetFile", "round-robin"])
	Utility.RegisterFunction("NewAuthenticationService_Client", authentication_client.NewAuthenticationService_Client)

	// Give base info to retreive it configuration.
	if len(os.Args) == 2 {
		s_impl.Id = os.Args[1] // The second argument must be the port number
	} else if len(os.Args) == 3 {
		s_impl.Id = os.Args[1]         // The second argument must be the port number
		s_impl.ConfigPath = os.Args[2] // The second argument must be the port number
	}

	// Here I will retreive the list of connections from file if there are some...
	err := s_impl.Init()
	if err != nil {
		log.Fatalf("fail to initialyse service %s: %s", s_impl.Name, s_impl.Id)
	}

	if s_impl.Address == "" {
		s_impl.Address, _ = config.GetAddress()
	}

	// Register the echo services
	authenticationpb.RegisterAuthenticationServiceServer(s_impl.grpcServer, s_impl)
	reflection.Register(s_impl.grpcServer)

	s_impl.removeExpiredSessions()

	// That function will set the value to be eable to create symetric encryption keys.
	// So peer will be able to generate valid jwt token by themself.
	macAddress, err := config.GetMacAddress()
	if err != nil {
		log.Fatalln(err)
	}

	err = s_impl.setKey(macAddress)
	if err != nil {
		log.Fatalln(err)
	}

	// Start the service.
	s_impl.StartService()

	// Exit loop...
	s_impl.exit_ <- true
}
