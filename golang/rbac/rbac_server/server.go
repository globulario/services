package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/event/event_client"
	"github.com/globulario/services/golang/globular_client"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/interceptors"
	"github.com/globulario/services/golang/log/log_client"
	"github.com/globulario/services/golang/log/logpb"
	"github.com/globulario/services/golang/rbac/rbac_client"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/resource/resource_client"
	"github.com/globulario/services/golang/resource/resourcepb"
	"github.com/globulario/services/golang/storage/storage_store"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"google.golang.org/protobuf/encoding/protojson"
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
	Id                   string
	Mac                  string
	Name                 string
	Domain               string
	Address              string
	Path                 string
	Proto                string
	Port                 int
	Proxy                int
	AllowAllOrigins      bool
	AllowedOrigins       string // comma separated string.
	Protocol             string
	Version              string
	PublisherId          string
	KeepUpToDate         bool
	Plaform              string
	Checksum             string
	KeepAlive            bool
	Description          string
	Keywords             []string
	Repositories         []string
	Discoveries          []string
	Process              int
	ProxyProcess         int
	ConfigPath           string
	LastError            string
	State                string
	ModTime              int64
	CacheAddress         string

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

	// Here I will keep files info in memory...
	cache *storage_store.BigCache_store // Keep permission in cache for faster access.

	// The permission store.
	permissions storage_store.Store
}

// Set item value
func (srv *server) setItem(key string, val []byte) error {

	// I will set the value in the cache first.
	err := srv.cache.SetItem(key, val)
	if err != nil {
		return err
	}

	// I will set the value in the store.
	return srv.permissions.SetItem(key, val)
}

// Retreive item
func (srv *server) getItem(key string) ([]byte, error) {

	// I will use the cache first
	val, err := srv.cache.GetItem(key)
	if err == nil {
		return val, nil
	}

	// I will use the store.
	return srv.permissions.GetItem(key)
}

// Remove item.
func (srv *server) removeItem(key string) error {

	// I will remove the value from the cache first.
	err := srv.cache.RemoveItem(key)
	if err != nil {
		return err
	}

	// I will remove the value from the store.
	return srv.permissions.RemoveItem(key)
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
	if pid == -1 {

		// I will clear the cache.
		srv.cache.Clear()

		// I will close the permissions.
		srv.permissions.Close()

	}
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

////////////////////////////////////////////////////////////////////////////////////////
// Event function
////////////////////////////////////////////////////////////////////////////////////////

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

////////////////////////////////////////////////////////////////////////////////////////
// Logger function
////////////////////////////////////////////////////////////////////////////////////////
/**
 * Get the log client.
 */
func (srv *server) GetLogClient() (*log_client.Log_Client, error) {
	// validate the port has not change...
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

/**
 * Return an application with a given id
 */
func (srv *server) getAccount(accountId string) (*resourcepb.Account, error) {

	if !strings.Contains(accountId, "@") {
		accountId = accountId + "@" + srv.Domain
	}

	data, err := srv.cache.GetItem(accountId)
	if err == nil {
		a := new(resourcepb.Account)
		err := protojson.Unmarshal(data, a)
		if err == nil {
			return a, nil
		}
	}

	resourceClient, err := srv.getResourceClient(srv.Address)
	if err != nil {
		return nil, err
	}

	account, err := resourceClient.GetAccount(accountId)
	if err != nil {
		return nil, err
	}

	// here I will set save the group in the cache for further use...
	jsonStr, err := protojson.Marshal(account)
	if err == nil {
		srv.cache.SetItem(accountId, jsonStr)
	}

	return account, nil

}

func (srv *server) accountExist(id string) (bool, string) {

	a, err := srv.getAccount(id)
	if err != nil {
		return false, ""
	}

	return true, a.Id + "@" + a.Domain

}

/**
 * Return a group with a given id
 */
func (srv *server) getGroup(groupId string) (*resourcepb.Group, error) {

	// I will add the domain if it is not already there...
	if !strings.Contains(groupId, "@") {
		groupId = groupId + "@" + srv.Domain
	}

	// I will try to get the information from the cache to save time...
	data, err := srv.cache.GetItem(groupId)
	if err == nil {
		g := new(resourcepb.Group)
		err = protojson.Unmarshal(data, g)
		if err == nil {
			return g, nil
		}
	}

	resourceClient, err := srv.getResourceClient(srv.Address)
	if err != nil {
		return nil, err
	}

	groups, err := resourceClient.GetGroups(`{"_id":"` + groupId + `"}`)
	if err != nil {
		return nil, err
	}

	if len(groups) == 0 {
		return nil, errors.New("no group found wiht name or _id " + groupId)
	}

	// here I will set save the group in the cache for further use...
	jsonStr, err := protojson.Marshal(groups[0])
	if err == nil {
		srv.cache.SetItem(groupId, []byte(jsonStr))
	}

	return groups[0], nil

}

/**
 * Test if a group exist.
 */
func (srv *server) groupExist(id string) (bool, string) {

	g, err := srv.getGroup(id)
	if err != nil || g == nil {
		fmt.Println("fail to find group ", id)
		return false, ""
	}
	return true, g.Id + "@" + g.Domain

}

/**
 * Return an application with a given id
 */
func (srv *server) getApplication(applicationId string) (*resourcepb.Application, error) {
	// I will add the domain if it is not already there...
	if !strings.Contains(applicationId, "@") {
		applicationId = applicationId + "@" + srv.Domain
	}

	// Try to get the application with the _id or the name.
	q0 := `{"_id":"` + applicationId + `"}`
	q1 := `{"name":"` + applicationId + `"}`

	resourceClient, err := srv.getResourceClient(srv.Address)
	if err != nil {
		return nil, err
	}

	applications, err := resourceClient.GetApplications(q0)
	if err != nil || len(applications) == 0 {
		applications, err = resourceClient.GetApplications(q1)
		if err != nil {
			return nil, err
		}
	}

	if len(applications) == 0 {
		return nil, errors.New("no application found with name or _id " + applicationId)
	}

	return applications[0], nil

}

/**
 * Test if a application exist.
 */
func (srv *server) applicationExist(id string) (bool, string) {

	a, err := srv.getApplication(id)
	if err != nil || a == nil {
		return false, ""
	}

	return true, a.Id + "@" + a.Domain
}

/**
 * Return a peer with a given id
 */
func (srv *server) getPeer(peerId string) (*resourcepb.Peer, error) {
	address, _ := config.GetAddress()
	resourceClient, err := srv.getResourceClient(address)
	if err != nil {
		return nil, err
	}

	peers, err := resourceClient.GetPeers(`{"mac":"` + peerId + `"}`)
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
func (srv *server) peerExist(id string) bool {
	p, err := srv.getPeer(id)
	if err != nil || p == nil {
		return false
	}
	return true
}

/**
 * Return a peer with a given id
 */
func (srv *server) getOrganization(organizationId string) (*resourcepb.Organization, error) {

	if !strings.Contains(organizationId, "@") {
		organizationId = organizationId + "@" + srv.Domain
	}

	resourceClient, err := srv.getResourceClient(organizationId)
	if err != nil {
		return nil, err
	}

	organizations, err := resourceClient.GetOrganizations(`{"_id":"` + organizationId + `"}`)
	if err != nil {
		return nil, err
	}

	if len(organizations) == 0 {
		return nil, errors.New("no organization found wiht name or _id " + organizationId)
	}

	return organizations[0], nil

}

/**
 * Test if a organization exist.
 */
func (srv *server) organizationExist(id string) (bool, string) {

	o, err := srv.getOrganization(id)
	if err != nil || o == nil {
		return false, ""
	}

	return true, o.Id + "@" + o.Domain

}

func (srv *server) getRoles() ([]*resourcepb.Role, error) {

	// so here I will get the role from it domain resource manager.
	resource_, err := srv.getResourceClient(srv.Address)
	if err != nil {
		return nil, err
	}

	roles, err := resource_.GetRoles(``)
	if err != nil || len(roles) == 1 {
		return nil, err
	}

	return roles, nil
}

func (srv *server) getGroups() ([]*resourcepb.Group, error) {

	// so here I will get the role from it domain resource manager.
	resource_, err := srv.getResourceClient(srv.Address)
	if err != nil {
		return nil, err
	}

	groups, err := resource_.GetGroups(`{}`)
	if err != nil || len(groups) == 1 {
		return nil, err
	}

	return groups, nil
}

func (srv *server) getOrganizations() ([]*resourcepb.Organization, error) {

	// so here I will get the role from it domain resource manager.
	resource_, err := srv.getResourceClient(srv.Address)
	if err != nil {
		return nil, err
	}

	organizations, err := resource_.GetOrganizations(``)
	if err != nil || len(organizations) == 1 {
		return nil, err
	}

	return organizations, nil
}

/**
 * Return a role with a given id
 */
func (srv *server) getRole(roleId string) (*resourcepb.Role, error) {

	if !strings.Contains(roleId, "@") {
		roleId = roleId + "@" + srv.Domain
	}

	resourceClient, err := srv.getResourceClient(srv.Address)
	if err != nil {
		return nil, err
	}

	roles, err := resourceClient.GetRoles(`{"_id":"` + roleId + `"}`)
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
func (srv *server) roleExist(id string) (bool, string) {

	r, err := srv.getRole(id)
	if err != nil || r == nil {
		return false, ""
	}

	return true, r.Id + "@" + r.Domain

}

// //////////////////////////////////////////////////////////////////////////////////////
// RBAC specific functions
// //////////////////////////////////////////////////////////////////////////////////////
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
	s_impl.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	s_impl.Port = defaultPort
	s_impl.Proxy = defaultProxy
	s_impl.Protocol = "grpc"
	s_impl.Domain, _ = config.GetDomain()
	s_impl.Address, _ = config.GetAddress()
	s_impl.Version = "0.0.1"
	s_impl.PublisherId = "localhost"
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
	s_impl.KeepUpToDate = true
	s_impl.CacheAddress = s_impl.Address

	// register new client creator.
	Utility.RegisterFunction("NewRbacService_Client", rbac_client.NewRbacService_Client)

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
		log.Fatalf("fail to initialyse service %s: %s with error: %s", s_impl.Name, s_impl.Id, err.Error())
	}

	if s_impl.Address == "" {
		s_impl.Address, _ = config.GetAddress()
	}

	if s_impl.CacheAddress == "localhost" {
		s_impl.CacheAddress = s_impl.Address
	}

	if len(s_impl.Root) == 0 {
		s_impl.Root = config.GetDataDir()
	}

	// Set the cache
	s_impl.cache = storage_store.NewBigCache_store()
	err = s_impl.cache.Open("")
	if err != nil {
		fmt.Println("fail to read/create cache folder with error: ", s_impl.Root+"/cache", err)
	}

	// Register the rbac services
	rbacpb.RegisterRbacServiceServer(s_impl.grpcServer, s_impl)
	reflection.Register(s_impl.grpcServer)

	// Set the permission store.
	s_impl.permissions = storage_store.NewBadger_store()
	err = s_impl.permissions.Open(`{"path":"` + s_impl.Root + `", "name":"permissions"}`)
	if err != nil {
		fmt.Println("fail to read/create permissions folder with error: ", s_impl.Root+"/permissions", err)
	}

	// Need to be the owner in order to change permissions
	s_impl.setActionResourcesPermissions(map[string]interface{}{"action": "/rbac.RbacService/SetResourcePermissions", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "owner"}}})

	if err != nil {
		fmt.Println("Fail to connect to event channel generate_video_preview_event")
	}

	// I will remove used space values for the data base so It will be recalculated each time the server start...
	ids_, err := s_impl.getItem("USED_SPACE")
	ids := make([]string, 0)
	if err == nil {
		err := json.Unmarshal(ids_, &ids)
		if err == nil {
			for i := 0; i < len(ids); i++ {
				s_impl.removeItem(ids[i])
			}
		}
	}

	// Start the service.
	s_impl.StartService()

}
