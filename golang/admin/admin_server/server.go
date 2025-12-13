package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/globulario/services/golang/admin/admin_client"
	"github.com/globulario/services/golang/admin/adminpb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/globular_client"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/rbac/rbac_client"
	"github.com/globulario/services/golang/resource/resourcepb"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// Defaults
var (
	defaultPort  = 10000
	defaultProxy = defaultPort + 1

	allowAllOrigins   = true
	allowedOriginsStr = ""

	srv *server

)

// Service impl
type server struct {
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
	AllowedOrigins  string
	Protocol        string
	Version         string
	PublisherID     string
	KeepUpToDate    bool
	Checksum        string
	Plaform         string
	KeepAlive       bool
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

	TLS                bool
	CertFile           string
	KeyFile            string
	CertAuthorityTrust string

	Permissions  []interface{}
	Dependencies []string

	// Admin-specific
	WebRoot          string
	ApplicationsRoot string

	grpcServer *grpc.Server
}

// --- Getters/Setters required by Globular (unchanged signatures) ---
func (srv *server) GetAddress() string               { return srv.Address }
func (srv *server) SetAddress(a string)              { srv.Address = a }
func (srv *server) GetProcess() int                  { return srv.Process }
func (srv *server) SetProcess(pid int)               { srv.Process = pid }
func (srv *server) GetProxyProcess() int             { return srv.ProxyProcess }
func (srv *server) SetProxyProcess(pid int)          { srv.ProxyProcess = pid }
func (srv *server) GetId() string                    { return srv.Id }
func (srv *server) SetId(id string)                  { srv.Id = id }
func (srv *server) GetName() string                  { return srv.Name }
func (srv *server) SetName(name string)              { srv.Name = name }
func (srv *server) GetMac() string                   { return srv.Mac }
func (srv *server) SetMac(mac string)                { srv.Mac = mac }
func (srv *server) GetChecksum() string              { return srv.Checksum }
func (srv *server) SetChecksum(c string)             { srv.Checksum = c }
func (srv *server) GetPlatform() string              { return srv.Plaform }
func (srv *server) SetPlatform(p string)             { srv.Plaform = p }
func (srv *server) GetState() string                 { return srv.State }
func (srv *server) SetState(st string)               { srv.State = st }
func (srv *server) GetDescription() string           { return srv.Description }
func (srv *server) SetDescription(d string)          { srv.Description = d }
func (srv *server) GetKeywords() []string            { return srv.Keywords }
func (srv *server) SetKeywords(k []string)           { srv.Keywords = k }
func (srv *server) GetRepositories() []string        { return srv.Repositories }
func (srv *server) SetRepositories(r []string)       { srv.Repositories = r }
func (srv *server) GetDiscoveries() []string         { return srv.Discoveries }
func (srv *server) SetDiscoveries(d []string)        { srv.Discoveries = d }
func (srv *server) GetLastError() string             { return srv.LastError }
func (srv *server) SetLastError(e string)            { srv.LastError = e }
func (srv *server) SetModTime(t int64)               { srv.ModTime = t }
func (srv *server) GetModTime() int64                { return srv.ModTime }
func (srv *server) GetConfigurationPath() string     { return srv.ConfigPath }
func (srv *server) SetConfigurationPath(p string)    { srv.ConfigPath = p }
func (srv *server) Dist(path string) (string, error) { return globular.Dist(path, srv) }
func (srv *server) GetDependencies() []string {
	if srv.Dependencies == nil {
		srv.Dependencies = []string{}
	}
	return srv.Dependencies
}
func (srv *server) SetDependency(dep string) {
	if srv.Dependencies == nil {
		srv.Dependencies = []string{}
	}
	if !Utility.Contains(srv.Dependencies, dep) {
		srv.Dependencies = append(srv.Dependencies, dep)
	}
}
func (srv *server) GetPath() string                { return srv.Path }
func (srv *server) SetPath(p string)               { srv.Path = p }
func (srv *server) GetProto() string               { return srv.Proto }
func (srv *server) SetProto(p string)              { srv.Proto = p }
func (srv *server) GetPort() int                   { return srv.Port }
func (srv *server) SetPort(port int)               { srv.Port = port }
func (srv *server) GetProxy() int                  { return srv.Proxy }
func (srv *server) SetProxy(px int)                { srv.Proxy = px }
func (srv *server) GetProtocol() string            { return srv.Protocol }
func (srv *server) SetProtocol(p string)           { srv.Protocol = p }
func (srv *server) GetAllowAllOrigins() bool       { return srv.AllowAllOrigins }
func (srv *server) SetAllowAllOrigins(b bool)      { srv.AllowAllOrigins = b }
func (srv *server) GetAllowedOrigins() string      { return srv.AllowedOrigins }
func (srv *server) SetAllowedOrigins(s string)     { srv.AllowedOrigins = s }
func (srv *server) GetDomain() string              { return srv.Domain }
func (srv *server) SetDomain(d string)             { srv.Domain = d }
func (srv *server) GetTls() bool                   { return srv.TLS }
func (srv *server) SetTls(b bool)                  { srv.TLS = b }
func (srv *server) GetCertAuthorityTrust() string  { return srv.CertAuthorityTrust }
func (srv *server) SetCertAuthorityTrust(c string) { srv.CertAuthorityTrust = c }
func (srv *server) GetCertFile() string            { return srv.CertFile }
func (srv *server) SetCertFile(c string)           { srv.CertFile = c }
func (srv *server) GetKeyFile() string             { return srv.KeyFile }
func (srv *server) SetKeyFile(k string)            { srv.KeyFile = k }
func (srv *server) GetVersion() string             { return srv.Version }
func (srv *server) SetVersion(v string)            { srv.Version = v }
func (srv *server) GetPublisherID() string         { return srv.PublisherID }
func (srv *server) SetPublisherID(p string)        { srv.PublisherID = p }
func (srv *server) GetKeepUpToDate() bool          { return srv.KeepUpToDate }
func (srv *server) SetKeepUptoDate(b bool)         { srv.KeepUpToDate = b }
func (srv *server) GetKeepAlive() bool             { return srv.KeepAlive }
func (srv *server) SetKeepAlive(b bool)            { srv.KeepAlive = b }
func (srv *server) GetPermissions() []interface{}  { return srv.Permissions }
func (srv *server) SetPermissions(p []interface{}) { srv.Permissions = p }

// Lifecycle
func (srv *server) Init() error {
	if err := globular.InitService(srv); err != nil {
		return err
	}
	gs, err := globular.InitGrpcServer(srv) // interceptors are wired inside
	if err != nil {
		return err
	}
	srv.grpcServer = gs
	return nil
}
func (srv *server) Save() error         { return globular.SaveService(srv) }
func (srv *server) StartService() error { return globular.StartService(srv, srv.grpcServer) }
func (srv *server) StopService() error  { return globular.StopService(srv, srv.grpcServer) }

// RolesDefault returns a curated set of roles for FileService.
func (srv *server) RolesDefault() []resourcepb.Role {
	domain, _ := config.GetDomain()

	return []resourcepb.Role{
		{
			Id:          "role:admin.process.viewer",
			Name:        "Process Viewer",
			Domain:      domain,
			Description: "View processes and PIDs; no control.",
			Actions: []string{
				"/admin.AdminService/HasRunningProcess",
				"/admin.AdminService/GetProcessInfos",
				"/admin.AdminService/GetPids",
			},
			TypeName: "resource.Role",
		},
		{
			Id:          "role:admin.process.operator",
			Name:        "Process Operator",
			Domain:      domain,
			Description: "Run commands and terminate processes.",
			Actions: []string{
				"/admin.AdminService/RunCmd",
				"/admin.AdminService/KillProcess",
				"/admin.AdminService/KillProcesses",
				// include viewer abilities
				"/admin.AdminService/HasRunningProcess",
				"/admin.AdminService/GetProcessInfos",
				"/admin.AdminService/GetPids",
			},
			TypeName: "resource.Role",
		},
		{
			Id:          "role:admin.env.manager",
			Name:        "Env Manager",
			Domain:      domain,
			Description: "Read, set, and unset environment variables.",
			Actions: []string{
				"/admin.AdminService/GetEnvironmentVariable",
				"/admin.AdminService/SetEnvironmentVariable",
				"/admin.AdminService/UnsetEnvironmentVariable",
			},
			TypeName: "resource.Role",
		},
		{
			Id:          "role:admin.config.editor",
			Name:        "Config Editor",
			Domain:      domain,
			Description: "Save server configuration.",
			Actions: []string{
				"/admin.AdminService/SaveConfig",
			},
			TypeName: "resource.Role",
		},
		{
			Id:          "role:admin.certs.manager",
			Name:        "Certificates Manager",
			Domain:      domain,
			Description: "Obtain/write TLS certificates for domains.",
			Actions: []string{
				"/admin.AdminService/GetCertificates",
			},
			TypeName: "resource.Role",
		},
		{
			Id:          "role:admin.dist.downloader",
			Name:        "Distribution Downloader",
			Domain:      domain,
			Description: "Download the Globular executable for a platform.",
			Actions: []string{
				"/admin.AdminService/DownloadGlobular",
			},
			TypeName: "resource.Role",
		},
		{
			Id:          "role:admin.updater",
			Name:        "Server Updater",
			Domain:      domain,
			Description: "Stream updates to Globular servers.",
			Actions: []string{
				"/admin.AdminService/Update",
			},
			TypeName: "resource.Role",
		},
		{
			Id:          "role:admin.hosts.viewer",
			Name:        "Hosts Viewer",
			Domain:      domain,
			Description: "List available hosts on the network.",
			Actions: []string{
				"/admin.AdminService/GetAvailableHosts",
			},
			TypeName: "resource.Role",
		},
		{
			Id:          "role:admin.fileinfo.viewer",
			Name:        "Admin FileInfo Viewer",
			Domain:      domain,
			Description: "Read file info via AdminService (host-level path).",
			Actions: []string{
				"/admin.AdminService/GetFileInfo",
			},
			TypeName: "resource.Role",
		},
		{
			Id:          "role:admin.admin",
			Name:        "AdminService Admin",
			Domain:      domain,
			Description: "Full control over AdminService.",
			Actions: []string{
				"/admin.AdminService/Update",
				"/admin.AdminService/DownloadGlobular",
				"/admin.AdminService/GetCertificates",
				"/admin.AdminService/HasRunningProcess",
				"/admin.AdminService/GetProcessInfos",
				"/admin.AdminService/RunCmd",
				"/admin.AdminService/SetEnvironmentVariable",
				"/admin.AdminService/GetEnvironmentVariable",
				"/admin.AdminService/UnsetEnvironmentVariable",
				"/admin.AdminService/KillProcess",
				"/admin.AdminService/KillProcesses",
				"/admin.AdminService/GetPids",
				"/admin.AdminService/SaveConfig",
				"/admin.AdminService/GetFileInfo",
				"/admin.AdminService/GetAvailableHosts",
			},
			TypeName: "resource.Role",
		},
	}
}

// Optional helpers
func GetRbacClient(address string) (*rbac_client.Rbac_Client, error) {
	Utility.RegisterFunction("NewRbacService_Client", rbac_client.NewRbacService_Client)
	client, err := globular_client.GetClient(address, "rbac.RbacService", "NewRbacService_Client")
	if err != nil {
		return nil, err
	}
	return client.(*rbac_client.Rbac_Client), nil
}

// --- logger to STDERR so stdout stays clean for JSON outputs ---
var logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
	Level: slog.LevelInfo,
}))

func main() {

	srv = new(server)

	// Fill ONLY fields that do NOT touch etcd/config yet
	srv.Name = string(adminpb.File_admin_proto.Services().Get(0).FullName())
	srv.Proto = adminpb.File_admin_proto.Path()
	srv.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	srv.Port = defaultPort
	srv.Proxy = defaultProxy
	srv.Protocol = "grpc"
	srv.Version = "0.0.1"
	srv.PublisherID = "localhost"
	srv.Description = "Admin service must be used with privilege"
	srv.Keywords = []string{"Manager", "Administrator", "Admin"}
	srv.Repositories = []string{}
	srv.Discoveries = []string{}
	srv.Dependencies = []string{"rbac.RbacService"}
	srv.Permissions = []interface{}{
		// --- Software distribution / updates
		map[string]interface{}{"action": "/admin.AdminService/Update",
			// No resource path arg; the action itself is sensitive.
			"permission": "write",
		},
		map[string]interface{}{"action": "/admin.AdminService/DownloadGlobular",
			// Reading the server binary is sensitive but has no resource arg.
			"permission": "read",
		},

		// --- Certificates
		map[string]interface{}{"action": "/admin.AdminService/GetCertificates",
			// GetCertificatesRequest.domain (resource to obtain a cert for)
			// GetCertificatesRequest.path   (filesystem path where certs will be written)
			"permission": "write", // overall capability
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "permission": "write"}, // domain as resource
				map[string]interface{}{"index": 2, "permission": "write"}, // path to write keys/certs
			},
		},

		// --- Processes: inspection / control
		map[string]interface{}{"action": "/admin.AdminService/HasRunningProcess", "resources": []interface{}{
			// HasRunningProcessRequest.name
			map[string]interface{}{"index": 0, "permission": "read"},
		}},
		map[string]interface{}{"action": "/admin.AdminService/GetProcessInfos", "resources": []interface{}{
			// GetProcessInfosRequest.name
			map[string]interface{}{"index": 0, "permission": "read"},
		}},
		map[string]interface{}{"action": "/admin.AdminService/RunCmd",
			// Running arbitrary commands is inherently dangerous.
			"permission": "execute",
			"resources": []interface{}{
				// RunCmdRequest.path (cwd)
				map[string]interface{}{"index": 3, "permission": "execute"},
			},
		},
		map[string]interface{}{"action": "/admin.AdminService/KillProcess", "resources": []interface{}{
			// KillProcessRequest.pid
			map[string]interface{}{"index": 0, "permission": "delete"},
		}},
		map[string]interface{}{"action": "/admin.AdminService/KillProcesses", "resources": []interface{}{
			// KillProcessesRequest.name
			map[string]interface{}{"index": 0, "permission": "delete"},
		}},
		map[string]interface{}{"action": "/admin.AdminService/GetPids", "resources": []interface{}{
			// GetPidsRequest.name
			map[string]interface{}{"index": 0, "permission": "read"},
		}},

		// --- Environment variables
		map[string]interface{}{"action": "/admin.AdminService/SetEnvironmentVariable", "resources": []interface{}{
			// SetEnvironmentVariableRequest.name
			map[string]interface{}{"index": 0, "permission": "write"},
		}},
		map[string]interface{}{"action": "/admin.AdminService/GetEnvironmentVariable", "resources": []interface{}{
			// GetEnvironmentVariableRequest.name
			map[string]interface{}{"index": 0, "permission": "read"},
		}},
		map[string]interface{}{"action": "/admin.AdminService/UnsetEnvironmentVariable", "resources": []interface{}{
			// UnsetEnvironmentVariableRequest.name
			map[string]interface{}{"index": 0, "permission": "delete"},
		}},

		// --- Config
		map[string]interface{}{"action": "/admin.AdminService/SaveConfig",
			// Opaque blob; protect the action itself.
			"permission": "write",
		},

		// --- File info on host
		map[string]interface{}{"action": "/admin.AdminService/GetFileInfo", "resources": []interface{}{
			// GetFileInfoRequest.path
			map[string]interface{}{"index": 0, "permission": "read"},
		}},

		// --- Network inventory
		map[string]interface{}{"action": "/admin.AdminService/GetAvailableHosts",
			// No request args; scanning/listing hosts is privileged.
			"resources":  []interface{}{},
			"permission": "list",
		},
	}

	srv.Process = -1
	srv.ProxyProcess = -1
	srv.KeepAlive = true
	srv.KeepUpToDate = true
	srv.AllowAllOrigins = allowAllOrigins
	srv.AllowedOrigins = allowedOriginsStr

	// ---- CLI flags handled BEFORE any call that might touch etcd ----
	args := os.Args[1:]
	if len(args) == 0 {
		srv.Id = Utility.GenerateUUID(srv.Name + ":" + srv.Address)
		allocator, err := config.NewDefaultPortAllocator()
		if err != nil {
			fmt.Println("fail to create port allocator", "error", err)
			os.Exit(1)
		}
		p, err := allocator.Next(srv.Id)
		if err != nil {
			fmt.Println("fail to allocate port", "error", err)
			os.Exit(1)
		}
		srv.Port = p
	}

	for _, a := range args {
		switch strings.ToLower(a) {
		case "--describe":
			// runtime snapshot without etcd
			srv.Process = os.Getpid()
			srv.State = "starting"

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
			os.Stdout.Write(b)
			os.Stdout.Write([]byte("\n"))
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
			os.Stdout.Write(b)
			os.Stdout.Write([]byte("\n"))
			return
		case "--help", "-h", "/?":
			printUsage()
			return
		case "--debug":
			logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
				Level: slog.LevelDebug,
			}))
			slog.SetDefault(logger)
		case "--version", "-v":
			fmt.Fprintf(os.Stdout, "%s\n", srv.Version)
		}
	}

	// Optional positional args: <id> [configPath]
	if len(args) == 1 && !strings.HasPrefix(args[0], "-") {
		srv.Id = args[0]
	} else if len(args) == 2 && !strings.HasPrefix(args[0], "-") && !strings.HasPrefix(args[1], "-") {
		srv.Id = args[0]
		srv.ConfigPath = args[1]
	}

	// Safe to touch config now (etcd/file fallback inside config)
	if d, err := config.GetDomain(); err == nil {
		srv.Domain = d
	} else {
		srv.Domain = "localhost"
	}
	if a, err := config.GetAddress(); err == nil && strings.TrimSpace(a) != "" {
		srv.Address = a
	}
	// Admin-specific dirs
	srv.WebRoot = config.GetWebRootDir()
	srv.ApplicationsRoot = filepath.Join(config.GetDataDir(), "applications")

	// Register client ctor (for dynamic routing usage elsewhere)
	Utility.RegisterFunction("NewAdminService_Client", admin_client.NewAdminService_Client)

	start := time.Now()
	if err := srv.Init(); err != nil {
		logger.Error("service init failed", "service", srv.Name, "id", srv.Id, "err", err)
		os.Exit(1)
	}

	adminpb.RegisterAdminServiceServer(srv.grpcServer, srv)
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

func printUsage() {
	fmt.Fprintf(os.Stdout, `
Usage: %s [options] <id> [configPath]

Options:
  --describe      Print service description as JSON (no etcd/config access)
  --health        Print service health as JSON (no etcd/config access)

Arguments:
  <id>            Service instance ID
  [configPath]    Optional path to configuration file

Example:
  %s my-admin-id /etc/globular/admin/config.json

`, filepath.Base(os.Args[0]), filepath.Base(os.Args[0]))
}
