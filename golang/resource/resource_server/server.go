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
	Utility "github.com/globulario/utility"
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
	defaultPort  = 10000
	defaultProxy = defaultPort + 1

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
