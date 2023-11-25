package main

import (
	"bytes"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/applications_manager/applications_managerpb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/event/event_client"
	"github.com/globulario/services/golang/event/eventpb"
	"github.com/globulario/services/golang/globular_client"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/interceptors"
	"github.com/globulario/services/golang/rbac/rbac_client"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/resource/resource_client"
	"github.com/globulario/services/golang/resource/resourcepb"
	"github.com/globulario/services/golang/security"
	"github.com/golang/protobuf/jsonpb"
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
)

// Value need by Globular to start the services...
type server struct {
	// The global attribute of the services.
	Id              string
	Mac             string
	Name            string
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
	LastError       string
	ModTime         int64
	State           string
	TLS             bool

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

// //////////////////////////////////////////////////////////////////////////////////////
// Resource manager function
// //////////////////////////////////////////////////////////////////////////////////////
func (srv *server) getResourceClient(address string) (*resource_client.Resource_Client, error) {
	Utility.RegisterFunction("NewResourceService_Client", resource_client.NewResourceService_Client)
	client, err := globular_client.GetClient(address, "resource.ResourceService", "NewResourceService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*resource_client.Resource_Client), nil
}

func (srv *server) getApplication(applicationId string) (*resourcepb.Application, error) {

	if strings.Contains(applicationId, "@") {
		if strings.Split(applicationId, "@")[1] != srv.Domain {
			return nil, errors.New("you can only get application in your own domain")
		}
		applicationId = strings.Split(applicationId, "@")[0]
	}

	resourceClient, err := srv.getResourceClient(srv.Address)
	if err != nil {
		return nil, err
	}

	applications, err := resourceClient.GetApplications(`{"_id":"` + applicationId + `"}`)
	if err != nil {
		return nil, err
	}

	if len(applications) == 0 {
		return nil, errors.New("no application found with name or _id " + applicationId)
	}

	return applications[0], nil
}

func (srv *server) deleteApplication(token, applicationId string) error {

	_, err := security.ValidateToken(token)
	if err != nil {
		return err
	}

	resourceClient, err := srv.getResourceClient(srv.Address)
	if err != nil {
		return err
	}

	return resourceClient.DeleteApplication(token, applicationId)
}

func (srv *server) createApplication(token, id, name, domain, password, path, publisherId, version, description, alias, icon string, actions, keywords []string) error {

	if domain != srv.Domain {
		return errors.New("you can only create application in your own domain")
	}

	resourceClient, err := srv.getResourceClient(srv.Address)
	if err != nil {
		return err
	}

	err = resourceClient.CreateApplication(token, id, name, domain, password, path, publisherId, version, description, alias, icon, actions, keywords)

	return err

}

func (srv *server) createRole(token, id, name string, actions []string) error {
	resourceClient, err := srv.getResourceClient(srv.Address)
	if err != nil {
		return err
	}

	return resourceClient.CreateRole(token, id, name, actions)
}

func (srv *server) createGroup(token, id, name, description string) error {
	resourceClient, err := srv.getResourceClient(srv.Address)
	if err != nil {
		return err
	}
	return resourceClient.CreateGroup(token, id, name, description)
}

// ////////////////////// rbac service ///////////////////////////////////////
func (srv *server) GetRbacClient() (*rbac_client.Rbac_Client, error) {
	Utility.RegisterFunction("NewRbacService_Client", rbac_client.NewRbacService_Client)
	client, err := globular_client.GetClient(srv.Address, "rbac.RbacService", "NewRbacService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*rbac_client.Rbac_Client), nil
}

func (srv *server) addResourceOwner(path, resourceType, subject string, subjectType rbacpb.SubjectType) error {
	rbac_client_, err := srv.GetRbacClient()
	if err != nil {
		return err
	}
	return rbac_client_.AddResourceOwner(path, resourceType, subject, subjectType)
}

// /////////////////// event service functions ////////////////////////////////////

func (srv *server) getEventClient(address string) (*event_client.Event_Client, error) {
	Utility.RegisterFunction("NewEventService_Client", event_client.NewEventService_Client)
	client, err := globular_client.GetClient(address, "event.EventService", "NewEventService_Client")
	if err != nil {

		return nil, err
	}
	return client.(*event_client.Event_Client), nil
}

func (srv *server) publish(address, event string, data []byte) error {
	eventClient, err := srv.getEventClient(address)
	if err != nil {
		return err
	}
	err = eventClient.Publish(event, data)
	if err != nil {
		fmt.Println("fail to publish event", event, srv.Address, "with error", err)
	}
	return err
}

func (srv *server) subscribe(address, evt string, listener func(evt *eventpb.Event)) error {
	eventClient, err := srv.getEventClient(address)
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

func updateApplication(srv *server, application *resourcepb.Application) func(evt *eventpb.Event) {
	return func(evt *eventpb.Event) {

		descriptor := new(resourcepb.PackageDescriptor)
		err := jsonpb.UnmarshalString(string(evt.Data), descriptor)
		applicationId := Utility.GenerateUUID(application.Publisherid + "%" + application.Name + "%" + application.Version)

		// be sure the Id is set correctly.
		application.Id = applicationId
		descriptor.Id = application.Id

		if err == nil {
			token, err := security.GetLocalToken(srv.Mac)
			if err != nil {
				fmt.Println(err)
				return
			}

			// Here I will uninstall the application.

			err = srv.uninstallApplication(token, applicationId)
			if err == nil {
				// Get the new package...
				repository := descriptor.Repositories[0]
				package_repository, err := GetRepositoryClient(repository)
				if err != nil {
					fmt.Println("570 fail to install application with error: ", err)
					return
				}

				bundle, err := package_repository.DownloadBundle(descriptor, "webapp")
				if err != nil {
					fmt.Println("576 fail to install application with error: ", err)
					return
				}

				// Create the file.
				r := bytes.NewReader(bundle.Binairies)

				// Now I will install the applicaiton.
				err = srv.installApplication(token, srv.Domain, descriptor.Id, descriptor.Name, descriptor.PublisherId, descriptor.Version, descriptor.Description, descriptor.Icon, descriptor.Alias, r, descriptor.Actions, descriptor.Keywords, descriptor.Roles, descriptor.Groups, false)
				if err != nil {
					fmt.Println("588 fail to install application with error: ", err)
					return
				}

			}
		}
	}
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
	s_impl.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	s_impl.Port = defaultPort
	s_impl.Proxy = defaultProxy
	s_impl.Protocol = "grpc"
	s_impl.Domain, _ = config.GetDomain()
	s_impl.Address, _ = config.GetAddress()
	s_impl.Version = "0.0.1"
	s_impl.PublisherId = "globulario@globule-dell.globular.cloud"
	s_impl.Description = "Application manager service"
	s_impl.Keywords = []string{"Install, Uninstall, Deploy applications"}
	s_impl.Repositories = make([]string, 0)
	s_impl.Discoveries = make([]string, 0)
	s_impl.Dependencies = []string{"discovery.PackageDiscovery", "event.EventService", "resource.ResourceService"}
	s_impl.Permissions = make([]interface{}, 1)
	s_impl.WebRoot = config.GetWebRootDir()
	s_impl.AllowAllOrigins = allow_all_origins
	s_impl.AllowedOrigins = allowed_origins
	s_impl.Process = -1
	s_impl.ProxyProcess = -1
	s_impl.KeepAlive = true
	s_impl.KeepUpToDate = true

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
	applications_managerpb.RegisterApplicationManagerServiceServer(s_impl.grpcServer, s_impl)
	reflection.Register(s_impl.grpcServer)

	// The user must part of owner in order to be able to deploy an application...
	s_impl.Permissions[0] = map[string]interface{}{"action": "/applications_manager.ApplicationManagerService/DeployApplication", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "owner"}}}

	// keep applications up to date...
	go func() {
		resource_client, err := s_impl.getResourceClient(s_impl.Address)
		if err == nil {
			// retreive all applications.
			applications, err := resource_client.GetApplications("")
			if err == nil {
				for i := 0; i < len(applications); i++ {
					application := applications[i]
					evt := application.Publisherid + ":" + application.Name
					s_impl.subscribe(s_impl.Address, evt, updateApplication(s_impl, application))
				}
			}
		}
	}()

	// Start the service.
	s_impl.StartService()
}
