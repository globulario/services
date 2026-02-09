package main

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
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
		Version:             cfg.Version,
		PublisherID:         cfg.PublisherID,
		Description:         cfg.Description,
		Keywords:            globular.CloneStringSlice(cfg.Keywords),
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
	exe := filepath.Base(os.Args[0])
	os.Stdout.WriteString(`
Usage: ` + exe + ` [options] <id> [configPath]

Options:
  --describe      Print service description as JSON (no etcd/config access)
  --health        Print service health as JSON (no etcd/config access)

Arguments:
  <id>            Service instance ID
  [configPath]    Optional path to configuration file
`)
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

	Utility.RegisterFunction("NewMailService_Client", mail_client.NewMailService_Client)

	start := time.Now()
	if err := srv.Init(); err != nil {
		logger.Error("service init failed", "service", srv.Name, "id", srv.Id, "err", err)
		os.Exit(1)
	}

	mailpb.RegisterMailServiceServer(srv.grpcServer, srv)
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
		logger.Error("service start failed", "service", srv.Name, "err", err)
		os.Exit(1)
	}
}
