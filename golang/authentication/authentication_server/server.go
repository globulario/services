package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/davecourtois/Utility"
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
func (svr *server) GetConfigurationPath() string {
	return svr.ConfigPath
}

func (svr *server) SetConfigurationPath(path string) {
	svr.ConfigPath = path
}

// The http address where the configuration can be found /config
func (svr *server) GetAddress() string {
	return svr.Address
}

func (svr *server) SetAddress(address string) {
	svr.Address = address
}

func (svr *server) GetProcess() int {
	return svr.Process
}

func (svr *server) SetProcess(pid int) {
	svr.Process = pid
}

func (svr *server) GetProxyProcess() int {
	return svr.ProxyProcess
}

func (svr *server) SetProxyProcess(pid int) {
	svr.ProxyProcess = pid
}

// The current service state
func (svr *server) GetState() string {
	return svr.State
}

func (svr *server) SetState(state string) {
	svr.State = state
}

// The last error
func (svr *server) GetLastError() string {
	return svr.LastError
}

func (svr *server) SetLastError(err string) {
	svr.LastError = err
}

// The modeTime
func (svr *server) SetModTime(modtime int64) {
	svr.ModTime = modtime
}
func (svr *server) GetModTime() int64 {
	return svr.ModTime
}

// Globular services implementation...
// The id of a particular service instance.
func (server *server) GetId() string {
	return server.Id
}
func (server *server) SetId(id string) {
	server.Id = id
}

// The name of a service, must be the gRpc Service name.
func (server *server) GetName() string {
	return server.Name
}
func (server *server) SetName(name string) {
	server.Name = name
}

func (server *server) GetMac() string {
	return server.Mac
}

func (svr *server) SetMac(mac string) {
	svr.Mac = mac
}

func (svr *server) GetChecksum() string {

	return svr.Checksum
}

func (svr *server) SetChecksum(checksum string) {
	svr.Checksum = checksum
}

func (svr *server) GetPlatform() string {
	return svr.Plaform
}

func (svr *server) SetPlatform(platform string) {
	svr.Plaform = platform
}

// The description of the service
func (server *server) GetDescription() string {
	return server.Description
}
func (server *server) SetDescription(description string) {
	server.Description = description
}

// The list of keywords of the services.
func (server *server) GetKeywords() []string {
	return server.Keywords
}
func (server *server) SetKeywords(keywords []string) {
	server.Keywords = keywords
}

// The path of the .proto file.
func (svr *server) GetConfigPort() int {
	return svr.ConfigPort
}

func (svr *server) SetConfigPort(port int) {
	svr.ConfigPort = port
}

// Return the address where the configuration can be found...
func (svr *server) GetConfigAddress() string {
	return svr.GetDomain() + ":" + Utility.ToString(svr.ConfigPort)
}

func (server *server) GetRepositories() []string {
	return server.Repositories
}
func (server *server) SetRepositories(repositories []string) {
	server.Repositories = repositories
}

func (server *server) GetDiscoveries() []string {
	return server.Discoveries
}
func (server *server) SetDiscoveries(discoveries []string) {
	server.Discoveries = discoveries
}

// Dist
func (server *server) Dist(path string) (string, error) {

	return globular.Dist(path, server)
}

func (server *server) GetDependencies() []string {

	if server.Dependencies == nil {
		server.Dependencies = make([]string, 0)
	}

	return server.Dependencies
}

func (server *server) SetDependency(dependency string) {
	if server.Dependencies == nil {
		server.Dependencies = make([]string, 0)
	}

	// Append the depency to the list.
	if !Utility.Contains(server.Dependencies, dependency) {
		server.Dependencies = append(server.Dependencies, dependency)
	}
}

// The path of the executable.
func (server *server) GetPath() string {
	return server.Path
}
func (server *server) SetPath(path string) {
	server.Path = path
}

// The path of the .proto file.
func (server *server) GetProto() string {
	return server.Proto
}
func (server *server) SetProto(proto string) {
	server.Proto = proto
}

// The gRpc port.
func (server *server) GetPort() int {
	return server.Port
}
func (server *server) SetPort(port int) {
	server.Port = port
}

// The reverse proxy port (use by gRpc Web)
func (server *server) GetProxy() int {
	return server.Proxy
}
func (server *server) SetProxy(proxy int) {
	server.Proxy = proxy
}

// Can be one of http/https/tls
func (server *server) GetProtocol() string {
	return server.Protocol
}
func (server *server) SetProtocol(protocol string) {
	server.Protocol = protocol
}

// Return true if all Origins are allowed to access the mircoservice.
func (server *server) GetAllowAllOrigins() bool {
	return server.AllowAllOrigins
}
func (server *server) SetAllowAllOrigins(allowAllOrigins bool) {
	server.AllowAllOrigins = allowAllOrigins
}

// If AllowAllOrigins is false then AllowedOrigins will contain the
// list of address that can reach the services.
func (server *server) GetAllowedOrigins() string {
	return server.AllowedOrigins
}

func (server *server) SetAllowedOrigins(allowedOrigins string) {
	server.AllowedOrigins = allowedOrigins
}

// Can be a ip address or domain name.
func (server *server) GetDomain() string {
	return server.Domain
}
func (server *server) SetDomain(domain string) {
	server.Domain = domain
}

// TLS section

// If true the service run with TLS. The
func (server *server) GetTls() bool {
	return server.TLS
}
func (server *server) SetTls(hasTls bool) {
	server.TLS = hasTls
}

// The certificate authority file
func (server *server) GetCertAuthorityTrust() string {
	return server.CertAuthorityTrust
}
func (server *server) SetCertAuthorityTrust(ca string) {
	server.CertAuthorityTrust = ca
}

// The certificate file.
func (server *server) GetCertFile() string {
	return server.CertFile
}
func (server *server) SetCertFile(certFile string) {
	server.CertFile = certFile
}

// The key file.
func (server *server) GetKeyFile() string {
	return server.KeyFile
}
func (server *server) SetKeyFile(keyFile string) {
	server.KeyFile = keyFile
}

// The service version
func (server *server) GetVersion() string {
	return server.Version
}
func (server *server) SetVersion(version string) {
	server.Version = version
}

// The publisher id.
func (server *server) GetPublisherId() string {
	return server.PublisherId
}
func (server *server) SetPublisherId(publisherId string) {
	server.PublisherId = publisherId
}

func (server *server) GetKeepUpToDate() bool {
	return server.KeepUpToDate
}
func (server *server) SetKeepUptoDate(val bool) {
	server.KeepUpToDate = val
}

func (server *server) GetKeepAlive() bool {
	return server.KeepAlive
}
func (server *server) SetKeepAlive(val bool) {
	server.KeepAlive = val
}

func (server *server) GetPermissions() []interface{} {
	return server.Permissions
}
func (server *server) SetPermissions(permissions []interface{}) {
	server.Permissions = permissions
}

// Create the configuration file if is not already exist.
func (server *server) Init() error {

	// Get the configuration path.
	err := globular.InitService(server)
	if err != nil {
		return err
	}

	// Initialyse GRPC server.
	server.grpcServer, err = globular.InitGrpcServer(server, interceptors.ServerUnaryInterceptor, interceptors.ServerStreamInterceptor)
	if err != nil {
		return err
	}

	return nil

}

// Save the configuration values.
func (server *server) Save() error {
	// Create the file...
	return globular.SaveService(server)
}

func (server *server) StartService() error {
	return globular.StartService(server, server.grpcServer)
}

func (server *server) StopService() error {
	return globular.StopService(server, server.grpcServer)
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

// Authenticate user with LDAP server.
func (svr *server) authenticateLdap(userId string, password string) error {
	ldap_client_, err := GetLdapClient(svr.Address)
	if err != nil {
		fmt.Println("fail to connect to ldap service with error: ", err)
		return err
	}

	// Return autentication result.
	return ldap_client_.Authenticate(svr.LdapConnectionId, userId, password)
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

func (svr *server) addResourceOwner(path, resourceType, subject string, subjectType rbacpb.SubjectType) error {
	rbac_client_, err := GetRbacClient(svr.Address)
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
func (server *server) GetLogClient() (*log_client.Log_Client, error) {
	Utility.RegisterFunction("NewLogService_Client", log_client.NewLogService_Client)
	client, err := globular_client.GetClient(server.Address, "log.LogService", "NewLogService_Client")
	if err != nil {
		return nil, err
	}

	return client.(*log_client.Log_Client), nil
}
func (server *server) logServiceInfo(method, fileLine, functionName, infos string) error {
	log_client_, err := server.GetLogClient()
	if err != nil {
		return err
	}
	return log_client_.Log(server.Name, server.Domain, method, logpb.LogLevel_INFO_MESSAGE, infos, fileLine, functionName)
}

func (server *server) logServiceError(method, fileLine, functionName, infos string) error {
	log_client_, err := server.GetLogClient()
	if err != nil {
		return err
	}
	return log_client_.Log(server.Name, server.Address, method, logpb.LogLevel_ERROR_MESSAGE, infos, fileLine, functionName)
}

// /////////////////// event service functions ////////////////////////////////////
func (svr *server) getEventClient() (*event_client.Event_Client, error) {
	Utility.RegisterFunction("NewEventService_Client", event_client.NewEventService_Client)
	client, err := globular_client.GetClient(svr.Address, "event.EventService", "NewEventService_Client")

	if err != nil {
		return nil, err
	}

	return client.(*event_client.Event_Client), nil
}

func (svr *server) publish(event string, data []byte) error {
	eventClient, err := svr.getEventClient()
	if err != nil {
		return err
	}
	return eventClient.Publish(event, data)
}

// /////////////////// resource service functions ////////////////////////////////////
func (svr *server) getResourceClient(address string) (*resource_client.Resource_Client, error) {
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
func (svr *server) getSessions() ([]*resourcepb.Session, error) {
	resourceClient, err := svr.getResourceClient(svr.GetDomain())
	if err != nil {
		return nil, err
	}

	return resourceClient.GetSessions(`{"state":0}`)
}

/** Now yet use **/
func (svr *server) removeSession(accountId string) error {
	resourceClient, err := svr.getResourceClient(svr.GetDomain())
	if err != nil {
		return err
	}

	return resourceClient.RemoveSession(accountId)
}

func (svr *server) updateSession(session *resourcepb.Session) error {
	resourceClient, err := svr.getResourceClient(svr.GetDomain())
	if err != nil {
		return err
	}

	return resourceClient.UpdateSession(session)
}

func (svr *server) getSession(accountId string) (*resourcepb.Session, error) {
	resourceClient, err := svr.getResourceClient(svr.GetDomain())
	if err != nil {
		return nil, err
	}

	return resourceClient.GetSession(accountId)
}

/**
 * Retreive an account with a given id.
 */
func (svr *server) getAccount(accountId string) (*resourcepb.Account, error) {
	resourceClient, err := svr.getResourceClient(svr.GetDomain())
	if err != nil {
		return nil, err
	}

	return resourceClient.GetAccount(accountId)
}

func (svr *server) changeAccountPassword(accountId, token, oldPassword, newPassword string) error {
	// take the first part of the account.
	if strings.Contains(accountId, "@") {
		accountId = strings.Split(accountId, "@")[0]
	}

	resourceClient, err := svr.getResourceClient(svr.Address)
	if err != nil {
		return err
	}
	return resourceClient.SetAccountPassword(accountId, token, oldPassword, newPassword)
}

/**
 * Return a peer with a given id
 */
func (svr *server) getPeers() ([]*resourcepb.Peer, error) {
	resourceClient, err := svr.getResourceClient(svr.Address)
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
func (server *server) validatePassword(password string, hashedPassword string) error {

	return bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
}

/**
 * Return the hash value of a given password.
 */
func (server *server) hashPassword(password string) (string, error) {
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
func (server *server) removeExpiredSessions() {
	ticker := time.NewTicker(time.Duration(server.WatchSessionsDelay) * time.Second)
	go func() {
		for {
			select {
			case <-ticker.C:
				// Connect to service update events...
				// I will iterate over the list token and close expired session...
				sessions, err := server.getSessions()
				if err == nil {
					for i := 0; i < len(sessions); i++ {
						session := sessions[i]
						if session.ExpireAt < time.Now().Unix() {
							session.State = 1
							server.updateSession(session)
						}
					}
				}
			case <-server.exit_:
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
	s_impl.PublisherId = "globulario@globule-dell.globular.cloud"
	s_impl.Description = "Authentication service"
	s_impl.Keywords = []string{"Authentication"}
	s_impl.Repositories = make([]string, 0)
	s_impl.Discoveries = make([]string, 0)
	s_impl.Dependencies = []string{"event.EventService", "resource.ResourceService"}
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

	// Register the echo services
	authenticationpb.RegisterAuthenticationServiceServer(s_impl.grpcServer, s_impl)
	reflection.Register(s_impl.grpcServer)

	s_impl.removeExpiredSessions()

	// That function will set the value to be eable to create symetric encryption keys.
	// So peer will be able to generate valid jwt token by themself.
	macAddress, err := Utility.MyMacAddr(Utility.MyLocalIP())
	if err != nil {
		log.Fatalf("fail to get the mac address %s", err.Error())
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
