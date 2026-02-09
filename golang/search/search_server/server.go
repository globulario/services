package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/search/searchpb"
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
		map[string]interface{}{"action": "/search.SearchService/Stop", "permission": "admin"},
		map[string]interface{}{"action": "/search.SearchService/GetEngineVersion", "permission": "read"},
		map[string]interface{}{"action": "/search.SearchService/IndexJsonObject", "permission": "write"},
		map[string]interface{}{"action": "/search.SearchService/Count", "permission": "read"},
		map[string]interface{}{"action": "/search.SearchService/DeleteDocument", "permission": "write"},
		map[string]interface{}{"action": "/search.SearchService/SearchDocuments", "permission": "read"},
	}
}

func initializeServerDefaults() *server {
	cfg := DefaultConfig()
	s := &server{
		Name:            string(searchpb.File_search_proto.Services().Get(0).FullName()),
		Proto:           searchpb.File_search_proto.Path(),
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
		Dependencies:    []string{},
		Permissions:     loadDefaultPermissions(),
	}

	s.Domain, s.Address = globular.GetDefaultDomainAddress(s.Port)
	return s
}

func setupGrpc(s *server) {
	searchpb.RegisterSearchServiceServer(s.grpcServer, s)
	reflection.Register(s.grpcServer)
}

func printUsage() {
	fmt.Fprintf(os.Stdout, `
Usage: %s [options] <id> [configPath]

Options:
  --describe   Print service description as JSON (no etcd/config access)
  --health     Print service health as JSON (no etcd/config access)

`, filepath.Base(os.Args[0]))
}

func main() {
	srv := initializeServerDefaults()

	args := os.Args[1:]
	for _, a := range args {
		if strings.ToLower(a) == "--debug" {
			logger = slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelDebug}))
			break
		}
	}

	if globular.HandleInformationalFlags(srv, args, logger, printUsage) {
		return
	}

	if err := globular.AllocatePortIfNeeded(srv, args); err != nil {
		logger.Error("port allocation failed", "error", err)
		os.Exit(1)
	}

	globular.ParsePositionalArgs(srv, args)
	globular.LoadRuntimeConfig(srv)

	if srv.Domain == "" {
		srv.Domain = "localhost"
	}
	if srv.Address == "" {
		srv.Address = fmt.Sprintf("localhost:%d", srv.Port)
	}

	start := time.Now()
	if err := srv.Init(); err != nil {
		logger.Error("service init failed", "service", srv.Name, "id", srv.Id, "err", err)
		os.Exit(1)
	}

	setupGrpc(srv)

	logger.Info("service ready",
		"service", srv.Name,
		"port", srv.Port,
		"proxy", srv.Proxy,
		"protocol", srv.Protocol,
		"domain", srv.Domain,
		"listen_ms", time.Since(start).Milliseconds())

	lifecycle := globular.NewLifecycleManager(srv, logger)
	if err := lifecycle.Start(); err != nil {
		logger.Error("service start failed", "service", srv.Name, "err", err)
		os.Exit(1)
	}
}
