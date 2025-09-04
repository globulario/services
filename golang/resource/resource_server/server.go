// Package main refactors the original server.go to match the clean structure
// you liked (Echo-style):
//   - slog for structured logs
//   - --describe and --health CLI paths that do not touch etcd before printing
//   - lean main() that wires Globular lifecycle (Init/Save/Start/Stop)
//   - clear getters/setters to satisfy Globular's service contract
//   - concise, readable errors
package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	// Gate calls into config/etcd until after we handle --describe/--health
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/event/event_client"
	"github.com/globulario/services/golang/globular_client"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/persistence/persistence_client"
	"github.com/globulario/services/golang/persistence/persistence_store"
	"github.com/globulario/services/golang/rbac/rbac_client"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/security"
	Utility "github.com/globulario/utility"
	"github.com/txn2/txeh"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	// Keep the original service proto import so we can register the server type
	// (handlers are defined elsewhere in this package).
	"github.com/globulario/services/golang/resource/resource_client"
	"github.com/globulario/services/golang/resource/resourcepb"
)

// -----------------------------------------------------------------------------
// Defaults & CORS
// -----------------------------------------------------------------------------

var (
	defaultPort  = 10029
	defaultProxy = 10030

	allowAllOrigins   = true
	allowedOriginsStr = ""
)

// -----------------------------------------------------------------------------
// Server type (unchanged fields preserved, ordering cleaned)
// -----------------------------------------------------------------------------

type server struct {
	// Core metadata
	Id           string
	Mac          string
	Name         string
	Domain       string
	Address      string
	Path         string
	Proto        string
	Port         int
	Proxy        int
	Protocol     string
	Version      string
	PublisherID  string
	Description  string
	Keywords     []string
	Repositories []string
	Discoveries  []string

	// Policy / ops
	AllowAllOrigins bool
	AllowedOrigins  string
	KeepUpToDate    bool
	Plaform         string
	Checksum        string
	KeepAlive       bool
	Permissions     []interface{}
	Dependencies    []string
	Process         int
	ProxyProcess    int
	ConfigPath      string
	LastError       string
	State           string
	ModTime         int64

	// TLS
	TLS                bool
	CertFile           string
	KeyFile            string
	CertAuthorityTrust string

	// The session time out.
	SessionTimeout int

	// Data store where account, role ect are keep...
	store   persistence_store.Store
	isReady bool

	// The grpc server.
	grpcServer *grpc.Server

	// The backend infos.
	Backend_type               string
	Backend_address            string
	Backend_port               int64
	Backend_user               string
	Backend_password           string
	Backend_replication_factor int64
	DataPath                   string
}

// -----------------------------------------------------------------------------
// Globular service contract — getters/setters kept concise
// -----------------------------------------------------------------------------

func (s *server) GetConfigurationPath() string          { return s.ConfigPath }
func (s *server) SetConfigurationPath(path string)      { s.ConfigPath = path }
func (s *server) GetAddress() string                    { return s.Address }
func (s *server) SetAddress(address string)             { s.Address = address }
func (s *server) GetProcess() int                       { return s.Process }
func (s *server) SetProcess(pid int)                    { s.Process = pid }
func (s *server) GetProxyProcess() int                  { return s.ProxyProcess }
func (s *server) SetProxyProcess(pid int)               { s.ProxyProcess = pid }
func (s *server) GetLastError() string                  { return s.LastError }
func (s *server) SetLastError(err string)               { s.LastError = err }
func (s *server) SetModTime(modtime int64)              { s.ModTime = modtime }
func (s *server) GetModTime() int64                     { return s.ModTime }
func (s *server) GetId() string                         { return s.Id }
func (s *server) SetId(id string)                       { s.Id = id }
func (s *server) GetName() string                       { return s.Name }
func (s *server) SetName(name string)                   { s.Name = name }
func (s *server) GetMac() string                        { return s.Mac }
func (s *server) SetMac(mac string)                     { s.Mac = mac }
func (s *server) GetDescription() string                { return s.Description }
func (s *server) SetDescription(description string)     { s.Description = description }
func (s *server) GetKeywords() []string                 { return s.Keywords }
func (s *server) SetKeywords(keywords []string)         { s.Keywords = keywords }
func (s *server) GetRepositories() []string             { return s.Repositories }
func (s *server) SetRepositories(repositories []string) { s.Repositories = repositories }
func (s *server) GetDiscoveries() []string              { return s.Discoveries }
func (s *server) SetDiscoveries(discoveries []string)   { s.Discoveries = discoveries }
func (s *server) Dist(path string) (string, error)      { return globular.Dist(path, s) }
func (s *server) GetDependencies() []string {
	if s.Dependencies == nil {
		s.Dependencies = make([]string, 0)
	}
	return s.Dependencies
}
func (s *server) SetDependency(dep string) {
	if s.Dependencies == nil {
		s.Dependencies = make([]string, 0)
	}
	if !Utility.Contains(s.Dependencies, dep) {
		s.Dependencies = append(s.Dependencies, dep)
	}
}
func (s *server) GetChecksum() string               { return s.Checksum }
func (s *server) SetChecksum(checksum string)       { s.Checksum = checksum }
func (s *server) GetPlatform() string               { return s.Plaform }
func (s *server) SetPlatform(platform string)       { s.Plaform = platform }
func (s *server) GetPath() string                   { return s.Path }
func (s *server) SetPath(path string)               { s.Path = path }
func (s *server) GetProto() string                  { return s.Proto }
func (s *server) SetProto(proto string)             { s.Proto = proto }
func (s *server) GetPort() int                      { return s.Port }
func (s *server) SetPort(port int)                  { s.Port = port }
func (s *server) GetProxy() int                     { return s.Proxy }
func (s *server) SetProxy(proxy int)                { s.Proxy = proxy }
func (s *server) GetProtocol() string               { return s.Protocol }
func (s *server) SetProtocol(protocol string)       { s.Protocol = protocol }
func (s *server) GetAllowAllOrigins() bool          { return s.AllowAllOrigins }
func (s *server) SetAllowAllOrigins(v bool)         { s.AllowAllOrigins = v }
func (s *server) GetAllowedOrigins() string         { return s.AllowedOrigins }
func (s *server) SetAllowedOrigins(v string)        { s.AllowedOrigins = v }
func (s *server) GetState() string                  { return s.State }
func (s *server) SetState(state string)             { s.State = state }
func (s *server) GetDomain() string                 { return s.Domain }
func (s *server) SetDomain(domain string)           { s.Domain = domain }
func (s *server) GetTls() bool                      { return s.TLS }
func (s *server) SetTls(hasTls bool)                { s.TLS = hasTls }
func (s *server) GetCertAuthorityTrust() string     { return s.CertAuthorityTrust }
func (s *server) SetCertAuthorityTrust(ca string)   { s.CertAuthorityTrust = ca }
func (s *server) GetCertFile() string               { return s.CertFile }
func (s *server) SetCertFile(certFile string)       { s.CertFile = certFile }
func (s *server) GetKeyFile() string                { return s.KeyFile }
func (s *server) SetKeyFile(keyFile string)         { s.KeyFile = keyFile }
func (s *server) GetVersion() string                { return s.Version }
func (s *server) SetVersion(version string)         { s.Version = version }
func (s *server) GetPublisherID() string            { return s.PublisherID }
func (s *server) SetPublisherID(id string)          { s.PublisherID = id }
func (s *server) GetKeepUpToDate() bool             { return s.KeepUpToDate }
func (s *server) SetKeepUptoDate(val bool)          { s.KeepUpToDate = val }
func (s *server) GetKeepAlive() bool                { return s.KeepAlive }
func (s *server) SetKeepAlive(val bool)             { s.KeepAlive = val }
func (s *server) GetPermissions() []interface{}     { return s.Permissions }
func (s *server) SetPermissions(perm []interface{}) { s.Permissions = perm }

// Lifecycle
func (s *server) Init() error {
	if err := globular.InitService(s); err != nil {
		return err
	}
	gs, err := globular.InitGrpcServer(s)
	if err != nil {
		return err
	}
	s.grpcServer = gs
	return nil
}
func (s *server) Save() error         { return globular.SaveService(s) }
func (s *server) StartService() error { return globular.StartService(s, s.grpcServer) }
func (s *server) StopService() error  { return globular.StopService(s, s.grpcServer) }

// ////////////////////// Resource Client ////////////////////////////////////////////
func getResourceClient(address string) (*resource_client.Resource_Client, error) {
	Utility.RegisterFunction("NewResourceService_Client", resource_client.NewResourceService_Client)
	client, err := globular_client.GetClient(address, "resource.ResourceService", "NewResourceService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*resource_client.Resource_Client), nil
}

// ////////////////////// persistence Client ////////////////////////////////////////////
func getPersistenceClient(address string) (*persistence_client.Persistence_Client, error) {
	Utility.RegisterFunction("NewPersistenceService_Client", persistence_client.NewPersistenceService_Client)
	client, err := globular_client.GetClient(address, "persistence.PersistenceService", "NewPersistenceService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*persistence_client.Persistence_Client), nil
}

// /////////////////// event service functions ////////////////////////////////////
func getEventClient(address string) (*event_client.Event_Client, error) {
	Utility.RegisterFunction("NewEventService_Client", event_client.NewEventService_Client)
	client, err := globular_client.GetClient(address, "event.EventService", "NewEventService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*event_client.Event_Client), nil
}

// when services state change that publish
func (srv *server) publishEvent(evt string, data []byte, address string) error {

	client, err := getEventClient(address)
	if err != nil {
		return err
	}

	err = client.Publish(evt, data)
	return err
}

// Public event to a peer other than the default one...
func (srv *server) publishRemoteEvent(address, evt string, data []byte) error {

	client, err := getEventClient(address)
	if err != nil {
		return err
	}

	return client.Publish(evt, data)
}

// ///////////////////////////////////// return the peers infos from a given peer /////////////////////////////
func (srv *server) getPeerInfos(address, mac string) (*resourcepb.Peer, error) {

	client, err := getResourceClient(address)
	if err != nil {
		logger.Error("fail to connect with remote resource service", "error", err)
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

	client, err := getResourceClient(address)
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
		logger.Error("fail to set host entry", "address", address, "error", err)
		return err
	}

	if peer.ExternalIpAddress == Utility.MyIP() {
		hosts.AddHost(peer.LocalIpAddress, address)
	}

	err = hosts.Save()
	if err != nil {
		logger.Error("fail to save hosts", "ip", peer.LocalIpAddress, "address", address, "error", err)
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
		logger.Error("fail to save hosts file", "error", err)
	}

	return err
}

//////////////////////////////////////// RBAC Functions ///////////////////////////////////////////////

/**
 * Get the rbac client.
 */
func getRbacClient(address string) (*rbac_client.Rbac_Client, error) {
	Utility.RegisterFunction("NewRbacService_Client", rbac_client.NewRbacService_Client)
	client, err := globular_client.GetClient(address, "rbac.RbacService", "NewRbacService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*rbac_client.Rbac_Client), nil
}

func (srv *server) addResourceOwner(path, resourceType, subject string, subjectType rbacpb.SubjectType) error {
	rbac_client_, err := getRbacClient(srv.Address)
	if err != nil {
		return err
	}

	err = rbac_client_.AddResourceOwner(path, resourceType, subject, subjectType)
	return err
}

func (srv *server) deleteResourcePermissions(path string) error {
	rbac_client_, err := getRbacClient(srv.Address)
	if err != nil {
		return err
	}
	return rbac_client_.DeleteResourcePermissions(path)
}

func (srv *server) deleteAllAccess(suject string, subjectType rbacpb.SubjectType) error {
	rbac_client_, err := getRbacClient(srv.Address)
	if err != nil {
		return err
	}
	return rbac_client_.DeleteAllAccess(suject, subjectType)
}

func (srv *server) SetAccountAllocatedSpace(accountId string, space uint64) error {
	rbac_client_, err := getRbacClient(srv.Address)
	if err != nil {
		return err
	}
	return rbac_client_.SetAccountAllocatedSpace(accountId, space)
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

		switch srv.Backend_type {
		case "SCYLLA":
			srv.store = new(persistence_store.ScyllaStore)
			options := map[string]interface{}{"keyspace": "local_resource", "replication_factor": srv.Backend_replication_factor, "hosts": []string{srv.Backend_address}, "port": srv.Backend_port}
			options_, _ := Utility.ToJson(options)
			options_str = string(options_)
		case "MONGO":

			process, err := Utility.GetProcessIdsByName("mongod")
			if err != nil {
				logger.Error("fail to get process id for mongod", "error", err)
				return nil, err
			}

			if len(process) == 0 {
				logger.Error("mongod is not running on this server, please start it before starting the resource server")
				return nil, errors.New("mongod is not running on this server, please start it before starting the resource server")
			}

			srv.store = new(persistence_store.MongoStore)

		case "SQL":

			srv.store = new(persistence_store.SqlStore)
			options := map[string]interface{}{"driver": "sqlite3", "charset": "utf8", "path": srv.DataPath + "/sql-data"}
			options_, _ := Utility.ToJson(options)
			options_str = string(options_)

		default:
			return nil, errors.New("unknown backend type " + srv.Backend_type)
		}

		// Connect to the store.
		err := srv.store.Connect("local_resource", srv.Backend_address, int32(srv.Backend_port), srv.Backend_user, srv.Backend_password, "local_resource", 5000, options_str)
		if err != nil {

			logger.Error("fail to connect to store", "error", err)
			os.Exit(1)
		}

		err = srv.store.Ping(context.Background(), "local_resource")
		if err != nil {
			logger.Error("fail to reach store", "error", err)
			return nil, err
		}

		srv.isReady = true

		logger.Info("store is running and ready to be used", "address", srv.Backend_address+":"+Utility.ToString(srv.Backend_port))

		switch srv.Backend_type {
		case "SQL":
			// Create tables if not already exist.
			err := srv.store.(*persistence_store.SqlStore).CreateTable(context.Background(), "local_resource", "local_resource", "Accounts", []string{"name TEXT", "email TEXT", "domain TEXT", "password TEXT", "refresh_token TEXT"})
			if err != nil {
				logger.Error("fail to create table Accounts", "error", err)
			}

			err = srv.store.(*persistence_store.SqlStore).CreateTable(context.Background(), "local_resource", "local_resource", "Applications", []string{"name TEXT", "domain TEXT", "description TEXT", "icon TEXT", "alias TEXT", "password TEXT", "store TEXT", "last_deployed INTEGER", "path TEXT", "version TEXT", "PublisherID TEXT", "creation_date INTEGER"})
			if err != nil {
				logger.Error("fail to create table Applications", "error", err)
			}

			// Create organizations table.
			err = srv.store.(*persistence_store.SqlStore).CreateTable(context.Background(), "local_resource", "local_resource", "Organizations", []string{"name TEXT", "domain TEXT", "description TEXT", "icon TEXT", "email TEXT"})
			if err != nil {
				logger.Error("fail to create table Organizations", "error", err)
			}

			// Create roles table.
			err = srv.store.(*persistence_store.SqlStore).CreateTable(context.Background(), "local_resource", "local_resource", "Roles", []string{"name TEXT", "domain TEXT", "description TEXT"})
			if err != nil {
				logger.Error("fail to create table Roles", "error", err)
			}

			// Create groups table.
			err = srv.store.(*persistence_store.SqlStore).CreateTable(context.Background(), "local_resource", "local_resource", "Groups", []string{"name TEXT", "domain TEXT", "description TEXT"})
			if err != nil {
				logger.Error("fail to create table Groups", "error", err)
			}

			// Create peers table.
			err = srv.store.(*persistence_store.SqlStore).CreateTable(context.Background(), "local_resource", "local_resource", "Peers", []string{"domain TEXT", "hostname TEXT", "external_ip_address TEXT", "local_ip_address TEXT", "mac TEXT", "protocol TEXT", "state INTEGER", "PortHTTP INTEGER", "PortHTTPS INTEGER"})
			if err != nil {
				logger.Error("fail to create table Peers", "error", err)
			}

			// Create peers table.
			err = srv.store.(*persistence_store.SqlStore).CreateTable(context.Background(), "local_resource", "local_resource", "Peers", []string{"domain TEXT", "hostname TEXT", "external_ip_address TEXT", "local_ip_address TEXT", "mac TEXT", "protocol TEXT", "state INTEGER", "PortHTTP INTEGER", "PortHTTPS INTEGER"})
			if err != nil {
				logger.Error("fail to create table Peers", "error", err)
			}

			// Create the sessions table.
			err = srv.store.(*persistence_store.SqlStore).CreateTable(context.Background(), "local_resource", "local_resource", "Sessions", []string{"accountId TEXT", "domain TEXT", "state INTEGER", "last_state_time INTEGER", "expire_at INTEGER"})
			if err != nil {
				logger.Error("fail to create table Sessions", "error", err)
			}

			// Create the notifications table.
			err = srv.store.(*persistence_store.SqlStore).CreateTable(context.Background(), "local_resource", "local_resource", "Notifications", []string{"date REAL", "domain TEXT", "message TEXT", "recipient TEXT", "sender TEXT", "mac TEXT", "notification_type INTEGER"})
			if err != nil {
				logger.Error("fail to create table Notifications", "error", err)
			}
		case "SCYLLA":
			// Create tables if not already exist.
			err := srv.store.(*persistence_store.ScyllaStore).CreateTable(context.Background(), "local_resource", "local_resource", "Accounts", []string{"name TEXT", "email TEXT", "domain TEXT", "password TEXT"})
			if err != nil {
				logger.Error("fail to create table Accounts", "error", err)
			}

			err = srv.store.(*persistence_store.ScyllaStore).CreateTable(context.Background(), "local_resource", "local_resource", "Applications", []string{"name TEXT", "domain TEXT", "description TEXT", "icon TEXT", "alias TEXT", "password TEXT", "store TEXT", "last_deployed INT", "path TEXT", "version TEXT", "PublisherID TEXT", "creation_date INT"})
			if err != nil {
				logger.Error("fail to create table Applications", "error", err)
			}

			// Create organizations table.
			err = srv.store.(*persistence_store.ScyllaStore).CreateTable(context.Background(), "local_resource", "local_resource", "Organizations", []string{"name TEXT", "domain TEXT", "description TEXT", "icon TEXT", "email TEXT"})
			if err != nil {
				logger.Error("fail to create table Organizations", "error", err)
			}

			// Create roles table.
			err = srv.store.(*persistence_store.ScyllaStore).CreateTable(context.Background(), "local_resource", "local_resource", "Roles", []string{"name TEXT", "domain TEXT", "description TEXT"})
			if err != nil {
				logger.Error("fail to create table Roles", "error", err)
			}

			// Create groups table.
			err = srv.store.(*persistence_store.ScyllaStore).CreateTable(context.Background(), "local_resource", "local_resource", "Groups", []string{"name TEXT", "domain TEXT", "description TEXT"})
			if err != nil {
				logger.Error("fail to create table Groups", "error", err)
			}

			// Create peers table.
			err = srv.store.(*persistence_store.ScyllaStore).CreateTable(context.Background(), "local_resource", "local_resource", "Peers", []string{"domain TEXT", "hostname TEXT", "external_ip_address TEXT", "local_ip_address TEXT", "mac TEXT", "protocol TEXT", "state INT", "PortHTTP INT", "PortHTTPS INT"})
			if err != nil {
				logger.Error("fail to create table Peers", "error", err)
			}

			// Create the sessions table.
			err = srv.store.(*persistence_store.ScyllaStore).CreateTable(context.Background(), "local_resource", "local_resource", "Sessions", []string{"accountId TEXT", "domain TEXT", "state INT", "last_state_time BIGINT", "expire_at BIGINT"})
			if err != nil {
				logger.Error("fail to create table Sessions", "error", err)
			}
			if err != nil {
				logger.Error("fail to create table Sessions", "error", err)
			}

			// Create the notifications table.
			err = srv.store.(*persistence_store.ScyllaStore).CreateTable(context.Background(), "local_resource", "local_resource", "Notifications", []string{"date DOUBLE", "domain TEXT", "message TEXT", "recipient TEXT", "sender TEXT", "mac TEXT", "notification_type INT"})
			if err != nil {
				logger.Error("fail to create table Notifications", "error", err)
			}
		}
	} else if !srv.isReady {
		nbTry := 100
		for range nbTry {
			time.Sleep(100 * time.Millisecond)
			if srv.isReady {
				break
			}
		}
	}

	return srv.store, nil
}

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
	for i := range members {

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
	logger.Info("group created", "group_id", id, "owner", owner)
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
			client, err := getResourceClient(domain)
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
			client, err := getResourceClient(domain)
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
			client, err := getResourceClient(domain)
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
				return errors.New(" named " + targetId + " already exist in  " + field + "!")
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
			createTable := `CREATE TABLE IF NOT EXISTS ` + sourceCollection + `_` + field + ` (source_id TEXT, target_id TEXT, PRIMARY KEY (source_id, target_id))`
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

// -----------------------------------------------------------------------------
// main()
// -----------------------------------------------------------------------------

func main() {
	s := new(server)

	// Populate ONLY safe defaults before config/etcd is touched.
	s.Name = string(resourcepb.File_resource_proto.Services().Get(0).FullName())
	s.Proto = resourcepb.File_resource_proto.Path()
	s.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	s.Port = defaultPort
	s.Proxy = defaultProxy
	s.Protocol = "grpc"
	s.Version = "0.0.1"
	s.PublisherID = "localhost"
	s.Description = "Resource service"
	s.Keywords = []string{"resource", "rbac", "accounts", "globular"}
	s.Repositories = make([]string, 0)
	s.Discoveries = make([]string, 0)
	s.Dependencies = make([]string, 0)
	s.Permissions = make([]interface{}, 0)
	s.Process = -1
	s.ProxyProcess = -1
	s.AllowAllOrigins = allowAllOrigins
	s.AllowedOrigins = allowedOriginsStr
	s.KeepAlive = true
	s.KeepUpToDate = true

	// ---- CLI flags handled BEFORE any call that might touch etcd ----
	args := os.Args[1:]
	if len(args) == 0 {
		printUsage()
		return
	}

	for _, a := range args {
		switch strings.ToLower(a) {
		case "--describe":
			// Only compute ephemeral data here; avoid etcd
			s.Process = os.Getpid()
			s.State = "starting"
			if v, ok := os.LookupEnv("GLOBULAR_DOMAIN"); ok && v != "" {
				s.Domain = strings.ToLower(v)
			} else {
				s.Domain = "localhost"
			}
			if v, ok := os.LookupEnv("GLOBULAR_ADDRESS"); ok && v != "" {
				s.Address = strings.ToLower(v)
			} else {
				s.Address = "localhost:" + Utility.ToString(s.Port)
			}
			b, err := globular.DescribeJSON(s)
			if err != nil {
				logger.Error("describe error", "service", s.Name, "id", s.Id, "err", err)
				os.Exit(2)
			}
			_, _ = os.Stdout.Write(b)
			_, _ = os.Stdout.Write([]byte("\n"))
			return
		case "--health":
			if s.Port == 0 || s.Name == "" {
				logger.Error("health error: uninitialized", "service", s.Name, "port", s.Port)
				os.Exit(2)
			}
			b, err := globular.HealthJSON(s, &globular.HealthOptions{Timeout: 1500 * time.Millisecond})
			if err != nil {
				logger.Error("health error", "service", s.Name, "id", s.Id, "err", err)
				os.Exit(2)
			}
			_, _ = os.Stdout.Write(b)
			_, _ = os.Stdout.Write([]byte("\n"))
			return
		}
	}

	// Optional positional args (unchanged behavior)
	if len(args) == 1 && !strings.HasPrefix(args[0], "-") {
		s.Id = args[0]
	} else if len(args) == 2 && !strings.HasPrefix(args[0], "-") && !strings.HasPrefix(args[1], "-") {
		s.Id = args[0]
		s.ConfigPath = args[1]
	}

	// Now it’s safe to read local config (may try etcd or file fallback)
	if d, err := config.GetDomain(); err == nil {
		s.Domain = d
	} else {
		s.Domain = "localhost"
	}
	if a, err := config.GetAddress(); err == nil && strings.TrimSpace(a) != "" {
		s.Address = a
	}

	start := time.Now()
	if err := s.Init(); err != nil {
		logger.Error("service init failed", "service", s.Name, "id", s.Id, "err", err)
		os.Exit(1)
	}

	// Register service implementation handlers (implemented elsewhere in this package)
	resourcepb.RegisterResourceServiceServer(s.grpcServer, s)
	reflection.Register(s.grpcServer)

	logger.Info("service ready",
		"service", s.Name,
		"port", s.Port,
		"proxy", s.Proxy,
		"protocol", s.Protocol,
		"domain", s.Domain,
		"listen_ms", time.Since(start).Milliseconds(),
	)

	if err := s.StartService(); err != nil {
		logger.Error("service start failed", "service", s.Name, "id", s.Id, "err", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  resource_server [service_id] [config_path]")
	fmt.Println("Options:")
	fmt.Println("  --describe    Print service metadata as JSON and exit")
	fmt.Println("  --health      Print service health as JSON and exit")
	fmt.Println("Examples:")
	fmt.Println("  resource_server my-id /etc/globular/resource/config.json")
	fmt.Println("  resource_server --describe")
	fmt.Println("  resource_server --health")
}
