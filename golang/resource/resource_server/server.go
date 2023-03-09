package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/davecourtois/Utility"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/event/event_client"
	"github.com/globulario/services/golang/globular_client"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/interceptors"
	"github.com/globulario/services/golang/log/log_client"
	"github.com/globulario/services/golang/log/logpb"
	"github.com/globulario/services/golang/persistence/persistence_client"
	"github.com/globulario/services/golang/persistence/persistence_store"
	"github.com/globulario/services/golang/rbac/rbac_client"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/resource/resourcepb"
	"github.com/globulario/services/golang/security"
	"github.com/txn2/txeh"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"golang.org/x/crypto/bcrypt"
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

	TLS bool

	// svr-signed X.509 public keys for distribution
	CertFile string

	// a private RSA key to sign and authenticate the public key
	KeyFile string

	// a private RSA key to sign and authenticate the public key
	CertAuthorityTrust string

	Permissions []interface{} // contains the action permission for the services.

	Dependencies []string // The list of services needed by this services.

	// The backend infos.
	Backend_address  string
	Backend_port     int64
	Backend_user     string
	Backend_password string
	DataPath         string

	// The session time out.
	SessionTimeout int

	// Data store where account, role ect are keep...
	store   persistence_store.Store
	isReady bool

	// The grpc server.
	grpcServer *grpc.Server
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

// The current service state
func (svr *server) GetState() string {
	return svr.State
}

func (svr *server) SetState(state string) {
	svr.State = state
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

// /////////////////// resource service functions ////////////////////////////////////
func GetEventClient(address string) (*event_client.Event_Client, error) {
	Utility.RegisterFunction("NewEventService_Client", event_client.NewEventService_Client)
	client, err := globular_client.GetClient(address, "event.EventService", "NewEventService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*event_client.Event_Client), nil
}

// when services state change that publish
func (server *server) publishEvent(evt string, data []byte, domain string) error {

	client, err := GetEventClient(domain)
	if err != nil {
		return err
	}

	err = client.Publish(evt, data)
	return err
}

// Public event to a peer other than the default one...
func (server *server) publishRemoteEvent(address, evt string, data []byte) error {

	client, err := GetEventClient(address)
	if err != nil {
		return err
	}

	return client.Publish(evt, data)
}

// ///////////////////////////////////// return the peers infos from a given peer /////////////////////////////
func (server *server) getPeerInfos(address, mac string) (*resourcepb.Peer, error) {

	client, err := GetResourceClient(address)
	if err != nil {
		fmt.Println("fail to connect with remote resource service with err: ", err)
		return nil, err
	}

	peers, err := client.GetPeers(`{"mac":"` + mac + `"}`)
	if err != nil {
		return nil, err
	}

	if len(peers) == 0 {
		return nil, errors.New("no peer found with mac address " + mac + " at address " + address)
	}

	return peers[0], nil

}

/** Retreive the peer public key */
func (server *server) getPeerPublicKey(address, mac string) (string, error) {

	macAddress, err := Utility.MyMacAddr(Utility.MyLocalIP())
	if err != nil {
		return "", err
	}

	if len(mac) == 0 {
		mac = macAddress
	}

	if mac == macAddress {
		key, err := security.GetPeerKey(mac)
		if err != nil {
			return "", err
		}

		return string(key), nil
	}

	client, err := GetResourceClient(address)
	if err != nil {
		return "", err
	}

	return client.GetPeerPublicKey(mac)
}

/** Set the host if it's part of the same local network. */
func (server *server) setLocalHosts(peer *resourcepb.Peer) error {
	fmt.Println("try to set ip and domain in /etc/host with value ip:", peer.LocalIpAddress, " domain:", peer.GetDomain())
	// Finaly I will set the domain in the hosts file...
	hosts, err := txeh.NewHostsDefault()
	if err != nil {
		fmt.Println("fail to set host entry ", peer.LocalIpAddress, peer.GetDomain(), " with error ", err)
		return err
	}

	if peer.ExternalIpAddress == Utility.MyIP() {
		hosts.AddHost(peer.LocalIpAddress, peer.GetDomain())
	} else {
		fmt.Println("528 peer is not on the same network...")
		return errors.New("the peer is not on the same local network")
	}

	err = hosts.Save()
	if err != nil {
		fmt.Println("fail to save hosts ", peer.LocalIpAddress, peer.GetDomain(), " with error ", err)
		return err
	}

	fmt.Println("peer whit address ", peer.LocalIpAddress, " was added to /etc/hosts with domain: ", peer.GetDomain())

	return nil
}

/** Set the host if it's part of the same local network. */
func (server *server) removeFromLocalHosts(peer *resourcepb.Peer) error {
	// Finaly I will set the domain in the hosts file...
	hosts, err := txeh.NewHostsDefault()
	if err != nil {
		return err
	}

	domain := peer.GetDomain()

	if peer.ExternalIpAddress == Utility.MyIP() {
		hosts.RemoveHost(domain)
	} else {
		return errors.New("the peer is not on the same local network")
	}

	err = hosts.Save()
	if err != nil {
		fmt.Println("fail to save hosts file with error ", err)
	}

	return err
}

// ///////////////////////////////////// Get Persistence Client //////////////////////////////////////////
func GetPersistenceClient(address string) (*persistence_client.Persistence_Client, error) {
	Utility.RegisterFunction("NewPersistenceService_Client", persistence_client.NewPersistenceService_Client)
	client, err := globular_client.GetClient(address, "persistence.PersistenceService", "NewPersistenceService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*persistence_client.Persistence_Client), nil
}

// Create the application connections in the backend.
func (server *server) createApplicationConnection(app *resourcepb.Application) error {
	persistence_client_, err := GetPersistenceClient(server.Domain)
	if err != nil {
		return err
	}

	err = persistence_client_.CreateConnection(app.Id, app.Id+"_db", server.Domain, 27017, 0, app.Id, app.Password, 500, "", false)
	if err != nil {
		return err
	}

	return nil
}

//////////////////////////////////////// RBAC Functions ///////////////////////////////////////////////

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

func (svr *server) deleteResourcePermissions(path string) error {
	rbac_client_, err := GetRbacClient(svr.Address)
	if err != nil {
		return err
	}
	return rbac_client_.DeleteResourcePermissions(path)
}

func (svr *server) deleteAllAccess(suject string, subjectType rbacpb.SubjectType) error {
	rbac_client_, err := GetRbacClient(svr.Address)
	if err != nil {
		return err
	}
	return rbac_client_.DeleteAllAccess(suject, subjectType)
}

//////////////////////////////////////// Resource Functions ///////////////////////////////////////////////

// Create the configuration file if is not already exist.
func (svr *server) Init() error {

	// Get the configuration path.
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

////////////////////////////////// Resource functions ///////////////////////////////////////////////

// MongoDB backend, it must reside on the same server as the resource server (at this time...)

/**
 * Connection to mongo db local store.
 */
func (svr *server) getPersistenceStore() (persistence_store.Store, error) {
	// That service made user of persistence service.

	if svr.store == nil {

		svr.store = new(persistence_store.MongoStore)
		// Start the store if is not already running...
		err := svr.store.Start(svr.Backend_user, svr.Backend_password, int(int32(svr.Backend_port)), svr.DataPath)
		if err != nil {
			// codes.
			fmt.Println("fail to start MongoDB store with error ", err)
			return nil, err
		}

		err = svr.store.Connect("local_resource", svr.Backend_address, int32(svr.Backend_port), svr.Backend_user, svr.Backend_password, "local_resource", 5000, "")
		if err != nil {
			fmt.Println("fail to connect MongoDB store with error ", err)
			return nil, err
		}

		err = svr.store.Ping(context.Background(), "local_resource")

		svr.isReady = true

	} else if !svr.isReady {
		nbTry := 100
		for i := 0; i < nbTry; i++ {
			time.Sleep(100 * time.Millisecond)
			if svr.isReady {
				break
			}
		}
	}

	return svr.store, nil
}

/**
 *  hashPassword return the bcrypt hash of the password.
 */
func (resource_server *server) hashPassword(password string) (string, error) {
	haspassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(haspassword), nil
}

/**
 * Return the hash password.
 */
func (resource_server *server) validatePassword(password string, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

/**
 * Register an Account.
 */
func (resource_server *server) registerAccount(domain, id, name, email, password string, organizations []string, roles []string, groups []string) error {

	localDomain, err := config.GetDomain()
	if err != nil {
		return err
	}

	if domain != localDomain {
		return errors.New("you cant register account with domain " + domain + " on domain " + localDomain)
	}

	// That service made user of persistence service.
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return err
	}

	// first of all the Persistence service must be active.
	count, err := p.Count(context.Background(), "local_resource", "local_resource", "Accounts", `{"$or":[{"_id":"`+id+`"},{"name":"`+id+`"},{"name":"`+name+`"} ]}`, "")
	if err != nil {
		return err
	}

	// one account already exist for the name.
	if count == 1 {
		return errors.New("account with name " + name + " already exist!")
	}

	// set the account object and set it basic roles.
	account := make(map[string]interface{})
	account["_id"] = id
	account["name"] = name
	account["email"] = email
	account["domain"] = domain
	account["password"], _ = resource_server.hashPassword(password) // hide the password...

	// List of aggregation.
	account["roles"] = make([]interface{}, 0)
	account["groups"] = make([]interface{}, 0)
	account["organizations"] = make([]interface{}, 0)

	// append guest role if not already exist.
	if !Utility.Contains(roles, "guest@"+localDomain) {
		roles = append(roles, "guest@"+localDomain)
	}

	// Here I will insert the account in the database.
	_, err = p.InsertOne(context.Background(), "local_resource", "local_resource", "Accounts", account, "")
	if err != nil {
		return err
	}

	// replace @ and . by _  * active directory
	name = strings.ReplaceAll(strings.ReplaceAll(name, "@", "_"), ".", "_")

	// Each account will have their own database and a use that can read and write
	// into it.
	// Here I will wrote the script for mongoDB...
	createUserScript := fmt.Sprintf("db=db.getSiblingDB('%s_db');db.createCollection('user_data');db=db.getSiblingDB('admin');db.createUser({user: '%s', pwd: '%s',roles: [{ role: 'dbOwner', db: '%s_db' }]});", name, name, password, name)

	// I will execute the sript with the admin function.
	p.RunAdminCmd(context.Background(), "local_resource", resource_server.Backend_user, resource_server.Backend_password, createUserScript)

	// Organizations
	for i := 0; i < len(organizations); i++ {
		resource_server.createCrossReferences(organizations[i], "Organizations", "accounts", id, "Accounts", "organizations")
	}

	// Roles
	for i := 0; i < len(roles); i++ {
		resource_server.createCrossReferences(roles[i], "Roles", "members", id, "Accounts", "roles")
	}

	// Groups
	for i := 0; i < len(groups); i++ {
		resource_server.createCrossReferences(groups[i], "Groups", "members", id, "Accounts", "groups")
	}

	// Create the user file directory.
	path := "/users/" + id + "@" + localDomain
	Utility.CreateDirIfNotExist(config.GetDataDir() + "/files" + path)
	err = resource_server.addResourceOwner(path, "file", id, rbacpb.SubjectType_ACCOUNT)
	return err
}

func (resource_server *server) deleteReference(p persistence_store.Store, refId, targetId, targetField, targetCollection string) error {

	fmt.Println("try to remove ", refId, "from", targetId, "field", targetField, "collection", targetCollection)
	if strings.Contains(targetId, "@") {
		domain := strings.Split(targetId, "@")[1]
		targetId = strings.Split(targetId, "@")[0]

		localDomain, err := config.GetDomain()
		if err != nil {
			return err
		}

		if localDomain != domain {
			// so here I will redirect the call to the resource server at remote location.
			client, err := GetResourceClient(domain)
			if err != nil {
				return err
			}
			fmt.Println("remote call from ", localDomain, "to", domain)
			err = client.DeleteReference(refId, targetId, targetField, targetCollection)
			if err != nil {
				return err
			}

			return nil
		}
	}

	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", targetCollection, `{"_id":"`+targetId+`"}`, ``)
	if err != nil {
		return err
	}

	target := values.(map[string]interface{})

	if target[targetField] == nil {
		return errors.New("No field named " + targetField + " was found in object with id " + targetId + "!")
	}

	references := []interface{}(target[targetField].(primitive.A))
	references_ := make([]interface{}, 0)
	for j := 0; j < len(references); j++ {
		if references[j].(map[string]interface{})["$id"] != refId {
			references_ = append(references_, references[j])
		}
	}

	target[targetField] = references_
	jsonStr := serialyseObject(target)

	err = p.ReplaceOne(context.Background(), "local_resource", "local_resource", targetCollection, `{"_id":"`+targetId+`"}`, jsonStr, ``)
	if err != nil {
		return err
	}
	return nil
}

func (resource_server *server) getRemoteAccount(id string, domain string) (*resourcepb.Account, error) {
	fmt.Println("get account ", id, "from", domain)
	client, err := GetResourceClient(domain)
	if err != nil {
		return nil, err
	}

	return client.GetAccount(id)
}

func (resource_server *server) createReference(p persistence_store.Store, id, sourceCollection, field, targetId, targetCollection string) error {

	var err error
	var source map[string]interface{}
	localDomain, err := config.GetDomain()
	if err != nil {
		return err
	}

	if strings.Contains(id, "@") {
		domain := strings.Split(id, "@")[1]
		id = strings.Split(id, "@")[0]

		if localDomain != domain {
			// so here I will redirect the call to the resource server at remote location.
			client, err := GetResourceClient(domain)
			if err != nil {
				return err
			}

			err = client.CreateReference(id, sourceCollection, field, targetId, targetCollection)
			if err != nil {
				return err
			}

			return nil // exit...
		}
	}

	if !strings.Contains(targetId, "@") {
		targetId += "@" + localDomain
	}

	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", sourceCollection, `{"_id":"`+id+`"}`, ``)
	if err != nil {
		return errors.New("fail to find object with id " + id + " in collection " + sourceCollection + " err: " + err.Error())
	}

	source = values.(map[string]interface{})

	references := make([]interface{}, 0)
	if source[field] != nil {
		references = []interface{}(source[field].(primitive.A))
	}

	for j := 0; j < len(references); j++ {
		if references[j].(map[string]interface{})["$id"] == targetId {
			return errors.New(" named " + targetId + " aleready exist in  " + field + "!")
		}
	}

	// append the account.
	source[field] = append(references, map[string]interface{}{"$ref": targetCollection, "$id": targetId, "$db": "local_resource"})
	jsonStr := serialyseObject(source)

	err = p.ReplaceOne(context.Background(), "local_resource", "local_resource", sourceCollection, `{"_id":"`+id+`"}`, jsonStr, ``)
	if err != nil {
		return err
	}

	return nil
}

func (resource_server *server) createCrossReferences(sourceId, sourceCollection, sourceField, targetId, targetCollection, targetField string) error {
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return err
	}

	err = resource_server.createReference(p, targetId, targetCollection, targetField, sourceId, sourceCollection)
	if err != nil {
		return err
	}

	err = resource_server.createReference(p, sourceId, sourceCollection, sourceField, targetId, targetCollection)

	return err

}

//////////////////////////// Loggin info ///////////////////////////////////////

// That function is necessary to serialyse reference and kept field orders
func serialyseObject(obj map[string]interface{}) string {
	// Here I will save the role.
	jsonStr, _ := Utility.ToJson(obj)
	jsonStr = strings.ReplaceAll(jsonStr, `"$ref"`, `"__a__"`)
	jsonStr = strings.ReplaceAll(jsonStr, `"$id"`, `"__b__"`)
	jsonStr = strings.ReplaceAll(jsonStr, `"$db"`, `"__c__"`)

	obj_ := make(map[string]interface{}, 0)

	json.Unmarshal([]byte(jsonStr), &obj_)
	jsonStr, _ = Utility.ToJson(obj_)
	jsonStr = strings.ReplaceAll(jsonStr, `"__a__"`, `"$ref"`)
	jsonStr = strings.ReplaceAll(jsonStr, `"__b__"`, `"$id"`)
	jsonStr = strings.ReplaceAll(jsonStr, `"__c__"`, `"$db"`)

	return jsonStr
}

func (resource_server *server) createGroup(id, name, owner, description string, members []string) error {

	localDomain, err := config.GetDomain()
	if err != nil {
		return err
	}

	// test if the given domain is the local domain.
	if strings.Contains(id, "@") {
		domain := strings.Split(id, "@")[1]
		id = strings.Split(id, "@")[0]
		if domain != localDomain {
			return errors.New("you can't register group " + id + " with domain " + domain + " on domain " + localDomain)
		}
	}

	// Get the persistence connection
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return err
	}

	// Here I will first look if a peer with a same name already exist on the
	// resources...
	count, _ := p.Count(context.Background(), "local_resource", "local_resource", "Groups", `{"$or":[{"_id":"`+id+`"},{"name":"`+id+`"},{"name":"`+name+`"} ]}`, "")
	if count > 0 {
		return errors.New("Group with name '" + id + "' already exist!")
	}

	// No authorization exist for that peer I will insert it.
	// Here will create the new peer.
	g := make(map[string]interface{}, 0)
	g["_id"] = id
	g["name"] = name
	g["description"] = description
	g["domain"] = resource_server.Domain

	_, err = p.InsertOne(context.Background(), "local_resource", "local_resource", "Groups", g, "")
	if err != nil {
		return err
	}

	// Create references.
	for i := 0; i < len(members); i++ {
		err := resource_server.createCrossReferences(id, "Groups", "members", members[i], "Accounts", "groups")
		if err != nil {
			return err
		}
	}

	// Now create the resource permission.

	resource_server.addResourceOwner(id+"@"+resource_server.Domain, "group", owner, rbacpb.SubjectType_ACCOUNT)
	fmt.Println("group ", id, "was create with owner ", owner)
	return nil
}

func (resource_server *server) CreateAccountDir() error {
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return err
	}

	accounts, err := p.Find(context.Background(), "local_resource", "local_resource", "Accounts", "{}", "")
	if err != nil {
		return err
	}
	for i := 0; i < len(accounts); i++ {
		a := accounts[i].(map[string]interface{})
		id := a["_id"].(string)
		domain := a["domain"].(string)
		path := "/users/" + id + "@" + domain
		if !Utility.Exists(config.GetDataDir() + "/files" + path) {
			Utility.CreateDirIfNotExist(config.GetDataDir() + "/files" + path)
			resource_server.addResourceOwner(path, "file", id, rbacpb.SubjectType_ACCOUNT)
		}
	}

	return nil
}

func (resource_server *server) createRole(id, name, owner string, actions []string) error {
	localDomain, err := config.GetDomain()
	if err != nil {
		return err
	}

	// test if the given domain is the local domain.
	if strings.Contains(id, "@") {
		domain := strings.Split(id, "@")[1]
		id = strings.Split(id, "@")[0]
		if domain != localDomain {
			return errors.New("you can't create role " + id + " with domain " + domain + " on domain " + localDomain)
		}
	}

	// That service made user of persistence service.
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return err
	}

	_, err = p.FindOne(context.Background(), "local_resource", "local_resource", "Roles", `{"$or":[{"_id":"`+id+`"},{"name":"`+id+`"},{"name":"`+name+`"} ]}`, ``)
	if err == nil {
		return errors.New("Role named " + name + " already exist!")
	}

	// Here will create the new role.
	role := make(map[string]interface{})
	role["_id"] = id
	role["name"] = name
	role["actions"] = actions
	role["domain"] = resource_server.Domain

	_, err = p.InsertOne(context.Background(), "local_resource", "local_resource", "Roles", role, "")
	if err != nil {
		return err
	}

	resource_server.addResourceOwner(id+"@"+resource_server.Domain, "role", owner, rbacpb.SubjectType_ACCOUNT)
	return nil
}

/**
 * Delete application Data from the backend.
 */
func (resource_server *server) deleteApplication(applicationId string) error {

	if strings.Contains(applicationId, "@") {
		domain := strings.Split(applicationId, "@")[1]
		applicationId = strings.Split(applicationId, "@")[0]

		localDomain, err := config.GetDomain()
		if err != nil {
			return err
		}

		if localDomain != domain {
			return errors.New("i cant's delete object from domain " + domain + " from domain " + localDomain)
		}
	}

	// That service made user of persistence service.
	p, err := resource_server.getPersistenceStore()
	if err != nil {
		return err
	}

	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Applications", `{"_id":"`+applicationId+`"}`, ``)
	if err != nil {
		return err
	}

	application := values.(map[string]interface{})

	// I will remove it from organization...
	if application["organizations"] != nil {
		organizations := []interface{}(application["organizations"].(primitive.A))

		for i := 0; i < len(organizations); i++ {
			organizationId := organizations[i].(map[string]interface{})["$id"].(string)
			resource_server.deleteReference(p, applicationId, organizationId, "applications", "Organizations")
		}
	}

	// I will remove the directory.
	err = os.RemoveAll(application["path"].(string))
	if err != nil {
		return err
	}

	// Now I will remove the database create for the application.
	err = p.DeleteDatabase(context.Background(), "local_resource", applicationId+"_db")
	if err != nil {
		return err
	}

	// Finaly I will remove the entry in  the table.
	err = p.DeleteOne(context.Background(), "local_resource", "local_resource", "Applications", `{"_id":"`+applicationId+`"}`, "")
	if err != nil {
		return err
	}

	// Delete permissions
	err = p.Delete(context.Background(), "local_resource", "local_resource", "Permissions", `{"owner":"`+applicationId+`"}`, "")
	if err != nil {
		return err
	}

	// Drop the application user.
	// Here I will drop the db user.
	dropUserScript := fmt.Sprintf(
		`db=db.getSiblingDB('admin');db.dropUser('%s', {w: 'majority', wtimeout: 4000})`,
		applicationId)

	// I will execute the sript with the admin function.
	err = p.RunAdminCmd(context.Background(), "local_resource", resource_server.Backend_user, resource_server.Backend_password, dropUserScript)
	if err != nil {
		return err
	}

	// set back the domain part
	applicationId = application["_id"].(string) + "@" + application["domain"].(string)

	resource_server.deleteAllAccess(applicationId, rbacpb.SubjectType_APPLICATION)
	resource_server.deleteResourcePermissions(applicationId)
	resource_server.publishEvent("delete_application_"+applicationId+"_evt", []byte{}, application["domain"].(string))
	resource_server.publishEvent("delete_application_evt", []byte(applicationId), application["domain"].(string))

	return nil
}

// That service is use to give access to SQL.
// port number must be pass as argument.
func main() {

	log.Println("start service resource manager")
	// Set the log information in case of crash...
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Initialyse service with default values.
	s_impl := new(server)
	s_impl.Name = string(resourcepb.File_resource_proto.Services().Get(0).FullName())
	s_impl.Proto = resourcepb.File_resource_proto.Path()
	s_impl.Port = defaultPort
	s_impl.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	s_impl.Proxy = defaultProxy
	s_impl.Protocol = "grpc"
	s_impl.Domain, _ = config.GetDomain()
	s_impl.Address, _ = config.GetAddress()
	s_impl.Version = "0.0.1"
	s_impl.PublisherId = "globulario"
	s_impl.Description = "Resource manager service. Resources are Group, Account, Role, Organization and Peer."
	s_impl.Keywords = []string{"Resource"}
	s_impl.Repositories = make([]string, 0)
	s_impl.Discoveries = make([]string, 0)
	s_impl.Dependencies = []string{"log.LogService"}
	s_impl.Permissions = make([]interface{}, 23)
	s_impl.AllowAllOrigins = allow_all_origins
	s_impl.AllowedOrigins = allowed_origins
	s_impl.Process = -1
	s_impl.ProxyProcess = -1
	s_impl.SessionTimeout = 15
	s_impl.KeepAlive = true

	// Backend informations.
	s_impl.Backend_address = "localhost"
	s_impl.Backend_port = 27017
	s_impl.Backend_user = "sa"
	s_impl.Backend_password = "adminadmin"
	s_impl.DataPath = config.GetDataDir()

	// Set the Permissions...
	s_impl.Permissions[0] = map[string]interface{}{"action": "/resource.ResourceService/DeletePermissions", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "delete"}}}
	s_impl.Permissions[1] = map[string]interface{}{"action": "/resource.ResourceService/SetResourceOwner", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "write"}}}
	s_impl.Permissions[2] = map[string]interface{}{"action": "/resource.ResourceService/DeleteResourceOwner", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "write"}}}

	// Group function...
	s_impl.Permissions[3] = map[string]interface{}{"action": "/resource.ResourceService/AddGroupMemberAccount", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "owner"}}}
	s_impl.Permissions[4] = map[string]interface{}{"action": "/resource.ResourceService/DeleteGroup", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "owner"}}}
	s_impl.Permissions[5] = map[string]interface{}{"action": "/resource.ResourceService/RemoveGroupMemberAccount", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "owner"}}}
	s_impl.Permissions[6] = map[string]interface{}{"action": "/resource.ResourceService/UpdateGroup", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "owner"}}}

	// Organization function...
	s_impl.Permissions[7] = map[string]interface{}{"action": "/resource.ResourceService/UpdateOrganization", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "owner"}}}
	s_impl.Permissions[8] = map[string]interface{}{"action": "/resource.ResourceService/DeleteOrganization", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "owner"}}}
	s_impl.Permissions[9] = map[string]interface{}{"action": "/resource.ResourceService/AddOrganizationAccount", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "owner"}}}
	s_impl.Permissions[10] = map[string]interface{}{"action": "/resource.ResourceService/AddOrganizationGroup", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "owner"}}}
	s_impl.Permissions[11] = map[string]interface{}{"action": "/resource.ResourceService/AddOrganizationRole", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "owner"}}}
	s_impl.Permissions[12] = map[string]interface{}{"action": "/resource.ResourceService/AddOrganizationApplication", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "owner"}}}
	s_impl.Permissions[13] = map[string]interface{}{"action": "/resource.ResourceService/RemoveOrganizationGroup", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "owner"}}}
	s_impl.Permissions[14] = map[string]interface{}{"action": "/resource.ResourceService/RemoveOrganizationRole", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "owner"}}}
	s_impl.Permissions[15] = map[string]interface{}{"action": "/resource.ResourceService/RemoveOrganizationApplication", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "owner"}}}

	// account
	s_impl.Permissions[16] = map[string]interface{}{"action": "/resource.ResourceService/SetEmail", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "owner"}}}
	s_impl.Permissions[17] = map[string]interface{}{"action": "/resource.ResourceService/SetAccountPassword", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "owner"}}}

	// application
	s_impl.Permissions[18] = map[string]interface{}{"action": "/resource.ResourceService/UpdateApplication", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "owner"}}}
	s_impl.Permissions[19] = map[string]interface{}{"action": "/resource.ResourceService/DeleteApplication", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "owner"}}}
	s_impl.Permissions[20] = map[string]interface{}{"action": "/resource.ResourceService/AddApplicationActions", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "owner"}}}
	s_impl.Permissions[21] = map[string]interface{}{"action": "/resource.ResourceService/RemoveApplicationAction", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "owner"}}}
	s_impl.Permissions[22] = map[string]interface{}{"action": "/resource.ResourceService/RemoveApplicationsAction", "resources": []interface{}{map[string]interface{}{"index": 0, "permission": "owner"}}}

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

	if s_impl.SessionTimeout == 0 {
		s_impl.SessionTimeout = 15 // set back 15 minumtes (default value.)
	}

	// Register the resource services
	resourcepb.RegisterResourceServiceServer(s_impl.grpcServer, s_impl)
	reflection.Register(s_impl.grpcServer)

	go func() {
		// Can do anything
		s_impl.createRole("admin", "admin", "sa", []string{})

		//  Register the guest role
		s_impl.createRole("guest", "guest", "sa", []string{
			"/admin.AdminService/RunCmd",
			"/admin.AdminService/SaveConfig",
			"/conversation.ConversationService/AcceptInvitation",
			"/conversation.ConversationService/Connect",
			"/conversation.ConversationService/CreateConversation",
			"/conversation.ConversationService/DeclineInvitation",
			"/conversation.ConversationService/DeleteConversation",
			"/conversation.ConversationService/DeleteMessage",
			"/conversation.ConversationService/Disconnect",
			"/conversation.ConversationService/DislikeMessage",
			"/conversation.ConversationService/FindConversations",
			"/conversation.ConversationService/FindMessages",
			"/conversation.ConversationService/GetConversations",
			"/conversation.ConversationService/GetReceivedInvitations",
			"/conversation.ConversationService/GetSentInvitations",
			"/conversation.ConversationService/JoinConversation",
			"/conversation.ConversationService/KickoutFromConversation",
			"/conversation.ConversationService/LeaveConversation",
			"/conversation.ConversationService/LikeMessage",
			"/conversation.ConversationService/RevokeInvitation",
			"/conversation.ConversationService/SendInvitation",
			"/conversation.ConversationService/SendMessage",
			"/conversation.ConversationService/SetMessageRead",
			"/file.FileService/Copy",
			"/file.FileService/CreateAchive",
			"/file.FileService/CreateDir",
			"/file.FileService/DeleteDir",
			"/file.FileService/DeleteFile",
			"/file.FileService/FileUploadHandler",
			"/file.FileService/GetFileInfo",
			"/file.FileService/GetThumbnails",
			"/file.FileService/Move",
			"/file.FileService/ReadDir",
			"/file.FileService/ReadFile",
			"/file.FileService/Rename",
			"/file.FileService/SaveFile",
			"/file.FileService/WriteExcelFile",
			"/file.FileService/CreateAchive",
			"/log.LogService/GetLog",
			"/log.LogService/Log",
			"/persistence.PersistenceService/Count",
			"/persistence.PersistenceService/CreateConnection",
			"/persistence.PersistenceService/Delete",
			"/persistence.PersistenceService/DeleteOne",
			"/persistence.PersistenceService/Find",
			"/persistence.PersistenceService/FindOne",
			"/persistence.PersistenceService/InsertOne",
			"/persistence.PersistenceService/ReplaceOne",
			"/persistence.PersistenceService/UpdateOne",
			"/rbac.RbacService/DeleteResourcePermission",
			"/rbac.RbacService/DeleteResourcePermissions",
			"/rbac.RbacService/DeleteSubjectShare",
			"/rbac.RbacService/GetResourcePermissions",
			"/rbac.RbacService/GetSharedResource",
			"/rbac.RbacService/SetActionResourcesPermissions",
			"/rbac.RbacService/SetResourcePermission",
			"/rbac.RbacService/SetResourcePermissions",
			"/resource.ResourceService/GetGroups",
			"/resource.ResourceService/GetApplications",
			"/resource.ResourceService/GetOrganizations",
			"/resource.ResourceService/GetRoles",
			"/resource.ResourceService/GetPeers",
			"/resource.ResourceService/GetAccounts",
			"/resource.ResourceService/SetAccountContact",
			"/resource.ResourceService/GetNotifications",
			"/resource.ResourceService/CreateNotification",
			"/resource.ResourceService/DeleteNotification",
		})

		// Here I will create user directories if their not already exist...
		s_impl.CreateAccountDir()
	}()

	// Start the service.
	s_impl.StartService()

}
