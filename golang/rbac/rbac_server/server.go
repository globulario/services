package main

import (
	"errors"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/config"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/interceptors"
	"github.com/globulario/services/golang/log/log_client"
	"github.com/globulario/services/golang/log/logpb"
	"github.com/globulario/services/golang/rbac/rbac_client"
	"github.com/globulario/services/golang/resource/resource_client"
	"github.com/globulario/services/golang/resource/resourcepb"
	"github.com/globulario/services/golang/storage/storage_store"
	"google.golang.org/grpc"

	//"google.golang.org/grpc/grpclog"
	"github.com/globulario/services/golang/rbac/rbacpb"
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

	domain string = "localhost"
)

// Value need by Globular to start the services...
type server struct {
	// The global attribute of the services.
	Id              string
	Mac             string
	Name            string
	Domain          string
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
	Description     string
	Keywords        []string
	Repositories    []string
	Discoveries     []string
	Process         int
	ProxyProcess    int
	ConfigPath      string
	LastError       string

	TLS bool

	// The path where the permissions data will be store.
	Root string

	// server-signed X.509 public keys for distribution
	CertFile string

	// a private RSA key to sign and authenticate the public key
	KeyFile string

	// a private RSA key to sign and authenticate the public key
	CertAuthorityTrust string

	Permissions []interface{} // contains the action permission for the services.

	Dependencies []string // The list of services needed by this services.

	// The grpc server.
	grpcServer *grpc.Server

	// RBAC store.
	permissions *storage_store.LevelDB_store
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

func (svr *server) GetMac() string {
	return svr.Mac
}

func (svr *server) SetMac(mac string) {
	svr.Mac = mac
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

func (server *server) GetPlatform() string {
	return globular.GetPlatform()
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

// Singleton.
var (
	resourceClient *resource_client.Resource_Client
	log_client_    *log_client.Log_Client
)

////////////////////////////////////////////////////////////////////////////////////////
// Logger function
////////////////////////////////////////////////////////////////////////////////////////
/**
 * Get the log client.
 */
func (server *server) GetLogClient() (*log_client.Log_Client, error) {
	var err error
	if log_client_ == nil {
		address, _:= config.GetAddress()
		log_client_, err = log_client.NewLogService_Client(address, "log.LogService")
		if err != nil {
			return nil, err
		}

	}
	return log_client_, nil
}
func (server *server) logServiceInfo(method, fileLine, functionName, infos string) {
	log_client_, err := server.GetLogClient()
	if err != nil {
		return
	}
	log_client_.Log(server.Name, server.Domain, method, logpb.LogLevel_INFO_MESSAGE, infos, fileLine, functionName)
}

func (server *server) logServiceError(method, fileLine, functionName, infos string) {
	log_client_, err := server.GetLogClient()
	if err != nil {
		return
	}
	log_client_.Log(server.Name, server.Domain, method, logpb.LogLevel_ERROR_MESSAGE, infos, fileLine, functionName)
}

////////////////////////////////////////////////////////////////////////////////////////
// Resource manager function
////////////////////////////////////////////////////////////////////////////////////////
func (server *server) getResourceClient() (*resource_client.Resource_Client, error) {

	var err error
	if resourceClient != nil {
		return resourceClient, nil
	}
	address, _ := config.GetAddress()
	resourceClient, err = resource_client.NewResourceService_Client(address, "resource.ResourceService")
	if err != nil {
		resourceClient = nil
		return nil, err
	}

	return resourceClient, nil
}

/**
 * Return an application with a given id
 */
func (server *server) getAccount(accountId string) (*resourcepb.Account, error) {
	resourceClient, err := server.getResourceClient()
	if err != nil {
		return nil, err
	}

	return resourceClient.GetAccount(accountId)
}

func (server *server) accountExist(id string) bool{
	a, err := server.getAccount(id)
	if err != nil || a == nil {
		return false
	}
	return true
}

/**
 * Return a group with a given id
 */
func (server *server) getGroup(groupId string) (*resourcepb.Group, error) {
	resourceClient, err := server.getResourceClient()
	if err != nil {
		return nil, err
	}

	groups, err := resourceClient.GetGroups(`{"$or":[{"_id":"` + groupId + `"},{"name":"` + groupId + `"} ]}`)
	if err != nil {
		return nil, err
	}

	if len(groups) == 0 {
		return nil, errors.New("no group found wiht name or _id " + groupId)
	}

	return groups[0], nil
}

/**
 * Test if a group exist.
 */
func (server *server)groupExist(id string) bool{
	g, err := server.getGroup(id)
	if err != nil || g == nil {
		return false
	}
	return true
}

/**
 * Return an application with a given id
 */
func (server *server) getApplication(applicationId string) (*resourcepb.Application, error) {
	resourceClient, err := server.getResourceClient()
	if err != nil {
		return nil, err
	}

	applications, err := resourceClient.GetApplications(`{"$or":[{"_id":"` + applicationId + `"},{"name":"` + applicationId + `"} ]}`)
	if err != nil {
		return nil, err
	}

	if len(applications) == 0 {
		return nil, errors.New("no application found wiht name or _id " + applicationId)
	}

	//str_, _ := Utility.ToJson(applications[0])
	//server.logServiceInfo("getApplication", Utility.FileLine(), Utility.FunctionName(), str_ )

	return applications[0], nil
}

/**
 * Test if a application exist.
 */
 func (server *server) applicationExist(id string) bool{
	g, err := server.getApplication(id)
	if err != nil || g == nil {
		return false
	}
	return true
}

/**
 * Return a peer with a given id
 */
func (server *server) getPeer(peerId string) (*resourcepb.Peer, error) {
	resourceClient, err := server.getResourceClient()
	if err != nil {
		return nil, err
	}

	peers, err := resourceClient.GetPeers(`{"$or":[{"domain":"` + peerId + `"},{"mac":"` + peerId + `"} ]}`)
	if err != nil {
		return nil, err
	}

	if len(peers) == 0 {
		return nil, errors.New("no peer found wiht name or _id " + peerId)
	}

	return peers[0], nil
}


/**
 * Test if a peer exist.
 */
 func (server *server) peerExist(id string) bool{
	g, err := server.getPeer(id)
	if err != nil || g == nil {
		return false
	}
	return true
}

/**
 * Return a peer with a given id
 */
 func (server *server) getOrganization(organisationId string) (*resourcepb.Organization, error) {
	resourceClient, err := server.getResourceClient()
	if err != nil {
		return nil, err
	}

	organisations, err := resourceClient.GetOrganizations(`{"$or":[{"_id":"` + organisationId + `"},{"name":"` + organisationId + `"} ]}`)
	if err != nil {
		return nil, err
	}

	if len(organisations) == 0 {
		return nil, errors.New("no organization found wiht name or _id " + organisationId)
	}

	return organisations[0], nil
}


/**
 * Test if a organisation exist.
 */
 func (server *server) organisationExist(id string) bool{
	o, err := server.getOrganization(id)
	if err != nil || o == nil {
		return false
	}
	return true
}

/**
 * Return a role with a given id
 */
func (server *server) getRole(roleId string) (*resourcepb.Role, error) {
	resourceClient, err := server.getResourceClient()
	if err != nil {
		return nil, err
	}

	roles, err := resourceClient.GetRoles(`{"$or":[{"_id":"` + roleId + `"},{"name":"` + roleId + `"} ]}`)
	if err != nil {
		return nil, err
	}

	if len(roles) == 0 {
		return nil, errors.New("no role found wiht name or _id " + roleId)
	}

	return roles[0], nil
}

/**
 * Test if a role exist.
 */
 func (server *server) roleExist(id string) bool{
	r, err := server.getRole(id)
	if err != nil || r == nil {
		return false
	}
	return true
}

////////////////////////////////////////////////////////////////////////////////////////
// RBAC specific functions
////////////////////////////////////////////////////////////////////////////////////////
func (server *server) GetPermissions() []interface{} {
	return server.Permissions
}
func (server *server) SetPermissions(permissions []interface{}) {
	server.Permissions = permissions
}

// Create the configuration file if is not already exist.
func (server *server) Init() error {

	// That function is use to get access to other server.
	Utility.RegisterFunction("NewrbacService_Client", rbac_client.NewRbacService_Client)

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

// That service is use to give access to SQL.
// port number must be pass as argument.
func main() {

	// set the logger.
	//grpclog.SetLogger(log.New(os.Stdout, "rbac_service: ", log.LstdFlags))

	// Set the log information in case of crash...
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Initialyse service with default values.
	s_impl := new(server)
	s_impl.Name = string(rbacpb.File_rbac_proto.Services().Get(0).FullName())
	s_impl.Proto = rbacpb.File_rbac_proto.Path()
	s_impl.Port = defaultPort
	s_impl.Proxy = defaultProxy
	s_impl.Protocol = "grpc"
	s_impl.Domain = domain
	s_impl.Version = "0.0.1"
	s_impl.PublisherId = "globulario"
	s_impl.Description = "The authencation server, validate user authentity"
	s_impl.Keywords = []string{"Example", "rbac", "Test", "Service"}
	s_impl.Repositories = make([]string, 0)
	s_impl.Discoveries = make([]string, 0)
	s_impl.Dependencies = []string{"resource.ResourceService"}
	s_impl.Permissions = make([]interface{}, 0)
	s_impl.Process = -1
	s_impl.ProxyProcess = -1
	s_impl.AllowAllOrigins = allow_all_origins
	s_impl.AllowedOrigins = allowed_origins
	s_impl.KeepAlive = true

	// Here I will retreive the list of connections from file if there are some...
	err := s_impl.Init()
	if err != nil {
		log.Fatalf("fail to initialyse service %s: %s", s_impl.Name, s_impl.Id)
	}
	if len(os.Args) == 2 {
		s_impl.Port, _ = strconv.Atoi(os.Args[1]) // The second argument must be the port number
	}

	// The rbac storage.
	s_impl.permissions = storage_store.NewLevelDB_store()
	err = s_impl.permissions.Open(`{"path":"` + s_impl.Root + `", "name":"permissions"}`)
	if err != nil {
		log.Println(err)
	}

	// Register the rbac services
	rbacpb.RegisterRbacServiceServer(s_impl.grpcServer, s_impl)
	reflection.Register(s_impl.grpcServer)

	// Need to be the owner in order to change permissions
	s_impl.setActionResourcesPermissions(map[string]interface{}{"action": "/rbac.RbacService/SetResourcePermissions", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "owner"}}})

	// Start the service.
	s_impl.StartService()

}
