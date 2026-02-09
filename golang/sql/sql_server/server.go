package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"

	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/sql/sql_client"
	"github.com/globulario/services/golang/sql/sqlpb"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc/reflection"

	// Drivers (keep as side-effect imports; add others as needed)
	_ "github.com/alexbrainman/odbc"
	_ "github.com/denisenkom/go-mssqldb"
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
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
		map[string]interface{}{"action": "/sql.SqlService/Stop", "permission": "admin", "resources": []interface{}{}},
		map[string]interface{}{
			"action":     "/sql.SqlService/CreateConnection",
			"permission": "admin",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "Connection.Id", "permission": "admin"},
			},
		},
		map[string]interface{}{
			"action":     "/sql.SqlService/DeleteConnection",
			"permission": "admin",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "Id", "permission": "admin"},
			},
		},
		map[string]interface{}{
			"action":     "/sql.SqlService/Ping",
			"permission": "read",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "Id", "permission": "read"},
			},
		},
		map[string]interface{}{
			"action":     "/sql.SqlService/QueryContext",
			"permission": "read",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "Query.ConnectionId", "permission": "read"},
			},
		},
		map[string]interface{}{
			"action":     "/sql.SqlService/ExecContext",
			"permission": "write",
			"resources": []interface{}{
				map[string]interface{}{"index": 0, "field": "Query.ConnectionId", "permission": "write"},
			},
		},
	}
}

func initializeServerDefaults() *server {
	cfg := DefaultConfig()
	s := &server{
		Name:            string(sqlpb.File_sql_proto.Services().Get(0).FullName()),
		Proto:           sqlpb.File_sql_proto.Path(),
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
	fmt.Println("  sql_server [service_id] [config_path]")
	fmt.Println("Options:")
	fmt.Println("  --describe    Print service metadata as JSON and exit")
	fmt.Println("  --health      Print service health as JSON and exit")
	fmt.Println("Examples:")
	fmt.Println("  sql_server my-sql-id /etc/globular/sql/config.json")
	fmt.Println("  sql_server --describe")
	fmt.Println("  sql_server --health")
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

	Utility.RegisterFunction("NewSqlService_Client", sql_client.NewSqlService_Client)

	start := time.Now()
	if err := srv.Init(); err != nil {
		logger.Error("service init failed", "service", srv.Name, "id", srv.Id, "err", err)
		os.Exit(1)
	}

	sqlpb.RegisterSqlServiceServer(srv.grpcServer, srv)
	reflection.Register(srv.grpcServer)

	logger.Info("service ready",
		"service", srv.Name,
		"port", srv.Port,
		"proxy", srv.Proxy,
		"protocol", srv.Protocol,
		"domain", srv.Domain,
		"listen_ms", time.Since(start).Milliseconds())

	lifecycle := globular.NewLifecycleManager(srv, logger)
	if err := lifecycle.Start(); err != nil {
		logger.Error("service start failed", "service", srv.Name, "id", srv.Id, "err", err)
		os.Exit(1)
	}
}
