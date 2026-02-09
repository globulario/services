package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"time"

	globular "github.com/globulario/services/golang/globular_service"
	"github.com/globulario/services/golang/mail/mail_client"
	"github.com/globulario/services/golang/mail/mailpb"
	Utility "github.com/globulario/utility"
	"google.golang.org/grpc/reflection"
)

var (
	defaultPort       = 10000
	defaultProxy      = defaultPort + 1
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
		map[string]interface{}{"action": "/mail.MailService/Send", "permission": "write"},
		map[string]interface{}{"action": "/mail.MailService/SendWithAttachments", "permission": "write"},
		map[string]interface{}{"action": "/mail.MailService/Stop", "permission": "write"},
	}
}

func initializeServerDefaults() *server {
	cfg := DefaultConfig()
	s := &server{
		Name:                string(mailpb.File_mail_proto.Services().Get(0).FullName()),
		Proto:               mailpb.File_mail_proto.Path(),
		Path:                func() string { p, _ := filepath.Abs(filepath.Dir(os.Args[0])); return p }(),
		Port:                cfg.Port,
		Proxy:               cfg.Proxy,
		Protocol:            cfg.Protocol,
		Version:             Version,
		PublisherID:         cfg.PublisherID,
		Description:         "Mail service with SMTP/SMTPS/IMAP/IMAPS servers for email sending and management",
		Keywords:            []string{"mail", "email", "smtp", "smtps", "imap", "imaps", "messaging", "notification"},
		Repositories:        globular.CloneStringSlice(cfg.Repositories),
		Discoveries:         globular.CloneStringSlice(cfg.Discoveries),
		AllowAllOrigins:     cfg.AllowAllOrigins,
		AllowedOrigins:      cfg.AllowedOrigins,
		KeepUpToDate:        cfg.KeepUpToDate,
		KeepAlive:           cfg.KeepAlive,
		Process:             -1,
		ProxyProcess:        -1,
		Dependencies:        globular.CloneStringSlice(cfg.Dependencies),
		Permissions:         loadDefaultPermissions(),
		Connections:         map[string]connection{},
		Persistence_address: cfg.PersistenceAddress,
		SMTP_Port:           cfg.SMTP_Port,
		SMTPS_Port:          cfg.SMTPS_Port,
		SMTP_ALT_Port:       cfg.SMTP_ALT_Port,
		IMAP_Port:           cfg.IMAP_Port,
		IMAPS_Port:          cfg.IMAPS_Port,
		IMAP_ALT_Port:       cfg.IMAP_ALT_Port,
		Password:            cfg.Password,
		DbIpV4:              cfg.DbIpV4,
		logger:              logger,
	}

	s.Domain, s.Address = globular.GetDefaultDomainAddress(s.Port)
	return s
}

func printUsage() {
	fmt.Println("Mail Service - Email sending and management with SMTP/IMAP")
	fmt.Println()
	fmt.Println("USAGE:")
	fmt.Println("  mail-service [OPTIONS] [id] [config_path]")
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
	fmt.Println("  • Email sending via SMTP/SMTPS relay")
	fmt.Println("  • Attachment support (SendWithAttachments)")
	fmt.Println("  • Embedded SMTP/SMTPS server support")
	fmt.Println("  • Embedded IMAP/IMAPS server support")
	fmt.Println("  • Multiple connection management")
	fmt.Println("  • Persistence integration for configuration storage")
	fmt.Println()
	fmt.Println("EXAMPLES:")
	fmt.Println("  # Start with default configuration")
	fmt.Println("  mail-service")
	fmt.Println()
	fmt.Println("  # Start with debug logging enabled")
	fmt.Println("  mail-service --debug")
	fmt.Println()
	fmt.Println("  # Check service version")
	fmt.Println("  mail-service --version")
	fmt.Println()
	fmt.Println("  # Start with custom service ID")
	fmt.Println("  mail-service mail-1")
	fmt.Println()
	fmt.Println("  # Start with custom domain via environment")
	fmt.Println("  GLOBULAR_DOMAIN=example.com mail-service")
}

func printVersion() {
	info := map[string]string{
		"service":    "mail",
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

	Utility.RegisterFunction("NewMailService_Client", mail_client.NewMailService_Client)

	// Log service start
	logger.Info("starting mail service",
		"service", srv.Name,
		"version", srv.Version,
		"domain", srv.Domain,
		"address", srv.Address,
		"smtp_port", srv.SMTP_Port,
		"smtps_port", srv.SMTPS_Port,
		"imap_port", srv.IMAP_Port,
		"imaps_port", srv.IMAPS_Port)

	start := time.Now()
	if err := srv.Init(); err != nil {
		logger.Error("service init failed", "service", srv.Name, "id", srv.Id, "err", err)
		os.Exit(1)
	}
	logger.Info("service initialized", "duration_ms", time.Since(start).Milliseconds())

	mailpb.RegisterMailServiceServer(srv.grpcServer, srv)
	reflection.Register(srv.grpcServer)
	logger.Debug("gRPC handlers registered")

	logger.Info("service ready",
		"service", srv.Name,
		"version", srv.Version,
		"port", srv.Port,
		"proxy", srv.Proxy,
		"protocol", srv.Protocol,
		"domain", srv.Domain,
		"startup_ms", time.Since(start).Milliseconds())

	lifecycle := globular.NewLifecycleManager(srv, logger)
	if err := lifecycle.Start(); err != nil {
		logger.Error("service start failed", "service", srv.Name, "err", err)
		os.Exit(1)
	}
}
