package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/davecourtois/Utility"
	"google.golang.org/protobuf/encoding/protojson"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/event/event_client"
	"github.com/globulario/services/golang/event/eventpb"
	"github.com/globulario/services/golang/globular_client"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/interceptors"
	"github.com/globulario/services/golang/log/log_client"
	"github.com/globulario/services/golang/log/logpb"
	"github.com/globulario/services/golang/process"
	"github.com/globulario/services/golang/rbac/rbac_client"
	"github.com/globulario/services/golang/resource/resource_client"
	"github.com/globulario/services/golang/resource/resourcepb"
	"github.com/globulario/services/golang/security"
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

// /////////////////// resource service functions ////////////////////////////////////

func (srv *server) getEventClient(address string) (*event_client.Event_Client, error) {
	Utility.RegisterFunction("NewEventService_Client", event_client.NewEventService_Client)
	client, err := globular_client.GetClient(address, "event.EventService", "NewEventService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*event_client.Event_Client), nil
}

// when services state change that publish
func (srv *server) publishUpdateServiceConfigEvent(config map[string]interface{}) error {
	data, err := json.Marshal(config)
	if err != nil {
		return err
	}

	client, err := srv.getEventClient(srv.Domain)
	if err != nil {
		return err
	}

	return client.Publish("update_globular_service_configuration_evt", data)
}

func (srv *server) publish(domain, event string, data []byte) error {
	eventClient, err := srv.getEventClient(domain)
	if err != nil {
		return err
	}
	err = eventClient.Publish(event, data)
	if err != nil {
		fmt.Println("fail to publish event", event, srv.Domain, "with error", err)
	}
	return err
}

func (srv *server) subscribe(domain, evt string, listener func(evt *eventpb.Event)) error {
	eventClient, err := srv.getEventClient(domain)
	if err != nil {
		fmt.Println("fail to get event client with error: ", err)
		return err
	}

	err = eventClient.Subscribe(evt, srv.Id, listener)
	if err != nil {

		fmt.Println("fail to subscribe to event with error: ", err)
		return err
	}

	// register a listener...
	return nil
}

// /////////////////// resource service functions ////////////////////////////////////
func (srv *server) getResourceClient() (*resource_client.Resource_Client, error) {
	Utility.RegisterFunction("NewResourceService_Client", resource_client.NewResourceService_Client)
	client, err := globular_client.GetClient(srv.Address, "resource.ResourceService", "NewResourceService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*resource_client.Resource_Client), nil
}

// ///////////////////// Resource manager function ////////////////////////////////////////
func (srv *server) removeRolesAction(action string) error {

	resourceClient, err := srv.getResourceClient()
	if err != nil {
		return err
	}

	return resourceClient.RemoveRolesAction(action)
}

func (srv *server) removeApplicationsAction(token, action string) error {

	resourceClient, err := srv.getResourceClient()
	if err != nil {
		return err
	}

	return resourceClient.RemoveApplicationsAction(token, action)
}

func (srv *server) removePeersAction(action string) error {
	resourceClient, err := srv.getResourceClient()
	if err != nil {
		return err
	}

	return resourceClient.RemovePeersAction("", action)
}

func (srv *server) setRoleActions(roleId string, actions []string) error {

	resourceClient, err := srv.getResourceClient()
	if err != nil {
		return err
	}

	return resourceClient.AddRoleActions(roleId, actions)
}

///////////////////// RBAC service function /////////////////////////////////////
/**
 * Get the rbac client.
 */
func (srv *server) GetRbacClient() (*rbac_client.Rbac_Client, error) {
	Utility.RegisterFunction("NewRbacService_Client", rbac_client.NewRbacService_Client)
	client, err := globular_client.GetClient(srv.Address, "rbac.RbacService", "NewRbacService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*rbac_client.Rbac_Client), nil
}

func (srv *server) setActionResourcesPermissions(permissions map[string]interface{}) error {
	rbac_client_, err := srv.GetRbacClient()
	if err != nil {
		return err
	}
	return rbac_client_.SetActionResourcesPermissions(permissions)
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

///////////////////////////// Service manager functions ///////////////////////////////////

// Stop a given service instance.
func (srv *server) stopService(s map[string]interface{}) error {
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

	return srv.publishUpdateServiceConfigEvent(s)
}

// uninstall service
func (srv *server) uninstallService(token, publisherId, serviceId, version string, deletePermissions bool) error {
	// First of all I will stop the running service(s) instance.
	services, err := config.GetServicesConfigurations()
	if err != nil {
		return err
	}
	for _, s := range services {
		// Stop the instance of the service.
		if s["PublisherId"].(string) == publisherId && s["Id"].(string) == serviceId && s["Version"].(string) == version {
			// First of all I will unsubcribe to the package event...
			srv.stopService(s)

			// Get the list of method to remove from the list of actions.
			toDelete, err := config.GetServiceMethods(s["Name"].(string), publisherId, version)
			if err != nil {
				return err
			}

			methods := make([]string, 0)
			for i := 0; i < len(srv.methods); i++ {
				if !Utility.Contains(toDelete, srv.methods[i]) {
					methods = append(methods, srv.methods[i])
				}
			}

			// Keep permissions use when we update a service.
			if deletePermissions {
				// Now I will remove action permissions
				for i := 0; i < len(toDelete); i++ {

					// Delete it from Role.
					srv.removeRolesAction(toDelete[i])

					// Delete it from Application.
					srv.removeApplicationsAction(token, toDelete[i])

					// Delete it from Peer.
					srv.removePeersAction(toDelete[i])
				}
			}

			srv.methods = methods
			srv.registerMethods()

			// Test if the path exit.
			path := srv.Root + "/services/" + publisherId + "/" + s["Name"].(string) + "/" + version + "/" + serviceId

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
func (srv *server) registerMethods() error {

	// Here I will persit the sa role if it dosent already exist.
	err := srv.setRoleActions("sa", srv.methods)
	if err != nil {
		return err
	}

	return nil
}

// That function will update a service to the version receive in the event (as part of descriptor).
func updateService(srv *server, service map[string]interface{}) func(evt *eventpb.Event) {
	return func(evt *eventpb.Event) {
		fmt.Println("update service received", string(evt.Name))
		if service["KeepUpToDate"].(bool) {
			descriptor := new(resourcepb.PackageDescriptor)
			err := protojson.Unmarshal(evt.Data, descriptor)
			if err == nil {
				fmt.Println("update service received", descriptor.Name, descriptor.PublisherId, descriptor.Id, descriptor.Version)
				token, err := security.GetLocalToken(srv.Mac)
				if err != nil {
					fmt.Println(err)
					return
				}

				// uninstall the service.
				if srv.stopService(service) == nil {
					if srv.uninstallService(token, descriptor.PublisherId, descriptor.Id, service["Version"].(string), true) == nil {
						err = srv.installService(token, descriptor)
						if err != nil {
							fmt.Println("fail to update service with error: ", err)
						} else {
							fmt.Println(service["Name"], "was updated")
						}
					}
				}
			}
		}
	}
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
	s_impl.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	s_impl.Protocol = "grpc"
	s_impl.Domain, _ = config.GetDomain()
	s_impl.Address, _ = config.GetAddress()
	s_impl.Version = "0.0.1"
	s_impl.PublisherId = "localhost"
	s_impl.Description = "Mircoservice manager service"
	s_impl.Keywords = []string{"Manager", "Service"}
	s_impl.Repositories = make([]string, 0)
	s_impl.Discoveries = make([]string, 0)
	s_impl.Dependencies = []string{"resource.ResourceService", "rbac.RbacService", "event.EventService"}
	s_impl.Permissions = make([]interface{}, 0)
	s_impl.Process = -1
	s_impl.ProxyProcess = -1
	s_impl.KeepAlive = true
	s_impl.KeepUpToDate = true
	s_impl.AllowAllOrigins = allow_all_origins
	s_impl.AllowedOrigins = allowed_origins
	s_impl.done = make(chan bool)

	// Create a new sync map.
	s_impl.methods = make([]string, 0)
	s_impl.PortsRange = "10000-10100"

	// The server root...
	s_impl.Root = config.GetGlobularExecPath()

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

	if s_impl.Address == "" {
		s_impl.Address, _ = config.GetAddress()
	}

	if err != nil {
		log.Fatalf("fail to initialyse service %s: %s", s_impl.Name, s_impl.Id)
	}
	s_impl.Root = strings.ReplaceAll(s_impl.Root, "\\", "/")

	// Register the service manager services
	services_managerpb.RegisterServicesManagerServiceServer(s_impl.grpcServer, s_impl)
	reflection.Register(s_impl.grpcServer)

	// keep applications up to date...
	go func() {

		// retreive all services configuations.
		services, err := config.GetServicesConfigurations()
		if err == nil {
			for i := 0; i < len(services); i++ {
				service := services[i]
				evt := service["PublisherId"].(string) + ":" + service["Id"].(string)
				values := strings.Split(service["PublisherId"].(string), "@")
				if len(values) == 2 {
					s_impl.subscribe(values[1], evt, updateService(s_impl, service))
				}
			}
		}

	}()

	// Start the service manager service.
	s_impl.StartService()
}
