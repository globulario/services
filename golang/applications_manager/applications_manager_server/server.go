package main

import (
	"log"
	"os"
	"strconv"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/applications_manager/applications_manager_client"
	"github.com/globulario/services/golang/applications_manager/applications_managerpb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/discovery/discovery_client"
	"github.com/globulario/services/golang/event/event_client"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/interceptors"
	"github.com/globulario/services/golang/log/log_client"
	"github.com/globulario/services/golang/log/logpb"
	"github.com/globulario/services/golang/rbac/rbac_client"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/resource/resource_client"
	"github.com/globulario/services/golang/resource/resourcepb"
	"google.golang.org/grpc"

	//"google.golang.org/grpc/grpclog"
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

	rbac_client_  *rbac_client.Rbac_Client
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
	Process	int
	ProxyProcess    int
	ConfigPath string
	LastError string
	ModTime 		int64

	TLS bool

	// svr-signed X.509 public keys for distribution
	CertFile string

	// a private RSA key to sign and authenticate the public key
	KeyFile string

	// a private RSA key to sign and authenticate the public key
	CertAuthorityTrust string

	Permissions []interface{} // contains the action permission for the services.

	Dependencies []string // The list of services needed by this services.

	// The grpc server.
	grpcServer *grpc.Server

	// The webroot
	WebRoot string
}

// The path of the configuration.
func (svr *server) GetConfigurationPath() string {
	return svr.ConfigPath
}

func (svr *server) SetServiceConfiguration(path string) {
	svr.ConfigPath = path
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
func (svr *server) GetId() string {
	return svr.Id
}
func (svr *server) SetId(id string) {
	svr.Id = id
}

// The name of a service, must be the gRpc Service name.
func (svr *server) GetName() string {
	return svr.Name
}
func (svr *server) SetName(name string) {
	svr.Name = name
}

func (svr *server) GetMac() string {
	return svr.Mac
}

func (svr *server) SetMac(mac string) {
	svr.Mac = mac
}

// The description of the service
func (svr *server) GetDescription() string {
	return svr.Description
}
func (svr *server) SetDescription(description string) {
	svr.Description = description
}

// The list of keywords of the services.
func (svr *server) GetKeywords() []string {
	return svr.Keywords
}
func (svr *server) SetKeywords(keywords []string) {
	svr.Keywords = keywords
}

func (svr *server) GetRepositories() []string {
	return svr.Repositories
}
func (svr *server) SetRepositories(repositories []string) {
	svr.Repositories = repositories
}

func (svr *server) GetDiscoveries() []string {
	return svr.Discoveries
}
func (svr *server) SetDiscoveries(discoveries []string) {
	svr.Discoveries = discoveries
}

// Dist
func (svr *server) Dist(path string) (string, error) {

	return globular.Dist(path, svr)
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

func (svr *server) GetPlatform() string {
	return globular.GetPlatform()
}

// The path of the executable.
func (svr *server) GetPath() string {
	return svr.Path
}
func (svr *server) SetPath(path string) {
	svr.Path = path
}

// The path of the .proto file.
func (svr *server) GetProto() string {
	return svr.Proto
}
func (svr *server) SetProto(proto string) {
	svr.Proto = proto
}

// The gRpc port.
func (svr *server) GetPort() int {
	return svr.Port
}
func (svr *server) SetPort(port int) {
	svr.Port = port
}

// The reverse proxy port (use by gRpc Web)
func (svr *server) GetProxy() int {
	return svr.Proxy
}
func (svr *server) SetProxy(proxy int) {
	svr.Proxy = proxy
}

// Can be one of http/https/tls
func (svr *server) GetProtocol() string {
	return svr.Protocol
}
func (svr *server) SetProtocol(protocol string) {
	svr.Protocol = protocol
}

// Return true if all Origins are allowed to access the mircoservice.
func (svr *server) GetAllowAllOrigins() bool {
	return svr.AllowAllOrigins
}
func (svr *server) SetAllowAllOrigins(allowAllOrigins bool) {
	svr.AllowAllOrigins = allowAllOrigins
}

// If AllowAllOrigins is false then AllowedOrigins will contain the
// list of address that can reach the services.
func (svr *server) GetAllowedOrigins() string {
	return svr.AllowedOrigins
}

func (svr *server) SetAllowedOrigins(allowedOrigins string) {
	svr.AllowedOrigins = allowedOrigins
}

// Can be a ip address or domain name.
func (svr *server) GetDomain() string {
	return svr.Domain
}
func (svr *server) SetDomain(domain string) {
	svr.Domain = domain
}

// TLS section

// If true the service run with TLS. The
func (svr *server) GetTls() bool {
	return svr.TLS
}
func (svr *server) SetTls(hasTls bool) {
	svr.TLS = hasTls
}

// The certificate authority file
func (svr *server) GetCertAuthorityTrust() string {
	return svr.CertAuthorityTrust
}
func (svr *server) SetCertAuthorityTrust(ca string) {
	svr.CertAuthorityTrust = ca
}

// The certificate file.
func (svr *server) GetCertFile() string {
	return svr.CertFile
}
func (svr *server) SetCertFile(certFile string) {
	svr.CertFile = certFile
}

// The key file.
func (svr *server) GetKeyFile() string {
	return svr.KeyFile
}
func (svr *server) SetKeyFile(keyFile string) {
	svr.KeyFile = keyFile
}

// The service version
func (svr *server) GetVersion() string {
	return svr.Version
}
func (svr *server) SetVersion(version string) {
	svr.Version = version
}

// The publisher id.
func (svr *server) GetPublisherId() string {
	return svr.PublisherId
}
func (svr *server) SetPublisherId(publisherId string) {
	svr.PublisherId = publisherId
}

func (svr *server) GetKeepUpToDate() bool {
	return svr.KeepUpToDate
}
func (svr *server) SetKeepUptoDate(val bool) {
	svr.KeepUpToDate = val
}

func (svr *server) GetKeepAlive() bool {
	return svr.KeepAlive
}
func (svr *server) SetKeepAlive(val bool) {
	svr.KeepAlive = val
}

func (svr *server) GetPermissions() []interface{} {
	return svr.Permissions
}
func (svr *server) SetPermissions(permissions []interface{}) {
	svr.Permissions = permissions
}

// Create the configuration file if is not already exist.
func (svr *server) Init() error {

	// That function is use to get access to other server.
	Utility.RegisterFunction("NewApplicationsManager_Client", applications_manager_client.NewApplicationsManager_Client)

	err := globular.InitService(svr)
	if err != nil {
		return err
	}

	// Initialyse GRPC server.
	svr.grpcServer, err = globular.InitGrpcServer(svr, interceptors.ServerUnaryInterceptor, interceptors.ServerStreamInterceptor)
	if err != nil {
		return err
	}

	return nil

}

// Save the configuration values.
func (svr *server) Save() error {
	// Create the file...
	return globular.SaveService(svr)
}

func (svr *server) StartService() error {
	return globular.StartService(svr, svr.grpcServer)
}

func (svr *server) StopService() error {
	return globular.StopService(svr, svr.grpcServer)
}

var (
	resourceClient  *resource_client.Resource_Client
	discoveryClient *discovery_client.Dicovery_Client
	event_client_   *event_client.Event_Client
	log_client_     *log_client.Log_Client
)

////////////////////////////////////////////////////////////////////////////////////////
// Resource manager function
////////////////////////////////////////////////////////////////////////////////////////
func (svr *server) getResourceClient() (*resource_client.Resource_Client, error) {
	var err error
	if resourceClient != nil {
		return resourceClient, nil
	}

	resourceClient, err = resource_client.NewResourceService_Client(svr.Domain, "resource.ResourceService")
	if err != nil {
		resourceClient = nil
		return nil, err
	}

	return resourceClient, nil
}

func (svr *server) deleteApplication(applicationId string) error {
	resourceClient, err := svr.getResourceClient()
	if err != nil {
		return err
	}

	return resourceClient.DeleteApplication(applicationId)
}

func (svr *server) createApplication(id, password, path, publisherId, version, description, alias, icon string, actions, keywords []string) error {
	resourceClient, err := svr.getResourceClient()
	if err != nil {
		return err
	}

	return resourceClient.CreateApplication(id, password, path, publisherId, version, description, alias, icon, actions, keywords)

}

func (svr *server) getApplicationVersion(id string) (string, error) {
	resourceClient, err := svr.getResourceClient()
	if err != nil {
		return "", err
	}

	return resourceClient.GetApplicationVersion(id)
}

func (svr *server) getApplication(id string) (*resourcepb.Application, error) {
	resourceClient, err := svr.getResourceClient()
	if err != nil {
		return nil, err
	}

	applications, err := resourceClient.GetApplications(`{"_id":"`+ id +`"}`)
	if err != nil {
		return nil,  err
	}
	return applications[0], nil
}

func (svr *server) createRole(id, name string, actions []string) error {
	resourceClient, err := svr.getResourceClient()
	if err != nil {
		return err
	}

	return resourceClient.CreateRole(id, name, actions)
}

func (svr *server) createGroup(id, name, description string) error {
	resourceClient, err := svr.getResourceClient()
	if err != nil {
		return err
	}
	return resourceClient.CreateGroup(id, name, description)
}

func (svr *server) createNotification(notification *resourcepb.Notification) error {
	resourceClient, err := svr.getResourceClient()
	if err != nil {
		return err
	}

	return resourceClient.CreateNotification(notification)
}

//////////////////////// Package Repository services /////////////////////////////////
func (svr *server) getDsicoveryClient() (*discovery_client.Dicovery_Client, error) {
	if discoveryClient != nil {
		return discoveryClient, nil
	}

	discoveryClient, err := discovery_client.NewDiscoveryService_Client(svr.Domain, "discovery.PackageDiscovery")
	if err != nil {
		return nil, err
	}

	return discoveryClient, nil
}

func (server *server) publishApplication(user, organization, path, name, domain, version, description, icon, alias, repositoryId, discoveryId string, actions, keywords []string, roles []*resourcepb.Role, groups []*resourcepb.Group) error {
	discoveryClient, err := server.getDsicoveryClient()

	if err != nil {
		return err
	}

	// Publish the application...
	return discoveryClient.PublishApplication(user, organization, path, name, domain, version, description, icon, alias, repositoryId, discoveryId, actions, keywords, roles, groups)
}

///////////////////////  Log Services functions ////////////////////////////////////////////////

/**
 * Get the log client.
 */
func (server *server) GetLogClient() (*log_client.Log_Client, error) {
	var err error
	if log_client_ == nil {
		address, _ := config.GetAddress()
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

///////////////////// event service functions ////////////////////////////////////
func (svr *server) getEventClient() (*event_client.Event_Client, error) {
	var err error
	if event_client_ != nil {
		return event_client_, nil
	}
	event_client_, err = event_client.NewEventService_Client(svr.Domain, "event.EventService")
	if err != nil {
		return nil, err
	}

	return event_client_, nil
}

func (svr *server) publish(event string, data []byte) error {
	eventClient, err := svr.getEventClient()
	if err != nil {
		return err
	}
	return eventClient.Publish(event, data)
}


func (server *server) GetRbacClient() (*rbac_client.Rbac_Client, error) {
	var err error
	if rbac_client_ == nil {
		address, _:= config.GetAddress()
		rbac_client_, err = rbac_client.NewRbacService_Client(address, "rbac.RbacService")
		if err != nil {
			return nil, err
		}

	}
	return rbac_client_, nil
}

func (server *server) setActionResourcesPermissions(permissions map[string]interface{}) error {
	rbac_client_, err := server.GetRbacClient()
	if err != nil {
		return err
	}
	return rbac_client_.SetActionResourcesPermissions(permissions)
}

func (svr *server) addResourceOwner(path string, subject string, subjectType rbacpb.SubjectType) error {
	rbac_client_, err := svr.GetRbacClient()
	if err != nil {
		return err
	}
	return rbac_client_.AddResourceOwner(path, subject, subjectType)
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
	s_impl.Name = string(applications_managerpb.File_applications_manager_proto.Services().Get(0).FullName())
	s_impl.Proto = applications_managerpb.File_applications_manager_proto.Path()
	s_impl.Port = defaultPort
	s_impl.Proxy = defaultProxy
	s_impl.Protocol = "grpc"
	s_impl.Domain = domain
	s_impl.Version = "0.0.1"
	s_impl.PublisherId = "globulario"
	s_impl.Description = "Application manager service"
	s_impl.Keywords = []string{"Install, Uninstall, Deploy applications"}
	s_impl.Repositories = make([]string, 0)
	s_impl.Discoveries = make([]string, 0)
	s_impl.Dependencies = []string{"discovery.PackageDiscovery", "event.EventService", "resource.ResourceService"}
	s_impl.Permissions = make([]interface{}, 0)
	s_impl.WebRoot = config.GetWebRootDir()
	s_impl.AllowAllOrigins = allow_all_origins
	s_impl.AllowedOrigins = allowed_origins
	s_impl.Process = -1
	s_impl.ProxyProcess = -1
	s_impl.KeepAlive = true

	// Here I will retreive the list of connections from file if there are some...
	err := s_impl.Init()
	if err != nil {
		log.Fatalf("fail to initialyse service %s: %s", s_impl.Name, s_impl.Id)
	}
	if len(os.Args) == 2 {
		s_impl.Port, _ = strconv.Atoi(os.Args[1]) // The second argument must be the port number
	}

	// Register the echo services
	applications_managerpb.RegisterApplicationManagerServiceServer(s_impl.grpcServer, s_impl)
	reflection.Register(s_impl.grpcServer)

	// The user must part of owner in order to be able to deploy an application...
	s_impl.Permissions = append(s_impl.Permissions, map[string]interface{}{"action": "/applications_manager.ApplicationManagerService/DeployApplication", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "owner"}}})

	// Set the permissions
	s_impl.setActionResourcesPermissions(s_impl.Permissions[0].(map[string]interface{}))

	// Start the service.
	s_impl.StartService()
}
