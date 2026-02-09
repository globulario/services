package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
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
		Version:         cfg.Version,
		PublisherID:     cfg.PublisherID,
		Description:     cfg.Description,
		Keywords:        globular.CloneStringSlice(cfg.Keywords),
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

func main() {
	srv := initializeServerDefaults()
	args := os.Args[1:]

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

	for _, a := range args {
		if strings.ToLower(a) == "--debug" {
			logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
			break
		}
	}

	if globular.HandleInformationalFlags(srv, args, logger, printUsage) {
		return
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

	start := time.Now()
	if err := srv.Init(); err != nil {
		logger.Error("service init failed", "service", srv.Name, "id", srv.Id, "err", err)
		os.Exit(1)
	}

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

	lifecycle := globular.NewLifecycleManager(srv, logger)
	if err := lifecycle.Start(); err != nil {
		logger.Error("service start failed", "service", srv.Name, "id", srv.Id, "err", err)
		os.Exit(1)
	}
}
