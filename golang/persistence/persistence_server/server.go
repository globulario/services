// Package main implements the Persistence gRPC service wired for Globular.
// It provides structured logging with slog, safe --describe/--health handling
// before touching config/etcd, and preserves all public getters/setters and
// service lifecycle methods. Store connections (Mongo/SQL) are initialized in Init.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/resource/resourcepb"
	Utility "github.com/globulario/utility"

	"github.com/globulario/services/golang/persistence/persistence_client"
	"github.com/globulario/services/golang/persistence/persistence_store"
	"github.com/globulario/services/golang/persistence/persistencepb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// -----------------------------------------------------------------------------
// Consts, defaults & logger
// -----------------------------------------------------------------------------

const (
	BufferSize = 1024 * 5 // chunk size
)

var (
	defaultPort       = 10035
	defaultProxy      = 10036
	allowAllOrigins   = true
	allowedOriginsStr = ""

	logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
)

// -----------------------------------------------------------------------------
// Types
// -----------------------------------------------------------------------------

// connection describes a datastore connection.
type connection struct {
	Id       string
	Name     string
	Host     string
	Store    persistencepb.StoreType
	User     string
	Port     int32
	Timeout  int32
	Options  string
	Password string
}

// server implements the Globular service + Persistence fields.
type server struct {
	// Core metadata
	Id              string
	Mac             string
	Name            string
	Path            string
	Port            int
	Proto           string
	Proxy           int
	Protocol        string
	AllowAllOrigins bool
	AllowedOrigins  string
	Domain          string
	Address         string
	Description     string
	Keywords        []string
	Repositories    []string
	Discoveries     []string

	// TLS
	CertAuthorityTrust string
	CertFile           string
	KeyFile            string
	TLS                bool

	// Versioning / ops
	Version      string
	PublisherID  string
	KeepUpToDate bool
	Plaform      string
	Checksum     string
	KeepAlive    bool
	Permissions  []interface{}
	Dependencies []string
	Process      int
	ProxyProcess int
	ConfigPath   string
	LastError    string
	ModTime      int64
	State        string

	// Runtime
	grpcServer *grpc.Server

	// Saved connections (from config)
	Connections map[string]connection
	// Unsaved (runtime) connections
	connections map[string]connection
	// Active stores keyed by connection id
	stores map[string]persistence_store.Store
}

// -----------------------------------------------------------------------------
// Globular service contract (public prototypes preserved)
// -----------------------------------------------------------------------------

func (srv *server) GetConfigurationPath() string          { return srv.ConfigPath }
func (srv *server) SetConfigurationPath(path string)      { srv.ConfigPath = path }
func (srv *server) GetAddress() string                    { return srv.Address }
func (srv *server) SetAddress(address string)             { srv.Address = address }
func (srv *server) GetProcess() int                       { return srv.Process }
func (srv *server) SetProcess(pid int)                    { srv.Process = pid }
func (srv *server) GetProxyProcess() int                  { return srv.ProxyProcess }
func (srv *server) SetProxyProcess(pid int)               { srv.ProxyProcess = pid }
func (srv *server) GetState() string                      { return srv.State }
func (srv *server) SetState(state string)                 { srv.State = state }
func (srv *server) GetLastError() string                  { return srv.LastError }
func (srv *server) SetLastError(err string)               { srv.LastError = err }
func (srv *server) SetModTime(modtime int64)              { srv.ModTime = modtime }
func (srv *server) GetModTime() int64                     { return srv.ModTime }
func (srv *server) GetId() string                         { return srv.Id }
func (srv *server) SetId(id string)                       { srv.Id = id }
func (srv *server) GetName() string                       { return srv.Name }
func (srv *server) SetName(name string)                   { srv.Name = name }
func (srv *server) GetMac() string                        { return srv.Mac }
func (srv *server) SetMac(mac string)                     { srv.Mac = mac }
func (srv *server) GetDescription() string                { return srv.Description }
func (srv *server) SetDescription(description string)     { srv.Description = description }
func (srv *server) GetKeywords() []string                 { return srv.Keywords }
func (srv *server) SetKeywords(keywords []string)         { srv.Keywords = keywords }
func (srv *server) GetRepositories() []string             { return srv.Repositories }
func (srv *server) SetRepositories(repositories []string) { srv.Repositories = repositories }
func (srv *server) GetDiscoveries() []string              { return srv.Discoveries }
func (srv *server) SetDiscoveries(discoveries []string)   { srv.Discoveries = discoveries }

func (srv *server) RolesDefault() []resourcepb.Role {
	domain, _ := config.GetDomain()

	return []resourcepb.Role{
		{
			Id:          "role:persistence.viewer",
			Name:        "Persistence Viewer",
			Domain:      domain,
			Description: "Read-only access to query data and check connection health.",
			Actions: []string{
				"/persistence.PersistenceService/Ping",
				"/persistence.PersistenceService/Count",
				"/persistence.PersistenceService/Find",
				"/persistence.PersistenceService/FindOne",
				"/persistence.PersistenceService/Aggregate",
			},
			TypeName: "resource.Role",
		},
		{
			Id:          "role:persistence.editor",
			Name:        "Persistence Editor",
			Domain:      domain,
			Description: "Create, update, and delete data records (no schema or connection admin).",
			Actions: []string{
				"/persistence.PersistenceService/Count",
				"/persistence.PersistenceService/Find",
				"/persistence.PersistenceService/FindOne",
				"/persistence.PersistenceService/Aggregate",
				"/persistence.PersistenceService/InsertOne",
				"/persistence.PersistenceService/InsertMany",
				"/persistence.PersistenceService/Update",
				"/persistence.PersistenceService/UpdateOne",
				"/persistence.PersistenceService/ReplaceOne",
				"/persistence.PersistenceService/Delete",
				"/persistence.PersistenceService/DeleteOne",
			},
			TypeName: "resource.Role",
		},
		{
			Id:          "role:persistence.admin",
			Name:        "Persistence Admin",
			Domain:      domain,
			Description: "Full control over PersistenceService: connections, schema (DB/collections), data, and admin commands.",
			Actions: []string{
				"/persistence.PersistenceService/Stop",

				"/persistence.PersistenceService/CreateConnection",
				"/persistence.PersistenceService/DeleteConnection",
				"/persistence.PersistenceService/Connect",
				"/persistence.PersistenceService/Disconnect",
				"/persistence.PersistenceService/Ping",

				"/persistence.PersistenceService/CreateDatabase",
				"/persistence.PersistenceService/DeleteDatabase",
				"/persistence.PersistenceService/CreateCollection",
				"/persistence.PersistenceService/DeleteCollection",

				"/persistence.PersistenceService/Count",
				"/persistence.PersistenceService/Find",
				"/persistence.PersistenceService/FindOne",
				"/persistence.PersistenceService/Aggregate",
				"/persistence.PersistenceService/InsertOne",
				"/persistence.PersistenceService/InsertMany",
				"/persistence.PersistenceService/Update",
				"/persistence.PersistenceService/UpdateOne",
				"/persistence.PersistenceService/ReplaceOne",
				"/persistence.PersistenceService/Delete",
				"/persistence.PersistenceService/DeleteOne",

				"/persistence.PersistenceService/RunAdminCmd",
			},
			TypeName: "resource.Role",
		},
	}
}

func (srv *server) Dist(path string) (string, error) { return globular.Dist(path, srv) }

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
	if !Utility.Contains(srv.Dependencies, dependency) {
		srv.Dependencies = append(srv.Dependencies, dependency)
	}
}

func (srv *server) GetChecksum() string               { return srv.Checksum }
func (srv *server) SetChecksum(checksum string)       { srv.Checksum = checksum }
func (srv *server) GetPlatform() string               { return srv.Plaform }
func (srv *server) SetPlatform(platform string)       { srv.Plaform = platform }
func (srv *server) GetPath() string                   { return srv.Path }
func (srv *server) SetPath(path string)               { srv.Path = path }
func (srv *server) GetProto() string                  { return srv.Proto }
func (srv *server) SetProto(proto string)             { srv.Proto = proto }
func (srv *server) GetPort() int                      { return srv.Port }
func (srv *server) SetPort(port int)                  { srv.Port = port }
func (srv *server) GetProxy() int                     { return srv.Proxy }
func (srv *server) SetProxy(proxy int)                { srv.Proxy = proxy }
func (srv *server) GetProtocol() string               { return srv.Protocol }
func (srv *server) SetProtocol(protocol string)       { srv.Protocol = protocol }
func (srv *server) GetAllowAllOrigins() bool          { return srv.AllowAllOrigins }
func (srv *server) SetAllowAllOrigins(b bool)         { srv.AllowAllOrigins = b }
func (srv *server) GetAllowedOrigins() string         { return srv.AllowedOrigins }
func (srv *server) SetAllowedOrigins(s string)        { srv.AllowedOrigins = s }
func (srv *server) GetDomain() string                 { return srv.Domain }
func (srv *server) SetDomain(domain string)           { srv.Domain = domain }
func (srv *server) GetTls() bool                      { return srv.TLS }
func (srv *server) SetTls(hasTls bool)                { srv.TLS = hasTls }
func (srv *server) GetCertAuthorityTrust() string     { return srv.CertAuthorityTrust }
func (srv *server) SetCertAuthorityTrust(ca string)   { srv.CertAuthorityTrust = ca }
func (srv *server) GetCertFile() string               { return srv.CertFile }
func (srv *server) SetCertFile(certFile string)       { srv.CertFile = certFile }
func (srv *server) GetKeyFile() string                { return srv.KeyFile }
func (srv *server) SetKeyFile(keyFile string)         { srv.KeyFile = keyFile }
func (srv *server) GetVersion() string                { return srv.Version }
func (srv *server) SetVersion(version string)         { srv.Version = version }
func (srv *server) GetPublisherID() string            { return srv.PublisherID }
func (srv *server) SetPublisherID(PublisherID string) { srv.PublisherID = PublisherID }
func (srv *server) GetKeepUpToDate() bool             { return srv.KeepUpToDate }
func (srv *server) SetKeepUptoDate(val bool)          { srv.KeepUpToDate = val }
func (srv *server) GetKeepAlive() bool                { return srv.KeepAlive }
func (srv *server) SetKeepAlive(val bool)             { srv.KeepAlive = val }
func (srv *server) GetPermissions() []interface{}     { return srv.Permissions }
func (srv *server) SetPermissions(p []interface{})    { srv.Permissions = p }

// Init creates/loads configuration, initializes gRPC and connects stores.
func (srv *server) Init() error {
	srv.connections = make(map[string]connection)
	srv.stores = make(map[string]persistence_store.Store)

	if err := globular.InitService(srv); err != nil {
		return err
	}
	gs, err := globular.InitGrpcServer(srv)
	if err != nil {
		return err
	}
	srv.grpcServer = gs

	// Initialize stores for configured connections.

	for _, c := range srv.Connections {
		switch c.Store {
		case persistencepb.StoreType_MONGO:
			s := new(persistence_store.MongoStore)
			if err := s.Connect(c.Id, c.Host, c.Port, c.User, c.Password, c.Name, c.Timeout, c.Options); err != nil {
				return err
			}
			srv.stores[c.Id] = s
		case persistencepb.StoreType_SQL:
			s := new(persistence_store.SqlStore)
			if err := s.Connect(c.Id, c.Host, c.Port, c.User, c.Password, c.Name, c.Timeout, c.Options); err != nil {
				return err
			}
			srv.stores[c.Id] = s
		}
	}
	return nil
}

// Save persists configuration.
func (srv *server) Save() error { return globular.SaveService(srv) }

// StartService starts gRPC (and proxy if configured).
func (srv *server) StartService() error { return globular.StartService(srv, srv.grpcServer) }

// StopService gracefully stops gRPC.
func (srv *server) StopService() error { return globular.StopService(srv, srv.grpcServer) }

// Stop RPC.
func (srv *server) Stop(context.Context, *persistencepb.StopRequest) (*persistencepb.StopResponse, error) {
	return &persistencepb.StopResponse{}, srv.StopService()
}


// -----------------------------------------------------------------------------
// main with --describe / --health
// -----------------------------------------------------------------------------

func main() {
	srv := new(server)

	// Fill ONLY fields that do NOT require config/etcd yet.
	srv.Name = string(persistencepb.File_persistence_proto.Services().Get(0).FullName())
	srv.Port = defaultPort
	srv.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	srv.Proto = persistencepb.File_persistence_proto.Path()
	srv.Proxy = defaultProxy
	srv.Protocol = "grpc"
	srv.Version = "0.0.1"
	srv.PublisherID = "localhost"
	srv.AllowAllOrigins = allowAllOrigins
	srv.AllowedOrigins = allowedOriginsStr
	srv.Permissions = make([]interface{}, 0)
	srv.Keywords = make([]string, 0)
	srv.Repositories = make([]string, 0)
	srv.Discoveries = make([]string, 0)
	srv.Dependencies = []string{"authentication.AuthenticationService", "event.EventService"}
	srv.Process = -1
	srv.ProxyProcess = -1
	srv.KeepAlive = true
	srv.KeepUpToDate = true

	srv.Permissions = []interface{}{
		// ---- Stop the service
		map[string]interface{}{
			"action":     "/persistence.PersistenceService/Stop",
			"permission": "admin",
		},

		// ---- Connection lifecycle
		map[string]interface{}{
			"action":     "/persistence.PersistenceService/CreateConnection",
			"permission": "admin",
			"resources": []interface{}{
				// CreateConnectionRqst.connection.id
				map[string]interface{}{"index": 0, "field": "Connection.Id", "permission": "admin"},
			},
		},
		map[string]interface{}{
			"action":     "/persistence.PersistenceService/DeleteConnection",
			"permission": "admin",
			"resources": []interface{}{
				// DeleteConnectionRqst.id
				map[string]interface{}{"index": 0, "field": "Id", "permission": "admin"},
			},
		},
		map[string]interface{}{
			"action":     "/persistence.PersistenceService/Connect",
			"permission": "admin",
			"resources": []interface{}{
				// ConnectRqst.connectionId
				map[string]interface{}{"index": 0, "field": "ConnectionId", "permission": "admin"},
			},
		},
		map[string]interface{}{
			"action":     "/persistence.PersistenceService/Disconnect",
			"permission": "admin",
			"resources": []interface{}{
				// DisconnectRqst.connectionId
				map[string]interface{}{"index": 0, "field": "ConnectionId", "permission": "admin"},
			},
		},

		// ---- Health / introspection
		map[string]interface{}{
			"action":     "/persistence.PersistenceService/Ping",
			"permission": "read",
			"resources": []interface{}{
				// PingConnectionRqst.id
				map[string]interface{}{"index": 0, "field": "Id", "permission": "read"},
			},
		},

		// ---- Database & collection admin
		map[string]interface{}{
			"action":     "/persistence.PersistenceService/CreateDatabase",
			"permission": "write",
			"resources": []interface{}{
				// CreateDatabaseRqst.id
				map[string]interface{}{"index": 0, "field": "Id", "permission": "write"},
				// CreateDatabaseRqst.database
				map[string]interface{}{"index": 0, "field": "Database", "permission": "write"},
			},
		},
		map[string]interface{}{
			"action":     "/persistence.PersistenceService/DeleteDatabase",
			"permission": "write",
			"resources": []interface{}{
				// DeleteDatabaseRqst.id
				map[string]interface{}{"index": 0, "field": "Id", "permission": "write"},
				// DeleteDatabaseRqst.database
				map[string]interface{}{"index": 0, "field": "Database", "permission": "write"},
			},
		},
		map[string]interface{}{
			"action":     "/persistence.PersistenceService/CreateCollection",
			"permission": "write",
			"resources": []interface{}{
				// CreateCollectionRqst.id
				map[string]interface{}{"index": 0, "field": "Id", "permission": "write"},
				// CreateCollectionRqst.database
				map[string]interface{}{"index": 0, "field": "Database", "permission": "write"},
				// CreateCollectionRqst.collection
				map[string]interface{}{"index": 0, "field": "Collection", "permission": "write"},
			},
		},
		map[string]interface{}{
			"action":     "/persistence.PersistenceService/DeleteCollection",
			"permission": "write",
			"resources": []interface{}{
				// DeleteCollectionRqst.id
				map[string]interface{}{"index": 0, "field": "Id", "permission": "write"},
				// DeleteCollectionRqst.database
				map[string]interface{}{"index": 0, "field": "Database", "permission": "write"},
				// DeleteCollectionRqst.collection
				map[string]interface{}{"index": 0, "field": "Collection", "permission": "write"},
			},
		},

		// ---- Read-only data operations
		map[string]interface{}{
			"action":     "/persistence.PersistenceService/Count",
			"permission": "read",
			"resources": []interface{}{
				// CountRqst.id / database / collection
				map[string]interface{}{"index": 0, "field": "Id", "permission": "read"},
				map[string]interface{}{"index": 0, "field": "Database", "permission": "read"},
				map[string]interface{}{"index": 0, "field": "Collection", "permission": "read"},
			},
		},
		map[string]interface{}{
			"action":     "/persistence.PersistenceService/Find",
			"permission": "read",
			"resources": []interface{}{
				// FindRqst.id / database / collection
				map[string]interface{}{"index": 0, "field": "Id", "permission": "read"},
				map[string]interface{}{"index": 0, "field": "Database", "permission": "read"},
				map[string]interface{}{"index": 0, "field": "Collection", "permission": "read"},
			},
		},
		map[string]interface{}{
			"action":     "/persistence.PersistenceService/FindOne",
			"permission": "read",
			"resources": []interface{}{
				// FindOneRqst.id / database / collection
				map[string]interface{}{"index": 0, "field": "Id", "permission": "read"},
				map[string]interface{}{"index": 0, "field": "Database", "permission": "read"},
				map[string]interface{}{"index": 0, "field": "Collection", "permission": "read"},
			},
		},
		map[string]interface{}{
			"action":     "/persistence.PersistenceService/Aggregate",
			"permission": "read",
			"resources": []interface{}{
				// AggregateRqst.id / database / collection
				map[string]interface{}{"index": 0, "field": "Id", "permission": "read"},
				map[string]interface{}{"index": 0, "field": "Database", "permission": "read"},
				map[string]interface{}{"index": 0, "field": "Collection", "permission": "read"},
			},
		},

		// ---- Write data operations
		map[string]interface{}{
			"action":     "/persistence.PersistenceService/InsertOne",
			"permission": "write",
			"resources": []interface{}{
				// InsertOneRqst.id / database / collection
				map[string]interface{}{"index": 0, "field": "Id", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "Database", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "Collection", "permission": "write"},
			},
		},
		map[string]interface{}{
			"action":     "/persistence.PersistenceService/InsertMany",
			"permission": "write",
			"resources": []interface{}{
				// InsertManyRqst.id / database / collection
				map[string]interface{}{"index": 0, "field": "Id", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "Database", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "Collection", "permission": "write"},
			},
		},
		map[string]interface{}{
			"action":     "/persistence.PersistenceService/Update",
			"permission": "write",
			"resources": []interface{}{
				// UpdateRqst.id / database / collection
				map[string]interface{}{"index": 0, "field": "Id", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "Database", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "Collection", "permission": "write"},
			},
		},
		map[string]interface{}{
			"action":     "/persistence.PersistenceService/UpdateOne",
			"permission": "write",
			"resources": []interface{}{
				// UpdateOneRqst.id / database / collection
				map[string]interface{}{"index": 0, "field": "Id", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "Database", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "Collection", "permission": "write"},
			},
		},
		map[string]interface{}{
			"action":     "/persistence.PersistenceService/ReplaceOne",
			"permission": "write",
			"resources": []interface{}{
				// ReplaceOneRqst.id / database / collection
				map[string]interface{}{"index": 0, "field": "Id", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "Database", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "Collection", "permission": "write"},
			},
		},
		map[string]interface{}{
			"action":     "/persistence.PersistenceService/Delete",
			"permission": "write",
			"resources": []interface{}{
				// DeleteRqst.id / database / collection
				map[string]interface{}{"index": 0, "field": "Id", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "Database", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "Collection", "permission": "write"},
			},
		},
		map[string]interface{}{
			"action":     "/persistence.PersistenceService/DeleteOne",
			"permission": "write",
			"resources": []interface{}{
				// DeleteOneRqst.id / database / collection
				map[string]interface{}{"index": 0, "field": "Id", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "Database", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "Collection", "permission": "write"},
			},
		},

		// ---- Admin commands on the store
		map[string]interface{}{
			"action":     "/persistence.PersistenceService/RunAdminCmd",
			"permission": "admin",
			"resources": []interface{}{
				// RunAdminCmdRqst.connectionId
				map[string]interface{}{"index": 0, "field": "ConnectionId", "permission": "admin"},
			},
		},
	}

	// Register client ctor
	Utility.RegisterFunction("NewPersistenceService_Client", persistence_client.NewPersistenceService_Client)

	// ---- CLI handling BEFORE config access ----
	args := os.Args[1:]
	if len(args) == 0 {
		srv.Id = Utility.GenerateUUID(srv.Name + ":" + srv.Address)
		allocator, err := config.NewDefaultPortAllocator()
		if err != nil {
			logger.Error("fail to create port allocator", "error", err)
			os.Exit(1)
		}
		p, err := allocator.Next(srv.Id)
		if err != nil {
			logger.Error("fail to allocate port", "error", err)
			os.Exit(1)
		}
		srv.Port = p
	}

	// Optional positional overrides (id, config path) if they don't start with '-'
	if len(args) == 1 && !strings.HasPrefix(args[0], "-") {
		srv.Id = args[0]
	} else if len(args) == 2 && !strings.HasPrefix(args[0], "-") && !strings.HasPrefix(args[1], "-") {
		srv.Id = args[0]
		srv.ConfigPath = args[1]
	}

	// Flags first (no etcd/config access here)
	for _, a := range args {
		switch strings.ToLower(a) {
		case "--describe":
			srv.Process = os.Getpid()
			srv.State = "starting"

			// Safe defaults for domain/address without etcd
			if v, ok := os.LookupEnv("GLOBULAR_DOMAIN"); ok && v != "" {
				srv.Domain = strings.ToLower(v)
			} else {
				srv.Domain = "localhost"
			}
			if v, ok := os.LookupEnv("GLOBULAR_ADDRESS"); ok && v != "" {
				srv.Address = strings.ToLower(v)
			} else {
				srv.Address = "localhost:" + Utility.ToString(srv.Port)
			}

			b, err := globular.DescribeJSON(srv)
			if err != nil {
				logger.Error("describe error", "service", srv.Name, "id", srv.Id, "err", err)
				os.Exit(2)
			}
			_, _ = os.Stdout.Write(b)
			_, _ = os.Stdout.Write([]byte("\n"))
			return

		case "--health":
			if srv.Port == 0 || srv.Name == "" {
				logger.Error("health error: uninitialized", "service", srv.Name, "port", srv.Port)
				os.Exit(2)
			}
			b, err := globular.HealthJSON(srv, &globular.HealthOptions{
				Timeout:     1500 * time.Millisecond,
				ServiceName: "",
			})
			if err != nil {
				logger.Error("health error", "service", srv.Name, "id", srv.Id, "err", err)
				os.Exit(2)
			}
			_, _ = os.Stdout.Write(b)
			_, _ = os.Stdout.Write([]byte("\n"))
			return
		case "--help", "-h", "/?":
			printUsage()
			return
		case "--debug":
			logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
				Level: slog.LevelDebug,
			}))
			slog.SetDefault(logger)
		}
	}

	// Now safe to access config (may read etcd / file fallback)
	if d, err := config.GetDomain(); err == nil && d != "" {
		srv.Domain = d
	}
	if a, err := config.GetAddress(); err == nil && strings.TrimSpace(a) != "" {
		srv.Address = a
	}

	// Init service
	start := time.Now()
	if err := srv.Init(); err != nil {
		logger.Error("service init failed", "service", srv.Name, "id", srv.Id, "err", err)
		os.Exit(1)
	}

	// Register RPCs + reflection
	persistencepb.RegisterPersistenceServiceServer(srv.grpcServer, srv)
	reflection.Register(srv.grpcServer)

	logger.Info("service ready",
		"service", srv.Name,
		"port", srv.Port,
		"proxy", srv.Proxy,
		"protocol", srv.Protocol,
		"domain", srv.Domain,
		"listen_ms", time.Since(start).Milliseconds(),
	)

	if err := srv.StartService(); err != nil {
		logger.Error("service start failed", "service", srv.Name, "id", srv.Id, "err", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, `Usage:
  %[1]s [service-id] [config-path] [--describe|--health|flags...]

Options:
  --describe    Print service metadata as JSON (no config/etcd access)
  --health      Print health status as JSON (no config/etcd access)

Positional arguments:
  service-id    Optional: Override the service instance ID
  config-path   Optional: Override the config file path

Environment variables:
  GLOBULAR_DOMAIN   Set the service domain (default: localhost)
  GLOBULAR_ADDRESS  Set the service address (default: localhost:<port>)

`, filepath.Base(os.Args[0]))
}
