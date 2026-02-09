package main

import (
	"context"
	"os"
	"runtime"
	"strconv"

	"github.com/globulario/services/golang/config"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/resource/resourcepb"
	"github.com/globulario/services/golang/sql/sqlpb"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc"
)

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
	}

	return dsn
}

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

// Globular service contract (getters/setters)
func (srv *server) GetConfigurationPath() string     { return srv.ConfigPath }
func (srv *server) SetConfigurationPath(path string) { srv.ConfigPath = path }
func (srv *server) GetAddress() string               { return srv.Address }
func (srv *server) SetAddress(address string)        { srv.Address = address }
func (srv *server) GetProcess() int                  { return srv.Process }
func (srv *server) SetProcess(pid int)               { srv.Process = pid }
func (srv *server) GetProxyProcess() int             { return srv.ProxyProcess }
func (srv *server) SetProxyProcess(pid int)          { srv.ProxyProcess = pid }
func (srv *server) GetState() string                 { return srv.State }
func (srv *server) SetState(state string)            { srv.State = state }
func (srv *server) GetLastError() string             { return srv.LastError }
func (srv *server) SetLastError(err string)          { srv.LastError = err }
func (srv *server) SetModTime(t int64)               { srv.ModTime = t }
func (srv *server) GetModTime() int64                { return srv.ModTime }
func (srv *server) GetId() string                    { return srv.Id }
func (srv *server) SetId(id string)                  { srv.Id = id }
func (srv *server) GetName() string                  { return srv.Name }
func (srv *server) SetName(name string)              { srv.Name = name }
func (srv *server) GetMac() string                   { return srv.Mac }
func (srv *server) SetMac(mac string)                { srv.Mac = mac }
func (srv *server) GetDescription() string           { return srv.Description }
func (srv *server) SetDescription(d string)          { srv.Description = d }
func (srv *server) GetKeywords() []string            { return srv.Keywords }
func (srv *server) SetKeywords(k []string)           { srv.Keywords = k }
func (srv *server) GetRepositories() []string        { return srv.Repositories }
func (srv *server) SetRepositories(r []string)       { srv.Repositories = r }
func (srv *server) GetDiscoveries() []string         { return srv.Discoveries }
func (srv *server) SetDiscoveries(d []string)        { srv.Discoveries = d }
func (srv *server) Dist(p string) (string, error)    { return globular.Dist(p, srv) }
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
			Description: "Read + write queries on allowed connections.",
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

func (srv *server) Save() error { return globular.SaveService(srv) }

func (srv *server) StartService() error { return globular.StartService(srv, srv.grpcServer) }

func (srv *server) StopService() error { return globular.StopService(srv, srv.grpcServer) }

func (srv *server) Stop(ctx context.Context, _ *sqlpb.StopRequest) (*sqlpb.StopResponse, error) {
	return &sqlpb.StopResponse{}, srv.StopService()
}

func (srv *server) GetGrpcServer() *grpc.Server { return srv.grpcServer }
