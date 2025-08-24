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

	Utility "github.com/davecourtois/!utility"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/event/event_client"
	"github.com/globulario/services/golang/globular_client"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/interceptors"
	"github.com/globulario/services/golang/log/log_client"
	"github.com/globulario/services/golang/log/logpb"
	"github.com/globulario/services/golang/persistence/persistence_store"
	"github.com/globulario/services/golang/rbac/rbac_client"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/resource/resource_client"
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

	// srv-signed X.509 public keys for distribution
	CertFile string

	// a private RSA key to sign and authenticate the public key
	KeyFile string

	// a private RSA key to sign and authenticate the public key
	CertAuthorityTrust string

	Permissions []interface{} // contains the action permission for the services.

	Dependencies []string // The list of services needed by this services.

	// The backend infos.
	Backend_type               string
	Backend_address            string
	Backend_port               int64
	Backend_user               string
	Backend_password           string
	Backend_replication_factor int64
	DataPath                   string

	// The session time out.
	SessionTimeout int

	// Data store where account, role ect are keep...
	store   persistence_store.Store
	isReady bool

	// The grpc server.
	grpcServer *grpc.Server
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

// The current service state
func (srv *server) GetState() string {
	return srv.State
}

func (srv *server) SetState(state string) {
	srv.State = state
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
func (srv *server) publishEvent(evt string, data []byte, address string) error {

	client, err := GetEventClient(address)
	if err != nil {
		return err
	}

	err = client.Publish(evt, data)
	return err
}

// Public event to a peer other than the default one...
func (srv *server) publishRemoteEvent(address, evt string, data []byte) error {

	client, err := GetEventClient(address)
	if err != nil {
		return err
	}

	return client.Publish(evt, data)
}

// ///////////////////////////////////// return the peers infos from a given peer /////////////////////////////
func (srv *server) getPeerInfos(address, mac string) (*resourcepb.Peer, error) {

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
func (srv *server) getPeerPublicKey(address, mac string) (string, error) {

	if len(mac) == 0 {
		mac = srv.Mac
	}

	if mac == srv.Mac {
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
func (srv *server) setLocalHosts(peer *resourcepb.Peer) error {

	// Finaly I will set the domain in the hosts file...
	hosts, err := txeh.NewHostsDefault()
	address := peer.GetHostname()
	if peer.GetDomain() != "localhost" {
		address = address + "." + peer.GetDomain()
	}

	if err != nil {
		fmt.Println("fail to set host entry ", address, " with error ", err)
		return err
	}

	if peer.ExternalIpAddress == Utility.MyIP() {
		hosts.AddHost(peer.LocalIpAddress, address)
	}

	err = hosts.Save()
	if err != nil {
		fmt.Println("fail to save hosts ", peer.LocalIpAddress, address, " with error ", err)
		return err
	}

	return nil
}

/** Set the host if it's part of the same local network. */
func (srv *server) removeFromLocalHosts(peer *resourcepb.Peer) error {
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

func (srv *server) addResourceOwner(path, resourceType, subject string, subjectType rbacpb.SubjectType) error {
	rbac_client_, err := GetRbacClient(srv.Address)
	if err != nil {
		return err
	}

	err = rbac_client_.AddResourceOwner(path, resourceType, subject, subjectType)
	return err
}

func (srv *server) deleteResourcePermissions(path string) error {
	rbac_client_, err := GetRbacClient(srv.Address)
	if err != nil {
		return err
	}
	return rbac_client_.DeleteResourcePermissions(path)
}

func (srv *server) deleteAllAccess(suject string, subjectType rbacpb.SubjectType) error {
	rbac_client_, err := GetRbacClient(srv.Address)
	if err != nil {
		return err
	}
	return rbac_client_.DeleteAllAccess(suject, subjectType)
}

func (srv *server) SetAccountAllocatedSpace(accountId string, space uint64) error {
	rbac_client_, err := GetRbacClient(srv.Address)
	if err != nil {
		return err
	}
	return rbac_client_.SetAccountAllocatedSpace(accountId, space)
}

//////////////////////////////////////// Resource Functions ///////////////////////////////////////////////

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

////////////////////////////////// Resource functions ///////////////////////////////////////////////

// ** MONGO backend, it must reside on the same server as the resource server (at this time...)

/**
 * Connection to mongo db local store.
 */
func (srv *server) getPersistenceStore() (persistence_store.Store, error) {
	// That service made user of persistence service.

	if srv.store == nil {

		var options_str = ""

		if srv.Backend_type == "SCYLLA" {
			srv.store = new(persistence_store.ScyllaStore)
			options := map[string]interface{}{"keyspace": "local_resource", "replication_factor": srv.Backend_replication_factor, "hosts": []string{srv.Backend_address}, "port": srv.Backend_port}
			options_, _ := Utility.ToJson(options)
			options_str = string(options_)
		} else if srv.Backend_type == "MONGO" {

			process, err := Utility.GetProcessIdsByName("mongod")
			if err != nil {
				fmt.Println("fail to get process id for mongod with error ", err)
				return nil, err
			}

			if len(process) == 0 {
				fmt.Println("mongod is not running on this server, please start it before starting the resource server")
				return nil, errors.New("mongod is not running on this server, please start it before starting the resource server")
			}

			srv.store = new(persistence_store.MongoStore)

		} else if srv.Backend_type == "SQL" {

			srv.store = new(persistence_store.SqlStore)
			options := map[string]interface{}{"driver": "sqlite3", "charset": "utf8", "path": srv.DataPath + "/sql-data"}
			options_, _ := Utility.ToJson(options)
			options_str = string(options_)

		} else {
			return nil, errors.New("unknown backend type " + srv.Backend_type)
		}

		// Connect to the store.
		err := srv.store.Connect("local_resource", srv.Backend_address, int32(srv.Backend_port), srv.Backend_user, srv.Backend_password, "local_resource", 5000, options_str)
		if err != nil {

			fmt.Println("fail to connect to store with error ", err)
			os.Exit(1)
		}

		err = srv.store.Ping(context.Background(), "local_resource")
		if err != nil {
			fmt.Println("fail to reach store with error ", err)
			return nil, err
		}

		srv.isReady = true

		fmt.Println("store ", srv.Backend_address+":"+Utility.ToString(srv.Backend_port), "is runing and ready to be used.")

		if srv.Backend_type == "SQL" {
			// Create tables if not already exist.
			err := srv.store.(*persistence_store.SqlStore).CreateTable(context.Background(), "local_resource", "local_resource", "Accounts", []string{"name TEXT", "email TEXT", "domain TEXT", "password TEXT", "refresh_token TEXT"})
			if err != nil {
				fmt.Println("fail to create table Accounts with error ", err)
			}

			err = srv.store.(*persistence_store.SqlStore).CreateTable(context.Background(), "local_resource", "local_resource", "Applications", []string{"name TEXT", "domain TEXT", "description TEXT", "icon TEXT", "alias TEXT", "password TEXT", "store TEXT", "last_deployed INTEGER", "path TEXT", "version TEXT", "publisherid TEXT", "creation_date INTEGER"})
			if err != nil {
				fmt.Println("fail to create table Organizations with error ", err)
			}

			// Create organizations table.
			err = srv.store.(*persistence_store.SqlStore).CreateTable(context.Background(), "local_resource", "local_resource", "Organizations", []string{"name TEXT", "domain TEXT", "description TEXT", "icon TEXT", "email TEXT"})
			if err != nil {
				fmt.Println("fail to create table Organizations with error ", err)
			}

			// Create roles table.
			err = srv.store.(*persistence_store.SqlStore).CreateTable(context.Background(), "local_resource", "local_resource", "Roles", []string{"name TEXT", "domain TEXT", "description TEXT"})
			if err != nil {
				fmt.Println("fail to create table Roles with error ", err)
			}

			// Create groups table.
			err = srv.store.(*persistence_store.SqlStore).CreateTable(context.Background(), "local_resource", "local_resource", "Groups", []string{"name TEXT", "domain TEXT", "description TEXT"})
			if err != nil {
				fmt.Println("fail to create table Groups with error ", err)
			}

			// Create peers table.
			err = srv.store.(*persistence_store.SqlStore).CreateTable(context.Background(), "local_resource", "local_resource", "Peers", []string{"domain TEXT", "hostname TEXT", "external_ip_address TEXT", "local_ip_address TEXT", "mac TEXT", "protocol TEXT", "state INTEGER", "portHttp INTEGER", "portHttps INTEGER"})
			if err != nil {
				fmt.Println("fail to create table Peers with error ", err)
			}

			// Create the sessions table.
			err = srv.store.(*persistence_store.SqlStore).CreateTable(context.Background(), "local_resource", "local_resource", "Sessions", []string{"accountId TEXT", "domain TEXT", "state INTEGER", "last_state_time INTEGER", "expire_at INTEGER"})
			if err != nil {
				fmt.Println("fail to create table Sessions with error ", err)
			}

			// Create the notifications table.
			err = srv.store.(*persistence_store.SqlStore).CreateTable(context.Background(), "local_resource", "local_resource", "Notifications", []string{"date REAL", "domain TEXT", "message TEXT", "recipient TEXT", "sender TEXT", "mac TEXT", "notification_type INTEGER"})
			if err != nil {
				fmt.Println("fail to create table Notifications with error ", err)
			}
		} else if srv.Backend_type == "SCYLLA" {
			// Create tables if not already exist.
			err := srv.store.(*persistence_store.ScyllaStore).CreateTable(context.Background(), "local_resource", "local_resource", "Accounts", []string{"name TEXT", "email TEXT", "domain TEXT", "password TEXT"})
			if err != nil {
				fmt.Println("fail to create table Accounts with error ", err)
			}

			err = srv.store.(*persistence_store.ScyllaStore).CreateTable(context.Background(), "local_resource", "local_resource", "Applications", []string{"name TEXT", "domain TEXT", "description TEXT", "icon TEXT", "alias TEXT", "password TEXT", "store TEXT", "last_deployed INT", "path TEXT", "version TEXT", "publisherid TEXT", "creation_date INT"})
			if err != nil {
				fmt.Println("fail to create table Organizations with error ", err)
			}

			// Create organizations table.
			err = srv.store.(*persistence_store.ScyllaStore).CreateTable(context.Background(), "local_resource", "local_resource", "Organizations", []string{"name TEXT", "domain TEXT", "description TEXT", "icon TEXT", "email TEXT"})
			if err != nil {
				fmt.Println("fail to create table Organizations with error ", err)
			}

			// Create roles table.
			err = srv.store.(*persistence_store.ScyllaStore).CreateTable(context.Background(), "local_resource", "local_resource", "Roles", []string{"name TEXT", "domain TEXT", "description TEXT"})
			if err != nil {
				fmt.Println("fail to create table Roles with error ", err)
			}

			// Create groups table.
			err = srv.store.(*persistence_store.ScyllaStore).CreateTable(context.Background(), "local_resource", "local_resource", "Groups", []string{"name TEXT", "domain TEXT", "description TEXT"})
			if err != nil {
				fmt.Println("fail to create table Groups with error ", err)
			}

			// Create peers table.
			err = srv.store.(*persistence_store.ScyllaStore).CreateTable(context.Background(), "local_resource", "local_resource", "Peers", []string{"domain TEXT", "hostname TEXT", "external_ip_address TEXT", "local_ip_address TEXT", "mac TEXT", "protocol TEXT", "state INT", "portHttp INT", "portHttps INT"})
			if err != nil {
				fmt.Println("fail to create table Peers with error ", err)
			}

			// Create the sessions table.
			err = srv.store.(*persistence_store.ScyllaStore).CreateTable(context.Background(), "local_resource", "local_resource", "Sessions", []string{"accountId TEXT", "domain TEXT", "state INT", "last_state_time BIGINT", "expire_at BIGINT"})
			if err != nil {
				fmt.Println("fail to create table Sessions with error ", err)
			}

			// Create the notifications table.
			err = srv.store.(*persistence_store.ScyllaStore).CreateTable(context.Background(), "local_resource", "local_resource", "Notifications", []string{"date DOUBLE", "domain TEXT", "message TEXT", "recipient TEXT", "sender TEXT", "mac TEXT", "notification_type INT"})
			if err != nil {
				fmt.Println("fail to create table Notifications with error ", err)
			}
		}
	} else if !srv.isReady {
		nbTry := 100
		for i := 0; i < nbTry; i++ {
			time.Sleep(100 * time.Millisecond)
			if srv.isReady {
				break
			}
		}
	}

	return srv.store, nil
}

/**
 *  hashPassword return the bcrypt hash of the password.
 */
func (srv *server) hashPassword(password string) (string, error) {
	haspassword, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("failed to hash password: %w", err)
	}
	return string(haspassword), nil
}

/**
 * Return the hash password.
 */
func (srv *server) validatePassword(password string, hash string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
}

/**
 * Register an Account.
 */
func (srv *server) registerAccount(domain, id, name, email, password, refresh_token, first_name, last_name, middle_name, profile_picture string, organizations []string, roles []string, groups []string) error {

	localDomain, err := config.GetDomain()
	if err != nil {
		return err
	}

	if domain != localDomain {
		return errors.New("you cant register account with domain " + domain + " on domain " + localDomain)
	}

	// That service made user of persistence service.
	p, err := srv.getPersistenceStore()
	if err != nil {
		return err
	}

	// Check if the account already exist.
	q := `{"_id":"` + id + `"}`

	// first of all the Persistence service must be active.
	count, _ := p.Count(context.Background(), "local_resource", "local_resource", "Accounts", q, "")

	// one account already exist for the name we look for.
	if count == 1 && len(refresh_token) == 0 {
		return errors.New("account with name " + name + " already exist!")
	} else if count == 1 && len(refresh_token) != 0 {
		// so here I will update the account with the refresh token.
		err = p.UpdateOne(context.Background(), "local_resource", "local_resource", "Accounts", q, `{"$set":{"refresh_token":"`+refresh_token+`"}}`, "")
		if err != nil {
			fmt.Println("fail to update account with error ", err)
			return err
		}
		fmt.Println("account with name ", name, " has been updated with refresh token")
		return nil
	}

	// set the account object and set it basic roles.
	account := make(map[string]interface{})
	account["_id"] = id
	account["name"] = name
	account["email"] = email
	account["domain"] = domain

	if len(refresh_token) == 0 {
		account["password"], err = srv.hashPassword(password) // hide the password...
		if err != nil {
			fmt.Println("fail to hash password with error ", err)
			return err
		}

	} else {
		account["refresh_token"] = refresh_token
		account["password"] = ""
	}

	// List of aggregation.
	account["roles"] = make([]interface{}, 0)
	account["groups"] = make([]interface{}, 0)
	account["organizations"] = make([]interface{}, 0)
	account["typeName"] = "Account"

	// append guest role if not already exist.
	if !Utility.Contains(roles, "guest@"+localDomain) {
		roles = append(roles, "guest@"+localDomain)
	}

	// Here I will insert the account in the database.
	_, err = p.InsertOne(context.Background(), "local_resource", "local_resource", "Accounts", account, "")
	if err != nil {
		fmt.Printf("fail to create account %s with error %s", name, err.Error())
		return err
	}

	// replace @ and . by _  * active directory
	name = strings.ReplaceAll(strings.ReplaceAll(name, "@", "_"), ".", "_")

	// Each account will have their own database and a use that can read and write

	// Organizations
	for i := 0; i < len(organizations); i++ {
		if !strings.Contains(organizations[i], "@") {
			organizations[i] = organizations[i] + "@" + localDomain
		}

		srv.createCrossReferences(organizations[i], "Organizations", "accounts", id, "Accounts", "organizations")
	}

	// Roles
	for i := 0; i < len(roles); i++ {
		if !strings.Contains(roles[i], "@") {
			roles[i] = roles[i] + "@" + localDomain
		}
		srv.createCrossReferences(roles[i], "Roles", "members", id, "Accounts", "roles")
	}

	// Groups
	for i := 0; i < len(groups); i++ {
		if !strings.Contains(groups[i], "@") {
			groups[i] = groups[i] + "@" + localDomain
		}
		srv.createCrossReferences(groups[i], "Groups", "members", id, "Accounts", "groups")
	}

	// Create the user file directory.
	path := "/users/" + id + "@" + localDomain
	Utility.CreateDirIfNotExist(config.GetDataDir() + "/files" + path)
	err = srv.addResourceOwner(path, "file", id+"@"+localDomain, rbacpb.SubjectType_ACCOUNT)
	if err != nil {
		fmt.Println("fail to add resource owner with error ", err)
	}

	// I will execute the sript with the admin function.
	// TODO implement the admin function for scylla and sql.
	if p.GetStoreType() == "MONGO" {
		createUserScript := fmt.Sprintf("db=db.getSiblingDB('%s_db');db.createCollection('user_data');db=db.getSiblingDB('admin');db.createUser({user: '%s', pwd: '%s',roles: [{ role: 'dbOwner', db: '%s_db' }]});", name, name, password, name)
		err = p.RunAdminCmd(context.Background(), "local_resource", srv.Backend_user, srv.Backend_password, createUserScript)
		if err != nil {
			return err
		}
	} else if p.GetStoreType() == "SCYLLA" {

		createUserScript := fmt.Sprintf("CREATE KEYSPACE IF NOT EXISTS %s_db WITH REPLICATION = { 'class':'SimpleStrategy', 'replication_factor': %d }; CREATE TABLE %s_db.user_data (id text PRIMARY KEY, first_name text, last_name text, middle_name text, email text, profile_picture text); INSERT INTO %s_db.user_data (id, email, first_name, last_name, middle_name, profile_picture) VALUES ('%s', '%s', '%s', '%s', '%s', '%s');", name, srv.Backend_replication_factor, name, name, id, email, first_name, last_name, middle_name, profile_picture)
		err = p.RunAdminCmd(context.Background(), "local_resource", srv.Backend_user, srv.Backend_password, createUserScript)
		if err != nil {
			return err
		}
	} else if p.GetStoreType() == "SQL" {
		// Create the user_data table in the user database
		err = p.(*persistence_store.SqlStore).CreateTable(context.Background(), "local_resource", name+"_db", "user_data", []string{"first_name TEXT", "last_name TEXT", "middle_name TEXT", "email TEXT", "profile_picture TEXT"})
		if err != nil {
			return err
		}

	}

	// Here I will set user data in the database.
	data := make(map[string]interface{}, 0)
	data["_id"] = id
	data["first_name"] = first_name
	data["last_name"] = last_name
	data["middle_name"] = ""
	data["email"] = email
	data["profile_picture"] = profile_picture

	_, err = p.InsertOne(context.Background(), "local_resource", name+"_db", "user_data", data, "")

	// Now I will allocate the new account disk space.
	srv.SetAccountAllocatedSpace(id, 0)

	return err
}

func (srv *server) deleteReference(p persistence_store.Store, refId, targetId, targetField, targetCollection string) error {

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

			err = client.DeleteReference(refId, targetId, targetField, targetCollection)
			if err != nil {
				return err
			}

			return nil
		}
	}

	if strings.Contains(refId, "@") {
		domain := strings.Split(refId, "@")[1]
		refId = strings.Split(refId, "@")[0]

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

			err = client.DeleteReference(refId, targetId, targetField, targetCollection)
			if err != nil {
				return err
			}

			return nil
		}
	}

	q := `{"_id":"` + targetId + `"}`
	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", targetCollection, q, ``)
	if err != nil {
		return err
	}

	target := values.(map[string]interface{})

	if target[targetField] == nil {
		return errors.New("No field named " + targetField + " was found in object with id " + targetId + "!")
	}

	var references []interface{}
	switch target[targetField].(type) {
	case primitive.A:
		references = []interface{}(target[targetField].(primitive.A))
	case []interface{}:
		references = target[targetField].([]interface{})
	}

	references_ := make([]interface{}, 0)
	for j := 0; j < len(references); j++ {
		if references[j].(map[string]interface{})["$id"] != refId {
			references_ = append(references_, references[j])
		}
	}

	target[targetField] = references_

	jsonStr := serialyseObject(target)

	err = p.ReplaceOne(context.Background(), "local_resource", "local_resource", targetCollection, q, jsonStr, ``)
	if err != nil {
		return err
	}

	return nil
}

func (srv *server) getRemoteAccount(id string, domain string) (*resourcepb.Account, error) {
	fmt.Println("get account ", id, "from", domain)
	client, err := GetResourceClient(domain)
	if err != nil {
		return nil, err
	}

	return client.GetAccount(id)
}

func (srv *server) createReference(p persistence_store.Store, id, sourceCollection, field, targetId, targetCollection string) error {

	var err error
	var source map[string]interface{}

	// the must contain the domain in the id.
	if !strings.Contains(targetId, "@") {
		return errors.New("target id must be a valid id with domain")
	}

	// Here I will check if the target id is on the same domain as the source id.
	if strings.Split(targetId, "@")[1] != srv.Domain {

		// TODO create a remote reference... (not implemented yet)
		return errors.New("target id must be on the same domain as the source id")
	}

	// TODO see how to handle the case where the target id is not on the same domain as the source id.
	targetId = strings.Split(targetId, "@")[0] // remove the domain from the id.

	if strings.Contains(id, "@") {
		domain := strings.Split(id, "@")[1]
		id = strings.Split(id, "@")[0]

		// if the domain is not the same as the local domain then I will redirect the call to the remote resource srv.
		if srv.Domain != domain {
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

	// I will first check if the reference already exist.
	q := `{"_id":"` + id + `"}`

	// Get the source object.
	source_values, err := p.FindOne(context.Background(), "local_resource", "local_resource", sourceCollection, q, ``)
	if err != nil {
		return errors.New("fail to find object with id " + id + " in collection " + sourceCollection + " at address " + srv.Address + " err: " + err.Error())
	}

	// append the account.
	source = source_values.(map[string]interface{})
	// be sure that the target id is a valid id.
	if source["_id"] == nil {
		return errors.New("No _id field was found in object with id " + id + "!")
	}

	// append the domain to the id.
	if p.GetStoreType() == "MONGO" {
		var references []interface{}
		if source[field] != nil {
			switch source[field].(type) {
			case primitive.A:
				references = []interface{}(source[field].(primitive.A))
			case []interface{}:
				references = source[field].([]interface{})
			}
		}

		for j := 0; j < len(references); j++ {
			if references[j].(map[string]interface{})["$id"] == targetId {
				return errors.New(" named " + targetId + " aleready exist in  " + field + "!")
			}
		}

		source[field] = append(references, map[string]interface{}{"$ref": targetCollection, "$id": targetId, "$db": "local_resource"})
		jsonStr := serialyseObject(source)

		err = p.ReplaceOne(context.Background(), "local_resource", "local_resource", sourceCollection, q, jsonStr, ``)
		if err != nil {
			return err
		}
	} else if p.GetStoreType() == "SQL" || p.GetStoreType() == "SCYLLA" {

		// I will create the table if not already exist.
		if p.GetStoreType() == "SQL" {
			createTable := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS `+sourceCollection+`_`+field+` (source_id TEXT, target_id TEXT, FOREIGN KEY (source_id) REFERENCES %s(_id) ON DELETE CASCADE, FOREIGN KEY (target_id) REFERENCES %s(_id) ON DELETE CASCADE)`, sourceCollection, targetCollection)
			_, err := p.(*persistence_store.SqlStore).ExecContext("local_resource", "local_resource", createTable, nil, 0)
			if err != nil {
				return err
			}

		} else if p.GetStoreType() == "SCYLLA" {
			// the foreign key is not supported by SCYLLA.
			createTable := fmt.Sprintf(`CREATE TABLE IF NOT EXISTS ` + sourceCollection + `_` + field + ` (source_id TEXT, target_id TEXT, PRIMARY KEY (source_id, target_id))`)
			session := p.(*persistence_store.ScyllaStore).GetSession("local_resource")

			if session == nil {
				return errors.New("fail to get session for local_resource")
			}

			err = session.Query(createTable).Exec()
			if err != nil {
				return err
			}
		}

		// Here I will insert the reference in the database.

		// I will first check if the reference already exist.
		q = `SELECT * FROM ` + sourceCollection + `_` + field + ` WHERE source_id='` + Utility.ToString(source["_id"]) + `' AND target_id='` + targetId + `'`
		if p.GetStoreType() == "SCYLLA" {
			q += ` ALLOW FILTERING`
		}

		count, _ := p.Count(context.Background(), "local_resource", "local_resource", sourceCollection+`_`+field, q, ``)

		if count == 0 {
			q = `INSERT INTO ` + sourceCollection + `_` + field + ` (source_id, target_id) VALUES (?,?)`

			if p.GetStoreType() == "SCYLLA" {

				session := p.(*persistence_store.ScyllaStore).GetSession("local_resource")
				if session == nil {
					return errors.New("fail to get session for local_resource")
				}

				err = session.Query(q, source["_id"], targetId).Exec()
				if err != nil {
					return err
				}

			} else if p.GetStoreType() == "SQL" {
				_, err = p.(*persistence_store.SqlStore).ExecContext("local_resource", "local_resource", q, []interface{}{source["_id"], targetId}, 0)
				if err != nil {
					return err
				}
			}
		}
	}

	return nil
}

func (srv *server) createCrossReferences(sourceId, sourceCollection, sourceField, targetId, targetCollection, targetField string) error {
	p, err := srv.getPersistenceStore()
	if err != nil {
		return err
	}

	err = srv.createReference(p, targetId, targetCollection, targetField, sourceId, sourceCollection)
	if err != nil {
		return err
	}

	err = srv.createReference(p, sourceId, sourceCollection, sourceField, targetId, targetCollection)

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

func (srv *server) createGroup(id, name, owner, description string, members []string) error {

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
	p, err := srv.getPersistenceStore()
	if err != nil {
		return err
	}

	q := `{"_id":"` + id + `"}`

	// Here I will first look if a peer with a same name already exist on the
	// resources...
	count, _ := p.Count(context.Background(), "local_resource", "local_resource", "Groups", q, "")
	if count > 0 {
		return errors.New("Group with name '" + id + "' already exist!")
	}

	// No authorization exist for that peer I will insert it.
	// Here will create the new peer.
	g := make(map[string]interface{}, 0)
	g["_id"] = id
	g["name"] = name
	g["description"] = description
	g["domain"] = localDomain
	g["typeName"] = "Group"

	_, err = p.InsertOne(context.Background(), "local_resource", "local_resource", "Groups", g, "")
	if err != nil {
		return err
	}

	// Create references.
	for i := 0; i < len(members); i++ {

		if !strings.Contains(members[i], "@") {
			members[i] = members[i] + "@" + localDomain
		}

		err := srv.createCrossReferences(id, "Groups", "members", members[i], "Accounts", "groups")
		if err != nil {
			return err
		}
	}

	// Now create the resource permission.
	srv.addResourceOwner(id+"@"+srv.Domain, "group", owner, rbacpb.SubjectType_ACCOUNT)
	fmt.Println("group ", id, "was create with owner ", owner)
	return nil
}

/**
 * Create account dir for all account in the database if not already exist.
 */
func (srv *server) CreateAccountDir() error {
	p, err := srv.getPersistenceStore()
	if err != nil {
		return err
	}

	q := `{}`

	// Make sure some account exist on the server.
	count, _ := p.Count(context.Background(), "local_resource", "local_resource", "Accounts", q, "")
	if count == 0 {
		return errors.New("no account exist in the database")
	}

	accounts, err := p.Find(context.Background(), "local_resource", "local_resource", "Accounts", q, "")
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
			srv.addResourceOwner(path, "file", id+"@"+domain, rbacpb.SubjectType_ACCOUNT)
		}
	}

	return nil
}

func (srv *server) createRole(id, name, owner string, description string, actions []string) error {
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
	p, err := srv.getPersistenceStore()
	if err != nil {
		return err
	}

	var q string
	q = `{"_id":"` + id + `"}`

	_, err = p.FindOne(context.Background(), "local_resource", "local_resource", "Roles", q, ``)
	if err == nil {
		return errors.New("role named " + name + " already exist!")
	}

	// Here will create the new role.
	role := make(map[string]interface{})
	role["_id"] = id
	role["name"] = name
	role["actions"] = actions
	role["domain"] = localDomain
	role["description"] = description
	role["typeName"] = "Role"

	_, err = p.InsertOne(context.Background(), "local_resource", "local_resource", "Roles", role, "")
	if err != nil {
		return err
	}

	if name != "guest" && name != "admin" {
		srv.addResourceOwner(id+"@"+srv.Domain, "role", owner, rbacpb.SubjectType_ACCOUNT)
	}

	return nil
}

/**
 * Delete application Data from the backend.
 */
func (srv *server) deleteApplication(applicationId string) error {

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
	p, err := srv.getPersistenceStore()
	if err != nil {
		return err
	}

	q := `{"_id":"` + applicationId + `"}`

	values, err := p.FindOne(context.Background(), "local_resource", "local_resource", "Applications", q, ``)
	if err != nil {
		return err
	}

	// I will remove all the access to the application, before removing the application.
	srv.deleteAllAccess(applicationId, rbacpb.SubjectType_APPLICATION)
	srv.deleteResourcePermissions(applicationId)

	application := values.(map[string]interface{})

	// I will remove it from organization...
	if application["organizations"] != nil {

		var organizations []interface{}
		switch values.(map[string]interface{})["organizations"].(type) {
		case primitive.A:
			organizations = []interface{}(values.(map[string]interface{})["organizations"].(primitive.A))
		case []interface{}:
			organizations = values.(map[string]interface{})["organizations"].([]interface{})
		}
		for i := 0; i < len(organizations); i++ {
			organizationId := organizations[i].(map[string]interface{})["$id"].(string)
			srv.deleteReference(p, applicationId, organizationId, "applications", "Organizations")
		}
	}

	// I will remove the directory.
	err = os.RemoveAll(application["path"].(string))
	if err != nil {
		return err
	}

	var name string
	if application["name"] != nil {
		name = application["name"].(string)
	} else if application["Name"] != nil {
		name = application["Name"].(string)
	}

	// Set the database name.
	db := name
	db = strings.ReplaceAll(db, ".", "_")
	db = strings.ReplaceAll(db, "@", "_")
	db = strings.ReplaceAll(db, "-", "_")
	db = strings.ReplaceAll(db, " ", "_")
	db += "_db"

	// Now I will remove the database create for the application.
	err = p.DeleteDatabase(context.Background(), "local_resource", db)
	if err != nil {
		return err
	}

	// Finaly I will remove the entry in  the table.
	err = p.DeleteOne(context.Background(), "local_resource", "local_resource", "Applications", q, "")
	if err != nil {
		return err
	}

	// Drop the application user.
	// Here I will drop the db user.
	var dropUserScript string
	if p.GetStoreType() == "MONGO" {
		dropUserScript = fmt.Sprintf(
			`db=db.getSiblingDB('admin');db.dropUser('%s', {w: 'majority', wtimeout: 4000})`,
			applicationId)
	} else if p.GetStoreType() == "SCYLLA" {
		dropUserScript = fmt.Sprintf("DROP KEYSPACE IF EXISTS %s;", applicationId)
	} else if p.GetStoreType() == "SQL" {
		dropUserScript = fmt.Sprintf("DROP DATABASE IF EXISTS %s;", applicationId)
	} else {
		return errors.New("unknown backend type " + p.GetStoreType())
	}

	// I will execute the sript with the admin function.
	// TODO implement drop user for scylla and sql
	if p.GetStoreType() == "MONGO" {
		err = p.RunAdminCmd(context.Background(), "local_resource", srv.Backend_user, srv.Backend_password, dropUserScript)
		if err != nil {
			return err
		}
	}

	// set back the domain part
	applicationId = application["_id"].(string) + "@" + application["domain"].(string)

	srv.publishEvent("delete_application_"+applicationId+"_evt", []byte{}, application["domain"].(string))
	srv.publishEvent("delete_application_evt", []byte(applicationId), application["domain"].(string))

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
	s_impl.PublisherId = "localhost"
	s_impl.Description = "Resource manager service. Resources are Group, Account, Role, Organization and Peer."
	s_impl.Keywords = []string{"Resource"}
	s_impl.Repositories = make([]string, 0)
	s_impl.Discoveries = make([]string, 0)
	s_impl.Dependencies = []string{"authentication.AuthenticationService", "log.LogService", "persistence.PersistenceService"}
	s_impl.Permissions = make([]interface{}, 23)
	s_impl.AllowAllOrigins = allow_all_origins
	s_impl.AllowedOrigins = allowed_origins
	s_impl.Process = -1
	s_impl.ProxyProcess = -1
	s_impl.SessionTimeout = 15
	s_impl.KeepAlive = true
	s_impl.KeepUpToDate = true

	// register new client creator.
	Utility.RegisterFunction("NewResourceService_Client", resource_client.NewResourceService_Client)

	// Backend informations.
	s_impl.Backend_type = "SQL" // use SQL as default backend.
	s_impl.Backend_address = s_impl.Address
	s_impl.Backend_replication_factor = 1
	s_impl.Backend_port = 27018 // Here I will use the port beside the default one in case MONGO is already exist
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

	if s_impl.Address == "" {
		s_impl.Address, _ = config.GetAddress()
	}

	if s_impl.Backend_address == "localhost" {
		s_impl.Backend_address = s_impl.Address
	}

	if s_impl.SessionTimeout == 0 {
		s_impl.SessionTimeout = 15 // set back 15 minumtes (default value.)
	}

	// Register the resource services
	resourcepb.RegisterResourceServiceServer(s_impl.grpcServer, s_impl)
	reflection.Register(s_impl.grpcServer)

	go func() {

		// Can do anything
		s_impl.createRole("admin", "admin", "sa", "the super admin role", []string{})

		//  Register the guest role
		s_impl.createRole("guest", "guest", "sa", "the guest role, every user must a least have this role", []string{
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
			"/title.TitleService/GetPublisherById",
			"/title.TitleService/CreatePerson",
			"/title.TitleService/GetPersonById",
			"/title.TitleService/GetAudioById",
			"/title.TitleService/GetAlbum",
			"/title.TitleService/GetVideoById",
			"/title.TitleService/GetFileTitles",
			"/title.TitleService/GetFileAudios",
			"/title.TitleService/GetTitleFiles",
			"/title.TitleService/SearchTitles",
			"/title.TitleService/SearchPersons",
		})

		// Here I will create user directories if their not already exist...
		s_impl.CreateAccountDir()

	}()

	// Start the service.
	s_impl.StartService()

}
