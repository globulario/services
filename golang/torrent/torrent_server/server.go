// Package main wires the Torrent gRPC service into the Globular runtime.
// It follows the same layout as the "echo" reference: structured logging via
// slog, --describe/--health fast‑paths, clear errors, and documented public
// methods that satisfy Globular's service contract.
package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/anacrolix/torrent"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/event/event_client"
	"github.com/globulario/services/golang/globular_client"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/rbac/rbac_client"
	"github.com/globulario/services/golang/rbac/rbacpb"
	"github.com/globulario/services/golang/resource/resourcepb"
	"github.com/globulario/services/golang/torrent/torrent_client"
	"github.com/globulario/services/golang/torrent/torrentpb"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
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

// logger is a process-wide structured logger.
var logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

// -----------------------------------------------------------------------------
// server implements Globular plumbing + Torrent runtime fields
// -----------------------------------------------------------------------------

type server struct {
	// Core metadata
	Id          string
	Mac         string
	Name        string
	Domain      string
	Address     string
	Path        string
	Proto       string
	Port        int
	Proxy       int
	Protocol    string
	Version     string
	PublisherID string
	Description string
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

	// Torrent-specific runtime
	DownloadDir string       // where files are downloaded before being copied
	Seed        bool         // keep seeding after completion
	torrent_client_ *torrent.Client
	actions         chan map[string]interface{} // internal action bus
	done            chan bool                   // shutdown signal
}

// -----------------------------------------------------------------------------
// Globular service contract (getters/setters) — documented and consistent
// -----------------------------------------------------------------------------

// GetConfigurationPath returns the path to this service's configuration file.
func (srv *server) GetConfigurationPath() string { return srv.ConfigPath }

// SetConfigurationPath sets the path to this service's configuration file.
func (srv *server) SetConfigurationPath(path string) { srv.ConfigPath = path }

// GetAddress returns the HTTP address where /config can be reached.
func (srv *server) GetAddress() string { return srv.Address }

// SetAddress sets the HTTP address where /config can be reached.
func (srv *server) SetAddress(address string) { srv.Address = address }

// GetProcess returns the process id of the service, or -1 if not started.
func (srv *server) GetProcess() int { return srv.Process }

// SetProcess records the process id of the service.
func (srv *server) SetProcess(pid int) { srv.Process = pid }

// GetProxyProcess returns the reverse-proxy process id, or -1 if not started.
func (srv *server) GetProxyProcess() int { return srv.ProxyProcess }

// SetProxyProcess records the reverse-proxy process id.
func (srv *server) SetProxyProcess(pid int) { srv.ProxyProcess = pid }

// GetState returns the current service state (e.g., "running").
func (srv *server) GetState() string { return srv.State }

// SetState updates the current service state.
func (srv *server) SetState(state string) { srv.State = state }

// GetLastError returns the last error message recorded by the service.
func (srv *server) GetLastError() string { return srv.LastError }

// SetLastError records the last error message.
func (srv *server) SetLastError(err string) { srv.LastError = err }

// SetModTime sets the last modification time (unix seconds).
func (srv *server) SetModTime(modtime int64) { srv.ModTime = modtime }

// GetModTime returns the last modification time (unix seconds).
func (srv *server) GetModTime() int64 { return srv.ModTime }

// GetId returns the unique id of this service instance.
func (srv *server) GetId() string { return srv.Id }

// SetId sets the unique id of this service instance.
func (srv *server) SetId(id string) { srv.Id = id }

// GetName returns the gRPC service name.
func (srv *server) GetName() string { return srv.Name }

// SetName sets the gRPC service name.
func (srv *server) SetName(name string) { srv.Name = name }

// GetDescription returns the service description.
func (srv *server) GetDescription() string { return srv.Description }

// SetDescription sets the service description.
func (srv *server) SetDescription(description string) { srv.Description = description }

// GetMac returns the MAC address of the host (if set by the platform).
func (srv *server) GetMac() string { return srv.Mac }

// SetMac sets the MAC address of the host.
func (srv *server) SetMac(mac string) { srv.Mac = mac }

// GetKeywords returns the service keywords.
func (srv *server) GetKeywords() []string { return srv.Keywords }

// SetKeywords sets the service keywords.
func (srv *server) SetKeywords(keywords []string) { srv.Keywords = keywords }

// GetRepositories returns repositories associated with the service.
func (srv *server) GetRepositories() []string { return srv.Repositories }

// SetRepositories sets repositories associated with the service.
func (srv *server) SetRepositories(repositories []string) { srv.Repositories = repositories }

// GetDiscoveries returns discovery endpoints for the service.
func (srv *server) GetDiscoveries() []string { return srv.Discoveries }

// SetDiscoveries sets discovery endpoints for the service.
func (srv *server) SetDiscoveries(discoveries []string) { srv.Discoveries = discoveries }

// Dist packages (distributes) the service into the given path using Globular.
func (srv *server) Dist(path string) (string, error) { return globular.Dist(path, srv) }

// GetDependencies returns the list of dependent services.
func (srv *server) GetDependencies() []string {
	if srv.Dependencies == nil {
		srv.Dependencies = make([]string, 0)
	}
	return srv.Dependencies
}

// SetDependency appends a dependency if it is not already present.
func (srv *server) SetDependency(dependency string) {
	if srv.Dependencies == nil {
		srv.Dependencies = make([]string, 0)
	}
	if !Utility.Contains(srv.Dependencies, dependency) {
		srv.Dependencies = append(srv.Dependencies, dependency)
	}
}

// GetChecksum returns the binary checksum.
func (srv *server) GetChecksum() string { return srv.Checksum }

// SetChecksum sets the binary checksum.
func (srv *server) SetChecksum(checksum string) { srv.Checksum = checksum }

// GetPlatform returns the service platform (e.g., "linux/amd64").
func (srv *server) GetPlatform() string { return srv.Plaform }

// SetPlatform sets the service platform (e.g., "linux/amd64").
func (srv *server) SetPlatform(platform string) { srv.Plaform = platform }

// GetPath returns the executable path.
func (srv *server) GetPath() string { return srv.Path }

// SetPath sets the executable path.
func (srv *server) SetPath(path string) { srv.Path = path }

// GetProto returns the path to the .proto file.
func (srv *server) GetProto() string { return srv.Proto }

// SetProto sets the path to the .proto file.
func (srv *server) SetProto(proto string) { srv.Proto = proto }

// GetPort returns the gRPC port.
func (srv *server) GetPort() int { return srv.Port }

// SetPort sets the gRPC port.
func (srv *server) SetPort(port int) { srv.Port = port }

// GetProxy returns the reverse proxy port (for gRPC-Web).
func (srv *server) GetProxy() int { return srv.Proxy }

// SetProxy sets the reverse proxy port (for gRPC-Web).
func (srv *server) SetProxy(proxy int) { srv.Proxy = proxy }

// GetProtocol returns the network protocol (e.g., "grpc", "tls", "https").
func (srv *server) GetProtocol() string { return srv.Protocol }

// SetProtocol sets the network protocol (e.g., "grpc", "tls", "https").
func (srv *server) SetProtocol(protocol string) { srv.Protocol = protocol }

// GetAllowAllOrigins returns whether all origins are allowed.
func (srv *server) GetAllowAllOrigins() bool { return srv.AllowAllOrigins }

// SetAllowAllOrigins toggles whether all origins are allowed.
func (srv *server) SetAllowAllOrigins(allowAllOrigins bool) { srv.AllowAllOrigins = allowAllOrigins }

// GetAllowedOrigins returns the comma-separated list of allowed origins.
func (srv *server) GetAllowedOrigins() string { return srv.AllowedOrigins }

// SetAllowedOrigins sets the comma-separated list of allowed origins.
func (srv *server) SetAllowedOrigins(allowedOrigins string) { srv.AllowedOrigins = allowedOrigins }

// GetDomain returns the configured domain (ip or DNS name).
func (srv *server) GetDomain() string { return srv.Domain }

// SetDomain sets the configured domain (ip or DNS name).
func (srv *server) SetDomain(domain string) { srv.Domain = domain }

// GetTls returns true when TLS is enabled.
func (srv *server) GetTls() bool { return srv.TLS }

// SetTls enables or disables TLS.
func (srv *server) SetTls(hasTls bool) { srv.TLS = hasTls }

// GetCertAuthorityTrust returns the CA bundle path.
func (srv *server) GetCertAuthorityTrust() string { return srv.CertAuthorityTrust }

// SetCertAuthorityTrust sets the CA bundle path.
func (srv *server) SetCertAuthorityTrust(ca string) { srv.CertAuthorityTrust = ca }

// GetCertFile returns the TLS certificate path.
func (srv *server) GetCertFile() string { return srv.CertFile }

// SetCertFile sets the TLS certificate path.
func (srv *server) SetCertFile(certFile string) { srv.CertFile = certFile }

// GetKeyFile returns the TLS private key path.
func (srv *server) GetKeyFile() string { return srv.KeyFile }

// SetKeyFile sets the TLS private key path.
func (srv *server) SetKeyFile(keyFile string) { srv.KeyFile = keyFile }

// GetVersion returns the service version.
func (srv *server) GetVersion() string { return srv.Version }

// SetVersion sets the service version.
func (srv *server) SetVersion(version string) { srv.Version = version }

// GetPublisherID returns the publisher ID.
func (srv *server) GetPublisherID() string { return srv.PublisherID }

// SetPublisherID sets the publisher ID.
func (srv *server) SetPublisherID(PublisherID string) { srv.PublisherID = PublisherID }

// GetKeepUpToDate returns whether auto-updates are enabled.
func (srv *server) GetKeepUpToDate() bool { return srv.KeepUpToDate }

// SetKeepUptoDate toggles auto-updates.
func (srv *server) SetKeepUptoDate(val bool) { srv.KeepUpToDate = val }

// GetKeepAlive returns whether the service should be kept alive by the supervisor.
func (srv *server) GetKeepAlive() bool { return srv.KeepAlive }

// SetKeepAlive toggles keep-alive behavior.
func (srv *server) SetKeepAlive(val bool) { srv.KeepAlive = val }

// GetPermissions returns the action permissions configured for this service.
func (srv *server) GetPermissions() []interface{} { return srv.Permissions }

// SetPermissions sets the action permissions for this service.
func (srv *server) SetPermissions(permissions []interface{}) { srv.Permissions = permissions }

// Default roles for the Torrent service.
func (srv *server) RolesDefault() []resourcepb.Role {
	domain, _ := config.GetDomain()

	view := []string{
		"/torrent.TorrentService/GetTorrentInfos",
		"/torrent.TorrentService/GetTorrentLnks",
	}

	write := append([]string{
		"/torrent.TorrentService/DownloadTorrent",
		"/torrent.TorrentService/DropTorrent",
	}, view...) // writers can also view

	// No extra admin-only RPCs in this proto; admin includes all.
	admin := append([]string{}, write...)

	return []resourcepb.Role{
		{
			Id:          "role:torrent.viewer",
			Name:        "Torrent Viewer",
			Domain:      domain,
			Description: "Read-only: list saved links and stream torrent progress.",
			Actions:     view,
			TypeName:    "resource.Role",
		},
		{
			Id:          "role:torrent.user",
			Name:        "Torrent User",
			Domain:      domain,
			Description: "Start downloads/seeding, drop torrents, and view progress.",
			Actions:     write,
			TypeName:    "resource.Role",
		},
		{
			Id:          "role:torrent.admin",
			Name:        "Torrent Admin",
			Domain:      domain,
			Description: "Full control over torrent operations.",
			Actions:     admin,
			TypeName:    "resource.Role",
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

// StartService begins serving gRPC (and proxy if configured).
func (srv *server) StartService() error { return globular.StartService(srv, srv.grpcServer) }

// StopService gracefully stops the running gRPC server.
func (srv *server) StopService() error { return globular.StopService(srv, srv.grpcServer) }

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


// getEventClient returns a connected Event client.
func (srv *server) getEventClient() (*event_client.Event_Client, error) {
	Utility.RegisterFunction("NewEventService_Client", event_client.NewEventService_Client)
	c, err := globular_client.GetClient(srv.Address, "event.EventService", "NewEventService_Client")
	if err != nil {
		return nil, err
	}
	return c.(*event_client.Event_Client), nil
}


// -----------------------------------------------------------------------------
// main — follows the "echo" server layout with --describe and --health
// -----------------------------------------------------------------------------

func main() {
	srv := new(server)

	// Fill ONLY fields that do NOT call into config/etcd yet.
	srv.Name = string(torrentpb.File_torrent_proto.Services().Get(0).FullName())
	srv.Proto = torrentpb.File_torrent_proto.Path()
	srv.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	srv.Port = defaultPort
	srv.Proxy = defaultProxy
	srv.Protocol = "grpc"
	srv.Version = "0.0.1"
	srv.PublisherID = "localhost"
	srv.Description = "Torrent gRPC service for Globular."
	srv.Keywords = []string{"Torrent", "Download", "P2P", "Service"}
	srv.Repositories = make([]string, 0)
	srv.Discoveries = make([]string, 0)
	srv.Dependencies = make([]string, 0)

	srv.Permissions = []interface{}{
		// ---- Start a new torrent (download or seed)
		map[string]interface{}{
			"action":     "/torrent.TorrentService/DownloadTorrent",
			"permission": "write",
			"resources": []interface{}{
				// DownloadTorrentRequest.link
				map[string]interface{}{"index": 0, "field": "Link", "permission": "write"},
				// DownloadTorrentRequest.dest
				map[string]interface{}{"index": 0, "field": "Dest", "permission": "write"},
				// DownloadTorrentRequest.seed
				map[string]interface{}{"index": 0, "field": "Seed", "permission": "write"},
			},
		},

		// ---- Stream active torrent infos
		map[string]interface{}{
			"action":     "/torrent.TorrentService/GetTorrentInfos",
			"permission": "read",
			"resources":  []interface{}{},
		},

		// ---- Drop (remove) a torrent
		map[string]interface{}{
			"action":     "/torrent.TorrentService/DropTorrent",
			"permission": "write",
			"resources": []interface{}{
				// DropTorrentRequest.name
				map[string]interface{}{"index": 0, "field": "Name", "permission": "write"},
			},
		},

		// ---- List saved torrent links
		map[string]interface{}{
			"action":     "/torrent.TorrentService/GetTorrentLnks",
			"permission": "read",
			"resources":  []interface{}{},
		},
	}
	srv.Process = -1
	srv.ProxyProcess = -1
	srv.AllowAllOrigins = allowAllOrigins
	srv.AllowedOrigins = allowedOriginsStr
	srv.KeepAlive = true
	srv.KeepUpToDate = true
	srv.DownloadDir = config.GetDataDir() + "/torrents"
	srv.Seed = false

	// Register public client ctor for other services
	Utility.RegisterFunction("NewTorrentService_Client", torrent_client.NewTorrentService_Client)

	// ---- CLI flags handled BEFORE any call that might touch etcd ----
	args := os.Args[1:]

	if len(args) == 0 {
		printUsage()
		return
	}

	for _, a := range args {
		switch strings.ToLower(a) {
		case "--describe":
			// best-effort runtime fields without hitting etcd
			srv.Process = os.Getpid()
			srv.State = "starting"

			// Provide defaults that don’t require etcd
			if v, ok := os.LookupEnv("GLOBULAR_DOMAIN"); ok && v != "" { srv.Domain = strings.ToLower(v) } else { srv.Domain = "localhost" }
			if v, ok := os.LookupEnv("GLOBULAR_ADDRESS"); ok && v != "" { srv.Address = strings.ToLower(v) } else { srv.Address = "localhost:" + Utility.ToString(srv.Port) }

			b, err := globular.DescribeJSON(srv)
			if err != nil {
				logger.Error("describe error", "service", srv.Name, "id", srv.Id, "err", err)
				os.Exit(2)
			}
			_, _ = os.Stdout.Write(b); _, _ = os.Stdout.Write([]byte("\n"))
			return

		case "--health":
			if srv.Port == 0 || srv.Name == "" {
				logger.Error("health error: uninitialized", "service", srv.Name, "port", srv.Port)
				os.Exit(2)
			}
			b, err := globular.HealthJSON(srv, &globular.HealthOptions{Timeout: 1500 * time.Millisecond})
			if err != nil {
				logger.Error("health error", "service", srv.Name, "id", srv.Id, "err", err)
				os.Exit(2)
			}
			_, _ = os.Stdout.Write(b); _, _ = os.Stdout.Write([]byte("\n"))
			return
		}
	}

	// Optional positional args (unchanged)
	if len(args) == 1 && !strings.HasPrefix(args[0], "-") {
		srv.Id = args[0]
	} else if len(args) == 2 && !strings.HasPrefix(args[0], "-") && !strings.HasPrefix(args[1], "-") {
		srv.Id = args[0]
		srv.ConfigPath = args[1]
	}

	// Now it’s safe to read local config (may use etcd or file fallback)
	if d, err := config.GetDomain(); err == nil { srv.Domain = d } else { srv.Domain = "localhost" }
	if a, err := config.GetAddress(); err == nil && strings.TrimSpace(a) != "" { srv.Address = a }

	start := time.Now()
	if err := srv.Init(); err != nil {
		logger.Error("service init failed", "service", srv.Name, "id", srv.Id, "err", err)
		os.Exit(1)
	}

	// gRPC wiring
	torrentpb.RegisterTorrentServiceServer(srv.grpcServer, srv)
	reflection.Register(srv.grpcServer)

	// Action channels and permissions
	srv.actions = make(chan map[string]interface{})
	srv.done = make(chan bool)
	srv.Permissions = append(srv.Permissions, map[string]interface{}{
		"action": "/torrent.TorrentService/DownloadTorrentRequest",
		"resources": []interface{}{map[string]interface{}{"index": 1, "permission": "write"}},
	})

	// Torrent client init
	if err := Utility.CreateDirIfNotExist(srv.DownloadDir); err != nil {
		logger.Error("ensure download dir failed", "dir", srv.DownloadDir, "err", err)
		os.Exit(1)
	}
	cfg := torrent.NewDefaultClientConfig()
	cfg.DataDir = srv.DownloadDir
	cfg.Seed = srv.Seed
	client, err := torrent.NewClient(cfg)
	if err != nil {
		logger.Error("torrent client start failed", "err", err)
		os.Exit(1)
	}
	srv.torrent_client_ = client

	// Start the torrent processor loop (defined in torrent.go).
	srv.processTorrent()

	// Resume previously saved links asynchronously.
	go func() {
		lnks, err := srv.readTorrentLnks()
		if err != nil {
			logger.Warn("read saved torrent links failed", "err", err)
			return
		}
		for _, lnk := range lnks {
			logger.Info("resuming torrent", "name", lnk.Name)
			if err := srv.downloadTorrent(lnk.Lnk, lnk.Dir, lnk.Seed, lnk.Owner); err != nil {
				logger.Error("resume torrent failed", "name", lnk.Name, "err", err)
			}
		}
	}()

	logger.Info("service ready",
		"service", srv.Name,
		"port", srv.Port,
		"proxy", srv.Proxy,
		"protocol", srv.Protocol,
		"domain", srv.Domain,
		"download_dir", srv.DownloadDir,
		"listen_ms", time.Since(start).Milliseconds(),
	)

	if err := srv.StartService(); err != nil {
		logger.Error("service start failed", "service", srv.Name, "id", srv.Id, "err", err)
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  torrent_server [service_id] [config_path]")
	fmt.Println("Options:")
	fmt.Println("  --describe    Print service metadata as JSON and exit")
	fmt.Println("  --health      Print service health as JSON and exit")
	fmt.Println("Examples:")
	fmt.Println("  torrent_server my-id /etc/globular/torrent/config.json")
	fmt.Println("  torrent_server --describe")
	fmt.Println("  torrent_server --health")
}