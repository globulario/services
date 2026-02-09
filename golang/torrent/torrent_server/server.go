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
	"github.com/globulario/services/golang/torrent/torrent_client"
	"github.com/globulario/services/golang/torrent/torrentpb"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc/reflection"
)

var (
	defaultPort       = 10000
	defaultProxy      = defaultPort + 1
	allowAllOrigins   = true
	allowedOriginsStr = ""
)

var logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo}))

func loadDefaultPermissions() []interface{} {
	return []interface{}{
		map[string]interface{}{
			"action":     "/torrent.TorrentService/DownloadTorrent",
			"permission": "write",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "Link", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "Dest", "permission": "write"},
				map[string]interface{}{"index": 0, "field": "Seed", "permission": "write"},
			},
		},
		map[string]interface{}{
			"action":     "/torrent.TorrentService/GetTorrentInfos",
			"permission": "read",
			"resources":  []interface{}{},
		},
		map[string]interface{}{
			"action":     "/torrent.TorrentService/DropTorrent",
			"permission": "write",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "Name", "permission": "write"},
			},
		},
		map[string]interface{}{
			"action":     "/torrent.TorrentService/GetTorrentLnks",
			"permission": "read",
			"resources":  []interface{}{},
		},
	}
}

func initializeServerDefaults() *server {
	cfg := DefaultConfig()
	s := &server{
		Name:            string(torrentpb.File_torrent_proto.Services().Get(0).FullName()),
		Proto:           torrentpb.File_torrent_proto.Path(),
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
		DownloadDir:     cfg.DownloadDir,
		Seed:            cfg.Seed,
		UseMinio:        cfg.UseMinio,
		MinioEndpoint:   cfg.MinioEndpoint,
		MinioAccessKey:  cfg.MinioAccessKey,
		MinioSecretKey:  cfg.MinioSecretKey,
		MinioBucket:     cfg.MinioBucket,
		MinioPrefix:     cfg.MinioPrefix,
		MinioUseSSL:     cfg.MinioUseSSL,
	}

	s.Domain, s.Address = globular.GetDefaultDomainAddress(s.Port)
	return s
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

func main() {
	srv := initializeServerDefaults()
	args := os.Args[1:]

	if len(args) == 0 {
		if srv.Id == "" {
			srv.Id = Utility.GenerateUUID(srv.Name + ":" + srv.Address)
		}
		if allocator, err := config.NewDefaultPortAllocator(); err == nil {
			if p, err := allocator.Next(srv.Id); err == nil {
				srv.Port = p
			}
		}
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

	if srv.Domain == "" || srv.Address == "" {
		srv.Domain, srv.Address = globular.GetDefaultDomainAddress(srv.Port)
	}
	if srv.Id == "" {
		srv.Id = Utility.GenerateUUID(srv.Name + ":" + srv.Address)
	}

	Utility.RegisterFunction("NewTorrentService_Client", torrent_client.NewTorrentService_Client)

	start := time.Now()
	if err := srv.Init(); err != nil {
		logger.Error("service init failed", "service", srv.Name, "id", srv.Id, "err", err)
		os.Exit(1)
	}

	torrentpb.RegisterTorrentServiceServer(srv.grpcServer, srv)
	reflection.Register(srv.grpcServer)

	logger.Info("service ready",
		"service", srv.Name,
		"port", srv.Port,
		"proxy", srv.Proxy,
		"protocol", srv.Protocol,
		"domain", srv.Domain,
		"download_dir", srv.DownloadDir,
		"listen_ms", time.Since(start).Milliseconds(),
	)

	lifecycle := globular.NewLifecycleManager(srv, logger)
	if err := lifecycle.Start(); err != nil {
		logger.Error("service start failed", "service", srv.Name, "id", srv.Id, "err", err)
		os.Exit(1)
	}
}
