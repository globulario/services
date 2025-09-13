// Package main implements the SQL gRPC service wired for Globular.
// It mirrors the modern structure (slog, --describe/--health, clean getters)
// shown in the Echo example while keeping SqlService registration intact.
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/resource/resourcepb"
	"github.com/globulario/services/golang/sql/sql_client"
	"github.com/globulario/services/golang/sql/sqlpb"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	// Drivers (keep as side-effect imports; add others as needed)
	_ "github.com/alexbrainman/odbc"
	_ "github.com/denisenkom/go-mssqldb"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
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
// Connection model
// -----------------------------------------------------------------------------

// connection holds a single database connection’s configuration.
type connection struct {
	Id       string
	Name     string
	Host     string
	Charset  string
	Driver   string
	User     string
	Password string
	Port     int32
	Path     string // path to sqlite3 database (directory)
}

// getConnectionString builds a DSN based on the driver and connection fields.
func (c *connection) getConnectionString() string {
	var dsn string

	switch c.Driver {
	case "mssql":
		dsn += "server=" + c.Host + ";"
		dsn += "user=" + c.User + ";"
		dsn += "password=" + c.Password + ";"
		dsn += "port=" + strconv.Itoa(int(c.Port)) + ";"
		dsn += "database=" + c.Name + ";"
		dsn += "driver=mssql;"
		dsn += "charset=" + c.Charset + ";"

	case "mysql":
		dsn += c.User + ":" + c.Password + "@tcp(" + c.Host + ":" + strconv.Itoa(int(c.Port)) + ")/" + c.Name
		dsn += "?charset=" + c.Charset + ";"

	case "postgres":
		// Note: original code used MySQL-style tcp string for postgres; keep behavior.
		dsn += c.User + ":" + c.Password + "@tcp(" + c.Host + ":" + strconv.Itoa(int(c.Port)) + ")/" + c.Name
		dsn += "?charset=" + c.Charset + ";"

	case "odbc":
		if runtime.GOOS == "windows" {
			dsn += "driver=sql server;"
		} else {
			dsn += "driver=freetds;"
		}
		dsn += "server=" + c.Host + ";"
		dsn += "database=" + c.Name + ";"
		dsn += "uid=" + c.User + ";"
		dsn += "pwd=" + c.Password + ";"
		dsn += "port=" + strconv.Itoa(int(c.Port)) + ";"
		dsn += "charset=" + c.Charset + ";"

	case "sqlite3":
		dsn += c.Path + string(os.PathSeparator) + c.Name

	default:
		// fall back to empty; higher layers should validate
	}

	return dsn
}

// -----------------------------------------------------------------------------
// Server
// -----------------------------------------------------------------------------

// server implements Globular service plumbing + SqlService RPCs.
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

	// Runtime
	grpcServer *grpc.Server

	// SQL connection registry
	Connections map[string]connection
}

// --- Globular service contract (getters/setters) ---

func (srv *server) GetConfigurationPath() string     { return srv.ConfigPath }
func (srv *server) SetConfigurationPath(path string) { srv.ConfigPath = path }

func (srv *server) GetAddress() string            { return srv.Address }
func (srv *server) SetAddress(address string)     { srv.Address = address }
func (srv *server) GetProcess() int               { return srv.Process }
func (srv *server) SetProcess(pid int)            { srv.Process = pid }
func (srv *server) GetProxyProcess() int          { return srv.ProxyProcess }
func (srv *server) SetProxyProcess(pid int)       { srv.ProxyProcess = pid }
func (srv *server) GetState() string              { return srv.State }
func (srv *server) SetState(state string)         { srv.State = state }
func (srv *server) GetLastError() string          { return srv.LastError }
func (srv *server) SetLastError(err string)       { srv.LastError = err }
func (srv *server) SetModTime(t int64)            { srv.ModTime = t }
func (srv *server) GetModTime() int64             { return srv.ModTime }
func (srv *server) GetId() string                 { return srv.Id }
func (srv *server) SetId(id string)               { srv.Id = id }
func (srv *server) GetName() string               { return srv.Name }
func (srv *server) SetName(name string)           { srv.Name = name }
func (srv *server) GetMac() string                { return srv.Mac }
func (srv *server) SetMac(mac string)             { srv.Mac = mac }
func (srv *server) GetDescription() string        { return srv.Description }
func (srv *server) SetDescription(d string)       { srv.Description = d }
func (srv *server) GetKeywords() []string         { return srv.Keywords }
func (srv *server) SetKeywords(k []string)        { srv.Keywords = k }
func (srv *server) GetRepositories() []string     { return srv.Repositories }
func (srv *server) SetRepositories(r []string)    { srv.Repositories = r }
func (srv *server) GetDiscoveries() []string      { return srv.Discoveries }
func (srv *server) SetDiscoveries(d []string)     { srv.Discoveries = d }
func (srv *server) Dist(p string) (string, error) { return globular.Dist(p, srv) }

func (srv *server) GetDependencies() []string {
	if srv.Dependencies == nil {
		srv.Dependencies = make([]string, 0)
	}
	return srv.Dependencies
}
func (srv *server) SetDependency(dep string) {
	if srv.Dependencies == nil {
		srv.Dependencies = make([]string, 0)
	}
	if !Utility.Contains(srv.Dependencies, dep) {
		srv.Dependencies = append(srv.Dependencies, dep)
	}
}

func (srv *server) GetChecksum() string             { return srv.Checksum }
func (srv *server) SetChecksum(cs string)           { srv.Checksum = cs }
func (srv *server) GetPlatform() string             { return srv.Plaform }
func (srv *server) SetPlatform(p string)            { srv.Plaform = p }
func (srv *server) GetPath() string                 { return srv.Path }
func (srv *server) SetPath(p string)                { srv.Path = p }
func (srv *server) GetProto() string                { return srv.Proto }
func (srv *server) SetProto(p string)               { srv.Proto = p }
func (srv *server) GetPort() int                    { return srv.Port }
func (srv *server) SetPort(port int)                { srv.Port = port }
func (srv *server) GetProxy() int                   { return srv.Proxy }
func (srv *server) SetProxy(proxy int)              { srv.Proxy = proxy }
func (srv *server) GetProtocol() string             { return srv.Protocol }
func (srv *server) SetProtocol(protocol string)     { srv.Protocol = protocol }
func (srv *server) GetAllowAllOrigins() bool        { return srv.AllowAllOrigins }
func (srv *server) SetAllowAllOrigins(v bool)       { srv.AllowAllOrigins = v }
func (srv *server) GetAllowedOrigins() string       { return srv.AllowedOrigins }
func (srv *server) SetAllowedOrigins(v string)      { srv.AllowedOrigins = v }
func (srv *server) GetDomain() string               { return srv.Domain }
func (srv *server) SetDomain(domain string)         { srv.Domain = domain }
func (srv *server) GetTls() bool                    { return srv.TLS }
func (srv *server) SetTls(v bool)                   { srv.TLS = v }
func (srv *server) GetCertAuthorityTrust() string   { return srv.CertAuthorityTrust }
func (srv *server) SetCertAuthorityTrust(ca string) { srv.CertAuthorityTrust = ca }
func (srv *server) GetCertFile() string             { return srv.CertFile }
func (srv *server) SetCertFile(cf string)           { srv.CertFile = cf }
func (srv *server) GetKeyFile() string              { return srv.KeyFile }
func (srv *server) SetKeyFile(kf string)            { srv.KeyFile = kf }
func (srv *server) GetVersion() string              { return srv.Version }
func (srv *server) SetVersion(v string)             { srv.Version = v }
func (srv *server) GetPublisherID() string          { return srv.PublisherID }
func (srv *server) SetPublisherID(id string)        { srv.PublisherID = id }
func (srv *server) GetKeepUpToDate() bool           { return srv.KeepUpToDate }
func (srv *server) SetKeepUptoDate(v bool)          { srv.KeepUpToDate = v }
func (srv *server) GetKeepAlive() bool              { return srv.KeepAlive }
func (srv *server) SetKeepAlive(v bool)             { srv.KeepAlive = v }
func (srv *server) GetPermissions() []interface{}   { return srv.Permissions }
func (srv *server) SetPermissions(p []interface{})  { srv.Permissions = p }

func (srv *server) RolesDefault() []resourcepb.Role {
	domain, _ := config.GetDomain()

	return []resourcepb.Role{
		{
			Id:          "role:sql.viewer",
			Name:        "SQL Viewer",
			Domain:      domain,
			Description: "Read-only access: ping connections and run SELECT queries.",
			Actions: []string{
				"/sql.SqlService/Ping",
				"/sql.SqlService/QueryContext",
			},
			TypeName: "resource.Role",
		},
		{
			Id:          "role:sql.writer",
			Name:        "SQL Writer",
			Domain:      domain,
			Description: "Read + write queries (INSERT/UPDATE/DELETE) on allowed connections.",
			Actions: []string{
				"/sql.SqlService/Ping",
				"/sql.SqlService/QueryContext",
				"/sql.SqlService/ExecContext",
			},
			TypeName: "resource.Role",
		},
		{
			Id:          "role:sql.admin",
			Name:        "SQL Admin",
			Domain:      domain,
			Description: "Full control over SQL connections and operations.",
			Actions: []string{
				"/sql.SqlService/Stop",
				"/sql.SqlService/CreateConnection",
				"/sql.SqlService/DeleteConnection",
				"/sql.SqlService/Ping",
				"/sql.SqlService/QueryContext",
				"/sql.SqlService/ExecContext",
			},
			TypeName: "resource.Role",
		},
	}
}

// Init initializes the service configuration and gRPC server.
func (srv *server) Init() error {
	if err := globular.InitService(srv); err != nil {
		return err
	}
	gs, err := globular.InitGrpcServer(srv)
	if err != nil {
		return err
	}
	srv.grpcServer = gs
	return nil
}

// Save persists the current configuration to disk.
func (srv *server) Save() error { return globular.SaveService(srv) }

// StartService starts the gRPC server (and proxy if configured).
func (srv *server) StartService() error { return globular.StartService(srv, srv.grpcServer) }

// StopService gracefully stops the gRPC server.
func (srv *server) StopService() error { return globular.StopService(srv, srv.grpcServer) }

// Stop stops the service via RPC.
func (srv *server) Stop(ctx context.Context, _ *sqlpb.StopRequest) (*sqlpb.StopResponse, error) {
	return &sqlpb.StopResponse{}, srv.StopService()
}

// -----------------------------------------------------------------------------
// Logging
// -----------------------------------------------------------------------------

var logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
	Level: slog.LevelInfo,
}))

// -----------------------------------------------------------------------------
// main
// -----------------------------------------------------------------------------

// main configures and starts the SQL service.
func main() {
	srv := new(server)
	srv.Connections = make(map[string]connection)

	// Fill fields that do NOT require etcd/config access yet.
	srv.Name = string(sqlpb.File_sql_proto.Services().Get(0).FullName())
	srv.Proto = sqlpb.File_sql_proto.Path()
	srv.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	srv.Port = defaultPort
	srv.Proxy = defaultProxy
	srv.Protocol = "grpc"
	srv.Version = "0.0.1"
	srv.PublisherID = "localhost"
	srv.Description = "SQL access microservice."
	srv.Keywords = []string{"SQL", "Database", "Service"}
	srv.Repositories = make([]string, 0)
	srv.Discoveries = make([]string, 0)
	srv.Dependencies = []string{ "rbac.RbacService", "log.LogService" }
	srv.Permissions = []interface{}{
		// ---- Stop the SQL service (service-level admin)
		map[string]interface{}{
			"action":     "/sql.SqlService/Stop",
			"permission": "admin",
			"resources":  []interface{}{},
		},

		// ---- Create a connection (admin)
		map[string]interface{}{
			"action":     "/sql.SqlService/CreateConnection",
			"permission": "admin",
			"resources": []interface{}{
				// CreateConnectionRqst.connection.id
				map[string]interface{}{"index": 0, "field": "Connection.Id", "permission": "admin"},
			},
		},

		// ---- Delete a connection (admin)
		map[string]interface{}{
			"action":     "/sql.SqlService/DeleteConnection",
			"permission": "admin",
			"resources": []interface{}{
				// DeleteConnectionRqst.id
				map[string]interface{}{"index": 0, "field": "Id", "permission": "admin"},
			},
		},

		// ---- Ping a connection (read)
		map[string]interface{}{
			"action":     "/sql.SqlService/Ping",
			"permission": "read",
			"resources": []interface{}{
				// PingConnectionRqst.id
				map[string]interface{}{"index": 0, "field": "Id", "permission": "read"},
			},
		},

		// ---- Query (read-only)
		map[string]interface{}{
			"action":     "/sql.SqlService/QueryContext",
			"permission": "read",
			"resources": []interface{}{
				// QueryContextRqst.query.connectionId
				map[string]interface{}{"index": 0, "field": "Query.ConnectionId", "permission": "read"},
			},
		},

		// ---- Exec (write)
		map[string]interface{}{
			"action":     "/sql.SqlService/ExecContext",
			"permission": "write",
			"resources": []interface{}{
				// ExecContextRqst.query.connectionId
				map[string]interface{}{"index": 0, "field": "Query.ConnectionId", "permission": "write"},
			},
		},
	}

	srv.Process = -1
	srv.ProxyProcess = -1
	srv.AllowAllOrigins = allowAllOrigins
	srv.AllowedOrigins = allowedOriginsStr
	srv.KeepAlive = true
	srv.KeepUpToDate = true

	// ---- CLI flags handled BEFORE any call that might touch etcd ----
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

	for _, a := range args {
		switch strings.ToLower(a) {
		case "--describe":
			// supply runtime/deducible info without etcd dependency
			srv.Process = os.Getpid()
			srv.State = "starting"

			if v, ok := os.LookupEnv("GLOBULAR_DOMAIN"); ok && strings.TrimSpace(v) != "" {
				srv.Domain = strings.ToLower(v)
			} else {
				srv.Domain = "localhost"
			}
			if v, ok := os.LookupEnv("GLOBULAR_ADDRESS"); ok && strings.TrimSpace(v) != "" {
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
			if srv.Port == 0 || strings.TrimSpace(srv.Name) == "" {
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
		}
	}

	// Optional positional args (unchanged behavior)
	if len(args) == 1 && !strings.HasPrefix(args[0], "-") {
		srv.Id = args[0]
	} else if len(args) == 2 && !strings.HasPrefix(args[0], "-") && !strings.HasPrefix(args[1], "-") {
		srv.Id = args[0]
		srv.ConfigPath = args[1]
	}

	// Safe to call config now
	if d, err := config.GetDomain(); err == nil {
		srv.Domain = d
	} else {
		srv.Domain = "localhost"
	}
	if a, err := config.GetAddress(); err == nil && strings.TrimSpace(a) != "" {
		srv.Address = a
	}

	// Client ctor registration
	Utility.RegisterFunction("NewSqlService_Client", sql_client.NewSqlService_Client)

	start := time.Now()
	if err := srv.Init(); err != nil {
		logger.Error("service init failed", "service", srv.Name, "id", srv.Id, "err", err)
		os.Exit(1)
	}

	// Register the SQL service implementation (RPCs implemented elsewhere in this package).
	sqlpb.RegisterSqlServiceServer(srv.grpcServer, srv)
	reflection.Register(srv.grpcServer)

	logger.Info("service ready",
		"service", srv.Name,
		"port", srv.Port,
		"proxy", srv.Proxy,
		"protocol", srv.Protocol,
		"domain", srv.Domain,
		"listen_ms", time.Since(start).Milliseconds())

	if err := srv.StartService(); err != nil {
		logger.Error("service start failed", "service", srv.Name, "id", srv.Id, "err", err)
		os.Exit(1)
	}
}

// printUsage mirrors the Echo sample’s user guidance.
func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  sql_server [service_id] [config_path]")
	fmt.Println("Options:")
	fmt.Println("  --describe    Print service metadata as JSON and exit")
	fmt.Println("  --health      Print service health as JSON and exit")
	fmt.Println("Examples:")
	fmt.Println("  sql_server my-sql-id /etc/globular/sql/config.json")
	fmt.Println("  sql_server --describe")
	fmt.Println("  sql_server --health")
}
