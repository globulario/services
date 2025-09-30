// Package main implements the Storage gRPC service wired for Globular.
// It follows the same structure and CLI behavior as the Echo example:
// - slog for structured logging
// - --describe and --health flags handled BEFORE any config/etcd calls
// - optional positional args: [service_id] [config_path]
package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	// NOTE: we import config but call it only AFTER handling --describe/--health
	"github.com/globulario/services/golang/config"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/resource/resourcepb"
	Utility "github.com/globulario/utility"

	"github.com/globulario/services/golang/storage/storage_client"
	"github.com/globulario/services/golang/storage/storage_store"
	"github.com/globulario/services/golang/storage/storagepb"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

// --- defaults & CORS ---
var (
	defaultPort        = 10013
	defaultProxy       = 10014
	allowAllOrigins    = true
	allowedOriginsList = ""
)

// global logger
var logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
	Level: slog.LevelInfo,
}))

// connection holds info about an opened store
type connection struct {
	Id   string
	Name string
	Type storagepb.StoreType
}

// server implements the Storage service + Globular service hooks.
type server struct {
	// Core metadata / config
	Id              string
	Name            string
	Mac             string
	Domain          string
	Address         string
	Path            string
	Proto           string
	Port            int
	Proxy           int
	Protocol        string
	Version         string
	PublisherID     string
	Description     string
	Keywords        []string
	Repositories    []string
	Discoveries     []string
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

	// Storage runtime
	Connections map[string]connection
	stores      map[string]storage_store.Store
	storeLocks  sync.Map // map[id]*sync.Mutex
}

// --- Globular contract: getters/setters ---

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
func (srv *server) SetModTime(modtime int64)         { srv.ModTime = modtime }
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
func (srv *server) GetChecksum() string              { return srv.Checksum }
func (srv *server) SetChecksum(cs string)            { srv.Checksum = cs }
func (srv *server) GetPlatform() string              { return srv.Plaform }
func (srv *server) SetPlatform(p string)             { srv.Plaform = p }
func (srv *server) GetPath() string                  { return srv.Path }
func (srv *server) SetPath(path string)              { srv.Path = path }
func (srv *server) GetProto() string                 { return srv.Proto }
func (srv *server) SetProto(proto string)            { srv.Proto = proto }
func (srv *server) GetPort() int                     { return srv.Port }
func (srv *server) SetPort(port int)                 { srv.Port = port }
func (srv *server) GetProxy() int                    { return srv.Proxy }
func (srv *server) SetProxy(proxy int)               { srv.Proxy = proxy }
func (srv *server) GetProtocol() string              { return srv.Protocol }
func (srv *server) SetProtocol(p string)             { srv.Protocol = p }
func (srv *server) GetAllowAllOrigins() bool         { return srv.AllowAllOrigins }
func (srv *server) SetAllowAllOrigins(b bool)        { srv.AllowAllOrigins = b }
func (srv *server) GetAllowedOrigins() string        { return srv.AllowedOrigins }
func (srv *server) SetAllowedOrigins(s string)       { srv.AllowedOrigins = s }
func (srv *server) GetDomain() string                { return srv.Domain }
func (srv *server) SetDomain(d string)               { srv.Domain = d }
func (srv *server) GetTls() bool                     { return srv.TLS }
func (srv *server) SetTls(b bool)                    { srv.TLS = b }
func (srv *server) GetCertAuthorityTrust() string    { return srv.CertAuthorityTrust }
func (srv *server) SetCertAuthorityTrust(ca string)  { srv.CertAuthorityTrust = ca }
func (srv *server) GetCertFile() string              { return srv.CertFile }
func (srv *server) SetCertFile(cf string)            { srv.CertFile = cf }
func (srv *server) GetKeyFile() string               { return srv.KeyFile }
func (srv *server) SetKeyFile(kf string)             { srv.KeyFile = kf }
func (srv *server) GetVersion() string               { return srv.Version }
func (srv *server) SetVersion(v string)              { srv.Version = v }
func (srv *server) GetPublisherID() string           { return srv.PublisherID }
func (srv *server) SetPublisherID(pid string)        { srv.PublisherID = pid }
func (srv *server) GetKeepUpToDate() bool            { return srv.KeepUpToDate }
func (srv *server) SetKeepUptoDate(val bool)         { srv.KeepUpToDate = val }
func (srv *server) GetKeepAlive() bool               { return srv.KeepAlive }
func (srv *server) SetKeepAlive(val bool)            { srv.KeepAlive = val }
func (srv *server) GetPermissions() []interface{}    { return srv.Permissions }
func (srv *server) SetPermissions(p []interface{})   { srv.Permissions = p }
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

// Dist packages the service for distribution via Globular.
func (srv *server) Dist(path string) (string, error) { return globular.Dist(path, srv) }

// RolesDefault returns the default roles defined by this service.
func (srv *server) RolesDefault() []resourcepb.Role {
	domain, _ := config.GetDomain()

	return []resourcepb.Role{
		{
			Id:          "role:storage.viewer",
			Name:        "Storage Viewer",
			Domain:      domain,
			Description: "Read-only: open stores and read items.",
			Actions: []string{
				"/storage.StorageService/Open",
				"/storage.StorageService/GetItem",
			},
			TypeName: "resource.Role",
		},
		{
			Id:          "role:storage.writer",
			Name:        "Storage Writer",
			Domain:      domain,
			Description: "Read and write items; can close/clear stores.",
			Actions: []string{
				"/storage.StorageService/Open",
				"/storage.StorageService/Close",
				"/storage.StorageService/GetItem",
				"/storage.StorageService/SetItem",
				"/storage.StorageService/SetLargeItem",
				"/storage.StorageService/RemoveItem",
				"/storage.StorageService/Clear",
			},
			TypeName: "resource.Role",
		},
		{
			Id:          "role:storage.admin",
			Name:        "Storage Admin",
			Domain:      domain,
			Description: "Full control over storage connections and stores.",
			Actions: []string{
				"/storage.StorageService/Stop",
				"/storage.StorageService/CreateConnection",
				"/storage.StorageService/DeleteConnection",
				"/storage.StorageService/Open",
				"/storage.StorageService/Close",
				"/storage.StorageService/GetItem",
				"/storage.StorageService/SetItem",
				"/storage.StorageService/SetLargeItem",
				"/storage.StorageService/RemoveItem",
				"/storage.StorageService/Clear",
				"/storage.StorageService/Drop",
			},
			TypeName: "resource.Role",
		},
	}
}

// Init prepares config/runtime and initializes the gRPC server.
func (srv *server) Init() error {
	srv.stores = make(map[string]storage_store.Store)
	srv.Connections = make(map[string]connection)

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

// StopService gracefully stops the gRPC server.
func (srv *server) StopService() error { return globular.StopService(srv, srv.grpcServer) }

// Stop is the gRPC endpoint to stop the service.
func (srv *server) Stop(context.Context, *storagepb.StopRequest) (*storagepb.StopResponse, error) {
	return &storagepb.StopResponse{}, srv.StopService()
}

// --- main entrypoint ---

func main() {
	srv := new(server)

	// Fill ONLY fields that don’t touch etcd/config yet.
	srv.Name = string(storagepb.File_storage_proto.Services().Get(0).FullName())
	srv.Proto = storagepb.File_storage_proto.Path()
	srv.Path, _ = filepath.Abs(filepath.Dir(os.Args[0]))
	srv.Port = defaultPort
	srv.Proxy = defaultProxy
	srv.Protocol = "grpc"
	srv.Version = "0.0.1"
	srv.PublisherID = "localhost"
	srv.Description = "Storage service."
	srv.Keywords = []string{"Storage", "KV", "Blob"}
	srv.Repositories = make([]string, 0)
	srv.Discoveries = make([]string, 0)
	srv.Dependencies = []string{ "rbac.RbacService", "log.LogService" }
	srv.Permissions = []interface{}{
		// ---- Stop the storage service
		map[string]interface{}{
			"action":     "/storage.StorageService/Stop",
			"permission": "admin",
			"resources":  []interface{}{},
		},

		// ---- Create a storage connection
		map[string]interface{}{
			"action":     "/storage.StorageService/CreateConnection",
			"permission": "admin",
			"resources": []interface{}{
				// CreateConnectionRqst.connection.id
				map[string]interface{}{"index": 0, "field": "Connection.Id", "permission": "admin"},
			},
		},

		// ---- Delete a storage connection
		map[string]interface{}{
			"action":     "/storage.StorageService/DeleteConnection",
			"permission": "admin",
			"resources": []interface{}{
				// DeleteConnectionRqst.id
				map[string]interface{}{"index": 0, "field": "Id", "permission": "admin"},
			},
		},

		// ---- Open a store (required before read/write ops)
		map[string]interface{}{
			"action":     "/storage.StorageService/Open",
			"permission": "read",
			"resources": []interface{}{
				// OpenRqst.id
				map[string]interface{}{"index": 0, "field": "Id", "permission": "read"},
			},
		},

		// ---- Close a store
		map[string]interface{}{
			"action":     "/storage.StorageService/Close",
			"permission": "write",
			"resources": []interface{}{
				// CloseRqst.id
				map[string]interface{}{"index": 0, "field": "Id", "permission": "write"},
			},
		},

		// ---- Set a small item (key/value)
		map[string]interface{}{
			"action":     "/storage.StorageService/SetItem",
			"permission": "write",
			"resources": []interface{}{
				// SetItemRequest.id
				map[string]interface{}{"index": 0, "field": "Id", "permission": "write"},
				// SetItemRequest.key
				map[string]interface{}{"index": 0, "field": "Key", "permission": "write"},
			},
		},

		// ---- Set a large item (streaming)
		map[string]interface{}{
			"action":     "/storage.StorageService/SetLargeItem",
			"permission": "write",
			"resources": []interface{}{
				// SetLargeItemRequest.id
				map[string]interface{}{"index": 0, "field": "Id", "permission": "write"},
				// SetLargeItemRequest.key
				map[string]interface{}{"index": 0, "field": "Key", "permission": "write"},
			},
		},

		// ---- Get an item (streaming response)
		map[string]interface{}{
			"action":     "/storage.StorageService/GetItem",
			"permission": "read",
			"resources": []interface{}{
				// GetItemRequest.id
				map[string]interface{}{"index": 0, "field": "Id", "permission": "read"},
				// GetItemRequest.key
				map[string]interface{}{"index": 0, "field": "Key", "permission": "read"},
			},
		},

		// ---- Remove a specific item
		map[string]interface{}{
			"action":     "/storage.StorageService/RemoveItem",
			"permission": "write",
			"resources": []interface{}{
				// RemoveItemRequest.id
				map[string]interface{}{"index": 0, "field": "Id", "permission": "write"},
				// RemoveItemRequest.key
				map[string]interface{}{"index": 0, "field": "Key", "permission": "write"},
			},
		},

		// ---- Clear all items from a store
		map[string]interface{}{
			"action":     "/storage.StorageService/Clear",
			"permission": "write",
			"resources": []interface{}{
				// ClearRequest.id
				map[string]interface{}{"index": 0, "field": "Id", "permission": "write"},
			},
		},

		// ---- Drop (destroy) a store
		map[string]interface{}{
			"action":     "/storage.StorageService/Drop",
			"permission": "admin",
			"resources": []interface{}{
				// DropRequest.id
				map[string]interface{}{"index": 0, "field": "Id", "permission": "admin"},
			},
		},
	}

	srv.Process = -1
	srv.ProxyProcess = -1
	srv.AllowAllOrigins = allowAllOrigins
	srv.AllowedOrigins = allowedOriginsList
	srv.KeepAlive = true
	srv.KeepUpToDate = true

	// Register client ctor for dynamic wiring
	Utility.RegisterFunction("NewStorageService_Client", storage_client.NewStorageService_Client)

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
			// Provide runtime info without reading external config.
			srv.Process = os.Getpid()
			srv.State = "starting"

			// Fill Domain/Address with safe defaults or env hints.
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
				ServiceName: "", // overall
			})
			if err != nil {
				logger.Error("health error", "service", srv.Name, "id", srv.Id, "err", err)
				os.Exit(2)
			}
			_, _ = os.Stdout.Write(b)
			_, _ = os.Stdout.Write([]byte("\n"))
			return
		case "--debug":
			logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
		case "--help", "-h", "/?":
			printUsage()
			return
		case "--version", "-v":
			logger.Info(srv.Name + " version " + srv.Version)
			return
		default:
			// skip unknown flags for now (e.g. positional args
		}
	}

	// Optional positional args (unchanged)
	if len(args) == 1 && !strings.HasPrefix(args[0], "-") {
		srv.Id = args[0]
	} else if len(args) == 2 && !strings.HasPrefix(args[0], "-") && !strings.HasPrefix(args[1], "-") {
		srv.Id = args[0]
		srv.ConfigPath = args[1]
	}

	// Now safe to read local config (may hit etcd or file fallback)
	if d, err := config.GetDomain(); err == nil && strings.TrimSpace(d) != "" {
		srv.Domain = d
	} else if srv.Domain == "" {
		srv.Domain = "localhost"
	}
	if a, err := config.GetAddress(); err == nil && strings.TrimSpace(a) != "" {
		srv.Address = a
	}

	start := time.Now()
	if err := srv.Init(); err != nil {
		logger.Error("service init failed", "service", srv.Name, "id", srv.Id, "err", err)
		os.Exit(1)
	}

	// Register gRPC service and reflection
	storagepb.RegisterStorageServiceServer(srv.grpcServer, srv)
	reflection.Register(srv.grpcServer)

	logger.Info("service ready",
		"service", srv.Name,
		"id", srv.Id,
		"domain", srv.Domain,
		"address", srv.Address,
		"port", srv.Port,
		"proxy", srv.Proxy,
		"protocol", srv.Protocol,
		"listen_ms", time.Since(start).Milliseconds())

	if err := srv.StartService(); err != nil {
		logger.Error("service start failed", "service", srv.Name, "id", srv.Id, "err", err)
		os.Exit(1)
	}
}

// printUsage matches the style of the Echo example.
func printUsage() {
	fmt.Println("Usage:")
	fmt.Println("  storage_server [service_id] [config_path]")
	fmt.Println("Options:")
	fmt.Println("  --describe    Print service metadata as JSON and exit")
	fmt.Println("  --health      Print service health as JSON and exit")
	fmt.Println("Examples:")
	fmt.Println("  storage_server my-storage-id /etc/globular/storage/config.json")
	fmt.Println("  storage_server --describe")
	fmt.Println("  storage_server --health")
}
