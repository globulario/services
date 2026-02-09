package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	"github.com/globulario/services/golang/config"
	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/storage/storage_client"
	"github.com/globulario/services/golang/storage/storagepb"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc/reflection"
)

var (
	defaultPort       = 10013
	defaultProxy      = 10014
	allowAllOrigins   = true
	allowedOriginsStr = ""
)

// Version information (set via ldflags during build)
var (
	Version   = "0.0.1"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

var logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

func loadDefaultPermissions() []interface{} {
	return []interface{}{
		map[string]interface{}{"action": "/storage.StorageService/Stop", "permission": "admin", "resources": []interface{}{}},
		map[string]interface{}{
			"action":     "/storage.StorageService/CreateConnection",
			"permission": "admin",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "Connection.Id", "permission": "admin"},
			},
		},
		map[string]interface{}{
			"action":     "/storage.StorageService/DeleteConnection",
			"permission": "admin",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "Id", "permission": "admin"},
			},
		},
		map[string]interface{}{
			"action":     "/storage.StorageService/Open",
			"permission": "read",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "Id", "permission": "read"},
			},
		},
		map[string]interface{}{
			"action":     "/storage.StorageService/Close",
			"permission": "write",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "Id", "permission": "write"},
			},
		},
		map[string]interface{}{
			"action":     "/storage.StorageService/SetItem",
			"permission": "write",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "Id", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "Key", "permission": "write"},
			},
		},
		map[string]interface{}{
			"action":     "/storage.StorageService/SetLargeItem",
			"permission": "write",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "Id", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "Key", "permission": "write"},
			},
		},
		map[string]interface{}{
			"action":     "/storage.StorageService/GetItem",
			"permission": "read",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "Id", "permission": "read"},
				map[string]interface{}{"index": 0, "field": "Key", "permission": "read"},
			},
		},
		map[string]interface{}{
			"action":     "/storage.StorageService/RemoveItem",
			"permission": "write",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "Id", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "Key", "permission": "write"},
			},
		},
		map[string]interface{}{
			"action":     "/storage.StorageService/Clear",
			"permission": "write",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "Id", "permission": "write"},
			},
		},
		map[string]interface{}{
			"action":     "/storage.StorageService/Drop",
			"permission": "admin",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "Id", "permission": "admin"},
			},
		},
	}
}

func initializeServerDefaults() *server {
	cfg := DefaultConfig()
	s := &server{
		Name:            string(storagepb.File_storage_proto.Services().Get(0).FullName()),
		Proto:           storagepb.File_storage_proto.Path(),
		Path:            func() string { p, _ := filepath.Abs(filepath.Dir(os.Args[0])); return p }(),
		Port:            cfg.Port,
		Proxy:           cfg.Proxy,
		Protocol:        cfg.Protocol,
		Version:         Version,
		PublisherID:     cfg.PublisherID,
		Description:     "Storage service with key-value store management and multiple backend support",
		Keywords:        []string{"storage", "kv", "keyvalue", "database", "badger", "scylla", "persistence"},
		Repositories:    globular.CloneStringSlice(cfg.Repositories),
		Discoveries:     globular.CloneStringSlice(cfg.Discoveries),
		AllowAllOrigins: cfg.AllowAllOrigins,
		AllowedOrigins:  cfg.AllowedOrigins,
		KeepUpToDate:    cfg.KeepUpToDate,
		KeepAlive:       cfg.KeepAlive,
		Process:         -1,
		ProxyProcess:    -1,
		Dependencies:    globular.CloneStringSlice(cfg.Dependencies),
		Permissions:     loadDefaultPermissions(),
		Connections:     map[string]connection{},
	}

	s.Domain, s.Address = globular.GetDefaultDomainAddress(s.Port)
	return s
}

func printUsage() {
	fmt.Println("Storage Service - Key-value store with multiple backend support")
	fmt.Println()
	fmt.Println("USAGE:")
	fmt.Println("  storage-service [OPTIONS] [id] [config_path]")
	fmt.Println()
	fmt.Println("OPTIONS:")
	fmt.Println("  --debug       Enable debug logging")
	fmt.Println("  --describe    Print service description as JSON and exit")
	fmt.Println("  --health      Print service health status as JSON and exit")
	fmt.Println("  --version     Print version information as JSON and exit")
	fmt.Println("  --help        Show this help message and exit")
	fmt.Println()
	fmt.Println("POSITIONAL ARGUMENTS:")
	fmt.Println("  id            Optional service instance ID")
	fmt.Println("  config_path   Optional configuration file path")
	fmt.Println()
	fmt.Println("ENVIRONMENT VARIABLES:")
	fmt.Println("  GLOBULAR_DOMAIN    Override service domain")
	fmt.Println("  GLOBULAR_ADDRESS   Override service address (host:port)")
	fmt.Println()
	fmt.Println("FEATURES:")
	fmt.Println("  • Multiple storage backend support (Badger, ScyllaDB)")
	fmt.Println("  • Connection management with Open/Close operations")
	fmt.Println("  • Key-value operations (Set, Get, Remove)")
	fmt.Println("  • Bulk operations (Clear, Drop)")
	fmt.Println("  • Large item support for big values")
	fmt.Println("  • RBAC permissions (admin, read, write)")
	fmt.Println()
	fmt.Println("EXAMPLES:")
	fmt.Println("  # Start with default configuration")
	fmt.Println("  storage-service")
	fmt.Println()
	fmt.Println("  # Start with debug logging enabled")
	fmt.Println("  storage-service --debug")
	fmt.Println()
	fmt.Println("  # Check service version")
	fmt.Println("  storage-service --version")
	fmt.Println()
	fmt.Println("  # Start with custom service ID")
	fmt.Println("  storage-service my-storage-id")
	fmt.Println()
	fmt.Println("  # Start with custom domain via environment")
	fmt.Println("  GLOBULAR_DOMAIN=example.com storage-service")
}

func printVersion() {
	info := map[string]string{
		"service":    "storage",
		"version":    Version,
		"build_time": BuildTime,
		"git_commit": GitCommit,
	}
	data, _ := json.MarshalIndent(info, "", "  ")
	fmt.Println(string(data))
}

func main() {
	srv := initializeServerDefaults()

	// Define CLI flags
	var (
		enableDebug  = flag.Bool("debug", false, "enable debug logging")
		showVersion  = flag.Bool("version", false, "print version information as JSON and exit")
		showHelp     = flag.Bool("help", false, "show usage information and exit")
		showDescribe = flag.Bool("describe", false, "print service description as JSON and exit")
		showHealth   = flag.Bool("health", false, "print service health status as JSON and exit")
	)

	flag.Usage = printUsage
	flag.Parse()

	// Enable debug logging if requested
	if *enableDebug {
		logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
		logger.Debug("debug logging enabled")
	}

	// Handle informational flags
	if *showHelp {
		printUsage()
		return
	}

	if *showVersion {
		printVersion()
		return
	}

	if *showDescribe {
		data, _ := json.MarshalIndent(srv, "", "  ")
		fmt.Println(string(data))
		return
	}

	if *showHealth {
		health := map[string]interface{}{
			"service": srv.Name,
			"status":  "healthy",
			"version": srv.Version,
		}
		data, _ := json.MarshalIndent(health, "", "  ")
		fmt.Println(string(data))
		return
	}

	// Handle port allocation and positional arguments
	args := flag.Args()

	if len(args) == 0 {
		if srv.Id == "" {
			srv.Id = Utility.GenerateUUID(srv.Name + ":" + srv.Address)
		}
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
		srv.Proxy = p + 1
		srv.Domain, srv.Address = globular.GetDefaultDomainAddress(srv.Port)
	}

	globular.ParsePositionalArgs(srv, args)

	if err := globular.AllocatePortIfNeeded(srv, args); err != nil {
		logger.Error("port allocation failed", "error", err)
		os.Exit(1)
	}

	globular.LoadRuntimeConfig(srv)

	if srv.Id == "" {
		srv.Id = Utility.GenerateUUID(srv.Name + ":" + srv.Address)
	}

	if srv.Domain == "" || srv.Address == "" {
		srv.Domain, srv.Address = globular.GetDefaultDomainAddress(srv.Port)
	}

	Utility.RegisterFunction("NewStorageService_Client", storage_client.NewStorageService_Client)

	// Log service start
	logger.Info("starting storage service",
		"service", srv.Name,
		"version", srv.Version,
		"id", srv.Id,
		"domain", srv.Domain,
		"address", srv.Address)

	start := time.Now()
	if err := srv.Init(); err != nil {
		logger.Error("service init failed", "service", srv.Name, "id", srv.Id, "err", err)
		os.Exit(1)
	}
	logger.Info("service initialized", "duration_ms", time.Since(start).Milliseconds())

	storagepb.RegisterStorageServiceServer(srv.grpcServer, srv)
	reflection.Register(srv.grpcServer)
	logger.Debug("gRPC handlers registered")

	logger.Info("service ready",
		"service", srv.Name,
		"version", srv.Version,
		"id", srv.Id,
		"domain", srv.Domain,
		"address", srv.Address,
		"port", srv.Port,
		"proxy", srv.Proxy,
		"protocol", srv.Protocol,
		"startup_ms", time.Since(start).Milliseconds())

	lifecycle := globular.NewLifecycleManager(srv, logger)
	if err := lifecycle.Start(); err != nil {
		logger.Error("service start failed", "service", srv.Name, "id", srv.Id, "err", err)
		os.Exit(1)
	}
}
