package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/discovery/discovery_client"
	"github.com/globulario/services/golang/discovery/discoverypb"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/interceptors"
	"github.com/globulario/services/golang/log/log_client"
	"github.com/globulario/services/golang/log/logpb"
	"github.com/globulario/services/golang/rbac/rbac_client"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/repository/repository_client"
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

func (server *server) GetPermissions() []interface{} {
	return server.Permissions
}
func (server *server) SetPermissions(permissions []interface{}) {
	server.Permissions = permissions
}

// Create the configuration file if is not already exist.
func (server *server) Init() error {

	// That function is use to get access to other server.
	Utility.RegisterFunction("NewDiscoveryService_Client", discovery_client.NewDiscoveryService_Client)

	// Get the configuration path.
	dir, _ := filepath.Abs(filepath.Dir(os.Args[0]))

	err := globular.InitService(dir+"/config.json", server)
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
	dir, _ := filepath.Abs(filepath.Dir(os.Args[0]))
	return globular.SaveService(dir+"/config.json", server)
}

func (server *server) StartService() error {
	return globular.StartService(server, server.grpcServer)
}

func (server *server) StopService() error {
	return globular.StopService(server, server.grpcServer)
}

var (
	resourceClient *resource_client.Resource_Client
	rbac_client_   *rbac_client.Rbac_Client
)

//////////////////////// RBAC function //////////////////////////////////////////////
/**
 * Get the rbac client.
 */
func GetRbacClient(domain string) (*rbac_client.Rbac_Client, error) {
	var err error
	if rbac_client_ == nil {
		rbac_client_, err = rbac_client.NewRbacService_Client(domain, "rbac.RbacService")
		if err != nil {
			return nil, err
		}

	}
	return rbac_client_, nil
}

func (server *server) getResourcePermissions(path string) (*rbacpb.Permissions, error) {
	rbac_client_, err := GetRbacClient(server.Domain)
	if err != nil {
		return nil, err
	}

	return rbac_client_.GetResourcePermissions(path)
}

func (server *server) setResourcePermissions(path string, permissions *rbacpb.Permissions) error {
	rbac_client_, err := GetRbacClient(server.Domain)
	if err != nil {
		return err
	}
	return rbac_client_.SetResourcePermissions(path, permissions)
}

func (server *server) validateAccess(subject string, subjectType rbacpb.SubjectType, name string, path string) (bool, bool, error) {
	rbac_client_, err := GetRbacClient(server.Domain)
	if err != nil {
		return false, false, err
	}

	return rbac_client_.ValidateAccess(subject, subjectType, name, path)

}

func (svr *server) addResourceOwner(path string, subject string, subjectType rbacpb.SubjectType) error {
	rbac_client_, err := GetRbacClient(svr.Domain)
	if err != nil {
		return err
	}
	return rbac_client_.AddResourceOwner(path, subject, subjectType)
}

////////////////////////////////////////////////////////////////////////////////////////
// Resource manager function
////////////////////////////////////////////////////////////////////////////////////////
func (server *server) getResourceClient() (*resource_client.Resource_Client, error) {
	var err error
	if resourceClient != nil {
		return resourceClient, nil
	}
	address, _:= config.GetAddress()
	resourceClient, err = resource_client.NewResourceService_Client(address, "resource.ResourceService")
	if err != nil {
		resourceClient = nil
		return nil, err
	}

	return resourceClient, nil
}

/////////////////////// Resource function ///////////////////////////////////////////
func (server *server) isOrganizationMember(user, organization string) (bool, error) {
	resourceClient, err := server.getResourceClient()
	if err != nil {
		return false, err
	}

	return resourceClient.IsOrganizationMemeber(user, organization)
}

// Create/Update the package descriptor.
func (server *server) publishPackageDescriptor(descriptor *resourcepb.PackageDescriptor) error {
	resourceClient, err := server.getResourceClient()
	if err != nil {
		return err
	}

	return resourceClient.SetPackageDescriptor(descriptor)
}

///////////////////////  Log Services functions ////////////////////////////////////////////////
var (
	log_client_ *log_client.Log_Client
)

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

/////////////////////// Discovery specific function /////////////////////////////////

// Publish a package, the package can contain an application or a services.
func (server *server) publishPackage(user, organization, discovery, repository, platform, path string, descriptor *resourcepb.PackageDescriptor) error {

	// Ladies and Gentlemans After one year after tow years services as resource!
	path_ := descriptor.PublisherId + "/" + descriptor.Name + "/" + descriptor.Id + "/" + descriptor.Version

	// So here I will set the permissions
	var permissions *rbacpb.Permissions
	var err error
	permissions, err = server.getResourcePermissions(path_)
	if err != nil {
		// Create the permission...
		permissions = &rbacpb.Permissions{
			Allowed: []*rbacpb.Permission{
				//  Exemple of possible permission values.
				&rbacpb.Permission{
					Name:          "publish", // member of the organization can publish the service.
					Applications:  []string{},
					Accounts:      []string{},
					Groups:        []string{},
					Peers:         []string{},
					Organizations: []string{organization},
				},
			},
			Denied: []*rbacpb.Permission{},
			Owners: &rbacpb.Permission{
				Name:          "owner",
				Accounts:      []string{user},
				Applications:  []string{},
				Groups:        []string{},
				Peers:         []string{},
				Organizations: []string{},
			},
		}

		// Set the permissions.
		err = server.setResourcePermissions(path_, permissions)
		if err != nil {
			fmt.Println("fail to publish package with error: ", err.Error())
			return err
		}
	}

	// Test the permission before actualy publish the package.
	hasAccess, isDenied, err := server.validateAccess(user, rbacpb.SubjectType_ACCOUNT, "publish", path_)
	if !hasAccess || isDenied || err != nil {
		fmt.Println("publishPackage 527 ", err)
		return err
	}
	
	// Append the user into the list of owner if is not already part of it.
	if !Utility.Contains(permissions.Owners.Accounts, user) {
		permissions.Owners.Accounts = append(permissions.Owners.Accounts, user)
	}

	// Save the permissions.
	err = server.setResourcePermissions(path_, permissions)
	if err != nil {
		fmt.Println("publishPackage 539 ",err)
		return err
	}
	
	// Fist of all publish the package descriptor.
	err = server.publishPackageDescriptor(descriptor)
	if err != nil {
		fmt.Println("publishPackage 546 ",err)
		return err
	}

	// Connect to the repository resourcepb.
	services_repository, err := repository_client.NewRepositoryService_Client(repository, "repository.PackageRepository")
	if err != nil {
		err = errors.New("Fail to connect to package repository at " + repository)
		fmt.Println("publishPackage 554 ",err)
		return err
	}

	// UploadBundle
	server.logServiceInfo(Utility.FunctionName(), Utility.FileLine(), "Upload package bundle ", discovery+" "+descriptor.Id+" "+descriptor.PublisherId+" "+descriptor.Version+" "+platform+" "+path)
	err = services_repository.UploadBundle(discovery, descriptor.Id, descriptor.PublisherId, descriptor.Version, platform, path)
	
	// Upload the package bundle to the repository.
	return err
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
	s_impl.Name = string(discoverypb.File_discovery_proto.Services().Get(0).FullName())
	s_impl.Proto = discoverypb.File_discovery_proto.Path()
	s_impl.Port = defaultPort
	s_impl.Proxy = defaultProxy
	s_impl.Protocol = "grpc"
	s_impl.Domain = domain
	s_impl.Version = "0.0.1"
	s_impl.PublisherId = "globulario"
	s_impl.Description = "Service discovery client"
	s_impl.Keywords = []string{"Discovery", "Package", "Service", "Application"}
	s_impl.Repositories = make([]string, 0)
	s_impl.Discoveries = make([]string, 0)
	s_impl.Dependencies = []string{"rbac.RbacService", "resource.ResourceService"}
	s_impl.Permissions = make([]interface{}, 0)
	s_impl.Process = -1
	s_impl.ProxyProcess = -1
	s_impl.KeepAlive = true
	s_impl.AllowAllOrigins = allow_all_origins
	s_impl.AllowedOrigins = allowed_origins

	// Here I will retreive the list of connections from file if there are some...
	err := s_impl.Init()
	if err != nil {
		log.Fatalf("fail to initialyse service %s: %s", s_impl.Name, s_impl.Id)
	}
	if len(os.Args) == 2 {
		s_impl.Port, _ = strconv.Atoi(os.Args[1]) // The second argument must be the port number
	}

	// Register the echo services
	discoverypb.RegisterPackageDiscoveryServer(s_impl.grpcServer, s_impl)
	reflection.Register(s_impl.grpcServer)

	// Start the service.
	s_impl.StartService()

}
