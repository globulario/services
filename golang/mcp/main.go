// Package main implements a Globular MCP server that exposes read-only
// operator tools over stdio for AI assistants (Claude Code).
//
// Usage:
//
//	globular-mcp-server
//
// The server communicates via JSON-RPC 2.0 over stdin/stdout.
// All tools are strictly read-only / diagnostic / preview.
package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/globulario/services/golang/config"
)

func main() {
	// Suppress log output to stderr (MCP uses stdout for JSON-RPC).
	log.SetOutput(os.Stderr)
	log.SetFlags(log.Ltime | log.Lshortfile)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg := loadConfig()
	if !cfg.Enabled {
		log.Println("globular-mcp-server: disabled by config")
		return
	}

	initAuditLog(cfg)

	srv := newServer(cfg)
	registerAllTools(srv)

	toolCount := len(srv.tools)
	log.Printf("globular-mcp-server: starting (tools=%d, read_only=%v, transport=%s)",
		toolCount, cfg.ReadOnly, cfg.Transport)

	switch cfg.Transport {
	case "http":
		addr := cfg.HTTPListenAddr
		if addr == "" {
			// Port must come from etcd — no hardcoded default.
			sc, err := config.GetServiceConfigurationById("mcp.McpService")
			if err != nil {
				log.Fatalf("mcp: cannot determine listen port: service not configured in etcd: %v", err)
			}
			p, ok := sc["Port"].(float64)
			if !ok || p <= 0 {
				log.Fatalf("mcp: etcd config for mcp.McpService has no valid Port field")
			}
			addr = fmt.Sprintf("0.0.0.0:%d", int(p))
		}
		if err := srv.serveHTTP(ctx, addr); err != nil {
			log.Fatalf("globular-mcp-server: %v", err)
		}
	default: // "stdio"
		if err := srv.serveStdio(ctx); err != nil {
			log.Fatalf("globular-mcp-server: %v", err)
		}
	}
}
