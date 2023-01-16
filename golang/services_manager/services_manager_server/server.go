package main

import (
	"encoding/json"
	"log"
	"os"
	"strings"

	"github.com/davecourtois/Utility"

	"sync"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/config/config_client"
	"github.com/globulario/services/golang/event/event_client"
	"github.com/globulario/services/golang/globular_client"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/interceptors"
	"github.com/globulario/services/golang/log/log_client"
	"github.com/globulario/services/golang/log/logpb"
	"github.com/globulario/services/golang/process"
	"github.com/globulario/services/golang/rbac/rbac_client"
	"github.com/globulario/services/golang/resource/resource_client"
	"github.com/globulario/services/golang/services_manager/services_managerpb"
	"google.golang.org/grpc"

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
	Plaform         string
	Checksum        string
	KeepAlive       bool
	Description     string
	Keywords        []string
	Repositories    []string
	Discoveries     []string
	Process         int
	ProxyProcess    int
	ConfigPath      string
	LastError       string
	State           string
	ModTime         int64
	TLS             bool

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

	// The list of install services.
	services *sync.Map

	// The list of (gRpc) method's supported by this server.
	methods []string

	// The server root...
	Root string

	// The path where tls certificates are located.
	Creds string

	// The data path
	DataPath string

	// The porst Range
	PortsRange string

	// https certificate path
	Certificate string

	// https certificate bundle path
	CertificateAuthorityBundle string

	// When the service is stop...
	done chan bool
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

// /////////////////// resource service functions ////////////////////////////////////
func (server *server) getEventClient() (*event_client.Event_Client, error) {
	Utility.RegisterFunction("NewEventService_Client", event_client.NewEventService_Client)
	client, err := globular_client.GetClient(server.GetAddress(), "event.EventService", "NewEventService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*event_client.Event_Client), nil
}

// when services state change that publish
func (server *server) publishUpdateServiceConfigEvent(config map[string]interface{}) error {
	data, err := json.Marshal(config)
	if err != nil {
		return err
	}

	client, err := server.getEventClient()
	if err != nil {
		return err
	}

	return client.Publish("update_globular_service_configuration_evt", data)
}

// /////////////////// resource service functions ////////////////////////////////////
func (server *server) getResourceClient() (*resource_client.Resource_Client, error) {
	Utility.RegisterFunction("NewResourceService_Client", resource_client.NewResourceService_Client)
	client, err := globular_client.GetClient(server.Address, "resource.ResourceService", "NewResourceService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*resource_client.Resource_Client), nil
}

// ///////////////////// Resource manager function ////////////////////////////////////////
func (server *server) removeRolesAction(action string) error {

	resourceClient, err := server.getResourceClient()
	if err != nil {
		return err
	}

	return resourceClient.RemoveRolesAction(action)
}

func (server *server) removeApplicationsAction(token, action string) error {

	resourceClient, err := server.getResourceClient()
	if err != nil {
		return err
	}

	return resourceClient.RemoveApplicationsAction(token, action)
}

func (server *server) removePeersAction(action string) error {
	resourceClient, err := server.getResourceClient()
	if err != nil {
		return err
	}

	return resourceClient.RemovePeersAction("", action)
}

func (server *server) setRoleActions(roleId string, actions []string) error {

	resourceClient, err := server.getResourceClient()
	if err != nil {
		return err
	}

	return resourceClient.AddRoleActions(roleId, actions)
}

///////////////////// RBAC service function /////////////////////////////////////
/**
 * Get the rbac client.
 */
func (server *server) GetRbacClient() (*rbac_client.Rbac_Client, error) {
	Utility.RegisterFunction("NewRbacService_Client", rbac_client.NewRbacService_Client)
	client, err := globular_client.GetClient(server.Address, "rbac.RbacService", "NewRbacService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*rbac_client.Rbac_Client), nil
}

func (server *server) setActionResourcesPermissions(permissions map[string]interface{}) error {
	rbac_client_, err := server.GetRbacClient()
	if err != nil {
		return err
	}
	return rbac_client_.SetActionResourcesPermissions(permissions)
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
func (server *server) logServiceInfo(method, fileLine, functionName, infos string) error{
	log_client_, err := server.GetLogClient()
	if err != nil {
		return err
	}
	return log_client_.Log(server.Name, server.Domain, method, logpb.LogLevel_INFO_MESSAGE, infos, fileLine, functionName)
}

func (server *server) logServiceError(method, fileLine, functionName, infos string) error{
	log_client_, err := server.GetLogClient()
	if err != nil {
		return err
	}
	return log_client_.Log(server.Name, server.Address, method, logpb.LogLevel_ERROR_MESSAGE, infos, fileLine, functionName)
}

///////////////////////////// Service manager functions ///////////////////////////////////

// Stop a given service instance.
func (server *server) stopService(s map[string]interface{}) error {
	// Kill the service process
	err := process.KillServiceProcess(s)
	if err != nil {
		return err
	}

	// Save it config...
	s["State"] = "killed"
	s["Process"] = -1
	s["ProxyProcess"] = -1

	err = config.SaveServiceConfiguration(s)
	if err != nil {
		return err
	}

	return server.publishUpdateServiceConfigEvent(s)
}

// uninstall service
func (server *server) uninstallService(token, publisherId, serviceId, version string, deletePermissions bool) error {
	// First of all I will stop the running service(s) instance.
	services, err := config_client.GetServicesConfigurations()
	if err != nil {
		return err
	}
	for _, s := range services {
		// Stop the instance of the service.
		if s["PublisherId"].(string) == publisherId && s["Id"].(string) == serviceId && s["Version"].(string) == version {
			// First of all I will unsubcribe to the package event...
			server.stopService(s)

			// Get the list of method to remove from the list of actions.
			toDelete, err := config.GetServiceMethods(s["Name"].(string), publisherId, version)
			if err != nil {
				return err
			}

			methods := make([]string, 0)
			for i := 0; i < len(server.methods); i++ {
				if !Utility.Contains(toDelete, server.methods[i]) {
					methods = append(methods, server.methods[i])
				}
			}

			// Keep permissions use when we update a service.
			if deletePermissions {
				// Now I will remove action permissions
				for i := 0; i < len(toDelete); i++ {

					// Delete it from Role.
					server.removeRolesAction(toDelete[i])

					// Delete it from Application.
					server.removeApplicationsAction(token, toDelete[i])

					// Delete it from Peer.
					server.removePeersAction(toDelete[i])
				}
			}

			server.methods = methods
			server.registerMethods()

			// Test if the path exit.
			path := server.Root + "/services/" + publisherId + "/" + s["Name"].(string) + "/" + version + "/" + serviceId

			// Now I will remove the service.
			// Service are located into the packagespb...
			if Utility.Exists(path) {
				// remove directory and sub-directory.
				err := os.RemoveAll(path)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// Set admin method, guest role will be set in resource service directly because
// method are static.
func (server *server) registerMethods() error {

	// Here I will persit the sa role if it dosent already exist.
	err := server.setRoleActions("sa", server.methods)
	if err != nil {
		return err
	}

	return nil
}

// That service is use to give access to SQL.
// port number must be pass as argument.
func main() {

	// Set the log information in case of crash...
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Initialyse service with default values.
	s_impl := new(server)
	s_impl.Name = string(services_managerpb.File_services_manager_proto.Services().Get(0).FullName())
	s_impl.Proto = services_managerpb.File_services_manager_proto.Path()
	s_impl.Port = defaultPort
	s_impl.Proxy = defaultProxy
	s_impl.Path = os.Args[0]
	s_impl.Protocol = "grpc"
	s_impl.Domain, _ = config.GetDomain()
	s_impl.Address, _ = config.GetAddress()
	s_impl.Version = "0.0.1"
	s_impl.PublisherId = "globulario"
	s_impl.Description = "Mircoservice manager service"
	s_impl.Keywords = []string{"Manager", "Service"}
	s_impl.Repositories = make([]string, 0)
	s_impl.Discoveries = make([]string, 0)
	s_impl.Dependencies = []string{"resource.ResourceService", "rbac.RbacService", "event.EventService"}
	s_impl.Permissions = make([]interface{}, 0)
	s_impl.Process = -1
	s_impl.ProxyProcess = -1
	s_impl.KeepAlive = true
	s_impl.AllowAllOrigins = allow_all_origins
	s_impl.AllowedOrigins = allowed_origins
	s_impl.done = make(chan bool)

	// Create a new sync map.
	s_impl.services = new(sync.Map)
	s_impl.methods = make([]string, 0)
	s_impl.PortsRange = "10000-10100"

	// The server root...
	s_impl.Root = config.GetRootDir()

	// Set the paths
	s_impl.DataPath = config.GetDataDir()
	s_impl.Creds = config.GetConfigDir() + "/tls"

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
	s_impl.Root = strings.ReplaceAll(s_impl.Root, "\\", "/")

	// Register the echo services
	services_managerpb.RegisterServicesManagerServiceServer(s_impl.grpcServer, s_impl)
	reflection.Register(s_impl.grpcServer)

	// Start the service manager service.
	s_impl.StartService()
}
