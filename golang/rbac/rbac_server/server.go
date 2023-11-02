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
	"github.com/globulario/services/golang/resource/resource_client"
	"github.com/globulario/services/golang/resource/resourcepb"
	"github.com/globulario/services/golang/storage/storage_store"
	"github.com/golang/protobuf/jsonpb"
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
	CacheAddress    string
	CacheType       string

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
	permissions storage_store.Store

	// Here I will keep files info in memory...
	cache *storage_store.BigCache_store // todo use cache instead of memory...

}

// Set item value
func (svr *server) setItem(key string, val []byte) error {

	// set item in the cache...
	svr.cache.SetItem(key, val)

	return svr.permissions.SetItem(key, val)
}

// Retreive item
func (svr *server) getItem(key string) ([]byte, error) {

	// I will use the cache first
	val, err := svr.cache.GetItem(key)
	if err == nil {
		return val, nil
	}

	// Try with the store...
	val, err = svr.permissions.GetItem(key)
	if err == nil {
		svr.cache.SetItem(key, val)
	}

	return val, err
}

// Remove item.
func (svr *server) removeItem(key string) error {

	// remove item from the store...
	svr.cache.RemoveItem(key)

	return svr.permissions.RemoveItem(key)
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

////////////////////////////////////////////////////////////////////////////////////////
// Event function
////////////////////////////////////////////////////////////////////////////////////////

func (server *server) getEventClient() (*event_client.Event_Client, error) {
	Utility.RegisterFunction("NewEventService_Client", event_client.NewEventService_Client)
	client, err := globular_client.GetClient(server.Address, "event.EventService", "NewEventService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*event_client.Event_Client), nil
}

func (server *server) publish(event string, data []byte) error {
	eventClient, err := server.getEventClient()
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
func (server *server) GetLogClient() (*log_client.Log_Client, error) {
	// validate the port has not change...
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

// //////////////////////////////////////////////////////////////////////////////////////
// Resource manager function
// //////////////////////////////////////////////////////////////////////////////////////
func (server *server) getResourceClient(address string) (*resource_client.Resource_Client, error) {
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
func (server *server) getAccount(accountId string) (*resourcepb.Account, error) {

	data, err := server.cache.GetItem(accountId)
	if err == nil {
		a := new(resourcepb.Account)
		err = jsonpb.UnmarshalString(string(data), a)
		if err == nil {
			return a, nil
		}
	}

	localDomain, _ := config.GetDomain()
	var domain string

	if strings.Contains(accountId, "@") {
		if len(strings.Split(accountId, "@")[1]) > 0 {
			domain = strings.Split(accountId, "@")[1]

			// that can happen when the globule domain has change after configurations...
			// the domain was empty when first install, so the
			hostname, _ := os.Hostname()
			if domain == hostname {
				if strings.HasPrefix(localDomain, domain) {
					// in that particular case the domain has been change...
					domain = localDomain
				}
			}
		}
		accountId = strings.Split(accountId, "@")[0]
	} else {
		domain = localDomain
	}

	if localDomain != domain && len(domain) > 0 {

		// so here I will get the account from it domain resource manager.
		resource_, err := server.getResourceClient(domain)
		if err != nil {
			return nil, err
		}

		account, err := resource_.GetAccount(accountId)
		if err != nil {
			return nil, err
		}

		var marshaler jsonpb.Marshaler
		jsonStr, err := marshaler.MarshalToString(account)
		if err == nil {
			server.cache.SetItem(accountId+"@"+domain, []byte(jsonStr))
		}

		// In that case I will
		return account, nil

	} else {
		resourceClient, err := server.getResourceClient(server.Address)
		if err != nil {
			fmt.Println("fail to get account ", accountId)
			return nil, err
		}

		account, err := resourceClient.GetAccount(accountId)
		if err != nil {
			return nil, err
		}

		// here I will set save the group in the cache for further use...
		var marshaler jsonpb.Marshaler
		jsonStr, err := marshaler.MarshalToString(account)
		if err == nil {
			server.cache.SetItem(accountId+"@"+domain, []byte(jsonStr))
		}

		return account, nil
	}
}

func (server *server) accountExist(id string) (bool, string) {

	a, err := server.getAccount(id)
	if err != nil {
		return false, ""
	}

	return true, a.Id + "@" + a.Domain

}

/**
 * Return a group with a given id
 */
func (server *server) getGroup(groupId string) (*resourcepb.Group, error) {

	// I will try to get the information from the cache to save time...
	data, err := server.cache.GetItem(groupId)
	if err == nil {
		g := new(resourcepb.Group)
		err = jsonpb.UnmarshalString(string(data), g)
		if err == nil {
			return g, nil
		}
	}

	localDomain, _ := config.GetDomain()
	var domain string

	if strings.Contains(groupId, "@") {
		if len(strings.Split(groupId, "@")[1]) > 0 {
			domain = strings.Split(groupId, "@")[1]
		}
		groupId = strings.Split(groupId, "@")[0]
	} else {
		domain = localDomain
	}

	if localDomain != domain && len(domain) > 0 {

		// so here I will get the group from it domain resource manager.
		resource_, err := server.getResourceClient(domain)
		if err != nil {
			return nil, err
		}

		groups, err := resource_.GetGroups(`{"_id":"` + groupId + `"}`)
		if err != nil {
			return nil, err
		}

		// here I will set save the group in the cache for further use...
		var marshaler jsonpb.Marshaler
		jsonStr, err := marshaler.MarshalToString(groups[0])
		if err == nil {
			server.cache.SetItem(groupId+"@"+domain, []byte(jsonStr))
		}

		// In that case I will
		return groups[0], nil

	} else {
		resourceClient, err := server.getResourceClient(domain)
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
		var marshaler jsonpb.Marshaler
		jsonStr, err := marshaler.MarshalToString(groups[0])
		if err == nil {
			server.cache.SetItem(groupId+"@"+domain, []byte(jsonStr))
		}

		return groups[0], nil
	}
}

/**
 * Test if a group exist.
 */
func (server *server) groupExist(id string) (bool, string) {

	g, err := server.getGroup(id)
	if err != nil || g == nil {
		fmt.Println("fail to find group ", id)
		return false, ""
	}
	return true, g.Id + "@" + g.Domain

}

/**
 * Return an application with a given id
 */
func (server *server) getApplication(applicationId string) (*resourcepb.Application, error) {

	localDomain, _ := config.GetDomain()
	var domain string

	if strings.Contains(applicationId, "@") {
		if len(strings.Split(applicationId, "@")[1]) > 0 {
			domain = strings.Split(applicationId, "@")[1]
		}

		applicationId = strings.Split(applicationId, "@")[0]

	}

	// Try to get the application with the _id or the name.
	q0 := `{"_id":"` + applicationId + `"}`
	q1:= `{"name":"` + applicationId + `"}`
	
	if localDomain != domain && len(domain) > 0 {

		// so here I will get the account from it domain resource manager.
		resource_, err := server.getResourceClient(domain)
		if err != nil {
			return nil, err
		}

		applications, err := resource_.GetApplications(q0)
		if err != nil || len(applications) != 1 {
			applications, err = resource_.GetApplications(q1)
			if err != nil || len(applications) != 1 {
				return nil, err
			}
			return nil, err
		}

		// In that case I will
		return applications[0], nil

	} else {
		resourceClient, err := server.getResourceClient(localDomain)
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
}

/**
 * Test if a application exist.
 */
func (server *server) applicationExist(id string) (bool, string) {

	a, err := server.getApplication(id)
	if err != nil || a == nil {
		return false, ""
	}
	return true, a.Id + "@" + a.Domain
}

/**
 * Return a peer with a given id
 */
func (server *server) getPeer(peerId string) (*resourcepb.Peer, error) {
	address, _ := config.GetAddress()
	resourceClient, err := server.getResourceClient(address)
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
func (server *server) peerExist(id string) bool {
	p, err := server.getPeer(id)
	if err != nil || p == nil {
		return false
	}
	return true
}

/**
 * Return a peer with a given id
 */
func (server *server) getOrganization(organizationId string) (*resourcepb.Organization, error) {

	localDomain, _ := config.GetDomain()
	var domain string

	if strings.Contains(organizationId, "@") {
		if len(strings.Split(organizationId, "@")[1]) > 0 {
			domain = strings.Split(organizationId, "@")[1]
		}
		organizationId = strings.Split(organizationId, "@")[0]

	}

	if localDomain != domain && len(domain) > 0 {

		// so here I will get the account from it domain resource manager.
		resource_, err := server.getResourceClient(domain)
		if err != nil {
			return nil, err
		}

		organizations, err := resource_.GetOrganizations(`{"_id":"` + organizationId + `"}`)
		if err != nil || len(organizations) != 1 {
			return nil, err
		}

		// In that case I will
		return organizations[0], nil

	} else {

		resourceClient, err := server.getResourceClient(localDomain)
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
}

/**
 * Test if a organization exist.
 */
func (server *server) organizationExist(id string) (bool, string) {

	o, err := server.getOrganization(id)
	if err != nil || o == nil {
		return false, ""
	}

	return true, o.Id + "@" + o.Domain

}

func (server *server) getRoles() ([]*resourcepb.Role, error) {
	localDomain, _ := config.GetDomain()
	// so here I will get the role from it domain resource manager.
	resource_, err := server.getResourceClient(localDomain)
	if err != nil {
		return nil, err
	}

	roles, err := resource_.GetRoles(``)
	if err != nil || len(roles) == 1 {
		return nil, err
	}

	return roles, nil
}

func (server *server) getGroups() ([]*resourcepb.Group, error) {
	localDomain, _ := config.GetDomain()
	// so here I will get the role from it domain resource manager.
	resource_, err := server.getResourceClient(localDomain)
	if err != nil {
		return nil, err
	}

	groups, err := resource_.GetGroups(`{}`)
	if err != nil || len(groups) == 1 {
		return nil, err
	}

	return groups, nil
}

func (server *server) getOrganizations() ([]*resourcepb.Organization, error) {
	localDomain, _ := config.GetDomain()
	// so here I will get the role from it domain resource manager.
	resource_, err := server.getResourceClient(localDomain)
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
func (server *server) getRole(roleId string) (*resourcepb.Role, error) {

	localDomain, _ := config.GetDomain()
	var domain string
	if strings.Contains(roleId, "@") {
		if len(strings.Split(roleId, "@")[1]) > 0 {
			domain = strings.Split(roleId, "@")[1]
		}

		roleId = strings.Split(roleId, "@")[0]
	}

	if localDomain != domain && len(domain) > 0 {

		// so here I will get the role from it domain resource manager.
		resource_, err := server.getResourceClient(domain)
		if err != nil {
			return nil, err
		}

		roles, err := resource_.GetRoles(`{"_id":"` + roleId + `"}`)
		if err != nil || len(roles) == 1 {
			return nil, err
		}

		// In that case I will
		return roles[0], nil

	} else {
		resourceClient, err := server.getResourceClient(localDomain)
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
}

/**
 * Test if a role exist.
 */
func (server *server) roleExist(id string) (bool, string) {

	r, err := server.getRole(id)
	if err != nil || r == nil {
		return false, ""
	}

	return true, r.Id + "@" + r.Domain

}

// //////////////////////////////////////////////////////////////////////////////////////
// RBAC specific functions
// //////////////////////////////////////////////////////////////////////////////////////
func (server *server) GetPermissions() []interface{} {
	return server.Permissions
}
func (server *server) SetPermissions(permissions []interface{}) {
	server.Permissions = permissions
}

// Create the configuration file if is not already exist.
func (server *server) Init() error {

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
	s_impl.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	s_impl.Port = defaultPort
	s_impl.Proxy = defaultProxy
	s_impl.Protocol = "grpc"
	s_impl.Domain, _ = config.GetDomain()
	s_impl.Address, _ = config.GetAddress()
	s_impl.Version = "0.0.1"
	s_impl.PublisherId = "globulario@globule-dell.globular.cloud"
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
	s_impl.CacheType = ""

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

	// The rbac storage.
	//s_impl.permissions = storage_store.NewBadger_store()
	if s_impl.CacheType == "scylla" {
		s_impl.permissions = storage_store.NewScylla_store(s_impl.CacheAddress, `permissions`, 3)
	} else {
		s_impl.permissions = storage_store.NewBadger_store()
	}
	err = s_impl.permissions.Open(`{"path":"` + s_impl.Root + `", "name":"permissions"}`)
	if err != nil {
		fmt.Println("fail to read/create permissions folder with error: ", s_impl.Root+"/permissions", err)
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

	// Need to be the owner in order to change permissions
	s_impl.setActionResourcesPermissions(map[string]interface{}{"action": "/rbac.RbacService/SetResourcePermissions", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "owner"}}})

	if err != nil {
		fmt.Println("Fail to connect to event channel generate_video_preview_event")
	}

	// I will remove used space values for the data base so It will be recalculated each time the server start...
	ids_, err := s_impl.permissions.GetItem("USED_SPACE")
	ids := make([]string, 0)
	if err == nil {
		err := json.Unmarshal(ids_, &ids)
		if err == nil {
			for i := 0; i < len(ids); i++ {
				s_impl.permissions.RemoveItem(ids[i])
			}
		}
	}

	// Start the service.
	s_impl.StartService()

}
